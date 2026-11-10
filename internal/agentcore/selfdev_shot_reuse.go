package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/shotsandbox"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// The shot verdict reuse (docs/design/shot-verdict-reuse-2026-11-09.md,
// section 12). A computer that carries a self-developed platform layer re-ran
// the model-judged shot review on every platform deploy, because a clean rebase
// has a new operation and a new host build job and so no earlier verdict can
// match (FM2). This is the branch of candidateShotGate that spares the round
// when nothing the judged shots depend on has changed:
//
//   - the operation was accepted under a clean rebase receipt (its decision's
//     authority, not its id), and the receipt, read back from the tape, names it;
//   - the operation it re-applies has a passing verdict or reuse on this
//     computer's tape, bound to that operation, its bundle and this plan;
//   - the source ids of frontend/, nix/, flake.nix, flake.lock and the shot
//     driver's packages are equal at the base the pass was judged on, the
//     receipt's stack base and the booted base (build output is never compared:
//     the host build bakes the revision and date into the bundle);
//   - a liveness pass over the candidate in the shot sandbox is clean: the app
//     reached its ready marker and the page's /api calls all succeeded.
//
// Then the gate commits a shot_reuse event under the verdict's own key and the
// swap proceeds, with no model call. Any other outcome leaves the gate as it
// was: the judged round. The decision is a pure function of facts the IO
// gathers (decideShotReuse), so every refusal is a table row.

// shotLivenessMarker is the ready marker the frontend sets once it has
// resolved the session and mounted (frontend/src/App.svelte, data-auth-state).
const shotLivenessMarker = `.app-root[data-auth-state="signed_in"]`

// shotLivenessSettle lets the shell's first calls land before the count.
const shotLivenessSettle = 2 * time.Second


// shotInputIDsAt reads the source ids of one base commit from the release
// repository (the repository sourceproj.Reapply already requires both bases in).
func (rt *Runtime) shotInputIDsAt(ctx context.Context, commit string) (computerevent.ShotInputIDs, error) {
	if rt.shotReuseSource != nil {
		return rt.shotReuseSource(ctx, commit)
	}
	repo := rt.releaseRepo()
	if repo == "" {
		return computerevent.ShotInputIDs{}, errors.New("no release repository")
	}
	ids, err := sourceproj.ObjectIDs(ctx, repo, commit, computerevent.ShotInputPaths...)
	if err != nil {
		return computerevent.ShotInputIDs{}, err
	}
	out, ok := computerevent.ShotInputIDsFromList(ids)
	if !ok {
		return computerevent.ShotInputIDs{}, errors.New("the source ids do not match the path list")
	}
	return out, nil
}

// shotReusePrior is the prior event as the tape holds it.
type shotReusePrior struct {
	Digest string
	Kind   computerevent.EventKind
	// Verdict is set for a shot_verdict, Reuse for a shot_reuse.
	Verdict *computerevent.ShotVerdictPayload
	Reuse   *computerevent.ShotReusePayload
	// OriginRead says the reuse's origin was read back from the tape as a
	// shot_verdict pass with the digest the reuse recorded.
	OriginRead bool
}

// shotReuseFacts are the inputs of the decision, all gathered by the caller.
type shotReuseFacts struct {
	OperationID, BundleDigest, HostBuildJobID, PlanDigest string
	BundlePatchSHA                                        string
	BuiltBase, RuntimeBooted                              string
	PlanHasVideo                                          bool
	Receipt                                               selfdevprotocol.CleanRebaseReceipt
	Prior                                                 *shotReusePrior
	// StackIDs and BootedIDs are the source ids at the receipt's stack base
	// and booted base.
	StackIDs, BootedIDs computerevent.ShotInputIDs
	Env                 computerevent.ShotDriverEnv
}

// shotReuseClaim is what a granted reuse will record (everything but the
// liveness result and the job id, which the run supplies).
type shotReuseClaim struct {
	RebaseOf, PriorEvent, PriorKind, OriginEvent, OriginKey string
	ChainDepth                                              int
	JudgedBase, BootedBase                                  string
	Judged, Booted                                          computerevent.ShotInputIDs
	Driver                                                  computerevent.ShotDriverEnv
}

