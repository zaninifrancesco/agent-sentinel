package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/zaninifrancesco/agent-sentinel/internal/cursorhooks"
)

// Cursor hook events Sentinel listens to. Shell, file reads, file edits and the
// end of a turn (it closes the calls Cursor never reported back on) are
// always installed; the MCP pair is opt-in because an MCP server that already
// goes through `sentinel mcp` would be recorded twice.
var (
	coreEvents = []string{cursorhooks.EventBeforeShell, cursorhooks.EventAfterShell, cursorhooks.EventBeforeRead, cursorhooks.EventAfterEdit, cursorhooks.EventStop}
	mcpEvents  = []string{cursorhooks.EventBeforeMCP, cursorhooks.EventAfterMCP}
)

// ours matches the command of an entry Sentinel wrote: "<path>/sentinel hook",
// quoted or not, with or without flags (and for a binary renamed sentinel-dev).
var ours = regexp.MustCompile(`sentinel[\w.-]*['"]?\s+hook(\s|$)`)

// hooksFile is a hooks.json that keeps everything it does not understand.
type hooksFile struct {
	Version json.RawMessage
	Hooks   map[string][]json.RawMessage
	Extra   map[string]json.RawMessage
}

func parseHooksFile(data []byte) (*hooksFile, error) {
	f := &hooksFile{Version: json.RawMessage("1"), Hooks: map[string][]json.RawMessage{}, Extra: map[string]json.RawMessage{}}
	if len(bytes.TrimSpace(data)) == 0 {
		return f, nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}
	for k, v := range top {
		switch k {
		case "version":
			f.Version = v
		case "hooks":
			if err := json.Unmarshal(v, &f.Hooks); err != nil {
				return nil, fmt.Errorf(`"hooks" is not an object of arrays: %w`, err)
			}
		default:
			f.Extra[k] = v
		}
	}
	return f, nil
}

// marshal writes version, then hooks, then whatever else the file had.
func (f *hooksFile) marshal() []byte {
	var b bytes.Buffer
	b.WriteString("{\n")
	fmt.Fprintf(&b, "  \"version\": %s", f.Version)
	if len(f.Hooks) > 0 {
		hb, _ := json.MarshalIndent(f.Hooks, "  ", "  ")
		fmt.Fprintf(&b, ",\n  \"hooks\": %s", hb)
	}
	keys := make([]string, 0, len(f.Extra))
	for k := range f.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		kb, _ := json.Marshal(k)
		var v bytes.Buffer
		_ = json.Indent(&v, f.Extra[k], "  ", "  ")
		fmt.Fprintf(&b, ",\n  %s: %s", kb, v.String())
	}
	b.WriteString("\n}\n")
	return b.Bytes()
}

func isOurs(entry json.RawMessage) bool {
	var e struct {
		Command string `json:"command"`
	}
	return json.Unmarshal(entry, &e) == nil && ours.MatchString(e.Command)
}

// removeOurs drops Sentinel's entries from every event and returns how many.
func (f *hooksFile) removeOurs() int {
	n := 0
	for ev, entries := range f.Hooks {
		kept := entries[:0:0]
		for _, e := range entries {
			if isOurs(e) {
				n++
				continue
			}
			kept = append(kept, e)
		}
		if len(kept) == 0 {
			delete(f.Hooks, ev)
		} else {
			f.Hooks[ev] = kept
		}
	}
	return n
}

// installOurs replaces Sentinel's entries with fresh ones for events, leaving
// every other hook as it was.
func (f *hooksFile) installOurs(binary string, events []string) {
	f.removeOurs()
	command := shellQuote(binary) + " hook --mode observe"
	for _, ev := range events {
		entry := map[string]any{"command": command, "timeout": hookProcessTimeoutSec}
		b, _ := json.Marshal(entry)
		f.Hooks[ev] = append(f.Hooks[ev], b)
	}
}

func shellQuote(s string) string {
	if !strings.ContainsAny(s, " \t'\"$`\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// runHookInstall implements `sentinel hook install` and `uninstall`.
func runHookInstall(args []string, install bool) int {
	name := "install"
	if !install {
		name = "uninstall"
	}
	fs := flag.NewFlagSet("hook "+name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	agent := fs.String("agent", "cursor", "which agent to record: cursor (.cursor/hooks.json) or claude (.claude/settings.local.json)")
	user := fs.Bool("user", false, "use the agent's user-level file (~/.cursor/hooks.json, ~/.claude/settings.json: every project) instead of this project's")
	dir := fs.String("dir", ".", "project directory (ignored with --user)")
	mcp := fs.Bool("mcp", false, "also record MCP tool calls made through the agent (skip it for servers already behind `sentinel mcp`)")
	fs.String("offline", "", "removed: observation hooks always acknowledge neutrally")
	capture := addCaptureFlags(fs)
	dry := fs.Bool("dry-run", false, "print the resulting file and write nothing")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if err := capture.validate(fs); err != nil {
		fmt.Fprintf(os.Stderr, "sentinel hook %s: %v\n", name, err)
		return 2
	}
	removedOffline := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "offline" {
			removedOffline = true
		}
	})
	if removedOffline || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "sentinel hook: remove --offline and positional arguments; the observation release never asks or denies")
		return 2
	}

	switch *agent {
	case "cursor":
	case "claude":
		return runClaudeHookInstall(install, *user, *dir, *mcp, *dry)
	default:
		fmt.Fprintf(os.Stderr, "sentinel hook %s: --agent must be cursor or claude, not %q\n", name, *agent)
		return 2
	}

	path, err := hooksPath(*user, *dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 1
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "sentinel: cannot read %s: %v\n", path, err)
		return 1
	}
	f, err := parseHooksFile(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %s is %v; not touching it\n", path, err)
		return 1
	}

	if install {
		bin, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sentinel: cannot find my own path: %v\n", err)
			return 1
		}
		if bin, err = filepath.Abs(bin); err != nil {
			fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
			return 1
		}
		events := coreEvents
		if *mcp {
			events = append(append([]string{}, coreEvents...), mcpEvents...)
		}
		f.installOurs(bin, events)
	} else if f.removeOurs() == 0 {
		fmt.Fprintf(os.Stderr, "sentinel: no Sentinel hooks in %s\n", path)
		return 0
	}

	out := f.marshal()
	if *dry {
		os.Stdout.Write(out)
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 1
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: cannot write %s: %v\n", path, err)
		return 1
	}
	if install {
		fmt.Fprintf(os.Stderr, "sentinel: hooks written to %s (other hooks left as they were).\n", path)
		fmt.Fprintf(os.Stderr, "Observation hooks do not block commands, even offline. Start `sentinel serve` to record; Cursor reloads hooks.json on save.\n")
	} else {
		fmt.Fprintf(os.Stderr, "sentinel: Sentinel hooks removed from %s.\n", path)
	}
	return 0
}

func hooksPath(user bool, dir string) (string, error) {
	if user {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cursor", "hooks.json"), nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, ".cursor", "hooks.json"), nil
}
