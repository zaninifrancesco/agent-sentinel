package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The project's own hooks.json already holds another tool's hook (Impeccable).
// Installing must add ours next to it and uninstalling must leave it alone.
const existing = `{
  "version": 1,
  "hooks": {
    "preToolUse": [
      { "command": "'/Users/me/.cursor/skills/impeccable/scripts/impeccable' hook-before-edit", "timeout": 5 }
    ],
    "afterFileEdit": [
      { "command": "./format.sh" }
    ]
  },
  "somethingElse": { "keep": true }
}`

func commandsOf(t *testing.T, f *hooksFile, event string) []string {
	t.Helper()
	var out []string
	for _, e := range f.Hooks[event] {
		var h struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(e, &h); err != nil {
			t.Fatal(err)
		}
		out = append(out, h.Command)
	}
	return out
}

func TestInstallKeepsOtherHooksAndIsIdempotent(t *testing.T) {
	f, err := parseHooksFile([]byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	f.installOurs("/opt/sentinel", coreEvents)
	f.installOurs("/opt/sentinel", coreEvents) // run it twice

	if got := commandsOf(t, f, "preToolUse"); len(got) != 1 || !strings.Contains(got[0], "impeccable") {
		t.Fatalf("another tool's hook was disturbed: %v", got)
	}
	after := commandsOf(t, f, "afterFileEdit")
	if len(after) != 2 || after[0] != "./format.sh" || after[1] != "/opt/sentinel hook --mode observe" {
		t.Fatalf("afterFileEdit = %v", after)
	}
	for _, ev := range coreEvents {
		n := 0
		for _, c := range commandsOf(t, f, ev) {
			if strings.HasSuffix(c, "sentinel hook --mode observe") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s has %d Sentinel entries after two installs", ev, n)
		}
	}
	if _, ok := f.Extra["somethingElse"]; !ok {
		t.Fatal("unknown top-level keys must survive")
	}
	if !strings.Contains(string(f.Hooks["beforeShellExecution"][0]), `"timeout":6`) {
		t.Fatalf("observation hook must have a short process timeout: %s", f.Hooks["beforeShellExecution"][0])
	}
}

func TestUninstallRemovesOnlyOurs(t *testing.T) {
	f, _ := parseHooksFile([]byte(existing))
	f.installOurs("/opt/sentinel dev/sentinel", append(append([]string{}, coreEvents...), mcpEvents...))
	if n := f.removeOurs(); n != 7 {
		t.Fatalf("removed %d entries, want 7 (quoted path and a flag included)", n)
	}
	if _, ok := f.Hooks["beforeShellExecution"]; ok {
		t.Fatal("empty events must be dropped")
	}
	if got := commandsOf(t, f, "afterFileEdit"); len(got) != 1 || got[0] != "./format.sh" {
		t.Fatalf("afterFileEdit = %v", got)
	}
	if got := commandsOf(t, f, "preToolUse"); len(got) != 1 {
		t.Fatalf("preToolUse = %v", got)
	}
}

func TestMarshalIsValidJSONWithVersionFirst(t *testing.T) {
	f, _ := parseHooksFile([]byte(existing))
	f.installOurs("/opt/sentinel", coreEvents)
	out := string(f.marshal())
	if !strings.HasPrefix(out, "{\n  \"version\": 1") {
		t.Fatalf("version should come first:\n%s", out)
	}
	var back map[string]any
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	// Reading our own output back finds the same thing.
	again, err := parseHooksFile([]byte(out))
	if err != nil || len(again.Hooks) != len(f.Hooks) {
		t.Fatalf("round trip failed: %v", err)
	}
}

func TestInstallIntoMissingOrEmptyFile(t *testing.T) {
	f, err := parseHooksFile(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.installOurs("/opt/sentinel", coreEvents)
	if !strings.Contains(string(f.marshal()), "beforeShellExecution") {
		t.Fatal("hooks missing from a fresh file")
	}
	if _, err := parseHooksFile([]byte("{not json")); err == nil {
		t.Fatal("a broken hooks.json must be refused, not overwritten")
	}
}

func TestNeutralAnswers(t *testing.T) {
	for event, want := range map[string]string{
		"beforeShellExecution": "{\"permission\":\"allow\"}\n",
		"beforeReadFile":       "{\"permission\":\"allow\"}\n",
		"beforeMCPExecution":   "{\"permission\":\"allow\"}\n",
		"afterShellExecution":  "{}\n", "afterMCPExecution": "{}\n", "stop": "{}\n",
	} {
		if got := string(neutralAnswer([]byte(`{"hook_event_name":"` + event + `"}`))); got != want {
			t.Errorf("%s: %q, want %q", event, got, want)
		}
	}
	if string(neutralAnswer([]byte("garbage"))) != "{}\n" {
		t.Fatal("unreadable input must not produce a decision")
	}
}

func TestShellQuote(t *testing.T) {
	if shellQuote("/opt/sentinel") != "/opt/sentinel" {
		t.Fatal("plain paths stay plain")
	}
	if got := shellQuote("/My Apps/it's/sentinel"); got != `'/My Apps/it'\''s/sentinel'` {
		t.Fatalf("got %s", got)
	}
}

// The whole command against a real directory: the file is created, another
// tool's hook survives an install and an uninstall, and a broken file is left
// alone.
func TestInstallCommandOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cursor", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Skipf("cannot create a .cursor directory here: %v", err)
	}
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := runHookInstall([]string{"--dir", dir, "--mcp"}, true); code != 0 {
		t.Fatalf("install exited %d", code)
	}
	if code := runHookInstall([]string{"--dir", dir, "--mcp"}, true); code != 0 {
		t.Fatalf("second install exited %d", code)
	}
	data, _ := os.ReadFile(path)
	f, err := parseHooksFile(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range append(append([]string{}, coreEvents...), mcpEvents...) {
		n := 0
		for _, c := range commandsOf(t, f, ev) {
			if ours.MatchString(c) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: %d Sentinel entries after installing twice", ev, n)
		}
	}
	if got := commandsOf(t, f, "preToolUse"); len(got) != 1 || !strings.Contains(got[0], "impeccable") {
		t.Fatalf("another tool's hook was lost: %v", got)
	}

	if code := runHookInstall([]string{"--dir", dir}, false); code != 0 {
		t.Fatalf("uninstall exited %d", code)
	}
	data, _ = os.ReadFile(path)
	f, _ = parseHooksFile(data)
	if len(f.Hooks) != 2 || len(commandsOf(t, f, "afterFileEdit")) != 1 || len(commandsOf(t, f, "preToolUse")) != 1 {
		t.Fatalf("uninstall must leave exactly the other tools' hooks, got %v", f.Hooks)
	}

	// A file we cannot parse is never overwritten.
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runHookInstall([]string{"--dir", dir}, true); code == 0 {
		t.Fatal("install over a broken file must fail")
	}
	if data, _ := os.ReadFile(path); string(data) != "{not json" {
		t.Fatalf("the broken file was modified: %q", data)
	}
}

func TestShrinkReadKeepsTheSizeAndOnlyTouchesReads(t *testing.T) {
	big := strings.Repeat("é", maxForward) // two bytes each: well over the cap
	in, _ := json.Marshal(map[string]any{"hook_event_name": "beforeReadFile", "file_path": "/p/big", "content": big})
	out := shrinkRead(in)
	var got struct {
		Content      string `json:"content"`
		ContentBytes int    `json:"content_bytes"`
		FilePath     string `json:"file_path"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Content) > maxForward || got.ContentBytes != len(big) || got.FilePath != "/p/big" {
		t.Fatalf("content %d bytes, content_bytes %d, path %q", len(got.Content), got.ContentBytes, got.FilePath)
	}
	if strings.ContainsRune(got.Content, '\uFFFD') {
		t.Fatal("a multi-byte character was cut in half")
	}

	small := []byte(`{"hook_event_name":"beforeReadFile","file_path":"/p/a","content":"hi"}`)
	if string(shrinkRead(small)) != string(small) {
		t.Fatal("a small read must pass untouched")
	}
	shellIn, _ := json.Marshal(map[string]any{"hook_event_name": "afterShellExecution", "output": big})
	if string(shrinkRead(shellIn)) != string(shellIn) {
		t.Fatal("other hooks must pass untouched")
	}
	if string(shrinkRead([]byte("garbage"))) != "garbage" {
		t.Fatal("unreadable input must pass untouched")
	}
}

func TestNeutralReadAnswerStaysInsideTheReadSchema(t *testing.T) {
	var got map[string]string
	if err := json.Unmarshal(neutralAnswer([]byte(`{"hook_event_name":"beforeReadFile"}`)), &got); err != nil || len(got) != 1 || got["permission"] != "allow" {
		t.Fatalf("invalid read acknowledgement: %v (%v)", got, err)
	}
}

func TestInstallRejectsOldEnforcementOptionsWithoutWriting(t *testing.T) {
	for _, agent := range []string{"cursor", "claude"} {
		for _, option := range [][]string{{"--offline", "deny"}, {"--approval-timeout", "30s"}, {"--policy", "off"}} {
			dir := t.TempDir()
			args := append([]string{"--agent", agent, "--dir", dir}, option...)
			if code := runHookInstall(args, true); code != 2 {
				t.Fatalf("%v exited %d, want migration error", args, code)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("rejected install changed project: %v, %v", entries, err)
			}
		}
	}
}
