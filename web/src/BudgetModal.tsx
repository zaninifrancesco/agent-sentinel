import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import type { SentinelConfig } from "./types";
import { formatTokens, parseTokens, parseUsd } from "./format";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  config: SentinelConfig;
  /** The cockpit's own estimate of what has been spent so far. */
  spent: { cost: number; tokens: number };
  /** Called with the config the server returns once the limits are saved. */
  onSaved: (config: SentinelConfig) => void;
}

/** Set or remove the session's cost and token limits. */
export function BudgetModal({ isOpen, onClose, config, spent, onSaved }: Props) {
  const [cost, setCost] = useState("");
  const [tokens, setTokens] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const first = useRef<HTMLInputElement>(null);

  // Start from the limits in force each time the window opens.
  useEffect(() => {
    if (!isOpen) return;
    setCost(config.maxCostUsd > 0 ? config.maxCostUsd.toFixed(2) : "");
    setTokens(config.maxTokens > 0 ? String(config.maxTokens) : "");
    setError(null);
    setBusy(false);
    const t = window.setTimeout(() => first.current?.select(), 50);
    return () => window.clearTimeout(t);
    // Only on open: a config refresh while typing must not wipe the fields.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen]);

  // Esc closes this window only. ⌘/Ctrl+Enter is the approval shortcut of the
  // dock: it must never approve the call behind this window.
  useEffect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        onClose();
      } else if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        e.stopPropagation();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const costValue = parseUsd(cost);
  const tokenValue = parseTokens(tokens);
  const valid = costValue !== null && tokenValue !== null;

  let notice: string | null = null;
  if (valid) {
    if (costValue === 0 && tokenValue === 0) {
      notice = "No limit: the agent will never be stopped for spend.";
    } else if ((costValue > 0 && spent.cost >= costValue) || (tokenValue > 0 && spent.tokens >= tokenValue)) {
      notice = "The estimate is already past this limit: the next tool call will wait for your approval.";
    }
  }

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!valid || busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/budget", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ maxCostUsd: costValue, maxTokens: tokenValue }),
      });
      if (!res.ok) {
        setError((await res.text()).trim() || `The server refused the change (${res.status}).`);
        setBusy(false);
        return;
      }
      onSaved((await res.json()) as SentinelConfig);
      onClose();
    } catch {
      setError("Could not reach Sentinel. Is the session still running?");
      setBusy(false);
    }
  };

  const field =
    "mt-1 w-full rounded-sm border bg-sheet px-2.5 py-1.5 font-mono text-sm text-ink placeholder:text-ink-3 focus:outline-none";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-[#15140f]/60 p-4">
      <form
        onSubmit={save}
        role="dialog"
        aria-modal="true"
        aria-label="Session budget"
        className="w-full max-w-md overflow-hidden rounded-sm border border-rule-strong bg-sheet"
      >
        <div className="flex items-center justify-between border-b border-rule-strong bg-paper px-5 py-4">
          <h3 className="text-sm font-semibold text-ink">Session budget</h3>
          <button type="button" onClick={onClose} aria-label="Close" className="text-ink-2 hover:text-ink">
            <X className="size-4" />
          </button>
        </div>

        <div className="space-y-4 p-6 text-xs text-ink-2">
          <p>
            When the estimate reaches a limit, the next tool call is held until you approve it. Leave a field empty for
            no limit. Changes apply from the next call.
          </p>

          <label className="block">
            <span className="font-medium text-ink">Cost limit, USD</span>
            <input
              ref={first}
              value={cost}
              onChange={(e) => setCost(e.target.value)}
              inputMode="decimal"
              placeholder="no limit"
              aria-invalid={costValue === null}
              className={`${field} ${costValue === null ? "border-signal" : "border-rule-strong focus:border-ink"}`}
            />
            {costValue === null && <span className="mt-1 block text-signal">Use a number like 2 or 2.50.</span>}
          </label>

          <label className="block">
            <span className="font-medium text-ink">Token limit</span>
            <input
              value={tokens}
              onChange={(e) => setTokens(e.target.value)}
              inputMode="numeric"
              placeholder="no limit"
              aria-invalid={tokenValue === null}
              className={`${field} ${tokenValue === null ? "border-signal" : "border-rule-strong focus:border-ink"}`}
            />
            {tokenValue === null && <span className="mt-1 block text-signal">Use a number like 100000 or 100k.</span>}
          </label>

          <div className="rounded-sm border border-rule bg-paper px-4 py-3">
            <div className="flex justify-between">
              <span>Estimated so far</span>
              <span className="fig font-mono text-ink">
                ~${spent.cost.toFixed(3)} · {formatTokens(spent.tokens)} tokens
              </span>
            </div>
            <p className="mt-1.5 text-ink-3">
              Estimated from the size of tool calls and results. The model's own context is not counted, so real spend
              is higher.
            </p>
          </div>

          {notice && <p className="font-medium text-ink">{notice}</p>}
          {error && (
            <p role="alert" className="font-medium text-signal">
              {error}
            </p>
          )}
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-rule bg-paper px-6 py-3.5">
          <button
            type="button"
            onClick={onClose}
            className="rounded-sm border border-rule-strong bg-paper px-3 py-1.5 text-xs text-ink hover:border-ink"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={!valid || busy}
            className="rounded-sm border-2 border-ink bg-ink px-4 py-1.5 text-xs font-semibold text-sheet hover:bg-ink-2 disabled:opacity-40"
          >
            {busy ? "Saving…" : "Save limits"}
          </button>
        </div>
      </form>
    </div>
  );
}
