// Package recorder defines the canonical event schema and the flight
// recorder that stores it.
package recorder

import (
	"encoding/json"
	"time"
)

// EventType identifies what happened.
type EventType string

const (
	EventSessionStarted    EventType = "SessionStarted"
	EventSessionCompleted  EventType = "SessionCompleted"
	EventRequest           EventType = "Request"          // generic MCP/JSON-RPC request
	EventResponse          EventType = "Response"         // generic response
	EventNotification      EventType = "Notification"     // JSON-RPC notification
	EventToolCallRequest   EventType = "ToolCallRequest"  // tools/call
	EventToolCallResponse  EventType = "ToolCallResponse" // result of tools/call
	EventRawOutput         EventType = "RawOutput"        // non JSON-RPC line seen on the wire
	EventApprovalResolved  EventType = "ApprovalResolved" // a held call was approved, rejected or timed out
	EventBudgetChanged     EventType = "BudgetChanged"    // a human changed the session budget from the cockpit
	EventCaptureDiagnostic EventType = "CaptureDiagnostic"
)

// Direction is the way a frame travelled through the proxy.
type Direction string

const (
	ClientToServer Direction = "client->server"
	ServerToClient Direction = "server->client"
)

// RiskLevel retains historical policy results. Observers use RiskNone.
type RiskLevel string

const (
	RiskNone     RiskLevel = "none"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Status is the lifecycle state of an event.
type Status string

const (
	StatusPending  Status = "pending"
	StatusAwaiting Status = "awaiting_approval" // held by the policy, waiting for a human
	StatusOK       Status = "ok"
	StatusError    Status = "error"
	StatusBlocked  Status = "blocked"  // refused by policy
	StatusRejected Status = "rejected" // a human said no (or the approval timed out)

	// StatusInterrupted: the call started and never reported a result, because it
	// was skipped, declined, cancelled or cut short. Not a failure of the tool.
	StatusInterrupted Status = "interrupted"
)

// Event is the immutable record of one thing that happened in a session.
type Event struct {
	SchemaVersion  int             `json:"schemaVersion,omitempty"`
	RecordingID    string          `json:"recordingId,omitempty"`
	Source         string          `json:"source,omitempty"`
	SourceID       string          `json:"sourceId,omitempty"`
	AgentSessionID string          `json:"agentSessionId,omitempty"`
	ConnectionID   string          `json:"connectionId,omitempty"`
	ProjectRef     string          `json:"projectRef,omitempty"`
	CallID         string          `json:"callId,omitempty"`
	NativeCallID   *NativeID       `json:"nativeCallId,omitempty"`
	Correlation    *Correlation    `json:"correlation,omitempty"`
	Timing         *Timing         `json:"timing,omitempty"`
	Seq            uint64          `json:"seq"`
	SessionID      string          `json:"sessionId"`
	Timestamp      time.Time       `json:"timestamp"`
	Type           EventType       `json:"type"`
	Direction      Direction       `json:"direction,omitempty"`
	Method         string          `json:"method,omitempty"`
	ToolName       string          `json:"toolName,omitempty"`
	RPCID          string          `json:"rpcId,omitempty"`
	Risk           RiskLevel       `json:"risk"`
	Decision       string          `json:"decision,omitempty"` // policy outcome: allow|warn|approve|block
	Rule           string          `json:"rule,omitempty"`     // id of the rule that fired
	Reason         string          `json:"reason,omitempty"`   // human-readable why
	Status         Status          `json:"status"`
	DurationMs     int64           `json:"durationMs,omitempty"` // set on responses
	Payload        json.RawMessage `json:"payload,omitempty"`    // raw frame, or JSON string for RawOutput
}

// Correlation explains a pairing without rewriting the original observations.
// CandidateCallIDs names every request affected by an ambiguous overlap.
type Correlation struct {
	Method           string   `json:"method"`  // native, fallback, none
	Quality          string   `json:"quality"` // exact, ambiguous, unmatched
	RequestSeq       uint64   `json:"requestSeq,omitempty"`
	CandidateCallIDs []string `json:"candidateCallIds,omitempty"`
	Synthetic        bool     `json:"synthetic,omitempty"`
}

// Timing preserves measured zero separately from missing legacy durationMs.
type Timing struct {
	DurationMs int64  `json:"durationMs"`
	Source     string `json:"source"` // proxy-monotonic or source-reported
}

// NativeID keeps both type and exact value. Numbers are text so a browser's
// JSON parser cannot round an identifier larger than Number.MAX_SAFE_INTEGER.
type NativeID struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func NativeIDFromJSON(raw string) *NativeID {
	if raw == "" {
		return nil
	}
	if raw == "null" {
		return &NativeID{Type: "null"}
	}
	var value string
	if json.Unmarshal([]byte(raw), &value) == nil {
		return &NativeID{Type: "string", Value: value}
	}
	return &NativeID{Type: "number", Value: raw}
}

// Session is recording metadata (the name is retained for API compatibility).
// One recording can contain several native agent sessions.
type Session struct {
	ID        string    `json:"id"`
	Command   []string  `json:"command"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
}
