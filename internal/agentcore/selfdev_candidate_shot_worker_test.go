//go:build linux

package agentcore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/shotsandbox"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The candidate-shot worker (selfdev_candidate_shot_worker.go, design 3.3).
// Failure modes, decided before the code:
//
//   - FM4 a run writes the live state, or the two runs do not start alike:
//     the sandbox gets private copies that are identical at the start and
//     independent after, and the lease-withheld kinds are not in them;
//   - FM5 the plans differ: both passes get the frozen plan (the before one
//     without the video);
//   - FM8 the review model fails or rejects: the verdict is a fail, with the
//     screens still saved for the owner to read;
//   - FM9 the candidate does not serve: the verdict carries its cause and
//     stderr tail, and nothing swaps;
//   - FM6/N4 a worker that dies silently is restarted twice; the third death
//     writes "shot worker died";
//   - a terminal operation gets no shots.

// shotLauncher stands in for the sandbox: it records what each pass got and
// writes a PNG per plan view into the pass's output directory.
type shotLauncher struct {
	mu          sync.Mutex
	specs       map[string]shotsandbox.Spec
	order       []string
	digests     map[string]string
	binaries    map[string]string
	retained    map[string]string
	frontends   map[string]string
	failAfter   *shotsandbox.Result
	panicAlways bool
	panics      atomic.Int32
}

