package recorder

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStandardRedactsBeforePersistenceAndLiveDelivery(t *testing.T) {
	var log bytes.Buffer
	command := []string{"server", "--api-key", "synthetic-launch-secret"}
	rec := New(command, &log)
	_, live, cancel := rec.Watch(2)
	defer cancel()
	raw := json.RawMessage(`{"api_key":"synthetic-structured-secret","nested":[{"credentials":{"value":"nested-sensitive-value"}}],"text":"token=synthetic-text-secret","token":123,"n":9007199254740993}`)
	e := rec.Record(Event{Type: EventToolCallRequest, Payload: raw, Capture: ObservedPayload("mcp-frame", "mcp-frame", len(raw)), AgentSessionID: "sk-synthetic-session-secret-0000", NativeCallID: &NativeID{Type: "string", Value: "sk-synthetic-native-secret-0000"}})
	b, _ := json.Marshal(<-live)
	for _, secret := range []string{"synthetic-structured-secret", "nested-sensitive-value", "synthetic-text-secret", "synthetic-launch-secret", "sk-synthetic-session-secret-0000", "sk-synthetic-native-secret-0000"} {
		if bytes.Contains(log.Bytes(), []byte(secret)) || bytes.Contains(b, []byte(secret)) {
			t.Fatalf("secret persisted or streamed: %q", secret)
		}
	}
	if command[2] != "synthetic-launch-secret" || rec.Session().Command[2] != "[REDACTED]" {
		t.Fatal("capture changed source command or failed to sanitize recording metadata")
	}
	if !bytes.Contains(raw, []byte("synthetic-structured-secret")) {
		t.Fatal("caller input mutated")
	}
	if e.Capture.Completeness != "partial" || len(e.Capture.Changes) != 1 || e.Capture.Changes[0].Kind != "redacted" {
		t.Fatalf("redaction not declared: %+v", e.Capture)
	}
	stored, _ := json.Marshal(e.Payload)
	if e.Capture.StoredBytes != len(stored) || *e.Capture.ObservedBytes != int64(len(raw)) || !bytes.Contains(stored, []byte(`9007199254740993`)) || !bytes.Contains(stored, []byte(`"token":123`)) {
		t.Fatalf("size or numeric precision changed: %s %+v", stored, e.Capture)
	}
	if len(e.MetadataRedactedFields) != 2 || !strings.HasPrefix(e.AgentSessionID, "redacted-id/") {
		t.Fatalf("metadata redaction missing: %+v", e)
	}
	loaded, err := Load(bytes.NewReader(log.Bytes()))
	if err != nil || loaded.Session().Capture == nil || loaded.Session().Capture.Redaction != RedactionVersion || loaded.Events()[1].Capture.StoredBytes != e.Capture.StoredBytes {
		t.Fatalf("capture metadata lost during replay: %v", err)
	}
}

func TestCaptureBoundsUTF8JSONAfterRedactionInBothProfiles(t *testing.T) {
	for _, profile := range []string{"standard", "full"} {
		t.Run(profile, func(t *testing.T) {
			settings, _ := CaptureOptions(profile, 256)
			rec, err := NewWithCapture(nil, nil, settings)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(map[string]any{"body": strings.Repeat("è🙂", 1000), "password": "synthetic-private-value"})
			e := rec.Record(Event{Payload: raw, Capture: ObservedPayload("mcp-frame", "mcp-frame", len(raw))})
			if !json.Valid(e.Payload) || !utf8.Valid(e.Payload) || len(e.Payload) > 256 || e.Capture.StoredBytes > 256 || e.Capture.Completeness != "partial" {
				t.Fatalf("invalid retained prefix: %s %+v", e.Payload, e.Capture)
			}
			changes := e.Capture.Changes
			if changes[len(changes)-1].Kind != "truncated" || profile == "standard" && changes[0].Kind != "redacted" {
				t.Fatalf("incorrect transformation order: %+v", changes)
			}
			if *e.Capture.ObservedBytes != int64(len(raw)) || e.Capture.NormalizedBytes != len(raw) {
				t.Fatal("original size reconstructed from prefix")
			}
		})
	}
}

func TestFullRetainsSourceValueWithoutRedactionButRecordsLimits(t *testing.T) {
	settings, _ := CaptureOptions("full", 0)
	rec, err := NewWithCapture(nil, nil, settings)
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"password":"synthetic-full-value"}`)
	e := rec.Record(Event{Payload: raw, Capture: ObservedPayload("mcp-frame", "mcp-frame", len(raw))})
	if !bytes.Contains(e.Payload, []byte("synthetic-full-value")) || e.Capture.Completeness != "complete" || settings.MaxPayloadBytes != 1<<20 || rec.Session().Capture.Redaction != "none" {
		t.Fatalf("full profile misrepresented: %+v", e.Capture)
	}
}

func TestCaptureDistinguishesUnknownSizeFromObservedZero(t *testing.T) {
	rec := New(nil, nil)
	unknown := rec.Record(Event{Payload: json.RawMessage(`""`)})
	zero := rec.Record(Event{Payload: json.RawMessage(`""`), Capture: ObservedPayload("text", "utf8-text", 0)})
	if unknown.Capture.ObservedBytes != nil || unknown.Capture.Completeness != "unknown" || zero.Capture.ObservedBytes == nil || *zero.Capture.ObservedBytes != 0 || zero.Capture.StoredBytes != 2 {
		t.Fatal("missing original size became zero")
	}
}

func TestCaptureRejectsInvalidSettings(t *testing.T) {
	for _, settings := range []CaptureSettings{{Profile: "other"}, {Profile: "standard", MaxPayloadBytes: 255}, {Profile: "full", MaxPayloadBytes: 17 << 20}} {
		if _, err := NewWithCapture(nil, nil, settings); err == nil {
			t.Fatalf("invalid settings accepted: %+v", settings)
		}
	}
}

func TestTextRedactionCoversHeadersAssignmentsAndPrivateKeys(t *testing.T) {
	for _, text := range []string{
		"Authorization: Bearer synthetic-token-value-1234",
		"API_KEY='synthetic assignment with spaces'",
		`{"password":"synthetic embedded json"}`,
		"-----BEGIN PRIVATE KEY-----\nsynthetic-private-material\n-----END PRIVATE KEY-----",
	} {
		clean, count := redactText(text)
		if count == 0 || strings.Contains(clean, "synthetic") {
			t.Fatalf("unredacted text: %q", clean)
		}
	}
}

func TestJSONPrefixHandlesLargeCollectionsAndKeys(t *testing.T) {
	settings, _ := CaptureOptions("standard", 256)
	rec, _ := NewWithCapture(nil, nil, settings)
	for _, value := range []any{map[string]any{strings.Repeat("key", 1000): "value", "small": true}, make([]any, 1000), strings.Repeat("\x00🙂", 1000)} {
		raw, _ := json.Marshal(value)
		e := rec.Record(Event{Payload: raw})
		if !json.Valid(e.Payload) || len(e.Payload) > 256 {
			t.Fatalf("unbounded or invalid JSON: %s", e.Payload)
		}
	}
}
