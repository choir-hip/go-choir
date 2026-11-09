package agentcore

// The package branch of the materializer, the package revert and the platform
// layer's separation from it (docs/design/package-operation-decision-2026-11-06.md
// 2.3 and 2.5). Ways this could fail, written first:
//   - a build, uses or install-review refusal still publishes the release;
//   - a crash between publish and record leaves the operation materializing,
//     or settles it by driving the branch again (a second publish);
//   - a listed release with no install is recorded applied, or one with an
//     install is closed failed;
//   - a package revert calls the updater, creates a rollback row, or leaves the
//     install in force;
//   - a platform revert whose prior is a package event restores the package's
//     state, or undoes the package install;
//   - a package operation enters stack bookkeeping (a release readers match,
//     a rebase offer, a clean rebase, the live platform set).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type packageBundleSpec struct {
	patch   []byte
	base    string
	screens map[string][]byte
}

const newPackageName = "demo-new"
const newPackageRoot = "frontend/src/packages/" + newPackageName

// fakePackagePublisher stands in for corpusd's publish-by-decision route.
type fakePackagePublisher struct {
	mu        sync.Mutex
	publishes []platform.PackagePublishByDecisionRequest
	// publish answers a publish; nil echoes the request's release id.
	publish func(platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error)
	// list answers the Library lookup; nil means the publisher offers no
	// lookup route (the fallback on install records).
	list func(operationID string) ([]PackageReleaseRef, error)
}

func (f *fakePackagePublisher) PublishPackageByDecision(_ context.Context, request platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
	f.mu.Lock()
	f.publishes = append(f.publishes, request)
	publish := f.publish
	f.mu.Unlock()
	if publish != nil {
		return publish(request)
	}
	return platform.PackageReleaseView{ReleaseID: request.ReleaseID}, http.StatusCreated, nil
}

// fakeListingPublisher adds the lookup half.
type fakeListingPublisher struct{ *fakePackagePublisher }

func (f fakeListingPublisher) PackageReleasesForOperation(_ context.Context, operationID string) ([]PackageReleaseRef, error) {
	f.mu.Lock()
	list := f.list
	f.mu.Unlock()
	if list == nil {
		return nil, nil
	}
	return list(operationID)
}

func (f *fakePackagePublisher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.publishes)
}

// uses-widening builder: a module that imports an SDK name its manifest does
// not hold the grant of.
type mutatingBuilder struct {
	inner  *fakeModuleBuilder
	mutate func(*pkgruntime.Module)
}

func (b *mutatingBuilder) Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error) {
	module, err := b.inner.Build(ctx, rec, treePatch)
	if err == nil && b.mutate != nil {
		b.mutate(&module)
	}
	return module, err
}

type packageOpWorld struct {
	fx      *derivableSelfDevFixture
	repo    string
	base    string
	builder *fakeModuleBuilder
	pub     *fakePackagePublisher
	files   map[string]string
	// parent, when set, is the package block's parent of the decisions this
	// world appends (an update's track head).
	parent map[string]any
	// screens, when set, replaces the single after screen of a proposal.
	screens map[string][]byte
}

func newPackageOpWorld(t *testing.T, name string) *packageOpWorld {
	t.Helper()
	fx := joinFixture(t, "pkg-"+name, joinTestBaseA, joinTestBaseA)
	repo, base := packageSourceRepo(t)
	w := &packageOpWorld{fx: fx, repo: repo, base: base, builder: &fakeModuleBuilder{}, pub: &fakePackagePublisher{}}
	w.files = map[string]string{
		newPackageRoot + "/choir-package.json":  strings.ReplaceAll(strings.ReplaceAll(draftManifest, `"reader"`, `"`+newPackageName+`"`), "Reader", "Demo new"),
		newPackageRoot + "/App.svelte":          "<p>demo new</p>\n",
		newPackageRoot + "/choir-contract.json": `{"schema": "choir.package-contract/v1", "ready": "[data-demo]"}` + "\n",
	}
	fx.configure(func(rt *Runtime) {
		rt.releaseSourceDir = repo
		rt.packageBuilder = w.builder
		rt.packageModules = pkgruntime.Store{Dir: t.TempDir()}
		rt.packagePublisher = fakeListingPublisher{w.pub}
	})
	return w
}

// propose freezes a package operation (kind package), has it decided by a
// consensus authority and returns it. The materializer's observer drives it
// from there.
func (w *packageOpWorld) propose(t *testing.T, key string) selfdev.Operation {
	t.Helper()
	patch, _ := releasePatch(t, w.repo, w.files)
	screens := w.screens
	if screens == nil {
		screens = map[string][]byte{"home.png": mediaPNG(t, 64, 40, 3, false)}
	}
	w.fx.packageSpec = &packageBundleSpec{patch: patch, base: w.base, screens: screens}
	defer func() { w.fx.packageSpec = nil }()
	operation, bundleDigest, _ := w.fx.seedOperationToAwaitingApproval(t, key)
	if !operation.IsPackage() {
		// The seed returns the awaiting row; re-read for the kind.
		var err error
		if operation, err = w.fx.operations.Get(context.Background(), w.fx.computerID, operation.OperationID); err != nil || !operation.IsPackage() {
			t.Fatalf("the frozen operation is not a package operation: %+v err=%v", operation, err)
		}
	}
	w.appendConsensusDecision(t, operation, bundleDigest, "decision-"+key)
	return operation
}

