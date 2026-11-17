package agentcore

// Station L1: the validators of design 3.2, the run of three invalid lines,
// the line reader, the fixture of station L1a and the wiring (failure list
// items F16 to F27, F40 to F43).

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// One case sends one line among valid ones and says whether it is kept.
type lineCase struct {
	name string
	line string
	keep bool
}

func runLineCases(t *testing.T, cases []lineCase, prepare func(*Runtime, *fakeRadio)) {
	t.Helper()
	rt, fake := radioRuntime(t, linesScript())
	seedInputs(t, rt, fake)
	if prepare != nil {
		prepare(rt, fake)
	}
	for i, c := range cases {
		fake.script = linesScript(titled("before", nil), c.line, titled("after", nil))
		_, evs := progRun(t, rt, progBody(withRequestID("case-"+string(rune('a'+i%26))+strings.Repeat("x", i/26))))
		got := segmentTitles(evs)
		want := []string{"before", "after"}
		if c.keep {
			want = nil
			var m map[string]any
			_ = json.Unmarshal([]byte(c.line), &m)
			want = []string{"before", titleOf(m), "after"}
		}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s: streamed %q, want %q", c.name, got, want)
		}
		// A dropped line had no claim and no segment record.
		sid := stretchIDOf(t, evs)
		if n := len(stretchClaims(t, rt, sid)); n != len(want) {
			t.Errorf("%s: %d claims for %d streamed content segments", c.name, n, len(want))
		}
		if len(eventsNamed(evs, "done")) != 1 {
			t.Errorf("%s: the call did not end with done: %+v", c.name, evs)
		}
	}
}

func titleOf(m map[string]any) string {
	s, _ := m["title"].(string)
	return strings.TrimSpace(s)
}

func TestF16_PHeard(t *testing.T) {
	runLineCases(t, []lineCase{
		{"1.7", titled("t17", map[string]any{"p_heard": 1.7}), false},
		{"-0.1", titled("tneg", map[string]any{"p_heard": -0.1}), false},
		{"a string", titled("tstr", map[string]any{"p_heard": "0.7"}), false},
		{"NaN spelled", titled("tnan", map[string]any{"p_heard": "NaN"}), false},
		{"null", titled("tnull", map[string]any{"p_heard": json.RawMessage("null")}), false},
		{"absent", titled("tabs", map[string]any{"p_heard": nil}), false},
		{"an array", titled("tarr", map[string]any{"p_heard": []any{0.5}}), false},
		{"0", titled("t0", map[string]any{"p_heard": 0}), true},
		{"1", titled("t1", map[string]any{"p_heard": 1}), true},
	}, nil)
}

func TestF17_CausedByGrammar(t *testing.T) {
	runLineCases(t, []lineCase{
		{"absent", titled("a", map[string]any{"caused_by": nil}), false},
		{"unknown class", titled("b", map[string]any{"caused_by": "vibes"}), false},
		{"hunch", titled("c", map[string]any{"caused_by": "hunch"}), false},
		{"bare lesson", titled("d", map[string]any{"caused_by": "lesson"}), false},
		{"bare instruction", titled("e", map[string]any{"caused_by": "instruction"}), false},
		{"bare edition", titled("f", map[string]any{"caused_by": "edition"}), false},
		{"importance with a ref", titled("g", map[string]any{"caused_by": "importance:3"}), false},
		{"turn ref with no id", titled("h", map[string]any{"caused_by": "instruction:turn/"}), false},
		{"turn ref with a colon", titled("i", map[string]any{"caused_by": "instruction:turn/a:b"}), false},
		{"not a string", titled("j", map[string]any{"caused_by": 3}), false},
		{"a ref of the cause list", titled("k", map[string]any{"caused_by": []any{"explore"}}), false},
		{"importance", titled("l", map[string]any{"caused_by": "importance"}), true},
		{"explore", titled("m", map[string]any{"caused_by": "explore"}), true},
	}, nil)
}

