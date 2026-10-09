package cursorhooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// The inputs below follow https://code.claude.com/docs/en/hooks.

func postClaude(t *testing.T, h *Handler, ctx context.Context, in map[string]any) claudeOutput {
	t.Helper()
	b, _ := json.Marshal(in)
	req := httptest.NewRequest(http.MethodPost, "/api/hook", strings.NewReader(string(b))).WithContext(ctx)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var out claudeOutput
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad response %q: %v", rr.Body.String(), err)
	}
	return out
}

func pre(tool, id string, input map[string]any) map[string]any {
	return map[string]any{
		"hook_event_name": EventClaudePre, "session_id": "s1", "cwd": "/proj",
		"tool_name": tool, "tool_input": input, "tool_use_id": id,
	}
}

func claudePost(tool, id string, input map[string]any, response any) map[string]any {
	return map[string]any{
		"hook_event_name": EventClaudePost, "session_id": "s1", "cwd": "/proj",
		"tool_name": tool, "tool_input": input, "tool_response": response, "tool_use_id": id, "duration_ms": 12,
	}
}

func bash(cmd string) map[string]any { return map[string]any{"command": cmd, "description": "test"} }

func decision(o claudeOutput) string {
	if o.HookSpecificOutput == nil {
		return ""
	}
	return o.HookSpecificOutput.PermissionDecision
}

func TestClaudeHarmlessCommandSaysNothingSoClaudeKeepsItsOwnRules(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()

	out := postClaude(t, h, ctx, pre("Bash", "toolu_1", bash("ls -la")))
	if out.HookSpecificOutput != nil || out.SystemMessage != "" {
		t.Fatalf("an unobjectionable call must get an empty answer (an explicit allow would skip Claude's own prompt), got %+v", out)
	}
	postClaude(t, h, ctx, claudePost("Bash", "toolu_1", bash("ls -la"), map[string]any{"stdout": "total 8", "stderr": "", "interrupted": false}))

	req := waitEvent(t, h.Rec, "request", ofType(recorder.EventToolCallRequest))
	res := waitEvent(t, h.Rec, "response", ofType(recorder.EventToolCallResponse))
	if req.ToolName != "shell" || res.RPCID != req.RPCID || res.Status != recorder.StatusOK || res.DurationMs != 12 {
		t.Fatalf("not paired: %+v / %+v", req, res)
	}
	if !strings.Contains(string(req.Payload), `"command":"ls -la"`) || !strings.Contains(string(res.Payload), "total 8") {
		t.Fatalf("payloads: %s / %s", req.Payload, res.Payload)
	}
}

func TestClaudeIdenticalCommandsPairByToolUseID(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()
	postClaude(t, h, ctx, pre("Bash", "a", bash("sleep 1")))
	postClaude(t, h, ctx, pre("Bash", "b", bash("sleep 1")))
	// b finishes first.
	postClaude(t, h, ctx, claudePost("Bash", "b", bash("sleep 1"), map[string]any{"stdout": "B"}))
	postClaude(t, h, ctx, claudePost("Bash", "a", bash("sleep 1"), map[string]any{"stdout": "A"}))

	reqs, resps := requests(h), responses(h.Rec)
	if len(reqs) != 2 || len(resps) != 2 {
		t.Fatalf("got %d requests, %d responses", len(reqs), len(resps))
	}
	if resps[0].RPCID != reqs[1].RPCID || !strings.Contains(string(resps[0].Payload), "B") ||
		resps[1].RPCID != reqs[0].RPCID || !strings.Contains(string(resps[1].Payload), "A") {
		t.Fatalf("results crossed: %+v", resps)
	}
}

func TestClaudeBlocksADotenvRead(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	out := postClaude(t, h, context.Background(), pre("Read", "r1", map[string]any{"file_path": "/proj/.env.local"}))
	if decision(out) != "deny" || !strings.Contains(out.HookSpecificOutput.PermissionDecisionReason, "dotenv-access") {
		t.Fatalf("got %+v", out)
	}
	if out.HookSpecificOutput.HookEventName != "PreToolUse" || out.SystemMessage == "" {
		t.Fatalf("Claude needs the event name, and the user the message: %+v", out)
	}
	req := waitEvent(t, h.Rec, "request", ofType(recorder.EventToolCallRequest))
	if req.ToolName != "read_file" || req.Status != recorder.StatusBlocked {
		t.Fatalf("got %+v", req)
	}
}

