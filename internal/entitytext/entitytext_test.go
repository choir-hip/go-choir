package entitytext

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes of slice c0's deterministic extraction, tested first
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, "Slice c0",
// §1 and §8 G1 to G5). Story text in these tests is invented.

func proposalKeys(ps []Proposal) map[string]Proposal {
	out := map[string]Proposal{}
	for _, p := range ps {
		out[p.Key] = p
	}
	return out
}

func hitByID(hs []Hit) map[string]Hit {
	out := map[string]Hit{}
	for _, h := range hs {
		out[h.EntityID] = h
	}
	return out
}

// Identity does not move: matching folds diacritics, identity does not, so
// the ids slice a minted stay the ids.
func TestIdentityIsUnchangedByTheMatchKey(t *testing.T) {
	pins := map[string]string{
		"Anthropic":           "ent:44f3664f1c36df4726a462d4",
		"Aleksandar Vučić":    "ent:e79324625481145d11bfa7c3",
		"Aleksandar Vucic":    "ent:73775e3fff753f4342f268a7",
		"The New York Times":  "ent:604ded6a4226a4d0935cc3e6",
		"Volodymyr Zelenskyy": "ent:00ffc564f088eb2d03abb575",
	}
	for name, want := range pins {
		if got := types.MintedEntityID(name); got != want {
			t.Errorf("MintedEntityID(%q) = %s, want %s (identity moved)", name, got, want)
		}
	}
	if Key("Aleksandar Vučić") != Key("Aleksandar Vucic") {
		t.Errorf("the match key does not fold diacritics: %q vs %q", Key("Aleksandar Vučić"), Key("Aleksandar Vucic"))
	}
}

// G3: possessives, diacritics, case, compatibility forms and company forms
// fold to one key; distinct spellings stay distinct (no fuzzy matching).
func TestKeyFoldsSurfaceForms(t *testing.T) {
	same := [][]string{
		{"Vučić", "Vucic", "VUČIĆ's", "vučić’s"},
		{"Anthropic", "anthropic", "Anthropic's", "Anthropic, Inc.", "ＡＮＴＨＲＯＰＩＣ"},
		{"Łukasz Kowalski", "Lukasz Kowalski"},
		{"U.S. Department of Justice", "US Department of Justice"},
		{"Rolls-Royce", "Rolls Royce"},
		{"O'Brien", "OBrien"},
	}
	for _, group := range same {
		want := Key(group[0])
		if want == "" {
			t.Fatalf("Key(%q) is empty", group[0])
		}
		for _, s := range group[1:] {
			if got := Key(s); got != want {
				t.Errorf("Key(%q) = %q, want %q", s, got, want)
			}
		}
	}
	if Key("Zelenskyy") == Key("Zelensky") {
		t.Errorf("two spellings share a key")
	}
	for _, empty := range []string{"", "The", "Inc.", "'s"} {
		if got := Key(empty); got != "" {
			t.Errorf("Key(%q) = %q, want empty", empty, got)
		}
	}
}

func TestSentencesAndTokens(t *testing.T) {
	d := Parse("", "Mr. Smith met Dr. Jones in the U.S. on Jan. 5 with John F. Kennedy Jr. present. Then they left!\nA new line starts here. He said: \"Peace is near,\" and left.")
	if got := d.Sentences(); got != 4 {
		t.Fatalf("Sentences() = %d, want 4", got)
	}
	starts := map[string]bool{}
	for _, tok := range d.Body {
		if tok.SentenceStart {
			starts[tok.Surface] = true
		}
	}
	for _, want := range []string{"Mr", "Then", "A", "Peace"} {
		if !starts[want] {
			t.Errorf("%q is not marked as a sentence start (starts: %v)", want, starts)
		}
	}
	for _, not := range []string{"Smith", "Jones", "Kennedy", "present"} {
		if starts[not] {
			t.Errorf("%q is marked as a sentence start", not)
		}
	}
	var us *Token
	for i := range d.Body {
		if d.Body[i].Key == "us" {
			us = &d.Body[i]
		}
	}
	if us == nil || !us.Acronym || us.Surface != "U.S." {
		t.Fatalf("U.S. is not one acronym token: %+v", us)
	}
}

