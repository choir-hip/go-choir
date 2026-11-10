package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/shotsandbox"
)

// The shot verdict reuse (selfdev_shot_reuse.go; design section 12.5). Ways it
// can fail are listed there, with the test for each. These are the pure
// decision, the liveness verdict and the event reader/writer; the gate on a
// real clean rebase is covered by the clean rebase fixture test below.

func testIDs(seed string) computerevent.ShotInputIDs {
	list := make([]string, len(computerevent.ShotInputPaths))
	for i, p := range computerevent.ShotInputPaths {
		list[i] = computerevent.DigestBytes([]byte(seed + p))[:40]
	}
	ids, _ := computerevent.ShotInputIDsFromList(list)
	return ids
}


const (
	testStackBase  = "1111111111111111111111111111111111111111"
	testBootedBase = "2222222222222222222222222222222222222222"
)

func testSHA(c string) string { return strings.Repeat(c, 64) }

func goodShotReuseFacts() shotReuseFacts {
	ids := testIDs("same")
	plan := testSHA("7")
	return shotReuseFacts{
		OperationID: "clean-rebase-new", BundleDigest: testSHA("b"), HostBuildJobID: testSHA("6"), PlanDigest: plan,
		BundlePatchSHA: testSHA("a"), BuiltBase: testBootedBase, RuntimeBooted: testBootedBase,
		Receipt: selfdevprotocol.CleanRebaseReceipt{
			OperationID: "clean-rebase-new", BundleDigest: testSHA("b"), HostBuildJobID: testSHA("6"), SourcePatchSHA256: testSHA("a"),
			RebaseOf: "op-prior", PriorBundleDigest: testSHA("c"), StackBase: testStackBase, BootedBase: testBootedBase,
		},
		Prior: &shotReusePrior{Digest: testSHA("e"), Kind: computerevent.EventShotVerdict, Verdict: &computerevent.ShotVerdictPayload{
			OperationID: "op-prior", BundleDigest: testSHA("c"), HostBuildJobID: testSHA("d"), ShotJobID: "shot-1", PlanSHA256: plan,
			Verdict:  computerevent.ShotVerdictPass,
			Findings: []computerevent.ShotFindingRecord{{Pair: "01-home", BeforeSHA256: testSHA("1"), AfterSHA256: testSHA("2")}},
		}},
		StackIDs: ids, BootedIDs: ids,
		Env: computerevent.ShotDriverEnv{Browser: "/nix/store/b/chromium", Fontconfig: "/nix/store/f/fonts.conf", Ffmpeg: "/nix/store/m/ffmpeg"},
	}
}

func goodShotReusePayload(f shotReuseFacts, depth int) *computerevent.ShotReusePayload {
	return &computerevent.ShotReusePayload{
		OperationID: "op-prior", BundleDigest: testSHA("c"), HostBuildJobID: testSHA("d"), ShotJobID: "liveness-x", PlanSHA256: f.PlanDigest,
		Verdict: computerevent.ShotVerdictPass, Reason: computerevent.ShotReuseReason,
		RebaseOf: "op-older", PriorEvent: testSHA("9"), PriorKind: string(computerevent.EventShotVerdict), OriginEvent: testSHA("9"),
		OriginKey: computerevent.ShotVerdictKey("op-older", testSHA("8")), ChainDepth: depth,
		JudgedBase: testStackBase, Judged: f.StackIDs, BootedBase: testStackBase, Booted: f.StackIDs, Driver: f.Env,
		Liveness: computerevent.ShotLiveness{Passed: true, Requests: 3, Marker: shotLivenessMarker},
	}
}

func TestShotReuseDecisionGrantsAFirstHop(t *testing.T) {
	claim, why := decideShotReuse(goodShotReuseFacts())
	if why != "" {
		t.Fatalf("refused: %s", why)
	}
	if claim.ChainDepth != 1 || claim.OriginEvent != testSHA("e") || claim.JudgedBase != testStackBase || claim.BootedBase != testBootedBase || claim.RebaseOf != "op-prior" ||
		claim.OriginKey != computerevent.ShotVerdictKey("op-prior", testSHA("d")) {
		t.Fatalf("claim = %+v", claim)
	}
}

