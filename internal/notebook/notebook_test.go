package notebook

import (
	"strconv"
	"strings"
	"testing"
)

// Failure list, written before the code (the page and its budget):
//
//	B1 a line the listener wrote is cut, moved or rewritten by a desk patch;
//	B2 a line with a tag in its text, a newline or over 200 characters is written;
//	B3 a full section takes a new line with no displacement, or a displacement
//	   names a line that is not live in that section;
//	B4 the sections Autoradio writes pass 1,000 words;
//	B5 more than 10 steering lines are kept, or the newest is the one dropped;
//	B6 a struck line stays, or the listener's corrected words are lost or not
//	   moved to "What you asked for";
//	B7 counts are written to a line the patch does not name, or a count changes
//	   any text;
//	B8 render(parse(x)) changes a document the listener edited by hand;
//	B9 a tap cuts a bullet the listener put under Recent steering, or the same
//	   tap twice writes two lines (TestN7, TestN7_ARepeatedTapIsOneLine).

func fresh(t *testing.T, ops ...Op) Doc {
	t.Helper()
	d, err := Fresh().Apply(Patch{Ops: ops})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestB1_ListenerLinesSurviveEverything(t *testing.T) {
	md := "# My notebook\n\n## What you asked for\n- Mornings: world news first.\nA paragraph the listener wrote.\n\n## What Autoradio has learned\n- Short pieces are heard. [L1 · heard 2 of 3]\n- My own line in the wrong place.\n"
	d := Parse(md)
	d2, err := d.Apply(Patch{Ops: []Op{{Op: "add", Section: "learned", ID: "L2", Text: "New.", Replaces: []string{"L1"}}}, Counts: Counts{"L2": {1, 1}}})
	if err != nil {
		t.Fatal(err)
	}
	out := d2.Render()
	for _, want := range []string{"- Mornings: world news first.", "A paragraph the listener wrote.", "- My own line in the wrong place.", "- New. [L2 · heard 1 of 1]"} {
		if !strings.Contains(out, want) {
			t.Errorf("(B1) lost %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "L1") {
		t.Errorf("(B3) a replaced line stays:\n%s", out)
	}
}

func TestB2_TextShape(t *testing.T) {
	for name, text := range map[string]string{
		"tag": "Hello [L9 · heard 1 of 1]", "tag2": "x [T3", "newline": "a\nb", "long": strings.Repeat("x", 201),
		"empty": "  ", "dash": "- sneaky", "heading": "# big",
	} {
		if CheckText(text) == nil {
			t.Errorf("(B2) %s accepted", name)
		}
	}
	if err := CheckText(strings.Repeat("é", 200)); err != nil {
		t.Errorf("(B2) 200 characters refused: %v", err)
	}
}

func TestB3_FullSectionNeedsADisplacement(t *testing.T) {
	var ops []Op
	for i := 1; i <= MaxLearned; i++ {
		ops = append(ops, Op{Op: "add", Section: "learned", ID: "L" + itoa(i), Text: "Lesson " + itoa(i) + "."})
	}
	d := fresh(t, ops...)
	if _, err := d.Apply(Patch{Ops: []Op{{Op: "add", Section: "learned", ID: "L13", Text: "One more."}}}); err == nil {
		t.Error("(B3) the 13th lesson was written")
	}
	if _, err := d.Apply(Patch{Ops: []Op{{Op: "add", Section: "learned", ID: "L13", Text: "One more.", Replaces: []string{"T1"}}}}); err == nil {
		t.Error("(B3) a displacement of a line that is not live")
	}
	d2, err := d.Apply(Patch{Ops: []Op{{Op: "add", Section: "learned", ID: "L13", Text: "Merged.", Replaces: []string{"L2", "L3"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d2.Lines(SecLearned)); n != MaxLearned-1 {
		t.Errorf("(B3) %d lines after a merge of two", n)
	}
	// Trying holds 3.
	d3 := fresh(t, Op{Op: "add", Section: "trying", ID: "T1", Text: "a"}, Op{Op: "add", Section: "trying", ID: "T2", Text: "b"}, Op{Op: "add", Section: "trying", ID: "T3", Text: "c"})
	if _, err := d3.Apply(Patch{Ops: []Op{{Op: "add", Section: "trying", ID: "T4", Text: "d"}}}); err == nil {
		t.Error("(B3) a 4th trying line")
	}
	if _, err := d3.Apply(Patch{Ops: []Op{{Op: "add", Section: "learned", ID: "T9", Text: "wrong letter"}}}); err == nil {
		t.Error("(B3) a T id in learned")
	}
}

func TestB4_TheHardWordCap(t *testing.T) {
	long := strings.Repeat("word ", 39) + "end" // 40 words a line
	var ops []Op
	for i := 1; i <= MaxLearned; i++ {
		ops = append(ops, Op{Op: "add", Section: "learned", ID: "L" + itoa(i), Text: long})
	}
	d := fresh(t, ops...) // 12 lines of about 50 words: under the cap
	if w := d.WrittenWords(); w >= HardWords || w < 400 {
		t.Fatalf("setup: %d words", w)
	}
	// Taps are never refused for the page's words, but they count.
	var taps []Op
	for i := 0; i < 10; i++ {
		taps = append(taps, Op{Op: "tap", Ref: "tap:e" + itoa(i), Text: "More like this: \"t" + itoa(i) + " " + strings.Repeat("w ", 60) + "\""})
	}
	d2, err := d.Apply(Patch{Ops: taps})
	if err != nil {
		t.Fatalf("(N6) taps refused for words: %v", err)
	}
	if d2.WrittenWords() <= HardWords {
		t.Fatalf("setup: %d words", d2.WrittenWords())
	}
	// An add that would leave the page over the cap is refused.
	if _, err := d2.Apply(Patch{Ops: []Op{{Op: "add", Section: "learned", ID: "L13", Text: "Merged.", Replaces: []string{"L1"}}}}); err == nil {
		t.Error("(B4) the page passed 1,000 words")
	}
	// The listener's own bullets are never counted.
	lis := "# My notebook\n\n## What you asked for\n" + strings.Repeat("- "+strings.Repeat("mine ", 100)+"\n", 20)
	d3, err := Parse(lis).Apply(Patch{Ops: []Op{{Op: "add", Section: "learned", ID: "L1", Text: "Fine."}}})
	if err != nil || d3.WrittenWords() > 10 {
		t.Errorf("(N6) listener words refused or counted: %v %d", err, d3.WrittenWords())
	}
}

func TestB5_StewardingKeepsTheNewestTen(t *testing.T) {
	d := Fresh()
	for i := 0; i < 13; i++ {
		var err error
		d, err = d.ApplyOp(Op{Op: "tap", Ref: "tap:e" + itoa(i), Text: "More like this: \"t" + itoa(i) + "\" (1 Jan)"})
		if err != nil {
			t.Fatal(err)
		}
	}
	ls := d.Lines(SecSteering)
	if len(ls) != MaxSteering || !strings.Contains(ls[len(ls)-1].Text, "t12") || strings.Contains(ls[0].Text, "\"t0\"") {
		t.Errorf("(B5) %d lines, first %q last %q", len(ls), ls[0].Text, ls[len(ls)-1].Text)
	}
}

func TestN7_TheSteeringCapCutsOnlyAutoradiosLines(t *testing.T) {
	d := Parse("# My notebook\n\n## Recent steering\n- my own note about steering\n")
	for i := 0; i < 12; i++ {
		var err error
		d, err = d.ApplyOp(Op{Op: "tap", Ref: "tap:e" + itoa(i), Text: "Less like this: \"t" + itoa(i) + "\" (1 Jan)"})
		if err != nil {
			t.Fatal(err)
		}
	}
	ls := d.Lines(SecSteering)
	if len(ls) != MaxSteering+1 || ls[0].Text != "my own note about steering" {
		t.Errorf("(N7) %d lines, first %q", len(ls), ls[0].Text)
	}
}

func TestN8_NoLinksInALesson(t *testing.T) {
	for _, text := range []string{"See https://example.com now", "a [link](x) here", "www.example.com is heard", "<https://x.y>"} {
		if CheckText(text) == nil {
			t.Errorf("(N8) %q accepted", text)
		}
	}
}

func TestN9_UntagMakesTheListenersWordsTheirOwn(t *testing.T) {
	d := fresh(t, Op{Op: "add", Section: "learned", ID: "L4", Text: "Mine to lose."})
	d2, err := d.Apply(Patch{Ops: []Op{{Op: "untag", ID: "L4", Text: "I reworded this [L9 oddly"}}})
	if err != nil {
		t.Fatal(err)
	}
	ls := d2.Lines(SecLearned)
	if len(ls) != 1 || ls[0].Tag != "" || ls[0].Text != "I reworded this [L9 oddly" || len(d2.Tagged()) != 0 {
		t.Errorf("(N9) %+v", ls)
	}
}

func TestB6_StrikeAndCorrection(t *testing.T) {
	d := fresh(t, Op{Op: "add", Section: "learned", ID: "L7", Text: "Policy pieces are skipped."})
	d2, err := d.Apply(Patch{Ops: []Op{{Op: "strike", ID: "L7", Replacement: "I like policy pieces."}}})
	if err != nil {
		t.Fatal(err)
	}
	asked := d2.Lines(SecAsked)
	if len(d2.Lines(SecLearned)) != 0 || len(asked) != 1 || asked[0].Text != "I like policy pieces. (you corrected L7)" {
		t.Errorf("(B6) learned %v asked %v", d2.Lines(SecLearned), asked)
	}
	if _, err := d.Apply(Patch{Ops: []Op{{Op: "retire", ID: "L99"}}}); err == nil {
		t.Error("(B6) retiring a line that is not live")
	}
}

func TestB7_CountsTouchOnlyTaggedLines(t *testing.T) {
	d := fresh(t, Op{Op: "add", Section: "learned", ID: "L1", Text: "A."}, Op{Op: "ask", Text: "Mine."})
	d2, err := d.Apply(Patch{Counts: Counts{"L1": {9, 11}, "L5": {1, 1}, "mine": {3, 3}}})
	if err != nil {
		t.Fatal(err)
	}
	out := d2.Render()
	if !strings.Contains(out, "- A. [L1 · heard 9 of 11]") || strings.Contains(out, "L5") || !strings.Contains(out, "- Mine.\n") {
		t.Errorf("(B7)\n%s", out)
	}
	if ls := d2.Lines(SecLearned); ls[0].Ref() != "lesson:L1" || d2.Lines(SecAsked)[0].Ref() != MineRef("Mine.") {
		t.Errorf("(B7) refs %q %q", ls[0].Ref(), d2.Lines(SecAsked)[0].Ref())
	}
}

func TestB8_RoundTripOfAHandEditedDocument(t *testing.T) {
	md := "# My notebook\n\n## What you asked for\n- one\n- two\n\nsome prose\n\n## What Autoradio has learned\n- x [L1 · heard 0 of 0]\n\n## My own section\n- keep me\n"
	if got := Parse(md).Render(); strings.TrimSpace(got) != strings.TrimSpace(md) {
		t.Errorf("(B8)\n%s\n--- want ---\n%s", got, md)
	}
}

func itoa(i int) string { return strconv.Itoa(i) }

func TestN7_ARepeatedTapIsOneLine(t *testing.T) {
	d := Fresh()
	for _, text := range []string{`Less like this: "Harbor fees rise" (1 Jan 08:00)`, `Less like this: "Harbor fees rise" (1 Jan 08:07)`, `More like this: "Harbor fees rise" (1 Jan 08:09)`} {
		var err error
		if d, err = d.ApplyOp(Op{Op: "tap", Ref: "tap:e", Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(d.Lines(SecSteering)); n != 2 {
		t.Errorf("(B9) %d lines, want the less and the more", n)
	}
}
