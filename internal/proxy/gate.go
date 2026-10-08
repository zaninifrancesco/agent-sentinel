package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/protocol"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// DefaultApprovalTimeout is how long a held call waits for a human.
const DefaultApprovalTimeout = 2 * time.Minute

// syncWriter serialises writes: the pump goroutine and the approval
// goroutines both write to the same pipe, and lines must never interleave.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// Gate is the execution boundary. It looks at every tools/call going from the
// agent to the MCP server and, before it is forwarded, lets the policy decide:
//
//	allow / warn      -> forward (warn is flagged in the cockpit)
//	block             -> never reaches the server; the agent gets an error result
//	require approval  -> held; forwarded only if a human approves in the cockpit
//
// Everything is fail-closed: a call that needs approval when nobody can
// approve it is refused, a timeout is a refusal, and a tools/call the gate
// cannot even parse is refused.
type Gate struct {
	engine    *policy.Engine
	budget    *policy.Budget
	approvals *policy.Broker
	timeout   time.Duration

	icpt     *Interceptor
	rec      *recorder.Recorder
	toServer io.Writer
	toClient io.Writer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	closed bool
	held   map[string]context.CancelFunc // rpc id -> abort the wait
	notes  map[string]string             // rpc id -> operator note to append to the result
}

func newGate(ctx context.Context, p *StdioProxy, toServer, toClient io.Writer) *Gate {
	gctx, cancel := context.WithCancel(ctx)
	timeout := p.ApprovalTimeout
	if timeout <= 0 {
		timeout = DefaultApprovalTimeout
	}
	return &Gate{
		engine:    p.Policy,
		budget:    p.Budget,
		approvals: p.Approvals,
		timeout:   timeout,
		icpt:      NewInterceptor(p.Rec),
		rec:       p.Rec,
		toServer:  toServer,
		toClient:  toClient,
		ctx:       gctx,
		cancel:    cancel,
		held:      make(map[string]context.CancelFunc),
		notes:     make(map[string]string),
	}
}

func (g *Gate) active() bool { return g.engine != nil || g.budget != nil }

// Shutdown aborts every pending approval and waits for the goroutines.
func (g *Gate) Shutdown() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	g.cancel()
	g.wg.Wait()
}

// FromClient is the Filter for agent -> server traffic.
func (g *Gate) FromClient(line []byte) []byte {
	if !g.active() {
		g.icpt.Observe(recorder.ClientToServer, line)
		return line
	}
	frame := bytes.TrimRight(line, "\r\n")
	trimmed := bytes.TrimSpace(frame)

	// A JSON-RPC batch could smuggle a tools/call past the per-message
	// inspection below. MCP no longer uses batches, so refuse them.
	if len(trimmed) > 0 && trimmed[0] == '[' && bytes.Contains(trimmed, []byte(`"tools/call"`)) {
		g.refuseBatch(trimmed)
		return nil
	}

	msg, err := protocol.Parse(frame)
	if err != nil {
		g.icpt.Observe(recorder.ClientToServer, line)
		return line
	}

	switch {
	case msg.Kind() == protocol.KindNotification && msg.Method == "notifications/cancelled":
		held := g.onCancelled(msg)
		g.icpt.Observe(recorder.ClientToServer, line)
		if held {
			return nil // the server never saw that request
		}
		return line
	case msg.Kind() == protocol.KindRequest && msg.Method == protocol.MethodToolsCall:
		return g.onToolCall(msg, frame, line)
	default:
		g.icpt.Observe(recorder.ClientToServer, line)
		return line
	}
}

func (g *Gate) onToolCall(msg *protocol.Message, frame, line []byte) []byte {
	params, err := msg.AsCallTool()
	if err != nil {
		g.deny(msg, frame, "", policy.Result{
			Decision: policy.Block, Risk: recorder.RiskHigh, RuleID: "unreadable-call",
			Reason: "tools/call params could not be inspected: " + err.Error(),
		}, recorder.StatusBlocked, "")
		return nil
	}

	res := policy.Result{Decision: policy.Allow, Risk: recorder.RiskNone}
	if g.engine != nil {
		res = g.engine.Evaluate(policy.NewCall(params.Name, params.Arguments))
	}
	budgetHit := false
	if g.budget != nil {
		g.budget.AddOutput(len(frame))
		if hit, why := g.budget.Exceeded(); hit {
			budgetHit = true
			if res.Decision < policy.RequireApproval {
				res = policy.Result{Decision: policy.RequireApproval, Risk: recorder.RiskHigh, RuleID: "budget", Reason: why}
			}
		}
	}

	switch res.Decision {
	case policy.Block:
		g.deny(msg, frame, params.Name, res, recorder.StatusBlocked, "")
		return nil
	case policy.RequireApproval:
		if g.approvals == nil {
			res.Reason += " (approval needed but no cockpit is attached; start with --ui)"
			g.deny(msg, frame, params.Name, res, recorder.StatusBlocked, "")
			return nil
		}
		g.hold(msg, frame, params.Name, res, budgetHit)
		return nil
	default: // Allow, Warn
		ann := &annotation{track: true}
		if res.Decision != policy.Allow {
			ann.decision, ann.rule, ann.reason, ann.risk = res.Decision.String(), res.RuleID, res.Reason, res.Risk
		}
		g.icpt.onRequest(recorder.ClientToServer, msg, frame, ann)
		return line
	}
}

