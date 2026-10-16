package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/zaninifrancesco/agent-sentinel/internal/cursorhooks"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
	serverpkg "github.com/zaninifrancesco/agent-sentinel/internal/server"
)

// runServe implements `sentinel serve`: a long-running Sentinel with the
// cockpit and passive hook recorder, without an MCP proxy or permission gate.
func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logPath := fs.String("log", "", `JSONL event log path ("-" = disabled; default ~/.sentinel/sessions/<timestamp>.jsonl)`)
	port := fs.Int("port", defaultServePort, "cockpit and hook port")
	open := fs.Bool("open", false, "open the cockpit in the browser")
	dev := fs.Bool("dev", false, "also accept the Vite dev server (npm run dev) as WebSocket origin")
	capture := addCaptureFlags(fs)
	payloads := addRecorderFlags(fs)
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
	if err := capture.validate(fs); err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 2
	}
	settings, err := payloads.settings()
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
	rec, err := recorder.NewWithCapture([]string{"agent hooks"}, w, settings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 2
	}
	defer rec.Close()
	if path != "" {
		fmt.Fprintf(os.Stderr, "sentinel: session %s recording to %s\n", rec.Session().ID, path)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hooks := cursorhooks.NewObserver(rec)
	hooks.Logf = func(format string, args ...any) { fmt.Fprintf(os.Stderr, "sentinel: "+format+"\n", args...) }
	cfg := serverpkg.Config{Mode: "observe", Policy: "off", RecordingPath: path}
	url, err := startCockpit(ctx, rec, *port, *dev, nil, nil, hooks, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "sentinel: cockpit at %s, recording agent hooks (Ctrl-C to quit)\n", url)
	fmt.Fprintln(os.Stderr, "sentinel: observe mode; commands are never blocked or held. Your agent's own permissions still apply.")
	if *open {
		openBrowser(url)
	}

	<-ctx.Done()
	return 0
}
