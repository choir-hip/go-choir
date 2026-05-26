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
		"Never describe coordination as already done unless the tool action really happened",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("factual first-revision prompt missing %q:\n%s", want, request)
		}
	}
}

func TestVTextPromptForPartialFindingsForbidsFalseFollowupClaims(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-baseball",
		RevisionID: "rev-baseball-v2",
		Content:    "# Baseball\n\nPartial findings received; final scores are still missing.",
		AuthorKind: types.AuthorAppAgent,
	}
	recent := []ChannelMessage{{
		Role:    AgentProfileResearcher,
		From:    "researcher:one",
		Content: "Findings: identified matchups, but final scores are still unavailable from this packet.",
	}}
	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"seed_prompt": "Last Night in Baseball",
	}, vtextAgentRevisionRequest{
		Intent: "integrate_worker_findings",
		Prompt: "Last Night in Baseball",
	}, "", true, false, recent, nil)

	for _, want := range []string{
		"This VText run was woken by worker findings",
		"Make those findings visible with edit_vtext as this turn's next document revision before spawning additional workers",
		"If recent worker findings are only partial and the document needs more evidence",
		"write an honest partial revision first",
		"Do not write that a follow-up researcher was dispatched",
		"Never describe coordination as already done unless the tool action really happened",
		"Phrases such as \"researcher dispatched\"",
		"If you only edit_vtext, phrase remaining work as \"next needed\" or \"still unresolved\"",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("partial-findings prompt missing %q:\n%s", want, request)
		}
	}
}

func TestVTextPromptPreservesExplicitHardConstraints(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-long-rubric",
		RevisionID: "rev-long-rubric",
		Content:    "1. Direct user edits\nEvidence pending.\n\nUSER_LONG_RUBRIC_MARKER: preserve this exact marker.",
		AuthorKind: types.AuthorUser,
	}
	request := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "The final brief must have exactly these numbered section headings and sections 1, 7, and 12 must contain sentences beginning SECTION 1 UPDATE:, SECTION 7 UPDATE:, and SECTION 12 UPDATE:. The final Source Ledger must include [S1], [S2], [S3], and [CMD].",
	}, "", false, false, nil, nil)

	for _, want := range []string{
		"Hard requirements checklist for the next canonical revision:",
		"Required sentence prefix: SECTION 1 UPDATE:",
		"Required sentence prefix: SECTION 7 UPDATE:",
		"Required sentence prefix: SECTION 12 UPDATE:",
		"Preserve exact marker line: USER_LONG_RUBRIC_MARKER: preserve this exact marker.",
		"Pending command evidence rule: before a super delivery exists",
		"Preserve explicit hard requirements from the original user request and current document across every revision",
		"exact marker strings",
		"required headings or section counts",
		"required labels or sentence prefixes",
		"target hashes",
		"Before a replace_all edit, audit the complete replacement against those hard requirements",
		"Do not replace a requested numbered/sectioned document with a different report outline",
		"Never use `[CMD]` as a pending/requested/target-only label, including in the initial v1 scaffold",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("hard-constraint prompt missing %q:\n%s", want, request)
		}
	}
}

func TestVTextPromptRestoresFinalCommandEvidenceRequirementAfterSuperDelivery(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-long-rubric-super",
		RevisionID: "rev-long-rubric-super",
		Content:    "1. Direct user edits\nEvidence pending.\n\nUSER_LONG_RUBRIC_MARKER: preserve this exact marker.",
		AuthorKind: types.AuthorAppAgent,
	}
	recent := []ChannelMessage{{
		Role:    AgentProfileSuper,
		From:    "super:one",
		Content: "Worker update ready.\n\nFindings:\n- [CMD] command exited 0 and printed the expected hash.",
	}}
	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"seed_prompt": "The final Source Ledger must include [S1], [S2], [S3], and [CMD].",
	}, vtextAgentRevisionRequest{
		Intent: "integrate_super_evidence",
	}, "", true, false, recent, nil)

	for _, want := range []string{
		"Final command evidence label: [CMD] (final-only:",
		"[CMD] command exited 0",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("super-evidence prompt missing %q:\n%s", want, request)
		}
	}
	if strings.Contains(request, "Pending command evidence rule: before a super delivery exists") {
		t.Fatalf("super-evidence prompt should not retain pending-only command rule:\n%s", request)
	}
}

func TestVTextPromptPrioritizesSuperAfterResearchForMixedObligation(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-mixed-obligation",
		RevisionID: "rev-mixed-obligation",
		Content:    "# Working brief\n\nCommand evidence pending.",
		AuthorKind: types.AuthorAppAgent,
	}
	recent := []ChannelMessage{{
		Role:    AgentProfileResearcher,
		From:    "researcher:one",
		Content: "Worker update ready.\n\nFindings:\n- [S1] VText documents have durable revisions.",
	}}
	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"seed_prompt": "Research VText durable drafts and run exactly one command: printf \"durable draft\" | shasum -a 256. The final Source Ledger must include [S1], [S2], [S3], and [CMD].",
	}, vtextAgentRevisionRequest{
		Intent: "integrate_worker_findings",
	}, "", true, false, recent, nil)

	for _, want := range []string{
		"recent worker messages do not include a super delivery",
		"next side-effectful action should be request_super_execution before another source-only edit",
		"Do not attempt a full-document rewrite in this worker-wake turn before the super request exists",
		"Keep the request_super_execution objective concise and concrete",
		"must not use the final [CMD] evidence label before the super delivery arrives",
		"Do not spend a worker-wake turn only improving source text while that execution obligation has no super request",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("mixed-obligation prompt missing %q:\n%s", want, request)
		}
	}
}

func TestVTextSuperContinuationObjectiveRequiresCoagentUpdate(t *testing.T) {
	objective := buildVTextSuperContinuationObjective("run printf test")
	for _, want := range []string{
		"Reporting contract",
		"Run each side-effectful command or tool payload at most once per model response",
		"do not emit duplicate same-turn bash calls in parallel",
		"After any command result, call submit_coagent_update",
		"If the command fails, still call submit_coagent_update",
		"VText only consumes addressed coagent updates",
	} {
		if !strings.Contains(objective, want) {
			t.Fatalf("super continuation objective missing %q:\n%s", want, objective)
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
		"Temporal grounding",
		"Current UTC date/time at delegation",
		"For relative-date requests",
		"First checkpoint protocol",
		"Run exactly one web_search call before the first submit_coagent_update call",
		"do not issue parallel search calls before the first update",
		"As soon as you have 2-4 grounded facts or a precise blocker, call submit_coagent_update",
		"omit the evidence array rather than sending malformed evidence",
		"For live scores, schedules, current rankings, weather, or similar time-sensitive lookups",
		"prefer official league/event/source pages or established scoreboards",
		"do not treat blocked HTML scoreboard pages as terminal by themselves",
		"verified final, live/pending, scheduled, or snippet-only",
		"after that batch, call submit_coagent_update again with the new material cluster or blocker",
	} {
		if !strings.Contains(objective, want) {
			t.Fatalf("research continuation objective missing %q:\n%s", want, objective)
		}
	}
}
