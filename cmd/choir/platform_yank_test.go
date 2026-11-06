package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// F1: the platform yank is the operator's, on the host: it names the
// operator, reads the token from the environment (never argv), requires a
// reason, and calls only corpusd's operator route.
func TestPlatformYankCallsTheOperatorRouteWithTheOperatorsToken(t *testing.T) {
	var got *http.Request
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"changed":true}`))
	}))
	defer srv.Close()
	release := strings.Repeat("a", 64)
	var stdout, stderr bytes.Buffer
	t.Setenv("CHOIR_PLATFORM_OPERATOR_TOKEN", "")
	if code := run([]string{"platform", "yank", "--corpusd", srv.URL, "--operator", "ops", "--release", release, "--reason", "cve"}, &stdout, &stderr); code != 2 || got != nil {
		t.Fatalf("no token: exit %d, called %v", code, got != nil)
	}
	t.Setenv("CHOIR_PLATFORM_OPERATOR_TOKEN", "tok")
	if code := run([]string{"platform", "yank", "--corpusd", srv.URL, "--operator", "ops", "--release", release}, &stdout, &stderr); code != 2 || got != nil {
		t.Fatalf("no reason: exit %d", code)
	}
	if code := run([]string{"platform", "yank", "--corpusd", srv.URL, "--operator", "ops", "--release", release, "--reason", "cve", "--force-to", strings.Repeat("b", 64)}, &stdout, &stderr); code != 0 {
		t.Fatalf("yank exit %d: %s", code, stderr.String())
	}
	var sent map[string]string
	if got.URL.Path != "/internal/platform/operator/packages/yank" || got.Header.Get("X-Choir-Operator") != "ops" || got.Header.Get("X-Choir-Operator-Token") != "tok" ||
		json.Unmarshal(body, &sent) != nil || sent["release_id"] != release || sent["reason"] != "cve" || sent["forced_release"] != strings.Repeat("b", 64) {
		t.Fatalf("yank call = %s %v %s", got.URL.Path, got.Header, body)
	}
}
