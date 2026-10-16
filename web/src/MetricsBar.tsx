import type { Metrics } from "./metrics";
import { PAYLOAD_NOTE } from "./metrics";
import { formatBytes, formatDuration } from "./format";
import { AlertTriangle, Activity, Database, Clock } from "lucide-react";

export function MetricsBar({ metrics: m, onErrors }: { metrics: Metrics; onErrors: () => void }) {
  return (
    <div className="measurements flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-rule bg-sheet px-6 py-2.5 text-xs" aria-label="Recording measurements">
      <div className="measurement flex items-center gap-1.5">
        <Activity aria-hidden className="size-3.5 text-ink-3" />
        <span className="text-ink-2">Calls</span>
        <strong className="font-semibold text-ink font-mono text-[13px]">{m.calls}</strong>
      </div>

      {m.running > 0 && (
        <div className="measurement flex items-center gap-1.5">
          <span className="size-2 rounded-full bg-accent animate-pulse" />
          <span className="text-ink-2">Open</span>
          <strong className="font-semibold text-accent font-mono text-[13px]">{m.running}</strong>
        </div>
      )}

      <button
        type="button"
        className={`measurement flex items-center gap-1.5 rounded px-1.5 py-0.5 transition-colors ${
          m.errors > 0
            ? "bg-warn/10 text-warn font-semibold hover:bg-warn/20 cursor-pointer"
            : "text-ink-2 hover:bg-paper cursor-pointer"
        }`}
        onClick={onErrors}
        aria-label={`Inspect ${m.errors} reported tool errors`}
        title={m.errors > 0 ? "Click to filter to error calls" : "No reported errors"}
      >
        {m.errors > 0 && <AlertTriangle aria-hidden className="size-3.5 text-warn shrink-0" />}
        <span>Errors</span>
        <strong className={`font-mono text-[13px] ${m.errors ? "text-warn" : "text-ink"}`}>
          {m.errors}
        </strong>
      </button>

      <div className="measurement flex items-center gap-1.5" title={PAYLOAD_NOTE}>
        <Database aria-hidden className="size-3.5 text-ink-3" />
        <span className="text-ink-2">Retained JSON</span>
        <strong className="font-semibold text-ink font-mono text-[13px]">{formatBytes(m.payloadBytes)}</strong>
      </div>

      <div
        className="measurement flex items-center gap-1.5 sm:ml-auto"
        title={`${m.latencySamples} completed calls with available tool durations; not capture intervals. Interrupted calls and historical holds excluded.`}
      >
        <Clock aria-hidden className="size-3.5 text-ink-3" />
        <span className="text-ink-2">Median / p95</span>
        <strong className="font-semibold text-ink font-mono text-[13px]">
          {m.latencySamples ? `${formatDuration(m.p50)} / ${formatDuration(m.p95)}` : "– / –"}
        </strong>
        {m.latencySamples > 0 && (
          <span className="rounded bg-paper px-1 py-0.2 font-mono text-[10px] text-ink-3">
            n={m.latencySamples}
          </span>
        )}
      </div>
    </div>
  );
}
