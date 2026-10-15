package recorder

import (
	"encoding/json"
	"sync"
	"time"
)

type trackedCall struct {
	request Event
	started time.Time
}

type callGroup struct {
	calls     []trackedCall
	remaining int
	ambiguous bool
}

// CallTracker serializes request publication and response matching. Repeated
// concurrent identities remain ambiguous until the whole overlap drains;
// consuming one response must not make the next pairing appear exact.
type CallTracker struct {
	mu      sync.Mutex
	rec     *Recorder
	pending map[string]*callGroup
	Now     func() time.Time
}

func NewCallTracker(rec *Recorder) *CallTracker {
	return &CallTracker{rec: rec, pending: make(map[string]*callGroup), Now: time.Now}
}

func matchingKey(e Event, identity string, response bool) string {
	if identity == "" {
		return ""
	}
	origin := e.Direction
	if response {
		if origin == ServerToClient {
			origin = ClientToServer
		} else {
			origin = ServerToClient
		}
	}
	b, _ := json.Marshal([]string{e.SourceID, e.AgentSessionID, e.ConnectionID, string(origin), identity})
	return string(b)
}

func candidateIDs(g *callGroup) []string {
	ids := make([]string, len(g.calls))
	for n, c := range g.calls {
		ids[n] = c.request.CallID
	}
	return ids
}

// Begin records a request. An empty identity deliberately disables matching.
func (t *CallTracker) Begin(e Event, identity, method string) Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	e.CallID = t.rec.NewIdentity("call")
	e.Correlation = &Correlation{Method: method, Quality: "exact"}
	key := matchingKey(e, identity, false)
	if key == "" {
		e.Correlation = &Correlation{Method: "none", Quality: "unmatched"}
	}
	started := t.Now()
	e = t.rec.Record(e)
	if key == "" {
		return e
	}
	g := t.pending[key]
	if g == nil {
		g = &callGroup{}
		t.pending[key] = g
	}
	g.calls = append(g.calls, trackedCall{request: e, started: started})
	g.remaining++
	if len(g.calls) > 1 {
		g.ambiguous = true
		t.diagnostic(e, g, "Overlapping requests share the same correlation identity; responses cannot be assigned safely.")
	}
	return e
}

// End preserves orphan responses and never fabricates a missing request.
// measure is true only for a local proxy round trip with an exact pairing.
func (t *CallTracker) End(e Event, identity, method string, measure bool, finish func(Event) Event) Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := matchingKey(e, identity, true)
	g := t.pending[key]
	e.CallID = t.rec.NewIdentity("call")
	e.Correlation = &Correlation{Method: method, Quality: "unmatched"}
	if key == "" {
		e.Correlation.Method = "none"
	}
	if g != nil {
		g.remaining--
		if g.remaining == 0 {
			delete(t.pending, key)
		}
		if g.ambiguous {
			e.Correlation.Quality = "ambiguous"
			e.Correlation.CandidateCallIDs = candidateIDs(g)
			e.Timing, e.DurationMs = nil, 0
		} else {
			c := g.calls[0]
			e.CallID = c.request.CallID
			e.Correlation.Quality, e.Correlation.RequestSeq = "exact", c.request.Seq
			e.Method, e.ToolName = c.request.Method, c.request.ToolName
			if e.RPCID == "" {
				e.RPCID = c.request.RPCID
			}
			if e.ProjectRef == "" {
				e.ProjectRef = c.request.ProjectRef
			}
			if c.request.Type == EventToolCallRequest {
				e.Type = EventToolCallResponse
			}
			if measure {
				elapsed := t.Now().Sub(c.started).Milliseconds()
				if elapsed >= 0 {
					e.Timing = &Timing{DurationMs: elapsed, Source: "proxy-monotonic"}
					e.DurationMs = elapsed
				}
			}
		}
	}
	if finish != nil {
		e = finish(e)
	}
	return t.rec.Record(e)
}

func (t *CallTracker) diagnostic(e Event, g *callGroup, reason string) {
	e.Seq, e.CallID, e.RPCID, e.NativeCallID, e.Payload = 0, "", "", nil, nil
	e.Type, e.Status, e.Reason = EventCaptureDiagnostic, StatusOK, reason
	e.Capture = nil
	e.Timing, e.DurationMs = nil, 0
	e.Correlation = &Correlation{Method: e.Correlation.Method, Quality: "ambiguous", CandidateCallIDs: candidateIDs(g)}
	t.rec.Record(e)
}

// Interrupt closes only a known agent session in one source namespace.
// Ambiguous groups cannot establish which calls lack a result, so retain a
// diagnostic instead of inventing interruptions for individual requests.
func (t *CallTracker) Interrupt(source, session, reason string) int {
	if session == "" {
		return 0
	}
	session = t.rec.captureIdentity(session)
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for key, g := range t.pending {
		req := g.calls[0].request
		if req.Source != source || req.AgentSessionID != session {
			continue
		}
		delete(t.pending, key)
		if g.ambiguous {
			t.diagnostic(req, g, reason+"; overlapping calls remain unresolved.")
			continue
		}
		e := req
		e.Seq, e.Timestamp, e.Payload = 0, time.Time{}, nil
		e.Type, e.Direction, e.Status, e.Reason = EventToolCallResponse, ServerToClient, StatusInterrupted, reason
		e.Capture = nil
		e.Timing, e.DurationMs = nil, 0
		e.Correlation = &Correlation{Method: req.Correlation.Method, Quality: "exact", RequestSeq: req.Seq, Synthetic: true}
		t.rec.Record(e)
		n++
	}
	return n
}
