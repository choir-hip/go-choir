package hostboot

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The failure modes these fixtures cover are listed in
// docs/archive/reports/patch-lanes-plan-2026-10-15.md, slice 1.

type system struct {
	name     string
	kernel   string // store dir name of the kernel package
	modules  string // store dir name of the modules tree
	params   string // kernel-params file content
	modprobe string // etc/modprobe.d/nixos.conf content ("" = no file)
}

type fixture struct {
	t    *testing.T
	root string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, root: root}
}

func (f *fixture) write(rel, content string) {
	f.t.Helper()
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) link(rel, target string) {
	f.t.Helper()
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Symlink(target, p); err != nil {
		f.t.Fatal(err)
	}
}

// system builds a NixOS system closure in the fixture store and returns its
// absolute path. The kernel and modules links are absolute, as on a host.
func (f *fixture) system(s system) string {
	f.t.Helper()
	store := filepath.Join(f.root, "nix/store")
	dir := filepath.Join(store, s.name)
	f.write("nix/store/"+s.kernel+"/bzImage", s.kernel)
	f.write("nix/store/"+s.modules+"/lib/modules/.keep", "")
	f.link("nix/store/"+s.name+"/kernel", filepath.Join(store, s.kernel, "bzImage"))
	f.link("nix/store/"+s.name+"/kernel-modules", filepath.Join(store, s.modules))
	f.write("nix/store/"+s.name+"/kernel-params", s.params)
	if s.modprobe != "" {
		f.write("nix/store/"+s.name+"/etc/modprobe.d/nixos.conf", s.modprobe)
	}
	return dir
}

func (f *fixture) generation(n int, target string, mtime time.Time) {
	f.t.Helper()
	rel := filepath.Join("nix/var/nix/profiles", "system-"+itoa(n)+"-link")
	f.link(rel, target)
	setLinkMtime(f.t, filepath.Join(f.root, rel), mtime)
}

func (f *fixture) proc(release, cmdline string, btime int64) {
	f.write("proc/sys/kernel/osrelease", release+"\n")
	f.write("proc/cmdline", cmdline+"\n")
	f.write("proc/stat", "cpu  1 2 3\nbtime "+itoa64(btime)+"\nprocesses 9\n")
}

func (f *fixture) live(module, param, value string) {
	f.write("sys/module/"+module+"/parameters/"+param, value+"\n")
}

var (
	oldSys = system{name: "aaa-nixos-system-old", kernel: "k1-linux-6.18.16", modules: "m1-linux-6.18.16-modules", params: "loglevel=4"}
	newSys = system{name: "bbb-nixos-system-new", kernel: "k2-linux-6.18.55", modules: "m2-linux-6.18.55-modules",
		params: "loglevel=4  kvm.halt_poll_ns=0\n", modprobe: "options kvm_intel nested=0\n"}
	t0  = time.Date(2026, 10, 14, 23, 50, 0, 0, time.UTC)
	now = t0.Add(2 * time.Hour)
)

// nodeBToday: booted 6.18.16 with nothing declared; 6.18.55, halt_poll_ns=0
// and nested=0 activated but not booted.
func nodeBToday(t *testing.T) *fixture {
	f := newFixture(t)
	old, cur := f.system(oldSys), f.system(newSys)
	f.link("run/booted-system", old)
	f.link("run/current-system", cur)
	f.generation(41, old, t0.Add(-216*24*time.Hour))
	f.generation(42, cur, t0)
	f.proc("6.18.16", "BOOT_IMAGE=(hd0,gpt2)//kernels/k1-bzImage init=/nix/store/aaa/init loglevel=4", t0.Add(-216*24*time.Hour).Unix())
	f.live("kvm_intel", "nested", "Y")
	f.live("kvm", "halt_poll_ns", "200000")
	return f
}

