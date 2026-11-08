package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The reviews route, failure modes written before the code:
//
//	R1  an accepted review's sealed verdict and management's seat record are
//	    not readable by the owner (seal_valid, served model, pWithinCeiling);
//	R2  a rejected review's reason is not readable;
//	R3  an unknown release is an error instead of an empty answer;
//	R4  another computer's review of the same release is visible;
//	R5  a verdict whose seal does not verify reads as valid.

func reviewsOf(t *testing.T, e *installEnv, release string) (int, reviewsResponse) {
	t.Helper()
	w := packagesCall(t, e.h, packagesReviewsPath+"?release="+release, e.owner)
	var body reviewsResponse
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
	}
	return w.Code, body
}

func TestReviewsRouteAcceptedReview(t *testing.T) {
	f := newNotesReviewFixture(t)
	if code, _, reason := f.post(t); code != http.StatusAccepted {
		t.Fatalf("install = %d %s", code, reason)
	}
	f.judge(t, true)
	code, body := reviewsOf(t, f.e, f.id)
	if code != http.StatusOK || body.Verdict == nil {
		t.Fatalf("R1: %d %+v", code, body)
	}
	v := body.Verdict
	if v.Verdict != installreview.VerdictAccept || !v.SealValid || len(v.Seats) != 1 {
		t.Fatalf("R1: verdict %+v", v)
	}
	if seat := v.Seats[0]; seat.PWithinCeiling != 0.9 || seat.Verdict != installreview.VerdictAccept || seat.RecordID == "" {
		t.Fatalf("R1: seat %+v", seat)
	}
	if len(body.Records) < 2 {
		t.Fatalf("R1: records %d", len(body.Records))
	}
	// R5: a signer that is not the sealer reads as not valid.
	f.e.rt.installReviewSigner = newReviewSigner(t)
	if _, body = reviewsOf(t, f.e, f.id); body.Verdict == nil || body.Verdict.SealValid || body.Verdict.SealError == "" {
		t.Fatalf("R5: %+v", body.Verdict)
	}
}

func TestReviewsRouteRejectedReview(t *testing.T) {
	f := newNotesReviewFixture(t)
	if code, _, reason := f.post(t); code != http.StatusAccepted {
		t.Fatalf("install = %d %s", code, reason)
	}
	f.judge(t, false)
	_, body := reviewsOf(t, f.e, f.id)
	if body.Verdict == nil || body.Verdict.Verdict != installreview.VerdictReject || !body.Verdict.SealValid {
		t.Fatalf("R2: %+v", body.Verdict)
	}
	if !strings.Contains(body.Verdict.Reason, "it posts notes to a sink") || len(body.Verdict.Seats) != 1 || !strings.Contains(body.Verdict.Seats[0].Reason, "it posts notes to a sink") {
		t.Fatalf("R2: reason not readable: %+v", body.Verdict)
	}
}

func TestReviewsRouteUnknownAndOtherComputer(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	// R3: unknown release is empty, not an error.
	unknown := strings.Repeat("c", 64)
	code, body := reviewsOf(t, e, unknown)
	if code != http.StatusOK || body.Verdict != nil || body.Records == nil || len(body.Records) != 0 {
		t.Fatalf("R3: %d %+v", code, body)
	}
	if w := packagesCall(t, e.h, packagesReviewsPath+"?release=nope", e.owner); w.Code != http.StatusBadRequest {
		t.Fatalf("a bad release = %d", w.Code)
	}
	// R4: a verdict on another computer for the same release id is not visible.
	subject := installreview.Subject{ReleaseID: unknown, PatchSHA256: strings.Repeat("b", 64), Package: installName, Publisher: "publisher-x"}
	other := installreview.VerdictRecord(subject, installreview.Plan{Risk: types.ImpactI1}, nil, nil, true, "computer-other", time.Now().UTC())
	if _, err := e.rt.store.AppendCommitmentRecord(context.Background(), e.owner, "computer-other", other); err != nil {
		t.Fatal(err)
	}
	if _, body = reviewsOf(t, e, unknown); body.Verdict != nil || len(body.Records) != 0 {
		t.Fatalf("R4: another computer's review is visible: %+v", body)
	}
	// And another owner's on this computer.
	if _, err := e.rt.store.AppendCommitmentRecord(context.Background(), "owner-other", e.computer, other); err != nil {
		t.Fatal(err)
	}
	if _, body = reviewsOf(t, e, unknown); body.Verdict != nil || len(body.Records) != 0 {
		t.Fatalf("R4: another owner's review is visible: %+v", body)
	}
}
