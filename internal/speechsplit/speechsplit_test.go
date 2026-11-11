package speechsplit

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// Failure list (written before the code):
//
//  1. a sentence is held back until the stream ends (the phone speaks late);
//  2. a period inside a number, a decimal, a domain or a dotted abbreviation
//     ends a sentence ("3.5 million", "U.S. Senate", "e.g. this");
//  3. a title or abbreviation ends one ("Dr. Smith", "Mrs. Jones", "St. Louis");
//  4. an initial ends one ("J. K. Rowling");
//  5. a sentence break is missed after a number ("in 2024. The talks");
//  6. "?" and "!" do not end one, or a closing quote is split from its sentence;
//  7. the split depends on how the stream was cut into deltas;
//  8. an unterminated final fragment is dropped;
//  9. a run with no punctuation never speaks (no forced flush at 200);
// 10. markdown marks reach the ear;
// 11. empty or whitespace-only pieces are emitted.

func split(deltas ...string) []string {
	var s Splitter
	var out []string
	for _, d := range deltas {
		out = append(out, s.Push(d)...)
	}
	return append(out, s.Flush()...)
}

func TestSplitterTable(t *testing.T) {
	for name, c := range map[string]struct {
		in   string
		want []string
	}{
		"plain":                    {"The strike ended. Talks resume Monday.", []string{"The strike ended.", "Talks resume Monday."}},
		"decimal":                  {"The fund holds 3.5 million euros. It is old.", []string{"The fund holds 3.5 million euros.", "It is old."}},
		"currency":                 {"It cost $4.99 today. Fine.", []string{"It cost $4.99 today.", "Fine."}},
		"domain":                   {"See example.com for more. Thanks.", []string{"See example.com for more.", "Thanks."}},
		"US abbreviation":          {"The U.S. Senate voted. It passed.", []string{"The U.S. Senate voted.", "It passed."}},
		"eg":                       {"Use a tool, e.g. a hammer. Then rest.", []string{"Use a tool, e.g. a hammer.", "Then rest."}},
		"Dr":                       {"Dr. Smith spoke. Mrs. Jones left.", []string{"Dr. Smith spoke.", "Mrs. Jones left."}},
		"St":                       {"He lives in St. Louis. It is warm.", []string{"He lives in St. Louis.", "It is warm."}},
		"initials":                 {"J. K. Rowling wrote it. Yes.", []string{"J. K. Rowling wrote it.", "Yes."}},
		"number then sentence":     {"It happened in 2024. The talks failed.", []string{"It happened in 2024.", "The talks failed."}},
		"soft pm ends":             {"We left at 5 p.m. We were late.", []string{"We left at 5 p.m.", "We were late."}},
		"soft pm continues":        {"At 5 p.m. tomorrow we leave.", []string{"At 5 p.m. tomorrow we leave."}},
		"etc ends":                 {"Bring tents, food, etc. Then go.", []string{"Bring tents, food, etc.", "Then go."}},
		"question and exclaim":     {"Is that so? It is! Good.", []string{"Is that so?", "It is!", "Good."}},
		"closing quote":            {`He said "stop." Then he left.`, []string{`He said "stop."`, "Then he left."}},
		"ellipsis":                 {"Well... maybe. Not sure.", []string{"Well... maybe.", "Not sure."}},
		"newline":                  {"First line\nSecond line", []string{"First line", "Second line"}},
		"unterminated tail":        {"One. Two without end", []string{"One.", "Two without end"}},
		"lone star is the text":    {"Compute 5 * 3 now. Done.", []string{"Compute 5 * 3 now.", "Done."}},
		"paired emphasis":          {"This is *very* important. Yes.", []string{"This is very important.", "Yes."}},
		"answer is no":             {"The answer is no. Talks resume.", []string{"The answer is no.", "Talks resume."}},
		"no then number":           {"See no. 5 now. Done.", []string{"See no. 5 now.", "Done."}},
		"negative number":          {"-5 degrees is cold. Yes.", []string{"-5 degrees is cold.", "Yes."}},
		"hash number":              {"#1 story today. Next.", []string{"#1 story today.", "Next."}},
		"tilde and C sharp":        {"About ~3 million use C#. Fine.", []string{"About ~3 million use C#.", "Fine."}},
		"underscore and pipe":      {"Use snake_case and a|b here. Ok.", []string{"Use snake_case and a|b here.", "Ok."}},
		"heading mark":             {"## Heading\nBody text.", []string{"Heading", "Body text."}},
		"markdown":                 {"**Bold** claim. Use `code` here.", []string{"Bold claim.", "Use code here."}},
		"only whitespace":          {"  \n  ", nil},
		"lowercase after period":   {"It is v. old. Maybe", []string{"It is v. old.", "Maybe"}},
		"cjk":                      {"今天下雨。明天晴天。", []string{"今天下雨。", "明天晴天。"}},
		"cjk closing quote stays":  {"他说：“下雨了。”然后走了。", []string{"他说：“下雨了。”", "然后走了。"}},
		"urdu trailing space":      {"یہ سچ ہے۔ وہ چلا گیا۔ ٹھیک", []string{"یہ سچ ہے۔", "وہ چلا گیا۔", "ٹھیک"}},
		"hindi quote and space":    {"उसने कहा \"ठीक है।\" फिर चला गया। अंत", []string{"उसने कहा \"ठीक है।\"", "फिर चला गया।", "अंत"}},
		"arabic":                   {"هل هذا صحيح؟ نعم۔ شكرا", []string{"هل هذا صحيح؟", "نعم۔", "شكرا"}},
		"devanagari":               {"यह सच है। वह चला गया॥ ठीक है", []string{"यह सच है।", "वह चला गया॥", "ठीक है"}},
		"no. abbreviation":         {"See no. 5 for details. Done.", []string{"See no. 5 for details.", "Done."}},
		"month abbreviation":       {"On Jan. 5 it began. Done.", []string{"On Jan. 5 it began.", "Done."}},
		"list bullet is plain":     {"- first item. second", []string{"first item. second"}},
		"multiple spaces collapse": {"Aa  bb   cc.  Dd ee.", []string{"Aa bb cc.", "Dd ee."}},
	} {
		if got := split(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q\n got %q\nwant %q", name, c.in, got, c.want)
		}
	}
}

