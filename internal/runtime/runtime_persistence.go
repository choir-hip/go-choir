package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"

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
	if mutation := agentMutationForRun(rec); mutation != nil {
		if err := st.CreateAgentMutation(ctx, *mutation); err != nil {
			log.Printf("runtime: texture agent revision run %s: create mutation: %v", rec.RunID, err)
		}
	}

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
		bus.Publish(events.RuntimeEvent{
			Record: *evRec,
			Actor:  events.ActorRuntime,
			Cause:  events.CauseTaskLifecycle,
		})
	}
	return nil
}

func agentMutationForRun(rec *types.RunRecord) *store.AgentMutation {
	if rec == nil || !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
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
		State:               "pending",
		ScheduledMessageSeq: scheduledSeq,
		CreatedAt:           rec.CreatedAt,
	}
}
