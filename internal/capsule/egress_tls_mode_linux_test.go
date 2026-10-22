//go:build linux

package capsule

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/mirrorgrant"
)

// The mode is the executor's: the daemon reads it from its own environment
// and the executor writes it on every policy it builds, the Claude
// delegate's included. A capsule never supplies a policy.
func TestExecutorSetsTheTunnelIdentityModeOnEveryPolicy(t *testing.T) {
	e := &Executor{}
	for _, claude := range []bool{false, true} {
		if got := e.egressPolicyLocked(claude, mirrorgrant.Status{}).TunnelIdentity; got != TunnelIdentityObserve {
			t.Fatalf("claude=%t: default mode %q, want observe", claude, got)
		}
	}
	e.SetEgressTunnelIdentity(TunnelIdentityEnforce)
	for _, claude := range []bool{false, true} {
		if got := e.egressPolicyLocked(claude, mirrorgrant.Status{}).TunnelIdentity; got != TunnelIdentityEnforce {
			t.Fatalf("claude=%t: mode %q, want enforce", claude, got)
		}
	}
	e.SetEgressTunnelIdentity("bogus")
	if got := e.egressPolicyLocked(false, mirrorgrant.Status{}).TunnelIdentity; got != TunnelIdentityObserve {
		t.Fatalf("an unknown mode set %q, want observe", got)
	}
}
