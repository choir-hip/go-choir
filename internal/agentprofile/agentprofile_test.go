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
			AllowedDelegateTargets: []string{Texture},
		},
		Researcher: {
			Profile: Researcher, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
		},
		Texture: {
			Profile: Texture, AllowMemoryTools: true, AllowCoAgentTools: true,
			AllowedDelegateTargets: []string{Researcher},
		},
		Processor: {
			Profile: Processor, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedDelegateTargets: []string{Texture},
		},
		Reconciler: {
			Profile: Reconciler, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedDelegateTargets: []string{Texture},
		},
		Email:   {Profile: Email},
		CoSuper: {Profile: CoSuper},
		Super: {
			Profile: Super, AllowReadOnlyFiles: true, AllowResearchTools: true,
			AllowEvidenceTools: true, AllowMemoryTools: true,
			AllowModelDiagnosticTools: true, AllowCoAgentTools: true,
			AllowedDelegateTargets: []string{Researcher, CoSuper},
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

func TestCanDelegate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		caller string
		target string
		want   bool
	}{
		{Conductor, Texture, true},
		{Conductor, "document_agent", true},
		{Processor, Texture, true},
		{Reconciler, Texture, true},
		{Texture, Researcher, true},
		{CoSuper, Researcher, false},
		{Super, Researcher, true},
		{Super, CoSuper, true},
		{Researcher, Texture, false},
		{Texture, Super, false},
		{Email, Researcher, false},
		{"unknown", Researcher, false},
		{Super, "unknown", false},
	} {
		if got := CanDelegate(tt.caller, tt.target); got != tt.want {
			t.Errorf("CanDelegate(%q, %q) = %v, want %v", tt.caller, tt.target, got, tt.want)
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
