package proxy

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// When the host has no memory or disk to wake a computer, the person gets a
// plain, prompt "busy, try again" answer with a retry hint (never a hang, and
// never the host's memory figures). Failure modes:
//  1. the answer carrying host figures ("host memory available ... MiB");
//  2. no Retry-After;
//  3. a non-503 status (a 502 or a hang reads as a broken computer).
func TestHostFullWakeRefusalIsAPlainBusyAnswer(t *testing.T) {
	for _, kind := range []string{string(vmctl.RecoveryRefusalHostMemoryPressure), string(vmctl.RecoveryRefusalHostDiskPressure)} {
		rec := httptest.NewRecorder()
		writeRecoveryRefusalResponse(rec, &vmctl.RecoveryRefusalError{
			Kind:              kind,
			Reason:            "host memory available 812 MiB; 0 MiB reserved for starts in flight; this computer needs 4096 MiB above the 4800 MiB floor",
			RetryAfterSeconds: 45,
		})
		if rec.Code != 503 {
			t.Fatalf("%s: status %d, want 503", kind, rec.Code)
		}
		if rec.Header().Get("Retry-After") != "45" {
			t.Fatalf("%s: Retry-After = %q", kind, rec.Header().Get("Retry-After"))
		}
		body := rec.Body.String()
		for _, leak := range []string{"MiB", "floor", "host memory"} {
			if strings.Contains(body, leak) {
				t.Fatalf("%s: the answer carries the host's figures (%q): %s", kind, leak, body)
			}
		}
		var parsed map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatal(err)
		}
		if msg, _ := parsed["error"].(string); !strings.Contains(strings.ToLower(msg), "busy") || !strings.Contains(strings.ToLower(msg), "try again") {
			t.Fatalf("%s: error = %q, want a plain busy, try-again sentence", kind, msg)
		}
		if parsed["kind"] != kind {
			t.Fatalf("%s: kind = %v, the client still reads it", kind, parsed["kind"])
		}
	}
	// Other refusals keep their reason: it is the person's own computer's state.
	rec := httptest.NewRecorder()
	writeRecoveryRefusalResponse(rec, &vmctl.RecoveryRefusalError{Kind: string(vmctl.RecoveryRefusalTailExcess), Reason: "recovery tail excess"})
	if !strings.Contains(rec.Body.String(), "recovery tail excess") {
		t.Fatalf("a recovery refusal lost its reason: %s", rec.Body.String())
	}
}
