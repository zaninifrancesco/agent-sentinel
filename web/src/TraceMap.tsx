import { useEffect, useRef, useState, useMemo, type CSSProperties, type KeyboardEvent } from "react";
import { 
  GitBranch, Terminal, FileText, FileEdit, Search, Cpu, ZoomIn, ZoomOut, 
  Clock, Zap, Layers, Activity, RotateCcw
} from "lucide-react";
import type { ProfilerTimeline, Row } from "./types";
import { toolColor, waterfallInterval } from "./trace";
import { toolDuration } from "./metrics";
import { StatusIcon } from "./Timeline";
import { formatDuration, formatTime } from "./format";

interface Props {
  rows: Row[];
  timeline?: ProfilerTimeline;
  selectedKey: string | null;
  onSelect: (row: Row) => void;
  loading: boolean;
  emptyTitle: string;
  emptyDescription: string;
}

function toolIcon(name: string) {
  const lower = name.toLowerCase();
  if (lower.includes("shell") || lower.includes("bash") || lower.includes("exec") || lower.includes("cmd")) {
    return Terminal;
  }
  if (lower.includes("read") || lower.includes("cat") || lower.includes("view")) {
    return FileText;
  }
  if (lower.includes("edit") || lower.includes("write") || lower.includes("patch") || lower.includes("save")) {
    return FileEdit;
  }
  if (lower.includes("search") || lower.includes("grep") || lower.includes("find")) {
    return Search;
  }
  return Cpu;
}