func sideOf(spec shotsandbox.Spec) string {
	if strings.Contains(spec.WorkDir, string(filepath.Separator)+"before"+string(filepath.Separator)) {
		return "before"
	}
	return "after"
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func testPNG(t *testing.T, w, h, seed int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x + seed), uint8(y), uint8(x ^ y), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (l *shotLauncher) launch(t *testing.T) func(context.Context, shotsandbox.Spec) (shotsandbox.Result, error) {
	return func(_ context.Context, spec shotsandbox.Spec) (shotsandbox.Result, error) {
		if l.panicAlways {
			l.panics.Add(1)
			panic("the sandbox exploded")
		}
		side := sideOf(spec)
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.specs == nil {
			l.specs, l.digests, l.binaries, l.retained, l.frontends = map[string]shotsandbox.Spec{}, map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}
		}
		l.specs[side], l.order = spec, append(l.order, side)
		l.digests[side] = fileDigest(t, spec.StorePath+".db")
		l.binaries[side] = string(mustRead(t, spec.Binary))
		l.frontends[side] = string(mustRead(t, filepath.Join(spec.Frontend, "index.html")))
		if side == "after" && l.failAfter != nil {
			return *l.failAfter, nil
		}
		for i, v := range spec.Plan.Views {
			seed := 0
			if side == "after" {
				seed = 40
			}
			if err := os.WriteFile(filepath.Join(spec.OutDir, v.Name+".png"), testPNG(t, 1440, 900, seed+i), 0o644); err != nil {
				return shotsandbox.Result{}, err
			}
		}
		if side == "before" {
			// A before run that scribbles on its state copy must not reach
			// the after run's.
			f, err := os.OpenFile(spec.StorePath+".db", os.O_APPEND|os.O_WRONLY, 0)
			if err == nil {
				_, _ = f.WriteString("scribble")
				f.Close()
			}
		}
		return shotsandbox.Result{}, nil
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type workerFixture struct {
	*derivableSelfDevFixture
	launcher  *shotLauncher
	judge     *scriptedJudge
	filesRoot string
	opID      string
}

func newWorkerFixture(t *testing.T, computerID string, answer func(name string) string) *workerFixture {
	t.Helper()
	fx, _ := newHostBuildFixture(t, computerID, "done")
	wf := &workerFixture{derivableSelfDevFixture: fx, launcher: &shotLauncher{}, filesRoot: t.TempDir()}
	t.Setenv("AUTOPUTER_FILES_ROOT", wf.filesRoot)
	if err := os.WriteFile(filepath.Join(wf.filesRoot, "notes.txt"), []byte("the owner's file"), 0o644); err != nil {
		t.Fatal(err)
	}
	running := filepath.Join(t.TempDir(), "running-autoputer")
	if err := os.WriteFile(running, []byte("running-build"), 0o755); err != nil {
		t.Fatal(err)
	}
	frontend := t.TempDir()
	if err := os.WriteFile(filepath.Join(frontend, "index.html"), []byte("<html>running</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	wf.judge = &scriptedJudge{answer: func(name string) string {
		if answer != nil {
			return answer(name)
		}
		return allYes
	}}
	fx.configure(func(rt *Runtime) {
		rt.candidateShotStarter = nil // the real worker
		rt.cfg.StorePath = filepath.Join(t.TempDir(), "runtime.db")
		rt.shotReviewCall = wf.judge.call
		rt.candidateShotDeps = &candidateShotDeps{
			launch: wf.launcher.launch(t), workRoot: filepath.Join(t.TempDir(), "shot"),
			runningBinary: running, runningFrontend: frontend, browser: "chromium-fake",
		}
		// A lease is a kind the capsule copy withholds.
		if err := rt.store.Leases().PutLease(context.Background(), lease.Record{LeaseID: "lease-shot", OwnerID: "owner", ComputerID: computerID, Holder: "engineering", HolderRunID: "run:shot", State: lease.StateActive}); err != nil {
			t.Fatal(err)
		}
	})
	return wf
}

// drive freezes an operation, decides it, and runs the materializer until the
// shot worker has written its verdict or the operation leaves accepted.
func (wf *workerFixture) drive(t *testing.T, opID string) selfdev.Operation {
	t.Helper()
	operation, bundleDigest, _ := wf.seedOperationToAwaitingApproval(t, opID)
	wf.opID = operation.OperationID
	wf.appendDecision(t, operation, bundleDigest, "decision-"+opID)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		current, err := wf.operations.Get(context.Background(), wf.computerID, wf.opID)
		if err != nil {
			t.Fatal(err)
		}
		if current.State != selfdev.StateAccepted && current.State != selfdev.StateMaterializing && current.State != selfdev.StateAwaitingApproval {
			return current
		}
		// The production retry wake polls a pending operation again; the
		// test stands in for it.
		if current.State == selfdev.StateAccepted {
			wf.rt.reconcileSelfDevelopmentMaterialization(context.Background())
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("operation %s did not leave accepted", wf.opID)
	return selfdev.Operation{}
}

func (wf *workerFixture) shotEvent(t *testing.T) computerevent.CandidateShotPayload {
	t.Helper()
	operation, err := wf.operations.Get(context.Background(), wf.computerID, wf.opID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(wf.updaterRoot, "incoming", operation.BundleDigest, "bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bundle = decodeBundleForTest(t, raw)
	payload, ok := wf.rt.readCandidateShot(context.Background(), operation, bundle)
	if !ok {
		t.Fatalf("no candidate_shot event for %s", wf.opID)
	}
	return payload
}

func TestCandidateShotWorkerShootsBeforeAndAfterAndRecordsThePass(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-pass", nil)
	final := wf.drive(t, "op-worker-pass")
	if final.State != selfdev.StateApplied {
		t.Fatalf("operation = %s (%s), want applied after a passing shot", final.State, final.TerminalError)
	}
	payload := wf.shotEvent(t)
	if payload.Verdict != computerevent.CandidateShotPass || len(payload.Shots) != 2 || payload.Copy == nil || payload.Copy.Bytes <= 0 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Shots[0].Name != "01-before-home.png" || payload.Shots[1].Name != "01-after-home.png" {
		t.Fatalf("shot names = %+v", payload.Shots)
	}
	if payload.Review == "" || !strings.Contains(payload.Review, "vision-x") {
		t.Fatalf("review = %q", payload.Review)
	}

	l := wf.launcher
	if len(l.order) != 2 || l.order[0] != "before" || l.order[1] != "after" {
		t.Fatalf("passes = %v, want the running build first, then the candidate", l.order)
	}
	// The running build serves the before pass, the host build's binary the after.
	if l.binaries["before"] != "running-build" || !strings.Contains(l.binaries["after"], "host-built") {
		t.Fatalf("binaries = %q", l.binaries)
	}
	// FM5: one plan for both, the video only where the plan asks.
	if len(l.specs["before"].Plan.Views) != 1 || l.specs["before"].Plan.Views[0].Name != l.specs["after"].Plan.Views[0].Name || l.specs["before"].Plan.Views[0].Path != l.specs["after"].Plan.Views[0].Path {
		t.Fatalf("plans differ: %+v vs %+v", l.specs["before"].Plan, l.specs["after"].Plan)
	}
	if !strings.Contains(l.frontends["after"], "host-built selfdev-probe") || !strings.Contains(l.frontends["before"], "running") {
		t.Fatal("a pass was handed the wrong frontend")
	}
	// FM4: identical copies at the start, independent after, private paths.
	if l.digests["before"] != l.digests["after"] {
		t.Fatalf("the two state copies differ at the start: %s vs %s", l.digests["before"], l.digests["after"])
	}
	for _, side := range []string{"before", "after"} {
		if strings.HasPrefix(l.specs[side].StorePath, filepath.Dir(wf.rt.cfg.StorePath)) || strings.HasPrefix(l.specs[side].FilesRoot, wf.filesRoot) {
			t.Fatalf("the %s pass was handed a live path: %+v", side, l.specs[side])
		}
	}
	// The copy withholds the kinds an engineering capsule's copy withholds:
	// the live store still has the lease, the copy has none.
	if live, _ := wf.rt.store.Leases().ListLeases(context.Background(), "owner", wf.computerID); len(live) != 1 {
		t.Fatalf("the live store lost its lease: %d", len(live))
	}
	// The work directory is gone after the run.
	if entries, _ := os.ReadDir(wf.rt.candidateShotDeps.workRoot); len(entries) != 0 {
		t.Fatalf("work directory left behind: %v", entries)
	}
	// The screens are saved where the existing screens are: the asset store and Self-development/<op>/screens/.
	for _, s := range payload.Shots {
		path := filepath.Join(wf.filesRoot, filepath.FromSlash(yaegikernel.SelfDevelopmentScreensDir(wf.opID)), s.Name)
		if fileDigest(t, path) != s.SHA256 {
			t.Fatalf("%s in the file tree is not the recorded shot", s.Name)
		}
		if !wf.rt.imageStore().Has(s.SHA256) {
			t.Fatalf("%s is not in the asset store", s.Name)
		}
	}
	// CandidateScreens and the Report's note read the event for a plan bundle.
	listing, err := wf.rt.CandidateScreens(context.Background(), wf.computerID, wf.opID)
	if err != nil || len(listing.Screens) != 2 || listing.Screens[0].Width != 1440 || !listing.Screens[1].Resolvable {
		t.Fatalf("CandidateScreens = %+v, %v", listing, err)
	}
	toolCtx := wf.rt.assignedEngineeringCapsuleToolCtx(&types.RunRecord{RunID: "run-worker", ComputerID: wf.computerID, OwnerID: "owner"}, "")
	if toolCtx.CandidateShot == nil {
		t.Fatal("the tool context cannot read the candidate shot")
	}
	operation, _ := wf.operations.Get(context.Background(), wf.computerID, wf.opID)
	bundle := decodeBundleForTest(t, mustRead(t, filepath.Join(wf.updaterRoot, "incoming", operation.BundleDigest, "bundle.json")))
	if note := candidateShotScreensNote(toolCtx, context.Background(), operation, bundle); !strings.Contains(note, "01-after-home.png") || !strings.Contains(note, "pass") {
		t.Fatalf("note = %q", note)
	}
}

// The lease-withheld kinds are not in the copies the passes get, and the live
// owner file the layer links is read-only evidence: its bytes are untouched.
func TestCandidateShotCopyWithholdsLeasesAndLinksOwnerFiles(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-copy", nil)
	var leases, owned atomic.Int32
	inner := wf.rt.candidateShotDeps.launch
	wf.configure(func(rt *Runtime) {
		rt.candidateShotDeps.launch = func(ctx context.Context, spec shotsandbox.Spec) (shotsandbox.Result, error) {
			db, err := sql.Open("sqlite", dbx.SQLiteDSN(spec.StorePath+".db", dbx.SQLiteOptions{}, false))
			if err != nil {
				t.Error(err)
			} else {
				var n int
				if err := db.QueryRow(`SELECT COUNT(*) FROM og_objects WHERE object_kind = 'choir.lease'`).Scan(&n); err == nil {
					leases.Add(int32(n))
				}
				db.Close()
			}
			if data, err := os.ReadFile(filepath.Join(spec.FilesRoot, "notes.txt")); err == nil && string(data) == "the owner's file" {
				owned.Add(1)
			}
			return inner(ctx, spec)
		}
	})
	wf.drive(t, "op-worker-copy")
	if leases.Load() != 0 {
		t.Fatalf("the shot copy holds %d leases: it must withhold what a capsule copy withholds", leases.Load())
	}
	if owned.Load() != 2 {
		t.Fatalf("the owner's file reached %d of 2 passes", owned.Load())
	}
	if string(mustRead(t, filepath.Join(wf.filesRoot, "notes.txt"))) != "the owner's file" {
		t.Fatal("the live owner file changed")
	}
}

// FM9: a candidate that does not serve closes the operation failed with its
// stderr tail, and nothing swaps.
func TestCandidateShotCandidateThatDoesNotServeFailsTheOperation(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-noserve", nil)
	wf.launcher.failAfter = &shotsandbox.Result{Error: "the candidate autoputer exited before port 41234 accepted connections: panic: store schema is newer", Tail: "panic: store schema is newer"}
	final := wf.drive(t, "op-worker-noserve")
	if final.State != selfdev.StateFailed || !strings.Contains(final.TerminalError, "shot review: the shots of the candidate failed") || !strings.Contains(final.TerminalError, "store schema is newer") {
		t.Fatalf("operation = %s %q", final.State, final.TerminalError)
	}
	if wf.applyCalls.Load() != 0 || wf.service.restarts != 0 {
		t.Fatal("a candidate that did not serve was swapped in")
	}
	if payload := wf.shotEvent(t); payload.Verdict != computerevent.CandidateShotFail {
		t.Fatalf("payload = %+v", payload)
	}
}

// FM8: the review rejecting a pair is a fail naming the shot, and the screens
// are still saved so the owner can see what was rejected.
func TestCandidateShotReviewRejectionFailsAndKeepsTheScreens(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-reject", func(name string) string {
		if strings.Contains(name, "01-after-home.png") {
			return `"q1":"yes","q2":"no","q3":"yes","q4":"n/a","q6":"yes","why":"the list is empty"`
		}
		return allYes
	})
	final := wf.drive(t, "op-worker-reject")
	if final.State != selfdev.StateFailed || !strings.Contains(final.TerminalError, "shot review: rejected 1 of 2 shots: 01-after-home.png Q2") {
		t.Fatalf("operation = %s %q", final.State, final.TerminalError)
	}
	payload := wf.shotEvent(t)
	if payload.Verdict != computerevent.CandidateShotFail || len(payload.Shots) != 2 {
		t.Fatalf("payload = %+v", payload)
	}
	if _, err := os.Stat(filepath.Join(wf.filesRoot, filepath.FromSlash(yaegikernel.SelfDevelopmentScreensDir(wf.opID)), "01-after-home.png")); err != nil {
		t.Fatalf("the rejected screen was not kept: %v", err)
	}
}

// FM8: a review model that errors is a fail, never a pass by default.
func TestCandidateShotReviewModelErrorFailsClosed(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-modelerr", nil)
	wf.judge.err = errors.New("gateway 502")
	final := wf.drive(t, "op-worker-modelerr")
	if final.State != selfdev.StateFailed || !strings.Contains(final.TerminalError, "the shot review did not run") || !strings.Contains(final.TerminalError, "gateway 502") {
		t.Fatalf("operation = %s %q", final.State, final.TerminalError)
	}
	if wf.applyCalls.Load() != 0 {
		t.Fatal("an unreviewed candidate was swapped in")
	}
}

// N4: a worker that dies silently is restarted at most twice; the third
// silent death writes the failing verdict "shot worker died".
func TestCandidateShotWatchdogRestartsTwiceThenFails(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-died", nil)
	wf.launcher.panicAlways = true
	final := wf.drive(t, "op-worker-died")
	if final.State != selfdev.StateFailed || !strings.Contains(final.TerminalError, "shot review: "+candidateShotWorkerDied) {
		t.Fatalf("operation = %s %q", final.State, final.TerminalError)
	}
	if n := wf.launcher.panics.Load(); n != candidateShotMaxStarts {
		t.Fatalf("the worker ran %d times before the verdict, want the first start and two restarts (%d)", n, candidateShotMaxStarts)
	}
	wf.rt.shotWorkersMu.Lock()
	remaining := len(wf.rt.shotWorkers)
	wf.rt.shotWorkersMu.Unlock()
	if remaining != 0 {
		t.Fatalf("the watchdog kept %d entries after the verdict", remaining)
	}
}

// A worker finds the operation no longer accepted and writes nothing.
func TestCandidateShotWorkerSkipsATerminalOperation(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-terminal", nil)
	operation, _, _ := wf.seedOperationToAwaitingApproval(t, "op-worker-terminal")
	// awaiting approval is not accepted: no shots are taken for it.
	state := &shotWorkerState{running: true, starts: 1}
	wf.rt.shotWorkersMu.Lock()
	wf.rt.shotWorkers = map[string]*shotWorkerState{"k": state}
	wf.rt.shotWorkersMu.Unlock()
	bundle := decodeBundleForTest(t, mustRead(t, filepath.Join(wf.updaterRoot, "incoming", operation.BundleDigest, "bundle.json")))
	wf.rt.runCandidateShotWorker("k", state, operation, bundle, hostBuiltRelease{})
	if n := len(wf.launcher.order); n != 0 {
		t.Fatalf("a non-accepted operation was shot (%d passes)", n)
	}
	if wf.countEventKinds()[computerevent.EventCandidateShot] != 0 {
		t.Fatal("a non-accepted operation got a candidate_shot")
	}
	if state.running {
		t.Fatal("the worker did not mark itself finished")
	}
}

// The event is idempotent on (operation, host build job): a re-run after a
// planned restart cannot write a second one.
func TestAppendCandidateShotIsIdempotentPerHostBuild(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-idem", nil)
	wf.configure(func(rt *Runtime) {
		rt.candidateShotStarter = func(selfdev.Operation, transaction.CapsuleEffectBundle, hostBuiltRelease) {}
	})
	operation, bundleDigest, _ := wf.seedOperationToAwaitingApproval(t, "op-worker-idem")
	wf.appendDecision(t, operation, bundleDigest, "decision-op-worker-idem")
	operation = wf.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	plan := strings.Repeat("c", 64)
	pass := computerevent.CandidateShotPayload{HostBuildJobID: "job-1", PlanSHA256: plan, Verdict: computerevent.CandidateShotPass, Shots: []computerevent.CandidateShotFile{{Name: "01-before-home.png", SHA256: strings.Repeat("a", 64), Bytes: 1}}}
	fail := computerevent.CandidateShotPayload{HostBuildJobID: "job-1", PlanSHA256: plan, Verdict: computerevent.CandidateShotFail, Cause: "late"}
	for _, p := range []computerevent.CandidateShotPayload{pass, fail} {
		if err := wf.rt.appendCandidateShot(context.Background(), operation, p); err != nil {
			t.Fatal(err)
		}
	}
	if n := wf.countEventKinds()[computerevent.EventCandidateShot]; n != 1 {
		t.Fatalf("candidate_shot events = %d, want 1", n)
	}
}

func TestCandidateShotEnvNamesTheMissingBrowserAndFfmpeg(t *testing.T) {
	t.Setenv("CHOIR_BROWSER", "")
	t.Setenv("CHOIR_BROWSER_BIN", "")
	t.Setenv("CHOIR_FFMPEG", "")
	t.Setenv("CHOIR_FFMPEG_BIN", "")
	plan := capsule.ShotPlan{Views: []capsule.ShotView{{Name: "home", Path: "/", Viewport: "desktop"}}, Video: &capsule.ShotVideo{View: "home", Seconds: 3}}
	if _, err := (candidateShotDeps{}).env(plan); err == nil || !strings.Contains(err.Error(), "no browser") {
		t.Fatalf("no browser: %v", err)
	}
	if _, err := (candidateShotDeps{browser: "b"}).env(plan); err == nil || !strings.Contains(err.Error(), "no ffmpeg") {
		t.Fatalf("no ffmpeg: %v", err)
	}
	if env, err := (candidateShotDeps{browser: "b", ffmpeg: "f"}).env(plan); err != nil || env.ffmpeg != "f" {
		t.Fatalf("env = %+v %v", env, err)
	}
	t.Setenv("CHOIR_BROWSER_BIN", "/nix/chromium")
	t.Setenv("CHOIR_FONTCONFIG_FILE", "/nix/fonts.conf")
	env, err := (candidateShotDeps{}).env(capsule.ShotPlan{})
	if err != nil || env.browser != "/nix/chromium" || env.fontconfig != "/nix/fonts.conf" {
		t.Fatalf("env = %+v %v", env, err)
	}
}

func decodeBundleForTest(t *testing.T, raw []byte) transaction.CapsuleEffectBundle {
	t.Helper()
	var bundle transaction.CapsuleEffectBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	return bundle
}
