package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The management task's story page and the candidate stories offered for an
// arrival (docs/design/semantic-layer-2026-11-20.md, landing A). Failure list,
// written before the code:
//
//  P1  the page is not what the prompt promises: a story updated inside the
//      window is missing, or carries a lede (headlines only), or a story older
//      than the window is on it.
//  P2  a full window does not fit the section bound by construction, or the
//      bound is met by a silent fallback to the newest 50.
//  P3  a bound that truncates does so quietly.
//  P4  an empty window falls back to older stories instead of saying so.
//  C1  a story that holds one of the arrival's nearest captures, or whose
//      research searches returned the arrival's page, is not offered; or a
//      story with the same title words and no such link is offered (title
//      words are gone); or a story is dropped for its age.
//  C2  candidates are not ranked by nearest captures held, then by the
//      context the trajectory holds (version), then newest; more than the
//      bound are offered; a story with a main point offers its lede instead,
//      or one without offers nothing to judge it by.
//  C3  a failed or missing neighbours request refuses an open or hides the
//      page, instead of a note beside the page.
//  J1  an open with a missing or out-of-bound main point, an unanswered
//      candidate, a "same" answer, a succeeds without a next, a next without a
//      succeeds, two nexts, a malformed judgment or no reason is accepted; or
//      a refusal does not name the shape.
//  J2  on the 22 pairs of the design's section 9, the rule's answer does not
//      fold unclear into a feed, or opens a story where the answer is same.
//  J3  an open that answers next does not write main_point and succeeds into
//      the story's trajectory, or StorySummary does not return them.
//  J4  the judgment costs a model call: a task's calls change with candidates.
//  L1  the log line leaves out the captures, the sources, the main point, the
//      successor or the answers.
//  T1  the prompt names a window, a key or a field the page does not carry.

// docs/evidence/duplicate-stories-on-the-paper-2026-11-20.md section 2.2:
// the earlier story's headline and the later story's headline, as published.
var evidencePairs = []struct{ kind, earlier, later string }{
	{"D", "Six killed in suspected gas explosion at Georgia apartment building", "Six killed in suspected gas explosion at Georgia apartment building"},
	{"D", "Michigan home health agency owner sentenced to 4 years for $1.6M Medicare fraud scheme", "Michigan home health agency owner sentenced to 4 years for $1.6M Medicare fraud scheme"},
	{"D", "Islam projected to become the world's largest religion by 2035, Pew forecasts", "Pew projects Islam will overtake Christianity as the world's largest religion by 2035"},
	{"D", "Paris 2024's economic legacy was 'very limited', France's audit court finds", "French audit finds Paris 2024 had very limited short-term economic impact"},
	{"D", "UK sanctions eight Israeli settlement firms; Australia joins, Israel vows countermeasures", "UK sanctions eight Israeli firms building West Bank settlements; Sa'ar vows countermeasure"},
	{"D", "One company wins rights to mine off American Samoa at first U.S. deep-sea mining lease auction", "US sells first deep-sea mining leases off American Samoa for $6m, days after court voided"},
	{"D", "Russia launches mass missile strike on Ukraine after Putin vows revenge", "Russia launches massive missile and drone attack on Ukraine overnight"},
	{"D", "UK sanctions eight Israeli settlement firms; Australia joins, Israel vows countermeasures", "Australia sanctions settler leader Daniella Weiss and a construction company over West Bank settlements"},
	{"D", "Indian national Nikhil Gupta sentenced to 19 years 7 months for plot to kill Sikh leader Pannun", "Indian man sentenced to 19½ years for plot to kill Sikh separatist leader in New York"},
	{"D", "Five aid groups grade Trump's Gaza peace plan 4 out of 26 and call it 'displacement by design'", "Humanitarian groups give Trump's Gaza 'peace' plan 4 out of 26 one year on"},
	{"D", "ICJ to deliver verdict on Thursday in historic genocide case against Myanmar over Rohingya", "ICJ finds Myanmar committed genocide against Rohingya in historic ruling"},
	{"D", "Matan Kahana elected Knesset speaker, giving Israel's opposition bloc control of parliament", "Matan Kahana elected speaker of Israel's Knesset as opposition bloc takes control"},
	{"D", "Ukraine confirms strikes on three oil tanks at Tatarstan pipeline station on Druzhba network", "Ukraine strikes Kaleykino Druzhba pipeline hub in Tatarstan, hitting three oil tanks"},
	{"D", "Russia reportedly planned a mass-casualty false-flag attack in Poland to be blamed on Ukraine", "Russia reportedly planned mass-casualty sabotage in Poland to be blamed on Ukraine"},
	{"F", "Eight killed in Brooklyn building collapse; city orders demolition of neighboring damaged buildings", "New video captures deadly Brooklyn building collapse that killed eight"},
	{"F", "Bukele due to register for unprecedented third El Salvador presidential term", "Bukele registers for an unprecedented third term in El Salvador"},
}

// evidenceLookAlikes are the three pairs the evidence file counted as distinct.
var evidenceLookAlikes = []struct{ earlier, later string }{
	{"Philippines charges Thomas Markle over his comments about the royal family", "US judge finds Thomas Markle fit to stand trial"},
	{"Third US carrier reaches the Middle East as tensions with Iran climb", "USS George Washington leaves Yokosuka for its patrol"},
	{"South Korea cracks down on AI cheating ahead of the college entrance exam", "South Korea sends nationwide disaster alerts on exam day"},
}

