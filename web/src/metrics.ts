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
  tokens: number;
  /** Estimated USD. See `computeMetrics`. */
  cost: number;
  /** Median and 95th percentile of real tool latency, in ms. */
  p50: number;
  p95: number;
}

/**
 * The numbers shown in the status strip and written in the exported report.
 *
 * Tokens and cost are an ESTIMATE from the real size of what crossed the proxy
 * (~4 characters per token). The proxy cannot see the LLM API traffic, so this
 * only covers tool traffic and ignores the conversation context:
 *  - call arguments were written by the model  -> output tokens
 *  - tool results are fed back to the model    -> input tokens
 */
export function computeMetrics(rows: Row[]): Metrics {
  const tools = rows.filter((r) => r.kind === "tool");
  const durations = rows
    // An interrupted call's duration is how long Sentinel waited to close it, not the tool's latency.
    .filter((r) => r.durationMs !== undefined && r.durationMs > 0 && r.status !== "rejected" && r.status !== "interrupted")
    .map((r) => r.durationMs!)
    .sort((a, b) => a - b);
  const at = (q: number) => (durations.length ? durations[Math.min(durations.length - 1, Math.floor(durations.length * q))] : 0);

  const approx = (payload: unknown) => (payload === undefined ? 0 : Math.ceil(JSON.stringify(payload).length / 4));
  let tin = 0;
  let tout = 0;
  for (const r of tools) {
    tout += approx(r.request?.payload);
    tin += approx(r.response?.payload);
  }

  return {
    calls: tools.length,
    waiting: rows.filter((r) => r.status === "awaiting_approval").length,
    running: rows.filter((r) => r.status === "pending").length,
    refused: rows.filter((r) => r.status === "blocked" || r.status === "rejected").length,
    errors: rows.filter((r) => r.status === "error").length,
    interrupted: rows.filter((r) => r.status === "interrupted").length,
    tokens: tin + tout,
    cost: (tin / 1_000_000) * 3 + (tout / 1_000_000) * 15,
    p50: at(0.5),
    p95: at(0.95),
  };
}

/** The pricing assumption behind the cost estimate, for tooltips and the report. */
export const COST_NOTE =
  "Estimated from the size of tool calls and results (~4 chars/token, $3/M in, $15/M out). Does not include the LLM conversation context.";
