package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/shotsandbox"
)

// Review round 2 (design 12.7). N1: every input the shot pass runs is compared,
// so a change to any one of them takes the judged round.
func TestShotReuseEveryInputPathIsCompared(t *testing.T) {
	paths := computerevent.ShotInputPaths
	for i, path := range paths {
		f := goodShotReuseFacts()
		list := f.BootedIDs.List()
		list[i] = testIDs("moved").List()[i]
		booted, ok := computerevent.ShotInputIDsFromList(list)
		if !ok {
			t.Fatal("list")
		}
		f.BootedIDs = booted
		if _, why := decideShotReuse(f); !strings.Contains(why, "changed") {
			t.Errorf("a change to %s was not refused: %q", path, why)
		}
		// And a missing id at that path is incomplete, not equal to another
		// missing one.
		list = f.StackIDs.List()
		list[i] = ""
		g := goodShotReuseFacts()
		g.StackIDs, _ = computerevent.ShotInputIDsFromList(list)
		g.BootedIDs = g.StackIDs
		if _, why := decideShotReuse(g); !strings.Contains(why, "could not all be read") {
			t.Errorf("a missing id for %s was not refused: %q", path, why)
		}
	}
	// A reuse recorded with the narrower six-id set is not a hop.
	f := goodShotReuseFacts()
	reuse := goodShotReusePayload(f, 2)
	reuse.PriorKind = string(computerevent.EventShotReuse)
	reuse.BootedBase = testStackBase
	raw, _ := json.Marshal(reuse)
	var generic map[string]any
	_ = json.Unmarshal(raw, &generic)
	for _, side := range []string{"judged", "booted"} {
		old := generic[side].(map[string]any)
		for _, k := range []string{"go_mod", "go_sum", "capsule", "lease", "objectgraph", "pkgruntime", "appdesk_firstparty", "appdesk_required", "styles"} {
			delete(old, k)
		}
	}
	narrow, _ := json.Marshal(generic)
	var decoded computerevent.ShotReusePayload
	if err := json.Unmarshal(narrow, &decoded); err != nil {
		t.Fatal(err)
	}
	f.Prior = &shotReusePrior{Digest: testSHA("f"), Kind: computerevent.EventShotReuse, Reuse: &decoded, OriginRead: true}
	if _, why := decideShotReuse(f); !strings.Contains(why, "does not decode") {
		t.Fatalf("a reuse with the narrower id set was accepted: %q", why)
	}
}

// N1: the paths are real files and directories of the repository.
func TestShotInputPathsExist(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, p := range computerevent.ShotInputPaths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Errorf("shot input path %s: %v", p, err)
		}
	}
	if len(computerevent.ShotInputPaths) != len(computerevent.ShotInputIDs{}.List()) {
		t.Fatal("the path list and the id list differ in length")
	}
}

