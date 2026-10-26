package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// U4 slice 2 (docs/design/u4-stack-rebase-build-2026-10-26.md, question 5 and
// failure modes D1, D2a, D5, D6, D9): the store's records admission binds one
// rebase notice to one assignment inside the open's own transaction. Ways it
// can fail, written first:
//   - management names a notice but sends unrelated work (objective without
//     the notice id, spec rebase_of or base that differs from the notice);
//   - a forged notice, or a notice in another Texture's inbox (D5);
//   - the root's own Cast does not carry the notice (the root never made it);
//   - a superseded attempt opens work (the latest attempt only);
//   - a notice from before a crash boot is acted on after it (D6);
//   - two opens race on one notice and both commit (D9);
//   - a pair that is already claimed opens a second assignment;
//   - the runtime did not admit a rebase spec yet the store binds a claim, or
//     the reverse;
//   - a refused admission leaves a Cast, a round, a row or a claim behind.

const (
	rebaseAdmOp   = "op-a"
	rebaseAdmBase = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type rebaseAdmission struct {
	s    *Store
	seed EngineeringAssignmentSeed
	open types.OpenEngineeringAssignmentRequest
}

func rebaseAdmNoticeID(n int) string { return types.StackRebaseNoticeID(rebaseAdmOp, rebaseAdmBase, n) }

// newRebaseAdmission seeds the records authority with the root's Cast naming
// attempt 1, commits the notice into the root's inbox, and builds the open
// for assignment 0 with management's bound Cast.
func newRebaseAdmission(t *testing.T, count int) *rebaseAdmission {
	t.Helper()
	s := openTestStore(t)
	body := `{"kind":"execution_request","rebase_notice":"` + rebaseAdmNoticeID(1) + `"}`
	seed, err := SeedEngineeringAssignmentAuthorityWithCastBody(s, inboxTestOwner, inboxTestComputer, "master", count, body)
	if err != nil {
		t.Fatal(err)
	}
	a := &rebaseAdmission{s: s, seed: seed}
	a.commitNotice(t, 1, seed.TextureInbox)
	a.open = a.openFor(t, 0, rebaseAdmNoticeID(1))
	return a
}

func (a *rebaseAdmission) commitNotice(t *testing.T, attempt int, inbox string) {
	t.Helper()
	rec, ok := types.NewStackRebaseNotice(types.StackRebaseNotice{
		TextureInbox: inbox, ComputerID: inboxTestComputer, OperationID: rebaseAdmOp, BootedBase: rebaseAdmBase,
		StackBase: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Attempt: attempt, DocID: strings.TrimPrefix(inbox, "texture:"),
		OwnerRetry: attempt > types.RebaseNoticeMaxAttempts,
	})
	if !ok {
		t.Fatal("notice did not build")
	}
	mustCommitInbox(t, a.s, rec.RecordID, nil, rec)
}

// openFor builds the open of seed assignment index whose Cast names notice.
func (a *rebaseAdmission) openFor(t *testing.T, index int, noticeID string) types.OpenEngineeringAssignmentRequest {
	t.Helper()
	open := SeedEngineeringOpenRequest(a.seed, index, "cap-"+a.seed.AssignmentIDs[index])
	cast := &open.Records.Records[0]
	cast.Directive.Body = `{"parent":"` + a.seed.TextureCastID + `","rebase_notice":"` + noticeID + `","rebase_of":"` + rebaseAdmOp + `","base":"` + rebaseAdmBase + `"}`
	cast.Directive.Objective = "Rebase the applied change op-a onto the platform base (rebase notice " + noticeID + ")"
	open.Rebase = &types.RebaseAdmission{BootedAt: time.Now().UTC().Add(-time.Hour)}
	return a.seal(open)
}

func (a *rebaseAdmission) seal(open types.OpenEngineeringAssignmentRequest) types.OpenEngineeringAssignmentRequest {
	open.Records.CommandDigest, _ = ComputeCommitInboxRecordsDigest(open.Records)
	open.CommandDigest, _ = ComputeOpenEngineeringAssignmentDigest(open)
	return open
}

func (a *rebaseAdmission) claim(t *testing.T) (types.StackRebaseClaim, error) {
	t.Helper()
	return a.s.GetRebaseNoticeClaim(context.Background(), inboxTestOwner, inboxTestComputer, types.StackRebasePairKey(rebaseAdmOp, rebaseAdmBase))
}

// requireNothingCommitted pins "a failed admission writes nothing".
func (a *rebaseAdmission) requireNothingCommitted(t *testing.T, open types.OpenEngineeringAssignmentRequest) {
	t.Helper()
	if _, err := a.claim(t); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a refused open left a claim: %v", err)
	}
	if _, err := a.s.GetEngineeringAssignment(context.Background(), inboxTestOwner, inboxTestComputer, open.AssignmentID, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a refused open left an assignment: %v", err)
	}
	if rec, err := a.s.GetCommitmentRecord(context.Background(), inboxTestOwner, inboxTestComputer, open.Binding.ParentControlID); err != nil || rec != nil {
		t.Fatalf("a refused open left management's Cast: %v %v", rec, err)
	}
}

