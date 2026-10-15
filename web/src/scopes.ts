import type { Row, SentinelEvent } from "./types";

export function rowEvent(row: Row): SentinelEvent | undefined {
  return row.request ?? row.response ?? row.event;
}

export function scopeKey(row: Row): string {
  const event = rowEvent(row);
  if (!event) return "unattributed";
  const source = event.sourceId ?? event.source ?? "";
  if (event.agentSessionId) return JSON.stringify([source, "session", event.agentSessionId]);
  if (event.connectionId) return JSON.stringify([source, "connection", event.connectionId]);
  return "unattributed";
}

export function recordingScopes(rows: Row[]): { id: string; label: string }[] {
  const scopes = new Map<string, string>();
  for (const row of rows) {
    const e = rowEvent(row);
    const source = e?.source === "claude-hook" ? "Claude Code" : e?.source === "cursor-hook" ? "Cursor" : e?.source === "mcp-stdio" ? "MCP" : e?.source ?? "Source";
    const label = e?.agentSessionId ? `${source} · ${e.agentSessionId}` : e?.connectionId ? `MCP connection · ${e.connectionId}` : "Recording / unidentified session";
    scopes.set(scopeKey(row), label);
  }
  return [...scopes].map(([id, label]) => ({ id, label }));
}

export function filterScope(rows: Row[], scope: string): Row[] {
  return scope ? rows.filter((row) => scopeKey(row) === scope) : rows;
}
