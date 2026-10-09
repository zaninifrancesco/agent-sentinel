package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A settings file as a Claude Code user really has it: other keys, in an order
// they chose, and a hook of their own on an event Sentinel also uses.
const claudeSettings = `{
  "model": "opus",
  "permissions": {
    "allow": ["Bash(npm test)"],
    "deny": ["Read(./secrets/**)"]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "./check-bash.sh"}]
      }
    ],
    "Notification": [
      {"hooks": [{"type": "command", "command": "notify-send hi"}]}
    ]
  },
  "env": {"FOO": "bar"}
}
`

func readClaudeSettings(t *testing.T, path string) (*orderedObject, map[string][]json.RawMessage) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	o, err := parseOrdered(data)
	if err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, data)
	}
	hooks, err := o.claudeHooks()
	if err != nil {
		t.Fatal(err)
	}
	return o, hooks
}

func claudeCommands(t *testing.T, hooks map[string][]json.RawMessage, event string) []string {
	t.Helper()
	var out []string
	for _, g := range hooks[event] {
		var group struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		}
		if err := json.Unmarshal(g, &group); err != nil {
			t.Fatal(err)
		}
		for _, h := range group.Hooks {
			out = append(out, h.Command)
		}
	}
	return out
}

func countOurs(cmds []string) int {
	n := 0
	for _, c := range cmds {
		if ours.MatchString(c) {
			n++
		}
	}
	return n
}

func TestClaudeInstallAddsOnlySentinelsEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Skipf("cannot create a .claude directory here: %v", err)
	}
	if err := os.WriteFile(path, []byte(claudeSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ { // twice: the second must replace, not duplicate
		if code := runHookInstall([]string{"--agent", "claude", "--dir", dir}, true); code != 0 {
			t.Fatalf("install exited %d", code)
		}
	}

	o, hooks := readClaudeSettings(t, path)
	for _, ev := range []string{"PreToolUse", "PostToolUse", "PostToolUseFailure", "Stop", "SessionEnd"} {
		if n := countOurs(claudeCommands(t, hooks, ev)); n != 1 {
			t.Errorf("%s: %d Sentinel entries after installing twice", ev, n)
		}
	}
	if got := claudeCommands(t, hooks, "PreToolUse"); len(got) != 2 || got[0] != "./check-bash.sh" {
		t.Fatalf("the user's own hook must stay first and untouched: %v", got)
	}
	if got := claudeCommands(t, hooks, "Notification"); len(got) != 1 || got[0] != "notify-send hi" {
		t.Fatalf("an event Sentinel does not use was changed: %v", got)
	}
	if strings.Join(o.keys, ",") != "model,permissions,hooks,env" {
		t.Fatalf("the user's keys were reordered or lost: %v", o.keys)
	}
	if !strings.Contains(string(o.vals["permissions"]), "Read(./secrets/**)") || string(o.vals["env"]) != `{"FOO": "bar"}` {
		t.Fatalf("other settings were rewritten: %s / %s", o.vals["permissions"], o.vals["env"])
	}

	// What Claude Code needs from each entry.
	var pre struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(hooks["PreToolUse"][1], &pre); err != nil {
		t.Fatal(err)
	}
	if pre.Matcher != `^(Bash|PowerShell|Read|Edit|Write|MultiEdit|NotebookEdit|Glob|Grep|WebFetch|WebSearch)$` {
		t.Fatalf("matcher = %q (MCP is opt-in)", pre.Matcher)
	}
	if len(pre.Hooks) != 1 || pre.Hooks[0].Type != "command" || pre.Hooks[0].Timeout != 135 {
		t.Fatalf("the gate must wait out the approval (120s + 15s): %+v", pre.Hooks)
	}
	if strings.Contains(string(hooks["Stop"][0]), `"matcher"`) {
		t.Fatalf("Stop takes no matcher: %s", hooks["Stop"][0])
	}

	// Uninstall leaves exactly what the user had.
	if code := runHookInstall([]string{"--agent", "claude", "--dir", dir}, false); code != 0 {
		t.Fatalf("uninstall exited %d", code)
	}
	o, hooks = readClaudeSettings(t, path)
	if got := claudeCommands(t, hooks, "PreToolUse"); len(got) != 1 || got[0] != "./check-bash.sh" {
		t.Fatalf("uninstall must leave the user's hook: %v", got)
	}
	for _, ev := range []string{"PostToolUse", "PostToolUseFailure", "Stop", "SessionEnd"} {
		if _, ok := hooks[ev]; ok {
			t.Errorf("%s should be gone, it only held Sentinel's entry", ev)
		}
	}
	if strings.Join(o.keys, ",") != "model,permissions,hooks,env" {
		t.Fatalf("keys after uninstall: %v", o.keys)
	}
}

func TestClaudeInstallOptions(t *testing.T) {
	dir := t.TempDir()
	if code := runHookInstall([]string{"--agent", "claude", "--dir", dir, "--mcp", "--offline", "deny", "--approval-timeout", "30s"}, true); code != 0 {
		t.Fatalf("install exited %d", code)
	}
	path := filepath.Join(dir, ".claude", "settings.local.json")
	_, hooks := readClaudeSettings(t, path)
	first := string(hooks["PreToolUse"][0])
	for _, want := range []string{`mcp__.*`, `--offline deny`, `"timeout": 45`} {
		if !strings.Contains(first, want) {
			t.Errorf("PreToolUse entry lacks %q: %s", want, first)
		}
	}

	// A file that does not exist is created; one we cannot parse is left alone.
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runHookInstall([]string{"--agent", "claude", "--dir", dir}, true); code == 0 {
		t.Fatal("install over a broken file must fail")
	}
	if data, _ := os.ReadFile(path); string(data) != "{not json" {
		t.Fatalf("the broken file was modified: %q", data)
	}
	if code := runHookInstall([]string{"--agent", "nobody", "--dir", dir}, true); code != 2 {
		t.Fatalf("an unknown agent must be refused, exited %d", code)
	}
}

func TestClaudeUninstallOfAFileWithoutSentinelChangesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Skipf("cannot create a .claude directory here: %v", err)
	}
	if err := os.WriteFile(path, []byte(claudeSettings), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runHookInstall([]string{"--agent", "claude", "--dir", dir}, false); code != 0 {
		t.Fatalf("exited %d", code)
	}
	if data, _ := os.ReadFile(path); string(data) != claudeSettings {
		t.Fatalf("the file must be byte for byte the same:\n%s", data)
	}
}

func TestClaudeOfflineAnswers(t *testing.T) {
	pre := []byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash"}`)
	for mode, want := range map[string]string{"ask": "ask", "deny": "deny"} {
		var got struct {
			Out struct {
				Event    string `json:"hookEventName"`
				Decision string `json:"permissionDecision"`
				Reason   string `json:"permissionDecisionReason"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(offlineAnswer(pre, mode), &got); err != nil || got.Out.Decision != want || got.Out.Event != "PreToolUse" || got.Out.Reason == "" {
			t.Errorf("%s: %+v (%v)", mode, got, err)
		}
	}
	if string(offlineAnswer(pre, "allow")) != "{}\n" {
		t.Fatal("--offline allow says nothing, so Claude Code's own rules apply")
	}
	for _, ev := range []string{"PostToolUse", "PostToolUseFailure", "Stop", "SessionEnd"} {
		if string(offlineAnswer([]byte(`{"hook_event_name":"`+ev+`"}`), "deny")) != "{}\n" {
			t.Errorf("%s has no decision to make", ev)
		}
	}
}
