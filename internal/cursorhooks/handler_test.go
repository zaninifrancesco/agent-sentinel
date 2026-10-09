package cursorhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func newHandler(t *testing.T, approvals *policy.Broker, timeout time.Duration) *Handler {
	t.Helper()
	e, err := policy.NewEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	return New(recorder.New([]string{"cursor hooks"}, nil), e, nil, approvals, timeout)
}

func post(t *testing.T, h *Handler, ctx context.Context, in map[string]any) Response {
	t.Helper()
	b, _ := json.Marshal(in)
	req := httptest.NewRequest(http.MethodPost, "/api/hook", strings.NewReader(string(b))).WithContext(ctx)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad response %q: %v", rr.Body.String(), err)
	}
	return resp
}

func shell(cmd string) map[string]any {
	return map[string]any{"hook_event_name": EventBeforeShell, "conversation_id": "c1", "command": cmd, "cwd": "/proj"}
}

func after(cmd, output string, durationMs float64) map[string]any {
	return map[string]any{"hook_event_name": EventAfterShell, "conversation_id": "c1", "command": cmd, "output": output, "duration": durationMs}
}

func waitEvent(t *testing.T, rec *recorder.Recorder, what string, pred func(recorder.Event) bool) recorder.Event {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range rec.Events() {
			if pred(e) {
				return e
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
	return recorder.Event{}
}

func awaiting(e recorder.Event) bool { return e.Status == recorder.StatusAwaiting }

func ofType(typ recorder.EventType) func(recorder.Event) bool {
	return func(e recorder.Event) bool { return e.Type == typ }
}

func TestHarmlessShellIsAllowedAndCompleted(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()

	if r := post(t, h, ctx, shell("ls -la")); r.Permission != "allow" {
		t.Fatalf("got %+v", r)
	}
	post(t, h, ctx, after("ls -la", "total 8", 42))

	req := waitEvent(t, h.Rec, "request", ofType(recorder.EventToolCallRequest))
	res := waitEvent(t, h.Rec, "response", ofType(recorder.EventToolCallResponse))
	if req.ToolName != "shell" || req.RPCID == "" || res.RPCID != req.RPCID {
		t.Fatalf("request/response not paired: %+v / %+v", req, res)
	}
	if res.DurationMs != 42 || res.Status != recorder.StatusOK || !strings.Contains(string(res.Payload), "total 8") {
		t.Fatalf("bad response event: %+v", res)
	}
	if !strings.Contains(string(req.Payload), `"command":"ls -la"`) {
		t.Fatalf("command missing from the request payload: %s", req.Payload)
	}
}

func TestDestructiveShellIsBlocked(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	r := post(t, h, context.Background(), shell("rm -rf /"))
	if r.Permission != "deny" || !strings.Contains(r.AgentMessage, "NOT executed") || r.UserMessage == "" {
		t.Fatalf("got %+v", r)
	}
	req := waitEvent(t, h.Rec, "blocked request", ofType(recorder.EventToolCallRequest))
	if req.Status != recorder.StatusBlocked || req.Rule != "rm-root" || req.Risk != recorder.RiskCritical {
		t.Fatalf("bad event %+v", req)
	}
	res := waitEvent(t, h.Rec, "refusal", ofType(recorder.EventToolCallResponse))
	if res.Status != recorder.StatusBlocked || res.DurationMs != 0 {
		t.Fatalf("a refusal has no duration and is blocked: %+v", res)
	}
}

// holdAndAnswer runs a command that needs approval, answers it as the cockpit
// would, and returns what Cursor was told.
func holdAndAnswer(t *testing.T, h *Handler, b *policy.Broker, cmd string, v policy.Verdict) Response {
	t.Helper()
	done := make(chan Response, 1)
	go func() { done <- post(t, h, context.Background(), shell(cmd)) }()
	ev := waitEvent(t, h.Rec, "held call", awaiting)
	if ev.Decision != "approve" || ev.Rule == "" {
		t.Fatalf("held without a reason: %+v", ev)
	}
	if err := b.Resolve(ev.Seq, v); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("hook never answered")
		return Response{}
	}
}

func TestApprovedCommandRunsWithTheOperatorNote(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	r := holdAndAnswer(t, h, b, "sudo systemctl restart nginx", policy.Verdict{Approved: true, Feedback: "only nginx"})
	if r.Permission != "allow" || !strings.Contains(r.AgentMessage, "only nginx") {
		t.Fatalf("got %+v", r)
	}
	resolved := waitEvent(t, h.Rec, "resolution", ofType(recorder.EventApprovalResolved))
	if resolved.Decision != "approved" {
		t.Fatalf("bad resolution %+v", resolved)
	}
	// Once Cursor reports the run, it closes the same row.
	post(t, h, context.Background(), after("sudo systemctl restart nginx", "ok", 300))
	res := waitEvent(t, h.Rec, "response", ofType(recorder.EventToolCallResponse))
	if res.RPCID != resolved.RPCID || res.DurationMs != 300 {
		t.Fatalf("response not tied to the approved call: %+v", res)
	}
}

func TestRejectedCommandIsDeniedWithFeedback(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	r := holdAndAnswer(t, h, b, "git push --force origin main", policy.Verdict{Approved: false, Feedback: "use a PR"})
	if r.Permission != "deny" || !strings.Contains(r.AgentMessage, "use a PR") || !strings.Contains(r.AgentMessage, "NOT executed") {
		t.Fatalf("got %+v", r)
	}
	res := waitEvent(t, h.Rec, "refusal", ofType(recorder.EventToolCallResponse))
	if res.Status != recorder.StatusRejected || res.DurationMs != 0 {
		t.Fatalf("bad refusal %+v", res)
	}
}

func TestUnansweredCommandTimesOutRefused(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), 80*time.Millisecond)
	r := post(t, h, context.Background(), shell("sudo rm file"))
	if r.Permission != "deny" || !strings.Contains(r.AgentMessage, "No human approved") {
		t.Fatalf("timeout must refuse: %+v", r)
	}
	if got := waitEvent(t, h.Rec, "resolution", ofType(recorder.EventApprovalResolved)); got.Decision != "timeout" {
		t.Fatalf("bad resolution %+v", got)
	}
}

