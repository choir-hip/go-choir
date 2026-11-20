package entitytext

// The lookups the entity layer's ready list uses beyond the exact key and the
// near form (docs/design/entity-layer-2026-11-20.md section 4.2). Failure
// list, written before the tests:
//
//  L1 "Hegseth" does not offer "Pete Brian Hegseth" (a middle word), or
//     offers a row that lacks one of the mention's words;
//  L2 "Donald Trump" does not offer the entity named "Trump", or offers the
//     entity named "Donald Trump" itself, or a closed one-word entity;
//  L3 "UAE" does not offer "United Arab Emirates", or "United States" does
//     not offer "U.S.", or a one-letter initialism links anything;
//  L4 a removed alias still answers a lookup;
//  L5 a word shared by several names does not count them.

import (
	"reflect"
	"testing"
)

func TestSubsetOffersNamesHoldingEveryWord(t *testing.T) {
	g := NewGazetteer()
	g.Add("e1", "Pete Brian Hegseth")
	g.Add("e2", "Pete Hegseth")
	g.Add("e3", "Pete Smith")
	g.Add("e4", "Jensen Huang")
	if got, want := g.Subset(Key("Hegseth")), []string{"e1", "e2"}; !reflect.DeepEqual(got, want) { // L1
		t.Fatalf("Subset(Hegseth) = %v, want %v", got, want)
	}
	if got, want := g.Subset(Key("Pete Hegseth")), []string{"e1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Subset(Pete Hegseth) = %v, want %v (the row of the same words is Lookup's)", got, want)
	}
	if got := g.Subset(Key("Huang Pete")); got != nil {
		t.Fatalf("a row lacking a word was offered: %v", got)
	}
	if got := g.Subset("the"); got != nil {
		t.Fatalf("a closed word offered %v", got)
	}
}

func TestWithinOffersTheShorterNamesOfAMention(t *testing.T) {
	g := NewGazetteer()
	g.Add("trump", "Trump")
	g.Add("donald", "Donald Trump")
	g.Add("us", "U.S.")
	g.Add("of", "Of")
	if got, want := g.Within(Key("Donald Trump")), []string{"trump"}; !reflect.DeepEqual(got, want) { // L2
		t.Fatalf("Within(Donald Trump) = %v, want %v", got, want)
	}
	if got := g.Within(Key("Donald Trump Jr")); !reflect.DeepEqual(got, []string{"donald", "trump"}) {
		t.Fatalf("Within(Donald J Trump) = %v, want both shorter names", got)
	}
	if got := g.Within(Key("Trump")); got != nil {
		t.Fatalf("a one-word mention is within nothing: %v", got)
	}
	if got := g.Within(Key("Bank of America")); got != nil {
		t.Fatalf("a closed one-word entity was offered: %v", got)
	}
}

func TestInitialsLinkAnInitialismAndItsName(t *testing.T) {
	g := NewGazetteer()
	g.Add("uae", "United Arab Emirates")
	g.Add("us", "U.S.")
	g.Add("usa", "United States")
	g.Add("bm", "Bank of Montreal")
	g.Add("ab", "Asia Bank")
	if got, want := g.Initials(Key("UAE")), []string{"uae"}; !reflect.DeepEqual(got, want) { // L3
		t.Fatalf("Initials(UAE) = %v, want %v", got, want)
	}
	if got, want := g.Initials(Key("U.S.")), []string{"usa"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Initials(U.S.) = %v, want %v", got, want)
	}
	if got, want := g.Initials(Key("United States")), []string{"us"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Initials(United States) = %v, want %v", got, want)
	}
	if got, want := g.Initials(Key("BM")), []string{"bm"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Initials(BM) = %v, want %v (connectors are left out of the initials)", got, want)
	}
	if got := g.Initials("a"); got != nil {
		t.Fatalf("one letter linked %v", got)
	}
}

func TestRemovedNamesLeaveTheNewIndexes(t *testing.T) { // L4
	g := NewGazetteer()
	g.Add("uae", "United Arab Emirates")
	g.Remove("uae", "United Arab Emirates")
	if got := g.Initials("uae"); got != nil {
		t.Fatalf("a removed name still answers Initials: %v", got)
	}
	if got := g.Subset("emirates"); got != nil {
		t.Fatalf("a removed name still answers Subset: %v", got)
	}
	if got := g.KeysWith("arab"); len(got) != 0 {
		t.Fatalf("a removed name still holds a word: %v", got)
	}
	g.Add("a", "Alpha Beta")
	g.Add("a", "Beta Gamma")
	g.RemoveEntity("a")
	if len(g.toks) != 0 || len(g.inits) != 0 || len(g.ends) != 0 {
		t.Fatalf("indexes not empty after RemoveEntity: %d %d %d", len(g.toks), len(g.inits), len(g.ends))
	}
}

func TestKeysWithCountsTheNamesBuiltOnAWord(t *testing.T) { // L5
	g := NewGazetteer()
	g.Add("a", "George Washington")
	g.Add("b", "Washington Post")
	g.Add("c", "Washington")
	if got, want := g.KeysWith("washington"), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("KeysWith = %v, want %v (two or more words only)", got, want)
	}
}
