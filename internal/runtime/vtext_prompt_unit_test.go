package runtime

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestDefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence(t *testing.T) {
	raw, err := fs.ReadFile(promptDefaultsFS, "prompt_defaults/vtext.md")
	if err != nil {
		t.Fatalf("read default vtext prompt: %v", err)
	}
	prompt := string(raw)
	normalizedPrompt := strings.Join(strings.Fields(prompt), " ")
	for _, want := range []string{
		"VText owns canonical document versions",
		"Use `record_vtext_decision` for audit-worthy off-document choices",
		"If the owner explicitly asks VText to record an off-document decision note",
		"unless the requested record would be false, unsafe, or outside VText authority",
		"Do not put agent process rationale",
		"These are obligations and affordances, not a forced tool sequence",
		"VText may write, ask researcher, ask super, ask both, ask neither, wait, or report a blocker",
	} {
		if !strings.Contains(normalizedPrompt, want) {
			t.Fatalf("default vtext prompt missing %q:\n%s", want, prompt)
		}
	}
	assertNoForcedSemanticDelegation(t, prompt)
}

func TestRecordVTextDecisionToolDescriptionKeepsDecisionsOffDocument(t *testing.T) {
	tool := newRecordVTextDecisionTool(&Runtime{})
	if !strings.Contains(tool.Description, "outside the canonical document") ||
		!strings.Contains(tool.Description, "owner explicitly asks VText to record an off-document decision note") ||
		!strings.Contains(tool.Description, "Do not use it for ordinary sentence-level edits") ||
		!strings.Contains(tool.Description, "do not put agent process rationale into document text") {
		t.Fatalf("record_vtext_decision description is too weak: %q", tool.Description)
	}
	if _, ok := tool.Parameters["properties"].(map[string]any)["decision_kind"]; !ok {
		t.Fatalf("record_vtext_decision schema missing decision_kind: %#v", tool.Parameters)
	}
}

func assertNoForcedSemanticDelegation(t *testing.T, prompt string) {
	t.Helper()
	for _, forbidden := range []string{
		"call spawn_agent",
		"call spawn_agent with role=\"researcher\" in this run",
		"then call spawn_agent",
		"researcher spawn in the same run",
		"spawn a researcher",
		"Open new researcher work when",
		"call request_super_execution",
		"request_super_execution in the same run",
		"first call request_super_execution",
		"then request_super_execution",
		"Use request_super_execution when",
		"next side-effectful action should be request_super_execution",
		"start needed worker request before ending run",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("vtext prompt contains forced semantic delegation %q:\n%s", forbidden, prompt)
		}
	}
}

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
		"For factual/current/search requests, the first revision should be a short working brief with explicit uncertainty and no ungrounded claims; if more evidence is needed, researcher delegation is available as a VText choice.",
		"Worker messages can wake later vtext runs and trigger the next revision.",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("initial vtext prompt missing %q:\n%s", want, request)
		}
	}
	assertNoForcedSemanticDelegation(t, request)
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
		"write a brief working revision with explicit uncertainty and record what evidence is needed",
		"VText may then choose researcher, super, both, neither, or a blocker",
		"Never describe coordination as already done unless the tool action really happened",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("factual first-revision prompt missing %q:\n%s", want, request)
		}
	}
	assertNoForcedSemanticDelegation(t, request)
}

func TestVTextPromptUsesDiffFirstContextForDirectUserEdits(t *testing.T) {
	current := types.Revision{
		DocID:            "doc-direct-edit",
		RevisionID:       "rev-user-draft",
		ParentRevisionID: "rev-appagent-base",
		Content:          "# Brief\n\nMake this section sharper.\n\nOld paragraph rewritten as final prose.",
		AuthorKind:       types.AuthorUser,
	}
	previous := &types.Revision{
		DocID:      "doc-direct-edit",
		RevisionID: "rev-appagent-base",
		Content:    "# Brief\n\nOld paragraph.",
		AuthorKind: types.AuthorAppAgent,
	}
	request := buildAgentRevisionRequest(current, previous, nil, vtextAgentRevisionRequest{
		Intent: "revise",
	}, "replace Old paragraph with sharper final prose", false, nil, []string{
		"rev-1 old user diff that should not be preloaded",
		"rev-2 another old user diff that should not be preloaded",
	})

	for _, want := range []string{
		"User edit diff from previous canonical revision to current user-authored draft:",
		"Interpret the user edit diff as the instruction-bearing control surface",
		"Consume instruction-like text when it is not intended as final prose",
		"remove the stale target text instead of appending a competing alternative",
		"Do not require //edit markers",
		"Default context is intentionally small: current head plus the exact user edit diff",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("diff-first prompt missing %q:\n%s", want, request)
		}
	}
	for _, forbidden := range []string{
		"User-authored revision diffs (oldest to newest):",
		"rev-1 old user diff that should not be preloaded",
		"rev-2 another old user diff that should not be preloaded",
	} {
		if strings.Contains(request, forbidden) {
			t.Fatalf("diff-first prompt preloaded forbidden history %q:\n%s", forbidden, request)
		}
	}
}

