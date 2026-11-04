package agentcore

// A superseded rebase (docs/problems/noop-rebase-never-clears-base-moved-2026-11-04.md):
// a rebase round whose engineer finds the new base already ships its own
// version of the applied change freezes no source change. Ways it can fail,
// written before the code:
//
//   1. the round still has no outcome: Freeze refuses the empty candidate
//      -> ...FreezeAdmitsTheEmptyCandidate
//   2. an ordinary assignment's empty Freeze is admitted (failure 15 comes
//      back) -> ...OrdinaryEmptyFreezeIsNotARebase
//   3. an empty candidate drops a path the new base never took over and is
//      admitted -> ...RefusesAChangeTheBaseNeverTookOver (Freeze and apply)
//   4. the materializer refuses it (candidate_does_not_apply), or admits one
//      whose base it cannot prove -> ...ApplyGatePassesAndWritesTheReceipt,
//      ...ApplyGateRefusesAnUnprovableBase
//   5. the owner's apply record reads like an ordinary rebase ("still
//      contains that change"), so the drop is approved unseen
//      -> ...ApplyGatePassesAndWritesTheReceipt
//   6. once applied the stack stays stack_base_moved and Casts stay refused
//      -> ...AppliedLeavesTheStackOnTheBase
//   7. (review r1) Freeze and apply admit on different rules, a mixed verdict
//      (some paths replaced, some already carried) under-reports what is
//      dropped, and the effective tree errors after the apply
//      -> ...MixedVerdictNamesEveryDroppedPath, ...AppliedLeavesTheStackOnTheBase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// supersedingBaseChange is a moved base that ships its own a.txt (the stack
// adds one) and its own long.txt line 5 (the stack edits that line): it took
// over every path of stackChange.
func supersedingBaseChange() map[string]string {
	return map[string]string{"frontend/public/a.txt": "PLATFORM A\n", "frontend/public/long.txt": longText(map[int]string{5: "platform line 05"})}
}

func (f *rebaseContentFixture) rebaseRun() *types.RunRecord {
	return &types.RunRecord{OwnerID: f.owner, ComputerID: f.computer, Metadata: map[string]any{"assignment_id": f.assignment.AssignmentID, "assignment_attempt": 1}}
}

// reusedOn is the staged release of a capsule pinned to base that changed no
// source: no patch, and the running binary bound to base.
func reusedOn(base string) *capsule.StagedRelease {
	return &capsule.StagedRelease{Binary: &capsule.ReleaseBinaryProvenance{Origin: capsule.ReleaseBinaryReusedRunning, SHA256: strings.Repeat("1", 64), SourceBaseCommit: base}}
}

// emptyOperation inserts the round's candidate operation whose bundle has no
// source patch and the given release binary.
func (f *rebaseContentFixture) emptyOperation(t *testing.T, id string, binary *capsule.ReleaseBinaryProvenance, state string) selfdev.Operation {
	t.Helper()
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: f.computer, BaseEventHead: strings.Repeat("a", 64),
		TrajectoryRef: "trajectory-" + id, CapsuleIdentity: "capsule-" + id, ReleaseBinary: binary,
		VerifierReceipts: []string{strings.Repeat("f", 64)},
	}
	var err error
	if bundle.ContentDigest, err = bundle.ComputeContentDigest(); err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	digest := computerevent.DigestBytes(raw)
	dir := filepath.Join(f.updaterRoot, "incoming", digest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), raw, 0o400); err != nil {
		t.Fatal(err)
	}
	release := strings.Repeat("7", 64)
	if state == selfdev.StateApplied {
		release = writeCurrentRelease(t, f.updaterRoot, f.computer, "selfdev-"+id, nil)
	}
	insertReleaseOperation(t, f.s, releaseOpRow{operationID: id, computerID: f.computer, state: state, bundleDigest: digest, releaseDigest: release, idempotencyKey: f.ownKey()})
	op, err := f.rt.selfdevOperations.Get(context.Background(), f.computer, id)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func TestSupersededRebaseFreezeAdmitsTheEmptyCandidate(t *testing.T) {
	f := newRebaseContentFixtureOn(t, supersedingBaseChange())
	got, err := f.rt.verifyRebaseAtFreeze(context.Background(), f.rebaseRun(), reusedOn(f.moved), nil)
	if err != nil {
		t.Fatalf("Freeze refused a superseded rebase: %v", err)
	}
	if !got.Rebase || !got.SupersededPass || strings.Join(got.Superseded, ",") != "frontend/public/a.txt,frontend/public/long.txt" {
		t.Fatalf("verdict %+v", got)
	}
	// Freeze writes nothing: the materializer is the authority.
	if _, statErr := os.Stat(filepath.Join(f.updaterRoot, "rebase")); !os.IsNotExist(statErr) {
		t.Fatalf("Freeze wrote under rebase/: %v", statErr)
	}
	// A binary not bound to B' (a capsule build, or another base) proves no base.
	for name, staged := range map[string]*capsule.StagedRelease{
		"capsule-built": {Binary: &capsule.ReleaseBinaryProvenance{Origin: capsule.ReleaseBinaryCapsuleBuilt, SHA256: strings.Repeat("1", 64)}},
		"old base":      reusedOn(f.base),
		"no binary":     {},
	} {
		_, err := f.rt.verifyRebaseAtFreeze(context.Background(), f.rebaseRun(), staged, nil)
		var mismatch *rebaseContentMismatchError
		if !errors.As(err, &mismatch) || mismatch.Verdict.Reason != sourceproj.RebaseBaseMismatch {
			t.Fatalf("%s: want base_mismatch, got %v", name, err)
		}
	}
}

