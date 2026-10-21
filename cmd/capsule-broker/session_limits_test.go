//go:build linux

package main

import (
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
)

// RES-03: the capsule's exec-go-session worker is bounded. Before, only the
// capsule's cgroup bounded it, so a runaway cell was a runaway for the whole
// capsule and the kernel chose its victim by size: the broker or a build as
// readily as the worker. Now the worker volunteers for the OOM kill, holds a
// soft Go limit below the capsule's budget, and has an address-space cap.
//
// Ways it could fail, written first: the limits are set but the worker does
// not volunteer (the kernel kills the broker); the soft limit is above the
// capsule's 2 GiB, so it never binds; the cap is applied after the first cell.

const sessionLimitsHelperEnv = "CHOIR_SESSION_LIMITS_HELPER"

func TestSessionWorkerLimitsAreApplied(t *testing.T) {
	if os.Getenv(sessionLimitsHelperEnv) == "1" {
		if err := applySessionWorkerLimits(); err != nil {
			t.Fatalf("apply: %v", err)
		}
		adj, err := os.ReadFile("/proc/self/oom_score_adj")
		if err != nil || strings.TrimSpace(string(adj)) != "1000" {
			t.Fatalf("oom_score_adj = %q (%v), want 1000: the worker must die before the broker", adj, err)
		}
		if got := debug.SetMemoryLimit(-1); got != sessionWorkerSoftLimit {
			t.Fatalf("GOMEMLIMIT = %d, want %d", got, int64(sessionWorkerSoftLimit))
		}
		limits, _ := os.ReadFile("/proc/self/limits")
		if !strings.Contains(string(limits), strconv.FormatUint(sessionWorkerAddressCap, 10)) {
			t.Fatalf("RLIMIT_AS not applied:\n%s", limits)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSessionWorkerLimitsAreApplied$")
	cmd.Env = append(os.Environ(), sessionLimitsHelperEnv+"=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper: %v\n%s", err, out)
	}
}

// The soft limit sits below the engineering capsule's budget (2 GiB), or it
// would never bind before the capsule's own cgroup does.
func TestSessionWorkerSoftLimitBelowCapsuleBudget(t *testing.T) {
	if sessionWorkerSoftLimit <= 0 || sessionWorkerSoftLimit >= 2<<30 {
		t.Fatalf("soft limit %d must be positive and below the 2 GiB capsule", int64(sessionWorkerSoftLimit))
	}
}
