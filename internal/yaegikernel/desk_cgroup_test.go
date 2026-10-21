package yaegikernel

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// RES-01 / RES-02. The memory budget is a cgroup per worker, from the desk's
// manifest row. These tests run the cgroup logic against a directory standing
// in for the cgroup filesystem; the kernel's enforcement is proved by
// TestDeskCgroupKillsRunawayWorker (Linux, root) and on the deployed guest.
//
// Ways it could fail, written first: a worker gets no budget and the spawn
// proceeds (required mode); a role with no manifest budget gets a default; a
// desk gets another desk's number; swap is left on, so the budget is a
// suggestion; a crashed daemon's cgroups pile up; the budget kills the worker
// and the desk is told "signal: killed".

// fakeCgroupFS lets the tests use plain directories for cgroups: a real
// cgroup directory is removed by rmdir alone, a plain one holds the files the
// code wrote.
func fakeCgroupFS(t *testing.T) {
	t.Helper()
	prev := cgroupRmdir
	cgroupRmdir = os.RemoveAll
	t.Cleanup(func() { cgroupRmdir = prev })
}

func readCgroupFile(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("%s/%s: %v", dir, name, err)
	}
	return strings.TrimSpace(string(raw))
}

func TestWorkerCgroupIsTheDesksManifestBudget(t *testing.T) {
	fakeCgroupFS(t)
	for _, role := range []string{"management", "texture", "research", SessionRoleManagementTask, SessionRoleResearchChild} {
		role := role
		t.Run(role, func(t *testing.T) {
			root := t.TempDir()
			cg, err := planWorkerCgroup(DeskSessionWorkerConfig{CgroupRoot: root, Session: SessionWorkerConfig{Role: role}})
			if err != nil || cg == nil {
				t.Fatalf("plan: %v (cg %v)", err, cg)
			}
			defer cg.release()
			d, _ := deskmanifest.Lookup(role)
			if got := readCgroupFile(t, cg.path, "memory.max"); got != strconv.FormatUint(d.Memory.WorkerMaxBytes, 10) {
				t.Errorf("memory.max = %s, manifest %d", got, d.Memory.WorkerMaxBytes)
			}
			// No memory.high: the hard limit alone, so a runaway is killed.
			if _, err := os.Stat(filepath.Join(cg.path, "memory.high")); err == nil || d.Memory.WorkerHighBytes != 0 {
				t.Errorf("a desk worker's cgroup sets memory.high (manifest %d)", d.Memory.WorkerHighBytes)
			}
			for file, want := range map[string]string{"memory.swap.max": "0", "memory.oom.group": "1", "pids.max": strconv.Itoa(workerPidsMax)} {
				if got := readCgroupFile(t, cg.path, file); got != want {
					t.Errorf("%s = %s, want %s", file, got, want)
				}
			}
			if !strings.HasPrefix(filepath.Base(cg.path), workerCgroupPrefix) || filepath.Dir(cg.path) != root {
				t.Errorf("cgroup %s is not a w-* child of the desks root", cg.path)
			}
			if cg.softHeapLimit() >= cg.maxBytes {
				t.Errorf("soft heap limit %d must sit below memory.max %d", cg.softHeapLimit(), cg.maxBytes)
			}
		})
	}
}

func TestWorkerCgroupsAreDistinctPerSpawn(t *testing.T) {
	root := t.TempDir()
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		cg, err := planWorkerCgroup(DeskSessionWorkerConfig{CgroupRoot: root, Session: SessionWorkerConfig{Role: "research"}})
		if err != nil {
			t.Fatal(err)
		}
		if seen[cg.path] {
			t.Fatalf("two workers share cgroup %s", cg.path)
		}
		seen[cg.path] = true
	}
}

// Required mode never starts a worker without a budget; a role the manifest
// gives no budget is refused whenever a desks cgroup is configured.
func TestWorkerBudgetFailsClosed(t *testing.T) {
	fakeCgroupFS(t)
	t.Setenv(DeskCgroupRootEnv, "")
	if _, err := planWorkerCgroup(DeskSessionWorkerConfig{Confinement: CellConfinement{Mode: CellConfinementRequired}, Session: SessionWorkerConfig{Role: "research"}}); err == nil {
		t.Fatal("required mode planned a worker with no cgroup root")
	}
	if cg, err := planWorkerCgroup(DeskSessionWorkerConfig{Session: SessionWorkerConfig{Role: "research"}}); err != nil || cg != nil {
		t.Fatalf("auto mode with no cgroup root should run on rlimits alone: %v %v", cg, err)
	}
	root := t.TempDir()
	for _, role := range []string{"", "engineering", "conductor", "root", "app:never-registered"} {
		if cg, err := planWorkerCgroup(DeskSessionWorkerConfig{CgroupRoot: root, Session: SessionWorkerConfig{Role: role}}); err == nil {
			cg.release()
			t.Errorf("role %q was given a budget it has no manifest row for", role)
		}
	}
	// A root that is not there is a refusal, not a skip.
	if _, err := planWorkerCgroup(DeskSessionWorkerConfig{CgroupRoot: filepath.Join(root, "missing", "desks"), Session: SessionWorkerConfig{Role: "research"}}); err == nil {
		t.Error("a missing desks cgroup was skipped")
	}
	// The env selects the root when the config does not.
	t.Setenv(DeskCgroupRootEnv, root)
	cg, err := planWorkerCgroup(DeskSessionWorkerConfig{Confinement: CellConfinement{Mode: CellConfinementRequired}, Session: SessionWorkerConfig{Role: "texture"}})
	if err != nil || cg == nil {
		t.Fatalf("env root not used: %v", err)
	}
	cg.release()
}

