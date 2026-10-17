---
name: agent-sentinel
description: Autonomous setup, management, and inspection of Agent Sentinel — the local profiler & flight recorder for AI coding agents (Claude Code, Cursor, Codex). Use whenever the user asks to install, setup, start, configure, verify, or inspect Agent Sentinel, check agent latencies, view recorded tool calls, or debug agent performance.
version: 0.2.0
license: MIT
---

# Agent Sentinel Skill

Agent Sentinel is a local, passive flight recorder and profiler for AI coding agents (Cursor, Claude Code, Codex) and Model Context Protocol (MCP) servers.

It records actions without blocking commands, asking for approvals, or enforcing budgets. The coding agent's own permissions still apply. 100% local, zero cloud dependencies.

---

## 1. Setup & Installation Playbook

When asked to install, set up, or enable Agent Sentinel, execute the following steps in order. A failed check stops setup; report the failed command and output to the user.

### Step 1: Ensure Sentinel Binary is Available

Check if the binary exists and supports observation mode:

```bash
~/.sentinel/bin/sentinel version
~/.sentinel/bin/sentinel help
```

If the binary is missing or not executable:
- If running from within the `agent-sentinel` repository, run:
  ```bash
  ./install.sh
  ```
- If running standalone on the user's machine, run:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/zaninifrancesco/agent-sentinel/main/install.sh | bash
  ```
- Alternatively, if Go 1.27+ and Node.js are available in the repository root, build and install with:
  ```bash
  make install
  ```

Verify that `~/.sentinel/bin/sentinel version` succeeds before proceeding.

### Step 2: Start & Verify the Collector

Check if the collector is already running:

```bash
curl -sf http://127.0.0.1:8848/api/config
```

- If running, verify the config reports:
  `"mode":"observe"`, `"policy":"off"`, `"approvals":false`, `"budgetEditable":false`, and `"approvalTimeoutSec":0`.
- If not running (connection refused), start the collector in the background:
  ```bash
  nohup ~/.sentinel/bin/sentinel serve --mode observe > ~/.sentinel/serve.log 2>&1 &
  ```
- Wait a moment and check `curl -sf http://127.0.0.1:8848/api/config` again.

### Step 3: Install Hooks for the Current Agent

Target directory is the project root where the agent is active (or pass `--user` if the user asked to record every project).

#### For Cursor:
```bash
~/.sentinel/bin/sentinel hook install --dir "<target-project-dir>"
```
*Verifies `.cursor/hooks.json` contains Sentinel handlers for `beforeShellExecution`, `afterShellExecution`, `beforeReadFile`, `afterFileEdit`, and `stop`.*

#### For Claude Code:
```bash
~/.sentinel/bin/sentinel hook install --agent claude --dir "<target-project-dir>"
```
*Verifies `.claude/settings.local.json` contains Sentinel handlers for `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `Stop`, and `SessionEnd`.*

### Step 4: Prove Request and Response Recording (Self-Test)

Send a synthetic test event pair to verify end-to-end recording without executing any command.

**Cursor test:**
```bash
printf '%s\n' '{"hook_event_name":"beforeShellExecution","conversation_id":"setup","command":"echo sentinel-setup-ok","cwd":"<target-project-dir>"}' \
  | ~/.sentinel/bin/sentinel hook --mode observe

printf '%s\n' '{"hook_event_name":"afterShellExecution","conversation_id":"setup","command":"echo sentinel-setup-ok","output":"sentinel-setup-ok\n","duration":1}' \
  | ~/.sentinel/bin/sentinel hook --mode observe
```
The replies must be `{"permission":"allow"}` and `{}`.

**Claude Code test:**
```bash
printf '%s\n' '{"hook_event_name":"PreToolUse","session_id":"setup","cwd":"<target-project-dir>","tool_name":"Bash","tool_input":{"command":"echo sentinel-setup-ok"},"tool_use_id":"setup-1"}' \
  | ~/.sentinel/bin/sentinel hook --mode observe

printf '%s\n' '{"hook_event_name":"PostToolUse","session_id":"setup","tool_name":"Bash","tool_input":{"command":"echo sentinel-setup-ok"},"tool_response":{"stdout":"sentinel-setup-ok","stderr":"","interrupted":false},"tool_use_id":"setup-1","duration_ms":1}' \
  | ~/.sentinel/bin/sentinel hook --mode observe
```
Both replies must be `{}`.

**Confirm capture:**
```bash
curl -sf http://127.0.0.1:8848/api/events
```
Confirm that `echo sentinel-setup-ok` is recorded with status `ok`.

### Step 5: Report to the User

Provide a clean summary:
1. **Status**: Agent Sentinel is running in observation mode.
2. **Cockpit UI**: [http://127.0.0.1:8848](http://127.0.0.1:8848)
3. **Session Logs**: `~/.sentinel/sessions/*.jsonl`
4. **Agent note**: For Claude Code, remind the user to start a new session (`claude`) so that settings are freshly reloaded.

---

## 2. Inspecting Telemetry & Profiling Data

When the user asks about performance, latencies, or what tools were executed:

### Query Bottlenecks & Slowest Tools
```bash
curl -sf http://127.0.0.1:8848/api/v1/profiler
```
Returns:
- `top_completed_calls_by_duration`: Longest-running tool executions.
- `largest_retained_responses`: Heavy tool payloads.
- `tool_duration_summary`: Aggregate time spent per tool.

### Query Aggregated Analytics
```bash
curl -sf http://127.0.0.1:8848/api/v1/analytics
```
Returns session counts, p50/p95 execution durations, error counts, and daily UTC trends.

### Query Raw Event Log
```bash
curl -sf http://127.0.0.1:8848/api/events
```
Returns the array of recorded events for the active session.

---

## 3. Uninstallation / Disabling

When asked to uninstall or remove Sentinel:

```bash
# For Cursor
~/.sentinel/bin/sentinel hook uninstall --dir "<target-project-dir>"

# For Claude Code
~/.sentinel/bin/sentinel hook uninstall --agent claude --dir "<target-project-dir>"
```
Removes only Sentinel entries, leaving all custom user hooks intact.
