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

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/shotsandbox"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The shot job (selfdev_shot_job.go, desk_shots.go; docs/design/desk-shots-and-review-2026-11-07.md
// 3.3, FM4-FM9, FM13, FM14). Failure modes, decided before the code:
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
	mu        sync.Mutex
	specs     map[string]shotsandbox.Spec
	order     []string
	digests   map[string]string
	binaries  map[string]string
	retained  map[string]string
	frontends map[string]string
	sideModes map[string]os.FileMode
	// rewriteBefore makes the after pass overwrite the before pass's shots,
	// as a harness-aware candidate running as the same user could.
	rewriteBefore bool
	failAfter     *shotsandbox.Result
	blank         bool
	panicAlways   bool
	panics        atomic.Int32
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
		if l.sideModes == nil {
			l.sideModes = map[string]os.FileMode{}
		}
		l.sideModes[side] = mustStat(t, filepath.Dir(spec.StorePath)).Mode().Perm()
		if side == "after" && l.failAfter != nil {
			return *l.failAfter, nil
		}
		for i, v := range spec.Plan.Views {
			seed := 0
			if side == "after" {
				seed = 40
			}
			data := testPNG(t, 1440, 900, seed+i)
			if l.blank {
				data = blankPNG(t, 1440, 900)
			}
			if err := os.WriteFile(filepath.Join(spec.OutDir, v.Name+".png"), data, 0o644); err != nil {
				return shotsandbox.Result{}, err
			}
		}
		if side == "after" && l.rewriteBefore {
			for i, v := range spec.Plan.Views {
				forged := filepath.Join(filepath.Dir(filepath.Dir(spec.OutDir)), "before", "out", v.Name+".png")
				if err := os.WriteFile(forged, testPNG(t, 1440, 900, 40+i), 0o644); err != nil {
					return shotsandbox.Result{}, err
				}
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
	filesRoot string
	opID      string
	exec      toolregistry.ExecutionContext
}

func newWorkerFixture(t *testing.T, computerID string) *workerFixture {
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
	fx.configure(func(rt *Runtime) {
		rt.shotsAskedHook = nil // management is the test below
		rt.cfg.StorePath = filepath.Join(t.TempDir(), "runtime.db")
		shotTop := t.TempDir()
		rt.candidateShotDeps = &candidateShotDeps{
			launch: wf.launcher.launch(t), workRoot: filepath.Join(shotTop, "shot"), reachTop: shotTop,
			runningBinary: running, runningFrontend: frontend, browser: "chromium-fake",
		}
		// A lease is a kind the capsule copy withholds.
		if err := rt.store.Leases().PutLease(context.Background(), lease.Record{LeaseID: "lease-shot", OwnerID: "owner", ComputerID: computerID, Holder: "engineering", HolderRunID: "run:shot", State: lease.StateActive}); err != nil {
			t.Fatal(err)
		}
	})
	wf.exec = toolregistry.ExecutionContext{RunID: "run-mgmt-1", OwnerID: "owner-shots", ComputerID: computerID}
	return wf
}

// owed drives an operation to accepted with its host build done and the gate's
// shots-owed record committed, and returns it.
func (wf *workerFixture) owed(t *testing.T, opID string) selfdev.Operation {
	t.Helper()
	operation, bundleDigest, _ := wf.seedOperationToAwaitingApproval(t, opID)
	wf.opID = operation.OperationID
	wf.appendDecision(t, operation, bundleDigest, "decision-"+opID)
	accepted := wf.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	if err := wf.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false); !errors.Is(err, errCandidateShotPending) {
		t.Fatalf("the gate = %v, want pending with the shots asked", err)
	}
	return accepted
}

