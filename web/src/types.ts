// Mirrors internal/recorder/event.go.

export type EventType =
  | "SessionStarted"
  | "SessionCompleted"
  | "Request"
  | "Response"
  | "Notification"
  | "ToolCallRequest"
  | "ToolCallResponse"
  | "RawOutput"
  | "ApprovalResolved"
  | "BudgetChanged"
  | "CaptureDiagnostic"
  | (string & {}); // future event types remain inspectable

export type Status =
  | "pending"
  | "awaiting_approval"
  | "ok"
  | "error"
  | "blocked"
  | "rejected"
  | "interrupted"
  | "unknown";
export type Risk = "none" | "low" | "medium" | "high" | "critical";

export interface SentinelEvent {
  capture?: PayloadCapture;
  captureSettings?: CaptureSettings;
  metadataRedactedFields?: string[];
  schemaVersion?: number;
  recordingId?: string;
  source?: string;
  sourceId?: string;
  agentSessionId?: string;
  connectionId?: string;
  projectRef?: string;
  callId?: string;
  nativeCallId?: { type: "string" | "number" | "null"; value: string };
  correlation?: Correlation;
  timing?: { durationMs: number; source: "proxy-monotonic" | "source-reported" };
  seq: number;
  sessionId: string;
  timestamp: string;
  type: EventType;
  direction?: "client->server" | "server->client";
  method?: string;
  toolName?: string;
  rpcId?: string;
  risk: Risk;
  decision?: string; // policy outcome: allow | warn | approve | block (or the human verdict on ApprovalResolved)
  rule?: string; // id of the rule that fired
  reason?: string; // why
  status: Status;
  durationMs?: number;
  payload?: unknown;
}

export interface CaptureSettings {
  profile: "standard" | "full";
  maxPayloadBytes: number;
  redaction: string;
}

export interface PayloadCapture {
  representation: string;
  completeness: "complete" | "partial" | "omitted" | "unknown";
  observedBytes?: number;
  observedUnit?: string;
  observedField?: string;
  sourceReportedBytes?: number;
  sourceReportedUnit?: string;
  normalizedBytes: number;
  storedBytes: number;
  profile: "standard" | "full";
  changes?: { kind: string; stage: string; method: string; count?: number; beforeBytes?: number; afterBytes?: number }[];
}

export interface Correlation {
  method: "native" | "fallback" | "none";
  quality: "exact" | "ambiguous" | "unmatched";
  requestSeq?: number;
  candidateCallIds?: string[];
  synthetic?: boolean;
}

export interface Session {
  capture?: CaptureSettings;
  id: string;
  command: string[] | null;
  startedAt: string;
  endedAt?: string;
}

export type ServerMessage =
  | { type: "snapshot"; session: Session; events: SentinelEvent[] }
  | { type: "event"; event: SentinelEvent };

/** One row of the timeline: a request merged with its response, or a standalone event. */
export interface Row {
  correlation?: Correlation;
  key: string;
  kind: "tool" | "rpc" | "notification" | "raw" | "session";
  title: string;
  subtitle?: string;
  status: Status;
  risk: Risk;
  startedAt: string;
  durationMs?: number;
  request?: SentinelEvent;
  response?: SentinelEvent;
  resolution?: SentinelEvent; // how a held call was settled (ApprovalResolved)
  event?: SentinelEvent; // standalone
}

/** Served at /api/config. */
export interface SentinelConfig {
  capture?: CaptureSettings;
  mode?: "observe" | "replay"; // absent in older recordings/servers
  maxCostUsd: number; // 0 = no budget
  maxTokens: number;
  policy: string;
  approvals: boolean; // can this session approve held calls?
  /** Can the limits be changed from the cockpit? False when replaying. */
  budgetEditable: boolean;
  /** Seconds a held call waits for a decision before it is refused. */
  approvalTimeoutSec: number;
}
