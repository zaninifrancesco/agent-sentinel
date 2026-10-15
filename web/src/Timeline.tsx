import { useEffect, useRef, type KeyboardEvent } from "react";
import { Ban, Check, CircleDashed, CircleSlash, Loader, OctagonPause, TriangleAlert } from "lucide-react";
import type { Risk, Row, Status } from "./types";
import { formatBytes, formatDuration, formatTime } from "./format";
import { toolColor } from "./trace";
import { responseSize } from "./profile";

/** One consistent glyph set, 16px, 2px stroke. Colour only where it means something. */
export function StatusIcon({ status }: { status: Status }) {
  const cls = "size-4 shrink-0";
  switch (status) {
    case "ok":
      return <Check aria-label="ok" className={`${cls} text-ok`} strokeWidth={2.5} />;
    case "pending":
      return <Loader aria-label="running" className={`${cls} text-ink-2`} />;
    case "awaiting_approval":
      return <OctagonPause aria-label="historical hold" className={`${cls} text-ink`} />;
    case "blocked":
      return <Ban aria-label="blocked" className={`${cls} text-signal`} />;
    case "rejected":
      return <CircleSlash aria-label="rejected" className={`${cls} text-signal`} />;
    case "interrupted":
      return <CircleDashed aria-label="interrupted" className={`${cls} text-ink-2`} />;
    case "unknown":
      return <CircleDashed aria-label="unresolved pairing" className={`${cls} text-warn`} />;
    default:
      return <TriangleAlert aria-label="error" className={`${cls} text-warn`} />;
  }
}

/** Risk is written as a service mark: critical is filled, high is outlined. */
export function RiskChip({ risk }: { risk: Risk }) {
  if (risk === "none") return null;
  const style: Record<Exclude<Risk, "none">, string> = {
    critical: "bg-signal text-white",
    high: "border border-signal text-signal",
    medium: "border border-ink-3 text-ink-2",
    low: "border border-rule-strong text-ink-3",
  };
  return (
    <span className={`rounded-sm px-1.5 text-[11px] font-semibold uppercase leading-4 tracking-wide ${style[risk]}`}>
      {risk}
    </span>
  );
}

interface Props {
  rows: Row[];
  selectedKey: string | null;
  onSelect: (key: string) => void;
  follow: boolean;
  showResponseSize?: boolean;
  emptyTitle?: string;
  emptyDescription?: string;
}

export function Timeline({ rows, selectedKey, onSelect, follow, showResponseSize = false, emptyTitle = "No captured events", emptyDescription = "Connect an agent or MCP server to start recording." }: Props) {
  const scroller = useRef<HTMLDivElement>(null);
  const stick = useRef(true);

  const cols = showResponseSize
    ? "grid-cols-[56px_20px_minmax(0,1fr)_56px] sm:grid-cols-[56px_20px_minmax(0,1fr)_56px_72px]"
    : "grid-cols-[56px_20px_minmax(0,1fr)_56px]";

  // Navigation belongs to the focused list, never to unrelated app controls.
  const navigate = (event: KeyboardEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement;
    if (target.closest("input, textarea, select, [contenteditable=true], [role=dialog]")) return;
    const direction = event.key === "j" || event.key === "ArrowDown" ? 1 : event.key === "k" || event.key === "ArrowUp" ? -1 : 0;
    if (!direction || !rows.length) return;
    event.preventDefault();
    const current = rows.findIndex((row) => row.key === selectedKey);
    const next = Math.max(0, Math.min(rows.length - 1, current + direction));
    onSelect(rows[next].key);
    scroller.current?.querySelectorAll<HTMLButtonElement>("button[data-recorded-row]")[next]?.focus();
  };

  // Keep the newest event in view while following a live session.
  useEffect(() => {
    const el = scroller.current;
    if (el && follow && stick.current) el.scrollTop = el.scrollHeight;
  }, [rows, follow]);

  // Keep the selected row in view when moving with the keyboard.
  useEffect(() => {
    scroller.current
      ?.querySelector<HTMLElement>('[aria-current="true"]')
      ?.scrollIntoView({ block: "nearest" });
  }, [selectedKey]);

  return (
    <div
      ref={scroller}
      tabIndex={0}
      onKeyDown={navigate}
      role="region"
      aria-label="Captured event list"
      onScroll={(e) => {
        const el = e.currentTarget;
        stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
      }}
      className="min-h-0 flex-1 overflow-y-auto bg-sheet"
    >
      <div
        className={`sticky top-0 z-10 grid ${cols} items-center gap-x-3 border-b border-rule-strong bg-paper px-4 py-2.5 text-[10px] font-medium text-ink-2`}
      >
        <span>Time</span>
        <span aria-hidden />
        <span>{showResponseSize ? "Tool / arguments" : "Event"}</span>
        <span className="text-right">Duration</span>
        {showResponseSize && <span className="hidden text-right sm:block" title="Size of retained response JSON, not original output or context usage">Response</span>}
      </div>

      {rows.length === 0 ? (
        <div className="px-4 py-10 text-sm text-ink-2">
          <p className="font-semibold text-ink">{emptyTitle}</p>
          <p className="mt-1 max-w-sm text-ink-2">{emptyDescription}</p>
        </div>
      ) : (
        <ul>
          {rows.map((row) => {
            const isSelected = selectedKey === row.key;
            const bg = isSelected ? "bg-yellow-soft" : "hover:bg-paper";
            const bytes = responseSize(row);
            const strong = row.kind === "tool";
            const refused = row.status === "blocked" || row.status === "rejected";
            return (
              <li key={row.key} className="border-b border-rule">
                <button
                  data-recorded-row
                  onClick={() => onSelect(row.key)}
                  aria-current={isSelected ? "true" : undefined}
                  className={`timeline-row grid w-full ${cols} items-center gap-x-3 px-4 py-2.5 text-left ${bg}`}
                >
                  <span className="fig text-[11px] text-ink-2">{formatTime(row.startedAt)}</span>
                  <StatusIcon status={row.status} />
                  <span className="min-w-0">
                    <span
                      className={`flex items-center gap-2 truncate text-xs ${
                        strong ? "font-semibold text-ink" : row.kind === "session" ? "text-ink-3" : "text-ink-2"
                      } ${refused ? "line-through decoration-signal decoration-2" : ""}`}
                    >
                      {strong && <i aria-hidden className="series-dot" style={{ background: toolColor(row.request?.toolName ?? row.title) }} />}{row.title}
                    </span>
                    {row.subtitle && (
                      <span className="mt-1 block truncate font-mono text-[10px] text-ink-2">{row.subtitle}</span>
                    )}
                  </span>
                  <span className="fig text-right text-[11px] text-ink-2">
                    {row.durationMs !== undefined ? formatDuration(row.durationMs) : "–"}
                  </span>
                  {showResponseSize && <span className="fig hidden text-right text-[10px] text-ink-2 sm:block">{bytes === undefined ? "–" : formatBytes(bytes)}</span>}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
