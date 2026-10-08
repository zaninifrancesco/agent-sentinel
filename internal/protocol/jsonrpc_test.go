package protocol

import "testing"

func TestParseKinds(t *testing.T) {
	cases := []struct {
		name  string
		frame string
		want  Kind
	}{
		{"request int id", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, KindRequest},
		{"request string id", `{"jsonrpc":"2.0","id":"abc","method":"tools/call","params":{"name":"x"}}`, KindRequest},
		{"notification", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, KindNotification},
		{"response", `{"jsonrpc":"2.0","id":1,"result":{}}`, KindResponse},
		{"error", `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"nope"}}`, KindError},
		{"surrounding whitespace", "  {\"jsonrpc\":\"2.0\",\"method\":\"ping\"}\r\n", KindNotification},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := Parse([]byte(c.frame))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := m.Kind(); got != c.want {
				t.Fatalf("Kind = %v, want %v", got, c.want)
			}
		})
	}
}

func TestParseRejectsNonProtocol(t *testing.T) {
	for _, frame := range []string{
		"",
		"Server listening on port 3000",
		`{"hello":"world"}`,
		`{"jsonrpc":"1.0","id":1,"method":"x"}`,
		`{"jsonrpc":"2.0"`,
		`[1,2,3]`,
	} {
		if _, err := Parse([]byte(frame)); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", frame)
		}
	}
}

func TestIDPreservedExactly(t *testing.T) {
	m, err := Parse([]byte(`{"jsonrpc":"2.0","id":12345678901234567890,"method":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.IDString(); got != "12345678901234567890" {
		t.Fatalf("id = %s", got)
	}
}

func TestCallToolDecoding(t *testing.T) {
	m, err := Parse([]byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"read_file","arguments":{"path":".env"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	p, err := m.AsCallTool()
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "read_file" || p.Arguments["path"] != ".env" {
		t.Fatalf("unexpected params: %+v", p)
	}

	r, _ := Parse([]byte(`{"jsonrpc":"2.0","id":7,"result":{"content":[{"type":"text","text":"hi"}],"isError":true}}`))
	res, err := r.AsCallToolResult()
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || len(res.Content) != 1 || res.Content[0].Text != "hi" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestNewErrorResponseRoundTrip(t *testing.T) {
	b, err := NewErrorResponse(NewStringID("r1"), CodeBlockedBySentinel, "blocked").Marshal()
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(b)
	if err != nil {
		t.Fatalf("own output must parse: %v (%s)", err, b)
	}
	if m.Kind() != KindError || m.IDString() != `"r1"` || m.Error.Code != CodeBlockedBySentinel {
		t.Fatalf("unexpected: %s", b)
	}
}
