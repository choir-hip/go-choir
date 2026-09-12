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
		if err := NewWorkloadLandlock(os.Getenv(landlockAllowedEnv)).Apply(); err != nil {
			t.Fatalf("apply workload Landlock policy: %v", err)
		}
		if _, err := os.ReadFile("/etc/hostname"); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("outside-path read error = %v, want permission denied", err)
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
