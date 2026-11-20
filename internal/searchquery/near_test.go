package searchquery

import (
	"testing"
	"time"
)

// NearKey: "the same question" for a hold on re-asks. Failure list, written
// before the code (docs/definitions/corpus-first-search-one-verb-2026-11-19.md,
// Landing 1b; review of the re-ask holds, blocking finding 1):
//  1. A wording that differs only by word order, case, stop words, punctuation,
//     a weekday, "today"/"tonight", or today's or yesterday's date (a recency
//     decoration) gets a different key, so the web is asked again.
//  2. A search restricted to a different site, or to one against none, shares a
//     key with another: that is how a run hunts its second outlet, and a story
//     publishes only on two distinct outlets.
//  3. An exact phrase and its loose form share a key (an exact-phrase search
//     verifies a wording; answering it from loose results lets the run cite a
//     phrase the search never showed).
//  4. Any other explicit date, month or year is folded, so "earnings 2026-10-01"
//     and "earnings 2026-11-01" are one question.
//  5. Two different questions get one key: a different named entity, number,
//     negation or exclusion, a bare year, or a month name used as a word.
//  6. A query that is only recency words gets an empty-key collision with
//     every other one (it must have no near key at all).
var testNow = time.Date(2026, 11, 20, 14, 0, 0, 0, time.UTC)

func TestNearKeySameQuestion(t *testing.T) {
	same := [][2]string{
		{"Kharkiv missile attack", "missile attack Kharkiv"},
		{"Kharkiv missile attack", "the Kharkiv missile attack, today"},
		{"Kharkiv missile attack Friday", "kharkiv MISSILE attack tonight"}, // today is Friday 2026-11-20
		{"Kharkiv missile attack Thursday", "Kharkiv missile attack"},       // yesterday
		// today's and yesterday's date: a recency decoration
		{"Kharkiv missile attack 20 November 2026", "Kharkiv missile attack"},
		{"Kharkiv missile attack November 20", "Kharkiv missile attack"},
		{"Kharkiv missile attack 19 Nov", "Kharkiv missile attack"},
		{"Kharkiv missile attack 2026-11-20", "Kharkiv missile attack"},
		{"Kharkiv missile attack 1st of December", "Kharkiv missile attack 1st of December"},
		{"site:Reuters.com Kharkiv missile attack", "Kharkiv missile attack site:reuters.com"},
		{"site:https://www.reuters.com/world Kharkiv attack", "site:reuters.com Kharkiv attack"},
		{`"Kharkiv missile" attack`, `attack "kharkiv  MISSILE"`},
		{"pedestrian Nellis Stewart Las Vegas struck", "Las Vegas pedestrian struck Nellis Stewart, tonight"},
	}
	for _, c := range same {
		a, b := NearKeyAt(c[0], testNow), NearKeyAt(c[1], testNow)
		if a == "" || a != b {
			t.Errorf("NearKeyAt(%q)=%q, NearKeyAt(%q)=%q: want the same non-empty key", c[0], a, c[1], b)
		}
	}
}

func TestNearKeyKeepsDifferentQuestionsApart(t *testing.T) {
	differ := [][2]string{
		// a different site restriction is a different question
		{"site:reuters.com Kharkiv attack", "site:apnews.com Kharkiv attack"},
		{"site:reuters.com Kharkiv attack", "Kharkiv attack"},
		{"-site:tass.com Kharkiv attack", "Kharkiv attack"},
		// an exact phrase is not its loose form
		{`"Kharkiv missile" attack`, "Kharkiv missile attack"},
		// a weekday that is neither today nor yesterday names another day
		{"Kabul blast Friday", "Kabul blast Saturday"},
		{"Kabul blast Wednesday", "Kabul blast Tuesday"},
		{"Kabul blast Wednesday", "Kabul blast"},
		// proper nouns that contain a weekday or "today" are not recency
		{"Black Friday sales", "sales"},
		{"USA Today poll", "USA poll"},
		{"Friday Night Lights cast", "night lights cast"},
		// any other explicit date, month or year is part of the question
		{"earnings 2026-10-01", "earnings 2026-11-01"},
		{"earnings 2026-10-01", "earnings"},
		{"Kharkiv attack 3 November 2026", "Kharkiv attack"},
		{"Kharkiv attack November 2026", "Kharkiv attack"},
		{"Kharkiv attack 20 November 2025", "Kharkiv attack"},
		{"Kharkiv attack 2026", "Kharkiv attack"},
		{"2025 budget vote", "2026 budget vote"},
		// different entities, numbers, negations
		{"Kharkiv missile attack", "Kyiv missile attack"},
		{"7 killed in Kharkiv attack", "9 killed in Kharkiv attack"},
		{"Kharkiv attack", "Kharkiv attack not confirmed"},
		{"Kharkiv attack", "Kharkiv attack -casualties"},
		{"protest march Paris", "protest Paris"},
		{"Theresa May resigns", "Theresa resigns"},
		{"harbor strike mauritius", "harbor strike madagascar"},
		{"IT outage", "outage"},
		{"C++ compiler", "C compiler"},
		{"Boeing 737 crash", "Boeing 747 crash"},
		{"attack Kharkiv", "attack Kharkiv OR Odesa"},
	}
	for _, c := range differ {
		a, b := NearKeyAt(c[0], testNow), NearKeyAt(c[1], testNow)
		if a == b {
			t.Errorf("NearKeyAt(%q) == NearKeyAt(%q) == %q: two different questions must not merge", c[0], c[1], a)
		}
	}
}

func TestNearKeyOfOnlyRecencyWordsIsEmpty(t *testing.T) {
	for _, q := range []string{"20 November 2026", "today", "Friday tonight", `"" `, "the of", "2026-11-19 today"} {
		if k := NearKeyAt(q, testNow); k != "" {
			t.Errorf("NearKeyAt(%q) = %q, want empty: nothing is left to compare", q, k)
		}
	}
}

// What the key does and does not tell apart, stated so the next reader need not
// guess: a lower-case weekday that is today's or yesterday's folds; a
// capitalised one next to another capitalised word is a proper noun and stays;
// a capitalised "Today" or weekday followed by a lower-case word still folds
// ("Today programme" asks for "programme": a known limit, answered by fresh).
func TestNearKeyProperNounsAndTheLimit(t *testing.T) {
	if a, b := NearKeyAt("Black Friday sales", testNow), NearKeyAt("sales", testNow); a == b {
		t.Errorf("Black Friday must not fold to %q", b)
	}
	if a, b := NearKeyAt("Today programme", testNow), NearKeyAt("programme", testNow); a != b {
		t.Errorf("known limit: \"Today programme\" folds to %q, want %q", a, b)
	}
}

func TestSiteRestrictionsAreNormalisedHosts(t *testing.T) {
	got := SiteRestrictions("site:https://www.Reuters.com/world Kharkiv -site:tass.com site:apnews.com. attack")
	want := []string{"reuters.com", "apnews.com"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("SiteRestrictions = %v, want %v (the excluded site is not a restriction)", got, want)
	}
}
