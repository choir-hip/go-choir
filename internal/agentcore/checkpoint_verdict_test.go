package agentcore

// The guest's side of the host's checkpoint verdict (slice 2 of
// docs/design/apply-checkpoint-without-replay-2026-10-26.md), written before
// the code. Ways it could fail:
//
//  1. a bind does not say which deposit mode its live store is in, so the
//     host cannot tell residual R1 (a retained store its upcasting rebuild
//     cannot reproduce) from rows the tape cannot rebuild;
//  2. restore proceeds to a checkpoint the host proved not_derivable (it
//     downloads a base and replays before refusing, or does not refuse);
//  3. restore refuses a checkpoint for a reason that is not the host's proof:
//     pending, verified, unverified (R1 included), no verdict row, or a
//     platform that cannot answer all go on to restore's own reconstruction
//     gate, which stays the enforcement;
//  4. the bind report's CheckpointEligible stays a hard-coded true, or calls a
//     pending checkpoint ineligible;
//  5. the refusal surfaces as a server error instead of a visible conflict.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// verdictBaseSource is a base source that also answers the host's verdict.
type verdictBaseSource struct {
	*countingBaseSource
	verdict selfdevprotocol.CheckpointVerdict
	found   bool
	err     error
	asked   []string
}

func (v *verdictBaseSource) CheckpointVerdict(ctx context.Context, computerID, digest string) (selfdevprotocol.CheckpointVerdict, bool, error) {
	v.asked = append(v.asked, computerID+" "+digest)
	return v.verdict, v.found, v.err
}

func verdictOf(status, reason string) selfdevprotocol.CheckpointVerdict {
	return selfdevprotocol.CheckpointVerdict{Status: status, Reason: reason}
}

// Failure mode 1.
func TestCheckpointBindRecordsTheLiveDepositMode(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-bind-deposit-mode")
	upcast, err := nf.rt.store.DepositsUpcast()
	if err != nil {
		t.Fatal(err)
	}
	want := selfdevprotocol.DepositModeRetained
	if upcast {
		want = selfdevprotocol.DepositModeUpcast
	}
	report := nf.bind(t)
	if got := report.PublishedCheckpoint.Checkpoint.Request.DepositMode; got != want {
		t.Fatalf("published deposit mode %q, want the live store's %q", got, want)
	}
	if got, err := depositModeOf(nf.rt.store); err != nil || got != want {
		t.Fatalf("depositModeOf = %q %v, want %q", got, err, want)
	}
}

// Failures 2 and 3.
func TestRefuseNotDerivableCheckpointByHostVerdict(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-verdict-matrix")
	checkpoint := nf.bind(t).PublishedCheckpoint.Checkpoint
	for _, tc := range []struct {
		name    string
		verdict selfdevprotocol.CheckpointVerdict
		found   bool
		err     error
		refuse  bool
	}{
		{"not_derivable", verdictOf(selfdevprotocol.CheckpointVerdictNotDerivable, selfdevprotocol.VerdictReasonWitnessMismatch), true, nil, true},
		{"pending", verdictOf(selfdevprotocol.CheckpointVerdictPending, ""), true, nil, false},
		{"verified", verdictOf(selfdevprotocol.CheckpointVerdictVerified, ""), true, nil, false},
		{"unverified R1 base_upcast", verdictOf(selfdevprotocol.CheckpointVerdictUnverified, selfdevprotocol.VerdictReasonBaseUpcast), true, nil, false},
		{"no verdict row", selfdevprotocol.CheckpointVerdict{}, false, nil, false},
		{"platform cannot answer", selfdevprotocol.CheckpointVerdict{}, false, errors.New("corpusd unreachable"), false},
	} {
		source := &verdictBaseSource{countingBaseSource: nf.source, verdict: tc.verdict, found: tc.found, err: tc.err}
		nf.rt.restoreBaseSource = source
		err := nf.rt.refuseNotDerivableCheckpoint(context.Background(), nf.rt.cfg.ComputerID, checkpoint)
		if tc.refuse != errors.Is(err, ErrCheckpointNotDerivable) {
			t.Fatalf("%s: err = %v, refuse want %v", tc.name, err, tc.refuse)
		}
		if !tc.refuse && err != nil {
			t.Fatalf("%s: unexpected error %v", tc.name, err)
		}
		if len(source.asked) != 1 || !strings.HasSuffix(source.asked[0], checkpoint.Digest) {
			t.Fatalf("%s: asked %v, want one ask for the checkpoint", tc.name, source.asked)
		}
	}
	// A source that cannot give verdicts (an older platform client) is no refusal.
	nf.rt.restoreBaseSource = nf.source
	if err := nf.rt.refuseNotDerivableCheckpoint(context.Background(), nf.rt.cfg.ComputerID, checkpoint); err != nil {
		t.Fatalf("a source with no verdicts refused: %v", err)
	}
}

