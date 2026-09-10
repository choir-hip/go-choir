package agentprofile

import (
	"errors"
	"reflect"
	"testing"
)

func TestCanonical(t *testing.T) {
	t.Parallel()

	live := map[string][]string{
		Researcher: {"research", " RESEARCH "},
		CoSuper:    {"engineering", " ENGINEERING "},
		Texture:    {"texture", " TEXTURE "},
		Processor:  {"processor"},
		Reconciler: {"reconciler"},
		Email:      {"email"},
		Super:      {"management", " MANAGEMENT "},
		Conductor:  {"conductor", " CONDUCTOR "},
	}
	for want, values := range live {
		for _, value := range values {
			value := value
			t.Run(value, func(t *testing.T) {
				t.Parallel()
				got, err := Canonical(value)
				if err != nil {
					t.Fatalf("Canonical(%q) error = %v", value, err)
				}
				if got != want {
					t.Fatalf("Canonical(%q) = %q, want %q", value, got, want)
				}
			})
		}
	}
	// Retired V1 aliases and unknowns fail closed with the empty string,
	// never the input token.
	for _, value := range []string{
		"", "   ", "super", "co-super", "cosuper", "coagent", "co-agent",
		"researchers", "research-agent", "web-research", "texture-agent",
		"document_agent", "news-processor", "Custom_Profile", " Mixed Unknown ",
	} {
		got, err := Canonical(value)
		if err == nil {
			t.Fatalf("Canonical(%q) error = nil, want UnknownProfileError", value)
		}
		var unknown UnknownProfileError
		if !errors.As(err, &unknown) {
			t.Fatalf("Canonical(%q) error = %T, want UnknownProfileError", value, err)
		}
		if got != "" {
			t.Fatalf("Canonical(%q) = %q, want empty (fail-closed)", value, got)
		}
	}
}

func TestPolicyFor(t *testing.T) {
	t.Parallel()

	tests := map[string]Policy{
		Conductor: {
			Profile: Conductor, AllowCoAgentTools: true,
			AllowedSpawnTargets: []string{Texture},
		},
		Researcher: {
			Profile: Researcher, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedMessageTargets: []string{Texture},
		},
		Texture: {
			Profile: Texture, AllowMemoryTools: true, AllowCoAgentTools: false,
			AllowedSpawnTargets: []string{Researcher}, AllowedMessageTargets: []string{Researcher, Super},
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
		Email:   {Profile: Email},
		CoSuper: {Profile: CoSuper, AllowedMessageTargets: []string{Super}},
		Super: {
			Profile: Super, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedSpawnTargets: []string{Researcher}, AllowedMessageTargets: []string{Texture, Researcher},
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

	profiles := []string{Conductor, Super, CoSuper, Researcher, Texture, Processor, Reconciler, Email}
	spawn := map[string]map[string]bool{
		Conductor:  {Texture: true},
		Super:      {Researcher: true},
		Texture:    {Researcher: true},
		Processor:  {Texture: true},
		Reconciler: {Texture: true},
	}
	message := map[string]map[string]bool{
		Super:      {Texture: true, Researcher: true},
		CoSuper:    {Super: true},
		Researcher: {Texture: true},
		Texture:    {Researcher: true, Super: true},
		Processor:  {Texture: true},
		Reconciler: {Texture: true},
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
	if mustSpawn(Texture, Super) || !mustMessage(Texture, Super) {
		t.Fatal("Texture must message but never spawn Super")
	}
	if mustMessage(Texture, CoSuper) {
		t.Fatal("Texture must never message CoSuper")
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
		{"unknown", Researcher},
		{Super, "unknown"},
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

func TestIsTexture(t *testing.T) {
	t.Parallel()
	for _, profile := range []string{Texture, " TEXTURE "} {
		if !IsTexture(profile) {
			t.Errorf("IsTexture(%q) = false", profile)
		}
	}
	for _, profile := range []string{"", Researcher, "unknown", "texture-agent", "DOCUMENT_AGENT"} {
		if IsTexture(profile) {
			t.Errorf("IsTexture(%q) = true", profile)
		}
	}
}
