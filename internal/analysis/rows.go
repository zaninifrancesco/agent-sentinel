// Package analysis derives deterministic profiler views from retained evidence.
// It never changes captured events or makes execution decisions.
package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

const Version = "profiler-v1"

type Row struct {
	Key            string                `json:"key"`
	Kind           string                `json:"kind"`
	Title          string                `json:"title"`
	Subtitle       string                `json:"subtitle,omitempty"`
	Status         recorder.Status       `json:"status"`
	Risk           recorder.RiskLevel    `json:"risk"`
	StartedAt      time.Time             `json:"startedAt"`
	DurationMs     *int64                `json:"durationMs,omitempty"`
	ToolDurationMs *int64                `json:"toolDurationMs,omitempty"`
	Request        *recorder.Event       `json:"request,omitempty"`
	Response       *recorder.Event       `json:"response,omitempty"`
	Resolution     *recorder.Event       `json:"resolution,omitempty"`
	Event          *recorder.Event       `json:"event,omitempty"`
	Correlation    *recorder.Correlation `json:"correlation,omitempty"`
	RequestBytes   *int                  `json:"requestBytes,omitempty"`
	ResponseBytes  *int                  `json:"responseBytes,omitempty"`
	Span           *Span                 `json:"span,omitempty"`
}

type Span struct {
	StartMs int64  `json:"startMs"`
	EndMs   *int64 `json:"endMs,omitempty"`
}

func identity(e *recorder.Event, id string) string {
	b, _ := json.Marshal([]string{recordingID(e), e.SourceID, e.Source, e.AgentSessionID, e.ConnectionID, id})
	return string(b)
}
func recordingID(e *recorder.Event) string {
	if e.RecordingID != "" {
		return e.RecordingID
	}
	return e.SessionID
}
func legacyKey(e *recorder.Event, response bool) string {
	if e.RPCID == "" {
		return ""
	}
	direction := e.Direction
	if response {
		if direction == recorder.ClientToServer {
			direction = recorder.ServerToClient
		} else if direction == recorder.ServerToClient {
			direction = recorder.ClientToServer
		}
	}
	return identity(e, string(direction)+":"+e.RPCID)
}
func Evidence(r Row) *recorder.Event {
	if r.Request != nil {
		return r.Request
	}
	if r.Response != nil {
		return r.Response
	}
	return r.Event
}
func Scope(r Row) string {
	e := Evidence(r)
	if e == nil {
		return "unattributed"
	}
	source := e.SourceID
	if source == "" {
		source = e.Source
	}
	var parts []string
	if e.AgentSessionID != "" {
		parts = []string{source, "session", e.AgentSessionID}
	} else if e.ConnectionID != "" {
		parts = []string{source, "connection", e.ConnectionID}
	} else {
		return "unattributed"
	}
	b, _ := json.Marshal(parts)
	return string(b)
}
func payloadSize(e *recorder.Event) *int {
	if e == nil || len(e.Payload) == 0 {
		return nil
	}
	n := 0
	if e.Capture != nil {
		n = e.Capture.StoredBytes
	} else {
		var b bytes.Buffer
		if json.Compact(&b, e.Payload) == nil {
			n = b.Len()
		} else {
			n = len(e.Payload)
		}
	}
	return &n
}
func measured(e *recorder.Event) *int64 {
	if e.Status == recorder.StatusBlocked || e.Status == recorder.StatusRejected || e.Correlation != nil && e.Correlation.Synthetic {
		return nil
	}
	if e.Timing != nil && e.Timing.DurationMs >= 0 {
		v := e.Timing.DurationMs
		return &v
	}
	if e.DurationMs > 0 {
		v := e.DurationMs
		return &v
	}
	return nil
}
func ambiguous(r *Row, c *recorder.Correlation) {
	r.Correlation = c
	r.Status = "unknown"
	r.DurationMs = nil
}