func (w *packageOpWorld) appendConsensusDecision(t *testing.T, operation selfdev.Operation, bundleDigest, idempotencyKey string) {
	t.Helper()
	head := w.fx.waitForStableHead(t)
	id, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	// The poster's decision receipt, pinned on the tape by its digest: its
	// package block names the draft's parent (the track head of an update).
	pkgBlock := map[string]any{}
	if w.parent != nil {
		pkgBlock["parent"] = w.parent
	}
	receipt := computerevent.Receipt{ReceiptVersion: 1, ReceiptKind: decisionpolicy.DecisionReceiptKind, ReceiptID: "receipt-" + operation.OperationID, Issuer: "corpusd",
		KindFields: map[string]any{"kind": platform.ConsensusKindPackage, "package": pkgBlock}}
	receiptBytes, err := receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	consensus := computerevent.DigestBytes(receiptBytes)
	decision := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: id, ComputerID: w.fx.computerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotencyKey, RequestCommitment: computerevent.ZeroHead,
		TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID, PreviousHead: head.CanonicalEventHead,
		ParentEventID: operation.OperationID,
		ActorProfile:  agentprofile.Management, AuthorityRef: decisionpolicy.DecisionAuthorityRef(consensus), PrivacyClass: "owner",
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		RequireExpectedHead: true, PayloadCommitment: computerevent.ZeroHead, ProposedEffectRef: bundleDigest,
		DecisionRef: strings.Repeat("d", 64), VerifierRefs: append([]string(nil), operation.VerifierRefs...), ReducerVersion: computerevent.ReducerVersionV1,
	}
	target, err := computerevent.CanonicalJSON(map[string]string{"base_head": operation.BaseHead, "bundle_digest": bundleDigest})
	if err != nil {
		t.Fatal(err)
	}
	payloads := []computerevent.EventPayload{
		{Content: []byte(`{"mode":"apply"}`), MediaType: "application/vnd.choir.mode-receipt+json", PrivacyClass: "owner", Direction: computerevent.EventPayloadInput},
		{Content: receiptBytes, MediaType: "application/vnd.choir.consensus-decision-receipt+json", PrivacyClass: "owner", Direction: computerevent.EventPayloadInput},
	}
	if _, _, err := w.fx.appender.AppendNewPayloadSet(context.Background(), decision, computerevent.TransitionInput{TargetStateCommitment: computerevent.DigestBytes(target)}, payloads, nil); err != nil {
		t.Fatal(err)
	}
}

func (w *packageOpWorld) installs(t *testing.T) []store.PackageInstall {
	t.Helper()
	list, err := w.fx.rt.packageInstalls(context.Background(), "owner", w.fx.computerID)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func (w *packageOpWorld) pendingCleared(t *testing.T) {
	t.Helper()
	head := w.fx.waitForStableHead(t)
	if head.PendingTransitionRef != "" {
		t.Fatalf("the pending transition is not discharged: %+v", head)
	}
}

func TestPackageOperationPublishesInstallsAndRecordsWithNoOwnerAct(t *testing.T) {
	w := newPackageOpWorld(t, "apply")
	applyCalls := w.fx.applyCalls.Load()
	operation := w.propose(t, "op-pkg-apply")
	applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)

	if !applied.IsPackage() || applied.PackageReleaseID == "" || applied.PackageInstallID == "" || applied.ReleaseDigest != "" {
		t.Fatalf("applied package operation = %+v; want the release and install ids and no release digest", applied)
	}
	if w.pub.count() != 1 {
		t.Fatalf("publishes = %d, want exactly 1", w.pub.count())
	}
	request := w.pub.publishes[0]
	if request.OperationID != operation.OperationID || request.ClaimedPlatformBase != w.base || len(request.SourcePatch) == 0 || len(request.Media) != 1 ||
		!computerevent.IsSHA256(request.DecisionDigest) || len(request.TreePatch) == 0 || len(request.ReviewPatch) == 0 {
		t.Fatalf("publish request = %+v; want the persisted draft, the claimed base, the source patch, one screen and the decision digest", request)
	}
	if computerevent.DigestBytes(request.Media[0]) != computerevent.DigestBytes(mediaPNG(t, 64, 40, 3, false)) || request.ReleaseID != applied.PackageReleaseID {
		t.Fatalf("publish media or release id = %d bytes %s", len(request.Media[0]), request.ReleaseID)
	}
	installs := w.installs(t)
	if len(installs) != 1 || installs[0].InstallID != applied.PackageInstallID || installs[0].ReleaseID != applied.PackageReleaseID ||
		installs[0].Trigger != store.PackageInstallTriggerSelfdev || installs[0].Action != packageActionInstall {
		t.Fatalf("installs = %+v; want one selfdev install of %s", installs, applied.PackageReleaseID)
	}
	kinds := w.fx.countEventKinds()
	if kinds[computerevent.EventPackageApplied] != 1 || kinds[computerevent.EventMaterializationApplied] != 0 ||
		kinds[computerevent.EventCheckpointPublished] != 0 || kinds[computerevent.EventRouteProjectionUpdated] != 0 {
		t.Fatalf("events = %v; want one package_applied and no platform apply, checkpoint or route", kinds)
	}
	w.pendingCleared(t)
	if w.fx.applyCalls.Load() != applyCalls || w.fx.service.restarts != 0 {
		t.Fatalf("the updater was called (applies %d -> %d, restarts %d)", applyCalls, w.fx.applyCalls.Load(), w.fx.service.restarts)
	}
	// The draft was persisted once, keyed by the operation.
	if draft, found, err := w.fx.rt.loadPackageDraft(applied); err != nil || !found || draft.ReleaseID != applied.PackageReleaseID || draft.Package != newPackageName {
		t.Fatalf("persisted draft = %+v found=%v err=%v", draft, found, err)
	}
	// A second pass over the settled state does nothing.
	w.fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	if w.pub.count() != 1 || len(w.installs(t)) != 1 {
		t.Fatal("a later pass published or installed again")
	}
}

