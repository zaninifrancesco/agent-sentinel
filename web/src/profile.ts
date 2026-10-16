import type { Row } from "./types";
export type CallFilter = "all" | "errors" | "open" | "interrupted" | "unresolved";
export type CallSort = "time" | "duration" | "response";
/** A missing response measurement stays unavailable; the backend supplies bytes. */
export function responseSize(row: Row): number | undefined { return row.responseBytes; }
