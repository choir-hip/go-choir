package modelpolicy

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The crouton test arm (owner 2026-11-03: "run another vm with only it for
// every desk"). Ways it could fail, written before the code: the policy
// refuses provider "openrouter"; Assign remaps the model to the default
// pool because the catalog does not list it (a silent fallback to another
// provider); the management task ignores the role and keeps its ChatGPT
// pin; a role the arm names resolves the defaults.
func TestOpenRouterCroutonArmPolicyPinsEveryRole(t *testing.T) {
	const arm = `# owner-edited
[defaults]
provider = "openrouter"
model = "nvidia/nemotron-3.5-super-vl:free"
reasoning = "low"
`
	roles := []string{
		agentprofile.Conductor, agentprofile.Texture, agentprofile.Management, ManagementTaskRole,
		agentprofile.Engineering, agentprofile.Research, VerifierRole, MultimodalVerifierRole,
	}
	text := arm
	for _, role := range roles {
		text += "\n[roles." + role + "]\nprovider = \"openrouter\"\nmodel = \"nvidia/nemotron-3.5-super-vl:free\"\n"
	}
	p, err := parsePolicy(text, "/System/model-policy.toml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, role := range append(roles, "") {
		sel := p.Resolve(role)
		if sel.Provider != "openrouter" || sel.Model != "nvidia/nemotron-3.5-super-vl:free" {
			t.Fatalf("role %q resolves %s/%s", role, sel.Provider, sel.Model)
		}
		assigned, fallbacks, assignment := Assign(sel, "thread-1")
		if assigned.Provider != "openrouter" || assigned.Model != "nvidia/nemotron-3.5-super-vl:free" || len(fallbacks) != 0 || assignment != "explicit" {
			t.Fatalf("role %q assigns %s/%s fallbacks %v (%s)", role, assigned.Provider, assigned.Model, fallbacks, assignment)
		}
	}
	if got := provideriface.MaxInteractiveOutputTokensForSelection(p.Resolve(agentprofile.Engineering), agentprofile.Engineering); got != 0 {
		t.Fatalf("openrouter interactive budget = %d, want 0 (omitted)", got)
	}
}