// unrelatedTitles are held stories that share nothing with the pairs above.
var unrelatedTitles = []string{
	"Lufthansa to cut jobs as it struggles to turn a profit",
	"Princeton PhD student detained by ICE after neuroscience conference",
	"Google adds an AI chat sidebar to its desktop search homepage",
	"Nvidia's board keeps Jensen Huang out, interim CEO passes first test",
	"NASA outlines revised Moon landing schedule for crewed missions",
	"Senate passes major permitting reform bill, sends it to the House",
	"Nations League draw set for Friday with England and Spain awaiting opponents",
	"Italian CGIL calls a general strike over the budget",
	"Moscow extends detention of election expert to December",
	"Germ DM launches on Android as a phone-number-free messenger",
}

func sumAt(i int, headline, lede string, age time.Duration, now time.Time) store.StorySummary {
	return store.StorySummary{DocID: fmt.Sprintf("doc-%04d-0000-0000-0000-000000000000", i), Headline: headline, Lede: lede,
		UpdatedAt: now.Add(-age), Version: 1 + i%4}
}

// windowStories builds n stories updated across the last `span`, newest
// first, with headlines of the lengths the paper's are (the evidence file's
// titles, 60 to 110 characters).
func windowStories(n int, span time.Duration, now time.Time) []store.StorySummary {
	titles := []string{}
	for _, p := range evidencePairs {
		titles = append(titles, p.earlier, p.later)
	}
	titles = append(titles, unrelatedTitles...)
	out := make([]store.StorySummary, 0, n)
	for i := 0; i < n; i++ {
		age := time.Duration(int64(span) * int64(i) / int64(n+1))
		out = append(out, sumAt(i, fmt.Sprintf("%s (%d)", titles[i%len(titles)], i), strings.Repeat("A lede that must never be on the headline page. ", 8), age+time.Minute, now))
	}
	return out
}

