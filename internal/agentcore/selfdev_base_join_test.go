package agentcore

// A plain (not host-built) self-development release carries the base commit
// its candidate was authored against, so the updater's state-compatibility
// check refuses a candidate frozen on a base that is no longer the booted one
// (docs/problems/selfdev-apply-has-no-base-join-2026-10-26.md). Ways this
// could fail, written first:
//   - an old-base candidate installs over a newer booted base (the defect);
//   - the refusal is a bare error that does not name both commits;
//   - the join is gated on a switch (source projection) and silently absent;
//   - a case difference, or a dev build whose booted base names no commit,
//     refuses every candidate (refuse-everything);
//   - a short commit meets a full one by prefix and passes;
//   - a bundle with no source patch is refused, or claims a base it does not
//     have;
//   - an operation the updater journal already settled as applied is refused
//     (or fails) when a crash boot settles it after the base moved;
//   - a rollback to a prior release is refused by the join.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

const (
	joinTestBaseA = "0123456789abcdef0123456789abcdef01234567"
	joinTestBaseB = "89abcdef0123456789abcdef0123456789abcdef"

	plainSourcePatch = "diff --git a/workspace/platform/frontend/app.css b/workspace/platform/frontend/app.css\n" +
		"--- a/workspace/platform/frontend/app.css\n+++ b/workspace/platform/frontend/app.css\n@@ -1 +1 @@\n-a\n+b\n"
)

