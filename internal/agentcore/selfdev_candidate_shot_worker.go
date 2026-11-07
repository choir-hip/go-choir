package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/shotsandbox"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The candidate-shot worker (docs/design/source-only-freeze-and-candidate-
// shots-2026-11-06.md 3.3, S2). One per operation and host build, started by
// the gate (startCandidateShot) and run outside the materializer lock. It
// shoots the frozen plan against the running build and then against the
// host-built candidate, over private copies of the computer's state, in a
// loopback-only sandbox (internal/shotsandbox); has the per-shot review judge
// the pairs; saves the screens; and writes the one candidate_shot event.
//
// Failure modes, decided before the code (FM numbers are the design's):
//
//   - FM4 a shot writes the live state: the store is read once through
//     SnapshotToKinds into the worker's own directory, the before copy is a
//     plain file copy of that, the owner files are hardlinks the sandbox
//     cannot write (it runs as the capsule's unprivileged id), and nothing
//     but directories the worker made is ever chowned.
//   - FM5 the two runs differ: one frozen plan drives both; the before run
//     takes no video, the after run takes the plan's.
//   - FM6 a hang: the sandbox pass has its own view/plan/stage deadlines, the
//     review has its timeout, and the worker has one for the whole run. A
//     timeout inside a pass or the review is a fail verdict with its cause;
//     only a panic, or the worker's own deadline ending before a verdict, leaves
//     no event, and then the watchdog (ensure) counts a silent death and the
//     third writes "shot worker died".
//   - FM7 a crash: no event is written before the end, so a crash boot closes
//     the accepted operation by the existing rule; a planned-restart re-run
//     starts from scratch (the work directory is wiped first) and the event
//     is idempotent on (operation, host build job).
//   - FM8 the review model is unavailable: the verdict is a fail with that
//     cause, never a pass.
//   - FM9 the candidate does not start or serve: the sandbox Result names it
//     with the stderr tail, and the verdict fails with both.
//   - a disk that cannot hold the copies: the capsule state floor check
//     refuses, as a capsule copy would, and the verdict fails with it.
//   - the swap racing a still-running worker: the worker holds no lock the
//     materializer takes, and re-reads the operation before the run; a
//     terminal operation gets no event.

const (
	// candidateShotMaxStarts is the first start and its two restarts (N4).
	candidateShotMaxStarts = 3
	// candidateShotWorkerTimeout bounds the whole run, copies to event.
	candidateShotWorkerTimeout = 30 * time.Minute
	// candidateShotPassTimeout bounds one sandbox pass (before or after).
	candidateShotPassTimeout = 10 * time.Minute

	candidateShotWorkerDied = "shot worker died"
)

// shotWorkerState is the watchdog's memory of one operation's worker.
type shotWorkerState struct {
	running bool
	starts  int
}

// candidateShotDeps are the worker's seams: the sandbox launch and the
// paths a test cannot take from the machine. Nil uses the machine's own.
type candidateShotDeps struct {
	launch func(ctx context.Context, spec shotsandbox.Spec) (shotsandbox.Result, error)
	// workRoot holds the per-operation work directories.
	workRoot string
	// reachTop is where the sandbox reach check starts; empty is "/".
	reachTop string
	// runningBinary and runningFrontend are the running build's.
	runningBinary, runningFrontend string
	// browser, ffmpeg, fontconfig replace the environment's.
	browser, ffmpeg, fontconfig string
	// lchown and chownTree replace the ownership calls in tests.
	lchown    func(path string, uid, gid int) error
	chownTree func(root string, uid, gid int) error
}

// sweepShotWorkRootOnce removes what an earlier process left in the work
// root (a crash mid-run), before this process's first worker makes its own
// directory. Once per runtime: later workers only ever see their own.
func (rt *Runtime) sweepShotWorkRootOnce() {
	rt.shotSweepOnce.Do(func() {
		deps := rt.shotDeps()
		root := deps.workRoot
		if root == "" {
			root = defaultShotWorkRoot()
		}
		// The work root was under the capsule state directory before
		// 2026-11-07; a crashed run there left a state copy nobody sweeps.
		if deps.workRoot == "" {
			if v := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_STATE_DIR")); v != "" {
				if err := os.RemoveAll(filepath.Join(v, "shot")); err != nil {
					log.Printf("selfdev candidate shot: sweep the legacy work root: %v", err)
				}
			}
		}
		// The root is checked before anything under it is removed: a
		// planted symlink or someone else's directory is not swept.
		if err := prepareShotWorkRoot(root); err != nil {
			log.Printf("selfdev candidate shot: sweep: work root: %v", err)
			return
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
				log.Printf("selfdev candidate shot: sweep %s: %v", e.Name(), err)
			}
		}
	})
}

