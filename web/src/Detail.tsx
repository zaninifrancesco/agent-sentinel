import { useState } from "react";
import type { Row, SentinelEvent } from "./types";
import { StatusIcon, formatTime } from "./Timeline";
import { DiffViewer, diffFilename, extractDiff, isDiff } from "./DiffViewer";
import { Copy, Check, Code, FileText, Layers, ShieldCheck, AlertCircle } from "lucide-react";

export function Detail({ row }: { row: Row | null }) {
  const [copied, setCopied] = useState(false);
  const [activeTab, setActiveTab] = useState<"payload" | "diff" | "raw">("payload");

  if (!row) {
    return (
      <section className="grid place-items-center bg-ink-950 text-sm text-ink-500">
        <div className="flex flex-col items-center gap-2">
          <Layers className="size-8 text-ink-700" />
          <span>Select an event from the timeline to inspect</span>
          <span className="text-xs text-ink-600">Tip: use ↑ / ↓ or Cmd+K to navigate</span>
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
  const hasDiffContent = diffText !== null;

  const handleCopy = () => {
    navigator.clipboard.writeText(JSON.stringify(row, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <section className="flex flex-col h-full overflow-hidden bg-ink-950">
      {/* Detail Top Header */}
      <div className="border-b border-ink-800 bg-ink-900/60 p-5">
        <div className="flex items-center justify-between gap-3 mb-3">
          <div className="flex items-center gap-3">
            <StatusIcon status={row.status} />
            <h2 className="font-mono text-base font-semibold text-ink-100">{row.title}</h2>
            {row.subtitle && <span className="text-xs text-ink-400">{row.subtitle}</span>}
          </div>

          <button
            onClick={handleCopy}
            className="flex items-center gap-1.5 rounded-md border border-ink-700 bg-ink-800 px-2.5 py-1 text-xs text-ink-300 transition-colors hover:bg-ink-700 hover:text-ink-100"
          >
            {copied ? (
              <>
                <Check className="size-3.5 text-emerald-400" />
                <span className="text-emerald-400">Copied</span>
              </>
            ) : (
              <>
                <Copy className="size-3.5" />
                <span>Copy Event</span>
              </>
            )}
          </button>
        </div>

        {/* Metadata grid */}
        <div className="grid grid-cols-4 gap-4 rounded-xl border border-ink-800 bg-ink-900/90 p-3 font-mono text-xs">
          <div>
            <div className="text-[10px] uppercase text-ink-500">Status</div>
            <div className="text-ink-200 mt-0.5">{row.status}</div>
          </div>
          <div>
            <div className="text-[10px] uppercase text-ink-500">Started</div>
            <div className="text-ink-200 mt-0.5">{formatTime(row.startedAt)}</div>
          </div>
          <div>
            <div className="text-[10px] uppercase text-ink-500">Duration</div>
            <div className="text-ink-200 mt-0.5">{row.durationMs !== undefined ? `${row.durationMs} ms` : "—"}</div>
          </div>
          <div>
            <div className="text-[10px] uppercase text-ink-500">Risk Assessment</div>
            <div className="text-ink-200 mt-0.5 capitalize flex items-center gap-1">
              {row.risk === "critical" || row.risk === "high" ? (
                <AlertCircle className="size-3 text-rose-400" />
              ) : (
                <ShieldCheck className="size-3 text-emerald-400" />
              )}
              {row.risk}
            </div>
          </div>
        </div>

        {/* Policy verdict: what rule fired and what the human decided */}
        {(row.request?.rule || row.resolution) && (
          <div className="mt-3 rounded-xl border border-amber-500/30 bg-amber-950/10 p-3 text-xs">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-[10px] font-semibold uppercase tracking-wider text-amber-400">Policy</span>
              {row.request?.rule && (
                <span className="rounded bg-ink-800 px-1.5 py-0.5 font-mono text-[10px] text-amber-300">
                  {row.request.rule}
                </span>
              )}
              {row.request?.decision && (
                <span className="font-mono text-[10px] uppercase text-ink-400">→ {row.request.decision}</span>
              )}
            </div>
            {row.request?.reason && <p className="mt-1.5 break-words text-ink-300">{row.request.reason}</p>}
            {row.resolution && (
              <p className="mt-1.5 text-ink-200">
                Human verdict: <span className="font-mono font-semibold">{row.resolution.decision}</span>
                {row.resolution.reason ? <span className="text-ink-400"> — “{row.resolution.reason}”</span> : null}
              </p>
            )}
          </div>
        )}

        {/* Tabs */}
        <div className="flex items-center gap-2 mt-4">
          <button
            onClick={() => setActiveTab("payload")}
            className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-colors ${
              activeTab === "payload"
                ? "bg-ink-700 text-ink-100"
                : "text-ink-400 hover:bg-ink-800 hover:text-ink-200"
            }`}
          >
            <Layers className="size-3.5" /> Payload
          </button>

          {hasDiffContent && (
            <button
              onClick={() => setActiveTab("diff")}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-colors ${
                activeTab === "diff"
                  ? "bg-accent/20 text-accent ring-1 ring-accent/40"
                  : "text-accent/80 hover:bg-ink-800"
              }`}
            >
              <Code className="size-3.5" /> Visual Diff
            </button>
          )}

          <button
            onClick={() => setActiveTab("raw")}
            className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-colors ${
              activeTab === "raw"
                ? "bg-ink-700 text-ink-100"
                : "text-ink-400 hover:bg-ink-800 hover:text-ink-200"
            }`}
          >
            <FileText className="size-3.5" /> Raw JSON
          </button>
        </div>
      </div>

      {/* Main Content Area */}
      <div className="flex-1 overflow-y-auto p-5">
        {activeTab === "diff" && diffText ? (
          <DiffViewer text={diffText} filename={diffFilename(diffText)} />
        ) : activeTab === "raw" ? (
          <pre className="overflow-x-auto rounded-xl border border-ink-800 bg-ink-900 p-4 font-mono text-xs leading-relaxed text-ink-200">
            {JSON.stringify(row, null, 2)}
          </pre>
        ) : (
          <div className="space-y-6">
            {blocks.map(({ label, event }) => (
              <div key={event.seq} className="overflow-hidden rounded-xl border border-ink-800 bg-ink-900 shadow-md">
                <div className="flex items-center justify-between border-b border-ink-800 bg-ink-850 px-4 py-2 text-xs">
                  <div className="flex items-center gap-2 text-ink-300 font-medium">
                    <span className="uppercase text-[10px] tracking-wider text-ink-500 font-semibold">{label}</span>
                    <span>·</span>
                    <span className="font-mono text-[11px]">Seq #{event.seq}</span>
                  </div>
                  {event.direction && (
                    <span className="font-mono text-[10px] text-ink-500">{event.direction}</span>
                  )}
                </div>

                <div className="p-4">
                  <pre className="overflow-x-auto font-mono text-xs leading-relaxed text-ink-200 whitespace-pre-wrap">
                    {pretty(event.payload)}
                  </pre>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
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
