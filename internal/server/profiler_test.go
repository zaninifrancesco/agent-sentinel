package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/analysis"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func addProfileCall(rec *recorder.Recorder, n int) {
	id := fmt.Sprintf("call-%d", n)
	source := rec.Session().ID + "/source"
	req := rec.Record(recorder.Event{Type: recorder.EventToolCallRequest, Source: "cursor-hook", SourceID: source, AgentSessionID: "mock", CallID: id, ToolName: "shell", Status: recorder.StatusPending, Payload: json.RawMessage(fmt.Sprintf(`{"params":{"arguments":{"command":"synthetic-%d","tail":"full-argument-search"}}}`, n)), Correlation: &recorder.Correlation{Method: "native", Quality: "exact"}})
	rec.Record(recorder.Event{Type: recorder.EventToolCallResponse, Source: "cursor-hook", SourceID: source, AgentSessionID: "mock", CallID: id, ToolName: "shell", Status: recorder.StatusOK, Payload: json.RawMessage(`{"result":{"text":"retained body"}}`), Timing: &recorder.Timing{DurationMs: int64(n), Source: "source-reported"}, Correlation: &recorder.Correlation{Method: "native", Quality: "exact", RequestSeq: req.Seq}})
}
func profilerGET(s *Server, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1"+path, nil))
	return w
}
func decodeProfile(t *testing.T, w *httptest.ResponseRecorder) profilerPage {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body)
	}
	var page profilerPage
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestProfilerPagesPinSnapshotAndLoadBodiesOnlyOnDemand(t *testing.T) {
	rec := recorder.New(nil, nil)
	for i := 0; i < 5; i++ {
		addProfileCall(rec, i)
	}
	s := New(rec, nil)
	first := decodeProfile(t, profilerGET(s, "/api/v1/profiler?limit=2"))
	if strings.Contains(profilerGET(s, "/api/v1/profiler").Body.String(), `"endedAt"`) {
		t.Fatal("live snapshot exposed a zero end date as a finished session")
	}
	if first.Total != 5 || len(first.Rows) != 2 || first.Next == "" || first.Current == "" || first.Metrics.Calls != 5 {
		t.Fatalf("bad first page: %+v", first)
	}
	if first.Rows[0].Request.Payload != nil || first.Rows[0].Response.Payload != nil {
		t.Fatal("page included payloads")
	}
	rowPath := "/api/v1/rows/" + url.PathEscape(first.Rows[0].Key) + "?through=" + fmt.Sprint(first.Revision)
	w := profilerGET(s, rowPath)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "retained body") {
		t.Fatalf("detail missing: %s", w.Body)
	}
	addProfileCall(rec, 99)
	second := decodeProfile(t, profilerGET(s, "/api/v1/profiler?limit=2&cursor="+url.QueryEscape(first.Next)))
	if second.Revision != first.Revision || second.Total != 5 || second.Rows[0].Key == first.Rows[0].Key || second.Offset != 2 {
		t.Fatal("live append shifted a pinned page")
	}
	fresh := decodeProfile(t, profilerGET(s, "/api/v1/profiler?limit=2"))
	if fresh.Total != 6 || fresh.Revision <= first.Revision {
		t.Fatal("latest snapshot did not refresh")
	}
	if profilerGET(s, "/api/v1/profiler?limit=2&q=changed&cursor="+url.QueryEscape(first.Next)).Code != 400 {
		t.Fatal("cursor was accepted with a different query")
	}
	for _, path := range []string{"/api/v1/profiler?limit=0", "/api/v1/profiler?limit=201", "/api/v1/profiler?sort=bogus", "/api/v1/profiler?through=9999", "/api/v1/profiler?recordingId=wrong", "/api/v1/rows/missing"} {
		if profilerGET(s, path).Code == 200 {
			t.Fatalf("accepted invalid query: %s", path)
		}
	}
	filtered := decodeProfile(t, profilerGET(s, "/api/v1/profiler?limit=2&q=full-argument-search"))
	if filtered.Total != 6 {
		t.Fatal("search only used argument summaries")
	}
	focus := decodeProfile(t, profilerGET(s, "/api/v1/profiler?limit=2&focus="+url.QueryEscape(fresh.Profile.Longest[0].Key)))
	if focus.Offset != 4 {
		t.Fatal("ranking selection did not locate its page")
	}
	var report struct {
		Metrics  analysis.Metrics
		Revision uint64
		Rows     []analysis.Row
		Timeline analysis.Timeline
	}
	w = profilerGET(s, "/api/v1/report?through="+fmt.Sprint(first.Revision))
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Metrics != first.Metrics || report.Revision != first.Revision || report.Timeline.CoveredMs != first.Timeline.CoveredMs {
		t.Fatal("export and profiler used different metrics")
	}
	if strings.Contains(w.Body.String(), "retained body") {
		t.Fatal("HTML report metadata leaked retained bodies")
	}
	rec.Close()
	closed := decodeProfile(t, profilerGET(s, "/api/v1/profiler"))
	if closed.Session.EndedAt.IsZero() {
		t.Fatal("completed recording lost its end")
	}
	pinned := decodeProfile(t, profilerGET(s, "/api/v1/profiler?through="+fmt.Sprint(first.Revision)))
	if !pinned.Session.EndedAt.IsZero() {
		t.Fatal("completion leaked into an earlier snapshot")
	}
}

func TestTenThousandCallsArePagedAndReplayKeepsAnalysis(t *testing.T) {
	var log strings.Builder
	rec := recorder.New(nil, &log)
	for i := 0; i < 10000; i++ {
		addProfileCall(rec, i)
	}
	rec.Close()
	loaded, err := recorder.Load(strings.NewReader(log.String()))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	live := decodeProfile(t, profilerGET(New(rec, nil), "/api/v1/profiler?limit=50"))
	replay := decodeProfile(t, profilerGET(New(loaded, nil), "/api/v1/profiler?limit=50"))
	if live.Total != 10000 || len(live.Rows) != 50 || live.Metrics != replay.Metrics || live.Timeline.CoveredMs != replay.Timeline.CoveredMs || live.Revision != replay.Revision {
		t.Fatal("10k pagination/replay mismatch")
	}
	t.Logf("10k live and replay projections: %s; list page contains %d summaries", time.Since(start), len(live.Rows))
}
