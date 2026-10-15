import type { Metrics } from "./metrics";
import { PAYLOAD_NOTE } from "./metrics";
import { formatBytes, formatDuration } from "./format";

export function MetricsBar({ metrics: m, onErrors }: { metrics: Metrics; onErrors: () => void }) {
  return <div className="measurements" aria-label="Recording measurements">
    <span className="measurement"><span className="text-ink-2">Calls</span><strong>{m.calls}</strong></span>
    <span className="measurement"><span className="text-ink-2">Open</span><strong>{m.running}</strong></span>
    <button className="measurement hover:underline" onClick={onErrors} aria-label={`Inspect ${m.errors} reported tool errors`}><span className="text-ink-2">Errors</span><strong className={m.errors ? "text-warn" : ""}>{m.errors}</strong></button>
    <span className="measurement" title={PAYLOAD_NOTE}><span className="text-ink-2">JSON retained</span><strong>{formatBytes(m.payloadBytes)}</strong></span>
    <span className="measurement sm:ml-auto" title={`${m.latencySamples} completed calls with available tool durations; not capture intervals. Interrupted calls and historical holds excluded.`}><span className="text-ink-2">Median / p95</span><strong>{m.latencySamples ? `${formatDuration(m.p50)} / ${formatDuration(m.p95)}` : "– / –"}</strong><span className="hidden text-[10px] text-ink-3 xl:inline">n={m.latencySamples}</span></span>
  </div>;
}
