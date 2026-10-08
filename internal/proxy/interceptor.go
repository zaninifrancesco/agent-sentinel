package proxy

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/protocol"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// pendingCall remembers an in-flight request so that when the response comes
// back we can compute latency and tag it with the method / tool name.
type pendingCall struct {
	method   string
	toolName string
	started  time.Time
}

// Interceptor turns raw wire lines into recorder events and correlates
// requests with their responses by JSON-RPC id.
type Interceptor struct {
	rec     *recorder.Recorder
	mu      sync.Mutex
	pending map[string]pendingCall
	now     func() time.Time
}

func NewInterceptor(rec *recorder.Recorder) *Interceptor {
	return &Interceptor{
		rec:     rec,
		pending: make(map[string]pendingCall),
		now:     time.Now,
	}
}

// Observe is the proxy Observer: it classifies the line and records it.
func (i *Interceptor) Observe(dir recorder.Direction, line []byte) {
	frame := bytes.TrimRight(line, "\r\n")
	if len(bytes.TrimSpace(frame)) == 0 {
		return
	}

	msg, err := protocol.Parse(frame)
	if err != nil {
		// Free text (logs, banners...) mixed into the stream: record it as
		// raw output, never drop it silently.
		payload, _ := json.Marshal(string(frame))
		i.rec.Record(recorder.Event{
			Type:      recorder.EventRawOutput,
			Direction: dir,
			Status:    recorder.StatusOK,
			Payload:   payload,
		})
		return
	}

	switch msg.Kind() {
	case protocol.KindRequest:
		i.onRequest(dir, msg, frame)
	case protocol.KindNotification:
		i.rec.Record(recorder.Event{
			Type:      recorder.EventNotification,
			Direction: dir,
			Method:    msg.Method,
			Status:    recorder.StatusOK,
			Payload:   json.RawMessage(frame),
		})
	case protocol.KindResponse, protocol.KindError:
		i.onResponse(dir, msg, frame)
	}
}

func (i *Interceptor) onRequest(dir recorder.Direction, msg *protocol.Message, frame []byte) {
	ev := recorder.Event{
		Type:      recorder.EventRequest,
		Direction: dir,
		Method:    msg.Method,
		RPCID:     msg.IDString(),
		Status:    recorder.StatusPending,
		Payload:   json.RawMessage(frame),
	}
	call := pendingCall{method: msg.Method, started: i.now()}

	if msg.Method == protocol.MethodToolsCall {
		ev.Type = recorder.EventToolCallRequest
		if p, err := msg.AsCallTool(); err == nil {
			ev.ToolName = p.Name
			call.toolName = p.Name
		}
	}

	i.mu.Lock()
	i.pending[ev.RPCID] = call
	i.mu.Unlock()

	i.rec.Record(ev)
}

func (i *Interceptor) onResponse(dir recorder.Direction, msg *protocol.Message, frame []byte) {
	id := msg.IDString()

	i.mu.Lock()
	call, found := i.pending[id]
	delete(i.pending, id)
	i.mu.Unlock()

	ev := recorder.Event{
		Type:      recorder.EventResponse,
		Direction: dir,
		RPCID:     id,
		Status:    recorder.StatusOK,
		Payload:   json.RawMessage(frame),
	}
	if found {
		ev.Method = call.method
		ev.ToolName = call.toolName
		ev.DurationMs = i.now().Sub(call.started).Milliseconds()
		if call.method == protocol.MethodToolsCall {
			ev.Type = recorder.EventToolCallResponse
		}
	}

	if msg.Kind() == protocol.KindError {
		ev.Status = recorder.StatusError
	} else if call.method == protocol.MethodToolsCall {
		// MCP reports tool failures in-band with isError=true.
		if res, err := msg.AsCallToolResult(); err == nil && res.IsError {
			ev.Status = recorder.StatusError
		}
	}

	i.rec.Record(ev)
}
