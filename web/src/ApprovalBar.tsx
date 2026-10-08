import { useState, useEffect } from "react";
import { ShieldAlert, CheckCircle2, Ban, MessageSquarePlus, CornerDownLeft, Sparkles } from "lucide-react";
import type { Row } from "./types";

interface Props {
  pendingRow: Row | null;
  onApprove: (key: string, feedback?: string) => void;
  onBlock: (key: string) => void;
}

export function ApprovalBar({ pendingRow, onApprove, onBlock }: Props) {
  const [feedback, setFeedback] = useState("");
  const [showFeedbackInput, setShowFeedbackInput] = useState(false);

  useEffect(() => {
    if (!pendingRow) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented) return; // an overlay already handled this key
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
        e.preventDefault();
        onApprove(pendingRow.key, feedback.trim() || undefined);
        setFeedback("");
        setShowFeedbackInput(false);
      } else if (e.key === "Escape" && !showFeedbackInput) {
        e.preventDefault();
        onBlock(pendingRow.key);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [pendingRow, feedback, showFeedbackInput, onApprove, onBlock]);

  if (!pendingRow) return null;

  return (
    <div className="fixed bottom-6 left-1/2 z-40 -translate-x-1/2 animate-in fade-in slide-in-from-bottom-4 duration-200">
      <div className="flex flex-col overflow-hidden rounded-2xl border border-amber-500/40 bg-ink-900/95 p-4 shadow-2xl backdrop-blur-xl ring-1 ring-amber-500/20 max-w-xl w-[92vw]">
        {/* Banner header */}
        <div className="flex items-center justify-between gap-3 pb-3 border-b border-ink-800">
          <div className="flex items-center gap-2.5">
            <span className="relative flex size-3">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-400 opacity-75"></span>
              <span className="relative inline-flex size-3 rounded-full bg-amber-500"></span>
            </span>
            <ShieldAlert className="size-5 text-amber-400" />
            <div className="min-w-0">
              <div className="text-xs font-semibold uppercase tracking-wider text-amber-400">
                Human-in-the-Loop Intercept
              </div>
              <div className="truncate font-mono text-xs font-medium text-ink-100">
                Action requires review: <span className="text-accent">{pendingRow.title}</span>
              </div>
            </div>
          </div>

          <span className="rounded-full border border-amber-500/30 bg-amber-500/10 px-2 py-0.5 font-mono text-[10px] uppercase text-amber-300">
            {pendingRow.risk !== "none" ? `${pendingRow.risk} risk` : "Awaiting approval"}
          </span>
        </div>

        {/* Steer guidance input */}
        {showFeedbackInput && (
          <div className="mt-3 flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-950 p-2">
            <Sparkles className="size-4 text-accent shrink-0" />
            <input
              type="text"
              autoFocus
              value={feedback}
              onChange={(e) => setFeedback(e.target.value)}
              placeholder="Inject steering advice to agent (e.g. 'Use dry-run first', 'Avoid touching dist')..."
              className="w-full bg-transparent text-xs text-ink-100 placeholder:text-ink-500 focus:outline-none"
            />
          </div>
        )}

        {/* Action Controls */}
        <div className="mt-3 flex items-center justify-between gap-3">
          <button
            onClick={() => setShowFeedbackInput((prev) => !prev)}
            className="flex items-center gap-1.5 text-xs text-ink-400 hover:text-ink-200 transition-colors"
          >
            <MessageSquarePlus className="size-3.5" />
            <span>{showFeedbackInput ? "Hide steering" : "Add steering instruction"}</span>
          </button>

          <div className="flex items-center gap-2">
            <button
              onClick={() => onBlock(pendingRow.key)}
              className="flex items-center gap-1.5 rounded-lg border border-rose-500/30 bg-rose-950/40 px-3 py-1.5 text-xs font-medium text-rose-300 transition-colors hover:bg-rose-900/50 hover:text-rose-100"
            >
              <Ban className="size-3.5" />
              <span>Block Action</span>
              <kbd className="ml-1 rounded border border-rose-500/40 bg-rose-950/60 px-1 py-0.2 font-mono text-[10px]">
                Esc
              </kbd>
            </button>

            <button
              onClick={() => onApprove(pendingRow.key, feedback.trim() || undefined)}
              className="flex items-center gap-1.5 rounded-lg border border-emerald-500/40 bg-emerald-600 px-3.5 py-1.5 text-xs font-medium text-white shadow-lg shadow-emerald-950/50 transition-all hover:bg-emerald-500 active:scale-95"
            >
              <CheckCircle2 className="size-3.5" />
              <span>Approve & Continue</span>
              <span className="flex items-center gap-0.5 ml-1 rounded border border-emerald-400/40 bg-emerald-700/60 px-1 py-0.2 font-mono text-[10px]">
                <CornerDownLeft className="size-2.5" /> ⌘↵
              </span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
