//go:build linux

package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// jobFixture is an active capsule with a workspace and a job's log in its
// scratch, an executor bound to it, and one job that ended.
type jobFixture struct {
	executor *Executor
	caps     *Capsule
	job      *capsuleJob
	logSum   string
	got      []JobCompletion
}

func newJobFixture(t *testing.T, state string, exit int, log string) *jobFixture {
	t.Helper()
	merged := filepath.Join(t.TempDir(), "root")
	subject := filepath.Join(merged, "workspace", "platform")
	if err := os.MkdirAll(subject, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subject, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDigest, err := digestCanonicalSubjectTree(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	jobDir := filepath.Join(merged, "tmp", "choir-jobs", "job-build")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, capsulejob.LogFile), []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(log))
	caps := &Capsule{ID: "capsule-job", State: StateActive, MergedDir: merged, SourceSnapshotDigest: sourceDigest}
	capability := &Capability{CapabilityID: "cap-job", Handle: "handle-job", AgentRunID: "run-job",
		AgentRole: RoleEngineering, TargetCapsule: caps.ID, ExpiresAt: time.Now().Add(time.Hour)}
	f := &jobFixture{caps: caps, logSum: hex.EncodeToString(sum[:])}
	f.executor = &Executor{
		stateDir:          t.TempDir(),
		capsules:          map[string]*Capsule{caps.ID: caps},
		capabilities:      map[capKey]*Capability{{AgentRunID: "run-job", Handle: "handle-job"}: capability},
		revokedCaps:       map[string]bool{},
		executionReceipts: map[string]ExecutionReceipt{},
		grantedReceipts:   map[string]GrantedExecutionReceipt{},
		jobs:              map[string]*capsuleJob{},
	}
	f.executor.SetJobCompletionHandler(func(c JobCompletion) { f.got = append(f.got, c) })
	started := time.Now().UTC().Add(-3 * time.Minute)
	code := exit
	f.job = &capsuleJob{
		agentRunID: "run-job", handle: "handle-job", capsuleID: caps.ID, startDigest: sourceDigest,
		spec:   capsulejob.Spec{ID: "job-build", Owner: "run-job", Argv: []string{"go", "build", "./cmd/autoputer"}, Cwd: capsulejob.DefaultCwd, WallClockMS: 20 * 60 * 1000},
		notify: map[string]string{"run_id": "run-job"},
		status: capsulejob.Status{ID: "job-build", State: state, Argv: []string{"go", "build", "./cmd/autoputer"}, Cwd: capsulejob.DefaultCwd,
			EnvDigest: capsulejob.EnvDigest([]string{"PATH=/bin"}), WallClockMS: 20 * 60 * 1000, StartedAt: started,
			FinishedAt: started.Add(2 * time.Minute), ExitCode: &code, OutputSHA256: f.logSum, OutputBytes: int64(len(log)), Tail: log},
		done: make(chan struct{}),
	}
	f.executor.jobs["job-build"] = f.job
	return f
}

