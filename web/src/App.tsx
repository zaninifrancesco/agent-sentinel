import { useMemo, useState, useEffect } from "react";
import { Download, Search } from "lucide-react";
import { useSentinel, type Connection } from "./useSentinel";
import { useConfig } from "./useConfig";
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
  { id: "errors", label: "Problems" },
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
  const config = useConfig();
  const [approvalBusy, setApprovalBusy] = useState(false);
  const [approvalError, setApprovalError] = useState<string | null>(null);
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

  // Human-in-the-Loop: the proxy itself holds a call and marks it
  // "awaiting_approval"; the cockpit only has to show those, oldest first.
  // A finished (replayed) session has nothing left to approve.
  const awaiting = useMemo(
    () => (connection === "live" && config.approvals ? rows.filter((r) => r.status === "awaiting_approval") : []),
    [rows, connection, config.approvals]
  );
  const pendingRow = awaiting[0] ?? null;

  // A call that needs a decision takes the detail pane, so what is shown
  // beside the dock is what is being decided.
  const pendingKey = pendingRow?.key;
  useEffect(() => {
    if (pendingKey) setSelectedKey(pendingKey);
  }, [pendingKey]);

  const decide = async (row: Row, approved: boolean, feedback?: string) => {
    if (!row.request) return;
    setApprovalBusy(true);
    setApprovalError(null);
    try {
      const res = await fetch(`/api/approvals/${row.request.seq}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ approved, feedback: feedback ?? "" }),
      });
      // 409 = already settled (timed out or cancelled by the agent): the
      // timeline updates by itself, so there is nothing to show.
      if (!res.ok && res.status !== 409) {
        setApprovalError(`Sentinel refused the answer (${res.status}): ${(await res.text()).trim()}`);
      }
    } catch (err) {
      setApprovalError(`Could not reach Sentinel: ${String(err)}`);
    } finally {
      setApprovalBusy(false);
    }
  };
  const handleApprove = (row: Row, feedback?: string) => void decide(row, true, feedback);
  const handleBlock = (row: Row, feedback?: string) => void decide(row, false, feedback);

  return (
    <div className="flex h-full flex-col bg-paper text-ink">
      <header className="flex items-center gap-4 bg-ink px-4 py-2.5 text-white sm:gap-6 sm:px-6">
        <span className="whitespace-nowrap text-base font-bold tracking-tight">Agent Sentinel</span>

        <ConnectionBadge state={connection} />

        {session && (
          <div className="hidden min-w-0 truncate font-mono text-xs text-rule-strong md:block">
            session <span className="text-white">{session.id}</span>
            {session.command && session.command.length > 0 && (
              <>
                {" · "}
                <span title={session.command.join(" ")}>{session.command.join(" ")}</span>
              </>
            )}
          </div>
        )}

        <div className="ml-auto flex items-center gap-3">
          <button
            onClick={() => setIsCommandPaletteOpen(true)}
            className="flex items-center gap-2 rounded-sm border border-ink-3 px-3 py-1.5 text-xs text-rule hover:border-white hover:text-white"
          >
            <Search className="size-3.5" />
            <span className="hidden sm:inline">Search events</span>
            <kbd className="font-mono text-[11px] text-rule-strong">⌘K</kbd>
          </button>
          <button
            onClick={() => setIsExportModalOpen(true)}
            className="flex items-center gap-1.5 rounded-sm border border-ink-3 px-3 py-1.5 text-xs text-rule hover:border-white hover:text-white"
          >
            <Download className="size-3.5" />
            <span className="hidden sm:inline">Export</span>
          </button>
        </div>
      </header>

      <MetricsBar rows={rows} maxCostUsd={config.maxCostUsd} />

      <main className="grid min-h-0 flex-1 grid-cols-1 grid-rows-[minmax(0,2fr)_minmax(0,3fr)] lg:grid-cols-[minmax(420px,5fr)_7fr] lg:grid-rows-1">
        <div className="flex min-h-0 flex-col border-r border-rule-strong">
          <div role="tablist" aria-label="Filter events" className="flex items-center gap-5 border-b border-rule-strong bg-paper px-4">
            {FILTERS.map((f) => (
              <button
                key={f.id}
                role="tab"
                aria-selected={filter === f.id}
                onClick={() => setFilter(f.id)}
                className={`-mb-px border-b-2 py-2 text-sm font-medium ${
                  filter === f.id ? "border-ink text-ink" : "border-transparent text-ink-2 hover:text-ink"
                }`}
              >
                {f.label}
              </button>
            ))}
            <span className="fig ml-auto text-xs text-ink-2">{visible.length} events</span>
          </div>
          <Timeline
            rows={visible}
            selectedKey={selectedKey}
            onSelect={setSelectedKey}
            follow={connection === "live"}
          />
        </div>
        <Detail row={selected} />
      </main>

      {/* Human-in-the-Loop approval dock (part of the layout, below the panes) */}
      <ApprovalBar
        pendingRow={pendingRow}
        more={Math.max(0, awaiting.length - 1)}
        busy={approvalBusy}
        error={approvalError}
        timeoutSec={config.approvalTimeoutSec}
        onApprove={handleApprove}
        onBlock={handleBlock}
      />

      <CommandPalette
        isOpen={isCommandPaletteOpen}
        onClose={() => setIsCommandPaletteOpen(false)}
        rows={rows}
        onSelectRow={(key) => setSelectedKey(key)}
        onExport={() => setIsExportModalOpen(true)}
      />

      <ExportModal
        isOpen={isExportModalOpen}
        onClose={() => setIsExportModalOpen(false)}
        session={session}
        rows={rows}
      />
    </div>
  );
}

function ConnectionBadge({ state }: { state: Connection }) {
  const map: Record<Connection, { text: string; dot: string }> = {
    connecting: { text: "Connecting", dot: "border border-white" },
    live: { text: "Live", dot: "bg-white" },
    reconnecting: { text: "Reconnecting", dot: "border border-white" },
    ended: { text: "Session ended", dot: "border border-rule-strong" },
  };
  const { text, dot } = map[state];
  return (
    <span className="flex items-center gap-2 text-xs font-medium">
      <span className={`size-2 ${dot}`} />
      {text}
    </span>
  );
}
