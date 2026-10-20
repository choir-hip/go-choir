package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Owner ruling 2026-10-20: one event is one incident, one place, one time;
// a later development is a new event linked to the earlier one. The ways
// this could fail (F1-F8) are written in docs/problems/event-layer-false-
// merges-at-marginal-p-2026-10-16.md and summarised at the top of
// event_layer_test.go. These tests prove the plumbing: what Jev is asked,
// what a linked answer writes, and that true duplicates still merge. They
// cannot prove Jev reads the new text correctly; the staging blind sample
// (scripts/event-blind-sample.py) is that evidence.

// questionChoices returns the choices of capture key's question in call n.
func questionChoices(t *testing.T, j *eventJudge, call int, key string) []string {
	t.Helper()
	var q struct {
		Choices []string `json:"choices"`
	}
	if err := json.Unmarshal(j.asked[call][key], &q); err != nil {
		t.Fatal(err)
	}
	return q.Choices
}

func afterChoices(choices []string) []string {
	var out []string
	for _, c := range choices {
		if strings.HasPrefix(c, "after_") {
			out = append(out, c)
		}
	}
	return out
}

func joinRaw(m map[string]json.RawMessage) []byte {
	var b []byte
	for _, r := range m {
		b = append(b, r...)
	}
	return b
}

// F1: the request carries the one-incident definition everywhere Jev reads
// a definition, and nowhere says a development is the same event.
func TestEventLayerRequestDefinesAnEventAsOneIncident(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("d1", "Two container ships sink near Hormuz", "https://a.example.com/1", "")}); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("d2", "Ship attacks near Hormuz at a record high", "https://b.example.net/2", "")}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"one incident", "one place", "one time", "later development"} {
		if !strings.Contains(strings.ToLower(eventDefinition), want) {
			t.Fatalf("eventDefinition lacks %q: %s", want, eventDefinition)
		}
	}
	for i, call := range judge.calls {
		task, _ := call["task"].(string)
		if !strings.Contains(task, eventDefinition) {
			t.Fatalf("call %d task lacks the definition: %s", i, task)
		}
		for key, raw := range judge.asked[i] {
			var q struct {
				Instructions string `json:"instructions"`
			}
			_ = json.Unmarshal(raw, &q)
			if !strings.Contains(q.Instructions, eventDefinition) {
				t.Fatalf("call %d question %s lacks the definition: %s", i, key, raw)
			}
		}
		blob, _ := json.Marshal(call)
		for _, bad := range []string{"direct development", "same ongoing story in a different occurrence"} {
			if strings.Contains(string(blob), bad) || strings.Contains(string(joinRaw(judge.asked[i])), bad) {
				t.Fatalf("call %d still carries the story-level wording %q", i, bad)
			}
		}
	}
}

