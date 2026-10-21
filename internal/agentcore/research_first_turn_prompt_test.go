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
	if p := recordsInboxPrompts[agentprofile.Research]; !strings.Contains(p, "as soon as you have read one new source you can cite") {
		t.Fatalf("research first-turn prompt: %s", p)
	}
}
