# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Primary: a developer inspecting a local Cursor or Claude Code session to understand tool calls, errors, large outputs and repeated attempts. They use Sentinel live while the agent works and afterwards when debugging.

Secondary: an MCP server developer inspecting stdio requests, responses and protocol errors. Colleagues can read an exported offline report.

## Product Purpose

Agent Sentinel is a local profiler and flight recorder for AI agents. It records observable actions, helps a developer inspect their evidence and preserves sessions for offline debugging. It does not block commands, hold calls for approval or enforce spending limits. The coding agent's own permissions remain responsible for execution decisions.

Success: the developer can identify a concrete problem in a session, inspect the calls behind it and choose an informed change for the next attempt.

## Positioning

The Local Profiler & Flight Recorder for AI Agents. A single Go binary with an embedded cockpit, loopback-only by default, without cloud accounts or external databases. Offline describes Sentinel's recording and analysis; agents and MCP servers may make their own network requests.

## Operating Context

- The hook collector normally listens on `127.0.0.1:8848`; sessions stream to the cockpit over WebSocket and are stored in local JSONL files.
- Cursor and Claude Code hooks acknowledge neutrally, including when the collector is unavailable. Delivery has a one-second default HTTP timeout, configurable up to five seconds.
- MCP stdio traffic is forwarded unchanged in the observation path. A transport-process crash can still interrupt the connection.
- J/K and arrow keys navigate calls; Cmd/Ctrl+K searches. Export downloads a self-contained HTML report.
- Existing hook installations and installed executables require an explicit migration; normal development must not modify them silently.

## Capabilities and Constraints

Implemented in the first pivot phase:

- Live timeline, payload inspector, patch viewer where patch data exists, event search and dark/light themes.
- Tool-call counts, reported errors, available duration percentiles and retained payload JSON size. No fixed-price token or dollar estimates.
- Local JSONL recordings, replay of new and old traces, and an offline HTML report with expandable calls. Report payloads remain summarized.
- Historical risk and policy decisions remain readable only where recorded; there are no live approval or budget controls.
- The frontend remains embedded, with local fonts and no required external assets.

Planned, not implemented: reliable source/session identity and capture-quality metadata, waterfall, an analysis engine with evidence-linked findings, original payload-size measurements, recording catalog, configurable redaction/capture profiles and a JSONL export flow. See `docs/ROADMAP.md` and `docs/PROGRESS.md` for status.

The current data does not reveal model reasoning, actual context occupancy, provider billing or a full workspace history. Some hook payloads are synthesized, reduced or omitted. Retained JSON size must never be labelled original wire traffic or model usage. Absence of a duration or an error signal does not prove zero execution time or success.

## Evidence on Hand

`examples/demo-session.jsonl` is a historical enforcement trace. Synthetic hook fixtures and fake MCP servers exercise passive capture without running the commands in their payloads. `docs/profiler.png` shows a synthetic passive session; `docs/cockpit.png` is historical.

No customer results, performance claims or savings measurements may be fabricated.

## Product Principles

1. Inspect the evidence: every future diagnostic must link to the events supporting it.
2. Observe without changing execution permissions or adding confirmation prompts.
3. Honest measures: distinguish observed values, source-reported values, estimates and unknowns.
4. Make useful details easy to reach, with quiet summaries and progressive disclosure.
5. Keep recordings local and make capture limits visible; offline does not mean safe to share without inspection.
