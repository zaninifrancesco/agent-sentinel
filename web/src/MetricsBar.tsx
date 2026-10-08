import { useMemo } from "react";
import { DollarSign, Cpu, Gauge, Zap } from "lucide-react";
import type { Row } from "./types";

interface Props {
  rows: Row[];
}

export function MetricsBar({ rows }: Props) {
  const metrics = useMemo(() => {
    const timedRows = rows.filter((r) => r.durationMs !== undefined && r.durationMs > 0);
    const durations = timedRows.map((r) => r.durationMs!);

    // Latency calculations
    durations.sort((a, b) => a - b);
    const p50 = durations.length ? durations[Math.floor(durations.length * 0.5)] : 0;
    const p95 = durations.length ? durations[Math.floor(durations.length * 0.95)] : 0;

    // Approximate token & cost estimation (based on standard coding agent telemetry)
    const toolCount = rows.filter((r) => r.kind === "tool").length;
    const estimatedInputTokens = toolCount * 1250;
    const estimatedOutputTokens = toolCount * 320;
    const totalTokens = estimatedInputTokens + estimatedOutputTokens;

    // Stored cost model: ~$3/M in, $15/M out for high-tier models
    const estimatedCost = (estimatedInputTokens / 1_000_000) * 3.0 + (estimatedOutputTokens / 1_000_000) * 15.0;
    const budgetLimit = 2.0; // $2.00 threshold
    const budgetPercent = Math.min(100, Math.round((estimatedCost / budgetLimit) * 100));

    // Sparkline points
    const recent = timedRows.slice(-15).map((r) => r.durationMs!);
    const maxDur = Math.max(...recent, 100);
    const sparkPoints = recent.map((d, idx) => {
      const x = (idx / Math.max(1, recent.length - 1)) * 100;
      const y = 30 - (d / maxDur) * 26;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    }).join(" ");

    return {
      p50,
      p95,
      totalTokens,
      estimatedCost,
      budgetPercent,
      sparkPoints,
      hasRecent: recent.length > 1,
    };
  }, [rows]);

  return (
    <div className="flex items-center gap-6 border-b border-ink-800 bg-ink-950 px-5 py-2 text-xs">
      {/* Cost & Budget Meter */}
      <div className="flex items-center gap-2.5">
        <DollarSign className="size-3.5 text-emerald-400" />
        <div className="flex flex-col">
          <div className="flex items-center gap-1.5 font-mono text-[11px]">
            <span className="font-semibold text-ink-100">${metrics.estimatedCost.toFixed(3)}</span>
            <span className="text-ink-500">/ $2.00 max</span>
          </div>
          <div className="mt-0.5 h-1 w-24 overflow-hidden rounded-full bg-ink-800">
            <div
              className={`h-full transition-all duration-300 ${
                metrics.budgetPercent > 80 ? "bg-rose-500" : "bg-emerald-400"
              }`}
              style={{ width: `${Math.max(4, metrics.budgetPercent)}%` }}
            />
          </div>
        </div>
      </div>

      {/* Token Throughput */}
      <div className="flex items-center gap-2">
        <Cpu className="size-3.5 text-accent" />
        <span className="text-ink-400">Tokens:</span>
        <span className="font-mono text-ink-100">
          {metrics.totalTokens > 1000
            ? `${(metrics.totalTokens / 1000).toFixed(1)}k`
            : metrics.totalTokens}
        </span>
      </div>

      {/* Latency Percentiles */}
      <div className="flex items-center gap-2">
        <Gauge className="size-3.5 text-amber-400" />
        <span className="text-ink-400">p50:</span>
        <span className="font-mono text-ink-200">{metrics.p50}ms</span>
        <span className="text-ink-500">·</span>
        <span className="text-ink-400">p95:</span>
        <span className="font-mono text-ink-200">{metrics.p95}ms</span>
      </div>

      {/* Latency Sparkline */}
      {metrics.hasRecent && (
        <div className="ml-auto flex items-center gap-2">
          <span className="text-[11px] text-ink-500 flex items-center gap-1">
            <Zap className="size-3 text-accent" /> Latency trend:
          </span>
          <svg className="h-6 w-24 overflow-visible" viewBox="0 0 100 30">
            <polyline
              fill="none"
              stroke="#7aa2ff"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              points={metrics.sparkPoints}
            />
          </svg>
        </div>
      )}
    </div>
  );
}
