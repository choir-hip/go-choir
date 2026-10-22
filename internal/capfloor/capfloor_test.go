package capfloor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The daemon's capability floor (SDC SEC-15, first step; failure modes K1-K6
// in docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md, "Second
// batch").

func TestModeFromEnv(t *testing.T) {
	cases := map[string]Mode{
		"":          ModeOff,
		"off":       ModeOff,
		"observe":   ModeObserve,
		" Observe ": ModeObserve,
		"enforce":   ModeEnforce,
		// A typo strengthens (K4's sibling): enforce, never off.
		"enforced": ModeEnforce,
	}
	for raw, want := range cases {
		if got := ModeFromEnv(func(k string) string {
			if k != ModeEnv {
				t.Fatalf("read %s", k)
			}
			return raw
		}); got != want {
			t.Errorf("%q: %q, want %q", raw, got, want)
		}
	}
}

// K1: every kept capability names the code that needs it, and the set is the
// one the goal file lists; a capability added or removed here is a reviewed
// change.
func TestKeptCapabilitiesAreTheInventory(t *testing.T) {
	want := []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_DAC_READ_SEARCH", "CAP_FOWNER", "CAP_FSETID", "CAP_KILL",
		"CAP_SETGID", "CAP_SETUID", "CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_NET_ADMIN", "CAP_SYS_CHROOT",
		"CAP_SYS_PTRACE", "CAP_SYS_ADMIN", "CAP_SYS_RESOURCE", "CAP_SYS_TIME"}
	if len(Kept) != len(want) {
		t.Fatalf("kept %d capabilities, want %d", len(Kept), len(want))
	}
	got := map[string]bool{}
	for c, reason := range Kept {
		got[Name(c)] = true
		if len(strings.TrimSpace(reason)) < 20 {
			t.Errorf("%s is kept without a reason", Name(c))
		}
	}
	for _, n := range want {
		if !got[n] {
			t.Errorf("%s is not kept", n)
		}
	}
	// The capabilities a compromised daemon reaches for first are dropped.
	for _, c := range []int{capSysModule, capSysRawio, capSysBoot, capMknod, capNetRaw, capBPF, capPerfmon} {
		if _, kept := Kept[c]; kept {
			t.Errorf("%s is kept", Name(c))
		}
	}
}

func TestPlan(t *testing.T) {
	full := uint64(1)<<(capLastCap+1) - 1
	extra, missing := Plan(full)
	if len(missing) != 0 {
		t.Fatalf("a full set misses %v", missing)
	}
	if len(extra) != capLastCap+1-len(Kept) {
		t.Fatalf("extra %d, want %d", len(extra), capLastCap+1-len(Kept))
	}
	var keptMask uint64
	for c := range Kept {
		keptMask |= 1 << c
	}
	if extra, missing := Plan(keptMask); len(extra) != 0 || len(missing) != 0 {
		t.Fatalf("the floor itself: extra %v missing %v", extra, missing)
	}
	// K1's signal: a kept capability absent from the set (something else
	// already took it away) is reported, not added back.
	if _, missing := Plan(keptMask &^ (1 << capSysAdmin)); len(missing) != 1 || missing[0] != capSysAdmin {
		t.Fatalf("missing %v, want CAP_SYS_ADMIN", missing)
	}
}

func TestParseCapBnd(t *testing.T) {
	status := "Name:\tautoputer\nCapInh:\t0000000000000000\nCapBnd:\t000001ffffffffff\nNoNewPrivs:\t0\n"
	got, err := parseCapBnd(status)
	if err != nil || got != 0x1ffffffffff {
		t.Fatalf("CapBnd %x %v", got, err)
	}
	if _, err := parseCapBnd("Name:\tx\n"); err == nil {
		t.Fatal("a status with no CapBnd parsed")
	}
}

// The image ships observe: the floor logs what it would drop and the
// isolation self-test reports the daemon's set; enforce comes after a test
// computer runs a capsule and every desk under it.
func TestGuestImageShipsObserve(t *testing.T) {
	raw, err := os.ReadFile("../../nix/autoputer-vm.nix")
	if err != nil {
		t.Skipf("nix/autoputer-vm.nix not readable from the package: %v", err)
	}
	if !strings.Contains(string(raw), `CHOIR_RUNTIME_CAPABILITY_FLOOR = "observe";`) {
		t.Fatal(`the guest image does not set CHOIR_RUNTIME_CAPABILITY_FLOOR = "observe"`)
	}
}

// The staging probe judges the daemon's set against a floor it writes out
// itself; it must be this one.
func TestStagingProbeHoldsTheFloor(t *testing.T) {
	raw, err := os.ReadFile("../../scripts/desk_cell_confinement_probe.py")
	if err != nil {
		t.Skipf("probe script not readable: %v", err)
	}
	if want := `CAPFLOOR = "` + Hex(keptMask()) + `"`; !strings.Contains(string(raw), want) {
		t.Fatalf("the probe script lacks %s", want)
	}
}

