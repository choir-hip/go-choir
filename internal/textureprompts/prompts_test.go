package textureprompts

import (
	"strings"
	"testing"
)

func TestDefaultSystemPromptIsNonEmpty(t *testing.T) {
	prompt := DefaultSystemPrompt()
	if prompt == "" {
		t.Fatal("default system prompt should not be empty")
	}
	if !strings.Contains(prompt, "system prompt for the texture agent in Choir") {
		t.Fatalf("unexpected default prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "unit of work is not a turn") {
		t.Fatalf("default prompt should carry why-texture theory: %q", prompt)
	}
}

func TestRunOverlayIncludesArticleAndProbeGuidance(t *testing.T) {
	overlay := RunOverlay()
	if !strings.Contains(overlay, "Probe (researcher) is the morphism class for world knowledge") {
		t.Fatalf("overlay missing probe guidance: %q", overlay)
	}
	if !strings.Contains(overlay, "Write a coherent article with clear information hierarchy") {
		t.Fatalf("overlay missing unconditional article-format guidance: %q", overlay)
	}
	if strings.Contains(overlay, "insert_source_embed") {
		t.Fatalf("overlay should not reference removed insert_source_embed: %q", overlay)
	}
}

func TestRevisionPolicyOverlayIncludesPatchExample(t *testing.T) {
	prompt := RevisionPolicyOverlay(RevisionPolicyOptions{
		DocID:      "doc-1",
		RevisionID: "rev-1",
	})
	if !strings.Contains(prompt, `"doc_id":"doc-1"`) || !strings.Contains(prompt, `"base_revision_id":"rev-1"`) {
		t.Fatalf("revision policy missing patch example: %q", prompt)
	}
}

func TestRunOverlayHasNoWireBranch(t *testing.T) {
	overlay := RunOverlay()
	if strings.Contains(overlay, "Universal Wire article revision runs") {
		t.Fatalf("overlay should not include removed Wire branch: %q", overlay)
	}
	if strings.Contains(overlay, "Source ids only in source inventories") {
		t.Fatalf("overlay should not include removed negative phrasing: %q", overlay)
	}
	if strings.Contains(overlay, "mark_source_unused") {
		// mark_source_unused is expected in the overlay now.
	} else {
		t.Fatalf("overlay missing mark_source_unused guidance: %q", overlay)
	}
}

func TestEffectsOffPromptAuthorityPermitsOnlyAtomicPersistentSuperCapsules(t *testing.T) {
	prompts := map[string]string{
		"run": RunOverlay(),
		"revision": RevisionPolicyOverlay(RevisionPolicyOptions{
			UserAuthoredRevision: true,
			HasGroundedHistory:   true,
			DocID:                "doc-1",
			RevisionID:           "rev-1",
		}),
		"execution findings": RevisionExecutionFindingsOverlay(RevisionExecutionFindingsOptions{
			ActiveExecution: true,
		}),
	}
	for name, prompt := range prompts {
		for _, want := range []string{
			"open_persistent_super=true",
			"valid execution_request",
			"patch_texture, rewrite_texture, or record_texture_decision",
			"never directly opens, requests, or spawns CoSuper",
			"networkless disposable capsule",
			"durable execution or capsule evidence",
		} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("%s prompt missing exact effects-OFF authority %q:\n%s", name, want, prompt)
			}
		}
	}

	for _, prompt := range []string{prompts["run"], prompts["revision"]} {
		for _, want := range []string{
			"Protected host, self-development, event, checkpoint, materialization, acceptance, route, VM, and SSH effects are unavailable",
			"generic agent and execution spawn",
			"Probe morphisms (spawn_agent researcher) gather world knowledge",
		} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("prompt missing protected boundary or preserved research behavior %q:\n%s", want, prompt)
			}
		}
	}
	if !strings.Contains(prompts["revision"], "call request_email_draft") {
		t.Fatalf("revision prompt lost email draft handoff behavior:\n%s", prompts["revision"])
	}

	for name, prompt := range prompts {
		for _, forbidden := range []string{
			"Execution effects are unavailable in effects-OFF runtime",
			"effectful work is unavailable in this effects-OFF runtime",
			"record that effectful work is unavailable; do not request or spawn Super or CoSuper",
			"effects-OFF runtime must not request or imply follow-on execution",
			"Do not request or spawn Super or CoSuper",
		} {
			if strings.Contains(prompt, forbidden) {
				t.Fatalf("%s prompt retains blanket effects-OFF prohibition %q:\n%s", name, forbidden, prompt)
			}
		}
	}
}