// A job that ends done gets one receipt binding what ran (argv, cwd,
// environment digest, exit code, log digest, timing, state and both tree
// digests); its log is stored by content address; the runtime is handed the
// completion; the receipt re-opens from disk after a restart and resolves
// through Freeze's resolver like any execution receipt.
func TestJobEndMintsAReceiptFreezeResolves(t *testing.T) {
	f := newJobFixture(t, capsulejob.StateDone, 0, "ok: built bin/autoputer\n")
	if _, _, running := f.executor.RunningJob("run-job"); !running {
		t.Fatal("the job does not read as running before it ends")
	}
	f.executor.finishJob(f.job, f.caps, capsulejob.StateDone, "")
	if len(f.got) != 1 || f.got[0].Receipt == nil {
		t.Fatalf("completions: %+v", f.got)
	}
	r := *f.got[0].Receipt
	if !strings.HasPrefix(r.ReceiptRef, "capsule-exec:sha256:") || r.JobID != "job-build" || r.JobState != capsulejob.StateDone ||
		r.ExitCode != 0 || strings.Join(r.Argv, " ") != "go build ./cmd/autoputer" || r.Cwd != capsulejob.DefaultCwd ||
		r.EnvDigest != f.job.status.EnvDigest || r.StdoutDigest != f.logSum || r.DurationMS != 120000 ||
		r.Command != "job job-build: go build ./cmd/autoputer" || r.AgentRunID != "run-job" ||
		r.CapabilityHandleDigest != computerevent.DigestBytes([]byte("handle-job")) || r.SourceTreeDigest != f.caps.SourceSnapshotDigest ||
		r.WorktreeDigest != r.StartWorktreeDigest || r.WorktreeDigest == "" {
		t.Fatalf("receipt: %+v", r)
	}
	if r.LogRef != "capsule-job-log:sha256:"+f.logSum {
		t.Fatalf("log ref %q", r.LogRef)
	}
	stored, err := os.ReadFile(filepath.Join(f.executor.artifactRoot(), "job-logs", f.logSum+".log"))
	if err != nil || string(stored) != "ok: built bin/autoputer\n" {
		t.Fatalf("stored log: %q %v", stored, err)
	}
	if refusal := JobReceiptOutcomeRefusal(r, true); refusal != "" {
		t.Fatalf("a done job's receipt is refused: %s", refusal)
	}
	if _, _, running := f.executor.RunningJob("run-job"); running {
		t.Fatal("the job still reads as running")
	}
	if views := f.executor.JobViews("run-job"); len(views) != 1 || views[0].ReceiptRef != r.ReceiptRef || views[0].LogRef != r.LogRef {
		t.Fatalf("views: %+v", views)
	}

	restarted := &Executor{stateDir: f.executor.stateDir, executionReceipts: map[string]ExecutionReceipt{}}
	reopened, err := restarted.OpenExecutionReceipt(r.ReceiptRef)
	if err != nil {
		t.Fatalf("re-open after restart: %v", err)
	}
	a, _ := json.Marshal(reopened)
	b, _ := json.Marshal(r)
	if string(a) != string(b) {
		t.Fatalf("re-opened receipt differs:\n%s\n%s", a, b)
	}

	f.caps.State = StateFrozen
	resolved, err := f.executor.ResolveGrantedExecutionReceipts(context.Background(), "run-job", "handle-job", []string{r.ReceiptRef})
	if err != nil || len(resolved) != 1 || resolved[0].JobID != "job-build" {
		t.Fatalf("Freeze's resolver: %+v %v", resolved, err)
	}

	var rec jobRecord
	raw, err := os.ReadFile(filepath.Join(f.executor.jobRecordDir(), "job-build.json"))
	if err != nil || json.Unmarshal(raw, &rec) != nil || rec.State != capsulejob.StateDone || rec.ReceiptRef != r.ReceiptRef {
		t.Fatalf("job record: %s %v", raw, err)
	}
}

// Failed, timed-out and stopped jobs get receipts that record the failure
// and are refused in every slot; a job during which the tree changed is
// refused in the build and test slots, not the toolchain slot; a cell
// receipt keeps its meaning (exit 0 accepted, a panic's exit 1 refused).
func TestJobReceiptOutcomeRefusals(t *testing.T) {
	for _, tc := range []struct {
		state string
		exit  int
	}{
		{capsulejob.StateFailed, 2},
		{capsulejob.StateTimeout, 0},
		{capsulejob.StateTimeout, -1},
		{capsulejob.StateStopped, 0},
	} {
		f := newJobFixture(t, tc.state, tc.exit, "boom\n")
		f.executor.finishJob(f.job, f.caps, tc.state, "")
		r := *f.got[0].Receipt
		if r.JobState != tc.state || r.ExitCode == 0 {
			t.Fatalf("%s/%d: receipt state %s exit %d", tc.state, tc.exit, r.JobState, r.ExitCode)
		}
		for _, buildOrTest := range []bool{true, false} {
			if JobReceiptOutcomeRefusal(r, buildOrTest) == "" {
				t.Fatalf("%s/%d receipt accepted (build/test %v)", tc.state, tc.exit, buildOrTest)
			}
		}
	}

	f := newJobFixture(t, capsulejob.StateDone, 0, "ok\n")
	f.job.startDigest = "a-different-tree"
	f.executor.finishJob(f.job, f.caps, capsulejob.StateDone, "")
	r := *f.got[0].Receipt
	if JobReceiptOutcomeRefusal(r, true) == "" || JobReceiptOutcomeRefusal(r, false) != "" {
		t.Fatalf("tree changed during the job: build/test %q, toolchain %q", JobReceiptOutcomeRefusal(r, true), JobReceiptOutcomeRefusal(r, false))
	}
	if JobReceiptOutcomeRefusal(ExecutionReceipt{Command: "go_eval:x", ExitCode: 0}, true) != "" ||
		JobReceiptOutcomeRefusal(ExecutionReceipt{Command: "go_eval:x", ExitCode: 1}, true) == "" {
		t.Fatal("a cell receipt's meaning changed")
	}
}

