//go:build linux

package capsule

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// docs/problems/engineering-completion-path-cluster-2026-10-20.md, failure 3
// located: memory.reclaim answers EAGAIN on a partial reclaim, and
// os.WriteFile opened the pollable cgroup file through the runtime poller,
// which turns EAGAIN into a wait for writability that never comes. A full
// pipe reopened through /proc is the same shape: pollable, and EAGAIN on
// write. writeCgroupFile must return the EAGAIN, not park.
func TestCgroupFileWriteReturnsEAGAINInsteadOfParking(t *testing.T) {
	var fds [2]int
	if err := unix.Pipe2(fds[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fds[0])
	defer unix.Close(fds[1])
	chunk := make([]byte, 4096)
	for {
		if _, err := unix.Write(fds[1], chunk); err != nil {
			if !errors.Is(err, unix.EAGAIN) {
				t.Fatalf("filling the pipe: %v", err)
			}
			break
		}
	}
	path := fmt.Sprintf("/proc/self/fd/%d", fds[1])
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no /proc/self/fd: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- writeCgroupFile(path, "1") }()
	select {
	case err := <-done:
		if !errors.Is(err, unix.EAGAIN) {
			t.Fatalf("write to a full pollable file = %v, want EAGAIN", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writeCgroupFile parked on EAGAIN, as os.WriteFile does")
	}
}
