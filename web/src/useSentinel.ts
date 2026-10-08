import { useEffect, useMemo, useRef, useState } from "react";
import type { Row, SentinelEvent, ServerMessage, Session } from "./types";

export type Connection = "connecting" | "live" | "ended" | "reconnecting";

export interface SentinelState {
  session: Session | null;
  events: SentinelEvent[];
  rows: Row[];
  connection: Connection;
}

/** Subscribes to the Sentinel WebSocket (snapshot first, then live events). */
export function useSentinel(): SentinelState {
  const [session, setSession] = useState<Session | null>(null);
  const [events, setEvents] = useState<SentinelEvent[]>([]);
  const [connection, setConnection] = useState<Connection>("connecting");
  const ended = useRef(false);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let timer: number | undefined;
    let stopped = false;

    const connect = () => {
      const scheme = location.protocol === "https:" ? "wss" : "ws";
      ws = new WebSocket(`${scheme}://${location.host}/ws`);
      ws.onmessage = (m) => {
        const msg = JSON.parse(m.data) as ServerMessage;
        if (msg.type === "snapshot") {
          setSession(msg.session);
          setEvents(msg.events);
          ended.current = msg.events.some((e) => e.type === "SessionCompleted");
          setConnection(ended.current ? "ended" : "live");
        } else {
          if (msg.event.type === "SessionCompleted") ended.current = true;
          setEvents((prev) => [...prev, msg.event]);
        }
      };
      ws.onclose = () => {
        if (stopped) return;
        if (ended.current) {
          setConnection("ended");
          return;
        }
        setConnection("reconnecting");
        timer = window.setTimeout(connect, 1500);
      };
    };
    connect();

    return () => {
      stopped = true;
      window.clearTimeout(timer);
      ws?.close();
    };
  }, []);

  const rows = useMemo(() => buildRows(events), [events]);
  return { session, events, rows, connection };
}

/** Merges each request with its response (matched by JSON-RPC id). */
export function buildRows(events: SentinelEvent[]): Row[] {
  const rows: Row[] = [];
  const open = new Map<string, Row>(); // rpcId -> row awaiting a response

  for (const e of events) {
    switch (e.type) {
      case "ToolCallRequest":
      case "Request": {
        const isTool = e.type === "ToolCallRequest";
        const row: Row = {
          key: `req-${e.seq}`,
          kind: isTool ? "tool" : "rpc",
          title: isTool ? e.toolName || "tools/call" : e.method || "request",
          subtitle: isTool ? argSummary(e.payload) : undefined,
          status: e.status,
          risk: e.risk,
          startedAt: e.timestamp,
          request: e,
        };
        rows.push(row);
        if (e.rpcId) open.set(e.rpcId, row);
        break;
      }
      case "ToolCallResponse":
      case "Response": {
        const row = e.rpcId ? open.get(e.rpcId) : undefined;
        if (row) {
          row.response = e;
          row.status = e.status;
          // The server omits 0; a matched response always has a duration.
          row.durationMs = e.durationMs ?? 0;
          if (e.risk !== "none") row.risk = e.risk;
          open.delete(e.rpcId!);
        } else {
          rows.push({
            key: `res-${e.seq}`,
            kind: "rpc",
            title: e.method || "response",
            subtitle: "unmatched response",
            status: e.status,
            risk: e.risk,
            startedAt: e.timestamp,
            durationMs: e.durationMs,
            response: e,
          });
        }
        break;
      }
      case "ApprovalResolved": {
        // Settles a held call: it either runs now (still "pending" until the
        // response arrives) or is rejected.
        const row = e.rpcId ? open.get(e.rpcId) : undefined;
        if (row) {
          row.resolution = e;
          row.status = e.status;
        }
        break;
      }
      case "Notification":
        rows.push(standalone(e, "notification", e.method || "notification"));
        break;
      case "RawOutput":
        rows.push(standalone(e, "raw", "raw output", typeof e.payload === "string" ? e.payload : undefined));
        break;
      case "SessionStarted":
        rows.push(standalone(e, "session", "Session started"));
        break;
      case "SessionCompleted":
        rows.push(standalone(e, "session", "Session completed"));
        break;
    }
  }
  return rows;
}

/** First string argument of a tools/call (the command, the path...), on one line. */
function argSummary(payload: unknown): string | undefined {
  const args = (payload as { params?: { arguments?: unknown } } | undefined)?.params?.arguments;
  const first = (v: unknown): string | undefined => {
    if (typeof v === "string") return v;
    if (Array.isArray(v)) return v.map(first).find((x) => x !== undefined);
    if (v && typeof v === "object") return Object.values(v).map(first).find((x) => x !== undefined);
    return undefined;
  };
  const text = first(args)?.replace(/\s+/g, " ").trim();
  if (!text) return undefined;
  return text.length > 90 ? text.slice(0, 89) + "…" : text;
}

function standalone(e: SentinelEvent, kind: Row["kind"], title: string, subtitle?: string): Row {
  return {
    key: `ev-${e.seq}`,
    kind,
    title,
    subtitle,
    status: e.status,
    risk: e.risk,
    startedAt: e.timestamp,
    event: e,
  };
}
