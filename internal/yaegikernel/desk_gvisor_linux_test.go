//go:build linux

package yaegikernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/gomodule"
)

// MA6.4: every desk worker under gVisor, one sandbox per trust class.
//
// Ways this could fail, written first:
//  1. the fd 3 socketpair or the slot token does not cross into the sandbox,
//     so no worker becomes ready (every spawn test below serves a cell);
//  2. the worker runs in the sandbox but unconfined: no seccomp, as root,
//     with capabilities, or outside gVisor (the self-test reads its status
//     from inside and the gVisor marker);
//  3. a cell sees the host: a host file, another class's cells, the
//     network (the self-test's escape probes, with a host canary);
//  4. two classes share a sandbox, or one class gets two (the class test
//     reads the sandboxes back);
//  5. a sandbox outlives its last worker, or a killed worker lives on inside
//     its sandbox (the reap test);
//  6. gVisor missing quietly runs a non-first-party desk outside it (the
//     refusal test);
//  7. the overhead is far from the measured ratio (the benchmark, opt-in).
//
// These run only where runsc is: CHOIR_RUNSC names it (the agents VM:
// ~/bin/runsc, rootless). Root-mode runs (the guest's shape) need a root
// spawner; the boot self-test (POST /api/runtime/isolation-selftest) is the
// proof on a computer.

// TestDeskGvisorHelperProcess is the worker binary for these tests: the
// sandbox roles first, then the production worker entry.
func TestDeskGvisorHelperProcess(t *testing.T) {
	if code, ran := RunDeskSandboxRoleIfRequested(); ran {
		os.Exit(code)
	}
	if os.Getenv("CHOIR_DESK_GVISOR_CHILD") != "1" {
		return
	}
	// A test's fixture modules: the curated list this worker loads against.
	if raw := os.Getenv("CHOIR_TEST_GO_MODULES_CURATED"); raw != "" {
		curatedGoModules = func() ([]gomodule.Module, error) { return gomodule.ParseCurated([]byte(raw)) }
	}
	CloseInheritedFDs()
	cfg := SessionWorkerConfigFromEnv(nil, "")
	fd, conn, err := SessionWorkerSockFD(nil, "")
	if err != nil || fd < 0 || conn == nil {
		fmt.Fprintf(os.Stderr, "helper: no session socket: %v\n", err)
		os.Exit(2)
	}
	if err := RegisterCarriedAppDesk(nil, ""); err != nil {
		fmt.Fprintf(os.Stderr, "helper: %v\n", err)
		os.Exit(2)
	}
	if err := ApplyWorkerMemoryLimit(cfg.MemoryLimitBytes); err != nil {
		fmt.Fprintf(os.Stderr, "helper: %v\n", err)
		os.Exit(2)
	}
	if err := ConfineDeskSessionWorker(nil, ""); err != nil {
		fmt.Fprintf(os.Stderr, "helper: confine: %v\n", err)
		os.Exit(3)
	}
	RunDeskCellSelfTestIfRequested(nil, "")
	ExecuteWorkerSessionConn(conn, cfg)
	os.Exit(0)
}

// requireGvisor points the spawner at the test's runsc and a private state
// dir, or skips.
func requireGvisor(t *testing.T) {
	t.Helper()
	runsc := os.Getenv(RunscPathEnv)
	if runsc == "" {
		t.Skip("CHOIR_RUNSC names no runsc")
	}
	resetGvisorPool(t, func() []string { return []string{runsc} })
}

func resetGvisorPool(t *testing.T, candidates func() []string) {
	t.Helper()
	base, err := os.MkdirTemp("", "choir-gvisor-test-")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(base, 0o755)
	oldCand, oldBase := gvisorRunscCandidates, gvisorStateBase
	gvisorRunscCandidates = candidates
	gvisorStateBase = func() string { return filepath.Join(base, "g") }
	deskGvisor.mu.Lock()
	old := deskGvisor
	deskGvisor = newGvisorPool()
	old.mu.Unlock()
	t.Cleanup(func() {
		deskGvisor.mu.Lock()
		var left []*gvisorSandbox
		for k, sb := range deskGvisor.sandboxes {
			left = append(left, sb)
			delete(deskGvisor.sandboxes, k)
		}
		deskGvisor.mu.Unlock()
		for _, sb := range left {
			sb.stop()
		}
		gvisorRunscCandidates, gvisorStateBase = oldCand, oldBase
		os.RemoveAll(base)
	})
}

// gvisorTestConfig is a production-shaped worker config on the helper.
func gvisorTestConfig(t *testing.T, role string) DeskSessionWorkerConfig {
	t.Helper()
	shared, bin := gvisorTestBinary(t)
	return DeskSessionWorkerConfig{
		Bin:       bin,
		WorkerArg: "-test.run=^TestDeskGvisorHelperProcess$",
		Session: SessionWorkerConfig{
			AllowedPackages:  DefaultSafeStdlibPackagesList(),
			ComputerID:       "host-test",
			ActivationID:     "act-gvisor-" + role,
			Epoch:            1,
			AllowedRoot:      shared,
			Role:             role,
			MemoryLimitBytes: 8 << 30,
		},
		ProcessGroup: true,
		ExtraEnv:     []string{"CHOIR_DESK_GVISOR_CHILD=1"},
		ReadyTimeout: 30 * time.Second,
	}
}

