package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/runtime/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

func TestDefaultTexturePromptUsesDecisionNotesWithoutForcedSemanticSequence(t *testing.T) {
	prompt := textureprompts.DefaultSystemPrompt()
	normalizedPrompt := strings.Join(strings.Fields(prompt), " ")
	for _, want := range []string{
		"system prompt for the texture agent in Choir",
		"unit of work is not a turn",
		"idea level, not action by action",
		"Model priors may shape structure and tone, but they are not evidence",
		"marginal returns diminish",
		"Canonical document text is reader-facing belief state",
		"off-document decision texts",
		"Texture owns meaning and learning",
		"Super owns privileged execution",
		"chooses among them agentically",
	} {
		if !strings.Contains(normalizedPrompt, want) {
			t.Fatalf("default texture prompt missing %q:\n%s", want, prompt)
		}
	}
	assertNoForcedSemanticDelegation(t, prompt)
}

func TestRecordTextureDecisionToolDescriptionKeepsDecisionsOffDocument(t *testing.T) {
	tool := newRecordTextureDecisionTool(&Runtime{})
	if !strings.Contains(tool.Description, "outside the canonical document") ||
		!strings.Contains(tool.Description, "owner explicitly asks Texture to record an off-document decision note") ||
		!strings.Contains(tool.Description, "Do not use it for ordinary sentence-level edits") ||
		!strings.Contains(tool.Description, "do not put agent process rationale into document text") {
		t.Fatalf("record_texture_decision description is too weak: %q", tool.Description)
	}
	if _, ok := tool.Parameters["properties"].(map[string]any)["decision_kind"]; !ok {
		t.Fatalf("record_texture_decision schema missing decision_kind: %#v", tool.Parameters)
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
			t.Fatalf("texture prompt contains forced semantic delegation %q:\n%s", forbidden, prompt)
		}
	}
}

func TestTexturePromptInitialRevisionUsesSingleWriterLoop(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-current-events",
		RevisionID: "rev-current-events",
		Content:    "what's going on with iran deal now",
		AuthorKind: types.AuthorUser,
	}
	request := buildAgentRevisionRequest(current, nil, nil, textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "what's going on with iran deal now",
	}, "", false, nil, nil)

	for _, want := range []string{
		"Invariant: canonical meaning is Texture-owned",
		"For factual/current/search requests, do not answer substantive world facts from model recall",
		"immediate model-prior/interim V1 is allowed before retrieval only as an explicitly uncertain scaffold",
		"Probe morphisms (spawn_agent researcher) gather world knowledge",
		"depth scales with subject matter",
		"marginal returns diminish",
		"Worker messages can wake later texture runs and trigger the next revision.",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("initial texture prompt missing %q:\n%s", want, request)
		}
	}
	assertNoForcedSemanticDelegation(t, request)
}

func TestTexturePromptForFactualFirstRevisionForbidsUngroundedContent(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-nba",
		RevisionID: "rev-nba",
		Content:    "nba update",
		AuthorKind: types.AuthorUser,
	}
	request := buildAgentRevisionRequest(current, nil, nil, textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "nba update",
	}, "", false, nil, nil)

	for _, want := range []string{
		"do not answer substantive world facts from model recall",
		"immediate model-prior/interim V1 is allowed before retrieval only as an explicitly uncertain scaffold",
		"Do not add factual claims, citations, or coding results from model priors as grounded",
		"Probe and/or Execute morphisms are required",
		"violates invariant 3 unless Texture Audits an audit-worthy reason",
		"Never describe coordination as already done unless the tool action really happened",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("factual first-revision prompt missing %q:\n%s", want, request)
		}
	}
	assertNoForcedSemanticDelegation(t, request)
}

