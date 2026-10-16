package recorder

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// Recorder is an append-only log of events for a single session. It keeps
// everything in memory (for the future WebSocket stream and replay) and can
// optionally mirror each event to a JSONL writer for persistence.
type Recorder struct {
	mu          sync.RWMutex
	session     Session
	events      []Event
	seq         uint64
	identitySeq uint64
	sink        *json.Encoder
	subs        []chan Event
	closed      bool
	capture     CaptureSettings
	identityKey [32]byte
	persistence Persistence
}

// Persistence reports writes acknowledged by the sink, not fsync durability.
type Persistence struct {
	Enabled        bool   `json:"enabled"`
	WrittenThrough uint64 `json:"writtenThrough"`
	Error          string `json:"error,omitempty"`
}

func (r *Recorder) Persistence() Persistence {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.persistence
}

// NewIdentity reserves a recording-scoped identity, serialized in the log so
// replay never has to reconstruct it from tool names or payloads.
func (r *Recorder) NewIdentity(kind string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.identitySeq++
	return fmt.Sprintf("%s/%s/%d", r.session.ID, kind, r.identitySeq)
}

// New creates a recorder for a new session. sink may be nil.
func New(command []string, sink io.Writer) *Recorder {
	settings, _ := CaptureOptions("standard", 0)
	return newRecorder(command, sink, settings)
}

func NewWithCapture(command []string, sink io.Writer, settings CaptureSettings) (*Recorder, error) {
	validated, err := CaptureOptions(settings.Profile, settings.MaxPayloadBytes)
	if err != nil {
		return nil, err
	}
	return newRecorder(command, sink, validated), nil
}

func newRecorder(command []string, sink io.Writer, settings CaptureSettings) *Recorder {
	r := &Recorder{
		session: Session{ID: newID(), Command: captureCommand(command, settings), StartedAt: time.Now().UTC(), Capture: &settings},
		capture: settings,
	}
	_, _ = rand.Read(r.identityKey[:])
	if sink != nil {
		r.sink = json.NewEncoder(sink)
		r.persistence.Enabled = true
	}
	r.Record(Event{Type: EventSessionStarted, Status: StatusOK, CaptureSettings: &settings})
	return r
}

func (r *Recorder) CaptureSettings() CaptureSettings { return r.capture }

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
func (r *Recorder) Record(e Event) Event { return r.RecordHook(e, nil) }

// RecordHook is Record, but calls hook with the final event (Seq assigned)
// before the event becomes visible to subscribers, the sink or Events().
// The approval flow uses it to register a pending approval under that Seq, so
// the cockpit can never answer a request the proxy does not know about yet.
// hook runs under the recorder lock: it must be quick and must not call back
// into the recorder.
func (r *Recorder) RecordHook(e Event, hook func(Event)) Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	e.Seq = r.seq
	e.SessionID = r.session.ID
	e.SchemaVersion = 2
	e.RecordingID = r.session.ID
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if e.Risk == "" {
		e.Risk = RiskNone
	}
	e = r.captureEvent(e)
	if hook != nil {
		hook(e)
	}
	r.events = append(r.events, e)

	if r.sink != nil {
		if err := r.sink.Encode(e); err != nil {
			r.persistence.Error = err.Error()
		} else if r.persistence.Error == "" {
			r.persistence.WrittenThrough = e.Seq
		}
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

// Snapshot returns metadata and evidence under the same lock. Profiling pages
// pin the returned revision so later responses cannot change earlier pages.
func (r *Recorder) Snapshot() (Session, []Event, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.session, append([]Event(nil), r.events...), r.seq
}

func (r *Recorder) Revision() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.seq
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
