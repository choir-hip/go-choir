package capfloor

import (
	"io/fs"
	"os"
	"path/filepath"
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

// privilegedCalls are the syscalls that need a capability, as the daemon's
// Go source spells them, with the capability each needs (capabilities(7)).
var privilegedCalls = map[string]int{
	"Settimeofday(": capSysTime, "ClockSettime(": capSysTime, "Adjtimex(": capSysTime,
	"Mount(": capSysAdmin, "Unmount(": capSysAdmin, "Setns(": capSysAdmin, "PivotRoot(": capSysAdmin,
	"Sethostname(": capSysAdmin, "Chroot(": capSysChroot, "Mknod(": capMknod, "Mknodat(": capMknod,
	"Reboot(": capSysBoot, "Iopl(": capSysRawio, "Ioperm(": capSysRawio,
	"InitModule(": capSysModule, "FinitModule(": capSysModule, "DeleteModule(": capSysModule,
	"Klogctl(": capSyslog, "Acct(": capSysPacct,
}

// probeFiles call privileged syscalls to prove they are denied; the floor
// must not grant them.
var probeFiles = map[string]string{
	"internal/doctool/probe_linux.go":                  "the document-tool sandbox's probe expects every call denied",
	"internal/yaegikernel/desk_cell_selftest_linux.go": "the desk cell self-test expects every call denied",
}

// K1, review E1: every capability-needing syscall the guest daemon makes on
// its own paths (and the capsule broker it starts, which can hold no more)
// is under a kept capability. The hibernate resume steps the clock with
// settimeofday; without CAP_SYS_TIME enforce answers it 500 and the host
// cold-boots the guest.
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
			for _, line := range strings.Split(string(raw), "\n") {
				for call, c := range privilegedCalls {
					if !strings.Contains(line, "unix."+call) && !strings.Contains(line, "syscall."+call) {
						continue
					}
					seen[call] = true
					if _, kept := Kept[c]; !kept {
						t.Errorf("%s calls %s, which needs %s; the floor drops it", rel, strings.TrimSuffix(call, "("), Name(c))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// The scan sees the calls it is meant to see.
	for _, call := range []string{"Settimeofday(", "Mount("} {
		if !seen[call] {
			t.Errorf("the scan found no %s; it no longer reads the daemon's source", call)
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
