package agentcore

// U4 slice 1/2 (docs/design/u4-stack-rebase-build-2026-10-26.md, "Slices",
// slice 2 and failure modes D8, D10): the trigger and the bounded re-mint.
// Ways it can fail, written before the code:
//
//   1. a mint inside boot, or before the stay-up interval since boot
//      -> ...ArmsAfterTheStayUpIntervalNeverAtBoot
//   2. a notice with the state not holding (projected, objects missing)
//      -> ...OnlyForABaseMovedStackWithItsObjects
//   3. a switch off (rebase_offer, projection, root desk) still mints
//      -> ...EachSwitchOffMintsNothing
//   4. D8: no live root consumes an attempt -> ...NoLiveRootMintsNothingAndUsesNoAttempt
//   5. a live attempt is doubled before it closes -> ...AnAnsweredAttemptIsNotDoubled
//   6. D10: a re-mint loop; the cap is 3 -> ...CapsAtThreeAttemptsAndStops
//   7. a crash boot leaves the pair stuck, or re-uses the pre-boot attempt
//      -> ...CrashBootMintsTheNextAttemptNotTheOld
//   8. a planned restart mints a duplicate beside a live attempt
//      -> ...PlannedRestartKeepsALiveAttempt
//   9. a claim does not end the pair -> ...ClaimEndsTheMinting
//  10. a Cast whose round closed with no claim does not count as an attempt
//      that ended -> ...ARoundClosedWithoutAClaimClosesTheAttempt
//  11. the notice's inbox is a stale master -> ...NoticeGoesToTheCurrentRoot

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type noopRebaseTimer struct{}

func (noopRebaseTimer) Stop() bool { return true }

type rebaseFixture struct {
	rt         *Runtime
	s          *store.Store
	owner      string
	computer   string
	root       string // texture:<master>
	opID       string
	repo       string
	base       string // B
	moved      string // B'
	management string
	now        time.Time
}

// newRebaseFixture builds a computer with an applied stack op-a on base B,
// whose booted base then moved to B', a live root Texture, and all three
// switches on.
func newRebaseFixture(t *testing.T) *rebaseFixture {
	t.Helper()
	const computerID = "computer-rebase"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	f := &rebaseFixture{rt: rt, s: rt.store, owner: "owner-rebase", computer: computerID, opID: "op-a", repo: repo, base: base}
	rt.selfdevRouteOwnerID = f.owner
	f.management = persistentManagementAgentID(f.owner)
	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, f.opID, base, patch)
	f.moved = commitOnTop(t, repo, base, map[string]string{"frontend/public/other.txt": "platform\n"})
	setBootedBase(t, rt, f.moved)

	seedOwnerActDocument(t, f.s, f.owner, computerID, "doc-master")
	f.root = currentTextureAgentID("doc-master")
	if _, err := f.s.ClaimTextureRoot(context.Background(), nil, store.TextureRoot{OwnerID: f.owner, ComputerID: computerID, DocumentID: "doc-master", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	f.setSwitch(t, store.SettingSourceRebaseOffer, true)
	f.setSwitch(t, store.SettingTextureRootDesk, true)

	f.now = time.Now().UTC()
	rt.bootedAt = f.now.Add(-time.Hour) // a long-running boot
	rt.rebaseStayUp, rt.rebaseAnswerWindow = 5*time.Minute, 30*time.Minute
	rt.rebaseNow = func() time.Time { return f.now }
	// The tests fire the timer's work themselves; an armed timer never runs.
	rt.rebaseAfterFunc = func(time.Duration, func()) rebaseTimerHandle { return noopRebaseTimer{} }
	t.Cleanup(rt.stopRebaseNotice)
	return f
}

func (f *rebaseFixture) setSwitch(t *testing.T, key string, on bool) {
	t.Helper()
	if err := f.s.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: f.owner, ComputerID: f.computer, Key: key, Value: on}); err != nil {
		t.Fatal(err)
	}
}

func (f *rebaseFixture) noticeID(n int) string {
	return types.StackRebaseNoticeID(f.opID, f.moved, n)
}

