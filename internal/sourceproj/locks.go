package sourceproj

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// processStart bounds "left by a dead process": a lock older than this process
// was not taken by it, and reconcile (the only git mutator here, serialized)
// holds none of its own while it looks.
var processStart = time.Now()

// clearStaleLocks removes git lock files a dead process left in the projection's
// own paths: index.lock, HEAD.lock and the ref locks under refs/choir,
// refs/heads/choir. A lock is stale only when it predates this process and no
// live git process has this repository open (checked through /proc where it
// exists; where it does not, the age test stands alone and the caller's
// serialization is the rest of the proof). It never removes a lock of this
// process, and never one it cannot prove old. Removed paths are returned.
func clearStaleLocks(repo string) ([]string, error) {
	gitDir := filepath.Join(repo, ".git")
	var candidates []string
	for _, pattern := range []string{
		filepath.Join(gitDir, "*.lock"),
		filepath.Join(gitDir, "refs", "heads", "choir", "*.lock"),
		filepath.Join(gitDir, "refs", "choir", "*.lock"),
		filepath.Join(gitDir, "refs", "choir", "applied", "*.lock"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, matches...)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	if gitRunningIn(repo) {
		return nil, nil
	}
	var cleared []string
	for _, lock := range candidates {
		info, err := os.Lstat(lock)
		if err != nil || !info.Mode().IsRegular() || !info.ModTime().Before(processStart) {
			continue
		}
		if err := os.Remove(lock); err != nil {
			return cleared, err
		}
		cleared = append(cleared, lock)
	}
	return cleared, nil
}

// gitRunningIn reports whether any live git process names repo in its command
// line or has it as its working directory. Without /proc it reports false.
func gitRunningIn(repo string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	self := os.Getpid()
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || name[0] < '0' || name[0] > '9' || name == strconv.Itoa(self) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", name, "cmdline"))
		if err != nil || len(raw) == 0 {
			continue
		}
		args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		if base := filepath.Base(args[0]); base != "git" && !strings.HasPrefix(base, "git-") {
			continue
		}
		for _, arg := range args[1:] {
			if arg == repo || strings.HasPrefix(arg, repo+"/") {
				return true
			}
		}
		if cwd, err := os.Readlink(filepath.Join("/proc", name, "cwd")); err == nil && (cwd == repo || strings.HasPrefix(cwd, repo+"/")) {
			return true
		}
	}
	return false
}
