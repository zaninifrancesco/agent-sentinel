package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHookRemainsNeutralOnCollectorFailureOrLegacyDecision(t *testing.T) {
	for _, body := range []string{
		`{"hook_event_name":"beforeShellExecution","command":"synthetic"}`,
		`{"hook_event_name":"PreToolUse","tool_name":"Bash"}`,
	} {
		for _, reply := range []string{`{}`, `{"permission":"deny"}`, `{"permission":"ask"}`, `{"hookSpecificOutput":{"permissionDecision":"allow"}}`, `not json`} {
			t.Run(body+reply, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(reply)) }))
				defer srv.Close()
				got, err := observeHook(srv.URL, []byte(body), time.Second)
				if string(got) != string(neutralAnswer([]byte(body))) {
					t.Fatalf("collector changed hook permissions: %s", got)
				}
				if reply != `{}` && err == nil {
					t.Fatal("unexpected collector reply was not diagnosed")
				}
			})
		}
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	address := srv.URL
	srv.Close()
	got, err := observeHook(address, []byte(`{"hook_event_name":"PreToolUse"}`), 100*time.Millisecond)
	if err == nil || string(got) != "{}\n" {
		t.Fatalf("offline hook: %s, %v", got, err)
	}
}

func TestHookDeadlineCoversHeadersAndBody(t *testing.T) {
	for _, flushHeaders := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "body"}[flushHeaders], func(t *testing.T) {
			release := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if flushHeaders {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer srv.Close()
			defer close(release)
			start := time.Now()
			got, err := observeHook(srv.URL, []byte(`{"hook_event_name":"beforeShellExecution"}`), 50*time.Millisecond)
			if err == nil || string(got) != "{\"permission\":\"allow\"}\n" {
				t.Fatalf("deadline must return neutral response: %s, %v", got, err)
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("hook exceeded delivery deadline: %s", elapsed)
			}
		})
	}
}

func TestRemovedHookEnforcementOptionsFailBeforeReadingInput(t *testing.T) {
	for _, args := range [][]string{{"--offline", "deny"}, {"--offline", "allow"}, {"--mode", "enforce"}, {"--timeout", "0s"}, {"--timeout", "6s"}} {
		if code := runHook(args); code != 2 {
			t.Fatalf("%v exited %d, want migration/usage error", args, code)
		}
	}
}
