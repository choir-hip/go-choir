package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/trace"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type runSubmissionStore interface {
	UpsertAgent(context.Context, types.AgentRecord) error
	CreateRun(context.Context, types.RunRecord) error
	CreateAgentMutation(context.Context, store.AgentMutation) error
	AppendEvent(context.Context, *types.EventRecord) error
}

// persistSubmittedRun persists the agent, run, agent mutation, and the
// EventRunSubmitted event, then publishes the event on the bus. When traceStore
// is non-nil, the submitted event is also projected into the canonical trace
// observability schema (additive; failures are logged and swallowed so a Dolt
// outage degrades gracefully).
func persistSubmittedRun(ctx context.Context, st runSubmissionStore, bus *events.EventBus, agentRec types.AgentRecord, rec *types.RunRecord, promptLen int, traceStore trace.Store) error {
	if st == nil {
		return fmt.Errorf("runtime store is required")
	}
	if rec == nil {
		return fmt.Errorf("run record is required")
	}
	if err := st.UpsertAgent(ctx, agentRec); err != nil {
		return fmt.Errorf("persist agent: %w", err)
	}
	if err := st.CreateRun(ctx, *rec); err != nil {
		return fmt.Errorf("persist run: %w", err)
	}
	return persistSubmittedRunProjections(ctx, st, bus, rec, promptLen, traceStore)
}

func persistLifecycleSubmittedRun(ctx context.Context, st *store.Store, bus *events.EventBus, rec *types.RunRecord, promptLen int, traceStore trace.Store) error {
	if st == nil {
		return fmt.Errorf("runtime store is required")
	}
	if rec == nil {
		return fmt.Errorf("run record is required")
	}
	mutation := agentMutationForRun(rec)
	if mutation != nil {
		if err := st.CreateAgentMutation(ctx, *mutation); err != nil {
			return fmt.Errorf("persist Texture mutation authority: %w", err)
		}
	}
	req := types.ReplaceLifecycleActivationRequest{
		OwnerID: rec.OwnerID, ComputerID: rec.SandboxID, CommandID: "activation:" + rec.RunID,
		TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: *rec,
	}
	req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
	if _, err := st.ReplaceLifecycleActivation(ctx, req); err != nil {
		if mutation != nil {
			rollbackCtx := context.WithoutCancel(ctx)
			if staleErr := st.MarkAgentMutationStale(rollbackCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID); staleErr != nil {
				return fmt.Errorf("replace durable activation: %v; stale unbound mutation: %w", err, staleErr)
			}
		}
		return fmt.Errorf("replace durable activation: %w", err)
	}
	if err := persistSubmittedRunEvent(ctx, st, bus, rec, promptLen, traceStore); err != nil {
		if mutation == nil {
			return err
		}
		rollbackCtx := context.WithoutCancel(ctx)
		passivated := *rec
		passivated.State = types.RunPassivated
		passivated.Error = ""
		passivated.FinishedAt = nil
		passivated.UpdatedAt = time.Now().UTC()
		passivated.Metadata = cloneMetadata(passivated.Metadata)
		passivated.Metadata["passivated_reason"] = "submission_projection_failed"
		rollback := types.ReplaceLifecycleActivationRequest{
			OwnerID: passivated.OwnerID, ComputerID: passivated.SandboxID,
			CommandID:    "activation-submission-failed:" + passivated.RunID,
			TrajectoryID: passivated.TrajectoryID, AgentID: passivated.AgentID, Run: passivated,
		}
		rollback.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(rollback)
		if _, rollbackErr := st.ReplaceLifecycleActivation(rollbackCtx, rollback); rollbackErr != nil {
			return fmt.Errorf("%v; passivate failed submission: %w", err, rollbackErr)
		}
		if mutation != nil {
			if staleErr := st.MarkAgentMutationStale(rollbackCtx, passivated.OwnerID, agentMutationComputerID(&passivated), passivated.RunID); staleErr != nil {
				return fmt.Errorf("%v; stale failed submission mutation: %w", err, staleErr)
			}
		}
		return err
	}
	return nil
}

func persistSubmittedRunProjections(ctx context.Context, st runSubmissionStore, bus *events.EventBus, rec *types.RunRecord, promptLen int, traceStore trace.Store) error {
	if mutation := agentMutationForRun(rec); mutation != nil {
		if err := st.CreateAgentMutation(ctx, *mutation); err != nil {
			return fmt.Errorf("persist Texture mutation authority: %w", err)
		}
	}
	return persistSubmittedRunEvent(ctx, st, bus, rec, promptLen, traceStore)
}

func persistSubmittedRunEvent(ctx context.Context, st runSubmissionStore, bus *events.EventBus, rec *types.RunRecord, promptLen int, traceStore trace.Store) error {
	promptLenPayload, _ := json.Marshal(map[string]int{"prompt_length": promptLen})
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    rec.CreatedAt,
		Kind:         types.EventRunSubmitted,
		Payload:      promptLenPayload,
	}
	if err := st.AppendEvent(ctx, evRec); err != nil {
		return fmt.Errorf("persist submitted event: %w", err)
	}
	if traceStore != nil {
		tev := trace.FromEventRecord(evRec)
		if err := traceStore.Append(ctx, &tev); err != nil {
			log.Printf("runtime: trace store append submitted %s: %v", evRec.EventID, err)
		}
	}
	if bus != nil {
		bus.Publish(events.RuntimeEvent{Record: *evRec, Actor: events.ActorRuntime, Cause: events.CauseTaskLifecycle})
	}
	return nil
}

func agentMutationComputerID(rec *types.RunRecord) string {
	if rec == nil || strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id")) == "" {
		return ""
	}
	return strings.TrimSpace(rec.SandboxID)
}

func agentMutationForRun(rec *types.RunRecord) *store.AgentMutation {
	if rec == nil || !runHasProfile(rec, agentprofile.Texture) {
		return nil
	}
	docID := metadataStringValue(rec.Metadata, "doc_id")
	if docID == "" {
		return nil
	}
	scheduledSeq := int64(0)
	if rec.Metadata != nil {
		switch v := rec.Metadata["scheduled_message_seq"].(type) {
		case int64:
			scheduledSeq = v
		case int:
			scheduledSeq = int64(v)
		case float64:
			scheduledSeq = int64(v)
		}
	}
	return &store.AgentMutation{
		DocID:               docID,
		RunID:               rec.RunID,
		OwnerID:             rec.OwnerID,
		ComputerID:          agentMutationComputerID(rec),
		State:               "pending",
		ScheduledMessageSeq: scheduledSeq,
		CreatedAt:           rec.CreatedAt,
	}
}