// handOver gives dir itself (no follow) to uid:gid, then each named entry
// under it that exists, as a tree. The files layer gets only its top
// directory: its subdirectories were made owned and its files are live links.
func (d candidateShotDeps) handOver(dir string, uid, gid int, names ...string) error {
	lchown, chownTree := d.lchown, d.chownTree
	if lchown == nil {
		lchown = os.Lchown
	}
	if chownTree == nil {
		chownTree = capsule.ChownTreeNoFollow
	}
	if err := lchown(dir, uid, gid); err != nil {
		return err
	}
	// The files layer's top directory, never its links.
	if _, err := os.Lstat(filepath.Join(dir, "files")); err == nil {
		if err := lchown(filepath.Join(dir, "files"), uid, gid); err != nil {
			return err
		}
	}
	for _, name := range names {
		path := filepath.Join(dir, name)
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := chownTree(path, uid, gid); err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) shotDeps() candidateShotDeps {
	if rt.candidateShotDeps != nil {
		return *rt.candidateShotDeps
	}
	return candidateShotDeps{}
}

// ensureCandidateShotWorker is startCandidateShot's production body: start
// the worker unless one is running; restart a dead one at most twice; the
// third silent death writes the failing verdict (the watchdog, N4).
func (rt *Runtime) ensureCandidateShotWorker(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
	key := operation.OperationID + "/" + built.Job.JobID
	rt.shotWorkersMu.Lock()
	if rt.shotWorkers == nil {
		rt.shotWorkers = map[string]*shotWorkerState{}
	}
	state := rt.shotWorkers[key]
	if state == nil {
		state = &shotWorkerState{}
		rt.shotWorkers[key] = state
	}
	if state.running {
		rt.shotWorkersMu.Unlock()
		return
	}
	if state.starts >= candidateShotMaxStarts {
		rt.shotWorkersMu.Unlock()
		rt.finishCandidateShot(key, operation, bundle, built, func(base computerevent.CandidateShotPayload) computerevent.CandidateShotPayload {
			base.Verdict, base.Cause = computerevent.CandidateShotFail, candidateShotWorkerDied
			return base
		})
		return
	}
	state.starts++
	state.running = true
	rt.shotWorkersMu.Unlock()
	go rt.runCandidateShotWorker(key, state, operation, bundle, built)
}

// finishCandidateShot writes the event for a verdict built from the
// operation's bindings and forgets the watchdog state once it is written.
func (rt *Runtime) finishCandidateShot(key string, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease, verdict func(computerevent.CandidateShotPayload) computerevent.CandidateShotPayload) {
	planDigest, err := bundle.ShotPlan.Digest()
	if err != nil {
		log.Printf("selfdev candidate shot: operation %s: plan digest: %v", operation.OperationID, err)
		return
	}
	payload := verdict(computerevent.CandidateShotPayload{
		OperationID: operation.OperationID, BundleDigest: operation.BundleDigest, HostBuildJobID: built.Job.JobID, PlanSHA256: planDigest,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := rt.appendCandidateShot(ctx, operation, payload); err != nil {
		log.Printf("selfdev candidate shot: operation %s: write the %s verdict: %v", operation.OperationID, payload.Verdict, err)
		return
	}
	rt.shotWorkersMu.Lock()
	delete(rt.shotWorkers, key)
	rt.shotWorkersMu.Unlock()
}

func (rt *Runtime) runCandidateShotWorker(key string, state *shotWorkerState, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
	terminal := false
	defer func() {
		if r := recover(); r != nil {
			log.Printf("selfdev candidate shot: operation %s: worker panicked: %v", operation.OperationID, r)
		}
		rt.shotWorkersMu.Lock()
		state.running = false
		if terminal {
			delete(rt.shotWorkers, key) // N5: nothing will restart a worker for it
		}
		rt.shotWorkersMu.Unlock()
	}()
	rt.sweepShotWorkRootOnce()
	ctx, cancel := context.WithTimeout(context.Background(), candidateShotWorkerTimeout)
	defer cancel()
	// A terminal operation gets no shots; a missing store is a wiring defect
	// the watchdog will count.
	current, err := rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
	if err != nil {
		log.Printf("selfdev candidate shot: operation %s: read: %v", operation.OperationID, err)
		return
	}
	if current.State != selfdev.StateAccepted {
		log.Printf("selfdev candidate shot: operation %s is %s, not accepted; no shots", operation.OperationID, current.State)
		terminal = true
		return
	}
	started := time.Now()
	payload := rt.shootCandidate(ctx, current, bundle, built)
	log.Printf("selfdev candidate shot: operation %s host build %s: %s in %s %s", operation.OperationID, built.Job.JobID, payload.Verdict, time.Since(started).Round(time.Millisecond), payload.Cause)
	rt.finishCandidateShot(key, current, bundle, built, func(base computerevent.CandidateShotPayload) computerevent.CandidateShotPayload {
		payload.OperationID, payload.BundleDigest, payload.HostBuildJobID, payload.PlanSHA256 = base.OperationID, base.BundleDigest, base.HostBuildJobID, base.PlanSHA256
		return payload
	})
}

// shotSide is one pass's directory: a state copy, a files layer, the
// binaries and the outputs.
type shotSide struct {
	name, dir, binary, frontend string
}

func (s shotSide) spec(plan capsule.ShotPlan, env shotEnv) shotsandbox.Spec {
	return shotsandbox.Spec{
		Binary: s.binary, Frontend: s.frontend, StorePath: filepath.Join(s.dir, "state"), FilesRoot: filepath.Join(s.dir, "files"),
		OutDir: filepath.Join(s.dir, "out"), WorkDir: filepath.Join(s.dir, "work"), Plan: plan,
		Browser: env.browser, Ffmpeg: env.ffmpeg, Fontconfig: env.fontconfig,
		ViewTimeoutS: 60, PlanTimeoutS: int(candidateShotPassTimeout/time.Second) - 120, EncodeTimeoutS: 60, ReadyTimeoutS: 90,
	}
}

type shotEnv struct{ browser, ffmpeg, fontconfig string }

func (d candidateShotDeps) env(plan capsule.ShotPlan) (shotEnv, error) {
	pick := func(set string, keys ...string) string {
		if strings.TrimSpace(set) != "" {
			return set
		}
		for _, k := range keys {
			if v := strings.TrimSpace(os.Getenv(k)); v != "" {
				return v
			}
		}
		return ""
	}
	env := shotEnv{
		browser:    pick(d.browser, capsule.BrowserEnv, "CHOIR_BROWSER_BIN"),
		ffmpeg:     pick(d.ffmpeg, capsule.FfmpegEnv, "CHOIR_FFMPEG_BIN"),
		fontconfig: pick(d.fontconfig, capsule.FontconfigEnv, "CHOIR_FONTCONFIG_FILE"),
	}
	if env.browser == "" {
		return env, errors.New("this computer has no browser for the shots (CHOIR_BROWSER_BIN is unset)")
	}
	if plan.Video != nil && env.ffmpeg == "" {
		return env, errors.New("the plan asks for a video and this computer has no ffmpeg (CHOIR_FFMPEG_BIN is unset)")
	}
	return env, nil
}

// shootCandidate runs the whole shot for one operation and host build and
// returns its verdict. Every failure is a verdict with its cause; only the
// event write can fail outside it.
func (rt *Runtime) shootCandidate(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) (payload computerevent.CandidateShotPayload) {
	fail := func(format string, args ...any) computerevent.CandidateShotPayload {
		payload.Verdict, payload.Cause = computerevent.CandidateShotFail, truncateRunesShot(fmt.Sprintf(format, args...), 1800)
		return payload
	}
	if bundle.ShotPlan == nil {
		return fail("the bundle has no shot plan")
	}
	plan := *bundle.ShotPlan
	deps := rt.shotDeps()
	env, err := deps.env(plan)
	if err != nil {
		return fail("%v", err)
	}
	updaterRoot := strings.TrimSpace(rt.selfdevUpdaterRoot)
	if updaterRoot == "" || rt.store == nil {
		return fail("no updater root or store to shoot over")
	}

	// 1. The binaries and frontends of the two builds.
	candidateBinary, candidateFrontend, err := rt.materializeCandidateBuild(built, updaterRoot)
	if err != nil {
		return fail("candidate build: %v", err)
	}
	runningBinary, runningFrontend, err := rt.runningBuild(deps, updaterRoot)
	if err != nil {
		return fail("running build: %v", err)
	}

	// 2. The work directory: wiped first, so a re-run starts from scratch.
	root := deps.workRoot
	if root == "" {
		root = defaultShotWorkRoot()
	}
	reachTop := deps.reachTop
	if reachTop == "" {
		reachTop = "/"
	}
	opDir := filepath.Join(root, safeShotDirName(operation.OperationID))
	if err := prepareShotWorkRoot(root); err != nil {
		return fail("work root: %v", err)
	}
	_ = os.RemoveAll(opDir)
	if err := os.Mkdir(opDir, 0o711); err != nil {
		return fail("work directory: %v", err)
	}
	if err := os.Chmod(opDir, 0o711); err != nil {
		return fail("work directory: %v", err)
	}
	defer os.RemoveAll(opDir)
	uid, gid := capsule.ShotSandboxIDs()
	before := shotSide{name: "before", dir: filepath.Join(opDir, "before"), binary: filepath.Join(opDir, "bin", "running"), frontend: filepath.Join(opDir, "fe", "running")}
	after := shotSide{name: "after", dir: filepath.Join(opDir, "after"), binary: filepath.Join(opDir, "bin", "candidate"), frontend: filepath.Join(opDir, "fe", "candidate")}
	// The builds are reachable by name (0711); the side directories, which
	// will hold the state copies, are the sandbox user's alone (0700 after
	// the hand-over). Modes are set, not left to the umask.
	for d, mode := range map[string]fs.FileMode{filepath.Join(opDir, "bin"): 0o711, filepath.Join(opDir, "fe"): 0o711, before.dir: 0o700, after.dir: 0o700} {
		if err := mkdirMode(d, mode); err != nil {
			return fail("work directory: %v", err)
		}
	}
	for side, src := range map[shotSide][2]string{before: {runningBinary, runningFrontend}, after: {candidateBinary, candidateFrontend}} {
		if err := linkOrCopyFile(src[0], side.binary, 0o755); err != nil {
			return fail("%s binary: %v", side.name, err)
		}
		if err := linkOrCopyTree(src[1], side.frontend); err != nil {
			return fail("%s frontend: %v", side.name, err)
		}
		for p, need := range map[string]fs.FileMode{side.binary: 0o005, filepath.Join(side.frontend, "index.html"): 0o004} {
			if err := checkSandboxReach(reachTop, p, need); err != nil {
				return fail("%s: the sandbox user cannot reach the staged build: %v", side.name, err)
			}
		}
	}

	// 3. The state: one snapshot with the capsule copy's withheld kinds, a
	// plain copy of it for the before run, and a files layer per run.
	estimate, err := rt.store.SnapshotEstimate(ctx)
	if err != nil {
		return fail("state estimate: %v", err)
	}
	if err := capsule.CheckStateCopyFits(opDir, 2*estimate); err != nil {
		return fail("state copy: %v", err)
	}
	receipt, err := rt.store.SnapshotToKinds(ctx, after.dir, lease.CapsuleStoreKinds())
	if err != nil {
		return fail("state copy: %v", err)
	}
	payload.Copy = &computerevent.CandidateShotCopyReceipt{CopyMs: receipt.CopyMS, ScanMs: receipt.ScanMS, Bytes: receipt.Bytes}
	if err := copyStateCopy(after.dir, before.dir); err != nil {
		return fail("state copy for the before run: %v", err)
	}
	if err := capsule.StateVolumeAboveFloor(opDir); err != nil {
		return fail("state copy: %v", err)
	}
	filesRoot, sourceRoot := rt.filesRoot(), strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_SOURCE_ROOT"))
	for _, side := range []shotSide{before, after} {
		files := filepath.Join(side.dir, "files")
		if err := mkdirMode(files, 0o755); err != nil {
			return fail("files layer: %v", err)
		}
		if _, err := capsule.BuildShotFilesLayer(ctx, filesRoot, sourceRoot, files, uid, gid); err != nil {
			return fail("files layer: %v", err)
		}
		for _, d := range []string{"out", "work"} {
			if err := mkdirMode(filepath.Join(side.dir, d), 0o700); err != nil {
				return fail("work directory: %v", err)
			}
		}
		// The sandbox opens the store in WAL mode and must create
		// state.db-wal and -shm beside the db, so the side directory itself
		// is its own (S-B1). Below it only what the worker made is handed
		// over, never the files layer's linked inodes, which are the live
		// ones (the layer's directories were made owned by the walk); an
		// absent entry, a store with no sidecar, is skipped (S-B2).
		if err := deps.handOver(side.dir, uid, gid, "out", "work", "state.db", "state.texture"); err != nil {
			return fail("hand the work directory over: %v", err)
		}
	}

	// 4. The runs: the running build first, then the candidate, one plan.
	launch := deps.launch
	if launch == nil {
		launch = func(ctx context.Context, spec shotsandbox.Spec) (shotsandbox.Result, error) {
			return shotsandbox.Launch(ctx, "", spec)
		}
	}
	beforePlan := plan
	beforePlan.Video = nil // one video, of the candidate
	passes := []struct {
		side shotSide
		plan capsule.ShotPlan
		what string
	}{{before, beforePlan, "the running build"}, {after, plan, "the candidate"}}
	results := map[string]shotsandbox.Result{}
	// Each pass's shots are read as soon as it ends: both passes run as the
	// same sandbox user with no mount namespace, so a candidate could
	// otherwise rewrite the running build's shots before they are judged.
	taken := map[string]map[string][]byte{}
	for _, pass := range passes {
		passCtx, cancel := context.WithTimeout(ctx, candidateShotPassTimeout)
		result, err := launch(passCtx, pass.side.spec(pass.plan, env))
		cancel()
		if err != nil {
			return fail("the shots of %s did not run: %v", pass.what, err)
		}
		if result.Error != "" {
			cause := fmt.Sprintf("the shots of %s failed: %s", pass.what, result.Error)
			if result.Tail != "" && !strings.Contains(result.Error, result.Tail) {
				cause += " (stderr tail: " + result.Tail + ")"
			}
			return fail("%s", cause)
		}
		results[pass.side.name] = result
		taken[pass.side.name] = readShotOutputs(pass.side, pass.plan)
	}

	// 5. The pairs, named NN-before-<view> and NN-after-<view>.
	var screens []capsule.ScreenFile
	contents := map[string][]byte{}
	var total int64
	for i, view := range plan.Views {
		for _, side := range []shotSide{before, after} {
			name := fmt.Sprintf("%02d-%s-%s.png", i+1, side.name, view.Name)
			data, ok := taken[side.name][view.Name+".png"]
			if !ok || !capsule.ValidScreenName(name) {
				return fail("%s: the %s shot of view %s was not taken", name, side.name, view.Name)
			}
			if total += int64(len(data)); total > capsule.MaxScreensTotalBytes {
				return fail("the shots exceed %d bytes", capsule.MaxScreensTotalBytes)
			}
			width, height, blank, err := capsule.CheckScreenPNG(data)
			if err != nil {
				return fail("%s: %v", name, err)
			}
			sum := sha256.Sum256(data)
			screens = append(screens, capsule.ScreenFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data)), Width: width, Height: height, Blank: blank})
			contents[name] = data
		}
	}
	var video *computerevent.CandidateShotFile
	if plan.Video != nil {
		name := plan.Video.FileName()
		data, ok := taken[after.name][name]
		if !ok || !capsule.IsWebM(data) || len(data) > capsule.MaxVideoBytes {
			return fail("the plan's video %s was not recorded as a WebM of at most %d bytes", name, capsule.MaxVideoBytes)
		}
		sum := sha256.Sum256(data)
		video = &computerevent.CandidateShotFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))}
		contents[name] = data
	}

	// 6. Save them, so the review can resolve them and the owner can see
	// them whatever the verdict.
	images, files := rt.imageStore(), rt.filesRoot()
	for _, s := range screens {
		payload.Shots = append(payload.Shots, computerevent.CandidateShotFile{Name: s.Name, SHA256: s.SHA256, Bytes: s.Bytes})
		if err := storeScreenBytes(images, files, operation.OperationID, s.Name, s.SHA256, contents[s.Name], true); err != nil {
			return fail("save %s: %v", s.Name, err)
		}
	}
	if video != nil {
		payload.Shots = append(payload.Shots, *video)
		if err := storeScreenBytes(nil, files, operation.OperationID, video.Name, video.SHA256, contents[video.Name], false); err != nil {
			return fail("save %s: %v", video.Name, err)
		}
	}

	// 7. The review (FM8): the six questions of every pair, the numbered
	// shots of the owner's request checked, fail-closed.
	request, notes := rt.requestAndNotesForOperation(ctx, operation)
	call, err := rt.shotReviewCaller(rt.selfdevRouteOwnerID)
	if err != nil {
		return fail("the shot review did not run: %v", err)
	}
	review, err := reviewShotsNoted(ctx, call, screens, requestedShots(request), notes, images.Has)
	if err != nil {
		return fail("the shot review did not run: %v", err)
	}
	if raw, merr := json.Marshal(review); merr == nil {
		payload.Review = string(raw)
	}
	if review.rejected() {
		return fail("%s", strings.TrimPrefix(review.reason(""), shotReviewMarker+" "))
	}
	payload.Verdict = computerevent.CandidateShotPass
	return payload
}

