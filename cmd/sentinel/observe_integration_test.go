package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/cursorhooks"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
	"github.com/zaninifrancesco/agent-sentinel/internal/server"
)

func TestObservationCockpitRecordsWithoutControlEndpoints(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rec := recorder.New(nil, nil)
	defer rec.Close()
	url, err := startCockpit(ctx, rec, 0, false, nil, nil, cursorhooks.NewObserver(rec), server.Config{Mode: "observe", Policy: "off"})
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(url + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	var cfg server.Config
	err = json.NewDecoder(resp.Body).Decode(&cfg)
	resp.Body.Close()
	if err != nil || cfg.Mode != "observe" || cfg.Policy != "off" || cfg.Approvals || cfg.BudgetEditable || cfg.ApprovalTimeoutSec != 0 {
		t.Fatalf("unexpected config: %+v (%v)", cfg, err)
	}
	for _, endpoint := range []string{"/api/approvals/1", "/api/budget"} {
		resp, err := client.Post(url+endpoint, "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("control endpoint %s is active: %d", endpoint, resp.StatusCode)
		}
	}
	inputs := []string{
		`{"hook_event_name":"beforeShellExecution","conversation_id":"test","command":"cat .env"}`,
		`{"hook_event_name":"afterShellExecution","conversation_id":"test","command":"cat .env","output":"synthetic only","duration":1}`,
	}
	for _, input := range inputs {
		resp, err := client.Post(url+"/api/hook", "application/json", strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("hook failed: %d", resp.StatusCode)
		}
	}
	events := rec.Events()
	if len(events) != 3 || events[2].Status != recorder.StatusOK || events[1].Decision != "" || events[1].RPCID != events[2].RPCID {
		t.Fatalf("capture incomplete or gated: %+v", events)
	}
}
