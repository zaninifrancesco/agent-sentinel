import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { createServer } from "vite";

let server, report, journeyStops, captureSummary, capturedInterval, responseSize, toolDuration, timelineTracks, periodChange;
const backendMetrics = { calls: 17, waiting: 0, running: 0, refused: 0, errors: 2, interrupted: 0, payloadBytes: 8192, latencySamples: 12, p50: 333, p95: 987 };
const timeline = { startMs: 0, endMs: 1000, spanMs: 1000, coveredMs: 600, unattributedMs: 400, overlapMs: 200, peakConcurrency: 2, incompleteCalls: 0, gapCount: 1, gaps: [{ startMs: 600, endMs: 1000 }] };

before(async () => {
  server = await createServer({ configFile: false, appType: "custom", server: { middlewareMode: true, hmr: false, ws: false, watch: null } });
  ({ buildReportHtml: report } = await server.ssrLoadModule("/src/report.ts"));
  ({ journeyStops } = await server.ssrLoadModule("/src/journey.ts"));
  ({ captureSummary } = await server.ssrLoadModule("/src/capture.ts"));
  ({ capturedInterval, timelineTracks } = await server.ssrLoadModule("/src/trace.ts"));
  ({ responseSize } = await server.ssrLoadModule("/src/profile.ts"));
  ({ toolDuration } = await server.ssrLoadModule("/src/metrics.ts"));
  ({ periodChange } = await server.ssrLoadModule("/src/Analytics.tsx"));
});
after(async () => { await server?.close(); });

test("analytics period comparison distinguishes no baseline from a measured percentage", () => {
  assert.equal(periodChange(15, 10), "+50.0% calls vs previous period");
  assert.equal(periodChange(0, 10), "-100.0% calls vs previous period");
  assert.equal(periodChange(0, 0), "No observed calls in either period");
  assert.ok(periodChange(5, 0).includes("previous period had none"));
  assert.ok(!periodChange(5, 0).includes("%"));
});

function call(overrides = {}) {
  const request = { seq: 1, sessionId: "mock", timestamp: "2026-10-16T10:00:00Z", type: "ToolCallRequest", toolName: "shell", risk: "none", status: "pending" };
  return { key: "req-mock-1", kind: "tool", title: "shell", subtitle: "synthetic only", status: "ok", risk: "none", startedAt: request.timestamp, request, response: { ...request, seq: 2, type: "ToolCallResponse", status: "ok" }, durationMs: 1, ...overrides };
}
function buildReport(input = {}) {
  return report({ session: { id: "mock", command: [], startedAt: "2026-10-16T10:00:00Z" }, rows: [call()], config: { mode: "observe" }, exportedAt: new Date("2026-10-16T10:01:00Z"), metrics: backendMetrics, timeline, version: "profiler-v1", revision: 41, ...input });
}

test("offline report uses the backend snapshot's metrics instead of recomputing its rows", () => {
  const html = buildReport();
  assert.ok(html.includes("333 / 987 ms (12 samples)"));
  assert.ok(html.includes("8.0 KiB"));
  assert.ok(html.includes("profiler-v1 · snapshot #41"));
  assert.ok(html.includes("Matched tool coverage"));
  assert.ok(html.includes("600 ms"));
  assert.ok(html.includes("Unattributed interval"));
  assert.ok(html.includes("400 ms"));
  assert.ok(!html.includes("LLM thinking"));
});

test("offline report escapes captured values and analysis provenance and has no active content", () => {
  const html = buildReport({ rows: [call({ title: '<script>alert("mock")</script>', request: { ...call().request, agentSessionId: "<img src=x onerror=alert(1)>" } })], version: "<script>" });
  assert.ok(html.includes("&lt;script&gt;"));
  assert.ok(!html.includes("<script>"));
  assert.ok(!html.includes("<img src=x"));
  assert.ok(html.includes("Content-Security-Policy"));
  assert.ok(html.includes("default-src 'none'"));
  assert.ok(!html.includes("Cost (estimate)"));
  assert.ok(!html.includes("Historical policy decisions"));
});

