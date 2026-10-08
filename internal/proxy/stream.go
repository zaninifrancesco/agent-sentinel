// Package proxy sits between an MCP client and an MCP server, forwarding
// bytes untouched while observing the JSON-RPC frames that flow through.
package proxy

import (
	"bufio"
	"errors"
	"io"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// Observer is called for every line crossing the proxy, BEFORE it is
// forwarded. Observing first guarantees a request is always recorded before
// the server can possibly answer it (otherwise request/response correlation
// would race), and it is the hook where the policy engine will decide on
// the call in Milestone 3. It must be fast: parsing a frame costs
// microseconds. The slice is owned by the observer.
type Observer func(dir recorder.Direction, line []byte)

// pump copies src to dst line by line (the MCP stdio transport is newline
// delimited JSON).
//
// Properties:
//   - bytes are forwarded verbatim, including non-JSON lines and a final line
//     without trailing newline, so the proxy can never corrupt the stream;
//   - there is no maximum line length (bufio.Scanner would fail on big
//     tool results; bufio.Reader.ReadBytes does not);
//   - a write error to dst stops the pump, a clean EOF on src returns nil.
func pump(dst io.Writer, src io.Reader, dir recorder.Direction, obs Observer) error {
	r := bufio.NewReader(src)
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) > 0 {
			if obs != nil {
				obs(dir, line)
			}
			if _, err := dst.Write(line); err != nil {
				return err
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}
