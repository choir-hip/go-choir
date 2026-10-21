package capfloor

import (
	"os"
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
		"CAP_SYS_PTRACE", "CAP_SYS_ADMIN", "CAP_SYS_RESOURCE"}
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
	for _, c := range []int{capSysModule, capSysRawio, capSysBoot, capSysTime, capMknod, capNetRaw, capBPF, capPerfmon} {
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
