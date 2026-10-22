package vmmanager

import "fmt"

// Guest resolver per computer (SDC SEC-10,
// docs/problems/guest-dns-host-resolver-unused-2026-10-22.md). The host
// serves DNS to guests through Unbound on loopback, reached by a DNAT on each
// tap's host address :53 (VMCTL_GUEST_DNS_PORT, hostfirewall.go); the guest
// image points its resolver at the address choir.dns= names
// (go-choir-configure-network in nix/autoputer-vm.nix). A launch carries
// choir.dns=<its tap host address> only when both hold:
//
//	VMCTL_GUEST_DNS_PORT   the resolver's loopback port; 0 or unset: no DNAT
//	VMCTL_GUEST_DNS_ONLY   computer ids (computer_select.go); empty: none
//
// The port alone is not enough: it is set on every staging launch, and the
// argument is in the snapshot launch-config hash, so keying on it would void
// every computer's snapshot and move every guest's resolver in one deploy.
// Name computer ids: class: misses the platform computer's boot and the
// resume config (vmManagerConfigForOwnership never sets WarmnessClass), and
// "*" belongs to step 2 (public 53 out of the floor), after a probe.
//
// Kernel args are fixed at cold boot. A newly named computer's snapshot is
// refused (the hash moves) and it cold boots with the resolver; a computer
// dropped from the list cold boots once without it. Rollback: empty the list
// first; take the port or Unbound away only after the named computers have
// cold booted, or they are left with no resolver (resolved uses its fallback
// servers only when the link names none).

// guestDNSKernelParams is the boot argument for cfg's launch on the tap whose
// host address is hostIP; nil when the host serves no resolver or the list
// does not name the computer.
func (m *Manager) guestDNSKernelParams(cfg VMConfig, hostIP string) []string {
	if m.cfg.GuestDNSPort <= 0 || hostIP == "" || !selectsComputer(m.cfg.GuestDNSOnly, cfg) {
		return nil
	}
	return []string{fmt.Sprintf("choir.dns=%s", hostIP)}
}