func TestRebaseAdmissionOpensOnceAndClaimsThePair(t *testing.T) {
	a := newRebaseAdmission(t, 1)
	res, err := a.s.OpenEngineeringAssignment(context.Background(), a.open)
	if err != nil {
		t.Fatalf("a bound rebase Cast must open: %v", err)
	}
	claim, err := a.claim(t)
	if err != nil {
		t.Fatal(err)
	}
	if claim.AssignmentID != a.open.AssignmentID || claim.NoticeID != rebaseAdmNoticeID(1) || claim.Attempt != 1 ||
		claim.OperationID != rebaseAdmOp || claim.BootedBase != rebaseAdmBase {
		t.Fatalf("claim %+v", claim)
	}
	if res.Assignment.Binding.Kind != types.EngineeringAssignmentImplementation {
		t.Fatalf("a rebase round is an ordinary implementation assignment, got %s", res.Assignment.Binding.Kind)
	}
	// The same command replays: no second claim, no error.
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), a.open); err != nil {
		t.Fatalf("replay: %v", err)
	}
}

func TestRebaseAdmissionRefusesWhatIsNotBoundToTheNotice(t *testing.T) {
	cases := map[string]func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest){
		"no notice record": func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest) {
			open.Records.Records[0].Directive.Body = strings.ReplaceAll(open.Records.Records[0].Directive.Body, rebaseAdmNoticeID(1), rebaseAdmNoticeID(2))
			open.Records.Records[0].Directive.Objective = strings.ReplaceAll(open.Records.Records[0].Directive.Objective, rebaseAdmNoticeID(1), rebaseAdmNoticeID(2))
		},
		"objective without the notice id (D2a)": func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest) {
			open.Records.Records[0].Directive.Objective = "Add a dark mode toggle to settings"
		},
		"spec rebase_of differs (D2a)": func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest) {
			open.Records.Records[0].Directive.Body = strings.Replace(open.Records.Records[0].Directive.Body, `"rebase_of":"`+rebaseAdmOp+`"`, `"rebase_of":"op-other"`, 1)
		},
		"spec base differs (D2a)": func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest) {
			open.Records.Records[0].Directive.Body = strings.Replace(open.Records.Records[0].Directive.Body, `"base":"`+rebaseAdmBase+`"`, `"base":"cccccccccccccccccccccccccccccccccccccccc"`, 1)
		},
		"spec omits the operation": func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest) {
			open.Records.Records[0].Directive.Body = strings.Replace(open.Records.Records[0].Directive.Body, `"rebase_of":"`+rebaseAdmOp+`",`, "", 1)
		},
		"the runtime did not admit a rebase": func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest) {
			open.Rebase = nil
		},
		"a pre-boot notice after a markerless boot (D6)": func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest) {
			open.Rebase = &types.RebaseAdmission{BootedAt: time.Now().UTC().Add(time.Hour)}
		},
		"a superseded attempt": func(t *testing.T, a *rebaseAdmission, open *types.OpenEngineeringAssignmentRequest) {
			a.commitNotice(t, 2, a.seed.TextureInbox)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := newRebaseAdmission(t, 1)
			open := a.open
			recs := append([]types.CommitmentRecord(nil), open.Records.Records...)
			castCopy := recs[0]
			directive := *castCopy.Directive
			castCopy.Directive = &directive
			recs[0] = castCopy
			open.Records.Records = recs
			mutate(t, a, &open)
			open = a.seal(open)
			_, err := a.s.OpenEngineeringAssignment(context.Background(), open)
			if !errors.Is(err, ErrEngineeringAssignmentInvalid) {
				t.Fatalf("want ErrEngineeringAssignmentInvalid, got %v", err)
			}
			if want := rebaseAdmRefusalCode[name]; !strings.Contains(err.Error(), want) {
				t.Fatalf("refused for the wrong reason: want %q in %v", want, err)
			}
			a.requireNothingCommitted(t, open)
		})
	}
}

