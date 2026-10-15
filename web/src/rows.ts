import type { Correlation, Row, SentinelEvent } from "./types";
import { describeBudget } from "./format";

function identity(e: SentinelEvent, id: string): string {
  return JSON.stringify([e.recordingId ?? e.sessionId, e.sourceId ?? e.source ?? "", e.agentSessionId ?? "", e.connectionId ?? "", id]);
}

function legacyKey(e: SentinelEvent, response = false): string | undefined {
  if (!e.rpcId) return undefined;
  const origin = response ? e.direction === "server->client" ? "client->server" : e.direction === "client->server" ? "server->client" : "" : e.direction ?? "";
  return identity(e, `${origin}:${e.rpcId}`);
}

/** V2 uses explicit call identities and evidence references. Legacy pairing
 * is scoped by recording and origin, and never chooses an overlapping ID. */
export function buildRows(events: SentinelEvent[]): Row[] {
  const rows: Row[] = [];
  const open = new Map<string, Row>();
  const legacy = new Map<string, { rows: Row[]; remaining: number; ambiguous: boolean }>();
  const markAmbiguous = (row: Row, correlation: Correlation) => {
    row.correlation = correlation;
    row.status = "unknown";
    row.durationMs = undefined;
  };

  for (const e of events) {
    if (e.correlation?.quality === "ambiguous") {
      for (const candidate of e.correlation.candidateCallIds ?? []) {
        const row = open.get(identity(e, candidate));
        if (row) markAmbiguous(row, e.correlation);
      }
    }
    switch (e.type) {
      case "ToolCallRequest":
      case "Request": {
        const isTool = e.type === "ToolCallRequest";
        const row: Row = {
          key: `req-${e.recordingId ?? e.sessionId}-${e.seq}`,
          kind: isTool ? "tool" : "rpc",
          title: isTool ? e.toolName || "tools/call" : e.method || "request",
          subtitle: isTool ? argSummary(e.payload) : undefined,
          status: e.correlation?.quality === "unmatched" && e.status === "pending" ? "unknown" : e.status,
          risk: e.risk,
          startedAt: e.timestamp,
          request: e,
          correlation: e.correlation,
        };
        rows.push(row);
        if (e.callId) open.set(identity(e, e.callId), row);
        else {
          const key = legacyKey(e);
          if (key) {
            const group = legacy.get(key) ?? { rows: [], remaining: 0, ambiguous: false };
            group.rows.push(row); group.remaining++;
            if (group.rows.length > 1) {
              group.ambiguous = true;
              for (const candidate of group.rows) markAmbiguous(candidate, { method: "none", quality: "ambiguous" });
            }
            legacy.set(key, group);
          }
        }
        break;
      }
      case "ToolCallResponse":
      case "Response": {
        let row: Row | undefined;
        let correlation = e.correlation;
        if (e.callId) {
          const candidate = open.get(identity(e, e.callId));
          if (e.correlation?.quality === "exact" && candidate && (e.correlation.requestSeq === undefined || candidate.request?.seq === e.correlation.requestSeq) && candidate.correlation?.quality !== "ambiguous") {
            row = candidate;
            open.delete(identity(e, e.callId));
          }
          if (!row && correlation?.quality === "exact") correlation = candidate?.correlation?.quality === "ambiguous" ? candidate.correlation : { ...correlation, quality: "unmatched" };
        } else if (!(e.schemaVersion === 2 && e.source)) {
          const key = legacyKey(e, true);
          const group = key ? legacy.get(key) : undefined;
          if (group) {
            if (group.ambiguous) correlation = { method: "none", quality: "ambiguous" };
            else row = group.rows[0];
            group.remaining--;
            if (!group.remaining) legacy.delete(key!);
          }
        }
        if (row) {
          row.response = e;
          row.status = e.status;
          // Old adapters omit both zero and unavailable durations. Their
          // absence cannot establish a measured zero; keep it unknown.
          row.durationMs = e.status === "blocked" || e.status === "rejected" || e.correlation?.synthetic ? undefined : e.timing?.durationMs ?? e.durationMs;
          row.correlation = correlation;
          if (e.risk !== "none") row.risk = e.risk;
        } else {
          rows.push({
            key: `res-${e.recordingId ?? e.sessionId}-${e.seq}`,
            kind: "rpc",
            title: e.toolName || e.method || "response",
            subtitle: correlation?.quality === "ambiguous" ? "Ambiguous response · no request assigned" : "Unmatched response · no request captured",
            status: e.status,
            risk: e.risk,
            startedAt: e.timestamp,
            durationMs: correlation?.quality === "ambiguous" ? undefined : e.timing?.durationMs ?? e.durationMs,
            response: e,
            correlation: correlation ?? { method: "none", quality: "unmatched" },
          });
        }
        break;
      }
      case "ApprovalResolved": {
        // Settles a held call: it either runs now (still "pending" until the
        // response arrives) or is rejected.
        const key = legacyKey(e);
        const row = e.callId ? open.get(identity(e, e.callId)) : key ? legacy.get(key)?.rows[0] : undefined;
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
      case "CaptureDiagnostic":
        rows.push(standalone(e, "notification", "Capture diagnostic", e.reason));
        break;
      case "SessionStarted":
        rows.push(standalone(e, "session", "Session started"));
        break;
      case "SessionCompleted":
        rows.push(standalone(e, "session", "Session completed"));
        break;
      case "BudgetChanged": {
        const p = e.payload as
          | { maxCostUsd?: number; maxTokens?: number; previousMaxCostUsd?: number; previousMaxTokens?: number }
          | undefined;
        rows.push(
          standalone(
            e,
            "session",
            "Budget changed",
            p ? `${describeBudget(p.previousMaxCostUsd ?? 0, p.previousMaxTokens ?? 0)} → ${describeBudget(p.maxCostUsd ?? 0, p.maxTokens ?? 0)}` : e.reason,
          ),
        );
        break;
      }
      default:
        rows.push(standalone(e, "raw", e.type, "Uninterpreted captured event"));
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
    key: `ev-${e.recordingId ?? e.sessionId}-${e.seq}`,
    kind,
    title,
    subtitle,
    status: e.status,
    risk: e.risk,
    startedAt: e.timestamp,
    event: e,
  };
}
