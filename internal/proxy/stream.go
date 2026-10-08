// Package proxy sits between an MCP client and an MCP server, forwarding
// bytes untouched while observing the JSON-RPC frames that flow through.
package proxy

import (
	"bufio"
	"errors"
	"io"
)

// Filter sees every line crossing the proxy BEFORE it is forwarded and
// returns the bytes to forward: the line itself (usually), a modified copy, or
// nil to swallow it. Filtering first guarantees a request is always recorded
// before the server can possibly answer it (otherwise request/response
// correlation would race), and it is where the policy decides on a call. It
// must be fast: parsing a frame costs microseconds.
type Filter func(line []byte) []byte

// pump copies src to dst line by line (the MCP stdio transport is newline
// delimited JSON).
//
// Properties:
//   - bytes are forwarded verbatim unless the filter says otherwise, including
//     non-JSON lines and a final line without trailing newline, so the proxy
//     does not corrupt the stream by itself;
//   - there is no maximum line length (bufio.Scanner would fail on big
//     tool results; bufio.Reader.ReadBytes does not);
//   - a write error to dst stops the pump, a clean EOF on src returns nil.
func pump(dst io.Writer, src io.Reader, filter Filter) error {
	r := bufio.NewReader(src)
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) > 0 {
			if filter != nil {
				line = filter(line)
			}
			if len(line) > 0 {
				if _, err := dst.Write(line); err != nil {
					return err
				}
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
