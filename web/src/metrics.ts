import type { Row } from "./types";

export interface Metrics {
  calls: number;
  waiting: number;
  running: number;
  /** Blocked by policy or rejected by a human (or a timeout). Nothing ran. */
  refused: number;
  errors: number;
  /** Calls that never reported a result (skipped, declined, cancelled or cut short). Not tool errors. */
  interrupted: number;
  /** UTF-8 size of the retained, reserialized tool payload JSON. */
  payloadBytes: number;
  latencySamples: number;
  /** Median and 95th percentile of real tool latency, in ms. */
  p50: number;
  p95: number;
}

/** Metrics of retained tool events, not provider usage or billing. */
export function computeMetrics(rows: Row[]): Metrics {
  const tools = rows.filter((r) => r.kind === "tool");
  const durations = rows
    // An interrupted call's duration is how long Sentinel waited to close it, not the tool's latency.
    .filter((r) => r.kind === "tool" && r.durationMs !== undefined && Number.isFinite(r.durationMs) && r.durationMs >= 0 && (r.status === "ok" || r.status === "error") && r.request?.decision !== "approve")
    .map((r) => r.durationMs!)
    .sort((a, b) => a - b);
  const at = (q: number) => (durations.length ? durations[Math.max(0, Math.ceil(durations.length * q) - 1)] : 0);

  return {
    calls: tools.length,
    waiting: rows.filter((r) => r.status === "awaiting_approval").length,
    running: rows.filter((r) => r.status === "pending").length,
    refused: rows.filter((r) => r.status === "blocked" || r.status === "rejected").length,
    errors: rows.filter((r) => r.status === "error").length,
    interrupted: rows.filter((r) => r.status === "interrupted").length,
    payloadBytes: tools.reduce((total, row) => total + payloadSize(row.request?.payload) + payloadSize(row.response?.payload), 0),
    latencySamples: durations.length,
    p50: at(0.5),
    p95: at(0.95),
  };
}

/** This measures the payload JSON available to the UI, not original wire size. */
export function payloadSize(payload: unknown): number {
  return payload === undefined ? 0 : new TextEncoder().encode(JSON.stringify(payload)).length;
}

export const PAYLOAD_NOTE = "UTF-8 size of retained tool request and response JSON, serialized for display. May be truncated or synthesized by adapters; not wire volume, model tokens or context usage.";
