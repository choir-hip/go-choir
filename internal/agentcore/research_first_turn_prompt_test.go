package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

// Research's first turn asks for an interim packet after the first read
// source, not one packet at the end
// (docs/problems/research-more-slow-and-hidden-2026-10-21.md E5).
func TestResearchFirstTurnAsksForAnEarlyInterimPacket(t *testing.T) {
	if p, _ := recordsInboxPromptFor(agentprofile.Research); !strings.Contains(p, "as soon as you have read one new source you can cite") {
		t.Fatalf("research first-turn prompt: %s", p)
	}
}

// The first turn says the host searched the corpus for the objective and that
// its result is part of this message (research_brief.go), so the desk starts
// from what is held and does not search the corpus for the same words again.
func TestResearchFirstTurnNamesTheCorpusBrief(t *testing.T) {
	p, _ := recordsInboxPromptFor(agentprofile.Research)
	if !strings.Contains(p, "corpus brief") || !strings.Contains(p, "or a sentence saying why there is none") {
		t.Fatalf("research first-turn prompt does not mention the corpus brief: %s", p)
	}
}