// (7) however the stream is cut, the sentences are the same, one delta at a
// time included.
func TestSplitterIsIndependentOfDeltaBoundaries(t *testing.T) {
	text := "Dr. Smith said the U.S. grew 3.5 percent in 2024. Is that right? Yes! He left at 5 p.m. We stayed. The end"
	want := split(text)
	for step := 1; step <= 7; step++ {
		var deltas []string
		for i := 0; i < len(text); i += step {
			j := i + step
			if j > len(text) {
				j = len(text)
			}
			deltas = append(deltas, text[i:j])
		}
		if got := split(deltas...); !reflect.DeepEqual(got, want) {
			t.Fatalf("step %d: got %q want %q", step, got, want)
		}
	}
	if len(want) != 6 {
		t.Fatalf("sentences %q", want)
	}
}

// (1) a sentence is returned as soon as the first word of the next arrives,
// not at the end of the stream; and a complete sentence followed by its space
// is not held for the next sentence to finish.
func TestSplitterReturnsASentenceBeforeTheStreamEnds(t *testing.T) {
	var s Splitter
	if got := s.Push("The top story is a strike."); len(got) != 0 {
		t.Fatalf("a period at the end of the buffer was decided early: %q", got)
	}
	if got := s.Push(" Talks"); !reflect.DeepEqual(got, []string{"The top story is a strike."}) {
		t.Fatalf("got %q", got)
	}
}

// (9) forced flush.
func TestSplitterForcesAFlushOnlyAsATransportSafeguard(t *testing.T) {
	// A long real sentence is left whole: the cut is no steer on length.
	long := strings.Repeat("word ", 150) + "end." // 754 characters, one sentence
	if got := split(long); len(got) != 1 {
		t.Fatalf("a long sentence was cut: %d pieces", len(got))
	}
	words := strings.Repeat("word ", 900) // 4500 characters, no punctuation
	got := split(words)
	if len(got) < 2 {
		t.Fatalf("no forced flush: %q", got)
	}
	for _, g := range got {
		if len([]rune(g)) > MaxRunes {
			t.Fatalf("a sentence of %d runes", len([]rune(g)))
		}
	}
	if strings.Join(got, " ") != strings.TrimSpace(words) {
		t.Fatalf("text lost or changed: %q", got)
	}
	// A pause is preferred when there is one in the second half.
	text := strings.Repeat("a", 2500) + ", " + strings.Repeat("b", 3000)
	got = split(text)
	if len(got) != 2 || !strings.HasSuffix(got[0], ",") {
		t.Fatalf("pause cut: %q", got)
	}
	// An unbroken run is cut hard.
	if got := split(strings.Repeat("x", 9000)); len(got) < 3 {
		t.Fatalf("hard cut: %d pieces", len(got))
	}
}

// A forced cut at a multi-byte space (U+3000 ideographic, U+00A0) must leave
// whole runes on both sides.
func TestSplitterForcedCutAtAMultiByteSpaceKeepsWholeRunes(t *testing.T) {
	for _, space := range []string{"\u3000", "\u00a0", "\u2003"} {
		text := strings.Repeat("あ"+space, 4000) // 8000 runes, no terminator
		got := split(text)
		if len(got) < 2 {
			t.Fatalf("%q: no forced cut: %d pieces", space, len(got))
		}
		for i, g := range got {
			if !utf8.ValidString(g) || strings.ContainsRune(g, utf8.RuneError) {
				t.Fatalf("%q: piece %d has a broken rune", space, i)
			}
		}
		if strings.Join(strings.Fields(strings.Join(got, " ")), "") != strings.Repeat("あ", 4000) {
			t.Fatalf("%q: text lost or changed", space)
		}
	}
}