func (wf *workerFixture) start(t *testing.T) yaegikernel.ShotStartResult {
	t.Helper()
	raw, err := wf.rt.shotStartEgress(context.Background(), "management", wf.exec, json.RawMessage(`{"operation_id":"`+wf.opID+`"}`))
	if err != nil {
		t.Fatalf("ShotStart: %v", err)
	}
	var out yaegikernel.ShotStartResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (wf *workerFixture) wait(t *testing.T, jobID string) yaegikernel.ShotWaitResult {
	t.Helper()
	payload, _ := json.Marshal(yaegikernel.ShotWaitPayload{JobID: jobID, MaxSeconds: 60})
	raw, err := wf.rt.shotWaitEgress(context.Background(), "management", wf.exec, payload)
	if err != nil {
		t.Fatalf("ShotWait: %v", err)
	}
	var out yaegikernel.ShotWaitResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestShotJobShootsBeforeAndAfterAndRecordsTheResult(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-pass")
	wf.launcher.rewriteBefore = true
	wf.owed(t, "op-worker-pass")
	job := wf.start(t)
	// FM5: a second call is the same job.
	if again := wf.start(t); again.JobID != job.JobID {
		t.Fatalf("ShotStart twice = %s then %s", job.JobID, again.JobID)
	}
	res := wf.wait(t, job.JobID)
	if res.State != shotJobDone || len(res.Screens) != 2 || !res.ImagesQueued || res.RecordedSize != "1440x900" {
		t.Fatalf("result = %+v", res)
	}
	if res.Screens[0].Name != "01-before-home.png" || res.Screens[1].Name != "01-after-home.png" {
		t.Fatalf("shot names = %+v", res.Screens)
	}
	// The before shot is the running build's, read before the candidate ran.
	if want := sha256.Sum256(testPNG(t, 1440, 900, 0)); res.Screens[0].SHA256 != hex.EncodeToString(want[:]) {
		t.Fatal("the recorded before shot is not the running build's: the candidate rewrote it")
	}
	// The images are queued for this run as one user turn with the job's refs.
	turns, _ := wf.rt.shotImageInjector(&types.RunRecord{RunID: wf.exec.RunID, AgentProfile: agentprofile.Management})(false)
	if len(turns) != 1 {
		t.Fatalf("queued turns = %d", len(turns))
	}
	refs := imageRefsOf(turns)
	for _, s := range res.Screens {
		if !refs[shotImageRef(s.SHA256)] {
			t.Fatalf("the injected turn lacks %s", s.Name)
		}
	}
	l := wf.launcher
	if len(l.order) != 2 || l.order[0] != "before" || l.order[1] != "after" {
		t.Fatalf("passes = %v, want the running build first, then the candidate", l.order)
	}
	if l.binaries["before"] != "running-build" || !strings.Contains(l.binaries["after"], "host-built") {
		t.Fatalf("binaries = %q", l.binaries)
	}
	// FM5: one plan for both.
	if len(l.specs["before"].Plan.Views) != 1 || l.specs["before"].Plan.Views[0].Name != l.specs["after"].Plan.Views[0].Name {
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
		if mode := l.sideModes[side]; mode&0o077 != 0 {
			t.Fatalf("the %s side directory is %v: others can reach its state copy", side, mode)
		}
	}
	if live, _ := wf.rt.store.Leases().ListLeases(context.Background(), "owner", wf.computerID); len(live) != 1 {
		t.Fatalf("the live store lost its lease: %d", len(live))
	}
	if entries, _ := os.ReadDir(wf.rt.candidateShotDeps.workRoot); len(entries) != 0 {
		t.Fatalf("work directory left behind: %v", entries)
	}
	for _, s := range res.Screens {
		path := filepath.Join(wf.filesRoot, filepath.FromSlash(yaegikernel.SelfDevelopmentScreensDir(wf.opID)), s.Name)
		if fileDigest(t, path) != s.SHA256 {
			t.Fatalf("%s in the file tree is not the recorded shot", s.Name)
		}
		if !wf.rt.imageStore().Has(s.SHA256) {
			t.Fatalf("%s is not in the asset store", s.Name)
		}
	}
}

// FM7, FM13: a job still running in the durable result when this process runs
// none is closed "restarted" by whatever reads it; a later ShotStart starts a
// new job with a new id.
func TestShotJobOpenAtBootIsRestartedAndAFreshStartMakesANewJob(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-boot")
	wf.owed(t, "op-worker-boot")
	job := wf.start(t)
	if res := wf.wait(t, job.JobID); res.State != shotJobDone {
		t.Fatalf("first job = %+v", res)
	}
	// Plant an open job as a crashed process left it.
	key := shotJobKey(wf.opID, mustHostJobID(t, wf))
	rec, ok, err := wf.rt.readShotJobFile(key)
	if err != nil || !ok {
		t.Fatal(err)
	}
	rec.State, rec.Cause = shotJobRunning, ""
	if err := wf.rt.writeShotJobFile(rec); err != nil {
		t.Fatal(err)
	}
	if res := wf.wait(t, job.JobID); res.State != shotJobRestarted || res.Cause != shotJobEndedByRestart {
		t.Fatalf("after a boot = %+v, want restarted", res)
	}
	next := wf.start(t)
	if next.JobID == job.JobID {
		t.Fatal("ShotStart after a restart returned the dead job")
	}
	if res := wf.wait(t, next.JobID); res.State != shotJobDone {
		t.Fatalf("new job = %+v", res)
	}
	// The dead job's id is not the current job.
	if res := wf.wait(t, job.JobID); res.State != shotJobStale {
		t.Fatalf("a superseded job id = %+v", res)
	}
}

func mustHostJobID(t *testing.T, wf *workerFixture) string {
	t.Helper()
	info, refusal, err := wf.rt.shotsOwedFor(context.Background(), wf.computerID, wf.opID)
	if err != nil || refusal != "" {
		t.Fatalf("owed: %q %v", refusal, err)
	}
	return info.HostJobID
}

// FM14: ShotWait twice queues the images once per run (the second finds them
// in run memory), and a run that would hold more than the request cap is
// refused by name.
func TestShotWaitQueuesOncePerRunAndRefusesOverTheCap(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-queue")
	wf.owed(t, "op-worker-queue")
	job := wf.start(t)
	wf.wait(t, job.JobID)
	inject := wf.rt.shotImageInjector(&types.RunRecord{RunID: wf.exec.RunID, AgentProfile: agentprofile.Management})
	turns, _ := inject(false)
	if len(turns) != 1 {
		t.Fatalf("turns = %d", len(turns))
	}
	// The tool loop persists what it injects.
	appendMemoryMessage(t, wf.rt, wf.exec, turns[0])
	if res := wf.wait(t, job.JobID); !res.ImagesQueued {
		t.Fatalf("second wait = %+v", res)
	}
	if again, _ := inject(false); len(again) != 0 {
		t.Fatalf("the second ShotWait queued %d turns, want none: the images are in memory", len(again))
	}
	// Held 2 + 19 fresh blocks is over the cap: refused by name.
	if err := wf.rt.queueImageSet(context.Background(), wf.exec, "test", screensOf("c", 19, 0), json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "request cap") {
		t.Fatalf("held + fresh over the cap = %v", err)
	}
}

// The lease-withheld kinds are not in the copies the passes get, and the live
// owner file the layer links is read-only evidence: its bytes are untouched.
func TestShotJobCopyWithholdsLeasesAndLinksOwnerFiles(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-copy")
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
	wf.owed(t, "op-worker-copy")
	wf.wait(t, wf.start(t).JobID)
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

// FM6, FM9: a candidate that does not serve is a failed job with its stderr
// tail, and a sandbox that panics is a failed job too.
func TestShotJobFailuresAreFailedStatesWithTheirCause(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-noserve")
	wf.launcher.failAfter = &shotsandbox.Result{Error: "the candidate autoputer exited before port 41234 accepted connections: panic: store schema is newer", Tail: "panic: store schema is newer"}
	wf.owed(t, "op-worker-noserve")
	res := wf.wait(t, wf.start(t).JobID)
	if res.State != shotJobFailed || !strings.Contains(res.Cause, "the shots of the candidate failed") || !strings.Contains(res.Cause, "store schema is newer") {
		t.Fatalf("result = %+v", res)
	}
	wf2 := newWorkerFixture(t, "computer-worker-panic")
	wf2.launcher.panicAlways = true
	wf2.owed(t, "op-worker-panic")
	if res := wf2.wait(t, wf2.start(t).JobID); res.State != shotJobFailed || !strings.Contains(res.Cause, "panicked") {
		t.Fatalf("panic result = %+v", res)
	}
}

// The deterministic checks stay code: a blank shot fails the job.
func TestShotJobBlankShotFailsTheJob(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-blank")
	wf.launcher.blank = true
	wf.owed(t, "op-worker-blank")
	if res := wf.wait(t, wf.start(t).JobID); res.State != shotJobFailed || !strings.Contains(res.Cause, "blank") {
		t.Fatalf("result = %+v", res)
	}
}

// FM8: the shot verbs are management's.
func TestShotVerbsRefuseAnyOtherDesk(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-desk")
	wf.owed(t, "op-worker-desk")
	if _, err := wf.rt.shotStartEgress(context.Background(), "research", wf.exec, json.RawMessage(`{"operation_id":"`+wf.opID+`"}`)); err == nil {
		t.Fatal("research started shots")
	}
	if _, err := wf.rt.shotWaitEgress(context.Background(), "engineering", wf.exec, json.RawMessage(`{"job_id":"shot-x"}`)); err == nil {
		t.Fatal("engineering waited on shots")
	}
}

// FM9: an operation that owes no shots (not yet asked) is refused.
func TestShotStartRefusesAnOperationThatOwesNoShots(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-notowed")
	operation, _, _ := wf.seedOperationToAwaitingApproval(t, "op-worker-notowed")
	wf.opID = operation.OperationID
	if _, err := wf.rt.shotStartEgress(context.Background(), "management", wf.exec, json.RawMessage(`{"operation_id":"`+wf.opID+`"}`)); err == nil || !strings.Contains(err.Error(), "accepted") {
		t.Fatalf("ShotStart on an unaccepted operation = %v", err)
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

func blankPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
