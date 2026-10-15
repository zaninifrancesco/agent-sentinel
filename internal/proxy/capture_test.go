package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func runCaptureEchoServer() {
	in := bufio.NewReader(os.Stdin)
	for {
		line, err := in.ReadBytes('\n')
		var request struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(line, &request) == nil {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]string{"echo": string(line)}})
		}
		if err != nil {
			return
		}
	}
}

func TestProxyRedactionNeverChangesForwardedTraffic(t *testing.T) {
	t.Setenv("SENTINEL_FAKE_SERVER", "3")
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"api_key":"synthetic-wire-secret","body":"` + strings.Repeat("x", 1500) + `"}}}` + "\n"
	settings, _ := recorder.CaptureOptions("standard", 256)
	var log, out bytes.Buffer
	rec, _ := recorder.NewWithCapture(nil, &log, settings)
	p := &StdioProxy{Command: []string{os.Args[0]}, In: strings.NewReader(input), Out: &out, Stderr: io.Discard, Rec: rec}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Run(ctx); err != nil {
		t.Fatal(err)
	}
	// The fake MCP server echoes the exact request into its result. A clean
	// recorder must not imply that its sanitized copy was sent to the server.
	if !bytes.Contains(out.Bytes(), []byte("synthetic-wire-secret")) || !bytes.Contains(out.Bytes(), []byte(strings.Repeat("x", 1500))) {
		t.Fatalf("wire content changed: %s", out.Bytes())
	}
	var result struct {
		Result struct {
			Echo string `json:"echo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Result.Echo != input {
		t.Fatal("request bytes changed before reaching the server")
	}
	if bytes.Contains(log.Bytes(), []byte("synthetic-wire-secret")) {
		t.Fatal("wire copy bypassed recorder capture profile")
	}
	for _, e := range rec.Events() {
		if e.Source == "mcp-stdio" && (e.Capture == nil || e.Capture.ObservedBytes == nil || e.Capture.StoredBytes > 256) {
			t.Fatalf("missing bounded capture metadata: %+v", e.Capture)
		}
	}
}
