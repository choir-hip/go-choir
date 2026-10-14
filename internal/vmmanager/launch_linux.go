//go:build linux

package vmmanager

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// startWithThreadPolicy starts cmd from a dedicated OS thread that first
// opts in to speculation barriers and cuts its capability bounding set. Both
// are per-thread state that a forked child inherits and keeps across exec.
// The thread is never unlocked, so Go destroys it when the goroutine returns:
// the reduced bounding set and the forced mitigations never reach another
// vmctl goroutine (the runtime creates new threads from its template thread
// while the caller is locked).
func startWithThreadPolicy(cmd *exec.Cmd, p threadPolicy) (LaunchThreadReport, error) {
	type result struct {
		report LaunchThreadReport
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		// Deliberately no UnlockOSThread: see above.
		report := LaunchThreadReport{Supported: true}
		// No new privileges for the VMM whatever vmctl's unit says: it can
		// never gain privilege by exec. setuid(2) (the jailer's drop) is not
		// affected.
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			ch <- result{report, fmt.Errorf("set no_new_privs: %w", err)}
			return
		}
		report.NoNewPrivs = true
		if p.speculation {
			applySpeculationBarriers(&report)
		}
		if p.keepCaps != nil {
			if err := dropBoundingSet(&report, p.keepCaps, p.tolerateCaps); err != nil {
				ch <- result{report, err}
				return
			}
		}
		ch <- result{report, cmd.Start()}
	}()
	r := <-ch
	return r.report, r.err
}

// applySpeculationBarriers forces indirect-branch (IBPB/STIBP) and
// speculative-store-bypass (SSBD) mitigations on for the thread and every
// process it starts; "force" means the child cannot turn them off. A CPU or
// kernel that does not offer a control is recorded, not fatal.
func applySpeculationBarriers(report *LaunchThreadReport) {
	for _, ctrl := range []struct {
		name  string
		which uintptr
	}{
		{"indirect_branch", unix.PR_SPEC_INDIRECT_BRANCH},
		{"store_bypass", unix.PR_SPEC_STORE_BYPASS},
	} {
		if err := unix.Prctl(unix.PR_SET_SPECULATION_CTRL, ctrl.which, unix.PR_SPEC_FORCE_DISABLE, 0, 0); err != nil {
			report.Speculation = append(report.Speculation, ctrl.name+"=error:"+err.Error())
			report.Errors = append(report.Errors, fmt.Sprintf("speculation %s: %v", ctrl.name, err))
			continue
		}
		report.Speculation = append(report.Speculation, ctrl.name+"=force_disabled")
	}
}

// dropBoundingSet removes every capability not in keep from the thread's
// bounding set, so a root child holds at most keep after exec. It needs
// CAP_SETPCAP; a non-root vmctl skips it (its children hold no capability).
func dropBoundingSet(report *LaunchThreadReport, keep, tolerate []int) error {
	if os.Geteuid() != 0 {
		report.BoundingSkipped = "vmctl is not root"
		return nil
	}
	keepSet := map[int]bool{}
	for _, c := range keep {
		keepSet[c] = true
	}
	tolerateSet := map[int]bool{}
	for _, c := range tolerate {
		tolerateSet[c] = true
	}
	last := capLastCap()
	for c := 0; c <= last; c++ {
		in, err := unix.PrctlRetInt(unix.PR_CAPBSET_READ, uintptr(c), 0, 0, 0)
		if err != nil || in == 0 {
			continue
		}
		if keepSet[c] {
			report.BoundingKept = append(report.BoundingKept, capName(c))
			continue
		}
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(c), 0, 0, 0); err != nil {
			if tolerateSet[c] {
				report.Errors = append(report.Errors, fmt.Sprintf("drop %s: %v", capName(c), err))
				continue
			}
			return fmt.Errorf("drop %s from the VMM's bounding set: %w", capName(c), err)
		}
		report.BoundingDropped++
	}
	return nil
}

func capLastCap() int {
	data, err := os.ReadFile("/proc/sys/kernel/cap_last_cap")
	if err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && n > 0 && n < 64 {
			return n
		}
	}
	return unix.CAP_LAST_CAP
}

// watchProcessExit returns a channel closed when pid exits. It holds a pidfd,
// so it works for a process vmctl did not start (the jailer's child) and can
// never confuse it with a later process that reuses the pid.
func watchProcessExit(pid int) (<-chan struct{}, error) {
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return nil, err
	}
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		defer unix.Close(fd)
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		for {
			_, err := unix.Poll(fds, -1)
			if err == unix.EINTR {
				continue
			}
			return
		}
	}()
	return ch, nil
}

// cloneFile shares src's extents with dst (FICLONE); btrfs allows it across
// mounts of one filesystem.
func cloneFile(dst, src *os.File) error {
	return unix.IoctlFileClone(int(dst.Fd()), int(src.Fd()))
}

// dialUnixSocket connects to a unix socket whose path may exceed sun_path:
// it opens the directory and connects through /proc/self/fd.
func dialUnixSocket(ctx context.Context, path string) (net.Conn, error) {
	var d net.Dialer
	if len(path) <= maxUnixSocketPathBytes {
		return d.DialContext(ctx, "unix", path)
	}
	dirfd, err := unix.Open(filepath.Dir(path), unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dirfd)
	return d.DialContext(ctx, "unix", fmt.Sprintf("/proc/self/fd/%d/%s", dirfd, filepath.Base(path)))
}
