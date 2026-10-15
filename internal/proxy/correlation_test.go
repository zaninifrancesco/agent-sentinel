package proxy

import (
	"testing"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

func TestObserverScopesRPCIDsByTypeDirectionAndConnection(t *testing.T) {
	rec := recorder.New(nil, nil)
	a, b := NewInterceptor(rec), NewInterceptor(rec)
	fixed := time.Now()
	a.calls.Now = func() time.Time { return fixed }
	a.Observe(recorder.ClientToServer, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"numeric"}}`))
	a.Observe(recorder.ClientToServer, []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"string"}}`))
	a.Observe(recorder.ServerToClient, []byte(`{"jsonrpc":"2.0","id":1,"method":"sampling/createMessage"}`))
	b.Observe(recorder.ClientToServer, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"other-server"}}`))
	a.Observe(recorder.ClientToServer, []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	a.Observe(recorder.ServerToClient, []byte(`{"jsonrpc":"2.0","id":"1","result":{}}`))
	b.Observe(recorder.ServerToClient, []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	a.Observe(recorder.ServerToClient, []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	evs := rec.Events()
	for n, reqIndex := range []int{3, 2, 4, 1} {
		res, req := evs[5+n], evs[reqIndex]
		if res.CallID != req.CallID || res.ConnectionID != req.ConnectionID || res.Correlation.RequestSeq != req.Seq || *res.NativeCallID != *req.NativeCallID {
			t.Fatalf("RPC namespace collision: %+v", res)
		}
	}
	if evs[8].Timing == nil || evs[8].Timing.DurationMs != 0 || evs[8].Timing.Source != "proxy-monotonic" {
		t.Fatalf("zero timing lost: %+v", evs[8])
	}
}

func TestObserverKeepsDuplicateInflightRPCIDsAmbiguous(t *testing.T) {
	rec := recorder.New(nil, nil)
	i := NewInterceptor(rec)
	for n := 0; n < 2; n++ {
		i.Observe(recorder.ClientToServer, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"same"}}`))
	}
	for n := 0; n < 2; n++ {
		i.Observe(recorder.ServerToClient, []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}
	for _, e := range rec.Events() {
		if e.Type == recorder.EventResponse && (e.Correlation.Quality != "ambiguous" || e.Timing != nil) {
			t.Fatalf("arbitrary duration: %+v", e)
		}
	}
	// A duplicate result after completion remains an unmatched response.
	i.Observe(recorder.ServerToClient, []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	last := rec.Events()[len(rec.Events())-1]
	if last.Correlation.Quality != "unmatched" {
		t.Fatalf("duplicate consumed another call: %+v", last)
	}
}
