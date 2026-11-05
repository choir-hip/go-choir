package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
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

// d@e 14b71687 (deployed d16b911e7) counted one choir.texture_document
// undeclared_reader at WakeRecordsInbox: a research wake reads the document
// only for its trajectory id. That is the host's wake routing, read as the
// kernel, not the research desk's read of Texture's data.
func TestResearchWakeRoutingReadsTheDocumentAsTheKernel(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const owner, docID = "owner-wake-reader", "doc-wake-reader"
	computer := rt.cfg.ComputerID
	seedDurableTextureSubject(t, s, owner, docID)
	research := "research:" + docID
	now := time.Now().UTC()
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: research, OwnerID: owner, ComputerID: computer, Profile: agentprofile.Research, Role: agentprofile.Research,
		ChannelID: docID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	commit := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "wake-reader", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "wake-reader-note", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Body: "x"},
		Inbox:     research, SenderInbox: "texture:" + docID, Addressee: research, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: "texture:" + docID},
	}}}
	commit.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(commit)
	if _, err := s.CommitInboxRecords(ctx, commit); err != nil {
		t.Fatal(err)
	}
	rt.SetDispatchActor(func(_ context.Context, _, _, _, _, _, _, _ string) error { return nil })
	objectgraph.ResetUndeclaredReads()
	stop := objectgraph.TrackUndeclaredReadsFromProduct()
	t.Cleanup(stop)
	rec, err := rt.WakeRecordsInbox(ctx, owner, computer, research)
	if err != nil || rec == nil {
		t.Fatalf("wake = %v, %v; want a started run", rec, err)
	}
	assertNoUndeclaredTextureReads(t)
}
