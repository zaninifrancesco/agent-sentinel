package policy

import (
	"fmt"
	"sync"
)

// Price model used for the estimate, in USD per million tokens. It matches
// the cockpit's metrics bar so both show the same number.
const (
	inputUSDPerMTok  = 3.0
	outputUSDPerMTok = 15.0
	bytesPerToken    = 4
)

// Budget is the per-session circuit breaker. The proxy cannot see the LLM API
// traffic, so spend is ESTIMATED from the size of what crosses it:
//   - tool-call arguments were written by the model -> output tokens
//   - tool results are fed back to the model        -> input tokens
//
// A zero limit disables that limit.
type Budget struct {
	mu        sync.Mutex
	maxCost   float64
	maxTokens int64
	step      float64 // how much the cost limit grows each time a human approves going over
	stepTok   int64
	in, out   int64
}

func NewBudget(maxCostUSD float64, maxTokens int64) *Budget {
	return &Budget{maxCost: maxCostUSD, maxTokens: maxTokens, step: maxCostUSD, stepTok: maxTokens}
}

func tokens(nbytes int) int64 { return int64((nbytes + bytesPerToken - 1) / bytesPerToken) }

// AddOutput records bytes the model produced (a tool call).
func (b *Budget) AddOutput(nbytes int) {
	b.mu.Lock()
	b.out += tokens(nbytes)
	b.mu.Unlock()
}

// AddInput records bytes handed back to the model (a tool result).
func (b *Budget) AddInput(nbytes int) {
	b.mu.Lock()
	b.in += tokens(nbytes)
	b.mu.Unlock()
}

func (b *Budget) costLocked() float64 {
	return float64(b.in)/1e6*inputUSDPerMTok + float64(b.out)/1e6*outputUSDPerMTok
}

// Spent returns the current estimate.
func (b *Budget) Spent() (costUSD float64, totalTokens int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.costLocked(), b.in + b.out
}

// MaxCostUSD returns the current cost limit (0 = none).
func (b *Budget) MaxCostUSD() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxCost
}

// Limits returns the current cost and token limits (0 = none).
func (b *Budget) Limits() (costUSD float64, tokens int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxCost, b.maxTokens
}

// Enabled reports whether any limit is set. Safe on a nil Budget.
func (b *Budget) Enabled() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxCost > 0 || b.maxTokens > 0
}

// Upper bounds for a limit typed by a human; far above any real session, they
// only keep absurd or non-finite values out.
const (
	MaxLimitCostUSD = 1_000_000.0
	MaxLimitTokens  = int64(1_000_000_000_000)
)

// SetLimits replaces both limits (0 = no limit) and makes them the new step
// for later extensions. Spend so far is kept, so a limit below it holds the
// very next call for a human.
func (b *Budget) SetLimits(costUSD float64, tokens int64) error {
	// !(x >= 0) also rejects NaN.
	if !(costUSD >= 0) || costUSD > MaxLimitCostUSD {
		return fmt.Errorf("cost limit must be between 0 and %.0f USD", MaxLimitCostUSD)
	}
	if tokens < 0 || tokens > MaxLimitTokens {
		return fmt.Errorf("token limit must be between 0 and %d", MaxLimitTokens)
	}
	b.mu.Lock()
	b.maxCost, b.step = costUSD, costUSD
	b.maxTokens, b.stepTok = tokens, tokens
	b.mu.Unlock()
	return nil
}

// Exceeded reports whether a limit is crossed, with a human-readable reason.
func (b *Budget) Exceeded() (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxCost > 0 && b.costLocked() >= b.maxCost {
		return true, fmt.Sprintf("estimated cost $%.3f reached the $%.2f session budget", b.costLocked(), b.maxCost)
	}
	if b.maxTokens > 0 && b.in+b.out >= b.maxTokens {
		return true, fmt.Sprintf("estimated %d tokens reached the %d token session budget", b.in+b.out, b.maxTokens)
	}
	return false, ""
}

// Extend raises the limits by one step. Called when a human approves
// continuing past the budget, so they are asked again only after the next step.
func (b *Budget) Extend() {
	b.mu.Lock()
	b.maxCost += b.step
	b.maxTokens += b.stepTok
	b.mu.Unlock()
}
