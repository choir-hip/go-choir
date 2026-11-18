package searchquery

import "testing"

// One normaliser for "is this the same question" (search-reuse commit 2,
// docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md): the desk's
// search store and the gateway's result cache both key on Key. Failure list,
// written before the code:
//  1. Word order, case, quotes or punctuation make two queries differ
//     (a reordered or re-quoted query missed the cache: 12% of searches).
//  2. A bare year, or a stop word, makes two queries differ.
//  3. A real word, a site: operator, an OR or a -exclusion is dropped, so
//     two different questions collide and the second is answered from the first.
//  4. A query made only of dropped words has an empty key and collides with
//     every other such query.
func TestKeyTable(t *testing.T) {
	same := [][2]string{
		{"Harbor Strike Mauritius", "mauritius harbor strike"},
		{`"harbor strike" mauritius`, "harbor strike mauritius"},
		{"harbor, strike! (mauritius)", "harbor strike mauritius"},
		{"harbor  strike\tmauritius\n", "harbor strike mauritius"},
		{"harbor strike mauritius 2026", "mauritius harbor strike"},
		{"2026 harbor strike", "harbor strike"},
		{"the strike at the harbor", "harbor strike"},
		{"harbor strike site:Example.com", "site:example.com harbor strike"},
		{"harbor strike site:example.com.", "harbor strike site:example.com"},
		{"Café Münster", "münster café"},
		{"harbor-strike", "harbor strike"},
	}
	for _, c := range same {
		if a, b := Key(c[0]), Key(c[1]); a != b || a == "" {
			t.Errorf("Key(%q) = %q, Key(%q) = %q, want equal", c[0], a, c[1], b)
		}
	}
	differ := [][2]string{
		{"harbor strike", "harbor strikes"},
		{"harbor strike", "harbor strike site:example.com"},
		{"harbor strike site:a.example", "harbor strike site:b.example"},
		{"harbor strike", "harbor strike 2026 results"},
		{"cats or dogs", "cats dogs"},
		{"cats -dogs", "cats dogs"},
		{"filetype:pdf harbor", "harbor"},
		{"2026", "2027"},
		{"the", "a"},
		{"x", "y"},
	}
	for _, c := range differ {
		if a, b := Key(c[0]), Key(c[1]); a == b {
			t.Errorf("Key(%q) = Key(%q) = %q, want different", c[0], c[1], a)
		}
	}
	for _, q := range []string{"2026", "the of", "   ", ""} {
		_ = Key(q) // never panics
	}
	if Key("2026") != "2026" || Key("the of") != "of the" {
		t.Errorf("a query of only dropped words keeps its words: %q, %q", Key("2026"), Key("the of"))
	}
	if Key("") != "" || Key(" \t") != "" {
		t.Errorf("an empty query has an empty key")
	}
}

func TestHasRealWord(t *testing.T) {
	for q, want := range map[string]bool{"x": false, "x y z": false, " - ! ": false, "": false, "ab": true, "x ai": true, "5G": true, "q 1": false, "日本": true} {
		if got := HasRealWord(q); got != want {
			t.Errorf("HasRealWord(%q) = %v, want %v", q, got, want)
		}
	}
}
