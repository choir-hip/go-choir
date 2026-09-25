package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

const (
	WireReconcilerPublishCountThreshold   = 10
	WireReconcilerPublishDebounceInterval = 300 * time.Second
)

type wirePublishBatch struct {
	DocIDs       []string
	RevisionIDs  []string
	TriggeredAt  time.Time
	CycleID      string
	RequestID    string
	RequestKind  string
	MixedLineage bool
}

type wirePublishLineage struct {
	CycleID     string
	RequestID   string
	RequestKind string
}

const (
	wireReconcilerPublishDeadlineBatchKey = "story-corpus"
	wireReconcilerPublishDeadlineAgentID  = "reconciler:story-corpus"
)

type wirePublishDebounceEntry struct {
	DocID      string             `json:"doc_id"`
	RevisionID string             `json:"revision_id"`
	Lineage    wirePublishLineage `json:"lineage"`
}

func encodeWirePublishDebounceEntry(docID, revisionID string, lineage wirePublishLineage) (string, error) {
	entry := wirePublishDebounceEntry{
		DocID:      strings.TrimSpace(docID),
		RevisionID: strings.TrimSpace(revisionID),
		Lineage:    lineage,
	}
	if entry.DocID == "" || entry.RevisionID == "" {
		return "", fmt.Errorf("wire publish debounce entry requires document and revision")
	}
	content, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("encode wire publish debounce entry: %w", err)
	}
	return string(content), nil
}

func wirePublishBatchFromDebounceEntries(entries []store.WirePublishDebounceEntry, triggeredAt time.Time) (wirePublishBatch, error) {
	batch := wirePublishBatch{TriggeredAt: triggeredAt.UTC()}
	for _, persisted := range entries {
		var entry wirePublishDebounceEntry
		if err := json.Unmarshal([]byte(persisted.Content), &entry); err != nil {
			return wirePublishBatch{}, fmt.Errorf("decode wire publish debounce entry: %w", err)
		}
		entry.DocID = strings.TrimSpace(entry.DocID)
		entry.RevisionID = strings.TrimSpace(entry.RevisionID)
		if entry.DocID == "" || entry.RevisionID == "" {
			return wirePublishBatch{}, fmt.Errorf("wire publish debounce entry requires document and revision")
		}
		if len(batch.DocIDs) == 0 {
			batch.CycleID = entry.Lineage.CycleID
			batch.RequestID = entry.Lineage.RequestID
			batch.RequestKind = entry.Lineage.RequestKind
		} else if batch.CycleID != entry.Lineage.CycleID || batch.RequestID != entry.Lineage.RequestID || batch.RequestKind != entry.Lineage.RequestKind {
			batch.MixedLineage = true
		}
		batch.DocIDs = append(batch.DocIDs, entry.DocID)
		batch.RevisionIDs = append(batch.RevisionIDs, entry.RevisionID)
	}
	return batch, nil
}

func wireCanonicalRevisionEligibleForDebouncedReconciler(doc types.Document, rev types.Revision, rec *types.RunRecord) bool {
	return wirepublish.EligibleForAutonomousPublish(doc, rev, rec, universalWirePlatformOwnerID())
}

func wirePublishLineageForRun(rec *types.RunRecord) wirePublishLineage {
	if rec == nil {
		return wirePublishLineage{}
	}
	return wirePublishLineage{
		CycleID: firstNonEmptyString(
			metadataStringValue(rec.Metadata, "ingestion_handoff_cycle_id"),
			metadataStringValue(rec.Metadata, "source_network_cycle_id"),
		),
		RequestID: firstNonEmptyString(
			metadataStringValue(rec.Metadata, "ingestion_handoff_request_id"),
			metadataStringValue(rec.Metadata, "source_network_request_id"),
		),
		RequestKind: firstNonEmptyString(
			metadataStringValue(rec.Metadata, "ingestion_handoff_request_kind"),
			metadataStringValue(rec.Metadata, "source_network_request_kind"),
		),
	}
}

func wirePublishReconcilerRequestID(cycleID string) string {
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return ""
	}
	return "reconciler_publish_" + strings.TrimPrefix(cycleID, "cycle_")
}

