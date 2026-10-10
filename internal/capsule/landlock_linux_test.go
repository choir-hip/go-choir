//go:build linux

package capsule

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

const (
	landlockHelperEnv  = "CHOIR_LANDLOCK_TEST_HELPER"
	landlockAllowedEnv = "CHOIR_LANDLOCK_TEST_ALLOWED"
)

func TestWorkloadLandlockAppliesMixedRulesAndDeniesOutsidePath(t *testing.T) {
	if os.Getenv(landlockHelperEnv) == "1" {
		allowed := os.Getenv(landlockAllowedEnv)
		if err := NewWorkloadLandlock(allowed).Apply(); err != nil {
			t.Fatalf("apply workload Landlock policy: %v", err)
		}
		if _, err := os.ReadFile("/etc/hostname"); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("outside-path read error = %v, want permission denied", err)
		}
		if err := os.WriteFile(allowed+"/created.txt", []byte("ok\n"), 0o644); err != nil {
			t.Fatalf("create new file inside allowed dir: %v", err)
		}
		return
	}

	allowed := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkloadLandlockAppliesMixedRulesAndDeniesOutsidePath$")
	cmd.Env = append(os.Environ(), landlockHelperEnv+"=1", landlockAllowedEnv+"="+allowed)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Landlock helper: %v\n%s", err, output)
	}
}

// capsule-workload-cannot-run-a-shell-or-install-packages-2026-10-10: uv and
// pip rename a temp directory into a different parent. Without the refer
// right, Landlock refuses that with EXDEV ("Invalid cross-device link"),
// even though both sides are inside the allowed path. Named pipes in /tmp
// need make_fifo.
func TestWorkloadLandlockAllowsReparentingRenameAndFifo(t *testing.T) {
	if os.Getenv(landlockHelperEnv) == "refer" {
		allowed := os.Getenv(landlockAllowedEnv)
		if err := NewWorkloadLandlock(allowed).Apply(); err != nil {
			t.Fatalf("apply workload Landlock policy: %v", err)
		}
		for _, dir := range []string{allowed + "/cache/.tmp1", allowed + "/cache/archive"} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", dir, err)
			}
		}
		if err := os.WriteFile(allowed+"/cache/.tmp1/wheel", []byte("ok\n"), 0o644); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
		if err := os.Rename(allowed+"/cache/.tmp1", allowed+"/cache/archive/entry"); err != nil {
			t.Fatalf("reparenting rename inside the allowed path: %v", err)
		}
		if err := syscall.Mkfifo(allowed+"/fifo", 0o600); err != nil {
			t.Fatalf("mkfifo inside the allowed path: %v", err)
		}
		return
	}

	allowed := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkloadLandlockAllowsReparentingRenameAndFifo$")
	cmd.Env = append(os.Environ(), landlockHelperEnv+"=refer", landlockAllowedEnv+"="+allowed)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Landlock helper: %v\n%s", err, output)
	}
}
