package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func TestWebSocketSnapshotThenLive(t *testing.T) {
	rec := recorder.New([]string{"fake"}, nil)
	rec.Record(recorder.Event{Type: recorder.EventRequest, Method: "ping", Status: recorder.StatusPending})

	ts := httptest.NewServer(New(rec, nil).Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	_, b, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var snap snapshotMsg
	if err := json.Unmarshal(b, &snap); err != nil || snap.Type != "snapshot" || len(snap.Events) != 2 {
		t.Fatalf("bad snapshot (%v): %s", err, b)
	}

	rec.Record(recorder.Event{Type: recorder.EventNotification, Method: "later"})
	_, b, err = c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ev eventMsg
	if err := json.Unmarshal(b, &ev); err != nil || ev.Type != "event" || ev.Event.Method != "later" || ev.Event.Seq != 3 {
		t.Fatalf("bad live event (%v): %s", err, b)
	}
}

func TestAPIAndSPAFallback(t *testing.T) {
	rec := recorder.New(nil, nil)
	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<html>cockpit</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	ts := httptest.NewServer(New(rec, assets).Handler())
	defer ts.Close()

	get := func(path string) string {
		resp, err := ts.Client().Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var sb strings.Builder
		buf := make([]byte, 512)
		for {
			n, err := resp.Body.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return sb.String()
	}

	if !strings.Contains(get("/api/events"), "SessionStarted") {
		t.Error("/api/events should list the SessionStarted event")
	}
	if !strings.Contains(get("/some/client/route"), "cockpit") {
		t.Error("unknown paths should fall back to index.html")
	}
	if !strings.Contains(get("/assets/app.js"), "console.log") {
		t.Error("static assets should be served")
	}
}
