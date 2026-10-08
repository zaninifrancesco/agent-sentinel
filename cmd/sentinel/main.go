package main

import (
	"fmt"
	"os"
)

// Version is stamped at build time (make build, make dist): -X main.Version=...
var Version = "0.1.0-alpha"

const (
	Banner = `
   _____                    __     _____            __  _            __
  /  _  \   ____   ____   _/  |_  /  ___/  ____    /  |_(_)  ____   /  |
 /  /_\  \ / ___\_/ __ \  \   __\ \___ \ _/ __ \  /   __\ | /    \ /   |
/    |    / /_/  >  ___/   |  |   /____  >\  ___/   |  |  | |   |  \    |
\____|__  \___  / \___  >  |__|  /______  / \___  >  |__| |_|_|_|  /___|
        \/_____/      \/                \/      \/               \/    
   The Local-First Flight Recorder & Execution Boundary for AI Coding Agents
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
		// The PTY wrapper is not built yet. Say so instead of pretending.
		fmt.Fprintln(os.Stderr, "sentinel run is not implemented yet.")
		fmt.Fprintln(os.Stderr, "To supervise the Cursor agent: `sentinel hook install`, then `sentinel serve`.")
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
	fmt.Println("  sentinel serve [--port N] [--policy FILE|off] [--max-cost USD]")
	fmt.Println("                                Run the cockpit for the Cursor agent: Cursor hooks report here and")
	fmt.Println("                                wait for the verdict (policy, budget, your approval)")
	fmt.Println("  sentinel hook install [--user] [--mcp] [--dry-run]")
	fmt.Println("                                Add Sentinel to Cursor's hooks.json (uninstall removes it again)")
	fmt.Println("  sentinel hook                 What Cursor runs per agent step (reads JSON on stdin)")
	fmt.Println("  sentinel run <agent-command>  Not implemented yet (PTY wrapper)")
	fmt.Println("  sentinel mcp [--ui] [--port N] [--log FILE|-] -- <server-cmd> [args]")
	fmt.Println("                                Wrap an MCP stdio server, recording every JSON-RPC frame")
	fmt.Println("                                and enforcing the policy (block / ask a human / warn):")
	fmt.Println("                                  --policy FILE|off   custom rules (default: built-in)")
	fmt.Println("                                  --max-cost USD      estimated budget, 0 = none (default 2)")
	fmt.Println("                                  --approval-timeout  how long to wait for a human (default 2m)")
	fmt.Println("  sentinel ui [--port N] [FILE]  Replay a recorded session (default: latest) in the cockpit")
	fmt.Println("  sentinel version              Show current version")
	fmt.Println("  sentinel help                 Show this help message")
}
