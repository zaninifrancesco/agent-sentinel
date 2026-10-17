package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func projectRoot(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
	}
	t.Fatal("could not find project root containing go.mod")
	return ""
}

func TestClaudePluginManifestValid(t *testing.T) {
	root := projectRoot(t)
	path := filepath.Join(root, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing plugin manifest: %v", err)
	}
	var manifest struct {
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Description string   `json:"description"`
		Skills      []string `json:"skills"`
		Hooks       string   `json:"hooks"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("plugin.json is not valid JSON: %v", err)
	}
	if manifest.Name != "agent-sentinel" {
		t.Errorf("expected name 'agent-sentinel', got %q", manifest.Name)
	}
	if len(manifest.Skills) == 0 {
		t.Errorf("plugin manifest has no skills registered")
	}
	if manifest.Hooks == "" {
		t.Errorf("plugin manifest has no hooks registered")
	}
}

func TestClaudeMarketplaceValid(t *testing.T) {
	root := projectRoot(t)
	path := filepath.Join(root, ".claude-plugin", "marketplace.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing marketplace manifest: %v", err)
	}
	var marketplace struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &marketplace); err != nil {
		t.Fatalf("marketplace.json is not valid JSON: %v", err)
	}
	if len(marketplace.Plugins) == 0 {
		t.Errorf("marketplace has no plugins")
	}
}

func TestHooksJSONStructure(t *testing.T) {
	root := projectRoot(t)
	path := filepath.Join(root, "hooks", "hooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing hooks.json: %v", err)
	}
	var hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hooks); err != nil {
		t.Fatalf("hooks.json is not valid JSON: %v", err)
	}
	required := []string{"PreToolUse", "PostToolUse", "PostToolUseFailure", "Stop", "SessionEnd"}
	for _, req := range required {
		if _, ok := hooks[req]; !ok {
			t.Errorf("hooks.json missing required event %s", req)
		}
	}
}

func TestClaudeHookScriptExecution(t *testing.T) {
	root := projectRoot(t)
	script := filepath.Join(root, "scripts", "claude-hook.sh")
	fi, err := os.Stat(script)
	if err != nil {
		t.Fatalf("cannot stat %s: %v", script, err)
	}
	if fi.Mode()&0111 == 0 {
		t.Fatalf("%s is not executable", script)
	}

	// Claude Code expects JSON on stdout; stderr is reserved for diagnostics.
	// 1. With sentinel in environment / PATH: stdout must be "{}" even if collector is offline.
	cmd := exec.Command(script)
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"PreToolUse"}`)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if trimmed := strings.TrimSpace(string(stdout)); trimmed != "{}" {
		t.Errorf("expected '{}' on stdout, got %q", trimmed)
	}

	// 2. Fallback when sentinel is completely missing:
	cmdFallback := exec.Command(script)
	cmdFallback.Env = []string{"SENTINEL_BIN=/nonexistent/sentinel", "PATH=/bin:/usr/bin"}
	cmdFallback.Stdin = strings.NewReader(`{"hook_event_name":"PreToolUse"}`)
	outFallback, err := cmdFallback.Output()
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	if trimmed := strings.TrimSpace(string(outFallback)); trimmed != "{}" {
		t.Errorf("expected '{}' on fallback, got %q", trimmed)
	}
}

func TestInstallScriptSyntax(t *testing.T) {
	root := projectRoot(t)
	script := filepath.Join(root, "install.sh")
	cmd := exec.Command("sh", "-n", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install.sh syntax error: %v, output: %s", err, string(out))
	}
}

func TestSkillMarkdownFormat(t *testing.T) {
	root := projectRoot(t)
	skillPath := filepath.Join(root, "skills", "agent-sentinel", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("missing SKILL.md: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\nname: agent-sentinel") {
		t.Errorf("SKILL.md does not start with valid YAML frontmatter")
	}
	if !strings.Contains(content, "http://127.0.0.1:8848") {
		t.Errorf("SKILL.md missing reference to Cockpit URL")
	}
}
