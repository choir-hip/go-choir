package yaegikernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// choir.System1 is the research desk's closed-question verb
// (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md, F8 and
// F9): it exists on the research desk only, its handle authorizes only that
// desk, and a malformed question list is refused before it leaves the cell.

func system1Scope(t *testing.T, role, slot string) (*ChoirScope, *Broker) {
	t.Helper()
	broker, issuer, _, _ := testChoirFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-system1", 1, role, slot)
	if err != nil {
		t.Fatalf("scope for %q: %v", role, err)
	}
	return scope, broker
}

func TestSystem1IsExportedToTheResearchDeskOnly(t *testing.T) {
	cases := []struct {
		role, slot string
		want       bool
	}{
		{SessionRoleResearch, "", true},
		{"management", "", false},
		{"engineering", "implementation", false},
		{"engineering", "verifier", false},
		{"texture", "", false},
		{SessionRoleResearchChild, "", false},
		{SessionRoleManagementTask, "", false},
		{"", "", false},
		{"unknown-desk", "", false},
	}
	for _, tc := range cases {
		scope, _ := system1Scope(t, tc.role, tc.slot)
		_, got := scope.ChoirExports()["choir/choir"]["System1"]
		if got != tc.want {
			t.Errorf("role %q slot %q exports System1 = %v, want %v", tc.role, tc.slot, got, tc.want)
		}
		surface := DeskSurface(tc.role, tc.slot, scope.readOnly)
		if strings.Contains(surface, "choir.System1(") != tc.want {
			t.Errorf("role %q prompt surface lists System1 = %v, want %v", tc.role, !tc.want, tc.want)
		}
		if tc.want {
			if help := scope.Help("System1"); !strings.Contains(help, "System1(") {
				t.Errorf("choir.Help(System1) = %q", help)
			}
		}
	}
}

func TestSystem1HandleAuthorizesOnlyTheResearchDesk(t *testing.T) {
	good := []System1Question{{ID: "q1", Text: "Same incident?", Choices: []string{"yes", "no"}}}
	for _, role := range []string{"management", "engineering", "texture", SessionRoleResearchChild, SessionRoleManagementTask} {
		scope, broker := system1Scope(t, role, "")
		broker.SetEgress(func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
			t.Errorf("role %s reached the host egress", role)
			return json.RawMessage(`{}`), nil
		})
		if _, err := scope.System1(good); err == nil || !strings.Contains(err.Error(), "handle verification failed") {
			t.Errorf("role %s: System1 = %v, want a handle refusal", role, err)
		}
	}
	scope, broker := system1Scope(t, SessionRoleResearch, "")
	var got System1Payload
	broker.SetEgress(func(_ context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
		if action != ActionSystem1 {
			t.Fatalf("action = %q, want %q", action, ActionSystem1)
		}
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(`{"receipt_ref":"system1:e1","answers":[]}`), nil
	})
	out, err := scope.System1([]map[string]any{{"id": " q1 ", "text": "Same incident?", "choices": []string{"yes", "no"}, "context_refs": []string{"story:rev-1"}}})
	if err != nil {
		t.Fatalf("research System1: %v", err)
	}
	if !strings.Contains(string(out), "system1:e1") {
		t.Fatalf("result = %s", out)
	}
	if len(got.Questions) != 1 || got.Questions[0].ID != "q1" || got.Questions[0].ContextRefs[0] != "story:rev-1" {
		t.Fatalf("host saw %+v; want the normalized question", got.Questions)
	}
}

