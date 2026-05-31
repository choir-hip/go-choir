package runtime

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestVTextPromptInitialRevisionUsesSingleWriterLoop(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-current-events",
		RevisionID: "rev-current-events",
		Content:    "what's going on with iran deal now",
		AuthorKind: types.AuthorUser,
	}
	request := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "what's going on with iran deal now",
	}, "", false, nil, nil)

	for _, want := range []string{
		"Because VText owns the document, write the first useful owner-readable revision with edit_vtext before opening longer worker work.",
		"For factual/current/search requests, the first revision should be a short working brief with explicit uncertainty and no ungrounded claims, followed by a researcher spawn in the same run.",
		"Worker messages can wake later vtext runs and trigger the next revision.",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("initial vtext prompt missing %q:\n%s", want, request)
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
	}, "", false, nil, nil)

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
	}, "", true, recent, nil)

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
	}, "", false, nil, nil)

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

func TestVTextPromptPreservesInlineSourceRefs(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-source-ref",
		RevisionID: "rev-source-ref",
		Content:    "# Source Review\n\nThis claim cites [the clip](source:src-youtube-demo).",
		AuthorKind: types.AuthorUser,
	}
	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"source_entities": []vtextSourceEntity{
			{
				EntityID: "src-youtube-demo",
				Kind:     "youtube_video",
				Label:    "Demo clip",
				Target: vtextSourceEntityTarget{
					CanonicalURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
				},
				Display: vtextSourceEntityDisplay{
					OpenSurface: "video",
				},
				Evidence: vtextSourceEntityEvidence{
					TranscriptAvailability: "unavailable",
					ResearchState:          "pending",
				},
			},
		},
	}, vtextAgentRevisionRequest{
		Intent: "revise",
		Prompt: "Keep the citation attached while making the wording clearer.",
	}, "", false, nil, nil)

	for _, want := range []string{
		"Detected VText source entities:",
		"youtube_video Demo clip entity_id=src-youtube-demo",
		"Canonical inline Source Entity syntax is [label](source:ENTITY_ID)",
		"Preserve existing source: entity ids exactly",
		"Preserve inline source ref exactly: [the clip](source:src-youtube-demo)",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("source-ref prompt missing %q:\n%s", want, request)
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
	}, "", true, recent, nil)

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
	}, "", true, recent, nil)

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

func TestInitialVTextToolChoiceUsesExactTools(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		want     string
	}{
		{
			name: "current factual work starts with vtext edit",
			metadata: map[string]any{
				"type":            "vtext_agent_revision",
				"original_prompt": "what is the weather in boston now",
			},
			want: "function:edit_vtext",
		},
		{
			name: "mutable product work starts with vtext edit",
			metadata: map[string]any{
				"type":            "vtext_agent_revision",
				"original_prompt": "debug and fix the runtime gateway",
			},
			want: "function:edit_vtext",
		},
		{
			name: "creative direct document work edits vtext",
			metadata: map[string]any{
				"type":            "vtext_agent_revision",
				"original_prompt": "tell me a story about computers",
			},
			want: "function:edit_vtext",
		},
		{
			name: "worker wake leaves vtext free to choose",
			metadata: map[string]any{
				"type":                  "vtext_agent_revision",
				"original_prompt":       "research the sources and run one command",
				"scheduled_message_seq": int64(3),
			},
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := initialVTextToolChoice(&types.RunRecord{
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
