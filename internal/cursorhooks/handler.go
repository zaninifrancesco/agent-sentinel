// Package cursorhooks lets Sentinel supervise the Cursor agent through Cursor's
// hooks (https://cursor.com/docs/hooks).
//
// Cursor runs a short-lived command for each agent step and reads a permission
// decision from its stdout. `sentinel hook` forwards that JSON to a running
// `sentinel serve`, where Handler turns it into the same events, policy
// decisions and human approvals as an MCP tool call: a shell command is a
// "shell" tool call, an MCP call keeps its tool name, a file edit is recorded
// with its diff. The cockpit, the report and the budget need no special case.
package cursorhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// DefaultTimeout is how long a held command waits for a human.
const DefaultTimeout = 2 * time.Minute

// maxOutput caps the command output stored in the session (the full output
// stays in Cursor); the report and the cockpit do not need megabytes.
const maxOutput = 64 << 10

// Hook events Handler acts on. Anything else is acknowledged and ignored.
const (
	EventBeforeRead  = "beforeReadFile"
	EventBeforeShell = "beforeShellExecution"
	EventAfterShell  = "afterShellExecution"
	EventBeforeMCP   = "beforeMCPExecution"
	EventAfterMCP    = "afterMCPExecution"
	EventAfterEdit   = "afterFileEdit"
	EventStop        = "stop" // the agent's turn ended; it never answers with a decision
)

// Handler serves POST bodies that are Cursor hook inputs.
type Handler struct {
	Rec       *recorder.Recorder
	Engine    *policy.Engine // nil = policy off, only record
	Budget    *policy.Budget // nil = no budget
	Approvals *policy.Broker // nil = a call that needs approval is refused
	Timeout   time.Duration  // 0 = DefaultTimeout

	mu   sync.Mutex
	next uint64
	open map[string][]openCall // in-flight calls awaiting their "after" hook
}

type openCall struct {
	id      string
	tool    string
	started time.Time
}

// New returns a Handler. Engine, Budget and Approvals may be nil.
func New(rec *recorder.Recorder, engine *policy.Engine, budget *policy.Budget, approvals *policy.Broker, timeout time.Duration) *Handler {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Handler{Rec: rec, Engine: engine, Budget: budget, Approvals: approvals, Timeout: timeout, open: make(map[string][]openCall)}
}

