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
	if err := rt.reconcileTextureWorkerState(context.Background(), pending.ownerID, pending.docID); err != nil {
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
		if err := rt.reconcileTextureWorkerState(ctx, doc.OwnerID, doc.DocID); err != nil {
			log.Printf("runtime: reconcile doc %s: %v", doc.DocID, err)
		}
	}
}

// reconcileTextureWorkerState is the durable controller invariant for texture:
// if worker messages newer than the integrated checkpoint exist, and no synth
// run is active or pending, launch exactly one new synth run.
func (rt *Runtime) reconcileTextureWorkerState(ctx context.Context, ownerID, docID string) error {
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil
	}
	doc, err := rt.store.GetDocument(ctx, docID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("load doc for reconcile: %w", err)
	}
	checkpoint, err := rt.store.GetTextureControllerCheckpoint(ctx, doc.DocID, ownerID)
	if err != nil {
		return fmt.Errorf("load controller checkpoint: %w", err)
	}
	integratedSeq := int64(0)
	if checkpoint != nil {
		integratedSeq = checkpoint.IntegratedMessageSeq
	}
	latestMessage, found, err := rt.latestEligibleWorkerMessage(ctx, ownerID, doc.DocID, integratedSeq)
	if err != nil {
		return fmt.Errorf("latest eligible worker message: %w", err)
	}
	if !found {
		return nil
	}
	for _, agentID := range []string{currentTextureAgentID(doc.DocID)} {
		if _, found, err := rt.residentRunByAgent(ctx, ownerID, agentID); err != nil {
			return fmt.Errorf("check resident Texture loop: %w", err)
		} else if found {
			rt.scheduleTextureWorkerWake(ownerID, doc.DocID, latestMessage.FromRunID)
			return nil
		}
	}
	if mutation, err := rt.store.GetPendingAgentMutationByDoc(ctx, doc.DocID, ownerID); err == nil && mutation != nil {
		rt.scheduleTextureWorkerWake(ownerID, doc.DocID, latestMessage.FromRunID)
		return nil
	} else if err != nil {
		return fmt.Errorf("check pending doc mutation: %w", err)
	}
	_, err = rt.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{
		Intent: "integrate_worker_findings",
	}, latestMessage.FromRunID, latestMessage.Seq)
	if err != nil {
		return fmt.Errorf("start reconciled Texture revision: %w", err)
	}
	return nil
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