// rebaseAdmRefusalCode is the stable code each refusal above must carry, so a
// case never passes by failing for another reason.
var rebaseAdmRefusalCode = map[string]string{
	"no notice record":                               "rebase_notice_mismatch: notice " + "rebase-notice:op-a:" + rebaseAdmBase + ":2 is not a rebase notice on the tape",
	"objective without the notice id (D2a)":          "the objective does not name notice",
	"spec rebase_of differs (D2a)":                   "spec rebase_of",
	"spec base differs (D2a)":                        "spec base",
	"spec omits the operation":                       "spec rebase_of",
	"the runtime did not admit a rebase":             "the runtime did not admit a rebase",
	"a pre-boot notice after a markerless boot (D6)": "rebase_notice_interrupted",
	"a superseded attempt":                           "rebase_notice_superseded",
}

func TestRebaseAdmissionRefusesANoticeInAnotherTexturesInbox(t *testing.T) {
	// D5: the master was replaced. Attempt 1 went to the old master; the Cast
	// came from the new one.
	a := newRebaseAdmissionWithNoticeInbox(t, "texture:document-old")
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), a.open); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("a notice in another inbox must be refused, got %v", err)
	}
	a.requireNothingCommitted(t, a.open)
}

func newRebaseAdmissionWithNoticeInbox(t *testing.T, inbox string) *rebaseAdmission {
	t.Helper()
	s := openTestStore(t)
	body := `{"kind":"execution_request","rebase_notice":"` + rebaseAdmNoticeID(1) + `"}`
	seed, err := SeedEngineeringAssignmentAuthorityWithCastBody(s, inboxTestOwner, inboxTestComputer, "master", 1, body)
	if err != nil {
		t.Fatal(err)
	}
	a := &rebaseAdmission{s: s, seed: seed}
	a.commitNotice(t, 1, inbox)
	a.open = a.openFor(t, 0, rebaseAdmNoticeID(1))
	return a
}

func TestRebaseAdmissionRefusesAForgedNotice(t *testing.T) {
	// The record named by the Cast exists, but an agent wrote it: it is not a
	// notice, whatever its id.
	s := openTestStore(t)
	body := `{"kind":"execution_request","rebase_notice":"` + rebaseAdmNoticeID(1) + `"}`
	seed, err := SeedEngineeringAssignmentAuthorityWithCastBody(s, inboxTestOwner, inboxTestComputer, "master", 1, body)
	if err != nil {
		t.Fatal(err)
	}
	a := &rebaseAdmission{s: s, seed: seed}
	forged := inboxNote(rebaseAdmNoticeID(1), "management:"+inboxTestOwner, seed.TextureInbox)
	forged.SubjectRefs = map[string]string{"operation_id": rebaseAdmOp, "booted_base": rebaseAdmBase, "attempt": "1"}
	mustCommitInbox(t, s, "forged", nil, forged)
	open := a.openFor(t, 0, rebaseAdmNoticeID(1))
	if _, err := s.OpenEngineeringAssignment(context.Background(), open); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("a forged notice must be refused, got %v", err)
	}
	a.requireNothingCommitted(t, open)
}