func (rt *Runtime) noteWireEligiblePublish(ctx context.Context, docID, revisionID string, rec *types.RunRecord) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID := universalWirePlatformOwnerID()
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" {
		log.Printf("runtime: wire reconciler debounce skipped without computer identity")
		return
	}
	now := time.Now().UTC()
	lineage := wirePublishLineageForRun(rec)
	content, err := encodeWirePublishDebounceEntry(docID, revisionID, lineage)
	if err != nil {
		log.Printf("runtime: wire reconciler debounce encode doc=%s rev=%s: %v", docID, revisionID, err)
		return
	}
	pending, err := rt.store.RecordWirePublishDebounceEntry(ctx, ownerID, computerID, wireReconcilerPublishDeadlineAgentID, content, now, WireReconcilerPublishCountThreshold, WireReconcilerPublishDebounceInterval)
	if err != nil {
		log.Printf("runtime: wire reconciler debounce persist doc=%s rev=%s: %v", docID, revisionID, err)
		return
	}
	notBefore := pending.Deadline
	if pending.Due {
		notBefore = now
	}
	if notBefore.IsZero() {
		log.Printf("runtime: wire reconciler debounce persisted without deadline doc=%s rev=%s", docID, revisionID)
		return
	}
	rt.scheduleContinuation(ctx, ownerID, computerID, wireReconcilerPublishDeadlineAgentID,
		wireReconcilerPublishDeadlineUpdateKind, wireReconcilerPublishDeadlineBatchKey, "", "", notBefore)
	log.Printf("runtime: wire reconciler queued doc=%s rev=%s cycle=%s request=%s due=%t deadline=%s", docID, revisionID, lineage.CycleID, lineage.RequestID, pending.Due, notBefore.Format(time.RFC3339Nano))
}

// HandleWireReconcilerPublishDeadline consumes the durable pending batch that
// the deadline key names. A stale scheduled event observes the new deadline
// and schedules it again; only the event that atomically consumes the batch
// dispatches the reconciler.
func (rt *Runtime) HandleWireReconcilerPublishDeadline(ctx context.Context, ownerID, computerID, agentID, content string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("wire reconciler publish deadline: store unavailable")
	}
	if strings.TrimSpace(agentID) != wireReconcilerPublishDeadlineAgentID || strings.TrimSpace(content) != wireReconcilerPublishDeadlineBatchKey {
		return nil
	}
	now := time.Now().UTC()
	pending, err := rt.store.ConsumeDueWirePublishDebounceBatch(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), wireReconcilerPublishDeadlineAgentID, now, WireReconcilerPublishCountThreshold, WireReconcilerPublishDebounceInterval)
	if err != nil {
		return fmt.Errorf("consume wire reconciler publish deadline: %w", err)
	}
	if !pending.Fired {
		if !pending.Deadline.IsZero() {
			rt.scheduleContinuation(ctx, ownerID, computerID, wireReconcilerPublishDeadlineAgentID,
				wireReconcilerPublishDeadlineUpdateKind, wireReconcilerPublishDeadlineBatchKey, "", "", pending.Deadline)
		}
		return nil
	}
	batch, err := wirePublishBatchFromDebounceEntries(pending.Entries, now)
	if err != nil {
		return err
	}
	log.Printf("runtime: wire reconciler durable deadline fired docs=%d cycle=%s mixed_lineage=%t", len(batch.DocIDs), batch.CycleID, batch.MixedLineage)
	rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, batch)
	return nil
}

