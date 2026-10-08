import { useEffect, useRef } from "react";
import { Ban, Check, CircleSlash, Loader, OctagonPause, TriangleAlert } from "lucide-react";
import type { Risk, Row, Status } from "./types";
import { formatDuration, formatTime } from "./format";

/** One consistent glyph set, 16px, 2px stroke. Colour only where it means something. */
export function StatusIcon({ status }: { status: Status }) {
  const cls = "size-4 shrink-0";
  switch (status) {
    case "ok":
      return <Check aria-label="ok" className={`${cls} text-ok`} strokeWidth={2.5} />;
    case "pending":
      return <Loader aria-label="running" className={`${cls} text-ink-2`} />;
    case "awaiting_approval":
      return <OctagonPause aria-label="waiting for you" className={`${cls} text-ink`} />;
    case "blocked":
      return <Ban aria-label="blocked" className={`${cls} text-signal`} />;
    case "rejected":
      return <CircleSlash aria-label="rejected" className={`${cls} text-signal`} />;
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
}

const COLS =
  "grid-cols-[64px_20px_minmax(0,1fr)_56px] sm:grid-cols-[72px_20px_minmax(0,1fr)_200px_64px]";

export function Timeline({ rows, selectedKey, onSelect, follow }: Props) {
  const scroller = useRef<HTMLDivElement>(null);
  const stick = useRef(true);

  // While any call waits for the human, everything else steps back.
  const holding = rows.some((r) => r.status === "awaiting_approval");

  // Keyboard navigation: j/k or ArrowUp/ArrowDown to navigate rows
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const tag = document.activeElement?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") return;

      if (e.key === "j" || e.key === "ArrowDown") {
        e.preventDefault();
        const currentIdx = rows.findIndex((r) => r.key === selectedKey);
        const nextIdx = Math.min(rows.length - 1, currentIdx + 1);
        if (rows[nextIdx]) onSelect(rows[nextIdx].key);
      } else if (e.key === "k" || e.key === "ArrowUp") {
        e.preventDefault();
        const currentIdx = rows.findIndex((r) => r.key === selectedKey);
        const prevIdx = Math.max(0, currentIdx - 1);
        if (rows[prevIdx]) onSelect(rows[prevIdx].key);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [rows, selectedKey, onSelect]);

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
      onScroll={(e) => {
        const el = e.currentTarget;
        stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
      }}
      className="min-h-0 flex-1 overflow-y-auto bg-sheet"
    >
      <div
        className={`sticky top-0 z-10 grid ${COLS} items-center gap-x-3 border-b border-rule-strong bg-paper px-4 py-1.5 text-xs font-medium text-ink-2`}
      >
        <span>Time</span>
        <span aria-hidden />
        <span>Event</span>
        <span className="hidden text-right sm:block">Policy</span>
        <span className="text-right">Took</span>
      </div>

      {rows.length === 0 ? (
        <div className="px-4 py-10 text-sm text-ink-2">
          <p className="font-semibold text-ink">No calls yet</p>
          <p className="mt-1 max-w-sm text-ink-2">
            Start your agent through <code className="font-mono text-xs">sentinel mcp -- &lt;server&gt;</code>. Every
            call it makes appears here as a row.
          </p>
        </div>
      ) : (
        <ul>
          {rows.map((row) => {
            const isSelected = selectedKey === row.key;
            const isHeld = row.status === "awaiting_approval";
            const bg = isHeld ? "bg-yellow" : isSelected ? "bg-yellow-soft" : "hover:bg-paper";
            const recede = holding && !isHeld && !isSelected ? "row-recede" : "";
            const strong = row.kind === "tool";
            const refused = row.status === "blocked" || row.status === "rejected";
            return (
              <li key={row.key} className="border-b border-rule">
                <button
                  onClick={() => onSelect(row.key)}
                  aria-current={isSelected ? "true" : undefined}
                  className={`grid w-full ${COLS} items-center gap-x-3 px-4 py-2 text-left ${bg} ${recede} ${
                    isHeld && isSelected ? "outline-2 -outline-offset-2 outline-ink" : ""
                  }`}
                >
                  <span className="fig text-[13px] text-ink-2">{formatTime(row.startedAt)}</span>
                  <StatusIcon status={row.status} />
                  <span className="min-w-0">
                    <span
                      className={`block truncate text-sm ${
                        strong ? "font-semibold text-ink" : row.kind === "session" ? "text-ink-3" : "text-ink-2"
                      } ${refused ? "line-through decoration-signal decoration-2" : ""}`}
                    >
                      {row.title}
                    </span>
                    {row.subtitle && (
                      <span className="block truncate font-mono text-xs text-ink-2">{row.subtitle}</span>
                    )}
                  </span>
                  <span className="hidden min-w-0 items-center justify-end gap-2 sm:flex">
                    {row.request?.rule && (
                      <span className="min-w-0 truncate font-mono text-xs text-ink-2">{row.request.rule}</span>
                    )}
                    <RiskChip risk={row.risk} />
                  </span>
                  <span className="fig text-right text-[13px] text-ink-2">
                    {row.durationMs !== undefined ? formatDuration(row.durationMs) : ""}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
