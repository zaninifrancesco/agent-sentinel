export function formatTime(iso: string): string {
  const d = new Date(iso);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

/** Milliseconds as a timetable would write them: 55 ms, 1.2 s, 2:00. */
export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
  const total = Math.round(ms / 1000);
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, "0")}`;
}

/** Remaining seconds as m:ss. */
export function formatClock(seconds: number): string {
  const s = Math.max(0, Math.ceil(seconds));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}

/** 100000 -> "100k", 1500000 -> "1.5M". */
export function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${+(n / 1_000_000).toFixed(2)}M`;
  if (n >= 1000) return `${+(n / 1000).toFixed(1)}k`;
  return String(n);
}

/** What the budget modal accepts for a cost: "5", "2.50", "$5". Empty = no limit (0). */
export function parseUsd(text: string): number | null {
  const t = text.trim().replace(/^\$/, "").trim();
  if (t === "") return 0;
  if (!/^\d+(\.\d{1,2})?$/.test(t)) return null;
  const v = Number(t);
  return v <= 1_000_000 ? v : null;
}

/** Tokens as typed: "100000", "100k", "1.5M". Empty = no limit (0). */
export function parseTokens(text: string): number | null {
  const t = text.trim().replace(/[,_ ]/g, "");
  if (t === "") return 0;
  const m = /^(\d+(?:\.\d+)?)([kKmM])?$/.exec(t);
  if (!m) return null;
  const v = Math.round(Number(m[1]) * (m[2] ? (m[2].toLowerCase() === "k" ? 1e3 : 1e6) : 1));
  return Number.isSafeInteger(v) && v <= 1e12 ? v : null;
}

/** "$2.00, 100k tokens" - or what is missing, in words. */
export function describeBudget(costUsd: number, tokens: number): string {
  return `${costUsd > 0 ? `$${costUsd.toFixed(2)}` : "no cost limit"}, ${tokens > 0 ? `${formatTokens(tokens)} tokens` : "no token limit"}`;
}
