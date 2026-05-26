package runtime

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestVTextPromptCreativeDraftFastPath(t *testing.T) {
	prompt := "tell me a story about computers"
	if !vtextPromptAllowsUngroundedCreativeDraft(prompt) {
		t.Fatalf("expected %q to allow an ungrounded creative draft", prompt)
	}
	if vtextRevisionRequiresWorkerGrounding(false, types.AuthorAppAgent, true) {
		t.Fatal("creative conductor seed should not require worker grounding")
	}

	current := types.Revision{
		DocID:      "doc-story",
		RevisionID: "rev-story",
		Content:    "# tell me a story\n\ntell me a story about computers",
		AuthorKind: types.AuthorAppAgent,
	}
	request := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: prompt,
	}, "", false, true, nil, nil)

	for _, want := range []string{
		"You may call edit_vtext to produce the requested creative document without worker grounding",
		"Do not spawn researcher or request super for this creative draft",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("creative vtext prompt missing %q:\n%s", want, request)
		}
	}
}

func TestVTextPromptShortStoryFastPath(t *testing.T) {
	prompt := "Tell me a short story about a careful computer. Write it as a VText document, under 120 words."
	if !vtextPromptAllowsUngroundedCreativeDraft(prompt) {
		t.Fatalf("expected %q to allow an ungrounded creative draft", prompt)
	}
	if vtextRevisionRequiresWorkerGrounding(false, types.AuthorAppAgent, true) {
		t.Fatal("short story conductor seed should not require worker grounding")
	}
}

func TestVTextPromptStoryWithCurrentFactsRequiresGrounding(t *testing.T) {
	prompt := "What's the story with the Iran deal now?"
	if vtextPromptAllowsUngroundedCreativeDraft(prompt) {
		t.Fatalf("%q should require grounded research", prompt)
	}
}

func TestVTextPromptExplicitSentenceFastPath(t *testing.T) {
	prompt := "write one short sentence that says VText wrapper cleanup works"
	if !vtextPromptAllowsUngroundedCreativeDraft(prompt) {
		t.Fatalf("expected %q to allow direct drafting without worker grounding", prompt)
	}
}

func TestVTextPromptCurrentEventsRequiresResearcher(t *testing.T) {
	prompt := "what's going on with iran deal now"
	if vtextPromptAllowsUngroundedCreativeDraft(prompt) {
		t.Fatalf("%q should require grounded research", prompt)
	}

	current := types.Revision{
		DocID:      "doc-current-events",
		RevisionID: "rev-current-events",
		Content:    prompt,
		AuthorKind: types.AuthorAppAgent,
	}
	request := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: prompt,
	}, "", false, false, nil, nil)

	for _, want := range []string{
		"For factual/current claims, write a brief working revision with explicit uncertainty, then call spawn_agent with role=\"researcher\"",
		"Do not call edit_vtext with factual claims from model priors",
		"Ordinary factual, current-events, web, or \"what is going on now\" questions are research work, not super work",
		"Do not route them to request_super_execution",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("current-events vtext prompt missing %q:\n%s", want, request)
		}
	}
}

func TestVTextPromptForFactualFirstRevisionForbidsUngroundedContent(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-nba",
		RevisionID: "rev-nba",
		Content:    "nba update",
		AuthorKind: types.AuthorUser,
	}
	request := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "nba update",
	}, "", false, false, nil, nil)

	for _, want := range []string{
		"the first revision should be a short working brief with explicit uncertainty and no ungrounded claims",
		"Do not add factual claims, citations, or coding results from model priors",
		"write a brief working revision first, then start the needed worker request before ending the run",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("factual first-revision prompt missing %q:\n%s", want, request)
		}
	}
}

func TestInitialVTextToolChoiceUsesExactTools(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		prompt   string
		want     string
	}{
		{
			name: "current factual work starts researcher",
			metadata: map[string]any{
				"type":                      "vtext_agent_revision",
				"requires_worker_grounding": true,
				"original_prompt":           "what is the weather in boston now",
			},
			want: "function:spawn_agent",
		},
		{
			name: "mutable product work requests super",
			metadata: map[string]any{
				"type":                      "vtext_agent_revision",
				"requires_worker_grounding": true,
				"original_prompt":           "debug and fix the runtime gateway",
			},
			want: "function:request_super_execution",
		},
		{
			name: "creative direct document work edits vtext",
			metadata: map[string]any{
				"type":                      "vtext_agent_revision",
				"requires_worker_grounding": false,
				"original_prompt":           "tell me a story about computers",
			},
			want: "function:edit_vtext",
		},
		{
			name: "worker wake leaves vtext free to choose",
			metadata: map[string]any{
				"type":                      "vtext_agent_revision",
				"requires_worker_grounding": false,
				"original_prompt":           "research the sources and run one command",
				"scheduled_message_seq":     int64(3),
			},
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := initialVTextToolChoice(&types.RunRecord{
				Prompt:   tc.prompt,
				Metadata: tc.metadata,
			})
			if got != tc.want {
				t.Fatalf("initialVTextToolChoice = %q, want %q", got, tc.want)
			}
			if got == "required" {
				t.Fatal("VText must not use broad required tool choice")
			}
		})
	}
}

func TestVTextInitialEditContinuationClassifiesPrompts(t *testing.T) {
	tests := []struct {
		prompt       string
		wantResearch bool
		wantSuper    bool
	}{
		{prompt: "nba update", wantResearch: true},
		{prompt: "Last Night in Baseball", wantResearch: true},
		{prompt: "what's the weather in boston now", wantResearch: true},
		{prompt: "hey"},
		{prompt: "tell me a story about computers"},
		{prompt: "write a tiny bash command that counts files", wantSuper: true},
		{prompt: "research mud brick architecture and write a tiny shell command", wantResearch: true, wantSuper: true},
		{prompt: "debug and fix the runtime gateway", wantSuper: true},
	}
	for _, tc := range tests {
		t.Run(tc.prompt, func(t *testing.T) {
			if got := vtextPromptNeedsResearchContinuation(tc.prompt); got != tc.wantResearch {
				t.Fatalf("vtextPromptNeedsResearchContinuation(%q) = %v, want %v", tc.prompt, got, tc.wantResearch)
			}
			if got := vtextPromptNeedsSuperExecution(tc.prompt); got != tc.wantSuper {
				t.Fatalf("vtextPromptNeedsSuperExecution(%q) = %v, want %v", tc.prompt, got, tc.wantSuper)
			}
		})
	}
}

func TestVTextExplicitResearchWinsFirstContinuationForMixedPrompt(t *testing.T) {
	prompt := "research what mud brick architecture means and write a tiny shell command that would create a notes file for it"
	if !vtextPromptExplicitlyAsksResearchFirst(prompt) {
		t.Fatalf("expected explicit research-first marker for %q", prompt)
	}
	if !vtextPromptNeedsResearchContinuation(prompt) || !vtextPromptNeedsSuperExecution(prompt) {
		t.Fatalf("mixed prompt should classify as both research and super-capable")
	}
}

func TestVTextResearchContinuationObjectiveRequiresFastCheckpoint(t *testing.T) {
	objective := buildVTextResearchContinuationObjective("nba update")
	for _, want := range []string{
		"First checkpoint protocol",
		"Run at most one focused search batch",
		"As soon as you have 2-4 grounded facts or a precise blocker, call submit_coagent_update",
		"omit the evidence array rather than sending malformed evidence",
		"checkpoint each new material cluster",
	} {
		if !strings.Contains(objective, want) {
			t.Fatalf("research continuation objective missing %q:\n%s", want, objective)
		}
	}
}