// materializeCandidateBuild puts the host build's closure in the updater's
// private store (idempotent and content-addressed, as the apply needs it
// anyway) and returns the candidate autoputer and its extracted frontend.
func (rt *Runtime) materializeCandidateBuild(built hostBuiltRelease, updaterRoot string) (binary, frontend string, err error) {
	result := built.Job.Result
	entry := filepath.Clean("/" + result.LayeringEntrypoint)
	if result.LayeringEntrypoint == "" || entry == "/" || strings.HasPrefix(entry, "/../") || entry == "/.." {
		return "", "", fmt.Errorf("the host build's entrypoint %q is not a path in the private store", result.LayeringEntrypoint)
	}
	blob, err := os.ReadFile(filepath.Join(built.Dir, "closure.nar"))
	if err != nil {
		return "", "", fmt.Errorf("closure: %w", err)
	}
	if sum := sha256.Sum256(blob); hex.EncodeToString(sum[:]) != result.ClosureDigest {
		return "", "", errors.New("the staged closure is not the signed one")
	}
	store := filepath.Join(updaterRoot, "store")
	if _, err := updater.MaterializeClosure(blob, store); err != nil {
		return "", "", fmt.Errorf("materialize closure: %w", err)
	}
	binary = filepath.Join(store, entry)
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", "", fmt.Errorf("the candidate autoputer %s is not an executable in the closure", binary)
	}
	frontend = filepath.Join(built.Dir, "frontend")
	if _, err := os.Stat(filepath.Join(frontend, "index.html")); err != nil {
		return "", "", fmt.Errorf("the candidate frontend has no index.html: %w", err)
	}
	return binary, frontend, nil
}

