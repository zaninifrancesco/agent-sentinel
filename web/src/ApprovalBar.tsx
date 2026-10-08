import { useState, useEffect } from "react";
import { ShieldAlert, CheckCircle2, Ban, MessageSquarePlus, CornerDownLeft, Sparkles } from "lucide-react";
import type { Row } from "./types";

interface Props {
  pendingRow: Row | null;
  /** How many MORE calls are waiting behind this one. */
  more: number;
  busy: boolean;
  error: string | null;
  onApprove: (row: Row, feedback?: string) => void;
  onBlock: (row: Row, feedback?: string) => void;
}

export function ApprovalBar({ pendingRow, more, busy, error, onApprove, onBlock }: Props) {
  const [feedback, setFeedback] = useState("");
  const [showFeedbackInput, setShowFeedbackInput] = useState(false);
  const key = pendingRow?.key;

  // A new call to decide on starts with a clean steering note.
  useEffect(() => {
    setFeedback("");
    setShowFeedbackInput(false);
  }, [key]);

  useEffect(() => {
    if (!pendingRow || busy) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented) return; // an overlay already handled this key
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
        e.preventDefault();
        onApprove(pendingRow, feedback.trim() || undefined);
      } else if (e.key === "Escape" && !showFeedbackInput) {
        e.preventDefault();
        onBlock(pendingRow, feedback.trim() || undefined);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [pendingRow, busy, feedback, showFeedbackInput, onApprove, onBlock]);

  if (!pendingRow) return null;
  const why = pendingRow.request?.reason;
  const rule = pendingRow.request?.rule;

  return (
    // A docked bar in the page layout (not floating): it pushes the timeline up
    // instead of covering its last rows.
    <div className="shrink-0 border-t border-amber-500/40 bg-ink-900 px-5 py-3 animate-in fade-in slide-in-from-bottom-2 duration-200">
      <div className="mx-auto flex w-full max-w-3xl flex-col rounded-xl border border-amber-500/30 bg-ink-900 p-4 ring-1 ring-amber-500/10">
        {/* Banner header */}
        <div className="flex items-center justify-between gap-3 pb-3 border-b border-ink-800">
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="relative flex size-3 shrink-0">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-400 opacity-75"></span>
              <span className="relative inline-flex size-3 rounded-full bg-amber-500"></span>
            </span>
            <ShieldAlert className="size-5 shrink-0 text-amber-400" />
            <div className="min-w-0">
              <div className="text-xs font-semibold uppercase tracking-wider text-amber-400">
                The agent is paused — waiting for you
                {more > 0 && <span className="ml-2 text-ink-400 normal-case">+{more} more waiting</span>}
              </div>
              <div className="truncate font-mono text-xs font-medium text-ink-100">
                Tool call: <span className="text-accent">{pendingRow.title}</span>
              </div>
            </div>
          </div>

          <span className="shrink-0 rounded-full border border-amber-500/30 bg-amber-500/10 px-2 py-0.5 font-mono text-[10px] uppercase text-amber-300">
            {pendingRow.risk !== "none" ? `${pendingRow.risk} risk` : "review"}
          </span>
        </div>

        {/* Why the policy stopped this call */}
        {(rule || why) && (
          <div className="mt-3 rounded-lg border border-ink-800 bg-ink-950 p-2.5 text-xs">
            {rule && <span className="mr-2 rounded bg-ink-800 px-1.5 py-0.5 font-mono text-[10px] text-amber-300">{rule}</span>}
            <span className="break-words text-ink-300">{why}</span>
          </div>
        )}

        {/* Steer guidance input */}
        {showFeedbackInput && (
          <div className="mt-3 flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-950 p-2">
            <Sparkles className="size-4 text-accent shrink-0" />
            <input
              type="text"
              autoFocus
              value={feedback}
              onChange={(e) => setFeedback(e.target.value)}
              placeholder="Note for the agent (e.g. 'Use dry-run first', 'Avoid touching dist')..."
              className="w-full bg-transparent text-xs text-ink-100 placeholder:text-ink-500 focus:outline-none"
            />
          </div>
        )}

        {error && (
          <div role="alert" className="mt-3 rounded-lg border border-rose-500/40 bg-rose-950/40 p-2 text-xs text-rose-300">
            {error}
          </div>
        )}

        {/* Action Controls */}
        <div className="mt-3 flex items-center justify-between gap-3">
          <button
            onClick={() => setShowFeedbackInput((prev) => !prev)}
            className="flex items-center gap-1.5 text-xs text-ink-400 hover:text-ink-200 transition-colors"
          >
            <MessageSquarePlus className="size-3.5" />
            <span>{showFeedbackInput ? "Hide note" : "Add a note for the agent"}</span>
          </button>

          <div className="flex items-center gap-2">
            <button
              disabled={busy}
              onClick={() => onBlock(pendingRow, feedback.trim() || undefined)}
              className="flex items-center gap-1.5 rounded-lg border border-rose-500/30 bg-rose-950/40 px-3 py-1.5 text-xs font-medium text-rose-300 transition-colors hover:bg-rose-900/50 hover:text-rose-100 disabled:opacity-50"
            >
              <Ban className="size-3.5" />
              <span>Reject</span>
              <kbd className="ml-1 rounded border border-rose-500/40 bg-rose-950/60 px-1 py-0.2 font-mono text-[10px]">
                Esc
              </kbd>
            </button>

            <button
              disabled={busy}
              onClick={() => onApprove(pendingRow, feedback.trim() || undefined)}
              className="flex items-center gap-1.5 rounded-lg border border-emerald-500/40 bg-emerald-600 px-3.5 py-1.5 text-xs font-medium text-white shadow-lg shadow-emerald-950/50 transition-all hover:bg-emerald-500 active:scale-95 disabled:opacity-50"
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
