//go:build linux

package capsule

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
)

// A dead capsule broker looked hung for 46 minutes
// (docs/problems/engineering-cell-never-returns-2026-11-03.md): the host
// holds the broker's listener, so a dial after the broker died lands in the
// backlog and the read times out, and nobody read the broker's exit status
// after readiness. Failure modes written before the code:
//   - the exit is not logged, or the log lacks the status, the signal or the
//     broker's own last lines;
//   - a broker call after the exit waits for its read bound instead of
//     failing at once;
//   - a call in flight when the broker dies waits for its bound;
//   - the error does not say the broker exited or does not carry the status;
//   - the host's listener copy stays open (dials keep landing in a backlog);
//   - a running job never ends, so the job watcher, the host and the model
//     never learn it ended;
//   - a broker stopped by Destroy (or a failed spawn's cleanup) is reported
//     as a death, or marks the capsule's calls failed;
//   - the broker's stderr stops reaching the journal.

// deadBrokerCapsule is an active capsule whose "broker" is cmd: its stderr
// goes to the capsule's broker log, the host holds a listener nobody
// accepts on (as for a dead broker), and the broker client is connected to
// it.
func deadBrokerCapsule(t *testing.T, script string) (*Executor, *Capsule, *BrokerClient) {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "broker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	client := connectedBrokerClient(t, socket)
	caps := &Capsule{ID: "capsule-brokerexit", State: StateActive, listener: listener, broker: client}
	e := &Executor{capsules: map[string]*Capsule{caps.ID: caps}, jobs: map[string]*capsuleJob{}}
	cmd := exec.Command("sh", "-c", script)
	logTail := e.brokerLogs.register(caps.ID)
	pipe, err := attachBrokerLog(cmd, logTail, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := caps.startBrokerProcess(context.Background(), 5*time.Second, cmd); err != nil {
		pipe.abort()
		t.Fatal(err)
	}
	pipe.started()
	return e, caps, client
}

func waitBrokerWatch(t *testing.T, e *Executor, caps *Capsule) {
	t.Helper()
	done := make(chan struct{})
	go func() { e.watchBrokerExit(caps); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the broker watch did not return after the broker exited")
	}
}

func TestBrokerExitFailsCallsAtOnceAndClosesTheListener(t *testing.T) {
	shortBrokerBounds(t)
	brokerJobAnswerBound = 30 * time.Second
	e, caps, client := deadBrokerCapsule(t, `echo "runtime: failed to create new OS thread (have 255 already; errno=11)" >&2; echo "capsule broker launcher: exit status 2" >&2; exit 1`)
	waitBrokerWatch(t, e, caps)

	start := time.Now()
	_, err := client.Job(context.Background(), "job_status", nil, JobParams{ID: "job-d807dd9c4be9"})
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("a call after the broker exited waited %s", elapsed)
	}
	if !errors.Is(err, ErrBrokerExited) || !strings.Contains(err.Error(), "the capsule broker exited (exit status 1") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "capsule broker launcher: exit status 2") {
		t.Fatalf("the error lacks the broker's last line: %v", err)
	}
	if caps.listener != nil {
		t.Fatal("the host's listener copy is still held")
	}
	death := caps.brokerDeath()
	if !strings.HasPrefix(death, "exit status 1") {
		t.Fatalf("broker death %q", death)
	}
	logs := e.BrokerLogs()
	if len(logs) != 1 || !strings.HasPrefix(logs[0].Exit, "exit status 1") || len(logs[0].FirstFatal) == 0 ||
		!strings.Contains(logs[0].FirstFatal[0], "failed to create new OS thread") || len(logs[0].Lines) != 2 {
		t.Fatalf("broker logs %+v", logs)
	}
}

