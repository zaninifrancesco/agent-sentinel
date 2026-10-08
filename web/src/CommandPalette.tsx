import { useEffect, useState, useMemo, useRef } from "react";
import { Search, Wrench, AlertTriangle, Download, X, Terminal, Radio } from "lucide-react";
import type { Row } from "./types";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  rows: Row[];
  onSelectRow: (key: string) => void;
  onExport: () => void;
}

export function CommandPalette({ isOpen, onClose, rows, onSelectRow, onExport }: Props) {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  // A new query changes the list: start again from the first result.
  useEffect(() => setSelectedIndex(0), [query]);

  useEffect(() => {
    if (isOpen) {
      setQuery("");
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [isOpen]);

  const items = useMemo(() => {
    const list: Array<{
      id: string;
      title: string;
      subtitle: string;
      icon: React.ReactNode;
      action: () => void;
      category: "Actions" | "Events" | "Errors";
    }> = [];

    // Actions only show up when they match what was typed (or nothing was typed
    // yet), so searching for an event and pressing Enter never exports by accident.
    const q = query.trim().toLowerCase();
    const exportKeywords = "export standalone audit report html download deliverable";
    if (!q || q.split(/\s+/).every((word) => exportKeywords.includes(word))) {
      list.push({
        id: "action-export",
        title: "Export Standalone Audit Report",
        subtitle: "Generate self-contained HTML deliverable",
        icon: <Download className="size-4 text-ok" />,
        action: () => {
          onExport();
          onClose();
        },
        category: "Actions",
      });
    }

    // Filter events
    for (const r of rows) {
      if (
        !query ||
        r.title.toLowerCase().includes(query.toLowerCase()) ||
        (r.subtitle && r.subtitle.toLowerCase().includes(query.toLowerCase()))
      ) {
        const isErr = r.status === "error" || r.status === "blocked";
        list.push({
          id: r.key,
          title: r.title,
          subtitle: `${r.kind} · ${r.status}${r.durationMs ? ` · ${r.durationMs}ms` : ""}`,
          icon: isErr ? (
            <AlertTriangle className="size-4 text-signal" />
          ) : r.kind === "tool" ? (
            <Wrench className="size-4 text-ink" />
          ) : r.kind === "raw" ? (
            <Terminal className="size-4 text-ink-2" />
          ) : (
            <Radio className="size-4 text-ink-2" />
          ),
          action: () => {
            onSelectRow(r.key);
            onClose();
          },
          category: isErr ? "Errors" : "Events",
        });
      }
    }

    return list.slice(0, 15);
  }, [rows, query, onExport, onSelectRow, onClose]);

  // Keyboard navigation
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        // Capture phase + stopPropagation: Esc closes the palette only, it must
        // not also reach the approval dock ("Esc = block").
        e.preventDefault();
        e.stopPropagation();
        onClose();
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        e.stopPropagation(); // keep the timeline from moving behind the palette
        setSelectedIndex((prev) => (prev + 1) % Math.max(1, items.length));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        e.stopPropagation();
        setSelectedIndex((prev) => (prev - 1 + items.length) % Math.max(1, items.length));
      } else if (e.key === "Enter") {
        e.preventDefault();
        e.stopPropagation(); // Cmd+Enter must not approve the pending call
        if (items[selectedIndex]) {
          items[selectedIndex].action();
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown, true);
    return () => window.removeEventListener("keydown", handleKeyDown, true);
  }, [isOpen, items, selectedIndex, onClose]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-ink/40 pt-24">
      <div
        className="w-full max-w-xl overflow-hidden rounded-sm border border-rule-strong bg-sheet"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Search Input */}
        <div className="flex items-center gap-3 border-b border-rule-strong/80 px-4 py-3">
          <Search className="size-4 text-ink-2" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setSelectedIndex(0);
            }}
            placeholder="Search events, e.g. read_file"
            className="w-full bg-transparent font-mono text-sm text-ink placeholder:text-ink-2 focus:outline-none"
          />
          <button
            onClick={onClose}
            className="rounded p-1 text-ink-2 hover:bg-paper hover:text-ink"
          >
            <X className="size-4" />
          </button>
        </div>

        {/* Results List */}
        <div className="max-h-80 overflow-y-auto p-2">
          {items.length === 0 ? (
            <div className="py-8 text-center text-xs text-ink-2">
              No matching events or commands found.
            </div>
          ) : (
            <ul className="space-y-1">
              {items.map((item, idx) => (
                <li key={item.id}>
                  <button
                    onClick={item.action}
                    onMouseEnter={() => setSelectedIndex(idx)}
                    className={`flex w-full items-center gap-3 rounded-sm px-3 py-2 text-left ${
                      idx === selectedIndex
                        ? "bg-yellow text-ink"
                        : "text-ink-2 hover:bg-paper"
                    }`}
                  >
                    <div className="flex size-7 shrink-0 items-center justify-center rounded-sm border border-rule-strong bg-paper">
                      {item.icon}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-mono text-xs font-medium text-ink">
                        {item.title}
                      </div>
                      <div className="truncate text-[11px] text-ink-2">{item.subtitle}</div>
                    </div>
                    <span className="text-[10px] uppercase tracking-wider text-ink-2">
                      {item.category}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Footer shortcuts */}
        <div className="flex items-center justify-between border-t border-rule bg-paper px-4 py-2 text-[11px] text-ink-2">
          <div className="flex items-center gap-2">
            <span className="flex items-center gap-1">
              <kbd className="rounded border border-rule-strong bg-paper px-1 py-0.5 font-mono text-[10px]">
                ↑
              </kbd>
              <kbd className="rounded border border-rule-strong bg-paper px-1 py-0.5 font-mono text-[10px]">
                ↓
              </kbd>{" "}
              navigate
            </span>
            <span className="flex items-center gap-1">
              <kbd className="rounded border border-rule-strong bg-paper px-1.5 py-0.5 font-mono text-[10px]">
                ↵
              </kbd>{" "}
              select
            </span>
          </div>
          <span className="flex items-center gap-1">
            <kbd className="rounded border border-rule-strong bg-paper px-1 py-0.5 font-mono text-[10px]">
              esc
            </kbd>{" "}
            close
          </span>
        </div>
      </div>
    </div>
  );
}