func TestVTextPromptFocusesLongDirectUserEdits(t *testing.T) {
	var before strings.Builder
	before.WriteString("# Proposal\n\n")
	before.WriteString("Executive section before the edit.\n")
	for i := 0; i < 360; i++ {
		before.WriteString("Distant untouched appendix line that should not be preloaded into ordinary revise prompts.\n")
	}
	before.WriteString("Target section before user edit.\n")
	before.WriteString("Stale paragraph that should be replaced after the user instruction.\n")
	for i := 0; i < 80; i++ {
		before.WriteString("More untouched material after the edit.\n")
	}

	after := strings.Replace(before.String(),
		"Stale paragraph that should be replaced after the user instruction.\n",
		"Stale paragraph that should be replaced after the user instruction.\nUser note: replace the stale paragraph above with the cleaner appendix table wording.\nCleaner appendix table wording.\n",
		1)

	current := types.Revision{
		DocID:            "doc-long-direct-edit",
		RevisionID:       "rev-user-long-draft",
		ParentRevisionID: "rev-appagent-long-base",
		Content:          after,
		AuthorKind:       types.AuthorUser,
	}
	previous := &types.Revision{
		DocID:      "doc-long-direct-edit",
		RevisionID: "rev-appagent-long-base",
		Content:    before.String(),
		AuthorKind: types.AuthorAppAgent,
	}
	request := buildAgentRevisionRequest(current, previous, nil, vtextAgentRevisionRequest{
		Intent: "revise",
	}, "added a user note and cleaner appendix table wording", false, nil, nil)

	for _, want := range []string{
		"Focused current-head context for this long user-authored draft:",
		"Full document omitted for ordinary long-document edit latency.",
		"User note: replace the stale paragraph above with the cleaner appendix table wording.",
		"Cleaner appendix table wording.",
		"operation\":\"apply_edits",
		"complete current document is intentionally not preloaded",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("focused long-edit prompt missing %q:\n%s", want, request)
		}
	}
	if strings.Contains(request, "Current canonical document content:") {
		t.Fatalf("long direct-edit prompt should not include full current content section:\n%s", request)
	}
	if strings.Count(request, "Distant untouched appendix line that should not be preloaded") > 12 {
		t.Fatalf("long direct-edit prompt preloaded too much untouched content; count=%d len=%d", strings.Count(request, "Distant untouched appendix line that should not be preloaded"), len(request))
	}
	if len(request) >= len(after) {
		t.Fatalf("focused prompt len=%d should be smaller than current content len=%d", len(request), len(after))
	}
	if got := vtextAgentRevisionContextMode(current, previous); got != "focused_user_edit_diff" {
		t.Fatalf("context mode = %q, want focused_user_edit_diff", got)
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

func TestVTextPromptDerivesSourceServiceEntitiesFromResearcherUpdates(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-source-service",
		RevisionID: "rev-source-service-v1",
		Content:    "# Source Service Brief\n\nResearch checkpoint pending.",
		AuthorKind: types.AuthorAppAgent,
	}
	recent := []ChannelMessage{{
		Role: AgentProfileResearcher,
		From: "researcher:source",
		Content: strings.Join([]string{
			"Coagent update ready.",
			"Role: researcher.",
			"Kind: findings.",
			"Summary: Source Service returned current evidence.",
			"",
			"Findings:",
			"- The source-service result identified a current economy item.",
			"",
			"Refs:",
			"- source_service_item:srcitem_current_economy",
			"- Source Service Item ID: srcitem_labor_market_signal",
			"- source:gdelt:15min",
		}, "\n"),
	}}
	sourceEntities, changed := mergeVTextSourceEntities(nil, sourceServiceEntitiesFromWorkerMessages(recent))
	if !changed || len(sourceEntities) != 2 {
		t.Fatalf("source entities changed=%v len=%d: %#v", changed, len(sourceEntities), sourceEntities)
	}
	if sourceEntities[0].Target.TargetKind != "source_service_item" ||
		sourceEntities[0].Target.ItemID != "srcitem_current_economy" ||
		sourceEntities[0].Display.InlineMode != "collapsed_citation" ||
		sourceEntities[0].Display.OpenSurface != "source" ||
		sourceEntities[0].Evidence.ResearchState != "represented" {
		t.Fatalf("derived source entity = %#v", sourceEntities[0])
	}
	if sourceEntities[1].Target.TargetKind != "source_service_item" ||
		sourceEntities[1].Target.ItemID != "srcitem_labor_market_signal" {
		t.Fatalf("derived raw item source entity = %#v", sourceEntities[1])
	}

	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"source_entities": sourceEntities,
		"seed_prompt":     "Write a source-service grounded brief.",
	}, vtextAgentRevisionRequest{
		Intent: "integrate_worker_findings",
		Prompt: "Write a source-service grounded brief.",
	}, "", true, recent, nil)

	for _, want := range []string{
		"Detected VText source entities:",
		"source_service_item",
		"item_id=srcitem_current_economy",
		"Canonical inline Source Entity syntax is [label](source:ENTITY_ID)",
		"Make those findings visible with edit_vtext",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("source-service prompt missing %q:\n%s", want, request)
		}
	}
}

