package agentcore

// U4 slice 8 (docs/design/u4-stack-rebase-build-2026-10-26.md): the owner's
// retry act re-opens a spent pair. Ways it can fail, written before the code:
//
//   1. the act mints while automatic attempts remain or an attempt is live, so
//      the root is asked twice -> ...RefusesWhileAnAttemptIsLive
//   2. the act mints for a pair whose rebase round still runs
//      -> ...RefusesWhileARoundRuns
//   3. the act works with the switches off, off the moved state, or with no
//      live root -> ...RefusesWhatIsNotTheMovedStack
//   4. a spent pair (cap reached, or a claimed round that ended) stays spent
//      -> ...MintsAfterTheCapAndAfterAnEndedRound
//   5. a desk (management's ProductAPI) can call it; it is an owner act
//      -> ...IsNotOnTheDesksAllowlist, ...RequiresTheOwner
//   6. the owner's attempt runs a rebase that cannot be claimed, because its
//      claim collides with the first generation's, or the timer re-mints over it
//      -> ...AttemptFourIsAdmittedAndClaimedSeparately, ...TimerMintsNothingAfter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func (f *rebaseFixture) retry(t *testing.T) (rebaseRetryResult, error) {
	t.Helper()
	return f.rt.retryRebaseNotice(context.Background(), f.owner)
}

// exhaust mints the three automatic attempts, each closed by its window.
func (f *rebaseFixture) exhaust(t *testing.T) {
	t.Helper()
	for n := 1; n <= 3; n++ {
		if out := f.fire(t); out.Attempt != n {
			t.Fatalf("attempt %d: %+v", n, out)
		}
		f.now = f.now.Add(time.Hour)
	}
}

func TestRebaseRetryRefusesWhileAnAttemptIsLive(t *testing.T) {
	f := newRebaseFixture(t)
	if _, err := f.retry(t); err == nil || !strings.Contains(err.Error(), "rebase_retry_no_attempt") {
		t.Fatalf("no automatic attempt yet: %v", err)
	}
	f.fire(t) // attempt 1, unanswered, inside its window
	if _, err := f.retry(t); err == nil || !strings.Contains(err.Error(), "rebase_retry_attempt_live") {
		t.Fatalf("attempt 1 is live: %v", err)
	}
	// Closed by its window, but automatic attempts remain: the timer owns them.
	f.now = f.now.Add(time.Hour)
	if _, err := f.retry(t); err == nil || !strings.Contains(err.Error(), "rebase_retry_attempts_remain") {
		t.Fatalf("automatic attempts remain: %v", err)
	}
	if f.noticeCount(t) != 1 {
		t.Fatalf("a refused retry minted: %d notices", f.noticeCount(t))
	}
}

func TestRebaseRetryMintsAfterTheCapAndOnlyOncePerLiveAttempt(t *testing.T) {
	f := newRebaseFixture(t)
	f.exhaust(t)
	res, err := f.retry(t)
	if err != nil || res.Attempt != 4 || res.Minted != f.noticeID(4) {
		t.Fatalf("retry after the cap: %+v %v", res, err)
	}
	rec := f.notice(t, 4)
	if rec == nil || !types.IsStackRebaseNotice(*rec) || rec.SubjectRefs["owner_retry"] != "true" || rec.Inbox != f.root || !store.RecordWakesReader(*rec) {
		t.Fatalf("the owner's notice: %+v", rec)
	}
	if got := inboxRecordKind(*rec); got != "rebase_notice" {
		t.Fatalf("the root reads it as %q", got)
	}
	// Live until its window passes; then the owner may ask again (attempt 5).
	if _, err := f.retry(t); err == nil || !strings.Contains(err.Error(), "rebase_retry_attempt_live") {
		t.Fatalf("attempt 4 is live: %v", err)
	}
	f.now = f.now.Add(time.Hour)
	res, err = f.retry(t)
	if err != nil || res.Attempt != 5 {
		t.Fatalf("a second retry after attempt 4 closed: %+v %v", res, err)
	}
	// The timer never mints above the cap, whatever the owner did.
	f.now = f.now.Add(time.Hour)
	if out := f.fire(t); out.Minted != "" || out.Rearm != 0 {
		t.Fatalf("the timer minted over the owner's attempts: %+v", out)
	}
}

func TestRebaseRetryMintsAfterAClaimedRoundEnded(t *testing.T) {
	f := newRebaseContentFixture(t) // attempt 1 answered, claimed, a round open
	if _, err := f.rt.retryRebaseNotice(context.Background(), f.owner); err == nil || !strings.Contains(err.Error(), "rebase_retry_round_running") {
		t.Fatalf("a rebase round still runs: %v", err)
	}
	f.rt.assignmentRuntime = absentAssignmentCapsule{}
	c1eCancel(t, f.rt, f.assignment, "test: the round ends", true)
	f.now = f.now.Add(time.Minute)
	res, err := f.rt.retryRebaseNotice(context.Background(), f.owner)
	if err != nil || res.Attempt != 4 {
		t.Fatalf("a claimed pair whose round ended is spent; the owner may retry: %+v %v", res, err)
	}
	if rec := f.notice(t, 4); rec == nil || rec.SubjectRefs["owner_retry"] != "true" {
		t.Fatalf("notice 4: %+v", rec)
	}
}