// F9: a rebase of a rebase reuses a reuse, pinned to the origin.
func TestShotReuseChains(t *testing.T) {
	f := goodShotReuseFacts()
	reuse := goodShotReusePayload(f, 2)
	reuse.PriorKind = string(computerevent.EventShotReuse)
	reuse.OperationID, reuse.BundleDigest = "op-prior", testSHA("c")
	reuse.BootedBase = testStackBase
	f.Prior = &shotReusePrior{Digest: testSHA("f"), Kind: computerevent.EventShotReuse, Reuse: reuse, OriginRead: true}
	claim, why := decideShotReuse(f)
	if why != "" {
		t.Fatalf("a chained reuse was refused: %s", why)
	}
	if claim.ChainDepth != 3 || claim.OriginEvent != reuse.OriginEvent || claim.OriginKey != reuse.OriginKey || claim.JudgedBase != reuse.JudgedBase || claim.PriorEvent != testSHA("f") {
		t.Fatalf("claim = %+v, want depth 3 pinned to the origin", claim)
	}
}

func TestShotReuseDecisionRefusals(t *testing.T) {
	chained := func(f *shotReuseFacts, mut func(p *computerevent.ShotReusePayload)) {
		reuse := goodShotReusePayload(*f, 2)
		reuse.PriorKind = string(computerevent.EventShotReuse)
		reuse.BootedBase = testStackBase
		if mut != nil {
			mut(reuse)
		}
		f.Prior = &shotReusePrior{Digest: testSHA("f"), Kind: computerevent.EventShotReuse, Reuse: reuse, OriginRead: true}
	}
	rows := []struct {
		name string
		mut  func(f *shotReuseFacts)
		want string
	}{
		{"receipt_other_op", func(f *shotReuseFacts) { f.Receipt.OperationID = "clean-rebase-other" }, "does not name this operation"},
		{"receipt_other_bundle", func(f *shotReuseFacts) { f.Receipt.BundleDigest = testSHA("0") }, "does not name this operation"},
		{"receipt_other_job", func(f *shotReuseFacts) { f.Receipt.HostBuildJobID = testSHA("0") }, "does not name this operation"},
		{"patch_differs", func(f *shotReuseFacts) { f.Receipt.SourcePatchSHA256 = testSHA("0") }, "patch"},
		{"empty_patch", func(f *shotReuseFacts) { f.BundlePatchSHA, f.Receipt.SourcePatchSHA256 = "", "" }, "patch"},
		{"booted_mismatch_build", func(f *shotReuseFacts) { f.BuiltBase = testStackBase }, "booted base"},
		{"booted_mismatch_runtime", func(f *shotReuseFacts) { f.RuntimeBooted = "" }, "booted base"},
		{"empty_bases", func(f *shotReuseFacts) { f.Receipt.StackBase, f.Receipt.BootedBase = "", "" }, "two different bases"},
		{"same_bases", func(f *shotReuseFacts) { f.Receipt.StackBase = f.Receipt.BootedBase }, "two different bases"},
		{"no_prior", func(f *shotReuseFacts) { f.Prior = nil }, "no shot pass"},
		{"empty_prior_digest", func(f *shotReuseFacts) { f.Prior.Digest = "" }, "does not name"},
		{"empty_rebase_of", func(f *shotReuseFacts) { f.Receipt.RebaseOf = "" }, "does not name"},
		{"prior_other_op", func(f *shotReuseFacts) { f.Prior.Verdict.OperationID = "op-other" }, "not bound"},
		{"prior_other_bundle", func(f *shotReuseFacts) { f.Prior.Verdict.BundleDigest = testSHA("0") }, "not bound"},
		{"prior_other_plan", func(f *shotReuseFacts) { f.Prior.Verdict.PlanSHA256 = testSHA("0") }, "another plan"},
		{"prior_fail", func(f *shotReuseFacts) {
			f.Prior.Verdict.Verdict, f.Prior.Verdict.Reason, f.Prior.Verdict.Findings = computerevent.ShotVerdictFail, "q3 no", nil
		}, "not a pass"},
		{"prior_does_not_decode", func(f *shotReuseFacts) { f.Prior.Verdict = &computerevent.ShotVerdictPayload{} }, "does not decode"},
		{"prior_unknown_kind", func(f *shotReuseFacts) { f.Prior.Kind = computerevent.EventCandidateShot }, "candidate_shot"},
		{"empty_judged_ids", func(f *shotReuseFacts) {
			f.StackIDs, f.BootedIDs = computerevent.ShotInputIDs{}, computerevent.ShotInputIDs{}
		}, "could not all be read"},
		{"empty_booted_id", func(f *shotReuseFacts) { f.BootedIDs.Frontend = "" }, "could not all be read"},
		{"frontend_changed", func(f *shotReuseFacts) { f.BootedIDs.Frontend = testIDs("other").Frontend }, "changed"},
		{"flake_lock_changed", func(f *shotReuseFacts) { f.BootedIDs.FlakeLock = testIDs("other").FlakeLock }, "changed"},
		{"nix_changed", func(f *shotReuseFacts) { f.BootedIDs.Nix = testIDs("other").Nix }, "changed"},
		{"driver_source_changed", func(f *shotReuseFacts) { f.BootedIDs.ShotSandbox = testIDs("other").ShotSandbox }, "changed"},
		{"judged_mismatch_stack", func(f *shotReuseFacts) {
			chained(f, func(p *computerevent.ShotReusePayload) { p.Judged = testIDs("older"); p.Booted = p.Judged })
		}, "origin was judged on"},
		{"env_empty_browser", func(f *shotReuseFacts) { f.Env.Browser = "" }, "browser or fontconfig"},
		{"env_empty_fontconfig", func(f *shotReuseFacts) { f.Env.Fontconfig = "  " }, "browser or fontconfig"},
		{"env_no_ffmpeg_for_video", func(f *shotReuseFacts) { f.PlanHasVideo, f.Env.Ffmpeg = true, "" }, "ffmpeg"},
		{"env_changed", func(f *shotReuseFacts) { chained(f, nil); f.Env.Browser = "/nix/store/other/chromium" }, "driver environment"},
		{"origin_not_read", func(f *shotReuseFacts) { chained(f, nil); f.Prior.OriginRead = false }, "origin"},
		{"chain_too_deep", func(f *shotReuseFacts) {
			chained(f, func(p *computerevent.ShotReusePayload) { p.ChainDepth = computerevent.ShotReuseMaxChain })
		}, "bound"},
		{"chain_other_plan", func(f *shotReuseFacts) {
			chained(f, func(p *computerevent.ShotReusePayload) { p.PlanSHA256 = testSHA("0") })
		}, "another plan"},
		{"chain_other_op", func(f *shotReuseFacts) {
			chained(f, func(p *computerevent.ShotReusePayload) { p.OperationID = "op-other" })
		}, "not bound"},
		{"chain_wrong_stack_base", func(f *shotReuseFacts) {
			chained(f, func(p *computerevent.ShotReusePayload) { p.BootedBase = testBootedBase })
		}, "stack base"},
		{"chain_frontend_changed", func(f *shotReuseFacts) {
			chained(f, nil)
			f.StackIDs = testIDs("moved")
			f.BootedIDs = testIDs("moved")
		}, "origin was judged on"},
		{"chain_invalid_payload", func(f *shotReuseFacts) {
			chained(f, func(p *computerevent.ShotReusePayload) { p.Liveness.Passed = false })
		}, "does not decode"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			f := goodShotReuseFacts()
			row.mut(&f)
			claim, why := decideShotReuse(f)
			if why == "" || !strings.Contains(why, row.want) {
				t.Fatalf("refusal = %q (claim %+v), want it to contain %q", why, claim, row.want)
			}
			if claim.ChainDepth != 0 || claim.OriginEvent != "" {
				t.Fatalf("a refusal carried a claim: %+v", claim)
			}
		})
	}
}

