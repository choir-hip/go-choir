package agentcore

// Failure 15, problem 1, option A
// (docs/problems/engineering-completion-path-cluster-2026-10-20.md): a failed
// round's work reaches the retry as an inert, digest-bound patch the engineer
// may apply in its own fresh capsule. Ways the carry can fail (written before
// the code; each has a test below):
//
//  1. No prior failed round, yet something is offered or staged.
//  2. The newest failed round is not the one offered, or an older one is
//     silently dropped (older candidates must be named).
//  3. A finished chain (newest round completed), the assignment's own
//     candidate, or a verification candidate is offered.
//  4. An empty diff stages a patch or writes a record.
//  5. An oversize patch is truncated or mounted; the retry is blocked.
//  6. A lost candidate artifact fails the spawn instead of starting clean.
//  7. A patch for a different base is applied/offered without saying so, or
//     one that cannot apply is offered.
//  8. A patch naming a path outside the source tree is staged.
//  9. Staged bytes that no longer match the digest are mounted or named.
// 10. The decision changes between a spawn and its replay (restart), or a
//     restart re-offers differently.
// 11. The desk is told nothing, or told the patch is trusted or tested.
// 12. Anything is written outside <updater root>/carried (the live
//     computer's source or incoming/).
//
// Iteration on a rejected candidate (owner, 2026-10-25: "if we, in
// self-development, get part of the work done, but not everything, we can
// iteratively improve on it rather than having to start over"; ledger
// 2026-10-25 00:5x). A finished candidate's operation state decides:
//
// 13. A finished candidate whose operation was rejected, or failed before it
//     applied, is not offered, and the next round starts from the base.
// 14. A candidate whose operation applied (an upstreamed stack is still an
//     applied operation), is awaiting approval, or is in any other open state
//     is offered: its change would be applied twice, or would race the
//     decision still pending on it. This holds for a round that reported
//     failed as well as one that finished. An open state is not silent
//     either: admission holds the request until it settles, and an offer
//     that meets one anyway is a refusal that names it and is not recorded
//     (carry_select_test.go, S1 and S6).
// 15. A finished round with no operation (never proposed) is offered.
// 16. The rejection reason is lost, unbounded, or the notice still calls the
//     work unfinished instead of saying the patch is the starting point and
//     the new directive and the reason are what to change.
// 17. An operation read that fails is remembered as the decision instead of
//     leaving the retry clean and undecided.
// 18. A reason is read from an event that is not this operation's rejection.
// 19. A moved base that conflicts is refused without naming the conflicting
//     paths, so the engineer cannot redo only those.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type fakeCarriedHistory struct {
	entries []carriedCandidate
	err     error
}

func (f *fakeCarriedHistory) CarriedCandidates(context.Context, string, string, string) ([]carriedCandidate, error) {
	return f.entries, f.err
}

type fakeCarriedDeriver struct {
	patches   map[string]capsule.CarriedPatch // by candidate ref
	deriveErr error
	applyErr  error
	derived   [][2]string
	checked   []string
}

func (f *fakeCarriedDeriver) DeriveCarriedPatch(_ context.Context, baseRef, candidateRef string) (capsule.CarriedPatch, error) {
	f.derived = append(f.derived, [2]string{baseRef, candidateRef})
	if f.deriveErr != nil {
		return capsule.CarriedPatch{}, f.deriveErr
	}
	return f.patches[candidateRef], nil
}

func (f *fakeCarriedDeriver) CheckCarriedPatchApplies(_ context.Context, _ capsule.CarriedPatch, retryBaseRef string) error {
	f.checked = append(f.checked, retryBaseRef)
	return f.applyErr
}

const (
	baseA = "capsule-source-git:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:sha256:1111"
	baseB = "capsule-source-git:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb:sha256:2222"
)

func testCarriedPatch(files ...string) capsule.CarriedPatch {
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ -1 +1 @@\n-old\n+new\n", f, f, f, f)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return capsule.CarriedPatch{Patch: []byte(b.String()), SHA256: hex.EncodeToString(sum[:]), Files: files}
}

func failedRound(id, candidate, base string, age time.Duration) carriedCandidate {
	return carriedCandidate{
		CandidateID: "candidate-" + id, CandidateRef: candidate, AssignmentID: id, Attempt: 1, BaseRef: base,
		Kind: types.EngineeringAssignmentImplementation, Result: types.EngineeringResultFailed,
		CreatedAt: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC).Add(-age),
	}
}

func carriedInput(retryBase string) carriedPatchInput {
	return carriedPatchInput{OwnerID: "owner", ComputerID: "computer", TrajectoryID: "traj", AssignmentID: "assignment-retry", Attempt: 1, RetryBaseRef: retryBase}
}

