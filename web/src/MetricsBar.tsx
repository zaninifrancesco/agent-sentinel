import { useMemo } from "react";
import type { Row } from "./types";
import { computeMetrics, COST_NOTE } from "./metrics";

interface Props {
  rows: Row[];
  /** Session budget in USD from the running Sentinel; 0 = none. */
  maxCostUsd: number;
}

export function MetricsBar({ rows, maxCostUsd }: Props) {
  const m = useMemo(() => {
    const base = computeMetrics(rows);
    return { ...base, share: maxCostUsd > 0 ? Math.min(1, base.cost / maxCostUsd) : 0 };
  }, [rows, maxCostUsd]);

  const near = maxCostUsd > 0 && m.share > 0.8;

  return (
    <div className="flex flex-wrap items-center gap-x-7 gap-y-1 border-b border-rule-strong bg-paper px-6 py-2 text-[13px]">
      <Item label="Calls" value={m.calls} />
      <Item label="Waiting" value={m.waiting} />
      <Item label="Running" value={m.running} />
      <Item label="Refused" value={m.refused} alert={m.refused > 0} />
      <Item label="Errors" value={m.errors} />

      <div
        className="ml-auto flex items-center gap-2.5"
        title={COST_NOTE}
      >
        <span className="text-ink-2">Cost (estimate)</span>
        <span className="fig font-semibold text-ink">~${m.cost.toFixed(3)}</span>
        {maxCostUsd > 0 && (
          <>
            <span
              role="meter"
              aria-label="Estimated cost against budget"
              aria-valuemin={0}
              aria-valuemax={maxCostUsd}
              aria-valuenow={Number(m.cost.toFixed(3))}
              className="relative h-1.5 w-24 bg-rule"
            >
              <span
                className={`absolute inset-y-0 left-0 ${near ? "bg-signal" : "bg-ink"}`}
                style={{ width: `${Math.max(2, m.share * 100)}%` }}
              />
            </span>
            <span className={`fig ${near ? "font-semibold text-signal" : "text-ink-2"}`}>of ${maxCostUsd.toFixed(2)}</span>
          </>
        )}
      </div>
      <Item label="Tokens (est.)" value={m.tokens > 1000 ? `${(m.tokens / 1000).toFixed(1)}k` : m.tokens} />
      <Item label="Median / p95" value={`${m.p50} / ${m.p95} ms`} />
    </div>
  );
}

function Item({ label, value, alert }: { label: string; value: number | string; alert?: boolean }) {
  return (
    <div className="flex items-baseline gap-2">
      <span className="text-ink-2">{label}</span>
      <span className={`fig font-semibold ${alert ? "text-signal" : "text-ink"}`}>{value}</span>
    </div>
  );
}