// F6, F7: a stage error, a page that never called the backend, or any error
// status fails the liveness pass.
func TestLivenessVerdict(t *testing.T) {
	ok := shotsandbox.Result{API: &shotsandbox.APIStats{Requests: 4, ByStatus: map[string]int{"200": 3, "101": 1}}}
	live, why := shotLivenessVerdict(ok)
	if why != "" || !live.Passed || live.Requests != 4 || live.Marker != shotLivenessMarker {
		t.Fatalf("clean pass = %+v %q", live, why)
	}
	for name, res := range map[string]shotsandbox.Result{
		"stage_error": {Error: "wait_for timed out", API: &shotsandbox.APIStats{Requests: 4}},
		"no_api":      {},
		"no_requests": {API: &shotsandbox.APIStats{}},
		"errors":      {API: &shotsandbox.APIStats{Requests: 5, Failures: 1, Offenders: []string{"GET /api/x 500"}}},
	} {
		live, why := shotLivenessVerdict(res)
		if why == "" || live.Passed {
			t.Errorf("%s passed: %+v", name, live)
		}
	}
	// The plan waits for the real ready marker and has a settle step.
	plan := shotLivenessPlan()
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := plan.Views[0].Steps[0]; got.Selector != `.app-root[data-auth-state="signed_in"]` || got.Op != "wait_for" {
		t.Fatalf("liveness waits for %+v", got)
	}
}

type memPayloads map[string][]byte