// F3, F4, F5: the story-level pairs from the false-merge tables (problem
// doc, both reruns). Under the one-incident definition Jev answers each
// "a later development / another incident" (after_), or "new" when no
// earlier event was offered to link to. Either way the pair must be two
// events, neither corroborated; the link is set only when it was offered.
func TestEventLayerStoryLevelPairsAreSeparateLinkedEvents(t *testing.T) {
	pairs := []struct {
		name, opener, member string
		offered              bool // title words overlap, so the link is offered
	}{
		{"houthi live blog", "Iran war live: Saudi blasts Houthi war crimes", "Middle East updates: Houthis target Yemen government palace", true},
		{"france protests", "Are student protests being manipulated ahead of France presidential vote", "French police accused over war-like injuries to teenagers", false},
		{"starshield", "Ukraine says its Starshield access is back", "Wes Streeting urges US to back Ukraine on Washington visit", true},
		{"netanyahu", "Will Israeli voters punish Benjamin Netanyahu?", "In Israel, Netanyahu ultimate legacy is the opposition", true},
		{"recession", "What is your personal recession indicator?", "Bad news for Trump: it is still the economy", false},
		{"hormuz", "Ship attack reports near Hormuz at a record high", "Two container ships sink near Hormuz", true},
		{"indonesia fires", "Indonesia fights fire with dry ice as haze spreads", "Wildfire kills 4 firefighters at Mount Bromo", false},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			judge := &eventJudge{answers: map[string]eventAnswer{
				p.member: {target: p.opener, after: true, p: 0.98},
			}}
			rt, owner, computer := eventRuntime(t, judge)
			ctx := context.Background()
			first, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("o1", p.opener, "https://a.example.com/1", "")})
			if err != nil || first.Minted != 1 {
				t.Fatalf("opener: %+v %v", first, err)
			}
			openerEvent := ""
			for id := range first.Created {
				openerEvent = id
			}
			r, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("m1", p.member, "https://b.example.net/2", "")})
			if err != nil {
				t.Fatal(err)
			}
			offered := len(afterChoices(questionChoices(t, judge, 1, "k1"))) > 0
			if offered != p.offered {
				t.Fatalf("after_ offered = %v, want %v", offered, p.offered)
			}
			if r.Joined != 0 || r.Tentative != 0 || r.Minted != 1 || len(r.Corroborated) != 0 {
				t.Fatalf("report = %+v; a development must be its own event, uncorroborated", r)
			}
			m, found, err := rt.store.EventMembershipOf(ctx, owner, computer, "m1")
			if err != nil || !found || m.EventID == openerEvent || m.Tentative || !m.Opened {
				t.Fatalf("membership = %+v found=%v err=%v; want the opener of its own event", m, found, err)
			}
			ev, _ := rt.eventSummary(ctx, owner, computer, m.EventID)
			wantFollows := ""
			if p.offered {
				wantFollows = openerEvent
			}
			if ev.Follows != wantFollows {
				t.Fatalf("follows = %q, want %q", ev.Follows, wantFollows)
			}
			// F4: the link lends nothing in either direction.
			old, _ := rt.eventSummary(ctx, owner, computer, openerEvent)
			if len(old.Outlets) != 1 || old.Corroborated || len(ev.Outlets) != 1 || ev.Corroborated {
				t.Fatalf("old = %+v, new = %+v; the link must not count as corroboration", old, ev)
			}
		})
	}
}

// F2: true duplicates still merge and corroborate: two outlets, one
// incident, including updated details and the reveal's official page; the
// after_ choice is offered and not taken.
func TestEventLayerTrueDuplicatesStillMergeAndCorroborate(t *testing.T) {
	pairs := []struct{ name, opener, member string }{
		{"eurotunnel", "Nineteen injured in Eurotunnel accident", "Bus and train collide at Channel Tunnel terminal, 19 hurt"},
		{"updated detail", "Bus crash in Oregon kills 3", "Oregon bus crash death toll rises to 5"},
		{"official page", "Tesla reveals SpaceX-powered Roadster", "Tesla Roadster"},
		{"houthi strike", "Houthis strike Saudi oil depot in Jizan", "Saudi Jizan oil depot hit by Houthi missile strike"},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			judge := &eventJudge{answers: map[string]eventAnswer{p.member: {target: p.opener, p: 0.97}}}
			rt, owner, computer := eventRuntime(t, judge)
			ctx := context.Background()
			if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("o1", p.opener, "https://www.bbc.co.uk/news/1", "")}); err != nil {
				t.Fatal(err)
			}
			r, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("m1", p.member, "https://www.dw.com/en/2", "")})
			if err != nil {
				t.Fatal(err)
			}
			if r.Joined != 1 || r.Minted != 0 || len(r.Corroborated) != 1 {
				t.Fatalf("report = %+v; a true duplicate must join and corroborate", r)
			}
			ev, _ := rt.eventSummary(ctx, owner, computer, r.Corroborated[0])
			if ev.Follows != "" || len(ev.Outlets) != 2 {
				t.Fatalf("event = %+v", ev)
			}
		})
	}
}

