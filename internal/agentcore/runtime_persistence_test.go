package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type fakeRunSubmissionStore struct {
	agents      []types.AgentRecord
	runs        []types.RunRecord
	mutations   []store.AgentMutation
	events      []types.EventRecord
	mutationErr error
}

func (s *fakeRunSubmissionStore) UpsertAgent(_ context.Context, agent types.AgentRecord) error {
	s.agents = append(s.agents, agent)
	return nil
}

func (s *fakeRunSubmissionStore) CreateRun(_ context.Context, rec types.RunRecord) error {
	s.runs = append(s.runs, rec)
	return nil
}

func (s *fakeRunSubmissionStore) CreateAgentMutation(_ context.Context, mutation store.AgentMutation) error {
	if s.mutationErr != nil {
		return s.mutationErr
	}
	s.mutations = append(s.mutations, mutation)
	return nil
}

func (s *fakeRunSubmissionStore) AppendEvent(_ context.Context, event *types.EventRecord) error {
	s.events = append(s.events, *event)
	return nil
}

func TestPersistSubmittedRunUsesRuntimeStoreInterfaceWithoutDolt(t *testing.T) {
	createdAt := time.Date(2026, 5, 24, 1, 2, 3, 0, time.UTC)
	agent := types.AgentRecord{
		AgentID:   "texture:doc-1",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-test",
		Profile:   agentprofile.Texture,
		Role:      agentprofile.Texture,
		ChannelID: "doc-1",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	rec := &types.RunRecord{
		RunID:        "run-1",
		AgentID:      agent.AgentID,
		ChannelID:    agent.ChannelID,
		AgentProfile: agent.Profile,
		AgentRole:    agent.Role,
		OwnerID:      agent.OwnerID,
		SandboxID:    agent.SandboxID,
		State:        types.RunPending,
		Prompt:       "revise",
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
		Metadata: map[string]any{
			"type":                  "texture_agent_revision",
			"doc_id":                "doc-1",
			"scheduled_message_seq": float64(7),
		},
	}

	fake := &fakeRunSubmissionStore{}
	bus := events.NewEventBus()
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	if err := persistSubmittedRun(context.Background(), fake, bus, agent, rec, len(rec.Prompt), nil); err != nil {
		t.Fatalf("persistSubmittedRun: %v", err)
	}
	if len(fake.agents) != 1 || fake.agents[0].AgentID != agent.AgentID {
		t.Fatalf("agents = %+v", fake.agents)
	}
	if len(fake.runs) != 1 || fake.runs[0].RunID != rec.RunID {
		t.Fatalf("runs = %+v", fake.runs)
	}
	if len(fake.mutations) != 1 || fake.mutations[0].DocID != "doc-1" || fake.mutations[0].ScheduledMessageSeq != 7 {
		t.Fatalf("mutations = %+v", fake.mutations)
	}
	if len(fake.events) != 1 || fake.events[0].Kind != types.EventRunSubmitted {
		t.Fatalf("events = %+v", fake.events)
	}
	var payload map[string]int
	if err := json.Unmarshal(fake.events[0].Payload, &payload); err != nil {
		t.Fatalf("decode submitted payload: %v", err)
	}
	if payload["prompt_length"] != len(rec.Prompt) {
		t.Fatalf("prompt_length = %d, want %d", payload["prompt_length"], len(rec.Prompt))
	}
	select {
	case ev := <-ch:
		if ev.Record.EventID != fake.events[0].EventID || ev.Cause != events.CauseTaskLifecycle {
			t.Fatalf("published event = %+v, persisted = %+v", ev, fake.events[0])
		}
	default:
		t.Fatal("expected submitted event on bus")
	}
}

func TestPersistSubmittedRunFailsClosedWithoutTextureMutationAuthority(t *testing.T) {
	rec := &types.RunRecord{
		RunID: "run-no-mutation", AgentID: "texture:doc-no-mutation", ChannelID: "doc-no-mutation",
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		OwnerID: "user-alice", SandboxID: "sandbox-test", State: types.RunPending,
		Metadata: map[string]any{
			"type": "texture_agent_revision", "doc_id": "doc-no-mutation",
		},
	}
	agent := types.AgentRecord{
		AgentID: rec.AgentID, OwnerID: rec.OwnerID, SandboxID: rec.SandboxID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: rec.ChannelID,
	}
	fake := &fakeRunSubmissionStore{mutationErr: errors.New("mutation store unavailable")}
	err := persistSubmittedRun(context.Background(), fake, events.NewEventBus(), agent, rec, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "persist Texture mutation authority") {
		t.Fatalf("persistence error = %v, want mutation-authority refusal", err)
	}
	if len(fake.events) != 0 {
		t.Fatalf("submitted event persisted without mutation authority: %+v", fake.events)
	}
}