// decideShotReuse is the whole rule (design 12.2) as a function of its facts:
// a claim and "", or a zero claim and the reason it is refused. An empty,
// unreadable or unequal value refuses; no "" == "" ever matches.
func decideShotReuse(f shotReuseFacts) (shotReuseClaim, string) {
	r := f.Receipt
	switch {
	case strings.TrimSpace(f.OperationID) == "" || strings.TrimSpace(f.HostBuildJobID) == "" || !computerevent.IsSHA256(f.BundleDigest) || !computerevent.IsSHA256(f.PlanDigest):
		return shotReuseClaim{}, "the operation, host build, bundle or plan is unnamed"
	case r.OperationID != f.OperationID || r.BundleDigest != f.BundleDigest || r.HostBuildJobID != f.HostBuildJobID:
		return shotReuseClaim{}, "the receipt does not name this operation, bundle and host build"
	case !computerevent.IsSHA256(f.BundlePatchSHA) || r.SourcePatchSHA256 != f.BundlePatchSHA:
		return shotReuseClaim{}, "the receipt's patch is not the bundle's"
	case !computerevent.IsGitObjectID(r.BootedBase) || !computerevent.IsGitObjectID(r.StackBase) || r.BootedBase == r.StackBase:
		return shotReuseClaim{}, "the receipt does not name two different bases"
	case r.BootedBase != f.BuiltBase || r.BootedBase != f.RuntimeBooted:
		return shotReuseClaim{}, "the receipt's booted base is not the build's and the runtime's"
	case f.Prior == nil:
		return shotReuseClaim{}, "the operation re-applied has no shot pass on this computer's tape"
	case strings.TrimSpace(r.RebaseOf) == "" || !computerevent.IsSHA256(r.PriorBundleDigest) || !computerevent.IsSHA256(f.Prior.Digest):
		return shotReuseClaim{}, "the receipt does not name the operation re-applied, its bundle and its pass"
	}
	claim := shotReuseClaim{RebaseOf: r.RebaseOf, PriorEvent: f.Prior.Digest, PriorKind: string(f.Prior.Kind), BootedBase: r.BootedBase, Booted: f.BootedIDs, Driver: f.Env}
	switch f.Prior.Kind {
	case computerevent.EventShotVerdict:
		p := f.Prior.Verdict
		switch {
		case p == nil || p.Validate() != nil:
			return shotReuseClaim{}, "the prior verdict does not decode"
		case p.OperationID != r.RebaseOf || p.BundleDigest != r.PriorBundleDigest:
			return shotReuseClaim{}, "the prior verdict is not bound to the operation re-applied and its bundle"
		case p.PlanSHA256 != f.PlanDigest:
			return shotReuseClaim{}, "the prior verdict judged another plan"
		case p.Verdict != computerevent.ShotVerdictPass:
			return shotReuseClaim{}, "the prior verdict is not a pass"
		}
		claim.OriginEvent, claim.OriginKey, claim.ChainDepth = f.Prior.Digest, computerevent.ShotVerdictKey(p.OperationID, p.HostBuildJobID), 1
		claim.JudgedBase, claim.Judged = r.StackBase, f.StackIDs
	case computerevent.EventShotReuse:
		p := f.Prior.Reuse
		switch {
		case p == nil || p.Validate() != nil:
			return shotReuseClaim{}, "the prior reuse does not decode"
		case p.OperationID != r.RebaseOf || p.BundleDigest != r.PriorBundleDigest:
			return shotReuseClaim{}, "the prior reuse is not bound to the operation re-applied and its bundle"
		case p.PlanSHA256 != f.PlanDigest:
			return shotReuseClaim{}, "the prior reuse stands on another plan"
		case p.BootedBase != r.StackBase:
			return shotReuseClaim{}, "the prior reuse was not made on the receipt's stack base"
		case !f.Prior.OriginRead:
			return shotReuseClaim{}, "the prior reuse's origin is not a judged pass on the tape"
		case p.ChainDepth >= computerevent.ShotReuseMaxChain:
			return shotReuseClaim{}, "the chain of reuses is at its bound"
		case p.Driver != f.Env:
			return shotReuseClaim{}, "the driver environment differs from the prior reuse's"
		case p.Judged != f.StackIDs:
			return shotReuseClaim{}, "the source at the receipt's stack base is not what the origin was judged on"
		}
		claim.OriginEvent, claim.OriginKey, claim.ChainDepth = p.OriginEvent, p.OriginKey, p.ChainDepth+1
		claim.JudgedBase, claim.Judged = p.JudgedBase, p.Judged
	default:
		return shotReuseClaim{}, "the prior event is a " + string(f.Prior.Kind)
	}
	switch {
	case !claim.Judged.Complete() || !claim.Booted.Complete():
		return shotReuseClaim{}, "the source ids could not all be read"
	case claim.Judged != claim.Booted:
		return shotReuseClaim{}, "the frontend, flake, nix or shot driver source changed between the judged base and the booted base"
	case strings.TrimSpace(f.Env.Browser) == "" || strings.TrimSpace(f.Env.Fontconfig) == "":
		return shotReuseClaim{}, "the shot driver's browser or fontconfig is unset"
	case f.PlanHasVideo && strings.TrimSpace(f.Env.Ffmpeg) == "":
		return shotReuseClaim{}, "the plan records a video and ffmpeg is unset"
	}
	return claim, ""
}