// F5: a weak link answer sets no link; a sibling component takes its
// strongest link; the link survives an index rebuild from the store.
func TestEventLayerLinkIsEvidenceNotGuessed(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{
		"Hormuz tanker attacked":             {target: "Ship attacks near Hormuz at a record high", after: true, p: 0.4},
		"Second ship attacked near Hormuz":   {target: "Ship attacks near Hormuz at a record high", after: true, p: 0.8},
		"Third Hormuz ship attack, insurers": {target: "Second ship attacked near Hormuz", p: 0.95},
	}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	first, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("o1", "Ship attacks near Hormuz at a record high", "https://a.example.com/1", "")})
	if err != nil {
		t.Fatal(err)
	}
	var opener string
	for id := range first.Created {
		opener = id
	}
	// Below eventTentativeP: a new event, no link.
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("w1", "Hormuz tanker attacked", "https://b.example.net/2", "")}); err != nil {
		t.Fatal(err)
	}
	weak, _, _ := rt.store.EventMembershipOf(ctx, owner, computer, "w1")
	if ev, _ := rt.eventSummary(ctx, owner, computer, weak.EventID); ev.Follows != "" || weak.EventID == opener {
		t.Fatalf("weak answer: event %+v; want a new event with no link", ev)
	}
	// A sibling pair: one component, one event, linked to the earlier event.
	r, err := rt.resolveEvents(ctx, owner, computer, []eventInput{
		capItem("s1", "Second ship attacked near Hormuz", "https://c.example.org/3", ""),
		capItem("s2", "Third Hormuz ship attack, insurers", "https://d.example.io/4", ""),
	})
	if err != nil || r.Minted != 1 {
		t.Fatalf("siblings: %+v %v", r, err)
	}
	m, _, _ := rt.store.EventMembershipOf(ctx, owner, computer, "s2")
	ev, _ := rt.eventSummary(ctx, owner, computer, m.EventID)
	if ev.Follows != opener || ev.Members != 2 {
		t.Fatalf("sibling event = %+v; want 2 members following %s", ev, opener)
	}
	// Restart: the index rebuilt from the store keeps the link.
	rt.resetEventIndexForTest()
	if again, _ := rt.eventSummary(ctx, owner, computer, m.EventID); again.Follows != opener {
		t.Fatalf("after rebuild follows = %q, want %q", again.Follows, opener)
	}
}

// F6: the link choices are bounded per capture and the request stays under
// the context bound.
func TestEventLayerLinkChoicesAreBounded(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	var seed []eventInput
	for i := 0; i < 30; i++ {
		seed = append(seed, capItem(fmt.Sprintf("e%d", i), fmt.Sprintf("Storm %d batters coast of region %d", i, i), fmt.Sprintf("https://seed%d.example.com/%d", i, i), ""))
	}
	if _, err := rt.resolveEvents(ctx, owner, computer, seed); err != nil {
		t.Fatal(err)
	}
	judge.mu.Lock()
	before := len(judge.calls)
	judge.mu.Unlock()
	var chunk []eventInput
	for i := 0; i < eventChunkSize; i++ {
		chunk = append(chunk, capItem(fmt.Sprintf("n%d", i), fmt.Sprintf("Storm aftermath %d coast recovery", i), fmt.Sprintf("https://new%d.example.net/%d", i, i), ""))
	}
	if _, err := rt.resolveEvents(ctx, owner, computer, chunk); err != nil {
		t.Fatal(err)
	}
	judge.mu.Lock()
	defer judge.mu.Unlock()
	sawLinks := false
	for i := before; i < len(judge.calls); i++ {
		if judge.bytes[i] > eventStateMaxBytes {
			t.Fatalf("call %d = %d bytes, over the bound %d", i, judge.bytes[i], eventStateMaxBytes)
		}
		for key, raw := range judge.asked[i] {
			var q struct {
				Choices []string `json:"choices"`
			}
			_ = json.Unmarshal(raw, &q)
			if n := len(afterChoices(q.Choices)); n > eventLinkChoices {
				t.Fatalf("call %d question %s offers %d after_ choices, want at most %d", i, key, n, eventLinkChoices)
			} else if n > 0 {
				sawLinks = true
			}
		}
	}
	if !sawLinks {
		t.Fatal("no after_ choice was offered for captures that share words with known events")
	}
}

// F7: the blind judge script gives its judges the resolver's definition,
// verbatim, so the sample measures what Jev was asked.
func TestBlindSampleScriptCarriesTheResolverDefinition(t *testing.T) {
	b, err := os.ReadFile("../../scripts/event-blind-sample.py")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), eventDefinition) {
		t.Fatalf("scripts/event-blind-sample.py does not contain eventDefinition verbatim:\n%s", eventDefinition)
	}
	if strings.Contains(string(b), "direct development") {
		t.Fatal("the blind script still treats a development as the same event")
	}
}
