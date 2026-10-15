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
let buildTrace;
let capturedInterval;
let recordingScopes;
let filterScope;

before(async () => {
  // Use Vite's existing TypeScript loader without opening a listening socket.
  server = await createServer({ configFile: false, appType: "custom", server: { middlewareMode: true, hmr: false, ws: false, watch: null } });
  ({ computeMetrics } = await server.ssrLoadModule("/src/metrics.ts"));
  ({ journeyStops } = await server.ssrLoadModule("/src/journey.ts"));
  ({ buildReportHtml } = await server.ssrLoadModule("/src/report.ts"));
  ({ buildProfile, filterCalls, sortCalls, recordedSpan } = await server.ssrLoadModule("/src/profile.ts"));
  ({ buildTrace, capturedInterval } = await server.ssrLoadModule("/src/trace.ts"));
  ({ buildRows } = await server.ssrLoadModule("/src/useSentinel.ts"));
  ({ recordingScopes, filterScope } = await server.ssrLoadModule("/src/scopes.ts"));
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


test("capture map uses observed timestamps instead of reported tool duration", () => {
  const row = call(5000, { response: { timestamp: "2026-10-15T10:00:00.020Z" } });
  const trace = buildTrace([row]);
  assert.equal(trace.span, 20);
  assert.equal(trace.timed[0].end - trace.timed[0].start, 20);
  assert.equal(trace.timed[0].row.durationMs, 5000);
  assert.equal(capturedInterval({ ...row, response: { timestamp: "2026-10-15T09:00:00Z" } }).end, undefined);
  assert.equal(capturedInterval({ ...row, startedAt: "invalid" }), undefined);
});

test("capture map keeps pending calls as points and honours an observed zero span", () => {
  const row = call(0);
  assert.equal(buildTrace([row]).span, 0);
  const pending = { ...row, response: undefined, status: "pending" };
  assert.equal(buildTrace([pending]).timed[0].end, undefined);
  assert.equal(buildTrace([]).timed.length, 0);
});

test("capture map bounds rendering while preserving all recent calls across tool lanes", () => {
  const rows = Array.from({ length: 140 }, (_, i) => call(i, { key: String(i), request: { toolName: "tool-" + i % 8 } }));
  const trace = buildTrace(rows);
  assert.equal(trace.omitted, 20);
  assert.equal(trace.timed.length, 120);
  assert.equal(trace.lanes.length, 6);
  assert.equal(trace.lanes.flatMap((lane) => lane.entries).length, 120);
  assert.equal(trace.lanes.at(-1).name, "Other tools");
  assert.equal(rows.length, 140);
});

function event(seq, type, overrides = {}) {
  return { seq, schemaVersion: 2, sessionId: "recording", recordingId: "recording", source: "claude-hook", sourceId: "collector/claude", agentSessionId: "session", timestamp: `2026-10-15T10:00:${String(seq).padStart(2,"0")}Z`, type, toolName: "shell", risk: "none", status: type.endsWith("Request") ? "pending" : "ok", ...overrides };
}

test("v2 projection uses stable scoped identities and exact evidence, not reused RPC IDs", () => {
  const a = event(1,"ToolCallRequest",{callId:"a",rpcId:"same"});
  const b = event(2,"ToolCallRequest",{callId:"b",rpcId:"same",agentSessionId:"other"});
  const resB = event(3,"ToolCallResponse",{callId:"b",rpcId:"same",agentSessionId:"other",correlation:{method:"native",quality:"exact",requestSeq:2},timing:{durationMs:0,source:"source-reported"}});
  const resA = event(4,"ToolCallResponse",{callId:"a",rpcId:"same",correlation:{method:"native",quality:"exact",requestSeq:1},timing:{durationMs:7,source:"source-reported"}});
  const rows = buildRows([a,b,resB,resA]);
  assert.equal(rows.length,2);
  assert.equal(rows[0].response.seq,4);
  assert.equal(rows[1].response.seq,3);
  assert.equal(rows[1].durationMs,0);
  const wrongScope = buildRows([a,{...resA,agentSessionId:"other"}]);
  assert.equal(wrongScope.length,2);
  assert.equal(wrongScope[0].response,undefined);
});

test("ambiguous evidence never acquires a result, duration or fabricated capture span", () => {
  const requests = [event(1,"ToolCallRequest",{callId:"a"}),event(2,"ToolCallRequest",{callId:"b"})];
  const correlation = {method:"fallback",quality:"ambiguous",candidateCallIds:["a","b"]};
  const rows=buildRows([...requests,event(3,"CaptureDiagnostic",{correlation}),event(4,"ToolCallResponse",{callId:"partial",correlation,durationMs:500})]);
  assert.deepEqual(rows.slice(0,2).map((r)=>r.status),["unknown","unknown"]);
  assert.ok(rows.slice(0,2).every((r)=>!r.response && r.durationMs===undefined));
  assert.equal(computeMetrics(rows).latencySamples,0);
  assert.equal(rows[3].kind,"rpc");
  assert.equal(capturedInterval(rows[3]),undefined);
  assert.equal(buildProfile(rows).longest.length,0);
  assert.equal(filterCalls(rows.filter((r)=>r.kind==="tool"),"unresolved","","").length,2);
});

test("legacy RPC pairing respects origin and keeps overlapping IDs ambiguous", () => {
  const req=event(1,"ToolCallRequest",{schemaVersion:undefined,source:undefined,callId:undefined,rpcId:"1",direction:"client->server"});
  const opposite={...req,seq:2,direction:"server->client",type:"Request"};
  const answer={...req,seq:3,type:"ToolCallResponse",direction:"server->client",status:"ok"};
  const rows=buildRows([req,opposite,answer]);
  assert.equal(rows[0].response.seq,3);
  assert.equal(rows[1].response,undefined);
  const ambiguous=buildRows([req,{...req,seq:2},answer,{...answer,seq:4}]);
  assert.equal(ambiguous.length,4);
  assert.ok(ambiguous.slice(0,2).every((r)=>r.status==="unknown" && !r.response));
});

test("synthesized interruptions preserve the request but do not measure execution time", () => {
  const rows=buildRows([event(1,"ToolCallRequest",{callId:"a"}),event(2,"ToolCallResponse",{callId:"a",status:"interrupted",durationMs:999,correlation:{method:"native",quality:"exact",requestSeq:1,synthetic:true}})]);
  assert.equal(rows.length,1);
  assert.equal(rows[0].durationMs,undefined);
  assert.equal(capturedInterval(rows[0]).end,undefined);
  assert.equal(journeyStops(rows[0]).at(-1).title,"Capture closed");
});

test("session scopes isolate agents with the same native session ID and retain unknown events", () => {
  const rows=buildRows([event(1,"ToolCallRequest",{callId:"a"}),event(2,"ToolCallRequest",{callId:"b",source:"cursor-hook",sourceId:"collector/cursor"}),event(3,"FutureCaptureEvent",{payload:{kept:true}})]);
  const scopes=recordingScopes(rows);
  assert.equal(scopes.length,2);
  const claude=scopes.find((s)=>s.label.startsWith("Claude Code"));
  const scoped=filterScope(rows,claude.id);
  assert.equal(computeMetrics(scoped).calls,1);
  assert.equal(scoped[1].title,"FutureCaptureEvent");
  assert.deepEqual(scoped[1].event.payload,{kept:true});
});

test("reports retain session identity, safe capture diagnostics and unresolved pairing", () => {
  const correlation={method:"fallback",quality:"ambiguous",candidateCallIds:["a","b"]};
  const rows=buildRows([event(1,"ToolCallRequest",{callId:"a",agentSessionId:"<script>session</script>"}),event(2,"CaptureDiagnostic",{correlation,agentSessionId:"<script>session</script>"})]);
  const html=buildReportHtml({session:{id:"recording",command:[],startedAt:rows[0].startedAt},rows,config:{mode:"observe"},exportedAt:new Date("2026-10-15T10:01:00Z")});
  assert.ok(html.includes("UNRESOLVED"));
  assert.ok(html.includes("Pairing unresolved"));
  assert.ok(html.includes("&lt;script&gt;session&lt;/script&gt;"));
  assert.ok(!html.includes("<script>"));
});
