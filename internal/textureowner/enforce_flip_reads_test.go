package textureowner

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Enforce flip S1, R3 (section 6.4) and D3 (section 6.5), written first.
//
//	FM4 R3's Texture identity leaks into a non-Texture caller.
//
// commitEvidenceEmbeds is Texture's own handler. The engineering run's context
// reaches it (the embed job is started on the settlement path), and
// asTextureReader declares only when the context names no reader, so the
// handler's reads of Texture's document and revision were counted as
// engineering's. The handler reads as Texture, for its own reads only.

func TestR3EvidenceEmbedReadsAsTextureWhateverTheCallerDeclared(t *testing.T) {
	rh := newRootHarness(t, true)
	engineering, ok := objectgraph.DeskWriter("engineering")
	if !ok {
		t.Fatal("no engineering writer")
	}
	docID, _ := rh.createDoc("r3-embed", "Harbor")
	ctx := objectgraph.WithReader(t.Context(), engineering)
	objectgraph.ResetReaderGateCounts()
	// The listing names no trajectory the document continues, so the handler
	// refuses after it has read the document (and its lineage).
	_, _, err := rh.h.commitEvidenceEmbeds(ctx, rootOwner, rootComputer, docID, agentcore.ScreensListing{TrajectoryID: "traj-not-this"})
	if err == nil {
		t.Fatal("expected the foreign trajectory to be refused")
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		if strings.Contains(key, "texture_") {
			t.Errorf("the embed handler's read was counted: %s x%d", key, n)
		}
	}
	// FM4: the identity did not leak. The caller's context still names
	// engineering, and engineering's own read of the document is still a
	// crossing.
	if got, _ := objectgraph.ReaderFrom(ctx); got != engineering {
		t.Fatalf("the caller's reader became %s", got)
	}
	objectgraph.ResetReaderGateCounts()
	if _, err := rh.s.GetLifecycleDocument(ctx, rootOwner, rootComputer, docID); err != nil {
		t.Fatal(err)
	}
	counted := false
	for key := range objectgraph.ReaderGateCounts() {
		counted = counted || strings.Contains(key, "texture_document|"+string(engineering)+"|wrong_reader")
	}
	if !counted {
		t.Errorf("engineering's own read of a Texture document is no longer counted: %v", objectgraph.ReaderGateCounts())
	}
}