func TestRebaseRetryRefusesWhatIsNotTheMovedStack(t *testing.T) {
	cases := map[string]func(t *testing.T, f *rebaseFixture){
		"source.rebase_offer off": func(t *testing.T, f *rebaseFixture) { f.setSwitch(t, store.SettingSourceRebaseOffer, false) },
		"texture.root_desk off":   func(t *testing.T, f *rebaseFixture) { f.setSwitch(t, store.SettingTextureRootDesk, false) },
		"the stack is projected":  func(t *testing.T, f *rebaseFixture) { setBootedBase(t, f.rt, f.base) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newRebaseFixture(t)
			f.exhaust(t)
			mutate(t, f)
			if _, err := f.retry(t); err == nil {
				t.Fatal("the retry minted")
			}
			if f.notice(t, 4) != nil {
				t.Fatal("a refused retry left a notice")
			}
		})
	}
}

func TestRebaseRetryIsNotOnTheDesksAllowlistAndRequiresTheOwner(t *testing.T) {
	path := "/api/computers/computer-rebase/self-development/rebase-retry"
	if err := validateProductAPIRoute(http.MethodPost, path); err == nil {
		t.Fatal("management's ProductAPI may post the rebase retry; it is an owner act")
	}
	f := newRebaseFixture(t)
	f.exhaust(t)
	handler := &APIHandler{rt: f.rt}
	call := func(method string, auth bool, computer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		if auth {
			req.Header.Set("X-Authenticated-User", f.owner)
		}
		req.Header.Set("X-Authenticated-Computer", computer)
		rec := httptest.NewRecorder()
		handler.HandleComputersRouter(rec, req)
		return rec
	}
	if rec := call(http.MethodPost, false, f.computer); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", rec.Code)
	}
	if rec := call(http.MethodGet, true, f.computer); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d", rec.Code)
	}
	if rec := call(http.MethodPost, true, "another-computer"); rec.Code != http.StatusForbidden {
		t.Fatalf("another computer = %d", rec.Code)
	}
	rec := call(http.MethodPost, true, f.computer)
	var body map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body["notice_id"] != f.noticeID(4) {
		t.Fatalf("owner retry = %d %s", rec.Code, rec.Body)
	}
	// A refusal is a 409 naming its code.
	rec = call(http.MethodPost, true, f.computer)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "rebase_retry_attempt_live") {
		t.Fatalf("a refused retry = %d %s", rec.Code, rec.Body)
	}
}

func TestRebaseRetryAttemptFourIsAdmittedAndClaimedSeparately(t *testing.T) {
	f := &rebaseCastFixture{rebaseFixture: newRebaseFixture(t), castID: "cast-root-4"}
	f.preflight = &fakeSourcePreflight{head: gitHead(t, f.repo)}
	f.rt.sourcePreflight = f.preflight
	f.exhaust(t)
	if _, err := f.retry(t); err != nil {
		t.Fatal(err)
	}
	f.rootCast(t, f.castID, f.noticeID(4))
	spec := f.spec(func(m map[string]string) { m["rebase_notice"] = f.noticeID(4) })
	objective := "Rebase the applied change " + f.opID + " (rebase notice " + f.noticeID(4) + ")"
	plan, err := f.plan(t, spec, objective)
	if err != nil {
		t.Fatalf("the owner's attempt must plan: %v", err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(context.Background(), plan, records); err != nil {
		t.Fatalf("open: %v", err)
	}
	pair := types.StackRebasePairKey(f.opID, f.moved)
	if _, err := f.s.GetRebaseNoticeClaim(context.Background(), f.owner, f.computer, pair); err == nil {
		t.Fatal("the owner's attempt wrote the first generation's claim")
	}
	claim, err := f.s.GetRebaseNoticeClaim(context.Background(), f.owner, f.computer, types.StackRebaseClaimKey(pair, 4))
	if err != nil || claim.AssignmentID != plan.AssignmentID || claim.Attempt != 4 {
		t.Fatalf("claim %+v %v", claim, err)
	}
	assignment, _ := f.s.GetEngineeringAssignment(context.Background(), f.owner, f.computer, plan.AssignmentID, 1)
	lineage, ok := f.rt.rebaseLineage(context.Background(), assignment)
	if !ok || lineage.Attempt != 4 || lineage.NoticeID != f.noticeID(4) {
		t.Fatalf("lineage %+v ok=%v", lineage, ok)
	}
	// The pair's claimed state now stops the gate for the same notice.
	if _, err := f.plan(t, spec, objective); err == nil || !strings.Contains(err.Error(), "rebase_pair_claimed") {
		t.Fatalf("a second Cast for the owner's notice: %v", err)
	}
}
