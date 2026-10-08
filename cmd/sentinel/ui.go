package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
	"github.com/zaninifrancesco/agent-sentinel/internal/server"
	"github.com/zaninifrancesco/agent-sentinel/internal/ui"
)

// startCockpit serves the dashboard for rec on 127.0.0.1:port (loopback only:
// the cockpit exposes everything the agent does) and returns its URL.
// The server stops when ctx is cancelled.
//
// With dev=true the Vite dev server origin may also open the WebSocket, so
// `npm run dev` (hot reload) can talk to a real Sentinel.
func startCockpit(ctx context.Context, rec *recorder.Recorder, port int, dev bool, broker *policy.Broker, budget *policy.Budget, cfg server.Config) (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", fmt.Errorf("cannot listen on port %d (use --port): %w", port, err)
	}
	srv := server.New(rec, ui.Assets())
	srv.Approvals = broker
	srv.Budget = budget
	srv.Config = cfg
	if dev {
		srv.AllowedOrigins = server.DevOrigins
	}
	go func() {
		if err := srv.Serve(ctx, ln); err != nil {
			fmt.Fprintf(os.Stderr, "sentinel: cockpit server stopped: %v\n", err)
		}
	}()
	return "http://" + ln.Addr().String(), nil
}

// runUI implements `sentinel ui [--port N] [--open] [session.jsonl]`: it
// replays a recorded session in the cockpit (the latest one by default).
func runUI(args []string) int {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Int("port", 8848, "port for the cockpit")
	open := fs.Bool("open", false, "open the cockpit in the default browser")
	dev := fs.Bool("dev", false, "also accept the Vite dev server (npm run dev) as WebSocket origin")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	path := fs.Arg(0)
	if path == "" {
		var err error
		if path, err = latestSession(); err != nil {
			fmt.Fprintf(os.Stderr, "sentinel ui: %v\n", err)
			return 1
		}
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel ui: %v\n", err)
		return 1
	}
	rec, err := recorder.Load(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel ui: %s: %v\n", path, err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	url, err := startCockpit(ctx, rec, *port, *dev, nil, nil, server.Config{Policy: "replay"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentinel ui: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "sentinel: replaying %s\nsentinel: cockpit at %s (Ctrl-C to quit)\n", path, url)
	if *open {
		openBrowser(url)
	}
	<-ctx.Done()
	return 0
}

func latestSession() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	files, _ := filepath.Glob(filepath.Join(home, ".sentinel", "sessions", "*.jsonl"))
	if len(files) == 0 {
		return "", fmt.Errorf("no recorded sessions found in ~/.sentinel/sessions (run `sentinel mcp` first)")
	}
	sort.Strings(files) // names are timestamps
	return files[len(files)-1], nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
