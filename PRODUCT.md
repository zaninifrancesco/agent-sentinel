# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Primary: a developer running an AI coding agent (Claude Code, Cursor, etc.) who keeps the Sentinel cockpit open on a second screen or window while the agent works. They glance at it to see what the agent is doing, and are pulled in when the agent is paused on a risky tool call and needs a fast, well-informed yes or no. Confirmed by the user.

Secondary (not the focus of design decisions): the same developer replaying a recorded session afterwards (`sentinel ui`).

## Product Purpose

Agent Sentinel is a single Go binary that sits between an AI coding agent and its MCP servers. It records every JSON-RPC frame (flight recorder), enforces a policy before a tool call is forwarded (block, ask a human, warn, budget breaker), and serves a local cockpit where the human watches the session live and approves or rejects held calls with an optional steering note for the agent.

Success: the developer trusts the agent with more autonomy because anything dangerous stops and asks them, and they can see at a glance what happened and why.

## Positioning

It is on the wire, not beside it: because it proxies the protocol it can stop a call before it executes, not just log it afterwards. It is local-first (loopback only, no cloud, no account) and ships as one binary with the UI embedded.

## Operating Context

- Runs on the developer's machine next to the agent; the cockpit is served on `127.0.0.1` and opened in a browser tab or window.
- Live sessions stream over a WebSocket; a held call blocks the agent until answered or until a timeout (default 2 minutes) rejects it.
- Approving is keyboard-driven today: ⌘/Ctrl+Enter approves, Esc rejects, J/K or arrows move through the timeline, ⌘K opens a command palette.
- Sessions are also stored as JSONL and replayed read-only.

## Capabilities and Constraints

- Timeline of events (tool calls merged with their responses, notifications, raw output), a detail panel with payload, raw JSON and a visual diff when a patch is present, a metrics bar, a command palette, an approval dock.
- Policy verdicts shown per call: rule id, reason, risk level (none to critical), decision (allow, warn, approve, block) and the human verdict.
- Cost and tokens are estimates from tool traffic size only (the proxy cannot see the LLM API); the UI must label them as estimates.
- The UI is a React + Vite + Tailwind SPA built into `internal/ui/dist` and embedded in the Go binary with `go:embed`. It must work offline, with no CDN assets or external fonts.
- Statuses the UI must express: pending, awaiting approval, ok, error, blocked (by policy), rejected (by a human or timeout).
- The standalone HTML report (Export button) is built in the browser from the timeline rows: summary facts, every policy and human decision with its rule and reason, and the timetable with each call's journey. It is one offline file with no script, follows the reader's light or dark theme, and leaves payloads out. It is a secondary surface; do not design the cockpit around it. A `sentinel export` command does not exist yet.
- Undecided: light theme, multi-session views, mobile layout.

## Evidence on Hand

Real data only: `examples/demo-session.jsonl` and live sessions produced with `examples/fake_mcp_server.py`. No customers, testimonials, benchmarks or brand assets exist; none may be fabricated.

## Product Principles

1. The decision moment comes first: when a call is waiting for a human, nothing else on screen should compete with it.
2. Show the evidence, not a verdict alone: every block or hold names the rule and quotes what triggered it.
3. Honest numbers: estimates are labelled as estimates.
4. Calm when nothing needs you, loud only when something does.
5. Keyboard first for the repeated actions, mouse always possible.
