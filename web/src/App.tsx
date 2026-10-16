import { useMemo, useState, useEffect } from "react";
import { 
  ArrowLeft, Download, Moon, Search, Sun, X, ChartNoAxesCombined, 
  ListTree, ScanLine, Radio, FileJson, HardDrive, ChevronLeft, ChevronRight, 
  PanelLeftClose, PanelLeftOpen, Activity, FolderClock, Copy, Check,
  PanelRightClose, PanelRightOpen, Keyboard
} from "lucide-react";
import { useProfiler, useRow, type Connection } from "./useProfiler";
import { useConfig } from "./useConfig";
import { useTheme } from "./theme";
import { Timeline } from "./Timeline";
import { Detail } from "./Detail";
import { CommandPalette } from "./CommandPalette";
import { MetricsBar } from "./MetricsBar";
import { TraceMap } from "./TraceMap";
import { ExportModal } from "./ExportModal";
import { Overview } from "./Overview";
import { Analytics } from "./Analytics";
import { ShortcutsModal } from "./ShortcutsModal";
import { EMPTY_METRICS } from "./metrics";
import { type CallFilter, type CallSort } from "./profile";
import { formatBytes, formatDuration, formatTime } from "./format";
import { toolColor } from "./trace";
import type { Row, HistorySession } from "./types";

type View = "analytics" | "sessions" | "overview" | "calls" | "events";
const VIEWS: { id: View; label: string }[] = [
  { id: "calls", label: "Timeline" },
  { id: "sessions", label: "Sessions" },
  { id: "analytics", label: "Statistics" },
  { id: "overview", label: "Tool profile" },
  { id: "events", label: "Event log" },
];
const FILTERS: { id: CallFilter; label: string }[] = [
  { id: "all", label: "All calls" },
  { id: "errors", label: "Errors" },
  { id: "open", label: "Open" },
  { id: "interrupted", label: "Interrupted" },
  { id: "unresolved", label: "Unresolved" },
];
const RAIL_KEY = "sentinel-rail-collapsed";

function initialRailCollapsed(): boolean {
  try {
    return localStorage.getItem(RAIL_KEY) === "true";
  } catch {
    return false;
  }
}