func TestF18_F19_F20_RefsResolveAgainstTheInput(t *testing.T) {
	runLineCases(t, []lineCase{
		{"a turn nobody spoke", titled("a", map[string]any{"caused_by": "instruction:turn/turn-404"}), false},
		{"a note nobody wrote", titled("b", map[string]any{"caused_by": "instruction:note/zz"}), false},
		{"a listed turn", titled("c", map[string]any{"caused_by": "instruction:turn/turn-1"}), true},
		{"a listed note", titled("d", map[string]any{"caused_by": "instruction:note/ab12"}), true},
		{"edition P0", titled("e", map[string]any{"caused_by": "edition:P0"}), false},
		{"edition P4 of 3", titled("f", map[string]any{"caused_by": "edition:P4"}), false},
		{"edition P9", titled("g", map[string]any{"caused_by": "edition:P9"}), false},
		{"edition P01", titled("h", map[string]any{"caused_by": "edition:P01"}), false},
		{"edition P1", titled("i", map[string]any{"caused_by": "edition:P1"}), true},
		{"edition P3", titled("j", map[string]any{"caused_by": "edition:P3"}), true},
		{"an unknown lesson", titled("k", map[string]any{"caused_by": "lesson:L99"}), false},
		{"a struck or never written lesson", titled("l", map[string]any{"caused_by": "lesson:L2"}), false},
		{"an active lesson", titled("m", map[string]any{"caused_by": "lesson:L1"}), true},
	}, nil)
}

func TestF19_NoEditionSentMeansNoEditionRef(t *testing.T) {
	rt, fake := radioRuntime(t, linesScript(titled("kept", map[string]any{"caused_by": "importance"}), titled("e1", map[string]any{"caused_by": "edition:P1"})))
	_ = fake
	_, evs := progRun(t, rt, progBody(func(m map[string]any) { delete(m, "edition_text") }))
	if got := segmentTitles(evs); strings.Join(got, "|") != "kept" {
		t.Fatalf("titles %q", got)
	}
}

func TestF20_ALessonCauseCarriesTheTallyTag(t *testing.T) {
	rt, fake := radioRuntime(t, linesScript(titled("lesson-led", map[string]any{"caused_by": "lesson:L1"})))
	seedInputs(t, rt, fake)
	fake.script = linesScript(titled("lesson-led", map[string]any{"caused_by": "lesson:L1"}))
	_, evs := progRun(t, rt, progBody())
	claims := stretchClaims(t, rt, stretchIDOf(t, evs))
	if len(claims) != 1 || tagValue(claims[0], "lesson:") != "L1" || tagValue(claims[0], "cause:") != "lesson" {
		t.Fatalf("claims %+v", claims)
	}
	if !strings.Contains(claims[0].Calibration.Claim.Criterion, "caused_by=lesson:L1") {
		t.Fatalf("criterion %q", claims[0].Calibration.Claim.Criterion)
	}
}

func TestF21_ASameResponseLessonCiteIsDroppedAndTheLessonLineWritesNothing(t *testing.T) {
	rt, _ := radioRuntime(t, linesScript(
		`{"kind":"lesson","text":"Harbour stories are heard through.","cites":["air:ev-1","air:ev-2"]}`,
		titled("cites the lesson just written", map[string]any{"caused_by": "lesson:L1"}),
		titled("fine", nil),
	))
	_, evs := progRun(t, rt, progBody())
	if got := segmentTitles(evs); strings.Join(got, "|") != "fine" {
		t.Fatalf("titles %q", got)
	}
	if n := len(tape(t, rt, "autoradio.lesson")); n != 0 {
		t.Fatalf("%d lesson records", n)
	}
	// A lesson line neither counts toward the run of three nor resets it: two
	// invalid lines, a lesson line and one more invalid line are a run of three.
	rt2, _ := radioRuntime(t, linesScript(
		titled("bad1", map[string]any{"p_heard": 9}), titled("bad2", map[string]any{"p_heard": 9}),
		`{"kind":"lesson","text":"x","cites":["air:ev-1","air:ev-2"]}`,
		titled("bad3", map[string]any{"p_heard": 9}), titled("never", nil),
	))
	_, evs = progRun(t, rt2, progBody())
	if len(segmentEvents(evs)) != 0 || len(eventsNamed(evs, "error")) != 1 {
		t.Fatalf("a lesson line reset or counted in the run: %+v", evs)
	}
	// And one invalid line each side of a lesson line is not a run.
	rt3, _ := radioRuntime(t, linesScript(titled("bad1", map[string]any{"p_heard": 9}), `{"kind":"lesson","text":"x","cites":["air:ev-1","air:ev-2"]}`, titled("bad2", map[string]any{"p_heard": 9}), titled("fine", nil)))
	_, evs = progRun(t, rt3, progBody())
	if strings.Join(segmentTitles(evs), "|") != "fine" || len(eventsNamed(evs, "done")) != 1 {
		t.Fatalf("two invalid lines around a lesson line: %+v", evs)
	}
}

