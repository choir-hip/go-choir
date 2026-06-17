package runtime

import (
	"strings"
	"testing"
)

func TestPromptStoreSeedsDefaults(t *testing.T) {
	store := NewPromptStore(t.TempDir())

	core, err := store.LoadCore()
	if err != nil {
		t.Fatalf("load core prompt: %v", err)
	}
	if !strings.Contains(core, "multiagent writing, research, and execution system") {
		t.Fatalf("unexpected core prompt: %q", core)
	}

	prompts, err := store.List("user-alice")
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}

	if len(prompts) != len(promptRoles()) {
		t.Fatalf("prompt count = %d, want %d", len(prompts), len(promptRoles()))
	}

	for _, prompt := range prompts {
		if prompt.Role == "" {
			t.Fatal("prompt role should not be empty")
		}
		if prompt.Content == "" {
			t.Fatalf("prompt %s content should not be empty", prompt.Role)
		}
		if prompt.Source != "default" {
			t.Fatalf("prompt %s source = %q, want default", prompt.Role, prompt.Source)
		}
	}
	if _, err := store.Load("user-alice", AgentProfileProcessor); err != nil {
		t.Fatalf("load processor prompt: %v", err)
	}
	if _, err := store.Load("user-alice", AgentProfileReconciler); err != nil {
		t.Fatalf("load reconciler prompt: %v", err)
	}
}

func TestPromptStoreSupportsUserOverridesAndReset(t *testing.T) {
	store := NewPromptStore(t.TempDir())

	saved, err := store.Save("user-alice", AgentProfileTexture, "Custom texture prompt")
	if err != nil {
		t.Fatalf("save prompt override: %v", err)
	}
	if saved.Source != "user" {
		t.Fatalf("saved source = %q, want user", saved.Source)
	}

	loaded, err := store.Load("user-alice", AgentProfileTexture)
	if err != nil {
		t.Fatalf("load prompt override: %v", err)
	}
	if loaded.Content != "Custom texture prompt" {
		t.Fatalf("loaded content = %q, want custom override", loaded.Content)
	}
	if loaded.Source != "user" {
		t.Fatalf("loaded source = %q, want user", loaded.Source)
	}

	reset, err := store.Reset("user-alice", AgentProfileTexture)
	if err != nil {
		t.Fatalf("reset prompt override: %v", err)
	}
	if reset.Source != "default" {
		t.Fatalf("reset source = %q, want default", reset.Source)
	}
	if reset.Content == "" || reset.Content == "Custom texture prompt" {
		t.Fatalf("reset content should return the default prompt, got %q", reset.Content)
	}
	if !strings.Contains(reset.Content, "system prompt for the texture agent in Choir") {
		t.Fatalf("reset content should load yaml-backed texture prompt, got %q", reset.Content)
	}
}
