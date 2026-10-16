package server

import (
	"net/http"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/analysis"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	periods := map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}
	period := q.Get("range")
	if period == "" {
		period = "7d"
	}
	duration, ok := periods[period]
	if !ok {
		http.Error(w, "range must be 24h, 7d or 30d", 400)
		return
	}
	to := time.Now().UTC()
	if raw := q.Get("to"); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "to must be RFC3339", 400)
			return
		}
		to = value.UTC()
	}
	if day := q.Get("day"); day != "" {
		if _, err := time.Parse("2006-01-02", day); err != nil {
			http.Error(w, "day must be YYYY-MM-DD in UTC", 400)
			return
		}
	}
	recordings, warnings := []analysis.Recording{}, []string{}
	if s.Catalog != nil {
		recordings, warnings = s.Catalog.Snapshot()
	}
	snapshot, err := s.profileSnapshot(0, "")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// The in-memory recording is authoritative while its JSONL is being written.
	filtered := recordings[:0]
	for _, item := range recordings {
		if item.Session.ID != snapshot.Session.ID {
			filtered = append(filtered, item)
		}
	}
	recordings = append(filtered, analysis.Recording{Session: snapshot.Session, Revision: snapshot.Revision, Rows: snapshot.Rows, Live: s.Config.Mode != "replay" && snapshot.Session.EndedAt.IsZero()})
	history := analysis.Historical(recordings, analysis.HistoryQuery{From: to.Add(-duration), To: to, Agent: q.Get("agent"), Project: q.Get("project"), Day: q.Get("day")})
	writeJSON(w, struct {
		analysis.History
		Warnings           []string             `json:"warnings"`
		Persistence        recorder.Persistence `json:"persistence"`
		CurrentRecordingID string               `json:"currentRecordingId"`
	}{history, warnings, s.rec.Persistence(), snapshot.Session.ID})
}