// Project uses explicit v2 references, and conservative origin-scoped legacy
// pairing. An ambiguous group is never resolved by response arrival order.
func Project(events []recorder.Event) []Row {
	rows := make([]Row, 0, len(events))
	open := map[string]int{}
	type group struct {
		indices   []int
		remaining int
		ambiguous bool
	}
	legacy := map[string]*group{}
	for i := range events {
		e := &events[i]
		if e.Correlation != nil && e.Correlation.Quality == "ambiguous" {
			for _, id := range e.Correlation.CandidateCallIDs {
				if index, ok := open[identity(e, id)]; ok {
					ambiguous(&rows[index], e.Correlation)
				}
			}
		}
		switch e.Type {
		case recorder.EventToolCallRequest, recorder.EventRequest:
			kind, title, subtitle := "rpc", e.Method, ""
			if title == "" {
				title = "request"
			}
			if e.Type == recorder.EventToolCallRequest {
				kind, title = "tool", e.ToolName
				if title == "" {
					title = "tools/call"
				}
				subtitle = argSummary(e.Payload)
			}
			status := e.Status
			if status == recorder.StatusPending && e.Correlation != nil && e.Correlation.Quality == "unmatched" {
				status = "unknown"
			}
			rows = append(rows, Row{Key: fmt.Sprintf("req-%s-%d", recordingID(e), e.Seq), Kind: kind, Title: title, Subtitle: subtitle, Status: status, Risk: e.Risk, StartedAt: e.Timestamp, Request: e, Correlation: e.Correlation})
			index := len(rows) - 1
			if e.CallID != "" {
				open[identity(e, e.CallID)] = index
			} else if key := legacyKey(e, false); key != "" {
				g := legacy[key]
				if g == nil {
					g = &group{}
					legacy[key] = g
				}
				g.indices = append(g.indices, index)
				g.remaining++
				if len(g.indices) > 1 {
					g.ambiguous = true
					for _, n := range g.indices {
						ambiguous(&rows[n], &recorder.Correlation{Method: "none", Quality: "ambiguous"})
					}
				}
			}
		case recorder.EventToolCallResponse, recorder.EventResponse:
			index := -1
			c := e.Correlation
			if e.CallID != "" {
				if n, ok := open[identity(e, e.CallID)]; ok && c != nil && c.Quality == "exact" && (c.RequestSeq == 0 || rows[n].Request.Seq == c.RequestSeq) {
					if rows[n].Correlation == nil || rows[n].Correlation.Quality != "ambiguous" {
						index = n
						delete(open, identity(e, e.CallID))
					} else {
						c = rows[n].Correlation
					}
				}
				if index < 0 && c != nil && c.Quality == "exact" {
					copy := *c
					copy.Quality = "unmatched"
					c = &copy
				}
			} else if !(e.SchemaVersion == 2 && e.Source != "") {
				key := legacyKey(e, true)
				if g := legacy[key]; g != nil {
					if g.ambiguous {
						c = &recorder.Correlation{Method: "none", Quality: "ambiguous"}
					} else {
						index = g.indices[0]
					}
					g.remaining--
					if g.remaining == 0 {
						delete(legacy, key)
					}
				}
			}
			if index >= 0 {
				r := &rows[index]
				r.Response = e
				r.Status = e.Status
				r.DurationMs = measured(e)
				r.Correlation = c
				if e.Risk != recorder.RiskNone && e.Risk != "" {
					r.Risk = e.Risk
				}
			} else {
				if c == nil {
					c = &recorder.Correlation{Method: "none", Quality: "unmatched"}
				}
				title := e.ToolName
				if title == "" {
					title = e.Method
				}
				if title == "" {
					title = "response"
				}
				subtitle := "Unmatched response · no request captured"
				if c.Quality == "ambiguous" {
					subtitle = "Ambiguous response · no request assigned"
				}
				r := Row{Key: fmt.Sprintf("res-%s-%d", recordingID(e), e.Seq), Kind: "rpc", Title: title, Subtitle: subtitle, Status: e.Status, Risk: e.Risk, StartedAt: e.Timestamp, Response: e, Correlation: c}
				if c.Quality != "ambiguous" {
					r.DurationMs = measured(e)
				}
				rows = append(rows, r)
			}
		case recorder.EventApprovalResolved:
			n, ok := open[identity(e, e.CallID)]
			if e.CallID == "" {
				if g := legacy[legacyKey(e, false)]; g != nil && !g.ambiguous {
					n, ok = g.indices[0], true
				}
			}
			if ok {
				rows[n].Resolution = e
				rows[n].Status = e.Status
			}
		default:
			kind, title, subtitle := "raw", string(e.Type), "Uninterpreted captured event"
			switch e.Type {
			case recorder.EventNotification:
				kind, title, subtitle = "notification", e.Method, ""
			case recorder.EventRawOutput:
				title, subtitle = "raw output", ""
				_ = json.Unmarshal(e.Payload, &subtitle)
			case recorder.EventCaptureDiagnostic:
				kind, title, subtitle = "notification", "Capture diagnostic", e.Reason
			case recorder.EventSessionStarted:
				kind, title, subtitle = "session", "Session started", ""
			case recorder.EventSessionCompleted:
				kind, title, subtitle = "session", "Session completed", ""
			case recorder.EventBudgetChanged:
				kind, title, subtitle = "session", "Budget changed", e.Reason
			}
			rows = append(rows, Row{Key: fmt.Sprintf("ev-%s-%d", recordingID(e), e.Seq), Kind: kind, Title: title, Subtitle: subtitle, Status: e.Status, Risk: e.Risk, StartedAt: e.Timestamp, Event: e})
		}
	}
	for i := range rows {
		r := &rows[i]
		if r.Risk == "" {
			r.Risk = recorder.RiskNone
		}
		r.RequestBytes = payloadSize(r.Request)
		r.ResponseBytes = payloadSize(r.Response)
		r.ToolDurationMs = Duration(*r)
		if r.Request != nil && !r.Request.Timestamp.IsZero() {
			r.Span = &Span{StartMs: r.Request.Timestamp.UnixMilli()}
			if r.Response != nil && !r.Response.Timestamp.IsZero() && (r.Response.Correlation == nil || !r.Response.Correlation.Synthetic) && (r.Correlation == nil || r.Correlation.Quality == "exact") && !r.Response.Timestamp.Before(r.Request.Timestamp) {
				end := r.Response.Timestamp.UnixMilli()
				r.Span.EndMs = &end
			}
		}
	}
	return rows
}

