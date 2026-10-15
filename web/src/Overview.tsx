import { ArrowUpRight, ArrowRight } from "lucide-react";
import type { Row } from "./types";
import type { buildProfile } from "./profile";
import { responseSize } from "./profile";
import { formatBytes, formatDuration } from "./format";
import { PAYLOAD_NOTE, toolDuration } from "./metrics";
import { StatusIcon } from "./Timeline";

interface Props {
  profile: ReturnType<typeof buildProfile>;
  onInspect: (row: Row) => void;
  onTool: (tool: string) => void;
  onCalls: () => void;
  connecting: boolean;
  replay: boolean;
}

export function Overview({ profile, onInspect, onTool, onCalls, connecting, replay }: Props) {
  return (
    <main className="min-h-0 flex-1 overflow-y-auto" aria-label="Recording overview">
      <div className="mx-auto max-w-[1600px] px-4 py-6 sm:px-6 lg:px-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Recording overview</h1>
            <p className="mt-1 text-sm text-ink-2">Inspect tool time, captured responses and reported errors.</p>
          </div>
          {profile.calls.length > 0 && <button onClick={onCalls} className="action-button">
            Inspect calls <ArrowRight aria-hidden className="size-4" />
          </button>}
        </div>

        {profile.calls.length === 0 ? (
          <section className="border-y border-rule-strong py-10 sm:py-14" aria-live="polite">
            <h2 className="text-lg font-semibold">{connecting ? "Connecting to the recorder" : replay ? "No tool calls in this recording" : "Ready for the first tool call"}</h2>
            <p className="mt-2 max-w-prose text-sm text-ink-2">
              {connecting ? "Recorded calls will appear when the local connection is available." : replay ? "The event log still contains any captured lifecycle or protocol events." : "Connect your coding agent with Sentinel hooks, or put an MCP stdio server behind the proxy. Tool activity and the calls worth inspecting will appear here."}
            </p>
            {!connecting && !replay && <div className="mt-6 flex flex-wrap gap-x-10 gap-y-5 text-sm">
              <div><p className="mb-2 font-medium">Agent hooks</p><code className="text-xs text-ink-2">sentinel hook install</code></div>
              <div><p className="mb-2 font-medium">MCP stdio</p><code className="text-xs text-ink-2">sentinel mcp -- &lt;server&gt;</code></div>
            </div>}
          </section>
        ) : (
          <div className="grid items-start gap-x-8 gap-y-8 xl:grid-cols-[minmax(0,3fr)_minmax(320px,2fr)]">
            <section className="min-w-0" aria-labelledby="tool-activity-title">
              <div className="section-heading">
                <h2 id="tool-activity-title" className="text-base font-semibold">Tool activity</h2>
                <span className="text-xs text-ink-2">{profile.tools.length} {profile.tools.length === 1 ? "tool" : "tools"}</span>
              </div>
              <table className="w-full table-fixed border-collapse text-left text-sm">
                <caption className="sr-only">Tool calls, reported errors, available durations and retained payload JSON. Select a tool to inspect its calls.</caption>
                <thead className="border-b border-rule-strong text-xs font-medium text-ink-2">
                  <tr>
                    <th scope="col" className="py-3 pr-3 font-medium">Tool</th>
                    <th scope="col" className="w-16 py-3 text-right font-medium">Calls</th>
                    <th scope="col" className="w-16 py-3 text-right font-medium">Errors</th>
                    <th scope="col" className="hidden w-32 py-3 text-right font-medium md:table-cell">Median / p95</th>
                    <th scope="col" className="w-24 py-3 pl-3 text-right font-medium" title={PAYLOAD_NOTE}>JSON retained</th>
                  </tr>
                </thead>
                <tbody>
                  {profile.tools.map(({ name, metrics }) => <tr key={name} className="border-b border-rule hover:bg-sheet">
                    <th scope="row" className="py-3 pr-3 font-semibold">
                      <button onClick={() => onTool(name)} className="group flex max-w-full items-center gap-2 py-1 text-left hover:underline" title={`Inspect ${name} calls`}>
                        <span className="truncate">{name}</span><ArrowUpRight aria-hidden className="size-3.5 shrink-0 text-ink-3 group-hover:text-ink" />
                      </button>
                    </th>
                    <td className="fig py-3 text-right">{metrics.calls}</td>
                    <td className={`fig py-3 text-right ${metrics.errors ? "font-semibold text-warn" : "text-ink-2"}`}>{metrics.errors}</td>
                    <td className="fig hidden py-3 text-right text-ink-2 md:table-cell" title={`${metrics.latencySamples} available duration samples`}>
                      {metrics.latencySamples ? `${formatDuration(metrics.p50)} / ${formatDuration(metrics.p95)}` : "–"}
                    </td>
                    <td className="fig py-3 pl-3 text-right text-ink-2">{formatBytes(metrics.payloadBytes)}</td>
                  </tr>)}
                </tbody>
              </table>
              <p className="mt-3 max-w-prose text-xs leading-relaxed text-ink-2">
                Durations use completed calls with an available measurement. Interrupted calls and historical approval holds are excluded. Select a tool to inspect its evidence.
              </p>
            </section>

            <div className="min-w-0 space-y-8">
              <Ranking title="Longest completed calls" rows={profile.longest} measure="duration" onInspect={onInspect}
                empty="No completed tool calls with an available duration." />
              <Ranking title="Largest captured responses" rows={profile.largest} measure="response" onInspect={onInspect}
                empty="No retained tool responses to compare yet." />
            </div>
          </div>
        )}

        <details className="mt-8 border-t border-rule-strong py-4 text-xs text-ink-2">
          <summary className="w-fit cursor-pointer py-1 font-medium text-ink">What this recording can show</summary>
          <div className="mt-3 max-w-prose space-y-2 leading-relaxed">
            <p>These summaries describe the tool events available in this recording. A recording may contain multiple agent conversations; they are not yet separated here.</p>
            <p>{PAYLOAD_NOTE} An absent duration is unavailable, not a measured zero.</p>
            <p>Time between tool calls is not model thinking time. This view does not measure the model context window or provider billing.</p>
          </div>
        </details>
      </div>
    </main>
  );
}

function Ranking({ title, rows, measure, onInspect, empty }: {
  title: string; rows: Row[]; measure: "duration" | "response"; onInspect: (row: Row) => void; empty: string;
}) {
  return <section aria-label={title}>
    <div className="section-heading"><h2 className="text-base font-semibold">{title}</h2><span className="text-xs text-ink-2">Top {rows.length || 5}</span></div>
    {rows.length === 0 ? <p className="py-5 text-sm text-ink-2">{empty}</p> : <ol>
      {rows.map((row) => <li key={row.key} className="border-b border-rule">
        <button onClick={() => onInspect(row)} className="group flex w-full items-center gap-3 py-3 text-left hover:bg-sheet">
          <StatusIcon status={row.status} />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-semibold">{row.title}</span>
            <span className="block truncate font-mono text-xs text-ink-2" title={row.subtitle}>{row.subtitle ?? "No argument summary"}</span>
          </span>
          <span className="fig shrink-0 text-sm font-medium">{measure === "duration" ? formatDuration(toolDuration(row)!) : formatBytes(responseSize(row)!)}</span>
          <ArrowUpRight aria-hidden className="size-3.5 shrink-0 text-ink-3 group-hover:text-ink" />
        </button>
      </li>)}
    </ol>}
    {measure === "response" && <p className="mt-2 text-xs text-ink-2">Retained response JSON; adapters may omit or truncate content.</p>}
  </section>;
}
