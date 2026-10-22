package hostboot

import (
	"strings"
	"testing"
	"time"
)

// The journal line is the alarm nobody has to ask for
// (docs/problems/host-kernel-216-days-unpatched-2026-10-15.md, 2026-10-22).
// Ways it could fail, written before the code:
//  1. a kernel path that is not a NixOS store name panics or prints garbage;
//  2. a pending reboot reads like a healthy host (no WARNING marker);
//  3. an overdue reboot is not told apart from a fresh one;
//  4. it drops booted_system, current_system, a kernel version or since-when;
//  5. an unreadable detector prints a clean line instead of saying so.

func TestKernelVersion(t *testing.T) {
	for path, want := range map[string]string{
		"/nix/store/abc123-linux-6.18.37/bzImage":         "6.18.37",
		"/nix/store/abc123-linux-6.18.16/bzImage":         "6.18.16",
		"/tmp/x/nix/store/k1-linux-6.18.55/bzImage":       "6.18.55",
		"/nix/store/abc123-linux-6.18.37-modules/lib":     "6.18.37",
		"/nix/store/abc123-linux-hardened-6.12.9/bzImage": "6.12.9",
		"/boot/vmlinuz": "",
		"":              "",
	} {
		if got := KernelVersion(path); got != want {
			t.Errorf("KernelVersion(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestLogLinePending(t *testing.T) {
	f := nodeBToday(t)
	line := Read(f.root, now).LogLine()
	for _, want := range []string{
		"WARNING host reboot pending",
		"booted_system=" + f.root + "/nix/store/aaa-nixos-system-old",
		"current_system=" + f.root + "/nix/store/bbb-nixos-system-new",
		"booted_kernel=6.18.16", "current_kernel=6.18.55", "running_kernel=6.18.16",
		"reasons=kernel,kernel_modules,kernel_params,module_param",
		"pending_since=" + t0.Format("2006-01-02T15:04:05Z"),
		"overdue=false",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("line lacks %q:\n%s", want, line)
		}
	}
	if overdue := Read(f.root, t0.Add(OverdueAfter + 24*time.Hour)).LogLine(); !strings.Contains(overdue, "OVERDUE") || !strings.Contains(overdue, "overdue=true") {
		t.Errorf("an 8-day pending reboot must say OVERDUE:\n%s", overdue)
	}
	if strings.Contains(line, "\n") {
		t.Errorf("one journal line, got newlines:\n%s", line)
	}
}

func TestLogLineClean(t *testing.T) {
	f := newFixture(t)
	cur := f.system(newSys)
	f.link("run/booted-system", cur)
	f.link("run/current-system", cur)
	f.proc("6.18.55", "loglevel=4 kvm.halt_poll_ns=0", now.Unix())
	f.live("kvm_intel", "nested", "N")
	f.live("kvm", "halt_poll_ns", "0")
	line := Read(f.root, now).LogLine()
	if strings.Contains(line, "WARNING") || !strings.Contains(line, "booted is activated") {
		t.Errorf("clean host line:\n%s", line)
	}
	for _, want := range []string{"booted_kernel=6.18.55", "current_kernel=6.18.55", "booted_system=", "current_system="} {
		if !strings.Contains(line, want) {
			t.Errorf("line lacks %q:\n%s", want, line)
		}
	}
}

func TestLogLineUnavailable(t *testing.T) {
	line := Read(t.TempDir(), now).LogLine()
	if !strings.Contains(line, "unavailable") || !strings.Contains(line, "booted system unreadable") {
		t.Errorf("unreadable detector line:\n%s", line)
	}
}
