import { useEffect, useState } from "react";
import { Download, Copy, Check, X, Sparkles } from "lucide-react";
import type { Session, Row } from "./types";

/**
 * Everything in the report that originates from the agent or an MCP server
 * (tool names, methods, ids...) is untrusted: a malicious server could name a
 * tool `<script>...</script>`. Escape it before it is put into HTML.
 */
function esc(value: unknown): string {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

interface Props {
  isOpen: boolean;
  onClose: () => void;
  session: Session | null;
  rows: Row[];
}

export function ExportModal({ isOpen, onClose, session, rows }: Props) {
  const [copied, setCopied] = useState(false);

  // Esc closes the modal. Capture phase + stopPropagation so it does not also
  // reach the approval dock, where Esc means "block".
  useEffect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        onClose();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const totalTools = rows.filter((r) => r.kind === "tool").length;
  const totalErrors = rows.filter((r) => r.status === "error" || r.status === "blocked").length;

  const generateReportHtml = () => {
    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <!-- Defense in depth: the report is static, so no script or network access at all. -->
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'" />
  <title>Agent Sentinel Audit Report - ${esc(session?.id || "session")}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, monospace; background: #0b0e14; color: #e6e9f2; margin: 40px auto; max-width: 900px; padding: 0 20px; }
    h1 { font-size: 24px; color: #7aa2ff; margin-bottom: 4px; }
    .badge { display: inline-block; padding: 4px 10px; border-radius: 999px; font-size: 12px; background: #212837; color: #a3adc2; }
    .card { background: #10141c; border: 1px solid #212837; border-radius: 12px; padding: 20px; margin-top: 20px; }
    .stats { display: flex; gap: 24px; margin-top: 15px; }
    .stat-val { font-size: 20px; font-weight: bold; color: #fff; }
    .event-row { padding: 12px; border-bottom: 1px solid #161b25; display: flex; justify-content: space-between; font-family: monospace; font-size: 13px; }
    .event-ok { color: #34d399; }
    .event-err { color: #f87171; }
  </style>
</head>
<body>
  <h1>🛡️ Agent Sentinel Session Audit</h1>
  <div class="badge">Session ID: ${esc(session?.id || "unknown")}</div>
  <div class="badge">Exported: ${new Date().toISOString()}</div>

  <div class="card">
    <h3>Session Summary</h3>
    <div class="stats">
      <div><div>Total Events</div><div class="stat-val">${rows.length}</div></div>
      <div><div>Tool Calls</div><div class="stat-val">${totalTools}</div></div>
      <div><div>Errors / Intercepts</div><div class="stat-val">${totalErrors}</div></div>
    </div>
  </div>

  <div class="card">
    <h3>Activity Timeline</h3>
    ${rows.map((r) => `
      <div class="event-row">
        <span>${esc(r.title)} ${r.subtitle ? `<small style="color:#5b667d">(${esc(r.subtitle)})</small>` : ""}</span>
        <span class="${r.status === "ok" ? "event-ok" : "event-err"}">${esc(r.status.toUpperCase())} ${r.durationMs ? `(${esc(r.durationMs)}ms)` : ""}</span>
      </div>
    `).join("")}
  </div>
</body>
</html>`;
  };

  const handleDownload = () => {
    const html = generateReportHtml();
    const blob = new Blob([html], { type: "text/html" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `sentinel-audit-${session?.id || "session"}.html`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(generateReportHtml());
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 p-4">
      <div className="w-full max-w-lg overflow-hidden rounded-sm border border-rule-strong bg-sheet">
        <div className="flex items-center justify-between border-b border-rule-strong bg-paper px-5 py-4">
          <div className="flex items-center gap-2">
            <Sparkles className="size-4 text-ink" />
            <h3 className="font-semibold text-sm text-ink">Export Standalone Audit Deliverable</h3>
          </div>
          <button onClick={onClose} className="text-ink-2 hover:text-ink">
            <X className="size-4" />
          </button>
        </div>

        <div className="p-6 text-xs text-ink-2 space-y-4">
          <p>
            Generates a self-contained, zero-dependency HTML file containing the complete execution timeline,
            tool call metrics, and audit evidence. Perfect for attaching to GitHub Pull Requests or sharing with your engineering team.
          </p>

          <div className="rounded-sm border border-rule bg-paper p-4 space-y-2">
            <div className="flex justify-between">
              <span className="text-ink-2">Session ID:</span>
              <span className="font-mono text-ink">{session?.id || "active"}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-ink-2">Total Recorded Events:</span>
              <span className="font-mono text-ink">{rows.length}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-ink-2">Format:</span>
              <span className="font-mono text-ok">Single-File HTML (Portable)</span>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-rule bg-paper px-6 py-3.5">
          <button
            onClick={handleCopy}
            className="flex items-center gap-1.5 rounded-sm border border-rule-strong bg-paper px-3 py-1.5 text-xs text-ink hover:bg-paper"
          >
            {copied ? <Check className="size-3.5 text-ok" /> : <Copy className="size-3.5" />}
            <span>{copied ? "Copied" : "Copy HTML"}</span>
          </button>

          <button
            onClick={handleDownload}
            className="flex items-center gap-1.5 rounded-sm border-2 border-ink bg-ink px-4 py-1.5 text-xs font-semibold text-white hover:bg-ink-2"
          >
            <Download className="size-3.5" />
            <span>Download .html</span>
          </button>
        </div>
      </div>
    </div>
  );
}