func TestPackageOperationRefusedBeforePublishPublishesNothing(t *testing.T) {
	cases := []struct {
		name   string
		inner  func(*packageOpWorld) PackageModuleBuilder
		reason string
	}{
		{"build", func(w *packageOpWorld) PackageModuleBuilder {
			return &fakeModuleBuilder{fail: errors.Join(pkgruntime.ErrBuild, errors.New("syntax error"))}
		}, "package_build_failed"},
		{"uses", func(w *packageOpWorld) PackageModuleBuilder {
			return &mutatingBuilder{inner: w.builder, mutate: func(m *pkgruntime.Module) {
				m.Uses = []string{"logout"}
				m.Manifest = json.RawMessage(`{"name":"demo-new"}`)
			}}
		}, pkgrelease.ReasonUsesExceedGrants},
		{"install review", func(w *packageOpWorld) PackageModuleBuilder {
			return &mutatingBuilder{inner: w.builder, mutate: func(m *pkgruntime.Module) {
				m.Manifest = json.RawMessage(`{"name":"demo-new","contributes":{"desks":[{"id":"d"}]}}`)
			}}
		}, "install_review_unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newPackageOpWorld(t, "refuse-"+strings.ReplaceAll(tc.name, " ", "-"))
			w.fx.configure(func(rt *Runtime) { rt.packageBuilder = tc.inner(w) })
			operation := w.propose(t, "op-pkg-refuse")
			failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
			if !strings.Contains(failed.TerminalError, "refused before publish") || !strings.Contains(failed.TerminalError, tc.reason) {
				t.Fatalf("terminal error = %q; want a refusal before publish naming %s", failed.TerminalError, tc.reason)
			}
			if w.pub.count() != 0 {
				t.Fatalf("a refused package was published %d time(s)", w.pub.count())
			}
			if len(w.installs(t)) != 0 {
				t.Fatalf("a refused package was installed: %+v", w.installs(t))
			}
			if kinds := w.fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 1 || kinds[computerevent.EventPackageApplied] != 0 {
				t.Fatalf("events = %v; want the failure discharged and no package_applied", kinds)
			}
			w.pendingCleared(t)
		})
	}
}

// The crash settle: a materializing package operation is settled from facts,
// by any later pass, never driven again.
func TestPackageOperationCrashSettlesFromFacts(t *testing.T) {
	unanswered := func(platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
		return platform.PackageReleaseView{}, 0, errors.New("connection reset by peer")
	}
	listDraftRelease := func(w *packageOpWorld) func(string) ([]PackageReleaseRef, error) {
		return func(operationID string) ([]PackageReleaseRef, error) {
			draft, found, err := w.fx.rt.loadPackageDraft(selfdev.Operation{OperationID: operationID, BundleDigest: w.bundleOf(t, operationID)})
			if err != nil || !found {
				return nil, errors.New("no draft")
			}
			return []PackageReleaseRef{{ReleaseID: draft.ReleaseID, OperationID: operationID}}, nil
		}
	}

	t.Run("nothing listed closes failed as unanswered", func(t *testing.T) {
		w := newPackageOpWorld(t, "settle-none")
		w.pub.publish = unanswered
		operation := w.propose(t, "op-pkg-settle-none")
		failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
		if want := "the publish was not answered and the Library lists no release for this operation"; failed.TerminalError != want || w.pub.count() != 1 || len(w.installs(t)) != 0 {
			t.Fatalf("operation = %+v publishes=%d; want failed %q after one publish (an unanswered publish is not a restart)", failed, w.pub.count(), want)
		}
		w.pendingCleared(t)
	})

	t.Run("listed with no install closes failed naming the release", func(t *testing.T) {
		w := newPackageOpWorld(t, "settle-listed")
		w.pub.publish = unanswered
		w.pub.list = listDraftRelease(w)
		operation := w.propose(t, "op-pkg-settle-listed")
		failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
		draft, _, _ := w.fx.rt.loadPackageDraft(failed)
		want := "published as " + draft.ReleaseID + "; not installed here; install it from the Library"
		if failed.TerminalError != want || failed.PackageReleaseID != "" {
			t.Fatalf("terminal error = %q release=%q; want %q", failed.TerminalError, failed.PackageReleaseID, want)
		}
		w.pendingCleared(t)
	})

	t.Run("listed with its install records package_applied", func(t *testing.T) {
		w := newPackageOpWorld(t, "settle-installed")
		w.pub.list = listDraftRelease(w)
		w.pub.publish = func(request platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
			// The release went public and the install landed, then the
			// computer lost the answer.
			id := request.ReleaseID
			if _, _, refusal := w.fx.rt.installPackage(context.Background(), "owner", w.fx.computerID, packageInstallRequest{
				ReleaseID: id, Record: request.Record, TreePatch: request.TreePatch, change: installChange{trigger: store.PackageInstallTriggerSelfdev},
			}); refusal != nil {
				t.Errorf("setup install: %v", refusal)
			}
			return platform.PackageReleaseView{}, 0, errors.New("connection reset by peer")
		}
		operation := w.propose(t, "op-pkg-settle-installed")
		applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
		if applied.PackageReleaseID == "" || applied.PackageInstallID != w.installs(t)[0].InstallID || len(w.installs(t)) != 1 {
			t.Fatalf("settled operation = %+v installs=%+v", applied, w.installs(t))
		}
		if kinds := w.fx.countEventKinds(); kinds[computerevent.EventPackageApplied] != 1 {
			t.Fatalf("events = %v; want one package_applied", kinds)
		}
		w.pendingCleared(t)
	})

	t.Run("a later pass settles a materializing operation and never publishes again", func(t *testing.T) {
		w := newPackageOpWorld(t, "settle-later")
		w.pub.publish = unanswered
		lookupFails := true
		var mu sync.Mutex
		w.pub.list = func(string) ([]PackageReleaseRef, error) {
			mu.Lock()
			defer mu.Unlock()
			if lookupFails {
				return nil, errors.New("the Library is unreachable")
			}
			return nil, nil
		}
		operation := w.propose(t, "op-pkg-settle-later")
		held := w.fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)
		time.Sleep(300 * time.Millisecond)
		if w.pub.count() != 1 {
			t.Fatalf("publishes = %d while the lookup is unproven, want 1", w.pub.count())
		}
		mu.Lock()
		lookupFails = false
		mu.Unlock()
		// A planned-restart pass or a crash-boot pass is the same pass: the
		// operation is materializing, so it is settled, not resumed.
		w.fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
		failed := w.fx.waitForState(t, held.OperationID, selfdev.StateFailed)
		if failed.TerminalError != selfdevInterruptedByRestart || w.pub.count() != 1 {
			t.Fatalf("operation = %+v publishes=%d; want failed as interrupted with no second publish", failed, w.pub.count())
		}
	})
}

