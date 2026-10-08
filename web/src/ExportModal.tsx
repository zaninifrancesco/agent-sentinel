import { useEffect, useState } from "react";
import { Download, Copy, Check, X, Sparkles } from "lucide-react";
import type { Session, Row, SentinelConfig } from "./types";
import { buildReportHtml } from "./report";
import { computeMetrics } from "./metrics";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  session: Session | null;
  rows: Row[];
  config: SentinelConfig;
}

export function ExportModal({ isOpen, onClose, session, rows, config }: Props) {
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

  const m = computeMetrics(rows);
  const generateReportHtml = () => buildReportHtml({ session, rows, config, exportedAt: new Date() });

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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-[#15140f]/60 p-4">
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

        <div className="space-y-4 p-6 text-xs text-ink-2">
          <p>
            One self-contained HTML file: what policy and you decided on each call, the full timetable with every
            call's journey, and the session numbers. It opens offline, runs no script, and follows the reader's
            light or dark theme.
          </p>

          <div className="space-y-2 rounded-sm border border-rule bg-paper p-4">
            <Line label="Session" value={session?.id || "active"} />
            <Line label="Events" value={String(rows.length)} />
            <Line label="Tool calls" value={String(m.calls)} />
            <Line label="Refused" value={String(m.refused)} alert={m.refused > 0} />
            {m.waiting > 0 && <Line label="Still waiting for a human" value={String(m.waiting)} />}
          </div>

          <p>
            Payloads are left out: the report shows each call's arguments as summarised in the cockpit, not the full
            request and response bodies. Cost and tokens are written as estimates.
          </p>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-rule bg-paper px-6 py-3.5">
          <button
            onClick={handleCopy}
            className="flex items-center gap-1.5 rounded-sm border border-rule-strong bg-paper px-3 py-1.5 text-xs text-ink hover:border-ink"
          >
            {copied ? <Check className="size-3.5 text-ok" /> : <Copy className="size-3.5" />}
            <span>{copied ? "Copied" : "Copy HTML"}</span>
          </button>

          <button
            onClick={handleDownload}
            className="flex items-center gap-1.5 rounded-sm border-2 border-ink bg-ink px-4 py-1.5 text-xs font-semibold text-sheet hover:bg-ink-2"
          >
            <Download className="size-3.5" />
            <span>Download .html</span>
          </button>
        </div>
      </div>
    </div>
  );
}

function Line({ label, value, alert }: { label: string; value: string; alert?: boolean }) {
  return (
    <div className="flex justify-between">
      <span className="text-ink-2">{label}</span>
      <span className={`fig font-mono ${alert ? "font-semibold text-signal" : "text-ink"}`}>{value}</span>
    </div>
  );
}
