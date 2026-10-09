package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/cursorhooks"
	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/proxy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
	serverpkg "github.com/zaninifrancesco/agent-sentinel/internal/server"
)

// runServe implements `sentinel serve`: a long-running Sentinel with the
// cockpit, the policy, the budget and the approvals, and no MCP proxy. Cursor's
// hooks (`sentinel hook`) report to it and wait for its verdict.
func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logPath := fs.String("log", "", `JSONL event log path ("-" = disabled; default ~/.sentinel/sessions/<timestamp>.jsonl)`)
	port := fs.Int("port", defaultServePort, "cockpit and hook port")
	open := fs.Bool("open", false, "open the cockpit in the browser")
	dev := fs.Bool("dev", false, "also accept the Vite dev server (npm run dev) as WebSocket origin")
	policyArg := fs.String("policy", "", `policy file (JSON), or "off" to only record; default: built-in rules`)
	maxCost := fs.Float64("max-cost", 2.0, "estimated session budget in USD before a human must confirm (0 = no limit)")
	approvalTimeout := fs.Duration("approval-timeout", proxy.DefaultApprovalTimeout, "how long a held call waits for a human before it is refused")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "sentinel serve takes no arguments; it waits for `sentinel hook` (see `sentinel hook install`)")
		return 2
	}
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	engine, budget, policyName, err := buildGuardrails(*policyArg, *maxCost, explicit["max-cost"])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 2
	}
	if budget == nil {
		budget = policy.NewBudget(0, 0) // so a limit can be set from the cockpit
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
	rec := recorder.New([]string{"cursor hooks"}, w)
	defer rec.Close()
	if path != "" {
		fmt.Fprintf(os.Stderr, "sentinel: session %s recording to %s\n", rec.Session().ID, path)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	wait := *approvalTimeout
	if wait <= 0 {
		wait = cursorhooks.DefaultTimeout
	}
	broker := policy.NewBroker()
	hooks := cursorhooks.New(rec, engine, budget, broker, wait)
	hooks.Logf = func(format string, args ...any) { fmt.Fprintf(os.Stderr, "sentinel: "+format+"\n", args...) }
	cfg := serverpkg.Config{Policy: policyName, ApprovalTimeoutSec: int(wait.Seconds())}
	url, err := startCockpit(ctx, rec, *port, *dev, broker, budget, hooks, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "sentinel: cockpit at %s, waiting for Cursor hooks (Ctrl-C to quit)\n", url)
	fmt.Fprintf(os.Stderr, "sentinel: policy %s; held calls wait %s for you. Not set up yet? Run `sentinel hook install`.\n", policyName, wait.Round(time.Second))
	if *open {
		openBrowser(url)
	}

	<-ctx.Done()
	return 0
}
