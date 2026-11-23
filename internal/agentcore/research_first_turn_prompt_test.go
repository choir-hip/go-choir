package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

// Research's first turn asks for an interim packet when there is a claim to cite, not one
// packet at the end (docs/problems/research-more-slow-and-hidden-2026-10-21.md E5). It no
// longer says "as soon as you have read one new source, and another each time you have read
// more": that read as a reason to fetch another source (2026-11-22).
func TestResearchFirstTurnAsksForAnEarlyInterimPacket(t *testing.T) {
	if p, _ := recordsInboxPromptFor(agentprofile.Research); !strings.Contains(p, "when you have a claim you can cite") {
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

// A brief that found nothing, or says the search did not answer, is not proof
// that nothing is held; the first turn sends the desk to one corpus search of
// its own, and does not describe what the host's search covered.
func TestResearchFirstTurnSaysAnEmptyBriefIsNotProof(t *testing.T) {
	p, _ := recordsInboxPromptFor(agentprofile.Research)
	for _, want := range []string{"or says the host's search did not answer", "is not proof that nothing is stored", "one corpus search in your own words before the web", "The host also searched"} {
		if !strings.Contains(p, want) {
			t.Fatalf("research first-turn prompt lacks %q: %s", want, p)
		}
	}
	for _, banned := range []string{"covered", "The host searched this computer"} {
		if strings.Contains(p, banned) {
			t.Fatalf("research first-turn prompt still says %q: %s", banned, p)
		}
	}
}
