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

	body, err := io.ReadAll(io.LimitReader(os.Stdin, 8<<20))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel hook: cannot read input: %v\n", err)
		return 1
	}

	out, err := forwardHook(strings.TrimRight(target, "/"), body)
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

// offlineAnswer is what Cursor is told when Sentinel could not decide. Only
// the "before" hooks carry a decision; the "after" ones just get an empty
// object.
func offlineAnswer(input []byte, mode string) []byte {
	var in struct {
		Event string `json:"hook_event_name"`
	}
	_ = json.Unmarshal(input, &in)
	if !strings.HasPrefix(in.Event, "before") {
		return []byte("{}\n")
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
