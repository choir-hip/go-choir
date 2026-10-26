package agentcore

// U4 slice 3 (docs/design/u4-stack-rebase-build-2026-10-26.md, question 7 and
// failure mode D11): a rebase candidate is checked, at Freeze and again at
// materialization, to contain the applied change. Ways it can fail, written
// before the code:
//
//   1. a dropped or altered clean hunk applies -> ...RefusesADroppedHunk
//   2. the check runs on an ordinary operation, or on journalOnly settlement
//      -> ...NeverChecksAnOrdinaryOperation, ...NeverChecksAJournalOnlySettlement
//   3. a candidate built on an older base applies, with or without base-join
//      -> ...RefusesACandidateOnAnotherBase, ...RefusesWhenTheBootedBaseMoved
//   4. no receipt, a receipt that varies between runs, or a receipt for a
//      refusal -> ...PassWritesTheLineageReceiptAndFact
//   5. the owner's apply record does not name the receipt, so it never
//      reaches the master document -> ...ApplyOwnerActNamesTheReceipt
//   6. Freeze lets a bad candidate through to the owner -> ...FreezeRefuses

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type rebaseContentFixture struct {
	*rebaseCastFixture
	updaterRoot string
	assignment  types.EngineeringAssignment
}

// newRebaseContentFixture opens the rebase assignment (claim written) over the
// rebase cast fixture. P (op-a) adds frontend/public/a.txt; B' adds other.txt.
func newRebaseContentFixture(t *testing.T) *rebaseContentFixture {
	t.Helper()
	f := &rebaseContentFixture{rebaseCastFixture: newRebaseCastFixture(t)}
	f.updaterRoot = f.rt.selfdevUpdaterRoot
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatal(err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(context.Background(), plan, records); err != nil {
		t.Fatal(err)
	}
	f.assignment, err = f.s.GetEngineeringAssignment(context.Background(), f.owner, f.computer, plan.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// operation inserts a candidate operation for the assignment (or for another
// key) with a frozen bundle whose source patch is candidate on base.
func (f *rebaseContentFixture) operation(t *testing.T, id, key, base string, candidate []byte) selfdev.Operation {
	t.Helper()
	digest := releaseFrozenBundle(t, f.updaterRoot, f.computer, id, base, candidate)
	insertReleaseOperation(t, f.s, releaseOpRow{operationID: id, computerID: f.computer, state: selfdev.StateAccepted, bundleDigest: digest, releaseDigest: strings.Repeat("7", 64), idempotencyKey: key})
	op, err := f.rt.selfdevOperations.Get(context.Background(), f.computer, id)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func (f *rebaseContentFixture) ownKey() string {
	return store.EngineeringFreezeIdempotencyKey(f.assignment.AssignmentID)
}

// candidate is a patch on B' that adds a.txt (P's change) plus extra files.
func (f *rebaseContentFixture) candidate(t *testing.T, files map[string]string) []byte {
	return patchOn(t, f.repo, f.moved, files)
}

func patchOn(t *testing.T, repo, rev string, files map[string]string) []byte {
	t.Helper()
	scratch := t.TempDir()
	releaseGit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	releaseGit(t, scratch, "checkout", "-q", rev)
	for path, content := range files {
		full := filepath.Join(scratch, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, scratch, "add", "-A")
	return []byte(releaseGit(t, scratch, "diff", "--cached", "--no-color") + "\n")
}

func TestRebaseContentPassWritesTheLineageReceiptAndFact(t *testing.T) {
	f := newRebaseContentFixture(t)
	good := patchOn(t, f.repo, f.moved, map[string]string{"frontend/public/a.txt": "A\n", "frontend/public/extra.txt": "while rebasing\n"})
	op := f.operation(t, "op-r", f.ownKey(), f.moved, good)
	checked, err := f.rt.verifyRebaseContentForOperation(context.Background(), op)
	if err != nil || !checked {
		t.Fatalf("checked=%v err=%v", checked, err)
	}
	path := filepath.Join(f.updaterRoot, "rebase", "lineage", "op-r.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lineage receipt: %v", err)
	}
	var receipt rebaseLineageReceipt
	if json.Unmarshal(raw, &receipt) != nil || receipt.Schema != rebaseLineageSchema || receipt.Verdict != "pass" ||
		receipt.RebaseOf != f.opID || receipt.NoticeID != f.noticeID(1) || receipt.Base != f.moved || receipt.StackBase != f.base ||
		len(receipt.Clean) != 1 || receipt.Clean[0] != "frontend/public/a.txt" || len(receipt.Extra) != 1 || receipt.Extra[0] != "frontend/public/extra.txt" ||
		receipt.PatchDigest == "" || receipt.CandidatePatchDigest == "" {
		t.Fatalf("receipt %+v", receipt)
	}
	// Deterministic: a second pass writes the same bytes.
	if _, err := f.rt.verifyRebaseContentForOperation(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != string(raw) {
		t.Fatal("the lineage receipt varies between runs")
	}
	facts, _ := os.ReadFile(filepath.Join(f.updaterRoot, sourceProjectionFactsFile))
	if !strings.Contains(string(facts), `"kind":"source_projection_rebased"`) || !strings.Contains(string(facts), rebaseReceiptSHA256(raw)) {
		t.Fatalf("no source_projection_rebased fact naming the receipt:\n%s", facts)
	}
	if got := f.rt.rebaseReceiptSentence(op.OperationID); !strings.Contains(got, rebaseReceiptSHA256(raw)) || !strings.Contains(got, f.opID) {
		t.Fatalf("owner-act sentence %q", got)
	}
}

func TestRebaseContentApplyOwnerActNamesTheReceipt(t *testing.T) {
	f := newRebaseContentFixture(t)
	good := patchOn(t, f.repo, f.moved, map[string]string{"frontend/public/a.txt": "A\n"})
	op := f.operation(t, "op-r", f.ownKey(), f.moved, good)
	if _, err := f.rt.verifyRebaseContentForOperation(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(f.updaterRoot, "rebase", "lineage", "op-r.json"))
	body := ownerActBody(f.owner, op, types.OwnerActApply, "") + f.rt.rebaseReceiptSentence(op.OperationID)
	if !strings.Contains(body, rebaseReceiptSHA256(raw)) {
		t.Fatalf("the apply owner-act body does not name the lineage receipt:\n%s", body)
	}
	// An ordinary operation's apply record is unchanged.
	if f.rt.rebaseReceiptSentence("op-none") != "" {
		t.Fatal("a sentence for an operation with no receipt")
	}
}

func TestRebaseContentRefusesADroppedHunk(t *testing.T) {
	f := newRebaseContentFixture(t)
	dropped := patchOn(t, f.repo, f.moved, map[string]string{"frontend/public/unrelated.txt": "x\n"})
	op := f.operation(t, "op-r", f.ownKey(), f.moved, dropped)
	_, err := f.rt.verifyRebaseContentForOperation(context.Background(), op)
	var mismatch *rebaseContentMismatchError
	if !errors.As(err, &mismatch) || mismatch.Verdict.Reason != sourceproj.RebaseCleanPathDiffers ||
		!strings.HasPrefix(err.Error(), "rebase_content_mismatch") || !strings.Contains(err.Error(), "frontend/public/a.txt") {
		t.Fatalf("want rebase_content_mismatch naming a.txt, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(f.updaterRoot, "rebase", "lineage", "op-r.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a refusal wrote a lineage receipt: %v", statErr)
	}
}

func TestRebaseContentRefusesACandidateOnAnotherBase(t *testing.T) {
	f := newRebaseContentFixture(t)
	onOld := patchOn(t, f.repo, f.base, map[string]string{"frontend/public/a.txt": "A\n"})
	op := f.operation(t, "op-r", f.ownKey(), f.base, onOld)
	_, err := f.rt.verifyRebaseContentForOperation(context.Background(), op)
	var mismatch *rebaseContentMismatchError
	if !errors.As(err, &mismatch) || mismatch.Verdict.Reason != sourceproj.RebaseBaseMismatch {
		t.Fatalf("a candidate on the old base: %v", err)
	}
}

func TestRebaseContentRefusesWhenTheBootedBaseMoved(t *testing.T) {
	f := newRebaseContentFixture(t)
	good := patchOn(t, f.repo, f.moved, map[string]string{"frontend/public/a.txt": "A\n"})
	op := f.operation(t, "op-r", f.ownKey(), f.moved, good)
	// The platform moved again after the claim: B'' is booted. The candidate on
	// B' is a stale one, with or without the base join.
	newer := commitOnTop(t, f.repo, f.moved, map[string]string{"frontend/public/newer.txt": "n\n"})
	setBootedBase(t, f.rt, newer)
	_, err := f.rt.verifyRebaseContentForOperation(context.Background(), op)
	var mismatch *rebaseContentMismatchError
	if !errors.As(err, &mismatch) || mismatch.Verdict.Reason != rebaseBootedBaseMoved {
		t.Fatalf("a booted base that moved since the claim: %v", err)
	}
}

func TestRebaseContentNeverChecksAnOrdinaryOperation(t *testing.T) {
	f := newRebaseContentFixture(t)
	bad := patchOn(t, f.repo, f.moved, map[string]string{"frontend/public/unrelated.txt": "x\n"})
	// Another assignment's key: no claim names it.
	op := f.operation(t, "op-ord", store.EngineeringFreezeIdempotencyKey("assignment-ordinary"), f.moved, bad)
	checked, err := f.rt.verifyRebaseContentForOperation(context.Background(), op)
	if err != nil || checked {
		t.Fatalf("an ordinary operation was checked: checked=%v err=%v", checked, err)
	}
	// Neither is an operation with an unrelated key shape.
	op2 := f.operation(t, "op-key", "rollback:something", f.moved, bad)
	if checked, err := f.rt.verifyRebaseContentForOperation(context.Background(), op2); err != nil || checked {
		t.Fatalf("checked=%v err=%v", checked, err)
	}
}

func TestRebaseContentNeverChecksAJournalOnlySettlement(t *testing.T) {
	f := newRebaseContentFixture(t)
	bad := patchOn(t, f.repo, f.moved, map[string]string{"frontend/public/unrelated.txt": "x\n"})
	op := f.operation(t, "op-r", f.ownKey(), f.moved, bad)
	if err := f.rt.rebaseContentGate(context.Background(), op, true); err != nil {
		t.Fatalf("a journal-only settlement was checked: %v", err)
	}
	if err := f.rt.rebaseContentGate(context.Background(), op, false); err == nil {
		t.Fatal("the live path did not refuse a bad candidate")
	}
}

func TestRebaseContentFreezeRefusesBeforeTheOwnerSees(t *testing.T) {
	f := newRebaseContentFixture(t)
	rec := &types.RunRecord{OwnerID: f.owner, ComputerID: f.computer, Metadata: map[string]any{"assignment_id": f.assignment.AssignmentID, "assignment_attempt": 1}}
	bad := patchOn(t, f.repo, f.moved, map[string]string{"frontend/public/unrelated.txt": "x\n"})
	staged := &capsule.StagedRelease{SourceBaseCommit: f.moved}
	err := f.rt.verifyRebaseAtFreeze(context.Background(), rec, staged, bad)
	if err == nil || !strings.HasPrefix(err.Error(), "rebase_content_mismatch") {
		t.Fatalf("Freeze let a dropped hunk through: %v", err)
	}
	good := patchOn(t, f.repo, f.moved, map[string]string{"frontend/public/a.txt": "A\n"})
	if err := f.rt.verifyRebaseAtFreeze(context.Background(), rec, staged, good); err != nil {
		t.Fatalf("Freeze refused a correct rebase: %v", err)
	}
	// Freeze writes no receipt: the materializer's run is the authority.
	if _, statErr := os.Stat(filepath.Join(f.updaterRoot, "rebase")); !os.IsNotExist(statErr) {
		t.Fatalf("Freeze wrote under rebase/: %v", statErr)
	}
	// An ordinary assignment's Freeze is untouched.
	ordinary := &types.RunRecord{OwnerID: f.owner, ComputerID: f.computer, Metadata: map[string]any{"assignment_id": "assignment-ordinary", "assignment_attempt": 1}}
	if err := f.rt.verifyRebaseAtFreeze(context.Background(), ordinary, staged, bad); err != nil {
		t.Fatalf("an ordinary Freeze was checked: %v", err)
	}
}
