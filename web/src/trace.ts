import type { Row } from "./types";
/** The Go projection supplies capture bounds, independently of tool duration. */
export function capturedInterval(row: Row): { start: number; end?: number } | undefined {
  return row.span ? { start: row.span.startMs, end: row.span.endMs } : undefined;
}

export function toolColor(name: string): string {
  const known: Record<string, number> = { shell: 1, read_file: 0, edit_file: 2, Grep: 3 };
  if (name in known) return `var(--series-${known[name]})`;
  let hash = 0;
  for (const char of name) hash = (hash * 31 + char.charCodeAt(0)) | 0;
  return `var(--series-${Math.abs(hash) % 5})`;
}
