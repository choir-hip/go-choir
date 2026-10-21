//go:build linux

package yaegikernel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// deskCgroupTestRoot makes a desks cgroup under the real cgroup v2 mount, or
// skips: the test needs root and a writable unified hierarchy (the guest has
// both; a privileged container does; CI usually does not).
func deskCgroupTestRoot(t *testing.T) string {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("needs root to make cgroups")
	}
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		t.Skip("no cgroup v2 unified hierarchy")
	}
	root := filepath.Join("/sys/fs/cgroup", fmt.Sprintf("choir-desks-test-%d", os.Getpid()))
	if err := EnsureDeskCgroupRoot(root, 1<<30, 768<<20); err != nil {
		t.Skipf("cannot make the desks cgroup here: %v", err)
	}
	t.Cleanup(func() { removeCgroupDir(root) })
	return root
}

// A cell that allocates without end is killed by the kernel at its worker's
// budget; the host process, a sibling worker in the same desks cgroup, and
// the next worker are untouched; the desk is told the budget did it; and no
// cgroup is left behind. This is the out-of-memory probe of the batch: the
// guest's 2026-10-16 failure was the kernel choosing the daemon.
func TestDeskCgroupKillsRunawayWorker(t *testing.T) {
	root := deskCgroupTestRoot(t)

	victimEnv := newConfinedTestEnv(t, "research_child", CellConfinementAuto)
	victimEnv.cfg.CgroupRoot = root
	bystanderEnv := newConfinedTestEnv(t, "research", CellConfinementAuto)
	bystanderEnv.cfg.CgroupRoot = root

	bystander, err := SpawnDeskSessionWorker(bystanderEnv.cfg)
	if err != nil {
		t.Fatalf("spawn bystander: %v", err)
	}
	defer bystander.Close()
	victim, err := SpawnDeskSessionWorker(victimEnv.cfg)
	if err != nil {
		t.Fatalf("spawn victim: %v", err)
	}
	// The worker started inside its budget cgroup, not beside it.
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", victim.PID()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), filepath.Base(root)+"/"+workerCgroupPrefix) {
		t.Fatalf("worker %d is not in a w-* child of the desks cgroup: %s", victim.PID(), raw)
	}
	if got := readCgroupFile(t, victim.budget.path, "memory.max"); got != fmt.Sprint(384<<20) {
		t.Fatalf("research_child memory.max = %s, want its manifest 384 MiB", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	_, evalErr := victim.Eval(ctx, `var keep [][]byte
for {
	b := make([]byte, 8<<20)
	for i := 0; i < len(b); i += 4096 {
		b[i] = 1
	}
	keep = append(keep, b)
}`)
	if evalErr == nil {
		t.Fatal("a cell that allocates without end returned normally")
	}
	if !victim.Dead() {
		t.Fatal("the runaway worker is not dead")
	}
	victim.Close()
	cause := victim.ExitCause()
	if !strings.Contains(cause, "killed by its memory budget") || !strings.Contains(cause, "384 MiB") {
		t.Fatalf("exit cause does not say the budget killed the worker: %q", cause)
	}

	// The sibling in the same desks cgroup still serves, and so does a new worker.
	res, err := bystander.Eval(ctx, `print(6*7)`)
	if err != nil || !strings.Contains(res.Stdout, "42") {
		t.Fatalf("the bystander worker was affected: %v %+v", err, res)
	}
	nextEnv := newConfinedTestEnv(t, "research_child", CellConfinementAuto)
	nextEnv.cfg.CgroupRoot = root
	next, err := SpawnDeskSessionWorker(nextEnv.cfg)
	if err != nil {
		t.Fatalf("a new worker cannot start after the kill: %v", err)
	}
	next.Close()
	bystander.Close()

	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), workerCgroupPrefix) {
			t.Errorf("cgroup %s left behind after every worker closed", e.Name())
		}
	}
}

// A worker spawned in required mode with no desks cgroup does not start.
func TestDeskCgroupRequiredModeRefusesWithoutRoot(t *testing.T) {
	t.Setenv(DeskCgroupRootEnv, "")
	env := newConfinedTestEnv(t, "research", CellConfinementRequired)
	if w, err := SpawnDeskSessionWorker(env.cfg); err == nil {
		w.Close()
		t.Fatal("required mode started a worker with no memory budget")
	}
}
