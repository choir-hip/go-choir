package autoradioqueue

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The queue the Autoradio desk edits (docs/design/autoradio-queue-desk-2026-11-15.md).
// The failure list, written before the code:
//
//  1. a hostile snapshot: more than MaxUpcoming (50) upcoming entries (well
//     formed but too large, which the route answers without the queue), a bad or duplicated
//     entry id (the current entry's id included), an over-long string, an
//     unknown item kind, an item missing the field its kind needs, a negative
//     or NaN position, a negative version: each refused with the field named;
//  2. unknown fields inside a snapshot are ignored (a newer phone), a null or
//     absent current is an empty queue, a missing upcoming is no entries;
//  3. an op on an id that is not in the queue is refused, and so is a move or
//     a remove of the entry that is playing (that is skip);
//  4. an op on an empty queue (skip with nothing playing, clear with nothing
//     upcoming, previous with nothing playing) is refused;
//  5. a move to the entry's own place is refused; a move past either end is
//     clamped, never refused;
//  6. ops are simulated in order: remove then move of the same id is refused,
//     clear then move is refused, a move sees the list the earlier ops left;
//  7. an added item must be a story the request named as a candidate; any other
//     kind, a route that was not named and a story already in the queue are
//     refused; the title is the candidate's, never the model's;
//  8. an op with no reason is refused, a long reason is cut at 160 bytes on a
//     rune boundary and made one line;
//  9. at most MaxOps ops are recorded; the next is refused;
// 10. every recorded op carries the base version of the snapshot, and skip
//     carries the id of the entry it skips;
// 11. the model's text: malformed JSON, a non-array, an empty array, an unknown
//     op name, an entry that is not an object, invented fields: no panic, the
//     bad ones dropped with a reason, the good ones kept; a code fence around
//     the array is tolerated;
// 12. the stream cut: the fence split across deltas at any byte, speech before
//     it passes through whole, a lone "<" in speech is not lost, a fence that is
//     not `queue` yields no ops, nothing is spoken from after the fence;
// 13. the fence counts only at the start of a line and only when a "[" is the
//     next thing after it: the exact line quoted mid-sentence, or at a line
//     start with words after it, is speech, whole, at every chunk size;
// 14. text after the ops array does not cost the ops: the array's ops stand.

func story(route, title string) Item { return Item{Kind: "story", Route: route, Title: title} }

func snap3() Snapshot {
	return Snapshot{
		Version: 7,
		Current: &Current{ID: "q12", Item: story("front", "Strike in Nancy"), PositionSeconds: 4.5},
		Upcoming: []Entry{
			{ID: "q13", Item: Item{Kind: "audio", URL: "https://example.test/a.mp3", Title: "Harbour talks", Show: "Up First"}, AddedBy: "dj"},
			{ID: "q14", Item: Item{Kind: "page", URL: "https://example.test/p", Title: "A long read"}, AddedBy: "listener"},
			{ID: "q15", Item: story("sport", "Cup final"), AddedBy: "programme"},
		},
	}
}

func ids(s Snapshot) string {
	var out []string
	for _, e := range s.Upcoming {
		out = append(out, e.ID)
	}
	return strings.Join(out, ",")
}

