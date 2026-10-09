# Setup playbook

Follow this when a user asks you to install Agent Sentinel, set it up, or "make it work". Do the steps in order. After each step, the check must pass before you continue. If a check fails, stop and report the command and its output. Do not invent a different install path.

"Working" means all of these:

1. `~/.sentinel/bin/sentinel version` prints a version.
2. `sentinel serve` is running, and `http://127.0.0.1:8848/api/config` returns JSON with `"approvals": true`.
3. The target project's hook file lists Sentinel, and every hook that was already there is unchanged. For Cursor that is `.cursor/hooks.json` with `beforeShellExecution`, `afterShellExecution`, `beforeReadFile`, `afterFileEdit` and `stop`. For Claude Code it is `.claude/settings.local.json` with `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `Stop` and `SessionEnd`.
4. A synthetic hook call is allowed and shows up in the cockpit.

Which agent? If the user did not say, ask: Cursor or Claude Code. Do both only when asked. Steps 3 to 5 below say what differs.

## Do not

- Do not run `sentinel run`. It is not implemented.
- Do not run `sentinel export`. The report is the Export button in the cockpit.
- Do not pass `--mcp` to `hook install` unless the user asked to gate MCP tools. A server already started with `sentinel mcp` would be judged twice.
- Do not pass `--user` unless the user asked for every project. The default is one project.
- Do not edit `hooks.json` or `.claude/settings.local.json` by hand. `sentinel hook install` adds Sentinel and leaves everything else alone.
- Do not move or rebuild over the binary after installing hooks. The hook file stores its absolute path.
- Do not commit `.cursor/`, `.claude/`, `.codex/` or `.impeccable/questions/`. They are local.
- Do not start `sentinel serve` in the foreground. It runs until stopped.

## 0. Choose the target project

The target is the project whose Cursor or Claude Code agent should be supervised. If the user said "this repo" or "here", that is the workspace root. Say which directory you chose.

Installing hooks inside the Agent Sentinel source tree only supervises an agent working in that tree. It does not turn Sentinel on for other projects.

## 1. Get the binary

If `~/.sentinel/bin/sentinel version` already prints a version, keep that binary and go to step 2. Rebuild only when the user asked to build this checkout.

From this repository you need Go 1.27+ and Node.js 20.19+ or 22.12+. Check both, then:

```bash
make install
~/.sentinel/bin/sentinel version
```

`make install` builds the cockpit with npm, embeds it, and copies the binary to `~/.sentinel/bin/sentinel`. That path is where it must stay.

If Go or Node is missing, stop and say which one. Do not download a release archive unless building from source is impossible.

## 2. Start the cockpit before installing hooks

Hooks installed while nothing is listening make the agent ask the user to confirm every later command, including yours. Start Sentinel first.

```bash
~/.sentinel/bin/sentinel serve
```

Run it in the background. It listens on `127.0.0.1:8848` and prints the session id and the JSONL log path; keep both.

Check:

```bash
curl -sf http://127.0.0.1:8848/api/config
```

The JSON must contain `"approvals": true` and `"budgetEditable": true`.

If the port is taken, curl it. A JSON config means Sentinel is already running: reuse it and do not start a second one. Anything else, stop and say the port is occupied. Do not switch ports. The installed hook calls `http://127.0.0.1:8848` and has no port flag.

## 3. Install the hooks

Cursor:

```bash
~/.sentinel/bin/sentinel hook install --dir "<target>"
```

Claude Code:

```bash
~/.sentinel/bin/sentinel hook install --agent claude --dir "<target>"
```

Check that `<target>/.cursor/hooks.json` contains exactly one `"sentinel hook"` command for each of `beforeShellExecution`, `afterShellExecution`, `beforeReadFile`, `afterFileEdit` and `stop`, and that every pre-existing hook is still there. Run it again if you need to; a second install replaces Sentinel's own entries instead of duplicating them.

For Claude Code, check that `<target>/.claude/settings.local.json` has exactly one `"sentinel hook"` command under each of `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `Stop` and `SessionEnd`, and that its other keys and hooks are still there. Claude Code reads hooks when a session starts: a `claude` already running in that project must be restarted, or use `/hooks` to see what it loaded.

Cursor reloads `hooks.json` when it changes. If the user has Cursor open on that project, tell them to look at Cursor Settings, Hooks tab. An empty list means Cursor has not loaded the file yet: reopen the project.

## 4. Prove the path works

This does not depend on Cursor or Claude Code having reloaded. Cursor reports a command twice, before it runs and after it finishes. Send both, in this order, or the row stays on "Running" forever:

```bash
printf '%s\n' '{"hook_event_name":"beforeShellExecution","conversation_id":"setup","command":"echo sentinel-setup-ok","cwd":"<target>"}' \
  | ~/.sentinel/bin/sentinel hook

printf '%s\n' '{"hook_event_name":"afterShellExecution","conversation_id":"setup","command":"echo sentinel-setup-ok","output":"sentinel-setup-ok\n","duration":1}' \
  | ~/.sentinel/bin/sentinel hook
```

The first reply must be `{"permission":"allow"}` and the second `{}`. Then:

```bash
curl -sf http://127.0.0.1:8848/api/events
```

Claude Code reports a tool call twice as well (`PreToolUse`, then `PostToolUse`), so send both:

```bash
printf '%s\n' '{"hook_event_name":"PreToolUse","session_id":"setup","cwd":"<target>","tool_name":"Bash","tool_input":{"command":"echo sentinel-setup-ok"},"tool_use_id":"setup-1"}' \
  | ~/.sentinel/bin/sentinel hook

printf '%s\n' '{"hook_event_name":"PostToolUse","session_id":"setup","tool_name":"Bash","tool_input":{"command":"echo sentinel-setup-ok"},"tool_response":{"stdout":"sentinel-setup-ok","stderr":"","interrupted":false},"tool_use_id":"setup-1","duration_ms":1}' \
  | ~/.sentinel/bin/sentinel hook
```

Both replies must be `{}`: for Claude Code, Sentinel says nothing about a call it has no objection to, and Claude's own permission rules still apply.

Either way, the events must include a `shell` tool call whose command is `echo sentinel-setup-ok`, followed by its response with status `ok`. If that call has no response, you only sent the first command. Send the second.

Tell the user the cockpit is at http://127.0.0.1:8848. For Claude Code, `claude` must be started after the hooks were installed. The first real agent command in that project should appear as a new row. `cat .env` is the demonstration of a block: the policy refuses it and the agent receives the refusal. Do not run `cat .env` yourself against a real secrets file.

## 5. Report back

Tell the user, and stop:

- the binary path and the version
- the cockpit URL and the JSONL log path `sentinel serve` printed
- the hook file path (`hooks.json` or `.claude/settings.local.json`)
- that the synthetic call was allowed and recorded
- that `sentinel serve` keeps running in the background, and that quitting it makes the agent ask for confirmation on every command until it is started again

## Undo

```bash
~/.sentinel/bin/sentinel hook uninstall --dir "<target>"                 # Cursor
~/.sentinel/bin/sentinel hook uninstall --agent claude --dir "<target>"  # Claude Code
```

Then stop the `sentinel serve` process. Uninstall removes only Sentinel's entries.