func carriedDeps(t *testing.T, history *fakeCarriedHistory, deriver *fakeCarriedDeriver) carriedPatchDeps {
	t.Helper()
	return carriedPatchDeps{History: history, Deriver: deriver, Root: filepath.Join(t.TempDir(), "updater")}
}

func TestCarriedOfferWithNoPriorRoundOffersNothing(t *testing.T) {
	deps := carriedDeps(t, &fakeCarriedHistory{}, &fakeCarriedDeriver{})
	offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if offer.Status != "" || offer.Dir != "" || carriedPatchNotice(offer) != "" {
		t.Fatalf("offer = %+v, notice %q; want nothing", offer, carriedPatchNotice(offer))
	}
	if _, err := os.Stat(filepath.Join(deps.Root, "carried")); !os.IsNotExist(err) {
		t.Fatalf("a clean retry created %s: %v", filepath.Join(deps.Root, "carried"), err)
	}
}

func TestCarriedOfferStagesTheFailedRoundsPatchReadOnly(t *testing.T) {
	patch := testCarriedPatch("internal/selfdevprobe/selfdevprobe.go", "internal/apihandler/routes.go")
	history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": patch}}
	deps := carriedDeps(t, history, deriver)
	offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if offer.Status != carriedOffered || offer.PatchSHA256 != patch.SHA256 {
		t.Fatalf("offer = %+v, want offered with the patch digest", offer)
	}
	if want := filepath.Join(deps.Root, "carried", patch.SHA256); offer.Dir != want {
		t.Fatalf("dir = %q, want content-addressed %q", offer.Dir, want)
	}
	if len(deriver.derived) != 1 || deriver.derived[0] != [2]string{baseA, "capsule-subject:sha256:c1"} {
		t.Fatalf("derived from %v; want the round's own base and candidate", deriver.derived)
	}
	if len(deriver.checked) != 0 {
		t.Fatalf("same base was apply-checked: %v", deriver.checked)
	}
	staged, err := os.ReadFile(filepath.Join(offer.Dir, "source.patch"))
	if err != nil || string(staged) != string(patch.Patch) {
		t.Fatalf("staged patch = %q, %v", staged, err)
	}
	for _, name := range []string{"source.patch", "carried.json"} {
		info, err := os.Stat(filepath.Join(offer.Dir, name))
		if err != nil || info.Mode().Perm()&0o222 != 0 {
			t.Fatalf("%s is writable or missing: %v, %v", name, info, err)
		}
	}
	if err := verifyStagedCarriedPatch(offer.Dir, patch.SHA256); err != nil {
		t.Fatalf("staged patch does not verify: %v", err)
	}
	if got := carriedPatchDirForSpawn(offer); got != offer.Dir {
		t.Fatalf("spawn dir = %q, want the staged dir", got)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{
		patch.SHA256, "internal/selfdevprobe/selfdevprobe.go", "assignment round-1", "/selfdev/carried/source.patch",
		"git apply", "not trusted", "re-run", "ignore",
	} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice does not say %q:\n%s", want, notice)
		}
	}
	for _, bad := range []string{"tests passed", "verified by"} {
		if strings.Contains(notice, bad) {
			t.Fatalf("notice claims %q:\n%s", bad, notice)
		}
	}
}

func TestCarriedOfferNewestFailedRoundWinsAndOlderAreNamed(t *testing.T) {
	newest, older := failedRound("round-3", "capsule-subject:sha256:c3", baseA, time.Hour), failedRound("round-2", "capsule-subject:sha256:c2", baseA, 2*time.Hour)
	oldest := failedRound("round-1", "capsule-subject:sha256:c1", baseA, 3*time.Hour)
	history := &fakeCarriedHistory{entries: []carriedCandidate{newest, older, oldest}}
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c3": testCarriedPatch("a.go")}}
	offer := offerCarriedPatch(context.Background(), carriedDeps(t, history, deriver), carriedInput(baseA))
	if offer.Status != carriedOffered || offer.SourceAssignmentID != "round-3" {
		t.Fatalf("offer = %+v, want the newest round", offer)
	}
	if len(deriver.derived) != 1 {
		t.Fatalf("derived %d patches, want exactly one", len(deriver.derived))
	}
	if strings.Join(offer.OlderCandidates, ",") != "candidate-round-2,candidate-round-1" {
		t.Fatalf("older = %v, want both named", offer.OlderCandidates)
	}
	if !strings.Contains(carriedPatchNotice(offer), "candidate-round-2") {
		t.Fatalf("notice does not reference the older candidates:\n%s", carriedPatchNotice(offer))
	}
}

