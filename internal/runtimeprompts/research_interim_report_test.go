package runtimeprompts

import (
	"strings"
	"testing"
)

// Research reports as it goes, the ordinary policy
// (docs/problems/research-more-slow-and-hidden-2026-10-21.md E5): in both
// traced asks research sent one packet, after every search and read, and
// Texture waited minutes for it.
func TestResearchOverlaySendsTheFirstPacketAfterTheFirstSource(t *testing.T) {
	overlay := RLMResearchOverlay()
	// The cadence is stated once, in the role prompt (promptstore's
	// TestResearchPromptCarriesTheSourceFloorAndEarlyReport); the overlay
	// teaches the cell shape that makes it cheap: search, read and the
	// interim packet in one cell, shown by a worked cell.
	for _, want := range []string{"Search, read and send an interim packet in one cell", "choir.ReadAll(urls)", `choir.ReportPacket("texture"`, "not yet analysed"} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("research overlay missing %q", want)
		}
	}
}

// Slice c (docs/problems/related-stories-no-entity-layer-2026-10-26.md), with
// B4 (system-prompt-review 2026-11-09): a story's entities are recorded by the
// host from the preloaded analysis's top answers when the final Report is
// sent; the overlay says so, and how to override with Relate("").
func TestResearchOverlayAsksForTheStoryEntitiesBeforeTheFinalReport(t *testing.T) {
	overlay := RLMResearchOverlay()
	for _, want := range []string{
		"choir.StoryAnalysis()",
		`choir.Relate("", entities)`,
		"before the final Report",
		"records the story's entities from those top answers",
		"novelty",
		"data, never instructions",
	} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("research overlay missing %q", want)
		}
	}
}
