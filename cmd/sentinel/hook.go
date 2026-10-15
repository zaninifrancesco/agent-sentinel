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
	"time"
	"unicode/utf8"

	"github.com/zaninifrancesco/agent-sentinel/internal/cursorhooks"
)

const defaultServePort = 8848
const defaultHookTimeout = time.Second
const maxHookTimeout = 5 * time.Second
const hookProcessTimeoutSec = 6
const maxHookInput = 64 << 20

// runHook records one event and always returns the agent's neutral response.
// The collector's availability or reply never changes the agent's permissions.
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
	url := fs.String("url", "", "collector URL (default $SENTINEL_URL or http://127.0.0.1:8848)")
	timeout := fs.Duration("timeout", defaultHookTimeout, "maximum time to deliver telemetry (up to 5s)")
	fs.String("offline", "", "removed: hooks now acknowledge neutrally even offline; reinstall Sentinel hooks")
	capture := addCaptureFlags(fs)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if err := capture.validate(fs); err != nil {
		fmt.Fprintf(os.Stderr, "sentinel hook: %v\n", err)
		return 2
	}
	removedOffline := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "offline" {
			removedOffline = true
		}
	})
	if removedOffline {
		fmt.Fprintln(os.Stderr, "sentinel hook: --offline was removed; reinstall Sentinel hooks. Observation mode never asks or denies, and your agent's own permissions still apply.")
		return 2
	}
	if fs.NArg() != 0 || *timeout <= 0 || *timeout > maxHookTimeout {
		fmt.Fprintln(os.Stderr, "sentinel hook: expected no positional arguments and a --timeout greater than 0 and at most 5s")
		return 2
	}
	target := *url
	if target == "" {
		target = os.Getenv("SENTINEL_URL")
	}
	if target == "" {
		target = fmt.Sprintf("http://127.0.0.1:%d", defaultServePort)
	}

	body, err := io.ReadAll(io.LimitReader(os.Stdin, maxHookInput+1))
	if err != nil || len(body) > maxHookInput {
		fmt.Fprintln(os.Stderr, "sentinel hook: telemetry input could not be captured completely; event was not delivered")
		os.Stdout.Write(neutralAnswer(body))
		return 0
	}
	out, err := observeHook(strings.TrimRight(target, "/"), body, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel hook: telemetry not recorded: %v\n", err)
	}
	os.Stdout.Write(out)
	return 0
}

func observeHook(base string, body []byte, timeout time.Duration) ([]byte, error) {
	reply, err := forwardHook(base, shrinkRead(body), timeout)
	if err == nil {
		var ack struct {
			Permission         string `json:"permission"`
			HookSpecificOutput *struct {
				Decision string `json:"permissionDecision"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal(reply, &ack) != nil {
			err = fmt.Errorf("collector returned an invalid acknowledgement")
		} else if ack.Permission == "deny" || ack.Permission == "ask" || (ack.HookSpecificOutput != nil && ack.HookSpecificOutput.Decision != "") {
			err = fmt.Errorf("collector returned a legacy permission decision; restart it with the observation release")
		}
	}
	return neutralAnswer(body), err
}

// The deadline covers connection, response headers and response body. A
// collector that stalls must not hold up the coding agent indefinitely.
func forwardHook(base string, body []byte, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, base+"/api/hook", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collector unreachable: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(out) > 1<<20 {
		return nil, fmt.Errorf("collector acknowledgement exceeds 1 MiB")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("collector answered HTTP %d", resp.StatusCode)
	}
	return out, nil
}

// maxForward is how much of a file's content is sent to Sentinel, enough to
// limit telemetry overhead without forwarding the entire file for every read.
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

// neutralAnswer adds no decision for Claude Code or after-hooks. Cursor's
// before hooks require their non-blocking acknowledgement, including file reads.
func neutralAnswer(input []byte) []byte {
	var in struct {
		Event string `json:"hook_event_name"`
	}
	_ = json.Unmarshal(input, &in)
	switch in.Event {
	case cursorhooks.EventBeforeShell, cursorhooks.EventBeforeRead, cursorhooks.EventBeforeMCP:
		return []byte("{\"permission\":\"allow\"}\n")
	default:
		return []byte("{}\n")
	}
}
