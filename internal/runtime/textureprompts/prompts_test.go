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
	if !strings.Contains(prompt, "system prompt of the Texture agent") {
		t.Fatalf("unexpected default prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "unit of work is not a turn") {
		t.Fatalf("default prompt should carry why-texture theory: %q", prompt)
	}
}

func TestRunOverlayIncludesWireBranch(t *testing.T) {
	general := RunOverlay(RunOverlayOptions{WireTexture: false})
	wire := RunOverlay(RunOverlayOptions{WireTexture: true})
	if !strings.Contains(general, "Probe (researcher) is the morphism class for world knowledge") {
		t.Fatalf("general overlay missing probe guidance: %q", general)
	}
	if !strings.Contains(wire, "Universal Wire article revision runs") {
		t.Fatalf("wire overlay missing wire guidance: %q", wire)
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

func TestRunOverlayWireBranchTemplate(t *testing.T) {
	general := RunOverlay(RunOverlayOptions{WireTexture: false})
	wire := RunOverlay(RunOverlayOptions{WireTexture: true})
	if strings.Contains(general, "Universal Wire article revision runs") {
		t.Fatalf("general overlay should not include wire branch: %q", general)
	}
	if !strings.Contains(wire, "Universal Wire article revision runs") {
		t.Fatalf("wire overlay missing wire branch: %q", wire)
	}
}