func TestClosedHookCancelsTheApproval(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Response, 1)
	go func() { done <- post(t, h, ctx, shell("sudo ls")) }()
	waitEvent(t, h.Rec, "held call", awaiting)
	cancel() // Cursor gave up on the hook
	r := <-done
	if r.Permission != "deny" {
		t.Fatalf("got %+v", r)
	}
	if got := waitEvent(t, h.Rec, "resolution", ofType(recorder.EventApprovalResolved)); got.Decision != "cancelled" {
		t.Fatalf("bad resolution %+v", got)
	}
	if len(b.Pending()) != 0 {
		t.Fatal("approval left pending")
	}
}

func TestNoCockpitFailsClosed(t *testing.T) {
	h := newHandler(t, nil, time.Second)
	r := post(t, h, context.Background(), shell("sudo ls"))
	if r.Permission != "deny" {
		t.Fatalf("approval with nobody to answer must refuse: %+v", r)
	}
}

func TestBudgetHoldsTheNextCall(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	h.Budget = policy.NewBudget(0.0001, 0)

	done := make(chan Response, 1)
	go func() { done <- post(t, h, context.Background(), shell("echo "+strings.Repeat("a", 2000))) }()
	ev := waitEvent(t, h.Rec, "budget hold", awaiting)
	if ev.Rule != "budget" {
		t.Fatalf("rule = %q, want budget", ev.Rule)
	}
	b.Resolve(ev.Seq, policy.Verdict{Approved: true})
	if r := <-done; r.Permission != "allow" {
		t.Fatalf("got %+v", r)
	}
	if c, _ := h.Budget.Limits(); c <= 0.0001 {
		t.Fatalf("approving past the budget must raise the limit, got %v", c)
	}
}

