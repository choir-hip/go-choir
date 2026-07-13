package runtime

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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

type wirePublishDebouncer struct {
	mu sync.Mutex

	pendingDocIDs      []string
	pendingRevisionIDs []string
	firstPendingAt     time.Time
	lastDispatch       time.Time
	pendingLineage     wirePublishLineage
	mixedLineage       bool
}

func newWirePublishDebouncer() *wirePublishDebouncer {
	return &wirePublishDebouncer{}
}

func (d *wirePublishDebouncer) record(docID, revisionID string, lineage wirePublishLineage, now time.Time) (wirePublishBatch, bool) {
	docID = strings.TrimSpace(docID)
	revisionID = strings.TrimSpace(revisionID)
	if docID == "" || revisionID == "" {
		return wirePublishBatch{}, false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.pendingDocIDs) == 0 {
		d.firstPendingAt = now
		d.pendingLineage = lineage
		d.mixedLineage = false
	} else if d.pendingLineage != lineage {
		d.mixedLineage = true
	}
	d.pendingDocIDs = append(d.pendingDocIDs, docID)
	d.pendingRevisionIDs = append(d.pendingRevisionIDs, revisionID)

	if len(d.pendingDocIDs) >= WireReconcilerPublishCountThreshold {
		return d.fireLocked(now), true
	}
	if d.publishBatchDueLocked(now) {
		return d.fireLocked(now), true
	}
	return wirePublishBatch{}, false
}

func (d *wirePublishDebouncer) fireDue(now time.Time) (wirePublishBatch, bool) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pendingDocIDs) == 0 || !d.publishBatchDueLocked(now) {
		return wirePublishBatch{}, false
	}
	return d.fireLocked(now), true
}

func (d *wirePublishDebouncer) nextDispatchDelay(now time.Time) (time.Duration, bool) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pendingDocIDs) == 0 {
		return 0, false
	}
	deadline := d.dispatchDeadlineLocked()
	remaining := deadline.Sub(now)
	if remaining <= 0 {
		return 0, true
	}
	return remaining, true
}

func (d *wirePublishDebouncer) dispatchDeadlineLocked() time.Time {
	if !d.lastDispatch.IsZero() {
		return d.lastDispatch.Add(WireReconcilerPublishDebounceInterval)
	}
	if d.firstPendingAt.IsZero() {
		return time.Time{}
	}
	return d.firstPendingAt.Add(WireReconcilerPublishDebounceInterval)
}

func (d *wirePublishDebouncer) publishBatchDueLocked(now time.Time) bool {
	if len(d.pendingDocIDs) == 0 {
		return false
	}
	deadline := d.dispatchDeadlineLocked()
	if deadline.IsZero() {
		return false
	}
	return !now.Before(deadline)
}

func (d *wirePublishDebouncer) fireLocked(now time.Time) wirePublishBatch {
	batch := wirePublishBatch{
		DocIDs:       append([]string(nil), d.pendingDocIDs...),
		RevisionIDs:  append([]string(nil), d.pendingRevisionIDs...),
		TriggeredAt:  now,
		CycleID:      d.pendingLineage.CycleID,
		RequestID:    d.pendingLineage.RequestID,
		RequestKind:  d.pendingLineage.RequestKind,
		MixedLineage: d.mixedLineage,
	}
	d.pendingDocIDs = nil
	d.pendingRevisionIDs = nil
	d.firstPendingAt = time.Time{}
	d.pendingLineage = wirePublishLineage{}
	d.mixedLineage = false
	d.lastDispatch = now
	return batch
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
	if rt == nil {
		return
	}
	if rt.wirePublishDebouncer == nil {
		rt.wirePublishDebouncer = newWirePublishDebouncer()
	}
	now := time.Now().UTC()
	lineage := wirePublishLineageForRun(rec)
	batch, fire := rt.wirePublishDebouncer.record(docID, revisionID, lineage, now)
	log.Printf("runtime: wire reconciler queued doc=%s rev=%s cycle=%s request=%s fire=%t", docID, revisionID, lineage.CycleID, lineage.RequestID, fire)
	if fire {
		rt.stopWirePublishDebouncerTimer()
		rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, batch)
		return
	}
	rt.scheduleWirePublishDebouncerTimer(now)
}

func (rt *Runtime) scheduleWirePublishDebouncerTimer(now time.Time) {
	if rt == nil || rt.wirePublishDebouncer == nil {
		return
	}
	delay, ok := rt.wirePublishDebouncer.nextDispatchDelay(now)
	if !ok {
		rt.stopWirePublishDebouncerTimer()
		return
	}
	rt.wirePublishDebounceMu.Lock()
	defer rt.wirePublishDebounceMu.Unlock()
	if rt.wirePublishTimer != nil {
		return
	}
	rt.wirePublishTimer = rt.textureWakeAfter(delay, func() {
		rt.onWirePublishDebouncerTimer()
	})
	log.Printf("runtime: wire reconciler timer scheduled delay=%s", delay)
}

func (rt *Runtime) stopWirePublishDebouncerTimer() {
	rt.wirePublishDebounceMu.Lock()
	defer rt.wirePublishDebounceMu.Unlock()
	if rt.wirePublishTimer != nil {
		rt.wirePublishTimer.Stop()
		rt.wirePublishTimer = nil
	}
}

func (rt *Runtime) onWirePublishDebouncerTimer() {
	if rt == nil {
		return
	}
	rt.wirePublishDebounceMu.Lock()
	rt.wirePublishTimer = nil
	rt.wirePublishDebounceMu.Unlock()

	if rt.wirePublishDebouncer == nil {
		return
	}
	batch, fire := rt.wirePublishDebouncer.fireDue(time.Now().UTC())
	if !fire {
		log.Printf("runtime: wire reconciler timer fired without a due batch")
		return
	}
	log.Printf("runtime: wire reconciler timer fired docs=%d cycle=%s mixed_lineage=%t", len(batch.DocIDs), batch.CycleID, batch.MixedLineage)
	rt.dispatchStoryCorpusReconcilerFromPublishBatch(context.Background(), batch)
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