func (fx *derivableSelfDevFixture) setBootedBuildCommit(t *testing.T, commit string) {
	t.Helper()
	if err := os.WriteFile(fx.bootedManifest, []byte("contract=choir-guest-image-v1\nbuild_commit="+commit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// joinFixture is a guest booted on bootedCommit that freezes a frontend-only
// candidate whose bundle records sourceBase (empty: no source patch).
func joinFixture(t *testing.T, name, bootedCommit, sourceBase string) *derivableSelfDevFixture {
	t.Helper()
	// The join is not gated: source projection stays off, as it is by default.
	sourceproj.SetSetting(false)
	t.Setenv(sourceproj.EnvSwitch, "")
	fx := newDerivableSelfDevFixtureBooted(t, "computer-join-"+name, bootedCommit)
	fx.sourceBase = sourceBase
	return fx
}

func (fx *derivableSelfDevFixture) decide(t *testing.T, key string) selfdev.Operation {
	t.Helper()
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, key)
	fx.appendDecision(t, operation, bundleDigest, "decision-"+key)
	return operation
}

func (fx *derivableSelfDevFixture) applyPlain(t *testing.T, key string) selfdev.Operation {
	t.Helper()
	operation := fx.decide(t, key)
	return fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
}

func (fx *derivableSelfDevFixture) currentManifest(t *testing.T) updater.ReleaseManifest {
	t.Helper()
	manifest, err := updater.ReadCurrentManifest(fx.updaterRoot)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func (fx *derivableSelfDevFixture) requireRefusedForBase(t *testing.T, key string, wantCommits ...string) {
	t.Helper()
	operation := fx.decide(t, key)
	// A typed refusal installs nothing, so it settles failed (a settled
	// state), never degraded: degraded is an open state meaning "applied,
	// with an unsettled tail" and would hold every new round (review 311).
	got := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	for _, want := range append([]string{"base_moved_since_freeze"}, wantCommits...) {
		if !strings.Contains(got.TerminalError, want) {
			t.Fatalf("terminal error %q does not name %q", got.TerminalError, want)
		}
	}
	fx.requireRefusalSettled(t)
}

// requireRefusalSettled is what a refused apply must leave behind: nothing
// installed, nothing restarted, no open operation, the canonical transition
// discharged, and a new round able to start.
func (fx *derivableSelfDevFixture) requireRefusalSettled(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if _, err := updater.ReadCurrentManifest(fx.updaterRoot); err == nil {
		t.Fatal("a refused candidate became the current release")
	}
	if fx.service.restarts != 0 {
		t.Fatalf("restarts = %d; a refusal restarts nothing", fx.service.restarts)
	}
	if fx.rt.SelfDevelopmentApplyMaterializing(ctx) {
		t.Fatal("a refused candidate leaves the operation materializing (desks held)")
	}
	if open, found, err := fx.operations.OpenOperation(ctx, fx.computerID); err != nil || found {
		t.Fatalf("open operation after a refusal = %+v found=%v err=%v; a refusal must settle", open, found, err)
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("events = %v; a refused candidate is not recorded as applied", kinds)
	}
	if kinds[computerevent.EventMaterializationFailed] != 1 {
		t.Fatalf("events = %v; a refusal discharges the pending transition with one materialization_failed", kinds)
	}
	if head, err := fx.store.Head(ctx, fx.computerID); err != nil || head == nil || head.PendingTransitionRef != "" {
		t.Fatalf("head after a refusal = %+v err=%v; the pending transition must be discharged", head, err)
	}
	// A new self-development round opens (it was ErrOperationOpen while the
	// refused operation sat in degraded).
	if _, err := fx.operations.Start(ctx, selfdev.StartRequest{
		ComputerID: fx.computerID, IdempotencyKey: "op-after-refusal", TrajectoryID: fx.trajectoryID,
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	}); err != nil {
		t.Fatalf("a new round after a refused apply: %v", err)
	}
}

func TestSelfDevPlainReleaseCarriesItsSourceBase(t *testing.T) {
	fx := joinFixture(t, "carries", joinTestBaseA, joinTestBaseA)
	fx.applyPlain(t, "op-join-carries")
	if got := fx.currentManifest(t).BaseCommit; got != joinTestBaseA {
		t.Fatalf("plain release base_commit = %q, want the bundle's source patch base %q", got, joinTestBaseA)
	}
}

func TestSelfDevPlainReleaseRefusedWhenTheBaseMovedSinceFreeze(t *testing.T) {
	fx := joinFixture(t, "moved", joinTestBaseB, joinTestBaseA)
	fx.requireRefusedForBase(t, "op-join-moved", joinTestBaseA, joinTestBaseB)
}

func TestSelfDevPlainReleaseBaseCommitCaseIsNotSignificant(t *testing.T) {
	fx := joinFixture(t, "case", joinTestBaseA, strings.ToUpper(joinTestBaseA))
	fx.applyPlain(t, "op-join-case")
}

// A short commit is not a prefix match: it cannot be proven to name the booted
// commit, so it is refused with the reason.
func TestSelfDevPlainReleaseShortBaseCommitIsRefused(t *testing.T) {
	fx := joinFixture(t, "short", joinTestBaseA, joinTestBaseA[:12])
	fx.requireRefusedForBase(t, "op-join-short", joinTestBaseA[:12], joinTestBaseA)
}

// A dev build's booted base names no verifiable commit (build_commit=local, or
// a dirty tree): the join is not claimed, so the candidate applies and the
// release records no base. Refusing here would refuse every candidate on a
// dev build.
func TestSelfDevPlainReleaseOnABootedBaseWithNoVerifiableCommit(t *testing.T) {
	for _, booted := range []string{"local", joinTestBaseA + "-dirty"} {
		fx := joinFixture(t, "dev", booted, joinTestBaseA)
		fx.applyPlain(t, "op-join-dev")
		if got := fx.currentManifest(t).BaseCommit; got != "" {
			t.Fatalf("booted %q: release claims base_commit %q on a base it could not verify", booted, got)
		}
	}
}

// A guest with no booted base image manifest wired cannot join either.
func TestSelfDevPlainReleaseOnAGuestWithNoBootedBaseWired(t *testing.T) {
	fx := joinFixture(t, "unwired", "", joinTestBaseA)
	fx.applyPlain(t, "op-join-unwired")
	if got := fx.currentManifest(t).BaseCommit; got != "" {
		t.Fatalf("release claims base_commit %q with no booted base wired", got)
	}
}

// A bundle with no source patch records no base: nothing to join, and the
// release claims none.
func TestSelfDevPlainReleaseWithNoSourcePatchClaimsNoBase(t *testing.T) {
	fx := joinFixture(t, "nopatch", joinTestBaseB, "")
	fx.applyPlain(t, "op-join-nopatch")
	if got := fx.currentManifest(t).BaseCommit; got != "" {
		t.Fatalf("release without a source patch claims base_commit %q", got)
	}
}

// b@c's selfdev-83fbdfa9: an operation whose updater journal is terminal
// `applied` is settled by a crash boot from the journal, never refused by the
// join, even though the booted base is no longer the one it applied on.
func TestCrashBootSettlesAppliedPlainOperationAfterTheBaseMoved(t *testing.T) {
	fx := joinFixture(t, "settle", joinTestBaseA, joinTestBaseA)
	ctx := context.Background()
	fx.controlFail.Store(true)
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-join-settle")
	fx.appendDecision(t, operation, bundleDigest, "decision-op-join-settle")
	// The apply finished in the updater; recording it failed (control plane
	// down), so the operation is parked materializing with a terminal journal.
	fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)
	applyKey := "selfdev-apply-" + fx.decisionDigest(t, "op-join-settle")
	waitForTerminalJournal(t, fx, applyKey)
	fx.rt.eventAppender.SetPostCommitObserver(func(computerevent.EventKind) {})
	waitForStableApplyCalls(fx)
	fx.controlFail.Store(false)

	// The platform moved the booted base while the guest was down.
	fx.setBootedBuildCommit(t, joinTestBaseB)
	fx.useDurableAttemptCount(t)
	fx.rt.closeSelfDevelopmentOperationsAfterCrash(ctx)
	callsBefore := fx.applyCalls.Load()
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)

	got := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if got.State == selfdev.StateDegraded || strings.Contains(got.TerminalError, "base_moved_since_freeze") {
		t.Fatalf("settlement refused by the join: %+v", got)
	}
	if calls := fx.applyCalls.Load(); calls != callsBefore {
		t.Fatalf("crash settlement called the updater again (%d -> %d)", callsBefore, calls)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 1 {
		t.Fatalf("events = %v; want the journaled apply recorded once", kinds)
	}
}

// A rollback to a prior plain release is never refused by the join: the
// rollback's manifest drops the base claim the pinned release carried, so a
// computer whose base moved still returns to its prior release. A release
// that never carried one (every older manifest) is unchanged, and a layered
// release keeps the base it was built on (its closure resolves against it).
func TestRollbackToAPlainReleaseIsNotRefusedByTheBaseJoin(t *testing.T) {
	fx := joinFixture(t, "rollback", joinTestBaseA, joinTestBaseA)
	applied := fx.applyPlain(t, "op-join-rollback")
	if fx.currentManifest(t).BaseCommit != joinTestBaseA {
		t.Fatal("the plain release to roll back to does not carry its base")
	}
	fx.setBootedBuildCommit(t, joinTestBaseB)

	pinned, releaseDir, err := updater.ReadPinnedManifest(fx.updaterRoot, applied.ReleaseDigest)
	if err != nil {
		t.Fatal(err)
	}
	decision := strings.Repeat("d", 64)
	manifest, err := rollbackApplyManifest(pinned, decision)
	if err != nil {
		t.Fatal(err)
	}
	request := updater.ApplyRequest{
		ComputerID: fx.computerID, RealizationID: "realization-derivable", OperationID: "op-join-rollback-apply",
		IdempotencyKey: "selfdev-rollback-apply-" + decision, AcceptedEventHead: decision,
		SourceDir: releaseDir, Manifest: manifest,
	}
	request.RequestCommitment, err = updater.ComputeApplyRequestCommitment(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fx.engine.Apply(context.Background(), request)
	if err != nil || result.Outcome != "applied" {
		t.Fatalf("rollback to a plain release on a moved base = %+v err=%v, want applied", result, err)
	}
}

func TestRollbackManifestBaseClaim(t *testing.T) {
	plain := updater.ReleaseManifest{Version: updater.ManifestVersion, ComputerID: "c", BaseCommit: joinTestBaseA}
	got, err := rollbackApplyManifest(plain, strings.Repeat("d", 64))
	if err != nil || got.BaseCommit != "" {
		t.Fatalf("plain rollback manifest base_commit = %q err=%v, want none", got.BaseCommit, err)
	}
	old := updater.ReleaseManifest{Version: updater.ManifestVersion, ComputerID: "c"}
	if got, err = rollbackApplyManifest(old, strings.Repeat("d", 64)); err != nil || got.BaseCommit != "" {
		t.Fatalf("older manifest rollback = %q err=%v", got.BaseCommit, err)
	}
	layered := updater.ReleaseManifest{
		Version: updater.ManifestVersion, ComputerID: "c", BaseCommit: joinTestBaseA,
		BaseImageManifestDigest: strings.Repeat("1", 64), ClosureDigest: strings.Repeat("2", 64), LayeringEntrypoint: "x/bin/autoputer",
	}
	if got, err = rollbackApplyManifest(layered, strings.Repeat("d", 64)); err != nil || got.BaseCommit != joinTestBaseA {
		t.Fatalf("layered rollback manifest base_commit = %q err=%v, want it kept", got.BaseCommit, err)
	}
}

func waitForTerminalJournal(t *testing.T, fx *derivableSelfDevFixture, key string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		journaled, found, err := updater.ReadJournalOutcome(fx.updaterRoot, key)
		if err == nil && found && journaled.Terminal {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("apply journal never became terminal (found=%v err=%v)", found, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func waitForStableApplyCalls(fx *derivableSelfDevFixture) {
	for stable, last := 0, int32(-1); stable < 3; {
		fx.rt.selfdevMaterializeMu.Lock()
		calls := fx.applyCalls.Load()
		fx.rt.selfdevMaterializeMu.Unlock()
		if calls == last {
			stable++
		} else {
			stable, last = 0, calls
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// A wired booted manifest that cannot be read (or carries no build commit) is
// not a dev build: the join cannot be proven, so the apply fails closed with a
// typed reason instead of silently dropping the check (review 311, DeepSeek
// F3). The updater is never asked.
func TestSelfDevPlainReleaseOnAnUnreadableWiredManifestIsRefused(t *testing.T) {
	cases := map[string]func(t *testing.T, fx *derivableSelfDevFixture){
		"missing": func(t *testing.T, fx *derivableSelfDevFixture) {
			if err := os.Remove(fx.bootedManifest); err != nil {
				t.Fatal(err)
			}
		},
		"is a directory": func(t *testing.T, fx *derivableSelfDevFixture) {
			if err := os.Remove(fx.bootedManifest); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(fx.bootedManifest, 0o700); err != nil {
				t.Fatal(err)
			}
		},
		"no build_commit": func(t *testing.T, fx *derivableSelfDevFixture) {
			if err := os.WriteFile(fx.bootedManifest, []byte("contract=choir-guest-image-v1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"garbage build_commit": func(t *testing.T, fx *derivableSelfDevFixture) {
			if err := os.WriteFile(fx.bootedManifest, []byte("build_commit=not-a-commit\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			fx := joinFixture(t, "unreadable", joinTestBaseA, joinTestBaseA)
			break_(t, fx)
			operation := fx.decide(t, "op-join-unreadable")
			got := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
			if !strings.Contains(got.TerminalError, "base_unverifiable") {
				t.Fatalf("terminal error %q does not carry the typed reason base_unverifiable", got.TerminalError)
			}
			if calls := fx.applyCalls.Load(); calls != 0 {
				t.Fatalf("the updater was called %d times for an apply that fails closed before it", calls)
			}
			fx.requireRefusalSettled(t)
		})
	}
}

// A crash boot that finds a decided operation whose updater journal is
// terminal `refused` settles it failed from the journal; the updater is not
// called again (the refusal is recorded, nothing was installed).
func TestCrashBootSettlesARefusedOperationFailedWithoutApplying(t *testing.T) {
	fx := joinFixture(t, "settle-refused", joinTestBaseA, joinTestBaseA)
	ctx := context.Background()
	operation := fx.decideWithoutActing(t, "op-join-settle-refused")
	key := "selfdev-apply-" + fx.decisionDigest(t, "op-join-settle-refused")
	if _, err := fx.engine.Apply(ctx, updater.ApplyRequest{IdempotencyKey: key}); err == nil {
		t.Fatal("a malformed apply request was not refused")
	}
	journaled, found, err := updater.ReadJournalOutcome(fx.updaterRoot, key)
	if err != nil || !found || !journaled.Terminal || journaled.Result.Outcome != "refused" {
		t.Fatalf("journal = %+v found=%v err=%v, want a terminal refused outcome", journaled, found, err)
	}
	fx.rt.closeSelfDevelopmentOperationsAfterCrash(ctx)
	callsBefore := fx.applyCalls.Load()
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	got := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if got.TerminalError == "" {
		t.Fatal("the settled operation records no refusal reason")
	}
	if calls := fx.applyCalls.Load(); calls != callsBefore {
		t.Fatalf("crash settlement called the updater again (%d -> %d)", callsBefore, calls)
	}
	fx.requireRefusalSettled(t)
}

// rollbackFixture applies two plain releases on base A (the second is the
// current one), moves the booted base to B, and opens the rollback to the
// first through the owner API: the shape a layered-or-plain rollback has in
// production.
func rollbackFixture(t *testing.T, name string) (fx *derivableSelfDevFixture, first selfdev.Operation) {
	t.Helper()
	fx = joinFixture(t, name, joinTestBaseA, joinTestBaseA)
	first = fx.applyPlain(t, "op-"+name+"-1")
	fx.applyPlain(t, "op-"+name+"-2")
	fx.setBootedBuildCommit(t, joinTestBaseB)
	return fx, first
}

func (fx *derivableSelfDevFixture) startRollbackTo(t *testing.T, target selfdev.Operation, key string) selfdev.Operation {
	t.Helper()
	ctx := context.Background()
	head := fx.waitForStableHead(t)
	slotID, err := routeledger.RouteSlotID("owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	slot, _, err := fx.ledger.Resolve(ctx, slotID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(selfDevelopmentRollbackRequest{
		ExpectedDesiredHead: head.DesiredEventHead, CurrentAppliedHead: head.EffectiveEventHead, ToAppliedHead: target.EffectiveHead,
		PriorMaterialization: target.MaterializationReceipt, PriorCheckpoint: target.CheckpointRef,
		ExpectedRouteGeneration: slot.Generation, IdempotencyKey: key,
	})
	recorder := httptest.NewRecorder()
	(&APIHandler{rt: fx.rt}).startSelfDevelopmentRollback(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body))), "owner", fx.computerID)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("rollback status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var operation selfdev.Operation
	if err := json.NewDecoder(recorder.Body).Decode(&operation); err != nil {
		t.Fatal(err)
	}
	return operation
}

// A rollback to a plain release drops that release's base claim by design
// (it must stay possible after the base moved), and the dropped claim is
// visible on the operation's terminal result (review 311, DeepSeek F2).
func TestRollbackToAPlainReleaseRecordsTheDroppedBaseClaim(t *testing.T) {
	fx, first := rollbackFixture(t, "rb-note")
	rollback := fx.startRollbackTo(t, first, "rollback-note")
	got := fx.waitForState(t, rollback.OperationID, selfdev.StateRolledBack)
	for _, want := range []string{"base claim dropped", joinTestBaseA} {
		if !strings.Contains(got.TerminalError, want) {
			t.Fatalf("rolled-back operation result %q does not record %q", got.TerminalError, want)
		}
	}
}

// A refused rollback keeps its typed refusal, like an apply: it settles
// failed and discharges the pending transition. Replacing the journaled
// `refused` outcome with a plain error left it rollback_pending, re-arming a
// 60 s retry wake forever and holding every deploy (review 311, Devin F1).
func TestRefusedRollbackSettlesFailed(t *testing.T) {
	fx, first := rollbackFixture(t, "rb-refused")
	// A pinned release directory that is writable is no longer trusted: the
	// updater refuses the rollback's source before any mutation.
	if err := os.Chmod(filepath.Join(fx.updaterRoot, "releases", first.ReleaseDigest), 0o755); err != nil {
		t.Fatal(err)
	}
	rollback := fx.startRollbackTo(t, first, "rollback-refused")
	got := fx.waitForState(t, rollback.OperationID, selfdev.StateFailed)
	if !strings.Contains(got.TerminalError, "pinned release") {
		t.Fatalf("terminal error %q does not carry the updater's refusal", got.TerminalError)
	}
	if fx.rt.SelfDevelopmentApplyMaterializing(context.Background()) {
		t.Fatal("a refused rollback holds the desks")
	}
	if open, found, err := fx.operations.OpenOperation(context.Background(), fx.computerID); err != nil || found {
		t.Fatalf("open operation after a refused rollback = %+v found=%v err=%v", open, found, err)
	}
	if head, err := fx.store.Head(context.Background(), fx.computerID); err != nil || head == nil || head.PendingTransitionRef != "" {
		t.Fatalf("head after a refused rollback = %+v err=%v; the pending transition must be discharged", head, err)
	}
}
