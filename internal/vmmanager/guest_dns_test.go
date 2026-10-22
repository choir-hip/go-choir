package vmmanager

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// SEC-10 (docs/problems/guest-dns-host-resolver-unused-2026-10-22.md): a
// computer the host selects boots with choir.dns=<its tap host address>, so
// the guest's network unit points its resolver at the host's Unbound (the
// DNAT on the tap host address :53, hostfirewall.go). Failure modes guarded:
//  1. reaching a computer nobody selected: VMCTL_GUEST_DNS_PORT is already
//     set on every staging launch, so the argument must need the selector
//     too; an unselected launch's fc-config is byte-identical to today's;
//  2. naming a resolver the host does not serve (port 0 or unset: no
//     DNAT, so no argument, even for a selected computer);
//  3. the wrong address (anything but this launch's tap host address, which
//     is where the DNAT listens);
//  4. the argument missing from one of the two boot paths (microvm store
//     disk and legacy rootfs), or appearing twice;
//  5. voiding the snapshot of a computer the change does not apply to, or
//     resuming a selected computer's pre-selection snapshot (its kernel
//     args are fixed at cold boot, so it would never get the resolver);
//  6. node-b selecting every computer ("*"), which is step 2's global flip.

func guestDNSArgs(args string) []string {
	var out []string
	for _, f := range strings.Fields(args) {
		if strings.HasPrefix(f, "choir.dns=") {
			out = append(out, f)
		}
	}
	return out
}

func TestGuestDNSBootArgPerComputer(t *testing.T) {
	for _, microvm := range []bool{false, true} {
		selected := balloonTestVMConfig()
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
		_, hostIP := base.guestAndHostIP(9001)
		wantArg := "choir.dns=" + hostIP
		wantOther := bootArgsOf(t, base.buildFirecrackerConfig(other, 9001))
		wantSelected := bootArgsOf(t, base.buildFirecrackerConfig(selected, 9001))
		if a := guestDNSArgs(wantOther); len(a) != 0 {
			t.Fatalf("microvm=%t: default boot args carry %v", microvm, a)
		}

		for _, tc := range []struct {
			name string
			port int
			only []string
			want bool
		}{
			{"port unset, named", 0, []string{"computer-selected"}, false},
			{"port set, empty list", 8053, nil, false},
			{"port set, another computer", 8053, []string{"computer-nobody"}, false},
			{"port set, named by computer id", 8053, []string{"computer-selected"}, true},
			{"port set, named by vm id", 8053, []string{"vm-balloon-1"}, true},
		} {
			mgr := balloonTestManager(t, func(c *ManagerConfig) {
				mutate(c)
				c.GuestDNSPort = tc.port
				c.GuestDNSOnly = tc.only
			})
			if got := bootArgsOf(t, mgr.buildFirecrackerConfig(other, 9001)); got != wantOther {
				t.Fatalf("microvm=%t %s: an unselected computer's boot args changed:\n got %s\nwant %s", microvm, tc.name, got, wantOther)
			}
			got := bootArgsOf(t, mgr.buildFirecrackerConfig(selected, 9001))
			if !tc.want {
				if got != wantSelected {
					t.Fatalf("microvm=%t %s: boot args changed:\n got %s\nwant %s", microvm, tc.name, got, wantSelected)
				}
				continue
			}
			if a := guestDNSArgs(got); !reflect.DeepEqual(a, []string{wantArg}) {
				t.Fatalf("microvm=%t %s: choir.dns args = %v, want exactly [%s]: %s", microvm, tc.name, a, wantArg, got)
			}
			// The argument is added, nothing else moves.
			if strings.Join(strings.Fields(strings.Replace(got, wantArg, "", 1)), " ") != strings.Join(strings.Fields(wantSelected), " ") {
				t.Fatalf("microvm=%t %s: more than choir.dns changed:\n got %s\nwant %s + %s", microvm, tc.name, got, wantSelected, wantArg)
			}
		}

		// The address follows the launch's tap: another host port, another
		// host address.
		mgr := balloonTestManager(t, func(c *ManagerConfig) {
			mutate(c)
			c.GuestDNSPort = 8053
			c.GuestDNSOnly = []string{"computer-selected"}
		})
		_, hostIP2 := mgr.guestAndHostIP(9002)
		if hostIP2 == hostIP {
			t.Fatalf("test needs two distinct tap addresses, got %s twice", hostIP)
		}
		if a := guestDNSArgs(bootArgsOf(t, mgr.buildFirecrackerConfig(selected, 9002))); !reflect.DeepEqual(a, []string{"choir.dns=" + hostIP2}) {
			t.Fatalf("microvm=%t: host port 9002 gave %v, want choir.dns=%s", microvm, a, hostIP2)
		}
	}
}

