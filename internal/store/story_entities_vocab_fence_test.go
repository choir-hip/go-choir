package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The research desk's story.entities tape event carries attachments naming
// how an entity relates to a story (central, incidental). The vocabulary write
// guard reads a leaf named "role" as a desk role, so the field is
// "entity_role". The cutover is set the way production sets it: the fenced
// migration (MigrateAndFenceServingVocabulary) writes the report that arms the
// guard. Earlier tests ran on stores that never cut over and missed the refusal
// (docs/problems/related-stories-not-showing-on-the-paper-2026-10-27.md).
func storyEntitiesEvent(id, field, value string) *types.EventRecord {
	payload, _ := json.Marshal(map[string]any{
		"computer_id": "c1",
		"story":       "pub-1",
		"attachments": []map[string]any{
			{"entity_id": "wd:Q1", field: value, "probability": 0.7, "top": true, "judgment_ref": "system1:e1", "question_id": "q1"},
			{"entity_id": "wd:Q2", field: "incidental", "probability": 0.2, "top": true, "judgment_ref": "system1:e1", "question_id": "q2"},
		},
	})
	return &types.EventRecord{EventID: id, Timestamp: time.Now().UTC(), RunID: "run-1", OwnerID: "owner-1", AgentID: "research:doc-1",
		ChannelID: "system1:c1", Kind: types.EventStoryEntities, Phase: "recorded", Payload: payload}
}

func TestStoryEntitiesEventWithEntityRoleIsAdmittedAfterCutover(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if _, err := s.MigrateAndFenceServingVocabulary(ctx, false, nil); err != nil {
		t.Fatal(err)
	}
	if !s.vocabCutover.Load() {
		t.Fatal("fixture: the fenced migration did not arm the write guard")
	}
	if err := s.AppendEvent(ctx, storyEntitiesEvent("se-1", "entity_role", "central")); err != nil {
		t.Fatalf("append story.entities with entity_role after cutover: %v", err)
	}
	events, err := s.ListEvents(ctx, "run-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []byte
	for _, e := range events {
		if e.Kind == types.EventStoryEntities {
			got = e.Payload
		}
	}
	if got == nil {
		t.Fatalf("the story.entities event did not read back (%d events)", len(events))
	}
	var rec struct {
		Attachments []struct {
			EntityID   string `json:"entity_id"`
			EntityRole string `json:"entity_role"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(got, &rec); err != nil || len(rec.Attachments) != 2 ||
		rec.Attachments[0].EntityRole != "central" || rec.Attachments[1].EntityRole != "incidental" {
		t.Fatalf("read back %s (%v), want central and incidental entity roles", got, err)
	}
}

// The guard stays: a desk-role leaf outside the vocabulary is still refused,
// and so is a bare "role" carrying an entity role (the shape that was refused
// on the paper).
func TestStoryEntitiesEventStillRefusesDeskRoleViolations(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if _, err := s.MigrateAndFenceServingVocabulary(ctx, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, storyEntitiesEvent("se-2", "role", "central")); err == nil || !strings.Contains(err.Error(), "vocab") {
		t.Fatalf("a bare role=central attachment: err = %v, want the vocabulary guard to refuse it", err)
	}
	if err := s.AppendEvent(ctx, storyEntitiesEvent("se-3", "role", "boss")); err == nil || !strings.Contains(err.Error(), "vocab") {
		t.Fatalf("role=boss: err = %v, want the vocabulary guard to refuse it", err)
	}
}
