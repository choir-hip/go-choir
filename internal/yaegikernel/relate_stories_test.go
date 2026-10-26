package yaegikernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// choir.RelateStories is the research desk's record of a judged pair relation
// (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md, "Build:
// the related_story edge", B1 and B4): it exists on the research desk only,
// its handle authorizes only that desk, and a malformed pair list is refused
// before it leaves the cell.

func pubID(t *testing.T, suffix string) string {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.ObjectKind("choir.publication"), "owner-1", suffix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestRelateStoriesIsExportedToTheResearchDeskOnly(t *testing.T) {
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
		_, got := scope.ChoirExports()["choir/choir"]["RelateStories"]
		if got != tc.want {
			t.Errorf("role %q slot %q exports RelateStories = %v, want %v", tc.role, tc.slot, got, tc.want)
		}
		surface := DeskSurface(tc.role, tc.slot, scope.readOnly)
		if strings.Contains(surface, "choir.RelateStories(") != tc.want {
			t.Errorf("role %q prompt surface lists RelateStories = %v, want %v", tc.role, !tc.want, tc.want)
		}
	}
}

func TestRelateStoriesHandleAuthorizesOnlyTheResearchDesk(t *testing.T) {
	pair := map[string]any{"story_a": pubID(t, "a"), "story_b": pubID(t, "b"), "relation": "same_incident", "judgment_ref": "system1:e1", "question_id": "q1"}
	for _, role := range []string{"management", "engineering", "texture", SessionRoleResearchChild, SessionRoleManagementTask} {
		scope, broker := system1Scope(t, role, "")
		broker.SetEgress(func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
			t.Errorf("role %s reached the host egress", role)
			return json.RawMessage(`{}`), nil
		})
		if _, err := scope.RelateStories([]map[string]any{pair}); err == nil || !strings.Contains(err.Error(), "handle verification failed") {
			t.Errorf("role %s: RelateStories = %v, want a handle refusal", role, err)
		}
	}
	scope, broker := system1Scope(t, SessionRoleResearch, "")
	var got RelateStoriesPayload
	broker.SetEgress(func(_ context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
		if action != ActionRelateStories {
			t.Fatalf("action = %q, want %q", action, ActionRelateStories)
		}
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(`{"recorded":[],"exported":true}`), nil
	})
	if _, err := scope.RelateStories([]map[string]any{pair}); err != nil {
		t.Fatalf("research RelateStories: %v", err)
	}
	if len(got.Pairs) != 1 || got.Pairs[0].Relation != types.StoryRelationSameIncident || got.Pairs[0].StoryA >= got.Pairs[0].StoryB {
		t.Fatalf("host saw %+v; want the normalized pair, stored order", got.Pairs)
	}
}

func TestRelateStoriesRefusesAMalformedPairListBeforeTheHost(t *testing.T) {
	a, b := pubID(t, "a"), pubID(t, "b")
	ok := func() RelateStoriesPair {
		return RelateStoriesPair{StoryA: a, StoryB: b, Relation: "same_incident", JudgmentRef: "system1:e1", QuestionID: "q1"}
	}
	with := func(f func(*RelateStoriesPair)) []RelateStoriesPair {
		p := ok()
		f(&p)
		return []RelateStoriesPair{p}
	}
	many := make([]RelateStoriesPair, RelateStoriesMaxPairs+1)
	for i := range many {
		many[i] = ok()
	}
	cases := []struct {
		name  string
		pairs []RelateStoriesPair
		want  string
	}{
		{"empty list", nil, "at least one"},
		{"too many", many, "12"},
		{"self pair", with(func(p *RelateStoriesPair) { p.StoryB = p.StoryA }), "itself"},
		{"empty story", with(func(p *RelateStoriesPair) { p.StoryA = " " }), "publication"},
		{"malformed id", with(func(p *RelateStoriesPair) { p.StoryA = "obj:choir.publication" }), "publication"},
		{"another kind", with(func(p *RelateStoriesPair) { p.StoryA = "obj:choir.web_capture:b3duZXI:abc" }), "publication"},
		{"unknown relation", with(func(p *RelateStoriesPair) { p.Relation = "unrelated" }), "relation"},
		{"no receipt", with(func(p *RelateStoriesPair) { p.JudgmentRef = "" }), "judgment_ref"},
		{"receipt without the prefix", with(func(p *RelateStoriesPair) { p.JudgmentRef = "e1" }), "judgment_ref"},
		{"no question id", with(func(p *RelateStoriesPair) { p.QuestionID = "" }), "question_id"},
		{"oversized reason", with(func(p *RelateStoriesPair) { p.Reason = strings.Repeat("r", RelateStoriesMaxReasonRunes+1) }), "reason"},
	}
	for _, tc := range cases {
		if _, err := ValidateRelateStoriesPairs(tc.pairs); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to contain %q", tc.name, err, tc.want)
		}
	}

	// Normalized: trimmed, relation spelled canonically, stored order, and the
	// same pair in both orders with the same relation and judgment is one.
	rev := ok()
	rev.StoryA, rev.StoryB = b, a
	rev.Relation = " Same_Incident "
	got, err := ValidateRelateStoriesPairs([]RelateStoriesPair{ok(), rev})
	if err != nil || len(got) != 1 || got[0].StoryA >= got[0].StoryB || got[0].Relation != "same_incident" {
		t.Fatalf("normalized = %+v, %v; want one pair in stored order", got, err)
	}

	scope, broker := system1Scope(t, SessionRoleResearch, "")
	broker.SetEgress(func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
		t.Error("a malformed pair list reached the host")
		return nil, nil
	})
	if _, err := scope.RelateStories([]map[string]any{{"story_a": a, "story_b": b, "relation": "same_incident", "judgment_ref": "system1:e1", "question_id": "q1", "probability": 0.99}}); err == nil {
		t.Error("a probability supplied by the cell was accepted; the host reads it from the judgment")
	}
	if _, err := scope.RelateStories("not a list"); err == nil {
		t.Error("a string argument was accepted")
	}
}
