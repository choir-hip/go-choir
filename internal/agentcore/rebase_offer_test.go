package agentcore

// U4 slice 2 (docs/design/u4-stack-rebase-build-2026-10-26.md, slice 4 and
// failure modes 1, 11, 12): the stack top's patch P is offered to the rebase
// round without an ordinary assignment ever carrying an applied operation.
// Ways it can fail, written before the code:
//
//   1. P is mounted only when it applies cleanly, so a conflict leaves the
//      engineer with no patch and no names -> ...MountsPEvenOnConflictAndNamesThePaths
//   2. an over-bound, digest-mismatched or outside-the-tree P is mounted
//      -> ...RefusesWhatTheOrdinaryOfferRefuses
//   3. a rebase offer is written where old code reads it, so a revert shows P
//      as an ordinary carry -> ...WritesWhereOldCodeNeverReads
//   4. an ordinary assignment on an applied trajectory is offered P
//      -> TestOrdinaryAssignmentNeverCarriesAnAppliedOperation
//   5. the decision changes between a spawn and its replay -> ...IsDecidedOnce
//   6. the notice tells the engineer the patch is tested, or nothing about the
//      clean/conflict paths -> ...NoticeSaysWhatToDo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func rebaseOfferDeps(t *testing.T, deriver carriedPatchDeriver, patch capsule.CarriedPatch, base string) rebaseOfferDeps {
	t.Helper()
	return rebaseOfferDeps{
		Deriver: deriver, Root: t.TempDir(),
		SourcePatch: func(_ context.Context, operationID string) (string, []byte, error) {
			if operationID != "op-a" {
				return "", nil, errors.New("no such operation")
			}
			return base, patch.Patch, nil
		},
	}
}

func rebaseOfferInput() rebaseOfferInput {
	return rebaseOfferInput{
		OwnerID: "owner", ComputerID: "computer", AssignmentID: "assignment-r", Attempt: 1, RetryBaseRef: baseB,
		Lineage: rebaseLineage{NoticeID: types.StackRebaseNoticeID("op-a", strings.Repeat("b", 40), 1), OperationID: "op-a", BootedBase: strings.Repeat("b", 40), Attempt: 1},
	}
}

func TestRebaseOfferMountsPCleanly(t *testing.T) {
	patch := testCarriedPatch("a.go", "b.go")
	deriver := &fakeCarriedDeriver{}
	deps := rebaseOfferDeps(t, deriver, patch, strings.Repeat("a", 40))
	offer := offerRebasePatch(context.Background(), deps, rebaseOfferInput())
	if offer.Status != carriedOffered || offer.PatchSHA256 != patch.SHA256 || offer.Dir == "" || carriedPatchDirForSpawn(offer) != offer.Dir {
		t.Fatalf("offer %+v", offer)
	}
	if len(deriver.checked) != 1 || deriver.checked[0] != baseB {
		t.Fatalf("P must be checked against B': %v", deriver.checked)
	}
	if offer.Schema != rebaseOfferSchema || offer.RebaseOf != "op-a" || offer.StackBase != strings.Repeat("a", 40) {
		t.Fatalf("offer lineage %+v", offer)
	}
	if got, err := os.ReadFile(filepath.Join(offer.Dir, "source.patch")); err != nil || string(got) != string(patch.Patch) {
		t.Fatalf("the staged patch is not P: %v", err)
	}
}

func TestRebaseOfferMountsPEvenOnConflictAndNamesThePaths(t *testing.T) {
	patch := testCarriedPatch("a.go", "b.go")
	deriver := &fakeCarriedDeriver{applyErr: &capsule.CarriedPatchConflictError{Paths: []string{"a.go"}, Clean: []string{"b.go"}}}
	deps := rebaseOfferDeps(t, deriver, patch, strings.Repeat("a", 40))
	offer := offerRebasePatch(context.Background(), deps, rebaseOfferInput())
	if offer.Status != carriedOffered || offer.Dir == "" {
		t.Fatalf("a conflicting P must still be mounted: %+v", offer)
	}
	if len(offer.ConflictPaths) != 1 || offer.ConflictPaths[0] != "a.go" || len(offer.CleanPaths) != 1 || offer.CleanPaths[0] != "b.go" {
		t.Fatalf("conflict %v clean %v", offer.ConflictPaths, offer.CleanPaths)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{"rebase", "a.go", "b.go", "not trusted", "op-a"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("the notice does not say %q:\n%s", want, notice)
		}
	}
	if strings.Contains(notice, "tested") && !strings.Contains(notice, "not been") && !strings.Contains(notice, "no test") && !strings.Contains(notice, "re-run") {
		t.Fatalf("the notice must not call the patch tested:\n%s", notice)
	}
}

