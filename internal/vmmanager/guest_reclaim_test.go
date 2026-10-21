package vmmanager

import (
	"reflect"
	"strings"
	"testing"
)

// RES-06, P4 of the FP8 station: proactive reclaim of cold guest page cache
// through the guest kernel's built-in DAMON_RECLAIM, turned on per computer
// by boot arguments. Failure modes guarded:
//  1. reaching a computer nobody selected (an empty list selects none, and
//     an unselected launch's boot args are byte-identical to today's);
//  2. reclaiming hot pages (min_age is a floor; nothing younger is reclaimed);
//  3. DAMON deactivating exactly when the guest is full of cache (the
//     default low watermark, 20% free, would turn it off; ours is 0.5%);
//  4. paging out anonymous memory in a guest without swap (skip_anon);
//  5. a malformed age making a boot argument the kernel rejects.

func TestGuestReclaimBootArgsPerComputer(t *testing.T) {
	selected := balloonTestVMConfig()
	selected.WarmnessClass = "test"
	other := balloonTestVMConfig()
	other.VMID = "vm-other"

	base := balloonTestManager(t, nil)
	want := bootArgsOf(t, base.buildFirecrackerConfig(other, 9001))
	if strings.Contains(want, "damon_reclaim") {
		t.Fatalf("default boot args carry damon_reclaim: %s", want)
	}

	mgr := balloonTestManager(t, func(c *ManagerConfig) { c.GuestReclaimOnly = []string{"class:test"} })
	if got := bootArgsOf(t, mgr.buildFirecrackerConfig(other, 9001)); got != want {
		t.Fatalf("an unselected computer's boot args changed:\n got %s\nwant %s", got, want)
	}
	args := bootArgsOf(t, mgr.buildFirecrackerConfig(selected, 9001))
	for _, p := range []string{
		"damon_reclaim.min_age=300000000",
		"damon_reclaim.wmarks_high=950",
		"damon_reclaim.wmarks_mid=900",
		"damon_reclaim.wmarks_low=5",
		"damon_reclaim.skip_anon=Y",
		"damon_reclaim.enabled=Y",
	} {
		if !strings.Contains(" "+args+" ", " "+p+" ") {
			t.Fatalf("selected boot args lack %s: %s", p, args)
		}
	}
	if !strings.HasSuffix(args, " damon_reclaim.enabled=Y") {
		t.Fatalf("enabled must come last, after its parameters: %s", args)
	}

	aged := balloonTestManager(t, func(c *ManagerConfig) {
		c.GuestReclaimOnly = []string{"*"}
		c.GuestReclaimMinAgeS = 90
	})
	if a := bootArgsOf(t, aged.buildFirecrackerConfig(other, 9001)); !strings.Contains(a, "damon_reclaim.min_age=90000000 ") {
		t.Fatalf("min age override missing: %s", a)
	}
}

func TestGuestReclaimConfigFromEnv(t *testing.T) {
	t.Setenv("VM_GUEST_RECLAIM_ONLY", "")
	t.Setenv("VM_GUEST_RECLAIM_MIN_AGE_S", "")
	if cfg := LoadConfigFromEnv(); len(cfg.GuestReclaimOnly) != 0 || cfg.GuestReclaimMinAgeS != 0 {
		t.Fatalf("default must be off: %v %d", cfg.GuestReclaimOnly, cfg.GuestReclaimMinAgeS)
	}
	t.Setenv("VM_GUEST_RECLAIM_ONLY", "computer-1,class:test")
	t.Setenv("VM_GUEST_RECLAIM_MIN_AGE_S", "120")
	cfg := LoadConfigFromEnv()
	if !reflect.DeepEqual(cfg.GuestReclaimOnly, []string{"computer-1", "class:test"}) || cfg.GuestReclaimMinAgeS != 120 {
		t.Fatalf("env = %v %d", cfg.GuestReclaimOnly, cfg.GuestReclaimMinAgeS)
	}
	for _, bad := range []string{"-5", "0", "x", "100000"} {
		t.Setenv("VM_GUEST_RECLAIM_MIN_AGE_S", bad)
		if got := LoadConfigFromEnv().GuestReclaimMinAgeS; got != 0 {
			t.Fatalf("min age %q accepted as %d; want the default", bad, got)
		}
	}
}