// bundleOf reads an operation's bundle digest (the draft is keyed by it).
func (w *packageOpWorld) bundleOf(t *testing.T, operationID string) string {
	t.Helper()
	op, err := w.fx.operations.Get(context.Background(), w.fx.computerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	return op.BundleDigest
}

func (w *packageOpWorld) revertCall(t *testing.T, operationID string) (int, selfdev.Operation) {
	t.Helper()
	return w.fx.revert(t, operationID)
}

func TestPackageRevertUninstallsAndNeverCallsTheUpdater(t *testing.T) {
	w := newPackageOpWorld(t, "revert")
	operation := w.propose(t, "op-pkg-revert")
	applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	applyCalls, restarts := w.fx.applyCalls.Load(), w.fx.service.restarts

	code, reverted := w.revertCall(t, applied.OperationID)
	if code != http.StatusOK || reverted.State != selfdev.StateRolledBack || reverted.OperationID != applied.OperationID {
		t.Fatalf("revert status=%d operation=%+v; want the package row rolled back", code, reverted)
	}
	if w.fx.applyCalls.Load() != applyCalls || w.fx.service.restarts != restarts {
		t.Fatalf("the package revert touched the updater")
	}
	if ops := w.fx.rollbackOps(t); len(ops) != 1 || ops[0].OperationID != applied.OperationID {
		t.Fatalf("rollback-state operations = %+v; want only the reverted package row (no rollback row)", ops)
	}
	installs := w.installs(t)
	if len(installs) != 2 || installs[0].Action != packageActionUninstall || installs[0].ReleaseID != applied.PackageReleaseID || len(currentPackageInstalls(installs)) != 0 {
		t.Fatalf("installs = %+v; want the install followed by its uninstall", installs)
	}
	if kinds := w.fx.countEventKinds(); kinds[computerevent.EventPackageReverted] != 1 || kinds[computerevent.EventRollbackApplied] != 0 || kinds[computerevent.EventRollbackRequested] != 0 {
		t.Fatalf("events = %v; want one package_reverted and no rollback", kinds)
	}
	// A second tap is the same act; the install being gone is tolerated.
	code, again := w.revertCall(t, applied.OperationID)
	if code != http.StatusOK || again.OperationID != applied.OperationID || len(w.installs(t)) != 2 {
		t.Fatalf("second tap status=%d operation=%+v installs=%d", code, again, len(w.installs(t)))
	}
}

func TestPackageRevertToleratesAnInstallAlreadyGone(t *testing.T) {
	w := newPackageOpWorld(t, "revert-gone")
	operation := w.propose(t, "op-pkg-revert-gone")
	applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if _, _, refusal := w.fx.rt.uninstallPackage(context.Background(), "owner", w.fx.computerID, applied.PackageReleaseID); refusal != nil {
		t.Fatal(refusal)
	}
	if code, reverted := w.revertCall(t, applied.OperationID); code != http.StatusOK || reverted.State != selfdev.StateRolledBack {
		t.Fatalf("revert status=%d operation=%+v; want rolled back though the install was already uninstalled", code, reverted)
	}
}

func TestPlatformRevertPastAPackageOperationFindsThePlatformPrior(t *testing.T) {
	w := newPackageOpWorld(t, "past")
	first := w.fx.applyPlain(t, "op-past-1")
	pkg := w.fx.waitForState(t, w.propose(t, "op-past-pkg").OperationID, selfdev.StateApplied)
	second := w.fx.applyPlain(t, "op-past-2")
	w.fx.setBootedBuildCommit(t, joinTestBaseB)

	// The platform operation after the package accepted on the package's
	// event: its expected prior is not a platform state.
	accepted, found, err := w.fx.store.EventByDigest(context.Background(), w.fx.computerID, second.DecisionEvent)
	if err != nil || !found {
		t.Fatalf("accept event: found=%v err=%v", found, err)
	}
	if event, ok, err := w.fx.store.EventByDigest(context.Background(), w.fx.computerID, accepted.ExpectedEffectiveEventHead); err != nil || !ok || event.EventKind != computerevent.EventPackageApplied {
		t.Fatalf("setup: the platform operation's expected prior is %+v ok=%v err=%v; want the package_applied event", event.EventKind, ok, err)
	}

	code, rollback := w.fx.revert(t, second.OperationID)
	if code != http.StatusCreated || selfdev.IsBaseRestore(rollback) || rollback.BaseHead != first.EffectiveHead {
		t.Fatalf("revert status=%d operation=%+v; want the rollback to the first platform operation (%s)", code, rollback, first.EffectiveHead)
	}
	w.fx.requireRolledBackTo(t, rollback, first)
	// A second tap, after the package events were skipped, is the same act.
	if code, again := w.fx.revert(t, second.OperationID); code != http.StatusOK || again.OperationID != rollback.OperationID {
		t.Fatalf("second tap status=%d operation=%s; want the first act %s", code, again.OperationID, rollback.OperationID)
	}
	// A platform rollback never undoes a package install.
	if current := currentPackageInstalls(w.installs(t)); len(current) != 1 || current[newPackageName].ReleaseID != pkg.PackageReleaseID {
		t.Fatalf("package installs after the platform revert = %+v; want the package still installed", current)
	}
	still, err := w.fx.operations.Get(context.Background(), w.fx.computerID, pkg.OperationID)
	if err != nil || still.State != selfdev.StateApplied {
		t.Fatalf("package operation after the platform revert = %+v err=%v; want applied", still, err)
	}
}

func TestPackageOperationsNeverEnterStackBookkeeping(t *testing.T) {
	w := newPackageOpWorld(t, "bookkeeping")
	first := w.fx.applyPlain(t, "op-book-1")
	pkg := w.fx.waitForState(t, w.propose(t, "op-book-pkg").OperationID, selfdev.StateApplied)
	ctx := context.Background()
	rt := w.fx.rt

	// The release readers match no package operation.
	for _, digest := range []string{pkg.ReleaseDigest, pkg.BundleDigest, first.ReleaseDigest, strings.Repeat("0", 64)} {
		if got, ok := rt.appliedOperationForRelease(ctx, digest); ok && got.OperationID == pkg.OperationID {
			t.Fatalf("appliedOperationForRelease(%q) returned the package operation", digest)
		}
	}
	// The rebase offer reads only applied platform operations' patches.
	if _, _, err := rt.rebaseOfferDeps().SourcePatch(ctx, pkg.OperationID); err == nil {
		t.Fatal("the rebase offer read a package operation's patch")
	}
	// A clean rebase of a package operation is refused.
	if retry, why := rt.cleanRebaseBeforeNotice(ctx, "owner", w.fx.computerID, pkg.OperationID, pkg.BaseHead, joinTestBaseB); retry || !strings.Contains(why, "package") {
		t.Fatalf("clean rebase of a package operation = retry=%v %q; want a refusal naming the layer", retry, why)
	}
	// The live platform set and the landed anchor are per layer.
	rows, err := w.fx.operations.ListByStates(ctx, w.fx.computerID, selfdev.StateApplied, selfdev.StateRolledBack)
	if err != nil {
		t.Fatal(err)
	}
	live := liveOperationSet(rows)
	if live[pkg.OperationID] || !live[first.OperationID] {
		t.Fatalf("live platform set = %v; want the first operation only", live)
	}
	h := &APIHandler{rt: rt}
	platformLatest, err := h.latestLandedOperation(ctx, w.fx.computerID, false)
	if err != nil || platformLatest == nil || platformLatest.OperationID != first.OperationID {
		t.Fatalf("latest landed platform operation = %+v err=%v; want the first operation", platformLatest, err)
	}
	packageLatest, err := h.latestLandedOperation(ctx, w.fx.computerID, true)
	if err != nil || packageLatest == nil || packageLatest.OperationID != pkg.OperationID {
		t.Fatalf("latest landed package operation = %+v err=%v; want the package operation", packageLatest, err)
	}
}

func TestStagedOperationKindClassifiesThePatch(t *testing.T) {
	dir := t.TempDir()
	repo, _ := packageSourceRepo(t)
	pkgPatch, _ := releasePatch(t, repo, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	platformPatch, _ := releasePatch(t, repo, map[string]string{"frontend/src/app.css": "body { color: red }\n"})
	mixedPatch, _ := releasePatch(t, repo, map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n", "frontend/src/app.css": "body { color: blue }\n"})
	for name, tc := range map[string]struct {
		patch []byte
		want  string
	}{"package": {pkgPatch, selfdev.KindPackage}, "platform": {platformPatch, ""}, "mixed": {mixedPatch, ""}} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(root+"/source.patch", tc.patch, 0o600); err != nil {
				t.Fatal(err)
			}
			if got := stagedOperationKind(root, computerevent.DigestBytes(tc.patch)); got != tc.want {
				t.Fatalf("kind = %q, want %q", got, tc.want)
			}
		})
	}
	if got := stagedOperationKind(dir, ""); got != "" {
		t.Fatalf("no source patch: kind = %q", got)
	}
}

