package recorder

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const RedactionVersion = "structured-text-v1"

type CaptureSettings struct {
	Profile         string `json:"profile"`
	MaxPayloadBytes int    `json:"maxPayloadBytes"`
	Redaction       string `json:"redaction"`
}

func CaptureOptions(profile string, limit int) (CaptureSettings, error) {
	if profile != "standard" && profile != "full" {
		return CaptureSettings{}, fmt.Errorf("unsupported capture profile %q: use standard or full", profile)
	}
	if limit == 0 {
		if profile == "full" {
			limit = 1 << 20
		} else {
			limit = 64 << 10
		}
	}
	if limit < 256 || limit > 16<<20 {
		return CaptureSettings{}, fmt.Errorf("max payload bytes must be between 256 and 16777216")
	}
	redaction := RedactionVersion
	if profile == "full" {
		redaction = "none"
	}
	return CaptureSettings{Profile: profile, MaxPayloadBytes: limit, Redaction: redaction}, nil
}

// PayloadCapture distinguishes the source value observed by an adapter from
// the normalized and retained JSON value. Units must not be interchanged.
type PayloadCapture struct {
	Representation      string          `json:"representation"`
	Completeness        string          `json:"completeness"` // complete, partial, omitted, unknown
	ObservedBytes       *int64          `json:"observedBytes,omitempty"`
	ObservedUnit        string          `json:"observedUnit,omitempty"`
	ObservedField       string          `json:"observedField,omitempty"`
	SourceReportedBytes *int64          `json:"sourceReportedBytes,omitempty"`
	SourceReportedUnit  string          `json:"sourceReportedUnit,omitempty"`
	NormalizedBytes     int             `json:"normalizedBytes"`
	StoredBytes         int             `json:"storedBytes"`
	Profile             string          `json:"profile"`
	Changes             []CaptureChange `json:"changes,omitempty"`
}

type CaptureChange struct {
	Kind        string `json:"kind"`  // normalized, redacted, truncated, omitted
	Stage       string `json:"stage"` // hook-client, adapter, recorder
	Method      string `json:"method"`
	Count       int    `json:"count,omitempty"`
	BeforeBytes int    `json:"beforeBytes,omitempty"`
	AfterBytes  int    `json:"afterBytes,omitempty"`
}

func ObservedPayload(representation, unit string, size int) *PayloadCapture {
	n := int64(size)
	return &PayloadCapture{Representation: representation, Completeness: "complete", ObservedBytes: &n, ObservedUnit: unit}
}

var sensitiveNames = map[string]bool{
	"password": true, "passwd": true, "apikey": true, "accesstoken": true,
	"refreshtoken": true, "clientsecret": true, "secret": true, "privatekey": true,
	"authorization": true, "proxyauthorization": true, "cookie": true, "setcookie": true,
	"secretaccesskey": true, "awssecretaccesskey": true, "credentials": true,
	"sessiontoken": true, "awssessiontoken": true,
}

func normalizedName(key string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, key)
}

var textSecrets = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----[\s\S]*?-----END (?:RSA |EC |OPENSSH )?PRIVATE KEY-----`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`(?i)\b(?:Bearer|Basic)[ \t]+[A-Za-z0-9_./+=-]{8,}`),
}
var secretAssignment = regexp.MustCompile(`(?i)(\b(?:password|passwd|api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|private[_-]?key|secret|(?:aws[_-]?)?secret[_-]?access[_-]?key|(?:aws[_-]?)?session[_-]?token|token|authorization|cookie)\b["']?)(\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,;]+)`)

func redactText(text string) (string, int) {
	count := 0
	for _, pattern := range textSecrets {
		text = pattern.ReplaceAllStringFunc(text, func(string) string { count++; return "[REDACTED]" })
	}
	text = secretAssignment.ReplaceAllStringFunc(text, func(match string) string {
		parts := secretAssignment.FindStringSubmatch(match)
		// An earlier pattern may already have removed this value.
		if strings.Contains(match, "[REDACTED]") {
			return match
		}
		count++
		return parts[1] + parts[2] + `"[REDACTED]"`
	})
	return text, count
}

func redactValue(value any) (any, int) {
	count := 0
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			name := normalizedName(key)
			_, isText := child.(string)
			if sensitiveNames[name] || name == "token" && isText {
				if child != nil && child != "" {
					v[key] = "[REDACTED]"
					count++
				}
				continue
			}
			var n int
			v[key], n = redactValue(child)
			count += n
		}
	case []any:
		for index, child := range v {
			var n int
			v[index], n = redactValue(child)
			count += n
		}
	case string:
		return redactText(v)
	}
	return value, count
}