// input is the union of the fields of the hooks we handle.
type input struct {
	Event          string          `json:"hook_event_name"`
	ConversationID string          `json:"conversation_id"`
	Command        string          `json:"command"`
	Cwd            string          `json:"cwd"`
	Output         string          `json:"output"`
	Duration       float64         `json:"duration"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	MCPServer      string          `json:"mcp_server_name"`
	ResultJSON     string          `json:"result_json"`
	FilePath       string          `json:"file_path"`
	Content        string          `json:"content"`       // beforeReadFile: never stored
	ContentBytes   int             `json:"content_bytes"` // set by `sentinel hook` when it shortened Content
	StopStatus     string          `json:"status"`        // stop: completed | aborted | error
	Edits          []struct {
		Old string `json:"old_string"`
		New string `json:"new_string"`
	} `json:"edits"`
}

// Response is what Cursor reads back from the hook.
type Response struct {
	Permission   string `json:"permission,omitempty"` // allow | deny | ask
	UserMessage  string `json:"user_message,omitempty"`
	AgentMessage string `json:"agent_message,omitempty"`
}

func allow() Response { return Response{Permission: "allow"} }

// ServeHTTP decodes one hook input and answers with Cursor's output schema.
// The caller (the server) has already applied the loopback and CSRF defences.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad hook input", http.StatusBadRequest)
		return
	}
	var resp Response
	switch in.Event {
	case EventBeforeShell:
		resp = h.before(r.Context(), in, "shell", shellArgs(in), in.Command)
	case EventBeforeMCP:
		tool, args := mcpCall(in)
		resp = h.before(r.Context(), in, tool, args, in.ToolName+"\x00"+string(in.ToolInput))
	case EventBeforeRead:
		resp = h.before(r.Context(), in, "read_file", readArgs(in), in.FilePath)
		// beforeReadFile answers with permission and user_message only; a
		// response that does not match its schema blocks the read.
		resp.AgentMessage = ""
	case EventAfterShell:
		h.afterShell(in)
	case EventAfterMCP:
		h.afterMCP(in)
	case EventAfterEdit:
		h.afterEdit(in)
	case EventStop:
		h.stop(in)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) newID() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	return fmt.Sprintf("h%d", h.next)
}

// shellArgs are the arguments recorded for a shell command. Cursor does not
// always send the working directory (it came empty in real use), and an empty
// argument is noise in the cockpit.
func shellArgs(in input) map[string]any {
	args := map[string]any{"command": in.Command}
	if in.Cwd != "" {
		args["cwd"] = in.Cwd
	}
	return args
}

// readArgs are the arguments recorded for a file read: the path and the size,
// never the content (it would put the file's secrets into the session log).
func readArgs(in input) map[string]any {
	return map[string]any{"file_path": in.FilePath, "bytes": readBytes(in)}
}

func readBytes(in input) int {
	if in.ContentBytes > 0 {
		return in.ContentBytes
	}
	return len(in.Content)
}

// maxScan caps how much of a file is searched for secrets.
const maxScan = 256 << 10

// contentFlag searches what the model is about to read for secrets. It only
// flags: the file path decides whether the read is allowed. The reason names
// the rule, not the secret, so a real key never lands in the log or the report.
func (h *Handler) contentFlag(in input) (policy.Result, bool) {
	if h.Engine == nil || in.Content == "" {
		return policy.Result{}, false
	}
	text := in.Content
	if len(text) > maxScan {
		text = text[:maxScan]
	}
	res := h.Engine.EvaluateWhere(policy.Call{Tool: "read_file", Texts: []string{text}},
		func(r policy.Rule) bool { return strings.HasPrefix(r.ID, "secret-") })
	if res.Decision == policy.Allow {
		return policy.Result{}, false
	}
	desc, _, _ := strings.Cut(res.Reason, " ("+res.RuleID+")")
	return policy.Result{
		Decision: policy.Warn, Risk: res.Risk, RuleID: res.RuleID,
		Reason: fmt.Sprintf("the file's content matches a secret rule: %s", desc),
	}, true
}

// mcpCall names an MCP tool as "server:tool" and parses its JSON parameters.
func mcpCall(in input) (string, map[string]any) {
	tool := in.ToolName
	if in.MCPServer != "" {
		tool = in.MCPServer + ":" + in.ToolName
	}
	return tool, parseArgs(in.ToolInput)
}

// parseArgs accepts the parameters as Cursor sends them for MCP (a JSON string
// holding JSON) or as a plain object. Anything unreadable is kept as text, so
// the policy still sees every character.
func parseArgs(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		var args map[string]any
		if json.Unmarshal([]byte(asString), &args) == nil {
			return args
		}
		return map[string]any{"raw": asString}
	}
	var args map[string]any
	if json.Unmarshal(raw, &args) == nil {
		return args
	}
	return map[string]any{"raw": string(raw)}
}

func callKey(in input, key string) string { return in.ConversationID + "\x00" + key }

func requestPayload(id, tool string, args map[string]any) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	return b
}

func resultPayload(id, text string, isError bool) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id,
		"result": map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isError},
	})
	return b
}

// before is the gate: it records the call, applies the policy and the budget,
// holds it for a human when needed, and returns Cursor's permission.
func (h *Handler) before(ctx context.Context, in input, tool string, args map[string]any, key string) Response {
	id := h.newID()
	payload := requestPayload(id, tool, args)

	res := policy.Result{Decision: policy.Allow, Risk: recorder.RiskNone}
	if h.Engine != nil {
		res = h.Engine.Evaluate(policy.NewCall(tool, args))
	}
	isRead := in.Event == EventBeforeRead
	if isRead && res.Decision == policy.Allow {
		if flag, ok := h.contentFlag(in); ok {
			res = flag
		}
	}
	budgetHit := false
	if h.Budget != nil {
		if isRead {
			h.Budget.AddInput(readBytes(in)) // the file goes to the model
		} else {
			h.Budget.AddOutput(len(payload))
		}
		if hit, why := h.Budget.Exceeded(); hit {
			budgetHit = true
			if res.Decision < policy.RequireApproval {
				res = policy.Result{Decision: policy.RequireApproval, Risk: recorder.RiskHigh, RuleID: "budget", Reason: why}
			}
		}
	}

	req := recorder.Event{
		Type: recorder.EventToolCallRequest, Direction: recorder.ClientToServer,
		Method: "tools/call", ToolName: tool, RPCID: id, Payload: payload,
		Status: recorder.StatusPending, Risk: res.Risk,
	}
	if res.Decision != policy.Allow {
		req.Decision, req.Rule, req.Reason = res.Decision.String(), res.RuleID, res.Reason
	}

	switch res.Decision {
	case policy.Block:
		req.Status = recorder.StatusBlocked
		h.Rec.Record(req)
		return h.refuse(req, recorder.StatusBlocked, res,
			fmt.Sprintf("Blocked by Agent Sentinel policy [%s]: %s. The call was NOT executed.", res.RuleID, res.Reason))

	case policy.RequireApproval:
		if h.Approvals == nil {
			req.Status = recorder.StatusBlocked
			req.Reason += " (approval needed but no cockpit is attached)"
			h.Rec.Record(req)
			return h.refuse(req, recorder.StatusBlocked, res,
				fmt.Sprintf("Blocked by Agent Sentinel [%s]: %s. Nobody can approve it. The call was NOT executed.", res.RuleID, res.Reason))
		}
		return h.hold(ctx, in, req, key, res, budgetHit)

	default:
		if isRead {
			// Nothing reports back after a read: the row is complete now.
			req.Status = recorder.StatusOK
			h.Rec.Record(req)
			return allow()
		}
		h.Rec.Record(req)
		h.trackOpen(in, key, id, tool)
		return allow()
	}
}

// hold parks the call until the cockpit answers, the timeout passes, or Cursor
// gives up on the hook (it closes the connection).
func (h *Handler) hold(ctx context.Context, in input, req recorder.Event, key string, res policy.Result, budgetHit bool) Response {
	req.Status = recorder.StatusAwaiting
	// Register the approval BEFORE the event is visible, so the cockpit can
	// never answer a request we do not know about yet.
	req = h.Rec.RecordHook(req, func(e recorder.Event) { h.Approvals.Open(e.Seq) })

	wctx, cancel := context.WithTimeout(ctx, h.Timeout)
	verdict, err := h.Approvals.Wait(wctx, req.Seq)
	cancel()

	switch {
	case err == nil && verdict.Approved:
		h.resolved(req, "approved", verdict.Feedback, recorder.StatusPending)
		if budgetHit && h.Budget != nil {
			h.Budget.Extend()
		}
		if in.Event == EventBeforeRead {
			h.Rec.Record(recorder.Event{
				Type: recorder.EventToolCallResponse, Direction: recorder.ServerToClient,
				Method: req.Method, ToolName: req.ToolName, RPCID: req.RPCID, Status: recorder.StatusOK,
				Risk: req.Risk, Decision: req.Decision, Rule: req.Rule, Reason: req.Reason,
				Payload: resultPayload(req.RPCID, "The operator allowed this file to be read.", false),
			})
		} else {
			h.trackOpen(in, key, req.RPCID, req.ToolName)
		}
		r := allow()
		if verdict.Feedback != "" {
			r.AgentMessage = "[Agent Sentinel] The operator approved this call with a note: " + verdict.Feedback
		}
		return r

	case err == nil:
		h.resolved(req, "rejected", verdict.Feedback, recorder.StatusRejected)
		text := "The human operator REJECTED this tool call. It was NOT executed."
		if verdict.Feedback != "" {
			text += " Operator feedback: " + verdict.Feedback
		}
		return h.refuse(req, recorder.StatusRejected, res, text)

	case errors.Is(err, policy.ErrTimeout):
		h.resolved(req, "timeout", fmt.Sprintf("no answer within %s", h.Timeout), recorder.StatusRejected)
		return h.refuse(req, recorder.StatusRejected, res,
			fmt.Sprintf("No human approved this tool call within %s, so it was NOT executed.", h.Timeout))

	default: // Cursor closed the hook: nobody is waiting for the answer
		h.resolved(req, "cancelled", "", recorder.StatusRejected)
		return Response{Permission: "deny", UserMessage: "Agent Sentinel: the approval was cancelled.", AgentMessage: "The approval request was cancelled. The call was NOT executed."}
	}
}

// refuse records the refusal as the call's response and builds Cursor's deny.
func (h *Handler) refuse(req recorder.Event, status recorder.Status, res policy.Result, agentText string) Response {
	h.Rec.Record(recorder.Event{
		Type: recorder.EventToolCallResponse, Direction: recorder.ServerToClient,
		Method: req.Method, ToolName: req.ToolName, RPCID: req.RPCID,
		Risk: req.Risk, Status: status, Decision: req.Decision, Rule: req.Rule, Reason: req.Reason,
		Payload: resultPayload(req.RPCID, agentText, true),
	})
	user := "Agent Sentinel refused this call."
	switch status {
	case recorder.StatusBlocked:
		user = fmt.Sprintf("Agent Sentinel blocked this call [%s]: %s", res.RuleID, res.Reason)
	case recorder.StatusRejected:
		user = "Agent Sentinel: this call was rejected in the cockpit."
	}
	return Response{Permission: "deny", UserMessage: user, AgentMessage: agentText}
}

func (h *Handler) resolved(req recorder.Event, decision, note string, status recorder.Status) {
	payload, _ := json.Marshal(map[string]string{"decision": decision, "note": note})
	h.Rec.Record(recorder.Event{
		Type: recorder.EventApprovalResolved, Direction: recorder.ClientToServer,
		Method: req.Method, ToolName: req.ToolName, RPCID: req.RPCID,
		Risk: req.Risk, Status: status, Decision: decision, Rule: req.Rule, Reason: note, Payload: payload,
	})
}

func (h *Handler) trackOpen(in input, key, id, tool string) {
	k := callKey(in, key)
	h.mu.Lock()
	h.open[k] = append(h.open[k], openCall{id: id, tool: tool, started: time.Now()})
	h.mu.Unlock()
}

// takeOpen pops the oldest in-flight call with this key (the same command run
// twice resolves in order).
func (h *Handler) takeOpen(in input, key string) (openCall, bool) {
	k := callKey(in, key)
	h.mu.Lock()
	defer h.mu.Unlock()
	q := h.open[k]
	if len(q) == 0 {
		return openCall{}, false
	}
	c := q[0]
	if len(q) == 1 {
		delete(h.open, k)
	} else {
		h.open[k] = q[1:]
	}
	return c, true
}

// complete records the response of a call. When its "before" was never seen
// (the hook was installed mid-command, or the daemon restarted) it records the
// request too, so the call still appears as one row.
func (h *Handler) complete(in input, key, tool string, args map[string]any, text string, status recorder.Status, durationMs int64) {
	c, ok := h.takeOpen(in, key)
	if !ok {
		c = openCall{id: h.newID(), tool: tool}
		h.Rec.Record(recorder.Event{
			Type: recorder.EventToolCallRequest, Direction: recorder.ClientToServer,
			Method: "tools/call", ToolName: tool, RPCID: c.id, Status: recorder.StatusPending,
			Payload: requestPayload(c.id, tool, args),
		})
	}
	if h.Budget != nil {
		h.Budget.AddInput(len(text))
	}
	h.Rec.Record(recorder.Event{
		Type: recorder.EventToolCallResponse, Direction: recorder.ServerToClient,
		Method: "tools/call", ToolName: c.tool, RPCID: c.id, Status: status, DurationMs: durationMs,
		Payload: resultPayload(c.id, truncate(text), status == recorder.StatusError),
	})
}

// stop closes every call of this conversation that never got its "after" hook.
// Cursor sends no "after" when the user skips a command or stops the agent, so
// without this the row would stay "Running" for the rest of the session.
func (h *Handler) stop(in input) {
	prefix := in.ConversationID + "\x00"
	h.mu.Lock()
	var left []openCall
	for k, q := range h.open {
		if strings.HasPrefix(k, prefix) {
			left = append(left, q...)
			delete(h.open, k)
		}
	}
	h.mu.Unlock()

	why := "Cursor ended the turn"
	if in.StopStatus != "" {
		why += " (" + in.StopStatus + ")"
	}
	text := why + " before reporting a result for this call. It may have been skipped, cancelled or interrupted."
	for _, c := range left {
		h.Rec.Record(recorder.Event{
			Type: recorder.EventToolCallResponse, Direction: recorder.ServerToClient,
			Method: "tools/call", ToolName: c.tool, RPCID: c.id, Status: recorder.StatusError,
			DurationMs: time.Since(c.started).Milliseconds(),
			Payload:    resultPayload(c.id, text, true),
		})
	}
}

func (h *Handler) afterShell(in input) {
	h.complete(in, in.Command, "shell", shellArgs(in), in.Output, recorder.StatusOK, int64(in.Duration))
}

func (h *Handler) afterMCP(in input) {
	tool, args := mcpCall(in)
	status := recorder.StatusOK
	var res struct {
		IsError bool `json:"isError"`
	}
	if json.Unmarshal([]byte(in.ResultJSON), &res) == nil && res.IsError {
		status = recorder.StatusError
	}
	h.complete(in, in.ToolName+"\x00"+string(in.ToolInput), tool, args, in.ResultJSON, status, int64(in.Duration))
}

// afterEdit records a file the agent wrote, with its diff. The edit has
// already happened, so policy can only flag it (a secret written to a file, a
// .env touched), never stop it.
func (h *Handler) afterEdit(in input) {
	id := h.newID()
	patch := editDiff(in)
	// "file_path" sorts before "patch", so the cockpit shows the file as the row subtitle.
	args := map[string]any{"file_path": in.FilePath, "patch": patch}

	ev := recorder.Event{
		Type: recorder.EventToolCallRequest, Direction: recorder.ClientToServer,
		Method: "tools/call", ToolName: "edit_file", RPCID: id, Status: recorder.StatusOK,
		Payload: requestPayload(id, "edit_file", args),
	}
	if h.Engine != nil {
		texts := []string{in.FilePath}
		for _, e := range in.Edits {
			texts = append(texts, e.New)
		}
		call := policy.NewCall("edit_file", map[string]any{"file_path": in.FilePath})
		call.Texts = texts
		if res := h.Engine.Evaluate(call); res.Decision != policy.Allow {
			ev.Decision, ev.Rule, ev.Risk = policy.Warn.String(), res.RuleID, res.Risk
			ev.Reason = "after the fact, the edit was already applied: " + res.Reason
		}
	}
	h.Rec.Record(ev)
}

// editDiff renders Cursor's search/replace edits as a unified diff the cockpit
// can show. Line numbers are unknown, so each hunk starts at line 1.
func editDiff(in input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", strings.TrimPrefix(in.FilePath, "/"), strings.TrimPrefix(in.FilePath, "/"))
	for _, e := range in.Edits {
		oldLines, newLines := splitLines(e.Old), splitLines(e.New)
		fmt.Fprintf(&b, "@@ -%s +%s @@\n", hunkRange(len(oldLines)), hunkRange(len(newLines)))
		for _, l := range oldLines {
			b.WriteString("-" + l + "\n")
		}
		for _, l := range newLines {
			b.WriteString("+" + l + "\n")
		}
	}
	return b.String()
}

// hunkRange is the "start,count" of one side of a hunk. An empty side starts
// at 0, as in a real diff of a pure insertion or deletion.
func hunkRange(n int) string {
	if n == 0 {
		return "0,0"
	}
	return fmt.Sprintf("1,%d", n)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func truncate(s string) string {
	if len(s) <= maxOutput {
		return s
	}
	cut := s[:maxOutput]
	for !utf8.ValidString(cut) && len(cut) > 0 {
		cut = cut[:len(cut)-1]
	}
	return cut + fmt.Sprintf("\n… [%d more bytes not stored]", len(s)-len(cut))
}
