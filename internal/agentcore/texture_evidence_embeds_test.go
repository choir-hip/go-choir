package agentcore

// The Freeze's side of station TE
// (docs/design/texture-evidence-embeds-te-2026-10-25.md section 2): a fresh
// Freeze that observed captures asks the Texture owner to embed them in the
// operation's document, when texture.evidence_embeds is on. Ways it could
// fail, written before the code:
//   - it asks with the switch off, or when the switch cannot be read (E13);
//   - a replayed freeze (no observation) or a freeze with no captures asks;
//   - the cell or the model can choose the operation (it must be the
//     Freeze's own result, after the runtime's observation);
//   - the ask runs on the reduction path, so a slow Texture commit holds
//     the engineering cell;
//   - no embedder bound panics or fails the Freeze;
//   - the observation key reaches the cell.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

type recordingEmbedder struct {
	mu    sync.Mutex
	calls []string
	gate  chan struct{}
}

func (e *recordingEmbedder) EmbedCandidateEvidence(_ context.Context, ownerID, computerID, selfdevComputerID, operationID string) (string, error) {
	if e.gate != nil {
		<-e.gate
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, ownerID+"|"+computerID+"|"+selfdevComputerID+"|"+operationID)
	return `{"embedded":1}`, nil
}

func (e *recordingEmbedder) got() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.calls...)
}

func TestFreshFreezeWithCapturesAsksTextureToEmbedThemWhenSwitchedOn(t *testing.T) {
	rt, s := testRuntime(t)
	computerID := rt.TextureComputerID()
	owner := "owner-evidence-embeds"
	r := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, rec: &types.RunRecord{RunID: "run-eng"}, scope: ReductionScope{
		FromRole: agentprofile.Engineering, OwnerID: owner, ComputerID: computerID,
	}}
	freeze := func(fresh bool, captures int) map[string]any {
		obs := freezeVisualObservation{Receipt: "capsule-exec:sha256:receipt"}
		for i := 0; i < captures; i++ {
			obs.Captures = append(obs.Captures, types.VisualCaptureObservation{Route: "/reader", Artifact: "artifact:sha256:" + string(rune('a'+i))})
		}
		out := map[string]any{"operation_id": "selfdev-op-1", "bundle_digest": "b1", freezeVisualKey: obs}
		if fresh {
			out[freezeSourcePathsKey] = []string{"frontend/src/App.svelte"}
		}
		return out
	}
	settle := func(out map[string]any) {
		r.settleFreezePrecommits(context.Background(), yaegikernel.StagedIntent{}, out, "assignment-1", time.Now().UTC())
		if _, leaked := out[freezeVisualKey]; leaked {
			t.Fatal("the visual observation key reached the cell")
		}
		rt.WaitEvidenceEmbeds()
	}

	// No embedder bound: nothing fails.
	settle(freeze(true, 1))

	embedder := &recordingEmbedder{}
	rt.SetTextureEvidenceEmbedder(embedder)
	settle(freeze(true, 2))
	if got := embedder.got(); len(got) != 0 {
		t.Fatalf("switch off, but Texture was asked: %v", got)
	}

	if err := s.PutComputerSetting(context.Background(), choirstore.ComputerSetting{OwnerID: owner, ComputerID: computerID, Key: choirstore.SettingTextureEvidenceEmbeds, Value: true}); err != nil {
		t.Fatal(err)
	}
	settle(freeze(false, 2)) // a replayed freeze
	settle(freeze(true, 0))  // no captures
	if got := embedder.got(); len(got) != 0 {
		t.Fatalf("a replayed freeze or one with no captures asked: %v", got)
	}
	settle(freeze(true, 2))
	if got := embedder.got(); len(got) != 1 || got[0] != owner+"|"+computerID+"|"+computerID+"|selfdev-op-1" {
		t.Fatalf("asks = %v; want one for the Freeze's operation", got)
	}

	// The ask does not hold the reduction: a blocked Texture commit does
	// not block the Freeze's settlement.
	blocked := &recordingEmbedder{gate: make(chan struct{})}
	rt.SetTextureEvidenceEmbedder(blocked)
	done := make(chan struct{})
	go func() {
		r.settleFreezePrecommits(context.Background(), yaegikernel.StagedIntent{}, freeze(true, 1), "assignment-1", time.Now().UTC())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the Freeze waited on Texture's commit")
	}
	close(blocked.gate)
	rt.WaitEvidenceEmbeds()
	if got := blocked.got(); len(got) != 1 {
		t.Fatalf("blocked asks = %v", got)
	}
}