func TestEnsureDeskCgroupRootSetsLimitsAndSweepsOrphans(t *testing.T) {
	fakeCgroupFS(t)
	parent := t.TempDir()
	root := filepath.Join(parent, "choir-desks")
	for _, d := range []string{"w-aaaa", "w-bbbb", "keep-me"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := EnsureDeskCgroupRoot(root, 4<<30, 3<<30); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if got := readCgroupFile(t, root, "memory.max"); got != strconv.Itoa(4<<30) {
		t.Errorf("aggregate memory.max = %s", got)
	}
	if got := readCgroupFile(t, root, "memory.high"); got != strconv.Itoa(3<<30) {
		t.Errorf("aggregate memory.high = %s", got)
	}
	if got := readCgroupFile(t, root, "cgroup.subtree_control"); got != "+memory +pids" {
		t.Errorf("subtree_control = %q", got)
	}
	for _, gone := range []string{"w-aaaa", "w-bbbb"} {
		if _, err := os.Stat(filepath.Join(root, gone)); !os.IsNotExist(err) {
			t.Errorf("orphan %s survived the boot sweep: %v", gone, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "keep-me")); err != nil {
		t.Errorf("the sweep removed a directory it did not create: %v", err)
	}
	if err := EnsureDeskCgroupRoot(root, 1<<30, 2<<30); err == nil {
		t.Error("memory.high above memory.max was accepted")
	}
	if err := EnsureDeskCgroupRoot("", 1, 1); err == nil {
		t.Error("an empty root was accepted")
	}
}

func TestBudgetNameTheKillInTheExitCause(t *testing.T) {
	fakeCgroupFS(t)
	root := t.TempDir()
	cg, err := createWorkerCgroup(root, "w-test", 384<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer cg.release()
	if got := cg.note(); got != "" {
		t.Errorf("a worker that hit nothing has a note: %q", got)
	}
	if err := os.WriteFile(filepath.Join(cg.path, "memory.events"), []byte("low 0\nhigh 3\nmax 1\noom 1\noom_kill 1\noom_group_kill 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := cg.note()
	if !strings.Contains(got, "killed by its memory budget") || !strings.Contains(got, "384 MiB") {
		t.Errorf("note does not say the budget killed the worker: %q", got)
	}
	cg.highBytes = 320 << 20
	if err := os.WriteFile(filepath.Join(cg.path, "memory.events"), []byte("high 4\nmax 0\noom_kill 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := cg.note(); !strings.Contains(got, "throttled") {
		t.Errorf("note does not report throttling: %q", got)
	}
}

// TestWorkerCgroupToleratesNoSwapController: a kernel without the swap
// controller has no memory.swap.max, and no swap to bound; the worker's
// cgroup is still created with its memory.max. Any other write failure (here
// memory.max) still fails closed.
func TestWorkerCgroupToleratesNoSwapController(t *testing.T) {
	fakeCgroupFS(t)
	prev := writeCgroupFile
	t.Cleanup(func() { writeCgroupFile = prev })
	fail := map[string]error{"memory.swap.max": &os.PathError{Op: "open", Err: syscall.ENOENT}}
	writeCgroupFile = func(dir, name, value string) error {
		if err := fail[name]; err != nil {
			return err
		}
		return prev(dir, name, value)
	}
	cg, err := createWorkerCgroup(t.TempDir(), "w-test", 1<<20, 0)
	if err != nil {
		t.Fatalf("no swap controller must not fail the cgroup: %v", err)
	}
	cg.release()

	fail = map[string]error{"memory.max": &os.PathError{Op: "open", Err: syscall.ENOENT}}
	if _, err := createWorkerCgroup(t.TempDir(), "w-test", 1<<20, 0); err == nil {
		t.Fatal("a missing memory.max must still fail closed")
	}
}