// privilegedCalls are the wrappers of syscall and golang.org/x/sys/unix that
// need a capability, with the capability each needs (capabilities(7)). Where
// it depends on the arguments, the strongest is named: IoctlIfreq sets
// interface flags (SIOCSIF*, CAP_NET_ADMIN); a setxattr outside user.* needs
// CAP_SYS_ADMIN.
var privilegedCalls = map[string]int{
	"Settimeofday": capSysTime, "ClockSettime": capSysTime, "Adjtimex": capSysTime, "ClockAdjtime": capSysTime,
	"Mount": capSysAdmin, "Unmount": capSysAdmin, "Setns": capSysAdmin, "PivotRoot": capSysAdmin,
	"Sethostname": capSysAdmin, "Setdomainname": capSysAdmin, "Fsopen": capSysAdmin, "Fsmount": capSysAdmin,
	"MoveMount": capSysAdmin, "OpenTree": capSysAdmin,
	"Setxattr": capSysAdmin, "Lsetxattr": capSysAdmin, "Fsetxattr": capSysAdmin,
	"Chroot": capSysChroot, "Mknod": capMknod, "Mknodat": capMknod,
	"Reboot": capSysBoot, "Iopl": capSysRawio, "Ioperm": capSysRawio,
	"InitModule": capSysModule, "FinitModule": capSysModule, "DeleteModule": capSysModule,
	"Klogctl": capSyslog, "Acct": capSysPacct,
	"IoctlIfreq": capNetAdmin,
	"Setuid":     capSetuid, "Setreuid": capSetuid, "Setresuid": capSetuid, "Setfsuid": capSetuid,
	"Setgid": capSetgid, "Setregid": capSetgid, "Setresgid": capSetgid, "Setfsgid": capSetgid, "Setgroups": capSetgid,
}

// privilegedSyscallNumbers are the same needs as raw syscall numbers, when
// one is the first argument of Syscall, Syscall6, RawSyscall or RawSyscall6.
var privilegedSyscallNumbers = map[string]int{
	"SYS_SETTIMEOFDAY": capSysTime, "SYS_CLOCK_SETTIME": capSysTime, "SYS_CLOCK_ADJTIME": capSysTime, "SYS_ADJTIMEX": capSysTime,
	"SYS_MOUNT": capSysAdmin, "SYS_UMOUNT2": capSysAdmin, "SYS_PIVOT_ROOT": capSysAdmin, "SYS_SETNS": capSysAdmin,
	"SYS_SETHOSTNAME": capSysAdmin, "SYS_SETDOMAINNAME": capSysAdmin, "SYS_FSOPEN": capSysAdmin, "SYS_FSMOUNT": capSysAdmin,
	"SYS_MOVE_MOUNT": capSysAdmin, "SYS_OPEN_TREE": capSysAdmin,
	"SYS_SETXATTR": capSysAdmin, "SYS_LSETXATTR": capSysAdmin, "SYS_FSETXATTR": capSysAdmin,
	"SYS_CHROOT": capSysChroot, "SYS_MKNOD": capMknod, "SYS_MKNODAT": capMknod,
	"SYS_REBOOT": capSysBoot, "SYS_KEXEC_LOAD": capSysBoot, "SYS_KEXEC_FILE_LOAD": capSysBoot,
	"SYS_IOPL": capSysRawio, "SYS_IOPERM": capSysRawio,
	"SYS_INIT_MODULE": capSysModule, "SYS_FINIT_MODULE": capSysModule, "SYS_DELETE_MODULE": capSysModule,
	"SYS_SYSLOG": capSyslog, "SYS_ACCT": capSysPacct, "SYS_BPF": capBPF, "SYS_PERF_EVENT_OPEN": capPerfmon,
	"SYS_SETUID": capSetuid, "SYS_SETREUID": capSetuid, "SYS_SETRESUID": capSetuid, "SYS_SETFSUID": capSetuid,
	"SYS_SETGID": capSetgid, "SYS_SETREGID": capSetgid, "SYS_SETRESGID": capSetgid, "SYS_SETFSGID": capSetgid, "SYS_SETGROUPS": capSetgid,
}

var rawSyscallFuncs = map[string]bool{"Syscall": true, "Syscall6": true, "RawSyscall": true, "RawSyscall6": true}

// probeFiles call privileged syscalls to prove they are denied; the floor
// must not grant them.
var probeFiles = map[string]string{
	"internal/doctool/probe_linux.go":                  "the document-tool sandbox's probe expects every call denied",
	"internal/yaegikernel/desk_cell_selftest_linux.go": "the desk cell self-test expects every call denied",
}

