import { useState } from "react";
import type { Row, SentinelEvent } from "./types";
import { RiskChip, StatusIcon } from "./Timeline";
import { formatDuration, formatTime } from "./format";
import { DiffViewer, diffFilename, extractDiff, isDiff } from "./DiffViewer";
import { Check, Copy } from "lucide-react";

type Tab = "payload" | "diff" | "raw";

export function Detail({ row }: { row: Row | null }) {
  const [copied, setCopied] = useState(false);
  const [activeTab, setActiveTab] = useState<Tab>("payload");

  if (!row) {
    return (
      <section className="grid place-items-center bg-paper p-8 text-sm text-ink-2">
        <div className="max-w-xs">
          <p className="font-semibold text-ink">Nothing selected</p>
          <p className="mt-1">
            Pick a row in the timetable, or move with <kbd className="font-mono text-xs">J</kbd> /{" "}
            <kbd className="font-mono text-xs">K</kbd>. <kbd className="font-mono text-xs">⌘K</kbd> searches.
          </p>
        </div>
      </section>
    );
  }

  const blocks: { label: string; event: SentinelEvent }[] = [];
  if (row.request) blocks.push({ label: "Request", event: row.request });
  if (row.response) blocks.push({ label: "Response", event: row.response });
  if (row.event) blocks.push({ label: "Payload", event: row.event });

  // Look for a real patch in the text a tool produced or was given, not in
  // the JSON-RPC envelope around it.
  let diffText: string | null = null;
  for (const { event } of blocks) {
    for (const candidate of textCandidates(event.payload)) {
      if (diffText === null && isDiff(candidate)) diffText = extractDiff(candidate);
    }
  }
  const tab: Tab = activeTab === "diff" && diffText === null ? "payload" : activeTab;

  const handleCopy = () => {
    navigator.clipboard.writeText(JSON.stringify(row, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const tabs: { id: Tab; label: string }[] = [
    { id: "payload", label: "Payload" },
    ...(diffText !== null ? [{ id: "diff" as Tab, label: "Diff" }] : []),
    { id: "raw", label: "Raw JSON" },
  ];

  return (
    <section className="flex min-h-0 flex-col overflow-hidden bg-paper">
      <div className="border-b border-rule-strong px-6 pb-0 pt-5">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h2 className="flex items-center gap-2.5 truncate text-2xl font-bold leading-tight text-ink">
              <StatusIcon status={row.status} />
              <span className="truncate">{row.title}</span>
            </h2>
            {row.subtitle && <p className="mt-1 truncate font-mono text-[13px] text-ink-2">{row.subtitle}</p>}
          </div>
          <button
            onClick={handleCopy}
            className="flex shrink-0 items-center gap-1.5 rounded-sm border border-rule-strong bg-sheet px-2.5 py-1 text-xs font-medium text-ink-2 hover:border-ink hover:text-ink"
          >
            {copied ? <Check className="size-3.5 text-ok" /> : <Copy className="size-3.5" />}
            {copied ? "Copied" : "Copy event"}
          </button>
        </div>

        <dl className="mt-4 grid grid-cols-4 gap-6 text-sm">
          <Field label="Time" value={formatTime(row.startedAt)} figure />
          <Field label="Took" value={row.durationMs !== undefined ? formatDuration(row.durationMs) : "–"} figure />
          <Field label="Status" value={statusText(row.status)} />
          <div>
            <dt className="text-xs text-ink-2">Risk</dt>
            <dd className="mt-0.5 flex h-5 items-center">
              {row.risk === "none" ? <span className="text-ink-2">none</span> : <RiskChip risk={row.risk} />}
            </dd>
          </div>
        </dl>

        <div role="tablist" className="mt-5 flex gap-5">
          {tabs.map((t) => (
            <button
              key={t.id}
              role="tab"
              aria-selected={tab === t.id}
              onClick={() => setActiveTab(t.id)}
              className={`-mb-px border-b-2 pb-2 text-sm font-medium ${
                tab === t.id ? "border-ink text-ink" : "border-transparent text-ink-2 hover:text-ink"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      <div className="min-h-0 flex-1 select-text overflow-y-auto px-6 py-5">
        {tab === "diff" && diffText ? (
          <DiffViewer text={diffText} filename={diffFilename(diffText)} />
        ) : tab === "raw" ? (
          <pre className="overflow-x-auto rounded-sm border border-rule bg-sheet p-4 font-mono text-xs leading-relaxed text-ink">
            {JSON.stringify(row, null, 2)}
          </pre>
        ) : (
          <div className="space-y-6">
            {row.kind === "tool" && row.request && <Journey row={row} />}
            {blocks.map(({ label, event }) => (
              <div key={event.seq} className="overflow-hidden rounded-sm border border-rule bg-sheet">
                <div className="flex items-center justify-between border-b border-rule bg-paper px-4 py-1.5 text-xs text-ink-2">
                  <span>
                    <span className="font-semibold text-ink">{label}</span>
                    <span className="fig ml-2">#{event.seq}</span>
                  </span>
                  {event.direction && <span className="font-mono">{event.direction}</span>}
                </div>
                <pre className="overflow-x-auto whitespace-pre-wrap p-4 font-mono text-xs leading-relaxed text-ink">
                  {pretty(event.payload)}
                </pre>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

function Field({ label, value, figure }: { label: string; value: string; figure?: boolean }) {
  return (
    <div>
      <dt className="text-xs text-ink-2">{label}</dt>
      <dd className={`mt-0.5 text-base font-semibold text-ink ${figure ? "fig" : ""}`}>{value}</dd>
    </div>
  );
}

function statusText(s: Row["status"]): string {
  return {
    pending: "Running",
    awaiting_approval: "Waiting for you",
    ok: "Done",
    error: "Tool error",
    blocked: "Blocked by policy",
    rejected: "Rejected",
  }[s];
}

type Tone = "done" | "alert" | "held" | "warn" | "quiet";

interface Stop {
  title: string;
  at?: string;
  text: string;
  detail?: string;
  tone: Tone;
}

/** The call as a journey: where it came from, what policy said, who decided, what came back. */
function Journey({ row }: { row: Row }) {
  const req = row.request!;
  const stops: Stop[] = [];

  stops.push({
    title: "Requested",
    at: req.timestamp,
    text: row.subtitle ?? row.title,
    tone: "done",
  });

  const decision = req.decision;
  stops.push({
    title: "Policy",
    at: req.timestamp,
    text: !decision
      ? "No rule matched"
      : decision === "block"
        ? `Blocked by ${req.rule}`
        : decision === "approve"
          ? `Needs approval · ${req.rule}`
          : `Allowed with a warning · ${req.rule}`,
    detail: req.reason,
    tone: !decision ? "quiet" : decision === "block" ? "alert" : decision === "approve" ? "held" : "warn",
  });

  if (row.status === "awaiting_approval") {
    stops.push({ title: "Human", text: "Waiting for you to approve or reject", tone: "held" });
  } else if (row.resolution) {
    const d = row.resolution.decision;
    const label =
      d === "approved" ? "Approved" : d === "rejected" ? "Rejected" : d === "timeout" ? "Timed out, refused" : "Cancelled by the agent";
    stops.push({
      title: "Human",
      at: row.resolution.timestamp,
      text: label,
      detail: row.resolution.reason ? `Note: ${row.resolution.reason}` : undefined,
      tone: d === "approved" ? "done" : "alert",
    });
  }

  if (row.response) {
    stops.push({
      title: "Answered",
      at: row.response.timestamp,
      text: row.status === "blocked" || row.status === "rejected" ? "Refusal sent to the agent, nothing ran" : statusText(row.status),
      detail: row.durationMs !== undefined ? `took ${formatDuration(row.durationMs)}` : undefined,
      tone: row.status === "ok" ? "done" : row.status === "error" ? "warn" : row.status === "pending" ? "quiet" : "alert",
    });
  } else if (row.status === "pending") {
    stops.push({ title: "Answered", text: "Running…", tone: "quiet" });
  }

  const dot: Record<Tone, string> = {
    done: "border-ink bg-ink",
    alert: "border-signal bg-signal",
    held: "border-ink bg-yellow",
    warn: "border-warn bg-warn",
    quiet: "border-rule-strong bg-sheet",
  };

  return (
    <ol className="relative space-y-4 pl-7" aria-label="Journey of this call">
      <span aria-hidden className="absolute bottom-2 left-[7px] top-2 w-px bg-rule-strong" />
      {stops.map((s, i) => (
        <li key={i} className="relative">
          <span
            aria-hidden
            className={`absolute -left-7 top-1 size-[15px] rounded-full border-2 ${dot[s.tone]}`}
          />
          <div className="flex items-baseline justify-between gap-4">
            <span className="text-sm font-semibold text-ink">{s.title}</span>
            {s.at && <span className="fig text-[13px] text-ink-2">{formatTime(s.at)}</span>}
          </div>
          <p className={`text-sm ${s.tone === "alert" ? "font-medium text-signal" : "text-ink"}`}>{s.text}</p>
          {s.detail && <p className="mt-0.5 break-words text-[13px] text-ink-2">{s.detail}</p>}
        </li>
      ))}
    </ol>
  );
}

/** Free-text strings of a JSON-RPC frame: tool result texts and call arguments. */
function textCandidates(payload: unknown): string[] {
  if (typeof payload === "string") return [payload];
  if (!payload || typeof payload !== "object") return [];
  const obj = payload as {
    result?: { content?: unknown };
    params?: { arguments?: unknown };
  };
  const out: string[] = [];
  const content = obj.result?.content;
  if (Array.isArray(content)) {
    for (const c of content) {
      if (c && typeof c === "object" && typeof (c as { text?: unknown }).text === "string") {
        out.push((c as { text: string }).text);
      }
    }
  }
  collectStrings(obj.params?.arguments, out);
  return out;
}

function collectStrings(v: unknown, out: string[]): void {
  if (typeof v === "string") out.push(v);
  else if (Array.isArray(v)) v.forEach((x) => collectStrings(x, out));
  else if (v && typeof v === "object") Object.values(v).forEach((x) => collectStrings(x, out));
}

function pretty(payload: unknown): string {
  if (payload === undefined) return "(no payload)";
  if (typeof payload === "string") return payload;
  return JSON.stringify(payload, null, 2);
}
