package analysis

import (
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func TestHistoricalFiltersPeriodsAndSampleDenominators(t *testing.T) {
	to := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	zero, slow := int64(0), int64(100)
	makeRow := func(status recorder.Status, duration *int64, ago time.Duration, project string) Row {
		return Row{Kind: "tool", Title: "shell", StartedAt: to.Add(-ago), Status: status, DurationMs: duration, Request: &recorder.Event{Source: "cursor-hook", SourceID: "source", AgentSessionID: "native", ProjectRef: project}}
	}
	rows := []Row{
		makeRow(recorder.StatusOK, &zero, time.Hour, "/project"),
		makeRow(recorder.StatusError, &slow, 2*time.Hour, "/project"),
		makeRow(recorder.StatusInterrupted, nil, 3*time.Hour, "/project"),
		makeRow(recorder.StatusPending, nil, 4*time.Hour, "/project"),
		makeRow(recorder.StatusOK, &slow, 25*time.Hour, "/project"),
		makeRow(recorder.StatusOK, nil, 24*time.Hour, "/other"),
		makeRow(recorder.StatusError, nil, 0, "/project"),
	}
	input := []Recording{{Session: recorder.Session{ID: "one"}, Rows: rows}, {Session: recorder.Session{ID: "two"}, Rows: []Row{makeRow(recorder.StatusOK, nil, 5*time.Hour, "/other")}}}
	h := Historical(input, HistoryQuery{From: to.Add(-24 * time.Hour), To: to, Project: "/project"})
	if h.Totals.Calls != 4 || h.Totals.ErrorSamples != 2 || *h.Totals.ErrorRate != .5 || h.Totals.LatencySamples != 2 || *h.Totals.P50 != 0 || *h.Totals.P95 != 100 || h.Totals.ToolDurationMs != 100 {
		t.Fatalf("bad totals: %+v", h.Totals)
	}
	if h.Previous.Calls != 1 || h.RecordingCount != 1 || h.NativeSessions != 1 || len(h.Sessions) != 1 {
		t.Fatalf("bad groups: %+v", h)
	}
	if len(h.AvailableProjects) != 2 {
		t.Fatal("filters removed available options")
	}
	empty := Historical(input, HistoryQuery{From: to.Add(-time.Hour), To: to, Agent: "missing"})
	if empty.Totals.ErrorRate != nil || empty.Totals.P50 != nil || empty.Totals.Calls != 0 {
		t.Fatal("missing samples became measurements")
	}
	// A selected day only narrows the drill-down. Aggregate totals stay stable.
	day := Historical(input, HistoryQuery{From: to.Add(-24 * time.Hour), To: to, Project: "/project", Day: "2026-10-15"})
	if day.Totals.Calls != h.Totals.Calls || len(day.Sessions) != 0 {
		t.Fatal("day changed the period totals")
	}
}

func TestHistoricalKeepsScopesAndUnknownDatesDistinct(t *testing.T) {
	to := time.Now().UTC()
	row := Row{Kind: "tool", Title: "read", Status: recorder.StatusOK, StartedAt: to.Add(-time.Hour), Request: &recorder.Event{Source: "claude-hook", AgentSessionID: "same"}}
	undated := row
	undated.StartedAt = time.Time{}
	h := Historical([]Recording{{Session: recorder.Session{ID: "a"}, Rows: []Row{row, undated}}, {Session: recorder.Session{ID: "b"}, Rows: []Row{row}}}, HistoryQuery{From: to.Add(-24 * time.Hour), To: to})
	if h.NativeSessions != 2 || len(h.Sessions) != 2 || h.UndatedCalls != 1 || h.Projects[0].Name != "unknown" {
		t.Fatalf("merged identities or dates: %+v", h)
	}
}

func TestHistoricalIncludesRecordingsWithoutCalls(t *testing.T) {
	to := time.Now().UTC()
	input := []Recording{{Session: recorder.Session{ID: "empty", StartedAt: to.Add(-time.Hour)}}}
	h := Historical(input, HistoryQuery{From: to.Add(-24 * time.Hour), To: to})
	if len(h.Sessions) != 1 || h.RecordingCount != 1 || h.NativeSessions != 0 || h.Sessions[0].Calls != 0 || h.Sessions[0].ErrorRate != nil {
		t.Fatalf("empty recording missing: %+v", h)
	}
}
