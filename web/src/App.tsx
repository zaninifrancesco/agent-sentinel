import { useMemo, useState, useEffect } from "react";
import { Download, Moon, Search, Sun } from "lucide-react";
import { useSentinel, type Connection } from "./useSentinel";
import { useConfig } from "./useConfig";
import { useTheme } from "./theme";
import { Timeline } from "./Timeline";
import { Detail } from "./Detail";
import { CommandPalette } from "./CommandPalette";
import { MetricsBar } from "./MetricsBar";
import { ExportModal } from "./ExportModal";
import { computeMetrics } from "./metrics";
import type { Row } from "./types";

type Filter = "all" | "tools" | "errors" | "raw";

const FILTERS: { id: Filter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "tools", label: "Tool calls" },
  { id: "errors", label: "Problems" },
  { id: "raw", label: "Raw output" },
];

function matches(row: Row, f: Filter): boolean {
  switch (f) {
    case "tools":
      return row.kind === "tool";
    case "errors":
      return (
        row.status === "error" ||
        row.status === "blocked" ||
        row.status === "rejected" ||
        row.status === "interrupted"
      );
    case "raw":
      return row.kind === "raw";
    default:
      return true;
  }
}

export default function App() {
  const { session, rows, connection } = useSentinel();
  const { config } = useConfig();
  const [theme, toggleTheme] = useTheme();
  const [filter, setFilter] = useState<Filter>("all");
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState(false);
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);

  // Global shortcut for Cmd+K / Ctrl+K
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setIsCommandPaletteOpen((prev) => !prev);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const metrics = useMemo(() => computeMetrics(rows), [rows]);

  const visible = useMemo(
    () => rows.filter((r) => matches(r, filter)),
    [rows, filter]
  );

  // Prefer a tool call, and keep selection visible after a filter changes.
  useEffect(() => {
    if (visible.length > 0 && !visible.some((row) => row.key === selectedKey)) {
      setSelectedKey((visible.find((row) => row.kind === "tool") ?? visible[0]).key);
    }
  }, [visible, selectedKey]);

  const selected = visible.find((r) => r.key === selectedKey) ?? null;

  return (
    <div className="flex h-full flex-col bg-paper text-ink">
      <header className="light-scope flex items-center gap-4 bg-ink px-4 py-2.5 text-white sm:gap-6 sm:px-6">
        <span className="whitespace-nowrap text-base font-bold tracking-tight">Agent Sentinel</span>

        <ConnectionBadge state={connection} />
        <span className="hidden text-xs text-rule sm:inline">{config.mode === "replay" ? "Recorded session" : "Local flight recorder"}</span>

        {session && (
          <div className="hidden min-w-0 truncate font-mono text-xs text-rule-strong md:block">
            session <span className="text-white">{session.id}</span>
            {session.command && session.command.length > 0 && (
              <>
                {" · "}
                <span title={session.command.join(" ")}>{session.command.join(" ")}</span>
              </>
            )}
          </div>
        )}

        <div className="ml-auto flex items-center gap-3">
          <button
            onClick={() => setIsCommandPaletteOpen(true)}
            className="flex items-center gap-2 rounded-sm border border-ink-3 px-3 py-1.5 text-xs text-rule hover:border-white hover:text-white"
          >
            <Search className="size-3.5" />
            <span className="hidden sm:inline">Search events</span>
            <kbd className="font-mono text-[11px] text-rule-strong">⌘K</kbd>
          </button>
          <button
            onClick={toggleTheme}
            aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
            title={theme === "dark" ? "Light theme" : "Dark theme"}
            className="flex items-center rounded-sm border border-ink-3 p-1.5 text-rule hover:border-white hover:text-white"
          >
            {theme === "dark" ? <Sun className="size-3.5" /> : <Moon className="size-3.5" />}
          </button>
          <button
            onClick={() => setIsExportModalOpen(true)}
            className="flex items-center gap-1.5 rounded-sm border border-ink-3 px-3 py-1.5 text-xs text-rule hover:border-white hover:text-white"
          >
            <Download className="size-3.5" />
            <span className="hidden sm:inline">Export</span>
          </button>
        </div>
      </header>

      <MetricsBar metrics={metrics} />

      <main className="grid min-h-0 flex-1 grid-cols-1 grid-rows-[minmax(0,2fr)_minmax(0,3fr)] lg:grid-cols-[minmax(420px,5fr)_7fr] lg:grid-rows-1">
        <div className="flex min-h-0 flex-col border-r border-rule-strong">
          <div role="tablist" aria-label="Filter events" className="flex items-center gap-5 border-b border-rule-strong bg-paper px-4">
            {FILTERS.map((f) => (
              <button
                key={f.id}
                role="tab"
                aria-selected={filter === f.id}
                onClick={() => setFilter(f.id)}
                className={`-mb-px border-b-2 py-2 text-sm font-medium ${
                  filter === f.id ? "border-ink text-ink" : "border-transparent text-ink-2 hover:text-ink"
                }`}
              >
                {f.label}
              </button>
            ))}
            <span className="fig ml-auto text-xs text-ink-2">{visible.length} events</span>
          </div>
          <Timeline
            rows={visible}
            selectedKey={selectedKey}
            onSelect={setSelectedKey}
            follow={connection === "live"}
          />
        </div>
        <Detail row={selected} />
      </main>

      <CommandPalette
        isOpen={isCommandPaletteOpen}
        onClose={() => setIsCommandPaletteOpen(false)}
        rows={rows}
        onSelectRow={(key) => { setFilter("all"); setSelectedKey(key); }}
        onExport={() => setIsExportModalOpen(true)}
      />

      <ExportModal
        isOpen={isExportModalOpen}
        onClose={() => setIsExportModalOpen(false)}
        session={session}
        rows={rows}
        config={config}
      />
    </div>
  );
}

function ConnectionBadge({ state }: { state: Connection }) {
  const map: Record<Connection, { text: string; dot: string }> = {
    connecting: { text: "Connecting", dot: "border border-white" },
    live: { text: "Live", dot: "bg-white" },
    reconnecting: { text: "Reconnecting", dot: "border border-white" },
    ended: { text: "Session ended", dot: "border border-rule-strong" },
  };
  const { text, dot } = map[state];
  return (
    <span className="flex items-center gap-2 text-xs font-medium">
      <span className={`size-2 ${dot}`} />
      {text}
    </span>
  );
}
