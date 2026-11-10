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
