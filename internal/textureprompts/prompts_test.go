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
