import { useMemo, useState, useEffect } from "react";
import {
  Activity,
  AlertTriangle,
  Clock,
  ShieldCheck,
  Wrench,
  Search,
  Download,
} from "lucide-react";
import { useSentinel, type Connection } from "./useSentinel";
import { Timeline } from "./Timeline";
import { Detail } from "./Detail";
import { CommandPalette } from "./CommandPalette";
import { ApprovalBar } from "./ApprovalBar";
import { MetricsBar } from "./MetricsBar";
import { ExportModal } from "./ExportModal";
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
      return (
        row.status === "error" ||
        row.status === "blocked" ||
        row.status === "rejected"
      );
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
  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState(false);
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);

  // Global shortcut for Cmd+K / Ctrl+K
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setIsCommandPaletteOpen((prev) => !prev);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const visible = useMemo(
    () => rows.filter((r) => matches(r, filter)),
    [rows, filter]
  );

  // Auto-select first item if none selected
  useEffect(() => {
    if (!selectedKey && visible.length > 0) {
      setSelectedKey(visible[0].key);
    }
  }, [visible, selectedKey]);

  const selected = rows.find((r) => r.key === selectedKey) ?? null;

  // Human-in-the-Loop: only a still-open, high-risk call in a LIVE session needs
  // a decision. Every call is "pending" while it is in flight, and a finished
  // (replayed) session has nothing left to approve.
  const pendingRow = useMemo(() => {
    if (connection !== "live") return null;
    return (
      rows.find((r) => r.status === "pending" && (r.risk === "high" || r.risk === "critical")) ?? null
    );
  }, [rows, connection]);

  const stats = useMemo(() => {
    const tools = rows.filter((r) => r.kind === "tool");
    const timed = tools.filter((r) => r.durationMs !== undefined);
    const avg = timed.length
      ? timed.reduce((s, r) => s + (r.durationMs ?? 0), 0) / timed.length
      : 0;
    return {
      tools: tools.length,
      errors: rows.filter(
        (r) => r.status === "error" || r.status === "blocked"
      ).length,
      pending: rows.filter((r) => r.status === "pending").length,
      avg: Math.round(avg),
    };
  }, [rows]);

  const handleApprove = (key: string, feedback?: string) => {
    console.log("Approved action:", key, "with feedback:", feedback);
    // In live mode with backend approval hook, sends POST to /api/approve
  };

  const handleBlock = (key: string) => {
    console.log("Blocked action:", key);
    // In live mode with backend approval hook, sends POST to /api/block
  };

  return (
    <div className="flex h-full flex-col bg-ink-950 text-ink-100 select-none">
      {/* Top Header */}
      <header className="flex items-center gap-5 border-b border-ink-800 bg-ink-900 px-5 py-2.5">
        <div className="flex items-center gap-2.5">
          <div className="flex size-7 items-center justify-center rounded-lg bg-accent/15 border border-accent/30 text-accent shadow-sm">
            <ShieldCheck className="size-4" />
          </div>
          <div className="flex flex-col">
            <span className="font-semibold text-xs tracking-tight text-ink-100">
              Agent Sentinel
            </span>
            <span className="text-[10px] text-ink-400 font-mono">Cockpit v0.1</span>
          </div>
        </div>

        <ConnectionBadge state={connection} />

        {session && (
          <div className="min-w-0 truncate font-mono text-xs text-ink-400">
            session <span className="text-accent">{session.id}</span>
            {session.command && session.command.length > 0 && (
              <>
                {" · "}
                <span title={session.command.join(" ")} className="text-ink-300">
                  {session.command.join(" ")}
                </span>
              </>
            )}
          </div>
        )}

        {/* Global Search / Command Palette Button */}
        <button
          onClick={() => setIsCommandPaletteOpen(true)}
          className="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-850 px-3 py-1.5 text-xs text-ink-400 hover:border-ink-600 hover:text-ink-200 transition-colors"
        >
          <Search className="size-3.5" />
          <span>Search events...</span>
          <kbd className="ml-2 rounded border border-ink-700 bg-ink-800 px-1.5 py-0.2 font-mono text-[10px] text-ink-400">
            ⌘K
          </kbd>
        </button>

        {/* Header Stats */}
        <div className="ml-auto flex items-center gap-5">
          <Stat icon={<Wrench className="size-3.5" />} label="tools" value={stats.tools} />
          <Stat icon={<Activity className="size-3.5" />} label="pending" value={stats.pending} />
          <Stat
            icon={<AlertTriangle className="size-3.5" />}
            label="errors"
            value={stats.errors}
            tone={stats.errors > 0 ? "bad" : undefined}
          />
          <Stat icon={<Clock className="size-3.5" />} label="avg" value={`${stats.avg} ms`} />

          <button
            onClick={() => setIsExportModalOpen(true)}
            className="flex items-center gap-1.5 rounded-lg border border-ink-700 bg-ink-800 px-3 py-1.5 text-xs text-ink-200 hover:bg-ink-700 hover:text-ink-100 transition-colors shadow-sm"
          >
            <Download className="size-3.5 text-accent" />
            <span>Export Deliverable</span>
          </button>
        </div>
      </header>

      {/* LLMOps Metrics Bar */}
      <MetricsBar rows={rows} />

      {/* Filter Toolbar */}
      <div className="flex items-center gap-1 border-b border-ink-800 bg-ink-900/60 px-5 py-2">
        {FILTERS.map((f) => (
          <button
            key={f.id}
            onClick={() => setFilter(f.id)}
            className={`rounded-md px-3 py-1 text-xs font-medium transition-colors ${
              filter === f.id
                ? "bg-ink-700 text-ink-100 shadow-sm"
                : "text-ink-400 hover:bg-ink-800 hover:text-ink-200"
            }`}
          >
            {f.label}
          </button>
        ))}
        <span className="ml-auto font-mono text-xs text-ink-500">
          {visible.length} events
        </span>
      </div>

      {/* Split Main Content Area */}
      <main className="grid min-h-0 flex-1 grid-cols-[minmax(380px,5fr)_7fr]">
        <Timeline
          rows={visible}
          selectedKey={selectedKey}
          onSelect={setSelectedKey}
          follow={connection === "live"}
        />
        <Detail row={selected} />
      </main>

      {/* Floating Human-in-the-Loop Approval Bar */}
      <ApprovalBar
        pendingRow={pendingRow}
        onApprove={handleApprove}
        onBlock={handleBlock}
      />

      {/* Command Palette Modal */}
      <CommandPalette
        isOpen={isCommandPaletteOpen}
        onClose={() => setIsCommandPaletteOpen(false)}
        rows={rows}
        onSelectRow={(key) => setSelectedKey(key)}
        onExport={() => setIsExportModalOpen(true)}
      />

      {/* Standalone Export Deliverable Modal */}
      <ExportModal
        isOpen={isExportModalOpen}
        onClose={() => setIsExportModalOpen(false)}
        session={session}
        rows={rows}
      />
    </div>
  );
}

