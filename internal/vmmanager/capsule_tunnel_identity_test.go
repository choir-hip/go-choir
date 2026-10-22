package vmmanager

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// SEC-06 (docs/problems/egress-proxy-tunnel-identity-2026-10-21.md): the
// capsule egress tunnel-identity check ships in observe on every computer and
// in enforce on the computers the host selects (owner rule 2026-10-22: a new
// enforcing rule ships in observe everywhere, plus enforce on one selected
// test computer in the same deploy). The selection is a kernel boot argument
// vmctl writes; the guest's boot script copies it into the daemon's
// environment file. Failure modes guarded:
//  1. reaching a computer nobody selected (an empty list selects none, and
//     an unselected launch's boot args are byte-identical to today's);
//  2. the selection missing from one of the two boot paths (microvm store
//     disk and legacy rootfs);
//  3. the boot argument carrying anything but enforce (the host only turns
//     enforce on; observe is the image's own default);
//  4. the node-b list naming every computer, which is the global flip and
//     belongs to the image line behind its own gate, not to this selector.

const tunnelEnforceArg = "choir.capsule_egress_tunnel_identity=enforce"

func TestCapsuleTunnelEnforceBootArgPerComputer(t *testing.T) {
	for _, microvm := range []bool{false, true} {
		selected := balloonTestVMConfig()
		selected.WarmnessClass = "test"
		selected.ComputerID = "computer-selected"
		other := balloonTestVMConfig()
		other.VMID = "vm-other"
		other.ComputerID = "computer-other"
		mutate := func(c *ManagerConfig) {}
		if microvm {
			selected.StoreDiskPath = "/opt/go-choir/guest/store.erofs"
			other.StoreDiskPath = "/opt/go-choir/guest/store.erofs"
			mutate = func(c *ManagerConfig) { c.KernelParams = "root=fstab init=/nix/store/init-a" }
		}

		base := balloonTestManager(t, mutate)
		want := bootArgsOf(t, base.buildFirecrackerConfig(other, 9001))
		if strings.Contains(want, "capsule_egress_tunnel_identity") {
			t.Fatalf("microvm=%t: default boot args carry the selection: %s", microvm, want)
		}
		if got := bootArgsOf(t, base.buildFirecrackerConfig(selected, 9001)); strings.Contains(got, "capsule_egress_tunnel_identity") {
			t.Fatalf("microvm=%t: an empty list selected a computer: %s", microvm, got)
		}

		for _, only := range [][]string{{"class:test"}, {"computer-selected"}} {
			mgr := balloonTestManager(t, func(c *ManagerConfig) {
				mutate(c)
				c.CapsuleTunnelEnforceOnly = only
			})
			if got := bootArgsOf(t, mgr.buildFirecrackerConfig(other, 9001)); got != want {
				t.Fatalf("microvm=%t %v: an unselected computer's boot args changed:\n got %s\nwant %s", microvm, only, got, want)
			}
			args := bootArgsOf(t, mgr.buildFirecrackerConfig(selected, 9001))
			if !strings.Contains(" "+args+" ", " "+tunnelEnforceArg+" ") {
				t.Fatalf("microvm=%t %v: selected boot args lack %s: %s", microvm, only, tunnelEnforceArg, args)
			}
			if n := strings.Count(args, "capsule_egress_tunnel_identity"); n != 1 {
				t.Fatalf("microvm=%t: selection appears %d times: %s", microvm, n, args)
			}
		}
	}
}

func TestCapsuleTunnelEnforceConfigFromEnv(t *testing.T) {
	t.Setenv("VM_CAPSULE_TUNNEL_ENFORCE_ONLY", "")
	if cfg := LoadConfigFromEnv(); len(cfg.CapsuleTunnelEnforceOnly) != 0 {
		t.Fatalf("default must select none: %v", cfg.CapsuleTunnelEnforceOnly)
	}
	t.Setenv("VM_CAPSULE_TUNNEL_ENFORCE_ONLY", "computer-1, class:test")
	if got := LoadConfigFromEnv().CapsuleTunnelEnforceOnly; !reflect.DeepEqual(got, []string{"computer-1", "class:test"}) {
		t.Fatalf("env = %v", got)
	}
}

// node-b names the selector (so a reader of the host config sees it) and
// never selects every computer with it.
func TestNodeBNamesTheTunnelEnforceSelector(t *testing.T) {
	raw, err := os.ReadFile("../../nix/node-b.nix")
	if err != nil {
		t.Skipf("nix/node-b.nix not readable from the package: %v", err)
	}
	m := regexp.MustCompile(`"VM_CAPSULE_TUNNEL_ENFORCE_ONLY=([^"]*)"`).FindAllStringSubmatch(string(raw), -1)
	if len(m) != 1 {
		t.Fatalf("node-b sets VM_CAPSULE_TUNNEL_ENFORCE_ONLY %d times, want once", len(m))
	}
	for _, tok := range parseComputerSelector(m[0][1]) {
		if tok == "*" {
			t.Fatal(`node-b selects every computer for enforce ("*"); the global flip is the guest image's line`)
		}
	}
}

// The load-bearing link: the boot argument reaches a named computer only
// because naming it changes the launch-config hash, so a snapshot taken
// under observe is refused and the computer cold boots into enforce. An
// empty list (nil or zero-length) and a list naming another computer leave
// the hash identical. This fails if expectedLaunchConfig stops rebuilding
// boot args from m.cfg: the observe snapshot would then resume in observe
// while vmctl's environment says enforce.
func TestCapsuleTunnelEnforceSelectionRefusesAnObserveSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name string
		only []string
		want string
	}{
		{"no selector", nil, ""},
		{"empty list", []string{}, ""},
		{"another computer", []string{"computer-other"}, ""},
		{"this computer id", []string{"computer-s3"}, snapshotReasonLaunchConfig},
		{"this vm id", []string{"vm-s3"}, snapshotReasonLaunchConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSnapshotFixture(t)
			f.serveLaunchAPI(t)
			if err := f.mgr.HibernateVM(f.vmID); err != nil {
				t.Fatal(err)
			}
			dir := f.mgr.snapshotDir(f.vmID)
			meta, err := readSnapshotMeta(dir)
			if err != nil {
				t.Fatal(err)
			}
			f.mgr.cfg.CapsuleTunnelEnforceOnly = tc.only
			_, expected := f.mgr.expectedLaunchConfig(meta, f.expect)
			hash, err := launchConfigHash(expected)
			if err != nil {
				t.Fatal(err)
			}
			named := strings.Contains(bootArgsOf(t, expected), tunnelEnforceArg)
			if same := hash == meta.LaunchConfigSHA256; same == (tc.want != "") || named != (tc.want != "") {
				t.Fatalf("hash unchanged = %t, enforce boot arg = %t; want the hash to move and the arg present only for a named computer", same, named)
			}
			v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: f.expect}
			if got := f.mgr.validateSnapshot(dir, meta, v); got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %q", got, tc.want)
			}
		})
	}
}