func TestTexturePromptUsesDiffFirstContextForDirectUserEdits(t *testing.T) {
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
	request := buildAgentRevisionRequest(current, previous, nil, textureAgentRevisionRequest{
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

func TestTexturePromptBarOwnerPromptIsCanonicalV0(t *testing.T) {
	reason := "M3.2 staging proof: user supplied the needed content and requested no research or execution worker."
	promptText := "Create a short Texture document. Record an off-document Texture decision note with exact reason " + reason + "."
	current := types.Revision{
		DocID:      "doc-prompt-bar-intake",
		RevisionID: "rev-prompt-bar-intake",
		Content:    promptText,
		AuthorKind: types.AuthorUser,
	}
	metadata := map[string]any{
		"seed_prompt":               promptText,
		"input_origin":              textureInputOriginUserPrompt,
		textureMetadataPromptUnixTS: int64(1718582400), // 2024-06-17T00:00:00Z
	}
	request := buildAgentRevisionRequest(current, nil, metadata, textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
	}, "", false, nil, nil)

	for _, want := range []string{
		"This canonical V0 content is the owner's original prompt/request for this Texture document.",
		"Treat the owner prompt as the request to fulfill: author the first useful reader-facing revision that addresses it.",
		"Keep private coordination rationale, explicit off-document decision reasons, and tool instructions out of the canonical document body",
		"Current canonical document content:\n---\n" + promptText + "\n---",
		"Owner prompt reference time: 2024-06-17T00:00:00Z UTC (Unix 1718582400)",
		`authoritative "now" when interpreting relative time words`,
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("prompt-bar owner-prompt V0 prompt missing %q:\n%s", want, request)
		}
	}
	for _, forbidden := range []string{
		"Treat this latest user-authored revision as the canonical input for the next version.",
		"Interpret the user edit diff as the instruction-bearing control surface.",
		"intentionally blank canonical document state",
		"(empty document)",
	} {
		if strings.Contains(request, forbidden) {
			t.Fatalf("prompt-bar owner-prompt V0 prompt contains forbidden blank/edit wording %q:\n%s", forbidden, request)
		}
	}
}

func TestTexturePromptFocusesLongDirectUserEdits(t *testing.T) {
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
	request := buildAgentRevisionRequest(current, previous, nil, textureAgentRevisionRequest{
		Intent: "revise",
	}, "added a user note and cleaner appendix table wording", false, nil, nil)

	for _, want := range []string{
		"Focused current-head context for this long user-authored draft:",
		"Full document omitted for ordinary long-document edit latency.",
		"User note: replace the stale paragraph above with the cleaner appendix table wording.",
		"Cleaner appendix table wording.",
		"call patch_texture",
		"structured operations against the current base revision",
		"complete current document is intentionally not preloaded",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("focused long-edit prompt missing %q:\n%s", want, request)
		}
	}
	for _, forbidden := range []string{
		`"op":"replace"`,
		`"find":"exact previous text"`,
		`"op":"append","text":"section text"`,
	} {
		if strings.Contains(request, forbidden) {
			t.Fatalf("focused long-edit prompt retained old operation example %q:\n%s", forbidden, request)
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
	if got := textureAgentRevisionContextMode(current, previous); got != "focused_user_edit_diff" {
		t.Fatalf("context mode = %q, want focused_user_edit_diff", got)
	}
}

func TestTexturePromptUsesStructuredPatchTextureOperationContract(t *testing.T) {
	bodyDoc, err := json.Marshal(texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-structured"},
			Content: []texturedoc.Node{{
				Type:  "heading",
				Attrs: map[string]any{"id": "heading-brief", "level": 2},
				Content: []texturedoc.Node{{
					Type: "text",
					Text: "Brief",
				}},
			}, {
				Type:  "paragraph",
				Attrs: map[string]any{"id": "paragraph-claim"},
				Content: []texturedoc.Node{{
					Type: "text",
					Text: "The claim needs source-backed revision.",
				}, {
					Type: "source_ref",
					Attrs: map[string]any{
						"id":               "ref-claim",
						"source_entity_id": "src-claim",
						"display_mode":     "numbered_ref",
					},
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("marshal body doc: %v", err)
	}
	current := types.Revision{
		DocID:      "doc-structured",
		RevisionID: "rev-structured",
		Content:    "## Brief\n\nThe claim needs source-backed revision. [1]",
		BodyDoc:    bodyDoc,
		AuthorKind: types.AuthorAppAgent,
	}
	request := buildAgentRevisionRequest(current, nil, nil, textureAgentRevisionRequest{
		Intent: "revise",
		Prompt: "Tighten the sourced claim.",
	}, "", false, nil, nil)

	for _, want := range []string{
		"Structured document outline for patch_texture block/node ids:",
		"- heading id=heading-brief level=2 text=\"Brief\"",
		"- paragraph id=paragraph-claim text=\"The claim needs source-backed revision.\"",
		"- source_ref id=ref-claim source_entity_id=src-claim display_mode=numbered_ref",
		"patch_texture accepts structured document operations only: update_block_text, insert_block, append_block, delete_node, insert_source_ref, and mark_source_unused",
		`"op":"update_block_text"`,
		`"block_id":"block id from the structured outline"`,
		`"op":"append_block"`,
		"To attach evidence from a listed source entity, use insert_source_ref with the exact source_entity_id value and place it after the text it supports",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("structured patch prompt missing %q:\n%s", want, request)
		}
	}
	for _, forbidden := range []string{
		`"op":"replace"`,
		`"find":"exact previous text"`,
		`"op":"append","text":"section text"`,
		"replace_all",
		"Canonical inline Source Entity syntax is [label](source:ENTITY_ID)",
	} {
		if strings.Contains(request, forbidden) {
			t.Fatalf("structured patch prompt retained old operation/source contract %q:\n%s", forbidden, request)
		}
	}
}

func TestTexturePromptForPartialFindingsForbidsFalseFollowupClaims(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-baseball",
		RevisionID: "rev-baseball-v2",
		Content:    "# Baseball\n\nPartial findings received; final scores are still missing.",
		AuthorKind: types.AuthorAppAgent,
	}
	recent := []ChannelMessage{{
		Role:    agentprofile.Researcher,
		From:    "researcher:one",
		Content: "Findings: identified matchups, but final scores are still unavailable from this packet.",
	}}
	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"seed_prompt": "Last Night in Baseball",
	}, textureAgentRevisionRequest{
		Intent: "integrate_worker_findings",
		Prompt: "Last Night in Baseball",
	}, "", true, recent, nil)

	for _, want := range []string{
		"This Texture run was woken by worker source packets",
		"Make the useful claims and packet.sources visible with patch_texture as this turn's next document revision before spawning additional workers",
		"If recent worker source packets are only partial and the document needs more evidence",
		"write only the reader-facing artifact state that the usable claims and packet.sources support",
		"Do not paste process metadata, source-status notes, or checkpoint labels into the canonical document body",
		"Do not write that a follow-up researcher was dispatched",
		"Never describe coordination as already done unless the tool action really happened",
		"Phrases such as \"researcher dispatched\"",
		"If you only patch_texture or rewrite_texture, phrase remaining work as \"next needed\" or \"still unresolved\"",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("partial-findings prompt missing %q:\n%s", want, request)
		}
	}
}

func TestTexturePromptNarrativeRoleWordsDoNotSwitchPolicyBranches(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-role-words",
		RevisionID: "rev-role-words",
		Content:    "Ask researcher to inspect the code, then deploy and verify it.",
		AuthorKind: types.AuthorUser,
	}
	recent := []ChannelMessage{{
		Role:    agentprofile.Researcher,
		From:    "researcher:one",
		Content: "A usable source packet is ready for incorporation.",
	}}
	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"seed_prompt": "Ask researcher to inspect the code, then deploy and verify it.",
	}, textureAgentRevisionRequest{
		Intent: "integrate_worker_findings",
		Prompt: "Ask researcher to inspect the code, then deploy and verify it.",
	}, "", true, recent, nil)

	if !strings.Contains(request, "This Texture run was woken by worker source packets") {
		t.Fatalf("narrative execution words suppressed worker incorporation:\n%s", request)
	}
	for _, forbidden := range []string{
		"The owner explicitly asked for researcher help.",
		"The original request still needs Execute evidence",
	} {
		if strings.Contains(request, forbidden) {
			t.Fatalf("narrative role words switched policy branch %q:\n%s", forbidden, request)
		}
	}

	structured := buildAgentRevisionRequest(current, nil, map[string]any{
		runMetadataExplicitResearcher: true,
	}, textureAgentRevisionRequest{Intent: "initial_conductor_workflow"}, "", true, nil, nil)
	if !strings.Contains(structured, "The owner explicitly asked for researcher help.") {
		t.Fatalf("structured researcher intent did not select the policy branch:\n%s", structured)
	}
}