// deny records the request as refused and answers the agent without ever
// involving the server.
func (g *Gate) deny(msg *protocol.Message, frame []byte, tool string, res policy.Result, status recorder.Status, text string) {
	req := g.icpt.onRequest(recorder.ClientToServer, msg, frame, &annotation{
		decision: res.Decision.String(), rule: res.RuleID, reason: res.Reason, risk: res.Risk, status: status,
	})
	if text == "" {
		text = fmt.Sprintf("Blocked by Agent Sentinel policy [%s]: %s. The call was NOT executed.", res.RuleID, res.Reason)
	}
	g.reply(req, status, text)
}

// reply sends a synthetic tool result (isError) to the agent and records it.
// The text is what the model reads, so it says what happened and why.
//
// No DurationMs is recorded: no tool ran, and the time a call spent waiting
// for a human (up to the approval timeout) is not tool latency.
func (g *Gate) reply(req recorder.Event, status recorder.Status, text string) {
	result, _ := json.Marshal(protocol.CallToolResult{
		Content: []protocol.Content{{Type: "text", Text: text}},
		IsError: true,
	})
	var id protocol.ID
	_ = id.UnmarshalJSON([]byte(req.RPCID))
	out := protocol.Message{JSONRPC: protocol.Version, ID: &id, Result: result}
	payload, _ := out.Marshal()

	g.rec.Record(recorder.Event{
		Type:      recorder.EventToolCallResponse,
		Direction: recorder.ServerToClient,
		Method:    req.Method,
		ToolName:  req.ToolName,
		RPCID:     req.RPCID,
		Risk:      req.Risk,
		Status:    status,
		Decision:  req.Decision,
		Rule:      req.Rule,
		Reason:    req.Reason,
		Payload:   payload,
	})
	_, _ = g.toClient.Write(append(payload, '\n'))
}

func (g *Gate) refuseBatch(batch []byte) {
	reason := "JSON-RPC batches containing tools/call are not allowed while a policy is active"
	payload, _ := json.Marshal(string(batch))
	g.rec.Record(recorder.Event{
		Type:      recorder.EventRawOutput,
		Direction: recorder.ClientToServer,
		Risk:      recorder.RiskHigh,
		Status:    recorder.StatusBlocked,
		Decision:  policy.Block.String(),
		Rule:      "batch-tools-call",
		Reason:    reason,
		Payload:   payload,
	})
	resp, _ := protocol.NewErrorResponse(protocol.ID{}, protocol.CodeInvalidRequest, reason).Marshal()
	_, _ = g.toClient.Write(append(resp, '\n'))
}

// hold parks a call that needs a human. The pump keeps running, so the agent
// can issue other (allowed) calls meanwhile.
func (g *Gate) hold(msg *protocol.Message, frame []byte, tool string, res policy.Result, budgetHit bool) {
	id := msg.IDString()
	ctx, cancel := context.WithCancel(g.ctx)

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		cancel()
		res.Reason += " (session is shutting down)"
		g.deny(msg, frame, tool, res, recorder.StatusBlocked, "")
		return
	}
	g.held[id] = cancel
	g.wg.Add(1)
	g.mu.Unlock()

	req := g.icpt.onRequest(recorder.ClientToServer, msg, frame, &annotation{
		decision: res.Decision.String(), rule: res.RuleID, reason: res.Reason, risk: res.Risk,
		status: recorder.StatusAwaiting,
		// Register the approval BEFORE the event becomes visible, so the
		// cockpit can never answer a request we do not know about yet.
		hook: func(e recorder.Event) { g.approvals.Open(e.Seq) },
	})
	go g.await(ctx, cancel, msg, bytes.Clone(frame), req, budgetHit)
}

