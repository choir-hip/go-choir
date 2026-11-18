package searchquery

import (
	"sync"
	"testing"
)

// One normaliser for "is this the same question" inside one run (search-reuse;
// the desk's search store is its only user: the gateway cache keeps its own
// key, because a merge there is silent to the caller). Failure list, written
// before the code, from the review of f7df119fb:
//  1. A year or a date is dropped, so "harbor strike 2025" and "... 2026"
//     (or two dates) are one query and the second gets last year's results.
//  2. A quoted phrase equals the same words unquoted, though a search engine
//     answers an exact phrase differently.
//  3. "C++" folds to "C" and "C#" to "C".
//  4. "IT", "US", "WHO", "AT" written in capitals fold into the stop words
//     "it", "us", "who", "at" and vanish.
//  5. Order, case of ordinary words, extra whitespace and punctuation
//     between words stop folding (the safe merges are lost).
//  6. A real word, a site: operator, an OR or a -exclusion is dropped.
//  7. A query of only dropped words has an empty key.
func TestKeyTable(t *testing.T) {
	same := [][2]string{
		{"Harbor Strike Mauritius", "mauritius harbor strike"},
		{"HARBOR STRIKE", "harbor strike"},
		{"harbor, strike! (mauritius)", "harbor strike mauritius"},
		{"harbor  strike\tmauritius\n", "harbor strike mauritius"},
		{"the strike at the harbor", "harbor strike"},
		{"harbor strike site:Example.com", "site:example.com harbor strike"},
		{"harbor strike site:example.com.", "harbor strike site:example.com"},
		{"Café Münster", "münster café"},
		{"harbor-strike", "harbor strike"},
		{`"harbor strike" mauritius`, `mauritius "Harbor  Strike"`},
		{"harbor strike 2026", "2026 harbor strike"},
		{"c++ compiler", "compiler C++"},
	}
	for _, c := range same {
		if a, b := Key(c[0]), Key(c[1]); a != b || a == "" {
			t.Errorf("Key(%q) = %q, Key(%q) = %q, want equal", c[0], a, c[1], b)
		}
	}
	differ := [][2]string{
		{"harbor strike", "harbor strikes"},
		{"harbor strike 2025", "harbor strike 2026"},
		{"harbor strike 2026", "harbor strike"},
		{"harbor strike 2026-11-18", "harbor strike 2025-11-18"},
		{"harbor strike 2026-11-18", "harbor strike 2026-11-17"},
		{`"harbor strike" mauritius`, "harbor strike mauritius"},
		{`"harbor strike"`, `"strike harbor"`},
		{"C++", "C"},
		{"C#", "C"},
		{"IT strike", "strike"},
		{"IT strike", "it strike"},
		{"US strike", "strike"},
		{"WHO report", "report"},
		{"strike AT harbor", "strike harbor"},
		{"harbor strike", "harbor strike site:example.com"},
		{"harbor strike site:a.example", "harbor strike site:b.example"},
		{"harbor strike", "harbor strike results"},
		{"cats or dogs", "cats dogs"},
		{"cats -dogs", "cats dogs"},
		{"filetype:pdf harbor", "harbor"},
		{"the", "a"},
		{"x", "y"},
	}
	for _, c := range differ {
		if a, b := Key(c[0]), Key(c[1]); a == b {
			t.Errorf("Key(%q) = Key(%q) = %q, want different", c[0], c[1], a)
		}
	}
	if Key("the of") != "of the" {
		t.Errorf("a query of only dropped words keeps its words: %q", Key("the of"))
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

// IsTimeSensitive is the one function the desk's reuse window and the
// gateway cache's fresh TTL both ask.
func TestIsTimeSensitive(t *testing.T) {
	for q, want := range map[string]bool{
		"breaking harbor strike": true, "harbor strike today": true, "latest harbor news": true,
		"strike in March": true, "harbor strike history": false, "harbor strike": false,
	} {
		if got := IsTimeSensitive(q); got != want {
			t.Errorf("IsTimeSensitive(%q) = %v, want %v", q, got, want)
		}
	}
}

func TestKeyIsSafeForConcurrentUse(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = Key(`"Harbor Strike" C++ IT 2026-11-18 site:a.example`)
			}
		}()
	}
	wg.Wait()
}