func (f *rebaseFixture) notice(t *testing.T, n int) *types.CommitmentRecord {
	t.Helper()
	rec, err := f.s.GetCommitmentRecord(context.Background(), f.owner, f.computer, f.noticeID(n))
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func (f *rebaseFixture) noticeCount(t *testing.T) int {
	t.Helper()
	count := 0
	for n := 1; n <= types.RebaseNoticeMaxAttempts+2; n++ {
		if f.notice(t, n) != nil {
			count++
		}
	}
	return count
}

// fire runs the timer's work now and returns its outcome.
func (f *rebaseFixture) fire(t *testing.T) rebaseNoticeOutcome {
	t.Helper()
	return f.rt.fireRebaseNotice(context.Background())
}

// rootCast commits the root's Cast to management naming notice, opening a round.
func (f *rebaseFixture) rootCast(t *testing.T, recordID, noticeID string) {
	t.Helper()
	body := `{"kind":"execution_request","rebase_notice":"` + noticeID + `"}`
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID, Kind: types.CommitmentKindDirective, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: f.root, CommittedAt: f.now.Format(time.RFC3339Nano)},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: f.computer, Inbox: f.management, SenderInbox: f.root, Addressee: f.management,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "rebase the applied change", Body: body},
		Cast:      &types.CommitmentCast{},
	}
	f.commit(t, recordID, rec)
}

// closeRound is management's final Report closing the root's open round.
func (f *rebaseFixture) closeRound(t *testing.T, recordID string) {
	t.Helper()
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID, Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: f.management, CommittedAt: f.now.Format(time.RFC3339Nano)},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: f.computer, Inbox: f.root, SenderInbox: f.management, Addressee: f.root,
		Final: true, Report: &types.CommitmentReport{StopReason: "completed"},
	}
	f.commit(t, recordID, rec)
}

func (f *rebaseFixture) commit(t *testing.T, commandID string, rec types.CommitmentRecord) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: commandID, Records: []types.CommitmentRecord{rec}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	req.CommandDigest = digest
	if _, err := f.s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("commit %s: %v", commandID, err)
	}
}

