// Mirrors internal/recorder/event.go.

export type EventType =
  | "SessionStarted"
  | "SessionCompleted"
  | "Request"
  | "Response"
  | "Notification"
  | "ToolCallRequest"
  | "ToolCallResponse"
  | "RawOutput";

export type Status = "pending" | "ok" | "error" | "blocked" | "rejected";
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
  event?: SentinelEvent; // standalone
}
