package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The story analysis reads the library's word counts (StoryLibraryIDF) over
// Texture's documents on the research desk's context. The reader gate refused
// that read ("wrong reader for kind choir.texture_document"), so the rarity of
// every entity was unknown
// (docs/problems/paper-judge-model-record-and-story-rarity-2026-11-09.md).
//
// Failure modes this guards: the IDF read counted as a crossing under the
// research desk's context (the defect); the test running with no declared
// reader, as the older analysis tests do, which never meets the gate; the
// analysis proposing entities with rarity 1 (unknown) after the read.
func TestStoryAnalysisLibraryFrequenciesReadAsTheCustodianUnderTheResearchDesk(t *testing.T) {
	f := newAnalysisFixture(t)
	f.turnOn(t)
	seedStoryDoc(t, f.s, f.owner, f.computer, "doc-other", "An older story", "Anthropic published guidelines on safety. Nothing else happened.")
	objectgraph.ResetReaderGateCounts()
	ctx := withDeskReader(context.Background(), agentprofile.Research)
	f.rt.buildStoryAnalysis(ctx, f.deskExec("run-idf", "doc-story"), f.owner, f.computer, "run-idf", "doc-story")
	requireNoReaderCrossings(t, "buildStoryAnalysis under the research desk's context")
	reqs := f.cpub.candidateCalls()
	if len(reqs) != 1 || len(reqs[0].Proposals) == 0 {
		t.Fatalf("candidate requests = %+v", reqs)
	}
	rare := false
	for _, p := range reqs[0].Proposals {
		if p.Rarity > 1 {
			rare = true
		}
	}
	if !rare {
		t.Errorf("every proposal has rarity <= 1: %+v", reqs[0].Proposals)
	}
}
