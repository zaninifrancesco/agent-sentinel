import { useEffect, useMemo, useRef, useState } from "react";
import type { Row, SentinelEvent, ServerMessage, Session } from "./types";
import { buildRows } from "./rows";
export { buildRows } from "./rows";

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
