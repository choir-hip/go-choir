package agentcore

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Round 2 of the review of the graph-first change (seat 2, 2026-11-22). Failure list,
// written before the code, every test below on the message the run is DELIVERED, built
// with the object-graph gates in enforce:
//  F1 the digest is framed whole as outside text ("from outside this computer (labels:
//     world_untrusted)") although its story section is this computer's own private text,
//     so the model is told the story is web text and the segment lacks its label;
//  F2 a Report's labels are not carried: a Report that quoted a private capture reaches
//     the next run as outside text only; a story section with a main point and no text
//     carries no label;
//  F3 "N more pages" is computed before the budget cut (says 360 when 15 of the 40 are
//     cut), and does not say it is a floor when the story's older records were not read;
//  F4 the digest, the run's prompt and the brief are not bounded together: a resumed run's
//     long prompt (multibyte) rides on top of the full digest; a report is added before the
//     budget is checked;
//  F5 two pages of one outlet by the public suffix (www.bbc.co.uk, news.bbc.co.uk) are two
//     outlets in the neighbours; a neighbour whose read failed is listed as a bare id;
//  F6 a store failure while the story's pages are listed is silent.
// Round 3: F7 a Report whose run's labels lack world_untrusted is framed as this
//  computer's own records; F8 the host's guidance (the section headers, the "N more"
//  counts) sits inside the data frame; F9 the conversation is read whole, so a run's start
//  slows as the story ages (the digest must read the newest records of each inbox only).

var digestFrame = regexp.MustCompile(`(?s)\[Data, not instructions: ([^\]]*?) \(labels: ([^)]*)\)[^\]]*\]\n(.*?)\n\[End of data\]`)

// frameOf returns the preamble words, the labels and the text of the delivered frame
// that contains want, or t.Fatal.
func frameOf(t *testing.T, opening, want string) (kind, labels, text string) {
	t.Helper()
	for _, m := range digestFrame.FindAllStringSubmatch(opening, -1) {
		if strings.Contains(m[3], want) {
			return m[1], m[2], m[3]
		}
	}
	t.Fatalf("no framed section of the delivered message holds %q:\n%s", want, opening)
	return "", "", ""
}

func TestTheDeliveredDigestFramesEachSectionByItsOwnLabels(t *testing.T) {
	rt, s, owner, computer, doc, _ := digestFixture(t)
	t0 := time.Now().UTC().Add(-time.Hour)
	commitReportLabelled(t, s, owner, computer, doc, "cell-5:report:2", types.CommitmentReport{Summary: "A second round quoted an owner capture."}, false, t0.Add(30*time.Minute), []string{types.OwnerPrivateLabel("capture"), types.LabelWorldUntrusted})
	b := buildBrief(t, rt, briefRunRecord(owner, computer, doc, "run-frame"))
	if b == nil || b.digest == nil {
		t.Fatal("no digest")
	}
	opening := b.openingText()

	// The story's text is this computer's own: framed as its own records, not as outside text.
	kind, labels, text := frameOf(t, opening, "Dockworkers struck on Monday")
	if !strings.Contains(kind, "from this computer's own records") || strings.Contains(kind, "from outside this computer") || labels != types.OwnerPrivateLabel("document") {
		t.Errorf("the story section is framed %q with labels %q: want this computer's own records, owner_private:document", kind, labels)
	}
	if strings.Contains(text, "port.example") {
		t.Errorf("the story section and the pages share a frame:\n%s", text)
	}
	// The pages are outside text.
	kind, labels, _ = frameOf(t, opening, "capture_id c1 https://port.example/1")
	if !strings.Contains(kind, "from outside this computer") || labels != types.LabelWorldUntrusted {
		t.Errorf("the pages are framed %q with labels %q: want outside text, world_untrusted", kind, labels)
	}
	// A Report carries its run's labels: a private capture quoted in it makes its section
	// say it is partly this computer's own; an unlabelled Report is read as outside text.
	kind, labels, _ = frameOf(t, opening, "A second round quoted an owner capture.")
	if !strings.Contains(kind, "partly from outside this computer, partly from this computer's own records") || !strings.Contains(labels, "owner_private:capture") || !strings.Contains(labels, "world_untrusted") {
		t.Errorf("the reports are framed %q with labels %q", kind, labels)
	}
	// What the run is marked with is the union of every section's.
	for _, want := range []string{types.OwnerPrivateLabel("document"), types.OwnerPrivateLabel("capture"), types.LabelWorldUntrusted} {
		if !b.digest.labels.Has(want) {
			t.Errorf("the run is not marked %s: %v", want, b.digest.labels)
		}
	}
}