func TestCarriedOfferIgnoresFinishedOwnAndVerificationRounds(t *testing.T) {
	finished := failedRound("round-2", "capsule-subject:sha256:c2", baseA, time.Hour)
	finished.Result = types.EngineeringResultCompleted
	older := failedRound("round-1", "capsule-subject:sha256:c1", baseA, 2*time.Hour)
	own := failedRound("assignment-retry", "capsule-subject:sha256:own", baseA, time.Minute)
	verify := failedRound("verify-1", "capsule-subject:sha256:v1", baseA, time.Minute)
	verify.Kind = types.EngineeringAssignmentVerification
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{
		"capsule-subject:sha256:c1": testCarriedPatch("a.go"), "capsule-subject:sha256:c2": testCarriedPatch("b.go"),
		"capsule-subject:sha256:own": testCarriedPatch("c.go"), "capsule-subject:sha256:v1": testCarriedPatch("d.go"),
	}}
	// The assignment's own and verification candidates are skipped; the
	// newest remaining round finished, so the chain is done and nothing older
	// than it is offered.
	history := &fakeCarriedHistory{entries: []carriedCandidate{own, verify, finished, older}}
	offer := offerCarriedPatch(context.Background(), carriedDeps(t, history, deriver), carriedInput(baseA))
	if offer.Status != "" || len(deriver.derived) != 0 {
		t.Fatalf("offer = %+v, derived %v; want nothing for a finished chain", offer, deriver.derived)
	}
}

func TestCarriedOfferEmptyDiffStagesNothing(t *testing.T) {
	history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
	deriver := &fakeCarriedDeriver{deriveErr: capsule.ErrCarriedPatchEmpty}
	deps := carriedDeps(t, history, deriver)
	offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if offer.Status != "" || offer.Dir != "" || carriedPatchNotice(offer) != "" {
		t.Fatalf("offer = %+v; want nothing for an empty diff", offer)
	}
	if _, err := os.Stat(filepath.Join(deps.Root, "carried")); !os.IsNotExist(err) {
		t.Fatalf("an empty diff wrote %s: %v", filepath.Join(deps.Root, "carried"), err)
	}
}

func TestCarriedOfferOversizeIsRefusedAndNeverMounted(t *testing.T) {
	history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
	deriver := &fakeCarriedDeriver{deriveErr: fmt.Errorf("%w: 3000000 bytes, above the 2097152 byte bound", capsule.ErrCarriedPatchTooLarge)}
	deps := carriedDeps(t, history, deriver)
	offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if offer.Status != carriedRefused || !strings.Contains(offer.Reason, "too large") || !strings.Contains(offer.Reason, "2097152") {
		t.Fatalf("offer = %+v; want a refusal that says too large and the bound", offer)
	}
	if offer.Dir != "" || carriedPatchDirForSpawn(offer) != "" {
		t.Fatalf("a refused patch has a dir: %+v", offer)
	}
	if notice := carriedPatchNotice(offer); !strings.Contains(notice, "clean") || !strings.Contains(notice, "too large") {
		t.Fatalf("refusal notice = %q; want the clean-start and the reason", notice)
	}
	if entries, _ := os.ReadDir(filepath.Join(deps.Root, "carried")); len(entries) != 1 || entries[0].Name() != "offers" {
		t.Fatalf("a refusal staged patch bytes: %v", entries)
	}
}

func TestCarriedOfferLostCandidateArtifactStartsCleanNotBlocked(t *testing.T) {
	history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
	deriver := &fakeCarriedDeriver{deriveErr: fmt.Errorf("%w: capsule-subject:sha256:c1", capsule.ErrSubjectArtifactUnavailable)}
	offer := offerCarriedPatch(context.Background(), carriedDeps(t, history, deriver), carriedInput(baseA))
	if offer.Status != carriedRefused || !strings.Contains(offer.Reason, "candidate artifact is unavailable") || offer.Dir != "" {
		t.Fatalf("offer = %+v; want a refusal naming the lost artifact", offer)
	}
	// A history read that fails is also never fatal and is not remembered.
	deps := carriedDeps(t, &fakeCarriedHistory{err: errors.New("store down")}, &fakeCarriedDeriver{})
	offer = offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if offer.Dir != "" {
		t.Fatalf("history failure produced a dir: %+v", offer)
	}
	if _, err := os.Stat(filepath.Join(deps.Root, "carried", "offers")); !os.IsNotExist(err) {
		t.Fatalf("a transient history failure was recorded as a decision: %v", err)
	}
}

