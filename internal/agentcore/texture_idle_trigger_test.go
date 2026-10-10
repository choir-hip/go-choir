package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Rerun 12 (docs/problems/m11-rerun-12-restore-refused-and-texture-reports-no-change-2026-10-10.md):
// a read-only Texture cell auto-committed a "no change" decision, and that
// commit consumed the engineering result the model had not yet read. The
// owner's document then said nothing had been built.
//
// Failure modes pinned:
//   - a read-only Texture cell commits a turn (consuming pending reports);
//   - an activation whose cells authored nothing never answers its trigger
//     (defect #5, a08defc0, returns);
//   - the answer misstates what the activation did (no_worker_needed after a
//     staged act);
//   - an activation that applied a revision commits a second, idle turn;
//   - an activation with no completed cell, or a failed one, is answered;
//   - a non-Texture activation is answered.

type recordingCellAuthorizer struct{ bodies []string }

func (a *recordingCellAuthorizer) CommitCellTextureAuthor(_ context.Context, _ *types.RunRecord, bodyJSON, _ string) (string, error) {
	a.bodies = append(a.bodies, bodyJSON)
	return `{"op":"decide"}`, nil
}

func idleTriggerTextureRun(t *testing.T) (*Runtime, *recordingCellAuthorizer, types.RunRecord) {
	t.Helper()
	rt, s := testRuntime(t)
	auth := &recordingCellAuthorizer{}
	rt.SetTextureCellAuthorizer(auth)
	const ownerID, docID = "owner-idle", "doc-idle"
	seedDurableTextureSubject(t, s, ownerID, docID)
	rec := types.RunRecord{
		RunID: "run-texture-idle", AgentID: currentTextureAgentID(docID), OwnerID: ownerID,
		ComputerID: "autoputer-test", ChannelID: docID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		State: types.RunCompleted, Metadata: map[string]any{"doc_id": docID},
	}
	return rt, auth, rec
}

func TestTextureReadOnlyCellCommitsNothing(t *testing.T) {
	deskTestWorkerBin(t)
	rt, auth, rec := idleTriggerTextureRun(t)
	rec.State = types.RunRunning
	if err := rt.store.CreateRun(context.Background(), rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{
		RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: rec.OwnerID,
		ChannelID: rec.ChannelID, ComputerID: rec.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture, RunRecord: &rec,
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Texture)
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
	if _, err := tool.Func(ctx, json.RawMessage(`{"source":"x := 1\n_ = x","timeout_ms":15000}`)); err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	if len(auth.bodies) != 0 {
		t.Fatalf("read-only cell committed a Texture turn: %v", auth.bodies)
	}
	if cells, ok := rt.takeTextureActivationCells(rec.RunID); !ok || cells.applied || cells.staged {
		t.Fatalf("cell record = %+v, %v; want one completed cell with nothing staged", cells, ok)
	}
}

func TestTextureActivationEndAnswersAnUnansweredTrigger(t *testing.T) {
	ctx := context.Background()
	t.Run("nothing staged answers no_worker_needed once", func(t *testing.T) {
		rt, auth, rec := idleTriggerTextureRun(t)
		rt.noteTextureCell(rec.RunID, nil)
		rt.noteTextureCell(rec.RunID, nil)
		if err := rt.answerIdleTextureTrigger(ctx, &rec); err != nil {
			t.Fatal(err)
		}
		if len(auth.bodies) != 1 || !strings.Contains(auth.bodies[0], `"decision_kind":"no_worker_needed"`) || !strings.Contains(auth.bodies[0], `"op":"decide"`) {
			t.Fatalf("answers = %v, want one no_worker_needed decide", auth.bodies)
		}
		if err := rt.answerIdleTextureTrigger(ctx, &rec); err != nil || len(auth.bodies) != 1 {
			t.Fatalf("second answer: %v, bodies=%d; want none", err, len(auth.bodies))
		}
	})
	t.Run("a staged act answers delegation_skipped", func(t *testing.T) {
		rt, auth, rec := idleTriggerTextureRun(t)
		rt.noteTextureCell(rec.RunID, nil)
		rt.noteTextureCell(rec.RunID, []yaegikernel.StagedIntent{{Kind: "ask"}})
		if err := rt.answerIdleTextureTrigger(ctx, &rec); err != nil {
			t.Fatal(err)
		}
		if len(auth.bodies) != 1 || !strings.Contains(auth.bodies[0], `"decision_kind":"delegation_skipped"`) {
			t.Fatalf("answers = %v, want delegation_skipped", auth.bodies)
		}
	})
	t.Run("an applied revision is not answered again", func(t *testing.T) {
		rt, auth, rec := idleTriggerTextureRun(t)
		rt.noteTextureCell(rec.RunID, []yaegikernel.StagedIntent{{Kind: yaegikernel.IntentTextureApply}})
		rt.noteTextureCell(rec.RunID, nil)
		if err := rt.answerIdleTextureTrigger(ctx, &rec); err != nil || len(auth.bodies) != 0 {
			t.Fatalf("applied activation answered: %v %v", err, auth.bodies)
		}
	})
	t.Run("no completed cell is not answered", func(t *testing.T) {
		rt, auth, rec := idleTriggerTextureRun(t)
		if err := rt.answerIdleTextureTrigger(ctx, &rec); err != nil || len(auth.bodies) != 0 {
			t.Fatalf("cell-less activation answered: %v %v", err, auth.bodies)
		}
	})
	t.Run("a failed activation is not answered", func(t *testing.T) {
		rt, auth, rec := idleTriggerTextureRun(t)
		rec.State = types.RunFailed
		rt.noteTextureCell(rec.RunID, nil)
		if err := rt.answerIdleTextureTrigger(ctx, &rec); err != nil || len(auth.bodies) != 0 {
			t.Fatalf("failed activation answered: %v %v", err, auth.bodies)
		}
	})
	t.Run("a non-Texture activation is not answered", func(t *testing.T) {
		rt, auth, rec := idleTriggerTextureRun(t)
		rec.AgentProfile, rec.AgentRole = agentprofile.Engineering, agentprofile.Engineering
		rt.noteTextureCell(rec.RunID, nil)
		if err := rt.answerIdleTextureTrigger(ctx, &rec); err != nil || len(auth.bodies) != 0 {
			t.Fatalf("engineering activation answered: %v %v", err, auth.bodies)
		}
	})
}
