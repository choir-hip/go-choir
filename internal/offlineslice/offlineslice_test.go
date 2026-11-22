package offlineslice

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// The harness's own rules, written before the code: arrival order, dedupe,
// no hindsight, earlier hours only.

func TestArrivalOrderIsByFetchThenID(t *testing.T) {
	caps := []Capture{
		{ID: "b", Fetched: ts("2026-11-15T01:00:00Z"), URLKey: "2", CanonKey: "2"},
		{ID: "a", Fetched: ts("2026-11-15T01:00:00Z"), URLKey: "1", CanonKey: "1"},
		{ID: "c", Fetched: ts("2026-11-14T23:00:00Z"), URLKey: "3", CanonKey: "3"},
	}
	kept, _ := Dedupe(caps)
	var ids []string
	for _, c := range kept {
		ids = append(ids, c.ID)
	}
	if !reflect.DeepEqual(ids, []string{"c", "a", "b"}) {
		t.Fatalf("order %v", ids)
	}
}

func TestDedupeKeepsEarliestByURLOrCanonicalKey(t *testing.T) {
	caps := []Capture{
		{ID: "late", Fetched: ts("2026-11-15T05:00:00Z"), URLKey: "u1", CanonKey: "c1"},
		{ID: "first", Fetched: ts("2026-11-15T01:00:00Z"), URLKey: "u1", CanonKey: "c1"},
		{ID: "samecanon", Fetched: ts("2026-11-15T02:00:00Z"), URLKey: "u2", CanonKey: "c1"},
		{ID: "sameurl", Fetched: ts("2026-11-15T03:00:00Z"), URLKey: "u1", CanonKey: "c9"},
		{ID: "other", Fetched: ts("2026-11-15T04:00:00Z"), URLKey: "u3", CanonKey: "c3"},
	}
	kept, dropped := Dedupe(caps)
	var ids []string
	for _, c := range kept {
		ids = append(ids, c.ID)
	}
	if !reflect.DeepEqual(ids, []string{"first", "other"}) || dropped != 3 {
		t.Fatalf("kept %v dropped %d", ids, dropped)
	}
}

func TestEntitiesAsOfNoHindsight(t *testing.T) {
	day := ts("2026-11-17T00:00:00Z")
	ents := []Entity{
		{EntityID: "old", Name: "Old", Aliases: []string{"O"}, Created: ts("2026-11-01T00:00:00Z"), Updated: ts("2026-11-01T00:00:00Z")},
		{EntityID: "oldmod", Name: "OldMod", Aliases: []string{"OM"}, Created: ts("2026-11-01T00:00:00Z"), Updated: ts("2026-11-18T00:00:00Z")},
		{EntityID: "same", Name: "Same", Created: ts("2026-11-17T00:00:00Z"), Updated: ts("2026-11-17T00:00:00Z")}, // created at the day start: not before it
		{EntityID: "new", Name: "New", Created: ts("2026-11-17T10:00:00Z"), Updated: ts("2026-11-17T10:00:00Z")},
		{EntityID: "dead", Name: "Dead", Created: ts("2026-11-01T00:00:00Z"), Updated: ts("2026-11-01T00:00:00Z"), Tombstone: true},
	}
	ids := func(es []Entity) []string {
		var o []string
		for _, e := range es {
			o = append(o, e.EntityID)
		}
		sort.Strings(o)
		return o
	}
	all := EntitiesAsOf(ents, day, AliasesAll)
	if !reflect.DeepEqual(ids(all), []string{"old", "oldmod"}) {
		t.Fatalf("entities %v", ids(all))
	}
	for _, e := range all {
		if e.EntityID == "oldmod" && len(e.Aliases) != 1 {
			t.Fatalf("AliasesAll drops aliases")
		}
	}
	asof := EntitiesAsOf(ents, day, AliasesAsOf)
	for _, e := range asof {
		if e.EntityID == "oldmod" && len(e.Aliases) != 0 {
			t.Fatalf("a row modified after the day must carry no aliases: %v", e.Aliases)
		}
		if e.EntityID == "old" && len(e.Aliases) != 1 {
			t.Fatalf("an unmodified row keeps its aliases")
		}
	}
	for _, e := range EntitiesAsOf(ents, day, AliasesNone) {
		if len(e.Aliases) != 0 {
			t.Fatalf("AliasesNone")
		}
	}
}

func TestStopsAsOf(t *testing.T) {
	day := ts("2026-11-17T00:00:00Z")
	z := time.Time{}
	stops := []Stop{
		{Key: "active-early", Active: true, Created: ts("2026-11-10T00:00:00Z")},
		{Key: "active-late", Active: true, Created: ts("2026-11-10T00:00:00Z"), Activated: ts("2026-11-18T00:00:00Z")},
		{Key: "active-created-late", Active: true, Created: ts("2026-11-19T00:00:00Z")},
		{Key: "revoked-after", Active: false, Created: ts("2026-11-10T00:00:00Z"), Activated: ts("2026-11-11T00:00:00Z"), Revoked: ts("2026-11-20T00:00:00Z")},
		{Key: "revoked-before", Active: false, Created: ts("2026-11-10T00:00:00Z"), Activated: ts("2026-11-11T00:00:00Z"), Revoked: ts("2026-11-12T00:00:00Z")},
		{Key: "never", Active: false, Created: ts("2026-11-10T00:00:00Z"), Activated: z, Revoked: z},
	}
	got := StopsAsOf(stops, day)
	sort.Strings(got)
	if !reflect.DeepEqual(got, []string{"active-early", "revoked-after"}) {
		t.Fatalf("stops %v", got)
	}
}

