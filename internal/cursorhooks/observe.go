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
	e.Payload, _ = json.Marshal(map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": status == recorder.StatusError}})
	e.Capture = h.responseCapture(in, tool)
	if h.Rec.CaptureSettings().Profile == "full" && isClaudeEvent(in.Event) && in.Event == EventClaudePost && in.fields["tool_response"] != nil {
		e.Payload, _ = json.Marshal(map[string]any{"result": in.ToolResponse})
		e.Capture.Completeness = "complete"
	}
	identity, method := hookIdentity(in, tool)
	h.calls.End(e, identity, method, false, nil)
}

func observedHook(in input, field, unit string, size int) *recorder.PayloadCapture {
	meta := recorder.ObservedPayload("hook-normalized", unit, size)
	meta.ObservedField = field
	if in.fields[field] == nil {
		meta.ObservedBytes = nil
		meta.Completeness = "unknown"
	}
	meta.Changes = []recorder.CaptureChange{{Kind: "normalized", Stage: "adapter", Method: "hook-normalization-v1"}}
	return meta
}

func (h *Handler) requestCapture(in input) *recorder.PayloadCapture {
	if in.Event == EventBeforeRead {
		meta := observedHook(in, "content", "utf8-text", len(in.Content))
		if in.ContentBytes != nil && *in.ContentBytes >= 0 {
			n := *in.ContentBytes
			meta.SourceReportedBytes = &n
			meta.SourceReportedUnit = "utf8-text"
			if n > int64(len(in.Content)) {
				meta.Completeness = "partial"
				meta.Changes = append(meta.Changes, recorder.CaptureChange{Kind: "truncated", Stage: "hook-client", Method: "read-forward-limit-v1"})
			}
		}
		if h.Rec.CaptureSettings().Profile == "standard" && in.fields["content"] != nil {
			meta.Completeness = "omitted"
			meta.Changes = append(meta.Changes, recorder.CaptureChange{Kind: "omitted", Stage: "adapter", Method: "read-content-omitted-v1", BeforeBytes: len(in.Content)})
		}
		return meta
	}
	if in.Event == EventBeforeShell {
		return observedHook(in, "command", "utf8-text", len(in.Command))
	}
	if in.Event == EventAfterEdit {
		meta := observedHook(in, "edits", "json-value", len(in.fields["edits"]))
		meta.Completeness = "unknown"
		return meta
	}
	meta := observedHook(in, "tool_input", "json-value", len(in.ToolInput))
	meta.Completeness = "unknown" // tool-specific normalization can omit source fields
	return meta
}

func (h *Handler) responseCapture(in input, tool string) *recorder.PayloadCapture {
	if in.Event == EventAfterShell {
		return observedHook(in, "output", "utf8-text", len(in.Output))
	}
	if in.Event == EventAfterMCP {
		return observedHook(in, "result_json", "utf8-text", len(in.ResultJSON))
	}
	if in.Event == EventClaudePostFailure {
		return observedHook(in, "error", "utf8-text", len(in.Error))
	}
	meta := observedHook(in, "tool_response", "json-value", len(in.ToolResponse))
	if h.Rec.CaptureSettings().Profile == "standard" {
		switch tool {
		case "read_file", "edit_file":
			if in.fields["tool_response"] != nil {
				meta.Completeness = "omitted"
				meta.Changes = append(meta.Changes, recorder.CaptureChange{Kind: "omitted", Stage: "adapter", Method: "tool-result-summary-v1", BeforeBytes: len(in.ToolResponse)})
			}
		case "shell":
			meta.Completeness = "unknown" // stdout/stderr text does not preserve the entire result object
		}
	}
	return meta
}
