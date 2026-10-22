//go:build linux

package yaegikernel

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReadDaemonIdentityIsThisProcess(t *testing.T) {
	a, b := ReadDaemonIdentity(), ReadDaemonIdentity()
	if a.Error != "" || a.PID != os.Getpid() || a.StartTicks == 0 || a.BootID == "" {
		t.Fatalf("identity = %+v", a)
	}
	if a != b {
		t.Fatalf("identity not stable: %+v then %+v", a, b)
	}
}

// The leg end to end on a real cgroup v2 hierarchy (root; the guest and a
// privileged container have it): the worker is killed by its cgroup, the
// sibling answers, this process survives, and nothing is left behind.
func TestDeskOOMSelfTestKillsTheWorkerAtItsBudget(t *testing.T) {
	root := deskCgroupTestRoot(t)
	env := newConfinedTestEnv(t, "research_child", CellConfinementAuto)
	env.cfg.CgroupRoot = root
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// research_child (384 MiB) keeps the test light; the deployed leg uses
	// research (768 MiB).
	rec := DeskOOMSelfTest(ctx, DeskOOMSelfTestConfig{Base: env.cfg, Role: "research_child"})
	if !rec.OK {
		t.Fatalf("OOM leg failed: %v\n%+v", rec.Failures, rec)
	}
	if rec.Worker.OOMKill < 1 || rec.Worker.Cgroup.MemoryMax != fmt.Sprint(384<<20) {
		t.Fatalf("worker = %+v", rec.Worker)
	}
	if !rec.Sibling.AfterAnswered || rec.DaemonAfter.PID != os.Getpid() {
		t.Fatalf("sibling or daemon affected: %+v %+v", rec.Sibling, rec.DaemonAfter)
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), workerCgroupPrefix) {
			t.Errorf("cgroup %s left behind", e.Name())
		}
	}
}
