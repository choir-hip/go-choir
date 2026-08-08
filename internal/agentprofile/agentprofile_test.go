package agentprofile

import (
	"reflect"
	"testing"
)

func TestCanonical(t *testing.T) {
	t.Parallel()

	aliases := map[string][]string{
		Researcher: {"researcher", "researchers", "research", "research-agent", "web-research", "web-researcher", " WEB_RESEARCHER "},
		CoSuper:    {"cosuper", "co-super", "coagent", "co-agent", " CO_AGENT "},
		Texture:    {"texture", "texture-agent", "document-agent", " DOCUMENT_AGENT "},
		Processor:  {"processor", "news-processor", "source-processor", "universal-wire-processor", " NEWS_PROCESSOR "},
		Reconciler: {"reconciler", "news-reconciler", "story-reconciler", "corpus-reconciler", "universal-wire-reconciler", " STORY_RECONCILER "},
		Email:      {"email", "email-agent", "email-appagent", "mail", "mail-agent", " EMAIL_APPAGENT "},
		Super:      {"super", " SUPER "},
		Conductor:  {"conductor", " CONDUCTOR "},
	}
	for want, values := range aliases {
		for _, value := range values {
			value := value
			t.Run(value, func(t *testing.T) {
				t.Parallel()
				if got := Canonical(value); got != want {
					t.Fatalf("Canonical(%q) = %q, want %q", value, got, want)
				}
			})
		}
	}
	for _, tt := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"Custom_Profile", "custom-profile"},
		{" Mixed Unknown ", "mixed unknown"},
	} {
		if got := Canonical(tt.in); got != tt.want {
			t.Fatalf("Canonical(%q) = %q, want %q", tt.in, got, tt.want)
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
			Profile: Texture, AllowMemoryTools: true, AllowCoAgentTools: true,
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
		Email: {Profile: Email},
		CoSuper: {
			Profile: CoSuper, AllowReadOnlyFiles: true, AllowEvidenceTools: true,
			AllowModelDiagnosticTools: true, AllowedMessageTargets: []string{Super, Texture},
		},
		Super: {
			Profile: Super, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedSpawnTargets: []string{Researcher, CoSuper}, AllowedMessageTargets: []string{Texture, Researcher, CoSuper},
		},
	}
	for profile, want := range tests {
		if got := PolicyFor(profile); !reflect.DeepEqual(got, want) {
			t.Errorf("PolicyFor(%q) = %#v, want %#v", profile, got, want)
		}
	}
	if got, want := PolicyFor(" NEWS_PROCESSOR "), tests[Processor]; !reflect.DeepEqual(got, want) {
		t.Errorf("PolicyFor(alias) = %#v, want %#v", got, want)
	}
	if got := PolicyFor(" Custom_Profile "); !reflect.DeepEqual(got, Policy{Profile: "Custom_Profile"}) {
		t.Errorf("PolicyFor(unknown) = %#v", got)
	}
	if got := PolicyFor("   "); !reflect.DeepEqual(got, Policy{}) {
		t.Errorf("PolicyFor(empty) = %#v", got)
	}
}

func TestSpawnAndMessagePoliciesAreSeparatedExhaustively(t *testing.T) {
	t.Parallel()

	profiles := []string{Conductor, Super, CoSuper, Researcher, Texture, Processor, Reconciler, Email}
	spawn := map[string]map[string]bool{
		Conductor:  {Texture: true},
		Super:      {Researcher: true, CoSuper: true},
		Texture:    {Researcher: true},
		Processor:  {Texture: true},
		Reconciler: {Texture: true},
	}
	message := map[string]map[string]bool{
		Super:      {Texture: true, Researcher: true, CoSuper: true},
		CoSuper:    {Super: true, Texture: true},
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
				if got, want := CanSpawn(caller, target), spawn[caller][target]; got != want {
					t.Errorf("CanSpawn(%q, %q) = %v, want %v", caller, target, got, want)
				}
				if got, want := CanMessage(caller, target), message[caller][target]; got != want {
					t.Errorf("CanMessage(%q, %q) = %v, want %v", caller, target, got, want)
				}
			}
		})
	}

	if CanSpawn(Texture, Super) || !CanMessage(Texture, Super) {
		t.Fatal("Texture must message but never spawn Super")
	}
	if CanMessage(Texture, CoSuper) {
		t.Fatal("Texture must never message CoSuper")
	}
	if !CanSpawn(Conductor, "document_agent") || CanMessage(Conductor, "document_agent") {
		t.Fatal("canonical aliases must preserve Conductor spawn-only Texture authority")
	}
	for _, check := range []struct {
		caller string
		target string
	}{
		{"unknown", Researcher},
		{Super, "unknown"},
		{"unknown", "unknown"},
	} {
		if CanSpawn(check.caller, check.target) || CanMessage(check.caller, check.target) {
			t.Errorf("unknown policy unexpectedly allowed %q -> %q", check.caller, check.target)
		}
	}
}

func TestIsTexture(t *testing.T) {
	t.Parallel()

	for _, profile := range []string{Texture, "texture-agent", "DOCUMENT_AGENT"} {
		if !IsTexture(profile) {
			t.Errorf("IsTexture(%q) = false", profile)
		}
	}
	for _, profile := range []string{"", Researcher, "unknown"} {
		if IsTexture(profile) {
			t.Errorf("IsTexture(%q) = true", profile)
		}
	}
}
