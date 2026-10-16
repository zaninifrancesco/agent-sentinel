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
	capture := addCaptureFlags(fs)
	payloads := addRecorderFlags(fs)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	server := fs.Args()
	if len(server) == 0 {
		fmt.Fprintln(os.Stderr, "sentinel mcp: missing server command, e.g.\n  sentinel mcp -- npx -y @modelcontextprotocol/server-filesystem .")
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

	rec, err := recorder.NewWithCapture(server, w, settings)
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

	fmt.Fprintln(os.Stderr, "sentinel: observe mode; MCP traffic is recorded without policy or spending limits.")
	if *withUI {
		cfg := serverpkg.Config{Mode: "observe", Policy: "off", RecordingPath: path}
		url, err := startCockpit(ctx, rec, *port, *dev, nil, nil, nil, cfg)
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