func TestSupersededRebaseOrdinaryEmptyFreezeIsNotARebase(t *testing.T) {
	f := newRebaseContentFixtureOn(t, supersedingBaseChange())
	ordinary := &types.RunRecord{OwnerID: f.owner, ComputerID: f.computer, Metadata: map[string]any{"assignment_id": "assignment-ordinary", "assignment_attempt": 1}}
	got, err := f.rt.verifyRebaseAtFreeze(context.Background(), ordinary, reusedOn(f.moved), nil)
	if err != nil || got.Rebase || len(got.Superseded) != 0 {
		t.Fatalf("an ordinary empty Freeze was treated as a rebase: %+v %v", got, err)
	}
	// Freeze then refuses it with failure 15's text.
	if err := freezeSourceChangeError(reusedOn(f.moved)); err == nil || !strings.Contains(err.Error(), "changed no /workspace/platform source") {
		t.Fatalf("failure 15 refusal: %v", err)
	}
}

func TestSupersededRebaseRefusesAChangeTheBaseNeverTookOver(t *testing.T) {
	// The standard topology: the moved base never touched a.txt, so dropping
	// the stack drops a change nobody superseded.
	f := newRebaseContentFixture(t)
	_, err := f.rt.verifyRebaseAtFreeze(context.Background(), f.rebaseRun(), reusedOn(f.moved), nil)
	var mismatch *rebaseContentMismatchError
	if !errors.As(err, &mismatch) || mismatch.Verdict.Reason != sourceproj.RebaseCleanPathDiffers ||
		!strings.Contains(err.Error(), "drops applied change "+f.opID) || !strings.Contains(err.Error(), "frontend/public/a.txt") {
		t.Fatalf("Freeze admitted a drop of a change the base never took over: %v", err)
	}
	op := f.emptyOperation(t, "op-r", reusedOn(f.moved).Binary, selfdev.StateAccepted)
	checked, err := f.rt.verifyRebaseContentForOperation(context.Background(), op)
	if !checked || !errors.As(err, &mismatch) || mismatch.Verdict.Reason != sourceproj.RebaseCleanPathDiffers {
		t.Fatalf("the apply gate admitted it: checked=%v %v", checked, err)
	}
	if _, statErr := os.Stat(filepath.Join(f.updaterRoot, "rebase", "lineage", "op-r.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a refusal wrote a lineage receipt: %v", statErr)
	}
}