func TestClaudeHeldCommandIsApprovedWithANote(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	done := make(chan claudeOutput, 1)
	go func() { done <- postClaude(t, h, context.Background(), pre("Bash", "x", bash("rm -rf build"))) }()

	ev := waitEvent(t, h.Rec, "hold", awaiting)
	if ev.Rule != "rm-recursive" {
		t.Fatalf("rule = %q", ev.Rule)
	}
	b.Resolve(ev.Seq, policy.Verdict{Approved: true, Feedback: "only build/"})
	out := <-done
	if decision(out) != "allow" || !strings.Contains(out.HookSpecificOutput.AdditionalContext, "only build/") {
		t.Fatalf("a human approval is an explicit allow that carries the note, got %+v", out)
	}
	// Approved: it runs, and its result closes the row.
	postClaude(t, h, context.Background(), claudePost("Bash", "x", bash("rm -rf build"), map[string]any{"stdout": ""}))
	if got := responses(h.Rec); len(got) != 1 || got[0].Status != recorder.StatusOK {
		t.Fatalf("got %+v", got)
	}
}

func TestClaudeHeldCommandIsRejectedWithFeedback(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	done := make(chan claudeOutput, 1)
	go func() { done <- postClaude(t, h, context.Background(), pre("Bash", "x", bash("rm -rf build"))) }()

	ev := waitEvent(t, h.Rec, "hold", awaiting)
	b.Resolve(ev.Seq, policy.Verdict{Approved: false, Feedback: "use trash"})
	out := <-done
	if decision(out) != "deny" || !strings.Contains(out.HookSpecificOutput.PermissionDecisionReason, "use trash") {
		t.Fatalf("got %+v", out)
	}
}

func TestClaudeEditsAreGatedBeforeTheyHappen(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()

	if out := postClaude(t, h, ctx, pre("Write", "w1", map[string]any{"file_path": "/proj/.env", "content": "A=1"})); decision(out) != "deny" {
		t.Fatalf("writing .env must be refused, got %+v", out)
	}

	// An ordinary edit: nothing said, and the diff is recorded.
	in := map[string]any{"file_path": "/proj/a.go", "old_string": "x := 1", "new_string": "x := 2"}
	if out := postClaude(t, h, ctx, pre("Edit", "e1", in)); out.HookSpecificOutput != nil {
		t.Fatalf("got %+v", out)
	}
	postClaude(t, h, ctx, claudePost("Edit", "e1", in, map[string]any{"filePath": "/proj/a.go", "originalFile": "SECRET FILE BODY"}))

	var edit recorder.Event
	for _, e := range requests(h) {
		if e.ToolName == "edit_file" && e.Status != recorder.StatusBlocked {
			edit = e
		}
	}
	if !strings.Contains(string(edit.Payload), "-x := 1") || !strings.Contains(string(edit.Payload), "+x := 2") {
		t.Fatalf("the diff is missing: %s", edit.Payload)
	}
	for _, e := range responses(h.Rec) {
		if strings.Contains(string(e.Payload), "SECRET FILE BODY") {
			t.Fatalf("the tool's own copy of the file must not be stored: %s", e.Payload)
		}
	}
}

func TestClaudeRemovingASecretIsNotLeakingIt(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	key := "sk-" + strings.Repeat("a1B2", 8)
	out := postClaude(t, h, context.Background(), pre("Edit", "e1",
		map[string]any{"file_path": "/proj/config.go", "old_string": `token := "` + key + `"`, "new_string": `token := os.Getenv("T")`}))
	if out.HookSpecificOutput != nil {
		t.Fatalf("removing a secret must pass, got %+v", out)
	}
	if ev := requests(h); len(ev) != 1 || ev[0].Rule != "" {
		t.Fatalf("no rule should fire: %+v", ev)
	}
}

func TestClaudeWritingASecretNeedsApproval(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	key := "sk-" + strings.Repeat("a1B2", 8)
	done := make(chan claudeOutput, 1)
	go func() {
		done <- postClaude(t, h, context.Background(), pre("Write", "w1", map[string]any{"file_path": "/proj/c.go", "content": `k := "` + key + `"`}))
	}()
	ev := waitEvent(t, h.Rec, "hold", awaiting)
	if ev.Rule != "secret-token" {
		t.Fatalf("rule = %q", ev.Rule)
	}
	b.Resolve(ev.Seq, policy.Verdict{Approved: false})
	if out := <-done; decision(out) != "deny" {
		t.Fatalf("got %+v", out)
	}
}