// N2: a key that something else occupies is read, not trusted.
func TestAppendShotReuseDoesNotTrustAnOccupiedKey(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-reuse-occupied", "done")
	var captured struct {
		operation selfdev.Operation
		bundle    transaction.CapsuleEffectBundle
		built     hostBuiltRelease
	}
	fx.configure(func(rt *Runtime) {
		rt.shotsAskedHook = func(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
			captured.operation, captured.bundle, captured.built = operation, bundle, built
			testWriteShotVerdict(t, rt, operation, bundle, built, computerevent.ShotVerdictFail, "q3 no")
		}
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-reuse-occupied")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-reuse-occupied")
	fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	planDigest, _ := captured.bundle.ShotPlan.Digest()
	claim, why := decideShotReuse(goodShotReuseFacts())
	if why != "" {
		t.Fatal(why)
	}
	run := shotReuseRun{claim: claim, jobID: "liveness-test", liveness: computerevent.ShotLiveness{Passed: true, Requests: 2, Marker: shotLivenessMarker}}
	err := fx.rt.appendShotReuse(context.Background(), captured.operation, captured.built.Job.JobID, planDigest, run)
	var failure candidateShotFailure
	if !errors.As(err, &failure) || !strings.Contains(failure.Cause, "q3 no") {
		t.Fatalf("a fail verdict on the key = %v, want its failure, not success", err)
	}
}

// N3: a panic in the liveness goroutine is a failed run, not a run that stays
// running; the failure is the judged path.
func TestShotLivenessPanicBecomesAFailedRun(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-reuse-panic", "done")
	fx.configure(func(rt *Runtime) {
		rt.shotLivenessPass = func(context.Context, selfdev.Operation, hostBuiltRelease, shotEnv) (shotsandbox.Result, error) {
			panic("boom")
		}
	})
	key := shotReuseRunKey("op", "job")
	fx.rt.setShotReuseRun(key, &shotReuseRun{state: shotReuseRunning})
	fx.rt.runShotLiveness(key, selfdev.Operation{OperationID: "op"}, hostBuiltRelease{}, shotEnv{})
	if run := fx.rt.getShotReuseRun(key); run == nil || run.state != shotReuseRefused {
		t.Fatalf("run after a panic = %+v, want refused", run)
	}
	// A clean pass becomes passed.
	fx.rt.shotLivenessPass = func(context.Context, selfdev.Operation, hostBuiltRelease, shotEnv) (shotsandbox.Result, error) {
		return shotsandbox.Result{API: &shotsandbox.APIStats{Requests: 2}}, nil
	}
	fx.rt.setShotReuseRun(key, &shotReuseRun{state: shotReuseRunning})
	fx.rt.runShotLiveness(key, selfdev.Operation{OperationID: "op"}, hostBuiltRelease{}, shotEnv{})
	if run := fx.rt.getShotReuseRun(key); run == nil || run.state != shotReusePassed || run.jobID == "" {
		t.Fatalf("run after a clean pass = %+v", run)
	}
}

// N4: a read that fails is not a standing refusal. The same pass, once the
// tape reads again, is decided on its merits.
func TestShotReuseUnreadFactsAreNotCached(t *testing.T) {
	f := newCleanRebaseE2E(t)
	ctx := context.Background()
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/long.txt": cleanLong(map[int]string{35: "platform line 35"})})
	f.boot(t, b2)
	if out := f.fire(t); out.Minted != "" {
		t.Fatalf("notice minted: %+v", out)
	}
	op, found := f.cleanOp(t, b2)
	if !found {
		t.Fatal("no clean rebase operation")
	}
	applied := f.waitForState(t, op.OperationID, selfdev.StateApplied)
	job := f.hostBuildOf(t, applied)
	raw, err := os.ReadFile(filepath.Join(f.updaterRoot, "incoming", applied.BundleDigest, "bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bundle transaction.CapsuleEffectBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	bundle.ShotPlan = &capsule.ShotPlan{Views: []capsule.ShotView{{Name: "home", Path: "/", Viewport: "desktop"}}}
	planDigest, _ := bundle.ShotPlan.Digest()
	built := hostBuiltRelease{Job: job}
	key := shotReuseRunKey(applied.OperationID, job.JobID)

	good := f.rt.eventPayloadReader
	f.rt.eventPayloadReader = failingPayloadReader{}
	if handled, err := f.rt.shotReuseGate(ctx, applied, bundle, &built, planDigest); handled || err != nil {
		t.Fatalf("unreadable tape = handled %v, %v", handled, err)
	}
	if f.rt.getShotReuseRun(key) != nil {
		t.Fatal("a tape read error was cached as a refusal")
	}
	f.rt.eventPayloadReader = good
	f.rt.shotReuseSource = func(context.Context, string) (computerevent.ShotInputIDs, error) {
		return computerevent.ShotInputIDs{}, errors.New("git: transient")
	}
	// No prior pass exists on this tape, so the order of the reads decides:
	// the prior read comes first and is a definitive refusal.
	if handled, err := f.rt.shotReuseGate(ctx, applied, bundle, &built, planDigest); handled || err != nil {
		t.Fatalf("no prior pass = handled %v, %v", handled, err)
	}
	if run := f.rt.getShotReuseRun(key); run == nil || run.state != shotReuseRefused {
		t.Fatalf("a definitive refusal was not cached: %+v", run)
	}
}
