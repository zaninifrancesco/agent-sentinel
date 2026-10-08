package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// Config is the optional JSON policy file (`sentinel mcp --policy FILE`).
//
//	{
//	  "rules": { "dotenv-access": "approve", "sudo": "block", "chmod-777": "allow" },
//	  "extraRules": [
//	    { "id": "no-prod-db", "description": "touches production DB",
//	      "pattern": "prod-db\\.internal", "decision": "approve", "risk": "high" }
//	  ],
//	  "maxCostUsd": 5.0,
//	  "maxTokens": 200000
//	}
type Config struct {
	// Rules overrides the decision of a built-in rule by id ("allow" disables it).
	Rules map[string]string `json:"rules"`
	// ExtraRules adds custom regex rules matched against every argument string.
	ExtraRules []ExtraRule `json:"extraRules"`
	// MaxCostUSD / MaxTokens set the session budget (nil = flag default).
	MaxCostUSD *float64 `json:"maxCostUsd"`
	MaxTokens  *int64   `json:"maxTokens"`
}

// ExtraRule is a user-defined regex rule.
type ExtraRule struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Pattern     string `json:"pattern"`
	Decision    string `json:"decision"`
	Risk        string `json:"risk"`
}

// LoadFile reads and validates a policy file.
func LoadFile(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // a typo in a security policy must not pass silently
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if _, err := NewEngine(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) apply(rules []Rule) ([]Rule, error) {
	known := map[string]int{}
	for i, r := range rules {
		known[r.ID] = i
	}

	disabled := map[string]bool{}
	for id, name := range c.Rules {
		i, ok := known[id]
		if !ok {
			return nil, fmt.Errorf("unknown rule id %q in \"rules\"", id)
		}
		d, err := ParseDecision(name)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", id, err)
		}
		if d == Allow {
			disabled[id] = true
			continue
		}
		rules[i].Decision = d
	}

	out := rules[:0:0]
	for _, r := range rules {
		if !disabled[r.ID] {
			out = append(out, r)
		}
	}

	for _, x := range c.ExtraRules {
		if x.ID == "" || x.Pattern == "" {
			return nil, fmt.Errorf("extra rule needs both \"id\" and \"pattern\"")
		}
		if _, dup := known[x.ID]; dup {
			return nil, fmt.Errorf("extra rule id %q clashes with a built-in rule", x.ID)
		}
		p, err := regexp.Compile(x.Pattern)
		if err != nil {
			return nil, fmt.Errorf("extra rule %q: %w", x.ID, err)
		}
		d, err := ParseDecision(x.Decision)
		if err != nil {
			return nil, fmt.Errorf("extra rule %q: %w", x.ID, err)
		}
		risk := recorder.RiskLevel(x.Risk)
		if _, ok := riskRank[risk]; !ok || x.Risk == "" {
			risk = recorder.RiskMedium
		}
		desc := x.Description
		if desc == "" {
			desc = "custom rule"
		}
		out = append(out, Rule{ID: x.ID, Description: desc, Decision: d, Risk: risk, Pattern: p})
	}
	return out, nil
}