// shotLivenessPlan is the fixed platform-owned view the liveness pass runs.
func shotLivenessPlan() capsule.ShotPlan {
	return capsule.ShotPlan{Views: []capsule.ShotView{{
		Name: "liveness", Path: "/", Viewport: capsule.ShotViewportDesktop,
		Steps: []capsule.ShotStep{
			{Op: capsule.ShotOpWaitFor, Selector: shotLivenessMarker, Ms: capsule.MaxShotStepMs},
			{Op: capsule.ShotOpWaitMs, Ms: int(shotLivenessSettle / time.Millisecond)},
		},
	}}}
}

// shotLivenessVerdict reads a sandbox Result as liveness: a stage error, a
// page that made no /api call, or any /api status of 400 or more fails it.
func shotLivenessVerdict(res shotsandbox.Result) (computerevent.ShotLiveness, string) {
	live := computerevent.ShotLiveness{Marker: shotLivenessMarker}
	if res.API != nil {
		live.Requests, live.Failures, live.ByStatus = res.API.Requests, res.API.Failures, res.API.ByStatus
	}
	switch {
	case res.Error != "":
		return live, "the liveness pass failed: " + res.Error
	case res.API == nil || res.API.Requests < 1:
		return live, "the page made no /api call, so nothing shows the backend serves it"
	case res.API.Failures != 0:
		return live, fmt.Sprintf("%d of %d /api responses were errors: %s", res.API.Failures, res.API.Requests, strings.Join(res.API.Offenders, "; "))
	}
	live.Passed = true
	return live, ""
}

// shotReuseRun is one liveness pass this process drives for an (operation,
// host build job), or the standing refusal of one.
type shotReuseRun struct {
	state    string // running, passed, refused
	claim    shotReuseClaim
	liveness computerevent.ShotLiveness
	jobID    string
}

const (
	shotReuseRunning = "running"
	shotReusePassed  = "passed"
	shotReuseRefused = "refused"
)

func shotReuseRunKey(operationID, hostJobID string) string { return operationID + "|" + hostJobID }

