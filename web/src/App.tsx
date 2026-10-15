import { useMemo, useState, useEffect } from "react";
import { ArrowLeft, Download, Moon, Search, Sun, X, ChartNoAxesCombined, ListTree, ScanLine, Radio, FileJson, HardDrive, ChevronLeft, ChevronRight, PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { useSentinel, type Connection } from "./useSentinel";
import { useConfig } from "./useConfig";
import { useTheme } from "./theme";
import { Timeline } from "./Timeline";
import { Detail } from "./Detail";
import { CommandPalette } from "./CommandPalette";
import { MetricsBar } from "./MetricsBar";
import { TraceMap } from "./TraceMap";
import { ExportModal } from "./ExportModal";
import { Overview } from "./Overview";
import { computeMetrics } from "./metrics";
import { buildProfile, filterCalls, recordedSpan, sortCalls, type CallFilter, type CallSort } from "./profile";
import { formatDuration, formatTime } from "./format";
import type { Row } from "./types";
import { filterScope, recordingScopes } from "./scopes";

type View = "overview" | "calls" | "events";
const VIEWS: { id: View; label: string }[] = [
  { id: "calls", label: "Timeline" }, { id: "overview", label: "Tool profile" }, { id: "events", label: "Event log" },
];
const FILTERS: { id: CallFilter; label: string }[] = [
  { id: "all", label: "All calls" }, { id: "errors", label: "Errors" }, { id: "open", label: "Open" }, { id: "interrupted", label: "Interrupted" },
  { id: "unresolved", label: "Unresolved" },
];
const RAIL_KEY = "sentinel-rail-collapsed";

function initialRailCollapsed(): boolean {
  try { return localStorage.getItem(RAIL_KEY) === "true"; } catch { return false; }
}

export default function App() {
  const { session, rows: allRows, connection } = useSentinel();
  const { config } = useConfig();
  const [theme, toggleTheme] = useTheme();
  const [railCollapsed, setRailCollapsed] = useState(initialRailCollapsed);
  const [view, setView] = useState<View>("calls");
  const [scope, setScope] = useState("");
  const scopes = useMemo(() => recordingScopes(allRows), [allRows]);
  const rows = useMemo(() => filterScope(allRows, scope), [allRows, scope]);
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
    try { localStorage.setItem(RAIL_KEY, String(railCollapsed)); } catch { /* preference is optional */ }
  }, [railCollapsed]);

  useEffect(() => {
    if (scope && !scopes.some((item) => item.id === scope)) setScope("");
  }, [scope, scopes]);

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

  const selectedIndex = visible.findIndex((row) => row.key === selectedKey);
  const selectCall = (key: string) => { setSelectedKey(key); setDetailOpen(true); };
  const viewIcons = { calls: ListTree, overview: ChartNoAxesCombined, events: FileJson };
  const subtitle = { calls: "Follow the sequence. Inspect every call.", overview: "Compare tool time, errors and retained output.", events: "Inspect protocol and recording lifecycle events." };
  return (
    <div className="app-shell">
      <aside className="app-rail" data-collapsed={railCollapsed} aria-label="Workspace navigation">
        <div className="rail-top">
          <div className="wordmark"><span className="brand-mark"><ScanLine aria-hidden className="size-4" /></span><span className="wordmark-label">Agent Sentinel</span></div>
          <button className="rail-toggle" aria-label={railCollapsed ? "Expand sidebar" : "Collapse sidebar"} aria-expanded={!railCollapsed} aria-controls="recording-navigation" onClick={() => setRailCollapsed((value) => !value)} title={railCollapsed ? "Expand sidebar" : "Collapse sidebar"}>
            {railCollapsed ? <PanelLeftOpen aria-hidden className="size-4" /> : <PanelLeftClose aria-hidden className="size-4" />}
          </button>
        </div>
        <div className="ml-auto text-[#b9c8e6] md:hidden"><ConnectionBadge state={connection} replay={replay} /></div>
        <nav id="recording-navigation" aria-label="Recording views" className="rail-nav">
          {VIEWS.map((item) => { const Icon = viewIcons[item.id]; return <button key={item.id} title={railCollapsed ? item.label : undefined} aria-label={item.label} aria-current={view === item.id ? "page" : undefined} onClick={() => changeView(item.id)} className="rail-link">
            <Icon aria-hidden className="size-4" /><span className="rail-label">{item.label}</span>{item.id === "calls" && <span className="count">{metrics.calls}</span>}
          </button>; })}
        </nav>
        <div className="rail-recording" aria-hidden={railCollapsed}>
          <div className="mb-3 flex items-center gap-2 text-[#e0e8fa]"><Radio aria-hidden className="size-3.5" />Current recording</div>
          <p className="break-all font-mono text-[10px] text-[#b9c8e6]">{session?.id ?? "Waiting for connection"}</p>
          {session && <p className="mt-3">Started {formatTime(session.startedAt)}</p>}
          {span !== undefined && <p className="mt-1">{formatDuration(span)} captured span</p>}
          {!!session?.command?.length && <p className="mt-3 break-words" title={session.command.join(" ")}>{session.command.join(" ")}</p>}
        </div>
        <div className="rail-foot"><ConnectionBadge state={connection} replay={replay} /><div className="rail-storage mt-4 flex items-center gap-2 border-t border-[#354461] pt-4"><HardDrive aria-hidden className="size-3.5" />Stored locally</div><p className="rail-observation mt-1">Observation only</p></div>
      </aside>
      <div className="app-workspace">
        <header className="workspace-header">
          <div className="min-w-0"><div className="flex flex-wrap items-center gap-3"><h1>{VIEWS.find((item) => item.id === view)?.label}</h1><span className="recording-id">{session?.id ?? "Connecting"}</span></div><p className="workspace-caption">{subtitle[view]}</p></div>
          {scopes.some((item) => item.id !== "unattributed") && <select aria-label="Filter agent session or connection" title={scopes.find((item) => item.id === scope)?.label ?? "All captured sessions and connections"} value={scope} onChange={(event) => { setScope(event.target.value); setDetailOpen(false); setSelectedKey(null); setQuery(""); setTool(""); setFilter("all"); }} className="field-input max-w-64 min-w-0 flex-1 sm:flex-none"><option value="">All sessions & connections</option>{scopes.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select>}
          <div className="flex items-center gap-2">
            <button disabled={!rows.length} onClick={() => setIsCommandPaletteOpen(true)} aria-label="Search events" className="icon-button"><Search aria-hidden className="size-4" /></button>
            <button onClick={toggleTheme} aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"} className="icon-button">{theme === "dark" ? <Sun aria-hidden className="size-4" /> : <Moon aria-hidden className="size-4" />}</button>
            <button disabled={!session} onClick={() => setIsExportModalOpen(true)} aria-label="Export session report" className="action-button"><Download aria-hidden className="size-3.5" /><span className="hidden sm:inline">Export report</span></button>
          </div>
        </header>
        <MetricsBar metrics={metrics} onErrors={() => openCalls("errors")} />
        {view === "overview" ? <Overview profile={profile} replay={replay} connecting={!replay && (connection === "connecting" || connection === "reconnecting")} onCalls={() => openCalls()} onTool={(name) => openCalls("all", name)} onInspect={inspect} /> : (
          <main className={`${view === "calls" ? "call-main " : ""}flex min-h-0 flex-1 flex-col`} aria-label={view === "calls" ? "Call inspector" : "Event log"}>
            {view === "calls" && <div className={`trace-holder ${detailOpen ? "hidden lg:block" : "block"}`}><TraceMap rows={profile.calls} selectedKey={selectedKey} onSelect={inspect} /></div>}
            <div className="inspector-workspace">
              <div className={`inspector-toolbar ${detailOpen ? "hidden lg:flex" : "flex"}`}>
                <label className="relative min-w-40 flex-1 sm:max-w-xs"><Search aria-hidden className="pointer-events-none absolute left-2.5 top-2.5 size-3.5 text-ink-3" /><input aria-label={view === "calls" ? "Filter calls by tool or arguments" : "Filter captured events"} value={query} onChange={(event) => { setQuery(event.target.value); setDetailOpen(false); }} placeholder={view === "calls" ? "Search calls or arguments…" : "Search events…"} className="field-input w-full pl-8" /></label>
                {view === "calls" && <><select aria-label="Filter by tool" value={tool} onChange={(event) => { setTool(event.target.value); setDetailOpen(false); }} className="field-input max-w-36"><option value="">All tools</option>{profile.tools.map(({ name }) => <option key={name} value={name}>{name}</option>)}</select><select aria-label="Sort calls" value={sort} onChange={(event) => { setSort(event.target.value as CallSort); setSelectedKey(null); }} className="field-input max-w-40"><option value="time">Capture order</option><option value="duration">Longest first</option><option value="response">Largest response</option></select></>}
                <span className="ml-auto text-[11px] text-ink-2">{visible.length} {view === "calls" ? "calls" : "entries"}</span>
              </div>
              {view === "calls" && <div role="group" aria-label="Call status filters" className={`call-status-filters items-center gap-1 border-b border-rule px-3 py-2 ${detailOpen ? "hidden lg:flex" : "flex flex-wrap"}`}>
                {FILTERS.map((item) => <button key={item.id} aria-pressed={filter === item.id} onClick={() => { setFilter(item.id); setDetailOpen(false); }} className="status-filter">{item.label}{item.id === "errors" && metrics.errors > 0 && <span className="text-warn">{metrics.errors}</span>}</button>)}
                {tool && <button onClick={() => setTool("")} className="status-filter ml-auto" aria-label={`Clear ${tool} filter`}><span className="max-w-24 truncate">{tool}</span><X aria-hidden className="size-3" /></button>}
              </div>}
              <div className="inspector-split">
                <div className={`event-list-pane min-h-0 flex-col lg:border-r lg:border-rule ${detailOpen ? "hidden lg:flex" : "flex"}`}>
                  <Timeline rows={visible} selectedKey={selectedKey} onSelect={selectCall} follow={!replay && connection === "live" && (view === "events" || sort === "time")} showResponseSize={view === "calls"}
                    emptyTitle={rows.length === 0 ? "Your first call starts here" : "No matching calls or events"} emptyDescription={rows.length === 0 ? "Connect an agent with sentinel hook install, or observe an MCP server with sentinel mcp. Calls appear here as they are captured." : "Change the status, tool or search filters to inspect other events."} />
                </div>
                <div className={`call-detail-pane min-h-0 flex-col ${detailOpen ? "flex" : "hidden lg:flex"}`}>
                  <div className="flex items-center justify-between border-b border-rule bg-paper px-3 py-1.5 text-[11px] text-ink-2">
                    <span className="hidden lg:inline">Call details</span>
                    <button onClick={() => setDetailOpen(false)} className="flex items-center gap-1.5 py-2 font-medium text-accent lg:hidden"><ArrowLeft aria-hidden className="size-3.5" />Back to {view === "calls" ? "timeline" : "event log"}</button>
                    <div className="flex items-center gap-1"><span className="mr-2">{selectedIndex >= 0 ? `${selectedIndex + 1} of ${visible.length}` : "No selection"}</span><button className="icon-button" aria-label="Inspect previous event" disabled={selectedIndex <= 0} onClick={() => selectCall(visible[selectedIndex - 1].key)}><ChevronLeft aria-hidden className="size-3.5" /></button><button className="icon-button" aria-label="Inspect next event" disabled={selectedIndex < 0 || selectedIndex >= visible.length - 1} onClick={() => selectCall(visible[selectedIndex + 1].key)}><ChevronRight aria-hidden className="size-3.5" /></button></div>
                  </div>
                  <Detail key={selected?.key ?? "empty"} row={selected} />
                </div>
              </div>
            </div>
          </main>
        )}
      </div>
      <CommandPalette isOpen={isCommandPaletteOpen} onClose={() => setIsCommandPaletteOpen(false)} rows={rows}
        onSelectRow={(key) => { const row = rows.find((entry) => entry.key === key); if (row) inspect(row); }} onExport={() => setIsExportModalOpen(true)} />
      <ExportModal isOpen={isExportModalOpen} onClose={() => setIsExportModalOpen(false)} session={session} rows={allRows} config={config} />
    </div>
  );
}

function ConnectionBadge({ state, replay }: { state: Connection; replay: boolean }) {
  const labels: Record<Connection, string> = { connecting: "Connecting", live: "Live capture", reconnecting: "Reconnecting", ended: "Capture ended" };
  return <span role="status" className="capture-status"><i aria-hidden style={{ color: !replay && state === "live" ? "#76cbb8" : "#a6b5d1" }} />{replay ? "Replay" : labels[state]}</span>;
}
