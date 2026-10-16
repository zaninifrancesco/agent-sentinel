package server

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/analysis"
	"github.com/zaninifrancesco/agent-sentinel/internal/catalog"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func TestAnalyticsOverlayAndHistoricalDrilldown(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	old := recorder.New(nil, &log)
	addProfileCall(old, 10)
	old.Close()
	if err := os.WriteFile(filepath.Join(dir, "old.jsonl"), log.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	live := recorder.New(nil, nil)
	addProfileCall(live, 20)
	s := New(live, nil)
	s.Catalog = catalog.New(dir, "")
	s.Config.Mode = "observe"
	to := url.QueryEscape(time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano))
	w := profilerGET(s, "/api/v1/analytics?to="+to)
	var history analysis.History
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || history.Totals.Calls != 2 || history.RecordingCount != 2 || history.NativeSessions != 2 {
		t.Fatalf("bad analytics: %s", w.Body)
	}
	query := "recordingId=" + url.QueryEscape(old.Session().ID)
	page := decodeProfile(t, profilerGET(s, "/api/v1/profiler?"+query))
	if !page.ReadOnly || page.Session.ID != old.Session().ID || page.Metrics.Calls != 1 {
		t.Fatal("archive did not open")
	}
	row := profilerGET(s, "/api/v1/rows/"+url.PathEscape(page.Rows[0].Key)+"?"+query)
	if row.Code != 200 {
		t.Fatalf("archive payload missing: %s", row.Body)
	}
	if report := profilerGET(s, "/api/v1/report?"+query); report.Code != 200 {
		t.Fatalf("archive report missing: %s", report.Body)
	}
	if s.rec.Session().ID != live.Session().ID {
		t.Fatal("browsing changed the live collector")
	}
	for _, path := range []string{"/api/v1/analytics?range=bad", "/api/v1/analytics?to=bad", "/api/v1/analytics?day=bad", "/api/v1/profiler?recordingId=../../outside"} {
		if profilerGET(s, path).Code == 200 {
			t.Fatalf("invalid request accepted: %s", path)
		}
	}
}
