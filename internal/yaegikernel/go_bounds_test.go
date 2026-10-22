package yaegikernel

import (
	"strings"
	"testing"
)

// A capsule's Go workloads get its CPU quota as GOMAXPROCS and a memory bound
// as GOMEMLIMIT
// (docs/problems/capsule-release-build-runs-at-the-2gib-ceiling-2026-10-22.md).
// Ways it could fail, written before the code:
//
//	G1. a hop (executor, broker, session worker, cell, job, direct exec)
//	    drops the bound, and go build runs -p 4 inside one CPU again;
//	G2. a process outside a capsule (a desk session) gains a bound it never
//	    had;
//	G3. a malformed carried value reaches the workload: Go refuses to start
//	    on a malformed GOMEMLIMIT, and GOMAXPROCS=0 or a junk value is
//	    ignored or worse;
//	G4. a cell that needs more cannot override the bound;
//	G5. the bound is tied to the build path, so a capsule without the C
//	    toolchain runs unbounded;
//	G6. the carry variables reach the workload under their CHOIR_ names, or
//	    a payload sets the carry;
//	G7. a tiny GOMEMLIMIT (a few bytes or KiB) reaches the workload and its
//	    GC runs without pause;
//	G8. the carry is read from the autoputer's own environment at the
//	    broker launch, so an ambient CHOIR_CAPSULE_GO* on the guest service
//	    bounds every capsule, or a capsule the executor set no bound for.
func boundsEnv() map[string]string {
	env := guestEnv()
	env[CapsuleGoMaxProcsEnv] = "1"
	env[CapsuleGoMemLimitEnv] = "1342177280"
	return env
}

func TestWorkloadEnvGetsCapsuleGoBounds(t *testing.T) {
	const base = "/run/current-system/sw/bin:/bin:/usr/bin"
	_, extra := WorkloadToolchainEnv(base, lookupFrom(boundsEnv()))
	joined := "\n" + strings.Join(extra, "\n") + "\n"
	for _, want := range []string{"\nGOMAXPROCS=1\n", "\nGOMEMLIMIT=1342177280\n", "\nCGO_ENABLED=1\n"} {
		if !strings.Contains(joined, want) {
			t.Errorf("workload env missing %q in %q", strings.TrimSpace(want), extra)
		}
	}
	// G6: the carry names stay out of the workload.
	for _, absent := range []string{CapsuleGoMaxProcsEnv, CapsuleGoMemLimitEnv} {
		if strings.Contains(joined, absent) {
			t.Errorf("workload env exposes %s: %q", absent, extra)
		}
	}

	// G5: no build path, the bounds still apply (and no toolchain does).
	path, extra := WorkloadToolchainEnv(base, lookupFrom(map[string]string{CapsuleGoMaxProcsEnv: "2", CapsuleGoMemLimitEnv: "2013265920"}))
	if path != base || strings.Join(extra, ",") != "GOMAXPROCS=2,GOMEMLIMIT=2013265920" {
		t.Fatalf("bounds without a build path: PATH = %q, extra = %q", path, extra)
	}

	// G2: no carry, no bound.
	if _, extra := WorkloadToolchainEnv(base, lookupFrom(guestEnv())); strings.Contains(strings.Join(extra, "\n"), "GOMAXPROCS") || strings.Contains(strings.Join(extra, "\n"), "GOMEMLIMIT") {
		t.Fatalf("a bound appeared without a carry: %q", extra)
	}
	if _, extra := WorkloadToolchainEnv(base, lookupFrom(map[string]string{})); len(extra) != 0 {
		t.Fatalf("outside a capsule: %q", extra)
	}
}

// G3: only a positive decimal integer passes; anything else is dropped, so
// the workload runs with Go's defaults rather than refusing to start.
func TestWorkloadEnvDropsMalformedGoBounds(t *testing.T) {
	for _, bad := range []string{"", "0", "-1", "1.5", "1GiB", " 2", "2\n", "99999999999999999999", "abc", "1e9"} {
		_, extra := WorkloadToolchainEnv("/bin", lookupFrom(map[string]string{CapsuleGoMaxProcsEnv: bad, CapsuleGoMemLimitEnv: bad}))
		if len(extra) != 0 {
			t.Errorf("value %q passed: %q", bad, extra)
		}
	}
	// GOMAXPROCS above Go's own ceiling is dropped too.
	if _, extra := WorkloadToolchainEnv("/bin", lookupFrom(map[string]string{CapsuleGoMaxProcsEnv: "4096"})); len(extra) != 0 {
		t.Errorf("GOMAXPROCS 4096 passed: %q", extra)
	}
}

