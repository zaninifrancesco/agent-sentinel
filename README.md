# Agent Sentinel

**The Local Profiler & Flight Recorder for AI Agents.**

Agent Sentinel records coding-agent hooks and MCP stdio traffic in a local cockpit. Inspect tool calls, arguments, responses, errors, available durations and patches; save the recording and inspect it again offline.

A single Go executable embeds the web UI. Recording and inspection require no cloud account, API key, Docker service or external database. Sentinel does not send recordings to remote services. Your coding agent and MCP servers may still use the network independently.

## Development status

This branch starts the telemetry pivot with **passive capture**. Sentinel no longer evaluates command policies, holds calls for approval or enforces spending limits. The coding agent's own permissions still apply.

Available now:

- Passive Cursor and Claude Code hooks, with neutral responses even if the collector is offline.
- A bounded hook-delivery deadline: one second by default, configurable up to five seconds.
- MCP stdio proxying without a policy gate; requests and responses are forwarded unchanged.
- A timeline with a selectable recording map grouped by tool. Captured request/response intervals stay distinct from reported execution durations.
- A tool profile with per-tool activity, longest completed calls and largest retained responses, each linked to its captured evidence.
- A call inspector with status/tool filters, full-argument search, duration/response-size sorting, payload text search, previous/next navigation, available patch diffs and a separate event log.
- Counts of tool calls and errors, available duration percentiles and retained payload JSON size.
- Local JSONL recordings and read-only replay, including older enforcement traces.
- Standalone HTML session reports, without scripts or external assets.

**Planned, not implemented yet:** waterfall, structured capture-quality metadata, session separation, original output-size measurements, repeated-call findings, schema diagnostics and a recording catalog. See the [implementation roadmap](docs/ROADMAP.md).

![Trace Atlas timeline showing synthetic calls](docs/profiler.png)

This screenshot uses a synthetic JSONL recording. No command shown in it was executed by the fixture generator. The previous enforcement cockpit is retained in `docs/cockpit.png` as historical material.

## Build

Build prerequisites: Go 1.27+ and Node.js 20.19+ or 22.12+.

```bash
make build
./bin/sentinel version
./bin/sentinel help
```

For an installation, `make install` builds the embedded cockpit and copies the executable to `~/.sentinel/bin/sentinel`. Hook entries store its absolute path, so choose the final binary before installing hooks. Building this checkout is separate from replacing an existing installed binary.

An installed executable needs neither Go nor Node.js to run. Source builds use npm to obtain build dependencies; recording, replay and reports do not require a network connection.

## Record a coding agent

Choose the project you want to observe. Installing inside the Sentinel source tree only observes agents working in that tree.

First start the collector in a background terminal or process:

```bash
~/.sentinel/bin/sentinel serve --mode observe
```

It prints a session ID, JSONL path and cockpit URL: `http://127.0.0.1:8848`. Check the active mode before connecting hooks:

```bash
curl -sf http://127.0.0.1:8848/api/config
```

Expected: `"mode":"observe"`, `"approvals":false`, `"budgetEditable":false`, `"policy":"off"`.

Install for one agent in the target project:

```bash
# Cursor
~/.sentinel/bin/sentinel hook install --dir /path/to/project

# Claude Code
~/.sentinel/bin/sentinel hook install --agent claude --dir /path/to/project
```

Run only the command for the chosen agent. Install preserves other hooks and settings; running it again replaces Sentinel's entries rather than duplicating them. New entries include `hook --mode observe` and a six-second process timeout, allowing a margin around the client's delivery deadline.

Cursor reloads its hook file on change; if it has not loaded, reopen the project. Restart Claude Code after installation, because it loads hooks at session start.

MCP hooks are opt-in with `hook install --mcp`. Use either those hooks or the MCP proxy for the same traffic to avoid recording it twice. `--user` explicitly installs for all projects instead of one.

For the complete ordered setup and a synthetic verification, follow [AGENTS.md](AGENTS.md).

## Record an MCP stdio server

```bash
~/.sentinel/bin/sentinel mcp --ui -- /path/to/mcp-server arg1
```

The proxy writes protocol traffic to stdout and its own diagnostics to stderr. It does not modify tool permissions or server results. A proxy/server process failure can still interrupt the stdio connection.

Without `--ui`, it records without opening a cockpit listener. Use `--log /path/to/trace.jsonl` for a chosen log or `--log -` to disable file logging.

## Replay and export

```bash
~/.sentinel/bin/sentinel ui
~/.sentinel/bin/sentinel ui /path/to/trace.jsonl
```

The first opens the latest recording; the second opens the selected file. Replay never executes captured commands. Historical blocks, holds and human decisions remain visible in old recordings, without approval controls.

The cockpit's Export button downloads an offline HTML report with summary figures and expandable call history. It includes argument summaries, not full request/response payloads. The original JSONL is the trace for detailed local replay; a dedicated JSONL export flow is planned. `sentinel export` and `sentinel run` are not implemented.

## What the numbers mean

Durations are those available from the adapter or proxy; they are not the model's thinking time. Median and p95 include completed tool calls with available nonnegative durations, excluding interrupted calls and legacy approval-held spans. Zero milliseconds is valid; absent samples display as unavailable.

**JSON retained** measures UTF-8 bytes of retained tool payloads reserialized for display. Some hook payloads are synthesized, omitted or truncated. This is neither original wire volume nor actual model tokens or context usage. Sentinel no longer calculates dollars from fixed token-price assumptions.

Capture-quality metadata and better treatment of missing/ambiguous timing are planned in the next phase. Current logs do not reconstruct the full model conversation, task boundaries or the workspace state. Local recordings can contain commands, source text and tool output; inspect them before sharing. Configurable capture/redaction profiles are planned, not shipped in this phase.

## Migrating from enforcement releases

This is a behavior change. The new default is observation only; `--mode observe` makes that explicit. If you need the previous command-blocking behavior, keep the previous release.

The new CLI rejects `--policy`, `--max-cost`, `--approval-timeout` and `--offline` with migration guidance instead of silently ignoring them. Stop the old collector before starting the new one and explicitly reinstall Sentinel's hooks for each project you migrate. A new hook never forwards a legacy collector's permission decision to the agent; it returns neutrally and reports an unexpected decision on stderr.

Do not rebuild over an executable referenced by active hooks during normal development. Replacing that executable changes every project using it even if the hook files remain unchanged. Prepare and test the new build separately; perform the installed-binary replacement and hook migration only as an explicit upgrade.

Stopping the new collector loses telemetry while it is offline but does not add Sentinel confirmation prompts. Hook delivery errors are reported on stderr and acknowledged neutrally.

## Remove integration

```bash
~/.sentinel/bin/sentinel hook uninstall --dir /path/to/project
~/.sentinel/bin/sentinel hook uninstall --agent claude --dir /path/to/project
```

Use the command matching the installed agent, then stop the collector. Only Sentinel's entries are removed.

## Development checks

```bash
make test       # Go vet, race tests, frontend typecheck and telemetry/report tests
cd web
npm run build  # Typecheck and build the embedded UI
```

[Product direction](docs/PRODUCT_DIRECTION.md) · [Architecture](docs/ARCHITECTURE.md) · [Telemetry contract](docs/TELEMETRY_SPEC.md) · [Analysis specification](docs/ANALYSIS_SPEC.md) · [Roadmap](docs/ROADMAP.md) · [Progress](docs/PROGRESS.md)

MIT © Francesco Zanini. See [LICENSE](LICENSE).
