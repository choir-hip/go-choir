package yaegikernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// choir.Relate is the research desk's record of a published story's
// entities (docs/problems/related-stories-no-entity-layer-2026-10-26.md,
// slice a; design docs/reports/related-stories-phase3-research-review-2026-10-20.md
// §3.2 and §11 slice 1). Ways it can fail at the cell, written first:
//
//  E1 Another desk gets the verb, or its handle reaches the host.
//  E4 A malformed attachment leaves the cell: no story, a malformed story or
//     entity id, both or neither of entity_id and name, a new entity with no
//     type or an unknown one, a Wikidata id that is not a QID, an unknown
//     role, no System 1 receipt or question, an oversized reason, more than
//     12 attachments, the same entity twice (by id, or two new names that
//     normalise alike), or a probability the cell supplies.
//  E10 The desk's own story cannot be named before it is published (review
//     304 item 3): "" and "story:head" are the desk's own story, kept as
//     RelateOwnStory for the host to bind; any other non-publication form is
//     refused.

func TestRelateIsExportedToTheResearchDeskOnly(t *testing.T) {
	cases := []struct {
		role, slot string
		want       bool
	}{
		{SessionRoleResearch, "", true},
		{"management", "", false},
		{"engineering", "implementation", false},
		{"texture", "", false},
		{SessionRoleResearchChild, "", false},
		{SessionRoleManagementTask, "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		scope, _ := system1Scope(t, tc.role, tc.slot)
		_, got := scope.ChoirExports()["choir/choir"]["Relate"]
		if got != tc.want {
			t.Errorf("role %q slot %q exports Relate = %v, want %v", tc.role, tc.slot, got, tc.want)
		}
		surface := DeskSurface(tc.role, tc.slot, scope.readOnly)
		if strings.Contains(surface, "choir.Relate(") != tc.want {
			t.Errorf("role %q prompt surface lists Relate = %v, want %v", tc.role, !tc.want, tc.want)
		}
	}
}

func relateEntityOK() RelateEntity {
	return RelateEntity{Name: "Aleksandar Vucic", Type: types.EntityTypePerson, Role: types.EntityRoleCentral, JudgmentRef: "system1:e1", QuestionID: "q1"}
}

func TestRelateHandleAuthorizesOnlyTheResearchDesk(t *testing.T) { // E1
	story := pubID(t, "a")
	ent := map[string]any{"name": "Aleksandar Vucic", "type": "person", "role": "central", "judgment_ref": "system1:e1", "question_id": "q1"}
	for _, role := range []string{"management", "engineering", "texture", SessionRoleResearchChild, SessionRoleManagementTask} {
		scope, broker := system1Scope(t, role, "")
		broker.SetEgress(func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
			t.Errorf("role %s reached the host egress", role)
			return json.RawMessage(`{}`), nil
		})
		if _, err := scope.Relate(story, []map[string]any{ent}); err == nil || !strings.Contains(err.Error(), "handle verification failed") {
			t.Errorf("role %s: Relate = %v, want a handle refusal", role, err)
		}
	}
	scope, broker := system1Scope(t, SessionRoleResearch, "")
	var got RelatePayload
	broker.SetEgress(func(_ context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
		if action != ActionRelate {
			t.Fatalf("action = %q, want %q", action, ActionRelate)
		}
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(`{"attached":[],"exported":true}`), nil
	})
	if _, err := scope.Relate(" "+story+" ", []map[string]any{ent}); err != nil {
		t.Fatalf("research Relate: %v", err)
	}
	if got.Story != story || len(got.Entities) != 1 || got.Entities[0].Role != types.EntityRoleCentral || got.Entities[0].Name != "Aleksandar Vucic" {
		t.Fatalf("host saw %+v; want the trimmed story and the attachment", got)
	}
}