// A refusal corpusd answers (no publisher address, an expired decision) closes
// the operation failed with its reason; nothing is installed.
func TestPackageOperationPublishRefusalClosesFailedWithTheReason(t *testing.T) {
	w := newPackageOpWorld(t, "refused-publish")
	w.pub.publish = func(platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
		return platform.PackageReleaseView{}, http.StatusConflict, &selfdev.PackagePublishError{Status: http.StatusConflict, Reason: "publisher_address_required", Message: "no publisher address"}
	}
	operation := w.propose(t, "op-pkg-publish-refused")
	failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, "publisher_address_required") || len(w.installs(t)) != 0 || w.pub.count() != 1 {
		t.Fatalf("operation = %+v installs=%d publishes=%d", failed, len(w.installs(t)), w.pub.count())
	}
	w.pendingCleared(t)
}

// Without a Library lookup route, a persisted draft means the release may be
// public; the terminal error names it instead of claiming nothing happened.
func TestPackageSettleWithoutALookupNamesTheDraftedRelease(t *testing.T) {
	for _, unanswered := range []bool{true, false} {
		name := "boot pass"
		if unanswered {
			name = "unanswered publish"
		}
		t.Run(name, func(t *testing.T) {
			w := newPackageOpWorld(t, "nolister-"+strings.ReplaceAll(name, " ", "-"))
			w.fx.configure(func(rt *Runtime) { rt.packagePublisher = w.pub })
			w.pub.publish = func(platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
				return platform.PackageReleaseView{}, 0, errors.New("connection reset by peer")
			}
			operation := w.propose(t, "op-pkg-nolister")
			failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
			draft, found, err := w.fx.rt.loadPackageDraft(failed)
			if err != nil || !found {
				t.Fatalf("draft: found=%v err=%v", found, err)
			}
			if !strings.Contains(failed.TerminalError, "the release may be published as "+draft.ReleaseID) || strings.Contains(failed.TerminalError, "restart") {
				t.Fatalf("terminal error = %q; want the drafted release named and no restart", failed.TerminalError)
			}
			// The same facts from a later pass (a boot) say "interrupted".
			if got := packageUnlistedCause(draft, false, false); !strings.HasPrefix(got, "interrupted; the release may be published as "+draft.ReleaseID) {
				t.Fatalf("boot cause = %q", got)
			}
		})
	}
}

