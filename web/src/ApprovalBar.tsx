import { useEffect, useState } from "react";
import { Check, MessageSquarePlus, X } from "lucide-react";
import type { Row } from "./types";
import { RiskChip } from "./Timeline";
import { formatClock } from "./format";

interface Props {
  pendingRow: Row | null;
  /** How many MORE calls are waiting behind this one. */
  more: number;
  busy: boolean;
  error: string | null;
  /** Seconds a held call waits before the proxy refuses it. */
  timeoutSec: number;
  onApprove: (row: Row, feedback?: string) => void;
  onBlock: (row: Row, feedback?: string) => void;
}

/** Seconds since a timestamp, re-read every second. */
function useElapsed(since: string | undefined): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!since) return;
    setNow(Date.now());
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [since]);
  return since ? Math.max(0, (now - Date.parse(since)) / 1000) : 0;
}

export function ApprovalBar({ pendingRow, more, busy, error, timeoutSec, onApprove, onBlock }: Props) {
  const [feedback, setFeedback] = useState("");
  const [showNote, setShowNote] = useState(false);
  const key = pendingRow?.key;
  const elapsed = useElapsed(pendingRow?.request?.timestamp);

  // A new call to decide on starts with a clean note.
  useEffect(() => {
    setFeedback("");
    setShowNote(false);
  }, [key]);

  useEffect(() => {
    if (!pendingRow || busy) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented) return; // an overlay already handled this key
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
        e.preventDefault();
        onApprove(pendingRow, feedback.trim() || undefined);
      } else if (e.key === "Escape" && !showNote) {
        e.preventDefault();
        onBlock(pendingRow, feedback.trim() || undefined);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [pendingRow, busy, feedback, showNote, onApprove, onBlock]);

  if (!pendingRow) return null;
  const why = pendingRow.request?.reason;
  const rule = pendingRow.request?.rule;
  const total = timeoutSec > 0 ? timeoutSec : 120;
  const left = Math.max(0, total - elapsed);
  const turn = Math.min(1, elapsed / total) * 360;

  return (
    <section
      aria-label="A call is waiting for your decision"
      className="light-scope shrink-0 text-ink border-t-2 border-ink bg-yellow px-6 py-3"
    >
      <div className="mx-auto grid max-w-5xl grid-cols-[auto_minmax(0,1fr)] items-center gap-x-5 gap-y-3 lg:grid-cols-[auto_minmax(0,1fr)_auto]">
        <StationClock turn={turn} left={left} />

        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-x-2.5 gap-y-0.5">
            <h2 role="alert" className="min-w-0 max-w-full truncate text-lg font-bold text-ink">
              {pendingRow.title}
            </h2>
            <RiskChip risk={pendingRow.risk} />
            {rule && <span className="hidden truncate font-mono text-xs text-ink sm:inline">{rule}</span>}
            {more > 0 && <span className="shrink-0 text-xs font-medium text-ink">+{more} waiting</span>}
          </div>
          {pendingRow.subtitle && (
            <p className="mt-0.5 truncate font-mono text-[13px] text-ink">{pendingRow.subtitle}</p>
          )}
          {why && <p className="mt-0.5 line-clamp-2 break-words text-[13px] text-ink-2">{why}</p>}
          {showNote && (
            <input
              type="text"
              autoFocus
              value={feedback}
              onChange={(e) => setFeedback(e.target.value)}
              aria-label="Note for the agent"
              placeholder="Note the agent will read, e.g. “dry-run first”"
              className="mt-2 w-full max-w-lg rounded-sm border border-ink bg-sheet px-2.5 py-1.5 text-sm text-ink placeholder:text-ink-3"
            />
          )}
          {error && (
            <p role="alert" className="mt-2 rounded-sm bg-signal px-2.5 py-1.5 text-sm font-medium text-white">
              {error}
            </p>
          )}
        </div>

        <div className="col-span-2 flex items-center justify-end gap-2 lg:col-span-1">
          <button
            onClick={() => setShowNote((v) => !v)}
            aria-pressed={showNote}
            className="flex items-center gap-1.5 rounded-sm px-2 py-2 text-sm font-medium text-ink hover:bg-yellow-soft"
          >
            <MessageSquarePlus className="size-4" />
            Note
          </button>
          <button
            disabled={busy}
            onClick={() => onBlock(pendingRow, feedback.trim() || undefined)}
            className="flex items-center gap-1.5 rounded-sm border-2 border-ink bg-sheet px-3.5 py-2 text-sm font-semibold text-ink hover:bg-white disabled:opacity-50"
          >
            <X className="size-4" />
            Reject
            <kbd className="font-mono text-[11px] font-medium text-ink-2">Esc</kbd>
          </button>
          <button
            disabled={busy}
            onClick={() => onApprove(pendingRow, feedback.trim() || undefined)}
            className="flex items-center gap-1.5 rounded-sm border-2 border-ink bg-ink px-3.5 py-2 text-sm font-semibold text-white hover:bg-ink-2 disabled:opacity-50"
          >
            <Check className="size-4" />
            Approve
            <kbd className="font-mono text-[11px] font-medium text-yellow">⌘↵</kbd>
          </button>
        </div>
      </div>
    </section>
  );
}

/**
 * A station clock whose red second hand makes one turn over the approval
 * window. The remaining time is also written out: the drawing is not the only
 * carrier of the information.
 */
function StationClock({ turn, left }: { turn: number; left: number }) {
  const ticks = Array.from({ length: 60 }, (_, i) => i);
  return (
    <div className="flex items-center gap-3">
      <svg viewBox="0 0 64 64" className="size-16 shrink-0" role="img" aria-label={`${formatClock(left)} left to decide`}>
        <circle cx="32" cy="32" r="30" fill="#fbfaf5" stroke="#15140f" strokeWidth="2" />
        {ticks.map((i) => {
          const major = i % 5 === 0;
          return (
            <line
              key={i}
              x1="32"
              y1="5"
              x2="32"
              y2={major ? 12 : 8}
              stroke="#15140f"
              strokeWidth={major ? 2.4 : 1}
              transform={`rotate(${i * 6} 32 32)`}
            />
          );
        })}
        <g className="clock-hand" style={{ transform: `rotate(${turn}deg)` }}>
          <line x1="32" y1="38" x2="32" y2="10" stroke="#d5001c" strokeWidth="2" strokeLinecap="round" />
          <circle cx="32" cy="14" r="3.6" fill="#d5001c" />
        </g>
        <circle cx="32" cy="32" r="2.2" fill="#15140f" />
      </svg>
      <div className="leading-tight">
        <div className="fig text-2xl font-bold text-ink">{formatClock(left)}</div>
        <div className="text-xs text-ink">to decide</div>
      </div>
    </div>
  );
}
