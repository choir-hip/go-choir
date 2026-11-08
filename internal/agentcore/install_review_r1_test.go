package agentcore

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Build review r1 (docs/design/desk-install-review-2026-11-08.md section 8).
// Each test fails without its fix:
//
//	1  TestOwnerAndOperationReviewsOfOneReleaseDoNotCollide
//	2  TestInstallVerdictRefusesAPacketThatDrifted
//	3  TestPlannedRestartResumesAnAcceptedRowAndAnOwnerRetryClosesIt
//	4  TestOperationPassDoesNotRebuildWhileTheReviewIsOwed
//	5  TestABusyManagementDefersTheTickInsteadOfEndingTheChain
//	7  TestADueWakeWithNoOwedRecordClosesTheRow
//	9  cmd/choir TestInstallAnsweringAReviewWaitIsNotAnError

// Item 1: a package operation's review and an owner's install of the same
// release and subject are two reviews with two clocks. Before the fix both
// used attempt 1 under one record id, the owner's commit was refused
// "already committed" and the request 500ed with an open row and no clock.
func TestOwnerAndOperationReviewsOfOneReleaseDoNotCollide(t *testing.T) {
	w := newOpReviewWorld(t, "review-collide")
	operation := w.propose(t, "op-pkg-review-collide")
	release := w.release(t, operation)
	opOwed := w.waitOwed(t, release)
	if opOwed.Facts.Path != types.InstallReviewPathOperation {
		t.Fatalf("owed = %+v", opOwed.Facts)
	}
	draft, found, err := w.fx.rt.loadPackageDraft(operation)
	if err != nil || !found {
		t.Fatalf("draft: %v %v", found, err)
	}
	// The owner installs the same release (same bytes) while the operation's
	// review is owed.
	_, _, refusal := w.fx.rt.installPackage(context.Background(), w.owner(), w.fx.computerID, packageInstallRequest{
		ReleaseID: draft.ReleaseID, Record: draft.Record, TreePatch: draft.TreePatch,
	})
	if refusal == nil || refusal.reason != reasonInstallReviewOwed || refusal.status != http.StatusAccepted {
		t.Fatalf("owner install = %+v, want the 202 (before the fix: a 500 install_store)", refusal)
	}
	if refusal.owed == opOwed.Record.RecordID {
		t.Fatal("the owner's review shares the operation's owed record")
	}
	// Two reviews are open, each with its own record; the owner's has its clock.
	list, err := w.fx.rt.installReviewOwedList(context.Background(), w.owner(), w.fx.computerID, release)
	if err != nil || len(list) != 2 {
		t.Fatalf("reviews = %d, %v", len(list), err)
	}
	var due int
	wakes, _ := w.fx.rt.store.ListUnprojectedActorWakes(context.Background())
	for _, wk := range wakes {
		if wk.Kind == types.InstallReviewDueWakeKind {
			due++
		}
	}
	if due != 2 {
		t.Fatalf("the owner review's clock = %d wakes, want a tick and a deadline", due)
	}
	// A row whose owed record belongs to the other path is refused, never
	// waited on with no clock.
	row := store.PackageInstallPending{OwnerID: w.owner(), ComputerID: w.fx.computerID, SubjectDigest: "other-subject", Attempt: 1, ReleaseID: release,
		Package: newPackageName, Action: packageActionInstall, Record: draft.Record, TreePatch: draft.TreePatch, Change: encodePendingChange(installChange{}),
		OwedRecordID: opOwed.Record.RecordID}
	if err := w.fx.rt.store.PutPackageInstallPending(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	if refusal := w.fx.rt.healPendingInstall(context.Background(), w.owner(), w.fx.computerID, row); refusal == nil {
		t.Fatal("heal accepted an owed record of another path")
	}
	// Each review is judged by its own packet ref; the owner's accept installs
	// and leaves the operation's review open.
	run := types.RunRecord{RunID: "run-mgmt-r1", OwnerID: w.owner(), ComputerID: w.fx.computerID, AgentID: persistentManagementAgentID(w.owner()), AgentProfile: agentprofile.Management}
	reduce := &rlmCallReduction{active: true, mb: w.fx.rt, st: w.fx.rt.store, ledger: w.fx.rt.store, rec: &run, scope: ReductionScope{RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, DeskAgentID: run.AgentID}}
	if _, err := reduce.commitInstallVerdictIntent(context.Background(), stagedVerdict(release, refusal.owed, true)); err != nil {
		t.Fatalf("owner review verdict: %v", err)
	}
	w.fx.rt.installReviewRerun.Wait()
	if len(w.installs(t)) != 1 {
		t.Fatalf("installs = %d", len(w.installs(t)))
	}
}

// Item 2: a verdict judges the review whose packet the desk read. D2 is read,
// D2 closes, a newer install opens D4; a verdict naming D2 must not seal D4.
func TestInstallVerdictRefusesAPacketThatDrifted(t *testing.T) {
	f := newNotesReviewFixture(t)
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("first request")
	}
	d2 := f.packetRef()
	// The packet carries the ref.
	// D2 times out; the owner installs again and D4 opens on the same release.
	f.advance(types.InstallReviewOwedDeadline + time.Minute)
	if err := f.due(t, types.InstallReviewStageDeadline, 0); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("second request")
	}
	d4 := f.packetRef()
	if d2 == d4 {
		t.Fatal("no newer review opened")
	}
	if _, err := f.verdictRef(d2, true, 0.9, nil, ""); err == nil {
		t.Fatal("a verdict for the packet D2 sealed the newer review")
	}
	if _, err := f.verdictRef("install-review-owed:nonesuch", true, 0.9, nil, ""); err == nil || !strings.Contains(err.Error(), "no review") {
		t.Fatalf("a ref naming no review = %v", err)
	}
	if recs := reviewRecords(t, f.e, f.id); len(recs) != 0 || f.e.records(t) != 0 {
		t.Fatalf("a refused verdict sealed %d record(s) / installed %d", len(recs), f.e.records(t))
	}
	if row := f.pendingRow(t); !row.Open() || row.Attempt != 2 {
		t.Fatalf("D4 = %+v", row)
	}
	// The verdict that echoes D4's packet lands.
	if _, err := f.verdictRef(d4, true, 0.9, nil, ""); err != nil {
		t.Fatal(err)
	}
	f.e.rt.installReviewRerun.Wait()
	if f.e.records(t) != 1 {
		t.Fatalf("installs = %d", f.e.records(t))
	}
}