func TestAnUnlabelledReportIsReadAsOutsideTextAndAMainPointAloneIsLabelledPrivate(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-frame2", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Main only", "A main point with no text yet", "", nil, t0)
	// The Cast carries a main point and NO text and no labels (an old or short Cast).
	commitCastWithStory(t, s, owner, computer, doc, "cell-3:cast:r1", &types.CastStory{Title: "Main only", MainPoint: "A main point with no text yet"}, t0.Add(time.Minute))
	commitReport(t, s, owner, computer, doc, "cell-4:report:1", types.CommitmentReport{Summary: "An unlabelled Report."}, false, t0.Add(2*time.Minute))
	rt.researchDeps = &researchtools.Dependencies{Captures: digestSink{ctxSink: &ctxSink{urls: map[string]string{}}, text: map[string]string{}}}
	b := buildBrief(t, rt, briefRunRecord(owner, computer, doc, "run-main"))
	opening := b.openingText()
	kind, labels, _ := frameOf(t, opening, "Main point: A main point with no text yet")
	if !strings.Contains(kind, "from this computer's own records") || labels != types.OwnerPrivateLabel("document") {
		t.Errorf("a main point shown alone is framed %q with labels %q, want owner_private:document", kind, labels)
	}
	kind, labels, _ = frameOf(t, opening, "An unlabelled Report.")
	if !strings.Contains(kind, "from outside this computer") || labels != types.LabelWorldUntrusted {
		t.Errorf("an unlabelled Report is framed %q with labels %q, want outside text", kind, labels)
	}
}

// manyPagesStory opens a story that holds n captures, each with a long passage, and
// returns the runtime and the doc.
func manyPagesStory(t *testing.T, n int, extraRecords int) (*Runtime, string, string, string) {
	t.Helper()
	rt, s := testRuntime(t)
	owner, computer := "owner-count", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-2 * time.Hour)
	sink := digestSink{ctxSink: &ctxSink{urls: map[string]string{}}, text: map[string]string{}}
	var ids []string
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("p%03d", i)
		ids = append(ids, id)
		sink.urls[id], sink.text[id] = fmt.Sprintf("https://p%03d.example/x", i), strings.Repeat(fmt.Sprintf("passage %s words. ", id), 60)
	}
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Many pages", "A story that holds many pages", "", ids, t0)
	for i := 0; i < extraRecords; i++ {
		id := fmt.Sprintf("x%03d", i)
		sink.urls[id], sink.text[id] = fmt.Sprintf("https://x%03d.example/x", i), "extra "+id
		feedCastAt(t, s, owner, computer, doc, fmt.Sprintf("cell-f:cast:f%d", i), []string{id}, t0.Add(time.Duration(i+1)*time.Second))
	}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	return rt, owner, computer, doc
}

var moreCount = regexp.MustCompile(`\((at least )?(\d+) more pages are held and not listed here`)

func TestTheMorePagesCountIsTakenAfterTheBudgetCut(t *testing.T) {
	rt, owner, computer, doc := manyPagesStory(t, 400, 0)
	rec := briefRunRecord(owner, computer, doc, "run-count")
	// A prompt that leaves the digest room for only some of the 40 pages it selected.
	rec.Prompt = strings.Repeat("x", (researchDigestMaxTokens-1500)*4)
	d := buildDigest(t, rt, rec)
	if d == nil {
		t.Fatal("no digest")
	}
	shown := d.kinds["passages"] + d.kinds["pointers"]
	if shown == 0 || shown >= storyContextMaxPages {
		t.Fatalf("the budget did not cut the 40 selected pages (%d shown): the test proves nothing", shown)
	}
	m := moreCount.FindStringSubmatch(d.text)
	if m == nil {
		t.Fatalf("no 'more pages' line:\n%s", d.text)
	}
	if m[1] != "" {
		t.Fatalf("the story's records fit the window: the count is exact, not a floor: %q", m[0])
	}
	var got int
	fmt.Sscanf(m[2], "%d", &got)
	if got != 400-shown {
		t.Fatalf("the digest says %d more pages, but it holds 400 and lists %d: want %d", got, shown, 400-shown)
	}
}

