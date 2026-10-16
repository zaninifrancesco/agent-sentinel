package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/analysis"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

type profilerSnapshot struct {
	Session  recorder.Session
	Revision uint64
	Events   []recorder.Event
	Rows     []analysis.Row
}

// At most the newest projection is cached. Old cursors reconstruct a prefix
// of retained events; neither cache nor browser accumulates payload pages.
func (s *Server) profileSnapshot(through uint64, id string) (*profilerSnapshot, error) {
	s.profilerMu.Lock()
	defer s.profilerMu.Unlock()
	var current *profilerSnapshot
	if id != "" && id != s.rec.Session().ID {
		if s.Catalog == nil {
			return nil, fmt.Errorf("recording not found")
		}
		rec, err := s.Catalog.Open(id)
		if err != nil {
			return nil, err
		}
		session, events, revision := rec.Snapshot()
		current = &profilerSnapshot{session, revision, events, analysis.Project(events)}
	} else {
		if s.profilerCache == nil || s.profilerCache.Revision != s.rec.Revision() {
			session, events, revision := s.rec.Snapshot()
			session.EndedAt = time.Time{}
			for _, e := range events {
				if e.Type == recorder.EventSessionCompleted {
					session.EndedAt = e.Timestamp
				}
			}
			s.profilerCache = &profilerSnapshot{session, revision, events, analysis.Project(events)}
		}
		current = s.profilerCache
	}
	if through == 0 || through == current.Revision {
		return current, nil
	}
	if through > current.Revision {
		return nil, fmt.Errorf("snapshot revision is ahead of this recording")
	}
	events := []recorder.Event{}
	for _, e := range current.Events {
		if e.Seq <= through {
			events = append(events, e)
		}
	}
	session := current.Session
	session.EndedAt = time.Time{}
	for _, e := range events {
		if e.Type == recorder.EventSessionCompleted {
			session.EndedAt = e.Timestamp
		}
	}
	return &profilerSnapshot{session, through, events, analysis.Project(events)}, nil
}

type pageCursor struct {
	RecordingID string `json:"recordingId"`
	Revision    uint64 `json:"revision"`
	Offset      int    `json:"offset"`
	Signature   string `json:"signature"`
}
type profilerPage struct {
	ReadOnly bool                   `json:"readOnly"`
	Version  string                 `json:"version"`
	Revision uint64                 `json:"revision"`
	Session  recorder.Session       `json:"session"`
	Scopes   []analysis.ScopeOption `json:"scopes"`
	Metrics  analysis.Metrics       `json:"metrics"`
	Profile  analysis.Profile       `json:"profile"`
	Timeline analysis.Timeline      `json:"timeline"`
	Rows     []analysis.Row         `json:"rows"`
	Total    int                    `json:"total"`
	Offset   int                    `json:"offset"`
	Limit    int                    `json:"limit"`
	Next     string                 `json:"next,omitempty"`
	Current  string                 `json:"current"`
	Previous string                 `json:"previous,omitempty"`
}

