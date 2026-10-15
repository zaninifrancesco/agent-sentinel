package cursorhooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func TestObserverRecordsPreviouslyBlockedCallsWithoutPermissionOverrides(t *testing.T) {
	for _, agent := range []string{"cursor", "claude"} {
		t.Run(agent, func(t *testing.T) {
			rec := recorder.New(nil, nil)
			h := NewObserver(rec)
			before := `{"hook_event_name":"beforeShellExecution","conversation_id":"test","command":"cat .env"}`
			after := `{"hook_event_name":"afterShellExecution","conversation_id":"test","command":"cat .env","output":"synthetic","duration":1}`
			want := `{"permission":"allow"}`
			if agent == "claude" {
				before = `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Bash","tool_input":{"command":"cat .env"},"tool_use_id":"one"}`
				after = `{"hook_event_name":"PostToolUse","session_id":"test","tool_name":"Bash","tool_input":{"command":"cat .env"},"tool_response":{"stdout":"synthetic"},"tool_use_id":"one","duration_ms":1}`
				want = `{}`
			}
			for index, input := range []string{before, after} {
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/hook", strings.NewReader(input)))
				if rr.Code != http.StatusOK {
					t.Fatalf("hook failed: %s", rr.Body.String())
				}
				expected := want
				if index == 1 {
					expected = `{}`
				}
				if strings.TrimSpace(rr.Body.String()) != expected {
					t.Fatalf("permission override: %s, want %s", rr.Body.String(), expected)
				}
			}
			events := rec.Events()
			if len(events) != 3 || events[1].Type != recorder.EventToolCallRequest || events[2].Type != recorder.EventToolCallResponse || events[2].Status != recorder.StatusOK || events[1].RPCID != events[2].RPCID {
				t.Fatalf("call not recorded and completed: %+v", events)
			}
			for _, event := range events {
				if event.Decision != "" || event.Rule != "" || event.Risk != recorder.RiskNone {
					b, _ := json.Marshal(event)
					t.Fatalf("observer evaluated a policy: %s", b)
				}
			}
		})
	}
}