func TestTheMorePagesCountIsAFloorWhenTheStorysOlderRecordsAreNotRead(t *testing.T) {
	rt, owner, computer, doc := manyPagesStory(t, 5, storyMaterialMaxRecords+5)
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-window"))
	if d == nil {
		t.Fatal("no digest")
	}
	m := moreCount.FindStringSubmatch(d.text)
	if m == nil || m[1] == "" {
		t.Fatalf("a story with records past the window must say 'at least':\n%s", d.text)
	}
}

func TestTheDigestIsBuiltToFitWhatTheRunsPromptLeaves(t *testing.T) {
	rt, s, owner, computer, doc, _ := digestFixture(t)
	t0 := time.Now().UTC().Add(-time.Hour)
	// Six long multibyte Reports and a long multibyte story text: far more than the room.
	for i := 0; i < 6; i++ {
		commitReport(t, s, owner, computer, doc, fmt.Sprintf("cell-r:report:%d", i), types.CommitmentReport{
			Summary: strings.Repeat("研究報告", 400), Notes: []string{strings.Repeat("注", 600)},
		}, false, t0.Add(time.Duration(30+i)*time.Minute))
	}
	commitCastWithStory(t, s, owner, computer, doc, "cell-3:cast:big", &types.CastStory{Title: "Dock strike", MainPoint: "Whether the dock strike ends",
		Text: strings.Repeat("港", 7000), Labels: []string{types.OwnerPrivateLabel("document")}}, t0.Add(25*time.Minute))
	for _, promptTokens := range []int{0, 4000, 9000, 11900, 30000} {
		rec := briefRunRecord(owner, computer, doc, "run-fit")
		rec.Prompt = strings.Repeat("目", promptTokens*4/3) // 3 bytes a rune: promptTokens*4 bytes
		room := researchDigestMaxTokens - estimateTextTokens(rec.Prompt)
		d := buildDigest(t, rt, rec)
		if d == nil {
			t.Fatalf("prompt %d tokens: no digest value", promptTokens)
		}
		got := estimateTextTokens(d.openingText())
		if room < 0 {
			room = 0
		}
		if got > room {
			t.Errorf("prompt %d tokens: the delivered digest is %d estimated tokens, the room is %d", promptTokens, got, room)
		}
		if estimateTextTokens(rec.Prompt)+got > researchDigestMaxTokens && estimateTextTokens(rec.Prompt) < researchDigestMaxTokens {
			t.Errorf("prompt %d tokens: prompt and digest together pass %d", promptTokens, researchDigestMaxTokens)
		}
		if d.tokens != got && d.text != "" {
			t.Errorf("prompt %d tokens: the digest says %d tokens, the delivered text is %d", promptTokens, d.tokens, got)
		}
	}
}