func TestF22_Cites(t *testing.T) {
	runLineCases(t, []lineCase{
		{"outside the set", titled("a", map[string]any{"cites": []any{"pub:unknown-9@v1"}}), false},
		{"unversioned", titled("b", map[string]any{"cites": []any{"pub:harbour-1"}}), false},
		{"another version", titled("c", map[string]any{"cites": []any{"pub:harbour-1@v2"}}), false},
		{"one good one bad", titled("d", map[string]any{"cites": []any{"pub:harbour-1@v1", "pub:nope@v1"}}), false},
		{"an object with an unknown ref", titled("e", map[string]any{"cites": []any{map[string]any{"ref": "pub:nope@v1"}}}), false},
		{"an object with no ref", titled("f", map[string]any{"cites": []any{map[string]any{"label": "x"}}}), false},
		{"an empty ref", titled("g", map[string]any{"cites": []any{""}}), false},
		{"no cites on a story", titled("h", map[string]any{"cites": []any{}}), false},
		{"cites absent on a story", titled("i", map[string]any{"cites": nil}), false},
		{"an unknown role", titled("j", map[string]any{"cites": []any{map[string]any{"ref": "pub:harbour-1@v1", "role": "boss"}}}), false},
		{"a label with a newline", titled("k", map[string]any{"cites": []any{map[string]any{"ref": "pub:harbour-1@v1", "label": "a\nb"}}}), false},
		{"cites not an array", titled("l", map[string]any{"cites": "pub:harbour-1@v1"}), false},
		{"a mail ref with no mail candidates", titled("m", map[string]any{"kind": "bridge", "p_heard": nil, "caused_by": nil, "cites": []any{"mail:abc"}}), false},
		{"an ident citing outside the set", titled("m2", map[string]any{"kind": "ident", "p_heard": nil, "caused_by": nil, "cites": []any{"pub:unknown-9@v1"}}), false},
		{"a candidate cite", titled("n", map[string]any{"cites": []any{"pub:harbour-1@v1", "pub:budget-3@v1"}}), true},
		{"a cite object", titled("o", map[string]any{"cites": []any{map[string]any{"ref": "pub:budget-3@v1", "role": "mentioned", "label": "Budget"}}}), true},
		{"headlines", titled("p", map[string]any{"kind": "headlines", "cites": []any{"pub:harbour-1@v1", "pub:harbour-2@v1"}}), true},
	}, nil)
}

func TestF23_ForecastFieldsOnNonContentKinds(t *testing.T) {
	plain := func(kind, title string, over map[string]any) string {
		m := map[string]any{"kind": kind, "title": title, "text": "Words to say aloud.", "p_heard": nil, "caused_by": nil, "cites": nil}
		for k, v := range over {
			m[k] = v
		}
		return progLine(m)
	}
	var cases []lineCase
	for _, kind := range []string{"ident", "bridge"} {
		cases = append(cases,
			lineCase{kind + " with p_heard", plain(kind, kind+"-p", map[string]any{"p_heard": 0.5}), false},
			lineCase{kind + " with caused_by", plain(kind, kind+"-c", map[string]any{"caused_by": "explore"}), false},
		)
	}
	cases = append(cases, lineCase{"a plain ident (an extend takes no opening)", plain("ident", "ident-ok", nil), false}, lineCase{"a plain bridge", plain("bridge", "bridge-ok", nil), true})
	rt, _ := radioRuntime(t, linesScript())
	for i, c := range cases {
		rt.provider.(*fakeRadio).script = linesScript(c.line)
		_, evs := progRun(t, rt, progBody(withRequestID("n"+string(rune('a'+i)))))
		if got := len(segmentEvents(evs)); (got == 1) != c.keep {
			t.Errorf("%s: %d segments, keep=%v", c.name, got, c.keep)
		}
		if len(stretchClaims(t, rt, stretchIDOf(t, evs))) != 0 {
			t.Errorf("%s: a claim for a non-content kind", c.name)
		}
	}
}

