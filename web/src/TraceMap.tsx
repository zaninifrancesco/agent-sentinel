import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { GitBranch } from "lucide-react";
import type { ProfilerTimeline, Row } from "./types";
import { toolColor, waterfallInterval } from "./trace";
import { toolDuration } from "./metrics";
import { StatusIcon } from "./Timeline";
import { formatDuration, formatTime } from "./format";

export function TraceMap({ rows, timeline, selectedKey, onSelect, loading, emptyTitle, emptyDescription }: {
  rows: Row[]; timeline?: ProfilerTimeline; selectedKey: string | null; onSelect: (row: Row) => void;
  loading: boolean; emptyTitle: string; emptyDescription: string;
}) {
  const [zoom, setZoom] = useState(1);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const bars = useRef(new Map<string, HTMLSpanElement>());
  const calls = rows.filter((row) => row.kind === "tool");
  const first = timeline?.windowStartMs ?? timeline?.startMs;
  const last = timeline?.windowEndMs ?? timeline?.endMs ?? first;
  const span = first === undefined || last === undefined ? 0 : Math.max(0, last - first);
  const position = (time: number) => first !== undefined && span > 0 ? Math.max(0, Math.min(100, (time - first) / span * 100)) : 0;
  useEffect(() => {
    if (!selectedKey) return;
    (bars.current.get(selectedKey) ?? buttons.current.get(selectedKey))?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [selectedKey, zoom]);
  const navigate = (event: KeyboardEvent) => {
    if (!["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(event.key) || !calls.length) return;
    event.preventDefault();
    const index = calls.findIndex((row) => row.key === selectedKey);
    const step = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1 : -1;
    const row = calls[Math.max(0, Math.min(calls.length - 1, index + step))];
    onSelect(row);
    buttons.current.get(row.key)?.focus({ preventScroll: true });
  };
  return <section className="trace-map" aria-label="Call timeline">
    <div className="map-heading">
      <div className="flex min-w-0 items-center gap-2"><GitBranch aria-hidden className="size-4 text-accent" /><h2>Call timeline</h2></div>
      <label className="timeline-zoom">Zoom<select aria-label="Timeline zoom" value={zoom} onChange={(event) => setZoom(Number(event.target.value))}>{[1, 4, 16, 64].map((value) => <option key={value} value={value}>{value === 1 ? "Fit" : `${value}×`}</option>)}</select></label>
    </div>
    {timeline && <div className="waterfall-measures" title="Whole selected scope, independent of filters and page. Coverage is the union of captured intervals; tool duration is separate.">
      <span>Matched <strong>{formatDuration(timeline.coveredMs)}</strong></span>
      <span>Unattributed <strong>{formatDuration(timeline.unattributedMs)}</strong></span>
      <span>Peak <strong>{timeline.peakConcurrency} concurrent</strong></span>
    </div>}
    <div className="timeline-scroll" onKeyDown={navigate} tabIndex={0} role="region" aria-label="Scrollable call timeline" aria-busy={loading}>
      {loading ? <p className="waterfall-empty" role="status">Loading calls…</p> : calls.length === 0 ? <div className="waterfall-empty"><h3>{emptyTitle}</h3><p>{emptyDescription}</p></div> : <div className="timeline-canvas" style={{ width: `${zoom * 100}%` }}>
        <div className="waterfall-columns waterfall-column-head">
          <span className="waterfall-label">Tool / arguments</span>
          <div className="timeline-axis" aria-label="Captured time on this page">{first !== undefined ? <><span>{formatTime(new Date(first).toISOString())}</span>{[.5, 1].map((tick) => <span key={tick} style={{ left: `${tick * 100}%` }}>+{formatDuration(Math.round(span * tick))}</span>)}</> : <span>Captured time unavailable</span>}</div>
          <span className="waterfall-duration" title="Available tool duration, separate from the captured interval">Tool time</span>
        </div>
        {timeline && first !== undefined && <div className="waterfall-columns waterfall-gaps"><span className="waterfall-label">Unattributed</span><div className="timeline-gap-track" aria-label="Unattributed intervals">{timeline.gaps.map((gap, i) => <span key={i} className="gap-span" title={`Unattributed ${formatDuration(gap.endMs - gap.startMs)}`} style={{ left: `${position(gap.startMs)}%`, width: `${span ? (gap.endMs - gap.startMs) / span * 100 : 0}%` }} />)}</div><span className="waterfall-duration" /></div>}
        {calls.map((row) => {
          const captured = waterfallInterval(row);
          const duration = toolDuration(row);
          const point = captured?.end === undefined || captured.end === captured.start;
          const intervalText = !captured ? "Captured time unavailable" : captured.end === undefined ? "Capture end unavailable" : `Captured interval ${formatDuration(captured.end - captured.start)}`;
          const label = `${row.title}: ${row.subtitle ?? "call"}. ${intervalText}. Tool time ${duration === undefined ? "unavailable" : formatDuration(duration)}. ${row.status}`;
          return <button key={row.key} ref={(node) => { if (node) buttons.current.set(row.key, node); else buttons.current.delete(row.key); }} onClick={() => onSelect(row)} aria-label={label} aria-pressed={row.key === selectedKey} className={`waterfall-columns waterfall-call ${row.key === selectedKey ? "is-selected" : ""}`} style={{ "--call-color": toolColor(row.title) } as CSSProperties} title={label}>
            <span className="waterfall-label"><span className="waterfall-tool"><StatusIcon status={row.status} /><i aria-hidden className="series-dot" style={{ background: toolColor(row.title) }} /><strong>{row.title}</strong></span><span className="waterfall-arguments">{row.subtitle ?? "No argument preview"}</span></span>
            <span className="waterfall-track" aria-hidden>{[0, 50, 100].map((tick) => <span key={tick} className="map-gridline" style={{ left: `${tick}%` }} />)}{captured ? <span ref={(node) => { if (node) bars.current.set(row.key, node); else bars.current.delete(row.key); }} className={`waterfall-bar ${point ? "is-point" : ""} ${captured.end === undefined ? "is-open" : ""} ${row.status === "error" ? "is-error" : ""}`} style={{ left: `min(${position(captured.start)}%, calc(100% - ${point ? 8 : 2}px))`, width: `${!point ? Math.max(0, position(captured.end!) - position(captured.start)) : 0}%` }} /> : <span className="waterfall-unknown">Time unavailable</span>}</span>
            <span className="waterfall-duration">{duration === undefined ? <span aria-label="Tool duration unavailable">–</span> : formatDuration(duration)}</span>
          </button>;
        })}
      </div>}
    </div>
    <p className="waterfall-caption">Bars: captured intervals · Tool time: available durations. Hollow markers have no captured end.{timeline && timeline.gapCount > timeline.gaps.length ? ` First ${timeline.gaps.length} of ${timeline.gapCount} gaps shown.` : ""}</p>
  </section>;
}
