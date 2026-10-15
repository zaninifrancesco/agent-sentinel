import { useMemo, useState, useEffect } from "react";
import { ArrowLeft, Download, Moon, Search, Sun, X } from "lucide-react";
import { useSentinel, type Connection } from "./useSentinel";
import { useConfig } from "./useConfig";
import { useTheme } from "./theme";
import { Timeline } from "./Timeline";
import { Detail } from "./Detail";
import { CommandPalette } from "./CommandPalette";
import { MetricsBar } from "./MetricsBar";
import { ExportModal } from "./ExportModal";
import { Overview } from "./Overview";
import { computeMetrics } from "./metrics";
import { buildProfile, filterCalls, recordedSpan, sortCalls, type CallFilter, type CallSort } from "./profile";
import { formatDuration, formatTime } from "./format";
import type { Row } from "./types";

type View = "overview" | "calls" | "events";
const VIEWS: { id: View; label: string }[] = [
  { id: "overview", label: "Overview" }, { id: "calls", label: "Calls" }, { id: "events", label: "Event log" },
];
const FILTERS: { id: CallFilter; label: string }[] = [
  { id: "all", label: "All calls" }, { id: "errors", label: "Errors" }, { id: "open", label: "Open" }, { id: "interrupted", label: "Interrupted" },
];