// shotReuseGate is the reuse branch of candidateShotGate. handled false leaves
// the gate on its judged path; handled true returns err as the gate's answer
// (nil: the reuse is committed and the swap may proceed; pending: the liveness
// pass is running).
func (rt *Runtime) shotReuseGate(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built *hostBuiltRelease, planDigest string) (handled bool, err error) {
	if rt.store == nil || rt.eventPayloadReader == nil || rt.eventAppender == nil || built == nil || bundle.ShotPlan == nil {
		return false, nil
	}
	key := shotReuseRunKey(operation.OperationID, built.Job.JobID)
	if run := rt.getShotReuseRun(key); run != nil {
		switch run.state {
		case shotReuseRunning:
			return true, fmt.Errorf("%w: operation %s host build %s liveness running", errCandidateShotPending, operation.OperationID, built.Job.JobID)
		case shotReuseRefused:
			return false, nil
		}
		if err := rt.appendShotReuse(ctx, operation, built.Job.JobID, planDigest, *run); err != nil {
			return true, err
		}
		log.Printf("selfdev shot reuse: operation %s host build %s: reused the pass of %s (chain %d), liveness clean (%d /api responses); no judged round", operation.OperationID, built.Job.JobID, run.claim.RebaseOf, run.claim.ChainDepth, run.liveness.Requests)
		// Finished: the event on the key is the record from here on.
		rt.shotReuseMu.Lock()
		delete(rt.shotReuseRuns, key)
		rt.shotReuseMu.Unlock()
		return true, nil
	}

	// The first pass for this (operation, job) in this process: decide.
	refuse := func(why string) (bool, error) {
		rt.setShotReuseRun(key, &shotReuseRun{state: shotReuseRefused})
		log.Printf("selfdev shot reuse: operation %s host build %s: not reused (%s); the shots are judged", operation.OperationID, built.Job.JobID, why)
		return false, nil
	}
	// A read that failed is not a refusal: nothing is cached, this pass takes
	// the judged path and the next pass reads again (design 12.7, N2).
	unread := func(why string) (bool, error) {
		log.Printf("selfdev shot reuse: operation %s host build %s: not decided (%s); the shots are judged this pass", operation.OperationID, built.Job.JobID, why)
		return false, nil
	}
	transition, found, derr := rt.store.FinalizedDecisionForOperation(ctx, operation.ComputerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if derr != nil {
		return false, derr
	}
	if !found {
		return false, nil
	}
	receiptDigest, isRebase := selfdevprotocol.CleanRebaseReceiptDigestFromAuthority(transition.Request.Event.AuthorityRef)
	if !isRebase {
		return false, nil // any other decision: a new patch is new judged content
	}
	facts := shotReuseFacts{
		OperationID: operation.OperationID, BundleDigest: operation.BundleDigest, HostBuildJobID: built.Job.JobID, PlanDigest: planDigest,
		BundlePatchSHA: bundle.SourcePatchSHA256, BuiltBase: built.Job.BaseCommit, RuntimeBooted: rt.bootedBaseCommit(), PlanHasVideo: bundle.ShotPlan.Video != nil,
	}
	raw, ferr := rt.eventPayloadReader.FetchPayload(ctx, operation.ComputerID, receiptDigest)
	if ferr != nil || computerevent.DigestBytes(raw) != receiptDigest {
		if ferr != nil {
			return unread("the clean rebase receipt cannot be read from the tape: " + ferr.Error())
		}
		return refuse("the clean rebase receipt is not the one its digest names")
	}
	receipt, rerr := selfdevprotocol.DecodeCleanRebaseReceipt(raw)
	if rerr != nil {
		return refuse("the clean rebase receipt does not decode: " + rerr.Error())
	}
	facts.Receipt = receipt
	prior, perr := rt.shotReusePriorFor(ctx, operation.ComputerID, receipt.RebaseOf)
	if perr != nil {
		return unread("the prior pass cannot be read: " + perr.Error())
	}
	if prior == nil {
		return refuse("the operation re-applied has no shot pass on this computer's tape")
	}
	facts.Prior = prior
	var idsErr error
	if facts.StackIDs, idsErr = rt.shotInputIDsAt(ctx, receipt.StackBase); idsErr != nil {
		return unread("the source at the stack base is unreadable: " + idsErr.Error())
	}
	if facts.BootedIDs, idsErr = rt.shotInputIDsAt(ctx, receipt.BootedBase); idsErr != nil {
		return unread("the source at the booted base is unreadable: " + idsErr.Error())
	}
	env, eerr := rt.shotDeps().env(*bundle.ShotPlan)
	if eerr != nil {
		return refuse(eerr.Error())
	}
	facts.Env = computerevent.ShotDriverEnv{Browser: env.browser, Fontconfig: env.fontconfig, Ffmpeg: env.ffmpeg}
	claim, why := decideShotReuse(facts)
	if why != "" {
		return refuse(why)
	}

	// Granted so far; the page-level check decides. One run per (operation,
	// job): a concurrent pass finds it running.
	rt.shotReuseMu.Lock()
	if rt.shotReuseRuns == nil {
		rt.shotReuseRuns = map[string]*shotReuseRun{}
	}
	if rt.shotReuseRuns[key] != nil {
		rt.shotReuseMu.Unlock()
		return true, fmt.Errorf("%w: operation %s host build %s liveness running", errCandidateShotPending, operation.OperationID, built.Job.JobID)
	}
	rt.shotReuseRuns[key] = &shotReuseRun{state: shotReuseRunning, claim: claim}
	rt.shotReuseMu.Unlock()
	start := rt.shotReuseStart
	if start == nil {
		start = func(run func()) { go run() }
	}
	builtCopy := *built
	log.Printf("selfdev shot reuse: operation %s host build %s: the source the pass of %s was judged on is unchanged (%s..%s); running the liveness pass", operation.OperationID, built.Job.JobID, claim.RebaseOf, claim.JudgedBase[:12], claim.BootedBase[:12])
	start(func() { rt.runShotLiveness(key, operation, builtCopy, env) })
	if cur := rt.getShotReuseRun(key); cur != nil && cur.state != shotReuseRunning {
		// A synchronous start (tests) has finished: answer from its outcome.
		return rt.shotReuseGate(ctx, operation, bundle, built, planDigest)
	}
	return true, fmt.Errorf("%w: operation %s host build %s liveness running", errCandidateShotPending, operation.OperationID, built.Job.JobID)
}

func (rt *Runtime) setShotReuseRun(key string, run *shotReuseRun) {
	rt.shotReuseMu.Lock()
	defer rt.shotReuseMu.Unlock()
	if rt.shotReuseRuns == nil {
		rt.shotReuseRuns = map[string]*shotReuseRun{}
	}
	rt.shotReuseRuns[key] = run
}

// getShotReuseRun is a snapshot of the run, safe to read after the lock is
// released while the liveness goroutine finishes it.
func (rt *Runtime) getShotReuseRun(key string) *shotReuseRun {
	rt.shotReuseMu.Lock()
	defer rt.shotReuseMu.Unlock()
	if run := rt.shotReuseRuns[key]; run != nil {
		copied := *run
		return &copied
	}
	return nil
}

// safeLivenessPass turns a panic in the pass into its failed result, so a
// run never stays "running" in a live process (the goroutine has no caller to
// recover for it).
func (rt *Runtime) safeLivenessPass(ctx context.Context, operation selfdev.Operation, built hostBuiltRelease, env shotEnv) (res shotsandbox.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			res, err = shotsandbox.Result{}, fmt.Errorf("the liveness pass panicked: %v", r)
		}
	}()
	if rt.shotLivenessPass != nil {
		return rt.shotLivenessPass(ctx, operation, built, env)
	}
	return rt.livenessPass(ctx, operation, built, env)
}