func TestRelateRefusesAMalformedAttachmentBeforeTheHost(t *testing.T) { // E4
	story := pubID(t, "a")
	with := func(f func(*RelateEntity)) RelatePayload {
		e := relateEntityOK()
		f(&e)
		return RelatePayload{Story: story, Entities: []RelateEntity{e}}
	}
	many := RelatePayload{Story: story}
	for i := 0; i <= RelateMaxEntities; i++ {
		e := relateEntityOK()
		e.Name = "Entity " + strings.Repeat("x", i+1)
		many.Entities = append(many.Entities, e)
	}
	twice := RelatePayload{Story: story, Entities: []RelateEntity{relateEntityOK(), relateEntityOK()}}
	twice.Entities[1].Name = "aleksandar vucic's"
	twiceByID := RelatePayload{Story: story, Entities: []RelateEntity{
		{EntityID: "wd:Q312", Role: "central", JudgmentRef: "system1:e1", QuestionID: "q1"},
		{EntityID: "wd:Q312", Role: "incidental", JudgmentRef: "system1:e1", QuestionID: "q2"},
	}}
	cases := []struct {
		name string
		p    RelatePayload
		want string
	}{
		{"another story form", RelatePayload{Story: "story:tail", Entities: []RelateEntity{relateEntityOK()}}, "publication"},
		{"malformed story", RelatePayload{Story: "pub-12", Entities: []RelateEntity{relateEntityOK()}}, "publication"},
		{"no attachment", RelatePayload{Story: story}, "at least one"},
		{"too many", many, "12"},
		{"neither id nor name", with(func(e *RelateEntity) { e.Name = "" }), "entity_id or name"},
		{"both id and name", with(func(e *RelateEntity) { e.EntityID = "wd:Q312" }), "entity_id or name"},
		{"malformed entity id", with(func(e *RelateEntity) { e.Name, e.Type, e.EntityID = "", "", "ent:xyz" }), "entity_id"},
		{"type with an existing entity", with(func(e *RelateEntity) { e.Name, e.EntityID = "", "wd:Q312" }), "entity_id alone"},
		{"new entity with no type", with(func(e *RelateEntity) { e.Type = "" }), "type"},
		{"unknown type", with(func(e *RelateEntity) { e.Type = "topic" }), "type"},
		{"name that normalises to nothing", with(func(e *RelateEntity) { e.Name = "The Inc." }), "name"},
		{"overlong name", with(func(e *RelateEntity) { e.Name = strings.Repeat("n", RelateMaxNameRunes+1) }), "name"},
		{"wikidata not a QID", with(func(e *RelateEntity) { e.WikidataID = "312" }), "wikidata_id"},
		{"unknown role", with(func(e *RelateEntity) { e.Role = "mentioned" }), "role"},
		{"no receipt", with(func(e *RelateEntity) { e.JudgmentRef = "" }), "judgment_ref"},
		{"receipt without the prefix", with(func(e *RelateEntity) { e.JudgmentRef = "e1" }), "judgment_ref"},
		{"no question", with(func(e *RelateEntity) { e.QuestionID = "" }), "question_id"},
		{"oversized reason", with(func(e *RelateEntity) { e.Reason = strings.Repeat("r", RelateStoriesMaxReasonRunes+1) }), "reason"},
		{"one new entity twice", twice, "twice"},
		{"one entity id twice", twiceByID, "twice"},
	}
	for _, tc := range cases {
		if _, err := ValidateRelate(tc.p); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to contain %q", tc.name, err, tc.want)
		}
	}

	// Normalized: trimmed, role and type lower case, the Wikidata id kept.
	got, err := ValidateRelate(with(func(e *RelateEntity) {
		e.Name, e.Type, e.Role, e.WikidataID = "  Aleksandar Vucic ", " Person ", " Central ", " Q312 "
	}))
	if err != nil || got.Entities[0].Name != "Aleksandar Vucic" || got.Entities[0].Type != "person" || got.Entities[0].Role != "central" || got.Entities[0].WikidataID != "Q312" {
		t.Fatalf("normalized = %+v, %v", got, err)
	}

	scope, broker := system1Scope(t, SessionRoleResearch, "")
	broker.SetEgress(func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
		t.Error("a malformed attachment reached the host")
		return nil, nil
	})
	if _, err := scope.Relate(story, []map[string]any{{"name": "Aleksandar Vucic", "type": "person", "role": "central", "judgment_ref": "system1:e1", "question_id": "q1", "probability": 0.99}}); err == nil {
		t.Error("a probability supplied by the cell was accepted; the host reads it from the judgment")
	}
	if _, err := scope.Relate(story, "not a list"); err == nil {
		t.Error("a string argument was accepted")
	}
}

func TestRelateNamesTheDesksOwnStoryByEmptyOrHead(t *testing.T) { // E10
	for _, story := range []string{"", "story:head", "  story:head ", " "} {
		got, err := ValidateRelate(RelatePayload{Story: story, Entities: []RelateEntity{relateEntityOK()}})
		if err != nil || got.Story != RelateOwnStory {
			t.Errorf("story %q: got %q, %v; want %q", story, got.Story, err, RelateOwnStory)
		}
	}
	scope, broker := system1Scope(t, SessionRoleResearch, "")
	var sent RelatePayload
	broker.SetEgress(func(_ context.Context, _ BrokerAction, raw json.RawMessage) (json.RawMessage, error) {
		if err := json.Unmarshal(raw, &sent); err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(`{}`), nil
	})
	if _, err := scope.Relate("", []RelateEntity{relateEntityOK()}); err != nil {
		t.Fatalf("Relate(\"\"): %v", err)
	}
	if sent.Story != RelateOwnStory {
		t.Fatalf("the host was sent story %q, want %q for the host to bind", sent.Story, RelateOwnStory)
	}
}
