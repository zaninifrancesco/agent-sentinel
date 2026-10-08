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
	pending map[uint64]chan Verdict
}

func NewBroker() *Broker { return &Broker{pending: make(map[uint64]chan Verdict)} }

// Open registers a request waiting for a decision.
func (b *Broker) Open(id uint64) {
	b.mu.Lock()
	b.pending[id] = make(chan Verdict, 1)
	b.mu.Unlock()
}

// Wait blocks until the request is resolved or ctx ends (give it a deadline to
// get a timeout: that returns ErrTimeout). It always removes the request.
func (b *Broker) Wait(ctx context.Context, id uint64) (Verdict, error) {
	b.mu.Lock()
	ch := b.pending[id]
	b.mu.Unlock()
	if ch == nil {
		return Verdict{}, ErrNotPending
	}
	defer func() {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
	}()
	select {
	case v := <-ch:
		return v, nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Verdict{}, ErrTimeout
		}
		return Verdict{}, ctx.Err()
	}
}

// Resolve delivers a decision. It fails if the request is unknown or was
// already resolved/expired, so a double click cannot approve twice.
func (b *Broker) Resolve(id uint64, v Verdict) error {
	b.mu.Lock()
	ch, ok := b.pending[id]
	if ok {
		delete(b.pending, id)
	}
	b.mu.Unlock()
	if !ok {
		return ErrNotPending
	}
	ch <- v // buffered: never blocks
	return nil
}

// Pending lists the ids currently waiting for a decision.
func (b *Broker) Pending() []uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	ids := make([]uint64, 0, len(b.pending))
	for id := range b.pending {
		ids = append(ids, id)
	}
	return ids
}
