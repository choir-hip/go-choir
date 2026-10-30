package modelcatalog

import (
	"slices"
	"strings"
	"testing"
)

// Owner 2026-10-15: vision is on by default for every configured model;
// text-only models opt out by name
// (docs/archive/problems/models-configured-text-only-2026-10-15.md).
func TestVisionIsTheDefaultForConfiguredModels(t *testing.T) {
	for _, id := range []string{
		"gpt-6-luna",
		"deepseek-v4.1-flash",
		"us.anthropic.claude-haiku-4-5-20251001-v1:0",
		"us.anthropic.claude-sonnet-4-6",
		"us.anthropic.claude-opus-4-6-v1",
	} {
		if !SupportsImage(id) {
			t.Errorf("%s: SupportsImage = false (%s), want vision on", id, ImageRefusal(id))
		}
	}
}

func TestTextOnlyOptOutIsRespected(t *testing.T) {
	textOnly := TextOnlyModels()
	if len(textOnly) == 0 {
		t.Fatal("opt-out list is empty")
	}
	for id, reason := range textOnly {
		if _, ok := lookupID(id); !ok {
			t.Errorf("opt-out names %q, which is not in the catalog: delete the line", id)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("opt-out for %q carries no reason", id)
		}
		if SupportsImage(id) {
			t.Errorf("%s is opted out but SupportsImage = true", id)
		}
		if refusal := ImageRefusal(id); !strings.Contains(refusal, "opted out") {
			t.Errorf("%s refusal = %q, want it to name the opt-out", id, refusal)
		}
	}
}

func TestEveryCatalogEntryDeclaresItsModalities(t *testing.T) {
	textOnly := TextOnlyModels()
	for _, m := range SupportedModels() {
		if !slices.Contains(m.Modalities, ModalityText) {
			t.Errorf("%s: modalities %v lack text", m.ID, m.Modalities)
		}
		_, optedOut := textOnly[m.ID]
		if got := slices.Contains(m.Modalities, ModalityImage); got == optedOut {
			t.Errorf("%s: image in modalities = %v, opted out = %v", m.ID, got, optedOut)
		}
		if !slices.Contains(m.AdapterModalities, ModalityImage) {
			t.Errorf("%s: adapter modalities %v lack image", m.ID, m.AdapterModalities)
		}
	}
}

func TestUncataloguedModelIsRefusedImages(t *testing.T) {
	if SupportsImage("accounts/fireworks/models/kimi-k2p6") {
		t.Fatal("an uncatalogued model must not be assumed vision-capable")
	}
	if refusal := ImageRefusal("mystery-model"); !strings.Contains(refusal, "not in the Choir model catalog") {
		t.Fatalf("refusal = %q", refusal)
	}
}

func lookupID(id string) (ModelInfo, bool) {
	for _, m := range SupportedModels() {
		if m.ID == id {
			return m, true
		}
	}
	return ModelInfo{}, false
}
