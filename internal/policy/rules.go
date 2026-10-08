package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// Scope says what a rule's pattern is matched against.
type Scope int

const (
	// ScopeText matches the whole argument string (shell commands, SQL, file
	// contents...).
	ScopeText Scope = iota
	// ScopeToken matches each word of the string, so `cat ~/.ssh/id_rsa` and a
	// plain `path: ~/.ssh/id_rsa` argument are both caught.
	ScopeToken
)

// Rule is one check of the built-in or user-defined policy.
type Rule struct {
	ID          string
	Description string
	Decision    Decision
	Risk        recorder.RiskLevel
	Scope       Scope
	Pattern     *regexp.Regexp
}

var tokenSplit = regexp.MustCompile("[\\s'\"`=:,;|&()<>]+")

// match returns the offending fragment, or "" if the call is clean.
func (r Rule) match(c Call) string {
	for _, text := range c.Texts {
		switch r.Scope {
		case ScopeToken:
			for _, tok := range tokenSplit.Split(text, -1) {
				if tok != "" && r.Pattern.MatchString(tok) {
					return tok
				}
			}
		default:
			if loc := r.Pattern.FindStringIndex(text); loc != nil {
				return text[loc[0]:loc[1]]
			}
		}
	}
	return ""
}

func rx(p string) *regexp.Regexp { return regexp.MustCompile(p) }

