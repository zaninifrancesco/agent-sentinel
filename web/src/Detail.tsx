import { useState } from "react";
import type { Row, SentinelEvent } from "./types";
import { formatBytes, formatDuration, formatTime } from "./format";
import { responseSize } from "./profile";
import { journeyStops, statusText, type Tone } from "./journey";
import { DiffViewer, diffFilename, extractDiff, isDiff } from "./DiffViewer";
import { JsonView } from "./JsonView";
import { capturedInterval } from "./trace";
import { CaptureInfo } from "./CaptureInfo";
import { 
  Check, Copy, Search, Braces, Terminal, FileText, FileEdit, 
  Cpu, AlertTriangle, ArrowRight, CornerDownRight, CheckCircle2, Clock
} from "lucide-react";

type Tab = "payload" | "diff" | "journey" | "raw";

function toolIcon(name: string) {
  const lower = name.toLowerCase();
  if (lower.includes("shell") || lower.includes("bash") || lower.includes("exec") || lower.includes("cmd")) {
    return Terminal;
  }
  if (lower.includes("read") || lower.includes("cat") || lower.includes("view")) {
    return FileText;
  }
  if (lower.includes("edit") || lower.includes("write") || lower.includes("patch") || lower.includes("save")) {
    return FileEdit;
  }
  if (lower.includes("search") || lower.includes("grep") || lower.includes("find")) {
    return Search;
  }
  return Cpu;
}