// The log a cell can write is never trusted: a log whose bytes differ from
// the broker's digest, or a symlink planted in its place, is not stored, and
// the receipt still binds the broker's digest.
func TestJobLogIsNotStoredWhenTampered(t *testing.T) {
	f := newJobFixture(t, capsulejob.StateDone, 0, "ok\n")
	logPath := filepath.Join(f.caps.MergedDir, "tmp", "choir-jobs", "job-build", capsulejob.LogFile)
	if err := os.WriteFile(logPath, []byte("forged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ref := f.executor.storeJobLog(f.caps, f.job.status); ref != "" {
		t.Fatalf("tampered log stored as %s", ref)
	}
	secret := filepath.Join(t.TempDir(), "host-secret")
	if err := os.WriteFile(secret, []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(logPath)
	if err := os.Symlink(secret, logPath); err != nil {
		t.Fatal(err)
	}
	if ref := f.executor.storeJobLog(f.caps, f.job.status); ref != "" {
		t.Fatalf("a symlinked log was followed and stored as %s", ref)
	}
	f.executor.finishJob(f.job, f.caps, capsulejob.StateDone, "")
	if r := f.got[0].Receipt; r == nil || r.LogRef != "" || r.StdoutDigest != f.logSum {
		t.Fatalf("receipt: %+v", r)
	}
}

// A capsule that ends first closes its job as stopped, with no receipt; the
// runtime is still told, so a turn waiting on the job is released.
func TestJobClosedWithItsCapsule(t *testing.T) {
	f := newJobFixture(t, capsulejob.StateRunning, 0, "")
	f.job.status.ExitCode = nil
	delete(f.executor.capsules, f.caps.ID)
	if !f.executor.pollJob(f.job) {
		t.Fatal("the job did not close with its capsule")
	}
	if len(f.got) != 1 || f.got[0].Receipt != nil || f.got[0].View.State != capsulejob.StateStopped || f.job.receiptRef != "" || f.job.status.State != capsulejob.StateStopped {
		t.Fatalf("closed job: %+v, completions %d", f.job.status, len(f.got))
	}
}

// Restart rule: a job left open by an earlier process is closed as
// interrupted at the next construction, never resumed; an ended one is left
// alone.
func TestOpenJobsAreClosedAsInterruptedAtBoot(t *testing.T) {
	artifacts := t.TempDir()
	dir := filepath.Join(artifacts, "jobs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for id, state := range map[string]string{"job-open": capsulejob.StateRunning, "job-ended": capsulejob.StateDone} {
		if err := capsulejob.WriteJSONAtomic(dir, id+".json", jobRecord{ID: id, State: state}); err != nil {
			t.Fatal(err)
		}
	}
	e := NewExecutorWithArtifacts(t.TempDir(), artifacts, t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "no-broker"), 1<<30)
	if len(e.jobs) != 0 {
		t.Fatalf("a job was resumed: %v", e.jobs)
	}
	for id, want := range map[string]string{"job-open": capsulejob.StateInterrupted, "job-ended": capsulejob.StateDone} {
		var rec jobRecord
		raw, _ := os.ReadFile(filepath.Join(dir, id+".json"))
		if json.Unmarshal(raw, &rec) != nil || rec.State != want {
			t.Fatalf("%s: %s", id, raw)
		}
		if want == capsulejob.StateInterrupted && rec.Reason != "interrupted by a restart" {
			t.Fatalf("%s reason %q", id, rec.Reason)
		}
	}
}
