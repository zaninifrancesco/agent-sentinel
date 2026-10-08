import type { Risk, Row, Session, SentinelConfig, Status } from "./types";
import { formatDuration, formatTime } from "./format";
import { computeMetrics, COST_NOTE } from "./metrics";
import { journeyStops, type Stop, type Tone } from "./journey";

/**
 * Everything in the report that originates from the agent or an MCP server
 * (tool names, methods, ids, arguments...) is untrusted: a malicious server
 * could name a tool `<script>...</script>`. Escape it before it is put into
 * HTML. The report also carries a CSP with no script at all, as a second line.
 */
export function esc(value: unknown): string {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

export interface ReportInput {
  session: Session | null;
  rows: Row[];
  /** What the running Sentinel was configured with; null when unknown. */
  config: SentinelConfig | null;
  exportedAt: Date;
}

/** Statuses are never told by colour alone: each has a glyph and a word. */
const STATUS_MARK: Record<Status, { glyph: string; word: string; tone: Tone }> = {
  ok: { glyph: "✓", word: "OK", tone: "done" },
  pending: { glyph: "…", word: "RUNNING", tone: "quiet" },
  awaiting_approval: { glyph: "■", word: "WAITING", tone: "held" },
  error: { glyph: "▲", word: "ERROR", tone: "warn" },
  blocked: { glyph: "⊘", word: "BLOCKED", tone: "alert" },
  rejected: { glyph: "✕", word: "REJECTED", tone: "alert" },
};

/** How a policy decision ended, in one phrase. Undefined when policy had nothing to say. */
function outcome(row: Row): { text: string; tone: Tone } | undefined {
  const decision = row.request?.decision;
  if (!decision) return undefined;
  if (decision === "block") return { text: "Blocked by policy", tone: "alert" };
  if (decision === "approve") {
    if (row.status === "awaiting_approval") return { text: "Still waiting for a human", tone: "held" };
    switch (row.resolution?.decision) {
      case "approved":
        return { text: "Approved by a human", tone: "done" };
      case "rejected":
        return { text: "Rejected by a human", tone: "alert" };
      case "timeout":
        return { text: "Timed out, refused", tone: "alert" };
      case "cancelled":
        return { text: "Cancelled by the agent", tone: "alert" };
      default:
        return { text: "Needed approval", tone: "quiet" };
    }
  }
  return { text: "Allowed, flagged", tone: "warn" };
}

function riskChip(risk: Risk): string {
  if (risk === "none") return "";
  return `<span class="chip chip-${risk}">${esc(risk)}</span>`;
}

function stamp(iso: string | undefined): string {
  if (!iso) return "–";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso; // callers escape
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${formatTime(iso)}`;
}

function stopHtml(s: Stop): string {
  return `<li class="stop tone-${s.tone}">
        <div class="stop-head"><span class="stop-title">${esc(s.title)}</span>${s.at ? `<span class="fig">${esc(formatTime(s.at))}</span>` : ""}</div>
        <div class="stop-text">${esc(s.text)}</div>
        ${s.detail ? `<div class="stop-detail">${esc(s.detail)}</div>` : ""}
      </li>`;
}

function rowHtml(row: Row): string {
  const mark = STATUS_MARK[row.status];
  const refused = row.status === "blocked" || row.status === "rejected";
  const policy = [row.request?.rule ? `<code>${esc(row.request.rule)}</code>` : "", riskChip(row.risk)].join("");
  const cells = `
      <span class="fig t">${esc(formatTime(row.startedAt))}</span>
      <span class="status tone-${mark.tone}"><span aria-hidden="true">${mark.glyph}</span> ${mark.word}</span>
      <span class="ev kind-${row.kind}${refused ? " refused" : ""}">
        <span class="title">${esc(row.title)}</span>
        ${row.subtitle ? `<span class="sub">${esc(row.subtitle)}</span>` : ""}
      </span>
      <span class="pol">${policy}</span>
      <span class="fig took">${row.durationMs !== undefined ? esc(formatDuration(row.durationMs)) : ""}</span>`;

  // A tool call opens into its journey; plain protocol rows have nothing more to say.
  if (row.kind === "tool" && row.request) {
    return `<details class="row" id="${esc(row.key)}">
    <summary class="grid">${cells}</summary>
    <ol class="journey">${journeyStops(row).map(stopHtml).join("")}</ol>
  </details>`;
  }
  return `<div class="row"><div class="grid">${cells}</div></div>`;
}

/** The calls where policy or a human decided something: the evidence of an audit. */
function decisionsHtml(rows: Row[]): string {
  const decided = rows.filter((r) => r.kind === "tool" && r.request?.decision);
  if (decided.length === 0) {
    return `<p class="quiet">Policy raised nothing in this session: no call was blocked, held or flagged.</p>`;
  }
  return `<ul class="decisions">${decided
    .map((r) => {
      const o = outcome(r)!;
      const req = r.request!;
      // The operator's own words exist only when a human answered.
      const answered = r.resolution?.decision === "approved" || r.resolution?.decision === "rejected";
      const note = answered ? r.resolution?.reason : undefined;
      const timedOut = r.resolution?.decision === "timeout" ? r.resolution.reason : undefined;
      return `
    <li class="tone-${o.tone}">
      <div class="d-head">
        <span class="fig">${esc(formatTime(r.startedAt))}</span>
        <a class="title" href="#${esc(r.key)}">${esc(r.title)}</a>
        <span class="d-out">${esc(o.text)}</span>
        <span class="d-pol">${req.rule ? `<code>${esc(req.rule)}</code>` : ""}${riskChip(r.risk)}</span>
      </div>
      ${r.subtitle ? `<div class="sub">${esc(r.subtitle)}</div>` : ""}
      ${req.reason ? `<div class="d-reason">${esc(req.reason)}</div>` : ""}
      ${note ? `<div class="d-reason">Operator note: ${esc(note)}</div>` : ""}
      ${timedOut ? `<div class="d-reason">${esc(timedOut)}</div>` : ""}
    </li>`;
    })
    .join("")}</ul>`;
}

function fact(label: string, value: string): string {
  return `<div><dt>${esc(label)}</dt><dd>${value}</dd></div>`;
}

// The palette is the cockpit's (DESIGN.md), light by default and following the
// reader's system theme. The report is one offline file, so Archivo is not
// embedded: it falls back to the documented system sans.
const CSS = `
:root {
  color-scheme: light dark;
  --sans: ui-sans-serif, system-ui, sans-serif;
  --mono: ui-monospace, SFMono-Regular, Menlo, monospace;
  --paper: #f4f2ea; --sheet: #fbfaf5; --rule: #d9d5c7; --rule-strong: #b3ae9c;
  --ink: #15140f; --ink-2: #46433a; --ink-3: #6b675b;
  --yellow: #ffd92e; --yellow-soft: #fff0a3;
  --signal: #d5001c; --ok: #1b7040; --warn: #9a4d00;
}
@media (prefers-color-scheme: dark) {
  :root {
    --paper: #12110b; --sheet: #1a1912; --rule: #2b2920; --rule-strong: #4a4637;
    --ink: #efede3; --ink-2: #c4c0b0; --ink-3: #8f8b7b;
    --yellow-soft: #3b3310;
    --signal: #ec2c43; --ok: #4cc38a; --warn: #f0a040;
  }
}
* { box-sizing: border-box; }
body {
  margin: 0; background: var(--paper); color: var(--ink);
  font: 14px/1.45 var(--sans);
  font-variant-numeric: tabular-nums;
}
code, .sub, .cmd { font-family: var(--mono); font-size: 12px; }
.fig { font-variant-numeric: tabular-nums; letter-spacing: 0.01em; }
.bar { background: #15140f; color: #f4f2ea; padding: 10px 24px; font-weight: 700; display: flex; justify-content: space-between; gap: 16px; }
.bar span:last-child { font-weight: 400; color: #b3ae9c; }
main { max-width: 1000px; margin: 0 auto; padding: 28px 24px 56px; }
h1 { font-size: 24px; line-height: 1.2; margin: 0; overflow-wrap: anywhere; }
h2 { font-size: 18px; line-height: 1.3; margin: 40px 0 4px; }
.lede { margin: 0 0 12px; color: var(--ink-2); }
.quiet { color: var(--ink-2); margin: 8px 0; }
dl.facts { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 14px 24px; margin: 18px 0 0; padding: 16px 0; border-top: 2px solid var(--ink); border-bottom: 1px solid var(--rule-strong); }
dl.facts dt { font-size: 12px; font-weight: 500; color: var(--ink-2); }
dl.facts dd { margin: 2px 0 0; font-weight: 600; overflow-wrap: anywhere; }
dl.facts dd .alert { color: var(--signal); }
.cmd { font-weight: 500; }

.chip { display: inline-block; margin-left: 8px; padding: 0 6px; border-radius: 2px; font-size: 11px; font-weight: 600; line-height: 16px; letter-spacing: 0.04em; text-transform: uppercase; border: 1px solid var(--rule-strong); color: var(--ink-3); }
.chip-medium { border-color: var(--ink-3); color: var(--ink-2); }
.chip-high { border-color: var(--signal); color: var(--signal); }
.chip-critical { border-color: var(--signal); background: var(--signal); color: #fff; }

.tone-done { --tone: var(--ok); } .tone-warn { --tone: var(--warn); }
.tone-alert { --tone: var(--signal); } .tone-held { --tone: var(--ink); } .tone-quiet { --tone: var(--ink-3); }

ul.decisions { list-style: none; margin: 8px 0 0; padding: 0; border-top: 2px solid var(--ink); background: var(--sheet); }
.decisions li { padding: 10px 16px; border-bottom: 1px solid var(--rule); border-left: 4px solid var(--tone); }
.d-head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 14px; }
.d-head .title { font-weight: 600; color: var(--ink); }
.d-out { font-weight: 600; color: var(--tone); }
.d-pol { margin-left: auto; }
.d-reason { color: var(--ink-2); margin-top: 2px; }
.sub { color: var(--ink-2); display: block; overflow-wrap: anywhere; }

.table { margin-top: 8px; background: var(--sheet); border-top: 2px solid var(--ink); }
.head, .grid { display: grid; grid-template-columns: 72px 96px minmax(0, 1fr) 210px 64px; gap: 0 12px; align-items: center; padding: 8px 16px; }
.head { background: var(--paper); border-bottom: 1px solid var(--rule-strong); font-size: 12px; font-weight: 500; color: var(--ink-2); }
.head span:last-child, .took { text-align: right; }
.pol { text-align: right; }
.row { border-bottom: 1px solid var(--rule); }
.row .t { color: var(--ink-2); font-size: 13px; }
.status { font-size: 12px; font-weight: 600; letter-spacing: 0.03em; color: var(--tone); }
.ev .title { display: block; overflow-wrap: anywhere; }
.kind-tool .title { font-weight: 600; }
.kind-session .title, .kind-session .sub { color: var(--ink-3); }
.kind-rpc .title, .kind-notification .title, .kind-raw .title { color: var(--ink-2); }
.refused .title { text-decoration: line-through; text-decoration-color: var(--signal); text-decoration-thickness: 2px; }
.took { color: var(--ink-2); font-size: 13px; }
details.row > summary { list-style: none; cursor: pointer; }
details.row > summary::-webkit-details-marker { display: none; }
details.row > summary:hover { background: var(--paper); }
details.row[open] > summary { background: var(--yellow-soft); }
details.row:target > summary { outline: 2px solid var(--ink); outline-offset: -2px; }
:focus-visible { outline: 2px solid var(--ink); outline-offset: 2px; }

ol.journey { list-style: none; margin: 0; padding: 12px 16px 16px 112px; background: var(--paper); border-top: 1px solid var(--rule); }
.stop { position: relative; padding: 0 0 12px 24px; }
.stop::before { content: ""; position: absolute; left: 0; top: 4px; width: 11px; height: 11px; border-radius: 50%; border: 2px solid var(--tone); background: var(--tone); }
.stop.tone-quiet::before { background: var(--sheet); }
.stop.tone-held::before { background: var(--yellow); border-color: var(--ink); }
.stop::after { content: ""; position: absolute; left: 6px; top: 18px; bottom: -4px; width: 1px; background: var(--rule-strong); }
.stop:last-child { padding-bottom: 0; } .stop:last-child::after { display: none; }
.stop-head { display: flex; justify-content: space-between; gap: 16px; }
.stop-title { font-weight: 600; }
.stop-head .fig { color: var(--ink-2); font-size: 13px; }
.stop.tone-alert .stop-text { color: var(--signal); font-weight: 500; }
.stop-detail { color: var(--ink-2); font-size: 13px; overflow-wrap: anywhere; }

footer { margin-top: 40px; padding-top: 14px; border-top: 1px solid var(--rule-strong); color: var(--ink-2); font-size: 12px; }
footer p { margin: 0 0 4px; }

@media (max-width: 720px) {
  .head, .grid { grid-template-columns: 56px 84px minmax(0, 1fr) 56px; padding: 8px 12px; }
  .head span:nth-child(4), .pol { display: none; }
  ol.journey { padding-left: 16px; }
}
@media print {
  .bar { background: none; color: #000; border-bottom: 2px solid #000; }
}
`;

export function buildReportHtml({ session, rows, config, exportedAt }: ReportInput): string {
  const m = computeMetrics(rows);
  const tools = rows.filter((r) => r.kind === "tool");

  // How the held calls were settled.
  const held = tools.filter((r) => r.request?.decision === "approve");
  const settled = (d: string) => held.filter((r) => r.resolution?.decision === d).length;
  const stillWaiting = held.filter((r) => r.status === "awaiting_approval").length;
  const heldText =
    held.length === 0
      ? "none"
      : `${held.length}: ${settled("approved")} approved, ${settled("rejected")} rejected, ${settled("timeout")} timed out` +
        (stillWaiting ? `, ${stillWaiting} still waiting` : "");

  const sessionId = session?.id || "session";
  const start = session?.startedAt;
  const end = session?.endedAt;
  const spanMs = start && end ? new Date(end).getTime() - new Date(start).getTime() : NaN;
  const lasted = Number.isNaN(spanMs) ? undefined : formatDuration(spanMs);

  // "replay" and "unknown" describe the viewer, not the recorded session.
  const policy = config?.policy && config.policy !== "unknown" && config.policy !== "replay" ? config.policy : undefined;
  const budget = config && config.maxCostUsd > 0 ? `$${config.maxCostUsd.toFixed(2)}` : undefined;

  const facts = [
    fact("Command", session?.command?.length ? `<span class="cmd">${esc(session.command.join(" "))}</span>` : "–"),
    fact("Started", esc(stamp(start))),
    fact("Ended", end ? `${esc(stamp(end))}${lasted ? ` (${esc(lasted)})` : ""}` : "still running when exported"),
    fact("Policy", policy ? esc(policy) : "–"),
    fact("Tool calls", String(m.calls)),
    fact("Refused", `<span${m.refused > 0 ? ' class="alert"' : ""}>${m.refused}</span>`),
    fact("Held for a human", esc(heldText)),
    fact("Tool errors", String(m.errors)),
    fact("Median / p95", `${m.p50} / ${m.p95} ms`),
    fact(
      "Cost (estimate)",
      `~$${m.cost.toFixed(3)}${budget ? ` of ${esc(budget)} budget` : ""} · ~${m.tokens > 1000 ? `${(m.tokens / 1000).toFixed(1)}k` : m.tokens} tokens`,
    ),
  ].join("");

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <!-- The report is static: no script, no network, no external fonts. -->
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'" />
  <title>Agent Sentinel audit - ${esc(sessionId)}</title>
  <style>${CSS}</style>
</head>
<body>
  <div class="bar"><span>Agent Sentinel</span><span>Session audit</span></div>
  <main>
    <h1>${esc(sessionId)}</h1>
    <dl class="facts">${facts}</dl>

    <h2>What policy and you decided</h2>
    <p class="lede">Every call that was blocked, held for approval or flagged, with the rule that fired and why.</p>
    ${decisionsHtml(rows)}

    <h2>Timetable</h2>
    <p class="lede">Every event of the session, in order. Open a tool call to follow its journey.</p>
    <div class="table">
      <div class="head"><span>Time</span><span>Status</span><span>Event</span><span class="pol">Policy</span><span>Took</span></div>
      ${rows.length === 0 ? `<p class="quiet" style="padding:12px 16px">No events were recorded.</p>` : rows.map(rowHtml).join("")}
    </div>

    <footer>
      <p>Exported <time datetime="${esc(exportedAt.toISOString())}">${esc(stamp(exportedAt.toISOString()))}</time> by Agent Sentinel.</p>
      <p>Cost and tokens are estimates, not billing data. ${esc(COST_NOTE)}</p>
      <p>Arguments are shown as summarised in the cockpit; full request and response payloads are not included in this file.</p>
    </footer>
  </main>
</body>
</html>`;
}
