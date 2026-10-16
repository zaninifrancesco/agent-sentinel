import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { ChevronDown, ChevronUp, GitBranch } from "lucide-react";
import type { ProfilerTimeline, Row } from "./types";
import { timelineTracks, toolColor } from "./trace";
import { formatDuration, formatTime } from "./format";

export function TraceMap({ rows, timeline, selectedKey, onSelect }: {
  rows: Row[]; timeline?: ProfilerTimeline; selectedKey: string | null; onSelect: (row: Row) => void;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const [zoom, setZoom] = useState(1);
  const [hovered, setHovered] = useState<Row | null>(null);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const { tracks, unpositioned } = timelineTracks(rows);
  const calls = tracks.flat().sort((a, b) => a.span!.startMs - b.span!.startMs);
  const tools = [...new Set(calls.map((row) => row.title))];
  const first = timeline?.windowStartMs ?? timeline?.startMs ?? 0;
  const span = (timeline?.windowEndMs ?? timeline?.endMs ?? first) - first;
  const position = (time: number) => span > 0 ? Math.max(0, Math.min(100, (time - first) / span * 100)) : 0;
  const preview = hovered ?? calls.find((row) => row.key === selectedKey);
  useEffect(() => {
    if (selectedKey && !collapsed) buttons.current.get(selectedKey)?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [selectedKey, zoom, collapsed]);
  const navigate = (event: KeyboardEvent) => {
    if ((event.key !== "ArrowLeft" && event.key !== "ArrowRight") || !calls.length) return;
    event.preventDefault();
    const index = calls.findIndex((row) => row.key === selectedKey);
    const row = calls[Math.max(0, Math.min(calls.length - 1, index + (event.key === "ArrowRight" ? 1 : -1)))];
    onSelect(row);
    buttons.current.get(row.key)?.focus();
  };
  return <section className="trace-map" aria-label="Call timeline">
    <div className="map-heading">
      <div className="flex min-w-0 items-center gap-2"><GitBranch aria-hidden className="size-4 text-accent" /><h2>Call timeline</h2></div>
      <div className="flex items-center gap-2">
        {!collapsed && <label className="timeline-zoom">Zoom<select aria-label="Timeline zoom" value={zoom} onChange={(event) => setZoom(Number(event.target.value))}>{[1, 4, 16, 64].map((value) => <option key={value} value={value}>{value === 1 ? "Fit" : `${value}×`}</option>)}</select></label>}
        <button className="icon-button" onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? "Expand timeline" : "Collapse timeline"} aria-expanded={!collapsed}>{collapsed ? <ChevronDown aria-hidden className="size-4" /> : <ChevronUp aria-hidden className="size-4" />}</button>
      </div>
    </div>
    {!collapsed && (timeline?.startMs !== undefined ? <>
      <div className="waterfall-measures" title="All calls in the selected session scope; list filters and pagination do not change these totals. Coverage is the union of matched capture intervals, not measured execution time.">
        <span>Matched <strong>{formatDuration(timeline.coveredMs)}</strong></span>
        <span>Unattributed <strong>{formatDuration(timeline.unattributedMs)}</strong></span>
        <span>Peak <strong>{timeline.peakConcurrency} concurrent</strong></span>
      </div>
      <div className="timeline-legend" aria-label="Tool colours">{tools.map((tool) => <span key={tool}><i aria-hidden className="series-dot" style={{ background: toolColor(tool) }} />{tool}</span>)}<span className="timeline-help">Overlapping calls use extra tracks</span></div>
      <div className="timeline-scroll" onKeyDown={navigate} tabIndex={0} role="region" aria-label="Scrollable call timeline">
        <div className="timeline-canvas" style={{ width: `${zoom * 100}%` }}>
          <div className="timeline-axis"><span>{formatTime(new Date(first).toISOString())}</span>{[.25, .5, .75, 1].map((tick) => <span key={tick} style={{ left: `${tick * 100}%` }}>+{formatDuration(Math.round(span * tick))}</span>)}</div>
          <div className="timeline-gap-track" aria-label="Unattributed intervals">{timeline.gaps.map((gap, i) => <span key={i} className="gap-span" title={`Unattributed ${formatDuration(gap.endMs - gap.startMs)}`} style={{ left: `${position(gap.startMs)}%`, width: `${span ? (gap.endMs - gap.startMs) / span * 100 : 0}%` }} />)}</div>
          {tracks.map((track, index) => <div className="timeline-track" key={index} aria-label={`Track ${index + 1}`}>
            {[0, 25, 50, 75, 100].map((tick) => <span aria-hidden key={tick} className="map-gridline" style={{ left: `${tick}%` }} />)}
            {track.map((row) => {
              const active = row.key === selectedKey;
              const captured = row.span!;
              const point = captured.endMs === undefined || captured.endMs <= captured.startMs;
              const label = `${row.title}: ${row.subtitle ?? "call"}. ${captured.endMs === undefined ? "Capture end unavailable" : `Captured interval ${formatDuration(Math.max(0, captured.endMs - captured.startMs))}`}. ${row.status}`;
              return <button key={row.key} ref={(node) => { if (node) buttons.current.set(row.key, node); else buttons.current.delete(row.key); }} onClick={() => onSelect(row)} onMouseEnter={() => setHovered(row)} onMouseLeave={() => setHovered(null)} onFocus={() => setHovered(row)} onBlur={() => setHovered(null)} aria-label={label} aria-pressed={active} className={`timeline-call ${active ? "is-selected" : ""} ${row.status === "error" ? "is-error" : ""} ${point ? "is-point" : ""} ${captured.endMs === undefined ? "is-open" : ""}`} style={{ left: `min(${position(captured.startMs)}%, calc(100% - ${point ? 8 : 2}px))`, width: `${span && !point ? Math.max(0, position(captured.endMs!) - position(captured.startMs)) : 0}%`, "--call-color": toolColor(row.title) } as CSSProperties} title={label}><span>{row.title}</span><span className="timeline-call-summary">{row.subtitle}</span></button>;
            })}
          </div>)}
        </div>
      </div>
      {!calls.length && <p className="px-3 py-2 text-xs text-ink-2">No timestamped calls on this page. Adjust the filters to inspect other evidence.</p>}
      <div className="timeline-preview" aria-live="polite">{preview ? <><i aria-hidden className="series-dot" style={{ background: toolColor(preview.title) }} /><strong>{preview.title}</strong><span className="truncate">{preview.subtitle ?? "Call"}</span><span className="timeline-preview-duration">{preview.span?.endMs === undefined ? "End unavailable" : `${formatDuration(Math.max(0, preview.span.endMs - preview.span.startMs))} captured`}</span></> : <span>Select a call to inspect its arguments and result.</span>}</div>
      <div className="waterfall-caption">{calls.length} calls · {tracks.length} tracks on this page{unpositioned.length ? ` · ${unpositioned.length} without timestamps` : ""}<span>Hollow markers have no captured end. Grey strip: unattributed time.{timeline.gapCount > timeline.gaps.length ? ` First ${timeline.gaps.length} of ${timeline.gapCount} gaps shown.` : ""}</span></div>
    </> : <p className="px-4 pb-4 text-xs text-ink-2">The timeline will appear with timestamped calls. Reported tool durations remain separate from captured intervals.</p>)}
  </section>;
}