func argSummary(raw json.RawMessage) string {
	var p struct {
		Params struct {
			Arguments any `json:"arguments"`
		} `json:"params"`
	}
	_ = json.Unmarshal(raw, &p)
	var first func(any) string
	first = func(v any) string {
		switch x := v.(type) {
		case string:
			return x
		case []any:
			for _, child := range x {
				if s := first(child); s != "" {
					return s
				}
			}
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range []string{"command", "file_path", "path", "query"} {
				if s, ok := x[k].(string); ok && s != "" {
					return s
				}
			}
			for _, k := range keys {
				if s := first(x[k]); s != "" {
					return s
				}
			}
		}
		return ""
	}
	text := []rune(strings.Join(strings.Fields(first(p.Params.Arguments)), " "))
	if len(text) > 90 {
		return string(text[:89]) + "…"
	}
	return string(text)
}

// Summary strips all retained bodies, while keeping identities, coverage and
// byte measurements. Full events are retrieved only for the selected detail.
func Summary(r Row) Row {
	strip := func(e *recorder.Event) *recorder.Event {
		if e == nil {
			return nil
		}
		copy := *e
		copy.Payload = nil
		return &copy
	}
	r.Request = strip(r.Request)
	r.Response = strip(r.Response)
	r.Resolution = strip(r.Resolution)
	r.Event = strip(r.Event)
	return r
}
