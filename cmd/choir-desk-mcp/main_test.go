package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestUsageExitsTwo(t *testing.T) {
	for _, args := range [][]string{nil, {"nonsense"}, {"serve"}, {"serve", "--socket", ""}} {
		var out, errw bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &errw); code != 2 || out.Len() != 0 {
			t.Errorf("%v: code %d out %q", args, code, out.String())
		}
	}
}

// The probe server answers a handshake, lists one tool and calls it, with
// JSON-RPC alone on stdout.
func TestProbeEchoSpeaksMCP(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi"}}}`,
	}, "\n") + "\n"
	var out, errw bytes.Buffer
	if code := run([]string{"probe-echo"}, strings.NewReader(in), &out, &errw); code != 0 {
		t.Fatalf("code %d: %s", code, errw.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lines: %s", len(lines), out.String())
	}
	if !strings.Contains(lines[1], `"name":"echo"`) || !strings.Contains(lines[2], `echo echo {\"text\":\"hi\"}`) {
		t.Errorf("%s", out.String())
	}
}