func TestTheNeighboursKeepOneCapturePerOutletByThePublicSuffixAndDropAFailedRead(t *testing.T) {
	rt, _, owner, computer, doc, sink := digestFixture(t)
	sink.neighbours = map[string][]researchtools.CaptureNeighbour{"c1": {
		{CaptureID: "n1", Score: 0.95}, {CaptureID: "n2", Score: 0.94}, {CaptureID: "n3", Score: 0.93},
		{CaptureID: "n4", Score: 0.92}, {CaptureID: "n5", Score: 0.91}, {CaptureID: "n6", Score: 0.90}}}
	for id, url := range map[string]string{
		"n1": "https://www.bbc.co.uk/a", "n2": "https://news.bbc.co.uk/b", "n3": "https://www.theguardian.co.uk/c",
		"n4": "https://news.google.com/d", "n5": "https://news.google.com/e"} {
		sink.urls[id], sink.text[id] = url, "passage of "+id
	}
	sink.gone = map[string]bool{"n6": true}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-outlet"))
	if d == nil {
		t.Fatal("no digest")
	}
	if !strings.Contains(d.text, "capture_id n1 ") || strings.Contains(d.text, "capture_id n2") {
		t.Errorf("www.bbc.co.uk and news.bbc.co.uk are one outlet: the best (n1) is kept, n2 is not:\n%s", d.text)
	}
	if !strings.Contains(d.text, "capture_id n3 ") {
		t.Errorf("theguardian.co.uk is another outlet than bbc.co.uk:\n%s", d.text)
	}
	if !strings.Contains(d.text, "capture_id n4 ") || !strings.Contains(d.text, "capture_id n5 ") {
		t.Errorf("an aggregator's pages belong to no outlet and are both kept:\n%s", d.text)
	}
	if strings.Contains(d.text, "capture_id n6") {
		t.Errorf("a neighbour whose read failed is listed as a bare id:\n%s", d.text)
	}
	if !strings.Contains(d.text, "1 unreadable") {
		t.Errorf("the unreadable neighbour is not counted:\n%s", d.text)
	}
}

func TestAStoreFailureWhileListingTheStorysPagesIsDegradedAndLogged(t *testing.T) {
	rt, _, owner, computer, doc, _ := digestFixture(t)
	buf := briefCaptureLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := rt.researchDigestOpening(ctx, briefRunRecord(owner, computer, doc, "run-down"))
	if d == nil {
		t.Fatal("no digest value")
	}
	var seen bool
	for _, deg := range d.degraded {
		if deg == "story_material" {
			seen = true
		}
	}
	if !seen || !strings.Contains(buf.String(), "the story's records were not readable") {
		t.Fatalf("the story's records failed silently: degraded %v, log %q", d.degraded, buf.String())
	}
}

func TestAReportWhoseRunLabelsLackWorldUntrustedIsStillFramedAsOutsideText(t *testing.T) {
	rt, s, owner, computer, doc, _ := digestFixture(t)
	t0 := time.Now().UTC().Add(-time.Hour)
	// A research Report is built from outside text whatever labels its run carried: this
	// one's run was marked with a private capture alone.
	commitReportLabelled(t, s, owner, computer, doc, "cell-5:report:3", types.CommitmentReport{Summary: "A round marked with a private capture only."}, false, t0.Add(40*time.Minute), []string{types.OwnerPrivateLabel("capture")})
	b := buildBrief(t, rt, briefRunRecord(owner, computer, doc, "run-nolabel"))
	kind, labels, _ := frameOf(t, b.openingText(), "A round marked with a private capture only.")
	if !strings.Contains(kind, "partly from outside this computer") || !strings.Contains(labels, "world_untrusted") || !strings.Contains(labels, "owner_private:capture") {
		t.Fatalf("the Report is framed %q with labels %q: want outside and own, world_untrusted and owner_private:capture", kind, labels)
	}
}