func TestSameCommandTwiceResolvesInOrder(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()
	post(t, h, ctx, shell("make test"))
	post(t, h, ctx, shell("make test"))
	post(t, h, ctx, after("make test", "first", 10))
	post(t, h, ctx, after("make test", "second", 20))

	var reqs, ress []recorder.Event
	for _, e := range h.Rec.Events() {
		switch e.Type {
		case recorder.EventToolCallRequest:
			reqs = append(reqs, e)
		case recorder.EventToolCallResponse:
			ress = append(ress, e)
		}
	}
	if len(reqs) != 2 || len(ress) != 2 || ress[0].RPCID != reqs[0].RPCID || ress[1].RPCID != reqs[1].RPCID {
		t.Fatalf("responses not matched in order: %+v / %+v", reqs, ress)
	}
}

func TestAfterHookWithoutBeforeStillShowsTheCall(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	post(t, h, context.Background(), after("go build ./...", "", 900))
	var types []recorder.EventType
	for _, e := range h.Rec.Events() {
		if e.Type == recorder.EventToolCallRequest || e.Type == recorder.EventToolCallResponse {
			types = append(types, e.Type)
		}
	}
	if len(types) != 2 {
		t.Fatalf("expected a request and a response, got %v", types)
	}
}

func mcp(event string, extra map[string]any) map[string]any {
	in := map[string]any{
		"hook_event_name": event, "conversation_id": "c1",
		"tool_name": "create_issue", "mcp_server_name": "linear",
		"tool_input": `{"title":"fix the build"}`,
	}
	for k, v := range extra {
		in[k] = v
	}
	return in
}

func TestMCPCallsAreNamedByServerAndJudgedOnTheirArguments(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()

	if r := post(t, h, ctx, mcp(EventBeforeMCP, nil)); r.Permission != "allow" {
		t.Fatalf("got %+v", r)
	}
	post(t, h, ctx, mcp(EventAfterMCP, map[string]any{"result_json": `{"content":[],"isError":true}`, "duration": 77}))
	req := waitEvent(t, h.Rec, "request", ofType(recorder.EventToolCallRequest))
	res := waitEvent(t, h.Rec, "response", ofType(recorder.EventToolCallResponse))
	if req.ToolName != "linear:create_issue" || res.Status != recorder.StatusError || res.DurationMs != 77 {
		t.Fatalf("bad MCP events: %+v / %+v", req, res)
	}

	// A secret inside the JSON-string arguments is still seen by the policy.
	bad := mcp(EventBeforeMCP, map[string]any{"tool_input": `{"body":"key AKIAIOSFODNN7EXAMPLE"}`})
	done := make(chan Response, 1)
	go func() { done <- post(t, h, ctx, bad) }()
	waitEvent(t, h.Rec, "held secret", func(e recorder.Event) bool { return awaiting(e) && e.Rule == "secret-token" })
	h.Approvals.Resolve(h.Rec.Events()[len(h.Rec.Events())-1].Seq, policy.Verdict{Approved: false})
	if r := <-done; r.Permission != "deny" {
		t.Fatalf("got %+v", r)
	}
}

func TestFileEditIsRecordedWithItsDiffAndFlaggedAfterTheFact(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	post(t, h, context.Background(), map[string]any{
		"hook_event_name": EventAfterEdit, "file_path": "/proj/config.go",
		"edits": []map[string]string{{"old_string": "key := \"\"\n", "new_string": "key := \"AKIAIOSFODNN7EXAMPLE\"\n"}},
	})
	ev := waitEvent(t, h.Rec, "edit", ofType(recorder.EventToolCallRequest))
	if ev.ToolName != "edit_file" || ev.Status != recorder.StatusOK {
		t.Fatalf("bad edit event %+v", ev)
	}
	for _, want := range []string{"+++ b/proj/config.go", "@@ -1,1 +1,1 @@", `-key := \"\"`, `+key := \"AKIA`} {
		if !strings.Contains(string(ev.Payload), want) {
			t.Errorf("diff misses %q in %s", want, ev.Payload)
		}
	}
	// The cockpit titles a row with the first argument in key order: it must be the file, not the diff.
	if p := string(ev.Payload); strings.Index(p, `"file_path"`) < 0 || strings.Index(p, `"file_path"`) > strings.Index(p, `"patch"`) {
		t.Errorf("file_path must come before patch in %s", p)
	}
	if ev.Decision != "warn" || ev.Rule != "secret-token" || !strings.Contains(ev.Reason, "already applied") {
		t.Fatalf("a secret written to a file must be flagged, not pretend it was stopped: %+v", ev)
	}
}

