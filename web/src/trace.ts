import type { Row } from "./types";

/** Capture intervals are separate from adapter-reported execution durations. */
export function capturedInterval(row: Row): { start: number; end?: number } | undefined {
  if (!row.request) return undefined;
  const start = Date.parse(row.startedAt);
  if (!Number.isFinite(start)) return undefined;
  const end = row.response && !row.response.correlation?.synthetic ? Date.parse(row.response.timestamp) : NaN;
  return { start, end: Number.isFinite(end) && end >= start ? end : undefined };
}

export function toolColor(name: string): string {
  const known: Record<string, number> = { shell: 1, read_file: 0, edit_file: 2, Grep: 3 };
  if (name in known) return `var(--series-${known[name]})`;
  let hash = 0;
  for (const char of name) hash = (hash * 31 + char.charCodeAt(0)) | 0;
  return `var(--series-${Math.abs(hash) % 5})`;
}

export function buildTrace(rows: Row[], limit = 120) {
  const calls = rows.filter((row) => row.kind === "tool");
  const recent = calls.slice(-limit);
  const timed = recent.flatMap((row) => {
    const interval = capturedInterval(row);
    return interval ? [{ row, ...interval }] : [];
  }).sort((a, b) => a.start - b.start);
  const first = timed.length ? Math.min(...timed.map((entry) => entry.start)) : 0;
  const last = timed.length ? Math.max(...timed.map((entry) => entry.end ?? entry.start)) : first;
  const counts = new Map<string, number>();
  for (const { row } of timed) {
    const tool = row.request?.toolName ?? row.title;
    counts.set(tool, (counts.get(tool) ?? 0) + 1);
  }
  const names = [...counts].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([name]) => name);
  const named = names.slice(0, names.length > 6 ? 5 : 6);
  const lanes = named.map((name) => ({ name, entries: timed.filter(({ row }) => (row.request?.toolName ?? row.title) === name) }));
  if (names.length > 6) lanes.push({ name: "Other tools", entries: timed.filter(({ row }) => !named.includes(row.request?.toolName ?? row.title)) });
  return { first, last, span: last - first, timed, lanes, omitted: calls.length - recent.length, invalid: recent.length - timed.length };
}
