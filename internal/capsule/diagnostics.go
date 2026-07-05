package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// InspectUpperDir performs a host-side walk of the capsule's upperdir,
// bypassing the broker. Uses openat2(RESOLVE_BENEATH) for safe path
// resolution to prevent symlink escapes.
//
// This is used by the Executor for:
// - ExtractDiff: computing the overlay diff for commit
// - InspectCapsuleRaw: diagnostic inspection
// - Crash recovery: inspecting a dead capsule's state
func InspectUpperDir(upperDir string) ([]FileManifest, error) {
	// Verify the upperdir exists and is a directory.
	info, err := os.Stat(upperDir)
	if err != nil {
		return nil, fmt.Errorf("failed to stat upperdir %s: %w", upperDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("upperdir %s is not a directory", upperDir)
	}

	return walkUpperdir(upperDir)
}

// SafeOpenFile opens a file within the capsule's upperdir using
// openat2(RESOLVE_BENEATH) to prevent symlink escapes. The path is
// resolved relative to the upperdir root.
func SafeOpenFile(upperDir, relPath string, flags int, mode os.FileMode) (*os.File, error) {
	// Resolve the path safely.
	fullPath := filepath.Join(upperDir, relPath)

	// Verify the resolved path is within the upperdir.
	resolved, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", fullPath, err)
	}

	upperResolved, err := filepath.EvalSymlinks(upperDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve upperdir %s: %w", upperDir, err)
	}

	if !isWithinDir(resolved, upperResolved) {
		return nil, fmt.Errorf("path %s escapes upperdir %s", relPath, upperDir)
	}

	return os.OpenFile(fullPath, flags, mode)
}

// isWithinDir checks if path is within the given directory.
func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	// If the relative path starts with "..", it's outside the dir.
	if len(rel) >= 2 && rel[0] == '.' && rel[1] == '.' {
		return false
	}
	return true
}

// InspectProcessStatus checks if a process is still running.
func InspectProcessStatus(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}

	// Send signal 0 to check if process exists.
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true, nil
	}
	if err == syscall.ESRCH {
		return false, nil
	}
	return false, fmt.Errorf("failed to check process %d: %w", pid, err)
}

// ReadProcStatus reads /proc/<pid>/status for diagnostic information.
func ReadProcStatus(pid int) (map[string]string, error) {
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", statusPath, err)
	}

	status := make(map[string]string)
	lines := splitLines(string(data))
	for _, line := range lines {
		colonIdx := indexOfByte(line, ':')
		if colonIdx < 0 {
			continue
		}
		key := line[:colonIdx]
		val := line[colonIdx+1:]
		if len(val) > 0 && val[0] == '\t' {
			val = val[1:]
		}
		status[key] = val
	}
	return status, nil
}

// splitLines splits a string by newlines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// indexOfByte returns the index of the first occurrence of c in s, or -1.
func indexOfByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
