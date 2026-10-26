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
