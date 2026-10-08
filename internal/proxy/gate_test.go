package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// runEchoServer is a fake MCP server (SENTINEL_FAKE_SERVER=2) that answers
// every tools/call with "SERVER-RAN <tool>", so tests can tell whether a call
// really reached the server.
func runEchoServer() {
	r := bufio.NewReader(os.Stdin)
	for {
		line, err := r.ReadBytes('\n')
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if json.Unmarshal(line, &m) == nil && m.Method == "tools/call" {
			fmt.Printf(`{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"SERVER-RAN %s"}]}}`+"\n", m.ID, m.Params.Name)
		}
		if err != nil {
			return
		}
	}
}

func call(id int, tool, command string) string {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": map[string]any{"command": command}},
	})
	return string(b) + "\n"
}

func waitFor(t *testing.T, rec *recorder.Recorder, what string, pred func(recorder.Event) bool) recorder.Event {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
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

// runGate runs a proxy in front of the echo server. drive plays the client:
// it writes lines and decides when to hang up.
func runGate(t *testing.T, p *StdioProxy, drive func(rec *recorder.Recorder, w io.Writer)) string {
	t.Helper()
	t.Setenv("SENTINEL_FAKE_SERVER", "2")
	in, w := io.Pipe()
	var out bytes.Buffer
	p.Command = []string{os.Args[0]}
	p.In, p.Out, p.Stderr = in, &out, io.Discard
	p.Rec = recorder.New([]string{"echo"}, nil)

	go func() {
		defer w.Close()
		drive(p.Rec, w)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := p.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.String()
}

func newEngine(t *testing.T) *policy.Engine {
	t.Helper()
	e, err := policy.NewEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func responseTo(rec *recorder.Recorder, rpcID string) func(recorder.Event) bool {
	return func(e recorder.Event) bool { return e.Type == recorder.EventToolCallResponse && e.RPCID == rpcID }
}

func TestGateAllowsHarmlessAndFlagsWarn(t *testing.T) {
	p := &StdioProxy{Policy: newEngine(t)}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(1, "shell", "ls -la"))
		io.WriteString(w, call(2, "shell", "chmod 777 run.sh"))
		waitFor(t, rec, "response 2", responseTo(rec, "2"))
	})
	if !strings.Contains(out, "SERVER-RAN shell") {
		t.Fatalf("calls did not reach the server: %q", out)
	}
	var warned bool
	for _, e := range p.Rec.Events() {
		if e.Type == recorder.EventToolCallRequest && e.Decision == "warn" && e.Rule == "chmod-777" {
			warned = true
		}
	}
	if !warned {
		t.Fatal("chmod 777 was not flagged as warn")
	}
}

func TestGateBlockNeverReachesServer(t *testing.T) {
	p := &StdioProxy{Policy: newEngine(t)}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(7, "shell", "rm -rf /"))
		io.WriteString(w, call(8, "read", "cat .env"))
		waitFor(t, rec, "response 8", responseTo(rec, "8"))
	})
	if strings.Contains(out, "SERVER-RAN") {
		t.Fatalf("a blocked call reached the server: %q", out)
	}
	if strings.Count(out, "Blocked by Agent Sentinel policy") != 2 || !strings.Contains(out, `"isError":true`) {
		t.Fatalf("agent did not get both refusals: %q", out)
	}
	for _, e := range p.Rec.Events() {
		if e.Type == recorder.EventToolCallRequest && e.Status != recorder.StatusBlocked {
			t.Errorf("request %s status = %s, want blocked", e.RPCID, e.Status)
		}
	}
}

func TestGateApprovalApprovedWithFeedback(t *testing.T) {
	b := policy.NewBroker()
	p := &StdioProxy{Policy: newEngine(t), Approvals: b}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(5, "shell", "rm -rf ./build"))
		ev := waitFor(t, rec, "awaiting event", func(e recorder.Event) bool { return e.Status == recorder.StatusAwaiting })
		if ev.Rule != "rm-recursive" || ev.Decision != "approve" {
			t.Errorf("awaiting event = %+v", ev)
		}
		// While held, the call must NOT have reached the server.
		time.Sleep(100 * time.Millisecond)
		if err := b.Resolve(ev.Seq, policy.Verdict{Approved: true, Feedback: "only build/, nothing else"}); err != nil {
			t.Errorf("resolve: %v", err)
		}
		waitFor(t, rec, "response 5", responseTo(rec, "5"))
	})
	if !strings.Contains(out, "SERVER-RAN shell") {
		t.Fatalf("approved call was not forwarded: %q", out)
	}
	if !strings.Contains(out, "only build/, nothing else") {
		t.Fatalf("operator note not appended to the result: %q", out)
	}
	var sawResolved bool
	for _, e := range p.Rec.Events() {
		if e.Type == recorder.EventApprovalResolved && e.Decision == "approved" {
			sawResolved = true
		}
	}
	if !sawResolved {
		t.Fatal("no ApprovalResolved event recorded")
	}
}

func TestGateApprovalRejected(t *testing.T) {
	b := policy.NewBroker()
	p := &StdioProxy{Policy: newEngine(t), Approvals: b}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(5, "shell", "git push --force origin main"))
		ev := waitFor(t, rec, "awaiting event", func(e recorder.Event) bool { return e.Status == recorder.StatusAwaiting })
		b.Resolve(ev.Seq, policy.Verdict{Approved: false, Feedback: "use a PR instead"})
		waitFor(t, rec, "response 5", responseTo(rec, "5"))
	})
	if strings.Contains(out, "SERVER-RAN") {
		t.Fatalf("rejected call reached the server: %q", out)
	}
	if !strings.Contains(out, "REJECTED") || !strings.Contains(out, "use a PR instead") {
		t.Fatalf("agent did not get the rejection + feedback: %q", out)
	}
}

