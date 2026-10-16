import type { Row } from "./types";
/** The Go projection supplies capture bounds, independently of tool duration. */
export function capturedInterval(row: Row): { start: number; end?: number } | undefined {
  return row.span ? { start: row.span.startMs, end: row.span.endMs } : undefined;
}

/** Pack captured intervals chronologically, reusing the first available track.
 * Unknown ends are observations at their start, never invented running spans.
 * Coincident points get separate tracks so each remains inspectable.
 */
export function timelineTracks(rows: Row[]): { tracks: Row[][]; unpositioned: Row[] } {
  const unpositioned: Row[] = [];
  const positioned = rows.filter((row) => {
    if (row.kind !== "tool") return false;
    if (!row.span || !Number.isFinite(row.span.startMs)) { unpositioned.push(row); return false; }
    return true;
  }).sort((a, b) => a.span!.startMs - b.span!.startMs);
  const tracks: Row[][] = [];
  const occupied: { end: number; point: boolean }[] = [];
  for (const row of positioned) {
    const { startMs, endMs } = row.span!;
    const end = endMs !== undefined && Number.isFinite(endMs) && endMs >= startMs ? endMs : startMs;
    const point = end === startMs;
    let index = occupied.findIndex((last) => last.end < startMs || (last.end === startMs && !last.point));
    if (index < 0) { index = tracks.length; tracks.push([]); }
    tracks[index].push(row);
    occupied[index] = { end, point };
  }
  return { tracks, unpositioned };
}

export function toolColor(name: string): string {
  const known: Record<string, number> = { shell: 1, read_file: 0, edit_file: 2, Grep: 3 };
  if (name in known) return `var(--series-${known[name]})`;
  let hash = 0;
  for (const char of name) hash = (hash * 31 + char.charCodeAt(0)) | 0;
  return `var(--series-${Math.abs(hash) % 5})`;
}
