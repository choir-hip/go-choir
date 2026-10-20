package proxy

// Failure 14 (docs/problems/engineering-completion-path-cluster-2026-10-20.md):
// the release witness answered 502 "computer authority unavailable" for 12
// minutes after a restore and the proxy logged nothing, so which branch
// refused is unknown. Ways the log line can fail, written before the code:
//
//   - it is not written, or omits the computer, the route or the cause;
//   - it cannot tell a non-active realization from a route lookup error;
//   - it carries a credential (the request's bearer or API key).

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureProxyLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func TestComputerAuthorityFailureIsLoggedWithItsCause(t *testing.T) {
	logs := captureProxyLog(t)
	r := httptest.NewRequest(http.MethodGet, "/api/computers/computer-abc/self-development/operations/op-1/release-witness", nil)
	r.Header.Set("Authorization", "Bearer secret-api-key-value")
	logComputerAuthorityFailure(r, "computer-abc", &resolvedComputerTarget{ComputerID: "computer-abc", State: "restoring"},
		errors.New("resolve immutable ComputerVersion route slot-1: not found"))
	logComputerAuthorityFailure(r, "computer-abc", &resolvedComputerTarget{ComputerID: "computer-abc", State: "active", ComputerURL: "http://10.0.0.2:8080"}, nil)
	out := logs.String()
	for _, want := range []string{"computer=computer-abc", "release-witness", `state="restoring"`, "has_url=false",
		"resolve immutable ComputerVersion route", `state="active"`, "has_url=true", "cause=none"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret-api-key-value") || strings.Contains(out, "10.0.0.2") {
		t.Fatalf("log carries a credential or the guest address:\n%s", out)
	}
}

func TestResolveFailureWriterLogsServerErrors(t *testing.T) {
	logs := captureProxyLog(t)
	writeAutoputerResolveFailure(httptest.NewRecorder(), http.StatusBadGateway, "computer authority unavailable", errors.New("vmctl: dial refused"))
	if !strings.Contains(logs.String(), "vmctl: dial refused") {
		t.Fatalf("resolve failure not logged: %q", logs.String())
	}
}
