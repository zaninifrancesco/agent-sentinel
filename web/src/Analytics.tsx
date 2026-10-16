import { useEffect, useState } from "react";
import { ArrowRight, RefreshCw, X } from "lucide-react";
import { readJSON } from "./useProfiler";
import { formatBytes, formatDuration } from "./format";
import type { AnalyticsData, HistorySession, UsageBreakdown } from "./types";

const count = (value: number) => value.toLocaleString();
const percent = (value: number | null) => value === null ? "Unavailable" : `${(value * 100).toFixed(1)}%`;
const date = (value: string) => new Date(value).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" });
const label = (value: string) => ({ "cursor-hook": "Cursor", "claude-hook": "Claude Code", "mcp-stdio": "MCP stdio", unknown: "Unknown" }[value] ?? value);

export function periodChange(current: number, previous: number): string {
  if (previous === 0) return current === 0 ? "No observed calls in either period" : `${count(current)} more observed calls; previous period had none`;
  const change = (current - previous) / previous * 100;
  return `${change > 0 ? "+" : ""}${change.toFixed(1)}% calls vs previous period`;
}

interface Props {
  active: boolean;
  sessionsOnly: boolean;
  onOpen: (session: HistorySession) => void;
}

export function Analytics({ active, sessionsOnly, onOpen }: Props) {
  const [range, setRange] = useState("7d");
  const [agent, setAgent] = useState("");
  const [project, setProject] = useState("");
  const [day, setDay] = useState("");
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loadedQuery, setLoadedQuery] = useState("");
  const [error, setError] = useState("");
  const [refresh, setRefresh] = useState(0);
  const [sessionPage, setSessionPage] = useState(0);
  const query = new URLSearchParams({ range, agent, project, ...(day ? { day } : {}) }).toString();
  useEffect(() => {
    if (!active) return;
    const controller = new AbortController();
    let timer: number | undefined;
    const load = async () => {
      try {
        const next = await readJSON<AnalyticsData>(`/api/v1/analytics?${query}`, controller.signal);
        if (controller.signal.aborted) return;
        setData(next); setLoadedQuery(query); setError("");
      } catch (cause) {
        if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : "Could not read the recording archive");
      } finally {
        if (!controller.signal.aborted) timer = window.setTimeout(load, 10000);
      }
    };
    void load();
    return () => { controller.abort(); window.clearTimeout(timer); };
  }, [active, query, refresh]);
  const ready = data && loadedQuery === query;
  const filter = (set: (value: string) => void, value: string) => { set(value); setDay(""); setSessionPage(0); };
  const page = Math.min(sessionPage, Math.max(0, Math.ceil((data?.sessions.length ?? 0) / 25) - 1));

  return <main hidden={!active} className="analytics-main" aria-label={sessionsOnly ? "Session archive" : "Agent usage analytics"}>
    <div className="analytics-filters">
      <label>Period<select className="field-input" value={range} onChange={(event) => filter(setRange, event.target.value)}><option value="24h">Last 24 hours</option><option value="7d">Last 7 days</option><option value="30d">Last 30 days</option></select></label>
      <label>Agent<select className="field-input" value={agent} onChange={(event) => filter(setAgent, event.target.value)}><option value="">All agents</option>{data?.availableAgents.map((value) => <option key={value} value={value}>{label(value)}</option>)}</select></label>
      <label className="analytics-project">Project<select className="field-input" value={project} onChange={(event) => filter(setProject, event.target.value)}><option value="">All projects</option>{data?.availableProjects.map((value) => <option key={value} value={value}>{label(value)}</option>)}</select></label>
      <button className="action-button" onClick={() => setRefresh((n) => n + 1)}><RefreshCw aria-hidden className="size-3.5" />Refresh</button>
    </div>
    {error && <p role="alert" className="analytics-alert">Could not update analytics: {error} <button onClick={() => setRefresh((n) => n + 1)} className="text-accent underline">Retry</button>{ready && " · Showing the last successful snapshot."}</p>}
    {!ready ? <div role="status" className="analytics-loading"><p>Loading recorded activity…</p><div /><div /><div /></div> : <>
      <div className="analytics-context"><span>{date(data.from)} – {date(data.to)} UTC</span><span>{data.recordingCount} recordings · {data.nativeSessions} native sessions observed</span></div>
      {(data.warnings.length > 0 || data.persistence.error || !data.persistence.enabled) && <section className="analytics-alert" aria-label="Capture and archive notices">
        {data.persistence.error && <p role="alert">Recording could not be saved: {data.persistence.error}. The archive may be incomplete; live activity remains in memory.</p>}
        {!data.persistence.enabled && <p>The current collector has no active file writer. New activity is not being persisted by this process; historical recordings remain available.</p>}
        {data.warnings.length > 0 && <details><summary>{data.warnings.length} archive notices</summary><ul>{data.warnings.map((warning, i) => <li key={i}>{warning}</li>)}</ul></details>}
      </section>}
      {!sessionsOnly && <>
        <dl className="analytics-measures">
          <div><dt>Observed tool calls</dt><dd>{count(data.totals.calls)}</dd><small>{periodChange(data.totals.calls, data.previous.calls)}</small></div>
          <div><dt>Reported error rate</dt><dd>{percent(data.totals.errorRate)}</dd><small>{count(data.totals.errors)} errors / {count(data.totals.errorSamples)} known outcomes</small></div>
          <div><dt>Median tool duration</dt><dd>{data.totals.p50 === null ? "Unavailable" : formatDuration(data.totals.p50)}</dd><small>p95 {data.totals.p95 === null ? "unavailable" : formatDuration(data.totals.p95)} · {count(data.totals.latencySamples)} samples</small></div>
          <div><dt>JSON retained</dt><dd>{formatBytes(data.totals.retainedBytes)}</dd><small>{formatDuration(data.totals.toolDurationMs)} cumulative tool duration</small></div>
        </dl>
        <section className="analytics-trend" aria-labelledby="activity-heading">
          <div className="section-heading"><div><h2 id="activity-heading">Activity over time</h2><p>Observed calls per UTC day · select a day to explore its sessions</p></div><span className="analytics-legend"><i className="series-dot" />Calls <i className="series-dot analytics-error-dot" />Errors</span></div>
          <ActivityChart days={data.days} selected={day} onSelect={(value) => { setDay(day === value ? "" : value); setSessionPage(0); }} />
          <p className="analytics-note">Source: local JSONL archive and current recording. Empty days mean no captured calls in the archive; capture coverage is unknown.</p>
        </section>
        <div className="analytics-breakdowns">
          <Breakdown title="By agent" rows={data.agents} onSelect={(value) => filter(setAgent, value)} />
          <Breakdown title="By project" rows={data.projects} onSelect={(value) => filter(setProject, value)} />
          <Breakdown title="By tool" rows={data.tools} />
        </div>
      </>}
      <section className="analytics-sessions" aria-labelledby="sessions-heading">
        <div className="section-heading"><div><h2 id="sessions-heading">{sessionsOnly ? "Recorded sessions" : "Explore the sessions"}</h2><p>Each row is an agent session or connection within a recording. Counts follow the selected period.</p></div>{day && <button className="action-button" onClick={() => setDay("")}>{day} UTC<X aria-hidden className="size-3.5" /><span className="sr-only">Clear selected day</span></button>}</div>
        {data.sessions.length === 0 ? <div className="analytics-empty"><h3>No recorded sessions in this selection</h3><p>Choose another period or clear the agent and project filters. Capture agent activity with Sentinel hooks or the MCP proxy to build your history.</p>{(agent || project || day) && <button className="action-button" onClick={() => { setAgent(""); setProject(""); setDay(""); }}>Clear filters</button>}</div> : <div className="analytics-table-scroll"><table className="analytics-table"><caption className="sr-only">Captured sessions with agent, project, calls, error rate and last observed call. Open a session to inspect all its captured calls.</caption><thead><tr><th scope="col">Session / connection</th><th scope="col">Project</th><th scope="col">Calls</th><th scope="col">Error rate</th><th scope="col">Last activity · UTC</th><th scope="col"><span className="sr-only">Inspect</span></th></tr></thead><tbody>{data.sessions.slice(page * 25, (page + 1) * 25).map((session) => <tr key={`${session.recordingId}/${session.scope}`}>
          <th scope="row"><button className="analytics-session-link" onClick={() => onOpen(session)}><span>{label(session.agent)}</span><small title={session.nativeSessionId ?? session.recordingId}>{session.nativeSessionId ?? session.recordingId}</small></button><span className="analytics-session-state">{session.recoveredTail ? "Recovered tail · incomplete" : session.live ? "Collector active" : session.ended ? "Recording ended" : "Recording end not captured"}</span></th>
          <td><span className="analytics-project-name" title={session.projects.join(", ")}>{session.projects.map(label).join(", ")}</span></td><td className="fig">{count(session.calls)}</td><td className="fig">{percent(session.errorRate)}</td><td>{date(session.last)}</td><td><button className="icon-button" onClick={() => onOpen(session)} aria-label={`Inspect ${label(session.agent)} session ${session.nativeSessionId ?? session.recordingId}`}><ArrowRight aria-hidden className="size-4" /></button></td>
        </tr>)}</tbody></table></div>}
        {data.sessions.length > 25 && <div className="page-controls"><span>{page * 25 + 1}–{Math.min((page + 1) * 25, data.sessions.length)} of {data.sessions.length} sessions</span><button className="action-button" disabled={page === 0} onClick={() => setSessionPage(page - 1)}>Previous sessions</button><button className="action-button" disabled={(page + 1) * 25 >= data.sessions.length} onClick={() => setSessionPage(page + 1)}>Next sessions</button></div>}
      </section>
      <details className="analytics-method"><summary>How these metrics are calculated</summary><p>Calls are attributed to their captured start or observation time in [start, end). Error rate uses errors / (successes + errors); open calls, interruptions and historical policy outcomes are separate. Duration percentiles use all valid completed samples, including zero. Cumulative tool duration can exceed elapsed time when calls overlap. Retained JSON measures stored payloads, not model context, tokens or billing. Previous-period figures use the same filters and an equally long preceding interval. {data.undatedCalls > 0 && `${data.undatedCalls} calls without a timestamp were excluded.`}</p><p>Unknown identifies data not supplied by the source. Native-session counts are scoped to each recording; continuity across separate recordings is not inferred. Version: {data.version}.</p></details>
    </>}
  </main>;
}

