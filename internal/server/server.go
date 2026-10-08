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
	"time"

	"github.com/coder/websocket"

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
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.Handle("/", s.spa())
	return mux
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
