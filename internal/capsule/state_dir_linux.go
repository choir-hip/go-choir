//go:build linux

package capsule

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// StateFloorBytes is the free space the capsule state volume must keep
// before a new capsule is admitted. Capsule scratch shares the data disk
// with the computer's own state, so a full disk would starve the computer,
// not only the capsule.
const StateFloorBytes = int64(4 << 30)

// BootScopedStateDir returns the capsule state directory for this boot,
// root/boot-<boot id>, and removes the directories of earlier boots.
//
// Capsule state used to live on tmpfs, which a reboot empties. On the data
// volume it survives the reboot, while the capsules that owned it do not:
// a crash restart never resumes work, and a planned restart starts fresh
// capsules. A guest-core restart within one boot keeps the same directory,
// so orphan cleanup by capsule id still finds its residue. A directory
// that still has anything mounted under it is left alone.
func BootScopedStateDir(root string) (string, error) {
	raw, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("read boot id: %w", err)
	}
	return bootScopedStateDir(root, strings.TrimSpace(string(raw)))
}

func bootScopedStateDir(root, bootID string) (string, error) {
	if bootID == "" || strings.ContainsAny(bootID, "/.") {
		return "", fmt.Errorf("boot id is invalid")
	}
	root = filepath.Clean(root)
	current := filepath.Join(root, "boot-"+bootID)
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("list capsule state root: %w", err)
	}
	mounts, err := mountPointsUnder(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.HasPrefix(name, "boot-") || name == "boot-"+bootID {
			continue
		}
		stale := filepath.Join(root, name)
		if hasMountUnder(mounts, stale) {
			continue
		}
		if err := removePrivateTree(stale); err != nil {
			return "", fmt.Errorf("remove capsule state of an earlier boot: %w", err)
		}
	}
	if err := os.MkdirAll(current, 0o700); err != nil {
		return "", fmt.Errorf("create capsule state directory: %w", err)
	}
	if err := os.Chmod(current, 0o700); err != nil {
		return "", fmt.Errorf("secure capsule state directory: %w", err)
	}
	return current, nil
}

func mountPointsUnder(root string) ([]string, error) {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("read mount table: %w", err)
	}
	defer file.Close()
	var out []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		point := unescapeMountPath(fields[4])
		if point == root || strings.HasPrefix(point, root+"/") {
			out = append(out, point)
		}
	}
	return out, scanner.Err()
}

func hasMountUnder(mounts []string, dir string) bool {
	for _, point := range mounts {
		if point == dir || strings.HasPrefix(point, dir+"/") {
			return true
		}
	}
	return false
}

// unescapeMountPath reverses the octal escapes mountinfo uses for space,
// tab, newline and backslash.
func unescapeMountPath(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+3 < len(value) {
			var c byte
			valid := true
			for _, d := range value[i+1 : i+4] {
				if d < '0' || d > '7' {
					valid = false
					break
				}
				c = c*8 + byte(d-'0')
			}
			if valid {
				b.WriteByte(c)
				i += 3
				continue
			}
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

// stateFreeBytes reports the space available to the capsule state volume.
func stateFreeBytes(dir string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