test("historical policy and unresolved evidence stay inspectable", () => {
  const row = call();
  assert.deepEqual(journeyStops(row).map((stop) => stop.title), ["Requested", "Answered"]);
  const historical = call({ status: "blocked", request: { ...row.request, decision: "block", rule: "historical-rule" } });
  assert.ok(buildReport({ rows: [historical] }).includes("Historical policy decisions"));
  const unresolved = call({ status: "unknown", correlation: { method: "fallback", quality: "ambiguous" }, response: undefined });
  const html = buildReport({ rows: [unresolved] });
  assert.ok(html.includes("UNRESOLVED"));
  assert.ok(html.includes("Pairing unresolved"));
});

test("UI presents server measurements without inferring sizes or durations from missing bodies", () => {
  const row = call({ responseBytes: 0, toolDurationMs: 0, span: { startMs: 10, endMs: 10 }, durationMs: 99999 });
  assert.equal(responseSize(row), 0);
  assert.equal(toolDuration(row), 0);
  assert.deepEqual(capturedInterval(row), { start: 10, end: 10 });
  assert.equal(responseSize(call()), undefined);
  assert.equal(toolDuration(call()), undefined);
  assert.equal(capturedInterval(call()), undefined);
});

test("capture summaries distinguish omitted, partial, observed zero and unknown provenance", () => {
  assert.match(captureSummary({}), /quality not recorded/);
  const capture = { profile: "standard", representation: "hook-normalized", completeness: "partial", observedBytes: 0, observedUnit: "utf8-text", normalizedBytes: 100, storedBytes: 20, changes: [{ kind: "redacted", stage: "recorder", method: "v1" }, { kind: "truncated", stage: "recorder", method: "v1" }] };
  assert.match(captureSummary({ capture }), /0 B observed text UTF-8/);
  assert.match(captureSummary({ capture }), /redacted · truncated/);
  assert.match(captureSummary({ capture: { ...capture, completeness: "omitted", observedBytes: undefined } }), /Content omitted.*Observed size unknown/);
});

test("report summaries keep payload coverage when full bodies were not downloaded", () => {
  const row = call();
  row.response.capture = { profile: "standard", representation: "hook-normalized", completeness: "partial", observedBytes: 100000, observedUnit: "utf8-text", normalizedBytes: 100100, storedBytes: 65536, changes: [{ kind: "truncated", stage: "recorder", method: "json-prefix-v1" }] };
  const html = buildReport({ rows: [row], session: { id: "capture", startedAt: row.startedAt, command: [], capture: { profile: "standard", maxPayloadBytes: 65536, redaction: "structured-text-v1" } } });
  assert.ok(html.includes("structured-text-v1"));
  assert.ok(html.includes("Partial capture"));
  assert.ok(html.includes("observed text UTF-8"));
  assert.ok(html.includes("stored JSON"));
  assert.ok(html.includes("original size unknown"));
});

test("report intervals keep capture timing separate from reported duration", () => {
  const html = buildReport({ rows: [call({ span: { startMs: 100, endMs: 110 }, durationMs: 9999 })] });
  assert.ok(html.includes("captured interval 10 ms"));
  const partial = buildReport({ rows: [call({ span: { startMs: 100 }, response: undefined })] });
  assert.ok(partial.includes("capture end unavailable"));
});


test("timeline reuses tracks for sequential calls and separates overlapping tool types", () => {
  const shell = call({ key: "a", span: { startMs: 0, endMs: 100 } });
  const edit = call({ key: "b", title: "edit_file", span: { startMs: 20, endMs: 70 } });
  const read = call({ key: "c", title: "read_file", span: { startMs: 70, endMs: 90 } });
  const next = call({ key: "d", span: { startMs: 100, endMs: 200 } });
  const packed = timelineTracks([next, edit, shell, read]);
  assert.deepEqual(packed.tracks.map((track) => track.map((row) => row.key)), [["a", "d"], ["b", "c"]]);
  assert.equal(packed.unpositioned.length, 0);
});

test("timeline keeps coincident observations distinct without extending unknown ends", () => {
  const rows = [
    call({ key: "open", span: { startMs: 0 } }),
    call({ key: "zero", span: { startMs: 0, endMs: 0 } }),
    call({ key: "later", span: { startMs: 1, endMs: 5 } }),
    call({ key: "missing", span: undefined }),
    call({ key: "invalid", span: { startMs: NaN } }),
    call({ key: "event", kind: "session" }),
  ];
  const packed = timelineTracks(rows);
  assert.deepEqual(packed.tracks.map((track) => track.map((row) => row.key)), [["open", "later"], ["zero"]]);
  assert.deepEqual(packed.unpositioned.map((row) => row.key), ["missing", "invalid"]);
});