// runningBuild is the binary and frontend this computer is running now.
func (rt *Runtime) runningBuild(deps candidateShotDeps, updaterRoot string) (binary, frontend string, err error) {
	binary = deps.runningBinary
	if binary == "" {
		self, err := os.Executable()
		if err != nil {
			return "", "", fmt.Errorf("own binary: %w", err)
		}
		binary = self
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}
	frontend = deps.runningFrontend
	if frontend == "" {
		for _, candidate := range []string{filepath.Join(updaterRoot, "current", "frontend"), filepath.Join(strings.TrimSpace(os.Getenv("CHOIR_BASELINE_RELEASE_ROOT")), "frontend")} {
			if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
				if _, err := os.Stat(filepath.Join(resolved, "index.html")); err == nil {
					frontend = resolved
					break
				}
			}
		}
	}
	if frontend == "" {
		return "", "", errors.New("the running build's frontend was not found")
	}
	return binary, frontend, nil
}

// shotReviewCaller is the judge's per-shot model call (selfdev_shot_review.go).
func (rt *Runtime) shotReviewCaller(ownerID string) (shotReviewCall, error) {
	if rt.shotReviewCall != nil {
		return rt.shotReviewCall, nil
	}
	if rt.modelPolicy == nil {
		return nil, errors.New("the multimodal verifier is unavailable")
	}
	return func(ctx context.Context, prompt string, refs []string) (modelpolicy.VerifyOutcome, error) {
		return rt.modelPolicy.VerifyImages(ctx, ownerID, shotReviewRole, prompt, refs)
	}, nil
}