func TestRebaseAdmissionRefusesWhenTheRootsOwnCastDoesNotCarryTheNotice(t *testing.T) {
	s := openTestStore(t)
	// The root's Cast has a different packet: the root never made this Cast.
	seed, err := SeedEngineeringAssignmentAuthorityWithCastBody(s, inboxTestOwner, inboxTestComputer, "master", 1, `{"kind":"execution_request"}`)
	if err != nil {
		t.Fatal(err)
	}
	a := &rebaseAdmission{s: s, seed: seed}
	a.commitNotice(t, 1, seed.TextureInbox)
	open := a.openFor(t, 0, rebaseAdmNoticeID(1))
	if _, err := s.OpenEngineeringAssignment(context.Background(), open); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("a parent Cast without the notice must be refused, got %v", err)
	}
	a.requireNothingCommitted(t, open)
}

func TestRebaseAdmissionRefusesAnOrdinarySpecWithAnAdmissionBlock(t *testing.T) {
	// The runtime admitted a rebase, but management's spec names no notice.
	a := newRebaseAdmission(t, 1)
	open := a.open
	recs := append([]types.CommitmentRecord(nil), open.Records.Records...)
	directive := *recs[0].Directive
	directive.Body = `{"parent":"` + a.seed.TextureCastID + `"}`
	recs[0].Directive = &directive
	open.Records.Records = recs
	open = a.seal(open)
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), open); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("an admission block with no notice in the spec must be refused, got %v", err)
	}
	a.requireNothingCommitted(t, open)
}

func TestRebaseAdmissionPlannedBootKeepsAPreBootNotice(t *testing.T) {
	a := newRebaseAdmission(t, 1)
	open := a.open
	open.Rebase = &types.RebaseAdmission{BootedAt: time.Now().UTC().Add(time.Hour), PlannedBoot: true}
	open = a.seal(open)
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), open); err != nil {
		t.Fatalf("a planned restart resumes a pre-boot notice: %v", err)
	}
}

func TestRebaseAdmissionAnOrdinaryCastIsUntouched(t *testing.T) {
	s := openTestStore(t)
	seed, err := SeedEngineeringAssignmentAuthority(s, inboxTestOwner, inboxTestComputer, 1)
	if err != nil {
		t.Fatal(err)
	}
	open := SeedEngineeringOpenRequest(seed, 0, "cap")
	if _, err := s.OpenEngineeringAssignment(context.Background(), open); err != nil {
		t.Fatalf("an ordinary open: %v", err)
	}
	if _, err := s.GetRebaseNoticeClaim(context.Background(), inboxTestOwner, inboxTestComputer, types.StackRebasePairKey(rebaseAdmOp, rebaseAdmBase)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an ordinary open wrote a claim: %v", err)
	}
}

func TestRebaseAdmissionPairAlreadyClaimedOpensNothing(t *testing.T) {
	a := newRebaseAdmission(t, 2)
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), a.open); err != nil {
		t.Fatal(err)
	}
	second := a.openFor(t, 1, rebaseAdmNoticeID(1))
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), second); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("a second assignment for a claimed pair must be refused, got %v", err)
	}
	if _, err := a.s.GetEngineeringAssignment(context.Background(), inboxTestOwner, inboxTestComputer, second.AssignmentID, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the refused second open left an assignment: %v", err)
	}
	claim, _ := a.claim(t)
	if claim.AssignmentID != a.open.AssignmentID {
		t.Fatalf("the first claim must stand: %+v", claim)
	}
}

func TestRebaseAdmissionTwoConcurrentOpensProduceOneClaim(t *testing.T) {
	a := newRebaseAdmission(t, 2)
	opens := []types.OpenEngineeringAssignmentRequest{a.open, a.openFor(t, 1, rebaseAdmNoticeID(1))}
	var wg sync.WaitGroup
	errs := make([]error, len(opens))
	for i := range opens {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = a.s.OpenEngineeringAssignment(context.Background(), opens[i])
		}()
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrEngineeringAssignmentInvalid) && !errors.Is(err, ErrConcurrentStateChange) {
			t.Fatalf("a losing open failed with %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d opens committed for one notice, want exactly 1 (%v)", wins, errs)
	}
	claim, err := a.claim(t)
	if err != nil {
		t.Fatal(err)
	}
	winner := ""
	for i, err := range errs {
		if err == nil {
			winner = opens[i].AssignmentID
		}
	}
	if claim.AssignmentID != winner {
		t.Fatalf("claim names %s, the committed open is %s", claim.AssignmentID, winner)
	}
}

