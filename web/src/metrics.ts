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

// Measurements are supplied by the Go profiler for live, replay and export.
export const EMPTY_METRICS: Metrics = { calls: 0, waiting: 0, running: 0, refused: 0, errors: 0, interrupted: 0, payloadBytes: 0, latencySamples: 0, p50: 0, p95: 0 };
export function toolDuration(row: Row): number | undefined { return row.toolDurationMs; }

export const PAYLOAD_NOTE = "Stored request and matched-response JSON bytes for the selected recording scope; legacy logs use compact retained JSON. Unassigned responses are excluded. Observed source values use separate units. Not model tokens or context usage.";