// requestAndNotesForOperation reads the owner's words the operation's
// assignment answers, the way Verify does; "" when unreachable.
func (rt *Runtime) requestAndNotesForOperation(ctx context.Context, operation selfdev.Operation) (request, notes string) {
	assignment, ok := rt.assignmentOfOperation(ctx, operation)
	if !ok || rt.store == nil {
		return "", ""
	}
	ownerID := rt.selfdevRouteOwnerID
	notes = rt.ownerNotesForAssignment(ctx, ownerID, operation.ComputerID, assignment)
	if words, _, _, ok := rt.ownerWordsForAssignment(ctx, ownerID, operation.ComputerID, assignment); ok {
		request = words.Text
	}
	return request, notes
}

// assignmentOfOperation finds the assignment whose current operation this is.
func (rt *Runtime) assignmentOfOperation(ctx context.Context, operation selfdev.Operation) (types.EngineeringAssignment, bool) {
	if rt.store == nil || rt.selfdevOperations == nil {
		return types.EngineeringAssignment{}, false
	}
	assignments, err := rt.store.ListEngineeringAssignments(ctx, rt.selfdevRouteOwnerID, operation.ComputerID, operation.TrajectoryID)
	if err != nil {
		return types.EngineeringAssignment{}, false
	}
	for _, a := range assignments {
		op, _, found, err := assignmentOperation(ctx, rt.selfdevOperations, operation.ComputerID, a.AssignmentID)
		if err == nil && found && op.OperationID == operation.OperationID {
			return a, true
		}
	}
	return types.EngineeringAssignment{}, false
}