func TestCarriedOfferMovedBaseIsCheckedAndNamedNeverSilent(t *testing.T) {
	patch := testCarriedPatch("a.go")
	entry := []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}
	patches := map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": patch}

	applies := &fakeCarriedDeriver{patches: patches}
	offer := offerCarriedPatch(context.Background(), carriedDeps(t, &fakeCarriedHistory{entries: entry}, applies), carriedInput(baseB))
	if offer.Status != carriedOffered || !offer.BaseMoved || len(applies.checked) != 1 || applies.checked[0] != baseB {
		t.Fatalf("moved base that applies: offer %+v checked %v", offer, applies.checked)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{"aaaaaaaaaaaa", "bbbbbbbbbbbb", "base"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("moved-base notice does not name %q:\n%s", want, notice)
		}
	}

	conflicts := &fakeCarriedDeriver{patches: patches, applyErr: errors.New("error: a.go: patch does not apply")}
	deps := carriedDeps(t, &fakeCarriedHistory{entries: entry}, conflicts)
	offer = offerCarriedPatch(context.Background(), deps, carriedInput(baseB))
	if offer.Status != carriedRefused || offer.Dir != "" || carriedPatchDirForSpawn(offer) != "" {
		t.Fatalf("moved base that conflicts: offer %+v; want a refusal and no mount", offer)
	}
	for _, want := range []string{"a.go", "aaaaaaaaaaaa", "bbbbbbbbbbbb"} {
		if !strings.Contains(offer.Reason, want) {
			t.Fatalf("refusal %q does not name %q", offer.Reason, want)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(deps.Root, "carried")); len(entries) != 1 {
		t.Fatalf("a refusal staged patch bytes: %v", entries)
	}
}

func TestCarriedOfferRefusesPathsOutsideTheSourceTree(t *testing.T) {
	for _, bad := range []string{"../escape.go", "/etc/passwd", ".git/config", "a/../../b.go"} {
		patch := testCarriedPatch(bad)
		history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
		deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": patch}}
		deps := carriedDeps(t, history, deriver)
		offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
		if offer.Status != carriedRefused || offer.Dir != "" || !strings.Contains(offer.Reason, "outside the source tree") {
			t.Fatalf("path %q: offer %+v; want a refusal naming the tree", bad, offer)
		}
		if _, err := os.Stat(filepath.Join(deps.Root, "carried", patch.SHA256)); !os.IsNotExist(err) {
			t.Fatalf("path %q: the patch was staged: %v", bad, err)
		}
	}
}

func TestCarriedOfferTamperedStagedBytesAreNeverMountedOrNamed(t *testing.T) {
	patch := testCarriedPatch("a.go")
	history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": patch}}
	deps := carriedDeps(t, history, deriver)
	first := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if first.Status != carriedOffered {
		t.Fatalf("first offer = %+v", first)
	}
	staged := filepath.Join(first.Dir, "source.patch")
	if err := os.Chmod(staged, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyStagedCarriedPatch(first.Dir, patch.SHA256); !errors.Is(err, errCarriedPatchTampered) {
		t.Fatalf("tampered bytes verify = %v, want errCarriedPatchTampered", err)
	}
	// The decision is remembered, but the bytes are re-verified when it is
	// read again: a tampered stage is refused, not mounted.
	again := readCarriedOffer(deps.Root, "assignment-retry", 1)
	if again.Status != carriedRefused || again.Dir != "" || carriedPatchDirForSpawn(again) != "" {
		t.Fatalf("tampered stage read back as %+v", again)
	}
	if strings.Contains(carriedPatchNotice(again), "git apply") {
		t.Fatalf("a tampered patch is still offered:\n%s", carriedPatchNotice(again))
	}
}

func TestCarriedOfferIsDecidedOnceAndSurvivesARestart(t *testing.T) {
	patch := testCarriedPatch("a.go")
	history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": patch}}
	deps := carriedDeps(t, history, deriver)
	first := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))

	// A replay of the same spawn (resume after a crash between spawn and
	// bind) reads the recorded decision: no second derivation, even though
	// the candidate artifact has since gone.
	restarted := carriedPatchDeps{History: &fakeCarriedHistory{}, Deriver: &fakeCarriedDeriver{deriveErr: capsule.ErrSubjectArtifactUnavailable}, Root: deps.Root}
	replay := offerCarriedPatch(context.Background(), restarted, carriedInput(baseA))
	if replay.Status != carriedOffered || replay.Dir != first.Dir || replay.PatchSHA256 != first.PatchSHA256 {
		t.Fatalf("replay = %+v, first %+v; want the same decision", replay, first)
	}
	if got := restarted.Deriver.(*fakeCarriedDeriver).derived; len(got) != 0 {
		t.Fatalf("replay re-derived: %v", got)
	}
	// Nothing in this package starts an offer on its own: the only reader of
	// the history is offerCarriedPatch, called from the assignment spawn.
	read := readCarriedOffer(deps.Root, "assignment-retry", 1)
	if read.Status != carriedOffered || read.PatchSHA256 != patch.SHA256 {
		t.Fatalf("read back = %+v", read)
	}
	if none := readCarriedOffer(deps.Root, "assignment-other", 1); none.Status != "" {
		t.Fatalf("another assignment reads %+v", none)
	}
}

