package textureowner

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// d@e 14b71687 (deployed d16b911e7) counted one choir.texture_revision
// undeclared_reader at TextureActorOccurrencePostcondition. Texture's own
// controller checks its own occurrence, so the custodian is the reader.
func TestOccurrencePostconditionReadsTheRevisionAsTexture(t *testing.T) {
	h, _, rec := cellAuthorSetup(t, "postcond-reader")
	objectgraph.ResetUndeclaredReads()
	objectgraph.ResetReaderGateCounts()
	stop := objectgraph.TrackUndeclaredReadsFromProduct()
	t.Cleanup(stop)
	o := agentcore.TextureActorOccurrence{
		Version: agentcore.TextureActorOccurrenceVersion, Kind: agentcore.TextureActorOccurrenceDocumentRevision,
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, TrajectoryID: rec.TrajectoryID, DocumentID: rec.ChannelID,
		TargetAgentID: rec.AgentID, HeadRevisionID: "revision-cell-author-base-postcond-reader",
	}
	_, _ = h.TextureActorOccurrencePostcondition(context.Background(), o, rec.RunID)
	for key, n := range objectgraph.UndeclaredReads() {
		if strings.HasPrefix(key, "choir.texture_") {
			t.Errorf("undeclared read: %s x%d", key, n)
		}
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		if strings.HasPrefix(key, "choir.texture_") {
			t.Errorf("reader gate violation: %s x%d", key, n)
		}
	}
}
