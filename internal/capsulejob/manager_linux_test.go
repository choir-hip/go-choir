//go:build linux

package capsulejob

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary doubles as the supervisor stage: a Manager launches it
// with supervisorMarker in the job environment, and TestMain runs the
// supervisor instead of the tests (the broker launches its own binary the
// same way, without the job-exec hardening stage in between).
const supervisorMarker = "CAPSULEJOB_TEST_SUPERVISOR=1"

func TestMain(m *testing.M) {
	if os.Getenv("CAPSULEJOB_TEST_SUPERVISOR") == "1" {
		args := os.Args
		for i, a := range args {
			if a == "--" {
				os.Exit(RunSupervisorStage(args[i+1:], 3))
			}
		}
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func testManager(t *testing.T) *Manager {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &Manager{
		Root: t.TempDir(),
		Env:  []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/tmp", supervisorMarker},
		Launch: func(spec Spec) *exec.Cmd {
			return exec.Command(self, append([]string{"-test.run=^$", "--"}, spec.Argv...)...)
		},
		KillGrace: time.Second,
	}
}

func waitEnd(t *testing.T, m *Manager, id string, d time.Duration) Status {
	t.Helper()
	st, err := m.Wait(id, "run-1", d)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Terminal() {
		t.Fatalf("job %s still %s after %s: %+v", id, st.State, d, st)
	}
	return st
}

func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	s := string(raw)
	fields := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
	return len(fields) > 0 && fields[0] != "Z"
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no pid in %s", path)
	return 0
}

// Start, poll, complete: the job runs in its cwd, its combined output is in
// the log and the tail, the digest is the stored log's, and status.json
// mirrors the end.
func TestJobStartPollComplete(t *testing.T) {
	m := testManager(t)
	cwd := t.TempDir()
	st, err := m.Start(Spec{ID: "job-ok", Owner: "run-1", Argv: []string{"sh", "-c", "pwd; echo to-stdout; echo to-stderr >&2; sleep 1; echo last"}, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateRunning || st.Cwd != cwd || st.EnvDigest != EnvDigest(m.Env) || st.WallClockMS != DefaultWallClock.Milliseconds() {
		t.Fatalf("start: %+v", st)
	}
	if poll, err := m.Status("job-ok", "run-1"); err != nil || poll.State != StateRunning {
		t.Fatalf("poll while running: %+v %v", poll, err)
	}
	st = waitEnd(t, m, "job-ok", 20*time.Second)
	if st.State != StateDone || st.ExitCode == nil || *st.ExitCode != 0 {
		t.Fatalf("end: %+v", st)
	}
	for _, want := range []string{cwd, "to-stdout", "to-stderr", "last"} {
		if !strings.Contains(st.Tail, want) {
			t.Fatalf("tail %q lacks %q", st.Tail, want)
		}
	}
	log, err := os.ReadFile(filepath.Join(m.Root, "job-ok", LogFile))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(log)
	if st.OutputSHA256 != hex.EncodeToString(sum[:]) || st.OutputBytes != int64(len(log)) || st.Truncated {
		t.Fatalf("digest %s bytes %d truncated %v; log %d bytes %x", st.OutputSHA256, st.OutputBytes, st.Truncated, len(log), sum)
	}
	if st.FinishedAt.Before(st.StartedAt) || st.ElapsedMS < 900 {
		t.Fatalf("times: %+v", st)
	}
	raw, err := os.ReadFile(filepath.Join(m.Root, "job-ok", StatusFile))
	if err != nil || !strings.Contains(string(raw), `"state": "done"`) {
		t.Fatalf("status.json: %s %v", raw, err)
	}
	if _, err := m.Status("job-ok", "run-2"); err == nil {
		t.Fatal("another run read the job")
	}
}

func TestJobFailedExitCode(t *testing.T) {
	m := testManager(t)
	if _, err := m.Start(Spec{ID: "job-fail", Owner: "run-1", Argv: []string{"sh", "-c", "echo nope; exit 3"}, Cwd: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	st := waitEnd(t, m, "job-fail", 20*time.Second)
	if st.State != StateFailed || st.ExitCode == nil || *st.ExitCode != 3 {
		t.Fatalf("end: %+v", st)
	}
	if _, err := m.Start(Spec{ID: "job-missing", Owner: "run-1", Argv: []string{"no-such-command-xyz"}, Cwd: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	st = waitEnd(t, m, "job-missing", 20*time.Second)
	if st.State != StateFailed || !strings.Contains(st.Reason, "no-such-command-xyz") {
		t.Fatalf("missing command: %+v", st)
	}
}

// The cwd defaults to /workspace/platform and must exist; it is the
// directory the command runs in.
func TestJobCwd(t *testing.T) {
	spec, err := Spec{ID: "job-a", Argv: []string{"true"}}.Normalize()
	if err != nil || spec.Cwd != DefaultCwd {
		t.Fatalf("default cwd: %+v %v", spec, err)
	}
	for _, bad := range []string{"relative/dir", "/a/../b", "/a/"} {
		if _, err := (Spec{ID: "job-a", Argv: []string{"true"}, Cwd: bad}).Normalize(); err == nil {
			t.Fatalf("cwd %q accepted", bad)
		}
	}
	m := testManager(t)
	st, err := m.Start(Spec{ID: "job-nocwd", Owner: "run-1", Argv: []string{"true"}, Cwd: "/does/not/exist"})
	if err != nil || st.State != StateFailed || !strings.Contains(st.Reason, "/does/not/exist") {
		t.Fatalf("missing cwd: %+v %v", st, err)
	}
}

// The wall clock kills the job and everything it started.
func TestJobWallClockKillsTheJob(t *testing.T) {
	m := testManager(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	start := time.Now()
	if _, err := m.Start(Spec{ID: "job-slow", Owner: "run-1", Argv: []string{"sh", "-c", "sleep 300 & echo $! > " + pidFile + "; wait"}, Cwd: t.TempDir(), WallClockMS: MinWallClock.Milliseconds()}); err != nil {
		t.Fatal(err)
	}
	child := readPID(t, pidFile)
	st := waitEnd(t, m, "job-slow", 30*time.Second)
	if st.State != StateTimeout || !strings.Contains(st.Reason, "wall clock") {
		t.Fatalf("end: %+v", st)
	}
	if elapsed := time.Since(start); elapsed > MinWallClock+10*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
	if alive(child) {
		t.Fatalf("sleep %d outlived the timed-out job", child)
	}
}

func TestJobStop(t *testing.T) {
	m := testManager(t)
	if _, err := m.Start(Spec{ID: "job-stop", Owner: "run-1", Argv: []string{"sleep", "300"}, Cwd: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := m.Stop("job-stop", "run-1"); err != nil {
		t.Fatal(err)
	}
	st := waitEnd(t, m, "job-stop", 15*time.Second)
	if st.State != StateStopped {
		t.Fatalf("end: %+v", st)
	}
}

// One job at a time; a reused id is refused; a new job starts once the
// running one ends.
func TestJobConcurrencyLimit(t *testing.T) {
	m := testManager(t)
	if _, err := m.Start(Spec{ID: "job-one", Owner: "run-1", Argv: []string{"sleep", "300"}, Cwd: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(Spec{ID: "job-two", Owner: "run-1", Argv: []string{"true"}, Cwd: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "one job runs at a time") {
		t.Fatalf("second job: %v", err)
	}
	if _, err := m.Stop("job-one", "run-1"); err != nil {
		t.Fatal(err)
	}
	waitEnd(t, m, "job-one", 15*time.Second)
	if _, err := m.Start(Spec{ID: "job-one", Owner: "run-1", Argv: []string{"true"}, Cwd: t.TempDir()}); err == nil {
		t.Fatal("reused id accepted")
	}
	if _, err := m.Start(Spec{ID: "job-two", Owner: "run-1", Argv: []string{"true"}, Cwd: t.TempDir()}); err != nil {
		t.Fatalf("after the first ended: %v", err)
	}
	waitEnd(t, m, "job-two", 15*time.Second)
}

// Orphans: a backgrounded child and a setsid grandchild the command leaves
// running are killed and reaped when it exits, so neither lingers (nor
// stays a zombie under the capsule's PID 1).
func TestJobReapsOrphans(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid not installed")
	}
	m := testManager(t)
	dir := t.TempDir()
	bg, ss := filepath.Join(dir, "bg"), filepath.Join(dir, "ss")
	script := "sleep 300 & echo $! > " + bg + "; setsid sh -c 'echo $$ > " + ss + "; exec sleep 300' & sleep 0.5; exit 0"
	if _, err := m.Start(Spec{ID: "job-orphans", Owner: "run-1", Argv: []string{"sh", "-c", script}, Cwd: dir}); err != nil {
		t.Fatal(err)
	}
	bgPID, ssPID := readPID(t, bg), readPID(t, ss)
	st := waitEnd(t, m, "job-orphans", 30*time.Second)
	if st.State != StateDone || st.Orphans != 0 {
		t.Fatalf("end: %+v", st)
	}
	for _, pid := range []int{bgPID, ssPID} {
		if alive(pid) {
			t.Fatalf("pid %d outlived its job", pid)
		}
		if err := syscall.Kill(pid, 0); err == nil || !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("pid %d still exists (zombie?): %v", pid, err)
		}
	}
}

// The log keeps at most MaxLog bytes; the digest is of what it kept and the
// count is of everything written.
func TestJobLogCap(t *testing.T) {
	m := testManager(t)
	m.MaxLog = 1000
	if _, err := m.Start(Spec{ID: "job-loud", Owner: "run-1", Argv: []string{"sh", "-c", "head -c 5000 /dev/zero | tr '\\0' x; echo; echo END"}, Cwd: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	st := waitEnd(t, m, "job-loud", 20*time.Second)
	log, _ := os.ReadFile(filepath.Join(m.Root, "job-loud", LogFile))
	sum := sha256.Sum256(log)
	if len(log) != 1000 || !st.Truncated || st.OutputBytes != 5005 || st.OutputSHA256 != hex.EncodeToString(sum[:]) || !strings.HasSuffix(st.Tail, "END\n") {
		t.Fatalf("log %d bytes, status %+v", len(log), st)
	}
}

func TestSpecBounds(t *testing.T) {
	for _, bad := range []Spec{
		{ID: "", Argv: []string{"true"}},
		{ID: "../x", Argv: []string{"true"}},
		{ID: "job-a"},
		{ID: "job-a", Argv: []string{"true"}, WallClockMS: (MaxWallClock + time.Minute).Milliseconds()},
		{ID: "job-a", Argv: []string{"true"}, WallClockMS: 1000},
	} {
		if _, err := bad.Normalize(); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	if !ValidID(NewID()) {
		t.Fatal("NewID is not valid")
	}
}
