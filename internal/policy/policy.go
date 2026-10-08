// Package policy decides what happens to a tool call BEFORE it reaches the MCP
// server: let it through, warn, hold it for human approval, or block it.
package policy

import (
	"fmt"
	"strings"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// Decision is what the boundary does with a call. Higher is stricter.
type Decision int

const (
	Allow           Decision = iota // forward untouched
	Warn                            // forward, but flag it in the cockpit
	RequireApproval                 // hold until a human approves or rejects
	Block                           // refuse immediately
)

func (d Decision) String() string {
	switch d {
	case Allow:
		return "allow"
	case Warn:
		return "warn"
	case RequireApproval:
		return "approve"
	case Block:
		return "block"
	}
	return "unknown"
}

// ParseDecision accepts the names used in policy files.
func ParseDecision(s string) (Decision, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "allow":
		return Allow, nil
	case "warn":
		return Warn, nil
	case "approve", "require_approval", "require-approval":
		return RequireApproval, nil
	case "block":
		return Block, nil
	}
	return Allow, fmt.Errorf("unknown decision %q (want allow, warn, approve or block)", s)
}

// Call is a tool call as seen by the policy engine.
type Call struct {
	Tool  string
	Texts []string // every string found anywhere in the arguments
}

// NewCall flattens the arguments of a tools/call into the list of strings the
// rules inspect. Tool-agnostic on purpose: an unknown server may name its
// "command" argument anything, and the policy must still see it.
func NewCall(tool string, args map[string]any) Call {
	c := Call{Tool: tool}
	collect(args, &c.Texts)
	return c
}

func collect(v any, out *[]string) {
	switch t := v.(type) {
	case string:
		*out = append(*out, t)
	case []any:
		for _, x := range t {
			collect(x, out)
		}
	case map[string]any:
		for _, x := range t {
			collect(x, out)
		}
	}
}

// Result is the outcome of evaluating one call.
type Result struct {
	Decision Decision
	Risk     recorder.RiskLevel
	RuleID   string // empty when nothing matched
	Reason   string
}

var riskRank = map[recorder.RiskLevel]int{
	recorder.RiskNone: 0, recorder.RiskLow: 1, recorder.RiskMedium: 2,
	recorder.RiskHigh: 3, recorder.RiskCritical: 4,
}

// stricter reports whether a is a stronger outcome than b.
func stricter(a, b Result) bool {
	if a.Decision != b.Decision {
		return a.Decision > b.Decision
	}
	return riskRank[a.Risk] > riskRank[b.Risk]
}