// runShotLiveness runs the pass and records its outcome on the run. A restart
// loses it: nothing is persisted, and the next gate pass of a still-accepted
// operation runs a fresh pass, which is a new check, not a resumed one.
func (rt *Runtime) runShotLiveness(key string, operation selfdev.Operation, built hostBuiltRelease, env shotEnv) {
	ctx, cancel := context.WithTimeout(context.Background(), candidateShotPassTimeout+time.Minute)
	defer cancel()
	res, err := rt.safeLivenessPass(ctx, operation, built, env)
	var live computerevent.ShotLiveness
	why := ""
	if err != nil {
		live, why = computerevent.ShotLiveness{Marker: shotLivenessMarker}, "the liveness pass did not run: "+err.Error()
	} else {
		live, why = shotLivenessVerdict(res)
	}
	rt.shotReuseMu.Lock()
	if run := rt.shotReuseRuns[key]; run != nil {
		run.liveness = live
		run.jobID = "liveness-" + computerevent.DigestBytes([]byte(key))[:16]
		if why != "" {
			run.state = shotReuseRefused
		} else {
			run.state = shotReusePassed
		}
	}
	rt.shotReuseMu.Unlock()
	if why != "" {
		log.Printf("selfdev shot reuse: operation %s host build %s: liveness failed (%s); the shots are judged", operation.OperationID, built.Job.JobID, why)
		rt.triggerSelfDevelopmentReconcile()
		return
	}
	log.Printf("selfdev shot reuse: operation %s host build %s: liveness clean (%d /api responses, statuses %v)", operation.OperationID, built.Job.JobID, live.Requests, live.ByStatus)
	rt.triggerSelfDevelopmentReconcile()
}