// readCandidateShot reads the operation's candidate_shot event for the host
// build job its bundle derives (the job id is a function of the request), and
// checks its bindings. Not found says none yet.
func (rt *Runtime) readCandidateShot(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle) (computerevent.CandidateShotPayload, bool) {
	if rt == nil || rt.store == nil || rt.eventPayloadReader == nil || bundle.ShotPlan == nil {
		return computerevent.CandidateShotPayload{}, false
	}
	request, err := rt.selfDevelopmentHostBuildRequest(operation, bundle)
	if err != nil {
		return computerevent.CandidateShotPayload{}, false
	}
	jobID, err := request.JobID()
	if err != nil {
		return computerevent.CandidateShotPayload{}, false
	}
	event, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, computerevent.CandidateShotKey(operation.OperationID, jobID))
	if err != nil || !found || event.EventKind != computerevent.EventCandidateShot {
		return computerevent.CandidateShotPayload{}, false
	}
	raw, err := rt.eventPayloadReader.FetchPayload(ctx, operation.ComputerID, event.PayloadCommitment)
	if err != nil {
		return computerevent.CandidateShotPayload{}, false
	}
	var payload computerevent.CandidateShotPayload
	planDigest, derr := bundle.ShotPlan.Digest()
	if json.Unmarshal(raw, &payload) != nil || payload.Validate() != nil || derr != nil ||
		payload.OperationID != operation.OperationID || payload.BundleDigest != operation.BundleDigest || payload.HostBuildJobID != jobID || payload.PlanSHA256 != planDigest {
		return computerevent.CandidateShotPayload{}, false
	}
	return payload, true
}

