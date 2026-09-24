package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type researchFindingEvidenceInput struct {
	Kind      string          `json:"kind"`
	SourceURI string          `json:"source_uri,omitempty"`
	Title     string          `json:"title,omitempty"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

func ensureFindingEvidence(ctx context.Context, s *store.Store, ownerID, agentID, findingID string, index int, item researchFindingEvidenceInput) (types.EvidenceRecord, error) {
	kind := strings.TrimSpace(item.Kind)
	if kind == "" {
		return types.EvidenceRecord{}, fmt.Errorf("evidence[%d].kind must not be empty", index)
	}
	content := strings.TrimSpace(item.Content)
	if content == "" {
		return types.EvidenceRecord{}, fmt.Errorf("evidence[%d].content must not be empty", index)
	}
	evidenceID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("choir:research-finding:"+findingID+fmt.Sprintf(":%d", index))).String()
	rec := types.EvidenceRecord{
		EvidenceID: evidenceID,
		OwnerID:    ownerID,
		AgentID:    agentID,
		Kind:       kind,
		SourceURI:  strings.TrimSpace(item.SourceURI),
		Title:      strings.TrimSpace(item.Title),
		Content:    item.Content,
		Metadata:   item.Metadata,
		CreatedAt:  time.Now().UTC(),
	}
	if len(rec.Metadata) == 0 {
		rec.Metadata = json.RawMessage(`{}`)
	}
	existing, err := s.GetEvidence(ctx, evidenceID, ownerID)
	if err == nil {
		if existing.AgentID != rec.AgentID || existing.Kind != rec.Kind || existing.SourceURI != rec.SourceURI || existing.Title != rec.Title || existing.Content != rec.Content || rawJSONText(existing.Metadata) != rawJSONText(rec.Metadata) {
			return types.EvidenceRecord{}, fmt.Errorf("finding_id %s reuses evidence slot %d with different payload", findingID, index)
		}
		return existing, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return types.EvidenceRecord{}, err
	}
	if err := s.CreateEvidence(ctx, rec); err != nil {
		return types.EvidenceRecord{}, err
	}
	return rec, nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func trimNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func rawJSONText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return strings.TrimSpace(string(raw))
}

func (rt *Runtime) emitChannelMessageEvent(ctx context.Context, message types.ChannelMessage, ownerID string) {
	payload, err := json.Marshal(map[string]any{
		"channel_id":    message.ChannelID,
		"cursor":        message.Seq,
		"from":          message.From,
		"from_agent_id": message.FromAgentID,
		"from_loop_id":  message.FromRunID,
		"to_agent_id":   message.ToAgentID,
		"to_loop_id":    message.ToRunID,
		"trajectory_id": message.TrajectoryID,
		"role":          message.Role,
		"content":       message.Content,
	})
	if err != nil {
		log.Printf("runtime: marshal channel event payload: %v", err)
		return
	}
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        message.FromRunID,
		AgentID:      message.FromAgentID,
		ChannelID:    message.ChannelID,
		OwnerID:      ownerID,
		TrajectoryID: message.TrajectoryID,
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventChannelMessage,
		Payload:      payload,
	}
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist channel event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorChannel,
		Cause:  events.CauseChannelMessage,
	})
}
