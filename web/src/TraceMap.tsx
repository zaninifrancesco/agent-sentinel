import { useMemo, useState, type KeyboardEvent } from "react";
import { ChevronDown, ChevronUp, GitBranch } from "lucide-react";
import type { Row } from "./types";
import { buildTrace, toolColor } from "./trace";
import { formatDuration, formatTime } from "./format";

export function TraceMap({ rows, selectedKey, onSelect }: { rows: Row[]; selectedKey: string | null; onSelect: (row: Row) => void }) {
  const trace = useMemo(() => buildTrace(rows), [rows]);
  const [collapsed, setCollapsed] = useState(false);
  const selected = trace.timed.find(({ row }) => row.key === selectedKey);
  const position = (time: number) => trace.span > 0 ? (time - trace.first) / trace.span * 100 : 0;
  const navigate = (event: KeyboardEvent) => {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    if (!trace.timed.length) return;
    event.preventDefault();
    const index = trace.timed.findIndex(({ row }) => row.key === selectedKey);
    const next = trace.timed[Math.max(0, Math.min(trace.timed.length - 1, index + (event.key === "ArrowRight" ? 1 : -1)))];
    onSelect(next.row);
  };
  return <section className="trace-map" aria-label="Recording map">
    <div className="map-heading">
      <div className="flex min-w-0 items-center gap-2"><GitBranch aria-hidden className="size-4 text-accent" /><h2>Recording map</h2><span className="hidden text-xs text-ink-2 sm:inline">Captured request → response</span></div>
      <button className="icon-button" onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? "Expand recording map" : "Collapse recording map"} aria-expanded={!collapsed}>{collapsed ? <ChevronDown aria-hidden className="size-4" /> : <ChevronUp aria-hidden className="size-4" />}</button>
    </div>
    {!collapsed && (trace.timed.length ? <>
      <div className="map-axis"><span>Tool</span><div className="relative h-5"><span className="absolute left-0">{formatTime(new Date(trace.first).toISOString())}</span><span className="absolute left-1/2 -translate-x-1/2">+{formatDuration(Math.round(trace.span / 2))}</span><span className="absolute right-0">+{formatDuration(trace.span)}</span></div></div>
      <div onKeyDown={navigate} className="map-lanes">
        {trace.lanes.map(({ name, entries }) => <div className="map-lane" key={name}>
          <span className="flex min-w-0 items-center gap-2"><i className="series-dot" style={{ background: toolColor(name) }} /><span className="truncate" title={name}>{name}</span><span className="ml-auto text-ink-3">{entries.length}</span></span>
          <div className="map-track">
            {[0, 25, 50, 75, 100].map((tick) => <span aria-hidden key={tick} className="map-gridline" style={{ left: `${tick}%` }} />)}
            {selected && <span aria-hidden className="map-cursor" style={{ left: `${position(selected.start)}%` }} />}
            {entries.map(({ row, start, end }) => {
              const active = row.key === selectedKey;
              const label = `${row.title}: ${row.subtitle ?? "call"}. ${end === undefined ? "No valid captured response time" : `Captured interval ${formatDuration(end - start)}`}. ${row.status}`;
              return <button key={row.key} onClick={() => onSelect(row)} aria-label={label} aria-pressed={active} tabIndex={active || (!selected && row.key === trace.timed[0]?.row.key) ? 0 : -1}
                className={`map-span ${active ? "is-selected" : ""} ${row.status === "error" ? "is-error" : ""} ${end === undefined ? "is-open" : ""}`}
                style={{ left: `min(${position(start)}%, calc(100% - 7px))`, width: `${trace.span && end !== undefined ? (end - start) / trace.span * 100 : 0}%`, background: row.status === "error" ? "var(--color-warn)" : toolColor(name) }} title={label} />;
            })}
          </div>
        </div>)}
      </div>
      <div className="map-caption"><span>Intervals reflect capture timing, not model thinking or reported tool duration.</span><span>{trace.omitted > 0 ? `Latest ${trace.timed.length} calls · ${trace.omitted} earlier in the list` : `${trace.timed.length} calls mapped`}{trace.invalid > 0 ? ` · ${trace.invalid} without valid timestamps` : ""}</span></div>
    </> : <p className="px-5 pb-5 text-xs text-ink-2">The time map will appear with timestamped tool calls. Your recording stays local.</p>)}
  </section>;
}
