//go:build linux

package shotsandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// procTaskDir lists this process's threads; a test points it at a fake.
var procTaskDir = "/proc/self/task"

// verifyThreadIDs refuses unless every thread of this process has all four
// of its uids (real, effective, saved, filesystem) and all four gids equal to
// uid and gid. Whether syscall.Setuid reaches every thread depends on the
// build (all threads since Go 1.16, by AllThreadsSyscall or, with cgo, by
// libc's setxid); the stage does not rely on that: it reads the answer from
// the kernel and runs nothing unless it is right (D-B1).
func verifyThreadIDs(uid, gid int) error {
	tasks, err := os.ReadDir(procTaskDir)
	if err != nil {
		return fmt.Errorf("read threads: %w", err)
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no threads listed in %s", procTaskDir)
	}
	for _, task := range tasks {
		raw, err := os.ReadFile(filepath.Join(procTaskDir, task.Name(), "status"))
		if err != nil {
			return fmt.Errorf("thread %s: %w", task.Name(), err)
		}
		var sawUID, sawGID bool
		for _, line := range strings.Split(string(raw), "\n") {
			key, rest, ok := strings.Cut(line, ":")
			if !ok || (key != "Uid" && key != "Gid") {
				continue
			}
			want, ids := uid, strings.Fields(rest)
			if key == "Gid" {
				want = gid
			}
			if len(ids) != 4 {
				return fmt.Errorf("thread %s: %s line %q is not four ids", task.Name(), key, line)
			}
			for _, id := range ids {
				if id != fmt.Sprint(want) {
					return fmt.Errorf("thread %s still has %s %s, want %d", task.Name(), key, strings.Join(ids, " "), want)
				}
			}
			if key == "Uid" {
				sawUID = true
			} else {
				sawGID = true
			}
		}
		if !sawUID || !sawGID {
			return fmt.Errorf("thread %s: status has no Uid or Gid line", task.Name())
		}
	}
	return nil
}