func TestF24_KindTextAndTitle(t *testing.T) {
	long := strings.Repeat("w ", 4600) // over every kind's range
	runLineCases(t, []lineCase{
		{"unknown kind advert", titled("a", map[string]any{"kind": "advert"}), false},
		{"kind lesson as a segment", titled("a2", map[string]any{"kind": "lesson"}), false},
		{"kind not a string", titled("b", map[string]any{"kind": 3}), false},
		{"no kind", titled("c", map[string]any{"kind": nil}), false},
		{"blank text", titled("d", map[string]any{"text": "  \n "}), false},
		{"no text", titled("e", map[string]any{"text": nil}), false},
		{"text not a string", titled("f", map[string]any{"text": 4}), false},
		{"text over the range", titled("g", map[string]any{"text": long}), false},
		{"headlines over the range", titled("g2", map[string]any{"kind": "headlines", "text": long}), false},
		{"no title on a story", progLine(map[string]any{"title": nil}), false},
		{"blank title on a story", progLine(map[string]any{"title": "   "}), false},
		{"newline in a title", titled("a\nb", nil), false},
		{"a tab in a title", titled("a\tb", nil), false},
		{"a control character", titled("a\u0007b", nil), false},
		{"over 200 bytes", titled(strings.Repeat("x", 201), nil), false},
		{"exactly 200 bytes", titled(strings.Repeat("y", 200), nil), true},
		{"padded title is trimmed", titled("  padded  ", nil), true},
		{"the update kind", titled("u", map[string]any{"kind": "update"}), true},
		{"extra unknown fields are ignored", titled("x", map[string]any{"urgency": "now", "mood": "calm"}), true},
	}, nil)
}

func TestF25_TheLineReader(t *testing.T) {
	l1, l2, l3 := titled("one", nil), titled("two", nil), titled("three", nil)
	// A line split across deltas, blank lines, fence lines, CRLF, and a last
	// line with no newline.
	rt, _ := radioRuntime(t, deltaScript("```json\n", l1[:10], l1[10:], "\n\n  \n", l2+"\r\n", "```\n", l3))
	_, evs := progRun(t, rt, progBody())
	if got := segmentTitles(evs); strings.Join(got, "|") != "one|two|three" {
		t.Fatalf("(F25) titles %q events %+v", got, evs)
	}
	// Not an object: invalid, and the run of three counts them.
	rt2, _ := radioRuntime(t, linesScript(`not json`, `[1,2]`, `"text"`, titled("never", nil)))
	_, evs = progRun(t, rt2, progBody())
	if len(segmentEvents(evs)) != 0 || len(eventsNamed(evs, "error")) != 1 {
		t.Fatalf("(F25) three non-object lines: %+v", evs)
	}
	// A line over 64 KB is invalid, not a crash and not a stuck buffer.
	huge := titled("huge", map[string]any{"text": strings.Repeat("a", 70<<10)})
	rt3, _ := radioRuntime(t, linesScript(huge, titled("after huge", nil)))
	_, evs = progRun(t, rt3, progBody())
	if got := segmentTitles(evs); strings.Join(got, "|") != "after huge" {
		t.Fatalf("(F25) after a huge line: %q", got)
	}
	// A huge line with no newline at all must not grow without bound: it
	// ends invalid at the end of the stream.
	rt4, _ := radioRuntime(t, deltaScript(strings.Repeat("z", 200<<10)))
	_, evs = progRun(t, rt4, progBody())
	if len(segmentEvents(evs)) != 0 {
		t.Fatalf("(F25) a huge unterminated line streamed")
	}
}

