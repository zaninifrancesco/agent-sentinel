import { useEffect, useRef } from "react";
import { CheckCircle2, CircleDashed, Flag, Radio, Terminal, Wrench, XCircle, Zap } from "lucide-react";
import type { Risk, Row, Status } from "./types";

export function formatTime(iso: string): string {
  const d = new Date(iso);
  const p = (n: number, w = 2) => String(n).padStart(w, "0");
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}.${p(d.getMilliseconds(), 3)}`;
}

const RISK_STYLE: Record<Risk, string> = {
  none: "",
  low: "border-sky-500/40 text-sky-300",
  medium: "border-amber-500/40 text-amber-300",
  high: "border-orange-500/50 text-orange-300",
  critical: "border-red-500/60 text-red-300",
};

export function StatusIcon({ status }: { status: Status }) {
  switch (status) {
    case "ok":
      return <CheckCircle2 className="size-4 text-emerald-400" />;
    case "pending":
      return <CircleDashed className="size-4 animate-spin text-amber-400" />;
    default:
      return <XCircle className="size-4 text-red-400" />;
  }
}

function KindIcon({ kind }: { kind: Row["kind"] }) {
  const cls = "size-4 text-ink-500";
  switch (kind) {
    case "tool":
      return <Wrench className="size-4 text-accent" />;
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

  // Keep the newest event in view, unless the user scrolled up to read.
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
      className="overflow-y-auto border-r border-ink-700 bg-ink-900"
    >
      {rows.length === 0 && <p className="p-6 text-sm text-ink-500">Waiting for agent activity…</p>}
      <ul>
        {rows.map((row) => (
          <li key={row.key}>
            <button
              onClick={() => onSelect(row.key)}
              className={
                "flex w-full items-center gap-3 border-b border-ink-800 px-4 py-2.5 text-left transition-colors " +
                (selectedKey === row.key ? "bg-ink-700/60" : "hover:bg-ink-850")
              }
            >
              <StatusIcon status={row.status} />
              <KindIcon kind={row.kind} />
              <div className="min-w-0 flex-1">
                <div className="truncate font-mono text-sm text-ink-100">{row.title}</div>
                {row.subtitle && <div className="truncate text-xs text-ink-500">{row.subtitle}</div>}
              </div>
              {row.risk !== "none" && (
                <span className={`rounded border px-1.5 py-0.5 text-[10px] uppercase ${RISK_STYLE[row.risk]}`}>
                  {row.risk}
                </span>
              )}
              {row.durationMs !== undefined && (
                <span className="font-mono text-xs text-ink-300">{row.durationMs} ms</span>
              )}
              <span className="font-mono text-[11px] text-ink-500">{formatTime(row.startedAt)}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
