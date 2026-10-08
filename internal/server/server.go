// Package server exposes the flight recorder to the Sentinel cockpit: a small
// HTTP API, a WebSocket event stream and the embedded single-page app.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// Wire messages sent over the WebSocket.
type snapshotMsg struct {
	Type    string           `json:"type"` // "snapshot"
	Session recorder.Session `json:"session"`
	Events  []recorder.Event `json:"events"`
}

type eventMsg struct {
	Type  string         `json:"type"` // "event"
	Event recorder.Event `json:"event"`
}

// Server serves one recorder (one session).
type Server struct {
	rec    *recorder.Recorder
	assets fs.FS // built SPA (index.html + assets); may be nil/empty

	// AllowedOrigins lists extra browser origins (host[:port] patterns) allowed
	// to open the WebSocket, on top of the server's own origin. Only meant for
	// the Vite dev server (`--dev`); empty by default.
	AllowedOrigins []string

	// Approvals receives the human decisions for held tool calls. Nil when
	// replaying a recorded session: there is nothing left to approve.
	Approvals *policy.Broker

	// Config is what the cockpit needs to know about the running session.
	Config Config
}

// Config is served at /api/config.
type Config struct {
	MaxCostUSD float64 `json:"maxCostUsd"` // 0 = no budget
	MaxTokens  int64   `json:"maxTokens"`
	Policy     string  `json:"policy"`    // "default", a file path or "off"
	Approvals  bool    `json:"approvals"` // can this session approve held calls?
}

func New(rec *recorder.Recorder, assets fs.FS) *Server {
	return &Server{rec: rec, assets: assets}
}

// DevOrigins are the origins of the Vite dev server (`npm run dev`).
var DevOrigins = []string{"localhost:5173", "127.0.0.1:5173"}

// Handler returns the HTTP handler with all routes mounted.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("POST /api/approvals/{seq}", s.handleApproval)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.Handle("/", s.spa())
	return s.loopbackOnly(mux)
}

// loopbackOnly rejects any request whose Host is not a loopback name. The
// listener is bound to 127.0.0.1, but a malicious page could still reach it
// through DNS rebinding (evil.com resolving to 127.0.0.1); that request would
// carry Host: evil.com, which we refuse.
func (s *Server) loopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		switch strings.Trim(host, "[]") {
		case "127.0.0.1", "localhost", "::1":
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "forbidden host", http.StatusForbidden)
		}
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := s.Config
	cfg.Approvals = s.Approvals != nil
	writeJSON(w, cfg)
}

type approvalRequest struct {
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback"`
}

// handleApproval resolves a held tool call. It is the most sensitive endpoint
// of the cockpit (it can let an agent run a dangerous command), so besides the
// loopback Host check it also defends against cross-site requests:
//   - Content-Type must be application/json, which a cross-origin page cannot
//     send without a CORS preflight that we never grant;
//   - if the browser sends an Origin it must be our own (or the dev server).
func (s *Server) handleApproval(w http.ResponseWriter, r *http.Request) {
	if s.Approvals == nil {
		http.Error(w, "this session has nothing to approve", http.StatusNotFound)
		return
	}
	if !s.originOK(r) {
		http.Error(w, "cross-origin request refused", http.StatusForbidden)
		return
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	seq, err := strconv.ParseUint(r.PathValue("seq"), 10, 64)
	if err != nil {
		http.Error(w, "bad seq", http.StatusBadRequest)
		return
	}
	var req approvalRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	switch err := s.Approvals.Resolve(seq, policy.Verdict{Approved: req.Approved, Feedback: strings.TrimSpace(req.Feedback)}); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, policy.ErrNotPending):
		// Already answered, timed out or cancelled by the agent.
		http.Error(w, "this request is no longer pending", http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // not a browser (curl, scripts): same trust level as the user
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	for _, o := range s.AllowedOrigins {
		if u.Host == o {
			return true
		}
	}
	return false
}

// Serve runs the server on ln until ctx is cancelled.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handleSession(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.rec.Session())
}

func (s *Server) handleEvents(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.rec.Events())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// handleWS sends a snapshot of the session followed by every new event.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Accept rejects cross-origin browsers by default, which is exactly what
	// we want for a local-only cockpit: no random website can read the
	// agent's activity through the user's browser.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.AllowedOrigins})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	// CloseRead drains control frames and cancels ctx when the peer leaves.
	ctx := conn.CloseRead(r.Context())

	history, live, cancel := s.rec.Watch(1024)
	defer cancel()

	if err := writeWS(ctx, conn, snapshotMsg{Type: "snapshot", Session: s.rec.Session(), Events: history}); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-live:
			if !ok {
				_ = conn.Close(websocket.StatusNormalClosure, "session ended")
				return
			}
			if err := writeWS(ctx, conn, eventMsg{Type: "event", Event: ev}); err != nil {
				return
			}
		}
	}
}

func writeWS(ctx context.Context, c *websocket.Conn, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, b)
}