func (f *rebaseFixture) claim(t *testing.T, n int) {
	t.Helper()
	pair := types.StackRebasePairKey(f.opID, f.moved)
	if err := f.s.PutRebaseNoticeClaim(context.Background(), f.owner, f.computer, types.StackRebaseClaim{
		PairKey: pair, OperationID: f.opID, BootedBase: f.moved, NoticeID: f.noticeID(n), Attempt: n, AssignmentID: "assignment-x", ClaimedAt: f.now,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRebaseNoticeMintsAttemptOneForABaseMovedStack(t *testing.T) {
	f := newRebaseFixture(t)
	out := f.fire(t)
	if out.Minted != f.noticeID(1) || out.Attempt != 1 {
		t.Fatalf("outcome %+v; want attempt 1 minted", out)
	}
	rec := f.notice(t, 1)
	if rec == nil || !types.IsStackRebaseNotice(*rec) || rec.Inbox != f.root {
		t.Fatalf("notice on the tape: %+v", rec)
	}
	if rec.SubjectRefs["operation_id"] != f.opID || rec.SubjectRefs["booted_base"] != f.moved || rec.SubjectRefs["stack_base"] != f.base ||
		rec.SubjectRefs["attempt"] != "1" || rec.SubjectRefs["doc_id"] != "doc-master" {
		t.Fatalf("subject refs %v", rec.SubjectRefs)
	}
	if !store.RecordWakesReader(*rec) {
		t.Fatal("the notice must wake the root")
	}
	// The same attempt answered nothing yet and is within its window: a second
	// fire mints nothing.
	if again := f.fire(t); again.Minted != "" || f.noticeCount(t) != 1 {
		t.Fatalf("second fire %+v, notices %d", again, f.noticeCount(t))
	}
}

func TestRebaseNoticeArmsAfterTheStayUpIntervalNeverAtBoot(t *testing.T) {
	f := newRebaseFixture(t)
	rt := f.rt
	// A fresh boot: the booted-at is now. Reconcile arms a timer; it mints
	// nothing and its due time is bootedAt + the stay-up interval.
	rt.bootedAt = f.now
	rt.rebaseStayUp = time.Hour // far enough that the real timer cannot fire in the test
	if _, err := rt.ReconcileSourceProjection(context.Background(), "boot"); err != nil {
		t.Fatal(err)
	}
	due, armed := rt.rebaseNoticeDueAt()
	if !armed {
		t.Fatal("reconcile at stack_base_moved with the switches on must arm the timer")
	}
	if due.Before(rt.bootedAt.Add(rt.rebaseStayUp)) {
		t.Fatalf("due %s is before bootedAt + stay-up %s", due, rt.bootedAt.Add(rt.rebaseStayUp))
	}
	if f.noticeCount(t) != 0 {
		t.Fatal("arming at boot minted a notice")
	}
	// A second reconcile does not arm a second timer.
	if _, err := rt.ReconcileSourceProjection(context.Background(), "cast"); err != nil {
		t.Fatal(err)
	}
	if due2, _ := rt.rebaseNoticeDueAt(); !due2.Equal(due) {
		t.Fatalf("a second reconcile moved the due time: %s -> %s", due, due2)
	}
	// Past the interval since boot, the due time is not in the past by more than now.
	rt.stopRebaseNotice()
	rt.bootedAt = f.now.Add(-2 * time.Hour)
	rt.rebaseStayUp = 5 * time.Minute
	if _, err := rt.ReconcileSourceProjection(context.Background(), "cast"); err != nil {
		t.Fatal(err)
	}
	if due3, armed := rt.rebaseNoticeDueAt(); !armed || due3.Before(f.now) {
		t.Fatalf("due %s armed=%v; an old boot arms for now or later, not the past", due3, armed)
	}
}

func TestRebaseNoticeOnlyForABaseMovedStackWithItsObjects(t *testing.T) {
	f := newRebaseFixture(t)
	// The booted base is B again: a projected stack, not a moved one.
	setBootedBase(t, f.rt, f.base)
	if out := f.fire(t); out.Minted != "" || f.noticeCount(t) != 0 {
		t.Fatalf("projected state minted %+v", out)
	}
	// The booted base's objects are missing (residual R3): reconcile still
	// answers stack_base_moved for a stack on another base, but the objects
	// are not in the guest repository, so no round and no notice (the fetch
	// supplies them first).
	missing := "dddddddddddddddddddddddddddddddddddddddd"
	setBootedBase(t, f.rt, missing)
	if out := f.fire(t); out.Minted != "" || out.Reason != "the booted base's objects are missing" {
		t.Fatalf("missing objects minted or misreported %+v", out)
	}
	if status := f.rt.sourceProjectionStatus(); status == nil || status.State != sourceproj.StateBaseMoved {
		t.Fatalf("fixture: want stack_base_moved with absent objects, got %+v", status)
	}
	for n := 1; n <= 3; n++ {
		if f.notice(t, n) != nil {
			t.Fatalf("attempt %d exists", n)
		}
	}
}

func TestRebaseNoticeEachSwitchOffMintsNothing(t *testing.T) {
	for _, key := range []string{store.SettingSourceRebaseOffer, store.SettingTextureRootDesk} {
		f := newRebaseFixture(t)
		f.setSwitch(t, key, false)
		if out := f.fire(t); out.Minted != "" || f.noticeCount(t) != 0 {
			t.Fatalf("%s off: minted %+v", key, out)
		}
		if _, armed := f.rt.rebaseNoticeDueAt(); armed {
			t.Fatalf("%s off: a timer is armed", key)
		}
	}
	// source.projection off: reconcile is disabled and nothing is read.
	f := newRebaseFixture(t)
	t.Setenv(sourceproj.EnvSwitch, "0")
	sourceproj.SetSetting(false)
	if out := f.fire(t); out.Minted != "" || f.noticeCount(t) != 0 {
		t.Fatalf("projection off: minted %+v", out)
	}
	// Turning everything back on mints (the switch is the only difference).
	t.Setenv(sourceproj.EnvSwitch, "1")
	if out := f.fire(t); out.Minted == "" {
		t.Fatalf("switches on again: %+v", out)
	}
}

func TestRebaseNoticeNoLiveRootMintsNothingAndUsesNoAttempt(t *testing.T) {
	// D8: a computer whose store has no root marker.
	rt, updaterRoot, repo, base := projectionRuntime(t, "computer-noroot")
	rt.selfdevRouteOwnerID = "owner-noroot"
	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, "computer-noroot", "op-a", base, patch)
	moved := commitOnTop(t, repo, base, map[string]string{"frontend/public/other.txt": "platform\n"})
	setBootedBase(t, rt, moved)
	for _, key := range []string{store.SettingSourceRebaseOffer, store.SettingTextureRootDesk} {
		if err := rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: "owner-noroot", ComputerID: "computer-noroot", Key: key, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
	rt.bootedAt = time.Now().Add(-time.Hour)
	t.Cleanup(rt.stopRebaseNotice)
	if out := rt.fireRebaseNotice(context.Background()); out.Minted != "" {
		t.Fatalf("no marker minted %+v", out)
	}
	// A settled (not live) master: its trajectory no longer takes a root run.
	if err := rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: "owner-noroot", ComputerID: "computer-noroot", Key: store.SettingTextureRootDesk, Value: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.ClaimTextureRoot(context.Background(), nil, store.TextureRoot{OwnerID: "owner-noroot", ComputerID: "computer-noroot", DocumentID: "doc-missing", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if out := rt.fireRebaseNotice(context.Background()); out.Minted != "" {
		t.Fatalf("a marked master that does not exist minted %+v", out)
	}
	for n := 1; n <= 3; n++ {
		rec, err := rt.store.GetCommitmentRecord(context.Background(), "owner-noroot", "computer-noroot", types.StackRebaseNoticeID("op-a", moved, n))
		if err != nil || rec != nil {
			t.Fatalf("attempt %d used up: %v %v", n, rec, err)
		}
	}
}

func TestRebaseNoticeAnAnsweredAttemptIsNotDoubled(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t)
	f.rootCast(t, "cast-root-1", f.noticeID(1)) // the root answered; its round is open
	// Long past the answer window: a Cast named it and the round is open, so
	// the attempt is live, not closed.
	f.now = f.now.Add(3 * time.Hour)
	if out := f.fire(t); out.Minted != "" || f.noticeCount(t) != 1 {
		t.Fatalf("a live, answered attempt was doubled: %+v (notices %d)", out, f.noticeCount(t))
	}
}

func TestRebaseNoticeUnansweredAttemptClosesAfterTheWindow(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t)
	f.now = f.now.Add(29 * time.Minute)
	if out := f.fire(t); out.Minted != "" {
		t.Fatalf("inside the answer window minted %+v", out)
	}
	f.now = f.now.Add(2 * time.Minute)
	out := f.fire(t)
	if out.Minted != f.noticeID(2) || out.Attempt != 2 {
		t.Fatalf("after the window want attempt 2, got %+v", out)
	}
	if f.notice(t, 1) == nil || f.notice(t, 2) == nil {
		t.Fatal("both attempts must stay on the tape; an attempt is never reopened")
	}
}

func TestRebaseNoticeARoundClosedWithoutAClaimClosesTheAttempt(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t)
	f.rootCast(t, "cast-root-1", f.noticeID(1))
	f.closeRound(t, "report-1") // management missed its turn: the round closed with no assignment
	out := f.fire(t)
	if out.Minted != f.noticeID(2) {
		t.Fatalf("a closed round with no claim must allow attempt 2, got %+v", out)
	}
}

func TestRebaseNoticeCapsAtThreeAttemptsAndStops(t *testing.T) {
	f := newRebaseFixture(t)
	for want := 1; want <= 3; want++ {
		out := f.fire(t)
		if out.Attempt != want || out.Minted != f.noticeID(want) {
			t.Fatalf("fire %d: %+v", want, out)
		}
		f.now = f.now.Add(time.Hour)
	}
	for i := 0; i < 3; i++ {
		if out := f.fire(t); out.Minted != "" || out.Rearm != 0 {
			t.Fatalf("after attempt 3 a fire minted or re-armed: %+v", out)
		}
		f.now = f.now.Add(time.Hour)
	}
	if got := f.noticeCount(t); got != 3 {
		t.Fatalf("%d notices on the tape, want exactly 3", got)
	}
}

func TestRebaseNoticeClaimEndsTheMinting(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t)
	f.claim(t, 1)
	f.now = f.now.Add(3 * time.Hour)
	if out := f.fire(t); out.Minted != "" || out.Rearm != 0 {
		t.Fatalf("a claimed pair minted or re-armed: %+v", out)
	}
	if f.noticeCount(t) != 1 {
		t.Fatalf("notices %d, want 1", f.noticeCount(t))
	}
}

func TestRebaseNoticeCrashBootMintsTheNextAttemptNotTheOld(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t) // attempt 1, minted in the previous boot
	pre := f.notice(t, 1)
	// A crash: a new boot, markerless. The attempt predates it.
	f.now = f.now.Add(time.Minute)
	f.rt.bootedAt = f.now
	f.rt.bootRestartPlanned = false
	// Before the stay-up interval the timer has not fired; the arming time is
	// the earliest a mint can happen.
	if _, err := f.rt.ReconcileSourceProjection(context.Background(), "boot"); err != nil {
		t.Fatal(err)
	}
	due, armed := f.rt.rebaseNoticeDueAt()
	if !armed || due.Before(f.rt.bootedAt.Add(f.rt.rebaseStayUp)) {
		t.Fatalf("crash boot armed=%v due=%s; want not before bootedAt + stay-up", armed, due)
	}
	if f.noticeCount(t) != 1 {
		t.Fatal("a mint happened before the interval")
	}
	// The interval has passed: the pre-boot attempt is closed by the crash.
	f.now = f.rt.bootedAt.Add(f.rt.rebaseStayUp + time.Second)
	out := f.fire(t)
	if out.Minted != f.noticeID(2) {
		t.Fatalf("want attempt 2 after the crash, got %+v", out)
	}
	if still := f.notice(t, 1); still == nil || still.RecordID != pre.RecordID {
		t.Fatal("attempt 1 must stay on the tape, closed; never rewritten")
	}
}