func (m memPayloads) FetchPayload(_ context.Context, _, digest string) ([]byte, error) {
	if raw, ok := m[digest]; ok {
		return raw, nil
	}
	return nil, errors.New("not found")
}

// F2: the reader accepts a reuse for this operation, job, bundle and plan and
// no other.
func TestReadShotReuseEventBindings(t *testing.T) {
	f := goodShotReuseFacts()
	payload := goodShotReusePayload(f, 1)
	payload.PriorKind = string(computerevent.EventShotVerdict)
	payload.OperationID, payload.HostBuildJobID, payload.BundleDigest = "clean-rebase-new", testSHA("6"), testSHA("b")
	operation := selfdev.Operation{OperationID: "clean-rebase-new", BundleDigest: testSHA("b"), ComputerID: "c"}
	read := func(p computerevent.ShotReusePayload, planDigest string) error {
		raw, _ := json.Marshal(p)
		rt := &Runtime{eventPayloadReader: memPayloads{"x": raw}}
		return rt.readShotReuseEvent(context.Background(), operation, testSHA("6"), planDigest, "key", computerevent.Event{EventKind: computerevent.EventShotReuse, PayloadCommitment: "x"})
	}
	if err := read(*payload, f.PlanDigest); err != nil {
		t.Fatalf("a bound reuse was refused: %v", err)
	}
	for name, mut := range map[string]func(p *computerevent.ShotReusePayload){
		"other_op":     func(p *computerevent.ShotReusePayload) { p.OperationID = "clean-rebase-other" },
		"other_job":    func(p *computerevent.ShotReusePayload) { p.HostBuildJobID = testSHA("0") },
		"other_bundle": func(p *computerevent.ShotReusePayload) { p.BundleDigest = testSHA("0") },
		"other_plan":   func(p *computerevent.ShotReusePayload) { p.PlanSHA256 = testSHA("0") },
		"not_a_pass":   func(p *computerevent.ShotReusePayload) { p.Verdict = computerevent.ShotVerdictFail },
		"no_liveness":  func(p *computerevent.ShotReusePayload) { p.Liveness = computerevent.ShotLiveness{} },
		"no_prior":     func(p *computerevent.ShotReusePayload) { p.PriorEvent = "" },
		"ids_differ":   func(p *computerevent.ShotReusePayload) { p.Booted = testIDs("other") },
	} {
		p := *payload
		mut(&p)
		var failure candidateShotFailure
		if err := read(p, f.PlanDigest); !errors.As(err, &failure) {
			t.Errorf("%s: err = %v, want a candidateShotFailure", name, err)
		}
	}
	rt := &Runtime{eventPayloadReader: memPayloads{}}
	if err := rt.readShotReuseEvent(context.Background(), operation, testSHA("6"), f.PlanDigest, "key", computerevent.Event{PayloadCommitment: "missing"}); err == nil {
		t.Error("an unreadable payload was accepted")
	}
}

func testWriteShotReuse(t *testing.T, rt *Runtime, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease, hostJobID string) {
	t.Helper()
	planDigest, err := bundle.ShotPlan.Digest()
	if err != nil {
		t.Error(err)
		return
	}
	f := goodShotReuseFacts()
	claim, why := decideShotReuse(f)
	if why != "" {
		t.Errorf("good facts refused: %s", why)
		return
	}
	run := shotReuseRun{state: shotReusePassed, claim: claim, jobID: "liveness-test",
		liveness: computerevent.ShotLiveness{Passed: true, Requests: 3, ByStatus: map[string]int{"200": 3}, Marker: shotLivenessMarker}}
	if err := rt.appendShotReuse(context.Background(), operation, hostJobID, planDigest, run); err != nil {
		t.Errorf("append shot_reuse: %v", err)
	}
}

// F2, F5: a shot_reuse event under the gate's key unlocks the swap exactly like
// a pass, on the real chain (the reducer binds it to the pending decision), and
// writing it twice is a replay.
func TestShotReuseEventUnlocksTheSwapAndIsIdempotent(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-reuse", "done")
	fx.configure(func(rt *Runtime) {
		rt.shotsAskedHook = func(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
			testWriteShotReuse(t, rt, operation, bundle, built, built.Job.JobID)
			testWriteShotReuse(t, rt, operation, bundle, built, built.Job.JobID) // a replay
		}
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-reuse")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-reuse")
	fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventShotReuse] != 1 || kinds[computerevent.EventShotVerdict] != 0 {
		t.Fatalf("events = %v, want one shot_reuse and no shot_verdict", kinds)
	}
}

