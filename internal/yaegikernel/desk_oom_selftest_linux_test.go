//go:build linux

package yaegikernel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	// The test spawns in auto mode (a container has no Landlock); the judge
	// requires the guest's required mode, so that is the one failure allowed.
	if len(rec.Failures) != 1 || !strings.Contains(rec.Failures[0], "mode") {
		t.Fatalf("OOM leg failed: %v\n%+v", rec.Failures, rec)
	}
	if !rec.Worker.CellStarted {
		t.Fatal("the start marker did not reach the host")
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

// Review bunch 22 item 4: a runaway cell that times out is reaped by Eval,
// which releases the worker's cgroup (w.budget = nil); the leg reads the
// cgroup by the path it saved before Eval and does not panic.
func TestDeskOOMRunawayTimeoutDoesNotPanic(t *testing.T) {
	t.Setenv(DeskCgroupRootEnv, "")
	env := newConfinedTestEnv(t, "research", CellConfinementAuto)
	w, err := SpawnDeskSessionWorker(env.cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "memory.events"), []byte("oom 0\noom_kill 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.budget = &workerCgroup{path: fake, maxBytes: 768 << 20}
	var out DeskOOMWorker
	// A busy loop, not an allocation: there is no real cgroup here.
	runDeskOOMRunaway(context.Background(), w, "for {}", 300*time.Millisecond, func() bool { return false }, &out)
	if !out.CellSent || !out.Dead || !strings.Contains(out.Result, "transport") {
		t.Fatalf("timeout not recorded: %+v", out)
	}
	if out.Cgroup.Path != fake || out.Cgroup.Events == nil {
		t.Fatalf("cgroup not read by its saved path: %+v", out.Cgroup)
	}
}

// Review bunch 22 item 5: the fixed cell's first act reaches the host as a
// frame the self-test's own egress hook records, before any allocation.
func TestDeskOOMStartMarkerReachesTheHost(t *testing.T) {
	t.Setenv(DeskCgroupRootEnv, "")
	env := newConfinedTestEnv(t, "research", CellConfinementAuto)
	var started atomic.Bool
	env.cfg.Egress = deskOOMStartMarkerEgress(&started)
	w, err := SpawnDeskSessionWorker(env.cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := w.Eval(ctx, deskOOMStartMarker+"\nprint(\"after\")")
	if err != nil || res.Error != "" {
		t.Fatalf("marker cell: %v %q", err, res.Error)
	}
	if !started.Load() {
		t.Fatal("the start marker did not reach the host's egress hook")
	}
	if !strings.HasPrefix(deskOOMSelfTestCell, deskOOMStartMarker) {
		t.Fatal("the runaway cell does not begin with the start marker")
	}
}
