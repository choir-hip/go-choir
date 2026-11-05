package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// d@e c4d733c89 counted choir.texture_document x1 and choir.texture_revision
// x2 as undeclared reads after a Texture trajectory whose final turn
// passivated and settled. The host paths that run at the end of a Texture
// activation and in its frame read on Texture's behalf and declare it
// (problems/undeclared-texture-reads-at-settlement-2026-11-05.md).
func assertNoUndeclaredTextureReads(t *testing.T) {
	t.Helper()
	for key, n := range objectgraph.UndeclaredReads() {
		if strings.HasPrefix(key, "choir.texture_") {
			t.Errorf("undeclared read: %s x%d", key, n)
		}
	}
}

func TestActivationEndIdleAnswerReadsTheDocumentAsItsCustodian(t *testing.T) {
	rt, _, rec := idleTriggerTextureRun(t)
	rt.noteTextureCell(rec.RunID, nil)
	objectgraph.ResetUndeclaredReads()
	stop := objectgraph.TrackUndeclaredReadsFromProduct()
	t.Cleanup(stop)
	if err := rt.answerIdleTextureTrigger(context.Background(), &rec); err != nil {
		t.Fatal(err)
	}
	assertNoUndeclaredTextureReads(t)
}

func TestFrameOwnerRevisionReadsTheRevisionAsItsCustodian(t *testing.T) {
	rt, _, rec := idleTriggerTextureRun(t)
	ctx := context.Background()
	doc, err := rt.store.GetLifecycleDocument(ctx, rec.OwnerID, rec.ComputerID, rec.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	rec.Metadata["current_revision_id"] = doc.CurrentRevisionID
	objectgraph.ResetUndeclaredReads()
	stop := objectgraph.TrackUndeclaredReadsFromProduct()
	t.Cleanup(stop)
	_ = rt.textureRunOwnerRevision(ctx, &rec, doc, types.Revision{RevisionID: "rev-other"})
	assertNoUndeclaredTextureReads(t)
}