// G1: a sentence-start capital alone, a common word, a title word, a day
// or month is never proposed; a word that recurs capitalised mid-sentence is.
func TestProposeIgnoresSentenceStartCapitalsAndCommonWords(t *testing.T) {
	d := Parse("", "Yesterday the vote passed. Senate leaders met on Monday in January. The session ended. Officials said nothing. President Joe Biden spoke. Later, Prime Minister Keir Starmer replied.")
	got := proposalKeys(Propose(d, nil))
	for _, bad := range []string{"yesterday", "senate", "monday", "january", "the", "officials", "president", "prime minister", "president joe biden", "prime minister keir starmer"} {
		if _, ok := got[bad]; ok {
			t.Errorf("proposed %q", bad)
		}
	}
	for _, want := range []string{"joe biden", "keir starmer"} {
		if _, ok := got[want]; !ok {
			t.Errorf("did not propose %q (got %v)", want, keysOf(got))
		}
	}

	d = Parse("", "Senate leaders met. Later the Senate voted, and the Senate adjourned.")
	got = proposalKeys(Propose(d, nil))
	if p, ok := got["senate"]; !ok || p.Count != 3 {
		t.Fatalf("senate recurring mid-sentence: got %+v, want a proposal counted 3", p)
	}

	d = Parse("", `He said: "Peace is near," and left.`)
	if got := proposalKeys(Propose(d, nil)); len(got) != 0 {
		t.Fatalf("a quoted sentence's first word was proposed: %v", keysOf(got))
	}
}

func TestProposeAcronyms(t *testing.T) {
	d := Parse("", "Leaders of NATO and the EU met. U.S. officials attended. The meeting was OK for all.")
	got := proposalKeys(Propose(d, nil))
	for _, want := range []string{"nato", "eu", "us"} {
		p, ok := got[want]
		if !ok || !p.Acronym {
			t.Errorf("acronym %q: got %+v", want, p)
		}
	}
	for _, bad := range []string{"ok", "leaders", "leaders of nato"} {
		if _, ok := got[bad]; ok {
			t.Errorf("proposed %q", bad)
		}
	}
}

// G2: a surname or short form merges into the one longest form it is a
// token suffix or prefix of, its occurrences added and its surface noted as
// an alias; a surname shared by two full names stays unmerged.
func TestProposeMergesShortFormsIntoTheLongestName(t *testing.T) {
	d := Parse("", "Volodymyr Zelenskyy spoke in Kyiv. Later, Zelenskyy said the talks failed. Zelenskyy's aides agreed. Analysts at Goldman expect more. Goldman Sachs reported.")
	got := proposalKeys(Propose(d, nil))
	z, ok := got["volodymyr zelenskyy"]
	if !ok || z.Count != 3 {
		t.Fatalf("Volodymyr Zelenskyy: got %+v, want count 3", z)
	}
	if !contains(z.Aliases, "Zelenskyy") {
		t.Errorf("Zelenskyy is not an alias candidate: %v", z.Aliases)
	}
	if _, ok := got["zelenskyy"]; ok {
		t.Errorf("the surname stayed a separate proposal")
	}
	if g, ok := got["goldman sachs"]; !ok || g.Count != 2 || !contains(g.Aliases, "Goldman") {
		t.Errorf("Goldman Sachs: got %+v, want count 2 with alias Goldman", g)
	}
	if _, ok := got["kyiv"]; !ok {
		t.Errorf("Kyiv not proposed")
	}

	d = Parse("", "John Smith and Jane Smith disagreed. Later, Smith resigned.")
	got = proposalKeys(Propose(d, nil))
	if s, ok := got["smith"]; !ok || s.Count != 1 {
		t.Errorf("a surname shared by two names must stay unmerged: got %+v", s)
	}
	for _, k := range []string{"john smith", "jane smith"} {
		if p, ok := got[k]; !ok || p.Count != 1 || len(p.Aliases) != 0 {
			t.Errorf("%s: got %+v", k, p)
		}
	}
}

// G3: possessives, case and diacritics count as one form.
func TestProposeCountsSurfaceFormsAsOne(t *testing.T) {
	d := Parse("", "The startup Anthropic released a model. Critics of anthropic were quiet. Anthropic's rivals reacted. Aleksandar Vučić met reporters. Later, Vucic left.")
	got := proposalKeys(Propose(d, nil))
	if a, ok := got["anthropic"]; !ok || a.Count != 3 {
		t.Errorf("anthropic: got %+v, want count 3", a)
	}
	if v, ok := got["aleksandar vucic"]; !ok || v.Count != 2 {
		t.Errorf("Aleksandar Vučić: got %+v, want count 2", v)
	}
	if _, ok := got["vucic"]; ok {
		t.Errorf("Vucic stayed a separate proposal")
	}
	d = Parse("", "an apple a day keeps the doctor away")
	if got := Propose(d, nil); len(got) != 0 {
		t.Errorf("a lower-case-only word was proposed: %+v", got)
	}
}