func TestGuestDNSConfigFromEnv(t *testing.T) {
	t.Setenv("VMCTL_GUEST_DNS_PORT", "")
	t.Setenv("VMCTL_GUEST_DNS_ONLY", "")
	if cfg := LoadConfigFromEnv(); cfg.GuestDNSPort != 0 || len(cfg.GuestDNSOnly) != 0 {
		t.Fatalf("default must be off: port %d, only %v", cfg.GuestDNSPort, cfg.GuestDNSOnly)
	}
	t.Setenv("VMCTL_GUEST_DNS_PORT", "8053")
	t.Setenv("VMCTL_GUEST_DNS_ONLY", "computer-1, vm-2")
	cfg := LoadConfigFromEnv()
	if cfg.GuestDNSPort != 8053 || !reflect.DeepEqual(cfg.GuestDNSOnly, []string{"computer-1", "vm-2"}) {
		t.Fatalf("env = port %d, only %v", cfg.GuestDNSPort, cfg.GuestDNSOnly)
	}
}

// node-b names the selector once (so a reader of the host config sees it)
// and never selects every computer with it.
func TestNodeBNamesTheGuestDNSSelector(t *testing.T) {
	raw, err := os.ReadFile("../../nix/node-b.nix")
	if err != nil {
		t.Skipf("nix/node-b.nix not readable from the package: %v", err)
	}
	m := regexp.MustCompile(`"VMCTL_GUEST_DNS_ONLY=([^"]*)"`).FindAllStringSubmatch(string(raw), -1)
	if len(m) != 1 {
		t.Fatalf("node-b sets VMCTL_GUEST_DNS_ONLY %d times, want once", len(m))
	}
	for _, tok := range parseComputerSelector(m[0][1]) {
		if tok == "*" {
			t.Fatal(`node-b selects every computer for the host resolver ("*"); that is step 2, after a probe`)
		}
	}
}

// Kernel args are fixed at cold boot, so a resumed snapshot keeps the
// command line it booted with. The argument reaches a named computer only
// because naming it moves the launch-config hash: its pre-selection
// snapshot is refused and it cold boots with the resolver. Every other
// case (port unset, empty list, another computer named, port set alone)
// leaves the hash identical and the snapshot resumes.
func TestGuestDNSSelectionVoidsOnlyTheNamedComputersSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name string
		port int
		only []string
		want string
	}{
		{"nothing set", 0, nil, ""},
		{"port set alone", 8053, nil, ""},
		{"port set, empty list", 8053, []string{}, ""},
		{"port set, another computer", 8053, []string{"computer-other"}, ""},
		{"port unset, this computer", 0, []string{"computer-s3"}, ""},
		{"port set, this computer id", 8053, []string{"computer-s3"}, snapshotReasonLaunchConfig},
		{"port set, this vm id", 8053, []string{"vm-s3"}, snapshotReasonLaunchConfig},
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
			f.mgr.cfg.GuestDNSPort = tc.port
			f.mgr.cfg.GuestDNSOnly = tc.only
			_, expected := f.mgr.expectedLaunchConfig(meta, f.expect)
			hash, err := launchConfigHash(expected)
			if err != nil {
				t.Fatal(err)
			}
			named := len(guestDNSArgs(bootArgsOf(t, expected))) > 0
			if same := hash == meta.LaunchConfigSHA256; same == (tc.want != "") || named != (tc.want != "") {
				t.Fatalf("hash unchanged = %t, choir.dns present = %t; want the hash to move and the arg present only for a named computer with the port set", same, named)
			}
			v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: f.expect}
			if got := f.mgr.validateSnapshot(dir, meta, v); got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %q", got, tc.want)
			}
		})
	}
}
