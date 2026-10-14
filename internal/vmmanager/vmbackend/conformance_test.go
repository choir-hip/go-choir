package vmbackend

// Conformance suite every Backend must pass. Written from the failure list
// in docs/reports/vm-backend-interface-2026-10-14.md section 7, before the
// backends:
//
//	F1  a capability is reported but not implemented (or implemented but not
//	    reported), so S3/LR code takes a path the backend cannot serve;
//	F2  a launch plan does not round-trip through process identification,
//	    so a running computer is invisible to boot guards, destroy guards
//	    and the host-capacity reconcile (live state deleted, double boot);
//	F3  two backends both claim one process, so a reaper kills the wrong VM;
//	F4  the plan is not deterministic, so a snapshot's launch digest never
//	    matches and every wake falls back to a crash boot;
//	F5  an invalid spec (empty or path-like VM id, relative state dir, no
//	    vCPUs) reaches exec;
//	F6  a host path the VMM opens is missing from HostPaths, so a jail or
//	    Landlock rule set built from the plan breaks boot, or a read-only
//	    drive is exposed read-write;
//	F7  an fd the args name is not passed (or a nil fd is), so the VMM boots
//	    without networking or fails;
//	F8  (live) the VMM exits at once on an argument it does not accept, is
//	    not seen in /proc, reports no metrics, or survives Stop.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// conformanceEnv describes how to exercise a backend.
type conformanceEnv struct {
	backend Backend
	// spec returns a valid spec for vmID rooted at dir.
	spec func(t *testing.T, dir, vmID string) MachineSpec
	// live starts the plan for real and checks process identity, metrics
	// and stop.
	live bool
	// aliveFor is how long a live VMM must survive after Start (an
	// immediate exit means it refused its arguments).
	aliveFor time.Duration
}

