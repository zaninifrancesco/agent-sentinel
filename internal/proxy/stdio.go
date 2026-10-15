package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// StdioProxy launches an MCP server as a child process and relays the MCP
// stdio transport between it and the client (our own stdin/stdout), recording
// every frame.
//
//	client (Claude Code, Cursor...) <-stdin/stdout-> sentinel <-pipes-> MCP server
type StdioProxy struct {
	Command []string  // server command and args
	In      io.Reader // client -> proxy (normally os.Stdin)
	Out     io.Writer // proxy -> client (normally os.Stdout)
	Stderr  io.Writer // server stderr passthrough (normally os.Stderr)
	Rec     *recorder.Recorder

	// Execution boundary (all optional; with none set the proxy only records).
	Policy          *policy.Engine
	Budget          *policy.Budget
	Approvals       *policy.Broker // nil: calls needing approval are refused
	ApprovalTimeout time.Duration  // 0 = DefaultApprovalTimeout
}

// Run blocks until the server has exited and all of its output was relayed
// to the client, or ctx is cancelled. It returns the server's exit error.
func (p *StdioProxy) Run(ctx context.Context) error {
	if len(p.Command) == 0 {
		return errors.New("no server command given")
	}
	cmd := exec.CommandContext(ctx, p.Command[0], p.Command[1:]...)
	cmd.Stderr = p.Stderr
	cmd.WaitDelay = 3 * time.Second
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }

	serverIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	serverOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %q: %w", p.Command[0], err)
	}

	toServer, toClient := &syncWriter{w: serverIn}, &syncWriter{w: p.Out}
	observer := NewInterceptor(p.Rec)
	fromClient := Filter(func(line []byte) []byte {
		observer.Observe(recorder.ClientToServer, line)
		return line
	})
	fromServer := Filter(func(line []byte) []byte {
		observer.Observe(recorder.ServerToClient, line)
		return line
	})
	shutdown := func() {}
	// Kept for legacy internal callers and regression tests. The observation
	// CLI never constructs these dependencies and never enters the gate.
	if p.Policy != nil || p.Budget != nil || p.Approvals != nil {
		gate := newGate(ctx, p, toServer, toClient)
		fromClient, fromServer, shutdown = gate.FromClient, gate.FromServer, gate.Shutdown
	}
	defer shutdown()

	// client -> server. When the client hangs up, close the server's stdin so
	// well-behaved servers shut down on their own. This goroutine is not
	// awaited: it may be blocked reading the client's stdin, and it ends by
	// itself on EOF or on the first failed write once the server is gone.
	go func() {
		_ = pump(toServer, p.In, fromClient)
		shutdown()
		_ = serverIn.Close()
	}()

	// server -> client. It must be fully drained BEFORE cmd.Wait, which
	// closes the pipe and could otherwise truncate the tail of the output.
	outDone := make(chan struct{})
	go func() {
		defer close(outDone)
		_ = pump(toClient, serverOut, fromServer)
	}()
	<-outDone

	return cmd.Wait()
}