func TestParseSnapshotRefusesAHostileQueue(t *testing.T) {
	many := `{"version":1,"upcoming":[`
	for i := 0; i < MaxUpcoming+1; i++ {
		if i > 0 {
			many += ","
		}
		many += `{"id":"e` + strings.Repeat("x", 3) + string(rune('a'+i%20)) + string(rune('a'+i/20%20)) + string(rune('a'+i/400)) + `","item":{"kind":"story","route":"r","title":"t"},"added_by":"dj"}`
	}
	many += `]}`
	for name, raw := range map[string]string{
		"too many":        many,
		"bad id":          `{"version":1,"upcoming":[{"id":"a b","item":{"kind":"story","route":"r","title":"t"}}]}`,
		"empty id":        `{"version":1,"upcoming":[{"id":"","item":{"kind":"story","route":"r","title":"t"}}]}`,
		"duplicate id":    `{"version":1,"upcoming":[{"id":"a","item":{"kind":"story","route":"r","title":"t"}},{"id":"a","item":{"kind":"story","route":"r","title":"t"}}]}`,
		"current dup":     `{"version":1,"current":{"id":"a","item":{"kind":"story","route":"r","title":"t"}},"upcoming":[{"id":"a","item":{"kind":"story","route":"r","title":"t"}}]}`,
		"long title":      `{"version":1,"upcoming":[{"id":"a","item":{"kind":"story","route":"r","title":"` + strings.Repeat("t", 301) + `"}}]}`,
		"long url":        `{"version":1,"upcoming":[{"id":"a","item":{"kind":"page","url":"` + strings.Repeat("u", 1025) + `","title":"t"}}]}`,
		"unknown kind":    `{"version":1,"upcoming":[{"id":"a","item":{"kind":"video","url":"u","title":"t"}}]}`,
		"story no route":  `{"version":1,"upcoming":[{"id":"a","item":{"kind":"story","title":"t"}}]}`,
		"page no url":     `{"version":1,"upcoming":[{"id":"a","item":{"kind":"page","title":"t"}}]}`,
		"doc no id":       `{"version":1,"upcoming":[{"id":"a","item":{"kind":"document","title":"t"}}]}`,
		"audio no url":    `{"version":1,"upcoming":[{"id":"a","item":{"kind":"audio","title":"t"}}]}`,
		"negative pos":    `{"version":1,"current":{"id":"a","item":{"kind":"story","route":"r","title":"t"},"position_seconds":-1}}`,
		"negative ver":    `{"version":-1}`,
		"not an object":   `[1,2]`,
		"version string":  `{"version":"7"}`,
		"huge position":   `{"version":1,"current":{"id":"a","item":{"kind":"story","route":"r","title":"t"},"position_seconds":1e300}}`,
		"added_by weird":  `{"version":1,"upcoming":[{"id":"a","item":{"kind":"story","route":"r","title":"t"},"added_by":"` + strings.Repeat("x", 40) + `"}]}`,
		"item missing":    `{"version":1,"upcoming":[{"id":"a"}]}`,
		"current no item": `{"version":1,"current":{"id":"a"}}`,
	} {
		if _, err := ParseSnapshot([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseSnapshotTakesAGoodQueueAndIgnoresWhatItDoesNotKnow(t *testing.T) {
	raw := `{"version":7,"future":{"x":1},"current":{"id":"q12","item":{"kind":"story","route":"front","title":"A","extra":1},"position_seconds":3.5,"more":2},
	"upcoming":[{"id":"q13","item":{"kind":"audio","url":"https://e.test/a.mp3","title":"T","show":"S","start":10,"end":null,"episode_id":null},"added_by":"dj"},
	{"id":"q14","item":{"kind":"document","id":"doc-1","title":"Book","at":120},"added_by":"listener"}]}`
	s, err := ParseSnapshot([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 7 || s.Current == nil || s.Current.ID != "q12" || len(s.Upcoming) != 2 || s.Upcoming[1].Item.At != 120 {
		t.Fatalf("%+v", s)
	}
	for _, empty := range []string{`{"version":0}`, `{"version":3,"current":null,"upcoming":null}`} {
		s, err := ParseSnapshot([]byte(empty))
		if err != nil || s.Current != nil || len(s.Upcoming) != 0 {
			t.Errorf("%s: %+v %v", empty, s, err)
		}
	}
}

func TestOpsOnAnEmptyQueueAreRefused(t *testing.T) {
	d := New(Snapshot{Version: 2}, nil)
	for name, err := range map[string]error{
		"skip":     d.Skip("because"),
		"previous": d.Previous("because"),
		"clear":    d.Clear("because"),
		"move":     d.Move("q1", 0, "because"),
		"remove":   d.Remove("q1", "because"),
	} {
		if err == nil {
			t.Errorf("%s on an empty queue was accepted", name)
		}
	}
	if len(d.Ops()) != 0 {
		t.Fatalf("recorded %+v", d.Ops())
	}
}

func TestMoveRemoveSkipAndTheirRefusals(t *testing.T) {
	d := New(snap3(), nil)
	for name, err := range map[string]error{
		"unknown move":   d.Move("nope", 0, "r"),
		"unknown remove": d.Remove("nope", "r"),
		"move current":   d.Move("q12", 0, "r"),
		"remove current": d.Remove("q12", "r"),
		"same place":     d.Move("q13", 0, "r"),
		"no reason":      d.Move("q14", 0, ""),
		"blank reason":   d.Move("q14", 0, " \n "),
	} {
		if err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if len(d.Ops()) != 0 {
		t.Fatalf("a refused op was recorded: %+v", d.Ops())
	}
	if err := d.Move("q14", 0, "you asked for the long read next"); err != nil {
		t.Fatal(err)
	}
	if got := ids(d.Snapshot()); got != "q14,q13,q15" {
		t.Fatalf("after move: %s", got)
	}
	// clamped past either end, never refused
	if err := d.Move("q14", 99, "last"); err != nil {
		t.Fatal(err)
	}
	if got := ids(d.Snapshot()); got != "q13,q15,q14" {
		t.Fatalf("after clamp high: %s", got)
	}
	if err := d.Move("q15", -5, "first"); err != nil {
		t.Fatal(err)
	}
	if got := ids(d.Snapshot()); got != "q15,q13,q14" {
		t.Fatalf("after clamp low: %s", got)
	}
	if err := d.Skip("moving on"); err != nil {
		t.Fatal(err)
	}
	ops := d.Ops()
	if len(ops) != 4 || ops[0].Op != "move" || ops[0].ID != "q14" || ops[0].ToIndex == nil || *ops[0].ToIndex != 0 {
		t.Fatalf("%+v", ops)
	}
	last := ops[3]
	if last.Op != "skip" || last.ID != "q12" {
		t.Fatalf("skip must name the current entry: %+v", last)
	}
	for _, o := range ops {
		if o.BaseVersion != 7 || o.Reason == "" {
			t.Fatalf("op without base version or reason: %+v", o)
		}
	}
}

func TestOpsAreSimulatedInOrder(t *testing.T) {
	d := New(snap3(), nil)
	if err := d.Remove("q14", "you skipped it"); err != nil {
		t.Fatal(err)
	}
	if err := d.Move("q14", 0, "r"); err == nil {
		t.Fatal("a move of an entry the same turn removed was accepted")
	}
	if err := d.Clear("nothing more"); err != nil {
		t.Fatal(err)
	}
	if err := d.Move("q13", 0, "r"); err == nil {
		t.Fatal("a move after clear was accepted")
	}
	if err := d.Clear("again"); err == nil {
		t.Fatal("a second clear of an empty list was accepted")
	}
	if got := ids(d.Snapshot()); got != "" {
		t.Fatalf("%s", got)
	}
}

func TestAddedItemsMustBeNamedStories(t *testing.T) {
	cands := []Candidate{{Route: "world", Title: "Talks resume"}, {Route: "sport", Title: "Cup final"}}
	d := New(snap3(), cands)
	for name, err := range map[string]error{
		"audio":         d.PlayNext(Item{Kind: "audio", URL: "https://x.test/a.mp3", Title: "t"}, "r"),
		"page":          d.Enqueue(Item{Kind: "page", URL: "https://x.test", Title: "t"}, "r"),
		"unnamed story": d.PlayNow(story("invented", "t"), "r"),
		"already there": d.PlayNext(story("sport", "Cup final"), "r"), // q15 is the same story
		"current":       d.Enqueue(story("front", "x"), "r"),
	} {
		if err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if len(d.Ops()) != 0 {
		t.Fatalf("%+v", d.Ops())
	}
	if err := d.PlayNext(story("world", "MODEL WROTE THIS TITLE"), "you asked for the news"); err != nil {
		t.Fatal(err)
	}
	if err := d.Enqueue(story("world", "t"), "r"); err == nil {
		t.Fatal("the same story twice in one turn was accepted")
	}
	o := d.Ops()[0]
	if o.Op != "play_next" || o.Item == nil || o.Item.Title != "Talks resume" || o.Item.Route != "world" || o.Item.Kind != "story" {
		t.Fatalf("%+v", o)
	}
	// the new entry sits at the front; an id-less entry cannot be named again
	if got := ids(d.Snapshot()); got != ",q13,q14,q15" {
		t.Fatalf("%q", got)
	}
	if err := d.Move("", 0, "r"); err == nil {
		t.Fatal("an empty id named the pending entry")
	}
	d2 := New(snap3(), cands)
	if err := d2.PlayNow(story("world", "x"), "now"); err != nil {
		t.Fatal(err)
	}
	if err := d2.Skip("r"); err == nil {
		t.Fatal("skip of the entry added this turn was accepted")
	}
}

func TestReasonsAreOneLineAndBounded(t *testing.T) {
	d := New(snap3(), nil)
	long := "é" + strings.Repeat("é", 200) + "\nsecond line"
	if err := d.Remove("q13", long); err != nil {
		t.Fatal(err)
	}
	r := d.Ops()[0].Reason
	if len(r) > MaxReasonBytes || strings.ContainsAny(r, "\n\r") || !validUTF8(r) {
		t.Fatalf("reason %q (%d)", r, len(r))
	}
}

func validUTF8(s string) bool { return strings.ToValidUTF8(s, "�") == s }

func TestAtMostMaxOps(t *testing.T) {
	var up []Entry
	for i := 0; i < MaxOps+3; i++ {
		up = append(up, Entry{ID: "e" + string(rune('a'+i)), Item: Item{Kind: "page", URL: "u", Title: "t"}})
	}
	d := New(Snapshot{Version: 1, Upcoming: up}, nil)
	for i := 0; i < MaxOps+3; i++ {
		err := d.Remove(up[i].ID, "r")
		if i < MaxOps && err != nil {
			t.Fatalf("op %d refused: %v", i, err)
		}
		if i >= MaxOps && err == nil {
			t.Fatalf("op %d accepted", i)
		}
	}
	if len(d.Ops()) != MaxOps {
		t.Fatalf("%d ops", len(d.Ops()))
	}
}

func TestApplyModelOps(t *testing.T) {
	cands := []Candidate{{Route: "world", Title: "Talks resume"}}
	good := `[{"op":"move","id":"q14","to_index":0,"reason":"long read next","extra":1},
	  {"op":"play_next","item":{"kind":"story","route":"world"},"reason":"the news"},
	  {"op":"teleport","id":"q13","reason":"x"},
	  {"op":"remove","id":"ghost","reason":"x"},
	  "not an object",
	  {"op":"move","id":"q13","reason":"no index"},
	  {"op":"remove","id":"q13","reason":"skipping the show"}]`
	for name, raw := range map[string]string{"plain": good, "fenced": "```json\n" + good + "\n```", "fence no lang": "```\n" + good + "\n```"} {
		d := New(snap3(), cands)
		drops := d.ApplyJSON(raw)
		ops := d.Ops()
		if len(ops) != 3 || ops[0].Op != "move" || ops[1].Op != "play_next" || ops[2].Op != "remove" {
			t.Errorf("%s: ops %+v", name, ops)
		}
		if len(drops) != 4 {
			t.Errorf("%s: drops %v", name, drops)
		}
	}
	for name, raw := range map[string]string{
		"empty":         ``,
		"not json":      `move the podcast`,
		"truncated":     `[{"op":"move","id":"q14"`,
		"object":        `{"op":"move","id":"q14","to_index":0,"reason":"r"}`,
		"empty array":   `[]`,
		"null":          `null`,
		"number":        `7`,
		"trailing junk": `[] extra`,
	} {
		d := New(snap3(), cands)
		drops := d.ApplyJSON(raw)
		if len(d.Ops()) != 0 {
			t.Errorf("%s: recorded %+v", name, d.Ops())
		}
		if raw != "" && raw != "[]" && len(drops) == 0 {
			t.Errorf("%s: no drop reason", name)
		}
	}
	// a huge array of junk neither panics nor records past MaxOps
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < 5000; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"op":"remove","id":"q13","reason":"r"}`)
	}
	b.WriteString("]")
	d := New(snap3(), nil)
	d.ApplyJSON(b.String())
	if len(d.Ops()) != 1 {
		t.Fatalf("%d ops from 5000 copies of one remove", len(d.Ops()))
	}
}

func TestOpsMarshalToThePhonesShape(t *testing.T) {
	d := New(snap3(), []Candidate{{Route: "world", Title: "Talks"}})
	_ = d.Move("q15", 0, "the cup final")
	_ = d.PlayNext(story("world", ""), "the news")
	raw, err := json.Marshal(d.Ops())
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"op":"move","id":"q15","to_index":0,"base_version":7,"reason":"the cup final"},{"op":"play_next","item":{"kind":"story","route":"world","title":"Talks"},"base_version":7,"reason":"the news"}]`
	if string(raw) != want {
		t.Fatalf("\n got %s\nwant %s", raw, want)
	}
}

func TestCutSplitsSpeechFromOpsAtAnyByte(t *testing.T) {
	whole := "Okay, the long read is next.\n" + FenceLine("AR1234") + "\n[{\"op\":\"move\",\"id\":\"q14\",\"to_index\":0,\"reason\":\"r\"}]"
	for cut1 := 0; cut1 <= len(whole); cut1++ {
		for _, cut2 := range []int{cut1, min(cut1+1, len(whole)), min(cut1+3, len(whole)), len(whole)} {
			c := NewCut("AR1234")
			var speech strings.Builder
			speech.WriteString(c.Push(whole[:cut1]))
			speech.WriteString(c.Push(whole[cut1:cut2]))
			speech.WriteString(c.Push(whole[cut2:]))
			speech.WriteString(c.Flush())
			if speech.String() != "Okay, the long read is next.\n" {
				t.Fatalf("cuts %d,%d: speech %q", cut1, cut2, speech.String())
			}
			if !c.Fenced() || !strings.HasPrefix(strings.TrimSpace(c.Ops()), `[{"op":"move"`) {
				t.Fatalf("cuts %d,%d: ops %q fenced=%t", cut1, cut2, c.Ops(), c.Fenced())
			}
		}
	}
}

// Anything that is not exactly the request's fence is speech, released whole:
// a lone "<", a "<<<" the model quotes, a page fence, an end fence, a fence
// with another token, a fence cut short at the end of the stream.
func TestCutPassesEverythingButTheExactFenceThrough(t *testing.T) {
	for name, text := range map[string]string{
		"lone bracket":                     "Is 3 < 4? Yes <",
		"quoted":                           "The sign says: <<< keep left. That is all.",
		"page fence":                       "The page is fenced as <<<AR1234 page>>> and the text follows.",
		"end queue":                        "Closing with <<<AR1234 end queue>>> is not it.",
		"other token":                      "Fine. <<<AR9999 queue>>> [{\"op\":\"clear\"}]",
		"no space":                         "Fine. <<<AR1234queue>>> [] ",
		"upper":                            "Fine. <<<AR1234 QUEUE>>> [] ",
		"unterminated":                     "Fine. <<<AR1234 queue",
		"trailing partial":                 "Fine. <<<AR12",
		"echo mid-sentence":                "To move it I would end with " + FenceLine("AR1234") + " and then the ops. The long read is next.",
		"echo at line start, words after":  "Intro.\n" + FenceLine("AR1234") + " and then the ops.",
		"echo at line start, a lone fence": "Intro.\n" + FenceLine("AR1234"),
		"echo twice then a real one":       "A " + FenceLine("AR1234") + " b\n" + FenceLine("AR1234") + " c",
	} {
		for _, chunk := range []int{1, 2, 5, 1000} {
			c := NewCut("AR1234")
			var got strings.Builder
			for i := 0; i < len(text); i += chunk {
				got.WriteString(c.Push(text[i:min(i+chunk, len(text))]))
			}
			got.WriteString(c.Flush())
			if got.String() != text || c.Fenced() || c.Ops() != "" {
				t.Errorf("%s (chunk %d): speech %q fenced=%t ops=%q", name, chunk, got.String(), c.Fenced(), c.Ops())
			}
		}
	}
}

func TestCutTakesTheFenceAtTheStartOfTheStreamAndAfterBlankSpace(t *testing.T) {
	for name, text := range map[string]string{
		"stream start":  FenceLine("AR1234") + `[{"op":"clear","reason":"r"}]`,
		"spaces then [": "Ok.\n" + FenceLine("AR1234") + " \n\n  [{\"op\":\"clear\",\"reason\":\"r\"}]",
	} {
		for _, chunk := range []int{1, 3, 1000} {
			c := NewCut("AR1234")
			var got strings.Builder
			for i := 0; i < len(text); i += chunk {
				got.WriteString(c.Push(text[i:min(i+chunk, len(text))]))
			}
			got.WriteString(c.Flush())
			if !c.Fenced() || !strings.Contains(c.Ops(), `"op":"clear"`) || strings.Contains(got.String(), "<<<") || strings.Contains(got.String(), "clear") {
				t.Errorf("%s (chunk %d): speech %q fenced=%t ops=%q", name, chunk, got.String(), c.Fenced(), c.Ops())
			}
		}
	}
}

// The shapes a model plausibly writes the fence in: indented, and with the array
// in a code fence. The ops never reach speech, whatever the chunking.
func TestCutTakesTheFenceIndentedAndBeforeACodeFence(t *testing.T) {
	arr := `[{"op":"clear","reason":"r"}]`
	for name, text := range map[string]string{
		"indented spaces": "Ok.\n  " + FenceLine("AR1234") + "\n" + arr,
		"indented tab":    "Ok.\n\t" + FenceLine("AR1234") + "\n" + arr,
		"indented first":  "  " + FenceLine("AR1234") + arr,
		"code fence":      "Ok.\n" + FenceLine("AR1234") + "\n```json\n" + arr + "\n```",
		"code fence bare": "Ok.\n" + FenceLine("AR1234") + "\n```\n" + arr + "\n```",
		"indent and code": "Ok.\n \t" + FenceLine("AR1234") + " ```json\n" + arr + "\n```",
	} {
		for chunk := 1; chunk <= len(text); chunk++ {
			c := NewCut("AR1234")
			var got strings.Builder
			for i := 0; i < len(text); i += chunk {
				got.WriteString(c.Push(text[i:min(i+chunk, len(text))]))
			}
			got.WriteString(c.Flush())
			if !c.Fenced() || strings.Contains(got.String(), "clear") || strings.Contains(got.String(), "<<<") || strings.Contains(got.String(), "`") {
				t.Fatalf("%s (chunk %d): speech %q fenced=%t", name, chunk, got.String(), c.Fenced())
			}
			d := New(snap3(), nil)
			d.ApplyJSON(c.Ops())
			if len(d.Ops()) != 1 {
				t.Fatalf("%s (chunk %d): the ops %q did not apply: %v", name, chunk, c.Ops(), d.Ops())
			}
		}
	}
	// an indented echo mid-sentence is still speech
	c := NewCut("AR1234")
	text := "Say " + FenceLine("AR1234") + " then more.\n   " + FenceLine("AR1234") + " words."
	if got := c.Push(text) + c.Flush(); got != text || c.Fenced() {
		t.Fatalf("%q", got)
	}
}

func TestTextAfterTheArrayDoesNotCostTheOps(t *testing.T) {
	d := New(snap3(), nil)
	drops := d.ApplyJSON(`[{"op":"remove","id":"q13","reason":"r"}] and that is all, ` + strings.Repeat("x", 5000))
	if len(d.Ops()) != 1 || len(drops) != 1 || len(drops[0]) > 200 {
		t.Fatalf("ops %+v drops %v", d.Ops(), drops)
	}
}

func TestBoundsAreSplitIntoTooLargeAndMalformed(t *testing.T) {
	var up []string
	for i := 0; i <= MaxUpcoming; i++ {
		up = append(up, fmt.Sprintf(`{"id":"e%d","item":{"kind":"story","route":"r","title":"t"}}`, i))
	}
	for name, raw := range map[string]string{
		"too many":   `{"version":1,"upcoming":[` + strings.Join(up, ",") + `]}`,
		"long title": `{"version":1,"upcoming":[{"id":"a","item":{"kind":"story","route":"r","title":"` + strings.Repeat("t", 301) + `"}}]}`,
		"long url":   `{"version":1,"upcoming":[{"id":"a","item":{"kind":"page","url":"` + strings.Repeat("u", 1025) + `","title":"t"}}]}`,
	} {
		if _, err := ParseSnapshot([]byte(raw)); !IsTooLarge(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, raw := range map[string]string{
		"bad id":  `{"version":1,"upcoming":[{"id":"a b","item":{"kind":"story","route":"r","title":"t"}}]}`,
		"no kind": `{"version":1,"upcoming":[{"id":"a","item":{"title":"t"}}]}`,
		"json":    `[`,
	} {
		if _, err := ParseSnapshot([]byte(raw)); err == nil || IsTooLarge(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// a realistic 800-byte URL is inside the bound
	ok := `{"version":1,"upcoming":[{"id":"a","item":{"kind":"audio","url":"` + strings.Repeat("u", 800) + `","title":"t"}}]}`
	if _, err := ParseSnapshot([]byte(ok)); err != nil {
		t.Fatal(err)
	}
}

func TestModelTextInDropReasonsIsBoundedAndOneLine(t *testing.T) {
	d := New(snap3(), nil)
	huge := strings.Repeat("x", 9000)
	drops := d.ApplyJSON(`[{"op":"enqueue","item":{"kind":"` + huge + `","route":"` + huge + `"},"reason":"r"},` +
		`{"op":"play_next","item":{"kind":"story","route":"` + huge + `"},"reason":"r"},{"op":"move","id":"` + huge + `","to_index":1,"reason":"r"}]`)
	if len(drops) != 3 {
		t.Fatalf("%v", drops)
	}
	for _, x := range drops {
		if len(x) > 300 {
			t.Fatalf("a drop reason of %d bytes", len(x))
		}
	}
	d2 := New(snap3(), nil)
	if err := d2.Remove("q13", "one\u2028two\u2029three"); err != nil {
		t.Fatal(err)
	}
	if r := d2.Ops()[0].Reason; strings.ContainsAny(r, "\u2028\u2029") {
		t.Fatalf("%q", r)
	}
}
