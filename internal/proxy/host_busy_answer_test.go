package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

// The compute panel's Wake button and a bearer client's wake meet the same
// full host. Failure modes: the panel answering 502 (a broken computer) with
// no Retry-After; the bearer answer carrying the host's figures.
func TestComputeRecoveryOfAFullHostIsBusyNotBadGateway(t *testing.T) {
	refusal := &vmctl.RecoveryRefusalError{
		Kind:              string(vmctl.RecoveryRefusalHostMemoryPressure),
		Reason:            "host memory available 812 MiB; this computer needs 4096 MiB above the 4800 MiB floor",
		RetryAfterSeconds: 30,
	}
	rec := httptest.NewRecorder()
	writeComputeRecoveryFailure(rec, fmt.Errorf("wake: %w", refusal))
	if rec.Code != 503 || rec.Header().Get("Retry-After") != "30" {
		t.Fatalf("status %d Retry-After %q, want 503 and 30: %s", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "MiB") || !strings.Contains(strings.ToLower(rec.Body.String()), "busy") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	// Any other failure stays a 502.
	rec = httptest.NewRecorder()
	writeComputeRecoveryFailure(rec, errors.New("vmctl unreachable"))
	if rec.Code != 502 {
		t.Fatalf("other failure = %d, want 502", rec.Code)
	}
}

func TestBearerWakeOfAFullHostCarriesNoHostFigures(t *testing.T) {
	for _, kind := range []vmctl.RecoveryRefusalKind{vmctl.RecoveryRefusalHostMemoryPressure, vmctl.RecoveryRefusalHostDiskPressure} {
		// A vmctl that refuses the resolve the way production does: a 503
		// with the kind, the figures in the reason, and Retry-After.
		var resolveHits int
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/internal/vmctl/resolve" {
				http.NotFound(w, r)
				return
			}
			resolveHits++
			w.Header().Set("Retry-After", "45")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "computer recovery blocked", "kind": string(kind), "retry_after_seconds": 45,
				"reason": "host memory available 812 MiB; this computer needs 4096 MiB above the 4800 MiB floor; host state disk 3001 MiB available, under its floor (32 GiB)",
			})
		}))
		defer fake.Close()
		h := &Handler{vmctlClient: vmctl.NewClient(fake.URL)}
		// The real wake path: runAPIKeyWake, then the writer.
		_, err := h.runAPIKeyWake(resolvedComputerTarget{ComputerID: "c", UserID: "u", DesktopID: "primary", State: "hibernated"})
		if err == nil || resolveHits != 1 {
			t.Fatalf("%s: wake = %v after %d resolves, want a refusal", kind, err, resolveHits)
		}
		// The cause itself, before any writer: it is what the answer carries.
		var wake *computerWakeError
		if !errors.As(err, &wake) || strings.Contains(wake.Cause, "MiB") || strings.Contains(wake.Cause, "812") || strings.Contains(wake.Cause, "floor") {
			t.Fatalf("%s: the wake cause carries the host's figures: %+v", kind, wake)
		}
		rec := httptest.NewRecorder()
		if !writeComputerWakeFailure(rec, err) {
			t.Fatalf("%s: a wake failure was not rendered", kind)
		}
		if rec.Code != 503 || rec.Header().Get("Retry-After") != "45" {
			t.Fatalf("%s: status %d Retry-After %q", kind, rec.Code, rec.Header().Get("Retry-After"))
		}
		for _, leak := range []string{"MiB", "GiB", "floor", "812", "3001", "4096"} {
			if strings.Contains(rec.Body.String(), leak) {
				t.Fatalf("%s: the answer carries the host's figures (%q): %s", kind, leak, rec.Body.String())
			}
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "busy") {
			t.Fatalf("%s: the answer does not say the host is busy: %s", kind, rec.Body.String())
		}
	}
	// Other refusals keep their cause: it is the computer's own state.
	if cause := wakeCause(&vmctl.RecoveryRefusalError{Kind: string(vmctl.RecoveryRefusalCrashLoop), Reason: "crashed 3 times"}); !strings.Contains(cause, "crashed 3 times") {
		t.Fatalf("a crash-loop cause lost its reason: %q", cause)
	}
}
