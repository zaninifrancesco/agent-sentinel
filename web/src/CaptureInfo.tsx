import type { SentinelEvent } from "./types";
import { captureSummary, observedUnit } from "./capture";
import { formatBytes } from "./format";

export function CaptureInfo({ event }: { event: SentinelEvent }) {
  const capture = event.capture;
  if (!capture) return <p className="border-b border-rule bg-paper px-3 py-2 text-[10px] text-ink-2">{captureSummary(event)}</p>;
  const value = (bytes: number | undefined, unit: string | undefined) => bytes === undefined ? "Not recorded" : `${formatBytes(bytes)} · ${observedUnit(unit)}`;
  return <details className="border-b border-rule bg-paper px-3 py-2 text-[11px] text-ink-2">
    <summary className={`cursor-pointer leading-relaxed ${capture.completeness === "partial" || capture.completeness === "omitted" ? "text-warn" : ""}`}>{captureSummary(event)}</summary>
    <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-3">
      <Measure label="Observed source value" value={value(capture.observedBytes, capture.observedUnit)} />
      <Measure label="Source field" value={capture.observedField ?? "frame"} />
      <Measure label="Normalized JSON" value={formatBytes(capture.normalizedBytes)} />
      <Measure label="Stored payload JSON" value={formatBytes(capture.storedBytes)} />
      {capture.sourceReportedBytes !== undefined && <Measure label="Hook client reported" value={value(capture.sourceReportedBytes, capture.sourceReportedUnit)} />}
      <Measure label="Profile" value={capture.profile} />
      <Measure label="Representation" value={capture.representation} />
    </dl>
    {!!capture.changes?.length && <ul className="mt-3 space-y-1 border-t border-rule pt-2">{capture.changes.map((change, index) => <li key={index}><span className="font-medium text-ink">{change.kind}</span> · {change.stage} · {change.method}{change.count ? ` · ${change.count} matches` : ""}{change.beforeBytes !== undefined && change.afterBytes !== undefined ? ` · ${formatBytes(change.beforeBytes)} → ${formatBytes(change.afterBytes)}` : ""}</li>)}</ul>}
    <p className="mt-3 leading-relaxed">Completeness refers to the stated source value. A normalized hook payload is not an original MCP frame. Source text, source JSON and retained JSON have different units; none measures model context.</p>
  </details>;
}

function Measure({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0"><dt>{label}</dt><dd className="mt-0.5 break-words font-medium text-ink">{value}</dd></div>;
}
