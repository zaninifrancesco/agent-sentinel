import { useState } from "react";
import type { Row, SentinelEvent } from "./types";
import { StatusIcon } from "./Timeline";
import { formatBytes, formatDuration, formatTime } from "./format";
import { responseSize } from "./profile";
import { journeyStops, statusText, type Tone } from "./journey";
import { DiffViewer, diffFilename, extractDiff, isDiff } from "./DiffViewer";
import { Check, Copy } from "lucide-react";

type Tab = "payload" | "diff" | "raw";

export function Detail({ row }: { row: Row | null }) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "error">("idle");
  const [activeTab, setActiveTab] = useState<Tab>("payload");
  if (!row) return <section className="grid min-h-0 flex-1 place-items-center bg-paper p-8 text-sm text-ink-2">
    <div className="max-w-xs"><h2 className="font-semibold text-ink">Inspect the evidence</h2><p className="mt-2">Select a captured call to see its arguments, result and available measurements.</p></div>
  </section>;

  const blocks: { label: string; event: SentinelEvent; payload: unknown }[] = [];
  if (row.request) blocks.push({ label: row.kind === "tool" ? "Arguments" : "Request", event: row.request, payload: row.kind === "tool" ? argumentsOf(row.request.payload) : row.request.payload });
  if (row.response) blocks.push({ label: row.status === "error" ? "Error / result" : "Result", event: row.response, payload: resultOf(row.response.payload) });
  if (row.event) blocks.push({ label: "Captured payload", event: row.event, payload: row.event.payload });

  let diffText: string | null = null;
  for (const { event } of blocks) for (const candidate of textCandidates(event.payload)) {
    if (diffText === null && isDiff(candidate)) diffText = extractDiff(candidate);
  }
  const tab = activeTab === "diff" && diffText === null ? "payload" : activeTab;
  const bytes = responseSize(row);
  const historical = !!row.request?.decision || !!row.resolution || row.risk !== "none";
  const tabs: { id: Tab; label: string }[] = [
    { id: "payload", label: row.kind === "tool" ? "Arguments & result" : "Payload" },
    ...(diffText !== null ? [{ id: "diff" as Tab, label: "Patch" }] : []),
    { id: "raw", label: "Event JSON" },
  ];
  const copy = async () => {
    try { await navigator.clipboard.writeText(JSON.stringify(row, null, 2)); setCopyState("copied"); }
    catch { setCopyState("error"); }
  };

  return <section className="flex min-h-0 flex-1 flex-col overflow-hidden bg-paper" aria-label="Selected event details">
    <div className="shrink-0 border-b border-rule-strong px-4 pt-5 sm:px-6">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0"><h2 className="flex items-center gap-2 text-xl font-bold"><StatusIcon status={row.status} /><span className="truncate">{row.title}</span></h2></div>
        <button onClick={() => void copy()} className="action-button shrink-0">{copyState === "copied" ? <Check aria-hidden className="size-3.5" /> : <Copy aria-hidden className="size-3.5" />} {copyState === "copied" ? "Copied" : "Copy event"}</button>
      </div>
      {copyState === "error" && <p role="status" className="mt-2 text-xs text-warn">Could not copy this event. You can select its text in Event JSON.</p>}
      <dl className="my-4 grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-4">
        <Field label="Captured at" value={formatTime(row.startedAt)} figure />
        <Field label="Duration" value={row.durationMs === undefined ? "Not available" : formatDuration(row.durationMs)} figure />
        <Field label="Response JSON" value={bytes === undefined ? "Not available" : formatBytes(bytes)} figure />
        <Field label="Status" value={row.kind === "tool" && row.status === "ok" && !row.response ? "Recorded without result" : statusText(row.status)} />
      </dl>
      <div role="group" aria-label="Inspector view" className="flex flex-wrap gap-x-5">
        {tabs.map((item) => <button key={item.id} aria-pressed={tab === item.id} onClick={() => setActiveTab(item.id)}
          className={`-mb-px border-b-2 pb-2.5 pt-1 text-sm font-medium ${tab === item.id ? "border-ink text-ink" : "border-transparent text-ink-2 hover:text-ink"}`}>{item.label}</button>)}
      </div>
    </div>
    <div className="min-h-0 flex-1 select-text overflow-y-auto">
      {tab === "diff" && diffText ? <div className="p-4 sm:p-6"><DiffViewer text={diffText} filename={diffFilename(diffText)} /></div> : tab === "raw" ? (
        <div className="p-4 sm:p-6"><p className="mb-3 text-xs text-ink-2">Captured event data, including normalized fields and any historical policy metadata.</p><pre className="overflow-x-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed">{JSON.stringify(row, null, 2)}</pre></div>
      ) : <>
        {blocks.map(({ label, event, payload }) => <section key={event.seq} className="border-b border-rule" aria-label={label}>
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-rule bg-sheet px-4 py-2.5 text-xs sm:px-6"><span className="font-semibold">{label}<span className="fig ml-2 font-normal text-ink-3">#{event.seq}</span></span><span className="text-ink-2">{event.direction}</span></div>
          <pre className="overflow-x-auto whitespace-pre-wrap break-words px-4 py-4 font-mono text-xs leading-relaxed sm:px-6">{pretty(payload)}</pre>
        </section>)}
        {row.kind === "tool" && !row.response && <p className="px-4 py-4 text-sm text-ink-2 sm:px-6">{row.status === "pending" ? "No response received yet. Duration and response size remain unavailable." : "No tool result was captured for this observation."}</p>}
        <details className="border-b border-rule px-4 py-4 text-xs sm:px-6">
          <summary className="w-fit cursor-pointer py-1 font-medium">Capture details</summary>
          <dl className="mt-3 grid grid-cols-2 gap-4"><Field label="Call ID" value={row.request?.rpcId ?? row.response?.rpcId ?? "Not available"} /><Field label="Recorded status" value={row.status} /></dl>
          <p className="mt-3 max-w-prose leading-relaxed text-ink-2">Payloads may be normalized, omitted or truncated by the adapter. Response JSON is the retained envelope size, not original tool output or model context usage. Missing timing is not a measured zero.</p>
        </details>
        {historical && <details className="px-4 py-4 sm:px-6"><summary className="mb-3 w-fit cursor-pointer py-1 text-xs font-medium">Historical policy and call history</summary><Journey row={row} /></details>}
      </>}
    </div>
  </section>;
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

function Field({ label, value, figure }: { label: string; value: string; figure?: boolean }) {
  return <div className="min-w-0"><dt className="text-xs text-ink-2">{label}</dt><dd className={`mt-1 break-words text-sm font-semibold [overflow-wrap:anywhere] ${figure ? "fig" : ""}`}>{value}</dd></div>;
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
