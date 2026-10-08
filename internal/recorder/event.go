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
	EventSessionStarted   EventType = "SessionStarted"
	EventSessionCompleted EventType = "SessionCompleted"
	EventRequest          EventType = "Request"          // generic MCP/JSON-RPC request
	EventResponse         EventType = "Response"         // generic response
	EventNotification     EventType = "Notification"     // JSON-RPC notification
	EventToolCallRequest  EventType = "ToolCallRequest"  // tools/call
	EventToolCallResponse EventType = "ToolCallResponse" // result of tools/call
	EventRawOutput        EventType = "RawOutput"        // non JSON-RPC line seen on the wire
	EventApprovalResolved EventType = "ApprovalResolved" // a held call was approved, rejected or timed out
	EventBudgetChanged    EventType = "BudgetChanged"    // a human changed the session budget from the cockpit
)

// Direction is the way a frame travelled through the proxy.
type Direction string

const (
	ClientToServer Direction = "client->server"
	ServerToClient Direction = "server->client"
)

// RiskLevel is assigned by the policy engine (Milestone 3). Until then
// everything is RiskNone.
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
)

// Event is the immutable record of one thing that happened in a session.
type Event struct {
	Seq        uint64          `json:"seq"`
	SessionID  string          `json:"sessionId"`
	Timestamp  time.Time       `json:"timestamp"`
	Type       EventType       `json:"type"`
	Direction  Direction       `json:"direction,omitempty"`
	Method     string          `json:"method,omitempty"`
	ToolName   string          `json:"toolName,omitempty"`
	RPCID      string          `json:"rpcId,omitempty"`
	Risk       RiskLevel       `json:"risk"`
	Decision   string          `json:"decision,omitempty"` // policy outcome: allow|warn|approve|block
	Rule       string          `json:"rule,omitempty"`     // id of the rule that fired
	Reason     string          `json:"reason,omitempty"`   // human-readable why
	Status     Status          `json:"status"`
	DurationMs int64           `json:"durationMs,omitempty"` // set on responses
	Payload    json.RawMessage `json:"payload,omitempty"`    // raw frame, or JSON string for RawOutput
}

// Session groups the events of one supervised agent run.
type Session struct {
	ID        string    `json:"id"`
	Command   []string  `json:"command"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
}