// livenessPass boots the candidate as the shot job does (its binary and
// frontend over a private copy of the state, in the loopback-only sandbox) and
// runs the fixed liveness view. One side only, no model.
func (rt *Runtime) livenessPass(ctx context.Context, operation selfdev.Operation, built hostBuiltRelease, env shotEnv) (shotsandbox.Result, error) {
	deps := rt.shotDeps()
	updaterRoot := strings.TrimSpace(rt.selfdevUpdaterRoot)
	if updaterRoot == "" || rt.store == nil {
		return shotsandbox.Result{}, errors.New("no updater root or store to run over")
	}
	rt.sweepShotWorkRootOnce()
	binary, frontend, err := rt.materializeCandidateBuild(built, updaterRoot)
	if err != nil {
		return shotsandbox.Result{}, fmt.Errorf("candidate build: %w", err)
	}
	root := deps.workRoot
	if root == "" {
		root = defaultShotWorkRoot()
	}
	reachTop := deps.reachTop
	if reachTop == "" {
		reachTop = "/"
	}
	if err := prepareShotWorkRoot(root); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("work root: %w", err)
	}
	opDir := filepath.Join(root, safeShotDirName("liveness:"+operation.OperationID))
	_ = os.RemoveAll(opDir)
	if err := os.Mkdir(opDir, 0o711); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("work directory: %w", err)
	}
	if err := os.Chmod(opDir, 0o711); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("work directory: %w", err)
	}
	defer os.RemoveAll(opDir)
	uid, gid := capsule.ShotSandboxIDs()
	side := shotSide{name: "after", dir: filepath.Join(opDir, "after"), binary: filepath.Join(opDir, "bin", "candidate"), frontend: filepath.Join(opDir, "fe", "candidate")}
	for d, mode := range map[string]fs.FileMode{filepath.Join(opDir, "bin"): 0o711, filepath.Join(opDir, "fe"): 0o711, side.dir: 0o700} {
		if err := mkdirMode(d, mode); err != nil {
			return shotsandbox.Result{}, fmt.Errorf("work directory: %w", err)
		}
	}
	if deps.workRoot == "" {
		if err := shotsandbox.CheckWorkDirLength(filepath.Join(side.dir, "work")); err != nil {
			return shotsandbox.Result{}, err
		}
	}
	if err := linkOrCopyFile(binary, side.binary, 0o755); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("binary: %w", err)
	}
	if err := linkOrCopyTree(frontend, side.frontend); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("frontend: %w", err)
	}
	for p, need := range map[string]fs.FileMode{side.binary: 0o005, filepath.Join(side.frontend, "index.html"): 0o004} {
		if err := checkSandboxReach(reachTop, p, need); err != nil {
			return shotsandbox.Result{}, fmt.Errorf("the sandbox user cannot reach the staged build: %w", err)
		}
	}
	estimate, err := rt.store.SnapshotEstimate(ctx)
	if err != nil {
		return shotsandbox.Result{}, fmt.Errorf("state estimate: %w", err)
	}
	if err := capsule.CheckStateCopyFits(opDir, estimate); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("state copy: %w", err)
	}
	if _, err := rt.store.SnapshotToKinds(ctx, side.dir, lease.CapsuleStoreKinds()); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("state copy: %w", err)
	}
	if err := capsule.StateVolumeAboveFloor(opDir); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("state copy: %w", err)
	}
	files := filepath.Join(side.dir, "files")
	if err := mkdirMode(files, 0o755); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("files layer: %w", err)
	}
	if _, err := capsule.BuildShotFilesLayer(ctx, rt.filesRoot(), strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_SOURCE_ROOT")), files, uid, gid); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("files layer: %w", err)
	}
	for _, d := range []string{"out", "work"} {
		if err := mkdirMode(filepath.Join(side.dir, d), 0o700); err != nil {
			return shotsandbox.Result{}, fmt.Errorf("work directory: %w", err)
		}
	}
	if err := deps.handOver(side.dir, uid, gid, "out", "work", "state.db", "state.texture"); err != nil {
		return shotsandbox.Result{}, fmt.Errorf("hand the work directory over: %w", err)
	}
	launch := deps.launch
	if launch == nil {
		launch = func(ctx context.Context, spec shotsandbox.Spec) (shotsandbox.Result, error) {
			return shotsandbox.Launch(ctx, "", spec)
		}
	}
	passCtx, cancel := context.WithTimeout(ctx, candidateShotPassTimeout)
	defer cancel()
	return launch(passCtx, side.spec(shotLivenessPlan(), env))
}