// U4 slice 8: the owner's retry re-opens a spent pair under attempt 4 or later.
// Ways it can fail: the later attempt collides with the pair's first claim, so
// a spent pair stays spent; two opens bind one owner generation; a gap in the
// attempt numbers lets an older attempt open after the owner retried.

// newRebaseAdmissionAt seeds the root's Cast naming attempt, commits that
// notice (owner-stamped above the cap) and builds the open for it.
func newRebaseAdmissionAt(t *testing.T, count, attempt int) *rebaseAdmission {
	t.Helper()
	s := openTestStore(t)
	body := `{"kind":"execution_request","rebase_notice":"` + rebaseAdmNoticeID(attempt) + `"}`
	seed, err := SeedEngineeringAssignmentAuthorityWithCastBody(s, inboxTestOwner, inboxTestComputer, "master", count, body)
	if err != nil {
		t.Fatal(err)
	}
	a := &rebaseAdmission{s: s, seed: seed}
	a.commitNotice(t, attempt, seed.TextureInbox)
	a.open = a.openFor(t, 0, rebaseAdmNoticeID(attempt))
	return a
}

func TestRebaseAdmissionOwnerRetryOpensAfterASpentPair(t *testing.T) {
	a := newRebaseAdmissionAt(t, 2, 4)
	// The pair was spent by an earlier automatic attempt: its claim stands.
	first := types.StackRebaseClaim{
		PairKey: types.StackRebasePairKey(rebaseAdmOp, rebaseAdmBase), OperationID: rebaseAdmOp, BootedBase: rebaseAdmBase,
		NoticeID: rebaseAdmNoticeID(1), Attempt: 1, AssignmentID: "assignment-earlier",
	}
	if err := a.s.PutRebaseNoticeClaim(context.Background(), inboxTestOwner, inboxTestComputer, first); err != nil {
		t.Fatal(err)
	}
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), a.open); err != nil {
		t.Fatalf("an owner retry must open after a spent pair: %v", err)
	}
	// Two claims now: the first generation's is untouched, the owner's is its own.
	gen0, err := a.s.GetRebaseNoticeClaim(context.Background(), inboxTestOwner, inboxTestComputer, types.StackRebasePairKey(rebaseAdmOp, rebaseAdmBase))
	if err != nil || gen0.AssignmentID != "assignment-earlier" {
		t.Fatalf("the first claim must stand: %+v %v", gen0, err)
	}
	gen1, err := a.s.GetRebaseNoticeClaim(context.Background(), inboxTestOwner, inboxTestComputer, types.StackRebaseClaimKey(types.StackRebasePairKey(rebaseAdmOp, rebaseAdmBase), 4))
	if err != nil || gen1.AssignmentID != a.open.AssignmentID || gen1.Attempt != 4 || gen1.NoticeID != rebaseAdmNoticeID(4) {
		t.Fatalf("the owner generation's claim: %+v %v", gen1, err)
	}
	// The same owner notice opens one assignment only.
	second := a.openFor(t, 1, rebaseAdmNoticeID(4))
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), second); !errors.Is(err, ErrEngineeringAssignmentInvalid) || !strings.Contains(err.Error(), "rebase_notice_claimed") {
		t.Fatalf("a second open for the owner's notice: %v", err)
	}
}

func TestRebaseAdmissionAnOlderAttemptIsSupersededAcrossAGap(t *testing.T) {
	// Attempt 2 was the last automatic one; the owner retried as 4 (no 3).
	a := newRebaseAdmissionAt(t, 1, 2)
	a.commitNotice(t, 4, a.seed.TextureInbox)
	_, err := a.s.OpenEngineeringAssignment(context.Background(), a.open)
	if !errors.Is(err, ErrEngineeringAssignmentInvalid) || !strings.Contains(err.Error(), "rebase_notice_superseded") {
		t.Fatalf("attempt 2 after the owner's attempt 4: %v", err)
	}
	a.requireNothingCommitted(t, a.open)
}

