import { useEffect, useState } from "react";
import type { ProfilerPage, Row } from "./types";

export type Connection = "connecting" | "live" | "ended" | "reconnecting";

export async function readJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(url, { signal });
  if (!response.ok) throw new Error((await response.text()).trim() || `Request failed (${response.status})`);
  return response.json() as Promise<T>;
}

/** A page replaces the previous one; full history and payloads never stream
 * into browser memory. Cursor pages stay pinned until Refresh latest. */
export function useProfiler(query: string) {
  const [data, setData] = useState<ProfilerPage | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadedQuery, setLoadedQuery] = useState("");
  const [error, setError] = useState("");
  const [connection, setConnection] = useState<Connection>("connecting");
  const [refresh, setRefresh] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    let timer: number | undefined;
    const load = async () => {
      let follow = true;
      try {
        const page = await readJSON<ProfilerPage>(`/api/v1/profiler?${query}`, controller.signal);
        if (controller.signal.aborted) return;
        follow = !page.session.endedAt && !new URLSearchParams(query).has("cursor");
        setData(page); setLoadedQuery(query); setError(""); setConnection(page.session.endedAt ? "ended" : "live");
      } catch (cause) {
        if (controller.signal.aborted) return;
        setError(cause instanceof Error ? cause.message : "Could not read this recording"); setConnection("reconnecting");
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
          if (follow) timer = window.setTimeout(load, 2000);
        }
      }
    };
    setLoading(true);
    void load();
    return () => { controller.abort(); window.clearTimeout(timer); };
  }, [query, refresh]);
  return { data, loading: loading || loadedQuery !== query, error, connection, retry: () => setRefresh((value) => value + 1) };
}

export function useRow(key: string | null, recordingId?: string, revision?: number) {
  const [detail, setDetail] = useState<{ key: string; revision: number; row: Row } | null>(null);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    if (!key || !recordingId || !revision) return;
    const controller = new AbortController(); setError("");
    const query = new URLSearchParams({ recordingId, through: String(revision) });
    void readJSON<Row>(`/api/v1/rows/${encodeURIComponent(key)}?${query}`, controller.signal)
      .then((row) => { if (!controller.signal.aborted) setDetail({ key, revision, row }); })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : "Could not load payloads"); });
    return () => controller.abort();
  }, [key, recordingId, revision, retry]);
  return { row: detail?.key === key && detail?.revision === revision ? detail.row : null, error, retry: () => setRetry((n) => n + 1) };
}