var (
	gvisorTestBinOnce sync.Once
	gvisorTestShared  string
)

// gvisorTestBinary is one 0755 copy of the test binary for the whole run:
// a sandbox serves one worker binary, as on a computer.
func gvisorTestBinary(t *testing.T) (string, string) {
	t.Helper()
	gvisorTestBinOnce.Do(func() {
		shared, err := os.MkdirTemp("", "choir-cell-test-")
		if err != nil {
			t.Fatal(err)
		}
		_ = os.Chmod(shared, 0o755)
		copyExecutable(t, os.Args[0], filepath.Join(shared, "worker.test"))
		gvisorTestShared = shared
	})
	return gvisorTestShared, filepath.Join(gvisorTestShared, "worker.test")
}

func registerTestAppDesk(t *testing.T, name string, uid uint32) {
	t.Helper()
	registerTestAppDeskBudget(t, name, uid, 256<<20)
}

func registerTestAppDeskBudget(t *testing.T, name string, uid uint32, budget uint64) {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: name, Verbs: []string{"Done", "ReadCapture"},
		Taint: deskmanifest.TaintUntrustedInput, UID: uid, WorkerMaxBytes: budget})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp(name) })
}

func gvEvalCell(t *testing.T, w *DeskSessionWorker, src, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := w.Eval(ctx, src)
	if err != nil {
		t.Fatalf("eval: %v (exit %q)", err, w.ExitCause())
	}
	if res.Error != "" || !strings.Contains(res.Stdout, want) {
		t.Fatalf("cell: stdout=%q error=%q, want %q", res.Stdout, res.Error, want)
	}
}

func TestGvisorDeskWorkersShareOneSandboxPerClass(t *testing.T) {
	requireGvisor(t)
	registerTestAppDesk(t, "app:gvtest-one", 62900)
	registerTestAppDesk(t, "app:gvtest-two", 62901)
	roles := []string{"management", "texture", "research", "app:gvtest-one", "app:gvtest-two", "research_web"}
	workers := map[string]*DeskSessionWorker{}
	defer func() {
		for _, w := range workers {
			w.Close()
		}
	}()
	for _, role := range roles {
		w, err := SpawnDeskSessionWorker(gvisorTestConfig(t, role))
		if err != nil {
			t.Fatalf("spawn %s: %v", role, err)
		}
		workers[role] = w
		if w.gvisor == nil {
			t.Fatalf("%s: not under gVisor", role)
		}
		gvEvalCell(t, w, "import \"fmt\"\nfmt.Println(\"cell\", 6*7)", "cell 42")
	}
	id := func(role string) string { _, id := workers[role].gvisor.sandboxKeyID(); return id }
	key := func(role string) string { k, _ := workers[role].gvisor.sandboxKeyID(); return k }
	if id("management") != id("texture") || key("management") != "a" {
		t.Fatalf("class a: management %s/%s, texture %s/%s", key("management"), id("management"), key("texture"), id("texture"))
	}
	if id("research") != id("research_web") || key("research") != "b" {
		t.Fatalf("class b: research %s, research_web %s", id("research"), id("research_web"))
	}
	distinct := map[string]bool{}
	for _, r := range []string{"management", "research", "app:gvtest-one", "app:gvtest-two"} {
		distinct[id(r)] = true
	}
	if len(distinct) != 4 {
		t.Fatalf("want four sandboxes (a, b and one per publisher), got %v", distinct)
	}
	st := GvisorStatusNow()
	raw, _ := json.MarshalIndent(st, "", " ")
	t.Logf("gVisor status with %d workers:\n%s", len(workers), raw)
	if len(st.Sandboxes) != 4 {
		t.Fatalf("read back %d sandboxes, want 4", len(st.Sandboxes))
	}
	var total uint64
	for _, sb := range st.Sandboxes {
		if sb.RSSBytes == 0 || len(sb.HostPIDs) < 2 {
			t.Fatalf("sandbox %s: rss %d host pids %v", sb.ID, sb.RSSBytes, sb.HostPIDs)
		}
		total += sb.RSSBytes
		t.Logf("sandbox %s class %s workers %d: RSS %.1f MiB PSS %.1f MiB (%d host processes)", sb.Key, sb.Class, sb.Workers, float64(sb.RSSBytes)/(1<<20), float64(sb.PSSBytes)/(1<<20), len(sb.HostPIDs))
	}
	t.Logf("total RSS of 4 sandboxes serving %d workers: %.1f MiB", len(workers), float64(total)/(1<<20))

	// A killed worker ends inside its sandbox; the sandbox lives on for its
	// class-mate and stops with its last worker.
	workers["texture"].Kill()
	delete(workers, "texture")
	gvEvalCell(t, workers["management"], "import \"fmt\"\nfmt.Println(\"still\", 1)", "still 1")
	workers["management"].Close()
	delete(workers, "management")
	for _, sb := range GvisorStatusNow().Sandboxes {
		if sb.Key == "a" {
			t.Fatalf("class a's sandbox outlived its last worker: %+v", sb)
		}
	}
}