func TestGateApprovalTimeoutFailsClosed(t *testing.T) {
	p := &StdioProxy{Policy: newEngine(t), Approvals: policy.NewBroker(), ApprovalTimeout: 80 * time.Millisecond}
	var resp recorder.Event
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(5, "shell", "sudo rm file"))
		resp = waitFor(t, rec, "response 5", responseTo(rec, "5"))
	})
	if strings.Contains(out, "SERVER-RAN") || !strings.Contains(out, "No human approved") {
		t.Fatalf("timeout must refuse the call: %q", out)
	}
	// The wait for the human is not tool latency: nothing ran, so no duration.
	if resp.DurationMs != 0 {
		t.Fatalf("a refused call must not report the approval wait as duration, got %dms", resp.DurationMs)
	}
}

func TestGateNoCockpitFailsClosed(t *testing.T) {
	p := &StdioProxy{Policy: newEngine(t)} // Approvals == nil
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(5, "shell", "git reset --hard"))
		waitFor(t, rec, "response 5", responseTo(rec, "5"))
	})
	if strings.Contains(out, "SERVER-RAN") || !strings.Contains(out, "no cockpit") {
		t.Fatalf("approval without cockpit must be refused: %q", out)
	}
}

func TestGateClientCancelDropsHeldCall(t *testing.T) {
	b := policy.NewBroker()
	p := &StdioProxy{Policy: newEngine(t), Approvals: b}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(5, "shell", "rm -rf ./x"))
		waitFor(t, rec, "awaiting", func(e recorder.Event) bool { return e.Status == recorder.StatusAwaiting })
		io.WriteString(w, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":5}}`+"\n")
		waitFor(t, rec, "cancelled", func(e recorder.Event) bool {
			return e.Type == recorder.EventApprovalResolved && e.Decision == "cancelled"
		})
	})
	if out != "" {
		t.Fatalf("a cancelled call must produce no reply, got %q", out)
	}
	if len(b.Pending()) != 0 {
		t.Fatalf("approval left pending: %v", b.Pending())
	}
}

func TestGateRefusesBatchWithToolCall(t *testing.T) {
	p := &StdioProxy{Policy: newEngine(t)}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"shell","arguments":{"command":"rm -rf /"}}}]`+"\n")
		waitFor(t, rec, "batch event", func(e recorder.Event) bool { return e.Rule == "batch-tools-call" })
	})
	if strings.Contains(out, "SERVER-RAN") || !strings.Contains(out, "batches") {
		t.Fatalf("batch must be refused: %q", out)
	}
}

func TestGateBudgetBreaker(t *testing.T) {
	b := policy.NewBroker()
	p := &StdioProxy{Policy: newEngine(t), Approvals: b, Budget: policy.NewBudget(0.0001, 0)}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(1, "shell", strings.Repeat("a", 2000))) // ~500 output tokens ≈ $0.0075
		ev := waitFor(t, rec, "budget hold", func(e recorder.Event) bool { return e.Status == recorder.StatusAwaiting })
		if ev.Rule != "budget" {
			t.Errorf("rule = %q, want budget", ev.Rule)
		}
		b.Resolve(ev.Seq, policy.Verdict{Approved: false})
		waitFor(t, rec, "response 1", responseTo(rec, "1"))
	})
	if strings.Contains(out, "SERVER-RAN") {
		t.Fatalf("over-budget call must not run when rejected: %q", out)
	}
}

// A budget created with no limits (as with --ui) lets everything through until
// a human sets one; from then on the next call is held.
func TestGateBudgetLimitSetMidSession(t *testing.T) {
	b := policy.NewBroker()
	budget := policy.NewBudget(0, 0)
	p := &StdioProxy{Policy: newEngine(t), Approvals: b, Budget: budget}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(1, "shell", "ls"))
		waitFor(t, rec, "response 1", responseTo(rec, "1"))

		if err := budget.SetLimits(0.0001, 0); err != nil {
			t.Error(err)
		}
		io.WriteString(w, call(2, "shell", strings.Repeat("a", 2000)))
		ev := waitFor(t, rec, "budget hold", func(e recorder.Event) bool { return e.Status == recorder.StatusAwaiting })
		if ev.Rule != "budget" {
			t.Errorf("rule = %q, want budget", ev.Rule)
		}
		b.Resolve(ev.Seq, policy.Verdict{Approved: false})
		waitFor(t, rec, "response 2", responseTo(rec, "2"))
	})
	if !strings.Contains(out, "SERVER-RAN shell") {
		t.Fatalf("the call before the limit must have run: %q", out)
	}
	if strings.Count(out, "SERVER-RAN") != 1 {
		t.Fatalf("the over-budget call must not run when rejected: %q", out)
	}
}

// A limitless budget must not change what passes: record-only mode stays
// record-only, batches included.
func TestGateEmptyBudgetDoesNotGate(t *testing.T) {
	p := &StdioProxy{Budget: policy.NewBudget(0, 0)}
	out := runGate(t, p, func(rec *recorder.Recorder, w io.Writer) {
		io.WriteString(w, call(1, "shell", "sudo rm file"))
		waitFor(t, rec, "response 1", responseTo(rec, "1"))
	})
	if !strings.Contains(out, "SERVER-RAN") {
		t.Fatalf("with no policy and no limit the call must pass: %q", out)
	}
}