func TestCarriedOfferWritesOnlyUnderItsOwnRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "updater")
	live := filepath.Join(parent, "live-source")
	if err := os.MkdirAll(filepath.Join(root, "incoming"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": testCarriedPatch("a.go")}}
	offer := offerCarriedPatch(context.Background(), carriedPatchDeps{History: history, Deriver: deriver, Root: root}, carriedInput(baseA))
	if offer.Status != carriedOffered {
		t.Fatalf("offer = %+v", offer)
	}
	for _, dir := range []string{live, filepath.Join(root, "incoming")} {
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Fatalf("%s was written: %v", dir, entries)
		}
	}
}

func TestRunContextTellsTheRetryAboutTheCarriedPatch(t *testing.T) {
	patch := testCarriedPatch("internal/x/x.go")
	history := &fakeCarriedHistory{entries: []carriedCandidate{failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)}}
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": patch}}
	deps := carriedDeps(t, history, deriver)
	in := carriedInput(baseA)
	if offer := offerCarriedPatch(context.Background(), deps, in); offer.Status != carriedOffered {
		t.Fatalf("offer = %+v", offer)
	}
	rt := &Runtime{selfdevUpdaterRoot: deps.Root}
	rec := &types.RunRecord{
		RunID: "run:assignment-retry", AgentID: "engineering:assignment-retry", AgentProfile: "engineering", AgentRole: "engineering",
		Metadata: map[string]any{"assignment_id": "assignment-retry", "assignment_attempt": 1, "assignment_kind": string(types.EngineeringAssignmentImplementation)},
	}
	got := rt.runContextForRun(rec)
	if !strings.Contains(got, patch.SHA256) || !strings.Contains(got, "/selfdev/carried/source.patch") {
		t.Fatalf("run context does not name the carried patch:\n%s", got)
	}
	// A verification assignment, or a run with no recorded offer, hears nothing.
	rec.Metadata["assignment_kind"] = string(types.EngineeringAssignmentVerification)
	if strings.Contains(rt.runContextForRun(rec), "carried") {
		t.Fatal("a verification assignment was told about a carried patch")
	}
	rec.Metadata["assignment_kind"] = string(types.EngineeringAssignmentImplementation)
	rec.Metadata["assignment_id"] = "assignment-unrelated"
	if strings.Contains(rt.runContextForRun(rec), "carried") {
		t.Fatal("an unrelated assignment was told about a carried patch")
	}
}

func finishedRound(id, candidate, base, opState, reason string, age time.Duration) carriedCandidate {
	entry := failedRound(id, candidate, base, age)
	entry.Result = types.EngineeringResultCompleted
	if opState != "" {
		entry.OperationID, entry.OperationState, entry.OperationReason = "selfdev-"+id, opState, reason
	}
	return entry
}

func TestCarriedOfferCarriesARejectedFinishedCandidateWithItsReason(t *testing.T) {
	patch := testCarriedPatch("frontend/src/apps/reader/Reader.svelte")
	rejected := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, selfdev.StateRejected, "keep the old PDF and EPUB apps beside the Reader", time.Hour)
	older := failedRound("round-1", "capsule-subject:sha256:c1", baseA, 2*time.Hour)
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c2": patch}}
	deps := carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{rejected, older}}, deriver)
	offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if offer.Status != carriedOffered || offer.SourceAssignmentID != "round-2" || offer.PatchSHA256 != patch.SHA256 {
		t.Fatalf("offer = %+v; want the rejected candidate offered", offer)
	}
	if offer.SourceOperationID != "selfdev-round-2" || offer.SourceOperationState != selfdev.StateRejected ||
		offer.SourceReason != "keep the old PDF and EPUB apps beside the Reader" {
		t.Fatalf("offer operation = %q %q %q; want the rejected operation and its reason", offer.SourceOperationID, offer.SourceOperationState, offer.SourceReason)
	}
	if strings.Join(offer.OlderCandidates, ",") != "candidate-round-1" {
		t.Fatalf("older = %v", offer.OlderCandidates)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{
		"rejected", "keep the old PDF and EPUB apps beside the Reader", "selfdev-round-2", "starting point",
		"git apply /selfdev/carried/source.patch", "new directive", "re-run", patch.SHA256,
	} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice does not say %q:\n%s", want, notice)
		}
	}
	for _, bad := range []string{"unfinished", "tests passed", "verified by"} {
		if strings.Contains(notice, bad) {
			t.Fatalf("notice says %q:\n%s", bad, notice)
		}
	}
	// Decided once: a replay reads the same reason without the history.
	replay := offerCarriedPatch(context.Background(), carriedPatchDeps{History: &fakeCarriedHistory{}, Deriver: &fakeCarriedDeriver{}, Root: deps.Root}, carriedInput(baseA))
	if replay.SourceReason != offer.SourceReason || replay.Status != carriedOffered {
		t.Fatalf("replay = %+v", replay)
	}
}