func TestGvisorDeskCellSelfTest(t *testing.T) {
	requireGvisor(t)
	base := gvisorTestConfig(t, "")
	// A host file any uid may read: inside a sandbox it is not in view.
	secretDir, err := os.MkdirTemp("", "choir-host-secret-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(secretDir) })
	_ = os.Chmod(secretDir, 0o755)
	secret := filepath.Join(secretDir, "owner-file")
	if err := os.WriteFile(secret, []byte("owner"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := DeskCellSelfTest(context.Background(), DeskCellSelfTestConfig{Base: base, ExtraReads: []string{secret, filepath.Dir(os.Args[0])}})
	raw, _ := json.MarshalIndent(rec, "", " ")
	t.Logf("receipt:\n%s", raw)
	if !rec.OK {
		t.Fatalf("self-test failures: %v", rec.Failures)
	}
	for _, w := range rec.Workers {
		if w.Profile != "gvisor" {
			t.Fatalf("%s: profile %s", w.Role, w.Profile)
		}
	}
}

func TestGvisorUnavailableRefusesNonFirstParty(t *testing.T) {
	resetGvisorPool(t, func() []string { return nil })
	registerTestAppDesk(t, "app:gvtest-refused", 62902)
	_, err := SpawnDeskSessionWorker(gvisorTestConfig(t, "app:gvtest-refused"))
	if err == nil || !strings.Contains(err.Error(), "not first-party") {
		t.Fatalf("app desk without gVisor: %v, want a refusal", err)
	}
	w, err := SpawnDeskSessionWorker(gvisorTestConfig(t, "research"))
	if err != nil {
		t.Fatalf("first-party desk without gVisor: %v", err)
	}
	defer w.Close()
	if w.gvisor != nil {
		t.Fatal("research claims a sandbox without gVisor")
	}
	gvEvalCell(t, w, "import \"fmt\"\nfmt.Println(\"cell\", 6*7)", "cell 42")
}

// TestGvisorCellBenchmark measures warm cells native and under gVisor
// (MA0.1's method): set CHOIR_GVISOR_BENCH=1.
func TestGvisorCellBenchmark(t *testing.T) {
	if os.Getenv("CHOIR_GVISOR_BENCH") != "1" {
		t.Skip("CHOIR_GVISOR_BENCH=1 runs the benchmark")
	}
	const n = 300
	cells := map[string]string{
		"trivial": "x := 1 + 1\n_ = x",
		"fib25":   "var fib func(int) int\nfib = func(n int) int { if n < 2 { return n }; return fib(n-1) + fib(n-2) }\n_ = fib(25)",
		"alloc":   "s := 0\nfor i := 0; i < 20000; i++ { b := make([]byte, 64); s += len(b) }\n_ = s",
		"verb":    "_, _ = choir.ReadCapture(\"bench\", 0, 1)",
	}
	run := func(t *testing.T, gv bool) map[string][2]time.Duration {
		if gv {
			requireGvisor(t)
		} else {
			resetGvisorPool(t, func() []string { return nil })
		}
		cfg := gvisorTestConfig(t, "research")
		cfg.Egress = func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{}`), nil
		}
		start := time.Now()
		w, err := SpawnDeskSessionWorker(cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer w.Close()
		t.Logf("gvisor=%v spawn to ready %s", gv, time.Since(start))
		out := map[string][2]time.Duration{}
		names := []string{"trivial", "verb", "alloc", "fib25"}
		for _, name := range names {
			var ds []time.Duration
			for i := 0; i < n; i++ {
				if name == "fib25" && i >= 30 {
					break
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				s := time.Now()
				res, err := w.Eval(ctx, cells[name])
				cancel()
				if err != nil || res.Error != "" {
					t.Fatalf("%s: %v %s", name, err, res.Error)
				}
				ds = append(ds, time.Since(s))
			}
			sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
			out[name] = [2]time.Duration{ds[len(ds)/2], ds[len(ds)*99/100]}
		}
		// MA6.5: a worker that loads the curated go-difflib; its spawn to
		// ready and a cell that diffs with it.
		cfg.Session.GoModules = encodedBundle(t, difflibBundle(t))
		start = time.Now()
		wm, err := SpawnDeskSessionWorker(cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer wm.Close()
		t.Logf("gvisor=%v spawn to ready with go modules %s", gv, time.Since(start))
		var ds []time.Duration
		for i := 0; i < n; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			s := time.Now()
			res, err := wm.Eval(ctx, moduleCell)
			cancel()
			if err != nil || res.Error != "" {
				t.Fatalf("module: %v %s", err, res.Error)
			}
			ds = append(ds, time.Since(s))
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		out["module"] = [2]time.Duration{ds[len(ds)/2], ds[len(ds)*99/100]}
		return out
	}
	var native, gv map[string][2]time.Duration
	t.Run("native", func(t *testing.T) { native = run(t, false) })
	t.Run("gvisor", func(t *testing.T) { gv = run(t, true) })
	for _, name := range []string{"trivial", "verb", "alloc", "fib25", "module"} {
		t.Logf("%-8s native p50 %8s p99 %8s | gvisor p50 %8s p99 %8s | ratio p50 %.2fx", name, native[name][0], native[name][1], gv[name][0], gv[name][1], float64(gv[name][0])/float64(native[name][0]))
	}
}

// TestGvisorDeskCellCallsAGoModuleAndAHostVerb is MA6.5's proof under the
// MA6.4 sandbox: the bundle crosses into the gVisor process spec, the worker
// loads the curated module, and one cell calls it and a host verb.
func TestGvisorDeskCellCallsAGoModuleAndAHostVerb(t *testing.T) {
	requireGvisor(t)
	cfg := gvisorTestConfig(t, "research")
	cfg.Session.GoModules = encodedBundle(t, difflibBundle(t))
	var seen atomic.Int32
	cfg.Egress = func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
		seen.Add(1)
		return json.RawMessage(`{"text":"from the desk's host"}`), nil
	}
	w, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if w.gvisor == nil {
		t.Fatal("the worker is not in a gVisor sandbox")
	}
	gvEvalCell(t, w, moduleCell, moduleCellWant)
	if seen.Load() != 1 {
		t.Fatalf("host verb calls: %d", seen.Load())
	}
	// A bad carry is refused inside the sandbox too.
	bad := gvisorTestConfig(t, "research")
	bad.Session.GoModules = cfg.Session.GoModules
	bad.ExtraEnv = append(bad.ExtraEnv, "CHOIR_DESK_SESSION_GO_MODULES_SHA256="+strings.Repeat("0", 64))
	if w2, err := SpawnDeskSessionWorker(bad); err == nil {
		w2.Close()
		t.Fatal("a gVisor worker with an altered carry became ready")
	}
}

// TestGvisorLargestGoModuleBundleCrosses: a bundle at the source cap crosses
// the environment into the sandbox in several parts and loads.
func TestGvisorLargestGoModuleBundleCrosses(t *testing.T) {
	requireGvisor(t)
	body := strings.Repeat("x", gomodule.MaxBundleBytes-4096)
	files := map[string][]byte{"big.go": []byte("package big\n\nconst S = `" + body + "`\n\nfunc N() int { return len(S) }\n")}
	mod := gomodule.Module{Path: "example.com/big", Version: "v0.1.0"}
	sum, err := gomodule.Hash1(mod.String(), files)
	if err != nil {
		t.Fatal(err)
	}
	mod.Sum = sum
	b, err := gomodule.Resolve([]gomodule.Module{mod}, []gomodule.ModuleSource{{Module: gomodule.Module{Path: mod.Path, Version: mod.Version}, Files: files}}, []string{mod.Path})
	if err != nil {
		t.Fatal(err)
	}
	cfg := gvisorTestConfig(t, "research")
	cfg.Session.GoModules = encodedBundle(t, b)
	cfg.ExtraEnv = append(cfg.ExtraEnv, "CHOIR_TEST_GO_MODULES_CURATED="+mod.Path+" "+mod.Version+" "+mod.Sum)
	if parts := len(goModulesEnv("P_", cfg.Session.GoModules)) - 2; parts < 4 {
		t.Fatalf("the cap bundle crosses in %d parts; the test wants several", parts)
	}
	start := time.Now()
	w, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	t.Logf("cap bundle (%d bytes encoded) spawn to ready %s", len(cfg.Session.GoModules), time.Since(start))
	gvEvalCell(t, w, "fmt.Println(big.N())", strconv.Itoa(len(body)))
}

// TestGvisorConcurrentSpawnsShareOneSandbox: concurrent spawns of one class
// start one sandbox, not one each.
func TestGvisorConcurrentSpawnsShareOneSandbox(t *testing.T) {
	requireGvisor(t)
	var wg sync.WaitGroup
	ws := make([]*DeskSessionWorker, 4)
	errs := make([]error, 4)
	for i := range ws {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ws[i], errs[i] = SpawnDeskSessionWorker(gvisorTestConfig(t, "research"))
		}(i)
	}
	wg.Wait()
	defer func() {
		for _, w := range ws {
			if w != nil {
				w.Close()
			}
		}
	}()
	ids := map[string]bool{}
	for i, w := range ws {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if w.gvisor == nil {
			t.Fatal("research worker not under gVisor")
		}
		_, id := w.gvisor.sandboxKeyID()
		ids[id] = true
		gvEvalCell(t, w, "import \"fmt\"\nfmt.Println(\"cell\", 6*7)", "cell 42")
	}
	if len(ids) != 1 {
		t.Fatalf("4 research workers in %d sandboxes, want 1", len(ids))
	}
	st := GvisorStatusNow()
	if len(st.Sandboxes) != 1 || st.Sandboxes[0].Workers != 4 {
		t.Fatalf("status %+v", st.Sandboxes)
	}
	t.Logf("one sandbox, 4 workers: RSS %.1f MiB, PSS %.1f MiB", float64(st.Sandboxes[0].RSSBytes)/(1<<20), float64(st.Sandboxes[0].PSSBytes)/(1<<20))
}

// TestGvisorRunawayEndsOnlyItsWorker: in a shared sandbox the worker's hard
// bound is its address-space limit; a runaway cell ends its own worker and
// its class-mate keeps serving.
func TestGvisorRunawayEndsOnlyItsWorker(t *testing.T) {
	requireGvisor(t)
	sib, err := SpawnDeskSessionWorker(gvisorTestConfig(t, "research"))
	if err != nil {
		t.Fatal(err)
	}
	defer sib.Close()
	w, err := SpawnDeskSessionWorker(gvisorTestConfig(t, "research"))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_, a := sib.gvisor.sandboxKeyID()
	_, b := w.gvisor.sandboxKeyID()
	if a != b {
		t.Fatal("research workers in two sandboxes")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	_, err = w.Eval(ctx, "var keep [][]byte\nfor {\n\tb := make([]byte, 8<<20)\n\tfor i := 0; i < len(b); i += 4096 {\n\t\tb[i] = 1\n\t}\n\tkeep = append(keep, b)\n}")
	w.Close()
	// Under -race the race runtime meets the address limit first, in its
	// own words.
	se := w.stderr.String()
	oom := deskOOMRuntimeOOM(se) || strings.Contains(se, "ThreadSanitizer failed to allocate") || strings.Contains(se, "address space collisions for -race mode")
	if err == nil || !w.Dead() || !oom {
		t.Fatalf("runaway: err %v dead %v exit %q stderr head %.600q", err, w.Dead(), w.ExitCause(), w.stderr.String())
	}
	t.Logf("runaway ended: %s", tailString(w.ExitCause(), 200))
	gvEvalCell(t, sib, "import \"fmt\"\nfmt.Println(\"cell\", 6*7)", "cell 42")
}

// guest2GiB is the desks cgroup of the smallest guest (2048 MiB: vmctl's
// interactive default): memory.max MemTotal/2, no memory.high
// (internal/autoputer/desk_cgroup.go, MA6.4 r4).
const (
	guest2GiBDesksMax  = 1 << 30
	guest2GiBDesksHigh = 0
)

// requireRootCgroup is the guest's shape: a root spawner and a desks cgroup
// sized as on a 2 GiB guest. It runs where the test is root and
// CHOIR_DESK_CGROUP_ROOT names a writable cgroup (a privileged container).
func requireRootCgroup(t *testing.T) string {
	t.Helper()
	requireGvisor(t)
	root := os.Getenv(DeskCgroupRootEnv)
	if os.Geteuid() != 0 || root == "" {
		t.Skip("needs a root spawner and " + DeskCgroupRootEnv)
	}
	if err := EnsureDeskCgroupRoot(root, guest2GiBDesksMax, guest2GiBDesksHigh); err != nil {
		t.Fatal(err)
	}
	return root
}

func rootConfig(t *testing.T, role, root string) DeskSessionWorkerConfig {
	cfg := gvisorTestConfig(t, role)
	cfg.Confinement = CellConfinement{Mode: CellConfinementRequired}
	cfg.CgroupRoot = root
	return cfg
}

// TestGvisorRequiredModeAsRoot: the boot self-test in the guest's shape.
func TestGvisorRequiredModeAsRoot(t *testing.T) {
	root := requireRootCgroup(t)
	base := rootConfig(t, "", root)
	rec := DeskCellSelfTest(context.Background(), DeskCellSelfTestConfig{Base: base, ExtraReads: []string{"/etc/hostname", root}})
	raw, _ := json.MarshalIndent(rec, "", " ")
	t.Logf("receipt:\n%s", raw)
	if !rec.OK {
		t.Fatalf("self-test failures: %v", rec.Failures)
	}
	var sum uint64
	for _, sb := range rec.Gvisor.Sandboxes {
		max, err := strconv.ParseUint(sb.MemoryMax, 10, 64)
		if sb.Cgroup == "" || err != nil {
			t.Fatalf("sandbox %s has no memory budget: %+v", sb.ID, sb)
		}
		sum += max
	}
	if sum > guest2GiBDesksMax {
		t.Fatalf("sandbox memory.max sum %d MiB exceeds the desks cgroup's %d MiB", sum>>20, guest2GiBDesksMax>>20)
	}
	t.Logf("sandbox memory.max sum %d MiB of %d MiB", sum>>20, guest2GiBDesksMax>>20)
}

// TestGvisorOOMLegOn2GiBGuest: the runaway worker's own address limit ends
// it before its sandbox's cgroup or the desks cgroup reaches an OOM, on the
// smallest guest's desks cgroup: its class-mate keeps serving and no
// oom_kill is recorded anywhere.
func TestGvisorOOMLegOn2GiBGuest(t *testing.T) {
	root := requireRootCgroup(t)
	before := readDeskOOMCgroup(root).Events["oom_kill"]
	rec := DeskOOMSelfTest(context.Background(), DeskOOMSelfTestConfig{Base: rootConfig(t, "", root), CellTimeout: 90 * time.Second})
	raw, _ := json.MarshalIndent(rec, "", " ")
	t.Logf("receipt:\n%s", raw)
	if !rec.OK {
		t.Fatalf("OOM leg failures: %v", rec.Failures)
	}
	if after := readDeskOOMCgroup(root).Events["oom_kill"]; after != before {
		t.Fatalf("desks cgroup oom_kill %d -> %d: the cgroup bound fired before the worker's own", before, after)
	}
	if rec.Worker.HeapShareBytes == 0 || rec.Worker.HeapShareBytes >= rec.BudgetBytes {
		t.Fatalf("heap share %d on a 1 GiB desks cgroup, want below the %d budget", rec.Worker.HeapShareBytes, rec.BudgetBytes)
	}
}

// TestGvisorAdmissionRefusesBeyondDesksMemory: workers are admitted while
// every claim fits in the desks cgroup, then refused by name; a reap lowers
// the sandbox's memory.max and admits again; the last worker out removes
// the sandbox's cgroup.
func TestGvisorAdmissionRefusesBeyondDesksMemory(t *testing.T) {
	root := requireRootCgroup(t)
	var ws []*DeskSessionWorker
	defer func() {
		for _, w := range ws {
			w.Close()
		}
	}()
	var refusal error
	for i := 0; i < 12; i++ {
		// A short deadline: past what fits, the spawn waits for a reap and
		// is refused by name when its deadline passes (r3).
		cfg := rootConfig(t, "research", root)
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		cfg.Context = ctx
		w, err := SpawnDeskSessionWorker(cfg)
		cancel()
		if err != nil {
			refusal = err
			break
		}
		ws = append(ws, w)
	}
	if refusal == nil || !errors.Is(refusal, errDesksMemoryCommitted) {
		t.Fatalf("after %d workers: %v, want the desks-memory refusal", len(ws), refusal)
	}
	if strings.Contains(refusal.Error(), "process confinement") || len(ws) < 3 {
		t.Fatalf("refusal after %d workers: %v", len(ws), refusal)
	}
	t.Logf("admitted %d research workers, then: %v", len(ws), refusal)
	cg := ws[0].gvisor.cgroupPath()
	readMax := func() uint64 {
		raw, _ := os.ReadFile(filepath.Join(cg, "memory.max"))
		n, _ := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
		return n
	}
	full := readMax()
	if full == 0 || full > guest2GiBDesksMax {
		t.Fatalf("sandbox memory.max %d", full)
	}
	deskGvisor.mu.Lock()
	committed := deskGvisor.committed
	deskGvisor.mu.Unlock()
	if committed > guest2GiBDesksMax {
		t.Fatalf("committed %d MiB beyond the desks cgroup", committed>>20)
	}
	last := ws[len(ws)-1]
	last.Close()
	ws = ws[:len(ws)-1]
	if lowered := readMax(); lowered >= full {
		t.Fatalf("memory.max %d after a reap, was %d: not lowered", lowered, full)
	} else {
		t.Logf("memory.max %d MiB -> %d MiB after one reap", full>>20, lowered>>20)
	}
	w, err := SpawnDeskSessionWorker(rootConfig(t, "research", root))
	if err != nil {
		t.Fatalf("after a reap: %v", err)
	}
	ws = append(ws, w)
	gvEvalCell(t, w, "import \"fmt\"\nfmt.Println(\"cell\", 6*7)", "cell 42")
	for _, w := range ws {
		w.Close()
	}
	ws = nil
	if _, err := os.Stat(cg); !os.IsNotExist(err) {
		t.Fatalf("sandbox cgroup %s after its last worker: %v", cg, err)
	}
	deskGvisor.mu.Lock()
	committed = deskGvisor.committed
	deskGvisor.mu.Unlock()
	if committed != 0 {
		t.Fatalf("committed %d after every worker left", committed)
	}
}

// TestGvisorAdmissionWaitsOn2GiBGuest: on the smallest guest's desks
// cgroup, management, Texture and research run; two research children
// spawned beside them wait for desks memory (visible as waiting), then each
// runs after a reap, and every cell completes with its wait recorded.
func TestGvisorAdmissionWaitsOn2GiBGuest(t *testing.T) {
	root := requireRootCgroup(t)
	SetDeskMemoryPressureHandler(nil)
	live := map[string]*DeskSessionWorker{}
	defer func() {
		for _, w := range live {
			w.Close()
		}
	}()
	for _, role := range []string{"management", "texture", "research"} {
		w, err := SpawnDeskSessionWorker(rootConfig(t, role, root))
		if err != nil {
			t.Fatalf("spawn %s: %v", role, err)
		}
		live[role] = w
		gvEvalCell(t, w, "import \"fmt\"\nfmt.Println(\"cell\", 6*7)", "cell 42")
	}
	type spawned struct {
		role string
		w    *DeskSessionWorker
		err  error
	}
	out := make(chan spawned, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, role := range []string{"research_web", "research_texture"} {
		cfg := rootConfig(t, role, root)
		cfg.Context = ctx
		go func(role string, cfg DeskSessionWorkerConfig) {
			w, err := SpawnDeskSessionWorker(cfg)
			out <- spawned{role, w, err}
		}(role, cfg)
	}
	deadline := time.Now().Add(10 * time.Second)
	for GvisorStatusNow().Waiting != 2 {
		if time.Now().After(deadline) {
			t.Fatalf("waiting %d, want the two children queued", GvisorStatusNow().Waiting)
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case s := <-out:
		t.Fatalf("%s spawned beside three workers on a 2 GiB guest (err %v)", s.role, s.err)
	case <-time.After(time.Second):
	}
	st := GvisorStatusNow()
	t.Logf("queued: %d waiting, %d MiB claimed", st.Waiting, st.CommittedBytes>>20)
	// research ends: one child runs.
	live["research"].Close()
	delete(live, "research")
	for i := 0; i < 2; i++ {
		var s spawned
		select {
		case s = <-out:
		case <-time.After(30 * time.Second):
			t.Fatalf("child %d never admitted", i)
		}
		if s.err != nil {
			t.Fatalf("%s: %v", s.role, s.err)
		}
		live[s.role] = s.w
		if s.w.AdmissionQueued() <= 0 {
			t.Fatalf("%s: queued %s, want its wait recorded", s.role, s.w.AdmissionQueued())
		}
		gvEvalCell(t, s.w, "import \"fmt\"\nfmt.Println(\"child\", 6*7)", "child 42")
		t.Logf("%s ran after waiting %s", s.role, s.w.AdmissionQueued().Round(time.Millisecond))
		if i == 0 {
			select {
			case s2 := <-out:
				t.Fatalf("%s admitted beside the first child (err %v)", s2.role, s2.err)
			case <-time.After(500 * time.Millisecond):
			}
		}
		// The child finishes: the next one runs.
		s.w.Close()
		delete(live, s.role)
	}
	gvEvalCell(t, live["management"], "import \"fmt\"\nfmt.Println(\"still\", 1)", "still 1")
	gvEvalCell(t, live["texture"], "import \"fmt\"\nfmt.Println(\"still\", 1)", "still 1")
}

// TestGvisorReapRemovesCellTmpDir: a reaped worker's tmp dir leaves the
// sandbox's /tmp while its class-mate lives on.
func TestGvisorReapRemovesCellTmpDir(t *testing.T) {
	requireGvisor(t)
	sib, err := SpawnDeskSessionWorker(gvisorTestConfig(t, "research"))
	if err != nil {
		t.Fatal(err)
	}
	defer sib.Close()
	w, err := SpawnDeskSessionWorker(gvisorTestConfig(t, "research"))
	if err != nil {
		t.Fatal(err)
	}
	gvEvalCell(t, w, "import \"fmt\"\nfmt.Println(\"cell\", 6*7)", "cell 42")
	dir := w.gvisor.sandboxDir
	// As nobody: a live dir (0700, another uid) is denied, a removed one is
	// absent.
	probe := func() string {
		rep, err := sib.gvisor.inspectWith(nobodyUID, filepath.Join(dir, "x"), "")
		if err != nil {
			t.Fatal(err)
		}
		return rep.FileError
	}
	if fe := probe(); !strings.Contains(fe, "permission denied") {
		t.Fatalf("live cell dir read as nobody: %q, want permission denied", fe)
	}
	w.Close()
	if fe := probe(); !strings.Contains(fe, "no such file") {
		t.Fatalf("reaped cell dir read as nobody: %q, want no such file", fe)
	}
	deskGvisor.mu.Lock()
	held := len(sib.gvisor.sb.heldUIDs)
	deskGvisor.mu.Unlock()
	if held != 0 {
		t.Fatalf("%d uids held after a confirmed kill", held)
	}
}

// TestGvisorRunscPathFailsClosed: a CHOIR_RUNSC naming nothing makes gVisor
// unavailable, cached, rather than falling back to another runsc.
func TestGvisorRunscPathFailsClosed(t *testing.T) {
	t.Setenv(RunscPathEnv, "/nonexistent/runsc")
	if got := gvisorRunscCandidates(); len(got) != 1 || got[0] != "/nonexistent/runsc" {
		t.Fatalf("candidates %v", got)
	}
	calls := 0
	resetGvisorPool(t, func() []string { calls++; return []string{"/nonexistent/runsc"} })
	for i := 0; i < 3; i++ {
		if ok, _ := gvisorAvailable(); ok {
			t.Fatal("available with no runsc")
		}
	}
	if calls != 1 {
		t.Fatalf("looked for runsc %d times, want 1 (cached)", calls)
	}
}

// TestWorkerShareFitsEveryGuest: on every guest size a share is within
// [the minimum, the budget]; above the minimum, the claims of
// gvisorShareWorkers workers in gvisorShareSandboxes sandboxes fit the
// desks cgroup; and the smallest guest admits at least three workers (the
// core desks) across three sandboxes.
func TestWorkerShareFitsEveryGuest(t *testing.T) {
	for _, guestMiB := range []uint64{2048, 3072, 4096, 8192, 16384} {
		desks := guestMiB << 20 / 2
		for _, budget := range []uint64{384 << 20, 512 << 20, 768 << 20} {
			share := workerShare(budget, desks)
			claim := gvisorWorkerClaim(share)
			if share > budget || share < gvisorWorkerMinBytes {
				t.Fatalf("guest %d MiB budget %d MiB: share %d MiB", guestMiB, budget>>20, share>>20)
			}
			fit := func(sandboxes uint64) uint64 { return (desks - sandboxes*gvisorSandboxOverheadBytes) / claim }
			if share > gvisorWorkerMinBytes && fit(gvisorShareSandboxes) < gvisorShareWorkers {
				t.Fatalf("guest %d MiB: %d workers at %d MiB claims do not fit beside %d sandboxes", guestMiB, gvisorShareWorkers, claim>>20, gvisorShareSandboxes)
			}
			if fit(gvisorShareSandboxes) < 3 {
				t.Fatalf("guest %d MiB: only %d workers fit beside three sandboxes", guestMiB, fit(gvisorShareSandboxes))
			}
			t.Logf("guest %5d MiB, desks %5d MiB, budget %3d MiB: share %3d MiB, claim %3d MiB, address limit %d MiB; workers that fit beside 2 / 3 sandboxes: %d / %d",
				guestMiB, desks>>20, budget>>20, share>>20, claim>>20, (share+gvisorAddressOverheadBytes)>>20, fit(2), fit(3))
		}
	}
}

// TestGvisorAddressCalibration re-measures gvisorAddressOverheadBytes: at
// each share a worker holding live 8 MiB slabs until it dies holds at least
// 90% of its share (else a Go or gVisor upgrade shrank every heap) and no
// more than its claim allows (else the claim no longer bounds it, and a
// runaway could reach its sandbox's cgroup). With a desks cgroup it also
// checks the sandbox's physical rise against the claim. Runs where runsc is
// (the agents VM, a privileged container, the guest); CHOIR_GVISOR_CALIB_TRIALS
// sets the trials per share (default 3).
func TestGvisorAddressCalibration(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("the race runtime's shadow memory is address space the calibration does not model")
	}
	requireGvisor(t)
	trials := 3
	if n, err := strconv.Atoi(os.Getenv("CHOIR_GVISOR_CALIB_TRIALS")); err == nil && n > 0 {
		trials = n
	}
	root := os.Getenv(DeskCgroupRootEnv)
	if root != "" {
		// A large desks cgroup: each worker gets its desk's budget as its
		// share (a delegated user cgroup works rootless).
		if err := EnsureDeskCgroupRoot(root, 8<<30, 7<<30); err != nil {
			t.Fatal(err)
		}
	}
	const runaway = "var keep [][]byte\nfor n := 8; ; n += 8 {\n\tb := make([]byte, 8<<20)\n\tfor i := 0; i < len(b); i += 4096 {\n\t\tb[i] = 1\n\t}\n\tkeep = append(keep, b)\n\t_, _ = choir.ReadCapture(\"held\", n, 1)\n}"
	for i, shareMiB := range []uint64{96, 124, 256, 512} {
		share := shareMiB << 20
		name := fmt.Sprintf("app:gvcalib-%d", shareMiB)
		registerTestAppDeskBudget(t, name, uint32(62910+i), share)
		var lo, hi int64 = 1 << 40, 0
		for trial := 0; trial < trials; trial++ {
			workerCrashes.reset()
			var held atomic.Int64
			cfg := gvisorTestConfig(t, name)
			cfg.CgroupRoot = root
			cfg.Egress = func(_ context.Context, _ BrokerAction, raw json.RawMessage) (json.RawMessage, error) {
				var p ReadCapturePayload
				_ = json.Unmarshal(raw, &p)
				held.Store(int64(p.FromBlock))
				return json.RawMessage(`{}`), nil
			}
			w, err := SpawnDeskSessionWorker(cfg)
			if err != nil {
				t.Fatalf("share %d MiB: spawn: %v", shareMiB, err)
			}
			if got := w.gvisor.heapShare(); got != share {
				w.Close()
				t.Fatalf("share %d MiB: the worker was admitted with %d", shareMiB, got)
			}
			cg := w.gvisor.cgroupPath()
			readCur := func() int64 {
				b, _ := os.ReadFile(filepath.Join(cg, "memory.current"))
				n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
				return n
			}
			var base int64
			var peak atomic.Int64
			stop := make(chan struct{})
			done := make(chan struct{})
			if cg != "" {
				base = readCur()
				go func() {
					defer close(done)
					for {
						select {
						case <-stop:
							return
						case <-time.After(time.Millisecond):
						}
						if c := readCur(); c > peak.Load() {
							peak.Store(c)
						}
					}
				}()
			} else {
				close(done)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			_, evalErr := w.Eval(ctx, runaway)
			cancel()
			close(stop)
			<-done
			dead := w.Dead()
			w.Close()
			if evalErr == nil || !dead {
				t.Fatalf("share %d MiB: the runaway did not end its worker", shareMiB)
			}
			h := held.Load() << 20
			if h < lo {
				lo = h
			}
			if h > hi {
				hi = h
			}
			if cg != "" {
				rise := uint64(peak.Load() - base)
				// base already holds the idle worker: the rise is its heap.
				if rise > gvisorHeapAllowance(share) {
					t.Errorf("share %d MiB: the sandbox rose %d MiB from the idle worker, past its heap allowance %d MiB", shareMiB, rise>>20, gvisorHeapAllowance(share)>>20)
				}
				t.Logf("share %d MiB trial %d: held %d MiB, sandbox rise %d MiB", shareMiB, trial, h>>20, rise>>20)
			}
		}
		limit := gvisorAddressLimit(0, share)
		t.Logf("%s share %4d MiB, address limit %4d MiB: held %d to %d MiB (claim %d MiB)", runtime.GOARCH, shareMiB, limit>>20, lo>>20, hi>>20, gvisorWorkerClaim(share)>>20)
		if uint64(lo) < share/10*9 {
			t.Errorf("share %d MiB: a worker held only %d MiB at address limit %d MiB, under 90%% of its share: recalibrate gvisorAddressOverheadBytes", shareMiB, lo>>20, limit>>20)
		}
		if uint64(hi) > gvisorHeapAllowance(share) {
			t.Errorf("share %d MiB: a worker held %d MiB, past its claim's heap allowance %d MiB: recalibrate gvisorAddressOverheadBytes or the claim", shareMiB, hi>>20, gvisorHeapAllowance(share)>>20)
		}
	}
}