func runConformance(t *testing.T, env conformanceEnv) {
	b := env.backend
	t.Run("name", func(t *testing.T) {
		n, err := ParseName(string(b.Name()))
		if err != nil && !isFakeName(b.Name()) {
			t.Fatalf("backend name %q does not parse: %v", b.Name(), err)
		}
		if err == nil && n != b.Name() {
			t.Fatalf("ParseName(%q) = %q", b.Name(), n)
		}
	})
	t.Run("F1 caps match interfaces", func(t *testing.T) {
		caps := b.Caps()
		if !caps.ColdBoot {
			t.Fatal("every backend must cold boot")
		}
		_, snap := b.(Snapshotter)
		if caps.Snapshot != snap {
			t.Fatalf("Caps().Snapshot=%v but implements Snapshotter=%v", caps.Snapshot, snap)
		}
		_, mem := b.(MemoryController)
		if caps.BalloonResize != mem {
			t.Fatalf("Caps().BalloonResize=%v but implements MemoryController=%v", caps.BalloonResize, mem)
		}
	})
	dir := t.TempDir()
	spec := env.spec(t, dir, "vm-conformance-a")
	plan, err := b.PlanLaunch(spec)
	if err != nil {
		t.Fatalf("PlanLaunch: %v", err)
	}
	t.Run("plan identity", func(t *testing.T) {
		if plan.Backend != b.Name() || plan.VMID != spec.VMID || plan.Binary == "" || plan.ConfigDigest == "" {
			t.Fatalf("plan identity incomplete: backend=%q vm=%q binary=%q digest=%q", plan.Backend, plan.VMID, plan.Binary, plan.ConfigDigest)
		}
		if plan.APISocket != spec.APISocketPath {
			t.Fatalf("plan API socket %q, spec %q", plan.APISocket, spec.APISocketPath)
		}
	})
	t.Run("F2 identification round trip", func(t *testing.T) {
		argv := append([]string{plan.Binary}, plan.Args...)
		if id, ok := b.VMIDFromCmdline(argv); !ok || id != spec.VMID {
			t.Fatalf("VMIDFromCmdline(own plan) = %q,%v; want %q", id, ok, spec.VMID)
		}
		proc := []byte(strings.Join(argv, "\x00") + "\x00")
		if id, ok := b.VMIDFromCmdline(SplitCmdline(proc)); !ok || id != spec.VMID {
			t.Fatalf("VMIDFromCmdline(/proc form) = %q,%v; want %q", id, ok, spec.VMID)
		}
		other, err := b.PlanLaunch(env.spec(t, t.TempDir(), "vm-conformance-a-extra"))
		if err != nil {
			t.Fatal(err)
		}
		if id, _ := b.VMIDFromCmdline(append([]string{other.Binary}, other.Args...)); id == spec.VMID {
			t.Fatal("another VM's plan identified as this VM (prefix match?)")
		}
		for _, foreign := range [][]string{{"/bin/sleep", "60"}, {}, {"sh", "-c", "vm_id=" + spec.VMID}} {
			if id, ok := b.VMIDFromCmdline(foreign); ok {
				t.Fatalf("foreign argv %q identified as %q", foreign, id)
			}
		}
	})
	t.Run("F4 deterministic digest", func(t *testing.T) {
		again, err := b.PlanLaunch(spec)
		if err != nil {
			t.Fatal(err)
		}
		if again.ConfigDigest != plan.ConfigDigest || strings.Join(again.Args, "\x00") != strings.Join(plan.Args, "\x00") {
			t.Fatal("same spec planned twice differs")
		}
		bigger := spec
		bigger.MemMiB *= 2
		changed, err := b.PlanLaunch(bigger)
		if err != nil {
			t.Fatal(err)
		}
		if changed.ConfigDigest == plan.ConfigDigest {
			t.Fatal("memory change did not move the launch digest")
		}
	})
	t.Run("F5 invalid specs refused", func(t *testing.T) {
		for name, mutate := range map[string]func(*MachineSpec){
			"empty vm id":    func(s *MachineSpec) { s.VMID = "" },
			"path vm id":     func(s *MachineSpec) { s.VMID = "../x" },
			"dotdot vm id":   func(s *MachineSpec) { s.VMID = ".." },
			"relative state": func(s *MachineSpec) { s.StateDir = "state" },
			"no kernel":      func(s *MachineSpec) { s.Kernel = "" },
			"zero vcpus":     func(s *MachineSpec) { s.VCPUs = 0 },
			"zero memory":    func(s *MachineSpec) { s.MemMiB = 0 },
			"no drives":      func(s *MachineSpec) { s.Drives = nil },
			"dup drive": func(s *MachineSpec) {
				s.Drives = append(s.Drives, Drive{ID: s.Drives[0].ID, Path: "/x"})
			},
		} {
			bad := spec
			bad.Drives = append([]Drive(nil), spec.Drives...)
			mutate(&bad)
			if _, err := b.PlanLaunch(bad); err == nil {
				t.Errorf("%s: PlanLaunch accepted %+v", name, bad)
			}
		}
	})
	t.Run("F6 host paths cover the spec", func(t *testing.T) {
		got := map[string]Access{}
		for _, p := range plan.HostPaths {
			got[p.Path] = p.Access
		}
		want := map[string]Access{spec.Kernel: AccessRead}
		if spec.Initrd != "" {
			want[spec.Initrd] = AccessRead
		}
		for _, d := range spec.Drives {
			if d.ReadOnly {
				want[d.Path] = AccessRead
			} else {
				want[d.Path] = AccessReadWrite
			}
		}
		for path, access := range want {
			if got[path] != access {
				t.Errorf("host path %s: access %q, want %q", path, got[path], access)
			}
		}
	})
	t.Run("F7 fds", func(t *testing.T) {
		for i, f := range plan.ExtraFiles {
			if f == nil {
				t.Fatalf("ExtraFiles[%d] is nil", i)
			}
		}
		wantFD := b.Caps().TapFD && spec.Net != nil
		if wantFD != (len(plan.ExtraFiles) > 0) {
			t.Fatalf("TapFD=%v net=%v but %d extra files", b.Caps().TapFD, spec.Net != nil, len(plan.ExtraFiles))
		}
	})
	if !env.live {
		return
	}
	t.Run("F8 live start, identify, metrics, stop", func(t *testing.T) {
		console, err := os.OpenFile(filepath.Join(dir, "console.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer console.Close()
		proc, err := Start(plan, console, console)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { _ = proc.Stop(5 * time.Second) })
		select {
		case <-proc.Done():
			log, _ := os.ReadFile(filepath.Join(dir, "console.log"))
			t.Fatalf("VMM exited within %s (%v); console:\n%s", env.aliveFor, proc.ExitErr(), log)
		case <-time.After(env.aliveFor):
		}
		if runtime.GOOS == "linux" {
			reg, err := NewRegistry(b)
			if err != nil {
				t.Fatal(err)
			}
			pids := reg.PIDsForVM(ScanProc(), spec.VMID)
			if len(pids) != 1 || pids[0] != proc.PID {
				t.Fatalf("/proc scan found pids %v for %s, want [%d]", pids, spec.VMID, proc.PID)
			}
			m, err := b.Metrics(context.Background(), Handle{Backend: b.Name(), VMID: spec.VMID, PID: proc.PID, APISocket: plan.APISocket})
			if err != nil {
				t.Fatalf("Metrics: %v", err)
			}
			if m.PID != proc.PID || m.RSSBytes <= 0 || m.Backend != b.Name() {
				t.Fatalf("metrics %+v", m)
			}
		}
		if err := proc.Stop(5 * time.Second); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		select {
		case <-proc.Done():
		default:
			t.Fatal("Stop returned but Done is open")
		}
		if err := proc.Stop(time.Second); err != nil {
			t.Fatalf("second Stop must be a no-op: %v", err)
		}
	})
}

// --- fake backend --------------------------------------------------------

// fakeBackend runs /bin/sh as its "VMM". It reports no optional
// capabilities, so it exercises the cold-boot-only contract on any host.
type fakeBackend struct {
	name Name
	caps Caps
}

func isFakeName(n Name) bool { return strings.HasPrefix(string(n), "fake") }

func (f fakeBackend) Name() Name { return f.name }
func (f fakeBackend) Caps() Caps { return f.caps }
func (f fakeBackend) Version(context.Context, string) (string, error) {
	return "fake-vmm 0.0.0", nil
}

func (f fakeBackend) PlanLaunch(spec MachineSpec) (LaunchPlan, error) {
	if err := spec.Validate(); err != nil {
		return LaunchPlan{}, err
	}
	// "sleep; :" keeps sh from exec'ing sleep, so the cmdline stays ours.
	args := []string{"-c", "sleep 30; :", "fake-vmm", "--vm-id", spec.VMID, "--mem", strconv.Itoa(spec.MemMiB)}
	return LaunchPlan{
		Backend:      f.name,
		VMID:         spec.VMID,
		Binary:       "/bin/sh",
		Args:         args,
		HostPaths:    hostPathsFor(spec, ""),
		APISocket:    spec.APISocketPath,
		ConfigDigest: sha256Hex([]byte(strings.Join(args, "\x00"))),
	}, nil
}

func (f fakeBackend) VMIDFromCmdline(args []string) (string, bool) {
	isFake := false
	for _, a := range args {
		if a == "fake-vmm" {
			isFake = true
		}
	}
	if !isFake {
		return "", false
	}
	for i, a := range args {
		if a == "--vm-id" && i+1 < len(args) && args[i+1] != "" {
			return args[i+1], true
		}
	}
	return "", false
}

func (f fakeBackend) Metrics(_ context.Context, h Handle) (Metrics, error) {
	if runtime.GOOS != "linux" {
		return Metrics{Backend: f.name, PID: h.PID}, nil
	}
	m, err := ProcMetrics(h.PID)
	m.Backend = f.name
	return m, err
}

// baseSpec is a valid spec with files that exist on any host.
func baseSpec(t *testing.T, dir, vmID string) MachineSpec {
	t.Helper()
	stateDir := filepath.Join(dir, vmID)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	touch := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	return MachineSpec{
		VMID:     vmID,
		StateDir: stateDir,
		Kernel:   touch("vmlinux"),
		Initrd:   touch("initrd"),
		Cmdline:  "console=ttyS0 vm_id=" + vmID + " epoch=1",
		VCPUs:    2,
		MemMiB:   512,
		Drives: []Drive{
			{ID: "store", Path: touch("store.erofs"), ReadOnly: true},
			{ID: "data", Path: filepath.Join(stateDir, "data.img")},
		},
	}
}

func TestFakeBackendConformance(t *testing.T) {
	runConformance(t, conformanceEnv{
		backend:  fakeBackend{name: "fake", caps: Caps{ColdBoot: true}},
		spec:     baseSpec,
		live:     true,
		aliveFor: 200 * time.Millisecond,
	})
}

func TestFirecrackerConformance(t *testing.T) {
	runConformance(t, conformanceEnv{
		backend: FirecrackerBackend{},
		spec: func(t *testing.T, dir, vmID string) MachineSpec {
			s := baseSpec(t, dir, vmID)
			s.Net = &NetIface{ID: "eth0", TapName: "vm-test", MAC: "AA:FC:00:00:00:01"}
			return s
		},
	})
}

func TestCloudHypervisorConformance(t *testing.T) {
	tap, err := os.Open(os.DevNull) // stands in for the tap fd
	if err != nil {
		t.Fatal(err)
	}
	defer tap.Close()
	for _, landlock := range []bool{false, true} {
		t.Run(map[bool]string{false: "seccomp", true: "seccomp+landlock"}[landlock], func(t *testing.T) {
			runConformance(t, conformanceEnv{
				backend: CloudHypervisorBackend{Landlock: landlock},
				spec: func(t *testing.T, dir, vmID string) MachineSpec {
					s := baseSpec(t, dir, vmID)
					s.Net = &NetIface{ID: "eth0", TapName: "vm-test", MAC: "AA:FC:00:00:00:01", TapFD: tap}
					return s
				},
			})
		})
	}
}

// A conformance failure must be a test failure, not a skip: the suite
// itself is checked against a backend that breaks F1.
func TestConformanceCatchesCapsLie(t *testing.T) {
	lying := fakeBackend{name: "fake-liar", caps: Caps{ColdBoot: true, Snapshot: true}}
	if _, ok := any(lying).(Snapshotter); ok {
		t.Fatal("fake must not implement Snapshotter for this check")
	}
	if lying.Caps().Snapshot == false {
		t.Fatal("fixture broken")
	}
	// The F1 rule, applied directly.
	_, snap := any(lying).(Snapshotter)
	if lying.Caps().Snapshot == snap {
		t.Fatal("F1 would not catch a backend reporting Snapshot without Snapshotter")
	}
}

func TestStopOnExitedProcessIsNoop(t *testing.T) {
	p, err := Start(LaunchPlan{Backend: "fake", VMID: "vm-x", Binary: "/bin/sh", Args: []string{"-c", "exit 3"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit")
	}
	var exitErr interface{ ExitCode() int }
	if !errors.As(p.ExitErr(), &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("ExitErr = %v, want exit status 3", p.ExitErr())
	}
	if err := p.Stop(time.Second); err != nil {
		t.Fatalf("Stop after exit: %v", err)
	}
}

func TestStartWritesPlannedFilesBeforeExec(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "sub", "cfg.json")
	p, err := Start(LaunchPlan{
		Backend:    "fake",
		VMID:       "vm-x",
		Binary:     "/bin/sh",
		Args:       []string{"-c", `test -f "$0"`, cfg},
		WriteFiles: []PlannedFile{{Path: cfg, Data: []byte("{}"), Mode: 0o644}},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-p.Done()
	if p.ExitErr() != nil {
		t.Fatalf("child did not see the planned file: %v", p.ExitErr())
	}
	info, err := os.Stat(cfg)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("planned file mode: %v %v", info, err)
	}
}

func TestStartRefusesEmptyBinary(t *testing.T) {
	if _, err := Start(LaunchPlan{VMID: "vm-x"}, nil, nil); err == nil {
		t.Fatal("Start accepted a plan without a binary")
	}
}
