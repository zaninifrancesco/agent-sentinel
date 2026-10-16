package catalog

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func TestCatalogRebuildDedupRefreshAndOpen(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	rec := recorder.New(nil, &log)
	rec.Record(recorder.Event{Type: recorder.EventToolCallRequest, ToolName: "shell", CallID: "one", Status: recorder.StatusPending})
	path := filepath.Join(dir, "one.jsonl")
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(path, log.Bytes())
	write(filepath.Join(dir, "copy.jsonl"), log.Bytes())
	c := New(dir, "")
	items, warnings := c.Snapshot()
	if len(items) != 1 || len(warnings) != 0 || items[0].Rows[1].Request.Payload != nil {
		t.Fatalf("bad index: %v %v", items, warnings)
	}
	if _, err := c.Open("../../outside"); err == nil {
		t.Fatal("accepted a path")
	}
	if got, err := c.Open(rec.Session().ID); err != nil || got.Revision() != rec.Revision() {
		t.Fatalf("open failed: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "copy.jsonl")); err != nil {
		t.Fatal(err)
	}
	rec.Close()
	write(path, log.Bytes())
	items, _ = c.Snapshot()
	if items[0].Session.EndedAt.IsZero() {
		t.Fatal("growth not refreshed")
	}
	items, _ = New(dir, "").Snapshot()
	if len(items) != 1 || items[0].Session.ID != rec.Session().ID {
		t.Fatal("catalog did not rebuild")
	}
	write(filepath.Join(dir, "partial.jsonl"), append(append([]byte(nil), log.Bytes()...), []byte(`{"seq":`)...))
	items, warnings = c.Snapshot()
	if len(items) != 0 || len(warnings) != 1 {
		t.Fatal("conflicting copies counted twice")
	}
	if _, err := c.Open(rec.Session().ID); err == nil {
		t.Fatal("opened conflicting recording")
	}
}

func TestCatalogRecoveryAndCorruptionAreVisible(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	recorder.New(nil, &log)
	if err := os.WriteFile(filepath.Join(dir, "partial.jsonl"), append(log.Bytes(), []byte(`{"seq":`)...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.jsonl"), []byte("not json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	items, warnings := New(dir, "").Snapshot()
	if len(items) != 1 || !items[0].Session.RecoveredTail || len(warnings) != 1 {
		t.Fatalf("recovery hidden: %v %v", items, warnings)
	}
}