func TestRebaseOfferRefusesWhatTheOrdinaryOfferRefuses(t *testing.T) {
	big := capsule.CarriedPatch{Patch: []byte(strings.Repeat("x", capsule.CarriedPatchMaxBytes+1)), Files: []string{"a.go"}}
	big.SHA256 = testCarriedPatch("a.go").SHA256
	outside := testCarriedPatch("../escape.go")
	mismatch := testCarriedPatch("a.go")
	mismatch.Patch = append(mismatch.Patch, []byte("# tampered\n")...)
	for name, patch := range map[string]capsule.CarriedPatch{"over the size bound": big, "outside the source tree": outside, "digest mismatch": mismatch} {
		t.Run(name, func(t *testing.T) {
			deriver := &fakeCarriedDeriver{}
			deps := rebaseOfferDeps(t, deriver, patch, strings.Repeat("a", 40))
			offer := offerRebasePatch(context.Background(), deps, rebaseOfferInput())
			if offer.Status != carriedRefused || offer.Dir != "" || carriedPatchDirForSpawn(offer) != "" {
				t.Fatalf("offer %+v; want a refusal and no mount", offer)
			}
			if entries, _ := os.ReadDir(filepath.Join(deps.Root, "carried")); len(entries) != 1 || entries[0].Name() != "rebase-offers" {
				t.Fatalf("a refusal staged patch bytes: %v", entries)
			}
			if notice := carriedPatchNotice(offer); !strings.Contains(notice, "rebase") || !strings.Contains(notice, offer.Reason) {
				t.Fatalf("a refused rebase offer says nothing useful:\n%s", notice)
			}
		})
	}
}

func TestRebaseOfferWritesWhereOldCodeNeverReads(t *testing.T) {
	patch := testCarriedPatch("a.go")
	deps := rebaseOfferDeps(t, &fakeCarriedDeriver{}, patch, strings.Repeat("a", 40))
	in := rebaseOfferInput()
	offer := offerRebasePatch(context.Background(), deps, in)
	if offer.Status != carriedOffered {
		t.Fatalf("offer %+v", offer)
	}
	if _, err := os.Stat(carriedOfferPath(deps.Root, in.AssignmentID, in.Attempt)); !os.IsNotExist(err) {
		t.Fatalf("a rebase offer was written at the ordinary offer path: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(deps.Root, "carried", "offers")); len(entries) != 0 {
		t.Fatalf("carried/offers holds %d entries for a rebase assignment", len(entries))
	}
	if old := readCarriedOffer(deps.Root, in.AssignmentID, in.Attempt); old.Status != "" {
		t.Fatalf("old code reads a rebase offer as an ordinary carry: %+v", old)
	}
	if _, ok := decodeCarriedOffer(deps.Root, in.AssignmentID, in.Attempt); ok {
		t.Fatal("old decode finds a rebase offer")
	}
	// New code finds it again after a restart, verified.
	again := readRebaseOffer(deps.Root, in.AssignmentID, in.Attempt)
	if again.Status != carriedOffered || again.Dir != offer.Dir {
		t.Fatalf("recorded offer %+v", again)
	}
	// The directory old code lists for offers is the only thing it walks; the
	// rebase directory sits beside it, schema-tagged.
	raw, err := os.ReadFile(filepath.Join(deps.Root, "carried", "rebase-offers", "assignment-r-1.json"))
	if err != nil || !strings.Contains(string(raw), rebaseOfferSchema) {
		t.Fatalf("rebase offer file: %v %s", err, raw)
	}
}

func TestRebaseOfferIsDecidedOnce(t *testing.T) {
	patch := testCarriedPatch("a.go")
	deriver := &fakeCarriedDeriver{}
	deps := rebaseOfferDeps(t, deriver, patch, strings.Repeat("a", 40))
	first := offerRebasePatch(context.Background(), deps, rebaseOfferInput())
	// A restart replays the spawn: the recorded decision stands, even if the
	// source read would now answer differently.
	deps.SourcePatch = func(context.Context, string) (string, []byte, error) { return "", nil, errors.New("gone") }
	second := offerRebasePatch(context.Background(), deps, rebaseOfferInput())
	if second.Status != first.Status || second.PatchSHA256 != first.PatchSHA256 || second.Dir != first.Dir {
		t.Fatalf("replay changed the decision: %+v vs %+v", first, second)
	}
	if len(deriver.checked) != 1 {
		t.Fatalf("a replay re-checked P: %v", deriver.checked)
	}
}

func TestRebaseOfferAnUnreadableSourceIsRefusedNotRecorded(t *testing.T) {
	deps := rebaseOfferDeps(t, &fakeCarriedDeriver{}, testCarriedPatch("a.go"), strings.Repeat("a", 40))
	deps.SourcePatch = func(context.Context, string) (string, []byte, error) { return "", nil, errors.New("bundle unavailable") }
	in := rebaseOfferInput()
	offer := offerRebasePatch(context.Background(), deps, in)
	if offer.Status != carriedRefused || offer.Dir != "" {
		t.Fatalf("offer %+v", offer)
	}
	if _, err := os.Stat(filepath.Join(deps.Root, "carried", "rebase-offers", "assignment-r-1.json")); !os.IsNotExist(err) {
		t.Fatalf("a transient failure was recorded as the decision: %v", err)
	}
}

func TestOrdinaryAssignmentNeverCarriesAnAppliedOperation(t *testing.T) {
	// The ordinary selection: an applied operation is not carriable, so a trajectory
	// whose only round applied offers nothing. U4 does not change that.
	applied := failedRound("round-1", "capsule-subject:sha256:c1", baseA, 0)
	applied.OperationState = selfdev.StateApplied
	if roundCarriable(applied) {
		t.Fatal("an applied operation became carriable")
	}
	history := &fakeCarriedHistory{entries: []carriedCandidate{applied}}
	deps := carriedDeps(t, history, &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": testCarriedPatch("a.go")}})
	if offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA)); offer.Status != "" || offer.Dir != "" {
		t.Fatalf("an ordinary assignment was offered an applied operation: %+v", offer)
	}
	if entries, _ := os.ReadDir(filepath.Join(deps.Root, "carried")); len(entries) != 0 {
		t.Fatalf("something was written for an ordinary assignment: %v", entries)
	}
}
