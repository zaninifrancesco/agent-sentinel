package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

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
}

// Run blocks until the server has exited and all of its output was relayed
// to the client, or ctx is cancelled. It returns the server's exit error.
func (p *StdioProxy) Run(ctx context.Context) error {
	if len(p.Command) == 0 {
		return errors.New("no server command given")
	}
	icpt := NewInterceptor(p.Rec)

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

	// client -> server. When the client hangs up, close the server's stdin so
	// well-behaved servers shut down on their own. This goroutine is not
	// awaited: it may be blocked reading the client's stdin, and it ends by
	// itself on EOF or on the first failed write once the server is gone.
	go func() {
		_ = pump(serverIn, p.In, recorder.ClientToServer, icpt.Observe)
		_ = serverIn.Close()
	}()

	// server -> client. It must be fully drained BEFORE cmd.Wait, which
	// closes the pipe and could otherwise truncate the tail of the output.
	outDone := make(chan struct{})
	go func() {
		defer close(outDone)
		_ = pump(p.Out, serverOut, recorder.ServerToClient, icpt.Observe)
	}()
	<-outDone

	return cmd.Wait()
}
