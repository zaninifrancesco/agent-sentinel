import { SlidersHorizontal } from "lucide-react";
import type { Metrics } from "./metrics";
import { COST_NOTE } from "./metrics";
import { formatTokens } from "./format";

interface Props {
  metrics: Metrics;
  /** Session limits in force; 0 = none. */
  maxCostUsd: number;
  maxTokens: number;
  /** Can the limits be changed from here? */
  editable: boolean;
  onEditBudget: () => void;
}

export function MetricsBar({ metrics: m, maxCostUsd, maxTokens, editable, onEditBudget }: Props) {
  const costShare = maxCostUsd > 0 ? Math.min(1, m.cost / maxCostUsd) : 0;
  const tokenShare = maxTokens > 0 ? Math.min(1, m.tokens / maxTokens) : 0;
  // The meter follows whichever limit is closer.
  const share = Math.max(costShare, tokenShare);
  const limited = maxCostUsd > 0 || maxTokens > 0;
  const near = limited && share > 0.8;

  return (
    <div className="flex flex-wrap items-center gap-x-7 gap-y-1 border-b border-rule-strong bg-paper px-6 py-2 text-[13px]">
      <Item label="Calls" value={m.calls} />
      <Item label="Waiting" value={m.waiting} />
      <Item label="Running" value={m.running} />
      <Item label="Refused" value={m.refused} alert={m.refused > 0} />
      <Item label="Errors" value={m.errors} />

      <div className="ml-auto flex flex-wrap items-center gap-x-5 gap-y-1" title={COST_NOTE}>
        <div className="flex items-baseline gap-2">
          <span className="text-ink-2">Cost (estimate)</span>
          <span className="fig font-semibold text-ink">~${m.cost.toFixed(3)}</span>
          <span className={`fig ${costShare > 0.8 ? "font-semibold text-signal" : "text-ink-2"}`}>
            {maxCostUsd > 0 ? `of $${maxCostUsd.toFixed(2)}` : "no limit"}
          </span>
        </div>

        <div className="flex items-baseline gap-2">
          <span className="text-ink-2">Tokens (est.)</span>
          <span className="fig font-semibold text-ink">{formatTokens(m.tokens)}</span>
          <span className={`fig ${tokenShare > 0.8 ? "font-semibold text-signal" : "text-ink-2"}`}>
            {maxTokens > 0 ? `of ${formatTokens(maxTokens)}` : "no limit"}
          </span>
        </div>

        {limited && (
          <span
            role="meter"
            aria-label="Estimated spend against the closest limit"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(share * 100)}
            className="relative h-1.5 w-24 bg-rule"
          >
            <span
              className={`absolute inset-y-0 left-0 ${near ? "bg-signal" : "bg-ink"}`}
              style={{ width: `${Math.max(2, share * 100)}%` }}
            />
          </span>
        )}

        {editable && (
          <button
            onClick={onEditBudget}
            className="flex items-center gap-1.5 rounded-sm border border-rule-strong bg-sheet px-2 py-0.5 text-xs font-medium text-ink hover:border-ink"
          >
            <SlidersHorizontal className="size-3.5" />
            Limits
          </button>
        )}
      </div>

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
