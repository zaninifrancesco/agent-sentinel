package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/zaninifrancesco/agent-sentinel/internal/cursorhooks"
)

const defaultServePort = 8848

// runHook implements `sentinel hook`, the command Cursor runs for each agent
// step. It forwards Cursor's JSON from stdin to the running `sentinel serve`
// and prints the answer, which Cursor reads as the permission decision.
//
// When Sentinel cannot answer (not running, wrong port, an error) it must not
// silently wave commands through, and it must not brick Cursor either: by
// default it answers "ask", which hands the decision back to Cursor's own
// confirmation. --offline changes that.
func runHook(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "install":
			return runHookInstall(args[1:], true)
		case "uninstall":
			return runHookInstall(args[1:], false)
		}
	}

	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	url := fs.String("url", "", "where `sentinel serve` listens (default $SENTINEL_URL or http://127.0.0.1:8848)")
	offline := fs.String("offline", "ask", "what to answer when Sentinel cannot be reached: ask, deny or allow")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	switch *offline {
	case "ask", "deny", "allow":
	default:
		fmt.Fprintf(os.Stderr, "sentinel hook: --offline must be ask, deny or allow, not %q\n", *offline)
		return 2
	}
	target := *url
	if target == "" {
		target = os.Getenv("SENTINEL_URL")
	}
	if target == "" {
		target = fmt.Sprintf("http://127.0.0.1:%d", defaultServePort)
	}

	// A file Cursor is about to read comes with its whole content.
	body, err := io.ReadAll(io.LimitReader(os.Stdin, 64<<20))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel hook: cannot read input: %v\n", err)
		return 1
	}

	out, err := forwardHook(strings.TrimRight(target, "/"), shrinkRead(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel hook: %v\n", err)
		out = offlineAnswer(body, *offline)
	}
	os.Stdout.Write(out)
	return 0
}

// forwardHook asks the running Sentinel. There is no overall deadline of our
// own: a held command legitimately waits for a human, and Cursor stops the hook
// itself when its configured timeout passes.
func forwardHook(base string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, base+"/api/hook", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Sentinel is not reachable at %s (is `sentinel serve` running?): %w", base, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Sentinel answered %d: %s", resp.StatusCode, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// maxForward is how much of a file's content is sent to Sentinel, enough to
// look for secrets without pushing megabytes through the hook for every read.
const maxForward = 256 << 10

// shrinkRead cuts the content of a beforeReadFile input down to maxForward,
// keeping the real size in content_bytes. Any other input passes untouched.
func shrinkRead(body []byte) []byte {
	var in map[string]any
	if json.Unmarshal(body, &in) != nil || in["hook_event_name"] != "beforeReadFile" {
		return body
	}
	content, _ := in["content"].(string)
	if len(content) <= maxForward {
		return body
	}
	cut := content[:maxForward]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	in["content"], in["content_bytes"] = cut, len(content)
	out, err := json.Marshal(in)
	if err != nil {
		return body
	}
	return out
}

// offlineAnswer is what Cursor is told when Sentinel could not decide. Only
// the "before" hooks carry a decision; the "after" ones just get an empty
// object.
//
// Reading a file can only be allowed or denied: Cursor has no "ask" for it, and
// a response outside its schema blocks the read. So with the default mode
// ("ask") an unreachable Sentinel lets reads through, while commands still go
// back to the user; --offline deny refuses both.
func offlineAnswer(input []byte, mode string) []byte {
	var in struct {
		Event string `json:"hook_event_name"`
	}
	_ = json.Unmarshal(input, &in)
	if in.Event == cursorhooks.EventClaudePre {
		return claudeOfflineAnswer(mode)
	}
	if !strings.HasPrefix(in.Event, "before") {
		return []byte("{}\n")
	}
	if in.Event == "beforeReadFile" {
		if mode == "deny" {
			return []byte(`{"permission":"deny","user_message":"Agent Sentinel could not be reached, so this file was not read."}` + "\n")
		}
		return []byte(`{"permission":"allow"}` + "\n")
	}
	resp := map[string]string{"permission": mode}
	switch mode {
	case "ask":
		resp["user_message"] = "Agent Sentinel could not be reached, so nothing checked this. Confirm it yourself."
		resp["agent_message"] = "Agent Sentinel is not running; the call needs the user's own confirmation."
	case "deny":
		resp["user_message"] = "Agent Sentinel could not be reached, so this call was refused."
		resp["agent_message"] = "Refused: Agent Sentinel is not running and the hook is set to fail closed. The call was NOT executed."
	}
	b, _ := json.Marshal(resp)
	return append(b, '\n')
}

// claudeOfflineAnswer is what Claude Code is told when Sentinel could not decide
// about a tool call. Silence would let the call through unchecked and unnoticed,
// so by default Claude Code is told to ask the user, which it does even in a
// mode that would otherwise approve the call by itself.
func claudeOfflineAnswer(mode string) []byte {
	if mode == "allow" {
		return []byte("{}\n")
	}
	d := map[string]string{"hookEventName": cursorhooks.EventClaudePre, "permissionDecision": mode}
	if mode == "deny" {
		d["permissionDecisionReason"] = "Agent Sentinel is not running and the hook is set to fail closed. The call was NOT executed."
	} else {
		d["permissionDecisionReason"] = "Agent Sentinel could not be reached, so nothing checked this call."
	}
	b, _ := json.Marshal(map[string]any{"hookSpecificOutput": d})
	return append(b, '\n')
}