func TestRebaseAdmissionRefusesAnAutomaticNoticeAboveTheCap(t *testing.T) {
	// An EmitterSystem record at attempt 4 that lacks the owner stamp is not a notice.
	s := openTestStore(t)
	body := `{"kind":"execution_request","rebase_notice":"` + rebaseAdmNoticeID(4) + `"}`
	seed, err := SeedEngineeringAssignmentAuthorityWithCastBody(s, inboxTestOwner, inboxTestComputer, "master", 1, body)
	if err != nil {
		t.Fatal(err)
	}
	a := &rebaseAdmission{s: s, seed: seed}
	rec := inboxNote(rebaseAdmNoticeID(4), "selfdev-rebase:"+rebaseAdmOp, seed.TextureInbox)
	rec.EmitterKind = types.CommitmentEmitterSystem
	rec.Provenance.AgentID = types.RebaseNoticeAgentID
	rec.SubjectRefs = map[string]string{"operation_id": rebaseAdmOp, "booted_base": rebaseAdmBase, "stack_base": "aaaa", "attempt": "4"}
	mustCommitInbox(t, s, "unstamped", nil, rec)
	open := a.openFor(t, 0, rebaseAdmNoticeID(4))
	if _, err := s.OpenEngineeringAssignment(context.Background(), open); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("an unstamped attempt 4 must be refused: %v", err)
	}
	a.requireNothingCommitted(t, open)
}

// Review 328 (DeepSeek F3, Devin 4): the owner's notice is committed under the
// open path's lock, after a re-check that no rebase round of the operation
// runs; an open for the last automatic attempt and the owner's retry cannot
// both succeed.
func TestCommitRebaseOwnerNoticeRefusesWhileARoundRuns(t *testing.T) {
	a := newRebaseAdmissionAt(t, 1, 1)
	if _, err := a.s.OpenEngineeringAssignment(context.Background(), a.open); err != nil {
		t.Fatal(err)
	}
	req := a.ownerNoticeRequest(t, 4)
	if _, err := a.s.CommitRebaseOwnerNotice(context.Background(), req, rebaseAdmOp); !errors.Is(err, ErrRebaseRoundRunning) {
		t.Fatalf("a live round must refuse the owner's notice: %v", err)
	}
	if rec, _ := a.s.GetCommitmentRecord(context.Background(), inboxTestOwner, inboxTestComputer, rebaseAdmNoticeID(4)); rec != nil {
		t.Fatal("a refused commit left the notice")
	}
}