export function Detail({ row }: { row: Row | null }) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>("payload");
  const [query, setQuery] = useState("");

  const triggerCopy = async (key: string, text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedKey(key);
      setTimeout(() => setCopiedKey((curr) => (curr === key ? null : curr)), 1800);
    } catch {
      setCopiedKey("error");
      setTimeout(() => setCopiedKey(null), 1800);
    }
  };

  if (!row) {
    return (
      <section className="grid min-h-0 flex-1 place-items-center bg-sheet p-8 text-sm text-ink-2">
        <div className="max-w-xs text-center">
          <div className="mx-auto mb-4 grid size-12 place-items-center rounded-xl border border-rule bg-paper shadow-2xs">
            <Braces aria-hidden className="size-6 text-accent" />
          </div>
          <h2 className="text-base font-semibold text-ink">Inspect Call Evidence</h2>
          <p className="mt-1.5 text-xs text-ink-2">
            Select any call from the timeline to inspect its arguments, results, captured execution timing and payload data.
          </p>
          <div className="mt-4 inline-flex items-center gap-2 rounded-md border border-rule bg-paper px-2.5 py-1 text-[11px] text-ink-3">
            <kbd className="font-mono text-ink">↑</kbd> <kbd className="font-mono text-ink">↓</kbd> or <kbd className="font-mono text-ink">J</kbd> <kbd className="font-mono text-ink">K</kbd> to navigate
          </div>
        </div>
      </section>
    );
  }

  const blocks: { label: string; event: SentinelEvent; payload: unknown; isInput: boolean }[] = [];
  if (row.request) {
    blocks.push({
      label: row.kind === "tool" ? "Input Arguments" : "Request",
      event: row.request,
      payload: row.kind === "tool" ? argumentsOf(row.request.payload) : row.request.payload,
      isInput: true,
    });
  }
  if (row.response) {
    blocks.push({
      label: row.status === "error" ? "Error / Result" : "Result",
      event: row.response,
      payload: resultOf(row.response.payload),
      isInput: false,
    });
  }
  if (row.event) {
    blocks.push({
      label: "Captured Payload",
      event: row.event,
      payload: row.event.payload,
      isInput: false,
    });
  }

  let diffText: string | null = null;
  let diffEvent: SentinelEvent | undefined;
  for (const { event } of blocks) {
    for (const candidate of textCandidates(event.payload)) {
      if (diffText === null && isDiff(candidate)) {
        diffText = extractDiff(candidate);
        diffEvent = event;
      }
    }
  }

  const tab = activeTab === "diff" && diffText === null ? "payload" : activeTab;
  const bytes = responseSize(row);
  const interval = capturedInterval(row);
  const capturedMs = interval?.end === undefined ? undefined : interval.end - interval.start;
  const historical = !!row.request?.decision || !!row.resolution || row.risk !== "none";
  const evidence = row.request ?? row.response ?? row.event;
  const correlation = row.correlation ?? evidence?.correlation;
  const timingSource = row.response?.timing?.source;
  const ToolIcon = toolIcon(row.title);

  const tabs: { id: Tab; label: string; badge?: string }[] = [
    { id: "payload", label: row.kind === "tool" ? "Arguments & Result" : "Payload" },
    ...(diffText !== null ? [{ id: "diff" as Tab, label: "Patch / Diff" }] : []),
    { id: "journey", label: "Timing Journey" },
    { id: "raw", label: "Event JSON" },
  ];

  const searchable = tab === "raw" ? JSON.stringify(row, null, 2) : blocks.map(({ payload }) => pretty(payload)).join("\n");
  const matches = query.trim() ? searchable.toLocaleLowerCase().split(query.toLocaleLowerCase()).length - 1 : undefined;

  const isError = row.status === "error";

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden bg-sheet" aria-label="Selected event details">
      {/* 1. Header Toolbar */}
      <div className="shrink-0 border-b border-rule bg-sheet px-4 pt-3.5 pb-2">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 flex-1 items-center gap-2">
            <span className="grid size-7 shrink-0 place-items-center rounded-md border border-rule bg-paper">
              <ToolIcon aria-hidden className="size-4 text-ink-2" />
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h2 className="truncate text-sm font-semibold text-ink">{row.title}</h2>
                <StatusBadge status={row.status} />
                {row.request?.seq && (
                  <span className="rounded bg-paper px-1.5 py-0.2 font-mono text-[10px] text-ink-3">
                    seq #{row.request.seq}
                  </span>
                )}
              </div>
              <p className="truncate font-mono text-[11px] text-ink-2" title={row.subtitle}>
                {row.subtitle ?? "Captured call"}
              </p>
            </div>
          </div>

          {/* Quick Copy Action */}
          <div className="flex items-center gap-1 shrink-0">
            <button
              onClick={() => void triggerCopy("full", JSON.stringify(row, null, 2))}
              className="action-button size-7 p-0 sm:size-auto sm:px-2 sm:py-1 text-[11px]"
              title="Copy entire call event as JSON"
              aria-label={copiedKey === "full" ? "Event JSON copied" : "Copy full event JSON"}
            >
              {copiedKey === "full" ? <Check aria-hidden className="size-3.5 text-ok" /> : <Copy aria-hidden className="size-3.5" />}
              <span className="hidden sm:inline">{copiedKey === "full" ? "Copied JSON" : "Copy JSON"}</span>
            </button>
          </div>
        </div>

        {/* Status Callout Banner for Errors */}
        {isError && (
          <div className="mt-2.5 flex items-start gap-2 rounded-md border border-warn/30 bg-warn/10 p-2 text-xs text-warn" role="alert">
            <AlertTriangle aria-hidden className="size-4 shrink-0 mt-0.5" />
            <div className="min-w-0 flex-1">
              <p className="font-semibold">Tool reported an error outcome</p>
              <p className="mt-0.5 text-[11px] opacity-90">
                Inspect the result payload below for stderr or error exception details.
              </p>
            </div>
          </div>
        )}

        {/* 2. Key Metrics Grid */}
        <dl className="mt-3 mb-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
          <MetricTile
            label={timingSource === "proxy-monotonic" ? "Proxy Round Trip" : timingSource === "source-reported" ? "Tool Duration" : "Tool Duration"}
            value={row.durationMs === undefined ? "Not available" : formatDuration(row.durationMs)}
            figure
          />
          <MetricTile
            label="Observed Interval"
            value={capturedMs === undefined ? "Not available" : formatDuration(capturedMs)}
            figure
          />
          <MetricTile
            label="Response JSON"
            value={bytes === undefined ? "Not available" : formatBytes(bytes)}
            figure
          />
          <MetricTile
            label="Captured At"
            value={formatTime(row.startedAt)}
            figure
          />
        </dl>

        {/* 3. Tab Buttons */}
        <div role="group" aria-label="Inspector view" className="flex flex-wrap gap-x-4 border-t border-rule/50 pt-2">
          {tabs.map((item) => (
            <button
              key={item.id}
              aria-pressed={tab === item.id}
              onClick={() => setActiveTab(item.id)}
              className={`-mb-px border-b-2 pb-1.5 pt-0.5 text-xs font-medium transition-colors ${
                tab === item.id
                  ? "border-accent text-accent font-semibold"
                  : "border-transparent text-ink-2 hover:text-ink"
              }`}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      {/* Search Input within payload */}
      {tab !== "diff" && tab !== "journey" && (
        <div className="shrink-0 border-b border-rule bg-paper/50 px-4 py-2">
          <label className="relative flex items-center gap-2 rounded-md border border-rule-strong bg-sheet px-2.5 py-1.5 shadow-2xs">
            <Search aria-hidden className="size-3.5 text-ink-3" />
            <input
              aria-label="Find in captured payload"
              placeholder="Filter or search in payload…"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              className="min-w-0 flex-1 bg-transparent text-xs text-ink placeholder:text-ink-3 focus:outline-hidden"
            />
            {query && (
              <button
                onClick={() => setQuery("")}
                aria-label="Clear search"
                className="text-[10px] text-ink-3 hover:text-ink"
              >
                Clear
              </button>
            )}
            <span aria-live="polite" className="shrink-0 font-mono text-[10px] text-ink-2">
              {matches === undefined ? "" : matches === 0 ? "No matches" : `${matches} found`}
            </span>
          </label>
        </div>
      )}

      {/* 4. Tab Contents */}
      <div className="min-h-0 flex-1 select-text overflow-y-auto">
        {/* Ambiguous or Unmatched Warnings */}
        {correlation?.quality === "ambiguous" && (
          <p className="mx-4 mt-3 rounded-md border border-rule bg-paper p-3 text-xs leading-relaxed text-warn">
            Ambiguous pairing. Several overlapping calls share the same identity. Results and durations are not assigned to individual requests.
          </p>
        )}
        {!row.request && row.response && (
          <p className="mx-4 mt-3 rounded-md border border-rule bg-paper p-3 text-xs leading-relaxed text-ink-2">
            Response captured without a matched request. Its start time and request → response interval are unavailable.
          </p>
        )}
        {row.request && row.status === "unknown" && correlation?.quality === "unmatched" && (
          <p className="mx-4 mt-3 rounded-md border border-rule bg-paper p-3 text-xs leading-relaxed text-ink-2">
            No reliable session or call identity was available. A later result cannot be paired with this request safely.
          </p>
        )}
        {row.response?.correlation?.synthetic && (
          <p className="mx-4 mt-3 rounded-md border border-rule bg-paper p-3 text-xs leading-relaxed text-ink-2">
            The agent ended the turn or session before a result was matched. This interruption is synthesized; no tool duration is measured.
          </p>
        )}

        {/* Tab: Patch / Diff */}
        {tab === "diff" && diffText ? (
          <div className="p-4 sm:p-6">
            {diffEvent && <CaptureInfo event={diffEvent} />}
            <DiffViewer text={diffText} filename={diffFilename(diffText)} />
          </div>
        ) : tab === "journey" ? (
          /* Tab: Timing & Execution Journey */
          <div className="p-4 sm:p-6">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-ink-3">Execution Journey</h3>
            <p className="mt-1 text-xs text-ink-2">
              Step-by-step telemetry trace showing when the request was dispatched, observed, executed, and completed.
            </p>
            <div className="mt-4">
              <CallJourneyTimeline row={row} intervalMs={capturedMs} />
            </div>

            {historical && (
              <div className="mt-6 border-t border-rule pt-4">
                <h4 className="text-xs font-semibold text-ink">Historical Policy Decisions</h4>
                <div className="mt-3">
                  <Journey row={row} />
                </div>
              </div>
            )}
          </div>
        ) : tab === "raw" ? (
          /* Tab: Raw Event JSON */
          <div className="p-4 sm:p-6">
            <div className="mb-3 flex items-center justify-between">
              <p className="text-xs text-ink-2">Complete normalized schema row with all metadata and raw payloads.</p>
              <button
                onClick={() => void triggerCopy("raw", JSON.stringify(row, null, 2))}
                className="action-button text-[11px]"
              >
                {copiedKey === "raw" ? <Check aria-hidden className="size-3 text-ok" /> : <Copy aria-hidden className="size-3" />}
                {copiedKey === "raw" ? "Copied" : "Copy Raw JSON"}
              </button>
            </div>
            <pre className="json-code rounded-lg border border-rule bg-paper p-3 font-mono text-[11px] leading-relaxed">
              <JsonView text={JSON.stringify(row, null, 2)} query={query} />
            </pre>
          </div>
        ) : (
          /* Tab: Arguments & Result (Primary) */
          <div className="space-y-4 p-4 sm:p-5">
            {blocks.map(({ label, event, payload, isInput }) => {
              const formattedPayload = pretty(payload);
              const copyId = `block-${event.seq}`;
              return (
                <section
                  key={event.seq}
                  className="payload-section overflow-hidden rounded-lg border border-rule bg-sheet shadow-2xs"
                  aria-label={label}
                >
                  <div className="payload-label flex items-center justify-between border-b border-rule bg-paper px-3 py-2 text-xs">
                    <span className="flex items-center gap-2 font-semibold text-ink">
                      {isInput ? <ArrowRight aria-hidden className="size-3.5 text-accent" /> : <CornerDownRight aria-hidden className="size-3.5 text-ok" />}
                      {label}
                      <span className="rounded bg-sheet px-1.5 py-0.2 font-mono text-[10px] text-ink-3 font-normal border border-rule">
                        #{event.seq}
                      </span>
                    </span>
                    <div className="flex items-center gap-2">
                      <span className="text-[11px] text-ink-2">
                        {event.direction === "client->server" ? "Request" : event.direction === "server->client" ? "Response" : "Observation"}
                      </span>
                      <button
                        onClick={() => void triggerCopy(copyId, formattedPayload)}
                        className="icon-button size-6"
                        title={`Copy ${label}`}
                        aria-label={copiedKey === copyId ? "Copied" : `Copy ${label}`}
                      >
                        {copiedKey === copyId ? <Check aria-hidden className="size-3 text-ok" /> : <Copy aria-hidden className="size-3" />}
                      </button>
                    </div>
                  </div>
                  <CaptureInfo event={event} />
                  <pre className="json-code max-h-[480px] overflow-auto p-3 font-mono text-[11px] leading-relaxed select-text">
                    <JsonView text={formattedPayload} query={query} />
                  </pre>
                </section>
              );
            })}

            {row.kind === "tool" && !row.response && (
              <p className="px-2 py-3 text-sm text-ink-2">
                {row.status === "pending"
                  ? "No response received yet. Duration and response size remain unavailable."
                  : row.status === "unknown"
                  ? "No result can be matched safely to this request. Inspect unassigned responses in Event log."
                  : "No tool result was captured for this observation."}
              </p>
            )}

            {/* Diagnostic and Capture Provenance Disclosure */}
            <details className="rounded-lg border border-rule bg-paper/40 p-3 text-xs text-ink-2">
              <summary className="w-fit cursor-pointer font-medium text-ink hover:underline">
                Capture & Provenance Details
              </summary>
              <dl className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
                <Field label="Recording" value={evidence?.recordingId ?? evidence?.sessionId ?? "Not available"} />
                <Field label="Source" value={evidence?.source ?? "Legacy / unknown"} />
                <Field label="Agent session" value={evidence?.agentSessionId ?? "Not available"} />
                <Field label="Connection" value={evidence?.connectionId ?? "Not available"} />
                <Field label="Call ID" value={evidence?.callId ?? evidence?.rpcId ?? "Not available"} />
                <Field
                  label="Native call ID"
                  value={
                    evidence?.nativeCallId === undefined
                      ? "Not available"
                      : `${evidence.nativeCallId.type} · ${
                          evidence.nativeCallId.type === "string"
                            ? JSON.stringify(evidence.nativeCallId.value)
                            : evidence.nativeCallId.type === "null"
                            ? "null"
                            : evidence.nativeCallId.value
                        }`
                  }
                />
                <Field
                  label="Pairing"
                  value={correlation ? `${correlation.method} · ${correlation.quality}` : "Legacy / unspecified"}
                />
                <Field
                  label="Request evidence"
                  value={
                    row.response?.correlation?.requestSeq
                      ? `#${row.response.correlation.requestSeq}`
                      : row.request
                      ? `#${row.request.seq}`
                      : "Not available"
                  }
                />
                {!!evidence?.metadataRedactedFields?.length && (
                  <Field label="Metadata redacted" value={evidence.metadataRedactedFields.join(", ")} />
                )}
              </dl>
              <p className="mt-3 text-[11px] leading-relaxed text-ink-3">
                Payloads may be normalized, omitted or truncated by the adapter. Response JSON is the retained envelope size, not original tool output or model context usage. Missing timing is not a measured zero.
              </p>
            </details>
          </div>
        )}
      </div>
    </section>
  );
}

function StatusBadge({ status }: { status: Row["status"] }) {
  if (status === "ok") {
    return (
      <span className="inline-flex items-center gap-1 rounded bg-ok/10 px-1.5 py-0.5 text-[10px] font-medium text-ok">
        <CheckCircle2 aria-hidden className="size-3" />
        Ok
      </span>
    );
  }
  if (status === "error") {
    return (
      <span className="inline-flex items-center gap-1 rounded bg-warn/15 px-1.5 py-0.5 text-[10px] font-semibold text-warn">
        <AlertTriangle aria-hidden className="size-3" />
        Error
      </span>
    );
  }
  if (status === "pending") {
    return (
      <span className="inline-flex items-center gap-1 rounded bg-accent/15 px-1.5 py-0.5 text-[10px] font-medium text-accent">
        <Clock aria-hidden className="size-3" />
        Pending
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 rounded bg-paper px-1.5 py-0.5 text-[10px] font-medium text-ink-2 border border-rule">
      {statusText(status)}
    </span>
  );
}

function MetricTile({ label, value, figure }: { label: string; value: string; figure?: boolean }) {
  return (
    <div className="rounded-md border border-rule bg-paper p-2">
      <dt className="text-[10px] font-medium text-ink-2">{label}</dt>
      <dd className={`mt-0.5 text-xs font-semibold text-ink truncate ${figure ? "fig font-mono" : ""}`}>
        {value}
      </dd>
    </div>
  );
}

function CallJourneyTimeline({ row, intervalMs }: { row: Row; intervalMs?: number }) {
  const reqTime = row.request ? formatTime(row.request.timestamp) : undefined;
  const resTime = row.response ? formatTime(row.response.timestamp) : undefined;

  return (
    <div className="relative pl-6 space-y-4 before:absolute before:left-2 before:top-2 before:bottom-2 before:w-0.5 before:bg-rule">
      {/* 1. Request dispatched */}
      <div className="relative flex items-start gap-3">
        <span className="absolute -left-6 top-1 size-4 rounded-full border-2 border-accent bg-sheet flex items-center justify-center">
          <span className="size-1.5 rounded-full bg-accent" />
        </span>
        <div className="text-xs">
          <p className="font-semibold text-ink">Request Dispatched by Agent</p>
          <p className="text-[11px] text-ink-2">
            Tool <strong className="text-ink">{row.title}</strong> initiated {reqTime ? `at ${reqTime}` : ""}
          </p>
          {row.request?.seq && (
            <span className="mt-1 inline-block font-mono text-[10px] text-ink-3">Event #{row.request.seq}</span>
          )}
        </div>
      </div>

      {/* 2. Tool Execution */}
      <div className="relative flex items-start gap-3">
        <span className="absolute -left-6 top-1 size-4 rounded-full border-2 border-rule-strong bg-paper flex items-center justify-center">
          <Clock aria-hidden className="size-2.5 text-ink-2" />
        </span>
        <div className="text-xs">
          <p className="font-semibold text-ink">Tool Execution</p>
          <p className="text-[11px] text-ink-2">
            Tool duration: <strong className="text-ink">{row.durationMs !== undefined ? formatDuration(row.durationMs) : "Unknown"}</strong>
            {intervalMs !== undefined && (
              <span className="text-ink-3"> (observed span: {formatDuration(intervalMs)})</span>
            )}
          </p>
        </div>
      </div>

      {/* 3. Response Received */}
      <div className="relative flex items-start gap-3">
        <span className={`absolute -left-6 top-1 size-4 rounded-full border-2 flex items-center justify-center ${
          row.status === "error" ? "border-warn bg-warn/20" : "border-ok bg-ok/20"
        }`}>
          <span className={`size-1.5 rounded-full ${row.status === "error" ? "bg-warn" : "bg-ok"}`} />
        </span>
        <div className="text-xs">
          <p className={`font-semibold ${row.status === "error" ? "text-warn" : "text-ink"}`}>
            {row.status === "error" ? "Tool Error Returned" : "Tool Response Completed"}
          </p>
          <p className="text-[11px] text-ink-2">
            Status: <strong className="text-ink">{statusText(row.status)}</strong> {resTime ? `at ${resTime}` : ""}
          </p>
          {row.response?.seq && (
            <span className="mt-1 inline-block font-mono text-[10px] text-ink-3">Event #{row.response.seq}</span>
          )}
        </div>
      </div>
    </div>
  );
}

function argumentsOf(payload: unknown): unknown {
  if (!payload || typeof payload !== "object") return payload;
  const params = (payload as { params?: { arguments?: unknown } }).params;
  return params?.arguments ?? payload;
}

function resultOf(payload: unknown): unknown {
  if (!payload || typeof payload !== "object") return payload;
  if ("result" in payload) return payload.result;
  if ("error" in payload) return payload.error;
  return payload;
}

function pretty(value: unknown): string {
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function* textCandidates(payload: unknown): Generator<string> {
  if (!payload) return;
  if (typeof payload === "string") {
    yield payload;
    return;
  }
  if (typeof payload === "object") {
    for (const val of Object.values(payload)) {
      if (typeof val === "string") yield val;
      else if (val && typeof val === "object") {
        yield* textCandidates(val);
      }
    }
  }
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-[10px] font-medium text-ink-2">{label}</dt>
      <dd className="mt-0.5 break-words font-mono text-xs font-normal text-ink [overflow-wrap:anywhere]">
        {value}
      </dd>
    </div>
  );
}

/** The call as a journey: where it came from, what policy said, who decided, what came back. */
function Journey({ row }: { row: Row }) {
  const stops = journeyStops(row);

  const dot: Record<Tone, string> = {
    done: "border-ink bg-ink",
    alert: "border-signal bg-signal",
    held: "border-ink bg-yellow",
    warn: "border-warn bg-warn",
    quiet: "border-rule-strong bg-sheet",
  };

  return (
    <ol className="relative space-y-4 pl-7" aria-label="Journey of this call">
      {stops.map((stop, index) => (
        <li key={index} className="relative text-xs">
          <span
            aria-hidden
            className={`absolute -left-7 top-1 size-3 rounded-full border-2 ${dot[stop.tone]}`}
          />
          {index < stops.length - 1 && (
            <span
              aria-hidden
              className="absolute -left-5.5 top-4 h-full w-0.5 bg-rule"
            />
          )}
          <p className="font-semibold text-ink">{stop.title}</p>
          <p className="text-ink-2">{stop.text}</p>
          {stop.detail && <p className="mt-0.5 text-[11px] text-ink-3">{stop.detail}</p>}
        </li>
      ))}
    </ol>
  );
}