func TestTheHostsGuidanceSitsOutsideTheDataFrame(t *testing.T) {
	// A story with a main point, a text, an earlier Report, 45 pages (five past the
	// listing) and neighbours of which some are left out: every section has its guidance.
	rt, s, owner, computer, doc, sink := digestFixture(t)
	t0 := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 45; i++ {
		id := fmt.Sprintf("q%02d", i)
		feedCastAt(t, s, owner, computer, doc, fmt.Sprintf("cell-q:cast:%s", id), []string{id}, t0.Add(time.Duration(30+i)*time.Minute))
		sink.urls[id], sink.text[id] = fmt.Sprintf("https://q%02d.example/x", i), "passage of "+id
	}
	sink.neighbours = map[string][]researchtools.CaptureNeighbour{"q44": {{CaptureID: "n1", Score: 0.95}, {CaptureID: "n2", Score: 0.94}}}
	sink.urls["n1"], sink.text["n1"] = "https://www.bbc.co.uk/a", "passage of n1"
	sink.urls["n2"], sink.text["n2"] = "https://news.bbc.co.uk/b", "passage of n2" // the same outlet: left out, counted
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	b := buildBrief(t, rt, briefRunRecord(owner, computer, doc, "run-guide"))
	opening := b.openingText()
	for _, guidance := range []string{
		"[This story]", "(story " + doc, "[Pages this story holds]", "[Earlier research on this story]",
		"more pages are held and not listed here", "[Nearby in the graph]", "more near captures",
	} {
		if !strings.Contains(opening, guidance) {
			t.Fatalf("the message lacks the guidance %q:\n%s", guidance, opening)
		}
		for _, m := range digestFrame.FindAllStringSubmatch(opening, -1) {
			if strings.Contains(m[3], guidance) {
				t.Errorf("the host's guidance %q is inside a data frame (%q):\n%s", guidance, m[1], m[3])
			}
		}
	}
	// The data itself is inside its frame: the story's title and text, the pages, the Report.
	for _, data := range []string{"Title: Dock strike", "Dockworkers struck on Monday", "capture_id q44", "Talks resumed on Tuesday", "capture_id n1"} {
		inside := false
		for _, m := range digestFrame.FindAllStringSubmatch(opening, -1) {
			inside = inside || strings.Contains(m[3], data)
		}
		if !inside {
			t.Errorf("%q is not inside a data frame:\n%s", data, opening)
		}
	}
}

// A story's age does not slow a run's start: the digest reads the newest records of the
// research inbox (the Texture's Casts) and of the Texture's inbox (the Reports), by
// cursor. The newest are always inside the windows; an older Cast or Report is not read.
func TestTheDigestReadsOnlyTheNewestRecordsOfAStorysInboxes(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-window", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-3 * time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Old story", "A story with a long history", "", nil, t0)
	rt.researchDeps = &researchtools.Dependencies{Captures: digestSink{ctxSink: &ctxSink{urls: map[string]string{}}, text: map[string]string{}}}

	// The oldest Cast carries a story; storyCastWindow Casts without one come after it.
	commitCastWithStory(t, s, owner, computer, doc, "cell-2:cast:old", &types.CastStory{Title: "Old story", Text: "THE OLDEST TEXT", Labels: []string{types.OwnerPrivateLabel("document")}}, t0.Add(time.Minute))
	for i := 0; i < storyCastWindow; i++ {
		researchCastAt(t, s, owner, computer, doc, fmt.Sprintf("cell-3:cast:n%03d", i), t0.Add(time.Duration(i+2)*time.Minute))
	}
	// An old Report, then more Texture-inbox records than the window holds.
	commitReport(t, s, owner, computer, doc, "cell-4:report:old", types.CommitmentReport{Summary: "THE OLDEST REPORT"}, false, t0.Add(90*time.Minute))
	for i := 0; i < storyMaterialMaxRecords; i++ {
		feedCastAt(t, s, owner, computer, doc, fmt.Sprintf("cell-5:cast:f%03d", i), nil, t0.Add(100*time.Minute+time.Duration(i)*time.Second))
	}
	commitReport(t, s, owner, computer, doc, "cell-6:report:new", types.CommitmentReport{Summary: "THE NEWEST REPORT"}, false, t0.Add(170*time.Minute))
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-window"))
	if d == nil {
		t.Fatal("no digest")
	}
	if strings.Contains(d.text, "THE OLDEST TEXT") || strings.Contains(d.text, "THE OLDEST REPORT") {
		t.Fatalf("the digest read records older than its windows:\n%s", d.text)
	}
	if !strings.Contains(d.text, "THE NEWEST REPORT") {
		t.Fatalf("the newest Report is outside the window:\n%s", d.text)
	}
	// A newer Cast that carries the story is always inside.
	commitCastWithStory(t, s, owner, computer, doc, "cell-7:cast:new", &types.CastStory{Title: "Old story", Text: "THE NEWEST TEXT", Labels: []string{types.OwnerPrivateLabel("document")}}, t0.Add(175*time.Minute))
	d = buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-window2"))
	if d == nil || !strings.Contains(d.text, "THE NEWEST TEXT") || strings.Contains(d.text, "THE OLDEST TEXT") {
		t.Fatalf("the newest Cast's story is not what the digest holds:\n%v", d)
	}
}
