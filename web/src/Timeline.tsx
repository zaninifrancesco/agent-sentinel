import { useEffect, useRef } from "react";
import { CheckCircle2, CircleDashed, Flag, Radio, Terminal, Wrench, XCircle, Zap, ShieldAlert } from "lucide-react";
import type { Risk, Row, Status } from "./types";

export function formatTime(iso: string): string {
  const d = new Date(iso);
  const p = (n: number, w = 2) => String(n).padStart(w, "0");
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}.${p(d.getMilliseconds(), 3)}`;
}

const RISK_STYLE: Record<Risk, string> = {
  none: "",
  low: "border-sky-500/40 text-sky-300 bg-sky-950/20",
  medium: "border-amber-500/40 text-amber-300 bg-amber-950/20",
  high: "border-orange-500/50 text-orange-300 bg-orange-950/30",
  critical: "border-red-500/60 text-red-300 bg-red-950/40 animate-pulse",
};

export function StatusIcon({ status }: { status: Status }) {
  switch (status) {
    case "ok":
      return <CheckCircle2 className="size-4 text-emerald-400 shrink-0" />;
    case "pending":
      return <CircleDashed className="size-4 animate-spin text-amber-400 shrink-0" />;
    case "blocked":
    case "rejected":
      return <ShieldAlert className="size-4 text-rose-400 shrink-0" />;
    default:
      return <XCircle className="size-4 text-rose-400 shrink-0" />;
  }
}

function KindIcon({ kind }: { kind: Row["kind"] }) {
  const cls = "size-4 text-ink-500 shrink-0";
  switch (kind) {
    case "tool":
      return <Wrench className="size-4 text-accent shrink-0" />;
    case "notification":
      return <Radio className={cls} />;
    case "raw":
      return <Terminal className={cls} />;
    case "session":
      return <Flag className={cls} />;
    default:
      return <Zap className={cls} />;
  }
}

interface Props {
  rows: Row[];
  selectedKey: string | null;
  onSelect: (key: string) => void;
  follow: boolean;
}

export function Timeline({ rows, selectedKey, onSelect, follow }: Props) {
  const scroller = useRef<HTMLDivElement>(null);
  const stick = useRef(true);

  // Keyboard navigation: j/k or ArrowUp/ArrowDown to navigate rows
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if typing in an input
      if (document.activeElement?.tagName === "INPUT" || document.activeElement?.tagName === "TEXTAREA") {
        return;
      }

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

  // Keep newest event in view if follow mode is active
  useEffect(() => {
    const el = scroller.current;
    if (el && follow && stick.current) el.scrollTop = el.scrollHeight;
  }, [rows, follow]);

  return (
    <div
      ref={scroller}
      onScroll={(e) => {
        const el = e.currentTarget;
        stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
      }}
      className="overflow-y-auto border-r border-ink-800 bg-ink-900/70"
    >
      {rows.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-12 text-center text-ink-500">
          <Terminal className="size-8 text-ink-700 mb-2" />
          <p className="text-xs">Waiting for agent activity...</p>
        </div>
      ) : (
        <ul className="divide-y divide-ink-800/60">
          {rows.map((row) => {
            const isSelected = selectedKey === row.key;
            return (
              <li key={row.key}>
                <button
                  onClick={() => onSelect(row.key)}
                  className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-all ${
                    isSelected
                      ? "bg-accent/10 border-l-2 border-accent pl-[14px]"
                      : "hover:bg-ink-850/80"
                  }`}
                >
                  <StatusIcon status={row.status} />
                  <KindIcon kind={row.kind} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className={`truncate font-mono text-xs font-medium ${isSelected ? "text-accent" : "text-ink-100"}`}>
                        {row.title}
                      </span>
                      {row.risk !== "none" && (
                        <span className={`rounded border px-1.5 py-0.2 text-[9px] font-semibold uppercase tracking-wider ${RISK_STYLE[row.risk]}`}>
                          {row.risk}
                        </span>
                      )}
                    </div>
                    {row.subtitle && (
                      <div className="truncate text-[11px] text-ink-400 mt-0.5">{row.subtitle}</div>
                    )}
                  </div>

                  <div className="flex flex-col items-end gap-1 shrink-0 font-mono text-[10px]">
                    {row.durationMs !== undefined && (
                      <span className="rounded bg-ink-800 px-1.5 py-0.5 text-ink-300">
                        {row.durationMs}ms
                      </span>
                    )}
                    <span className="text-ink-500">{formatTime(row.startedAt)}</span>
                  </div>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
