//go:build linux

package shotsandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// N8, N9. Failure modes, decided before the code:
//
//   - the launcher's supplementary groups survive into the stage and pass to
//     the autoputer, Chromium and ffmpeg: when the stage has dropped, every
//     thread's Groups line must be empty (or only the sandbox gid);
//   - a thread that exits between the listing and the read fails the stage
//     (a race, not a defect): ENOENT is skipped; any other read error refuses.

func writeTask(t *testing.T, root, tid, groups string) {
	t.Helper()
	dir := filepath.Join(root, tid)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("Name:\tx\nUid:\t7\t7\t7\t7\nGid:\t7\t7\t7\t7\nGroups:\t%s\n", groups)
	if err := os.WriteFile(filepath.Join(dir, "status"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyThreadIDsRequiresNoSupplementaryGroupsWhenDropped(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, "1", "")
	writeTask(t, root, "2", "7")
	old := procTaskDir
	procTaskDir = root
	t.Cleanup(func() { procTaskDir = old })
	if err := verifyThreadIDs(7, 7, true); err != nil {
		t.Fatalf("empty and gid-only groups = %v", err)
	}
	writeTask(t, root, "3", "7 100")
	if err := verifyThreadIDs(7, 7, true); err == nil || !strings.Contains(err.Error(), "supplementary group 100") {
		t.Fatalf("a kept supplementary group = %v", err)
	}
	// Without a drop (the unprivileged test mode) groups are not asked about.
	if err := verifyThreadIDs(7, 7, false); err != nil {
		t.Fatalf("no drop: %v", err)
	}
	// A status with no Groups line cannot show the groups are gone.
	other := t.TempDir()
	if err := os.Mkdir(filepath.Join(other, "1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "1", "status"), []byte("Uid:\t7\t7\t7\t7\nGid:\t7\t7\t7\t7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	procTaskDir = other
	if err := verifyThreadIDs(7, 7, true); err == nil || !strings.Contains(err.Error(), "no Groups line") {
		t.Fatalf("no Groups line = %v", err)
	}
}

func TestVerifyThreadIDsSkipsAThreadThatExitedAndRefusesOtherReadErrors(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, "1", "")
	// Thread 2 was listed and is gone: its directory has no status file.
	if err := os.Mkdir(filepath.Join(root, "2"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := procTaskDir
	procTaskDir = root
	t.Cleanup(func() { procTaskDir = old })
	if err := verifyThreadIDs(7, 7, true); err != nil {
		t.Fatalf("an exited thread failed the check: %v", err)
	}
	// Any other error refuses: status is a directory, so the read fails EISDIR.
	if err := os.Mkdir(filepath.Join(root, "3"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "3", "status"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifyThreadIDs(7, 7, true); err == nil {
		t.Fatal("a read error other than ENOENT was skipped")
	}
}
