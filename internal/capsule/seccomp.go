//go:build linux

package capsule

import (
	"fmt"

	"github.com/elastic/go-seccomp-bpf"
	"golang.org/x/sys/unix"
)

// denyEPERM is the seccomp action that denies a syscall with EPERM.
// SECCOMP_RET_ERRNO uses the low 16 bits as the errno value, so we must
// OR in unix.EPERM. Without this, denied syscalls would return errno=0
// (success) — a security-critical bug caught in v13 consensus.
var denyEPERM = seccomp.ActionErrno | seccomp.Action(unix.EPERM)

// WorkloadSeccompFilter returns the seccomp filter for capsule workloads.
// Default-allow with targeted denylist + socket family filtering.
// AF_UNIX (1) is allowed for broker control plane.
// AF_INET (2), AF_INET6 (10), AF_NETLINK (16), AF_VSOCK (40) are denied.
//
// Each denied socket family is its own SyscallGroup with NamesWithCondtions
// (note: library has typo "Condtions"). Separate groups are ORed at the
// BPF filter level. Rules within a single Conditions slice are ANDed.
func WorkloadSeccompFilter() seccomp.Filter {
	return seccomp.Filter{
		NoNewPrivs: true,
		Flag:       seccomp.FilterFlagTSync,
		Policy: seccomp.Policy{
			DefaultAction: seccomp.ActionAllow,
			Syscalls: []seccomp.SyscallGroup{
				// Targeted denylist: kernel keyring, ptrace, mount, modules,
				// namespace escape, perf, bpf.
				{
					Action: denyEPERM,
					Names: []string{
						"keyctl", "add_key", "request_key",
						"ptrace", "process_vm_readv", "process_vm_writev",
						"mount", "umount2", "pivot_root", "swapon", "swapoff",
						"reboot", "init_module", "finit_module", "delete_module",
						"kexec_load", "kexec_file_load",
						"perf_event_open", "fanotify_init",
						"bpf", "lookup_bpf_cookie",
						"unshare", "setns", // prevent namespace escape
					},
				},
				// Block socket(AF_INET, ...) — IPv4 networking.
				{
					Action: denyEPERM,
					NamesWithCondtions: []seccomp.NameWithConditions{{
						Name: "socket",
						Conditions: seccomp.ArgumentConditions{{
							Argument:  0, // socket(domain, type, protocol) — filter on domain
							Operation: seccomp.Equal,
							Value:     uint64(unix.AF_INET), // 2
						}},
					}},
				},
				// Block socket(AF_INET6, ...) — IPv6 networking.
				{
					Action: denyEPERM,
					NamesWithCondtions: []seccomp.NameWithConditions{{
						Name: "socket",
						Conditions: seccomp.ArgumentConditions{{
							Argument:  0,
							Operation: seccomp.Equal,
							Value:     uint64(unix.AF_INET6), // 10
						}},
					}},
				},
				// Block socket(AF_NETLINK, ...) — netlink (route discovery, etc).
				{
					Action: denyEPERM,
					NamesWithCondtions: []seccomp.NameWithConditions{{
						Name: "socket",
						Conditions: seccomp.ArgumentConditions{{
							Argument:  0,
							Operation: seccomp.Equal,
							Value:     uint64(unix.AF_NETLINK), // 16
						}},
					}},
				},
				// Block socket(AF_VSOCK, ...) — vsock to host (v8 security fix).
				// Prevents broker/workload from directly reaching HostAuthority.
				{
					Action: denyEPERM,
					NamesWithCondtions: []seccomp.NameWithConditions{{
						Name: "socket",
						Conditions: seccomp.ArgumentConditions{{
							Argument:  0,
							Operation: seccomp.Equal,
							Value:     uint64(unix.AF_VSOCK), // 40
						}},
					}},
				},
			},
		},
	}
}

// BrokerSeccompFilter applies the same network, namespace, mount, ptrace,
// module, BPF, keyring, and host-vsock denial floor inherited by workloads.
func BrokerSeccompFilter() seccomp.Filter {
	return WorkloadSeccompFilter()
}

// LoadWorkloadFilter loads the workload seccomp filter into the kernel.
// Must be called after fork, before exec of the workload process.
func LoadWorkloadFilter() error {
	filter := WorkloadSeccompFilter()
	if err := seccomp.LoadFilter(filter); err != nil {
		return fmt.Errorf("failed to load workload seccomp filter: %w", err)
	}
	return nil
}

// LoadBrokerFilter loads the broker seccomp filter into the kernel.
// Must be called after fork, before exec of the broker process.
func LoadBrokerFilter() error {
	filter := BrokerSeccompFilter()
	if err := seccomp.LoadFilter(filter); err != nil {
		return fmt.Errorf("failed to load broker seccomp filter: %w", err)
	}
	return nil
}
