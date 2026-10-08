import { useState } from "react";
import { Copy, Check, FileCode, Split, AlignLeft } from "lucide-react";

interface Props {
  text: string;
  filename?: string;
}

interface DiffLine {
  type: "add" | "del" | "header" | "normal";
  oldLineNumber?: number;
  newLineNumber?: number;
  content: string;
}

export function isDiff(text: string): boolean {
  if (!text || typeof text !== "string") return false;
  return (
    text.includes("--- ") ||
    text.includes("+++ ") ||
    text.includes("@@ -") ||
    /^[+-][^+-]/m.test(text)
  );
}

export function parseDiff(text: string): DiffLine[] {
  const lines = text.split("\n");
  const result: DiffLine[] = [];
  let oldLine = 1;
  let newLine = 1;

  for (const line of lines) {
    if (line.startsWith("@@")) {
      // Chunk header e.g. @@ -1,5 +1,6 @@
      const match = line.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (match) {
        oldLine = parseInt(match[1], 10);
        newLine = parseInt(match[2], 10);
      }
      result.push({ type: "header", content: line });
    } else if (line.startsWith("---") || line.startsWith("+++")) {
      result.push({ type: "header", content: line });
    } else if (line.startsWith("+")) {
      result.push({
        type: "add",
        newLineNumber: newLine++,
        content: line.slice(1),
      });
    } else if (line.startsWith("-")) {
      result.push({
        type: "del",
        oldLineNumber: oldLine++,
        content: line.slice(1),
      });
    } else {
      result.push({
        type: "normal",
        oldLineNumber: oldLine++,
        newLineNumber: newLine++,
        content: line.startsWith(" ") ? line.slice(1) : line,
      });
    }
  }

  return result;
}

export function DiffViewer({ text, filename }: Props) {
  const [copied, setCopied] = useState(false);
  const [viewMode, setViewMode] = useState<"unified" | "raw">("unified");

  const diffLines = parseDiff(text);

  const handleCopy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="overflow-hidden rounded-xl border border-ink-700 bg-ink-900 shadow-xl">
      {/* Diff Header */}
      <div className="flex items-center justify-between border-b border-ink-700 bg-ink-850 px-4 py-2.5">
        <div className="flex items-center gap-2">
          <FileCode className="size-4 text-accent" />
          <span className="font-mono text-xs font-medium text-ink-100">
            {filename || "diff-preview.patch"}
          </span>
          <span className="rounded-full bg-ink-700 px-2 py-0.5 font-mono text-[10px] text-ink-300">
            {diffLines.filter((l) => l.type === "add").length} additions,{" "}
            {diffLines.filter((l) => l.type === "del").length} deletions
          </span>
        </div>

        <div className="flex items-center gap-2">
          <div className="flex rounded-md bg-ink-800 p-0.5 text-[11px]">
            <button
              onClick={() => setViewMode("unified")}
              className={`flex items-center gap-1 rounded px-2 py-1 transition-colors ${
                viewMode === "unified"
                  ? "bg-ink-700 text-ink-100"
                  : "text-ink-400 hover:text-ink-200"
              }`}
            >
              <Split className="size-3" /> Unified
            </button>
            <button
              onClick={() => setViewMode("raw")}
              className={`flex items-center gap-1 rounded px-2 py-1 transition-colors ${
                viewMode === "raw"
                  ? "bg-ink-700 text-ink-100"
                  : "text-ink-400 hover:text-ink-200"
              }`}
            >
              <AlignLeft className="size-3" /> Raw
            </button>
          </div>

          <button
            onClick={handleCopy}
            className="flex items-center gap-1.5 rounded-md border border-ink-700 bg-ink-800 px-2.5 py-1 text-xs text-ink-300 transition-colors hover:bg-ink-700 hover:text-ink-100"
          >
            {copied ? (
              <>
                <Check className="size-3.5 text-emerald-400" />
                <span className="text-emerald-400">Copied</span>
              </>
            ) : (
              <>
                <Copy className="size-3.5" />
                <span>Copy</span>
              </>
            )}
          </button>
        </div>
      </div>

      {/* Diff Content */}
      {viewMode === "raw" ? (
        <pre className="overflow-x-auto p-4 font-mono text-xs leading-relaxed text-ink-200">
          {text}
        </pre>
      ) : (
        <div className="overflow-x-auto font-mono text-xs leading-relaxed">
          <table className="w-full border-collapse">
            <tbody>
              {diffLines.map((line, idx) => {
                if (line.type === "header") {
                  return (
                    <tr
                      key={idx}
                      className="border-y border-ink-800 bg-ink-850/80 text-ink-400"
                    >
                      <td
                        colSpan={3}
                        className="px-4 py-1 text-[11px] font-semibold text-accent/90"
                      >
                        {line.content}
                      </td>
                    </tr>
                  );
                }

                const isAdd = line.type === "add";
                const isDel = line.type === "del";

                return (
                  <tr
                    key={idx}
                    className={`transition-colors ${
                      isAdd
                        ? "bg-emerald-950/30 text-emerald-200 hover:bg-emerald-950/50"
                        : isDel
                        ? "bg-rose-950/30 text-rose-200 hover:bg-rose-950/50"
                        : "text-ink-200 hover:bg-ink-800/40"
                    }`}
                  >
                    <td className="w-12 select-none border-r border-ink-800/50 px-2 py-0.5 text-right text-[11px] text-ink-600">
                      {line.oldLineNumber ?? ""}
                    </td>
                    <td className="w-12 select-none border-r border-ink-800/50 px-2 py-0.5 text-right text-[11px] text-ink-600">
                      {line.newLineNumber ?? ""}
                    </td>
                    <td className="px-3 py-0.5 whitespace-pre">
                      <span className="mr-2 select-none font-bold">
                        {isAdd ? "+" : isDel ? "-" : " "}
                      </span>
                      {line.content}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
