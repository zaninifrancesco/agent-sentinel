import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { createServer } from "vite";

let server;
let computeMetrics;
let journeyStops;
let buildReportHtml;

before(async () => {
  // Use Vite's existing TypeScript loader without opening a listening socket.
  server = await createServer({ configFile: false, appType: "custom", server: { middlewareMode: true, hmr: false, ws: false, watch: null } });
  ({ computeMetrics } = await server.ssrLoadModule("/src/metrics.ts"));
  ({ journeyStops } = await server.ssrLoadModule("/src/journey.ts"));
  ({ buildReportHtml } = await server.ssrLoadModule("/src/report.ts"));
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
