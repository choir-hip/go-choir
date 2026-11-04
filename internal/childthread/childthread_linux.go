//go:build linux

// Package childthread starts a child whose Pdeathsig must mean "the parent
// process died", not "the thread that forked it ended".
//
// Linux sends a child its Pdeathsig when the parent THREAD that forked it
// exits (prctl(2), PR_SET_PDEATHSIG; golang/go#27505). Go ends an OS thread
// when a goroutine that called runtime.LockOSThread returns without
// unlocking, and the guest does that on purpose (guestmem reclaim,
// tapestore compaction, capfloor). If such a goroutine lands on the thread
// that forked a Pdeathsig child, the child is SIGKILLed: on 2026-11-03 that
// killed two engineering capsules' broker launchers, and with them every
// process of the capsule (docs/problems/engineering-cell-never-returns-2026-11-03.md).
//
// Start forks the child from a goroutine locked to its OS thread and keeps
// that goroutine (so the thread) until the child exits. It waits with
// waitid(WNOWAIT), which does not reap: the caller's cmd.Wait works as
// before and keeps the exit status.
package childthread

import (
	"errors"
	"os/exec"
	"runtime"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

var held atomic.Int64

// holding is how many threads are held for live children (tests).
func holding() int64 { return held.Load() }

// Start is cmd.Start on a thread that lives as long as the child.
func Start(cmd *exec.Cmd) error {
	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		held.Add(1)
		release := func() {
			held.Add(-1)
			runtime.UnlockOSThread()
		}
		if err := cmd.Start(); err != nil {
			release()
			started <- err
			return
		}
		defer release()
		pid := cmd.Process.Pid
		started <- nil
		// Block until the child exits, without reaping it. ECHILD: the
		// caller's Wait reaped it first; EINTR: retry.
		for {
			var info unix.Siginfo
			err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
			if !errors.Is(err, unix.EINTR) {
				return
			}
		}
	}()
	return <-started
}

// Run is cmd.Run with the start on a held thread.
func Run(cmd *exec.Cmd) error {
	if err := Start(cmd); err != nil {
		return err
	}
	return cmd.Wait()
}
