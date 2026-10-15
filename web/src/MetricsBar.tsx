import type { Metrics } from "./metrics";
import { PAYLOAD_NOTE } from "./metrics";

export function MetricsBar({ metrics: m }: { metrics: Metrics }) {
  return (
    <div className="flex flex-wrap items-center gap-x-7 gap-y-1 border-b border-rule-strong bg-paper px-4 py-2 text-[13px] sm:px-6">
      <Item label="Calls" value={m.calls} />
      <Item label="Running" value={m.running} />
      <Item label="Errors" value={m.errors} />
      {m.interrupted > 0 && <Item label="Interrupted" value={m.interrupted} />}
      {m.refused > 0 && <Item label="Historical refusals" value={m.refused} />}
      {m.waiting > 0 && <Item label="Historical holds" value={m.waiting} />}
      <span title={PAYLOAD_NOTE}>
        <Item label="Payload JSON" value={`${(m.payloadBytes / 1024).toFixed(1)} KiB`} />
      </span>
      <span className="sm:ml-auto" title={`${m.latencySamples} completed tool calls with a reported duration. Historical approval waits are excluded.`}>
        <Item label="Median / p95" value={m.latencySamples ? `${m.p50} / ${m.p95} ms` : "– / –"} />
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
