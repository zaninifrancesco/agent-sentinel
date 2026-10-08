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
  | "ApprovalResolved";

export type Status =
  | "pending"
  | "awaiting_approval"
  | "ok"
  | "error"
  | "blocked"
  | "rejected";
export type Risk = "none" | "low" | "medium" | "high" | "critical";

export interface SentinelEvent {
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

export interface Session {
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
  maxCostUsd: number; // 0 = no budget
  maxTokens: number;
  policy: string;
  approvals: boolean; // can this session approve held calls?
}