// DefaultRules is the built-in boundary: destructive commands, sensitive
// files and leaked secrets.
func DefaultRules() []Rule {
	const (
		high     = recorder.RiskHigh
		critical = recorder.RiskCritical
		medium   = recorder.RiskMedium
	)
	return []Rule{
		// --- Destructive commands -------------------------------------
		{
			ID: "rm-root", Description: "recursive delete of a root, home or wildcard path",
			Decision: Block, Risk: critical,
			Pattern: rx(`\brm\s+(?:-[a-zA-Z-]+\s+)+(?:/|~|\$HOME|/\*|\*)(?:\s|$)`),
		},
		{
			ID: "disk-destroy", Description: "formats or overwrites a disk, or fork bomb",
			Decision: Block, Risk: critical,
			Pattern: rx(`\bmkfs(?:\.\w+)?\b|\bdd\s+[^\n]*\bof=/dev/|:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`),
		},
		{
			ID: "rm-recursive", Description: "recursive file deletion",
			Decision: RequireApproval, Risk: high,
			Pattern: rx(`\brm\s+(?:-[a-zA-Z-]+\s+)*(?:-[a-zA-Z]*[rR][a-zA-Z]*|--recursive)\b`),
		},
		{
			ID: "git-force-push", Description: "force push can overwrite remote history",
			Decision: RequireApproval, Risk: high,
			Pattern: rx(`\bgit\s+push\b[^|;&\n]*(?:\s--force(?:-with-lease)?\b|\s-f\b)`),
		},
		{
			ID: "git-destructive", Description: "discards local changes or branches",
			Decision: RequireApproval, Risk: medium,
			Pattern: rx(`\bgit\s+(?:reset\s+--hard|clean\s+-[a-zA-Z]*f|checkout\s+--\s+\.|branch\s+-D)\b`),
		},
		{
			ID: "pipe-to-shell", Description: "downloads and executes a script",
			Decision: RequireApproval, Risk: high,
			Pattern: rx(`(?:curl|wget)\b[^|\n]*\|\s*(?:sudo\s+)?(?:ba|z|da)?sh\b`),
		},
		{
			ID: "sudo", Description: "privilege escalation",
			Decision: RequireApproval, Risk: high,
			Pattern: rx(`(?:^|[;&|]\s*|\s)sudo\s`),
		},
		{
			ID: "sql-destructive", Description: "destructive SQL statement",
			Decision: RequireApproval, Risk: high,
			Pattern: rx(`(?i)\bdrop\s+(?:table|database|schema)\b|\btruncate\s+table\b|\bdelete\s+from\s+\w+\s*(?:;|$)`),
		},
		{
			ID: "chmod-777", Description: "world-writable permissions",
			Decision: Warn, Risk: medium,
			Pattern: rx(`\bchmod\s+(?:-R\s+)?0?777\b`),
		},

		// --- Sensitive files ------------------------------------------
		{
			ID: "dotenv-access", Description: "reads or writes a .env file with secrets",
			Decision: Block, Risk: critical, Scope: ScopeToken,
			Pattern: rx(`(?i)(?:^|[/\\])\.env(?:\.(?:local|prod(?:uction)?|dev(?:elopment)?|stage|staging|test|secrets?))?$`),
		},
		{
			ID: "ssh-keys", Description: "touches SSH keys",
			Decision: Block, Risk: critical, Scope: ScopeToken,
			Pattern: rx(`(?i)(?:^|[/\\])\.ssh(?:[/\\]|$)|(?:^|[/\\])id_(?:rsa|dsa|ecdsa|ed25519)$`),
		},
		{
			ID: "credential-files", Description: "touches cloud or package-registry credentials",
			Decision: RequireApproval, Risk: high, Scope: ScopeToken,
			Pattern: rx(`(?i)(?:^|/)\.aws/(?:credentials|config)$|(?:^|/)\.config/gcloud(?:/|$)|(?:^|/)\.kube/config$|(?:^|/)\.docker/config\.json$|(?:^|/)\.(?:netrc|pgpass|npmrc|pypirc)$|(?:^|/)\.gnupg(?:/|$)`),
		},
		{
			ID: "private-key-files", Description: "touches a private key or certificate bundle",
			Decision: RequireApproval, Risk: high, Scope: ScopeToken,
			Pattern: rx(`(?i)\.(?:pem|p12|pfx|key)$`),
		},

		// --- Secrets in arguments -------------------------------------
		{
			ID: "secret-private-key", Description: "private key material in arguments",
			Decision: RequireApproval, Risk: critical,
			Pattern: rx(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY`),
		},
		{
			ID: "secret-token", Description: "API token or cloud key in arguments",
			Decision: RequireApproval, Risk: critical,
			Pattern: rx(`\bAKIA[0-9A-Z]{16}\b|\bgh[pousr]_[A-Za-z0-9]{36,}\b|\bxox[baprs]-[A-Za-z0-9-]{10,}|\bsk-[A-Za-z0-9_-]{20,}|\bAIza[0-9A-Za-z_-]{35}\b`),
		},
		{
			ID: "secret-assignment", Description: "hard-coded credential assignment",
			Decision: Warn, Risk: medium,
			Pattern: rx(`(?i)\b(?:api[_-]?key|secret|passwd|password|token)\b\s*[:=]\s*['"]?[A-Za-z0-9/+_.-]{16,}`),
		},
	}
}

// Engine evaluates calls against a set of rules.
type Engine struct {
	rules []Rule
}

// NewEngine builds an engine from a config; a nil config means the defaults.
func NewEngine(cfg *Config) (*Engine, error) {
	rules := DefaultRules()
	if cfg != nil {
		var err error
		if rules, err = cfg.apply(rules); err != nil {
			return nil, err
		}
	}
	return &Engine{rules: rules}, nil
}

// Rules returns the active rules (read-only use).
func (e *Engine) Rules() []Rule { return e.rules }

// Evaluate returns the strictest outcome among all matching rules.
func (e *Engine) Evaluate(c Call) Result {
	best := Result{Decision: Allow, Risk: recorder.RiskNone}
	for _, r := range e.rules {
		frag := r.match(c)
		if frag == "" {
			continue
		}
		res := Result{
			Decision: r.Decision,
			Risk:     r.Risk,
			RuleID:   r.ID,
			Reason:   fmt.Sprintf("%s (%s): %s", r.Description, r.ID, excerpt(frag)),
		}
		if stricter(res, best) {
			best = res
		}
	}
	return best
}

func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return fmt.Sprintf("%q", s)
}