// candidateShotScreens lists a plan bundle's screens from its candidate_shot
// event, as CandidateScreens does a screens bundle's from the bundle.
func (rt *Runtime) candidateShotScreens(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle) (ScreensListing, error) {
	payload, ok := rt.readCandidateShot(ctx, operation, bundle)
	if !ok {
		return ScreensListing{}, errors.New("the platform has not shot this candidate yet")
	}
	out := ScreensListing{OperationID: operation.OperationID, TrajectoryID: operation.TrajectoryID, ComputerID: operation.ComputerID, BundleDigest: operation.BundleDigest}
	store := rt.imageStore()
	dir := yaegikernel.SelfDevelopmentScreensDir(operation.OperationID)
	for _, s := range payload.Shots {
		if !capsule.ValidScreenName(s.Name) {
			continue // the video is a file, not a screen
		}
		row := ScreenListingRow{Name: s.Name, File: dir + "/" + s.Name, SHA256: s.SHA256, Bytes: s.Bytes, Resolvable: store.Has(s.SHA256)}
		row.Width, row.Height = screenDimensions(rt.filesRoot(), dir+"/"+s.Name)
		out.Screens = append(out.Screens, row)
	}
	if len(out.Screens) == 0 {
		return ScreensListing{}, errors.New("the candidate's shots recorded no screens")
	}
	return out, nil
}

// candidateShotScreensNote is frozenScreensNote for a plan bundle.
func candidateShotScreensNote(toolCtx *CapsuleToolCtx, ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle) string {
	payload, ok := toolCtx.CandidateShot(ctx, operation, bundle)
	if !ok {
		return platformScreensHeading + ": none yet; the platform shoots a plan bundle's views after the host build, before the swap."
	}
	var b strings.Builder
	b.WriteString(platformScreensHeading + " (candidate shot " + payload.Verdict + "):")
	for _, s := range payload.Shots {
		fmt.Fprintf(&b, "\n- %s: artifact:sha256:%s", s.Name, s.SHA256)
		if !strings.HasSuffix(s.Name, ".png") {
			b.WriteString(" (video)")
		}
	}
	if payload.Verdict == computerevent.CandidateShotFail && payload.Cause != "" {
		b.WriteString("\n- failed: " + payload.Cause)
	}
	return b.String()
}

// screenDimensions reads a saved PNG's size from the file tree, 0 when absent.
func screenDimensions(filesRoot, rel string) (int, int) {
	data, err := os.ReadFile(filepath.Join(filesRoot, filepath.FromSlash(rel)))
	if err != nil {
		return 0, 0
	}
	w, h, _, err := capsule.CheckScreenPNG(data)
	if err != nil {
		return 0, 0
	}
	return w, h
}