func TestNodeBTodayIsPending(t *testing.T) {
	f := nodeBToday(t)
	st := Read(f.root, now)
	if !st.Available || !st.RebootPending {
		t.Fatalf("want available and pending, got %+v", st)
	}
	wantReasons := []string{ReasonKernel, ReasonKernelModules, ReasonKernelParams, ReasonModuleParam}
	if !reflect.DeepEqual(st.Reasons, wantReasons) {
		t.Fatalf("reasons = %v, want %v", st.Reasons, wantReasons)
	}
	if !reflect.DeepEqual(st.ParamsNotOnCmdline, []string{"kvm.halt_poll_ns=0"}) {
		t.Fatalf("params not on cmdline = %v", st.ParamsNotOnCmdline)
	}
	want := []ModuleParam{
		{Module: "kvm_intel", Param: "nested", Declared: "0", Source: "modprobe.d", Live: "Y", Loaded: true, Match: false},
		{Module: "kvm", Param: "halt_poll_ns", Declared: "0", Source: "kernel-params", Live: "200000", Loaded: true, Match: false},
	}
	if !reflect.DeepEqual(st.ModuleParams, want) {
		t.Fatalf("module params = %+v", st.ModuleParams)
	}
	if st.RunningKernelRelease != "6.18.16" || st.PendingSince != t0.Format(time.RFC3339) || st.PendingOverdue {
		t.Fatalf("release/since/overdue = %q %q %v", st.RunningKernelRelease, st.PendingSince, st.PendingOverdue)
	}
	if st.BootedKernel == st.CurrentKernel || st.BootedKernel == "" {
		t.Fatalf("kernels not resolved: %q %q", st.BootedKernel, st.CurrentKernel)
	}
	if got := Read(f.root, t0.Add(8*24*time.Hour)); !got.PendingOverdue {
		t.Fatal("a reboot pending for 8 days must be overdue")
	}
}

// After the reboot: same system booted and current, command line in another
// order with bootloader tokens, booleans read back as N.
func TestAfterRebootIsClean(t *testing.T) {
	f := newFixture(t)
	cur := f.system(newSys)
	f.link("run/booted-system", cur)
	f.link("run/current-system", cur)
	f.generation(42, cur, t0)
	f.proc("6.18.55", "BOOT_IMAGE=(hd0,gpt2)//kernels/k2-bzImage init=/nix/store/bbb/init kvm.halt_poll_ns=0 loglevel=4", now.Unix())
	f.live("kvm_intel", "nested", "N")
	f.live("kvm", "halt_poll_ns", "0")
	st := Read(f.root, now)
	if !st.Available || st.RebootPending || len(st.Reasons) != 0 || st.PendingSince != "" {
		t.Fatalf("want clean, got %+v", st)
	}
}

// A modprobe-only change (boot.extraModprobeConfig) is invisible to the
// kernel paths; dashed module names normalize; relative links resolve; a
// module that is not loaded does not count; the booted generation was
// garbage-collected.
func TestModprobeOnlyChange(t *testing.T) {
	f := newFixture(t)
	booted := oldSys
	cur := oldSys
	cur.name = "ccc-nixos-system-modprobe"
	cur.modprobe = "options kvm-intel nested=0\noptions kvm_amd nested=0\n"
	b, c := f.system(booted), f.system(cur)
	f.link("run/booted-system", "../nix/store/"+filepath.Base(b))
	f.link("run/current-system", "../nix/store/"+filepath.Base(c))
	f.generation(7, c, t0)
	f.proc("6.18.16", "loglevel=4", t0.Unix())
	f.live("kvm_intel", "nested", "Y")
	st := Read(f.root, now)
	if !reflect.DeepEqual(st.Reasons, []string{ReasonModuleParam}) {
		t.Fatalf("reasons = %v, want [module_param]", st.Reasons)
	}
	if len(st.ModuleParams) != 2 || st.ModuleParams[0].Module != "kvm_intel" || st.ModuleParams[0].Match {
		t.Fatalf("kvm-intel must normalize and mismatch: %+v", st.ModuleParams)
	}
	if amd := st.ModuleParams[1]; amd.Module != "kvm_amd" || amd.Loaded || !amd.Match {
		t.Fatalf("an unloaded module must not count: %+v", amd)
	}
	if st.PendingSince != t0.Format(time.RFC3339) {
		t.Fatalf("pending since = %q", st.PendingSince)
	}
}

