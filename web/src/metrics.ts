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
  /** Median and p95 of available completed tool durations, in ms. */
  p50: number;
  p95: number;
}

/** Metrics of retained tool events, not provider usage or billing. */
export function computeMetrics(rows: Row[]): Metrics {
  const tools = rows.filter((r) => r.kind === "tool");
  const durations = tools
    .map(toolDuration)
    .filter((duration): duration is number => duration !== undefined)
    .sort((a, b) => a - b);
  const at = (q: number) => (durations.length ? durations[Math.max(0, Math.ceil(durations.length * q) - 1)] : 0);

  return {
    calls: tools.length,
    waiting: tools.filter((r) => r.status === "awaiting_approval").length,
    running: tools.filter((r) => r.status === "pending").length,
    refused: tools.filter((r) => r.status === "blocked" || r.status === "rejected").length,
    errors: tools.filter((r) => r.status === "error").length,
    interrupted: tools.filter((r) => r.status === "interrupted").length,
    payloadBytes: tools.reduce((total, row) => total + payloadSize(row.request?.payload) + payloadSize(row.response?.payload), 0),
    latencySamples: durations.length,
    p50: at(0.5),
    p95: at(0.95),
  };
}

/** Shared eligibility for percentiles, rankings and inspector sorting. */
export function toolDuration(row: Row): number | undefined {
  if (row.kind !== "tool" || (row.status !== "ok" && row.status !== "error") || row.request?.decision === "approve") return undefined;
  return row.durationMs !== undefined && Number.isFinite(row.durationMs) && row.durationMs >= 0 ? row.durationMs : undefined;
}

/** This measures the payload JSON available to the UI, not original wire size. */
export function payloadSize(payload: unknown): number {
  return payload === undefined ? 0 : new TextEncoder().encode(JSON.stringify(payload)).length;
}

export const PAYLOAD_NOTE = "UTF-8 size of retained tool request and response JSON, serialized for display. May be truncated or synthesized by adapters; not wire volume, model tokens or context usage.";