func TestRebaseNoticePlannedRestartKeepsALiveAttempt(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t)
	f.rootCast(t, "cast-root-1", f.noticeID(1)) // answered, round open
	f.now = f.now.Add(2 * time.Minute)
	f.rt.bootedAt = f.now
	f.rt.bootRestartPlanned = true // a planned restart parks and resumes a live round
	f.now = f.now.Add(f.rt.rebaseStayUp + time.Minute)
	if out := f.fire(t); out.Minted != "" || f.noticeCount(t) != 1 {
		t.Fatalf("a planned restart doubled a live attempt: %+v", out)
	}
}

func TestRebaseNoticeGoesToTheCurrentRoot(t *testing.T) {
	f := newRebaseFixture(t)
	// The master is replaced: the marker names a new document.
	seedOwnerActDocument(t, f.s, f.owner, f.computer, "doc-master-2")
	marker, err := f.s.GetTextureRoot(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.ClaimTextureRoot(context.Background(), &marker, store.TextureRoot{OwnerID: f.owner, ComputerID: f.computer, DocumentID: "doc-master-2", Generation: 2, PredecessorDocumentID: "doc-master"}); err != nil {
		t.Fatal(err)
	}
	out := f.fire(t)
	rec := f.notice(t, 1)
	if out.Minted == "" || rec == nil || rec.Inbox != currentTextureAgentID("doc-master-2") || rec.SubjectRefs["doc_id"] != "doc-master-2" {
		t.Fatalf("the notice must go to the current master: %+v %+v", out, rec)
	}
}

func TestRebaseNoticeRearmsWhileAttemptsRemain(t *testing.T) {
	f := newRebaseFixture(t)
	out := f.fire(t)
	if out.Rearm <= 0 {
		t.Fatalf("attempt 1 minted with attempts remaining must re-arm: %+v", out)
	}
	f.now = f.now.Add(time.Hour)
	f.fire(t) // attempt 2
	f.now = f.now.Add(time.Hour)
	if out = f.fire(t); out.Attempt != 3 || out.Rearm != 0 {
		t.Fatalf("attempt 3 is the last: no re-arm; got %+v", out)
	}
}

func TestRebaseNoticeShowsToTheRootAsARebaseNoticeNotAClosure(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t)
	rec := f.notice(t, 1)
	if got := inboxRecordKind(*rec); got != "rebase_notice" {
		t.Fatalf("the root reads the notice as %q, want rebase_notice (not closure)", got)
	}
}

// The setting handler arms or stops the timer: off is the live rollback.
func TestRebaseOfferSettingOffStopsTheTimer(t *testing.T) {
	f := newRebaseFixture(t)
	if _, err := f.rt.ReconcileSourceProjection(context.Background(), "cast"); err != nil {
		t.Fatal(err)
	}
	if _, armed := f.rt.rebaseNoticeDueAt(); !armed {
		t.Fatal("fixture: the timer must be armed")
	}
	f.rt.applyRebaseOfferSetting(false)
	if _, armed := f.rt.rebaseNoticeDueAt(); armed {
		t.Fatal("turning the switch off must stop the timer")
	}
}
