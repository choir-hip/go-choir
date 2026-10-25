package capsule

import (
	"strings"
	"testing"
)

// P8 (docs/evidence/selfdev-trace-reviews-2026-10-25.md round 4): the GoEval
// and Exec refusals name the matched rule and line, never the value, and
// still refuse exactly what DetectPrivateSecrets refuses.
func TestSecretBearingRefusalNamesTheRule(t *testing.T) {
	src := "x := 1\nreq.Header.Set(\"Authorization\", \"Bearer abcdefghijklmnopqrs\")\n"
	err := secretBearingRefusal("Go source", src)
	if err == nil {
		t.Fatal("a bearer token was not refused")
	}
	msg := err.Error()
	for _, want := range []string{"secret-bearing Go source cannot produce auditable execution evidence", "authorization_bearer at line 2", "Bearer followed by", "was not run"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("refusal %q lacks %q", msg, want)
		}
	}
	if strings.Contains(msg, "abcdefghijklmnopqrs") {
		t.Fatalf("refusal carries the value: %q", msg)
	}
	if err := secretBearingRefusal("command", "go test ./..."); err != nil {
		t.Fatalf("a clean command was refused: %v", err)
	}
	if err := secretBearingRefusal("command", "curl -d password=hunter2abc x"); err == nil || !strings.Contains(err.Error(), "secret-bearing command") || !strings.Contains(err.Error(), "credential_assignment at line 1") {
		t.Fatalf("command refusal = %v", err)
	}
}
