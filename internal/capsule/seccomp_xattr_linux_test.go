//go:build linux

package capsule

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elastic/go-seccomp-bpf/arch"
	"golang.org/x/sys/unix"
)

// docs/problems/engineering-completion-path-cluster-2026-10-20.md failure
// 24: cp --no-preserve=mode sets an ACL on each file it creates, the
// default-deny filter refused that xattr write with EPERM, and cp treats
// EPERM as fatal ("setting permissions"), so every first release copy
// failed. Refused with EOPNOTSUPP, cp falls back to chmod. Failure modes:
// an xattr call is allowed (it must stay refused, only the errno changes);
// an xattr call the kernel names is missing from the EOPNOTSUPP group and
// still answers EPERM; another refused call loses its EPERM.
func TestEveryXattrCallIsRefusedWithEOPNOTSUPPNeverAllowed(t *testing.T) {
	filters := map[string][]string{
		"workload":        capsuleAllowedSyscalls,
		"broker":          append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...),
		"delegate":        append(append([]string(nil), capsuleAllowedSyscalls...), delegateExtraSyscalls...),
		"broker-delegate": append(append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...), delegateExtraSyscalls...),
	}
	for name, allowed := range filters {
		for _, call := range allowed {
			if strings.Contains(call, "xattr") {
				t.Errorf("%s filter allows %s", name, call)
			}
		}
	}
	for _, table := range []*arch.Info{arch.X86_64, arch.AARCH64} {
		for call := range table.SyscallNames {
			if strings.Contains(call, "xattr") && !slices.Contains(capsuleXattrSyscalls, call) {
				t.Errorf("%s names %s, which is not refused with EOPNOTSUPP", table.Name, call)
			}
		}
	}
}

// With the workload filter loaded, an xattr write and read fail with
// EOPNOTSUPP and leave no attribute, chmod still works (cp's fallback), and
// a call outside both lists is still refused with EPERM. Runs in a helper
// process because a filter cannot be unloaded.
func TestWorkloadSeccompRefusesXattrWithEOPNOTSUPP(t *testing.T) {
	if os.Getenv(seccompHelperEnv) == "xattr" {
		path := os.Getenv("CHOIR_SECCOMP_XATTR_FILE")
		if err := LoadWorkloadFilter(); err != nil {
			t.Fatalf("load workload filter: %v", err)
		}
		if err := unix.Setxattr(path, "user.choir", []byte("y"), 0); err != unix.EOPNOTSUPP {
			t.Fatalf("setxattr = %v, want EOPNOTSUPP from the filter", err)
		}
		if err := unix.Lsetxattr(path, "system.posix_acl_access", []byte{2, 0, 0, 0}, 0); err != unix.EOPNOTSUPP {
			t.Fatalf("lsetxattr ACL = %v, want EOPNOTSUPP from the filter", err)
		}
		if _, err := unix.Getxattr(path, "user.choir", make([]byte, 8)); err != unix.EOPNOTSUPP {
			t.Fatalf("getxattr = %v, want EOPNOTSUPP from the filter", err)
		}
		if err := unix.Chmod(path, 0o644); err != nil {
			t.Fatalf("chmod, cp's fallback, = %v", err)
		}
		// sched_setscheduler is in neither list: the default refusal stays EPERM.
		if _, _, errno := unix.Syscall(unix.SYS_SCHED_SETSCHEDULER, 0, 0, 0); errno != unix.EPERM {
			t.Fatalf("sched_setscheduler = %v, want EPERM", errno)
		}
		return
	}
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkloadSeccompRefusesXattrWithEOPNOTSUPP$")
	cmd.Env = append(os.Environ(), seccompHelperEnv+"=xattr", "CHOIR_SECCOMP_XATTR_FILE="+path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seccomp helper: %v\n%s", err, output)
	}
	// The refused write left nothing (where the filesystem has user xattrs).
	if _, err := unix.Getxattr(path, "user.choir", make([]byte, 8)); err == nil {
		t.Fatal("the refused setxattr left an attribute")
	}
}
