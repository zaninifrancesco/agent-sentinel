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

func TestBudgetEndpoint(t *testing.T) {
	rec := recorder.New(nil, nil)
	b := policy.NewBudget(2, 0)
	srv := New(rec, nil)
	srv.Budget = b
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	post := func(body, contentType, origin, host string) int {
		req, _ := http.NewRequest("POST", ts.URL+"/api/budget", strings.NewReader(body))
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
	unchanged := func(what string) {
		t.Helper()
		if c, tk := b.Limits(); c != 2 || tk != 0 {
			t.Fatalf("%s changed the budget to %v / %v", what, c, tk)
		}
	}

	baseline := len(rec.Events()) // the recorder starts with a SessionStarted
	own := ts.URL
	body := `{"maxCostUsd":10,"maxTokens":50000}`
	for name, code := range map[string]int{
		"foreign origin":  post(body, "application/json", "https://evil.example", ""),
		"form content":    post(body, "text/plain", own, ""),
		"no content type": post(body, "", "", ""),
		"rebinding host":  post(body, "application/json", "", "evil.example:8848"),
		"negative cost":   post(`{"maxCostUsd":-1,"maxTokens":0}`, "application/json", own, ""),
		"negative tokens": post(`{"maxCostUsd":1,"maxTokens":-1}`, "application/json", own, ""),
		"huge cost":       post(`{"maxCostUsd":1e12,"maxTokens":0}`, "application/json", own, ""),
		"not a number":    post(`{"maxCostUsd":"lots","maxTokens":0}`, "application/json", own, ""),
		"unknown field":   post(`{"maxCostUsd":1,"maxTokens":0,"admin":true}`, "application/json", own, ""),
		"garbage":         post(`nope`, "application/json", own, ""),
	} {
		if code < 400 {
			t.Errorf("%s was accepted (%d)", name, code)
		}
		unchanged(name)
	}
	if len(rec.Events()) != baseline {
		t.Fatal("a refused change must not be recorded as one")
	}

	if code := post(body, "application/json", own, ""); code != http.StatusOK {
		t.Fatalf("legit change got %d", code)
	}
	if c, tk := b.Limits(); c != 10 || tk != 50000 {
		t.Fatalf("limits = %v / %v", c, tk)
	}

	// Raising the budget is part of the audit trail.
	evs := rec.Events()
	if len(evs) != baseline+1 {
		t.Fatalf("expected exactly one new event, got %d", len(evs)-baseline)
	}
	if last := evs[len(evs)-1]; last.Type != recorder.EventBudgetChanged || !strings.Contains(string(last.Payload), `"previousMaxCostUsd":2`) {
		t.Fatalf("budget change not recorded properly: %+v", last)
	}

	// /api/config follows the live limits.
	resp, err := http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var cfg Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil || cfg.MaxCostUSD != 10 || cfg.MaxTokens != 50000 || !cfg.BudgetEditable {
		t.Fatalf("config does not follow the budget: %+v (%v)", cfg, err)
	}
}

func TestBudgetEndpointWithoutBudget(t *testing.T) {
	ts := httptest.NewServer(New(recorder.New(nil, nil), nil).Handler())
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/api/budget", "application/json", strings.NewReader(`{"maxCostUsd":1}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("a replayed session must not accept budget changes, got %d", resp.StatusCode)
	}
}

func TestHookEndpointDefences(t *testing.T) {
	calls := 0
	srv := New(recorder.New(nil, nil), nil)
	srv.Hook = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"permission":"allow"}`))
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	post := func(contentType, origin, host string) int {
		req, _ := http.NewRequest("POST", ts.URL+"/api/hook", strings.NewReader(`{"hook_event_name":"beforeShellExecution"}`))
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

	for name, code := range map[string]int{
		"foreign origin":  post("application/json", "https://evil.example", ""),
		"form content":    post("text/plain", "", ""),
		"no content type": post("", "", ""),
		"rebinding host":  post("application/json", "", "evil.example:8848"),
	} {
		if code < 400 {
			t.Errorf("%s was accepted (%d)", name, code)
		}
	}
	if calls != 0 {
		t.Fatalf("a refused request reached the hook handler %d times", calls)
	}
	// `sentinel hook` is not a browser: no Origin, JSON content type.
	if code := post("application/json", "", ""); code != http.StatusOK || calls != 1 {
		t.Fatalf("legit hook got %d (handler calls: %d)", code, calls)
	}
}

func TestHookEndpointWithoutHandler(t *testing.T) {
	ts := httptest.NewServer(New(recorder.New(nil, nil), nil).Handler())
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/api/hook", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("got %d, want 404", resp.StatusCode)
	}
}
