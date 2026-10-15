package cursorhooks

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func captureInput(t *testing.T, h *Handler, input map[string]any) {
	t.Helper()
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	captureHook(t, h, string(b))
}

func TestObserverPreservesOutputSizeBeforeNormalizationAndLimits(t *testing.T) {
	var log bytes.Buffer
	rec := recorder.New(nil, &log)
	h := NewObserver(rec)
	output := strings.Repeat("è🙂", 20000) + "\nAPI_KEY=synthetic-output-credential"
	captureInput(t, h, map[string]any{"hook_event_name": EventBeforeShell, "conversation_id": "s", "command": "echo synthetic"})
	captureInput(t, h, map[string]any{"hook_event_name": EventAfterShell, "conversation_id": "s", "command": "echo synthetic", "output": output})
	e := rec.Events()[2]
	if e.Capture.ObservedBytes == nil || *e.Capture.ObservedBytes != int64(len(output)) || e.Capture.ObservedField != "output" || e.Capture.ObservedUnit != "utf8-text" || e.Capture.StoredBytes > 64<<10 || e.Capture.Completeness != "partial" {
		t.Fatalf("incorrect output coverage: %+v", e.Capture)
	}
	if bytes.Contains(log.Bytes(), []byte("synthetic-output-credential")) {
		t.Fatal("original output bypassed redaction")
	}
	if e.CallID != rec.Events()[1].CallID {
		t.Fatal("payload transformation broke pairing")
	}
}

func TestReadCaptureDistinguishesClientReductionAndAdapterOmission(t *testing.T) {
	for _, profile := range []string{"standard", "full"} {
		settings, _ := recorder.CaptureOptions(profile, 0)
		rec, _ := recorder.NewWithCapture(nil, nil, settings)
		h := NewObserver(rec)
		content := "synthetic file content"
		captureInput(t, h, map[string]any{"hook_event_name": EventBeforeRead, "conversation_id": "s", "file_path": "example.txt", "content": content, "content_bytes": 1000})
		e := rec.Events()[1]
		if *e.Capture.ObservedBytes != int64(len(content)) || *e.Capture.SourceReportedBytes != 1000 || e.Capture.SourceReportedUnit != "utf8-text" {
			t.Fatalf("upstream size lost: %+v", e.Capture)
		}
		if profile == "standard" {
			if e.Capture.Completeness != "omitted" || bytes.Contains(e.Payload, []byte(content)) {
				t.Fatalf("standard read body retained: %s", e.Payload)
			}
		} else if e.Capture.Completeness != "partial" || !bytes.Contains(e.Payload, []byte(content)) {
			t.Fatalf("full hid upstream truncation: %+v %s", e.Capture, e.Payload)
		}
	}
}

func TestClaudeFullRetainsResultWhileStandardDeclaresOmission(t *testing.T) {
	for _, profile := range []string{"standard", "full"} {
		settings, _ := recorder.CaptureOptions(profile, 0)
		rec, _ := recorder.NewWithCapture(nil, nil, settings)
		h := NewObserver(rec)
		captureInput(t, h, map[string]any{"hook_event_name": EventClaudePre, "session_id": "s", "tool_name": "Read", "tool_use_id": "read", "tool_input": map[string]string{"file_path": "example.txt"}})
		response := map[string]any{"content": "synthetic read body", "extra": "retained by full"}
		captureInput(t, h, map[string]any{"hook_event_name": EventClaudePost, "session_id": "s", "tool_name": "Read", "tool_use_id": "read", "tool_input": map[string]string{"file_path": "example.txt"}, "tool_response": response})
		e := rec.Events()[2]
		raw, _ := json.Marshal(response)
		if *e.Capture.ObservedBytes != int64(len(raw)) || e.Capture.ObservedUnit != "json-value" {
			t.Fatalf("result size mislabelled: %+v", e.Capture)
		}
		if profile == "standard" && (e.Capture.Completeness != "omitted" || bytes.Contains(e.Payload, []byte("synthetic read body"))) {
			t.Fatal("standard omission hidden")
		}
		if profile == "full" && (e.Capture.Completeness != "complete" || !bytes.Contains(e.Payload, []byte("synthetic read body")) || !bytes.Contains(e.Payload, []byte("retained by full"))) {
			t.Fatal("full result fields omitted")
		}
	}
}

func TestMetadataRedactionKeepsNativePairingAndSessionClosure(t *testing.T) {
	rec := recorder.New(nil, nil)
	h := NewObserver(rec)
	session := "sk-synthetic-private-session-0000"
	captureInput(t, h, map[string]any{"hook_event_name": EventClaudePre, "session_id": session, "tool_name": "Bash", "tool_use_id": "sk-synthetic-private-call-0000", "tool_input": map[string]string{"command": "echo synthetic"}})
	captureInput(t, h, map[string]any{"hook_event_name": EventClaudeSessionEnd, "session_id": session})
	if len(rec.Events()) != 3 || rec.Events()[2].Status != recorder.StatusInterrupted || rec.Events()[1].AgentSessionID != rec.Events()[2].AgentSessionID || rec.Events()[1].CallID != rec.Events()[2].CallID {
		t.Fatal("opaque identity broke closure")
	}
	b, _ := json.Marshal(rec.Events())
	if bytes.Contains(b, []byte(session)) || bytes.Contains(b, []byte("sk-synthetic-private-call-0000")) {
		t.Fatal("identity bypassed redaction")
	}
}
