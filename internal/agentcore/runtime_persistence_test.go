package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"

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

func TestPersistSubmittedRunFailsClosedWithoutTextureMutationAuthority(t *testing.T) {
	rec := &types.RunRecord{
		RunID: "run-no-mutation", AgentID: "texture:doc-no-mutation", ChannelID: "doc-no-mutation",
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		OwnerID: "user-alice", ComputerID: "autoputer-test", State: types.RunPending,
		Metadata: map[string]any{
			"type": "texture_agent_revision", "doc_id": "doc-no-mutation",
		},
	}
	agent := types.AgentRecord{
		AgentID: rec.AgentID, OwnerID: rec.OwnerID, ComputerID: rec.ComputerID,
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
