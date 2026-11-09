package modelpolicy

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// Owner 2026-11-09: low reasoning for every role; the written default file
// says the same, and the selfdev pool resolves by name.
func TestPlatformDefaultEffortsAreAllLow(t *testing.T) {
	p := fallbackPolicy(provideriface.Config{})
	if p.Defaults.ReasoningEffort != "low" || p.Defaults.Model != "pool:default" {
		t.Fatalf("defaults = %+v", p.Defaults)
	}
	for role, sel := range p.Roles {
		if sel.ReasoningEffort != "low" {
			t.Fatalf("role %s effort = %q", role, sel.ReasoningEffort)
		}
	}
	parsed, err := parsePolicy(defaultPolicyText(provideriface.Config{}), "file")
	if err != nil {
		t.Fatal(err)
	}
	for role, sel := range parsed.Roles {
		if sel.ReasoningEffort != "low" {
			t.Fatalf("file role %s effort = %q", role, sel.ReasoningEffort)
		}
	}
	selfdev, err := parsePolicy("# owner-edited\n[defaults]\nmodel = \"pool:selfdev\"\nreasoning = \"low\"\n", "file")
	if err != nil || selfdev.Defaults.Model != "pool:selfdev" {
		t.Fatalf("selfdev policy: %+v %v", selfdev.Defaults, err)
	}
}