func (g *Gate) await(ctx context.Context, cancel context.CancelFunc, msg *protocol.Message, frame []byte, req recorder.Event, budgetHit bool) {
	defer g.wg.Done()
	id := msg.IDString()

	wctx, wcancel := context.WithTimeout(ctx, g.timeout)
	verdict, err := g.approvals.Wait(wctx, req.Seq)
	wcancel()

	g.mu.Lock()
	delete(g.held, id)
	g.mu.Unlock()
	cancel()

	switch {
	case err == nil && verdict.Approved:
		g.resolved(req, "approved", verdict.Feedback, recorder.StatusPending)
		if budgetHit && g.budget != nil {
			g.budget.Extend()
		}
		if verdict.Feedback != "" {
			g.mu.Lock()
			g.notes[id] = verdict.Feedback
			g.mu.Unlock()
		}
		// The latency clock starts now: waiting for the human is not tool time.
		g.icpt.track(id, pendingCall{method: msg.Method, toolName: req.ToolName})
		_, _ = g.toServer.Write(append(frame, '\n'))

	case err == nil:
		g.resolved(req, "rejected", verdict.Feedback, recorder.StatusRejected)
		text := "The human operator REJECTED this tool call. It was NOT executed."
		if verdict.Feedback != "" {
			text += " Operator feedback: " + verdict.Feedback
		}
		g.reply(req, recorder.StatusRejected, text)

	case errors.Is(err, policy.ErrTimeout):
		g.resolved(req, "timeout", fmt.Sprintf("no answer within %s", g.timeout), recorder.StatusRejected)
		g.reply(req, recorder.StatusRejected, fmt.Sprintf(
			"No human approved this tool call within %s, so it was NOT executed.", g.timeout))

	default: // cancelled by the client, or the session ended: nobody is waiting for an answer
		g.resolved(req, "cancelled", "", recorder.StatusRejected)
	}
}

func (g *Gate) resolved(req recorder.Event, decision, note string, status recorder.Status) {
	payload, _ := json.Marshal(map[string]string{"decision": decision, "note": note})
	g.rec.Record(recorder.Event{
		Type:      recorder.EventApprovalResolved,
		Direction: recorder.ClientToServer,
		Method:    req.Method,
		ToolName:  req.ToolName,
		RPCID:     req.RPCID,
		Risk:      req.Risk,
		Status:    status,
		Decision:  decision,
		Rule:      req.Rule,
		Reason:    note,
		Payload:   payload,
	})
}

// onCancelled handles notifications/cancelled: if the client gives up on a
// call that is still held, stop waiting for the human. It reports whether
// the cancelled request was one of ours.
func (g *Gate) onCancelled(msg *protocol.Message) bool {
	var p struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if json.Unmarshal(msg.Params, &p) != nil || len(p.RequestID) == 0 {
		return false
	}
	var id protocol.ID
	_ = id.UnmarshalJSON(p.RequestID)

	g.mu.Lock()
	cancel, ok := g.held[id.String()]
	g.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// FromServer is the Filter for server -> agent traffic.
func (g *Gate) FromServer(line []byte) []byte {
	if g.active() {
		frame := bytes.TrimRight(line, "\r\n")
		if msg, err := protocol.Parse(frame); err == nil &&
			(msg.Kind() == protocol.KindResponse || msg.Kind() == protocol.KindError) {
			id := msg.IDString()
			if call, ok := g.icpt.peek(id); ok && call.method == protocol.MethodToolsCall {
				if g.budget != nil {
					g.budget.AddInput(len(frame))
				}
				if note := g.takeNote(id); note != "" && msg.Kind() == protocol.KindResponse {
					line = appendNote(msg, note, line)
				}
			}
		}
	}
	g.icpt.Observe(recorder.ServerToClient, line)
	return line
}

func (g *Gate) takeNote(id string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := g.notes[id]
	delete(g.notes, id)
	return n
}

// appendNote adds the operator's feedback as an extra text block of the tool
// result, so the agent reads it together with the output. On any problem it
// returns the original line untouched.
func appendNote(msg *protocol.Message, note string, orig []byte) []byte {
	var result map[string]json.RawMessage
	if json.Unmarshal(msg.Result, &result) != nil {
		return orig
	}
	var content []json.RawMessage
	if raw, ok := result["content"]; ok {
		if json.Unmarshal(raw, &content) != nil {
			return orig
		}
	}
	block, _ := json.Marshal(protocol.Content{Type: "text", Text: "[Agent Sentinel] The operator approved this call with a note: " + note})
	content = append(content, block)
	result["content"], _ = json.Marshal(content)
	msg.Result, _ = json.Marshal(result)
	out, err := msg.Marshal()
	if err != nil {
		return orig
	}
	return append(out, '\n')
}