// B1: the kind is read before publishing renames the staged tree away. A
// classification taken after the rename read a missing file and was always
// empty, so every package operation took the platform materializer.
func TestPublishFrozenCandidateClassifiesBeforeTheStagedTreeMoves(t *testing.T) {
	repo, _ := packageSourceRepo(t)
	pkgPatch, _ := releasePatch(t, repo, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	platformPatch, _ := releasePatch(t, repo, map[string]string{"frontend/src/app.css": "body { color: red }\n"})
	for name, tc := range map[string]struct {
		patch []byte
		want  string
	}{"package": {pkgPatch, selfdev.KindPackage}, "platform": {platformPatch, ""}} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			staged := filepath.Join(parent, "staged")
			frozen := filepath.Join(parent, "incoming", "frozen")
			if err := os.MkdirAll(filepath.Dir(frozen), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(staged, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(staged, "source.patch"), tc.patch, 0o600); err != nil {
				t.Fatal(err)
			}
			kind, err := publishFrozenCandidate(staged, frozen, computerevent.DigestBytes(tc.patch), []byte(`{"draft":true}`))
			if err != nil {
				t.Fatal(err)
			}
			if kind != tc.want {
				t.Fatalf("kind = %q, want %q", kind, tc.want)
			}
			if _, err := os.Stat(filepath.Join(frozen, "source.patch")); err != nil {
				t.Fatalf("the staged tree did not become the frozen bundle: %v", err)
			}
		})
	}
}

// B1 backstop: a decision's kind must agree with the operation's recorded kind.
func TestDecisionKindMustAgreeWithTheOperationKind(t *testing.T) {
	pkg, plat := selfdev.Operation{OperationID: "op", Kind: selfdev.KindPackage}, selfdev.Operation{OperationID: "op"}
	if checkOperationKindAgrees(pkg, true) != nil || checkOperationKindAgrees(plat, false) != nil {
		t.Fatal("matching kinds were refused")
	}
	if checkOperationKindAgrees(plat, true) == nil {
		t.Fatal("a package decision on a platform row was accepted (it would host-build a package patch)")
	}
	if checkOperationKindAgrees(pkg, false) == nil {
		t.Fatal("a platform decision on a package row was accepted")
	}
}

