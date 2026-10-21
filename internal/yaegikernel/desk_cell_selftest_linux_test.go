//go:build linux

package yaegikernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The self-test is the staging proof of desk cell confinement, so these
// tests ask whether it can lie. Ways it could (written first):
//  1. it passes an unconfined worker (the judge checks nothing real);
//  2. it fails a correctly confined worker on a kernel without Landlock
//     (CI, Docker) and so cannot run anywhere but staging;
//  3. the cross-desk read is vacuous: the canary is missing, or the read
//     fails with ENOENT;
//  4. a self-test worker survives the call (a leaked process or tmp dir);
//  5. required mode passes on a kernel without Landlock V5.

func selfTestBase(t *testing.T, mode CellConfinementMode, extraEnv ...string) DeskCellSelfTestConfig {
	t.Helper()
	shared, err := os.MkdirTemp("", "choir-cell-selftest-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(shared) })
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(shared, "worker.test")
	copyExecutable(t, os.Args[0], bin)
	root := filepath.Join(shared, "root")
	if err := os.Mkdir(root, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	return DeskCellSelfTestConfig{
		Base: DeskSessionWorkerConfig{
			Bin:       bin,
			WorkerArg: "-test.run=TestDeskCellConfinedHelperProcess",
			Session: SessionWorkerConfig{
				AllowedPackages: DefaultSafeStdlibPackagesList(),
				ComputerID:      "host-test",
				ActivationID:    "act-selftest",
				Epoch:           1,
				AllowedRoot:     root,
			},
			ProcessGroup: true,
			ExtraEnv:     append([]string{"CHOIR_DESK_CELL_CHILD=1"}, extraEnv...),
			Confinement:  CellConfinement{Mode: mode},
		},
	}
}

// haveCellUsers is true when the manifest gives every self-test desk a cell
// identity; the identity is numeric, so no user has to exist on the host.
func haveCellUsers() bool {
	for _, role := range DeskCellSelfTestRoles {
		if _, ok := DeskCellIdentity(role); !ok {
			return false
		}
	}
	return true
}

func runSelfTest(t *testing.T, cfg DeskCellSelfTestConfig) DeskCellSelfTestReceipt {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rec := DeskCellSelfTest(ctx, cfg)
	// Every self-test worker is closed and reaped, and its tmp dir removed.
	for _, w := range rec.Workers {
		if w.PID > 0 {
			if _, err := os.Stat("/proc/" + strconv.Itoa(w.PID)); !os.IsNotExist(err) {
				t.Errorf("self-test worker %s (pid %d) not reaped after the call", w.Role, w.PID)
			}
		}
		for _, target := range w.ReadTargets {
			if strings.Contains(target, deskCellSelfTestCanary) {
				if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
					t.Errorf("a desk's self-test tmp dir %s outlived the call", filepath.Dir(target))
				}
			}
		}
	}
	return rec
}

// TestDeskCellSelfTestPassesTheProductionShape: a root spawner, the three
// desk users, the real spawn path. On a kernel without Landlock the
// Landlock-only probes are reported, not judged; the cross-desk read is
// still refused (another uid's 0700 tmp dir).
func TestDeskCellSelfTestPassesTheProductionShape(t *testing.T) {
	if os.Geteuid() != 0 || !haveCellUsers() {
		t.Skip("needs a root spawner and the choir-cell-* users")
	}
	rec := runSelfTest(t, selfTestBase(t, CellConfinementAuto))
	// A sample receipt for checking the staging script's judge
	// (scripts/desk_cell_confinement_probe.py) against real output.
	if out := os.Getenv("CHOIR_DESK_CELL_SELFTEST_RECEIPT"); out != "" {
		raw, _ := json.MarshalIndent(rec, "", "  ")
		if err := os.WriteFile(out, raw, 0o644); err != nil {
			t.Fatalf("write sample receipt: %v", err)
		}
	}
	if !rec.OK {
		t.Fatalf("self-test failed a confined worker: %v\n%+v", rec.Failures, rec)
	}
	if len(rec.Workers) != 3 {
		t.Fatalf("workers = %d, want 3", len(rec.Workers))
	}
	canaryReads := 0
	for _, w := range rec.Workers {
		id, _ := DeskCellIdentity(w.Role)
		want := strconv.FormatUint(uint64(id), 10)
		if f := strings.Fields(w.Status["Uid"]); w.UserUID != want || len(f) != 4 || f[0] != want {
			t.Errorf("%s: Uid %q, want %s", w.Role, w.Status["Uid"], want)
		}
		if got := w.Probes["socket_inet_stream"]; !strings.HasPrefix(got, "EPERM") {
			t.Errorf("%s: socket_inet_stream = %q, want EPERM", w.Role, got)
		}
		for _, target := range w.ReadTargets {
			if strings.Contains(target, deskCellSelfTestCanary) {
				canaryReads++
				if got := w.Probes["read:"+target]; !strings.HasPrefix(got, "EACCES") {
					t.Errorf("%s: read of another desk's canary %s = %q, want EACCES", w.Role, target, got)
				}
			}
		}
	}
	if canaryReads != 2 {
		t.Fatalf("cross-desk reads = %d, want 2 (texture reads management's, research reads texture's)", canaryReads)
	}
}

// TestDeskCellSelfTestRefusesAnUnconfinedWorker is the mutation check: the
// same self-test against a worker that skips its confinement must fail.
func TestDeskCellSelfTestRefusesAnUnconfinedWorker(t *testing.T) {
	rec := runSelfTest(t, selfTestBase(t, CellConfinementAuto, "CHOIR_DESK_CELL_SKIP_CONFINE=1"))
	if rec.OK {
		t.Fatalf("self-test passed an unconfined worker: %+v", rec)
	}
	joined := strings.Join(rec.Failures, "\n")
	for _, want := range []string{"Seccomp=", "socket_inet_stream", "fork_exec"} {
		if !strings.Contains(joined, want) {
			t.Errorf("failures do not name %s:\n%s", want, joined)
		}
	}
}

// TestDeskCellSelfTestWithoutRoot: a dev or CI spawner cannot switch
// identity; the worker still loads no_new_privs and seccomp, and the judge
// asks only for that (plus Landlock where the kernel has it).
func TestDeskCellSelfTestWithoutRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("covers a non-root spawner")
	}
	rec := runSelfTest(t, selfTestBase(t, CellConfinementAuto))
	if !rec.OK {
		t.Fatalf("self-test failed a non-root confined worker: %v", rec.Failures)
	}
}