func TestTexturePromptPreservesExplicitHardConstraints(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-long-rubric",
		RevisionID: "rev-long-rubric",
		Content:    "1. Direct user edits\nEvidence pending.\n\nUSER_LONG_RUBRIC_MARKER: preserve this exact marker.",
		AuthorKind: types.AuthorUser,
	}
	request := buildAgentRevisionRequest(current, nil, nil, textureAgentRevisionRequest{
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
		"Before rewrite_texture, audit the complete replacement against those hard requirements",
		"Do not replace a requested numbered/sectioned document with a different report outline",
		"Never use `[CMD]` as a pending/requested/target-only label, including in the initial v1 scaffold",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("hard-constraint prompt missing %q:\n%s", want, request)
		}
	}
}

func TestTexturePromptDoesNotPreserveLegacyInlineSourceLinks(t *testing.T) {
	sourceEntities, err := json.Marshal([]texturedoc.SourceEntity{{
		SourceEntityID: "src-youtube-demo",
		Target: texturedoc.SourceTarget{
			Kind: "video",
			URI:  "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		Display: texturedoc.SourceDisplay{
			Mode:  "numbered_ref",
			Title: "Demo clip",
		},
		Evidence: texturedoc.SourceEvidence{
			ResearchState: "pending",
			OpenSurface:   "video",
		},
	}})
	if err != nil {
		t.Fatalf("marshal source entities: %v", err)
	}
	current := types.Revision{
		DocID:          "doc-source-ref",
		RevisionID:     "rev-source-ref",
		Content:        "# Source Review\n\nThis claim cites [the clip]" + "(source:src-youtube-demo).",
		SourceEntities: sourceEntities,
		AuthorKind:     types.AuthorUser,
	}
	request := buildAgentRevisionRequest(current, nil, nil, textureAgentRevisionRequest{
		Intent: "revise",
		Prompt: "Keep the citation attached while making the wording clearer.",
	}, "", false, nil, nil)

	for _, want := range []string{
		"Detected Texture source entities:",
		"video Demo clip entity_id=src-youtube-demo",
		"Preserve existing source_entity_id values exactly",
		"call patch_texture with insert_source_ref using the listed entity_id/source_entity_id value",
		"Do not write markdown links, source inventories, or Source: lines as substitutes",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("source-ref prompt missing %q:\n%s", want, request)
		}
	}
	for _, forbidden := range []string{
		"Canonical inline Source Entity syntax is [label](source:ENTITY_ID)",
		"Preserve inline source ref " + "exactly",
		"Preserve source entity identity from legacy inline " + "source ref",
	} {
		if strings.Contains(request, forbidden) {
			t.Fatalf("source-ref prompt retained forbidden old source-link instruction %q:\n%s", forbidden, request)
		}
	}
}

