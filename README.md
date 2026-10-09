# Agent Sentinel

**A local flight recorder, execution boundary and cockpit for AI coding agents.**

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![MCP](https://img.shields.io/badge/Protocol-MCP-8A2BE2.svg)](https://modelcontextprotocol.io)

Site: [agent-sentinel.runs-on.dev](https://agent-sentinel.runs-on.dev/)

Sentinel is a single Go binary. It sits between an AI coding agent and your machine, records what the agent does, stops what looks dangerous until you say yes, and shows it all in a local web cockpit.

It works with the **Cursor agent** and **Claude Code** (through their hooks) and with **any MCP server** (as a stdio proxy). Everything runs on your machine, on `127.0.0.1`: no cloud, no account.

A coding agent setting this up should follow [AGENTS.md](AGENTS.md). It is the ordered procedure, with the checks that mean it is actually working.

![The cockpit: a live timeline of the agent's calls, with a blocked read of .env.local and a recursive delete held for approval](docs/cockpit.png)

## What it does

- **Records** every shell command, file read, file edit and MCP call the agent makes, with its result and timing, in a live timeline. Sessions are saved as JSONL and can be replayed.
- **Judges** each call against a policy before it runs. Destructive commands and secret files are blocked, risky ones wait for you, and you answer from the cockpit, with an optional note the agent reads.
- **Limits spend.** A session budget (cost and tokens) holds the next call for your approval once the estimate reaches it. You can change it from the cockpit while the session runs.
- **Reports.** The cockpit exports a single offline HTML audit report: what policy and you decided on each call, and the timetable with every call's journey.

## Install

### Prebuilt binary (macOS and Linux)

Download the archive for your system from the [latest release](https://github.com/zaninifrancesco/agent-sentinel/releases), check it against `checksums.txt`, and put the binary somewhere it will stay:

```bash
shasum -a 256 -c checksums.txt --ignore-missing
tar -xzf sentinel_*_darwin_arm64.tar.gz          # or linux_amd64, linux_arm64, darwin_amd64
mkdir -p ~/.sentinel/bin
mv sentinel_*/sentinel ~/.sentinel/bin/
```

The binaries are not signed or notarized. If macOS refuses one that a browser downloaded, run `xattr -d com.apple.quarantine ~/.sentinel/bin/sentinel`.

Keep the binary where it is once you use it with Cursor or Claude Code: their hook files store its absolute path.

### From source

You need Go 1.27+ and Node.js 20.19+ (or 22.12+), because the cockpit is built with Vite and embedded in the binary.

```bash
git clone https://github.com/zaninifrancesco/agent-sentinel
cd agent-sentinel
make build                 # builds the cockpit, then bin/sentinel
make install               # copies it to ~/.sentinel/bin/sentinel
```

`make dist` builds the release archives, and pushing a tag such as `v0.1.0` publishes them.

## Use it with the Cursor agent

```bash
cd your-project
sentinel hook install      # adds Sentinel to .cursor/hooks.json; other hooks stay as they are
sentinel serve --open      # cockpit on http://127.0.0.1:8848
```

Then work with the agent in that project as usual. Cursor reports each step to `sentinel serve` and waits for its verdict:

| The agent... | Sentinel |
| --- | --- |
| runs a shell command | allows it, blocks it, or holds it for you |
| reads a file | judges the path (`.env`, SSH keys and credentials are blocked or held); only the path and size are recorded, never the content |
| edits a file | records the diff and flags a secret it wrote. The edit has already happened, so it can be flagged, not stopped |
| calls an MCP tool (`--mcp`) | judges it like a shell command |

If `sentinel serve` is not running, Cursor asks you to confirm each command instead of letting it through. File reads cannot be asked about, so they are allowed; `sentinel hook install --offline deny` refuses both.

`sentinel hook install --user` installs for every project, `--dry-run` shows the result without writing, and `sentinel hook uninstall` removes only Sentinel's entries.

A command Cursor never reports back on (skipped, for example) shows as "Running" until the turn ends, then as **Interrupted**. `sentinel serve` prints a line each time Cursor ends a turn, so you can see the hook is wired.

## Use it with Claude Code

```bash
cd your-project
sentinel hook install --agent claude   # adds Sentinel to .claude/settings.local.json; everything else in it stays as it is
sentinel serve --open                  # cockpit on http://127.0.0.1:8848
claude                                 # hooks are read when a session starts
```

`.claude/settings.local.json` is the project's personal settings file, which Claude Code keeps out of git. `--user` writes `~/.claude/settings.json` instead (every project). The same policy, budget, approvals and cockpit apply as for Cursor:

| The agent... | Sentinel |
| --- | --- |
| runs a shell command (`Bash`, `PowerShell`) | allows it, blocks it, or holds it for you |
| reads a file (`Read`) | judges the path before the read; only the path and the size returned are recorded, never the content |
| writes or edits a file (`Write`, `Edit`, `MultiEdit`, `NotebookEdit`) | judges the path and what is being written **before** the edit, and records the diff. A `.env` write is blocked, a secret written needs your approval |
| searches or fetches (`Glob`, `Grep`, `WebFetch`, `WebSearch`) | records it, with its result |
| calls an MCP tool (`--mcp`) | judges it like a shell command, named `server:tool` |

Two things work differently from Cursor:

- **Sentinel only objects.** A call the policy has no problem with gets no answer from Sentinel, so Claude Code's own permission rules still decide whether to ask you. An explicit "allow" would switch them off. Sentinel says "allow" only when you approved the call in the cockpit.
- **A call you rejected or the policy blocked never runs**, and Claude reads the reason (and your note) as the tool's result.

If `sentinel serve` is not running, Claude Code asks you to confirm each call (`--offline deny` refuses instead). A call that is interrupted with Esc, or that you decline in Claude Code's own prompt, sends no result; Sentinel closes it as **interrupted** when the turn ends or the session closes. `sentinel hook uninstall --agent claude` removes only Sentinel's entries.

## Use it in front of an MCP server

In your client's MCP configuration, run the server through Sentinel:

```bash
sentinel mcp --ui -- npx -y @modelcontextprotocol/server-filesystem .
```

The same policy, budget and approvals apply to every `tools/call`. Without `--ui` there is no cockpit to approve in, so anything that needs approval is refused.

## Replay a session

Sessions are written to `~/.sentinel/sessions/`.

```bash
sentinel ui                # the latest one
sentinel ui path/to/session.jsonl
```

## The default policy

| Decision | Rules |
| --- | --- |
| **Block** | `rm -rf /` or `~`, `mkfs`/`dd` onto a device, fork bombs, `.env` files, SSH keys |
| **Ask you** | recursive `rm`, `git push --force`, `git reset --hard` and similar, `curl ... \| sh`, `sudo`, destructive SQL, cloud/registry credentials, private keys, and API tokens or private keys written in a call |
| **Warn** | `chmod 777`, hard-coded credential assignments |

Change it with a JSON file, passed as `--policy FILE` (or `--policy off` to only record):

```json
{
  "rules": { "sudo": "block", "chmod-777": "allow" },
  "extraRules": [
    { "id": "no-prod-db", "description": "touches the production DB",
      "pattern": "prod-db\\.internal", "decision": "approve", "risk": "high" }
  ],
  "maxCostUsd": 5.0,
  "maxTokens": 200000
}
```

Unknown keys are rejected, so a typo in a security policy cannot pass silently.

## Budget

`--max-cost USD` (default 2) and `maxTokens` in the policy file set the session budget. When the estimate reaches it, the next call is held. If you approve, the limit rises by one step. The **Limits** button in the cockpit sets, raises or removes both limits during the session, and each change is recorded in the log.

Cost and tokens are **estimates** from the size of tool calls and results (about 4 characters per token). Sentinel does not see the model's own traffic, so real spend is higher.

## Commands

| Command | |
| --- | --- |
| `sentinel serve` | cockpit, policy, budget and approvals for the agents' hooks |
| `sentinel hook` | what Cursor or Claude Code runs per agent step (reads JSON on stdin) |
| `sentinel hook install` / `uninstall` | add or remove Sentinel in `.cursor/hooks.json`, or with `--agent claude` in `.claude/settings.local.json` |
| `sentinel mcp -- <server>` | proxy one MCP stdio server |
| `sentinel ui [FILE]` | replay a recorded session |
| `sentinel version`, `sentinel help` | |

Common flags: `--port N`, `--open`, `--policy`, `--max-cost`, `--approval-timeout` (how long a held call waits for you, default 2m), `--log FILE`.

## In the cockpit

`⌘/Ctrl+Enter` approves the held call, `Esc` rejects it, `J`/`K` move through the timeline, `⌘K` searches, **Limits** changes the budget and **Export** builds the audit report. The cockpit starts from the system light or dark theme and remembers a switch you make; the report follows the reader's system theme.

## What it does not do

- **It cannot undo or stop a file edit** made through Cursor's hooks, only record and flag it. Claude Code reports an edit before it happens, so there it can stop it.
- **It sees only what the agents' hooks and MCP report.** Anything an agent does outside them is not covered. A file the Claude Code user attaches with `@` is added to the prompt without any tool call, so no hook sees it.
- **Claude Code support is tested against its documented hook format, not yet against a live Claude Code.** Treat the first real session as the test, and open an issue for anything it does differently.
- **A command you stop in Cursor still shows as done.** Cursor then sends the "finished" event as if it had completed, with whatever output it had, so Sentinel cannot tell. Claude Code does say so, and the row shows as interrupted.
- **Cost figures are estimates**, as above.
- **One session per `sentinel serve`.** Switching projects does not start a new one.
- **There is no authentication.** The cockpit listens on `127.0.0.1` only and checks the Host and Origin of every request, but anything running as you on this machine can reach it.
- **macOS and Linux only** for now. Hook installation has not been tried on Windows.
- **Not built yet:** `sentinel run` (it will start Claude Code with Sentinel's hooks for that run only; there is no plan for a PTY wrapper for other terminal agents such as Aider) and `sentinel export` (the report is exported from the cockpit).

## Development

```bash
make test                  # go vet, go test -race
make dev-web               # Vite with hot reload, proxying to a running sentinel
cd web && npm run typecheck
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [PRODUCT.md](PRODUCT.md) and [DESIGN.md](DESIGN.md).

## Roadmap

- [x] Protocol sniffer and flight recorder (MCP over stdio)
- [x] Embedded cockpit with a live timeline
- [x] Policy engine, human approvals and the session budget
- [x] Diff viewer and the standalone HTML audit report
- [x] Supervision of the Cursor agent through its hooks
- [x] Supervision of Claude Code through its hooks (not yet tried on a live session)
- [x] Packaged releases for macOS and Linux (`v0.1.0-alpha`)
- [ ] `sentinel run claude`, starting Claude Code with Sentinel's hooks for that run only
- [ ] Frontend tests

## License

MIT, see [LICENSE](LICENSE).
