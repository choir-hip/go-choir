package agentprofile

import (
	"reflect"
	"testing"
)

func TestPolicyFor(t *testing.T) {
	t.Parallel()

	tests := map[string]Policy{
		Conductor: {
			Profile: Conductor, AllowCoAgentTools: true,
			AllowedSpawnTargets: []string{Texture},
		},
		Research: {
			Profile: Research, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedMessageTargets: []string{Texture},
		},
		Texture: {
			Profile: Texture, AllowMemoryTools: true, AllowCoAgentTools: false,
			AllowedSpawnTargets: []string{Research}, AllowedMessageTargets: []string{Research, Management},
		},
		Processor: {
			Profile: Processor, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedSpawnTargets: []string{Texture}, AllowedMessageTargets: []string{Texture},
		},
		Reconciler: {
			Profile: Reconciler, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedSpawnTargets: []string{Texture}, AllowedMessageTargets: []string{Texture},
		},
		Email:       {Profile: Email},
		Engineering: {Profile: Engineering, AllowedMessageTargets: []string{Management}},
		Management: {
			Profile: Management, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedSpawnTargets: []string{Research}, AllowedMessageTargets: []string{Texture, Research},
		},
	}
	for profile, want := range tests {
		got, err := PolicyFor(profile)
		if err != nil {
			t.Errorf("PolicyFor(%q) error = %v", profile, err)
		} else if !reflect.DeepEqual(got, want) {
			t.Errorf("PolicyFor(%q) = %#v, want %#v", profile, got, want)
		}
	}
	if _, err := PolicyFor(" NEWS_PROCESSOR "); err == nil {
		t.Error("PolicyFor(retired alias) error = nil, want UnknownProfileError")
	}
	if _, err := PolicyFor(" Custom_Profile "); err == nil {
		t.Error("PolicyFor(unknown) error = nil, want UnknownProfileError")
	}
	if got, err := PolicyFor("   "); err == nil || !reflect.DeepEqual(got, Policy{}) {
		t.Errorf("PolicyFor(empty) = %#v, %v, want (Policy{}, error)", got, err)
	}
}
func TestSpawnAndMessagePoliciesAreSeparatedExhaustively(t *testing.T) {
	t.Parallel()

	profiles := []string{Conductor, Management, Engineering, Research, Texture, Processor, Reconciler, Email}
	spawn := map[string]map[string]bool{
		Conductor:  {Texture: true},
		Management: {Research: true},
		Texture:    {Research: true},
		Processor:  {Texture: true},
		Reconciler: {Texture: true},
	}
	message := map[string]map[string]bool{
		Management:  {Texture: true, Research: true},
		Engineering: {Management: true},
		Research:    {Texture: true},
		Texture:     {Research: true, Management: true},
		Processor:   {Texture: true},
		Reconciler:  {Texture: true},
	}
	for _, caller := range profiles {
		caller := caller
		t.Run(caller, func(t *testing.T) {
			t.Parallel()
			for _, target := range profiles {
				gotSpawn, err := CanSpawn(caller, target)
				if err != nil {
					t.Fatalf("CanSpawn(%q, %q) error = %v", caller, target, err)
				}
				if gotSpawn != spawn[caller][target] {
					t.Errorf("CanSpawn(%q, %q) = %v, want %v", caller, target, gotSpawn, spawn[caller][target])
				}
				gotMsg, err := CanMessage(caller, target)
				if err != nil {
					t.Fatalf("CanMessage(%q, %q) error = %v", caller, target, err)
				}
				if gotMsg != message[caller][target] {
					t.Errorf("CanMessage(%q, %q) = %v, want %v", caller, target, gotMsg, message[caller][target])
				}
			}
		})
	}

	mustSpawn := func(caller, target string) bool {
		t.Helper()
		ok, err := CanSpawn(caller, target)
		if err != nil {
			t.Fatalf("CanSpawn(%q, %q) error = %v", caller, target, err)
		}
		return ok
	}
	mustMessage := func(caller, target string) bool {
		t.Helper()
		ok, err := CanMessage(caller, target)
		if err != nil {
			t.Fatalf("CanMessage(%q, %q) error = %v", caller, target, err)
		}
		return ok
	}
	if mustSpawn(Texture, Management) || !mustMessage(Texture, Management) {
		t.Fatal("Texture must message but never spawn Management")
	}
	if mustMessage(Texture, Engineering) {
		t.Fatal("Texture must never message Engineering")
	}
	if !mustSpawn(Conductor, Texture) || mustMessage(Conductor, Texture) {
		t.Fatal("Conductor keeps spawn-only Texture authority under V2 names")
	}
	if _, err := CanSpawn(Conductor, "document_agent"); err == nil {
		t.Fatal("retired alias document_agent must refuse spawn")
	}
	if _, err := CanMessage(Conductor, "document_agent"); err == nil {
		t.Fatal("retired alias document_agent must refuse message")
	}
	for _, check := range []struct {
		caller string
		target string
	}{
		{"unknown", Research},
		{Management, "unknown"},
		{"unknown", "unknown"},
	} {
		if ok, _ := CanSpawn(check.caller, check.target); ok {
			t.Errorf("unknown policy unexpectedly allowed spawn %q -> %q", check.caller, check.target)
		}
		if _, err := CanSpawn(check.caller, check.target); err == nil {
			t.Errorf("unknown policy missing spawn error %q -> %q", check.caller, check.target)
		}
		if ok, _ := CanMessage(check.caller, check.target); ok {
			t.Errorf("unknown policy unexpectedly allowed message %q -> %q", check.caller, check.target)
		}
		if _, err := CanMessage(check.caller, check.target); err == nil {
			t.Errorf("unknown policy missing message error %q -> %q", check.caller, check.target)
		}
	}
}

func TestCanonicalVerifierSpellingUnifies(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"verifier_multimodal", "verifier-multimodal", " VERIFIER-MULTIMODAL "} {
		got, err := Canonical(value)
		if err != nil {
			t.Fatalf("Canonical(%q) error = %v", value, err)
		}
		if got != "verifier_multimodal" {
			t.Fatalf("Canonical(%q) = %q, want verifier_multimodal", value, got)
		}
	}
}

// TestPolicyForDefaultIsNotDefaultAllow pins the fail-closed default: a
// canonical-but-unlisted profile (the verifier roles) gets a bare policy
// with every capability false and no spawn/message targets.
func TestPolicyForDefaultIsNotDefaultAllow(t *testing.T) {
	t.Parallel()
	for _, profile := range []string{"verifier", "verifier_multimodal"} {
		policy, err := PolicyFor(profile)
		if err != nil {
			t.Fatalf("PolicyFor(%q) error = %v", profile, err)
		}
		if policy.AllowReadOnlyFiles || policy.AllowResearchTools ||
			policy.AllowEvidenceTools || policy.AllowMemoryTools ||
			policy.AllowModelDiagnosticTools || policy.AllowCoAgentTools ||
			len(policy.AllowedSpawnTargets) != 0 || len(policy.AllowedMessageTargets) != 0 {
			t.Fatalf("PolicyFor(%q) grants capabilities: %+v", profile, policy)
		}
	}
}
