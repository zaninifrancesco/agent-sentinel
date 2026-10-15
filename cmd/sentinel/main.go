package main

import (
	"fmt"
	"os"
)

// Version is stamped at build time (make build, make dist): -X main.Version=...
var Version = "0.2.0-dev"

const (
	Banner = `
   _____                    __     _____            __  _            __
  /  _  \   ____   ____   _/  |_  /  ___/  ____    /  |_(_)  ____   /  |
 /  /_\  \ / ___\_/ __ \  \   __\ \___ \ _/ __ \  /   __\ | /    \ /   |
/    |    / /_/  >  ___/   |  |   /____  >\  ___/   |  |  | |   |  \    |
\____|__  \___  / \___  >  |__|  /______  / \___  >  |__| |_|_|_|  /___|
        \/_____/      \/                \/      \/               \/    
   The Local Profiler & Flight Recorder for AI Agents
`
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "version", "--version", "-v":
		fmt.Printf("Agent Sentinel v%s\n", Version)
	case "help", "--help", "-h":
		printHelp()
	case "mcp":
		os.Exit(runMCP(os.Args[2:]))
	case "ui":
		os.Exit(runUI(os.Args[2:]))
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "hook":
		os.Exit(runHook(os.Args[2:]))
	case "run":
		// Recording is provided by hooks and MCP stdio, not a PTY wrapper.
		fmt.Fprintln(os.Stderr, "sentinel run is not implemented yet.")
		fmt.Fprintln(os.Stderr, "To record a coding agent: `sentinel hook install`, then `sentinel serve`.")
		os.Exit(2)
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(Banner)
	fmt.Printf("Agent Sentinel v%s\n\n", Version)
	fmt.Println("Usage:")
	fmt.Println("  sentinel serve [--mode observe] [--port N] [--log FILE|-] [--open]")
	fmt.Println("                                Record Cursor / Claude Code hooks in the local cockpit")
	fmt.Println("  sentinel hook install [--agent cursor|claude] [--dir PATH] [--mcp] [--user] [--dry-run]")
	fmt.Println("                                Install passive hooks, preserving existing entries")
	fmt.Println("  sentinel hook uninstall [--agent cursor|claude] [--dir PATH] [--user]")
	fmt.Println("                                Remove only Sentinel hooks")
	fmt.Println("  sentinel hook [--mode observe] [--timeout 1s] [--url URL]")
	fmt.Println("                                Deliver hook JSON from stdin; neutral even offline")
	fmt.Println("  sentinel mcp [--mode observe] [--ui] [--port N] [--log FILE|-] -- <server-cmd> [args]")
	fmt.Println("                                Forward MCP stdio unchanged and record its frames")
	fmt.Println("  sentinel ui [--port N] [--open] [FILE]")
	fmt.Println("                                Inspect a recorded JSONL session (default: latest)")
	fmt.Println("  sentinel version              Show current version")
	fmt.Println("  sentinel help                 Show this help message")
	fmt.Println()
	fmt.Println("Observation only: no command blocking, approval holds or spending limits.")
	fmt.Println("Your coding agent's own permissions still apply. Existing installations require an explicit migration.")
	fmt.Println("Removed options: --policy, --max-cost, --approval-timeout, --offline. Keep the previous release if you need enforcement.")
}