func stopHook(conversation, status string) map[string]any {
	return map[string]any{"hook_event_name": EventStop, "conversation_id": conversation, "status": status}
}

func responses(rec *recorder.Recorder) []recorder.Event {
	var out []recorder.Event
	for _, e := range rec.Events() {
		if e.Type == recorder.EventToolCallResponse {
			out = append(out, e)
		}
	}
	return out
}

func TestStopClosesACommandThatNeverReportedBack(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()

	// The user pressed Skip: Cursor sent the "before" and will never send the "after".
	post(t, h, ctx, shell("sleep 600"))
	if n := len(responses(h.Rec)); n != 0 {
		t.Fatalf("a running command has no response yet, got %d", n)
	}
	if r := post(t, h, ctx, stopHook("c1", "aborted")); r != (Response{}) {
		t.Fatalf("stop must answer with an empty object, got %+v", r)
	}

	req := waitEvent(t, h.Rec, "request", ofType(recorder.EventToolCallRequest))
	res := waitEvent(t, h.Rec, "response", ofType(recorder.EventToolCallResponse))
	if res.RPCID != req.RPCID || res.Status != recorder.StatusError {
		t.Fatalf("the row must be closed with an error, got %+v", res)
	}
	if !strings.Contains(string(res.Payload), "aborted") {
		t.Fatalf("the response should say why: %s", res.Payload)
	}

	// A second stop has nothing left to close.
	post(t, h, ctx, stopHook("c1", "completed"))
	if n := len(responses(h.Rec)); n != 1 {
		t.Fatalf("stop closed a call twice: %d responses", n)
	}
}

func TestStopOnlyClosesItsOwnConversation(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()

	post(t, h, ctx, shell("sleep 600")) // conversation c1
	other := shell("sleep 700")
	other["conversation_id"] = "c2"
	post(t, h, ctx, other)

	post(t, h, ctx, stopHook("c2", "completed"))
	if got := responses(h.Rec); len(got) != 1 || !strings.Contains(string(got[0].Payload), "completed") {
		t.Fatalf("only c2 should be closed, got %+v", got)
	}
	// c1's command is still in flight and finishes normally.
	post(t, h, ctx, after("sleep 600", "", 5))
	if got := responses(h.Rec); len(got) != 2 || got[1].Status != recorder.StatusOK {
		t.Fatalf("c1 must still be answerable after c2's stop, got %+v", got)
	}
}

func TestStopAfterNormalCompletionAddsNothing(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	ctx := context.Background()
	post(t, h, ctx, shell("ls"))
	post(t, h, ctx, after("ls", "x", 1))
	post(t, h, ctx, stopHook("c1", "completed"))
	if got := responses(h.Rec); len(got) != 1 || got[0].Status != recorder.StatusOK {
		t.Fatalf("stop must not touch finished calls, got %+v", got)
	}
}

func TestUnknownEventsAndBadInput(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	if r := post(t, h, context.Background(), map[string]any{"hook_event_name": "sessionStart"}); r != (Response{}) {
		t.Fatalf("unknown events get an empty answer, got %+v", r)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("nope")))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("garbage got %d", rr.Code)
	}
}

