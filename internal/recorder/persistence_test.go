package recorder

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type failedSink struct{}

func (failedSink) Write([]byte) (int, error) { return 0, errors.New("synthetic disk failure") }

func TestPersistenceFailureDoesNotClaimSavedEvents(t *testing.T) {
	rec := New(nil, failedSink{})
	rec.Record(Event{Type: EventNotification})
	state := rec.Persistence()
	if !state.Enabled || state.WrittenThrough != 0 || state.Error == "" || len(rec.Events()) != 2 {
		t.Fatalf("bad persistence state: %+v", state)
	}
}

func TestLoadRecoversOnlyMalformedTailAtEOF(t *testing.T) {
	var log bytes.Buffer
	rec := New(nil, &log)
	rec.Close()
	original := log.String()
	reloaded, err := Load(strings.NewReader(original + `{"seq":`))
	if err != nil || !reloaded.Session().RecoveredTail || reloaded.Revision() != rec.Revision() {
		t.Fatalf("valid prefix lost: %v", err)
	}
	if _, err := Load(strings.NewReader(original + "broken\n")); err == nil {
		t.Fatal("corrupt complete line accepted")
	}
	if _, err := Load(strings.NewReader(original + "broken\n" + original)); err == nil {
		t.Fatal("corruption in the middle accepted")
	}
	if _, err := Load(strings.NewReader(`{"seq":`)); err == nil {
		t.Fatal("empty recovered recording accepted")
	}
}
