import { useDialogFocus } from "./useDialogFocus";
import { X, Keyboard } from "lucide-react";

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export function ShortcutsModal({ isOpen, onClose }: Props) {
  const dialog = useDialogFocus(isOpen);

  if (!isOpen) return null;

  const shortcuts = [
    { key: "↓ / ↑ or J / K", desc: "Navigate calls in timeline" },
    { key: "Enter / Space", desc: "Inspect selected call" },
    { key: "⌘ K / Ctrl K", desc: "Open command palette & search" },
    { key: "Esc", desc: "Close detail pane or modal" },
    { key: "1 / 2 / 4", desc: "Quick timeline zoom (Fit, 2×, 4×)" },
    { key: "?", desc: "Toggle keyboard shortcuts guide" },
  ];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ backgroundColor: "var(--overlay)" }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
      onKeyDown={(e) => { if (e.key === "Escape") onClose(); }}
    >
      <div
        ref={dialog}
        role="dialog"
        aria-modal="true"
        aria-labelledby="shortcuts-title"
        className="w-full max-w-md rounded-xl border border-rule bg-sheet p-5 shadow-2xl animate-in fade-in zoom-in-95 duration-150"
      >
        <div className="flex items-center justify-between border-b border-rule pb-3">
          <div className="flex items-center gap-2">
            <Keyboard aria-hidden className="size-4 text-accent" />
            <h2 id="shortcuts-title" className="text-sm font-semibold text-ink">Keyboard Shortcuts</h2>
          </div>
          <button
            onClick={onClose}
            aria-label="Close shortcuts guide"
            className="icon-button"
          >
            <X aria-hidden className="size-4" />
          </button>
        </div>

        <div className="mt-4 space-y-2.5">
          {shortcuts.map((item) => (
            <div key={item.key} className="flex items-center justify-between text-xs">
              <span className="text-ink-2">{item.desc}</span>
              <kbd className="rounded border border-rule-strong bg-paper px-2 py-1 font-mono text-[11px] font-medium text-ink shadow-xs">
                {item.key}
              </kbd>
            </div>
          ))}
        </div>

        <div className="mt-5 border-t border-rule pt-3 text-right">
          <button
            onClick={onClose}
            className="action-button"
          >
            Got it
          </button>
        </div>
      </div>
    </div>
  );
}