// defaultShotWorkRoot is where the per-operation work directories go: a
// sibling of the capsule state directory on the same volume (the data disk
// the floor check guards), else the temp directory. Not inside it: the
// capsule state directory is root-only (0700), and the sandbox user must
// traverse to what the worker stages for it (apply-proof-4, 2026-11-07).
func defaultShotWorkRoot() string {
	if v := strings.TrimSpace(os.Getenv("CHOIR_SHOT_WORK_DIR")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_STATE_DIR")); v != "" {
		return filepath.Join(filepath.Dir(filepath.Clean(v)), "choir-shot")
	}
	return filepath.Join(os.TempDir(), "choir-shot")
}

// prepareShotWorkRoot makes the work root root-owned and traverse-only for
// others (0711): the sandbox user can reach an operation directory by name
// but cannot list the root.
func prepareShotWorkRoot(root string) error {
	if err := os.MkdirAll(root, 0o711); err != nil {
		return err
	}
	// A root someone else made (the temp fallback is world-writable) could
	// have its operation directories swapped between the check and the run.
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", root)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Geteuid() {
		return fmt.Errorf("%s is owned by uid %d, not this process", root, st.Uid)
	}
	return os.Chmod(root, 0o711)
}

// checkSandboxReach fails unless others can traverse every directory below
// top down to path's parent and have the need bits on path (0o004 to read,
// 0o005 to exec): what the sandbox user, which is neither the owner nor in
// the group, needs. top itself is not checked; the worker passes "/".
func checkSandboxReach(top, path string, need fs.FileMode) error {
	top = filepath.Clean(top)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(top, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return fmt.Errorf("%s is not under %s", path, top)
	}
	dir := top
	parts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
	for _, part := range parts {
		if part == "." {
			continue
		}
		dir = filepath.Join(dir, part)
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o001 == 0 {
			return fmt.Errorf("%s is %v: others cannot traverse it", dir, info.Mode().Perm())
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&need != need {
		return fmt.Errorf("%s is %v: others lack %v on it", path, info.Mode().Perm(), need)
	}
	return nil
}

// readShotOutputs reads what a pass left for its plan: a PNG per view and the
// plan's video, each at most the bound its check will apply; a missing or
// oversized one is left out and fails by name when the pair is assembled.
func readShotOutputs(side shotSide, plan capsule.ShotPlan) map[string][]byte {
	names := map[string]int64{}
	for _, view := range plan.Views {
		names[view.Name+".png"] = capsule.MaxScreensTotalBytes
	}
	if plan.Video != nil {
		names[plan.Video.FileName()] = int64(capsule.MaxVideoBytes)
	}
	out := map[string][]byte{}
	for name, limit := range names {
		f, err := os.Open(filepath.Join(side.dir, "out", name))
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, limit+1))
		f.Close()
		if err != nil || int64(len(data)) > limit {
			continue
		}
		out[name] = data
	}
	return out
}

// mkdirMode makes dir with exactly mode, whatever the umask.
func mkdirMode(dir string, mode fs.FileMode) error {
	if err := os.Mkdir(dir, mode); err != nil {
		return err
	}
	return os.Chmod(dir, mode)
}

func safeShotDirName(operationID string) string {
	if plainOperationID.MatchString(operationID) && !strings.Contains(operationID, "..") {
		return operationID
	}
	sum := sha256.Sum256([]byte(operationID))
	return hex.EncodeToString(sum[:8])
}

// linkOrCopyFile puts src at dst: a hardlink when the volume allows it (the
// store's files are immutable and world-readable), else a copy.
func linkOrCopyFile(src, dst string, mode fs.FileMode) error {
	// A hardlink keeps the source's mode: link only a source that already
	// grants others what the copy would.
	// A source others can write would stay writable at a reachable name.
	if info, err := os.Stat(src); err == nil && info.Mode().Perm()&mode&0o005 == mode&0o005 && info.Mode().Perm()&0o022 == 0 {
		if err := os.Link(src, dst); err == nil {
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Chmod(mode); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// linkOrCopyTree does that for a directory of regular files.
func linkOrCopyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			return os.Chmod(target, 0o755)
		case d.Type().IsRegular():
			return linkOrCopyFile(path, target, 0o644)
		}
		return fmt.Errorf("%s is not a regular file", path)
	})
}

// copyStateCopy copies a store copy's state.db and state.texture/ from one
// directory to another: plain files, so both runs start identical.
func copyStateCopy(from, to string) error {
	if err := copyPlainFile(filepath.Join(from, "state.db"), filepath.Join(to, "state.db")); err != nil {
		return err
	}
	root := filepath.Join(from, "state.texture")
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			return copyPlainFile(path, target)
		}
		return fmt.Errorf("%s is not a regular file", path)
	})
}

func copyPlainFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
