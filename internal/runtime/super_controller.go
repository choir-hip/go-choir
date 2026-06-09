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

// reconcilePersistentSuperActor is the durable controller boundary for the
// user's privileged execution actor. VText can enqueue addressed work for the
// persistent super, but only this runtime controller starts or reuses the super
// execution loop that drains that inbox.
func (rt *Runtime) reconcilePersistentSuperActor(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if agentID == "" {
		agentID = persistentSuperAgentID(ownerID)
	}
	var blockedActive *types.RunRecord
	if active, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, agentID); err == nil {
		if active.State != types.RunBlocked {
			return &active, nil
		}
		blockedActive = &active
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("check active super run: %w", err)
	}

	deliveries, err := rt.store.ListPendingInboxDeliveries(ctx, ownerID, agentID, 100)
	if err != nil {
		return nil, fmt.Errorf("list super inbox: %w", err)
	}
	if len(deliveries) == 0 {
		if blockedActive != nil {
			return blockedActive, nil
		}
		return nil, nil
	}

	first := deliveries[0]
	metadata := map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      agentID,
		"request_source":        "super_inbox",
		"requested_by_run_id":   first.FromRunID,
		"requested_by_agent_id": first.FromAgentID,
		"requested_by_profile":  strings.TrimSpace(first.Role),
	}
	if first.ChannelID != "" {
		metadata[runMetadataChannelID] = first.ChannelID
	}
	if first.TrajectoryID != "" {
		metadata[runMetadataTrajectoryID] = first.TrajectoryID
	}
	if first.FromRunID != "" {
		if requester, err := rt.store.GetRun(ctx, first.FromRunID); err == nil {
			if metadataStringValue(requester.Metadata, "agent_profile") != "" && metadata["requested_by_profile"] == "" {
				metadata["requested_by_profile"] = metadataStringValue(requester.Metadata, "agent_profile")
			}
			if desktopID := metadataStringValue(requester.Metadata, runMetadataDesktopID); desktopID != "" {
				metadata[runMetadataDesktopID] = desktopID
			}
		}
	}

	rec, err := rt.createRunWithMetadata(ctx, buildPersistentSuperInboxPrompt(deliveries), ownerID, metadata)
	if err != nil {
		return nil, err
	}
	if err := rt.markSuperInboxDeliveriesStarted(ctx, deliveries, rec.RunID); err != nil {
		return nil, err
	}
	rt.startRunAsync(rec)
	return rec, nil
}

func (rt *Runtime) markSuperInboxDeliveriesStarted(ctx context.Context, deliveries []types.InboxDelivery, runID string) error {
	if len(deliveries) == 0 || strings.TrimSpace(runID) == "" {
		return nil
	}
	deliveryIDs := make([]string, 0, len(deliveries))
	for _, delivery := range deliveries {
		if strings.TrimSpace(delivery.DeliveryID) != "" {
			deliveryIDs = append(deliveryIDs, delivery.DeliveryID)
		}
	}
	if len(deliveryIDs) == 0 {
		return nil
	}
	if err := rt.store.MarkInboxDeliveriesDelivered(ctx, deliveryIDs, runID); err != nil {
		return fmt.Errorf("mark super inbox deliveries started: %w", err)
	}
	return nil
}

func (rt *Runtime) maybeContinuePersistentSuperInbox(ctx context.Context, rec *types.RunRecord) {
	if !isPersistentSuperInboxRun(rec) {
		return
	}
	if _, err := rt.reconcilePersistentSuperActor(ctx, rec.OwnerID, rec.AgentID); err != nil {
		log.Printf("runtime: continue persistent super inbox after %s: %v", rec.RunID, err)
	}
}

func isPersistentSuperInboxRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if agentProfileForRun(rec) != AgentProfileSuper {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") != "super_inbox" {
		return false
	}
	if strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.AgentID) == "" {
		return false
	}
	return rec.AgentID == persistentSuperAgentID(rec.OwnerID)
}

func buildPersistentSuperInboxPrompt(deliveries []types.InboxDelivery) string {
	var b strings.Builder
	b.WriteString("Process the pending inbox deliveries addressed to you as the user's persistent super actor.\n\n")
	b.WriteString("Use privileged tools only for the requested execution work. When you have artifacts, test results, references, questions, or proposals, report them back with submit_coagent_update to the addressed vtext document.\n")
	for i, delivery := range deliveries {
		b.WriteString("\nDelivery ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		if delivery.ChannelID != "" {
			b.WriteString(" channel=")
			b.WriteString(delivery.ChannelID)
		}
		if delivery.FromAgentID != "" {
			b.WriteString(" from=")
			b.WriteString(delivery.FromAgentID)
		}
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(delivery.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