func TestF26_TheRunOfThreeInvalidLines(t *testing.T) {
	bad := func(n string) string { return titled(n, map[string]any{"p_heard": 9}) }
	// The fixture's two cases, then the rest of the list.
	rt, _ := radioRuntime(t, linesScript(bad("1"), titled("v1", nil), bad("2"), titled("v2", nil), bad("3")))
	_, evs := progRun(t, rt, progBody())
	if got := segmentTitles(evs); strings.Join(got, "|") != "v1|v2" || len(eventsNamed(evs, "done")) != 1 || len(eventsNamed(evs, "error")) != 0 {
		t.Fatalf("(F26) three scattered: titles %q, events %+v", got, evs)
	}
	rt2, fake2 := radioRuntime(t, linesScript(titled("v1", nil), bad("1"), bad("2"), bad("3"), titled("never", nil), titled("never2", nil)))
	_, evs = progRun(t, rt2, progBody())
	sid := stretchIDOf(t, evs)
	if got := segmentTitles(evs); strings.Join(got, "|") != "v1" || len(eventsNamed(evs, "error")) != 1 || len(eventsNamed(evs, "done")) != 0 {
		t.Fatalf("(F26) a run of three: titles %q, events %+v", got, evs)
	}
	if n := len(stretchClaims(t, rt2, sid)); n != 1 {
		t.Fatalf("(F26) %d claims", n)
	}
	idx := getRecord(t, rt2, "autoradio-stretch:"+sid)
	if idx == nil || idx.SubjectRefs["status"] != "invalid_run" || idx.SubjectRefs["dropped_count"] != "3" {
		t.Fatalf("(F26) index %+v", idx)
	}
	if fake2.callCount() != 1 {
		t.Fatalf("(F26) the call walked on after the run: %d calls", fake2.callCount())
	}
	// The run is of any kind of invalidity, and a valid line resets it; blank
	// lines neither count nor reset.
	rt3, _ := radioRuntime(t, linesScript(bad("1"), "", bad("2"), "", `nope`, titled("never", nil)))
	_, evs = progRun(t, rt3, progBody())
	if len(segmentEvents(evs)) != 0 || len(eventsNamed(evs, "error")) != 1 {
		t.Fatalf("(F26) mixed kinds of invalid: %+v", evs)
	}
	rt4, _ := radioRuntime(t, linesScript(bad("1"), bad("2"), titled("reset", nil), bad("3"), bad("4"), titled("v", nil)))
	_, evs = progRun(t, rt4, progBody())
	if strings.Join(segmentTitles(evs), "|") != "reset|v" || len(eventsNamed(evs, "done")) != 1 {
		t.Fatalf("(F26) reset: %+v", evs)
	}
	// A transition is a valid line and resets the run too.
	transition := progLine(map[string]any{"kind": "transition", "title": "", "text": "Now, closer to home.", "p_heard": nil, "caused_by": nil, "cites": nil})
	rt5, _ := radioRuntime(t, linesScript(bad("1"), bad("2"), transition, bad("3"), titled("v", nil)))
	_, evs = progRun(t, rt5, progBody())
	if len(eventsNamed(evs, "done")) != 1 {
		t.Fatalf("(F26) transition reset: %+v", evs)
	}
}

// F27 is covered with F15 (the reasons are in the index).