export function TraceMap({
  rows,
  timeline,
  selectedKey,
  onSelect,
  loading,
  emptyTitle,
  emptyDescription,
}: Props) {
  const [zoom, setZoom] = useState(1);
  const [timeMode, setTimeMode] = useState<"relative" | "clock">("relative");
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const bars = useRef(new Map<string, HTMLSpanElement>());
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const calls = useMemo(() => rows.filter((row) => row.kind === "tool"), [rows]);

  const first = timeline?.windowStartMs ?? timeline?.startMs;
  const last = timeline?.windowEndMs ?? timeline?.endMs ?? first;
  const span = first === undefined || last === undefined ? 0 : Math.max(0, last - first);

  const position = (time: number) =>
    first !== undefined && span > 0 ? Math.max(0, Math.min(100, ((time - first) / span) * 100)) : 0;

  useEffect(() => {
    if (!selectedKey) return;
    (bars.current.get(selectedKey) ?? buttons.current.get(selectedKey))?.scrollIntoView({
      block: "nearest",
      inline: "nearest",
    });
  }, [selectedKey, zoom]);

  const navigate = (event: KeyboardEvent) => {
    if (!["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "j", "k", "Home", "End"].includes(event.key) || !calls.length) return;
    event.preventDefault();
    const index = calls.findIndex((row) => row.key === selectedKey);
    let targetIndex = index;

    if (event.key === "ArrowDown" || event.key === "j") {
      targetIndex = Math.min(calls.length - 1, index + 1);
    } else if (event.key === "ArrowUp" || event.key === "k") {
      targetIndex = Math.max(0, index - 1);
    } else if (event.key === "Home") {
      targetIndex = 0;
    } else if (event.key === "End") {
      targetIndex = calls.length - 1;
    } else if (event.key === "ArrowRight") {
      // Zoom in or next call
      targetIndex = Math.min(calls.length - 1, index + 1);
    } else if (event.key === "ArrowLeft") {
      // Zoom out or prev call
      targetIndex = Math.max(0, index - 1);
    }

    const row = calls[targetIndex];
    if (row) {
      onSelect(row);
      buttons.current.get(row.key)?.focus({ preventScroll: true });
    }
  };

  const zoomLevels = [1, 2, 4, 16, 64];

  const handleZoomIn = () => {
    const idx = zoomLevels.indexOf(zoom);
    if (idx < zoomLevels.length - 1) setZoom(zoomLevels[idx + 1]);
    else setZoom(Math.min(64, zoom * 2));
  };

  const handleZoomOut = () => {
    const idx = zoomLevels.indexOf(zoom);
    if (idx > 0) setZoom(zoomLevels[idx - 1]);
    else setZoom(1);
  };

  // Ratio calculation
  const totalCoveredMs = timeline?.coveredMs ?? 0;
  const totalUnattributedMs = timeline?.unattributedMs ?? 0;
  const totalKnownSpan = totalCoveredMs + totalUnattributedMs;
  const coveredPercent = totalKnownSpan > 0 ? Math.round((totalCoveredMs / totalKnownSpan) * 100) : 0;
  const unattributedPercent = totalKnownSpan > 0 ? 100 - coveredPercent : 0;

  return (
    <section className="trace-map flex min-h-0 flex-1 flex-col overflow-hidden bg-sheet" aria-label="Call timeline">
      {/* 1. Header Toolbar */}
      <div className="map-heading flex flex-wrap items-center justify-between gap-3 border-b border-rule bg-sheet px-4 py-2.5">
        <div className="flex items-center gap-2">
          <GitBranch aria-hidden className="size-4 text-accent" />
          <h2 className="text-xs font-semibold text-ink">Call timeline</h2>
          <span className="rounded border border-rule bg-paper px-1.5 py-0.5 font-mono text-[10px] text-ink-2">
            {calls.length} {calls.length === 1 ? "call" : "calls"}
          </span>
        </div>

        <div className="flex items-center gap-3">
          {/* Time format switcher */}
          <div className="hidden items-center rounded-md border border-rule bg-paper p-0.5 sm:flex" role="radiogroup" aria-label="Time format">
            <button
              onClick={() => setTimeMode("relative")}
              aria-checked={timeMode === "relative"}
              role="radio"
              className={`rounded px-2 py-0.5 text-[10px] font-medium transition-colors ${
                timeMode === "relative" ? "bg-sheet font-semibold text-accent shadow-xs" : "text-ink-2 hover:text-ink"
              }`}
            >
              +Offset
            </button>
            <button
              onClick={() => setTimeMode("clock")}
              aria-checked={timeMode === "clock"}
              role="radio"
              className={`rounded px-2 py-0.5 text-[10px] font-medium transition-colors ${
                timeMode === "clock" ? "bg-sheet font-semibold text-accent shadow-xs" : "text-ink-2 hover:text-ink"
              }`}
            >
              Clock
            </button>
          </div>

          {/* Zoom controls */}
          <div className="flex items-center gap-1">
            <button
              onClick={handleZoomOut}
              disabled={zoom <= 1}
              aria-label="Zoom out timeline"
              title="Zoom out"
              className="icon-button size-7"
            >
              <ZoomOut aria-hidden className="size-3.5" />
            </button>
            <label className="timeline-zoom flex items-center gap-1.5 text-[11px] text-ink-2">
              <span className="sr-only">Zoom</span>
              <select
                aria-label="Timeline zoom"
                value={zoom}
                onChange={(event) => setZoom(Number(event.target.value))}
                className="rounded border border-rule-strong bg-sheet px-1.5 py-0.5 text-[11px] font-medium text-ink"
              >
                {zoomLevels.map((value) => (
                  <option key={value} value={value}>
                    {value === 1 ? "Fit (100%)" : `${value}×`}
                  </option>
                ))}
              </select>
            </label>
            <button
              onClick={handleZoomIn}
              disabled={zoom >= 64}
              aria-label="Zoom in timeline"
              title="Zoom in"
              className="icon-button size-7"
            >
              <ZoomIn aria-hidden className="size-3.5" />
            </button>
            {zoom > 1 && (
              <button
                onClick={() => setZoom(1)}
                aria-label="Reset zoom to fit"
                title="Reset zoom to fit"
                className="icon-button size-7"
              >
                <RotateCcw aria-hidden className="size-3" />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* 2. Interactive Timeline Minimap / Activity Ribbon */}
      {timeline && span > 0 && (
        <div
          className="waterfall-measures border-b border-rule bg-paper px-4 py-2"
          title="Whole selected scope, independent of filters and page. Coverage is the union of captured intervals; tool duration is separate."
        >
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-[11px]">
            <div className="flex items-center gap-3">
              <span className="flex items-center gap-1 text-ink-2">
                <Zap aria-hidden className="size-3 text-accent" />
                Matched <strong>{formatDuration(timeline.coveredMs)}</strong>
                <span className="text-[10px] text-ink-3">({coveredPercent}%)</span>
              </span>
              <span className="flex items-center gap-1 text-ink-2">
                <Clock aria-hidden className="size-3 text-ink-3" />
                Unattributed <strong>{formatDuration(timeline.unattributedMs)}</strong>
                <span className="text-[10px] text-ink-3">({unattributedPercent}%)</span>
              </span>
            </div>

            <div className="flex items-center gap-3">
              <span className="flex items-center gap-1 text-ink-2">
                <Layers aria-hidden className="size-3 text-ink-3" />
                Peak <strong>{timeline.peakConcurrency} concurrent</strong>
              </span>
              {span > 0 && (
                <span className="text-ink-3">
                  Total span: <strong className="text-ink">{formatDuration(span)}</strong>
                </span>
              )}
            </div>
          </div>

          {/* Minimap distribution track */}
          <div
            className="group relative mt-1.5 h-3 w-full cursor-pointer overflow-hidden rounded bg-rule transition-colors hover:bg-rule-strong"
            onClick={(e) => {
              if (!scrollContainerRef.current) return;
              const rect = e.currentTarget.getBoundingClientRect();
              const clickPercent = (e.clientX - rect.left) / rect.width;
              const scrollWidth = scrollContainerRef.current.scrollWidth - scrollContainerRef.current.clientWidth;
              scrollContainerRef.current.scrollLeft = clickPercent * scrollWidth;
            }}
            title="Click to jump to time in timeline"
          >
            {/* Unattributed gaps */}
            {timeline.gaps.map((gap, i) => (
              <span
                key={`gap-${i}`}
                className="absolute top-0 bottom-0 bg-[#dbe2ee] opacity-80 dark:bg-[#1f2b3e]"
                style={{
                  left: `${position(gap.startMs)}%`,
                  width: `${Math.max(1, ((gap.endMs - gap.startMs) / span) * 100)}%`,
                }}
              />
            ))}
            {/* Call marks */}
            {calls.map((row) => {
              const cap = waterfallInterval(row);
              if (!cap) return null;
              const pStart = position(cap.start);
              const pEnd = cap.end ? position(cap.end) : pStart;
              const pWidth = Math.max(1, pEnd - pStart);
              const isError = row.status === "error";
              const isSelected = row.key === selectedKey;
              return (
                <span
                  key={`minimap-${row.key}`}
                  className={`absolute top-0 bottom-0 rounded-xs transition-opacity ${
                    isSelected ? "ring-1 ring-accent z-10" : ""
                  }`}
                  style={{
                    left: `${pStart}%`,
                    width: `${pWidth}%`,
                    background: isError ? "var(--color-warn)" : toolColor(row.title),
                    opacity: isSelected ? 1 : 0.75,
                  }}
                />
              );
            })}
          </div>
        </div>
      )}

      {/* 3. The Scrollable Waterfall Canvas */}
      <div
        ref={scrollContainerRef}
        className="timeline-scroll relative flex-1 min-h-0 min-width-0 overflow-auto focus:outline-hidden"
        onKeyDown={navigate}
        tabIndex={0}
        role="region"
        aria-label="Scrollable call timeline"
        aria-busy={loading}
      >
        {loading ? (
          <div className="waterfall-empty p-8 text-center text-ink-2" role="status">
            <Activity aria-hidden className="mx-auto mb-2 size-5 animate-pulse text-accent" />
            <p>Loading calls…</p>
          </div>
        ) : calls.length === 0 ? (
          <div className="waterfall-empty p-8 text-center">
            <h3 className="text-sm font-semibold text-ink">{emptyTitle}</h3>
            <p className="mt-1 text-xs text-ink-2">{emptyDescription}</p>
          </div>
        ) : (
          <div className="timeline-canvas min-w-full" style={{ width: `${zoom * 100}%` }}>
            {/* Sticky Column Headers */}
            <div className="waterfall-columns waterfall-column-head sticky top-0 z-10 grid min-h-9 items-center border-b border-rule bg-paper text-[10px] text-ink-2 shadow-2xs">
              <span className="waterfall-label sticky left-0 z-20 block bg-inherit px-3 py-2 font-medium text-ink">
                Tool / arguments
              </span>
              <div className="timeline-axis relative mx-1.5 h-5 font-mono text-[10px] text-ink-2" aria-label="Captured time on this page">
                {first !== undefined ? (
                  <>
                    <span className="absolute top-1 left-0 font-medium">
                      {timeMode === "clock" ? formatTime(new Date(first).toISOString()) : "0 ms"}
                    </span>
                    {[0.25, 0.5, 0.75, 1].map((tick) => (
                      <span
                        key={tick}
                        className="absolute top-1 whitespace-nowrap"
                        style={{
                          left: `${tick * 100}%`,
                          transform: tick === 1 ? "translateX(-100%)" : "translateX(-50%)",
                        }}
                      >
                        {timeMode === "clock"
                          ? formatTime(new Date(first + Math.round(span * tick)).toISOString())
                          : `+${formatDuration(Math.round(span * tick))}`}
                      </span>
                    ))}
                  </>
                ) : (
                  <span>Captured time unavailable</span>
                )}
              </div>
              <span className="waterfall-duration sticky right-0 z-20 block bg-inherit px-3 py-2 text-right font-medium text-ink" title="Available tool duration, separate from the captured interval">
                Tool time
              </span>
            </div>

            {/* Unattributed Gaps Row */}
            {timeline && first !== undefined && timeline.gaps.length > 0 && (
              <div className="waterfall-columns waterfall-gaps grid min-h-6 items-center border-b border-rule bg-paper/60 text-[9px] text-ink-2">
                <span className="waterfall-label sticky left-0 z-2 block bg-inherit px-3 py-1 text-ink-3">
                  Unattributed
                </span>
                <div className="timeline-gap-track relative mx-1.5 h-2" aria-label="Unattributed intervals">
                  {timeline.gaps.map((gap, i) => (
                    <span
                      key={i}
                      className="gap-span absolute top-0.5 h-1 min-w-[2px] rounded-xs bg-rule-strong"
                      title={`Unattributed ${formatDuration(gap.endMs - gap.startMs)} (Agent thinking / model generation / prompt time)`}
                      style={{
                        left: `${position(gap.startMs)}%`,
                        width: `${span ? ((gap.endMs - gap.startMs) / span) * 100 : 0}%`,
                      }}
                    />
                  ))}
                </div>
                <span className="waterfall-duration sticky right-0 z-2 block bg-inherit px-3" />
              </div>
            )}

            {/* Call Rows */}
            {calls.map((row) => {
              const captured = waterfallInterval(row);
              const duration = toolDuration(row);
              const point = captured?.end === undefined || captured.end === captured.start;
              const intervalText = !captured
                ? "Captured time unavailable"
                : captured.end === undefined
                ? "Capture end unavailable"
                : `Captured interval ${formatDuration(captured.end - captured.start)}`;
              const label = `${row.title}: ${row.subtitle ?? "call"}. ${intervalText}. Tool time ${
                duration === undefined ? "unavailable" : formatDuration(duration)
              }. ${row.status}`;

              const Icon = toolIcon(row.title);
              const isSelected = row.key === selectedKey;
              const callToolColor = toolColor(row.title);

              return (
                <button
                  key={row.key}
                  ref={(node) => {
                    if (node) buttons.current.set(row.key, node);
                    else buttons.current.delete(row.key);
                  }}
                  onClick={() => onSelect(row)}
                  aria-label={label}
                  aria-pressed={isSelected}
                  className={`waterfall-columns waterfall-call group relative grid w-full min-h-[56px] items-center border-b border-rule bg-sheet text-left transition-colors hover:bg-paper/70 ${
                    isSelected ? "is-selected bg-yellow-soft/80" : ""
                  }`}
                  style={{ "--call-color": callToolColor } as CSSProperties}
                  title={label}
                >
                  {/* Left Column: Tool & Argument Preview */}
                  <span className="waterfall-label sticky left-0 z-2 block min-w-0 bg-inherit px-3 py-2 text-left">
                    <span className="waterfall-tool flex items-center gap-1.5 min-w-0">
                      <StatusIcon status={row.status} />
                      <i aria-hidden className="series-dot size-1.5 rounded-xs shrink-0" style={{ background: callToolColor }} />
                      <Icon aria-hidden className="size-3.5 shrink-0 text-ink-2" />
                      <strong className="truncate text-xs font-semibold text-ink">{row.title}</strong>
                      {row.request?.seq && (
                        <span className="ml-0.5 rounded px-1 py-0.2 text-[9px] font-mono text-ink-3">
                          #{row.request.seq}
                        </span>
                      )}
                    </span>
                    <span className="waterfall-arguments mt-1 block truncate font-mono text-[10px] text-ink-2">
                      {row.subtitle ?? "No argument preview"}
                    </span>
                  </span>

                  {/* Center Column: Proportional Waterfall Track */}
                  <span className="waterfall-track relative block self-stretch min-w-0 mx-1.5" aria-hidden>
                    {/* Vertical grid lines */}
                    {[0, 25, 50, 75, 100].map((tick) => (
                      <span
                        key={tick}
                        className="map-gridline pointer-events-none absolute top-0 bottom-0 w-px bg-rule/50"
                        style={{ left: `${tick}%` }}
                      />
                    ))}

                    {/* The Interval Bar */}
                    {captured ? (
                      <span
                        ref={(node) => {
                          if (node) bars.current.set(row.key, node);
                          else bars.current.delete(row.key);
                        }}
                        className={`waterfall-bar absolute top-[18px] min-w-[2px] h-[20px] rounded-[3px] border transition-shadow ${
                          point ? "is-point" : ""
                        } ${captured.end === undefined ? "is-open" : ""} ${
                          row.status === "error" ? "is-error" : ""
                        } ${isSelected ? "ring-2 ring-accent ring-offset-1" : "hover:brightness-95"}`}
                        style={{
                          left: `min(${position(captured.start)}%, calc(100% - ${point ? 8 : 2}px))`,
                          width: `${!point ? Math.max(0, position(captured.end!) - position(captured.start)) : 0}%`,
                          borderColor: row.status === "error" ? "var(--color-warn)" : callToolColor,
                          backgroundColor:
                            row.status === "error"
                              ? "rgba(189, 67, 86, 0.15)"
                              : `color-mix(in srgb, ${callToolColor} 25%, var(--color-sheet))`,
                        }}
                      >
                        {/* Inline duration badge for sufficiently wide bars */}
                        {!point && (position(captured.end!) - position(captured.start)) > 8 && duration !== undefined && (
                          <span className="absolute inset-0 flex items-center justify-center overflow-hidden px-1 text-[9px] font-mono font-medium text-ink truncate pointer-events-none">
                            {formatDuration(duration)}
                          </span>
                        )}
                      </span>
                    ) : (
                      <span className="waterfall-unknown absolute top-5 left-1 text-[10px] text-ink-3 whitespace-nowrap">
                        Time unavailable
                      </span>
                    )}
                  </span>

                  {/* Right Column: Sticky Tool Duration */}
                  <span className="waterfall-duration sticky right-0 z-2 block bg-inherit px-3 py-2 text-right font-mono text-[10px] tabular-nums text-ink-2 whitespace-nowrap">
                    {duration === undefined ? (
                      <span aria-label="Tool duration unavailable">–</span>
                    ) : (
                      <span className="font-medium text-ink">{formatDuration(duration)}</span>
                    )}
                  </span>
                </button>
              );
            })}
          </div>
        )}
      </div>

      {/* 4. Footer Caption */}
      <div className="waterfall-caption flex flex-wrap items-center justify-between gap-2 border-t border-rule bg-paper px-3 py-1.5 text-[10px] text-ink-2">
        <p className="m-0">
          Bars: captured intervals · Tool time: available durations. Hollow markers have no captured end.
          {timeline && timeline.gapCount > timeline.gaps.length ? ` First ${timeline.gaps.length} of ${timeline.gapCount} gaps shown.` : ""}
        </p>
        <span className="hidden text-ink-3 md:inline">Use ↑ ↓ or J K to navigate calls</span>
      </div>
    </section>
  );
}