func TestClaudeReadResultIsCountedButNeverStored(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	h.Budget = policy.NewBudget(100, 0)
	ctx := context.Background()
	in := map[string]any{"file_path": "/proj/notes.txt"}
	body := strings.Repeat("top secret line\n", 200)

	postClaude(t, h, ctx, pre("Read", "r1", in))
	postClaude(t, h, ctx, claudePost("Read", "r1", in, map[string]any{"type": "text", "file": map[string]any{"filePath": "/proj/notes.txt", "content": body}}))

	for _, e := range h.Rec.Events() {
		if strings.Contains(string(e.Payload), "top secret line") {
			t.Fatalf("file content reached the log: %s", e.Payload)
		}
	}
	if _, tokens := h.Budget.Spent(); tokens < 800 {
		t.Fatalf("the file went to the model and must count towards the budget, got %d tokens", tokens)
	}
}

func TestClaudeFailuresAndInterruptsAreErrors(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()

	postClaude(t, h, ctx, pre("Bash", "f1", bash("npm test")))
	fail := claudePost("Bash", "f1", bash("npm test"), nil)
	fail["hook_event_name"] = EventClaudePostFailure
	delete(fail, "tool_response")
	fail["error"] = "Exit code 1\nCannot find module 'express'"
	postClaude(t, h, ctx, fail)

	postClaude(t, h, ctx, pre("Bash", "f2", bash("sleep 100")))
	postClaude(t, h, ctx, claudePost("Bash", "f2", bash("sleep 100"), map[string]any{"stdout": "", "interrupted": true}))

	got := responses(h.Rec)
	if len(got) != 2 || got[0].Status != recorder.StatusError || got[1].Status != recorder.StatusError {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(string(got[0].Payload), "Cannot find module") || !strings.Contains(string(got[1].Payload), "interrupted") {
		t.Fatalf("reasons missing: %s / %s", got[0].Payload, got[1].Payload)
	}
}

func TestClaudeMCPToolsAreNamedByServer(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	postClaude(t, h, context.Background(), pre("mcp__github__create_issue", "m1", map[string]any{"title": "hi"}))
	if reqs := requests(h); len(reqs) != 1 || reqs[0].ToolName != "github:create_issue" {
		t.Fatalf("got %+v", reqs)
	}
}

func TestClaudeStopAndSessionEndCloseWhatInterruptsLeftOpen(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()
	var lines []string
	h.Logf = func(f string, a ...any) { lines = append(lines, strings.TrimSpace(strings.ReplaceAll(f, "%", ""))) }

	// The user pressed Esc during a command: no PostToolUse will ever come.
	postClaude(t, h, ctx, pre("Bash", "i1", bash("sleep 600")))
	other := pre("Bash", "i2", bash("sleep 700"))
	other["session_id"] = "s2"
	postClaude(t, h, ctx, other)
	if n := len(responses(h.Rec)); n != 0 {
		t.Fatalf("nothing is closed yet, got %d", n)
	}

	postClaude(t, h, ctx, map[string]any{"hook_event_name": EventClaudeStop, "session_id": "s1", "stop_hook_active": false})
	got := responses(h.Rec)
	if len(got) != 1 || got[0].Status != recorder.StatusError || !strings.Contains(string(got[0].Payload), "ended the turn") {
		t.Fatalf("Stop must close s1's call only, got %+v", got)
	}

	postClaude(t, h, ctx, map[string]any{"hook_event_name": EventClaudeSessionEnd, "session_id": "s2", "reason": "prompt_input_exit"})
	got = responses(h.Rec)
	if len(got) != 2 || !strings.Contains(string(got[1].Payload), "prompt_input_exit") {
		t.Fatalf("SessionEnd must close s2's call, got %+v", got)
	}
	if len(lines) != 2 {
		t.Fatalf("each close is logged for the operator, got %v", lines)
	}
}

func TestClaudeEditsCountTowardsTheBudget(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	h.Budget = policy.NewBudget(100, 0)
	postClaude(t, h, context.Background(), pre("Write", "w", map[string]any{"file_path": "/proj/big.txt", "content": strings.Repeat("lorem ipsum\n", 400)}))
	if _, tokens := h.Budget.Spent(); tokens < 1000 {
		t.Fatalf("what the model writes is spend, got %d tokens", tokens)
	}
}