func TestF43_TheL1aFixtureRunsThroughTheHandler(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "mobile", "fixtures", "learning", "stretch_lines.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		SegmentLines []struct {
			Name  string          `json:"name"`
			Valid bool            `json:"valid"`
			Line  json.RawMessage `json:"line"`
		} `json:"segment_lines"`
		Runs []struct {
			Name         string   `json:"name"`
			Sequence     []string `json:"sequence"`
			EndsAfterIdx *int     `json:"ends_after_index"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	// The fixture's input names candidates by an unversioned ref ("pub:harbour-1")
	// and design 12.7 gives a candidate a version; the harness maps each ref to
	// version 1 and does not edit the fixture.
	fixCands := func(m map[string]any) {
		m["candidates"] = []any{
			map[string]any{"route": "harbour-1", "version": 1, "title": "Harbour 1", "dek": "d"},
			map[string]any{"route": "harbour-2", "version": 1, "title": "Harbour 2", "dek": "d"},
			map[string]any{"route": "budget-3", "version": 1, "title": "Budget 3", "dek": "d"},
		}
	}
	version := func(line string) string {
		for _, r := range []string{"harbour-1", "harbour-2", "budget-3"} {
			line = strings.ReplaceAll(line, `"pub:`+r+`"`, `"pub:`+r+`@v1"`)
		}
		return line
	}
	rt, fake := radioRuntime(t, linesScript())
	seedInputs(t, rt, fake)
	for i, l := range fx.SegmentLines {
		var v any
		if err := json.Unmarshal(l.Line, &v); err != nil {
			t.Fatal(err)
		}
		compact, _ := json.Marshal(v)
		line := version(string(compact))
		fake.script = linesScript(`{"kind":"bridge","text":"marker before."}`, line, `{"kind":"bridge","text":"marker after."}`)
		_, evs := progRun(t, rt, progBody(fixCands, withRequestID("fx-"+strings.Repeat("a", i/26)+string(rune('a'+i%26)))))
		got := len(segmentEvents(evs)) - 2
		if (got == 1) != l.Valid {
			t.Errorf("(F43) %q: valid=%v, but %d segments were streamed for it", l.Name, l.Valid, got)
		}
	}
	for _, run := range fx.Runs {
		var lines []string
		for i, s := range run.Sequence {
			if s == "valid" {
				lines = append(lines, titled("v"+string(rune('a'+i)), nil))
			} else {
				lines = append(lines, titled("i"+string(rune('a'+i)), map[string]any{"p_heard": 9}))
			}
		}
		fake.script = linesScript(lines...)
		_, evs := progRun(t, rt, progBody(withRequestID("run-"+strings.ReplaceAll(run.Name, " ", "-"))))
		ended := len(eventsNamed(evs, "error")) == 1
		if ended != (run.EndsAfterIdx != nil) {
			t.Errorf("(F43) run %q: ended=%v, fixture says %v", run.Name, ended, run.EndsAfterIdx)
		}
		if run.EndsAfterIdx != nil {
			// The fixture's index counts the lines read when the call ended.
			valid := 0
			for _, s := range run.Sequence[:*run.EndsAfterIdx] {
				if s == "valid" {
					valid++
				}
			}
			if got := len(segmentEvents(evs)); got != valid {
				t.Errorf("(F43) run %q: %d segments before the end, want %d", run.Name, got, valid)
			}
		}
	}
}

// ---- wiring and boundaries (F40, F41, F42) ----

// The production provider is a TextStreamProvider, so the handler is not 503
// in production; the handler's default ledger is the real store. Both are
// asserted at compile time.
var (
	_ provideriface.TextStreamProvider = (*gatewayruntime.Provider)(nil)
	_ autoradioLedger                  = (*store.Store)(nil)
)

func TestF40_TheProductionWiring(t *testing.T) {
	rt, _ := radioRuntime(t, twoStoryScript())
	h := NewAPIHandler(rt)
	if got := h.programmeLedger(); got != autoradioLedger(rt.store) {
		t.Fatalf("(F40) the default ledger is %T, not the runtime's store", got)
	}
	// The fake provider has no capability the gateway's lacks: only the
	// TextStreamProvider surface is used by the handler.
	var p provideriface.Provider = rt.provider
	if _, ok := p.(provideriface.TextStreamProvider); !ok {
		t.Fatalf("(F40) the test provider is not a TextStreamProvider")
	}
}

func TestF41_TheProgrammeHandlerHasNoPathToAnOutcome(t *testing.T) {
	files, _ := filepath.Glob("api_autoradio_programme*.go")
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		scanned++
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				switch x.Sel.Name {
				case "SettleAutoradio", "NewAutoradioSettlementRecord", "AutoradioHeardRule":
					t.Errorf("(F41) %s uses %s", f, x.Sel.Name)
				}
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					for _, bad := range []string{"autoradio.air", "autoradio.settlement", "autoradio.conflict", "autoradio-air:", "autoradio_heard"} {
						if strings.Contains(strings.ToLower(x.Value), bad) {
							t.Errorf("(F41) %s holds the literal %s", f, x.Value)
						}
					}
				}
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("(F41) no programme source files found")
	}
}

func TestF42_EveryRecordIsLabelledPrivateAndUnaddressed(t *testing.T) {
	rt, _ := radioRuntime(t, twoStoryScript())
	_, evs := progRun(t, rt, progBody())
	sid := stretchIDOf(t, evs)
	seen := map[string]int{}
	for _, row := range types.AutoradioRecordKinds {
		for _, rec := range tape(t, rt, row.Kind) {
			seen[row.Kind]++
			label, _ := types.AutoradioKindLabel(row.Kind)
			if !types.NewLabelSet(rec.Provenance.Labels...).Has(label) {
				t.Errorf("(F42) %s lacks its label %s", rec.RecordID, label)
			}
			if rec.Addressee != "" {
				t.Errorf("(F42) %s is addressed to %q", rec.RecordID, rec.Addressee)
			}
			if rec.Kind == types.CommitmentKindDirective && (rec.Directive == nil || rec.Directive.Subtype != types.CommitmentDirectiveNotice) {
				t.Errorf("(F42) %s is a directive but not a notice", rec.RecordID)
			}
		}
	}
	for _, kind := range []string{"autoradio.request", "autoradio.stretch_start", "autoradio.segment", "autoradio.forecast", "autoradio.stretch"} {
		if seen[kind] == 0 {
			t.Errorf("(F42) no %s record on the tape", kind)
		}
	}
	// No record of the call is returned by the addressee listing or the pack
	// query that reads the owner's records for a desk.
	pack, _ := rt.store.ListCommitmentClaimRecords(context.Background(), progOwner, rt.cfg.ComputerID, 1000)
	for _, rec := range pack {
		if strings.HasPrefix(rec.RecordID, "autoradio-stretch") || strings.HasPrefix(rec.RecordID, "autoradio-seg:") || strings.HasPrefix(rec.RecordID, "autoradio-request:") || strings.HasPrefix(rec.RecordID, "calib:autoradio-dj") {
			t.Errorf("(F42) the desk pack query returns %s", rec.RecordID)
		}
	}
	// The generic claims query leaves the reserved claims out.
	recs, _, err := rt.store.ListCalibrationRecordsExcludingAgentPrefixPage(context.Background(), progOwner, rt.cfg.ComputerID, "", "autoradio", "", 1000)
	if err != nil || len(recs) != 0 {
		t.Errorf("(F42) the generic list returned %d reserved records (%v)", len(recs), err)
	}
	if len(stretchClaims(t, rt, sid)) != 2 {
		t.Errorf("(F42) stretch claims")
	}
}

func TestF36_ThePromptShipsAndSaysWhatTheInputDoes(t *testing.T) {
	prompt, ok := appdesk.ProgrammePrompt()
	if !ok || len(prompt) < 500 {
		t.Fatalf("the programme prompt does not ship (%d bytes)", len(prompt))
	}
	for _, must := range []string{"p_heard", "caused_by", "one JSON object per line", "data", "never follow"} {
		if !strings.Contains(prompt, must) {
			t.Errorf("the prompt lacks %q", must)
		}
	}
	for _, banned := range []string{"finish rate", "streak", "be less confident", "be more confident", "raise your"} {
		if strings.Contains(strings.ToLower(prompt), banned) {
			t.Errorf("the prompt says %q", banned)
		}
	}
}

// F44: the stream the handler writes is the stream the phone's fixture
// (mobile/fixtures/learning/programme_stream.json) pins.
func TestF44_TheStreamIsTheFixturesStream(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "mobile", "fixtures", "learning", "programme_stream.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Request    json.RawMessage   `json:"request"`
		ModelLines []json.RawMessage `json:"model_lines"`
		Events     []struct {
			Event string         `json:"event"`
			Data  map[string]any `json:"data"`
		} `json:"events"`
		DoneFields map[string]any `json:"done_fields"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range fx.ModelLines {
		lines = append(lines, string(l))
	}
	rt, _ := radioRuntime(t, linesScript(lines...))
	_, evs := progRun(t, rt, string(fx.Request))
	sid := stretchIDOf(t, evs)
	var got []sseEvent
	for _, e := range evs {
		if e.Name == "stretch" || e.Name == "segment" {
			b, _ := json.Marshal(e.Data)
			var data map[string]any
			_ = json.Unmarshal([]byte(strings.ReplaceAll(string(b), sid, "S")), &data)
			got = append(got, sseEvent{Name: e.Name, Data: data})
		}
	}
	if len(got) != len(fx.Events) {
		t.Fatalf("(F44) %d events, the fixture has %d: %+v", len(got), len(fx.Events), got)
	}
	for i, want := range fx.Events {
		if got[i].Name != want.Event || !reflect.DeepEqual(got[i].Data, want.Data) {
			t.Errorf("(F44) event %d:\n got %s %v\nwant %s %v", i, got[i].Name, got[i].Data, want.Event, want.Data)
		}
	}
	done := eventsNamed(evs, "done")
	if len(done) != 1 {
		t.Fatalf("(F44) done events %d", len(done))
	}
	for k, want := range fx.DoneFields {
		g := done[0].Data[k]
		if s, ok := g.(string); ok {
			g = strings.ReplaceAll(s, sid, "S")
		}
		if !reflect.DeepEqual(g, want) {
			t.Errorf("(F44) done.%s = %v, want %v", k, g, want)
		}
	}
}
