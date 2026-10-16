package analysis

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

var epoch = time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

func at(ms int64) time.Time { return epoch.Add(time.Duration(ms) * time.Millisecond) }
func pair(seq uint64, id string, start, end, duration int64) []recorder.Event {
	req := recorder.Event{SchemaVersion: 2, RecordingID: "r", SessionID: "r", Source: "mcp-stdio", SourceID: "s", ConnectionID: "c", CallID: id, Seq: seq, Type: recorder.EventToolCallRequest, Timestamp: at(start), ToolName: "shell", Risk: recorder.RiskNone, Status: recorder.StatusPending, Payload: json.RawMessage(`{"params":{"arguments":{"command":"synthetic only"}}}`), Correlation: &recorder.Correlation{Method: "native", Quality: "exact"}}
	res := req
	res.Seq++
	res.Type = recorder.EventToolCallResponse
	res.Status = recorder.StatusOK
	res.Timestamp = at(end)
	res.Timing = &recorder.Timing{DurationMs: duration, Source: "source-reported"}
	res.Correlation = &recorder.Correlation{Method: "native", Quality: "exact", RequestSeq: seq}
	res.Payload = json.RawMessage(`{"result":"è🙂"}`)
	return []recorder.Event{req, res}
}

func TestUnionConcurrencyAndUnknownCaptureEnds(t *testing.T) {
	events := append(pair(1, "a", 100, 500, 0), pair(3, "b", 200, 700, 40)...)
	events = append(events, pair(5, "c", 900, 1000, 10)...)
	events = append(events, pair(7, "zero", 1000, 1000, 0)...)
	interrupted := pair(9, "stop", 1050, 1100, 99)
	interrupted[1].Correlation.Synthetic = true
	interrupted[1].Status = recorder.StatusInterrupted
	events = append(events, interrupted...)
	pending := pair(11, "pending", 1100, 1200, 20)[0]
	events = append(events, pending)
	rows := Project(events)
	timeline := Waterfall(recorder.Session{StartedAt: epoch, EndedAt: at(1200)}, rows, false)
	if timeline.SpanMs != 1200 || timeline.CoveredMs != 700 || timeline.UnattributedMs != 500 || timeline.OverlapMs != 300 || timeline.PeakConcurrency != 2 || timeline.IncompleteCalls != 2 {
		t.Fatalf("wrong union: %+v", timeline)
	}
	if timeline.GapCount != 3 || timeline.Gaps[0].StartMs != epoch.UnixMilli() || timeline.Gaps[2].EndMs != at(1200).UnixMilli() {
		t.Fatalf("wrong gaps: %+v", timeline.Gaps)
	}
	m := Measure(rows)
	if m.Calls != 6 || m.LatencySamples != 4 || m.P50 != 0 || m.P95 != 40 || m.Interrupted != 1 || m.Running != 1 {
		t.Fatalf("wrong metrics: %+v", m)
	}
	if rows[4].Span.EndMs != nil || rows[4].DurationMs != nil {
		t.Fatal("synthetic closure produced a duration")
	}
	// The same evidence cannot acquire additional time merely by being replayed.
	if !reflect.DeepEqual(timeline, Waterfall(recorder.Session{StartedAt: epoch, EndedAt: at(1200)}, Project(events), false)) {
		t.Fatal("non deterministic analysis")
	}
}

func TestUnknownsOrderingPayloadsAndScopes(t *testing.T) {
	events := append(pair(1, "zero", 0, 0, 0), pair(3, "slow", 10, 30, 20)...)
	events[3].Timestamp = at(5) // invalid capture end, but a separately reported duration
	missing := pair(5, "missing", 40, 50, 2)
	missing[1].Timing = nil
	missing[1].DurationMs = 0
	missing[1].ConnectionID = "another"
	events = append(events, missing...)
	rows := Project(events)
	if rows[1].Span.EndMs != nil {
		t.Fatal("inverted timestamp was accepted")
	}
	if rows[2].Response != nil || rows[3].Request != nil {
		t.Fatal("paired across connections")
	}
	if len(Scopes(rows)) != 2 {
		t.Fatal("connection scopes collapsed")
	}
	if len(InScope(rows, Scope(rows[0]))) != 3 {
		t.Fatal("scope mismatch")
	}
	Sort(rows, "duration")
	if rows[0].Title != "shell" || *rows[0].DurationMs != 20 || *rows[1].DurationMs != 0 {
		t.Fatal("duration sorting lost zero/unknown distinction")
	}
	summary := Summary(rows[0])
	if len(summary.Request.Payload) != 0 || len(summary.Response.Payload) != 0 || summary.ResponseBytes == nil || *summary.ResponseBytes != len(`{"result":"è🙂"}`) {
		t.Fatal("summary leaked body or lost byte size")
	}
	if !reflect.DeepEqual(Filter(rows, "calls", "all", "shell", "synthetic"), Filter(rows, "calls", "all", "shell", "SYNTHETIC")) {
		t.Fatal("search is not case independent")
	}
}

func TestV2FixturePreservesAmbiguousEvidenceAndReplay(t *testing.T) {
	f, err := os.Open("../../testdata/telemetry-v2.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rec, err := recorder.Load(f)
	if err != nil {
		t.Fatal(err)
	}
	rows := Project(rec.Events())
	m := Measure(rows)
	if m.Calls != 7 {
		t.Fatalf("calls: %+v", m)
	}
	unknown, orphan := 0, 0
	for _, r := range rows {
		if r.Kind == "tool" && r.Status == "unknown" {
			unknown++
		}
		if r.Response != nil && r.Request == nil {
			orphan++
		}
	}
	if unknown != 2 || orphan != 3 {
		t.Fatalf("ambiguous requests=%d unassigned responses=%d", unknown, orphan)
	}
	if len(Filter(rows, "calls", "unresolved", "", "")) != 2 {
		t.Fatal("unresolved filtering lost evidence")
	}
	for _, r := range rows {
		if r.Correlation != nil && r.Correlation.Quality == "ambiguous" && r.Span != nil && r.Span.EndMs != nil {
			t.Fatal("ambiguous evidence produced a matched span")
		}
	}
}

func TestGapRenderingIsBoundedButTotalsAreComplete(t *testing.T) {
	events := []recorder.Event{}
	for i := 0; i < 250; i++ {
		events = append(events, pair(uint64(i*2+1), string(rune(i+1)), int64(i*4+1), int64(i*4+2), 1)...)
	}
	timeline := Waterfall(recorder.Session{StartedAt: epoch, EndedAt: at(1000)}, Project(events), false)
	if len(timeline.Gaps) != 200 || timeline.GapCount != 251 || timeline.CoveredMs != 250 || timeline.UnattributedMs != 750 {
		t.Fatalf("bounded gap display hid totals: %+v", timeline)
	}
}
