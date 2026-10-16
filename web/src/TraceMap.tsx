import { useState, type KeyboardEvent } from "react";
import { ChevronDown, ChevronUp, GitBranch } from "lucide-react";
import type { ProfilerTimeline, Row } from "./types";
import { toolColor } from "./trace";
import { formatDuration, formatTime } from "./format";

export function TraceMap({ rows, timeline, selectedKey, onSelect }: {
  rows: Row[]; timeline?: ProfilerTimeline; selectedKey: string | null; onSelect: (row: Row) => void;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const calls = rows.filter((row) => row.kind === "tool");
  const first = timeline?.windowStartMs ?? timeline?.startMs ?? 0;
  const span = (timeline?.windowEndMs ?? timeline?.endMs ?? first) - first;
  const position = (time: number) => span > 0 ? Math.max(0, Math.min(100, (time - first) / span * 100)) : 0;
  const navigate = (event: KeyboardEvent) => {
    if ((event.key !== "ArrowLeft" && event.key !== "ArrowRight") || !calls.length) return;
    event.preventDefault();
    const index = calls.findIndex((row) => row.key === selectedKey);
    onSelect(calls[Math.max(0, Math.min(calls.length - 1, index + (event.key === "ArrowRight" ? 1 : -1)))]);
  };
  return <section className="trace-map" aria-label="Call waterfall">
    <div className="map-heading">
      <div className="flex min-w-0 items-center gap-2"><GitBranch aria-hidden className="size-4 text-accent" /><h2>Call waterfall</h2><span className="hidden text-xs text-ink-2 sm:inline">One lane per call</span></div>
      <button className="icon-button" onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? "Expand waterfall" : "Collapse waterfall"} aria-expanded={!collapsed}>{collapsed ? <ChevronDown aria-hidden className="size-4" /> : <ChevronUp aria-hidden className="size-4" />}</button>
    </div>
    {!collapsed && (timeline?.startMs !== undefined ? <>
      <div className="waterfall-measures" title="All calls in the selected session scope; list filters and pagination do not change these totals. Coverage is the union of matched capture intervals, not measured execution time.">
        <span>Matched <strong>{formatDuration(timeline.coveredMs)}</strong></span>
        <span>Unattributed <strong>{formatDuration(timeline.unattributedMs)}</strong></span>
        <span>Peak <strong>{timeline.peakConcurrency} concurrent</strong></span>
      </div>
      <div className="map-axis"><span>Current page</span><div className="relative h-5"><span className="absolute left-0">{formatTime(new Date(first).toISOString())}</span><span className="absolute left-1/2 -translate-x-1/2">+{formatDuration(Math.round(span / 2))}</span><span className="absolute right-0">+{formatDuration(span)}</span></div></div>
      <div className="map-lane waterfall-gap"><span className="truncate" title="Time outside matched capture intervals; includes unknown and unobserved activity">Unattributed</span><div className="map-track">{timeline.gaps.map((gap, i) => <span key={i} className="gap-span" title={`Unattributed ${formatDuration(gap.endMs - gap.startMs)}`} style={{ left: `${position(gap.startMs)}%`, width: `${span ? (gap.endMs - gap.startMs) / span * 100 : 0}%` }} />)}</div></div>
      <div onKeyDown={navigate} className="waterfall-lanes">
        {calls.map((row) => {
          const active = row.key === selectedKey;
          const captured = row.span;
          const label = `${row.title}: ${row.subtitle ?? "call"}. ${captured?.endMs === undefined ? "Capture end unavailable" : `Captured interval ${formatDuration(captured.endMs - captured.startMs)}`}. ${row.status}`;
          return <div className={`map-lane waterfall-lane ${active ? "is-active" : ""}`} key={row.key}>
            <button onClick={() => onSelect(row)} aria-label={`Inspect ${row.title}: ${row.subtitle ?? "call"}`} className="waterfall-label" title={row.subtitle}><i aria-hidden className="series-dot" style={{ background: toolColor(row.title) }} /><span className="truncate">{row.title}</span></button>
            <div className="map-track">
              {[0, 25, 50, 75, 100].map((tick) => <span aria-hidden key={tick} className="map-gridline" style={{ left: `${tick}%` }} />)}
              {captured ? <button onClick={() => onSelect(row)} aria-label={label} aria-pressed={active} className={`map-span ${active ? "is-selected" : ""} ${row.status === "error" ? "is-error" : ""} ${captured.endMs === undefined ? "is-open" : ""}`} style={{ left: `min(${position(captured.startMs)}%, calc(100% - 7px))`, width: `${span && captured.endMs !== undefined ? (captured.endMs - captured.startMs) / span * 100 : 0}%`, background: row.status === "error" ? "var(--color-warn)" : toolColor(row.title) }} title={label} /> : <span className="text-[10px] text-ink-2">Timestamp unavailable</span>}
            </div>
          </div>;
        })}
        {!calls.length && <p className="px-3 py-2 text-xs text-ink-2">No calls on this page. Adjust the filters to inspect other evidence.</p>}
      </div>
      <div className="waterfall-caption">Capture intervals · {calls.length} calls on this page · {timeline.incompleteCalls} without a measured end{timeline.gapCount > timeline.gaps.length ? ` · first ${timeline.gaps.length} of ${timeline.gapCount} gaps drawn` : ""}<span>Unattributed time can include model, user, scheduling and missing evidence.</span></div>
    </> : <p className="px-4 pb-4 text-xs text-ink-2">The waterfall will appear with timestamped calls. Reported tool durations remain separate from captured intervals.</p>)}
  </section>;
}
