package agentcore

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestWirePublishDebounceFiresAtCountThreshold(t *testing.T) {
	rt, _ := testAPISetup(t)
	ctx := context.Background()
	ownerID, computerID := universalWirePlatformOwnerID(), rt.TextureComputerID()
	now := time.Now().UTC()

	for i := range WireReconcilerPublishCountThreshold {
		content, err := encodeWirePublishDebounceEntry("doc-"+strconv.Itoa(i), "rev-"+strconv.Itoa(i), wirePublishLineage{})
		if err != nil {
			t.Fatal(err)
		}
		pending, err := rt.Store().RecordWirePublishDebounceEntry(ctx, ownerID, computerID, wireReconcilerPublishDeadlineAgentID, content, now, WireReconcilerPublishCountThreshold, WireReconcilerPublishDebounceInterval)
		if err != nil {
			t.Fatal(err)
		}
		if pending.Due != (i == WireReconcilerPublishCountThreshold-1) {
			t.Fatalf("publish %d due=%t", i+1, pending.Due)
		}
	}
	pending, err := rt.Store().ConsumeDueWirePublishDebounceBatch(ctx, ownerID, computerID, wireReconcilerPublishDeadlineAgentID, now, WireReconcilerPublishCountThreshold, WireReconcilerPublishDebounceInterval)
	if err != nil {
		t.Fatal(err)
	}
	if !pending.Fired || len(pending.Entries) != WireReconcilerPublishCountThreshold {
		t.Fatalf("threshold batch=%+v", pending)
	}
}

func TestNoteWireEligiblePublishSchedulesDurableDeadline(t *testing.T) {
	rt, _ := testAPISetup(t)
	rt.SetKernelMode()
	type scheduled struct {
		ownerID    string
		computerID string
		agentID    string
		kind       string
		content    string
		notBefore  time.Time
	}
	var got scheduled
	rt.SetScheduleActor(func(_ context.Context, ownerID, computerID, agentID, kind, content, _, _ string, notBefore time.Time) error {
		got = scheduled{ownerID: ownerID, computerID: computerID, agentID: agentID, kind: kind, content: content, notBefore: notBefore}
		return nil
	})

	rt.noteWireEligiblePublish(context.Background(), "doc-scheduled", "rev-scheduled", nil)

	if got.ownerID != universalWirePlatformOwnerID() || got.computerID != rt.TextureComputerID() || got.agentID != wireReconcilerPublishDeadlineAgentID ||
		got.kind != wireReconcilerPublishDeadlineUpdateKind || got.content != wireReconcilerPublishDeadlineBatchKey || got.notBefore.IsZero() {
		t.Fatalf("scheduled continuation=%+v", got)
	}
}

