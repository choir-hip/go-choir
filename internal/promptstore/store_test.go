package promptstore

import (
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"strings"
	"testing"
)

func TestPromptStoreSeedsDefaults(t *testing.T) {
	store := New(t.TempDir())

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
}

func TestPromptStoreSupportsUserOverridesAndReset(t *testing.T) {
	store := New(t.TempDir())

	saved, err := store.Save("user-alice", agentprofile.Texture, "Custom texture prompt")
	if err != nil {
		t.Fatalf("save prompt override: %v", err)
	}
	if saved.Source != "user" {
		t.Fatalf("saved source = %q, want user", saved.Source)
	}

	loaded, err := store.Load("user-alice", agentprofile.Texture)
	if err != nil {
		t.Fatalf("load prompt override: %v", err)
	}
	if loaded.Content != "Custom texture prompt" {
		t.Fatalf("loaded content = %q, want custom override", loaded.Content)
	}
	if loaded.Source != "user" {
		t.Fatalf("loaded source = %q, want user", loaded.Source)
	}

	reset, err := store.Reset("user-alice", agentprofile.Texture)
	if err != nil {
		t.Fatalf("reset prompt override: %v", err)
	}
	if reset.Source != "default" {
		t.Fatalf("reset source = %q, want default", reset.Source)
	}
	if reset.Content == "" || reset.Content == "Custom texture prompt" {
		t.Fatalf("reset content should return the default prompt, got %q", reset.Content)
	}
	if !strings.Contains(reset.Content, "You are Texture") {
		t.Fatalf("reset content should load yaml-backed texture prompt, got %q", reset.Content)
	}
}

// Support judgement left management
// (docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md): the default
// management prompt must not teach Judge or support_pairs records.
func TestManagementDefaultPromptCarriesNoSupportJudgement(t *testing.T) {
	prompt, err := New(t.TempDir()).Load("user-alice", agentprofile.Management)
	if err != nil {
		t.Fatalf("load management prompt: %v", err)
	}
	for _, absent := range []string{"choir.Judge", "support_pairs", "Judging citations"} {
		if strings.Contains(prompt.Content, absent) {
			t.Errorf("management prompt still says %q", absent)
		}
	}
}

// Research searches the computer's own corpus before the web (symptom 3 of
// docs/archive/problems/prompt-bar-research-slow-unsynthesized-mangled-2026-10-14.md).
func TestResearchPromptSearchesTheCorpusBeforeTheWeb(t *testing.T) {
	prompt, err := New(t.TempDir()).Load("user-alice", agentprofile.Research)
	if err != nil {
		t.Fatalf("load research prompt: %v", err)
	}
	corpus, web := strings.Index(prompt.Content, "choir.SearchWireCorpus("), strings.Index(prompt.Content, "choir.SearchStart(")
	if corpus < 0 || web < 0 || corpus > web {
		t.Fatalf("research prompt does not search the corpus before the web: %q", prompt.Content)
	}
}