func TestOpeningPageHoldsTheWholeWindowAsHeadlines(t *testing.T) {
	now := time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)
	in := windowStories(330, openingWindow, now)
	// Older than the window: none of these is on the page.
	for i := 0; i < 60; i++ {
		in = append(in, sumAt(1000+i, fmt.Sprintf("Older story %d", i), "", openingWindow+time.Duration(i+1)*time.Minute, now))
	}
	page, info := buildOpeningPage(in, now, openingWindow, managementTaskOpeningSectionTokens)
	if len(page.Stories) != 330 || info.InWindow != 330 || info.Shown != 330 || info.LeftOut != 0 || page.LeftOut != 0 {
		t.Fatalf("P1: page holds %d of %d window stories (left out %d, info %+v); the prompt promises the whole window", len(page.Stories), info.InWindow, page.LeftOut, info)
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), "lede") || strings.Contains(string(raw), "must never be") || strings.Contains(string(raw), "Older story") {
		t.Fatalf("P1: the page carries a lede or an older story: %.300s", raw)
	}
	for _, key := range []string{`"inbox":"texture:doc-0000`, `"headline"`, `"age"`, `"v"`, `"window":"8h"`, `"as_of"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("P1: the page has no %s: %.300s", key, raw)
		}
	}
	tokens := estimateTextTokens(string(raw))
	if tokens > managementTaskOpeningSectionTokens {
		t.Fatalf("P2: a full window of 330 stories is %d tokens, over the bound %d", tokens, managementTaskOpeningSectionTokens)
	}
	t.Logf("measured: 330 stories (8h) = %d bytes, %d estimated tokens (%.1f a story), bound %d", len(raw), tokens, float64(tokens)/330, managementTaskOpeningSectionTokens)
	// The newest story is first, its age is read from its last version.
	if page.Stories[0].Inbox != "texture:"+in[0].DocID || page.Stories[0].Age != "1m" {
		t.Fatalf("P1: first entry = %+v", page.Stories[0])
	}
}

func TestOpeningPageTruncatesLoudlyNeverToFifty(t *testing.T) {
	now := time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)
	in := windowStories(330, openingWindow, now)
	budget := 4000
	page, info := buildOpeningPage(in, now, openingWindow, budget)
	raw, _ := json.Marshal(page)
	if got := estimateTextTokens(string(raw)); got > budget {
		t.Fatalf("P2: a page built for %d tokens is %d", budget, got)
	}
	if info.LeftOut == 0 || info.Shown+info.LeftOut != 330 || len(page.Stories) != info.Shown || page.LeftOut != info.LeftOut {
		t.Fatalf("P3: shown %d, left out %d, page says %d of 330", info.Shown, info.LeftOut, page.LeftOut)
	}
	if info.Shown == 50 || info.Shown < 50 {
		t.Fatalf("P2: %d stories shown for a 4000-token bound; the page must fill the bound, not fall back to 50", info.Shown)
	}
	for _, want := range []string{fmt.Sprint(info.LeftOut), "choir.Stories(", "not listed"} {
		if !strings.Contains(page.Note, want) {
			t.Errorf("P3: the note does not say %q: %s", want, page.Note)
		}
	}
	// What is kept is the newest; Next continues exactly after the last listed.
	last := in[info.Shown-1]
	if page.Stories[info.Shown-1].Inbox != "texture:"+last.DocID || page.Next != storiesCursor(last.UpdatedAt, last.DocID) {
		t.Fatalf("P3: truncated page keeps the wrong stories or cursor: next %q", page.Next)
	}
}

func TestOpeningPageEmptyWindowSaysSo(t *testing.T) {
	now := time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)
	var old []store.StorySummary
	for i := 0; i < 80; i++ {
		old = append(old, sumAt(i, fmt.Sprintf("Old story %d", i), "", 9*time.Hour+time.Duration(i)*time.Minute, now))
	}
	page, info := buildOpeningPage(old, now, openingWindow, managementTaskOpeningSectionTokens)
	if len(page.Stories) != 0 || info.Shown != 0 || info.InWindow != 0 || !strings.Contains(page.Note, "no story") || !strings.Contains(page.Note, "8h") {
		t.Fatalf("P4: page of a quiet window = %+v", page)
	}
	if raw, _ := json.Marshal(page); !strings.Contains(string(raw), `"stories":[]`) {
		t.Fatalf("P4: an empty page must carry an empty list: %s", raw)
	}
}

// candidatePool is every earlier story of the pairs, the look-alike's earlier
// stories and unrelated stories, held at 1 to 70 hours.

func linksOf(near []string, leads ...string) arrivalLinks {
	return arrivalLinks{Near: near, Leads: leads}
}

func candidateWorld(now time.Time) (map[string]store.StorySummary, []types.CaptureArrival) {
	by := map[string]store.StorySummary{}
	add := func(doc, headline, lede, point string, v int, age time.Duration) {
		by[doc] = store.StorySummary{DocID: doc, Headline: headline, Lede: lede, MainPoint: point, Version: v, UpdatedAt: now.Add(-age)}
	}
	add("docA", "Rotterdam port workers strike", "Dockers walked out.", "Whether the Rotterdam dock strike ends and on what terms", 4, 30*time.Hour)
	add("docB", "Rotterdam port workers strike", "Dockers walked out.", "", 9, 2*time.Hour)
	add("docC", "Antwerp strike", "A second port.", "Whether the Antwerp dock strike ends", 2, 10*24*time.Hour)
	return by, []types.CaptureArrival{{CaptureID: "cap-1", Title: "Rotterdam port workers strike"}}
}

func TestCandidatesFoundByNeighboursAndLeadsNeverByTitleWords(t *testing.T) {
	now := time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC)
	by, arrivals := candidateWorld(now)
	// C1: no link, no candidate, however alike the titles (docA and docB share the arrival's words).
	if sec, offer, _, _ := buildCandidateSection(arrivals, map[string]arrivalLinks{}, by, now, 20000); sec != nil || len(offer.ByCapture) != 0 {
		t.Fatalf("C1: a story with the same title words and no link was offered: %+v", sec)
	}
	// A neighbour's story, a lead-only story and a ten-day-old story are offered.
	links := map[string]arrivalLinks{"cap-1": {Near: []string{"docA", "docC"}, Leads: []string{"docB"}}}
	sec, offer, src, _ := buildCandidateSection(arrivals, links, by, now, 20000)
	if sec == nil || len(sec.Captures) != 1 || len(sec.Captures[0].Candidates) != 3 {
		t.Fatalf("C1: candidates = %+v", sec)
	}
	if got := offer.ByCapture["cap-1"]; len(got) != 3 {
		t.Fatalf("C1: offer = %v", got)
	}
	if src.Near != 2 || src.Lead != 1 {
		t.Fatalf("C1: sources = %+v, want near 2 lead 1", src)
	}
	by2 := map[string]string{}
	for _, c := range sec.Captures[0].Candidates {
		by2[c.DocID] = strings.Join(c.FoundBy, ",")
	}
	if by2["docA"] != "near:1" || by2["docB"] != "lead" || by2["docC"] != "near:1" {
		t.Fatalf("C1: found_by = %v", by2)
	}
}

func TestCandidatesRankByNeighboursHeldThenContextThenNewest(t *testing.T) {
	now := time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC)
	by, arrivals := candidateWorld(now)
	// docC holds two of the arrival's neighbours: first. docA and docB hold one
	// each: the one with more accumulated context (v 9) before the newer.
	links := map[string]arrivalLinks{"cap-1": {Near: []string{"docA", "docC", "docB", "docC"}}}
	sec, _, _, _ := buildCandidateSection(arrivals, links, by, now, 20000)
	var order []string
	for _, c := range sec.Captures[0].Candidates {
		order = append(order, c.DocID)
	}
	if strings.Join(order, ",") != "docC,docB,docA" {
		t.Fatalf("C2: order = %v, want docC (2 held), docB (v9), docA", order)
	}
	// A recorded point is shown as the point; a story without one shows its lede, said so.
	for _, c := range sec.Captures[0].Candidates {
		switch c.DocID {
		case "docA":
			if c.MainPoint != by["docA"].MainPoint || c.PointIsLede {
				t.Errorf("C2: docA = %+v", c)
			}
		case "docB":
			if c.MainPoint != "Dockers walked out." || !c.PointIsLede {
				t.Errorf("C2: docB = %+v", c)
			}
		}
	}
	// At most candidatesPerCapture.
	many := map[string]store.StorySummary{}
	var near []string
	for i := 0; i < candidatesPerCapture+5; i++ {
		doc := fmt.Sprintf("doc%02d", i)
		many[doc] = store.StorySummary{DocID: doc, Headline: "h", UpdatedAt: now.Add(-time.Duration(i) * time.Hour), MainPoint: "A main point that is long enough"}
		near = append(near, doc)
	}
	sec, _, _, _ = buildCandidateSection(arrivals, map[string]arrivalLinks{"cap-1": {Near: near}}, many, now, 200000)
	if n := len(sec.Captures[0].Candidates); n != candidatesPerCapture {
		t.Fatalf("C2: %d candidates, want %d", n, candidatesPerCapture)
	}
	// The section bound leaves an arrival out and says so.
	sec, _, _, unlisted := buildCandidateSection(arrivals, map[string]arrivalLinks{"cap-1": {Near: near}}, many, now, 100)
	if unlisted != 1 || sec == nil || !strings.Contains(sec.Note, "not listed") {
		t.Fatalf("C2: unlisted = %d, section = %+v", unlisted, sec)
	}
}

func judgmentPacketOf(fields string) string {
	return `{"captures":["cap-1"],"hypotheses":[]` + fields + `}`
}

const goodPoint = `"main_point":"Whether the Rotterdam dock strike ends and on what terms"`

func TestOpenRefusalsNameTheShapeAndSendNothing(t *testing.T) {
	offer := &openingOffer{ByCapture: map[string][]string{"cap-1": {"docA", "docB"}}, Leased: []string{"cap-1", "cap-2"}}
	long := strings.Repeat("x", mainPointMaxRunes+1)
	cases := []struct{ name, packet, want string }{
		{"no main point", judgmentPacketOf(`,"judgment":[]`), "main point"},
		{"short main point", judgmentPacketOf(`,"main_point":"too short"`), "main point"},
		{"long main point", judgmentPacketOf(`,"main_point":"` + long + `"`), "main point"},
		{"unanswered candidate", judgmentPacketOf(`,` + goodPoint + `,"judgment":[{"story":"texture:docA","answer":"new","why":"different"}]`), "texture:docB"},
		{"same on an open", judgmentPacketOf(`,` + goodPoint + `,"judgment":[{"story":"texture:docA","answer":"same","why":"w"},{"story":"texture:docB","answer":"new","why":"w"}]`), "choir.Feed"},
		{"succeeds without next", judgmentPacketOf(`,` + goodPoint + `,"succeeds":"texture:docA","judgment":[{"story":"texture:docA","answer":"new","why":"w"},{"story":"texture:docB","answer":"new","why":"w"}]`), "needs a next answer"},
		{"next without succeeds", judgmentPacketOf(`,` + goodPoint + `,"judgment":[{"story":"texture:docA","answer":"next","why":"w"},{"story":"texture:docB","answer":"new","why":"w"}]`), "succeeds"},
		{"two nexts", judgmentPacketOf(`,` + goodPoint + `,"succeeds":"texture:docA","judgment":[{"story":"texture:docA","answer":"next","why":"w"},{"story":"texture:docB","answer":"next","why":"w"}]`), "two next"},
		{"succeeds the wrong story", judgmentPacketOf(`,` + goodPoint + `,"succeeds":"texture:docB","judgment":[{"story":"texture:docA","answer":"next","why":"w"},{"story":"texture:docB","answer":"new","why":"w"}]`), "not the story you answered next"},
		{"malformed judgment", judgmentPacketOf(`,` + goodPoint + `,"judgment":"same"`), "{story, answer, why}"},
		{"unknown answer", judgmentPacketOf(`,` + goodPoint + `,"judgment":[{"story":"texture:docA","answer":"maybe","why":"w"}]`), "same, next, new"},
		{"no reason", judgmentPacketOf(`,` + goodPoint + `,"judgment":[{"story":"texture:docA","answer":"new"},{"story":"texture:docB","answer":"new","why":"w"}]`), "why"},
		{"foreign capture", `{"captures":["cap-9"],` + goodPoint + `}`, "cap-9"},
		{"succeeds an invented story", judgmentPacketOf(`,` + goodPoint + `,"succeeds":"texture:docX","judgment":[{"story":"texture:docX","answer":"next","why":"w"},{"story":"texture:docA","answer":"new","why":"w"},{"story":"texture:docB","answer":"new","why":"w"}]`), "not a candidate story offered"},
		{"one story answered twice", judgmentPacketOf(`,` + goodPoint + `,"succeeds":"texture:docA","judgment":[{"story":"texture:docA","answer":"next","why":"w"},{"story":"texture:docA","answer":"next","why":"w"},{"story":"texture:docB","answer":"new","why":"w"}]`), "twice"},
		{"no captures named", `{"hypotheses":[],` + goodPoint + `}`, "names the captures"},
	}
	for _, c := range cases {
		err := requireJudgment(offer, c.packet)
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "nothing was sent") {
			t.Errorf("J1: %s: err = %v, want one naming %q and that nothing was sent", c.name, err, c.want)
		}
	}
	ok := judgmentPacketOf(`,` + goodPoint + `,"succeeds":"texture:docA","judgment":[{"story":"texture:docA","answer":"next","why":"the strike ended"},{"story":"texture:docB","answer":"new","why":"another port"}]`)
	if err := requireJudgment(offer, ok); err != nil {
		t.Errorf("J1: a complete successor open was refused: %v", err)
	}
	// Nothing offered: a main point alone opens. A nil offer (a resumed run) the same.
	if err := requireJudgment(nil, `{"captures":["cap-1"],`+goodPoint+`}`); err != nil {
		t.Errorf("J1: new matter with no candidate was refused: %v", err)
	}
	if err := requireJudgment(&openingOffer{ByCapture: map[string][]string{}}, `{"captures":["cap-1"]}`); err == nil {
		t.Errorf("J1: an open with no main point was accepted when nothing was offered")
	}
}

// The design's section 9: the 22 real pairs, as the rule answers them. An
// unclear pair is folded into a feed (owner: "Lean to revising"), so what a
// host sees of it is a feed, never an open. A same is a feed; a next is an open
// with succeeds; new is an open. "How the host answers an open" is all this
// table checks: the judgment itself is the model's.
func TestSectionNinePairsRevisedSuccessorNewAndUnclearFoldsToFeed(t *testing.T) {
	type pair struct {
		row            int
		earlier, later string
		rule           string // same, next, new, unclear
	}
	pairs := []pair{
		{1, "Six killed in Georgia gas explosion", "Six killed in Georgia gas explosion", "same"},
		{2, "Pew: Islam largest religion by 2035", "Pew: Islam largest religion by 2035", "same"},
		{3, "UK sanctions eight Israeli settlement firms", "UK sanctions eight Israeli settlement firms", "same"},
		{4, "UK sanctions firms, Australia joins", "Australia sanctions settler leader Weiss", "same"},
		{5, "Russia's mass missile strike on Ukraine", "Massive overnight attack on Ukraine", "same"},
		{6, "Missiles and a drone land in Poland", "Polish PM calls it an intentional provocation", "unclear"},
		{7, "Nikhil Gupta sentenced to 19 years 7 months", "Nikhil Gupta sentenced to 19 years 7 months", "same"},
		{8, "ICJ to deliver verdict Thursday on Myanmar", "ICJ finds genocide", "next"},
		{9, "First deep-sea mining lease auction off American Samoa", "US sells first leases for $6m", "same"},
		{10, "US loses measles-free status", "US loses measles-free status, a German ex-minister warns", "same"},
		{11, "Google adds an AI sidebar to search", "Google redesigns its search page", "same"},
		{12, "Trump calls Miller the most powerful man in the world", "Trump's press conference after Asia hails Miller", "unclear"},
		{13, "Kynev arrested", "Moscow extends Kynev's detention to December 24", "same"},
		{14, "Eight killed in Brooklyn collapse", "New video of the Brooklyn collapse", "same"},
		{15, "India and Japan set to review defence ties at 2+2", "India and Japan agree to expand cooperation", "next"},
		{16, "Senate clears the way for the permitting bill", "Senate passes the permitting bill 62-33", "next"},
		{17, "Kress passes first test as interim CEO, board keeps Huang out", "Nvidia confirms Huang resigned", "next"},
		{18, "Nations League draw set for Friday", "England drawn against Norway", "next"},
		{19, "French Senate delays police firearms vote", "Parliament approves police firearms law", "next"},
		{20, "Bukele due to register for a third term", "Bukele registers", "unclear"},
		{21, "Philippine charge against Thomas Markle", "US judge finds Markle fit for trial", "unclear"},
		{22, "A third US carrier reaches the Middle East", "USS George Washington leaves", "new"},
	}
	now := time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC)
	feeds, opens, successors := 0, 0, 0
	for _, p := range pairs {
		by := map[string]store.StorySummary{"docE": {DocID: "docE", Headline: p.earlier, MainPoint: "The main point of " + p.earlier, Version: 3, UpdatedAt: now.Add(-30 * time.Hour)}}
		arrivals := []types.CaptureArrival{{CaptureID: "cap-1", Title: p.later}}
		sec, offer, _, _ := buildCandidateSection(arrivals, map[string]arrivalLinks{"cap-1": {Near: []string{"docE"}}}, by, now, 20000)
		if sec == nil || len(offer.ByCapture["cap-1"]) != 1 {
			t.Fatalf("J2: row %d: the earlier story was not offered", p.row)
		}
		answer := p.rule
		if answer == "unclear" {
			answer = "same"
		}
		packet := judgmentPacketOf(`,"main_point":"The main point of ` + p.later + `, written once at open"`)
		switch answer {
		case "same":
			// A same answer on an open is refused: the capture is fed.
			err := requireJudgment(offer, packet[:len(packet)-1]+`,"judgment":[{"story":"texture:docE","answer":"same","why":"serves the point"}]}`)
			if err == nil || !strings.Contains(err.Error(), "choir.Feed") {
				t.Errorf("J2: row %d (%s): a same answer opened a story: %v", p.row, p.rule, err)
			}
			feeds++
		case "next":
			spec := packet[:len(packet)-1] + `,"succeeds":"texture:docE","judgment":[{"story":"texture:docE","answer":"next","why":"the main beat moved on"}]}`
			if err := requireJudgment(offer, spec); err != nil {
				t.Errorf("J2: row %d: a successor was refused: %v", p.row, err)
			}
			opens++
			successors++
		case "new":
			spec := packet[:len(packet)-1] + `,"judgment":[{"story":"texture:docE","answer":"new","why":"a different point"}]}`
			if err := requireJudgment(offer, spec); err != nil {
				t.Errorf("J2: row %d: new matter was refused: %v", p.row, err)
			}
			opens++
		}
	}
	// The design's tally, with the four unclear folded: 15 feeds, 6 successors, 1 new.
	if feeds != 15 || successors != 6 || opens != 7 {
		t.Fatalf("J2: feeds %d successors %d opens %d, want 15, 6, 7", feeds, successors, opens)
	}
}

func TestStoryChoiceLineCarriesPageCandidatesAndTheJudgment(t *testing.T) {
	d := choiceDetail{Window: openingWindow, InWindow: 312, Offered: []string{"docA", "docB"}, Answers: map[string]string{"docA": "next"},
		Sources: candidateSources{Near: 2, Lead: 1}, Captures: 3, MainPoint: 58, Succeeds: "docA"}
	got := managementStoryChoiceLine("run-9", "open", "doc-7", 312, 0, d)
	want := "management story choice run=run-9 choice=open story=doc-7 opening_shown=312 verb_shown=0 candidates_shown=312 window=8h page_in_window=312 page_left_out=0 offered=docA,docB answers=docA:next,docB:none captures=3 sources=near:2,lead:1 main_point=58 succeeds=docA"
	if got != want {
		t.Fatalf("L1: line = %q\nwant        %q", got, want)
	}
	got = managementStoryChoiceLine("run-9", "feed", "doc-4", 50, 25, choiceDetail{Window: openingWindow, InWindow: 400, LeftOut: 77, Captures: 2})
	want = "management story choice run=run-9 choice=feed story=doc-4 opening_shown=50 verb_shown=25 candidates_shown=75 window=8h page_in_window=400 page_left_out=77 offered=- answers=- captures=2 sources=near:0,lead:0 main_point=0 succeeds=-"
	if got != want {
		t.Fatalf("L1: line = %q\nwant        %q", got, want)
	}
	if got := storyResearchLine("run-1", "doc-4", 2, true, "feed"); got != "story research cast run=run-1 story=doc-4 round=2 opened=true cause=feed" {
		t.Fatalf("L1: research line = %q", got)
	}
}

func TestPromptNamesWhatThePageHolds(t *testing.T) {
	prompt := managementTaskPrompt
	for _, want := range []string{"8 hours", "headlines", "left_out", "candidate_stories", "main_point", "judgment", "next", "succeeds", "most context"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("T1: the prompt does not say %q: %s", want, prompt)
		}
	}
	if strings.Contains(prompt, "held_stories") || strings.Contains(prompt, "held_check") || strings.Contains(prompt, "same words") {
		t.Errorf("T1: the prompt still speaks of title-word held stories: %s", prompt)
	}
	page := openingPage{Stories: []openingStory{{}}}
	raw, _ := json.Marshal(page)
	cand, _ := json.Marshal(openingCandidateSection{Captures: []openingCandidateCapture{{Candidates: []candidateStory{{}}}}})
	body, err := os.ReadFile("../runtimeprompts/overlays/rlm_management_task_runtime.yaml")
	if err != nil {
		t.Fatal(err)
	}
	overlay := string(body)
	for _, key := range []string{"window", "as_of", "left_out", "next", "stories", "inbox", "headline", "age", "v"} {
		if !strings.Contains(string(raw), `"`+key+`"`) && key != "left_out" && key != "next" {
			t.Errorf("T1: page has no key %q: %s", key, raw)
		}
		if !strings.Contains(overlay, key) {
			t.Errorf("T1: the overlay does not name the page's key %q", key)
		}
	}
	for _, key := range []string{"candidate_stories", "capture_id", "main_point", "point_is_lede", "held", "found_by"} {
		if !strings.Contains(string(cand), `"`+key+`"`) && key != "point_is_lede" && key != "held" {
			t.Errorf("T1: candidate section has no key %q: %s", key, cand)
		}
		if !strings.Contains(overlay, key) {
			t.Errorf("T1: the overlay does not name %q", key)
		}
	}
	for _, v := range []string{"judgment", "next", "new", "succeeds", "main_point", "feed"} {
		if !strings.Contains(overlay, v) {
			t.Errorf("T1: the overlay does not state the answer shape (%s)", v)
		}
	}
	if strings.Contains(overlay, "held_check") || strings.Contains(overlay, "follow_up") {
		t.Errorf("T1: the overlay still teaches the title-word held check")
	}
}

type leadSink struct {
	neighbourSink
	leads    map[string][]string
	leadErr  error
	leadReqs int
}

func (s *leadSink) SearchLeadConversations(ctx context.Context, owner, computer string, ids []string) (map[string][]string, error) {
	s.leadReqs++
	return s.leads, s.leadErr
}

// A task's whole decision over a real store: a capture whose neighbour belongs
// to a held story is offered that story; an open that leaves it unanswered is
// refused; a next opens a successor whose trajectory records the main point
// and succeeds; the story library returns them; and the judgment added no model
// call (J4). One task per step: a task disposes of a capture once.
func TestTaskOpenAnswersCandidatesAndWritesMainPointAndSucceeds(t *testing.T) {
	f := newDelegationFixtureWith(t, "owner-main-point", false)
	ctx := context.Background()
	sink := &leadSink{neighbourSink: neighbourSink{near: map[string][]string{}}}
	f.rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	for i, title := range []string{"probe", "Rotterdam strike vote scheduled", "Rotterdam dockers vote to end the strike", "Rotterdam strike vote held"} {
		seedArrival(t, f.s, f.owner, f.computer, fmt.Sprintf("rss:m%d", i), []types.CaptureArrival{{CaptureID: fmt.Sprintf("cap-m%d", i), Title: title, URL: fmt.Sprintf("https://m.example/%d", i)}})
	}
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 4 {
		t.Fatalf("queued = %+v", queued)
	}
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	cast := func(fields string) string {
		return "var ids []string\nfor _, r := range choir.Records() { if r.Captures != nil { for _, c := range r.Captures.Items { ids = append(ids, c.CaptureID) } } }\n" +
			`choir.Cast("texture:new", "Rotterdam dock strike", map[string]any{"captures": ids, "publish": "auto"` + fields + `, "hypotheses": []any{map[string]any{"statement": "the strike ends", "probability": 0.5}}}, map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"})
choir.Done("opened")`
	}
	run := func(id, cell string) {
		t.Helper()
		var task types.ManagementTask
		for _, q := range queued {
			if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, id); d.LeaseTask == q.TaskID {
				task = q
			}
		}
		if task.TaskID == "" {
			t.Fatalf("no task leases %s", id)
		}
		f.model.mu.Lock()
		f.model.taskCell = cell
		f.model.mu.Unlock()
		rec, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, task.TaskInbox)
		if err != nil || rec == nil {
			t.Fatalf("start %s: %v, %v", task.TaskID, rec, err)
		}
		waitForStoredRunTerminalState(t, f.s, rec.RunID, 60*time.Second)
		eventually(t, func() bool { return f.task(task.TaskID).State == types.ManagementTaskClosed })
	}
	disposed := func(id string) types.CaptureDisposition {
		d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, id)
		return d
	}
	// Step 1: no main point is refused, nothing disposed; it fixes the calls of a refused task.
	before := f.model.callCount()
	run("cap-m0", cast(""))
	if out := f.model.lastOutput(); !strings.Contains(out, "main point") || disposed("cap-m0").Kind != "" {
		t.Fatalf("J1: an open with no main point was accepted or its capture disposed: %s", out)
	}
	callsRefused := f.model.callCount() - before
	// Step 2: new matter, nothing offered: a main point alone opens a story.
	before = f.model.callCount()
	run("cap-m1", cast(`, `+goodPoint))
	d1 := disposed("cap-m1")
	if d1.Kind != types.DispositionOpen || !strings.HasPrefix(d1.Inbox, "texture:") {
		t.Fatalf("the first story did not open: %+v (%s)", d1, f.model.lastOutput())
	}
	callsOpened := f.model.callCount() - before
	storyDoc := strings.TrimPrefix(d1.Inbox, "texture:")
	// Step 3: cap-m2's nearest capture is cap-m1 and a lead names the story: it is offered, and unanswered is refused.
	sink.near["cap-m2"], sink.near["cap-m3"] = []string{"cap-m1"}, []string{"cap-m1"}
	sink.leads = map[string][]string{"cap-m2": {storyDoc}, "cap-m3": {storyDoc}}
	before = f.model.callCount()
	run("cap-m2", cast(`, `+goodPoint))
	if out := f.model.lastOutput(); !strings.Contains(out, "texture:"+storyDoc) || !strings.Contains(out, "judgment") || disposed("cap-m2").Kind != "" {
		t.Fatalf("J1: an open leaving the offered story unanswered was not refused naming it: %s", out)
	}
	if got := f.model.callCount() - before; got != callsRefused {
		t.Fatalf("J4: a refused task offered candidates made %d model calls, one with none offered made %d", got, callsRefused)
	}
	// Step 4: next opens the successor.
	logs.Reset()
	before = f.model.callCount()
	run("cap-m3", cast(`, `+goodPoint+`, "succeeds": "texture:`+storyDoc+`", "judgment": []any{map[string]any{"story": "texture:`+storyDoc+`", "answer": "next", "why": "the vote is held: the strike is ending"}}`))
	if got := f.model.callCount() - before; got != callsOpened {
		t.Fatalf("J4: an open judged against a candidate made %d model calls, an open with none offered made %d", got, callsOpened)
	}
	d3 := disposed("cap-m3")
	if d3.Kind != types.DispositionOpen {
		t.Fatalf("J3: the successor did not open: %+v (%s)", d3, f.model.lastOutput())
	}
	successorDoc := strings.TrimPrefix(d3.Inbox, "texture:")
	docs, err := f.s.ListStoryDocuments(ctx, f.owner, f.computer, time.Time{}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]store.StorySummary{}
	for _, sum := range f.s.StorySummariesOf(ctx, docs) {
		got[sum.DocID] = sum
	}
	if got[successorDoc].MainPoint == "" || got[successorDoc].Succeeds != storyDoc || got[storyDoc].MainPoint == "" || got[storyDoc].Succeeds != "" {
		t.Fatalf("J3: summaries = %+v", got)
	}
	line := ""
	for _, l := range strings.Split(logs.String(), "\n") {
		if strings.Contains(l, "management story choice") && strings.Contains(l, "choice=open") {
			line = l
		}
	}
	for _, want := range []string{"story=" + successorDoc, "offered=" + storyDoc, "answers=" + storyDoc + ":next", "captures=1", "sources=near:1,lead:1", "succeeds=" + storyDoc} {
		if !strings.Contains(line, want) {
			t.Fatalf("L1: the open's line lacks %q: %q", want, line)
		}
	}
}

func (m *delegationModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// A neighbours request that fails, or a sink that does not answer one, leaves
// the page and a note; it never refuses an open (C3).
func TestCandidateReadFailureIsANoteNotARefusal(t *testing.T) {
	f, _, rec := openingFixture(t, "owner-cand-fail", "Harbor strike begins at Rotterdam port")
	for name, captures := range map[string]any{
		"error":   &leadSink{neighbourSink: neighbourSink{err: fmt.Errorf("corpusd is down")}, leadErr: fmt.Errorf("leads down")},
		"unavail": &neighbourSink{unavailable: "vectors are not ready"},
		"no sink": nil,
	} {
		if captures == nil {
			f.rt.researchDeps = nil
		} else {
			f.rt.researchDeps = &researchtools.Dependencies{Captures: captures.(researchtools.CaptureSink)}
		}
		o := f.rt.managementTaskOpening(context.Background(), rec)
		if o == nil || !strings.Contains(o.text, openingStoriesHeading) || !strings.Contains(o.text, openingCandidateHeading) || !strings.Contains(o.text, `"note":`) {
			t.Fatalf("C3: %s: the page or the note is missing: %v", name, o)
		}
		if err := requireJudgment(o.offer, `{"captures":["cap-o1"],`+goodPoint+`}`); err != nil {
			t.Fatalf("C3: %s: the open was refused: %v", name, err)
		}
	}
}

// The whole opening, over a real store: every story of the window is on the
// page; a story with identical title words and no link is not offered.
func TestTaskOpeningHoldsTheWholeWindowAndOffersNoTitleWordCandidates(t *testing.T) {
	f, _, rec := openingFixture(t, "owner-open-window", "Harbor strike begins at Rotterdam port")
	for i := 0; i < 260; i++ {
		openTestStory(t, f.s, f.owner, f.computer, fmt.Sprintf("Unrelated story number %03d about a harbour of nothing", i))
	}
	lookalike := openTestStory(t, f.s, f.owner, f.computer, "Harbor strike begins at Rotterdam port")
	o := f.rt.managementTaskOpening(context.Background(), rec)
	if o == nil {
		t.Fatal("no opening built")
	}
	var page openingPage
	if err := json.Unmarshal(o.storyOut, &page); err != nil {
		t.Fatalf("the delivered page is not an opening page: %v", err)
	}
	if page.Shown < 262 || page.LeftOut != 0 {
		t.Fatalf("P1: the page holds %d stories (left out %d) for 262 updated in the window", page.Shown, page.LeftOut)
	}
	if len(o.offer.ByCapture["cap-o1"]) != 0 {
		t.Fatalf("C1: a title-word look-alike %s was offered: %+v", lookalike, o.offer.ByCapture)
	}
	if o.offer.Shown != page.Shown {
		t.Fatalf("the offer's page size = %d, want %d", o.offer.Shown, page.Shown)
	}
	if f.rt.openingShownCount(rec.RunID) != 0 {
		t.Fatal("building recorded the page")
	}
	o.commit()
	if got := f.rt.openingShownCount(rec.RunID); got != page.Shown {
		t.Fatalf("opening_shown = %d, want %d", got, page.Shown)
	}
	f.rt.forgetStoriesShown(rec.RunID)
	if f.rt.openingOfferOf(rec.RunID) != nil {
		t.Fatal("forget left the offer behind")
	}
}

// A bound the page cannot meet is said, in the opening text and on the page.
func TestTaskOpeningOversizeWindowIsTruncatedLoudly(t *testing.T) {
	restore := managementTaskOpeningSectionTokens
	t.Cleanup(func() { managementTaskOpeningSectionTokens = restore })
	f, _, rec := openingFixture(t, "owner-open-trunc", "Harbor strike")
	for i := 0; i < 120; i++ {
		openTestStory(t, f.s, f.owner, f.computer, fmt.Sprintf("Story %03d with a headline of a realistic length for the paper", i))
	}
	full := f.rt.managementTaskOpening(context.Background(), rec)
	recordsJSON, _ := json.Marshal(full.records.view)
	managementTaskOpeningSectionTokens = estimateTextTokens(string(recordsJSON)) + 1
	if managementTaskOpeningSectionTokens < 2500 {
		managementTaskOpeningSectionTokens = 2500
	}
	o := f.rt.managementTaskOpening(context.Background(), rec)
	var page openingPage
	if o == nil || json.Unmarshal(o.storyOut, &page) != nil {
		t.Fatalf("no page: %v", o)
	}
	if page.LeftOut == 0 || page.Shown == 0 || !strings.Contains(o.text, `"left_out":`) || !strings.Contains(o.text, "not listed") {
		t.Fatalf("P3: a truncated page did not say so: shown %d left out %d: %.600s", page.Shown, page.LeftOut, o.text)
	}
}

// Through a task's own cell: an open that leaves an offered held story
// unanswered is refused naming it, and the same open with a verdict commits.
// The log line of the committed open names the page and the candidate.

func TestOneHugeHeadlineIsClippedNotDroppingThePage(t *testing.T) {
	now := time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)
	in := windowStories(40, openingWindow, now)
	in[0].Headline = strings.Repeat("A very long headline ", 3000)
	page, info := buildOpeningPage(in, now, openingWindow, 6000)
	if info.Shown < 30 || len(page.Stories) != info.Shown || !strings.HasSuffix(page.Stories[0].Headline, "…") || len([]rune(page.Stories[0].Headline)) > 301 {
		t.Fatalf("R7: shown %d, first headline %d runes", info.Shown, len([]rune(page.Stories[0].Headline)))
	}
	if raw, _ := json.Marshal(page); estimateTextTokens(string(raw)) > 6000 {
		t.Fatalf("R7: page over its bound")
	}
}