func (s *Server) handleProfiler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 50
	if q.Has("limit") {
		n, err := strconv.Atoi(q.Get("limit"))
		if err != nil || n < 1 || n > 200 {
			http.Error(w, "limit must be between 1 and 200", 400)
			return
		}
		limit = n
	}
	view, filter, order := q.Get("view"), q.Get("filter"), q.Get("sort")
	if view == "" {
		view = "calls"
	}
	if order == "" {
		order = "time"
	}
	if filter == "" {
		filter = "all"
	}
	if view != "calls" && view != "events" || order != "time" && order != "duration" && order != "response" || filter != "all" && filter != "errors" && filter != "open" && filter != "interrupted" && filter != "unresolved" {
		http.Error(w, "invalid profiler query", 400)
		return
	}
	signatureData, _ := json.Marshal([]any{view, filter, order, q.Get("scope"), q.Get("tool"), q.Get("q"), limit})
	hash := sha256.Sum256(signatureData)
	signature := hex.EncodeToString(hash[:])
	var cursor pageCursor
	if raw := q.Get("cursor"); raw != "" {
		b, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil || json.Unmarshal(b, &cursor) != nil || cursor.Offset < 0 || cursor.Revision == 0 || cursor.Signature != signature {
			http.Error(w, "invalid cursor for this query", 400)
			return
		}
	}
	through := cursor.Revision
	if q.Has("through") {
		n, err := strconv.ParseUint(q.Get("through"), 10, 64)
		if err != nil || n == 0 || through != 0 && through != n {
			http.Error(w, "invalid snapshot revision", 400)
			return
		}
		through = n
	}
	id := q.Get("recordingId")
	if id == "" {
		id = cursor.RecordingID
	}
	snapshot, err := s.profileSnapshot(through, id)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if cursor.RecordingID != "" && cursor.RecordingID != snapshot.Session.ID {
		http.Error(w, "recording changed; refresh the profiler", 409)
		return
	}
	if id := q.Get("recordingId"); id != "" && id != snapshot.Session.ID {
		http.Error(w, "recording changed; refresh the profiler", 409)
		return
	}
	scoped := analysis.InScope(snapshot.Rows, q.Get("scope"))
	rows := analysis.Filter(scoped, view, filter, q.Get("tool"), q.Get("q"))
	analysis.Sort(rows, order)
	offset := cursor.Offset
	if focus := q.Get("focus"); focus != "" {
		for i := range rows {
			if rows[i].Key == focus {
				offset = i / limit * limit
				break
			}
		}
	}
	if offset > len(rows) {
		http.Error(w, "cursor offset is outside this snapshot", 400)
		return
	}
	end := min(len(rows), offset+limit)
	page := profilerPage{Version: analysis.Version, Revision: snapshot.Revision, Session: snapshot.Session, Scopes: analysis.Scopes(snapshot.Rows), Metrics: analysis.Measure(scoped), Profile: analysis.ToolProfile(scoped), Timeline: analysis.PageWaterfall(snapshot.Session, scoped, rows[offset:end], q.Get("scope") != ""), Rows: []analysis.Row{}, Total: len(rows), Offset: offset, Limit: limit}
	page.ReadOnly = s.Config.Mode == "replay" || snapshot.Session.ID != s.rec.Session().ID
	for _, row := range rows[offset:end] {
		page.Rows = append(page.Rows, analysis.Summary(row))
	}
	encode := func(n int) string {
		b, _ := json.Marshal(pageCursor{snapshot.Session.ID, snapshot.Revision, n, signature})
		return base64.RawURLEncoding.EncodeToString(b)
	}
	page.Current = encode(offset)
	if end < len(rows) {
		page.Next = encode(end)
	}
	if offset > 0 {
		page.Previous = encode(max(0, offset-limit))
	}
	writeJSON(w, page)
}

func (s *Server) requestedSnapshot(w http.ResponseWriter, r *http.Request) *profilerSnapshot {
	var through uint64
	var err error
	if raw := r.URL.Query().Get("through"); raw != "" {
		through, err = strconv.ParseUint(raw, 10, 64)
		if err != nil || through == 0 {
			http.Error(w, "invalid revision", 400)
			return nil
		}
	}
	snapshot, err := s.profileSnapshot(through, r.URL.Query().Get("recordingId"))
	if err != nil {
		http.Error(w, err.Error(), 409)
		return nil
	}
	if id := r.URL.Query().Get("recordingId"); id != "" && id != snapshot.Session.ID {
		http.Error(w, "recording changed; refresh the profiler", 409)
		return nil
	}
	return snapshot
}
func (s *Server) handleRow(w http.ResponseWriter, r *http.Request) {
	snapshot := s.requestedSnapshot(w, r)
	if snapshot == nil {
		return
	}
	for _, row := range snapshot.Rows {
		if row.Key == r.PathValue("key") {
			writeJSON(w, row)
			return
		}
	}
	http.Error(w, "row not found in this snapshot", 404)
}
func (s *Server) handleReportData(w http.ResponseWriter, r *http.Request) {
	snapshot := s.requestedSnapshot(w, r)
	if snapshot == nil {
		return
	}
	// HTML exports include summaries and evidence metadata, not full bodies.
	rows := make([]analysis.Row, 0, len(snapshot.Rows))
	for _, row := range snapshot.Rows {
		rows = append(rows, analysis.Summary(row))
	}
	writeJSON(w, struct {
		Version  string            `json:"version"`
		Revision uint64            `json:"revision"`
		Session  recorder.Session  `json:"session"`
		Rows     []analysis.Row    `json:"rows"`
		Metrics  analysis.Metrics  `json:"metrics"`
		Timeline analysis.Timeline `json:"timeline"`
	}{analysis.Version, snapshot.Revision, snapshot.Session, rows, analysis.Measure(snapshot.Rows), analysis.Waterfall(snapshot.Session, snapshot.Rows, false)})
}