func TestTextureContentItemSourceEntityDefaultsToWholeResource(t *testing.T) {
	// After the D3 cutover, content-item source entities are whole_resource by
	// default; text_quote selectors (and their quote-match validation) come from
	// typed researcher findings, never from regex-scraping prose context.
	entity := contentItemRefToSourceEntity(types.ContentItem{
		ContentID:    "content-cloud-audit",
		Title:        "Cloud auditability source",
		SourceURL:    "https://example.com/cloud-audit",
		CanonicalURL: "https://example.com/cloud-audit",
		ContentHash:  "sha256-cloud-audit",
	})
	if entity.Kind != "content_item" ||
		entity.Target.TargetKind != "content_item" ||
		entity.Target.ContentID != "content-cloud-audit" ||
		entity.Display.OpenSurface != "source" ||
		entity.Evidence.ResearchState != "represented" ||
		len(entity.Selectors) != 1 ||
		entity.Selectors[0].SelectorKind != "whole_resource" ||
		entity.Selectors[0].ContentHash != "sha256-cloud-audit" ||
		entity.Selectors[0].TextQuote != "" {
		t.Fatalf("derived content item source entity = %#v", entity)
	}
}

func TestTexturePromptRestoresFinalCommandEvidenceRequirementAfterSuperDelivery(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-long-rubric-super",
		RevisionID: "rev-long-rubric-super",
		Content:    "1. Direct user edits\nEvidence pending.\n\nUSER_LONG_RUBRIC_MARKER: preserve this exact marker.",
		AuthorKind: types.AuthorAppAgent,
	}
	recent := []ChannelMessage{{
		Role:    agentprofile.Super,
		From:    "super:one",
		Content: "Worker update ready.\n\nFindings:\n- [CMD] command exited 0 and printed the expected hash.",
	}}
	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"seed_prompt": "The final Source Ledger must include [S1], [S2], [S3], and [CMD].",
	}, textureAgentRevisionRequest{
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

func TestTexturePromptMixedObligationKeepsGeneralExecuteAffordanceWithoutKeywordBranch(t *testing.T) {
	current := types.Revision{
		DocID:      "doc-mixed-obligation",
		RevisionID: "rev-mixed-obligation",
		Content:    "# Working brief\n\nCommand evidence pending.",
		AuthorKind: types.AuthorAppAgent,
	}
	recent := []ChannelMessage{{
		Role:    agentprofile.Researcher,
		From:    "researcher:one",
		Content: "Worker update ready.\n\nFindings:\n- [S1] Texture documents have durable revisions.",
	}}
	request := buildAgentRevisionRequest(current, nil, map[string]any{
		"seed_prompt": "Research Texture durable drafts and run exactly one command: printf \"durable draft\" | shasum -a 256. The final Source Ledger must include [S1], [S2], [S3], and [CMD].",
	}, textureAgentRevisionRequest{
		Intent: "integrate_worker_findings",
	}, "", true, recent, nil)

	for _, want := range []string{
		"This Texture run was woken by worker source packets",
		"Make the useful claims and packet.sources visible with patch_texture",
		"If the follow-up needs generated artifacts, execution, or verification, Execute (request_super_execution) is the morphism class for super-delivered evidence.",
		"if Texture does not use it, Audit the blocker instead of making a source-grounded edit look final",
		"Never use `[CMD]` as a pending/requested/target-only label",
	} {
		if !strings.Contains(request, want) {
			t.Fatalf("mixed-obligation prompt missing %q:\n%s", want, request)
		}
	}
	for _, forbidden := range []string{
		"recent worker messages do not include a super delivery",
		"The original request still needs Execute evidence",
	} {
		if strings.Contains(request, forbidden) {
			t.Fatalf("mixed-obligation narrative selected keyword branch %q:\n%s", forbidden, request)
		}
	}
	assertNoForcedSemanticDelegation(t, request)
}

func TestInitialTextureToolChoiceOnlyConstrainsMechanicalContinuations(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		want     string
	}{
		{
			name: "current factual work starts unconstrained",
			metadata: map[string]any{
				"type":            "texture_agent_revision",
				"original_prompt": "what is the weather in boston now",
			},
			want: "",
		},
		{
			name: "mutable product work starts unconstrained",
			metadata: map[string]any{
				"type":            "texture_agent_revision",
				"original_prompt": "debug and fix the runtime gateway",
			},
			want: "",
		},
		{
			name: "community wire operational proof starts unconstrained",
			metadata: map[string]any{
				"type":        "texture_agent_revision",
				"seed_prompt": "Universal Wire staging proof request: run the existing source-refresh/research/projection/publication flow, create or approve an Article Texture, update universal-wire/Wire.texture, then leave evidence ids and verifier proof.",
			},
			want: "",
		},
		{
			name: "creative direct document work starts unconstrained",
			metadata: map[string]any{
				"type":            "texture_agent_revision",
				"original_prompt": "tell me a story about computers",
			},
			want: "",
		},
		{
			name: "explicit decision note starts unconstrained",
			metadata: map[string]any{
				"type":            "texture_agent_revision",
				"original_prompt": "Create a short Texture document. Record an off-document Texture decision note with decision_kind no_worker_needed first.",
			},
			want: "",
		},
		{
			name: "direct user-authored revise requires durable action but not exact patch",
			metadata: map[string]any{
				"type":                "texture_agent_revision",
				"request_intent":      "revise",
				"current_author_kind": string(types.AuthorUser),
				"original_prompt":     "Research this and show visible work state while evidence is pending.",
			},
			want: "required",
		},
		{
			name: "scheduled non-coagent run leaves texture free to choose",
			metadata: map[string]any{
				"type":                  "texture_agent_revision",
				"original_prompt":       "research the sources and run one command",
				"scheduled_message_seq": int64(3),
			},
			want: "",
		},
		{
			name: "grounded integrate wake requires a durable action",
			metadata: map[string]any{
				"type":                  "texture_agent_revision",
				"original_prompt":       "integrate the researcher findings",
				"scheduled_message_seq": int64(3),
				"request_source":        "update_coagent",
			},
			want: "required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := initialTextureToolChoice(&types.RunRecord{
				Metadata: tc.metadata,
			})
			if got != tc.want {
				t.Fatalf("initialTextureToolChoice = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTextureActorToolLoopBudgetDefaultsAndOverrides(t *testing.T) {
	rec := &types.RunRecord{
		ChannelID: "doc-channel",
		Metadata: map[string]any{
			"type":   "texture_agent_revision",
			"doc_id": "doc-budget",
		},
	}
	budget := textureActorToolLoopBudget(rec)
	if budget.Label != "texture:doc-budget" {
		t.Fatalf("label = %q, want texture:doc-budget", budget.Label)
	}
	if budget.MaxProviderCalls != defaultTextureActorMaxProviderCalls {
		t.Fatalf("max provider calls = %d, want default %d", budget.MaxProviderCalls, defaultTextureActorMaxProviderCalls)
	}
	if budget.MaxTotalTokens != defaultTextureActorMaxTotalTokens {
		t.Fatalf("max total tokens = %d, want default %d", budget.MaxTotalTokens, defaultTextureActorMaxTotalTokens)
	}
	if budget.MaxElapsed != defaultTextureActorMaxElapsed {
		t.Fatalf("max elapsed = %s, want %s", budget.MaxElapsed, defaultTextureActorMaxElapsed)
	}

	rec.Metadata["actor_budget_max_provider_calls"] = int64(7)
	rec.Metadata["actor_budget_max_input_tokens"] = int64(1000)
	rec.Metadata["actor_budget_max_output_tokens"] = int64(2000)
	rec.Metadata["actor_budget_max_total_tokens"] = int64(3000)
	rec.Metadata["actor_budget_max_elapsed_seconds"] = int64(90)
	rec.Metadata["actor_budget_spent_provider_calls"] = int64(2)
	rec.Metadata["actor_budget_spent_input_tokens"] = int64(100)
	rec.Metadata["actor_budget_spent_output_tokens"] = int64(200)
	budget = textureActorToolLoopBudget(rec)
	if budget.MaxProviderCalls != 7 ||
		budget.MaxInputTokens != 1000 ||
		budget.MaxOutputTokens != 2000 ||
		budget.MaxTotalTokens != 3000 ||
		budget.MaxElapsed != 90*time.Second ||
		budget.SpentProviderCalls != 2 ||
		budget.SpentInputTokens != 100 ||
		budget.SpentOutputTokens != 200 {
		t.Fatalf("override budget = %+v", budget)
	}
}

func TestExplicitNoWorkerDecisionParsesWithoutNarrativeRouteOracle(t *testing.T) {
	prompt := strings.Join([]string{
		"Create a short Texture document for a deployed staging proof.",
		"Because this task is fully supplied and requires no research or execution worker,",
		"record an off-document Texture decision note with decision_kind no_worker_needed.",
		"Then write the concise reader-facing Texture revision.",
	}, " ")
	if !texturePromptExplicitlyRequestsDecisionNote(prompt) {
		t.Fatal("test prompt should explicitly request a decision note")
	}
	if !texturePromptExplicitlyRequestsNoWorkerDecision(prompt) {
		t.Fatal("no-worker decision note prompt should parse as an explicit decision request")
	}

	if texturePromptExplicitlyRequestsNoWorkerDecision("Debug and fix the runtime gateway, run tests, and verify the staging proof.") {
		t.Fatal("ordinary mutation prompt must not parse as a no-worker decision")
	}
}

func TestExplicitNoWorkerDecisionPromptParsesInitialDecision(t *testing.T) {
	prompt := strings.Join([]string{
		"Create a short Texture document titled M32_TEXTURE_DECISION_ROUTE_TEST.",
		"Because this task is fully supplied and requires no research or execution worker,",
		"record an off-document Texture decision note with decision_kind no_worker_needed,",
		"exact reason M3.2 staging proof: user supplied the needed content and requested no research or execution worker.,",
		"evidence ref staging-marker:M32_TEXTURE_DECISION_ROUTE_TEST,",
		"next action Write the concise reader-facing Texture revision.",
		"Then write the concise reader-facing Texture revision.",
	}, " ")
	decision, ok := explicitNoWorkerDecisionRequestFromPrompt(prompt)
	if !ok {
		t.Fatal("proof-style prompt should parse as an explicit initial decision")
	}
	if decision.DecisionKind != "no_worker_needed" {
		t.Fatalf("decision kind = %q", decision.DecisionKind)
	}
	if decision.Reason != "M3.2 staging proof: user supplied the needed content and requested no research or execution worker." {
		t.Fatalf("reason = %q", decision.Reason)
	}
	if len(decision.EvidenceRefs) != 1 || decision.EvidenceRefs[0] != "staging-marker:M32_TEXTURE_DECISION_ROUTE_TEST" {
		t.Fatalf("evidence refs = %#v", decision.EvidenceRefs)
	}
	if decision.NextAction != "Write the concise reader-facing Texture revision" {
		t.Fatalf("next action = %q", decision.NextAction)
	}
}
