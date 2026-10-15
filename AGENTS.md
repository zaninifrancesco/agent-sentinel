# Setup playbook — observation release

Follow this when asked to install or set up Agent Sentinel. Do the steps in order; a failed check stops setup. Report the failed command and output instead of inventing another installation path.

Sentinel records actions. It does not block commands, ask for approval or enforce budgets. The coding agent's own permissions still apply. The implementation roadmap is in `docs/ROADMAP.md`; planned analysis features must not be advertised as already shipped.

## Working means

1. `~/.sentinel/bin/sentinel version` prints a version and `sentinel help` describes observation mode.
2. A background `sentinel serve` answers at `http://127.0.0.1:8848/api/config` with `"mode":"observe"`, `"approvals":false`, `"budgetEditable":false` and `"policy":"off"`.
3. The target project's hook file has one Sentinel entry for each required event, with `hook --mode observe`; every pre-existing non-Sentinel hook and setting is unchanged.
4. A synthetic request and response are recorded as a completed call, without a policy decision.

If the user has not chosen an agent, ask Cursor or Claude Code. Configure both only when asked.

## Do not

- Do not run `sentinel run` or `sentinel export`; they are not implemented. HTML export is the cockpit's Export button.
- Do not pass `--policy`, `--max-cost`, `--approval-timeout` or `--offline`; these enforcement options are rejected by the observation release.
- Do not pass `--mcp` unless the user asked to record agent MCP hooks. Use one source for the same MCP traffic: hooks or the stdio proxy.
- Do not pass `--user` unless the user asked for every project.
- Do not edit agent hook files by hand; use `sentinel hook install` and `uninstall`.
- Do not move or rebuild over an executable referenced by existing hooks during routine development. Replacing it changes all integrations using that path.
- Do not commit `.cursor/`, `.claude/`, `.codex/` or `.impeccable/questions/`. They are local.
- Do not start `sentinel serve` in the foreground. It runs until stopped.
- Do not execute a dangerous command or read a real secrets file to test capture. Synthetic payloads do not need to run their command.

## 0. Choose the target and handle migration

The target is the project whose agent should be recorded. If the user said "here" or "this repo", use the workspace root. State the chosen directory. Hooks inside Sentinel's source tree do not record agents working on other projects.

If the target uses an enforcement release, explain that the new version removes Sentinel blocking and prompts. A general development request does not authorize replacing the installed executable or migrating every project that references it. An explicit observation-upgrade request authorizes that migration; preserve all unrelated hooks.

## 1. Get the observation binary

Check the existing installation:

```bash
~/.sentinel/bin/sentinel version
~/.sentinel/bin/sentinel help
```

Reuse it if it describes observation mode, unless the user asked to build this checkout. If it is an older enforcement binary, stop and report that an explicit upgrade is needed; do not silently replace it.

For a missing binary or an explicitly requested source build, check Go 1.27+ and Node.js 20.19+ or 22.12+, then:

```bash
make install
~/.sentinel/bin/sentinel version
~/.sentinel/bin/sentinel help
```

`make install` embeds the cockpit and copies the executable to `~/.sentinel/bin/sentinel`. If Go or Node is missing, stop and report which prerequisite is missing. Do not choose another installation path or download a release archive unless building from source is impossible.

## 2. Start and check the collector

If port 8848 is occupied, curl its config before starting anything. Reuse an observation collector. If the config is from an older enforcement release, stop and report the required explicit migration; if it is not Sentinel JSON, report that the port is occupied. Do not choose another port for installed hooks.

Start in the background and keep the printed session ID and JSONL path:

```bash
~/.sentinel/bin/sentinel serve --mode observe
```

Check:

```bash
curl -sf http://127.0.0.1:8848/api/config
```

Require `mode=observe`, `policy=off`, `approvals=false`, `budgetEditable=false` and `approvalTimeoutSec=0`. Starting first makes the setup evidence available; offline hooks no longer ask or deny.

## 3. Install the chosen agent's hooks

Read and preserve the target hook/settings file before installation so the check can compare non-Sentinel entries.

Cursor:

```bash
~/.sentinel/bin/sentinel hook install --dir "<target>"
```

Require exactly one Sentinel command under each of `beforeShellExecution`, `afterShellExecution`, `beforeReadFile`, `afterFileEdit` and `stop` in `<target>/.cursor/hooks.json`.

Claude Code:

```bash
~/.sentinel/bin/sentinel hook install --agent claude --dir "<target>"
```

Require exactly one Sentinel handler under each of `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `Stop` and `SessionEnd` in `<target>/.claude/settings.local.json`.

Each generated command includes `hook --mode observe` and the executable's absolute path. Process timeouts are six seconds, independent of tool execution time. A second install must replace Sentinel's own entries instead of duplicating them. Verify every other hook, matcher and setting is unchanged.

Cursor reloads hooks on file changes. Tell the user to inspect Cursor Settings, Hooks; if empty, reopen the project. Claude Code must be restarted after installation to load the hooks.

## 4. Prove request and response recording

Send only the pair for the chosen agent. These are synthetic hook messages; they do not execute the command.

Cursor:

```bash
printf '%s\n' '{"hook_event_name":"beforeShellExecution","conversation_id":"setup","command":"echo sentinel-setup-ok","cwd":"<target>"}' \
  | ~/.sentinel/bin/sentinel hook --mode observe

printf '%s\n' '{"hook_event_name":"afterShellExecution","conversation_id":"setup","command":"echo sentinel-setup-ok","output":"sentinel-setup-ok\n","duration":1}' \
  | ~/.sentinel/bin/sentinel hook --mode observe
```

The replies must be `{"permission":"allow"}` and `{}`.

Claude Code:

```bash
printf '%s\n' '{"hook_event_name":"PreToolUse","session_id":"setup","cwd":"<target>","tool_name":"Bash","tool_input":{"command":"echo sentinel-setup-ok"},"tool_use_id":"setup-1"}' \
  | ~/.sentinel/bin/sentinel hook --mode observe

printf '%s\n' '{"hook_event_name":"PostToolUse","session_id":"setup","tool_name":"Bash","tool_input":{"command":"echo sentinel-setup-ok"},"tool_response":{"stdout":"sentinel-setup-ok","stderr":"","interrupted":false},"tool_use_id":"setup-1","duration_ms":1}' \
  | ~/.sentinel/bin/sentinel hook --mode observe
```

Both replies must be `{}`. An explicit Claude `allow` would override native permissions and is not the observation contract.

Check:

```bash
curl -sf http://127.0.0.1:8848/api/events
```

Require a `shell` request for `echo sentinel-setup-ok` and its matching response with status `ok`, without `decision` or `rule`. A request alone is not completed setup: send the response before reporting success.

## 5. Report and stop

Report binary path and version; cockpit URL; session ID and JSONL path; hook file path; and successful synthetic capture. Explain that the background collector keeps recording until stopped. If it stops, new hooks acknowledge neutrally after a bounded delivery attempt: telemetry is unavailable, but Sentinel does not add confirmations. Native agent permissions still apply.

For Claude Code, remind the user to start a new session. The first real agent action should appear as a new row. Do not describe `.env` access as a Sentinel block demonstration; the observation release has no command policy.

## Undo

```bash
~/.sentinel/bin/sentinel hook uninstall --dir "<target>"
~/.sentinel/bin/sentinel hook uninstall --agent claude --dir "<target>"
```

Run the command for the chosen agent, then stop the collector. Uninstall removes only Sentinel entries and leaves other settings intact.
