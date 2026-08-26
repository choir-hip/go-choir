package yaegikernel

import (
	"testing"
)

func TestProfileAuthorization(t *testing.T) {
	registry := NewDefaultProfileRegistry()

	// 1. CoSuper Profile
	cosuper, err := registry.GetProfile(ProfileCoSuper)
	if err != nil {
		t.Fatalf("GetProfile(cosuper) failed: %v", err)
	}
	if cosuper.EffectsEnabled {
		t.Fatal("expected CoSuper external effects to be OFF (EffectsEnabled == false)")
	}

	// Verify CoSuper authorized actions
	for _, action := range []BrokerAction{ActionExec, ActionReadFile, ActionWriteFile, ActionAssign, ActionMessage} {
		if err := registry.AuthorizeAction(ProfileCoSuper, action); err != nil {
			t.Fatalf("expected CoSuper to be authorized for action %q, got error: %v", action, err)
		}
	}

	// 2. Researcher Profile
	researcher, err := registry.GetProfile(ProfileResearcher)
	if err != nil {
		t.Fatalf("GetProfile(researcher) failed: %v", err)
	}
	if researcher.EffectsEnabled {
		t.Fatal("expected Researcher external effects to be OFF")
	}

	// Verify Researcher authorized actions
	if err := registry.AuthorizeAction(ProfileResearcher, ActionReadFile); err != nil {
		t.Fatalf("expected Researcher to be authorized for read_file: %v", err)
	}
	if err := registry.AuthorizeAction(ProfileResearcher, ActionMessage); err != nil {
		t.Fatalf("expected Researcher to be authorized for message: %v", err)
	}

	// Verify Researcher REFUSES ActionExec (no Bash / arbitrary execution)
	if err := registry.AuthorizeAction(ProfileResearcher, ActionExec); err == nil {
		t.Fatal("expected Researcher to refuse ActionExec (no direct Bash)")
	}

	// Verify Researcher REFUSES ActionWriteFile
	if err := registry.AuthorizeAction(ProfileResearcher, ActionWriteFile); err == nil {
		t.Fatal("expected Researcher to refuse ActionWriteFile")
	}
}