func TestCarriedOfferCarriesAFinishedCandidateWhoseOperationFailedBeforeApplying(t *testing.T) {
	failedOp := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, selfdev.StateFailed, "host build failed: vite exited 1", time.Hour)
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c2": testCarriedPatch("a.go")}}
	offer := offerCarriedPatch(context.Background(), carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{failedOp}}, deriver), carriedInput(baseA))
	if offer.Status != carriedOffered || offer.SourceOperationState != selfdev.StateFailed {
		t.Fatalf("offer = %+v; want the failed operation's candidate", offer)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{"failed before it applied", "host build failed: vite exited 1", "starting point"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice does not say %q:\n%s", want, notice)
		}
	}
}

func TestCarriedOfferNeverCarriesAnAppliedPendingOrUnproposedRound(t *testing.T) {
	notCarried := []string{
		selfdev.StateApplied, selfdev.StateAwaitingApproval, selfdev.StateAccepted, selfdev.StateMaterializing,
		selfdev.StateDegraded, selfdev.StateFrozen, selfdev.StateVerified, selfdev.StateExecuting, selfdev.StateRequested,
		selfdev.StateRollbackPending, selfdev.StateRolledBack,
	}
	open := map[string]bool{}
	for _, state := range selfdev.OpenStates {
		open[state] = true
	}
	for _, state := range notCarried {
		for _, result := range []types.EngineeringAssignmentResultKind{types.EngineeringResultCompleted, types.EngineeringResultFailed} {
			newest := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, state, "", time.Hour)
			newest.Result = result
			older := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateRejected, "older rejection", 2*time.Hour)
			deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{
				"capsule-subject:sha256:c1": testCarriedPatch("a.go"), "capsule-subject:sha256:c2": testCarriedPatch("b.go"),
			}}
			deps := carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{newest, older}}, deriver)
			offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
			if len(deriver.derived) != 0 || carriedPatchDirForSpawn(offer) != "" {
				t.Fatalf("operation %s, result %s: offer %+v derived %v; want nothing carried (and no older round reached past it)", state, result, offer, deriver.derived)
			}
			switch {
			case open[state]:
				if offer.Status != carriedRefused || !offer.SourcePending || readCarriedOffer(deps.Root, "assignment-retry", 1).Status != "" {
					t.Fatalf("open operation %s, result %s: offer %+v; want an unrecorded pending refusal", state, result, offer)
				}
			case offer.Status != "":
				t.Fatalf("settled operation %s, result %s: offer %+v; want nothing", state, result, offer)
			}
		}
	}
	// A finished round that opened no operation was never proposed: nothing.
	unproposed := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, "", "", time.Hour)
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c2": testCarriedPatch("b.go")}}
	if offer := offerCarriedPatch(context.Background(), carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{unproposed}}, deriver), carriedInput(baseA)); offer.Status != "" {
		t.Fatalf("unproposed finished round: offer %+v", offer)
	}
}

func TestCarriedOfferBoundsTheReason(t *testing.T) {
	long := strings.Repeat("é", carriedReasonMaxBytes)
	rejected := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, selfdev.StateRejected, long+"\n\tsecond line", time.Hour)
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c2": testCarriedPatch("a.go")}}
	offer := offerCarriedPatch(context.Background(), carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{rejected}}, deriver), carriedInput(baseA))
	if len(offer.SourceReason) > carriedReasonMaxBytes+len("…") || !utf8.ValidString(offer.SourceReason) || strings.ContainsAny(offer.SourceReason, "\n\t") {
		t.Fatalf("reason is %d bytes, valid %v: %q...", len(offer.SourceReason), utf8.ValidString(offer.SourceReason), offer.SourceReason[:20])
	}
	// No reason recorded: the notice says so instead of quoting nothing.
	bare := finishedRound("round-3", "capsule-subject:sha256:c3", baseA, selfdev.StateRejected, "", time.Hour)
	deriver = &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c3": testCarriedPatch("a.go")}}
	offer = offerCarriedPatch(context.Background(), carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{bare}}, deriver), carriedInput(baseA))
	if notice := carriedPatchNotice(offer); !strings.Contains(notice, "no reason could be read") {
		t.Fatalf("reasonless notice:\n%s", notice)
	}
}