func TestWirePublishDebounceSurvivesStoreReopen(t *testing.T) {
	rt, _ := testAPISetup(t)
	ctx := context.Background()
	ownerID, computerID := universalWirePlatformOwnerID(), rt.TextureComputerID()
	recordedAt := time.Now().UTC().Add(-WireReconcilerPublishDebounceInterval - time.Second)
	content, err := encodeWirePublishDebounceEntry("doc-restart", "rev-restart", wirePublishLineage{CycleID: "cycle-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Store().RecordWirePublishDebounceEntry(ctx, ownerID, computerID, wireReconcilerPublishDeadlineAgentID, content, recordedAt, WireReconcilerPublishCountThreshold, WireReconcilerPublishDebounceInterval); err != nil {
		t.Fatal(err)
	}
	if err := rt.Store().Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(rt.cfg.StorePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	pending, err := reopened.ConsumeDueWirePublishDebounceBatch(ctx, ownerID, computerID, wireReconcilerPublishDeadlineAgentID, time.Now().UTC(), WireReconcilerPublishCountThreshold, WireReconcilerPublishDebounceInterval)
	if err != nil {
		t.Fatal(err)
	}
	if !pending.Fired || len(pending.Entries) != 1 {
		t.Fatalf("recovered batch=%+v", pending)
	}
	batch, err := wirePublishBatchFromDebounceEntries(pending.Entries, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.DocIDs) != 1 || batch.DocIDs[0] != "doc-restart" || batch.CycleID != "cycle-1" {
		t.Fatalf("recovered batch=%+v", batch)
	}
}

func TestDispatchStoryCorpusReconcilerCarriesSingleCycleLineage(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	handler.rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, wirePublishBatch{
		DocIDs:      []string{"doc-1"},
		RevisionIDs: []string{"rev-1"},
		CycleID:     "cycle-1",
		RequestID:   "processor-1",
		RequestKind: "processor",
	})
	runs, err := handler.rt.ListRunsByOwner(ctx, universalWirePlatformOwnerID(), 20)
	if err != nil {
		t.Fatalf("list reconciler runs: %v", err)
	}
	for _, run := range runs {
		if metadataStringValue(run.Metadata, runMetadataAgentProfile) != agentprofile.Reconciler {
			continue
		}
		if got := metadataStringValue(run.Metadata, "ingestion_handoff_cycle_id"); got != "cycle-1" {
			t.Fatalf("reconciler cycle id = %q, want cycle-1", got)
		}
		if got := metadataStringValue(run.Metadata, "ingestion_handoff_request_id"); got != "reconciler_publish_cycle-1" {
			t.Fatalf("reconciler request id = %q, want reconciler_publish_cycle-1", got)
		}
		if got := metadataStringValue(run.Metadata, "ingestion_handoff_request_kind"); got != "reconciler" {
			t.Fatalf("reconciler request kind = %q, want reconciler", got)
		}
		if got := metadataStringValue(run.Metadata, "source_network_request_id"); got != "processor-1" {
			t.Fatalf("source request id = %q, want processor-1", got)
		}
		if got := metadataIntValue(run.Metadata, "required_texture_revisions"); got != 1 {
			t.Fatalf("required Texture revisions = %d, want 1", got)
		}
		if got := metadataIntValue(run.Metadata, "required_child_runs"); got != 0 {
			t.Fatalf("generic required child runs = %d, want 0", got)
		}
		for _, want := range []string{
			"must produce one reconciler-owned canonical Texture revision",
			"call spawn_agent exactly once with role=texture",
			"channel_id set to that document id",
			"Do not create a new document",
			"end without the required existing-document Texture revision",
		} {
			if !strings.Contains(run.Prompt, want) {
				t.Fatalf("reconciler prompt missing %q: %s", want, run.Prompt)
			}
		}
		return
	}
	t.Fatal("reconciler run not found")
}

func TestWirePublishBatchDocumentContextIncludesCanonicalRevision(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := universalWirePlatformOwnerID()
	doc := types.Document{
		DocID:     "doc-context",
		OwnerID:   ownerID,
		Title:     "A canonical wire story",
		CreatedAt: now,
		UpdatedAt: now,
	}
	rev := types.Revision{
		RevisionID:  "rev-context",
		DocID:       doc.DocID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "acceptance",
		Content:     "# A canonical wire story\n\nThe reviewed claim is grounded here.",
		CreatedAt:   now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := handler.rt.Store().CreateRevision(ctx, rev); err != nil {
		t.Fatalf("create revision: %v", err)
	}

	prompt := handler.rt.wirePublishBatchDocumentContext(ctx, ownerID, wirePublishBatch{
		DocIDs:      []string{doc.DocID},
		RevisionIDs: []string{rev.RevisionID},
	})
	for _, want := range []string{
		"Canonical Texture context",
		"Title: A canonical wire story",
		"Revision: rev-context",
		"The reviewed claim is grounded here.",
		"do not search opaque ids as text",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("reconciler context missing %q: %s", want, prompt)
		}
	}
}

func TestWirePublishBatchDocumentContextTruncatesUTF8Safely(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := universalWirePlatformOwnerID()
	doc := types.Document{
		DocID:     "doc-utf8-context",
		OwnerID:   ownerID,
		Title:     "UTF-8 context",
		CreatedAt: now,
		UpdatedAt: now,
	}
	rev := types.Revision{
		RevisionID:  "rev-utf8-context",
		DocID:       doc.DocID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "acceptance",
		Content:     strings.Repeat("é", 2401),
		CreatedAt:   now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := handler.rt.Store().CreateRevision(ctx, rev); err != nil {
		t.Fatalf("create revision: %v", err)
	}

	prompt := handler.rt.wirePublishBatchDocumentContext(ctx, ownerID, wirePublishBatch{
		DocIDs:      []string{doc.DocID},
		RevisionIDs: []string{rev.RevisionID},
	})
	if !strings.Contains(prompt, strings.Repeat("é", 2400)+"…") {
		t.Fatalf("reconciler context was not truncated on a rune boundary")
	}
}

func TestDispatchStoryCorpusReconcilerDeduplicatesOneRunPerCycle(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	batch := wirePublishBatch{
		DocIDs:      []string{"doc-1"},
		RevisionIDs: []string{"rev-1"},
		CycleID:     "cycle-1",
		RequestID:   "processor-1",
		RequestKind: "processor",
	}
	handler.rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, batch)
	handler.rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, batch)
	runs, err := handler.rt.Store().ListRunsByIngestionHandoff(ctx, universalWirePlatformOwnerID(), agentprofile.Reconciler, "reconciler_publish_cycle-1", "reconciler", 20)
	if err != nil {
		t.Fatalf("list cycle reconciler runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("cycle reconciler runs = %d, want 1", len(runs))
	}
}

func TestDispatchStoryCorpusReconcilerOmitsFalseMixedCycleAttribution(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	handler.rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, wirePublishBatch{
		DocIDs:       []string{"doc-1", "doc-2"},
		RevisionIDs:  []string{"rev-1", "rev-2"},
		CycleID:      "cycle-1",
		MixedLineage: true,
	})
	runs, err := handler.rt.ListRunsByOwner(ctx, universalWirePlatformOwnerID(), 20)
	if err != nil {
		t.Fatalf("list reconciler runs: %v", err)
	}
	for _, run := range runs {
		if metadataStringValue(run.Metadata, runMetadataAgentProfile) != agentprofile.Reconciler {
			continue
		}
		if got := metadataStringValue(run.Metadata, "ingestion_handoff_cycle_id"); got != "" {
			t.Fatalf("mixed-lineage reconciler cycle id = %q, want empty", got)
		}
		return
	}
	t.Fatal("reconciler run not found")
}
