package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	Version = "0.1.0-alpha"
	Banner  = `
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
	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		runCmd.Parse(os.Args[2:])
		targetArgs := runCmd.Args()
		if len(targetArgs) == 0 {
			fmt.Println("Error: specify a command to supervise (e.g., sentinel run claude)")
			os.Exit(1)
		}
		fmt.Printf("Supervising command: %v\n", targetArgs)
		// Milestone 1: PTY runner will be attached here
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
	fmt.Println("  sentinel run <agent-command>  Supervise an AI coding agent process (e.g. sentinel run claude)")
	fmt.Println("  sentinel mcp [--log FILE|-] -- <server-cmd> [args]")
	fmt.Println("                                Wrap an MCP stdio server, recording every JSON-RPC frame")
	fmt.Println("  sentinel ui                   Launch the Sentinel Cockpit web dashboard")
	fmt.Println("  sentinel version              Show current version")
	fmt.Println("  sentinel help                 Show this help message")
}
