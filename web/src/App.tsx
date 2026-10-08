import { useMemo, useState } from "react";
import { Activity, AlertTriangle, Clock, ShieldCheck, Wrench } from "lucide-react";
import { useSentinel, type Connection } from "./useSentinel";
import { Timeline } from "./Timeline";
import { Detail } from "./Detail";
import type { Row } from "./types";

type Filter = "all" | "tools" | "errors" | "raw";

const FILTERS: { id: Filter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "tools", label: "Tool calls" },
  { id: "errors", label: "Errors" },
  { id: "raw", label: "Raw output" },
];

function matches(row: Row, f: Filter): boolean {
  switch (f) {
    case "tools":
      return row.kind === "tool";
    case "errors":
      return row.status === "error" || row.status === "blocked" || row.status === "rejected";
    case "raw":
      return row.kind === "raw";
    default:
      return true;
  }
}

export default function App() {
  const { session, rows, connection } = useSentinel();
  const [filter, setFilter] = useState<Filter>("all");
  const [selectedKey, setSelectedKey] = useState<string | null>(null);

  const visible = useMemo(() => rows.filter((r) => matches(r, filter)), [rows, filter]);
  const selected = rows.find((r) => r.key === selectedKey) ?? null;

  const stats = useMemo(() => {
    const tools = rows.filter((r) => r.kind === "tool");
    const timed = tools.filter((r) => r.durationMs !== undefined);
    const avg = timed.length ? timed.reduce((s, r) => s + (r.durationMs ?? 0), 0) / timed.length : 0;
    return {
      tools: tools.length,
      errors: rows.filter((r) => r.status === "error" || r.status === "blocked").length,
      pending: rows.filter((r) => r.status === "pending").length,
      avg: Math.round(avg),
    };
  }, [rows]);

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-6 border-b border-ink-700 bg-ink-900 px-5 py-3">
        <div className="flex items-center gap-2">
          <ShieldCheck className="size-5 text-accent" />
          <span className="font-semibold tracking-tight">Agent Sentinel</span>
        </div>
        <ConnectionBadge state={connection} />
        {session && (
          <div className="min-w-0 truncate font-mono text-xs text-ink-300">
            session <span className="text-ink-100">{session.id}</span>
            {session.command && session.command.length > 0 && (
              <>
                {" · "}
                <span title={session.command.join(" ")}>{session.command.join(" ")}</span>
              </>
            )}
          </div>
        )}
        <div className="ml-auto flex items-center gap-5">
          <Stat icon={<Wrench className="size-4" />} label="tool calls" value={stats.tools} />
          <Stat icon={<Activity className="size-4" />} label="pending" value={stats.pending} />
          <Stat
            icon={<AlertTriangle className="size-4" />}
            label="errors"
            value={stats.errors}
            tone={stats.errors > 0 ? "bad" : undefined}
          />
          <Stat icon={<Clock className="size-4" />} label="avg latency" value={`${stats.avg} ms`} />
        </div>
      </header>

      <div className="flex items-center gap-1 border-b border-ink-700 bg-ink-900 px-4 py-2">
        {FILTERS.map((f) => (
          <button
            key={f.id}
            onClick={() => setFilter(f.id)}
            className={
              "rounded-md px-3 py-1 text-xs font-medium transition-colors " +
              (filter === f.id ? "bg-ink-700 text-ink-100" : "text-ink-300 hover:bg-ink-800")
            }
          >
            {f.label}
          </button>
        ))}
        <span className="ml-auto text-xs text-ink-500">{visible.length} events</span>
      </div>

      <main className="grid min-h-0 flex-1 grid-cols-[minmax(380px,5fr)_7fr]">
        <Timeline rows={visible} selectedKey={selectedKey} onSelect={setSelectedKey} follow={connection === "live"} />
        <Detail row={selected} />
      </main>
    </div>
  );
}

function Stat(props: { icon: React.ReactNode; label: string; value: number | string; tone?: "bad" }) {
  return (
    <div className="flex items-center gap-2 text-xs text-ink-300">
      <span className={props.tone === "bad" ? "text-red-400" : "text-ink-500"}>{props.icon}</span>
      <span className={"font-mono text-sm " + (props.tone === "bad" ? "text-red-400" : "text-ink-100")}>
        {props.value}
      </span>
      <span>{props.label}</span>
    </div>
  );
}

function ConnectionBadge({ state }: { state: Connection }) {
  const map: Record<Connection, { text: string; dot: string }> = {
    connecting: { text: "connecting", dot: "bg-amber-400 animate-pulse" },
    live: { text: "live", dot: "bg-emerald-400 animate-pulse" },
    reconnecting: { text: "reconnecting", dot: "bg-amber-400 animate-pulse" },
    ended: { text: "session ended", dot: "bg-ink-500" },
  };
  const { text, dot } = map[state];
  return (
    <span className="flex items-center gap-2 rounded-full border border-ink-700 px-3 py-1 text-xs text-ink-300">
      <span className={`size-2 rounded-full ${dot}`} />
      {text}
    </span>
  );
}
