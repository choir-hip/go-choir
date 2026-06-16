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
)

const (
	WireReconcilerPublishCountThreshold   = 10
	WireReconcilerPublishDebounceInterval = 300 * time.Second
)

type wirePublishBatch struct {
	DocIDs      []string
	RevisionIDs []string
	TriggeredAt time.Time
}

type wirePublishDebouncer struct {
	mu sync.Mutex

	pendingDocIDs      []string
	pendingRevisionIDs []string
	firstPendingAt     time.Time
	lastDispatch       time.Time
}

func newWirePublishDebouncer() *wirePublishDebouncer {
	return &wirePublishDebouncer{}
}

func (d *wirePublishDebouncer) record(docID, revisionID string, now time.Time) (wirePublishBatch, bool) {
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
		DocIDs:      append([]string(nil), d.pendingDocIDs...),
		RevisionIDs: append([]string(nil), d.pendingRevisionIDs...),
		TriggeredAt: now,
	}
	d.pendingDocIDs = nil
	d.pendingRevisionIDs = nil
	d.firstPendingAt = time.Time{}
	d.lastDispatch = now
	return batch
}

func wireCanonicalRevisionEligibleForDebouncedReconciler(doc types.Document, rev types.Revision, rec *types.RunRecord) bool {
	return wirepublish.EligibleForAutonomousPublish(doc, rev, rec, universalWirePlatformOwnerID())
}

func (rt *Runtime) noteWireEligiblePublish(ctx context.Context, docID, revisionID string) {
	if rt == nil {
		return
	}
	if rt.wirePublishDebouncer == nil {
		rt.wirePublishDebouncer = newWirePublishDebouncer()
	}
	now := time.Now().UTC()
	batch, fire := rt.wirePublishDebouncer.record(docID, revisionID, now)
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
		return
	}
	rt.dispatchStoryCorpusReconcilerFromPublishBatch(context.Background(), batch)
}

func (rt *Runtime) dispatchStoryCorpusReconcilerFromPublishBatch(ctx context.Context, batch wirePublishBatch) {
	if rt == nil || len(batch.DocIDs) == 0 {
		return
	}
	ownerID := universalWirePlatformOwnerID()
	prompt := fmt.Sprintf(
		"Reconciler story-corpus: review the wire corpus after %d eligible platform publish(es). Note consensus, contradictions, drift, and candidate Texture updates on existing platform documents. Spawn Texture on existing doc ids when an edition revision is warranted.",
		len(batch.DocIDs),
	)
	prompt += "\n\nPublished document handles: " + strings.Join(batch.DocIDs, ", ")
	if len(batch.RevisionIDs) > 0 {
		prompt += "\nPublished revision handles: " + strings.Join(batch.RevisionIDs, ", ")
	}
	_, err := rt.StartRunWithMetadata(ctx, prompt, ownerID, map[string]any{
		runMetadataAgentProfile:    AgentProfileReconciler,
		runMetadataAgentRole:       AgentProfileReconciler,
		runMetadataReconcilerScope: "story-corpus",
		"activation_origin":        "publish_batch",
		"request_source":           "wire_publish_debouncer",
		"published_doc_ids":        batch.DocIDs,
		"published_revision_ids":   batch.RevisionIDs,
	})
	if err != nil {
		log.Printf("runtime: wire reconciler dispatch failed: %v", err)
	}
}