func TestBrokerExitEndsACallInFlight(t *testing.T) {
	shortBrokerBounds(t)
	brokerJobAnswerBound = 30 * time.Second
	e, caps, client := deadBrokerCapsule(t, `sleep 0.3; kill -9 $$`)
	go e.watchBrokerExit(caps)
	start := time.Now()
	_, err := client.Job(context.Background(), "job_status", nil, JobParams{ID: "job-x"})
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("a call in flight waited %s after the broker died", elapsed)
	}
	if !errors.Is(err, ErrBrokerExited) || !strings.Contains(err.Error(), "signal") {
		t.Fatalf("err = %v", err)
	}
}

func TestBrokerStoppedByDestroyIsNotADeath(t *testing.T) {
	shortBrokerBounds(t)
	e, caps, client := deadBrokerCapsule(t, `exit 0`)
	caps.stopping.Store(true)
	waitBrokerWatch(t, e, caps)
	if death := caps.brokerDeath(); death != "" {
		t.Fatalf("a stopped broker was recorded as dead: %q", death)
	}
	_, err := client.Job(context.Background(), "job_status", nil, JobParams{ID: "job-x"})
	if errors.Is(err, ErrBrokerExited) {
		t.Fatalf("a stopped broker marked the client: %v", err)
	}
}

func TestRunningJobEndsWhenItsBrokerDied(t *testing.T) {
	f := newJobFixture(t, capsulejob.StateRunning, 0, "")
	f.job.status.ExitCode = nil
	f.caps.recordBrokerDeath("killed by signal 9 at 22:31:05Z")
	if !f.executor.pollJob(f.job) {
		t.Fatal("the job did not end with its broker")
	}
	if len(f.got) != 1 || f.got[0].View.State != capsulejob.StateFailed || f.job.status.State != capsulejob.StateFailed ||
		!strings.Contains(f.job.status.Reason, "the capsule broker exited (killed by signal 9 at 22:31:05Z)") {
		t.Fatalf("job %+v, completions %d", f.job.status, len(f.got))
	}
}

// Linux sends Pdeathsig when the parent THREAD exits, not the process
// (golang/go#27505). The host starts the broker launcher with Pdeathsig
// SIGKILL, and the autoputer ends OS threads on purpose (guestmem reclaim,
// tapestore compaction, capfloor: LockOSThread, never unlocked). A
// goroutine that locks the thread which forked the launcher and then ends
// kills the launcher, the broker (its own Pdeathsig) and the whole capsule
// PID namespace, with no OOM, no log line and the listener still held.
// internal/childthread holds the hazard's regression test (a child forked
// from a thread that ends is killed; one started through childthread is
// not). Here: the broker launcher goes through it, so a launcher outlives
// 500 ended host threads, and a start error records no process.

func TestBrokerLauncherSurvivesHostThreadExits(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	caps := &Capsule{ID: "capsule-pinned"}
	if err := caps.startBrokerProcess(context.Background(), 5*time.Second, cmd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = caps.Process.Kill(); _ = caps.wait() })
	if caps.PID <= 0 || caps.Process == nil {
		t.Fatalf("process not recorded: %+v", caps)
	}
	// End many OS threads, as guestmem reclaim does once a minute.
	for i := 0; i < 500; i++ {
		done := make(chan struct{})
		go func() { runtime.LockOSThread(); close(done) }()
		<-done
	}
	select {
	case <-caps.processDone:
		t.Fatalf("the launcher died when host threads ended: %v", caps.processErr)
	case <-time.After(500 * time.Millisecond):
	}
	_ = caps.Process.Kill()
	select {
	case <-caps.processDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the exit was not reaped")
	}
}

func TestBrokerProcessStartErrorIsReturned(t *testing.T) {
	caps := &Capsule{ID: "capsule-nostart"}
	err := caps.startBrokerProcess(context.Background(), 5*time.Second, exec.Command("/nonexistent/broker"))
	if err == nil || caps.Process != nil {
		t.Fatalf("err %v, process %v", err, caps.Process)
	}
}