func TestConcurrentCallsKeepTheirOwnVerdicts(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r := post(t, h, context.Background(), shell("ls")); r.Permission != "allow" {
				t.Errorf("got %+v", r)
			}
		}()
	}
	wg.Wait()
	ids := map[string]bool{}
	for _, e := range h.Rec.Events() {
		if e.Type == recorder.EventToolCallRequest {
			if ids[e.RPCID] {
				t.Fatalf("duplicate rpc id %s", e.RPCID)
			}
			ids[e.RPCID] = true
		}
	}
	if len(ids) != 20 {
		t.Fatalf("got %d calls", len(ids))
	}
}

// Real Cursor sends an empty cwd, and writes new files as an insertion.
func TestEmptyCwdIsNotRecordedAndInsertionHunkStartsAtZero(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	in := shell("python3 hello.py")
	in["cwd"] = ""
	post(t, h, context.Background(), in)
	post(t, h, context.Background(), map[string]any{
		"hook_event_name": EventAfterEdit, "file_path": "/proj/new.py",
		"edits": []map[string]string{{"old_string": "", "new_string": "print(1)\n"}},
	})
	var gotShell, gotEdit string
	for _, e := range h.Rec.Events() {
		if e.Type != recorder.EventToolCallRequest {
			continue
		}
		if e.ToolName == "shell" {
			gotShell = string(e.Payload)
		} else {
			gotEdit = string(e.Payload)
		}
	}
	if strings.Contains(gotShell, "cwd") {
		t.Errorf("an empty cwd must not be recorded: %s", gotShell)
	}
	if !strings.Contains(gotEdit, "@@ -0,0 +1,1 @@") {
		t.Errorf("a pure insertion starts at 0,0: %s", gotEdit)
	}
}

func read(path, content string) map[string]any {
	return map[string]any{"hook_event_name": EventBeforeRead, "conversation_id": "c1", "file_path": path, "content": content}
}

func requests(h *Handler) []recorder.Event {
	var out []recorder.Event
	for _, e := range h.Rec.Events() {
		if e.Type == recorder.EventToolCallRequest {
			out = append(out, e)
		}
	}
	return out
}

func TestReadingAnOrdinaryFileIsAllowedAndNeverStoresItsContent(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	h.Budget = policy.NewBudget(0, 0)
	const content = "package main // CONTENT-MARKER"
	r := post(t, h, context.Background(), read("/proj/main.go", content))
	if r != (Response{Permission: "allow"}) {
		t.Fatalf("got %+v", r)
	}
	evs := requests(h)
	if len(evs) != 1 || evs[0].ToolName != "read_file" || evs[0].Status != recorder.StatusOK {
		t.Fatalf("bad events: %+v", evs)
	}
	for _, e := range h.Rec.Events() {
		if strings.Contains(string(e.Payload), "CONTENT-MARKER") || strings.Contains(e.Reason, "CONTENT-MARKER") {
			t.Fatalf("the file content leaked into the session log: %s", e.Payload)
		}
	}
	if !strings.Contains(string(evs[0].Payload), fmt.Sprintf(`"bytes":%d`, len(content))) || !strings.Contains(string(evs[0].Payload), `"file_path":"/proj/main.go"`) {
		t.Fatalf("path and size must be recorded: %s", evs[0].Payload)
	}
	if _, tokens := h.Budget.Spent(); tokens == 0 {
		t.Fatal("a file handed to the model counts as input tokens")
	}
}

func TestSensitiveFilesAreRefusedWithAnAnswerCursorAccepts(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	for path, rule := range map[string]string{
		"/proj/.env":                "dotenv-access",
		"/proj/.env.production":     "dotenv-access",
		"/Users/me/.ssh/id_ed25519": "ssh-keys",
	} {
		b, _ := json.Marshal(read(path, "SECRET=hunter2-NEVER-LOG"))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(b))))
		// beforeReadFile's schema is permission + user_message: nothing else.
		var raw map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &raw)
		if raw["permission"] != "deny" || raw["user_message"] == nil || raw["agent_message"] != nil || len(raw) != 2 {
			t.Fatalf("%s: answer outside the beforeReadFile schema: %s", path, rr.Body.String())
		}
		var found bool
		for _, e := range requests(h) {
			if strings.Contains(string(e.Payload), path) && e.Status == recorder.StatusBlocked && e.Rule == rule {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: expected a blocked event from %s", path, rule)
		}
	}
	for _, e := range h.Rec.Events() {
		if strings.Contains(string(e.Payload), "hunter2-NEVER-LOG") {
			t.Fatalf("the content of a refused file leaked into the log")
		}
	}
}

