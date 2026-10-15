package cursorhooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func (h *Handler) hookEnvelope(in input) recorder.Event {
	source, session := "cursor-hook", in.ConversationID
	var native *recorder.NativeID
	if isClaudeEvent(in.Event) {
		source, session = "claude-hook", in.SessionID
		if in.ToolUseID != "" {
			native = &recorder.NativeID{Type: "string", Value: in.ToolUseID}
		}
	}
	return recorder.Event{Source: source, SourceID: h.Rec.Session().ID + "/" + source, AgentSessionID: session, NativeCallID: native, ProjectRef: in.Cwd}
}

// Fallback keys retain JSON number precision and canonicalize object order.
// They use the original supplied arguments, never a normalized/truncated
// payload, and cannot cross source or conversation namespaces.
func hookIdentity(in input, tool string) (string, string) {
	if in.ConversationID == "" && in.SessionID == "" {
		return "", "none"
	}
	if isClaudeEvent(in.Event) && in.ToolUseID != "" {
		return "native/" + in.ToolUseID, "native"
	}
	var args any
	if tool == "shell" && !isClaudeEvent(in.Event) {
		args = in.Command
	} else {
		raw := in.ToolInput
		var encoded string
		if json.Unmarshal(raw, &encoded) == nil {
			raw = []byte(encoded)
		}
		if len(raw) == 0 {
			raw = []byte("{}")
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&args) != nil {
			return "", "none"
		}
		// Reject trailing data rather than assigning an identity to a prefix.
		if !json.Valid(raw) {
			return "", "none"
		}
	}
	canonical, _ := json.Marshal([]any{in.MCPServer, in.ToolName, tool, args})
	hash := sha256.Sum256(canonical)
	return "args/" + hex.EncodeToString(hash[:]), "fallback"
}

func reportedTiming(value *float64) *recorder.Timing {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value >= float64(math.MaxInt64) {
		return nil
	}
	return &recorder.Timing{DurationMs: int64(*value), Source: "source-reported"}
}

func legacyDuration(value *float64) int64 {
	if timing := reportedTiming(value); timing != nil {
		return timing.DurationMs
	}
	return 0
}

func (h *Handler) observeComplete(in input, tool, text string, status recorder.Status) {
	e := h.hookEnvelope(in)
	e.Type, e.Direction, e.Method = recorder.EventToolCallResponse, recorder.ServerToClient, "tools/call"
	e.ToolName, e.Status = tool, status
	e.Timing = reportedTiming(in.Duration)
	if isClaudeEvent(in.Event) {
		e.Timing = reportedTiming(in.DurationMS)
	}
	if e.Timing != nil {
		e.DurationMs = e.Timing.DurationMs
	}
	// rpcId is a compatibility alias for a matched internal hook request, not
	// the agent's native identity. Orphans keep no invented request id.
	e.Payload, _ = json.Marshal(map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": truncate(text)}}, "isError": status == recorder.StatusError}})
	identity, method := hookIdentity(in, tool)
	h.calls.End(e, identity, method, false, nil)
}
