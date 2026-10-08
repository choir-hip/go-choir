package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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
