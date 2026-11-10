package main

import (
	"strings"
	"testing"
)

// renderPoolSection must not silently drop spread fields, for any member kind:
// a bad pool is then refused by the server instead of rewritten into a
// different one.
func TestRenderPoolSectionKeepsSpreadFields(t *testing.T) {
	out := renderPoolSection(policyPool{Name: "p", Failover: true, Members: []policyPoolMember{
		{Model: "devin/swe-2-flash", Weight: 1, Spread: "g"},
		{Model: "bedrock/x", FallbackOnly: true, Spread: "g", SpreadWeight: 8},
	}})
	for _, want := range []string{`weight = 1, spread = "g" }`, `fallback_only = true, spread = "g", spread_weight = 8 }`} {
		if !strings.Contains(out, want) {
			t.Fatalf("render dropped a field, missing %q:\n%s", want, out)
		}
	}
}

// The write path refuses a plain-list default pool (it would drop the paid-tail
// spread); the read path tolerates an existing one (modelpolicy tests).
func TestPolicySetRefusesPlainListDefaultPool(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	bad := "# owner-edited\n[pools.default]\nmembers = [\"" + leadIceman + "\"]\n\n" + strings.TrimPrefix(cliPolicy, "# owner-edited\n")
	code, _, errOut := runPolicy("set", "--local", writeLocal(t, bad))
	if code == 0 || !strings.Contains(errOut, "spread") || readServerPolicy(t, root) != cliPolicy {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
}
