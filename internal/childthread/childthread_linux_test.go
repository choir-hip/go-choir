//go:build linux

package childthread

import (
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Linux sends Pdeathsig when the parent THREAD exits, not the process
// (golang/go#27505), and Go ends a thread when a goroutine that locked it
// returns without unlocking (guestmem reclaim, tapestore compaction and
// capfloor do so on purpose). A Pdeathsig child forked from a thread such a
// goroutine later lands on is SIGKILLed for no reason
// (docs/problems/engineering-cell-never-returns-2026-11-03.md). Failure
// modes written before the code:
//   - Start forks from a thread another goroutine can lock and end;
//   - the thread stays locked after the child exits (one leaked thread per
//     child);
//   - the thread stays locked after a failed start;
//   - Start reaps the child, so the caller's own cmd.Wait loses its status;
//   - the caller's Wait reaping first leaves the holder blocked forever.

func endThreads(n int) {
	for i := 0; i < n; i++ {
		done := make(chan struct{})
		go func() { runtime.LockOSThread(); close(done) }()
		<-done
	}
}

func sleeper() *exec.Cmd {
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return cmd
}

// The hazard itself: a child started from a thread that then exits is
// killed. Go wedges rather than ends the main thread, so one start may land
// there; of several, at least one must be killed.
func TestPdeathsigFiresWhenTheStartingThreadExits(t *testing.T) {
	const n = 8
	waited := make(chan error, n)
	var cmds []*exec.Cmd
	for i := 0; i < n; i++ {
		cmd := sleeper()
		started := make(chan error, 1)
		go func() {
			runtime.LockOSThread() // never unlocked: the thread ends with the goroutine
			started <- cmd.Start()
		}()
		if err := <-started; err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
		go func() { waited <- cmd.Wait() }()
	}
	t.Cleanup(func() {
		for _, cmd := range cmds {
			_ = cmd.Process.Kill()
		}
	})
	select {
	case err := <-waited:
		if err == nil || !strings.Contains(err.Error(), "killed") {
			t.Fatalf("a child ended with %v, want killed", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("every child outlived its starting thread; the hazard this guards is gone")
	}
}

func TestStartedChildSurvivesThreadExits(t *testing.T) {
	cmd := sleeper()
	if err := Start(cmd); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	endThreads(500)
	select {
	case err := <-waited:
		t.Fatalf("the child died when other threads ended: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-waited:
		// The caller's own Wait kept the status.
		if err == nil || !strings.Contains(err.Error(), "terminated") {
			t.Fatalf("wait = %v, want terminated", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the child was not reaped by the caller's Wait")
	}
	waitReleased(t)
}

func TestStartErrorReturnsTheThread(t *testing.T) {
	if err := Start(exec.Command("/nonexistent/child")); err == nil {
		t.Fatal("start of a missing binary succeeded")
	}
	if n := holding(); n != 0 {
		t.Fatalf("%d threads held after a failed start", n)
	}
}

func TestRunWaitsOnTheHeldThread(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 3")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	err := Run(cmd)
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 3 || err == nil {
		t.Fatalf("run: %v, state %v", err, cmd.ProcessState)
	}
	waitReleased(t)
}

// waitReleased: the holder returns its thread once the child exits.
func waitReleased(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for holding() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := holding(); n != 0 {
		t.Fatalf("%d threads still held after the child exited", n)
	}
}
