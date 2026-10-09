package policy

import (
	"context"
	"errors"
	"sync"
)

// Verdict is the human's answer to an approval request.
type Verdict struct {
	Approved bool
	// Feedback is the operator's steering note. On approval it is appended to
	// the tool result so the agent sees it; on rejection it is part of the
	// error the agent receives.
	Feedback string
}

var (
	ErrNotPending = errors.New("no pending approval with this id")
	ErrTimeout    = errors.New("approval timed out")
)

// Broker connects the proxy goroutine that holds a call with the cockpit
// that resolves it. Requests are keyed by the seq of the recorded event.
type Broker struct {
	mu      sync.Mutex
	pending map[uint64]*request
}

// request is one call held for a human. It stays in the table until Wait has
// collected its answer, so an answer that comes before Wait started is kept.
type request struct {
	answer   chan Verdict // buffered: sending never blocks
	answered bool
}

func NewBroker() *Broker { return &Broker{pending: make(map[uint64]*request)} }

// Open registers a request waiting for a decision.
func (b *Broker) Open(id uint64) {
	b.mu.Lock()
	b.pending[id] = &request{answer: make(chan Verdict, 1)}
	b.mu.Unlock()
}

// Wait blocks until the request is resolved or ctx ends (give it a deadline to
// get a timeout: that returns ErrTimeout). It always removes the request.
func (b *Broker) Wait(ctx context.Context, id uint64) (Verdict, error) {
	b.mu.Lock()
	r := b.pending[id]
	b.mu.Unlock()
	if r == nil {
		return Verdict{}, ErrNotPending
	}
	select {
	case v := <-r.answer:
		b.remove(id)
		return v, nil
	case <-ctx.Done():
		// An answer may have landed in the same instant. Resolve answers under
		// the lock, so under the lock either it is there (deliver it: the
		// human was told it counted) or it never will be (the request is
		// removed, and a late Resolve is refused).
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.pending, id)
		select {
		case v := <-r.answer:
			return v, nil
		default:
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Verdict{}, ErrTimeout
		}
		return Verdict{}, ctx.Err()
	}
}

func (b *Broker) remove(id uint64) {
	b.mu.Lock()
	delete(b.pending, id)
	b.mu.Unlock()
}

// Resolve delivers a decision. It fails if the request is unknown or was
// already resolved/expired, so a double click cannot approve twice.
func (b *Broker) Resolve(id uint64, v Verdict) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.pending[id]
	if !ok || r.answered {
		return ErrNotPending
	}
	r.answered = true
	r.answer <- v // buffered: never blocks
	return nil
}

// Pending lists the ids currently waiting for a decision.
func (b *Broker) Pending() []uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	ids := make([]uint64, 0, len(b.pending))
	for id, r := range b.pending {
		if !r.answered {
			ids = append(ids, id)
		}
	}
	return ids
}