func TestCarriedOfferMovedBaseConflictNamesTheConflictingPaths(t *testing.T) {
	rejected := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, selfdev.StateRejected, "keep the old apps", time.Hour)
	deriver := &fakeCarriedDeriver{
		patches:  map[string]capsule.CarriedPatch{"capsule-subject:sha256:c2": testCarriedPatch("a.go", "b.go", "c.go")},
		applyErr: &capsule.CarriedPatchConflictError{Paths: []string{"a.go", "c.go"}, Clean: []string{"b.go"}, Detail: "error: patch failed: a.go:1"},
	}
	offer := offerCarriedPatch(context.Background(), carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{rejected}}, deriver), carriedInput(baseB))
	if offer.Status != carriedRefused || offer.Dir != "" {
		t.Fatalf("offer = %+v; want a refusal", offer)
	}
	if strings.Join(offer.ConflictPaths, ",") != "a.go,c.go" {
		t.Fatalf("conflict paths = %v", offer.ConflictPaths)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{"a.go, c.go", "b.go", "keep the old apps", "rejected"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("refusal notice does not say %q:\n%s", want, notice)
		}
	}
	if strings.Contains(notice, "unfinished") {
		t.Fatalf("a rejected candidate's refusal calls it unfinished:\n%s", notice)
	}
}

type fakeCarriedOperations struct {
	ops map[string]selfdev.Operation // by idempotency key
	err error
}

func (f fakeCarriedOperations) GetByIdempotency(_ context.Context, _, key string) (selfdev.Operation, bool, error) {
	if f.err != nil {
		return selfdev.Operation{}, false, f.err
	}
	op, ok := f.ops[key]
	return op, ok, nil
}

func TestAnnotateCarriedOperationsReadsEachRoundsOwnOperation(t *testing.T) {
	entries := []carriedCandidate{
		finishedRound("round-rejected", "c1", baseA, "", "", time.Hour),
		finishedRound("round-failed", "c2", baseA, "", "", time.Hour),
		finishedRound("round-applied", "c3", baseA, "", "", time.Hour),
		finishedRound("round-none", "c4", baseA, "", "", time.Hour),
	}
	ops := fakeCarriedOperations{ops: map[string]selfdev.Operation{
		store.EngineeringFreezeIdempotencyKey("round-rejected"): {OperationID: "op-r", State: selfdev.StateRejected, DecisionEvent: "ev-r", TerminalError: "not this"},
		store.EngineeringFreezeIdempotencyKey("round-failed"):   {OperationID: "op-f", State: selfdev.StateFailed, TerminalError: "materialize failed"},
		store.EngineeringFreezeIdempotencyKey("round-applied"):  {OperationID: "op-a", State: selfdev.StateApplied},
	}}
	var asked []string
	rejection := func(_ context.Context, op selfdev.Operation) string {
		asked = append(asked, op.OperationID)
		return "owner says no"
	}
	if err := annotateCarriedOperations(context.Background(), ops, rejection, "computer", entries); err != nil {
		t.Fatal(err)
	}
	want := [][3]string{{"op-r", selfdev.StateRejected, "owner says no"}, {"op-f", selfdev.StateFailed, "materialize failed"}, {"op-a", selfdev.StateApplied, ""}, {"", "", ""}}
	for i, w := range want {
		if got := [3]string{entries[i].OperationID, entries[i].OperationState, entries[i].OperationReason}; got != w {
			t.Fatalf("entry %d = %v, want %v", i, got, w)
		}
	}
	if strings.Join(asked, ",") != "op-r" {
		t.Fatalf("rejection reason read for %v; want only the rejected operation", asked)
	}
	// A failed read is an error (the offer is then not decided), never a
	// round that looks unproposed.
	if err := annotateCarriedOperations(context.Background(), fakeCarriedOperations{err: errors.New("db down")}, rejection, "computer", entries); err == nil {
		t.Fatal("an operation read failure was swallowed")
	}
	// No operation store: rounds keep today's rule (no operation known).
	plain := []carriedCandidate{failedRound("round-1", "c1", baseA, time.Hour)}
	if err := annotateCarriedOperations(context.Background(), nil, rejection, "computer", plain); err != nil || plain[0].OperationState != "" {
		t.Fatalf("nil store: %v, %+v", err, plain[0])
	}
}

type fakeDecisionEvents struct {
	events map[string]computerevent.Event
	err    error
}

func (f fakeDecisionEvents) EventByDigest(_ context.Context, _, digest string) (computerevent.Event, bool, error) {
	if f.err != nil {
		return computerevent.Event{}, false, f.err
	}
	ev, ok := f.events[digest]
	return ev, ok, nil
}

type fakePayloads struct {
	byDigest map[string][]byte
}

func (f fakePayloads) FetchPayload(_ context.Context, _, digest string) ([]byte, error) {
	raw, ok := f.byDigest[digest]
	if !ok {
		return nil, errors.New("not found")
	}
	return raw, nil
}