func TestSupersededRebaseApplyGatePassesAndWritesTheReceipt(t *testing.T) {
	f := newRebaseContentFixtureOn(t, supersedingBaseChange())
	op := f.emptyOperation(t, "op-r", reusedOn(f.moved).Binary, selfdev.StateAccepted)
	if err := f.rt.rebaseContentGate(context.Background(), op, false); err != nil {
		t.Fatalf("the apply gate refused a superseded rebase: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(f.updaterRoot, "rebase", "lineage", "op-r.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt rebaseLineageReceipt
	if json.Unmarshal(raw, &receipt) != nil || receipt.Verdict != rebaseVerdictSuperseded || receipt.RebaseOf != f.opID ||
		receipt.Base != f.moved || strings.Join(receipt.Superseded, ",") != "frontend/public/a.txt,frontend/public/long.txt" {
		t.Fatalf("receipt %s", raw)
	}
	facts, _ := os.ReadFile(filepath.Join(f.updaterRoot, sourceProjectionFactsFile))
	if !strings.Contains(string(facts), `"kind":"source_projection_rebased"`) || !strings.Contains(string(facts), `"state":"superseded"`) {
		t.Fatalf("no superseded fact:\n%s", facts)
	}
	sentence := f.rt.rebaseReceiptSentence(op.OperationID)
	if !strings.Contains(sentence, "drops applied change "+f.opID) || !strings.Contains(sentence, rebaseReceiptSHA256(raw)) ||
		strings.Contains(sentence, "still contains") || !strings.Contains(sentence, "frontend/public/a.txt") {
		t.Fatalf("the owner's apply record does not say the change is dropped: %q", sentence)
	}
}

func TestSupersededRebaseApplyGateRefusesAnUnprovableBase(t *testing.T) {
	f := newRebaseContentFixtureOn(t, supersedingBaseChange())
	op := f.emptyOperation(t, "op-r", &capsule.ReleaseBinaryProvenance{Origin: capsule.ReleaseBinaryCapsuleBuilt, SHA256: strings.Repeat("1", 64)}, selfdev.StateAccepted)
	_, err := f.rt.verifyRebaseContentForOperation(context.Background(), op)
	var mismatch *rebaseContentMismatchError
	if !errors.As(err, &mismatch) || mismatch.Verdict.Reason != sourceproj.RebaseBaseMismatch {
		t.Fatalf("want base_mismatch, got %v", err)
	}
}

func TestSupersededRebaseAppliedLeavesTheStackOnTheBase(t *testing.T) {
	f := newRebaseContentFixtureOn(t, supersedingBaseChange())
	ctx := context.Background()
	if err := f.rt.sourceProjectionCastGate(ctx); err == nil || !strings.HasPrefix(err.Error(), string(sourceproj.StateBaseMoved)) {
		t.Fatalf("fixture: the stack must start on a moved base: %v", err)
	}
	f.emptyOperation(t, "op-r", reusedOn(f.moved).Binary, selfdev.StateApplied)
	result, err := f.rt.ReconcileSourceProjection(ctx, "apply")
	if err != nil || result.State != sourceproj.StateBase || result.Head != f.moved {
		t.Fatalf("after the superseded apply: %+v %v", result, err)
	}
	if err := f.rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("an ordinary Cast is still refused: %v", err)
	}
	// The effective tree is the booted base's, not a tree error.
	op, err := f.rt.selfdevOperations.Get(ctx, f.computer, "op-r")
	if err != nil {
		t.Fatal(err)
	}
	entry := f.rt.computeEffectiveTree(ctx, op.ReleaseDigest, "selfdev-op-r", "")
	if entry.treeError != "" || entry.treeHash != releaseGit(t, f.repo, "rev-parse", f.moved+"^{tree}") || entry.sourceBaseCommit != f.moved || entry.operationID != "op-r" {
		t.Fatalf("effective tree after a superseded apply = %+v", entry)
	}
}

func TestSupersededRebaseMixedVerdictNamesEveryDroppedPath(t *testing.T) {
	// The moved base ships its own a.txt and already carries the stack's
	// long.txt line 5 (plus an edit of its own, so the stack is not upstreamed).
	f := newRebaseContentFixtureOn(t, map[string]string{
		"frontend/public/a.txt":    "PLATFORM A\n",
		"frontend/public/long.txt": longText(map[int]string{5: "stack line 05", 35: "platform line 35"}),
	})
	got, err := f.rt.verifyRebaseAtFreeze(context.Background(), f.rebaseRun(), reusedOn(f.moved), nil)
	if err != nil || !got.SupersededPass || strings.Join(got.Superseded, ",") != "frontend/public/a.txt,frontend/public/long.txt" {
		t.Fatalf("Freeze on a mixed superseded verdict: %+v %v", got, err)
	}
	op := f.emptyOperation(t, "op-r", reusedOn(f.moved).Binary, selfdev.StateAccepted)
	if err := f.rt.rebaseContentGate(context.Background(), op, false); err != nil {
		t.Fatalf("the apply gate refused it: %v", err)
	}
	sentence := f.rt.rebaseReceiptSentence(op.OperationID)
	if !strings.Contains(sentence, "replaces it at: frontend/public/a.txt") || !strings.Contains(sentence, "already carries it at: frontend/public/long.txt") {
		t.Fatalf("the owner's record under-reports the drop: %q", sentence)
	}
}