func TestCommitRebaseOwnerNoticeCommitsWhenNoRoundRunsAndIsIdempotent(t *testing.T) {
	a := newRebaseAdmissionAt(t, 1, 1)
	req := a.ownerNoticeRequest(t, 4)
	if _, err := a.s.CommitRebaseOwnerNotice(context.Background(), req, rebaseAdmOp); err != nil {
		t.Fatalf("no round runs: %v", err)
	}
	if _, err := a.s.CommitRebaseOwnerNotice(context.Background(), req, rebaseAdmOp); err != nil {
		t.Fatalf("the same command replays: %v", err)
	}
	// A claim naming an assignment the store cannot read fails closed.
	b := newRebaseAdmissionAt(t, 1, 1)
	if err := b.s.PutRebaseNoticeClaim(context.Background(), inboxTestOwner, inboxTestComputer, types.StackRebaseClaim{
		PairKey: types.StackRebasePairKey(rebaseAdmOp, rebaseAdmBase), OperationID: rebaseAdmOp, BootedBase: rebaseAdmBase,
		NoticeID: rebaseAdmNoticeID(1), Attempt: 1, AssignmentID: "assignment-missing",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.s.CommitRebaseOwnerNotice(context.Background(), b.ownerNoticeRequest(t, 4), rebaseAdmOp); err == nil || errors.Is(err, ErrRebaseRoundRunning) {
		t.Fatalf("an unreadable claim must fail closed with its own error: %v", err)
	}
}

func TestAnOpenForTheLastAutomaticAttemptAndTheOwnersRetryNeverBothSucceed(t *testing.T) {
	a := newRebaseAdmissionAt(t, 1, 3)
	req := a.ownerNoticeRequest(t, 4)
	var wg sync.WaitGroup
	var openErr, retryErr error
	wg.Add(2)
	go func() { defer wg.Done(); _, openErr = a.s.OpenEngineeringAssignment(context.Background(), a.open) }()
	go func() {
		defer wg.Done()
		_, retryErr = a.s.CommitRebaseOwnerNotice(context.Background(), req, rebaseAdmOp)
	}()
	wg.Wait()
	if (openErr == nil) == (retryErr == nil) {
		t.Fatalf("exactly one of the open (%v) and the owner's retry (%v) may succeed", openErr, retryErr)
	}
}

func (a *rebaseAdmission) ownerNoticeRequest(t *testing.T, attempt int) types.CommitInboxRecordsRequest {
	t.Helper()
	rec, ok := types.NewStackRebaseNotice(types.StackRebaseNotice{
		TextureInbox: a.seed.TextureInbox, ComputerID: inboxTestComputer, OperationID: rebaseAdmOp, BootedBase: rebaseAdmBase,
		StackBase: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Attempt: attempt, DocID: strings.TrimPrefix(a.seed.TextureInbox, "texture:"), OwnerRetry: true,
	})
	if !ok {
		t.Fatal("owner notice did not build")
	}
	req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
	req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
	return req
}

// Re-review 337 (nit 3): the guarded commit takes only the owner's retry
// notice of the operation, never an arbitrary command.
func TestCommitRebaseOwnerNoticeTakesOnlyTheOwnersRetryNoticeOfTheOperation(t *testing.T) {
	a := newRebaseAdmissionAt(t, 1, 1)
	good := a.ownerNoticeRequest(t, 4)
	cases := map[string]func() types.CommitInboxRecordsRequest{
		"another operation": func() types.CommitInboxRecordsRequest { return good },
		"an automatic notice": func() types.CommitInboxRecordsRequest {
			rec, _ := types.NewStackRebaseNotice(types.StackRebaseNotice{
				TextureInbox: a.seed.TextureInbox, ComputerID: inboxTestComputer, OperationID: rebaseAdmOp, BootedBase: rebaseAdmBase,
				StackBase: "aaaa", Attempt: 2, DocID: "document-master",
			})
			req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
			req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
			return req
		},
		"an ordinary record": func() types.CommitInboxRecordsRequest {
			rec := inboxNote("rebase-notice:op-a:"+rebaseAdmBase+":4", "management:"+inboxTestOwner, a.seed.TextureInbox)
			req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
			req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
			return req
		},
		"two records": func() types.CommitInboxRecordsRequest {
			req := good
			req.Records = append(append([]types.CommitmentRecord(nil), good.Records...), inboxNote("extra", "management:"+inboxTestOwner, a.seed.TextureInbox))
			req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
			return req
		},
		"a command id that is not the record id": func() types.CommitInboxRecordsRequest {
			req := good
			req.CommandID = "some-other-command"
			req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
			return req
		},
	}
	for name, build := range cases {
		operation := rebaseAdmOp
		if name == "another operation" {
			operation = "op-other"
		}
		if _, err := a.s.CommitRebaseOwnerNotice(context.Background(), build(), operation); err == nil || errors.Is(err, ErrRebaseRoundRunning) {
			t.Fatalf("%s: want a refusal, got %v", name, err)
		}
	}
	if rec, _ := a.s.GetCommitmentRecord(context.Background(), inboxTestOwner, inboxTestComputer, rebaseAdmNoticeID(4)); rec != nil {
		t.Fatal("a refused commit left a record")
	}
	if _, err := a.s.CommitRebaseOwnerNotice(context.Background(), good, rebaseAdmOp); err != nil {
		t.Fatalf("the owner's notice of the operation: %v", err)
	}
}
