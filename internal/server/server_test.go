package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
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

func TestWebSocketOriginCheck(t *testing.T) {
	dial := func(allowed []string, origin string) error {
		srv := New(recorder.New(nil, nil), nil)
		srv.AllowedOrigins = allowed
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws", &websocket.DialOptions{
			HTTPHeader: map[string][]string{"Origin": {origin}},
		})
		if err == nil {
			c.CloseNow()
		}
		return err
	}

	if err := dial(nil, "http://evil.example"); err == nil {
		t.Error("a foreign origin must be rejected by default")
	}
	if err := dial(nil, "http://localhost:5173"); err == nil {
		t.Error("the Vite origin must be rejected unless --dev is set")
	}
	if err := dial(DevOrigins, "http://localhost:5173"); err != nil {
		t.Errorf("the Vite origin must be accepted with --dev: %v", err)
	}
	if err := dial(DevOrigins, "http://evil.example"); err == nil {
		t.Error("--dev must not open the door to other origins")
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

func TestApprovalEndpointDefences(t *testing.T) {
	rec := recorder.New(nil, nil)
	b := policy.NewBroker()
	srv := New(rec, nil)
	srv.Approvals = b
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	post := func(seq, body, contentType, origin, host string) int {
		req, _ := http.NewRequest("POST", ts.URL+"/api/approvals/"+seq, strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	pending := func() bool { return len(b.Pending()) == 1 }

	b.Open(5)
	own := ts.URL // same origin as the server
	body := `{"approved":true,"feedback":"ok"}`

	for name, code := range map[string]int{
		"foreign origin":  post("5", body, "application/json", "https://evil.example", ""),
		"form content":    post("5", body, "text/plain", own, ""),
		"no content type": post("5", body, "", "", ""),
		"rebinding host":  post("5", body, "application/json", "", "evil.example:8848"),
		"bad seq":         post("abc", body, "application/json", own, ""),
	} {
		if code < 400 {
			t.Errorf("%s was accepted (%d)", name, code)
		}
		if !pending() {
			t.Fatalf("%s resolved the approval", name)
		}
	}

	if code := post("5", body, "application/json", own, ""); code != http.StatusNoContent {
		t.Fatalf("legit approval got %d", code)
	}
	if pending() {
		t.Fatal("approval still pending after a legit POST")
	}
	if code := post("5", body, "application/json", own, ""); code != http.StatusConflict {
		t.Fatalf("second answer got %d, want 409", code)
	}
}

func TestConfigEndpoint(t *testing.T) {
	srv := New(recorder.New(nil, nil), nil)
	srv.Config = Config{MaxCostUSD: 3.5, Policy: "default"}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var cfg Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil || cfg.MaxCostUSD != 3.5 || cfg.Approvals {
		t.Fatalf("bad config %+v (%v)", cfg, err)
	}
}