func TestNeighboursAreEarlierHoursOnly(t *testing.T) {
	mk := func(id, at string, v ...float32) Capture { return Capture{ID: id, Fetched: ts(at), Vec: normalize(v)} }
	caps := []Capture{
		mk("a", "2026-11-15T01:10:00Z", 1, 0),
		mk("b", "2026-11-15T01:50:00Z", 1, 0), // same hour as a: not a neighbour of a, nor a of b
		mk("c", "2026-11-15T02:05:00Z", 1, 0.1),
		mk("d", "2026-11-15T03:00:00Z", 0, 1),
		{ID: "novec", Fetched: ts("2026-11-15T03:30:00Z")},
	}
	nb := Neighbours(caps, 10)
	if len(nb[0]) != 0 || len(nb[1]) != 0 {
		t.Fatalf("first hour has neighbours: %v %v", nb[0], nb[1])
	}
	got := map[int]bool{}
	for _, n := range nb[2] {
		got[n.Index] = true
	}
	if !got[0] || !got[1] || len(nb[2]) != 2 {
		t.Fatalf("c neighbours %v", nb[2])
	}
	for i, ns := range nb {
		for _, n := range ns {
			if n.Index >= i || caps[n.Index].Fetched.Truncate(time.Hour).Equal(caps[i].Fetched.Truncate(time.Hour)) {
				t.Fatalf("capture %d has neighbour %d from its own or a later hour", i, n.Index)
			}
		}
	}
	if len(nb[4]) != 0 {
		t.Fatalf("a capture with no vector has neighbours")
	}
	if nb[3][0].Cos > 0.2 {
		t.Fatalf("orthogonal vectors score %v", nb[3][0].Cos)
	}
}

// fakeRules: a mention per word of the title; "Known" resolves to entity E1, which exists from day 17 on.
type fakeRules struct {
	day       time.Time
	prepared  []time.Time
	entsAtDay map[string]bool
	sawKnown  []map[string]bool
}

func (f *fakeRules) Prepare(day time.Time, ents []Entity, stops []string) {
	f.prepared = append(f.prepared, day)
	f.entsAtDay = map[string]bool{}
	for _, e := range ents {
		f.entsAtDay[e.EntityID] = true
	}
}

func (f *fakeRules) Mentions(title, lead string, known map[string]bool) []Mention {
	k := map[string]bool{}
	for id := range known {
		k[id] = true
	}
	f.sawKnown = append(f.sawKnown, k)
	var out []Mention
	if title == "Known" && f.entsAtDay["E1"] {
		out = append(out, Mention{Key: "known", Phrase: "Known", Hit: true, InTitle: true, Cands: []Cand{{EntityID: "E1", Name: "Known", Via: "exact"}}})
	} else {
		out = append(out, Mention{Key: "k:" + title, Phrase: title, InTitle: true})
	}
	return out
}

func TestReplayPreparesRulesPerDayAndUsesOnlyEarlierSubjects(t *testing.T) {
	mk := func(id, at, title string, v ...float32) Capture {
		return Capture{ID: id, Fetched: ts(at), Title: title, Vec: normalize(v)}
	}
	caps := []Capture{
		mk("a", "2026-11-16T22:00:00Z", "Known", 1, 0),
		mk("b", "2026-11-17T01:00:00Z", "Known", 1, 0.05),
		mk("c", "2026-11-17T05:00:00Z", "Other", 1, 0.1),
	}
	ents := []Entity{{EntityID: "E1", Name: "Known", Created: ts("2026-11-16T23:00:00Z"), Updated: ts("2026-11-16T23:00:00Z")}}
	fr := &fakeRules{}
	res := Replay(caps, ents, nil, fr, AliasesAll, 10)
	if len(fr.prepared) != 2 || !fr.prepared[0].Equal(ts("2026-11-16T00:00:00Z")) || !fr.prepared[1].Equal(ts("2026-11-17T00:00:00Z")) {
		t.Fatalf("prepared %v", fr.prepared)
	}
	// a: day 16, entity not yet created -> a proposal, not a known label
	if len(res[0].A) != 0 || len(res[0].B) != 1 {
		t.Fatalf("a: %+v", res[0])
	}
	// b: day 17 sees E1; its neighbour a (earlier hour) carries only a new name, so no entity in `known`
	if len(res[1].A) != 1 || res[1].A[0].EntityID != "E1" {
		t.Fatalf("b: %+v", res[1])
	}
	if len(fr.sawKnown[1]) != 0 {
		t.Fatalf("b known %v", fr.sawKnown[1])
	}
	// b's (c) candidates come from a: the new name "Known" (key k:Known), which b does not itself hold as a new name
	if len(res[1].C) != 1 || res[1].C[0].Key != "n:k:Known" {
		t.Fatalf("b C: %+v", res[1].C)
	}
	// c: neighbours a and b; b's entity subject E1 is `known`; c is about "Other"
	if !fr.sawKnown[2]["E1"] {
		t.Fatalf("c known %v", fr.sawKnown[2])
	}
	// c's subjects from neighbours: E1 (from b) and n:k:Known (from a)
	keys := map[string]bool{}
	for _, s := range res[2].C {
		keys[s.Key] = true
	}
	if !keys["e:E1"] || !keys["n:k:Known"] || len(keys) != 2 {
		t.Fatalf("c C: %+v", res[2].C)
	}
}

func TestSalient(t *testing.T) {
	if !Salient(Mention{InTitle: true}) || !Salient(Mention{Count: 2}) || Salient(Mention{Count: 1, InLead: true}) {
		t.Fatal("salient rule: title, or two body occurrences")
	}
}

func TestLeadOfCutsFirstPassage(t *testing.T) {
	text := "First paragraph about Norway.\n\nSecond paragraph."
	got := LeadOf([]byte(text))
	if got == "" || len([]rune(got)) > 2400 {
		t.Fatalf("lead %q", got)
	}
}