// F2: a reuse written for another host build job does not unlock this one.
func TestShotReuseForeignJobDoesNotUnlockTheSwap(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-reuse-foreign", "done")
	fx.configure(func(rt *Runtime) {
		rt.shotsAskedHook = func(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
			testWriteShotReuse(t, rt, operation, bundle, built, "job-of-another-build")
		}
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-reuse-foreign")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-reuse-foreign")
	accepted := fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	if err := fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false); !errors.Is(err, errCandidateShotPending) {
		t.Fatalf("a pass with only a foreign reuse = %v, want pending", err)
	}
	if fx.applyCalls.Load() != 0 {
		t.Fatal("a reuse of another host build unlocked this swap")
	}
}

// F12: an operation decided by anything but a clean rebase receipt never
// enters the reuse branch: no run, no log, the judged path as before.
func TestShotReuseNotApplicable(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-reuse-na", "done")
	var asked atomic.Int32
	fx.configure(func(rt *Runtime) {
		rt.shotsAskedHook = func(selfdev.Operation, transaction.CapsuleEffectBundle, hostBuiltRelease) { asked.Add(1) }
		rt.shotReuseSource = func(context.Context, string) (computerevent.ShotInputIDs, error) {
			t.Error("the source ids were read for an owner-decided operation")
			return computerevent.ShotInputIDs{}, errors.New("unexpected")
		}
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-reuse-na")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-reuse-na")
	accepted := fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	if err := fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false); !errors.Is(err, errCandidateShotPending) {
		t.Fatalf("an owner-decided pass = %v, want the judged path's pending", err)
	}
	if asked.Load() == 0 {
		t.Fatal("the judged path did not run")
	}
	if len(fx.rt.shotReuseRuns) != 0 {
		t.Fatalf("an owner-decided operation recorded reuse runs: %v", fx.rt.shotReuseRuns)
	}
}

// F8, F10: one liveness run per (operation, job), whatever the number of
// passes; a run that has not finished answers pending; a new runtime has no
// run (a restart resumes nothing).
func TestShotReuseSingleFlight(t *testing.T) {
	rt := &Runtime{}
	key := shotReuseRunKey("op", "job")
	if rt.getShotReuseRun(key) != nil {
		t.Fatal("a new runtime holds a run")
	}
	rt.setShotReuseRun(key, &shotReuseRun{state: shotReuseRunning})
	snapshot := rt.getShotReuseRun(key)
	rt.shotReuseRuns[key].state = shotReusePassed
	if snapshot.state != shotReuseRunning {
		t.Fatal("a snapshot followed the run it copied")
	}
	if rt.getShotReuseRun(key).state != shotReusePassed {
		t.Fatal("the run did not finish")
	}
	if (&Runtime{}).getShotReuseRun(key) != nil {
		t.Fatal("a restarted runtime resumed a run")
	}
}

// The gate on a real clean rebase (the clean rebase fixture: real git, a real
// receipt through corpusd's admission). The prior change was approved with no
// shot pass on the tape, so the branch reads the decision and the receipt back,
// finds no pass to stand on and leaves the judged round, once, with the run
// recorded as refused so later passes do not repeat the work (F12, F1).
func TestShotReuseGateReadsTheReceiptAndRefusesWithoutAPriorPass(t *testing.T) {
	f := newCleanRebaseE2E(t)
	ctx := context.Background()
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/long.txt": cleanLong(map[int]string{35: "platform line 35"})})
	f.boot(t, b2)
	if out := f.fire(t); out.Minted != "" {
		t.Fatalf("the clean rebase minted a notice: %+v", out)
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
	planDigest, err := bundle.ShotPlan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	built := hostBuiltRelease{Job: job}
	var starts atomic.Int32
	f.rt.shotReuseStart = func(func()) { starts.Add(1) }
	handled, err := f.rt.shotReuseGate(ctx, applied, bundle, &built, planDigest)
	if handled || err != nil {
		t.Fatalf("the gate with no prior pass = handled %v, %v; want the judged path", handled, err)
	}
	run := f.rt.getShotReuseRun(shotReuseRunKey(applied.OperationID, job.JobID))
	if run == nil || run.state != shotReuseRefused || starts.Load() != 0 {
		t.Fatalf("run = %+v, starts %d; want one standing refusal and no liveness run", run, starts.Load())
	}
	// The same pass again is the cached refusal, not a second decision.
	if handled, err := f.rt.shotReuseGate(ctx, applied, bundle, &built, planDigest); handled || err != nil {
		t.Fatalf("second pass = handled %v, %v", handled, err)
	}
}