// B2: an update drafts over the owner's track head the decision names, so
// corpusd does not answer track_moved: v1 then v2 of one package.
func TestPackageUpdateDraftsOverTheTrackHead(t *testing.T) {
	w := newPackageOpWorld(t, "update")
	v1 := w.fx.waitForState(t, w.propose(t, "op-pkg-v1").OperationID, selfdev.StateApplied)

	// v1's tree is now the base's package subtree (the capsule for v2 is cast
	// from a base that holds it); the decision names v1 as the track head.
	for path, content := range w.files {
		full := filepath.Join(w.repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, w.repo, "add", "-A")
	releaseGit(t, w.repo, "commit", "-q", "-m", "v1")
	w.base = releaseGit(t, w.repo, "rev-parse", "HEAD")
	tree, present, err := pkgrelease.Subtree(context.Background(), w.repo, w.base, newPackageRoot)
	if err != nil || !present {
		t.Fatalf("subtree: %v %v", present, err)
	}
	w.parent = map[string]any{"kind": pkgrelease.ParentRelease, "release_id": v1.PackageReleaseID, "tree": tree}
	w.files = map[string]string{newPackageRoot + "/App.svelte": "<p>demo new v2</p>\n"}
	w.screens = map[string][]byte{"01-before-home.png": mediaPNG(t, 64, 40, 5, false), "01-after-home.png": mediaPNG(t, 64, 40, 7, false)}
	v2 := w.fx.waitForState(t, w.propose(t, "op-pkg-v2").OperationID, selfdev.StateApplied)

	if w.pub.count() != 2 {
		t.Fatalf("publishes = %d, want 2", w.pub.count())
	}
	rec, err := pkgrelease.Decode(w.pub.publishes[1].Record)
	if err != nil || rec.Parent.Kind != pkgrelease.ParentRelease || rec.Parent.ReleaseID != v1.PackageReleaseID || rec.Parent.Tree != tree {
		t.Fatalf("v2 parent = %+v err=%v; want release %s over tree %s", rec.Parent, err, v1.PackageReleaseID, tree)
	}
	current := currentPackageInstalls(w.installs(t))
	if len(current) != 1 || current[newPackageName].ReleaseID != v2.PackageReleaseID {
		t.Fatalf("installs in force = %+v; want v2 replacing v1", current)
	}

	// Reverting v1, the superseded update, uninstalls nothing: it is recorded
	// rolled back with the reason (revert uninstalls, it does not restore the
	// prior version).
	code, reverted := w.revertCall(t, v1.OperationID)
	if code != http.StatusOK || reverted.State != selfdev.StateRolledBack || !strings.Contains(reverted.TerminalError, "already superseded") {
		t.Fatalf("revert of the superseded v1: status=%d %+v", code, reverted)
	}
	if current := currentPackageInstalls(w.installs(t)); current[newPackageName].ReleaseID != v2.PackageReleaseID {
		t.Fatalf("reverting the stale v1 changed the install in force: %+v", current)
	}
}

// N1: package_reverted names the package_applied event it undoes.
func TestPackageRevertedEventNamesThePackageAppliedEvent(t *testing.T) {
	w := newPackageOpWorld(t, "revert-ref")
	applied := w.fx.waitForState(t, w.propose(t, "op-pkg-revert-ref").OperationID, selfdev.StateApplied)
	if code, _ := w.revertCall(t, applied.OperationID); code != http.StatusOK {
		t.Fatalf("revert status=%d", code)
	}
	ctx := context.Background()
	appliedEvent, found, err := w.fx.store.EventByIdempotency(ctx, w.fx.computerID, "selfdev-package_applied-"+applied.DecisionEvent)
	if err != nil || !found {
		t.Fatalf("package_applied event: %v %v", found, err)
	}
	appliedDigest, _ := appliedEvent.Digest()
	revertedEvent, found, err := w.fx.store.EventByIdempotency(ctx, w.fx.computerID, "selfdev-package_reverted-"+applied.OperationID)
	if err != nil || !found || revertedEvent.DecisionRef != appliedDigest {
		t.Fatalf("package_reverted DecisionRef = %q, want the package_applied digest %s (found=%v err=%v)", revertedEvent.DecisionRef, appliedDigest, found, err)
	}
}

// N4: the owner's abandon of an open package operation settles from the
// package facts, with a package-worded cause.
func TestAbandonOfAMaterializingPackageOperationUsesThePackageFacts(t *testing.T) {
	w := newPackageOpWorld(t, "abandon")
	w.pub.publish = func(platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
		return platform.PackageReleaseView{}, 0, errors.New("connection reset by peer")
	}
	// The Library lookup is unreachable, so the operation stays materializing.
	w.pub.list = func(string) ([]PackageReleaseRef, error) { return nil, errors.New("the Library is unreachable") }
	operation := w.propose(t, "op-pkg-abandon")
	held := w.fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)
	draft, _, _ := w.fx.rt.loadPackageDraft(held)

	settled, status, err := w.fx.rt.abandonSelfDevelopmentOperation(context.Background(), "owner", w.fx.computerID, selfDevelopmentAbandonRequest{
		OperationID: held.OperationID, Reason: "wrong package", IdempotencyKey: "abandon-1",
	})
	if err != nil || status != http.StatusOK || settled.State != selfdev.StateFailed {
		t.Fatalf("abandon status=%d err=%v operation=%+v", status, err, settled)
	}
	if !strings.Contains(settled.TerminalError, "abandoned by the owner: wrong package") || !strings.Contains(settled.TerminalError, "the release may be published as "+draft.ReleaseID) {
		t.Fatalf("terminal error = %q; want the owner's reason and the drafted release", settled.TerminalError)
	}
	w.pendingCleared(t)
}

