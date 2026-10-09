package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/cursorhooks"
)

// Claude Code keeps its hooks inside its settings file, next to permissions,
// model, env and whatever else the user put there
// (https://code.claude.com/docs/en/hooks#configuration):
//
//	{"hooks": {"PreToolUse": [{"matcher": "...", "hooks": [{"type": "command", ...}]}]}}
//
// So unlike hooks.json, the file is the user's, and installing must touch
// nothing but Sentinel's own entries, keys and their order included.

// claudeSettingsPath is where the hooks go: the project's personal settings
// (.claude/settings.local.json, which Claude Code keeps out of git) or, with
// user, the settings of every project.
func claudeSettingsPath(user bool, dir string) (string, error) {
	if user {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".claude", "settings.json"), nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, ".claude", "settings.local.json"), nil
}

// orderedObject is a JSON object that remembers the order of its keys and
// leaves every value it does not touch exactly as it was.
type orderedObject struct {
	keys  []string
	vals  map[string]json.RawMessage
	fresh map[string]bool // keys written by us: indented, where the others are kept as read
}

func parseOrdered(data []byte) (*orderedObject, error) {
	o := &orderedObject{vals: map[string]json.RawMessage{}, fresh: map[string]bool{}}
	if len(bytes.TrimSpace(data)) == 0 {
		return o, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil {
		return nil, err
	} else if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errors.New("not a JSON object")
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := kt.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		o.put(key, raw)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *orderedObject) put(key string, v json.RawMessage) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = v
}

// set writes a value of ours.
func (o *orderedObject) set(key string, v json.RawMessage) {
	o.put(key, v)
	o.fresh[key] = true
}

func (o *orderedObject) remove(key string) {
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i:i], o.keys[i+1:]...)
			return
		}
	}
}

func (o *orderedObject) marshal() []byte {
	if len(o.keys) == 0 {
		return []byte("{}\n")
	}
	var b bytes.Buffer
	b.WriteString("{\n")
	for i, k := range o.keys {
		kb, _ := json.Marshal(k)
		v := bytes.NewBuffer(nil)
		if !o.fresh[k] || json.Indent(v, o.vals[k], "  ", "  ") != nil {
			v.Reset()
			v.Write(o.vals[k]) // as the user wrote it
		}
		fmt.Fprintf(&b, "  %s: %s", kb, v.String())
		if i < len(o.keys)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return b.Bytes()
}

// claudeHooks reads the "hooks" key: events, each a list of matcher groups.
func (o *orderedObject) claudeHooks() (map[string][]json.RawMessage, error) {
	hooks := map[string][]json.RawMessage{}
	if raw, ok := o.vals["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return nil, fmt.Errorf(`"hooks" is not an object of arrays: %w`, err)
		}
	}
	return hooks, nil
}

func (o *orderedObject) setClaudeHooks(hooks map[string][]json.RawMessage) {
	if len(hooks) == 0 {
		o.remove("hooks")
		return
	}
	b, _ := json.Marshal(hooks)
	o.set("hooks", b)
}

type claudeHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type claudeGroup struct {
	Matcher string          `json:"matcher,omitempty"`
	Hooks   []claudeHandler `json:"hooks"`
}

// removeClaudeOurs drops Sentinel's handlers, and the matcher groups they leave
// empty, and returns how many handlers it removed. Groups that are not
// Sentinel's are returned byte for byte.
func removeClaudeOurs(hooks map[string][]json.RawMessage) int {
	n := 0
	for ev, groups := range hooks {
		kept := groups[:0:0]
		for _, g := range groups {
			var parsed struct {
				Hooks []json.RawMessage `json:"hooks"`
			}
			if json.Unmarshal(g, &parsed) != nil {
				kept = append(kept, g)
				continue
			}
			var left []json.RawMessage
			for _, h := range parsed.Hooks {
				if isOurs(h) {
					n++
				} else {
					left = append(left, h)
				}
			}
			switch {
			case len(left) == len(parsed.Hooks):
				kept = append(kept, g)
			case len(left) > 0: // a group shared with someone else's handlers
				var whole map[string]json.RawMessage
				_ = json.Unmarshal(g, &whole)
				whole["hooks"], _ = json.Marshal(left)
				rewritten, _ := json.Marshal(whole)
				kept = append(kept, rewritten)
			}
		}
		if len(kept) == 0 {
			delete(hooks, ev)
		} else {
			hooks[ev] = kept
		}
	}
	return n
}

// installClaudeOurs replaces Sentinel's handlers with fresh ones.
//
// The call that waits for a human gets the approval wait plus a margin (Claude
// Code cancels a hook at its timeout, and a cancelled PreToolUse lets the call
// through to Claude's own prompt). The others never wait for anyone.
func installClaudeOurs(hooks map[string][]json.RawMessage, binary string, approvalTimeout time.Duration, offline string, mcp bool) {
	removeClaudeOurs(hooks)
	command := shellQuote(binary) + " hook"
	if offline != "ask" {
		command += " --offline " + offline
	}
	add := func(event, matcher string, timeoutSec int) {
		g, _ := json.Marshal(claudeGroup{Matcher: matcher, Hooks: []claudeHandler{{Type: "command", Command: command, Timeout: timeoutSec}}})
		hooks[event] = append(hooks[event], g)
	}
	tools := cursorhooks.ClaudeToolMatcher(mcp)
	add(cursorhooks.EventClaudePre, tools, int((approvalTimeout + 15*time.Second).Seconds()))
	add(cursorhooks.EventClaudePost, tools, 30)
	add(cursorhooks.EventClaudePostFailure, tools, 30)
	add(cursorhooks.EventClaudeStop, "", 30)
	add(cursorhooks.EventClaudeSessionEnd, "", 10)
}

// runClaudeHookInstall implements `sentinel hook install --agent claude` and
// the matching uninstall.
func runClaudeHookInstall(install, user bool, dir, offline string, approvalTimeout time.Duration, mcp, dry bool) int {
	path, err := claudeSettingsPath(user, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 1
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "sentinel: cannot read %s: %v\n", path, err)
		return 1
	}
	settings, err := parseOrdered(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %s is not valid JSON (%v); not touching it\n", path, err)
		return 1
	}
	hooks, err := settings.claudeHooks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %s: %v; not touching it\n", path, err)
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
		installClaudeOurs(hooks, bin, approvalTimeout, offline, mcp)
	} else if removeClaudeOurs(hooks) == 0 {
		fmt.Fprintf(os.Stderr, "sentinel: no Sentinel hooks in %s\n", path)
		return 0
	}
	settings.setClaudeHooks(hooks)

	out := settings.marshal()
	if dry {
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
		fmt.Fprintf(os.Stderr, "sentinel: Claude Code hooks written to %s (everything else in the file left as it was).\n", path)
		fmt.Fprintf(os.Stderr, "Start the cockpit with `sentinel serve`, then start `claude` in this project (it reads hooks when a session starts).\n")
	} else {
		fmt.Fprintf(os.Stderr, "sentinel: Sentinel hooks removed from %s.\n", path)
	}
	return 0
}