func (rt *Runtime) dispatchStoryCorpusReconcilerFromPublishBatch(ctx context.Context, batch wirePublishBatch) {
	if rt == nil || len(batch.DocIDs) == 0 {
		return
	}
	ownerID := universalWirePlatformOwnerID()
	prompt := fmt.Sprintf(
		"Reconciler story-corpus: review the wire corpus after %d eligible platform publish(es). Note consensus, contradictions, drift, and editorial changes needed on the listed existing platform documents. This activation must produce one reconciler-owned canonical Texture revision: select exactly one listed document, call spawn_agent exactly once with role=texture and channel_id set to that document id, and direct the Texture agent to revise the existing canonical article using this review. Do not create a new document, spawn more than one Texture agent, merely summarize the review, or end without the required existing-document Texture revision.",
		len(batch.DocIDs),
	)
	prompt += "\n\nPublished document handles: " + strings.Join(batch.DocIDs, ", ")
	if len(batch.RevisionIDs) > 0 {
		prompt += "\nPublished revision handles: " + strings.Join(batch.RevisionIDs, ", ")
	}
	prompt += rt.wirePublishBatchDocumentContext(ctx, ownerID, batch)
	metadata := map[string]any{
		runMetadataAgentProfile:      agentprofile.Reconciler,
		runMetadataAgentRole:         agentprofile.Reconciler,
		runMetadataReconcilerScope:   "story-corpus",
		"activation_origin":          "publish_batch",
		"request_source":             "wire_publish_debouncer",
		"published_doc_ids":          batch.DocIDs,
		"published_revision_ids":     batch.RevisionIDs,
		"required_texture_revisions": 1,
	}
	if !batch.MixedLineage && strings.TrimSpace(batch.CycleID) != "" {
		reconcilerRequestID := wirePublishReconcilerRequestID(batch.CycleID)
		metadata["ingestion_handoff_cycle_id"] = batch.CycleID
		metadata["source_network_cycle_id"] = batch.CycleID
		metadata["ingestion_handoff_request_id"] = reconcilerRequestID
		metadata["source_network_request_id"] = batch.RequestID
		metadata["ingestion_handoff_request_kind"] = "reconciler"
		metadata["source_network_request_kind"] = batch.RequestKind
		existing, listErr := rt.store.ListRunsByIngestionHandoff(ctx, ownerID, agentprofile.Reconciler, reconcilerRequestID, "reconciler", 2)
		if listErr != nil {
			log.Printf("runtime: wire reconciler dedupe lookup failed cycle=%s request=%s: %v", batch.CycleID, reconcilerRequestID, listErr)
			return
		}
		if len(existing) > 0 {
			log.Printf("runtime: wire reconciler already exists run=%s cycle=%s request=%s; skipping duplicate publish batch", existing[0].RunID, batch.CycleID, reconcilerRequestID)
			return
		}
	} else if batch.MixedLineage {
		log.Printf("runtime: wire reconciler batch has mixed ingestion lineage; dispatching without a false cycle attribution")
	}
	rec, err := rt.StartRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		log.Printf("runtime: wire reconciler dispatch failed: %v", err)
		return
	}
	log.Printf("runtime: wire reconciler dispatched run=%s docs=%d cycle=%s request=%s", rec.RunID, len(batch.DocIDs), batch.CycleID, batch.RequestID)
}

func (rt *Runtime) wirePublishBatchDocumentContext(ctx context.Context, ownerID string, batch wirePublishBatch) string {
	if rt == nil || rt.store == nil {
		return ""
	}
	var b strings.Builder
	for i, docID := range batch.DocIDs {
		doc, err := rt.store.GetDocument(ctx, docID, ownerID)
		if err != nil {
			continue
		}
		revisionID := strings.TrimSpace(doc.CurrentRevisionID)
		if i < len(batch.RevisionIDs) && strings.TrimSpace(batch.RevisionIDs[i]) != "" {
			revisionID = strings.TrimSpace(batch.RevisionIDs[i])
		}
		rev, err := rt.store.GetRevision(ctx, revisionID, ownerID)
		if err != nil || rev.DocID != doc.DocID {
			rev, err = rt.store.GetRevision(ctx, strings.TrimSpace(doc.CurrentRevisionID), ownerID)
		}
		if err != nil || rev.DocID != doc.DocID {
			continue
		}
		content := strings.TrimSpace(rev.Content)
		const maxContextChars = 2400
		contentRunes := []rune(content)
		if len(contentRunes) > maxContextChars {
			content = strings.TrimSpace(string(contentRunes[:maxContextChars])) + "…"
		}
		if b.Len() == 0 {
			b.WriteString("\n\nCanonical Texture context (authoritative for this review; do not search opaque ids as text):")
		}
		fmt.Fprintf(&b, "\n\nDocument %s\nTitle: %s\nRevision: %s\nContent:\n%s", doc.DocID, strings.TrimSpace(doc.Title), rev.RevisionID, content)
	}
	if b.Len() > 0 {
		b.WriteString("\n\nReview the canonical content above directly. Use the listed document id as channel_id when spawning Texture for an update; corpus/source search is for related evidence, not for resolving these ids.")
	}
	return b.String()
}
