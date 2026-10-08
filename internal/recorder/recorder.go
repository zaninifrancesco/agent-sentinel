package recorder

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Recorder is an append-only log of events for a single session. It keeps
// everything in memory (for the future WebSocket stream and replay) and can
// optionally mirror each event to a JSONL writer for persistence.
type Recorder struct {
	mu      sync.RWMutex
	session Session
	events  []Event
	seq     uint64
	sink    *json.Encoder
	subs    []chan Event
	closed  bool
}

// New creates a recorder for a new session. sink may be nil.
func New(command []string, sink io.Writer) *Recorder {
	r := &Recorder{
		session: Session{ID: newID(), Command: command, StartedAt: time.Now().UTC()},
	}
	if sink != nil {
		r.sink = json.NewEncoder(sink)
	}
	r.Record(Event{Type: EventSessionStarted, Status: StatusOK})
	return r
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Session returns a copy of the session metadata.
func (r *Recorder) Session() Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.session
}

// Record appends an event, filling Seq, SessionID, Timestamp and defaults.
// It returns the stored event. Safe for concurrent use.
func (r *Recorder) Record(e Event) Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	e.Seq = r.seq
	e.SessionID = r.session.ID
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if e.Risk == "" {
		e.Risk = RiskNone
	}
	r.events = append(r.events, e)

	if r.sink != nil {
		_ = r.sink.Encode(e)
	}
	for _, ch := range r.subs {
		select {
		case ch <- e:
		default: // never let a slow subscriber stall the proxy hot path
		}
	}
	return e
}

// Events returns a snapshot of all events recorded so far.
func (r *Recorder) Events() []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

// Subscribe returns a channel receiving every future event. Events are
// dropped for subscribers whose buffer is full.
func (r *Recorder) Subscribe(buffer int) <-chan Event {
	ch := make(chan Event, buffer)
	r.mu.Lock()
	r.subs = append(r.subs, ch)
	r.mu.Unlock()
	return ch
}

// Watch atomically returns the events recorded so far and a channel with all
// the following ones, so a late subscriber (e.g. a browser that just opened
// the dashboard) sees neither gaps nor duplicates. Call cancel when done.
// The channel is closed when the recorder is closed or cancel is called.
func (r *Recorder) Watch(buffer int) (history []Event, live <-chan Event, cancel func()) {
	ch := make(chan Event, buffer)

	r.mu.Lock()
	history = make([]Event, len(r.events))
	copy(history, r.events)
	if r.closed {
		close(ch)
	} else {
		r.subs = append(r.subs, ch)
	}
	r.mu.Unlock()

	cancel = func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for i, s := range r.subs {
			if s == ch {
				r.subs = append(r.subs[:i], r.subs[i+1:]...)
				close(ch)
				return
			}
		}
	}
	return history, ch, cancel
}

// Close marks the session as completed and closes subscriber channels.
func (r *Recorder) Close() {
	r.mu.Lock()
	r.session.EndedAt = time.Now().UTC()
	r.mu.Unlock()

	r.Record(Event{Type: EventSessionCompleted, Status: StatusOK})

	r.mu.Lock()
	r.closed = true
	for _, ch := range r.subs {
		close(ch)
	}
	r.subs = nil
	r.mu.Unlock()
}