function ActivityChart({ days, selected, onSelect }: { days: UsageBreakdown[]; selected: string; onSelect: (day: string) => void }) {
  const max = Math.max(1, ...days.map((day) => day.calls));
  return <div className="activity-plot">
    <div className="activity-axis"><span>{count(max)}</span><span>Calls / day</span><span>0</span></div>
    <div className="activity-scroll"><div className="activity-bars" style={{ gridTemplateColumns: `repeat(${days.length}, minmax(28px, 1fr))` }}>{days.map((day) => <button key={day.name} aria-pressed={day.name === selected} onClick={() => onSelect(day.name)} className="activity-day" title={`${day.name} UTC: ${day.calls} calls, ${day.errors} errors`} aria-label={`${day.name} UTC: ${day.calls} observed calls, ${day.errors} errors. Show sessions.`}>
      <span className="activity-bar-area"><span className="activity-call-bar" style={{ height: `${day.calls / max * 100}%` }} /><span className="activity-error-bar" style={{ height: `${day.errors / max * 100}%` }} /></span><span className="activity-date">{day.name.slice(5)}</span>
    </button>)}</div><p className="activity-x-label">Date · UTC</p></div>
  </div>;
}

function Breakdown({ title, rows, onSelect }: { title: string; rows: UsageBreakdown[]; onSelect?: (value: string) => void }) {
  const [expanded, setExpanded] = useState(false);
  const shown = expanded ? rows : rows.slice(0,5);
  return <section className="analytics-breakdown"><h2>{title}</h2>{rows.length === 0 ? <p className="analytics-note">No captured calls in this period.</p> : <table><caption className="sr-only">{title}: observed calls and reported errors</caption><thead><tr><th scope="col">{title.replace("By ", "")}</th><th scope="col">Calls</th><th scope="col">Errors</th></tr></thead><tbody>{shown.map((row) => <tr key={row.name}><th scope="row">{onSelect ? <button onClick={() => onSelect(row.name)} title={row.name}>{label(row.name)}</button> : <span title={row.name}>{label(row.name)}</span>}</th><td className="fig">{count(row.calls)}</td><td className="fig">{count(row.errors)}</td></tr>)}</tbody></table>}{rows.length > 5 && <button className="text-accent text-xs underline mt-3" onClick={() => setExpanded((value) => !value)}>{expanded ? "Show top five" : `Show all ${rows.length}`}</button>}</section>;
}