// Rarity in the computer's library ranks a common name below a rare one;
// a title occurrence ranks first.
func TestProposeRanksByRarityAndTitle(t *testing.T) {
	idf := func(term string) float64 {
		if term == "senate" {
			return 0.2
		}
		return 3
	}
	d := Parse("", "Later the Senate met. Later Zelenskyy met. Then the Senate voted and Zelenskyy left.")
	ps := Propose(d, idf)
	if len(ps) < 2 || ps[0].Key != "zelenskyy" {
		t.Fatalf("rare name does not rank first: %+v", ps)
	}
	d = Parse("Senate passes the bill", "Later the Senate met. Later Zelenskyy met. Then the Senate voted and Zelenskyy left.")
	ps = Propose(d, idf)
	if len(ps) < 2 || ps[0].Key != "senate" || !ps[0].InTitle {
		t.Fatalf("title occurrence does not rank first: %+v", ps)
	}
}

// G4: text past MaxTextRunes is not read, and a full-size text is cheap.
func TestProposeIsBounded(t *testing.T) {
	filler := strings.Repeat("the market moved again today. ", MaxTextRunes/30+10)
	text := "Later, Halvorsen spoke. " + filler + " Later, Quentaro spoke."
	d := Parse("", text)
	got := proposalKeys(Propose(d, nil))
	if _, ok := got["halvorsen"]; !ok {
		t.Errorf("a name before the bound was not proposed")
	}
	if _, ok := got["quentaro"]; ok {
		t.Errorf("a name past the bound was read")
	}
	if n := len(d.Body); n > MaxTextRunes/2 {
		t.Errorf("%d tokens from a bounded text", n)
	}

	var b strings.Builder
	for i := 0; b.Len() < MaxTextRunes; i++ {
		fmt.Fprintf(&b, "Later, Name%c%c Person%d met Agency%d of Place%d. ", 'A'+i%26, 'a'+i%26, i, i, i%50)
	}
	start := time.Now()
	ps := Propose(Parse(strings.Repeat("Title ", 100), b.String()), nil)
	if el := time.Since(start); el > 500*time.Millisecond {
		t.Errorf("a full-size text took %s", el)
	}
	if len(ps) > MaxProposals {
		t.Errorf("%d proposals, at most %d", len(ps), MaxProposals)
	}
}

// The gazetteer: longest match first, folded keys, an entity's aliases.
func TestGazetteerMatchesLongestFirstOverNamesAndAliases(t *testing.T) {
	g := NewGazetteer()
	ny, nyt, vu := types.MintedEntityID("New York"), types.MintedEntityID("New York Times"), types.MintedEntityID("Aleksandar Vučić")
	for _, e := range [][2]string{{ny, "New York"}, {nyt, "The New York Times"}, {vu, "Aleksandar Vučić"}, {vu, "Vučić"}} {
		if !g.Add(e[0], e[1]) {
			t.Fatalf("Add(%s, %q) refused", e[0], e[1])
		}
	}
	d := Parse("Vucic visits New York", "The New York Times reported from New York. Aleksandar Vucic spoke. Later, Vučić's aides left.")
	got := hitByID(g.Match(d, nil))
	if h := got[nyt]; h.Count != 1 {
		t.Errorf("New York Times: %+v", h)
	}
	if h := got[ny]; h.Count != 1 || !h.InTitle {
		t.Errorf("New York (title and one body mention): %+v", h)
	}
	if h := got[vu]; h.Count != 2 || !h.InTitle {
		t.Errorf("Vučić by name and alias: %+v", h)
	}
	if e, k := g.Size(); e != 3 || k != 4 {
		t.Errorf("Size() = %d entities, %d keys", e, k)
	}
}

// G1 at the gazetteer: short one-word names match only as acronyms, a
// common or title word never matches alone, and an entity needs one
// capitalised occurrence before lower-case ones count.
func TestGazetteerShortAndCommonNamesNeverMatchAlone(t *testing.T) {
	g := NewGazetteer()
	fed, fbi, apple := types.MintedEntityID("Fed"), types.MintedEntityID("FBI"), types.MintedEntityID("Apple")
	g.Add(fed, "Fed")
	g.Add(fbi, "FBI")
	g.Add(apple, "Apple")
	if g.Add(types.MintedEntityID("Monday"), "Monday") {
		t.Errorf("a day name was accepted as a match key")
	}
	if got := hitByID(g.Match(Parse("", "The Fed raised rates. The fbi was quiet. Then the FBI spoke."), nil)); got[fed].Count != 0 || got[fbi].Count != 1 {
		t.Errorf("short names: %+v", got)
	}
	if got := g.Match(Parse("", "an apple a day keeps the doctor away"), nil); len(got) != 0 {
		t.Errorf("lower-case only matched: %+v", got)
	}
	if got := hitByID(g.Match(Parse("", "Apple said sales rose. Critics of apple disagreed."), nil)); got[apple].Count != 2 {
		t.Errorf("Apple with one capitalised occurrence: %+v", got[apple])
	}
}

