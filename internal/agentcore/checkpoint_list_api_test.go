package agentcore

// The guest's checkpoint surface after the manual bind is deleted (slice 3 of
// docs/archive/design/apply-checkpoint-without-replay-2026-10-26.md), written before
// the code. Ways it could fail:
//
//  1. POST /lifecycle/checkpoint still binds: a second, manual publication
//     path for the same object (standing question 5);
//  2. the listing cannot be read by the owner, or hides the host's verdict;
//  3. the artifact fetched by digest is not one restore can use;
//  4. a computer without platform control answers a listing from nothing;
//  5. the owner act that reopens a closed cadence is missing, or is reachable
//     by GET;
//  6. the cadence, run with its real seams, publishes through the same bind
//     the apply uses and the listing then shows it (the end-to-end artifact).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func apiRequest(t *testing.T, fx *checkpointHeadFixture, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	computerID := fx.rt.cfg.ComputerID
	request := httptest.NewRequest(method, "/api/computers/"+computerID+path, nil)
	request.Header.Set("X-Authenticated-User", "owner-checkpoint")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(fx.rt).HandleComputersRouter(response, request)
	return response
}

// Failure mode 1.
func TestManualCheckpointBindRouteIsDeleted(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-bind-route-gone")
	response := apiRequest(t, fx, http.MethodPost, "/lifecycle/checkpoint")
	if response.Code >= 200 && response.Code < 300 {
		t.Fatalf("POST /lifecycle/checkpoint answered %d: the manual bind is still reachable", response.Code)
	}
	if len(fx.platform.published) != 0 {
		t.Fatalf("%d checkpoints published through the deleted route", len(fx.platform.published))
	}
}

// Failures 2 and 3.
func TestCheckpointListingShowsVerdictsAndFetchesByDigest(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-listing")
	report, err := fx.rt.BindCheckpointRestoreSet(context.Background(), fx.rt.cfg.ComputerID)
	if err != nil || report.PublishedCheckpoint == nil {
		t.Fatalf("bind: %v", err)
	}
	digest := report.PublishedCheckpoint.Checkpoint.Digest

	response := apiRequest(t, fx, http.MethodGet, "/lifecycle/checkpoints")
	if response.Code != http.StatusOK {
		t.Fatalf("listing = %d %s", response.Code, response.Body.String())
	}
	var listing struct {
		Cadence     CheckpointCadenceStatus             `json:"cadence"`
		Checkpoints []selfdevprotocol.CheckpointSummary `json:"checkpoints"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Checkpoints) != 1 || listing.Checkpoints[0].CheckpointDigest != digest || listing.Checkpoints[0].Verdict.Status != selfdevprotocol.CheckpointVerdictPending {
		t.Fatalf("listing = %+v", listing)
	}
	if listing.Cadence.State == "" {
		t.Fatalf("the listing says nothing of the cadence: %+v", listing.Cadence)
	}

	fetch := apiRequest(t, fx, http.MethodGet, "/lifecycle/checkpoints?checkpoint_digest="+digest)
	if fetch.Code != http.StatusOK {
		t.Fatalf("fetch = %d %s", fetch.Code, fetch.Body.String())
	}
	var artifact selfdevprotocol.CheckpointResponse
	if err := json.Unmarshal(fetch.Body.Bytes(), &artifact); err != nil || artifact.Checkpoint.Digest != digest || artifact.Receipt.Signature == "" {
		t.Fatalf("fetched artifact = %+v %v", artifact, err)
	}
	if missing := apiRequest(t, fx, http.MethodGet, "/lifecycle/checkpoints?checkpoint_digest="+strings.Repeat("0", 64)); missing.Code != http.StatusNotFound {
		t.Fatalf("unknown digest = %d", missing.Code)
	}
	if post := apiRequest(t, fx, http.MethodPost, "/lifecycle/checkpoints"); post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST listing = %d, want 405", post.Code)
	}
}

// Failure mode 4.
func TestCheckpointListingWithoutPlatformControlIsUnavailable(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-listing-no-control")
	fx.rt.ownerRecoveryControl = nil
	if response := apiRequest(t, fx, http.MethodGet, "/lifecycle/checkpoints"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("listing without platform control = %d", response.Code)
	}
}

// Failure mode 5.
func TestResumeCheckpointsIsAPostOnlyOwnerRoute(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-resume-route")
	if response := apiRequest(t, fx, http.MethodGet, "/lifecycle/resume-checkpoints"); response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET resume = %d, want 405", response.Code)
	}
	// Close the cadence the way a crash twice does, then reopen it.
	for i := 0; i < checkpointCadenceAttemptLimit; i++ {
		if err := spendSettlementAttempt(fx.rt.cfg.StorePath, "checkpoint_cadence", fx.rt.cfg.ComputerID, checkpointCadenceAttemptLimit); err != nil {
			t.Fatal(err)
		}
	}
	cfg := checkpointCadenceConfig{EveryEvents: 1, Every: time.Hour}
	fx.rt.cadence.cfg = &cfg
	fx.rt.cadence.last = func(context.Context) (checkpointMark, bool, error) {
		return checkpointMark{Sequence: 0, At: time.Now()}, true, nil
	}
	if out, _ := fx.rt.checkpointCadenceTick(context.Background(), time.Now()); out != cadenceOff {
		t.Fatalf("cadence not closed: %s", out)
	}
	response := apiRequest(t, fx, http.MethodPost, "/lifecycle/resume-checkpoints")
	if response.Code != http.StatusOK {
		t.Fatalf("POST resume = %d %s", response.Code, response.Body.String())
	}
	var status CheckpointCadenceStatus
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil || status.State != "on" {
		t.Fatalf("status after resume = %+v %v", status, err)
	}
}

// Failure mode 6: the real seams, end to end. The cadence reads the newest
// checkpoint from the platform's listing, binds through BindCheckpointRestoreSet
// and publishes to the platform; the listing then carries it.
func TestCheckpointCadenceEndToEndPublishesAndIsListed(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-cadence-e2e")
	cfg := checkpointCadenceConfig{EveryEvents: 3, Every: 0}
	fx.rt.cadence.cfg = &cfg
	ctx := context.Background()
	now := time.Now().UTC()

	if out, err := fx.rt.checkpointCadenceTick(ctx, now); out != cadenceIdle || err != nil {
		t.Fatalf("a computer with no checkpoint and no new event: %s %v", out, err)
	}
	for i := 0; i < 4; i++ {
		fx.appendDeskEvent(t, "e2e-"+string(rune('a'+i)))
	}
	out, err := fx.rt.checkpointCadenceTick(ctx, now)
	if out != cadenceBound || err != nil {
		t.Fatalf("4 events past the (absent) last checkpoint: %s %v", out, err)
	}
	if len(fx.platform.published) != 1 {
		t.Fatalf("%d checkpoints published, want the cadence's one", len(fx.platform.published))
	}
	published := fx.platform.published[0].Checkpoint.Request
	if published.AcceptedEventHead != fx.liveHead(t) || !published.OwnerRecovery {
		t.Fatalf("published %+v, want an owner-recovery checkpoint at the live head", published)
	}
	// Quiet again: the listing's newest checkpoint is the head, nothing is due.
	if out, _ := fx.rt.checkpointCadenceTick(ctx, now.Add(24*time.Hour)); out != cadenceIdle || len(fx.platform.published) != 1 {
		t.Fatalf("a bound computer bound again with no new event: %s", out)
	}
}