// Failure modes (b@c 2026-11-09, Neo Vector Heart, trajectory 4b9398fc):
//  1. A package operation applies after Texture's last turn. The consensus
//     approve record promised "a separate record says when it is applied or
//     fails", but the package success path minted none, so Texture waited for
//     it forever and the work item stayed open.
//  2. The apply record is minted twice (a replayed pass): it must commit once.
//  3. A crash boot settles an interrupted package operation from facts: it
//     must mint nothing, so a restart never starts a Texture turn (the settle
//     path mints only for a live, unanswered publish).
func TestPackageOperationApplyWakesTextureOnce(t *testing.T) {
	w := newPackageOpWorld(t, "apply-wakes-texture")
	_, inbox := seedOwnerActDocument(t, w.fx.store, "owner", w.fx.computerID, "doc-pkg-apply")
	w.fx.trajectoryID = "trajectory-owner-act-doc-pkg-apply"
	operation := w.propose(t, "op-pkg-apply-wake")
	applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)

	applyRecords := func() []types.CommitmentRecord {
		var out []types.CommitmentRecord
		for _, rec := range ownerActRecords(t, w.fx.store, "owner", w.fx.computerID, inbox) {
			if rec.SubjectRefs["owner_act"] == types.OwnerActApply {
				out = append(out, rec)
			}
		}
		return out
	}
	apply := applyRecords()
	if len(apply) != 1 {
		t.Fatalf("a live package apply must wake Texture once; apply records = %d", len(apply))
	}
	checkOwnerActRecord(t, apply[0], inbox, operation.OperationID, types.OwnerActApply)
	if body := apply[0].Directive.Body; !strings.Contains(body, applied.PackageReleaseID) || !strings.Contains(body, applied.PackageInstallID) {
		t.Fatalf("the apply record must name the release and install: %q", body)
	}
	// A replayed pass commits the same record, not a second one.
	w.fx.rt.notifyTextureOfPackageApplied(context.Background(), applied, applied.PackageReleaseID, applied.PackageInstallID)
	if n := len(applyRecords()); n != 1 {
		t.Fatalf("a replayed apply minted %d records, want 1", n)
	}
}

// Build review r1 (Devin): every live settle of a package operation (a retry
// pass, a planned boot, an unanswered publish) tells the candidate's Texture
// its fate, applied or failed; only a settle a crash boot marked mints
// nothing, so a restart never starts a turn.
func TestPackageOperationSettleMintsOwnerActOnlyWhenLive(t *testing.T) {
	cases := []struct {
		name      string
		install   bool // the install landed before the answer was lost
		crash     bool
		wantState string
		wantAct   string
	}{
		{"live retry pass applies and mints one apply record", true, false, selfdev.StateApplied, types.OwnerActApply},
		{"crash-marked settle applies and mints no record", true, true, selfdev.StateApplied, ""},
		{"live retry pass fails and mints one fail record", false, false, selfdev.StateFailed, types.OwnerActFail},
		{"crash-marked settle fails and mints no record", false, true, selfdev.StateFailed, ""},
	}
	for i, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := newPackageOpWorld(t, "settle-live-"+strconv.Itoa(i))
			docID := "doc-settle-live-" + strconv.Itoa(i)
			_, inbox := seedOwnerActDocument(t, w.fx.store, "owner", w.fx.computerID, docID)
			w.fx.trajectoryID = "trajectory-owner-act-" + docID
			w.pub.publish = func(request platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
				if tc.install {
					if _, _, refusal := w.fx.rt.installPackage(context.Background(), "owner", w.fx.computerID, packageInstallRequest{
						ReleaseID: request.ReleaseID, Record: request.Record, TreePatch: request.TreePatch, change: installChange{trigger: store.PackageInstallTriggerSelfdev},
					}); refusal != nil {
						t.Errorf("setup install: %v", refusal)
					}
				}
				return platform.PackageReleaseView{}, 0, errors.New("connection reset by peer")
			}
			// The Library lookup is unreachable, so the operation stays
			// materializing until the retry pass below.
			var mu sync.Mutex
			lookupOK := false
			var operationID string
			w.pub.list = func(string) ([]PackageReleaseRef, error) {
				mu.Lock()
				defer mu.Unlock()
				if !lookupOK {
					return nil, errors.New("the Library is unreachable")
				}
				draft, found, err := w.fx.rt.loadPackageDraft(selfdev.Operation{OperationID: operationID, BundleDigest: w.bundleOf(t, operationID)})
				if err != nil || !found {
					return nil, errors.New("no draft")
				}
				return []PackageReleaseRef{{ReleaseID: draft.ReleaseID, OperationID: operationID}}, nil
			}
			operation := w.propose(t, "op-pkg-settle-live-"+strconv.Itoa(i))
			mu.Lock()
			operationID = operation.OperationID
			mu.Unlock()
			held := w.fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)
			if tc.crash {
				w.fx.rt.setSelfdevCrashMarks(map[string]bool{operationID: true})
			}
			mu.Lock()
			lookupOK = true
			mu.Unlock()
			// The materializer's own observer may win the race to settle;
			// either pass is a retry pass, so a loser's refusal is fine.
			_ = w.fx.rt.reconcilePackageOperation(context.Background(), held)
			w.fx.waitForState(t, operationID, tc.wantState)
			count := func(act string) int {
				n := 0
				for _, rec := range ownerActRecords(t, w.fx.store, "owner", w.fx.computerID, inbox) {
					if act == "" || rec.SubjectRefs["owner_act"] == act {
						n++
					}
				}
				return n
			}
			if tc.wantAct == "" {
				time.Sleep(300 * time.Millisecond)
				if n := count(""); n != 0 {
					t.Fatalf("a crash-marked settle minted %d owner-act record(s), want none", n)
				}
				return
			}
			deadline := time.Now().Add(5 * time.Second)
			for count(tc.wantAct) == 0 && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
			}
			time.Sleep(200 * time.Millisecond)
			if n := count(tc.wantAct); n != 1 {
				t.Fatalf("a live settle minted %d %s record(s), want exactly 1", n, tc.wantAct)
			}
			if n := count(""); n != 1 {
				t.Fatalf("a live settle minted %d owner-act record(s) in all, want 1", n)
			}
			for _, rec := range ownerActRecords(t, w.fx.store, "owner", w.fx.computerID, inbox) {
				checkOwnerActRecord(t, rec, inbox, operationID, tc.wantAct)
			}
		})
	}
}
