package vmmanager

// Capsule egress tunnel identity per computer (SDC SEC-06,
// docs/problems/egress-proxy-tunnel-identity-2026-10-21.md). The guest image
// runs the check in observe (CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY in
// nix/autoputer-vm.nix); the computers this selector names run it in enforce,
// so enforce is proved on a test computer in the same deploy that ships
// observe everywhere (owner rule 2026-10-22):
//
//	VM_CAPSULE_TUNNEL_ENFORCE_ONLY   computer ids (computer_select.go); empty: none
//
// Name computer ids only. selectsComputer also takes VM ids, class:<class>
// and *, but class: misses the platform computer's boot and the resume
// config (vmManagerConfigForOwnership never sets WarmnessClass), and * is
// the global flip, which belongs to the image line behind its gate.
//
// The selection is a kernel boot argument, so neither the guest nor a capsule
// can set it; the guest boot script copies only the exact value "enforce"
// into the daemon's environment file. Boot arguments are in the
// launch-config hash, so a newly selected computer cold boots once.

const capsuleTunnelEnforceKernelParam = "choir.capsule_egress_tunnel_identity=enforce"

// capsuleTunnelIdentityKernelParams is the boot argument for cfg's launch;
// nil for a computer the list does not name.
func (m *Manager) capsuleTunnelIdentityKernelParams(cfg VMConfig) []string {
	if !selectsComputer(m.cfg.CapsuleTunnelEnforceOnly, cfg) {
		return nil
	}
	return []string{capsuleTunnelEnforceKernelParam}
}
