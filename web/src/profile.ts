import type { Row, Session } from "./types";
import { computeMetrics, payloadSize, toolDuration, type Metrics } from "./metrics";

export type CallFilter = "all" | "errors" | "open" | "interrupted";
export type CallSort = "time" | "duration" | "response";

export interface ToolActivity {
  name: string;
  metrics: Metrics;
}

/** Summaries of existing evidence, without inferring loops or context usage. */
export function buildProfile(rows: Row[]) {
  const calls = rows.filter((row) => row.kind === "tool");
  const grouped = new Map<string, Row[]>();
  for (const row of calls) {
    const name = row.request?.toolName ?? row.title;
    const group = grouped.get(name) ?? [];
    group.push(row);
    grouped.set(name, group);
  }
  const tools: ToolActivity[] = [...grouped].map(([name, entries]) => ({ name, metrics: computeMetrics(entries) }));
  tools.sort((a, b) => b.metrics.calls - a.metrics.calls || a.name.localeCompare(b.name));
  return {
    calls,
    tools,
    longest: sortCalls(calls.filter((row) => toolDuration(row) !== undefined), "duration").slice(0, 5),
    largest: sortCalls(calls.filter((row) => responseSize(row) !== undefined), "response").slice(0, 5),
  };
}

/** Bytes of the retained response JSON. Missing response is not zero bytes. */
export function responseSize(row: Row): number | undefined {
  return row.response?.payload === undefined ? undefined : payloadSize(row.response.payload);
}

export function sortCalls(rows: Row[], sort: CallSort): Row[] {
  if (sort === "time") return [...rows];
  const measure = sort === "duration" ? toolDuration : responseSize;
  return [...rows].sort((a, b) => {
    const left = measure(a);
    const right = measure(b);
    if (left === undefined) return right === undefined ? 0 : 1;
    if (right === undefined) return -1;
    return right - left;
  });
}

export function filterCalls(rows: Row[], filter: CallFilter, tool: string, query: string): Row[] {
  const q = query.trim().toLocaleLowerCase();
  return rows.filter((row) => {
    if (tool && (row.request?.toolName ?? row.title) !== tool) return false;
    if (filter === "errors" && row.status !== "error") return false;
    if (filter === "open" && row.status !== "pending") return false;
    if (filter === "interrupted" && row.status !== "interrupted") return false;
    if (!q) return true;
    return `${row.title} ${row.subtitle ?? ""} ${JSON.stringify(row.request?.payload) ?? ""}`.toLocaleLowerCase().includes(q);
  });
}

/** Only the span of recorded timestamps, never total task time or LLM time. */
export function recordedSpan(session: Session | null, rows: Row[]): number | undefined {
  if (!session) return undefined;
  const start = Date.parse(session.startedAt);
  if (!Number.isFinite(start)) return undefined;
  const timestamps = rows.flatMap((row) => [row.startedAt, row.response?.timestamp, row.event?.timestamp]);
  if (session.endedAt) timestamps.push(session.endedAt);
  let last: number | undefined;
  for (const stamp of timestamps) {
    const time = stamp ? Date.parse(stamp) : NaN;
    if (Number.isFinite(time) && time >= start) last = Math.max(last ?? start, time);
  }
  return last === undefined ? undefined : last - start;
}
