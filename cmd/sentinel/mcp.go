package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/proxy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
	serverpkg "github.com/zaninifrancesco/agent-sentinel/internal/server"
)

// runMCP implements `sentinel mcp [flags] -- <server-cmd> [args...]`.
//
// IMPORTANT: stdout is the MCP protocol channel, so everything Sentinel itself
// wants to say goes to stderr.
func runMCP(args []string) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logPath := fs.String("log", "", `JSONL event log path ("-" = disabled; default ~/.sentinel/sessions/<timestamp>.jsonl)`)
	withUI := fs.Bool("ui", false, "serve the live cockpit while proxying")
	port := fs.Int("port", 8848, "cockpit port (with --ui)")
	open := fs.Bool("open", false, "open the cockpit in the browser (with --ui)")
	dev := fs.Bool("dev", false, "also accept the Vite dev server (npm run dev) as WebSocket origin (with --ui)")
	policyArg := fs.String("policy", "", `policy file (JSON), or "off" to only record; default: built-in rules`)
	maxCost := fs.Float64("max-cost", 2.0, "estimated session budget in USD before a human must confirm (0 = no limit)")
	approvalTimeout := fs.Duration("approval-timeout", proxy.DefaultApprovalTimeout, "how long a held call waits for a human before it is refused")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	server := fs.Args()
	if len(server) == 0 {
		fmt.Fprintln(os.Stderr, "sentinel mcp: missing server command, e.g.\n  sentinel mcp -- npx -y @modelcontextprotocol/server-filesystem .")
		return 2
	}

	engine, budget, policyName, err := buildGuardrails(*policyArg, *maxCost, explicit["max-cost"])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 2
	}

	sink, path, err := openLog(*logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: cannot open log: %v\n", err)
		return 1
	}
	var w io.Writer
	if sink != nil {
		defer sink.Close()
		w = sink
	}

	rec := recorder.New(server, w)
	defer rec.Close()
	if path != "" {
		fmt.Fprintf(os.Stderr, "sentinel: session %s recording to %s\n", rec.Session().ID, path)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Held calls can only be approved from a cockpit. Without --ui there is no
	// broker, so anything needing approval is refused (fail closed).
	var broker *policy.Broker
	if *withUI {
		broker = policy.NewBroker()
		wait := *approvalTimeout
		if wait <= 0 {
			wait = proxy.DefaultApprovalTimeout // the proxy applies the same fallback
		}
		// With a cockpit the budget always exists, even with no limits yet, so
		// a human can set one from the browser. /api/config reports its live
		// limits; the cockpit server owns changing them.
		if budget == nil {
			budget = policy.NewBudget(0, 0)
		}
		cfg := serverpkg.Config{Policy: policyName, ApprovalTimeoutSec: int(wait.Seconds())}
		url, err := startCockpit(ctx, rec, *port, *dev, broker, budget, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "sentinel: cockpit at %s\n", url)
		if *open {
			openBrowser(url)
		}
	}

	p := &proxy.StdioProxy{
		Command: server,
		In:      os.Stdin,
		Out:     os.Stdout,
		Stderr:  os.Stderr,
		Rec:     rec,

		Policy:          engine,
		Budget:          budget,
		Approvals:       broker,
		ApprovalTimeout: *approvalTimeout,
	}
	if err := p.Run(ctx); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 1
	}
	return 0
}

// buildGuardrails turns the CLI flags (and the optional policy file) into the
// policy engine and the budget. An explicit --max-cost beats the file.
func buildGuardrails(policyArg string, maxCost float64, costExplicit bool) (*policy.Engine, *policy.Budget, string, error) {
	var engine *policy.Engine
	var cfg *policy.Config
	name := "default"

	switch policyArg {
	case "off":
		name = "off"
	default:
		if policyArg != "" {
			c, err := policy.LoadFile(policyArg)
			if err != nil {
				return nil, nil, "", err
			}
			cfg, name = c, policyArg
		}
		e, err := policy.NewEngine(cfg)
		if err != nil {
			return nil, nil, "", err
		}
		engine = e
	}

	var maxTokens int64
	if cfg != nil {
		if cfg.MaxCostUSD != nil && !costExplicit {
			maxCost = *cfg.MaxCostUSD
		}
		if cfg.MaxTokens != nil {
			maxTokens = *cfg.MaxTokens
		}
	}
	var budget *policy.Budget
	if maxCost > 0 || maxTokens > 0 {
		budget = policy.NewBudget(maxCost, maxTokens)
	}
	return engine, budget, name, nil
}

func openLog(flagValue string) (*os.File, string, error) {
	if flagValue == "-" {
		return nil, "", nil
	}
	path := flagValue
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", err
		}
		dir := filepath.Join(home, ".sentinel", "sessions")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, "", err
		}
		path = filepath.Join(dir, time.Now().Format("20060102-150405")+".jsonl")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	return f, path, err
}
