package normtext

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// Failure modes pinned here (docs/definitions/choir-reeng-cut2-evidence-to-
// publication-2026-10-12.md, unit 2a, "Normalization differs between cid and
// matching"):
//   - a whitespace-only difference (line break, NBSP, doubled space,
//     zero-width character) changes a block's cid, so a re-capture with the
//     same words re-addresses its citations;
//   - a case difference does NOT change the cid, so two different texts
//     share an address;
//   - curly quotes, dashes, ligatures or full-width forms make the same words
//     normalize differently;
//   - the tolerant levels disagree with Normalize about whitespace or
//     invisible characters, so the matcher and the cid see different text;
//   - the offset map points at the wrong source bytes when one source rune
//     becomes several (ligature, ellipsis) or several become one (a combining
//     accent), or when a multibyte character or NBSP precedes the match;
//   - duplicate blocks get cids that depend on anything but their order, or
//     the k-th duplicate is not numbered k.

func TestNormalizeCollapsesWhitespaceAndKeepsCase(t *testing.T) {
	cases := map[string]string{
		"  Hello   World  ":             "Hello World",
		"line one\nline two":            "line one line two",
		"tab\there\r\nand\u00a0nbsp":    "tab here and nbsp",
		"a \u200b b":                    "a b",
		"co\u200bop\u00adera\ufefftion": "cooperation",
		"\u2003em\u2009thin\u3000ideo":  "em thin ideo",
		"":                              "",
		" \n\t\u00a0 ":                  "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeFoldsQuotesDashesAndCompatibilityForms(t *testing.T) {
	cases := map[string]string{
		"\u201cIt\u2019s\u201d":    `"It's"`,
		"\u2018single\u2019":       "'single'",
		"\u201equote\u201f":        `"quote"`,
		"a\u2014b \u2013 c\u2212d": "a-b - c-d",
		"\ufb01ne":                 "fine",
		"\uff21\uff22\uff23":       "ABC",
		"wait\u2026":               "wait...",
		"cafe\u0301":               "caf\u00e9",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFoldAndLooseLevels(t *testing.T) {
	if got := Fold("  The \u201cBig\u201d\nDOG "); got != `the "big" dog` {
		t.Errorf("Fold = %q", got)
	}
	// Loose drops punctuation without touching the whitespace collapse, as
	// mike's normalizeWithMap does: "U.S." -> "us", "foo, bar" -> "foo bar".
	looseCases := map[string]string{
		"U.S. policy":           "us policy",
		"foo, bar":              "foo bar",
		"foo , bar":             "foo bar",
		"\u201cIt\u2019s\u201d": "its",
		"state-of-the-art":      "stateoftheart",
		"Tokyo | Japan":         "tokyo japan",
		"37,400,068":            "37400068",
		"\u2014 lead":           "lead",
	}
	for in, want := range looseCases {
		if got := Loose(in); got != want {
			t.Errorf("Loose(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every level is Normalize plus extra folding, never a different whitespace
// or invisible-character rule: Fold(x) == lower(Normalize(x)).
func TestLevelsShareOneNormalization(t *testing.T) {
	inputs := []string{
		"Le caf\u00e9\u00a0\u201ccr\u00e8me\u201d co\u00fbte 3 \u20ac\n\u00e0 Paris.",
		"co\u200bop  \t ERA\u2014tion\u2026",
		"\ufb01nal  ANSWER",
	}
	for _, in := range inputs {
		if got, want := Fold(in), strings.ToLower(Normalize(in)); got != want {
			t.Errorf("Fold(%q) = %q, want lower(Normalize) %q", in, got, want)
		}
	}
}

func TestMapOffsetsPointAtSourceBytes(t *testing.T) {
	src := "Le caf\u00e9\u00a0\u201ccr\u00e8me\u201d\n co\u00fbte \ufb01n\u2026 cafe\u0301!"
	for _, level := range []Level{LevelCanonical, LevelFolded, LevelLoose} {
		m := Map(src, level)
		if len(m.Start) != len(m.Text) || len(m.End) != len(m.Text) {
			t.Fatalf("level %d: map lengths %d/%d for %d text bytes", level, len(m.Start), len(m.End), len(m.Text))
		}
		for i := range m.Text {
			if m.Start[i] < 0 || m.End[i] > len(src) || m.Start[i] >= m.End[i] {
				t.Fatalf("level %d byte %d: bad source range [%d,%d)", level, i, m.Start[i], m.End[i])
			}
			if !utf8.ValidString(src[m.Start[i]:m.End[i]]) {
				t.Fatalf("level %d byte %d: source range splits a rune", level, i)
			}
		}
		if level == LevelCanonical && m.Text != Normalize(src) {
			t.Fatalf("Map canonical text %q != Normalize %q", m.Text, Normalize(src))
		}
	}

	m := Map(src, LevelFolded)
	cases := []struct{ needle, want string }{
		{"caf\u00e9 \"cr\u00e8me\" co\u00fbte", "caf\u00e9\u00a0\u201ccr\u00e8me\u201d\n co\u00fbte"},
		{"fin...", "\ufb01n\u2026"},
		{"caf\u00e9!", "cafe\u0301!"},
		{"in", "\ufb01n"}, // a match inside an expansion widens to the whole source rune
	}
	for _, tc := range cases {
		i := strings.Index(m.Text, tc.needle)
		if i < 0 {
			t.Fatalf("needle %q not in folded text %q", tc.needle, m.Text)
		}
		start, end := m.Source(i, i+len(tc.needle))
		if got := src[start:end]; got != tc.want {
			t.Errorf("Source(%q) = %q, want %q", tc.needle, got, tc.want)
		}
	}
}

var cidPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

func TestCIDStableUnderWhitespaceNotCase(t *testing.T) {
	base := CID("paragraph", "The quick brown fox.")
	if !cidPattern.MatchString(base) {
		t.Fatalf("cid %q is not %d lowercase hex chars", base, CIDLength)
	}
	same := []string{
		"The quick brown fox.",
		"  The quick\nbrown\u00a0fox.  ",
		"The  quick \u200bbrown\tfox.",
	}
	for _, text := range same {
		if got := CID("paragraph", text); got != base {
			t.Errorf("CID(%q) = %q, want %q (whitespace must not re-address)", text, got, base)
		}
	}
	different := map[string]string{
		"case":  "the quick brown fox.",
		"words": "The quick brown cat.",
		"punct": "The quick brown fox!",
	}
	for name, text := range different {
		if got := CID("paragraph", text); got == base {
			t.Errorf("%s change kept cid %q", name, got)
		}
	}
	if CID("heading", "The quick brown fox.") == base {
		t.Errorf("kind change kept cid")
	}
}

func TestAssignCIDsNumbersDuplicatesInOrder(t *testing.T) {
	blocks := []Block{
		{Kind: "paragraph", Text: "Repeat me."},
		{Kind: "paragraph", Text: "Unique."},
		{Kind: "paragraph", Text: "Repeat  me."},
		{Kind: "heading", Text: "Repeat me."},
		{Kind: "paragraph", Text: "Repeat me.\n"},
	}
	ids := AssignCIDs(blocks)
	p := CID("paragraph", "Repeat me.")
	h := CID("heading", "Repeat me.")
	want := []string{p, CID("paragraph", "Unique."), p + ".1", h, p + ".2"}
	if len(ids) != len(want) {
		t.Fatalf("got %d ids, want %d", len(ids), len(want))
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
	again := AssignCIDs(blocks)
	for i := range ids {
		if again[i] != ids[i] {
			t.Fatalf("AssignCIDs not deterministic at %d: %q vs %q", i, again[i], ids[i])
		}
	}
	if got := AssignCIDs(nil); len(got) != 0 {
		t.Fatalf("AssignCIDs(nil) = %v", got)
	}
}