func TestVTextSourceServiceEntitiesResolveItemTitles(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/items/srcitem_current_economy" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(sourceapi.ResolveItemResponse{
			Provider: sourceapi.ProviderName,
			Item: sourceapi.ItemResult{
				TargetKind:   sourceapi.TargetKind,
				ItemID:       "srcitem_current_economy",
				SourceID:     "rss:market-wire",
				SourceType:   "rss",
				FetchID:      "fetch-market-wire",
				Title:        "Markets reprice rate-cut odds after inflation print",
				URL:          "https://example.test/markets/rates",
				CanonicalURL: "https://example.test/markets/rates",
				ContentHash:  "hash-rate-cut-odds",
			},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	recent := []ChannelMessage{{
		Role:    AgentProfileResearcher,
		From:    "researcher:source",
		Content: "Refs:\n- source_service_item:srcitem_current_economy",
	}}
	entities := (&Runtime{}).sourceEntitiesFromWorkerMessages(context.Background(), "user-source", recent)
	if len(entities) != 1 {
		t.Fatalf("source entities len = %d: %#v", len(entities), entities)
	}
	entity := entities[0]
	if entity.Label != "Markets reprice rate-cut odds after inflation print" ||
		entity.Target.SourceID != "rss:market-wire" ||
		entity.Target.FetchID != "fetch-market-wire" ||
		entity.Target.CanonicalURL != "https://example.test/markets/rates" ||
		len(entity.Selectors) != 1 ||
		entity.Selectors[0].ContentHash != "hash-rate-cut-odds" {
		t.Fatalf("source-service entity was not enriched from resolved item: %#v", entity)
	}
}

func TestVTextDerivesContentItemSourceEntitiesFromResearcherRefs(t *testing.T) {
	content := strings.Join([]string{
		"Findings:",
		"- The official source supports the bounded claim: \"Cloud providers should preserve auditability.\" content_id:content-cloud-audit",
		"- Duplicate JSON ref should not add a second entity: \"content_id\":\"content-cloud-audit\"",
	}, "\n")
	ids := contentItemIDsFromWorkerMessage(content)
	if len(ids) != 1 || ids[0] != "content-cloud-audit" {
		t.Fatalf("content item ids = %#v", ids)
	}

	entity := contentItemRefToSourceEntity(types.ContentItem{
		ContentID:    "content-cloud-audit",
		Title:        "Cloud auditability source",
		SourceURL:    "https://example.com/cloud-audit",
		CanonicalURL: "https://example.com/cloud-audit",
		ContentHash:  "sha256-cloud-audit",
	}, content)
	if entity.Kind != "content_item" ||
		entity.Target.TargetKind != "content_item" ||
		entity.Target.ContentID != "content-cloud-audit" ||
		entity.Display.OpenSurface != "source" ||
		entity.Evidence.ResearchState != "represented" ||
		entity.Selectors[0].SelectorKind != "text_quote" ||
		entity.Selectors[0].ContentHash != "sha256-cloud-audit" ||
		!strings.Contains(entity.Selectors[0].TextQuote, "Cloud providers should preserve auditability") {
		t.Fatalf("derived content item source entity = %#v", entity)
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
		"request_super_execution is available when VText chooses that the execution obligation is ready for super",
		"if VText does not use it, record the precise blocker or missing evidence",
		"Keep any request_super_execution objective concise and concrete",
		"must not use the final [CMD] evidence label before the super delivery arrives",
		"if VText does not use it, record the blocker instead of making a source-grounded edit look final",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("mixed-obligation prompt missing %q:\n%s", want, request)
		}
	}
	assertNoForcedSemanticDelegation(t, request)
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
			name: "mutable product work does not force super request",
			metadata: map[string]any{
				"type":            "vtext_agent_revision",
				"original_prompt": "debug and fix the runtime gateway",
			},
			want: "function:edit_vtext",
		},
		{
			name: "community wire operational proof does not force super request",
			metadata: map[string]any{
				"type":        "vtext_agent_revision",
				"seed_prompt": "Universal Wire staging proof request: run the existing source-refresh/research/projection/publication flow, create or approve an Article VText, update universal-wire/Wire.vtext, then leave evidence ids and verifier proof.",
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
			name: "explicit decision note starts with decision record",
			metadata: map[string]any{
				"type":            "vtext_agent_revision",
				"original_prompt": "Create a short VText document. Record an off-document VText decision note with decision_kind no_worker_needed first.",
			},
			want: "function:record_vtext_decision",
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