func TestModulesOnlyChange(t *testing.T) {
	f := newFixture(t)
	cur := oldSys
	cur.name = "ddd-nixos-system-modules"
	cur.modules = "m9-linux-6.18.16-modules-rebuilt"
	b, c := f.system(oldSys), f.system(cur)
	f.link("run/booted-system", b)
	f.link("run/current-system", c)
	f.proc("6.18.16", "loglevel=4", t0.Unix())
	st := Read(f.root, now)
	if !reflect.DeepEqual(st.Reasons, []string{ReasonKernelModules}) {
		t.Fatalf("reasons = %v, want [kernel_modules]", st.Reasons)
	}
	if st.PendingSince != "" {
		t.Fatalf("no profile generations: pending_since must be omitted, got %q", st.PendingSince)
	}
}

func TestNotNixOS(t *testing.T) {
	f := newFixture(t)
	f.proc("6.8.0", "quiet", t0.Unix())
	st := Read(f.root, now)
	if st.Available || st.RebootPending || st.Error != "booted system unreadable" {
		t.Fatalf("a host without /run/booted-system must be unavailable, never 'not pending': %+v", st)
	}
}

// TestPythonParity runs scripts/verify-host-hardening.py --boot-state-json on
// every fixture and requires the same JSON as Read. The script is the
// deploy receipt's and the host check's implementation of this contract.
func TestPythonParity(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	_, self, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(self), "..", "..", "scripts", "verify-host-hardening.py")
	cases := map[string]func(*testing.T) *fixture{
		"node-b-today": nodeBToday,
		"not-nixos": func(t *testing.T) *fixture {
			f := newFixture(t)
			f.proc("6.8.0", "quiet", t0.Unix())
			return f
		},
		"modprobe-only": func(t *testing.T) *fixture {
			f := newFixture(t)
			cur := oldSys
			cur.name = "ccc-nixos-system-modprobe"
			cur.modprobe = "options kvm-intel nested=0\noptions kvm_amd nested=0\n"
			b, c := f.system(oldSys), f.system(cur)
			f.link("run/booted-system", "../nix/store/"+filepath.Base(b))
			f.link("run/current-system", "../nix/store/"+filepath.Base(c))
			f.generation(7, c, t0)
			f.proc("6.18.16", "loglevel=4", t0.Unix())
			f.live("kvm_intel", "nested", "Y")
			return f
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			f := build(t)
			for _, at := range []time.Time{now, t0.Add(8 * 24 * time.Hour)} {
				goJSON, err := json.Marshal(Read(f.root, at))
				if err != nil {
					t.Fatal(err)
				}
				out, err := exec.Command(py, "-I", script, "--boot-state-json", "--root", f.root, "--now", itoa64(at.Unix())).Output()
				if err != nil {
					t.Fatalf("python: %v", err)
				}
				var g, p map[string]any
				if err := json.Unmarshal(goJSON, &g); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(out, &p); err != nil {
					t.Fatalf("python output: %v\n%s", err, out)
				}
				if !reflect.DeepEqual(g, p) {
					t.Fatalf("Go and Python disagree at %s\ngo:     %s\npython: %s", at.Format(time.RFC3339), goJSON, out)
				}
			}
		})
	}
}

func itoa(n int) string     { return strconv.Itoa(n) }
func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// setLinkMtime sets a symlink's own mtime: the profile link's mtime is the
// generation's activation time.
func setLinkMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	tv := []unix.Timeval{unix.NsecToTimeval(mtime.UnixNano()), unix.NsecToTimeval(mtime.UnixNano())}
	if err := unix.Lutimes(path, tv); err != nil {
		t.Fatal(err)
	}
}