// shotReusePriorFor reads the latest event on the shot-verdict family of an
// operation and, for a reuse, its origin. nil with no error: none yet.
func (rt *Runtime) shotReusePriorFor(ctx context.Context, computerID, operationID string) (*shotReusePrior, error) {
	if strings.TrimSpace(operationID) == "" {
		return nil, errors.New("no operation to read the pass of")
	}
	event, found, err := rt.store.LatestComputerEventByIdempotencyPrefix(ctx, computerID, "shot-verdict:"+operationID+":")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	digest, err := event.Digest()
	if err != nil {
		return nil, err
	}
	raw, err := rt.eventPayloadReader.FetchPayload(ctx, computerID, event.PayloadCommitment)
	if err != nil {
		return nil, err
	}
	prior := &shotReusePrior{Digest: digest, Kind: event.EventKind}
	switch event.EventKind {
	case computerevent.EventShotVerdict:
		var p computerevent.ShotVerdictPayload
		if json.Unmarshal(raw, &p) != nil {
			return nil, errors.New("the prior verdict does not decode")
		}
		prior.Verdict = &p
	case computerevent.EventShotReuse:
		var p computerevent.ShotReusePayload
		if json.Unmarshal(raw, &p) != nil {
			return nil, errors.New("the prior reuse does not decode")
		}
		prior.Reuse = &p
		prior.OriginRead = rt.shotReuseOriginIsAPass(ctx, computerID, p)
	}
	return prior, nil
}

// shotReuseOriginIsAPass re-reads a reuse's origin from the tape: a
// shot_verdict pass under the recorded key, with the recorded digest, for the
// operation the key names. A chain never stands on its own say-so.
func (rt *Runtime) shotReuseOriginIsAPass(ctx context.Context, computerID string, reuse computerevent.ShotReusePayload) bool {
	if reuse.Validate() != nil {
		return false
	}
	event, found, err := rt.store.EventByIdempotency(ctx, computerID, reuse.OriginKey)
	if err != nil || !found || event.EventKind != computerevent.EventShotVerdict {
		return false
	}
	if digest, derr := event.Digest(); derr != nil || digest != reuse.OriginEvent {
		return false
	}
	raw, err := rt.eventPayloadReader.FetchPayload(ctx, computerID, event.PayloadCommitment)
	if err != nil {
		return false
	}
	var p computerevent.ShotVerdictPayload
	if json.Unmarshal(raw, &p) != nil || p.Validate() != nil {
		return false
	}
	return p.Verdict == computerevent.ShotVerdictPass && computerevent.ShotVerdictKey(p.OperationID, p.HostBuildJobID) == reuse.OriginKey
}