// retainJSON emits a valid JSON prefix within budget. Object keys are sorted
// so the result is deterministic. Capture metadata, not a fake tool field,
// declares omitted entries and shortened strings.
func retainJSON(value any, budget int) []byte {
	switch v := value.(type) {
	case string:
		encoded, _ := json.Marshal(v)
		if len(encoded) <= budget {
			return encoded
		}
		lo, hi := 0, len(v)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			prefix := v[:mid]
			for len(prefix) > 0 && !utf8.ValidString(prefix) {
				prefix = prefix[:len(prefix)-1]
			}
			candidate, _ := json.Marshal(prefix)
			if len(candidate) <= budget {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		prefix := v[:lo]
		for len(prefix) > 0 && !utf8.ValidString(prefix) {
			prefix = prefix[:len(prefix)-1]
		}
		out, _ := json.Marshal(prefix)
		return out
	case map[string]any:
		out := []byte{'{'}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			name, _ := json.Marshal(key)
			comma := 0
			if len(out) > 1 {
				comma = 1
			}
			left := budget - len(out) - comma - len(name) - 2 // colon and closing brace
			if left < 4 {
				continue
			}
			child := retainJSON(v[key], left)
			if len(child) > left {
				continue
			}
			if comma > 0 {
				out = append(out, ',')
			}
			out = append(out, name...)
			out = append(out, ':')
			out = append(out, child...)
		}
		return append(out, '}')
	case []any:
		out := []byte{'['}
		for _, child := range v {
			comma := 0
			if len(out) > 1 {
				comma = 1
			}
			left := budget - len(out) - comma - 1
			if left < 4 {
				break
			}
			encoded := retainJSON(child, left)
			if len(encoded) > left {
				break
			}
			if comma > 0 {
				out = append(out, ',')
			}
			out = append(out, encoded...)
		}
		return append(out, ']')
	default:
		out, _ := json.Marshal(value)
		if len(out) > budget {
			return []byte("null")
		}
		return out
	}
}

func (r *Recorder) captureEvent(e Event) Event {
	if r.capture.Profile == "standard" {
		fields := append([]string(nil), e.MetadataRedactedFields...)
		for _, field := range []struct {
			name     string
			value    *string
			identity bool
		}{
			{"reason", &e.Reason, false}, {"projectRef", &e.ProjectRef, false}, {"method", &e.Method, false}, {"toolName", &e.ToolName, false},
			{"agentSessionId", &e.AgentSessionID, true}, {"sourceId", &e.SourceID, true}, {"rpcId", &e.RPCID, true},
		} {
			before := *field.value
			if field.identity {
				*field.value = r.captureIdentity(before)
			} else {
				*field.value, _ = redactText(before)
			}
			if *field.value != before {
				fields = append(fields, field.name)
			}
		}
		if e.NativeCallID != nil {
			native := *e.NativeCallID
			native.Value = r.captureIdentity(native.Value)
			if native.Value != e.NativeCallID.Value {
				fields = append(fields, "nativeCallId")
				e.NativeCallID = &native
			}
		}
		e.MetadataRedactedFields = fields
	}
	if len(e.Payload) == 0 {
		e.Capture = nil
		return e
	}
	metadata := PayloadCapture{Representation: "unspecified", Completeness: "unknown"}
	if e.Capture != nil {
		metadata = *e.Capture
		metadata.Changes = append([]CaptureChange(nil), e.Capture.Changes...)
	}
	metadata.Profile = r.capture.Profile
	canonical, err := json.Marshal(e.Payload)
	if err != nil {
		canonical, _ = json.Marshal(string(e.Payload))
		metadata.Representation = "text"
		metadata.Completeness = "unknown"
	}
	metadata.NormalizedBytes = len(canonical)
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	var value any
	_ = decoder.Decode(&value) // canonical is valid JSON, including the fallback above
	if r.capture.Profile == "standard" {
		clean, count := redactValue(value)
		value = clean
		if count > 0 {
			before := len(canonical)
			canonical, _ = json.Marshal(value)
			metadata.Changes = append(metadata.Changes, CaptureChange{Kind: "redacted", Stage: "recorder", Method: RedactionVersion, Count: count, BeforeBytes: before, AfterBytes: len(canonical)})
			if metadata.Completeness != "omitted" {
				metadata.Completeness = "partial"
			}
		}
	}
	if len(canonical) > r.capture.MaxPayloadBytes {
		before := len(canonical)
		canonical = retainJSON(value, r.capture.MaxPayloadBytes)
		metadata.Changes = append(metadata.Changes, CaptureChange{Kind: "truncated", Stage: "recorder", Method: "json-prefix-v1", BeforeBytes: before, AfterBytes: len(canonical)})
		if metadata.Completeness != "omitted" {
			metadata.Completeness = "partial"
		}
	}
	metadata.StoredBytes = len(canonical)
	e.Payload = canonical
	e.Capture = &metadata
	return e
}

func (r *Recorder) captureIdentity(value string) string {
	if r.capture.Profile != "standard" {
		return value
	}
	if clean, _ := redactText(value); clean == value {
		return value
	}
	mac := hmac.New(sha256.New, r.identityKey[:])
	_, _ = mac.Write([]byte(value))
	return "redacted-id/" + hex.EncodeToString(mac.Sum(nil))
}

func captureCommand(command []string, settings CaptureSettings) []string {
	out := append([]string(nil), command...)
	if settings.Profile != "standard" {
		return out
	}
	maskNext := false
	for index, arg := range out {
		if maskNext {
			out[index] = "[REDACTED]"
			maskNext = false
			continue
		}
		out[index], _ = redactText(arg)
		name := normalizedName(strings.TrimLeft(arg, "-"))
		maskNext = strings.HasPrefix(arg, "-") && (sensitiveNames[name] || name == "token")
	}
	return out
}