// G1, hop broker to session worker: the bounds carry passes the bounds,
// with or without a build path. G8: the toolchain carry, which the executor
// also reads from the autoputer's own environment, never does.
func TestCapsuleGoBoundsCarryPassesGoBounds(t *testing.T) {
	joined := "\n" + strings.Join(CapsuleGoBoundsCarry(lookupFrom(boundsEnv())), "\n") + "\n"
	for _, want := range []string{"\n" + CapsuleGoMaxProcsEnv + "=1\n", "\n" + CapsuleGoMemLimitEnv + "=1342177280\n"} {
		if !strings.Contains(joined, want) {
			t.Errorf("carry missing %q in %q", strings.TrimSpace(want), joined)
		}
	}
	only := CapsuleGoBoundsCarry(lookupFrom(map[string]string{CapsuleGoMaxProcsEnv: "1", CapsuleGoMemLimitEnv: "junk"}))
	if strings.Join(only, ",") != CapsuleGoMaxProcsEnv+"=1" {
		t.Fatalf("carry with a junk limit: %q", only)
	}
	if toolchain := strings.Join(BuildToolchainCarry(lookupFrom(boundsEnv())), "\n"); strings.Contains(toolchain, "CHOIR_CAPSULE_GO") {
		t.Fatalf("the toolchain carry passed the bounds: %q", toolchain)
	}
}

// G7: a carried GOMEMLIMIT under the 64 MiB floor is dropped.
func TestWorkloadEnvDropsGoMemLimitUnderTheFloor(t *testing.T) {
	for _, v := range []string{"1", "4096", "67108863"} {
		if _, extra := WorkloadToolchainEnv("/bin", lookupFrom(map[string]string{CapsuleGoMemLimitEnv: v})); len(extra) != 0 {
			t.Errorf("GOMEMLIMIT %s passed: %q", v, extra)
		}
	}
	if _, extra := WorkloadToolchainEnv("/bin", lookupFrom(map[string]string{CapsuleGoMemLimitEnv: "67108864"})); strings.Join(extra, ",") != "GOMEMLIMIT=67108864" {
		t.Fatalf("GOMEMLIMIT at the floor: %q", extra)
	}
}

// G1 (hop to a cell) and G4: a cell's choir.Exec gets the bounds, and its
// payload overrides them (the payload comes last); G6: a payload cannot set
// the carry.
func TestCellExecEnvBoundsAreDefaultsACellOverrides(t *testing.T) {
	env := cellExecEnv(nil, lookupFrom(boundsEnv()))
	joined := "\n" + strings.Join(env, "\n") + "\n"
	if !strings.Contains(joined, "\nGOMAXPROCS=1\n") || !strings.Contains(joined, "\nGOMEMLIMIT=1342177280\n") {
		t.Fatalf("cell env lost the bounds: %q", env)
	}
	env = cellExecEnv(map[string]string{"GOMAXPROCS": "4", "GOMEMLIMIT": "off"}, lookupFrom(boundsEnv()))
	if last(env, "GOMAXPROCS") != "4" || last(env, "GOMEMLIMIT") != "off" {
		t.Fatalf("a payload could not override the bounds: %q", env)
	}
	env = cellExecEnv(map[string]string{CapsuleGoMaxProcsEnv: "64"}, lookupFrom(map[string]string{}))
	if strings.Contains(strings.Join(env, "\n"), "GOMAXPROCS") {
		t.Fatalf("a payload set the carry: %q", env)
	}
}

// last is the value exec would use: os/exec keeps the last of duplicate keys.
func last(env []string, key string) string {
	value := ""
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			value = v
		}
	}
	return value
}