// Failure mode 2: the refusal comes before any base is downloaded, and the
// live realization is untouched.
func TestRestoreRefusesANotDerivableCheckpointBeforeAnyWork(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-restore-not-derivable")
	checkpoint := nf.bind(t).PublishedCheckpoint.Checkpoint
	source := &verdictBaseSource{countingBaseSource: nf.source, found: true,
		verdict: verdictOf(selfdevprotocol.CheckpointVerdictNotDerivable, selfdevprotocol.VerdictReasonWitnessMismatch)}
	nf.rt.restoreBaseSource = source
	headBefore := nf.liveHead(t)
	downloadsBefore := nf.source.downloadCount()

	result, err := nf.restore(t, checkpoint)

	if !errors.Is(err, ErrCheckpointNotDerivable) {
		t.Fatalf("restore to a not_derivable checkpoint: %v %#v", err, result)
	}
	if nf.source.downloadCount() != downloadsBefore {
		t.Fatal("restore downloaded a base before refusing a not_derivable checkpoint")
	}
	if nf.liveHead(t) != headBefore || nf.rt.store == nil {
		t.Fatal("the refused restore touched the live realization")
	}

	// The same checkpoint, judged verified, restores.
	source.verdict = verdictOf(selfdevprotocol.CheckpointVerdictVerified, "")
	if result, err := nf.restore(t, checkpoint); err != nil || !result.WitnessMatched {
		t.Fatalf("restore to a verified checkpoint: %v %#v", err, result)
	}
}

// Failure mode 5.
func TestNotDerivableRefusalIsAConflict(t *testing.T) {
	err := errors.Join(ErrCheckpointNotDerivable, errors.New("tables: og_objects"))
	if got := rematerializeRefusalStatus(err); got != http.StatusConflict {
		t.Fatalf("status %d, want 409", got)
	}
}

// Failure mode 4.
func TestBindReportCarriesTheHostVerdict(t *testing.T) {
	for _, tc := range []struct {
		name         string
		verdict      selfdevprotocol.CheckpointVerdict
		found        bool
		wantVerdict  string
		wantEligible bool
	}{
		{"pending right after publish", verdictOf(selfdevprotocol.CheckpointVerdictPending, ""), true, selfdevprotocol.CheckpointVerdictPending, true},
		{"already not_derivable", verdictOf(selfdevprotocol.CheckpointVerdictNotDerivable, selfdevprotocol.VerdictReasonWitnessMismatch), true, selfdevprotocol.CheckpointVerdictNotDerivable, false},
		{"verified", verdictOf(selfdevprotocol.CheckpointVerdictVerified, ""), true, selfdevprotocol.CheckpointVerdictVerified, true},
		{"no answer", selfdevprotocol.CheckpointVerdict{}, false, "", true},
	} {
		nf := newNoReplayFixture(t, "computer-bind-report-verdict")
		nf.rt.restoreBaseSource = &verdictBaseSource{countingBaseSource: nf.source, verdict: tc.verdict, found: tc.found}
		report := nf.bind(t)
		if report.CheckpointVerdict != tc.wantVerdict || report.CheckpointEligible != tc.wantEligible {
			t.Fatalf("%s: verdict %q eligible %v, want %q / %v", tc.name, report.CheckpointVerdict, report.CheckpointEligible, tc.wantVerdict, tc.wantEligible)
		}
	}
}

// Review 329 (DeepSeek 7): the refusal comes before the restore intent is
// appended, so a refused restore leaves nothing on the tape.
func TestRestoreRouteRefusesANotDerivableCheckpointBeforeAppendingTheIntent(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-restore-route-not-derivable")
	checkpoint := nf.bind(t).PublishedCheckpoint.Checkpoint
	nf.rt.restoreBaseSource = &verdictBaseSource{countingBaseSource: nf.source, found: true,
		verdict: verdictOf(selfdevprotocol.CheckpointVerdictNotDerivable, selfdevprotocol.VerdictReasonWitnessMismatch)}
	headBefore := nf.liveHead(t)

	body, _ := json.Marshal(restoreAPIRequest{Checkpoint: checkpoint})
	computerID := nf.rt.cfg.ComputerID
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/lifecycle/restore", bytes.NewReader(body))
	request.Header.Set("X-Authenticated-User", "owner-restore")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(nf.rt).HandleComputersRouter(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("restore route = %d %s, want 409", response.Code, response.Body.String())
	}
	if nf.liveHead(t) != headBefore {
		t.Fatal("the refused restore appended a restore intent to the tape")
	}
}