func TestCarriedRejectionReasonReadsOnlyTheOperationsOwnRejection(t *testing.T) {
	reason := []byte("keep the old PDF and EPUB apps")
	ref, err := computerevent.ArtifactRefFromDigest(computerevent.DigestBytes(reason))
	if err != nil {
		t.Fatal(err)
	}
	payloads := fakePayloads{byDigest: map[string][]byte{ref.Digest().String(): reason}}
	op := selfdev.Operation{OperationID: "op-1", ComputerID: "computer", State: selfdev.StateRejected, DecisionEvent: "ev-1"}
	good := computerevent.Event{EventKind: computerevent.EventEffectRejected, ParentEventID: "op-1", OutputArtifactRefs: []string{ref.String()}}
	cases := []struct {
		name   string
		op     selfdev.Operation
		events fakeDecisionEvents
		reader computerevent.ArtifactReader
		want   string
	}{
		{"the rejection", op, fakeDecisionEvents{events: map[string]computerevent.Event{"ev-1": good}}, payloads, string(reason)},
		{"an accept event", op, fakeDecisionEvents{events: map[string]computerevent.Event{"ev-1": func() computerevent.Event { e := good; e.EventKind = computerevent.EventEffectAccepted; return e }()}}, payloads, ""},
		{"another operation's event", op, fakeDecisionEvents{events: map[string]computerevent.Event{"ev-1": func() computerevent.Event { e := good; e.ParentEventID = "op-2"; return e }()}}, payloads, ""},
		{"no output payload", op, fakeDecisionEvents{events: map[string]computerevent.Event{"ev-1": func() computerevent.Event { e := good; e.OutputArtifactRefs = nil; return e }()}}, payloads, ""},
		{"payload unavailable", op, fakeDecisionEvents{events: map[string]computerevent.Event{"ev-1": good}}, fakePayloads{}, ""},
		{"event missing", op, fakeDecisionEvents{}, payloads, ""},
		{"event read error", op, fakeDecisionEvents{err: errors.New("db")}, payloads, ""},
		{"no decision event", selfdev.Operation{OperationID: "op-1", State: selfdev.StateRejected}, fakeDecisionEvents{events: map[string]computerevent.Event{"ev-1": good}}, payloads, ""},
		{"no payload reader", op, fakeDecisionEvents{events: map[string]computerevent.Event{"ev-1": good}}, nil, ""},
	}
	for _, tc := range cases {
		if got := carriedRejectionReason(context.Background(), tc.events, tc.reader, tc.op); got != tc.want {
			t.Errorf("%s: reason = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A round that reported failed has its operation closed as failed too
// (closeEndedAssignmentSelfdevOperation): it is still an unfinished round,
// not a finished candidate that failed to apply.
func TestCarriedOfferFailedRoundWithItsClosedOperationIsStillUnfinished(t *testing.T) {
	round := failedRound("round-1", "capsule-subject:sha256:c1", baseA, time.Hour)
	round.OperationID, round.OperationState, round.OperationReason = "selfdev-round-1", selfdev.StateFailed, "assignment failed before its candidate froze"
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": testCarriedPatch("a.go")}}
	offer := offerCarriedPatch(context.Background(), carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{round}}, deriver), carriedInput(baseA))
	if offer.Status != carriedOffered || offer.SourceOperationState != "" {
		t.Fatalf("offer = %+v; want offered as an unfinished round", offer)
	}
	if notice := carriedPatchNotice(offer); !strings.Contains(notice, "unfinished") || strings.Contains(notice, "finished a candidate") {
		t.Fatalf("notice:\n%s", notice)
	}
}

// The join is the real one: the operation openFreezeCandidate opens for an
// assignment is the one annotateCarriedOperations finds for that round, in
// its real state after a rejection, and a second assignment's operation is
// not confused with the first's.
func TestAnnotateCarriedOperationsFindsTheOperationAFreezeOpened(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	authorized := func(context.Context) error { return nil }
	opened, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-iterate", "assignment-first", "first try")
	if err != nil {
		t.Fatal(err)
	}
	state := opened.State
	for _, next := range []string{selfdev.StateFrozen, selfdev.StateVerified, selfdev.StateAwaitingApproval, selfdev.StateRejected} {
		if _, err := operations.Transition(ctx, computerID, opened.OperationID, state, next, nil); err != nil {
			t.Fatalf("%s -> %s: %v", state, next, err)
		}
		state = next
	}
	entries := []carriedCandidate{
		finishedRound("assignment-first", "c1", baseA, "", "", time.Hour),
		finishedRound("assignment-second", "c2", baseA, "", "", time.Minute),
	}
	if err := annotateCarriedOperations(ctx, operations, func(context.Context, selfdev.Operation) string { return "no" }, computerID, entries); err != nil {
		t.Fatal(err)
	}
	if entries[0].OperationID != store.EngineeringFreezeOperationID(computerID, "assignment-first") || entries[0].OperationState != selfdev.StateRejected || entries[0].OperationReason != "no" {
		t.Fatalf("first round = %+v", entries[0])
	}
	if entries[1].OperationID != "" || entries[1].OperationState != "" {
		t.Fatalf("second round (no operation) = %+v", entries[1])
	}
}
