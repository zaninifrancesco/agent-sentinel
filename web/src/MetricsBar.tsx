import { useMemo } from "react";
import type { Row } from "./types";

interface Props {
  rows: Row[];
  /** Session budget in USD from the running Sentinel; 0 = none. */
  maxCostUsd: number;
}

export function MetricsBar({ rows, maxCostUsd }: Props) {
  const m = useMemo(() => {
    const tools = rows.filter((r) => r.kind === "tool");
    const durations = rows
      .filter((r) => r.durationMs !== undefined && r.durationMs > 0 && r.status !== "rejected")
      .map((r) => r.durationMs!)
      .sort((a, b) => a - b);
    const at = (q: number) => (durations.length ? durations[Math.min(durations.length - 1, Math.floor(durations.length * q))] : 0);

    // Token & cost ESTIMATE from the real size of what crossed the proxy
    // (~4 characters per token). The proxy cannot see the LLM API traffic, so
    // this only covers tool traffic and ignores the conversation context:
    //  - call arguments were written by the model  -> output tokens
    //  - tool results are fed back to the model    -> input tokens
    const approx = (payload: unknown) => (payload === undefined ? 0 : Math.ceil(JSON.stringify(payload).length / 4));
    let tin = 0;
    let tout = 0;
    for (const r of tools) {
      tout += approx(r.request?.payload);
      tin += approx(r.response?.payload);
    }
    const cost = (tin / 1_000_000) * 3 + (tout / 1_000_000) * 15;

    return {
      calls: tools.length,
      waiting: rows.filter((r) => r.status === "awaiting_approval").length,
      running: rows.filter((r) => r.status === "pending").length,
      refused: rows.filter((r) => r.status === "blocked" || r.status === "rejected").length,
      errors: rows.filter((r) => r.status === "error").length,
      tokens: tin + tout,
      cost,
      share: maxCostUsd > 0 ? Math.min(1, cost / maxCostUsd) : 0,
      p50: at(0.5),
      p95: at(0.95),
    };
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
        title="Estimated from the size of tool calls and results (~4 chars/token, $3/M in, $15/M out). Does not include the LLM conversation context."
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
