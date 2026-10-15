package recorder

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestV2EnvelopeRoundTripAndExplicitZero(t *testing.T) {
	var log bytes.Buffer
	rec := New(nil, &log)
	rec.Record(Event{Type: EventToolCallResponse, CallID: "stable", Source: "test", Timing: &Timing{DurationMs: 0, Source: "source-reported"}})
	loaded, err := Load(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	e := loaded.Events()[1]
	if e.SchemaVersion != 2 || e.RecordingID != rec.Session().ID || e.CallID != "stable" || e.Timing == nil || e.Timing.DurationMs != 0 {
		t.Fatalf("lost envelope: %+v", e)
	}
	b, _ := json.Marshal(e)
	if !bytes.Contains(b, []byte(`"durationMs":0`)) {
		t.Fatalf("zero omitted: %s", b)
	}
}

func TestNativeIdentifiersKeepExactTypeAndNumericPrecision(t *testing.T) {
	for _, raw := range []string{`9007199254740993`, `"9007199254740993"`, `null`, `""`} {
		id := NativeIDFromJSON(raw)
		b, err := json.Marshal(id)
		if err != nil {
			t.Fatal(err)
		}
		var decoded NativeID
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != *id {
			t.Fatalf("identifier changed: %s", b)
		}
	}
	if NativeIDFromJSON(`9007199254740993`).Type != "number" || NativeIDFromJSON(`"9007199254740993"`).Type != "string" {
		t.Fatal("native types collapsed")
	}
}

func TestReplayPreservesLegacyAndUnknownEventTypes(t *testing.T) {
	legacy := `{"seq":1,"sessionId":"old","timestamp":"2026-10-15T10:00:00Z","type":"FutureLegacyEvent","status":"ok","payload":{"uninterpreted":true}}`
	rec, err := Load(strings.NewReader(legacy))
	if err != nil {
		t.Fatal(err)
	}
	e := rec.Events()[0]
	if e.SchemaVersion != 0 || e.RecordingID != "" || e.Type != "FutureLegacyEvent" || e.Timing != nil {
		t.Fatalf("rewritten legacy observation: %+v", e)
	}
}

func TestReplayRejectsUnsupportedOrMixedRecordingSchemas(t *testing.T) {
	for _, log := range []string{
		`{"schemaVersion":3,"seq":1,"recordingId":"new"}`,
		`{"schemaVersion":2,"seq":1,"recordingId":"new","sessionId":"other"}`,
		"{\"schemaVersion\":2,\"seq\":1,\"recordingId\":\"a\"}\n{\"schemaVersion\":2,\"seq\":2,\"recordingId\":\"b\"}",
		"{\"schemaVersion\":2,\"seq\":1,\"recordingId\":\"a\"}\n{\"schemaVersion\":2,\"seq\":1,\"recordingId\":\"a\"}",
	} {
		if _, err := Load(strings.NewReader(log)); err == nil {
			t.Fatalf("accepted invalid log: %s", log)
		}
	}
}