func TestSecretInsideAnAllowedFileIsFlaggedWithoutQuotingIt(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	r := post(t, h, context.Background(), read("/proj/config.go", "var key = \"AKIAIOSFODNN7EXAMPLE\""))
	if r.Permission != "allow" {
		t.Fatalf("content only flags, the path decides: %+v", r)
	}
	ev := requests(h)[0]
	if ev.Decision != "warn" || ev.Rule != "secret-token" || ev.Risk != recorder.RiskCritical {
		t.Fatalf("not flagged: %+v", ev)
	}
	if strings.Contains(ev.Reason, "AKIA") || strings.Contains(string(ev.Payload), "AKIA") {
		t.Fatalf("the key must not be copied into the log: %q", ev.Reason)
	}
}

func TestContentIsOnlySearchedForSecrets(t *testing.T) {
	h := newHandler(t, policy.NewBroker(), time.Second)
	// A script that mentions sudo and rm -rf is not a command being run.
	post(t, h, context.Background(), read("/proj/deploy.sh", "sudo systemctl restart app\nrm -rf /tmp/build\n"))
	if ev := requests(h)[0]; ev.Decision != "" || ev.Status != recorder.StatusOK {
		t.Fatalf("a shell script must read freely: %+v", ev)
	}
}

func TestReadNeedingApprovalWaitsForTheHuman(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	const cred = "/Users/me/.aws/credentials"

	// Approve.
	done := make(chan Response, 1)
	go func() { done <- post(t, h, context.Background(), read(cred, "x")) }()
	ev := waitEvent(t, h.Rec, "held read", awaiting)
	if ev.Rule != "credential-files" {
		t.Fatalf("rule = %q", ev.Rule)
	}
	b.Resolve(ev.Seq, policy.Verdict{Approved: true, Feedback: "just this once"})
	if r := <-done; r != (Response{Permission: "allow"}) {
		t.Fatalf("approved read got %+v (no agent_message allowed)", r)
	}
	res := waitEvent(t, h.Rec, "closing response", ofType(recorder.EventToolCallResponse))
	if res.Status != recorder.StatusOK || res.RPCID != ev.RPCID {
		t.Fatalf("an approved read must close its own row: %+v", res)
	}

	// Reject.
	h2 := newHandler(t, b, 5*time.Second)
	go func() { done <- post(t, h2, context.Background(), read(cred, "x")) }()
	ev2 := waitEvent(t, h2.Rec, "held read", awaiting)
	b.Resolve(ev2.Seq, policy.Verdict{Approved: false})
	if r := <-done; r.Permission != "deny" || r.AgentMessage != "" {
		t.Fatalf("rejected read got %+v", r)
	}
}

func TestReadsCountTowardsTheBudget(t *testing.T) {
	b := policy.NewBroker()
	h := newHandler(t, b, 5*time.Second)
	h.Budget = policy.NewBudget(0.0001, 0)
	done := make(chan Response, 1)
	go func() { done <- post(t, h, context.Background(), read("/proj/big.txt", strings.Repeat("a", 100_000))) }()
	ev := waitEvent(t, h.Rec, "budget hold", awaiting)
	if ev.Rule != "budget" {
		t.Fatalf("rule = %q, want budget", ev.Rule)
	}
	b.Resolve(ev.Seq, policy.Verdict{Approved: false})
	if r := <-done; r.Permission != "deny" {
		t.Fatalf("got %+v", r)
	}
}