export default function App() {
  const { session, rows, connection } = useSentinel();
  const { config } = useConfig();
  const [theme, toggleTheme] = useTheme();
  const [view, setView] = useState<View>("overview");
  const [filter, setFilter] = useState<CallFilter>("all");
  const [sort, setSort] = useState<CallSort>("time");
  const [tool, setTool] = useState("");
  const [query, setQuery] = useState("");
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState(false);
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);
  const replay = config.mode === "replay";

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key === "k" && !isExportModalOpen) {
        event.preventDefault();
        setIsCommandPaletteOpen((previous) => !previous);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isExportModalOpen]);

  const metrics = useMemo(() => computeMetrics(rows), [rows]);
  const profile = useMemo(() => buildProfile(rows), [rows]);
  const span = useMemo(() => recordedSpan(session, rows), [session, rows]);
  const visible = useMemo(() => {
    if (view === "events") {
      const q = query.trim().toLocaleLowerCase();
      return q ? rows.filter((row) => `${row.title} ${row.subtitle ?? ""}`.toLocaleLowerCase().includes(q)) : rows;
    }
    return sortCalls(filterCalls(profile.calls, filter, tool, query), sort);
  }, [rows, profile.calls, view, filter, tool, query, sort]);

  useEffect(() => {
    if (view !== "overview" && !visible.some((row) => row.key === selectedKey)) {
      setSelectedKey(visible[0]?.key ?? null);
    }
  }, [visible, selectedKey, view]);
  const selected = visible.find((row) => row.key === selectedKey) ?? null;

  const openCalls = (nextFilter: CallFilter = "all", nextTool = "") => {
    setFilter(nextFilter); setTool(nextTool); setQuery(""); setSort("time"); setDetailOpen(false); setView("calls");
  };
  const inspect = (row: Row) => {
    setFilter("all"); setTool(""); setQuery(""); setSort("time");
    setSelectedKey(row.key); setDetailOpen(true); setView(row.kind === "tool" ? "calls" : "events");
  };
  const changeView = (next: View) => {
    setView(next); setDetailOpen(false); setQuery("");
  };

  return (
    <div className="flex h-full min-w-0 flex-col bg-paper text-ink">
      <header className="light-scope flex flex-wrap items-center gap-x-4 gap-y-2 bg-ink px-4 py-3 text-white sm:px-6">
        <span className="whitespace-nowrap text-base font-bold tracking-tight">Agent Sentinel</span>
        <span className="hidden border-l border-ink-3 pl-4 text-xs text-rule sm:block">Local profiler</span>
        <ConnectionBadge state={connection} replay={replay} />
        <div className="ml-auto flex shrink-0 items-center gap-2">
          <button disabled={!rows.length} onClick={() => setIsCommandPaletteOpen(true)} aria-label="Search events" className="header-button">
            <Search aria-hidden className="size-4" /><span className="hidden md:inline">Search events</span><kbd className="hidden font-mono text-[11px] text-rule-strong md:inline">⌘K</kbd>
          </button>
          <button onClick={toggleTheme} aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"} className="header-button">
            {theme === "dark" ? <Sun aria-hidden className="size-4" /> : <Moon aria-hidden className="size-4" />}
          </button>
          <button disabled={!session} onClick={() => setIsExportModalOpen(true)} aria-label="Export session report" className="header-button">
            <Download aria-hidden className="size-4" /><span className="hidden sm:inline">Export report</span>
          </button>
        </div>
      </header>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-1 border-b border-rule bg-sheet px-4 py-2 text-xs text-ink-2 sm:px-6">
        <span className="min-w-0 truncate" title={session?.id}>Recording <span className="ml-1 font-mono text-ink">{session?.id ?? "waiting for connection"}</span></span>
        {session && <span>Started <span className="fig ml-1 text-ink">{formatTime(session.startedAt)}</span></span>}
        {span !== undefined && <span title="Span between captured timestamps, not total task or model time">Recorded span <span className="fig ml-1 text-ink">{formatDuration(span)}</span></span>}
        {!!session?.command?.length && <span className="min-w-0 truncate font-mono text-ink-3" title={session.command.join(" ")}>{session.command.join(" ")}</span>}
      </div>

      <nav aria-label="Recording views" className="flex items-center gap-6 border-b border-rule-strong px-4 sm:px-6">
        {VIEWS.map((item) => <button key={item.id} aria-current={view === item.id ? "page" : undefined} onClick={() => changeView(item.id)}
          className={`-mb-px flex items-center gap-2 border-b-2 py-3 text-sm font-medium ${view === item.id ? "border-ink text-ink" : "border-transparent text-ink-2 hover:text-ink"}`}>
          {item.label}{item.id === "calls" && <span className="fig text-xs text-ink-2">{metrics.calls}</span>}
        </button>)}
      </nav>
      <MetricsBar metrics={metrics} onErrors={() => openCalls("errors")} />

      {view === "overview" ? <Overview profile={profile} replay={replay} connecting={!replay && (connection === "connecting" || connection === "reconnecting")} onCalls={() => openCalls()} onTool={(name) => openCalls("all", name)} onInspect={inspect} /> : (
        <main className="flex min-h-0 flex-1 flex-col" aria-label={view === "calls" ? "Call inspector" : "Event log"}>
          <div className={`flex-wrap items-center gap-3 border-b border-rule-strong bg-paper px-4 py-3 sm:px-6 ${detailOpen ? "hidden lg:flex" : "flex"}`}>
            <label className="relative min-w-[160px] flex-1 sm:max-w-sm">
              <Search aria-hidden className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-ink-2" />
              <input aria-label={view === "calls" ? "Filter calls by tool or arguments" : "Filter captured events"} value={query} onChange={(event) => { setQuery(event.target.value); setDetailOpen(false); }}
                placeholder={view === "calls" ? "Filter tool or arguments…" : "Filter events…"} className="w-full rounded-sm border border-rule-strong bg-sheet py-2 pl-9 pr-3 text-sm placeholder:text-ink-3" />
            </label>
            {view === "calls" && <>
              <label className="flex items-center gap-2 text-xs text-ink-2">Sort
                <select aria-label="Sort calls" value={sort} onChange={(event) => { setSort(event.target.value as CallSort); setSelectedKey(null); }} className="rounded-sm border border-rule-strong bg-sheet px-2 py-2 text-sm text-ink">
                  <option value="time">Capture order</option><option value="duration">Longest first</option><option value="response">Largest response</option>
                </select>
              </label>
              {tool && <button onClick={() => setTool("")} className="action-button" aria-label={`Clear ${tool} filter`}><span className="max-w-40 truncate">{tool}</span><X aria-hidden className="size-3.5" /></button>}
            </>}
            <span className="fig ml-auto text-xs text-ink-2">{visible.length} {view === "calls" ? "calls" : "entries"}</span>
          </div>
          {view === "calls" && <div role="group" aria-label="Call status filters" className={`flex-wrap gap-x-5 gap-y-1 border-b border-rule px-4 sm:px-6 ${detailOpen ? "hidden lg:flex" : "flex"}`}>
            {FILTERS.map((item) => <button key={item.id} aria-pressed={filter === item.id} onClick={() => { setFilter(item.id); setDetailOpen(false); }}
              className={`-mb-px border-b-2 py-2.5 text-xs font-medium ${filter === item.id ? "border-ink text-ink" : "border-transparent text-ink-2 hover:text-ink"}`}>{item.label}</button>)}
          </div>}
          <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(360px,5fr)_7fr]">
            <div className={`min-h-0 flex-col lg:border-r lg:border-rule-strong ${detailOpen ? "hidden lg:flex" : "flex"}`}>
              <Timeline rows={visible} selectedKey={selectedKey} onSelect={(key) => { setSelectedKey(key); setDetailOpen(true); }}
                follow={!replay && connection === "live" && (view === "events" || sort === "time")} showResponseSize={view === "calls"}
                emptyTitle={rows.length === 0 ? "No captured events yet" : "No matching calls or events"}
                emptyDescription={rows.length === 0 ? "Connect an agent or MCP server to start recording." : "Change the filters or search to inspect other captured events."} />
            </div>
            <div className={`min-h-0 flex-col ${detailOpen ? "flex" : "hidden lg:flex"}`}>
              <button onClick={() => setDetailOpen(false)} className="flex items-center gap-2 border-b border-rule-strong px-4 py-3 text-sm font-medium lg:hidden"><ArrowLeft aria-hidden className="size-4" />Back to {view === "calls" ? "calls" : "event log"}</button>
              <Detail key={selected?.key ?? "empty"} row={selected} />
            </div>
          </div>
        </main>
      )}

      <CommandPalette isOpen={isCommandPaletteOpen} onClose={() => setIsCommandPaletteOpen(false)} rows={rows}
        onSelectRow={(key) => { const row = rows.find((entry) => entry.key === key); if (row) inspect(row); }} onExport={() => setIsExportModalOpen(true)} />
      <ExportModal isOpen={isExportModalOpen} onClose={() => setIsExportModalOpen(false)} session={session} rows={rows} config={config} />
    </div>
  );
}

function ConnectionBadge({ state, replay }: { state: Connection; replay: boolean }) {
  const labels: Record<Connection, string> = { connecting: "Connecting", live: "Live capture", reconnecting: "Reconnecting", ended: "Capture ended" };
  return <span role="status" className="flex items-center gap-2 text-xs font-medium"><span aria-hidden className={`size-1.5 ${!replay && state === "live" ? "bg-yellow" : "border border-rule-strong"}`} />{replay ? "Replay" : labels[state]}</span>;
}