// privilegedSite is one call in the daemon's source that needs a capability.
type privilegedSite struct {
	file, call string
	line, cap  int
}

// scanPrivilegedCalls parses Go source (so comments are not code and import
// aliases resolve) and returns every named privileged call into syscall or
// golang.org/x/sys/unix. A dot import of either is an error: its calls
// could not be seen.
func scanPrivilegedCalls(name string, src []byte) ([]privilegedSite, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	pkgs := map[string]bool{}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if path != "syscall" && path != "golang.org/x/sys/unix" {
			continue
		}
		local := filepath.Base(path)
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local == "." {
			return nil, fmt.Errorf("%s dot-imports %s", name, path)
		}
		if local != "_" {
			pkgs[local] = true
		}
	}
	selector := func(e ast.Expr) (string, bool) {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok || !pkgs[x.Name] {
			return "", false
		}
		return sel.Sel.Name, true
	}
	var sites []privilegedSite
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := selector(call.Fun)
		if !ok {
			return true
		}
		if c, ok := privilegedCalls[fn]; ok {
			sites = append(sites, privilegedSite{name, fn, fset.Position(call.Pos()).Line, c})
		}
		if rawSyscallFuncs[fn] && len(call.Args) > 0 {
			if num, ok := selector(call.Args[0]); ok {
				if c, ok := privilegedSyscallNumbers[num]; ok {
					sites = append(sites, privilegedSite{name, fn + "(" + num + ")", fset.Position(call.Pos()).Line, c})
				}
			}
		}
		return true
	})
	return sites, nil
}

// F4: the scan sees calls, not text: a comment neither trips nor satisfies
// it, an aliased import and a raw syscall number are seen, a dot import is
// refused.
func TestScanPrivilegedCallsReadsCode(t *testing.T) {
	src := `package p

import (
	sys "golang.org/x/sys/unix"
	"syscall"
)

// unix.Settimeofday(nil) and syscall.Mount("", "", "", 0, "") in a comment.
func f() {
	_ = sys.IoctlIfreq(0, 0, nil)
	_, _, _ = syscall.Syscall(syscall.SYS_SETTIMEOFDAY, 0, 0, 0)
	_ = sys.Fsetxattr(0, "user.x", nil, 0)
	s := "unix.Mount("
	_ = s
}
`
	sites, err := scanPrivilegedCalls("p.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range sites {
		got = append(got, s.call)
	}
	if strings.Join(got, ",") != "IoctlIfreq,Syscall(SYS_SETTIMEOFDAY),Fsetxattr" {
		t.Fatalf("sites %v", got)
	}
	if _, err := scanPrivilegedCalls("d.go", []byte("package p\nimport . \"golang.org/x/sys/unix\"\nfunc f() { _ = Getpid() }\n")); err == nil {
		t.Fatal("a dot import of x/sys/unix was accepted")
	}
}

// K1, review E1 and F4: every named capability-needing call the guest daemon
// makes on its own paths (and the capsule broker it starts, which can hold no
// more) is under a kept capability. The hibernate resume steps the clock with
// settimeofday; without CAP_SYS_TIME enforce answers it 500 and the host
// cold-boots the guest. Calls this list does not name, and binaries the
// daemon execs, are the enforce trial's to find.
func TestKeptCoversTheDaemonsPrivilegedSyscalls(t *testing.T) {
	root := "../.."
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Skipf("repository root not readable: %v", err)
	}
	seen := map[string]bool{}
	for _, dir := range []string{"internal", "cmd/autoputer", "cmd/capsule-broker"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if _, probe := probeFiles[rel]; probe {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sites, err := scanPrivilegedCalls(rel, raw)
			if err != nil {
				return err
			}
			for _, s := range sites {
				seen[s.call] = true
				if _, kept := Kept[s.cap]; !kept {
					t.Errorf("%s:%d calls %s, which needs %s; the floor drops it", s.file, s.line, s.call, Name(s.cap))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// The scan sees the calls it is meant to see (calls, never comments).
	for _, call := range []string{"Settimeofday", "Mount", "IoctlIfreq", "Fsetxattr"} {
		if !seen[call] {
			t.Errorf("the scan found no %s call; it no longer reads the daemon's source", call)
		}
	}
}

// Review E8: a Mode the floor does not know is refused, never a silent no-op
// (ModeFromEnv maps unknown text to enforce; a caller passing a raw Mode
// must not get off).
func TestApplyRefusesAnUnknownMode(t *testing.T) {
	if err := Apply(Mode("enforced")); err == nil {
		t.Fatal("Apply accepted an unknown mode")
	}
	if err := Apply(ModeOff); err != nil {
		t.Fatalf("off: %v", err)
	}
}
