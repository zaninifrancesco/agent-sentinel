import type { SentinelEvent } from "./types";
import { formatBytes } from "./format";

export function observedUnit(unit: string | undefined): string {
  return unit === "utf8-text" ? "text UTF-8" : unit === "json-value" ? "source JSON" : unit === "mcp-frame" ? "MCP frame" : unit ?? "unit unknown";
}

export function captureSummary(event: SentinelEvent): string {
  const c = event.capture;
  if (!c) return "Capture quality not recorded · original size unknown";
  const changes = [...new Set((c.changes ?? []).filter((change) => change.kind !== "normalized").map((change) => change.kind))];
  const state = c.completeness === "omitted" ? "Content omitted" : c.completeness === "partial" ? "Partial capture" : c.completeness === "complete" ? "Complete observed value" : "Source coverage unknown";
  return [state, ...changes, c.observedBytes === undefined ? "Observed size unknown" : `${formatBytes(c.observedBytes)} observed ${observedUnit(c.observedUnit)}`, `${formatBytes(c.storedBytes)} stored JSON`].join(" · ");
}