// Item 3: an accepted row's re-run is durable. The close open -> accepted mints
// a re-run wake in its own transaction; a planned restart (marker present: no
// crash closer) delivers it and the install finishes; and an owner retry that
// installs the same subject closes the accepted row.
func TestPlannedRestartResumesAnAcceptedRowAndAnOwnerRetryClosesIt(t *testing.T) {
	seal := func(f *reviewFixture, t *testing.T) store.PackageInstallPending {
		t.Helper()
		e := f.e
		if code, _, _ := f.post(t); code != http.StatusAccepted {
			t.Fatal("pend")
		}
		owed, _, err := e.rt.installReviewOwedOn(context.Background(), e.owner, e.computer, f.id)
		if err != nil {
			t.Fatal(err)
		}
		info, err := e.rt.installReviewSubjectOf(context.Background(), e.owner, e.computer, owed)
		if err != nil {
			t.Fatal(err)
		}
		if refusal := e.rt.sealInstallVerdict(context.Background(), e.owner, e.computer, info.Subject, info.Plan, info.Checks, nil, true, "", time.Now()); refusal != nil {
			t.Fatal(refusal)
		}
		row := f.pendingRow(t)
		if ok, err := e.rt.store.ClosePackageInstallPending(context.Background(), e.owner, e.computer, row.SubjectDigest, row.Attempt, store.PackageInstallPendingOpen, store.PackageInstallPendingAccepted, ""); err != nil || !ok {
			t.Fatal(err)
		}
		return f.pendingRow(t)
	}
	rerunWakes := func(f *reviewFixture) []string {
		wakes, err := f.e.rt.store.ListUnprojectedActorWakes(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, w := range wakes {
			if w.Kind == types.InstallReviewRerunWakeKind {
				out = append(out, w.Content)
			}
		}
		return out
	}

	// A planned restart: the accepted row survives (the crash closer does not
	// run), its re-run wake is delivered, the install finishes, the row closes
	// installed.
	f := newNotesReviewFixture(t)
	row := seal(f, t)
	wakes := rerunWakes(f)
	if len(wakes) != 1 {
		t.Fatalf("re-run wakes = %v, want one minted with the close", wakes)
	}
	if f.e.records(t) != 0 {
		t.Fatal("installed before the re-run")
	}
	if err := f.e.rt.HandleInstallReviewRerun(context.Background(), f.e.owner, f.e.computer, persistentManagementAgentID(f.e.owner), wakes[0]); err != nil {
		t.Fatal(err)
	}
	if got := f.pendingRow(t); got.State != store.PackageInstallPendingInstalled || f.e.records(t) != 1 {
		t.Fatalf("after the planned-restart wake: row %s, installs %d", got.State, f.e.records(t))
	}
	// A replayed wake does nothing.
	if err := f.e.rt.HandleInstallReviewRerun(context.Background(), f.e.owner, f.e.computer, persistentManagementAgentID(f.e.owner), wakes[0]); err != nil || f.e.records(t) != 1 {
		t.Fatalf("replay: %v, installs %d", err, f.e.records(t))
	}
	_ = row

	// A crash boot still closes the accepted row, and its pre-boot wake then
	// finds nothing to run.
	g := newReviewFixture(t, `{"grant":"ui.read.kind","args":["mail"]}`)
	seal(g, t)
	gw := rerunWakes(g)
	g.e.rt.closeOpenObligationsAtBoot(context.Background())
	if got := g.pendingRow(t); got.State != store.PackageInstallPendingInterrupted {
		t.Fatalf("crash boot left the row %s", got.State)
	}
	if err := g.e.rt.HandleInstallReviewRerun(context.Background(), g.e.owner, g.e.computer, persistentManagementAgentID(g.e.owner), gw[0]); err != nil || g.e.records(t) != 0 {
		t.Fatalf("a pre-boot wake after a crash: %v, installs %d", err, g.e.records(t))
	}

	// An owner retry installs the same subject and closes the accepted row.
	h := newReviewFixture(t, `{"grant":"ui.read.kind","args":["calendar"]}`)
	seal(h, t)
	if code, _, reason := h.post(t); code != http.StatusCreated {
		t.Fatalf("retry = %d %s", code, reason)
	}
	if got := h.pendingRow(t); got.State != store.PackageInstallPendingInstalled {
		t.Fatalf("FM: the retry left the accepted row %s (Library would show pending_review forever)", got.State)
	}
}

// Item 4: a 60-second pass over an owed operation review reads the owed record
// and the ledger; it does not rebuild the release.
func TestOperationPassDoesNotRebuildWhileTheReviewIsOwed(t *testing.T) {
	w := newOpReviewWorld(t, "review-norebuild")
	operation := w.propose(t, "op-pkg-review-norebuild")
	release := w.release(t, operation)
	w.waitOwed(t, release)
	w.pass()
	builder := w.fx.rt.packageBuilder.(*manifestBuilder)
	before := builder.builds
	for i := 0; i < 5; i++ {
		w.pass()
	}
	if after := builder.builds; after != before {
		t.Fatalf("5 pending passes built the release %d more time(s)", after-before)
	}
	// Once the verdict is sealed the pass builds and gates as usual.
	if _, err := w.judge(t, release, true, ""); err != nil {
		t.Fatal(err)
	}
	w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
}

// Item 5: a due tick that finds management busy is deferred, not spent, so a
// later one fires and the notice chain survives a busy moment.
func TestABusyManagementDefersTheTickInsteadOfEndingTheChain(t *testing.T) {
	f := newNotesReviewFixture(t)
	e := f.e
	if code, _, _ := f.post(t); code != http.StatusAccepted {
		t.Fatal("pend")
	}
	now := time.Now()
	if err := e.rt.store.CreateRun(context.Background(), types.RunRecord{
		RunID: "run-busy", OwnerID: e.owner, ComputerID: e.computer, AgentID: persistentManagementAgentID(e.owner), AgentProfile: agentprofile.Management,
		State: types.RunPending, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	e.rt.cfg.ComputerID = e.computer
	f.advance(types.InstallReviewOwedNoticeAfter + time.Second)
	if err := f.due(t, types.InstallReviewStageTick, 0); err == nil {
		t.Fatal("a tick over a busy management was spent (the chain would end)")
	}
	if n, _ := e.rt.installReviewNotices(context.Background(), e.owner, e.computer, f.id, f.subjectDigest(), "", 1); n != 0 {
		t.Fatalf("a notice went out to a busy management: %d", n)
	}
}

// Item 7: the deadline wake of a row with no owed record closes it instead of
// deferring forever.
func TestADueWakeWithNoOwedRecordClosesTheRow(t *testing.T) {
	g := newReviewFixture(t, `{"grant":"ui.read.kind","args":["mail"]}`)
	row := store.PackageInstallPending{
		OwnerID: g.e.owner, ComputerID: g.e.computer, SubjectDigest: g.subjectDigest(), Attempt: 1, ReleaseID: g.id, Package: installName, Action: packageActionInstall,
		Record: g.raw, TreePatch: g.tree, Change: encodePendingChange(installChange{}), OwedRecordID: types.InstallReviewOwedRecordID(g.id, g.subjectDigest(), "", 1),
	}
	if err := g.e.rt.store.PutPackageInstallPending(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	if err := g.due(t, types.InstallReviewStageDeadline, 0); err != nil {
		t.Fatalf("the deadline wake of a recordless row deferred: %v", err)
	}
	if got := g.pendingRow(t); got.Open() {
		t.Fatalf("row still %s", got.State)
	}
}

func stagedVerdict(release, ref string, accept bool) yaegikernel.StagedIntent {
	decision := "accept"
	if !accept {
		decision = "reject"
	}
	return yaegikernel.StagedIntent{LocalID: "v-r1", Kind: yaegikernel.IntentInstallVerdict, TargetRef: release, PacketRef: ref, Decision: decision, PWithinCeiling: 0.9}
}

