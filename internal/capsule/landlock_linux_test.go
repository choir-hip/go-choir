//go:build linux

package capsule

import (
	"errors"
	"os"
	"os/exec"
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
