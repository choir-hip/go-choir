package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

// TestMaterializeRefusalCarriesTypedWitness locks the guest-side typed refusal
// surface: the planner refusal must carry (kind, local, W, H) so the host can
// persist a durable recovery condition without parsing guest logs, and it must
// still satisfy errors.Is(err, projectionbase.ErrBaseRefused).
func TestMaterializeRefusalCarriesTypedWitness(t *testing.T) {
	var calls int
	server := bootTestServer(t, http.StatusOK,
		map[string]any{"computer_id": "computer-stale", "sequence": 148333, "canonical_event_head": strings.Repeat("a", 64)},
		http.StatusOK,
		map[string]any{"watermark_sequence": 13, "base_ref": strings.Repeat("b", 64)},
		&calls)
	defer server.Close()
	capability := func(ctx context.Context) (string, error) { return "test-cap", nil }

	_, _, err := materializeProjectionBaseIfNeeded(context.Background(), filepath.Join(t.TempDir(), "runtime.db"), "computer-stale", server.URL, capability, nil)
	if !errors.Is(err, projectionbase.ErrBaseRefused) {
		t.Fatalf("typed refusal must wrap ErrBaseRefused, got %v", err)
	}
	var refusal *ProjectionBaseRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want *ProjectionBaseRefusal", err)
	}
	if refusal.Kind != BootRefusalKindTailExcess {
		t.Fatalf("kind = %q, want %q", refusal.Kind, BootRefusalKindTailExcess)
	}
	if refusal.ComputerID != "computer-stale" || refusal.LocalSequence != 0 || refusal.WatermarkSequence != 13 || refusal.TargetSequence != 148333 {
		t.Fatalf("refusal witness = %+v", refusal)
	}
	if !refusal.EmptyStore || !refusal.ChainExists {
		t.Fatalf("refusal store shape = empty=%t chain=%t", refusal.EmptyStore, refusal.ChainExists)
	}
	if !strings.Contains(refusal.Error(), "recovery tail") {
		t.Fatalf("refusal error = %q", refusal.Error())
	}
}

// TestWriteBootRefusalHealthServesTypedStatus locks the /health schema the
// host parses: 503 with status=refused plus the refusal witness.
func TestWriteBootRefusalHealthServesTypedStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeBootRefusalHealth(recorder, &ProjectionBaseRefusal{
		ComputerID:        "computer-blocked",
		Kind:              BootRefusalKindTailExcess,
		Reason:            "recovery tail 423720 events exceeds 10000; publish a fresher base (local=0 W=148431 H=572151)",
		LocalSequence:     0,
		WatermarkSequence: 148431,
		TargetSequence:    572151,
		EmptyStore:        true,
		ChainExists:       true,
	})
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	var body struct {
		Status            string `json:"status"`
		Kind              string `json:"kind"`
		Reason            string `json:"reason"`
		ComputerID        string `json:"computer_id"`
		LocalSequence     uint64 `json:"local_sequence"`
		WatermarkSequence uint64 `json:"watermark_sequence"`
		TargetSequence    uint64 `json:"target_sequence"`
		EmptyStore        bool   `json:"empty_store"`
		ChainExists       bool   `json:"chain_exists"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode refusal health body: %v body=%s", err, recorder.Body.String())
	}
	if body.Status != "refused" || body.Kind != BootRefusalKindTailExcess {
		t.Fatalf("refusal health = %+v", body)
	}
	if body.ComputerID != "computer-blocked" || body.WatermarkSequence != 148431 || body.TargetSequence != 572151 || !body.EmptyStore || !body.ChainExists {
		t.Fatalf("refusal health witness = %+v", body)
	}
	if !strings.Contains(body.Reason, "publish a fresher base") {
		t.Fatalf("reason = %q", body.Reason)
	}
}

// TestStartupFailerServesTypedRefusalBeforeAndAfterServing locks O9/O20: a
// fatal startup error is published on /health as a typed refusal whether it
// happens before the server starts or while the replay gate is serving, so
// the host records the reason within one probe instead of waiting out its
// readiness deadline.
func TestStartupFailerServesTypedRefusalBeforeAndAfterServing(t *testing.T) {
	exits := make(chan int, 2)
	failer := &startupFailer{computerID: "computer-x", window: 0, exit: func(code int) { exits <- code }}

	// While serving: the replay gate is the /health handler.
	gate := &replayHealthGate{pending: true}
	failer.setGate(gate)
	failer.fatalf("autoputer: reconstruct computer event authority: %v", errors.New("tape unreadable"))
	if code := <-exits; code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	recorder := httptest.NewRecorder()
	gate.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body struct {
		Status     string `json:"status"`
		Kind       string `json:"kind"`
		Reason     string `json:"reason"`
		ComputerID string `json:"computer_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	if recorder.Code != http.StatusServiceUnavailable || body.Status != "refused" || body.Kind != BootRefusalKindStartupFailed {
		t.Fatalf("gate refusal = %d %+v", recorder.Code, body)
	}
	if body.ComputerID != "computer-x" || !strings.Contains(body.Reason, "tape unreadable") {
		t.Fatalf("gate refusal witness = %+v", body)
	}
}

// TestPrivacyKeyBootRefusalClassifiesMissingKeyOverExistingChain: a missing
// key file is a typed privacy_key_unavailable refusal only when a chain
// exists (the guest may create the key only before genesis); any other key
// error stays a generic startup failure carrying its reason.
func TestPrivacyKeyBootRefusalClassifiesMissingKeyOverExistingChain(t *testing.T) {
	missing := fmt.Errorf("privacy keyring: load guest key: %w", os.ErrNotExist)
	refusal := privacyKeyBootRefusal("computer-x", 2, missing)
	if refusal.Kind != BootRefusalKindPrivacyKeyUnavailable || !refusal.ChainExists || refusal.TargetSequence != 2 {
		t.Fatalf("missing key refusal = %+v", refusal)
	}
	if !strings.Contains(refusal.Reason, "load guest key") {
		t.Fatalf("reason = %q", refusal.Reason)
	}
	other := privacyKeyBootRefusal("computer-x", 2, errors.New("privacy keyring: guest key binding mismatch"))
	if other.Kind != BootRefusalKindStartupFailed || !strings.Contains(other.Reason, "binding mismatch") {
		t.Fatalf("other key error = %+v", other)
	}
	genesis := privacyKeyBootRefusal("computer-x", 0, missing)
	if genesis.Kind != BootRefusalKindStartupFailed {
		t.Fatalf("missing key without a chain is not a key-custody refusal: %+v", genesis)
	}
}

// TestStartupFailerExitsPromptlyAfterReadiness: once readiness was served the
// host no longer reads boot refusals, so a later fatal error restarts at once
// instead of holding the observation window.
func TestStartupFailerExitsPromptlyAfterReadiness(t *testing.T) {
	exits := make(chan int, 1)
	failer := &startupFailer{computerID: "computer-x", window: time.Hour, exit: func(code int) { exits <- code }}
	failer.setGate(&replayHealthGate{pending: false})
	done := make(chan struct{})
	go func() {
		failer.fatalf("autoputer: runtime startup refused: %v", errors.New("boom"))
		close(done)
	}()
	select {
	case code := <-exits:
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("post-readiness failure held the observation window")
	}
	<-done
}