function Stat(props: {
  icon: React.ReactNode;
  label: string;
  value: number | string;
  tone?: "bad";
}) {
  return (
    <div className="flex items-center gap-2 text-xs text-ink-400">
      <span className={props.tone === "bad" ? "text-rose-400" : "text-ink-500"}>
        {props.icon}
      </span>
      <span
        className={
          "font-mono text-xs font-semibold " +
          (props.tone === "bad" ? "text-rose-400" : "text-ink-100")
        }
      >
        {props.value}
      </span>
      <span className="text-[11px]">{props.label}</span>
    </div>
  );
}

function ConnectionBadge({ state }: { state: Connection }) {
  const map: Record<Connection, { text: string; dot: string }> = {
    connecting: { text: "connecting", dot: "bg-amber-400 animate-pulse" },
    live: { text: "live", dot: "bg-emerald-400 animate-pulse shadow-sm shadow-emerald-400" },
    reconnecting: { text: "reconnecting", dot: "bg-amber-400 animate-pulse" },
    ended: { text: "session ended", dot: "bg-ink-600" },
  };
  const { text, dot } = map[state];
  return (
    <span className="flex items-center gap-2 rounded-full border border-ink-800 bg-ink-850 px-2.5 py-1 text-xs text-ink-300">
      <span className={`size-2 rounded-full ${dot}`} />
      <span className="text-[11px] font-medium">{text}</span>
    </span>
  );
}