export default function App() {
  const { config } = useConfig();
  const [theme, toggleTheme] = useTheme();
  const [railCollapsed, setRailCollapsed] = useState(initialRailCollapsed);
  const [view, setView] = useState<View>("calls");
  const [recordingId, setRecordingId] = useState("");
  const [scope, setScope] = useState("");
  const [filter, setFilter] = useState<CallFilter>("all");
  const [sort, setSort] = useState<CallSort>("time");
  const [tool, setTool] = useState("");
  const [query, setQuery] = useState("");
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailExpanded, setDetailExpanded] = useState(false);
  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState(false);
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);
  const [isShortcutsModalOpen, setIsShortcutsModalOpen] = useState(false);
  const [copiedSessionId, setCopiedSessionId] = useState(false);
  const [cursor, setCursor] = useState("");
  const [focusKey, setFocusKey] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query), 200);
    return () => clearTimeout(timer);
  }, [query]);

  const pageQuery = useMemo(
    () =>
      new URLSearchParams({
        view: view === "events" ? "events" : "calls",
        scope,
        filter,
        sort,
        tool,
        q: debouncedQuery,
        limit: "50",
        ...(recordingId ? { recordingId } : {}),
        ...(cursor ? { cursor } : {}),
        ...(focusKey ? { focus: focusKey } : {}),
      }).toString(),
    [view, scope, filter, sort, tool, debouncedQuery, cursor, focusKey, recordingId]
  );

  const { data, loading, error, connection, retry } = useProfiler(pageQuery);
  const session = data?.session ?? null;
  const scopes = data?.scopes ?? [];
  const metrics = data?.metrics ?? EMPTY_METRICS;
  const profile = data?.profile ?? { calls: 0, tools: [], longest: [], largest: [] };
  const visible = data?.rows ?? [];
  const span = data?.timeline.spanMs;
  const detail = useRow(selectedKey, session?.id, data?.revision);

  // Cursor signatures include every filter. Reset before issuing a new query.
  const resetPage = () => {
    setCursor("");
    setFocusKey("");
  };

  const replay = config.mode === "replay" || data?.readOnly === true;
  const analyticsView = view === "analytics" || view === "sessions";

  useEffect(() => {
    try {
      localStorage.setItem(RAIL_KEY, String(railCollapsed));
    } catch {
      /* preference is optional */
    }
  }, [railCollapsed]);

  useEffect(() => {
    if (!loading && scope && !scopes.some((item) => item.id === scope)) {
      setScope("");
      resetPage();
    }
  }, [scope, scopes, loading]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key === "k" && !isExportModalOpen) {
        event.preventDefault();
        setIsShortcutsModalOpen(false);
        setIsCommandPaletteOpen((prev) => !prev);
      } else if (event.key === "?" && !isCommandPaletteOpen && !isExportModalOpen) {
        const target = event.target as HTMLElement | null;
        if (target?.tagName !== "INPUT" && target?.tagName !== "TEXTAREA") {
          event.preventDefault();
          setIsShortcutsModalOpen((prev) => !prev);
        }
      } else if (event.key === "Escape" && !isCommandPaletteOpen && !isExportModalOpen && !isShortcutsModalOpen) {
        setDetailOpen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isExportModalOpen, isCommandPaletteOpen, isShortcutsModalOpen]);

  useEffect(() => {
    if (!loading && !focusKey && !analyticsView && view !== "overview" && !visible.some((row) => row.key === selectedKey)) {
      setSelectedKey(visible[0]?.key ?? null);
    }
  }, [visible, selectedKey, view, loading, focusKey, analyticsView]);

  useEffect(() => {
    if (!loading && focusKey && data) {
      if (visible.some((row) => row.key === focusKey)) {
        setSelectedKey(focusKey);
        setCursor(data.current);
      }
      setFocusKey("");
    }
  }, [loading, focusKey, data, visible]);

  const selected = visible.find((row) => row.key === selectedKey) ?? null;

  const openCalls = (nextFilter: CallFilter = "all", nextTool = "") => {
    resetPage();
    setFilter(nextFilter);
    setTool(nextTool);
    setQuery("");
    setSort("time");
    setDetailOpen(false);
    setView("calls");
  };

  const inspect = (row: Row) => {
    resetPage();
    setFocusKey(row.key);
    setFilter("all");
    setTool("");
    setQuery("");
    setSort("time");
    setSelectedKey(row.key);
    setDetailOpen(true);
    setView(row.kind === "tool" ? "calls" : "events");
  };

  const changeView = (next: View) => {
    resetPage();
    setView(next);
    setDetailOpen(false);
    setQuery("");
  };

  const openHistory = (item: HistorySession) => {
    resetPage();
    setRecordingId(item.recordingId);
    setScope(item.scope);
    setFilter("all");
    setSort("time");
    setTool("");
    setQuery("");
    setDebouncedQuery("");
    setSelectedKey(null);
    setDetailOpen(false);
    setView("calls");
  };

  const copySessionId = async () => {
    if (!session?.id) return;
    try {
      await navigator.clipboard.writeText(session.id);
      setCopiedSessionId(true);
      setTimeout(() => setCopiedSessionId(false), 1600);
    } catch {
      /* ignore */
    }
  };

  const selectedIndex = visible.findIndex((row) => row.key === selectedKey);
  const selectCall = (key: string) => {
    setSelectedKey(key);
    setDetailOpen(true);
  };

  const viewIcons = {
    analytics: Activity,
    sessions: FolderClock,
    calls: ListTree,
    overview: ChartNoAxesCombined,
    events: FileJson,
  };

  const subtitle = {
    analytics: "Monitor agent activity across sessions, projects and time.",
    sessions: "Revisit the sessions behind your metrics.",
    calls: "Follow the sequence. Inspect every tool call, latency, and payload.",
    overview: "Compare tool time, errors and retained output.",
    events: "Inspect protocol and recording lifecycle events.",
  };

  return (
    <div className="app-shell flex h-[100dvh] min-w-0 bg-paper">
      {/* Sidebar Rail */}
      <aside className="app-rail" data-collapsed={railCollapsed} aria-label="Workspace navigation">
        <div className="rail-top">
          <div className="wordmark">
            <span className="brand-mark">
              <ScanLine aria-hidden className="size-4" />
            </span>
            <span className="wordmark-label">Agent Sentinel</span>
          </div>
          <button
            className="rail-toggle"
            aria-label={railCollapsed ? "Expand sidebar" : "Collapse sidebar"}
            aria-expanded={!railCollapsed}
            aria-controls="recording-navigation"
            onClick={() => setRailCollapsed((value) => !value)}
            title={railCollapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            {railCollapsed ? <PanelLeftOpen aria-hidden className="size-4" /> : <PanelLeftClose aria-hidden className="size-4" />}
          </button>
        </div>

        <div className="ml-auto text-[#b9c8e6] md:hidden">
          <ConnectionBadge state={connection} replay={replay} />
        </div>

        <nav id="recording-navigation" aria-label="Recording views" className="rail-nav">
          {VIEWS.map((item) => {
            const Icon = viewIcons[item.id];
            return (
              <button
                key={item.id}
                title={railCollapsed ? item.label : undefined}
                aria-label={item.label}
                aria-current={view === item.id ? "page" : undefined}
                onClick={() => changeView(item.id)}
                className="rail-link"
              >
                <Icon aria-hidden className="size-4" />
                <span className="rail-label">{item.label}</span>
                {item.id === "calls" && <span className="count">{metrics.calls}</span>}
              </button>
            );
          })}
        </nav>

        <div className="rail-recording" aria-hidden={railCollapsed}>
          <div className="mb-2 flex items-center gap-2 text-[#e0e8fa]">
            <Radio aria-hidden className="size-3.5" />
            {replay ? "Selected recording" : "Current recording"}
          </div>
          <p className="break-all font-mono text-[10px] text-[#b9c8e6]">{session?.id ?? "Waiting for connection"}</p>
          {session && <p className="mt-2 text-[10px]">Started {formatTime(session.startedAt)}</p>}
          {span !== undefined && <p className="mt-0.5 text-[10px]">{formatDuration(span)} captured span</p>}
          {!!session?.command?.length && (
            <p className="mt-2 break-words text-[10px]" title={session.command.join(" ")}>
              {session.command.join(" ")}
            </p>
          )}
        </div>

        <div className="rail-foot">
          <ConnectionBadge state={connection} replay={replay} />
          <div className="rail-storage mt-3 flex items-center gap-2 border-t border-[#354461] pt-3 text-[11px]">
            <HardDrive aria-hidden className="size-3.5" />
            Stored locally
          </div>
          <p className="rail-observation mt-1 text-[10px]">
            Observation only
            {session?.capture && (
              <span className="mt-1 block">
                {session.capture.profile === "full" ? "Full · no redaction" : "Standard · redacted"}
                <br />
                {formatBytes(session.capture.maxPayloadBytes)} per payload
              </span>
            )}
          </p>
        </div>
      </aside>

      {/* Main Workspace */}
      <div className="app-workspace flex min-w-0 flex-1 flex-col overflow-hidden">
        {/* Workspace Header */}
        <header className="workspace-header">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-xl font-bold tracking-tight text-ink">
                {VIEWS.find((item) => item.id === view)?.label}
              </h1>
              {!analyticsView && (
                <button
                  onClick={copySessionId}
                  className="recording-id group flex items-center gap-1.5 hover:bg-paper cursor-pointer transition-colors"
                  title="Click to copy Session ID"
                >
                  <span>{session?.id ?? "Connecting"}</span>
                  {copiedSessionId ? (
                    <Check aria-hidden className="size-3 text-ok" />
                  ) : (
                    <Copy aria-hidden className="size-3 text-ink-3 group-hover:text-ink" />
                  )}
                </button>
              )}
            </div>
            <p className="workspace-caption text-xs text-ink-2">{subtitle[view]}</p>
          </div>

          {!analyticsView && scopes.some((item) => item.id !== "unattributed") && (
            <select
              aria-label="Filter agent session or connection"
              title={scopes.find((item) => item.id === scope)?.label ?? "All captured sessions and connections"}
              value={scope}
              onChange={(event) => {
                resetPage();
                setScope(event.target.value);
                setDetailOpen(false);
                setSelectedKey(null);
                setQuery("");
                setTool("");
                setFilter("all");
              }}
              className="field-input max-w-64 min-w-0 flex-1 sm:flex-none"
            >
              <option value="">All sessions & connections</option>
              {scopes.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.label}
                </option>
              ))}
            </select>
          )}

          <div className="flex items-center gap-2">
            {!analyticsView && recordingId && (
              <button
                className="action-button"
                onClick={() => {
                  setRecordingId("");
                  setScope("");
                  resetPage();
                  setSelectedKey(null);
                  setDetailOpen(false);
                }}
              >
                Current recording
              </button>
            )}
            {!analyticsView && (
              <button
                disabled={!data}
                onClick={() => setIsCommandPaletteOpen(true)}
                aria-label="Search events (⌘K)"
                title="Search events (⌘K)"
                className="icon-button"
              >
                <Search aria-hidden className="size-4" />
              </button>
            )}
            <button
              onClick={() => setIsShortcutsModalOpen(true)}
              aria-label="Keyboard shortcuts (?)"
              title="Keyboard shortcuts (?)"
              className="icon-button hidden sm:inline-flex"
            >
              <Keyboard aria-hidden className="size-4" />
            </button>
            <button
              onClick={toggleTheme}
              aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
              title={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
              className="icon-button"
            >
              {theme === "dark" ? <Sun aria-hidden className="size-4" /> : <Moon aria-hidden className="size-4" />}
            </button>
            {!analyticsView && (
              <button
                disabled={!session}
                onClick={() => setIsExportModalOpen(true)}
                aria-label="Export session report"
                className="action-button font-medium"
              >
                <Download aria-hidden className="size-3.5" />
                <span className="hidden sm:inline">Export report</span>
              </button>
            )}
          </div>
        </header>

        {/* Global Error Banner */}
        {!analyticsView && error && (
          <div role="alert" className="flex flex-wrap items-center gap-3 border-b border-rule bg-paper px-5 py-3 text-xs text-warn">
            Could not update the profiler: {error}
            <button className="action-button" onClick={() => { resetPage(); retry(); }}>
              Refresh latest
            </button>
          </div>
        )}

        {/* Measurements Bar */}
        {!analyticsView && <MetricsBar metrics={metrics} onErrors={() => openCalls("errors")} />}
        {!analyticsView && session?.recoveredTail && (
          <p className="analytics-alert m-3">
            Recovered an incomplete final JSONL line. This recording may be missing events.
          </p>
        )}

        {/* Analytics view container */}
        <Analytics active={analyticsView} sessionsOnly={view === "sessions"} onOpen={openHistory} />

        {/* Main Calls / Timeline / Overview / Event Log */}
        {!analyticsView && (
          view === "overview" ? (
            <Overview
              profile={profile}
              replay={replay}
              connecting={!replay && (connection === "connecting" || connection === "reconnecting")}
              onCalls={() => openCalls()}
              onTool={(name) => openCalls("all", name)}
              onInspect={inspect}
            />
          ) : (
            <main
              className={`${view === "calls" ? "call-main " : ""}flex min-h-0 flex-1 flex-col`}
              aria-label={view === "calls" ? "Call inspector" : "Event log"}
            >
              <div className="inspector-workspace flex flex-1 flex-col overflow-hidden">
                {/* 1. Combined Interactive Toolbar */}
                <div className={`inspector-toolbar flex flex-wrap items-center justify-between gap-2.5 border-b border-rule px-4 py-2.5 bg-sheet ${detailOpen ? "hidden lg:flex" : "flex"}`}>
                  <div className="flex flex-wrap items-center gap-2 flex-1 min-w-0">
                    <label className="relative min-w-44 flex-1 sm:max-w-xs">
                      <Search aria-hidden className="pointer-events-none absolute left-2.5 top-2.5 size-3.5 text-ink-3" />
                      <input
                        aria-label={view === "calls" ? "Filter calls by tool or arguments" : "Filter captured events"}
                        value={query}
                        onChange={(event) => {
                          resetPage();
                          setQuery(event.target.value);
                          setDetailOpen(false);
                        }}
                        placeholder={view === "calls" ? "Search calls or arguments…" : "Search events…"}
                        className="field-input w-full pl-8 text-xs"
                      />
                    </label>

                    {view === "calls" && (
                      <>
                        <select
                          aria-label="Filter by tool"
                          value={tool}
                          onChange={(event) => {
                            resetPage();
                            setTool(event.target.value);
                            setDetailOpen(false);
                          }}
                          className="field-input max-w-36 text-xs"
                        >
                          <option value="">All tools</option>
                          {profile.tools.map(({ name }) => (
                            <option key={name} value={name}>
                              {name}
                            </option>
                          ))}
                        </select>

                        <select
                          aria-label="Sort calls"
                          value={sort}
                          onChange={(event) => {
                            resetPage();
                            setSort(event.target.value as CallSort);
                            setSelectedKey(null);
                          }}
                          className="field-input max-w-40 text-xs"
                        >
                          <option value="time">Capture order</option>
                          <option value="duration">Longest first</option>
                          <option value="response">Largest response</option>
                        </select>

                        {/* Status Filter Chips */}
                        <div role="group" aria-label="Call status filters" className="call-status-filters flex items-center gap-1">
                          {FILTERS.map((item) => (
                            <button
                              key={item.id}
                              aria-pressed={filter === item.id}
                              onClick={() => {
                                resetPage();
                                setFilter(item.id);
                                setDetailOpen(false);
                              }}
                              className="status-filter"
                            >
                              {item.label}
                              {item.id === "errors" && metrics.errors > 0 && (
                                <span className="ml-1 rounded px-1 text-[10px] bg-warn/15 text-warn font-semibold">
                                  {metrics.errors}
                                </span>
                              )}
                            </button>
                          ))}
                          {tool && (
                            <button
                              onClick={() => {
                                resetPage();
                                setTool("");
                              }}
                              className="status-filter inline-flex items-center gap-1 font-medium text-accent"
                              aria-label={`Clear ${tool} filter`}
                            >
                              <span className="max-w-24 truncate">{tool}</span>
                              <X aria-hidden className="size-3" />
                            </button>
                          )}
                        </div>
                      </>
                    )}
                  </div>

                  {/* Total entries counter */}
                  <div className="flex items-center gap-1.5 text-[11px] text-ink-2 shrink-0">
                    <span className="font-semibold text-ink fig">{data?.total ?? 0}</span>
                    <span>{view === "calls" ? "calls" : "entries"}</span>
                  </div>
                </div>

                {/* 2. Tool Series Quick Filter Ribbon (When in calls view) */}
                {view === "calls" && profile.tools.length > 1 && (
                  <div className={`flex items-center gap-1.5 overflow-x-auto border-b border-rule bg-paper/50 px-4 py-1.5 text-xs ${detailOpen ? "hidden lg:flex" : "flex"}`}>
                    <span className="text-[10px] uppercase font-semibold tracking-wider text-ink-3 shrink-0 mr-1">
                      Tools:
                    </span>
                    {profile.tools.map(({ name, metrics: m }) => {
                      const isSelected = tool === name;
                      const seriesColor = toolColor(name);
                      return (
                        <button
                          key={name}
                          onClick={() => {
                            resetPage();
                            setTool(isSelected ? "" : name);
                            setDetailOpen(false);
                          }}
                          className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] transition-colors shrink-0 ${
                            isSelected
                              ? "border-accent bg-accent/10 font-semibold text-accent"
                              : "border-rule bg-sheet text-ink-2 hover:bg-paper hover:text-ink"
                          }`}
                        >
                          <i aria-hidden className="size-1.5 rounded-full" style={{ background: seriesColor }} />
                          <span>{name}</span>
                          <span className="text-[10px] text-ink-3">({m.calls})</span>
                        </button>
                      );
                    })}
                  </div>
                )}

                {/* 3. The Main Split View */}
                <div
                  className={`inspector-split grid flex-1 min-h-0 overflow-hidden ${
                    detailExpanded
                      ? "grid-cols-1 lg:grid-cols-[minmax(0,1fr)_minmax(520px,1.4fr)]"
                      : "grid-cols-1 lg:grid-cols-[minmax(0,1.2fr)_minmax(360px,0.8fr)]"
                  }`}
                  aria-busy={loading}
                >
                  {/* Left Pane: Waterfall / Event Log */}
                  <div className={`event-list-pane min-h-0 flex-col lg:border-r lg:border-rule ${detailOpen ? "hidden lg:flex" : "flex"}`}>
                    {view === "calls" ? (
                      <TraceMap
                        rows={visible}
                        timeline={data?.timeline}
                        selectedKey={selectedKey}
                        onSelect={(row) => selectCall(row.key)}
                        loading={loading}
                        emptyTitle={metrics.calls === 0 ? "Your first call starts here" : "No matching calls"}
                        emptyDescription={
                          metrics.calls === 0
                            ? "Connect your agent with Sentinel hooks or the MCP proxy to capture calls."
                            : "Change the status, tool or search filters to inspect other calls."
                        }
                      />
                    ) : (
                      <Timeline
                        rows={visible}
                        selectedKey={selectedKey}
                        onSelect={selectCall}
                        follow={false}
                        emptyTitle="No matching events"
                        emptyDescription="Change the search or session filters to inspect other events."
                      />
                    )}

                    {/* Pagination Controls */}
                    <div className="page-controls flex items-center justify-between border-t border-rule bg-paper px-3 py-2 text-xs text-ink-2">
                      <span aria-live="polite">
                        {loading
                          ? "Loading page…"
                          : data?.total
                          ? `${data.offset + 1}–${data.offset + visible.length} of ${data.total}`
                          : "0 entries"}
                      </span>
                      <div className="flex items-center gap-1">
                        <button
                          className="icon-button size-7"
                          aria-label="Previous page"
                          disabled={loading || !data?.previous}
                          onClick={() => {
                            setFocusKey("");
                            setCursor(data?.previous ?? "");
                            setDetailOpen(false);
                          }}
                        >
                          <ChevronLeft aria-hidden className="size-3.5" />
                        </button>
                        <button
                          className="icon-button size-7"
                          aria-label="Next page"
                          disabled={loading || !data?.next}
                          onClick={() => {
                            setFocusKey("");
                            setCursor(data?.next ?? "");
                            setDetailOpen(false);
                          }}
                        >
                          <ChevronRight aria-hidden className="size-3.5" />
                        </button>
                      </div>
                      <button
                        className="text-accent hover:underline text-[11px]"
                        onClick={() => {
                          resetPage();
                          retry();
                        }}
                        title="Return to a fresh recording snapshot"
                      >
                        {cursor ? `Snapshot #${data?.revision} · Refresh` : "Refresh"}
                      </button>
                    </div>
                  </div>

                  {/* Right Pane: Detail Inspector */}
                  <div className={`call-detail-pane min-h-0 flex-col overflow-hidden bg-sheet ${detailOpen ? "flex" : "hidden lg:flex"}`}>
                    <div className="flex items-center justify-between border-b border-rule bg-paper px-4 py-2 text-xs text-ink-2 shrink-0">
                      <span className="hidden font-medium text-ink lg:inline">Evidence Inspector</span>
                      <button
                        onClick={() => setDetailOpen(false)}
                        className="flex items-center gap-1.5 font-medium text-accent lg:hidden py-1"
                      >
                        <ArrowLeft aria-hidden className="size-3.5" />
                        Back to {view === "calls" ? "timeline" : "event log"}
                      </button>
                      <div className="flex items-center gap-1.5">
                        <span className="mr-1 text-[11px] fig">
                          {selectedIndex >= 0 ? `${selectedIndex + 1} of ${visible.length}` : "No selection"}
                        </span>
                        <button
                          className="icon-button size-7"
                          aria-label="Inspect previous call"
                          title="Previous call"
                          disabled={selectedIndex <= 0}
                          onClick={() => selectCall(visible[selectedIndex - 1].key)}
                        >
                          <ChevronLeft aria-hidden className="size-3.5" />
                        </button>
                        <button
                          className="icon-button size-7"
                          aria-label="Inspect next call"
                          title="Next call"
                          disabled={selectedIndex < 0 || selectedIndex >= visible.length - 1}
                          onClick={() => selectCall(visible[selectedIndex + 1].key)}
                        >
                          <ChevronRight aria-hidden className="size-3.5" />
                        </button>
                        <button
                          className="icon-button size-7 hidden lg:inline-flex"
                          aria-label={detailExpanded ? "Restore balanced width" : "Expand inspector width"}
                          title={detailExpanded ? "Restore balanced width" : "Expand inspector width"}
                          onClick={() => setDetailExpanded(!detailExpanded)}
                        >
                          {detailExpanded ? (
                            <PanelRightClose aria-hidden className="size-3.5" />
                          ) : (
                            <PanelRightOpen aria-hidden className="size-3.5" />
                          )}
                        </button>
                      </div>
                    </div>

                    {selected && !detail.row ? (
                      <div className="flex-1 p-6 text-sm text-ink-2" role={detail.error ? "alert" : "status"}>
                        {detail.error ? (
                          <>
                            <p>{detail.error}</p>
                            <button className="action-button mt-3" onClick={detail.retry}>
                              Retry payload
                            </button>
                          </>
                        ) : (
                          <div className="space-y-3">
                            <p>Loading captured payloads…</p>
                            <div className="h-24 rounded-md bg-paper animate-pulse" />
                            <div className="h-32 rounded-md bg-paper animate-pulse" />
                          </div>
                        )}
                      </div>
                    ) : (
                      <Detail key={selected?.key ?? "empty"} row={selected ? detail.row : null} />
                    )}
                  </div>
                </div>
              </div>
            </main>
          )
        )}
      </div>

      {/* Global Modals */}
      <CommandPalette
        isOpen={isCommandPaletteOpen}
        onClose={() => setIsCommandPaletteOpen(false)}
        rows={visible}
        scope={scope}
        recordingId={session?.id ?? ""}
        onSelectRow={inspect}
        onExport={() => setIsExportModalOpen(true)}
      />
      <ExportModal
        isOpen={isExportModalOpen}
        onClose={() => setIsExportModalOpen(false)}
        session={session}
        revision={data?.revision}
        config={{ ...config, ...(replay ? { mode: "replay" as const } : {}), capture: session?.capture }}
      />
      <ShortcutsModal
        isOpen={isShortcutsModalOpen}
        onClose={() => setIsShortcutsModalOpen(false)}
      />
    </div>
  );
}

function ConnectionBadge({ state, replay }: { state: Connection; replay: boolean }) {
  const labels: Record<Connection, string> = {
    connecting: "Connecting",
    live: "Live capture",
    reconnecting: "Reconnecting",
    ended: "Capture ended",
  };
  return (
    <span role="status" className="capture-status inline-flex items-center gap-1.5 text-xs font-medium">
      <i
        aria-hidden
        className={`size-2 rounded-full ${!replay && state === "live" ? "bg-[#76cbb8] animate-pulse" : "bg-[#a6b5d1]"}`}
      />
      {replay ? "Replay" : labels[state]}
    </span>
  );
}
