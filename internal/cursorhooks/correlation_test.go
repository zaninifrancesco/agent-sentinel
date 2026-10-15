package cursorhooks

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func captureHook(t *testing.T, h *Handler, input string) {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/hook", strings.NewReader(input)))
	if rr.Code != 200 {
		t.Fatalf("hook: %d %s", rr.Code, rr.Body.String())
	}
}

func TestObserverSeparatesNativeSessionsAndSources(t *testing.T) {
	rec := recorder.New(nil, nil)
	h := NewObserver(rec)
	for _, session := range []string{"a", "b"} {
		captureHook(t, h, `{"hook_event_name":"PreToolUse","session_id":"`+session+`","tool_name":"Bash","tool_use_id":"same","tool_input":{"command":"synthetic"}}`)
	}
	captureHook(t, h, `{"hook_event_name":"beforeShellExecution","conversation_id":"a","command":"synthetic"}`)
	captureHook(t, h, `{"hook_event_name":"stop","conversation_id":"a"}`)
	for _, session := range []string{"b", "a"} {
		captureHook(t, h, `{"hook_event_name":"PostToolUse","session_id":"`+session+`","tool_name":"Bash","tool_use_id":"same","tool_input":{"command":"synthetic"},"duration_ms":0}`)
	}
	events := rec.Events()
	if len(events) != 7 {
		t.Fatalf("unexpected events: %+v", events)
	}
	if events[4].Source != "cursor-hook" || !events[4].Correlation.Synthetic || events[4].Timing != nil {
		t.Fatalf("incorrect interruption: %+v", events[4])
	}
	for n, requestIndex := range []int{2, 1} {
		response := events[5+n]
		request := events[requestIndex]
		if response.CallID != request.CallID || response.Correlation.RequestSeq != request.Seq || response.AgentSessionID != request.AgentSessionID || response.Timing == nil || response.Timing.DurationMs != 0 {
			t.Fatalf("cross-session or missing-zero pairing: %+v", response)
		}
	}
}

func TestObserverDoesNotGuessConcurrentFallbackPairs(t *testing.T) {
	rec := recorder.New(nil, nil)
	h := NewObserver(rec)
	before := `{"hook_event_name":"beforeShellExecution","conversation_id":"a","command":"synthetic"}`
	after := `{"hook_event_name":"afterShellExecution","conversation_id":"a","command":"synthetic","duration":9}`
	captureHook(t, h, before)
	captureHook(t, h, before)
	captureHook(t, h, after)
	captureHook(t, h, after)
	var responses []recorder.Event
	for _, e := range rec.Events() {
		if e.Type == recorder.EventToolCallResponse {
			responses = append(responses, e)
		}
	}
	if len(responses) != 2 {
		t.Fatal("responses missing")
	}
	for _, e := range responses {
		if e.Correlation.Quality != "ambiguous" || len(e.Correlation.CandidateCallIDs) != 2 || e.Correlation.RequestSeq != 0 || e.Timing != nil || e.RPCID != "" {
			t.Fatalf("guessed pairing: %+v", e)
		}
	}
	// Once the overlap drains, a new isolated call can be paired normally.
	captureHook(t, h, before)
	captureHook(t, h, after)
	last := rec.Events()[len(rec.Events())-1]
	if last.Correlation.Quality != "exact" {
		t.Fatalf("overlap never drained: %+v", last)
	}
}

func TestObserverCanonicalArgumentsAndMCPNamespaces(t *testing.T) {
	rec := recorder.New(nil, nil)
	h := NewObserver(rec)
	for _, server := range []string{"one", "two"} {
		captureHook(t, h, `{"hook_event_name":"beforeMCPExecution","conversation_id":"a","mcp_server_name":"`+server+`","tool_name":"lookup","tool_input":{"b":2,"a":9007199254740993}}`)
	}
	captureHook(t, h, `{"hook_event_name":"afterMCPExecution","conversation_id":"a","mcp_server_name":"two","tool_name":"lookup","tool_input":"{\"a\":9007199254740993,\"b\":2}","result_json":"{}"}`)
	e := rec.Events()[3]
	if e.CallID != rec.Events()[2].CallID || e.Correlation.Method != "fallback" || e.Timing != nil {
		t.Fatalf("incorrect canonical pair: %+v", e)
	}
	// An after-only observation stays a response; it does not invent a request.
	captureHook(t, h, `{"hook_event_name":"PostToolUse","session_id":"missing","tool_name":"Bash","tool_use_id":"orphan","duration_ms":-1}`)
	e = rec.Events()[4]
	if e.Correlation.Quality != "unmatched" || e.Correlation.RequestSeq != 0 || e.Timing != nil || len(rec.Events()) != 5 {
		t.Fatalf("fabricated request or time: %+v", e)
	}
}

func TestObserverConcurrentNativeCallsPublishRequestFirst(t *testing.T) {
	rec := recorder.New(nil, nil)
	h := NewObserver(rec)
	var wg sync.WaitGroup
	for n := 0; n < 40; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			body := map[string]any{"hook_event_name": "PreToolUse", "session_id": "parallel", "tool_name": "Bash", "tool_use_id": nID(n), "tool_input": map[string]string{"command": "synthetic"}}
			b, _ := json.Marshal(body)
			captureHook(t, h, string(b))
			body["hook_event_name"] = "PostToolUse"
			b, _ = json.Marshal(body)
			captureHook(t, h, string(b))
		}(n)
	}
	wg.Wait()
	requests := map[string]uint64{}
	for _, e := range rec.Events() {
		if e.Type == recorder.EventToolCallRequest {
			requests[e.CallID] = e.Seq
		}
		if e.Type == recorder.EventToolCallResponse && (e.Correlation.Quality != "exact" || requests[e.CallID] != e.Correlation.RequestSeq) {
			t.Fatalf("response published before request: %+v", e)
		}
	}
	if len(requests) != 40 {
		t.Fatalf("lost calls: %d", len(requests))
	}
}

func nID(n int) string { b, _ := json.Marshal(n); return "call-" + string(b) }