// TestDeskCellSelfTestJudgeRequiredModeNeedsLandlock: required mode on a
// kernel without Landlock V5 is a failure even if every probe looks right.
func TestDeskCellSelfTestJudgeRequiredModeNeedsLandlock(t *testing.T) {
	rec := DeskCellSelfTestReceipt{Mode: string(CellConfinementRequired), SpawnerUID: 0, LandlockABI: 4}
	fails := strings.Join(JudgeDeskCellSelfTest(rec), "\n")
	if !strings.Contains(fails, "Landlock ABI 4") {
		t.Fatalf("judge accepted required mode on Landlock ABI 4: %s", fails)
	}
}

// TestDeskCellSelfTestJudgeRefusesENOENT: a read that failed because the
// path is missing proves nothing.
func TestDeskCellSelfTestJudgeRefusesENOENT(t *testing.T) {
	probes := DeskCellSelfTestProbes{"write_own_tmp": "", "list_allowed_root": ""}
	for _, n := range deskCellDeniedProbes {
		probes[n] = "EPERM: operation not permitted"
	}
	probes["read:/etc/passwd"] = "ENOENT: no such file or directory"
	rec := DeskCellSelfTestReceipt{SpawnerUID: 1000, LandlockABI: 0, Workers: []DeskCellSelfTestWorker{{
		Role: "research", Status: map[string]string{"NoNewPrivs": "1", "Seccomp": "2"},
		Probes: probes, ReadTargets: []string{"/etc/passwd"}, CellServes: true,
	}}}
	fails := strings.Join(JudgeDeskCellSelfTest(rec), "\n")
	if !strings.Contains(fails, "proves nothing") {
		t.Fatalf("judge accepted an ENOENT read: %q", fails)
	}
}
