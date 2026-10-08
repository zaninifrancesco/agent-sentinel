import type { Row, SentinelEvent } from "./types";
import { StatusIcon, formatTime } from "./Timeline";

export function Detail({ row }: { row: Row | null }) {
  if (!row) {
    return (
      <section className="grid place-items-center bg-ink-950 text-sm text-ink-500">
        Select an event to inspect its payload
      </section>
    );
  }

  const blocks: { label: string; event: SentinelEvent }[] = [];
  if (row.request) blocks.push({ label: "Request", event: row.request });
  if (row.response) blocks.push({ label: "Response", event: row.response });
  if (row.event) blocks.push({ label: "Payload", event: row.event });

  return (
    <section className="min-h-0 overflow-y-auto bg-ink-950 p-6">
      <div className="mb-5 flex items-center gap-3">
        <StatusIcon status={row.status} />
        <h2 className="font-mono text-lg">{row.title}</h2>
        {row.subtitle && <span className="text-sm text-ink-500">{row.subtitle}</span>}
      </div>

      <dl className="mb-6 grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1.5 text-sm">
        <Meta k="status" v={row.status} />
        <Meta k="started" v={formatTime(row.startedAt)} />
        {row.durationMs !== undefined && <Meta k="latency" v={`${row.durationMs} ms`} />}
        {row.request?.rpcId && <Meta k="rpc id" v={row.request.rpcId} />}
        {row.risk !== "none" && <Meta k="risk" v={row.risk} />}
      </dl>

      {blocks.map(({ label, event }) => (
        <div key={event.seq} className="mb-6">
          <div className="mb-2 flex items-center gap-2 text-xs uppercase tracking-wider text-ink-500">
            {label}
            <span className="normal-case tracking-normal">
              #{event.seq} · {event.direction}
            </span>
          </div>
          <pre className="overflow-x-auto rounded-lg border border-ink-700 bg-ink-900 p-4 font-mono text-xs leading-relaxed text-ink-100">
            {pretty(event.payload)}
          </pre>
        </div>
      ))}
    </section>
  );
}

function Meta({ k, v }: { k: string; v: string }) {
  return (
    <>
      <dt className="text-ink-500">{k}</dt>
      <dd className="font-mono text-ink-100">{v}</dd>
    </>
  );
}

function pretty(payload: unknown): string {
  if (payload === undefined) return "(no payload)";
  if (typeof payload === "string") return payload;
  return JSON.stringify(payload, null, 2);
}
