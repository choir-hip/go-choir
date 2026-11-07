package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/shotsandbox"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// The shot job (docs/design/desk-shots-and-review-2026-11-07.md 3.3). Management
// starts one with choir.ShotStart for an accepted operation that owes shots;
// it runs outside the materializer lock, shoots the frozen plan against the
// running build and then against the host-built candidate, over private
// copies of the computer's state, in a loopback-only sandbox
// (internal/shotsandbox), saves the screens, and records a durable result
// keyed by (operation, host build job). The platform judges nothing: management
// looks at the images with its own model and records choir.ShotVerdict.
//
// Failure modes, decided before the code (FM numbers are the design's):
//
//   - a shot writes the live state: the store is read once through
//     SnapshotToKinds into the job's own directory, the before copy is a
//     plain file copy of that, the owner files are hardlinks the sandbox
//     cannot write (it runs as the capsule's unprivileged id), and nothing
//     but directories the job made is ever chowned.
//   - the two runs differ: one frozen plan drives both; the before run takes
//     no video, the after run takes the plan's.
//   - FM6 a hang or a failure (sandbox, browser, state copy, blank or
//     mis-sized shot): the sandbox pass has its own deadlines, the job one
//     for the whole run, and every failure is the job's failed state with its
//     cause, which management's verdict then names.
//   - FM7 a restart: the sandbox and browser die with the process, so a job
//     still "running" in the durable result when none runs in this process is
//     closed "ended by a restart" the first time anything reads it, and a
//     later ShotStart starts a new job with a new id.
//   - the candidate does not start or serve: the sandbox Result names it with
//     the stderr tail.
//   - a disk that cannot hold the copies: the capsule state floor check
//     refuses, as a capsule copy would.

const (
	// candidateShotWorkerTimeout bounds the whole job, copies to result.
	candidateShotWorkerTimeout = 30 * time.Minute
	// candidateShotPassTimeout bounds one sandbox pass (before or after).
	candidateShotPassTimeout = 10 * time.Minute
)

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
// returns the job's result. Every failure is a failed result with its cause.
func (rt *Runtime) shootCandidate(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) (result shotJobResult) {
	fail := func(format string, args ...any) shotJobResult {
		result.State, result.Cause = shotJobFailed, truncateRunesShot(fmt.Sprintf(format, args...), 1800)
		return result
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
	// The production layout must leave the browser room for its socket
	// under work/ (apply-proof-4 r2); a test's injected root is a long temp
	// path and runs no browser. Checked before anything is staged.
	if deps.workRoot == "" {
		for _, side := range []shotSide{before, after} {
			if err := shotsandbox.CheckWorkDirLength(filepath.Join(side.dir, "work")); err != nil {
				return fail("%s: %v", side.name, err)
			}
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
	_ = receipt
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
	// Each pass's shots are read as soon as it ends: both passes run as the
	// same sandbox user with no mount namespace, so a candidate could
	// otherwise rewrite the running build's shots before they are judged.
	taken := map[string]map[string][]byte{}
	for _, pass := range passes {
		passCtx, cancel := context.WithTimeout(ctx, candidateShotPassTimeout)
		res, err := launch(passCtx, pass.side.spec(pass.plan, env))
		cancel()
		if err != nil {
			return fail("the shots of %s did not run: %v", pass.what, err)
		}
		if res.Error != "" {
			cause := fmt.Sprintf("the shots of %s failed: %s", pass.what, res.Error)
			if res.Tail != "" && !strings.Contains(res.Error, res.Tail) {
				cause += " (stderr tail: " + res.Tail + ")"
			}
			return fail("%s", cause)
		}
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
			if blank {
				return fail("%s: the %s shot of view %s is blank", name, side.name, view.Name)
			}
			if !shotSizeOK(width, height) {
				return fail("%s: the %s shot of view %s is %dx%d, not a recorded size", name, side.name, view.Name, width, height)
			}
			sum := sha256.Sum256(data)
			screens = append(screens, capsule.ScreenFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data)), Width: width, Height: height, Blank: blank})
			contents[name] = data
		}
	}
	var video *computerevent.ShotFile
	if plan.Video != nil {
		name := plan.Video.FileName()
		data, ok := taken[after.name][name]
		if !ok || !capsule.IsWebM(data) || len(data) > capsule.MaxVideoBytes {
			return fail("the plan's video %s was not recorded as a WebM of at most %d bytes", name, capsule.MaxVideoBytes)
		}
		sum := sha256.Sum256(data)
		video = &computerevent.ShotFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))}
		contents[name] = data
	}

	// 6. Save them, so management's images resolve and the owner can see
	// them whatever the verdict.
	images, files := rt.imageStore(), rt.filesRoot()
	for _, s := range screens {
		if err := storeScreenBytes(images, files, operation.OperationID, s.Name, s.SHA256, contents[s.Name], true); err != nil {
			return fail("save %s: %v", s.Name, err)
		}
		result.Screens = append(result.Screens, shotJobScreen{Name: s.Name, SHA256: s.SHA256, Bytes: s.Bytes, Width: s.Width, Height: s.Height})
	}
	if video != nil {
		if err := storeScreenBytes(nil, files, operation.OperationID, video.Name, video.SHA256, contents[video.Name], false); err != nil {
			return fail("save %s: %v", video.Name, err)
		}
		result.Video = video
	}
	result.RecordedSize = recordedSizeLine(result.Screens)
	result.State = shotJobDone
	return result
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

// safeShotDirName is a short name for an operation's work directory: the
// deepest path the sandbox makes, the browser's socket under work/, must fit
// the unix socket limit, and an operation id is 40 bytes (apply-proof-4 r2).
func safeShotDirName(operationID string) string {
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