func TestSystem1RefusesAMalformedQuestionListBeforeTheHost(t *testing.T) {
	ok := func() System1Question {
		return System1Question{ID: "q1", Text: "Same incident?", Choices: []string{"yes", "no"}}
	}
	many := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = string(rune('a' + i))
		}
		return out
	}
	cases := []struct {
		name string
		qs   []System1Question
		want string
	}{
		{"empty list", nil, "at least one question"},
		{"too many questions", func() []System1Question {
			var qs []System1Question
			for i := 0; i <= System1MaxQuestions; i++ {
				q := ok()
				q.ID = "q" + strings.Repeat("x", i+1)
				qs = append(qs, q)
			}
			return qs
		}(), "at most"},
		{"empty id", []System1Question{{ID: "", Text: "t", Choices: []string{"a", "b"}}}, "id"},
		{"id with spaces", []System1Question{{ID: "a b", Text: "t", Choices: []string{"a", "b"}}}, "id"},
		{"duplicate ids", []System1Question{ok(), ok()}, "duplicate"},
		{"empty text", []System1Question{{ID: "q1", Text: "  ", Choices: []string{"a", "b"}}}, "text"},
		{"oversized text", []System1Question{{ID: "q1", Text: strings.Repeat("x", System1MaxTextRunes+1), Choices: []string{"a", "b"}}}, "text"},
		{"one choice", []System1Question{{ID: "q1", Text: "t", Choices: []string{"a"}}}, "choices"},
		{"too many choices", []System1Question{{ID: "q1", Text: "t", Choices: many(System1MaxChoices + 1)}}, "choices"},
		{"duplicate choices", []System1Question{{ID: "q1", Text: "t", Choices: []string{"a", " a "}}}, "duplicate choice"},
		{"empty choice", []System1Question{{ID: "q1", Text: "t", Choices: []string{"a", ""}}}, "choice"},
		{"oversized choice", []System1Question{{ID: "q1", Text: "t", Choices: []string{"a", strings.Repeat("c", System1MaxChoiceRunes+1)}}}, "choice"},
		{"too many refs on one question", []System1Question{{ID: "q1", Text: "t", Choices: []string{"a", "b"}, ContextRefs: []string{"story:1", "story:2", "story:3", "story:4", "story:5"}}}, "context_refs"},
		{"empty ref", []System1Question{{ID: "q1", Text: "t", Choices: []string{"a", "b"}, ContextRefs: []string{" "}}}, "context_refs"},
	}
	for _, tc := range cases {
		if _, err := ValidateSystem1Questions(tc.qs); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to contain %q", tc.name, err, tc.want)
		}
	}

	scope, broker := system1Scope(t, SessionRoleResearch, "")
	broker.SetEgress(func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
		t.Error("a malformed question list reached the host")
		return nil, nil
	})
	if _, err := scope.System1([]map[string]any{{"id": "q1", "text": "t", "choices": []string{"a", "b"}, "evidence": "the court ruled X"}}); err == nil || !strings.Contains(err.Error(), "context_refs") {
		t.Errorf("free-text evidence field: err = %v, want a refusal naming context_refs", err)
	}
	if _, err := scope.System1("not a list"); err == nil {
		t.Error("a string argument was accepted")
	}
	if _, err := scope.System1(nil); err == nil {
		t.Error("nil questions were accepted")
	}
}

func TestSystem1DistinctContextRefsAreBoundedPerCall(t *testing.T) {
	// Five questions of four distinct refs each is 20 refs, past the 16 a call may name.
	var qs []System1Question
	for i := 0; i < 5; i++ {
		var refs []string
		for j := 0; j < System1MaxRefsPerQuestion; j++ {
			refs = append(refs, "story:rev-"+string(rune('a'+i))+string(rune('a'+j)))
		}
		qs = append(qs, System1Question{ID: "q" + string(rune('a'+i)), Text: "t", Choices: []string{"a", "b"}, ContextRefs: refs})
	}
	if _, err := ValidateSystem1Questions(qs); err == nil || !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("err = %v, want a distinct-refs refusal", err)
	}
	// The same refs shared across questions count once.
	shared := []System1Question{
		{ID: "q1", Text: "t", Choices: []string{"a", "b"}, ContextRefs: []string{"story:rev-1", "story:rev-2"}},
		{ID: "q2", Text: "t", Choices: []string{"a", "b"}, ContextRefs: []string{"story:rev-2", "story:rev-1"}},
	}
	if _, err := ValidateSystem1Questions(shared); err != nil {
		t.Fatalf("shared refs: %v", err)
	}
}