// G6 at the trie: a mint is a hit on the next match, a removal is gone, and
// reads and writes interleave safely.
func TestGazetteerUpdatesInPlace(t *testing.T) {
	g := NewGazetteer()
	d := Parse("", "Later, Halvorsen Dynamics shipped.")
	if len(g.Match(d, nil)) != 0 {
		t.Fatal("empty gazetteer matched")
	}
	id := types.MintedEntityID("Halvorsen Dynamics")
	g.Add(id, "Halvorsen Dynamics")
	if got := hitByID(g.Match(d, nil)); got[id].Count != 1 {
		t.Fatalf("a mint is not a hit on the next match")
	}
	g.Remove(id, "Halvorsen Dynamics")
	if len(g.Match(d, nil)) != 0 {
		t.Fatal("a removed name still matches")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			g.Add(types.MintedEntityID(fmt.Sprintf("Entity Number%d", i)), fmt.Sprintf("Entity Number%d", i))
		}(i)
		go func() { defer wg.Done(); g.Match(d, nil) }()
	}
	wg.Wait()
	if e, _ := g.Size(); e != 8 {
		t.Errorf("Size() entities = %d, want 8", e)
	}
	g.RemoveEntity(types.MintedEntityID("Entity Number0"))
	if e, _ := g.Size(); e != 7 {
		t.Errorf("after RemoveEntity, entities = %d, want 7", e)
	}
}

// A key two entities share is marked ambiguous, never silently given to one.
func TestGazetteerMarksASharedKeyAmbiguous(t *testing.T) {
	g := NewGazetteer()
	a, b := types.MintedEntityID("Georgia"), types.WikidataEntityID("Q1428")
	g.Add(a, "Georgia")
	g.Add(b, "Georgia")
	got := hitByID(g.Match(Parse("", "Later, Georgia voted."), nil))
	if !got[a].Ambiguous || !got[b].Ambiguous {
		t.Fatalf("shared key not ambiguous: %+v", got)
	}
}

// G5: hubs keep at most hubSlots, are not dropped, and title candidates
// rank first.
func TestRankBoundsHubsAndPutsTitleFirst(t *testing.T) {
	var cs []Candidate
	for i := 0; i < 10; i++ {
		cs = append(cs, Candidate{Key: fmt.Sprintf("hub%d", i), Hub: true, Score: 100 - float64(i)})
	}
	for i := 0; i < 30; i++ {
		cs = append(cs, Candidate{Key: fmt.Sprintf("n%02d", i), Score: 50 - float64(i)})
	}
	cs = append(cs, Candidate{Key: "titled", InTitle: true, Score: 1})
	out := Rank(cs, 24, 4)
	if len(out) != 24 || out[0].Key != "titled" {
		t.Fatalf("Rank: %d out, first %+v", len(out), out[0])
	}
	hubs := 0
	for _, c := range out {
		if c.Hub {
			hubs++
		}
	}
	if hubs != 4 || out[1].Key != "hub0" {
		t.Errorf("hubs kept = %d (want 4, best first), second = %s", hubs, out[1].Key)
	}
	if out := Rank(cs[8:12], 24, 4); len(out) != 4 {
		t.Errorf("few candidates: %d out, want all 4", len(out))
	}
}

func keysOf(m map[string]Proposal) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestGazetteerLookupAndNear(t *testing.T) {
	g := NewGazetteer()
	vz, js, ja := types.MintedEntityID("Volodymyr Zelenskyy"), types.MintedEntityID("John Smith"), types.MintedEntityID("Jane Smith")
	g.Add(vz, "Volodymyr Zelenskyy")
	g.Add(vz, "Zelensky")
	g.Add(js, "John Smith")
	g.Add(ja, "Jane Smith")
	if got := g.Lookup(Key("Zelensky")); len(got) != 1 || got[0] != vz {
		t.Errorf("Lookup(alias) = %v", got)
	}
	if got := g.Lookup("volodymyr"); len(got) != 0 {
		t.Errorf("Lookup of a key prefix = %v", got)
	}
	if got := g.Near("zelenskyy"); len(got) != 1 || got[0] != vz {
		t.Errorf("Near(surname) = %v", got)
	}
	if got := g.Near("smith"); len(got) != 2 {
		t.Errorf("Near(shared surname) = %v, want both", got)
	}
	if got := g.Near("john smith"); len(got) != 0 {
		t.Errorf("Near of a whole key = %v, want none", got)
	}
	g.Remove(js, "John Smith")
	if got := g.Near("smith"); len(got) != 1 || got[0] != ja {
		t.Errorf("Near after a removal = %v", got)
	}
}
