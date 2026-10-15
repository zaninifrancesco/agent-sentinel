import type { Metrics } from "./metrics";
import { PAYLOAD_NOTE } from "./metrics";
import { formatBytes, formatDuration } from "./format";

export function MetricsBar({ metrics: m, onErrors }: { metrics: Metrics; onErrors: () => void }) {
  return (
    <div className="flex flex-wrap items-center gap-x-7 gap-y-1 border-b border-rule-strong bg-paper px-4 py-2 text-[13px] sm:px-6">
      <Item label="Tool calls" value={m.calls} />
      <Item label="Open" value={m.running} />
      <button onClick={onErrors} className="py-1 text-left hover:underline" aria-label={`Inspect ${m.errors} reported tool errors`}><Item label="Errors" value={m.errors} /></button>
      {m.interrupted > 0 && <Item label="Interrupted" value={m.interrupted} />}
      {m.refused > 0 && <Item label="Historical refusals" value={m.refused} />}
      {m.waiting > 0 && <Item label="Historical holds" value={m.waiting} />}
      <span title={PAYLOAD_NOTE}>
        <Item label="JSON retained" value={formatBytes(m.payloadBytes)} />
      </span>
      <span className="sm:ml-auto" title={`${m.latencySamples} completed tool calls with a reported duration. Historical approval waits are excluded.`}>
        <Item label={`Median / p95 (${m.latencySamples} samples)`} value={m.latencySamples ? `${formatDuration(m.p50)} / ${formatDuration(m.p95)}` : "– / –"} />
      </span>
    </div>
  );
}

function Item({ label, value }: { label: string; value: number | string }) {
  return (
    <span className="inline-flex items-baseline gap-2">
      <span className="text-ink-2">{label}</span>
      <span className="fig font-semibold text-ink">{value}</span>
    </span>
  );
}
