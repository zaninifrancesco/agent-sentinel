import type { Row } from "./types";
import { formatDuration } from "./format";

/** The status of a row in the words the cockpit uses for it. */
export function statusText(s: Row["status"]): string {
  return {
    pending: "Running",
    awaiting_approval: "Waiting for you",
    ok: "Done",
    error: "Tool error",
    blocked: "Blocked by policy",
    rejected: "Rejected",
    interrupted: "Interrupted",
  }[s];
}

export type Tone = "done" | "alert" | "held" | "warn" | "quiet";

export interface Stop {
  title: string;
  at?: string;
  text: string;
  detail?: string;
  tone: Tone;
}

/**
 * A tool call as a journey: where it came from, what policy said, who
 * decided, what came back. Shared by the detail pane and the exported report
 * so the two always tell the same story.
 */
export function journeyStops(row: Row): Stop[] {
  const req = row.request;
  if (!req) return [];
  const stops: Stop[] = [];

  stops.push({
    title: "Requested",
    at: req.timestamp,
    text: row.subtitle ?? row.title,
    tone: "done",
  });

  const decision = req.decision;
  stops.push({
    title: "Policy",
    at: req.timestamp,
    text: !decision
      ? "No rule matched"
      : decision === "block"
        ? `Blocked by ${req.rule}`
        : decision === "approve"
          ? `Needs approval · ${req.rule}`
          : `Allowed with a warning · ${req.rule}`,
    detail: req.reason,
    tone: !decision ? "quiet" : decision === "block" ? "alert" : decision === "approve" ? "held" : "warn",
  });

  if (row.status === "awaiting_approval") {
    stops.push({ title: "Human", text: "Waiting for you to approve or reject", tone: "held" });
  } else if (row.resolution) {
    const d = row.resolution.decision;
    const label =
      d === "approved" ? "Approved" : d === "rejected" ? "Rejected" : d === "timeout" ? "Timed out, refused" : "Cancelled by the agent";
    stops.push({
      title: "Human",
      at: row.resolution.timestamp,
      text: label,
      // On a timeout the reason is Sentinel's own message, not a note from the operator.
      detail: row.resolution.reason ? (d === "timeout" ? row.resolution.reason : `Note: ${row.resolution.reason}`) : undefined,
      tone: d === "approved" ? "done" : "alert",
    });
  }

  if (row.response) {
    stops.push({
      title: "Answered",
      at: row.response.timestamp,
      text:
        row.status === "blocked" || row.status === "rejected"
          ? "Refusal sent to the agent, nothing ran"
          : row.status === "interrupted"
            ? "Interrupted before it finished"
            : statusText(row.status),
      detail:
        row.durationMs !== undefined
          ? row.status === "interrupted"
            ? `after ${formatDuration(row.durationMs)}`
            : `took ${formatDuration(row.durationMs)}`
          : undefined,
      tone:
        row.status === "ok"
          ? "done"
          : row.status === "error" || row.status === "interrupted"
            ? "warn"
            : row.status === "pending"
              ? "quiet"
              : "alert",
    });
  } else if (row.status === "pending") {
    stops.push({ title: "Answered", text: "Running…", tone: "quiet" });
  }

  return stops;
}
