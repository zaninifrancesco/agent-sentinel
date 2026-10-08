# 🛡️ Agent Sentinel

> **The Local-First Flight Recorder, Execution Boundary & Cockpit for AI Coding Agents**

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://img.shields.io/badge/Go-1.23+-00ADD8.svg?style=flat&logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/Cockpit-React%20%2B%20Tailwind-61DAFB.svg?style=flat&logo=react)](https://react.dev)
[![MCP Compatible](https://img.shields.io/badge/Protocol-MCP%20%2F%20JSON--RPC-8A2BE2.svg)](https://modelcontextprotocol.io)

**Agent Sentinel** is a lightweight, zero-dependency execution supervisor and flight recorder for autonomous AI coding agents (Claude Code, Aider, Cursor, Codex CLI, and custom agent workflows).

It brings transparency, security guardrails, human-in-the-loop approvals, and rich audit reports to terminal and MCP-based AI development workflows.

---

## ⚡ The Problem: The Agent Black Box

Autonomous AI coding agents are changing software engineering, but their execution model remains a high-stakes black box:

- ❌ **Unbounded Execution & Accidental Disasters:** Runaway loops can wipe databases, delete repositories (`rm -rf`), or leak `.env` secrets.
- ❌ **Runaway API Costs:** No local circuit breaker to halt agents if token consumption spikes unexpectedly.
- ❌ **No Flight Recorder:** When an agent modifies 15 files and runs 30 commands, tracking the chain of decisions in a noisy terminal buffer is painful.
- ❌ **Lost Deliverables:** Architecture diagrams, benchmark reports, and diff explanations vanish once the terminal session ends.

---

## 🚀 The Solution: Agent Sentinel

```text
[ AI Coding Agent (Claude Code / Aider / CLI) ]
                       ↕ (MCP & PTY Proxy)
              ┌─────────────────┐
              │ AGENT SENTINEL  │
              └────────┬────────┘
     ┌─────────────────┼─────────────────┐
     ▼                 ▼                 ▼
[ Policy Engine ] [ Flight Recorder ] [ Local Cockpit UI ]
  • Cost Limits     • Event Log         • Live Timeline
  • Secret Shield   • Step Replay       • Visual Diff Viewer
  • Human Approvals • SQLite Store      • 1-Click HTML Report
```

Agent Sentinel sits transparently between your agent and your system:

1. **Transparent Proxying:** Intercepts MCP (Model Context Protocol) tool calls and terminal execution without modifying agent code.
2. **Security & Cost Boundary:** Halts dangerous commands or runaway token spend, pausing execution until approved via the browser UI.
3. **Local-First Cockpit:** Serves an embedded web dashboard (`localhost:8848`) with a live timeline, step-by-step reasoning, and visual file diffs.
4. **Standalone Report Generator:** Generates an all-in-one, zero-dependency HTML audit report to attach to GitHub PRs or share with your team.

---

## 🛠️ High-Level Architecture & Tech Stack

- **Backend & Core Engine:** Go (Golang) — compiled into a single static binary with zero external dependencies.
- **Frontend Cockpit:** React + TypeScript + Tailwind CSS — embedded directly into the Go executable via `go:embed`.
- **Protocols:** Model Context Protocol (MCP / JSON-RPC 2.0) + WebSocket streaming.
- **Storage:** Local event store with structured session exports.

For the full architectural breakdown, threat model, and event schema, see the [Architecture Document](docs/ARCHITECTURE.md).

---

## 💻 CLI Usage Preview

```bash
# Supervise the Cursor agent: shell commands, MCP calls and file edits.
sentinel hook install     # adds Sentinel to .cursor/hooks.json (other hooks stay)
sentinel serve --open     # cockpit + policy + budget; Cursor waits for your verdict

# Put Sentinel in front of one MCP server (Cursor, Claude Desktop, ...)
sentinel mcp --ui -- npx -y @modelcontextprotocol/server-filesystem .

# Replay a recorded session
sentinel ui
```

The audit report is exported from the cockpit (Export). `sentinel export` and
`sentinel run` (a PTY wrapper for terminal agents) are planned, not built yet.

---

## 🗺️ Roadmap & Milestones

- [ ] **Milestone 1: Core Proxy & Protocol Sniffer** (JSON-RPC MCP interceptor & CLI supervisor)
- [ ] **Milestone 2: Real-time Streaming & Embedded Cockpit** (WebSocket event bus & React UI via `go:embed`)
- [ ] **Milestone 3: Policy Engine & Human-in-the-Loop Hooks** (Command sandboxing, token circuit breaker, approval modal)
- [ ] **Milestone 4: Visual Diff Viewer & Standalone HTML Reports** (Git diff integration & self-contained deliverable exports)
- [ ] **Milestone 5: Production Packaging & Release** (Multi-platform binaries via GoReleaser, Homebrew formula)

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.
