package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type pendingTextureWake struct {
	ownerID string
	docID   string
	timer   textureWakeTimer
}

func textureWakeKey(ownerID, docID string) string {
	return strings.TrimSpace(ownerID) + "::" + strings.TrimSpace(docID)
}

func (rt *Runtime) scheduleTextureWorkerWake(ownerID, docID, _ string) {
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return
	}
	key := textureWakeKey(ownerID, docID)
	debounce := rt.cfg.TextureWakeDebounce
	rt.textureWakeMu.Lock()
	if pending, ok := rt.textureWakePending[key]; ok && pending.timer != nil {
		pending.timer.Stop()
	}
	timer := rt.textureWakeAfter(debounce, func() {
		rt.flushTextureWorkerWake(key)
	})
	rt.textureWakePending[key] = pendingTextureWake{
		ownerID: ownerID,
		docID:   docID,
		timer:   timer,
	}
	rt.textureWakeMu.Unlock()
}

func (rt *Runtime) flushTextureWorkerWake(key string) {
	rt.textureWakeMu.Lock()
	pending, ok := rt.textureWakePending[key]
	if ok {
		delete(rt.textureWakePending, key)
	}
	rt.textureWakeMu.Unlock()
	if !ok {
		return
	}
	if _, err := rt.reconcileTextureAgentWake(context.Background(), pending.ownerID, pending.docID); err != nil {
		log.Printf("runtime: reconcile texture wake failed for doc %s: %v", pending.docID, err)
	}
}

func (rt *Runtime) reconcileAllTextureDocuments(ctx context.Context) {
	docs, err := rt.store.ListAllDocuments(ctx, 2000)
	if err != nil {
		log.Printf("runtime: reconcile all texture docs: %v", err)
		return
	}
	for _, doc := range docs {
		if _, err := rt.reconcileTextureAgentWake(ctx, doc.OwnerID, doc.DocID); err != nil {
			log.Printf("runtime: reconcile doc %s: %v", doc.DocID, err)
		}
	}
}

// reconcileTextureWorkerState is retained as a doc-scoped alias for the unified
// coagent wake path used by Texture agents.
func (rt *Runtime) reconcileTextureWorkerState(ctx context.Context, ownerID, docID string) error {
	_, err := rt.reconcileTextureAgentWake(ctx, ownerID, docID)
	return err
}

// reconcileTextureAgentWake starts or reuses a Texture activation when pending
// update_coagent records are addressed to texture:<docID>. Delivery uses the
// same typed coagent update packets as other actors; integrate intent only
// selects the Texture revision run shape.
func (rt *Runtime) reconcileTextureAgentWake(ctx context.Context, ownerID, docID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil, nil
	}
	textureAgentID := currentTextureAgentID(docID)
	if _, found, err := rt.residentRunByAgent(ctx, ownerID, textureAgentID); err != nil {
		return nil, fmt.Errorf("check resident Texture loop: %w", err)
	} else if found {
		return nil, nil
	}
	updates, err := rt.store.ListPendingWorkerUpdates(ctx, ownerID, textureAgentID, 100)
	if err != nil {
		return nil, fmt.Errorf("list pending texture updates: %w", err)
	}
	if len(updates) == 0 {
		return nil, nil
	}
	if mutation, err := rt.store.GetPendingAgentMutationByDoc(ctx, docID, ownerID); err == nil && mutation != nil {
		rt.scheduleTextureWorkerWake(ownerID, docID, "")
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("check pending doc mutation: %w", err)
	}
	doc, err := rt.store.GetDocument(ctx, docID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load doc for texture wake: %w", err)
	}
	var scheduledSeq int64
	for _, update := range updates {
		if update.MessageSeq > scheduledSeq {
			scheduledSeq = update.MessageSeq
		}
	}
	rec, err := rt.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{
		Intent: "integrate_worker_findings",
	}, scheduledSeq)
	if err != nil {
		return nil, fmt.Errorf("start reconciled Texture revision: %w", err)
	}
	return rec, nil
}

func (rt *Runtime) latestEligibleWorkerMessage(ctx context.Context, ownerID, channelID string, afterSeq int64) (types.ChannelMessage, bool, error) {
	const batchSize = 200
	cache := make(map[string]bool)
	cursor := afterSeq
	var latest types.ChannelMessage
	found := false
	for {
		messages, err := rt.store.ListChannelMessages(ctx, ownerID, channelID, cursor, batchSize)
		if err != nil {
			return types.ChannelMessage{}, false, err
		}
		if len(messages) == 0 {
			break
		}
		for _, message := range messages {
			if message.Seq > cursor {
				cursor = message.Seq
			}
			ok, err := rt.isEligibleWorkerMessage(ctx, channelID, message, cache)
			if err != nil {
				return types.ChannelMessage{}, false, err
			}
			if !ok {
				continue
			}
			latest = message
			found = true
		}
		if len(messages) < batchSize {
			break
		}
	}
	return latest, found, nil
}

func (rt *Runtime) isEligibleWorkerMessage(ctx context.Context, docID string, message types.ChannelMessage, cache map[string]bool) (bool, error) {
	if strings.TrimSpace(message.ToAgentID) != "texture:"+strings.TrimSpace(docID) {
		return false, nil
	}
	runID := strings.TrimSpace(message.FromRunID)
	if runID == "" {
		return false, nil
	}
	if cached, ok := cache[runID]; ok {
		return cached, nil
	}
	run, err := rt.store.GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			cache[runID] = false
			return false, nil
		}
		return false, err
	}
	switch agentProfileForRun(&run) {
	case AgentProfileResearcher, AgentProfileSuper, AgentProfileVSuper, AgentProfileCoSuper:
		cache[runID] = true
		return true, nil
	default:
		cache[runID] = false
		return false, nil
	}
}
