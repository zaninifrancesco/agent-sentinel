package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// TestMain doubles as a fake MCP server: when re-executed with
// SENTINEL_FAKE_SERVER=1 the test binary speaks just enough MCP over stdio.
func TestMain(m *testing.M) {
	switch os.Getenv("SENTINEL_FAKE_SERVER") {
	case "1":
		runFakeServer()
		return
	case "2":
		runEchoServer()
		return
	}
	os.Exit(m.Run())
}

func runFakeServer() {
	fmt.Println("fake-server booting (not json)")
	r := bufio.NewReader(os.Stdin)
	for {
		line, err := r.ReadString('\n')
		if line = strings.TrimSpace(line); line != "" {
			switch {
			case strings.Contains(line, `"method":"initialize"`):
				fmt.Println(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`)
			case strings.Contains(line, `"name":"boom"`):
				fmt.Println(`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"bad"}],"isError":true}}`)
			case strings.Contains(line, `"method":"tools/call"`):
				fmt.Println(`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"ok"}]}}`)
			case strings.Contains(line, `"method":"nope"`):
				fmt.Println(`{"jsonrpc":"2.0","id":4,"error":{"code":-32601,"message":"no"}}`)
			}
		}
		if err != nil {
			return
		}
	}
}

func TestPumpForwardsVerbatim(t *testing.T) {
	input := "{\"a\":1}\nplain text\n\n" + strings.Repeat("x", 200_000) + "\nlast-without-newline"
	var out bytes.Buffer
	var seen int
	err := pump(&out, strings.NewReader(input), func(l []byte) []byte { seen++; return l })
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != input {
		t.Fatalf("output differs from input (len %d vs %d)", out.Len(), len(input))
	}
	if seen != 5 {
		t.Fatalf("observer saw %d lines, want 5", seen)
	}
}

func TestInterceptorCorrelatesAndFlagsErrors(t *testing.T) {
	rec := recorder.New(nil, nil)
	i := NewInterceptor(rec)

	i.Observe(recorder.ClientToServer, []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"boom"}}`+"\n"))
	time.Sleep(5 * time.Millisecond)
	i.Observe(recorder.ServerToClient, []byte(`{"jsonrpc":"2.0","id":2,"result":{"content":[],"isError":true}}`+"\n"))

	evs := rec.Events()
	req, resp := evs[1], evs[2] // evs[0] is SessionStarted
	if req.Type != recorder.EventToolCallRequest || req.ToolName != "boom" || req.Status != recorder.StatusPending {
		t.Fatalf("bad request event: %+v", req)
	}
	if resp.Type != recorder.EventToolCallResponse || resp.ToolName != "boom" || resp.Status != recorder.StatusError {
		t.Fatalf("bad response event: %+v", resp)
	}
	if resp.DurationMs < 1 {
		t.Fatalf("expected latency to be measured, got %dms", resp.DurationMs)
	}
}

func TestStdioProxyEndToEnd(t *testing.T) {
	t.Setenv("SENTINEL_FAKE_SERVER", "1")

	clientIn, clientW := io.Pipe()
	var clientOut bytes.Buffer
	rec := recorder.New([]string{"fake"}, nil)

	p := &StdioProxy{
		Command: []string{os.Args[0]},
		In:      clientIn,
		Out:     &clientOut,
		Stderr:  io.Discard,
		Rec:     rec,
	}

	go func() {
		defer clientW.Close()
		io.WriteString(clientW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n")
		io.WriteString(clientW, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
		io.WriteString(clientW, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"boom"}}`+"\n")
		io.WriteString(clientW, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"a.txt"}}}`+"\n")
		io.WriteString(clientW, `{"jsonrpc":"2.0","id":4,"method":"nope"}`+"\n")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := p.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 1. The client must receive everything the server wrote, untouched.
	wantOut := "fake-server booting (not json)\n" +
		`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"bad"}],"isError":true}}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"ok"}]}}` + "\n" +
		`{"jsonrpc":"2.0","id":4,"error":{"code":-32601,"message":"no"}}` + "\n"
	if clientOut.String() != wantOut {
		t.Fatalf("client output mismatch:\n got: %q\nwant: %q", clientOut.String(), wantOut)
	}

	// 2. The recorder must have seen it all.
	counts := map[recorder.EventType]int{}
	var tools []string
	var errs int
	for _, e := range rec.Events() {
		counts[e.Type]++
		if e.Type == recorder.EventToolCallRequest {
			tools = append(tools, e.ToolName)
		}
		if e.Status == recorder.StatusError {
			errs++
		}
	}
	if counts[recorder.EventToolCallRequest] != 2 || counts[recorder.EventToolCallResponse] != 2 {
		t.Errorf("tool call events: %v", counts)
	}
	if counts[recorder.EventNotification] != 1 || counts[recorder.EventRawOutput] != 1 {
		t.Errorf("notification/raw events: %v", counts)
	}
	if strings.Join(tools, ",") != "boom,read_file" {
		t.Errorf("tools = %v", tools)
	}
	if errs != 2 { // isError tool result + JSON-RPC error for "nope"
		t.Errorf("error events = %d, want 2", errs)
	}
}
