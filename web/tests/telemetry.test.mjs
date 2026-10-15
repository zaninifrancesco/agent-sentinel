import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { createServer } from "vite";

let server;
let computeMetrics;
let journeyStops;
let buildReportHtml;
let buildProfile;
let filterCalls;
let sortCalls;
let recordedSpan;
let buildRows;

before(async () => {
  // Use Vite's existing TypeScript loader without opening a listening socket.
  server = await createServer({ configFile: false, appType: "custom", server: { middlewareMode: true, hmr: false, ws: false, watch: null } });
  ({ computeMetrics } = await server.ssrLoadModule("/src/metrics.ts"));
  ({ journeyStops } = await server.ssrLoadModule("/src/journey.ts"));
  ({ buildReportHtml } = await server.ssrLoadModule("/src/report.ts"));
  ({ buildProfile, filterCalls, sortCalls, recordedSpan } = await server.ssrLoadModule("/src/profile.ts"));
  ({ buildRows } = await server.ssrLoadModule("/src/useSentinel.ts"));
});
after(async () => { await server?.close(); });

function call(durationMs, overrides = {}) {
  const request = { seq: 1, sessionId: "test", timestamp: "2026-10-15T10:00:00Z", type: "ToolCallRequest", toolName: "shell", risk: "none", status: "pending", payload: { params: { arguments: { command: "echo synthetic" } } } };
  return { key: "req-1", kind: "tool", title: "shell", status: "ok", risk: "none", startedAt: request.timestamp, request, response: { ...request, seq: 2, type: "ToolCallResponse", status: "ok" }, durationMs, ...overrides };
}

test("tool percentiles include zero, use nearest rank, and exclude interrupted and unrelated spans", () => {
  const metrics = computeMetrics([call(0), call(10), call(20), call(30), call(40), call(1_000, { status: "interrupted" }), call(2_000, { kind: "rpc" }), call(undefined), call(4_000, { request: { decision: "approve" } })]);
  assert.equal(metrics.latencySamples, 5);
  assert.equal(metrics.p50, 20);
  assert.equal(metrics.p95, 40);
  assert.equal(computeMetrics([]).latencySamples, 0);
  assert.equal(computeMetrics([call(0), call(100)]).p50, 0);
  assert.equal(computeMetrics([call(100, { kind: "rpc", status: "error" })]).errors, 0);
});

test("payload size counts retained UTF-8 JSON, without claiming model usage", () => {
  const row = call(1, { request: { payload: { text: "è🙂" } }, response: { payload: { content: "synthetic" } } });
  const expected = Buffer.byteLength(JSON.stringify(row.request.payload)) + Buffer.byteLength(JSON.stringify(row.response.payload));
  const metrics = computeMetrics([row]);
  assert.equal(metrics.payloadBytes, expected);
  assert.ok(!("cost" in metrics));
  assert.ok(!("tokens" in metrics));
});

test("observation journeys contain no imaginary policy evaluation", () => {
  const row = call(1);
  assert.deepEqual(journeyStops(row).map((stop) => stop.title), ["Requested", "Answered"]);
  const historical = call(1, { request: { ...row.request, decision: "block", rule: "old-rule" }, status: "blocked" });
  assert.ok(journeyStops(historical).some((stop) => stop.title === "Historical policy"));
});

test("offline reports escape captured text and retain historical decisions only when present", () => {
  const row = call(1, { title: '<script>alert("synthetic")</script>' });
  const input = { session: { id: "test", command: [], startedAt: row.startedAt }, rows: [row], config: { mode: "observe" }, exportedAt: new Date("2026-10-15T10:01:00Z") };
  const html = buildReportHtml(input);
  assert.ok(html.includes("Observation only"));
  assert.ok(html.includes("&lt;script&gt;"));
  assert.ok(!html.includes("<script>"));
  assert.ok(!html.includes("Historical policy decisions"));
  assert.ok(!html.includes("Cost (estimate)"));
  assert.ok(html.includes("Content-Security-Policy"));
  const historical = { ...row, status: "blocked", request: { ...row.request, decision: "block", rule: "old-rule" } };
  assert.ok(buildReportHtml({ ...input, rows: [historical] }).includes("Historical policy decisions"));
});

test("profile rankings distinguish unknown duration and absent responses from observed zero", () => {
  const rows = [call(undefined, { key: "unknown", response: undefined }), call(0, { key: "zero" }), call(40, { key: "slow" }), call(500, { key: "interrupted", status: "interrupted" })];
  const profile = buildProfile(rows);
  assert.deepEqual(profile.longest.map((row) => row.key), ["slow", "zero"]);
  assert.ok(!profile.largest.some((row) => row.key === "unknown"));
  assert.equal(sortCalls(rows, "duration").at(-1).key, "interrupted");
  assert.equal(profile.tools[0].metrics.calls, 4);
  assert.equal(profile.tools[0].metrics.latencySamples, 2);
});

test("calls filter by exact tool, status and full captured arguments", () => {
  const a = call(5, { key: "a", status: "error", subtitle: "truncated…", request: { toolName: "shell", payload: { command: "needle beyond the summary" } } });
  const b = call(10, { key: "b", status: "ok", request: { toolName: "read_file" } });
  assert.deepEqual(filterCalls([a, b], "errors", "shell", "NEEDLE").map((row) => row.key), ["a"]);
  assert.equal(filterCalls([a, b], "open", "", "").length, 0);
  assert.equal(filterCalls([a, b], "all", "read", "").length, 0);
});

test("missing source duration stays unavailable while explicit zero is preserved", () => {
  const row = call(0);
  const request = { ...row.request, rpcId: "same" };
  const response = { ...row.response, rpcId: "same", durationMs: undefined };
  assert.equal(buildRows([request, response])[0].durationMs, undefined);
  assert.equal(buildRows([request, { ...response, durationMs: 0 }])[0].durationMs, 0);
});

test("recording span uses captured timestamps rather than the current clock", () => {
  const row = call(5, { response: { timestamp: "2026-10-15T10:00:02Z" } });
  const session = { startedAt: row.startedAt };
  assert.equal(recordedSpan(session, [row]), 2_000);
  assert.equal(recordedSpan(null, [row]), undefined);
  assert.equal(recordedSpan({ startedAt: "invalid" }, [row]), undefined);
});