// appendShotReuse commits the operation's shot_reuse event, idempotently on
// the verdict's key (operation, host build job). The gate is its only writer.
func (rt *Runtime) appendShotReuse(ctx context.Context, operation selfdev.Operation, hostJobID, planDigest string, run shotReuseRun) error {
	c := run.claim
	payload := computerevent.ShotReusePayload{
		OperationID: operation.OperationID, BundleDigest: operation.BundleDigest, HostBuildJobID: hostJobID, ShotJobID: run.jobID,
		PlanSHA256: planDigest, Verdict: computerevent.ShotVerdictPass, Reason: computerevent.ShotReuseReason,
		RebaseOf: c.RebaseOf, PriorEvent: c.PriorEvent, PriorKind: c.PriorKind, OriginEvent: c.OriginEvent, OriginKey: c.OriginKey, ChainDepth: c.ChainDepth,
		JudgedBase: c.JudgedBase, Judged: c.Judged, BootedBase: c.BootedBase, Booted: c.Booted, Driver: c.Driver, Liveness: run.liveness,
	}
	if err := payload.Validate(); err != nil {
		return candidateShotFailure{Cause: "the reuse does not validate: " + err.Error()}
	}
	key := computerevent.ShotVerdictKey(operation.OperationID, hostJobID)
	if _, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, key); err != nil {
		return err
	} else if found {
		// A replay, or something else on the key: read it through the gate's
		// own readers rather than trusting it (review round 2, finding 2).
		existing, _, _ := rt.store.EventByIdempotency(ctx, operation.ComputerID, key)
		if existing.EventKind == computerevent.EventShotReuse {
			return rt.readShotReuseEvent(ctx, operation, hostJobID, planDigest, key, existing)
		}
		return rt.readShotVerdictEvent(ctx, operation, hostJobID, planDigest, key, existing)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return err
	}
	body, err := computerevent.CanonicalJSON(payload)
	if err != nil {
		return err
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
		EventKind: computerevent.EventShotReuse, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: key, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
		ActorProfile: platformUpdateActor(), AuthorityRef: "platform-shot-reuse:" + operation.OperationID + ":" + run.jobID, PrivacyClass: "owner",
		ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent,
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	_, _, err = rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, body, computerevent.ShotReuseMediaType, "owner")
	return err
}

// readShotReuseEvent checks the reuse on the gate's key against every
// binding, as readShotVerdictEvent does a verdict: a reuse for another
// operation, host build, bundle or plan, or one that does not validate, closes
// the operation failed rather than unlocking the swap.
func (rt *Runtime) readShotReuseEvent(ctx context.Context, operation selfdev.Operation, hostJobID, planDigest, key string, event computerevent.Event) error {
	if rt.eventPayloadReader == nil {
		return fmt.Errorf("shot reuse event %s is unreadable", key)
	}
	raw, err := rt.eventPayloadReader.FetchPayload(ctx, operation.ComputerID, event.PayloadCommitment)
	if err != nil {
		return fmt.Errorf("shot reuse payload: %w", err)
	}
	var payload computerevent.ShotReusePayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Validate() != nil {
		return candidateShotFailure{Cause: "the shot_reuse event does not decode"}
	}
	if payload.OperationID != operation.OperationID || payload.HostBuildJobID != hostJobID ||
		payload.BundleDigest != operation.BundleDigest || payload.PlanSHA256 != planDigest {
		return candidateShotFailure{Cause: "the shot_reuse event is not bound to this operation, host build and plan"}
	}
	return nil
}

// shotsOwedRebaseNote is added to the shots-owed text of a clean-rebase
// operation: the running build already contains the change, so BEFORE is not
// the base (design 12.1, S5).
const shotsOwedRebaseNote = " This operation is a clean rebase of a change already judged: the BEFORE image is the running build, which already contains the change, so the AFTER image is expected to match it (q2 yes), and the question is whether the booted base moved anything else on screen."
