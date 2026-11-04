package agentprofile

import (
	"reflect"
	"testing"
)

func TestPolicyFor(t *testing.T) {
	t.Parallel()

	tests := map[string]Policy{
		Conductor:   {Profile: Conductor},
		Research:    {Profile: Research},
		Texture:     {Profile: Texture},
		Email:       {Profile: Email},
		Engineering: {Profile: Engineering},
		Management:  {Profile: Management},
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
	for _, profile := range []string{"processor", "reconciler"} {
		if _, err := PolicyFor(profile); err == nil {
			t.Errorf("PolicyFor(%q) unexpectedly accepted a retired profile", profile)
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
