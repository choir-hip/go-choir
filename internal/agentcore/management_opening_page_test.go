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

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The management task's story page and the held stories offered for an
// arrival (docs/evidence/duplicate-stories-on-the-paper-2026-11-20.md,
// options 1 and 2). Failure list, written before the code:
//
//  P1  the page is not what the prompt promises: a story updated inside the
//      window is missing (the old page held the newest 50, a median 27
//      minutes back), or carries a lede (headlines only), or a story older
//      than the window is on it.
//  P2  a full window does not fit the section bound by construction, or the
//      bound is met by a silent fallback to the newest 50.
//  P3  a bound that truncates does so quietly: the page does not say how many
//      stories it left out, how old the oldest listed one is and how to read
//      the rest, or the log line does not say so.
//  P4  an empty window falls back to older stories instead of saying nothing
//      was updated in it.
//  H1  a held story of the last 72 hours that covers the arrival (the
//      evidence file's labelled pairs, by title) is not offered with enough to
//      judge it (headline, age, a line of lead), or a story with no words in
//      common is offered, or more than the top few are.
//  H2  a story older than 72 hours is offered.
//  H3  a texture:new Cast that leaves an offered candidate without a verdict
//      (same, follow_up or different), or with a verdict outside those three,
//      is accepted; or an answer "same" is refused by rule (the decision stays
//      the model's).
//  H4  the log line of a decision leaves out the page's size, window and
//      truncation, the candidates offered, or the verdicts.
//  T1  the prompt names a window, a key or a field the page does not carry.

// evidencePairs are labelled pairs of
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
func candidatePool(now time.Time) []store.StorySummary {
	var pool []store.StorySummary
	i := 0
	add := func(h, lede string, age time.Duration) {
		pool = append(pool, sumAt(i, h, lede, age, now))
		i++
	}
	for k, p := range evidencePairs {
		add(p.earlier, "The first paragraph of the earlier story, which says what happened in a sentence or two and then goes on at some length about the people involved and the places and the reasons behind it all.", time.Duration(2+k*3)*time.Hour)
	}
	for k, l := range evidenceLookAlikes {
		add(l.earlier, "A lede.", time.Duration(5+k)*time.Hour)
	}
	for k, u := range unrelatedTitles {
		add(u, "A lede.", time.Duration(1+k*6)*time.Hour)
	}
	add("A story held 80 hours ago: "+evidencePairs[2].earlier, "old", 80*time.Hour)
	return pool
}

func TestHeldStoriesOfTheLabelledPairsAreOffered(t *testing.T) {
	now := time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)
	pool := candidatePool(now)
	idx := newHeldIndex(pool, now)
	found := 0
	for k, p := range evidencePairs {
		cands := idx.find(p.later, heldPerCapture)
		hit := false
		for _, c := range cands {
			if c.DocID == pool[k].DocID {
				hit = true
				if c.Headline != p.earlier || c.Age == "" || c.Lede == "" {
					t.Errorf("H1: pair %d offered without enough to judge: %+v", k, c)
				}
			}
			if strings.HasPrefix(c.Headline, "A story held 80 hours") {
				t.Errorf("H2: pair %d was offered a story held 80 hours", k)
			}
		}
		if len(cands) > heldPerCapture {
			t.Errorf("H1: %d candidates for one arrival, the top %d only", len(cands), heldPerCapture)
		}
		if hit {
			found++
		} else {
			t.Logf("pair %d (%s) not offered %q; got %d candidates", k, p.kind, p.later, len(cands))
		}
	}
	t.Logf("held stories offered for %d of %d labelled pairs", found, len(evidencePairs))
	if found < 12 {
		t.Fatalf("H1: the earlier story was offered for %d of %d labelled pairs, want at least 12", found, len(evidencePairs))
	}
	// Identical titles are the plainest cases and must always be offered first.
	if c := idx.find(evidencePairs[0].later, heldPerCapture); len(c) == 0 || c[0].DocID != pool[0].DocID {
		t.Fatalf("H1: the identical title is not the first candidate: %+v", c)
	}
	// A story with no words in common offers nothing.
	if c := idx.find("Seismologists record a magnitude 5 tremor under the Pacific seabed", heldPerCapture); len(c) != 0 {
		t.Fatalf("H1: an unrelated arrival was offered %+v", c)
	}
}

// The three look-alikes the evidence file counted as distinct may be offered
// (a word overlap cannot tell them apart); the model answers "different" and
// the answer is accepted: no rule refuses an open.
func TestLookAlikesMayBeOfferedAndAreAnsweredDifferent(t *testing.T) {
	now := time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)
	pool := candidatePool(now)
	idx := newHeldIndex(pool, now)
	for k, l := range evidenceLookAlikes {
		offered := idx.find(l.later, heldPerCapture)
		var docs []string
		answers := []string{}
		for _, c := range offered {
			docs = append(docs, c.DocID)
			answers = append(answers, fmt.Sprintf(`{"story":"texture:%s","verdict":"different"}`, c.DocID))
		}
		offer := &openingOffer{ByCapture: map[string][]string{"cap-x": docs}}
		packet := fmt.Sprintf(`{"captures":["cap-x"],"held_check":[%s]}`, strings.Join(answers, ","))
		if err := requireHeldVerdicts(offer, packet); err != nil {
			t.Errorf("H3: look-alike %d answered different was refused: %v", k, err)
		}
		t.Logf("look-alike %d offered %d candidate(s)", k, len(offered))
	}
}

func TestHeldVerdictsAreRequiredForEveryOfferedCandidate(t *testing.T) {
	offer := &openingOffer{ByCapture: map[string][]string{"cap-1": {"docA", "docB"}, "cap-2": {"docC"}}}
	ok := func(packet string) error { return requireHeldVerdicts(offer, packet) }
	if err := ok(`{"captures":["cap-1"],"held_check":[{"story":"texture:docA","verdict":"same"},{"story":"docB","verdict":"follow_up"}]}`); err != nil {
		t.Fatalf("H3: a full answer (same, follow_up; bare doc id) was refused: %v", err)
	}
	if err := ok(`{"captures":["cap-1"],"held_check":[{"story":"texture:docA","verdict":"different"},{"story":"texture:docB","verdict":"different"}]}`); err != nil {
		t.Fatalf("H3: a different-different answer was refused: %v", err)
	}
	err := ok(`{"captures":["cap-1"],"held_check":[{"story":"texture:docA","verdict":"different"}]}`)
	if err == nil || !strings.Contains(err.Error(), "docB") || strings.Contains(err.Error(), "docC") || !strings.Contains(err.Error(), "held_check") {
		t.Fatalf("H3: a missing verdict for docB was not refused naming it (and only it): %v", err)
	}
	if err := ok(`{"captures":["cap-1"]}`); err == nil {
		t.Fatal("H3: an open with no held_check passed although two held stories were offered")
	}
	if err := ok(`{"captures":["cap-1"],"held_check":[{"story":"texture:docA","verdict":"maybe"},{"story":"texture:docB","verdict":"same"}]}`); err == nil || !strings.Contains(err.Error(), "maybe") {
		t.Fatalf("H3: a verdict outside same, follow_up and different was accepted: %v", err)
	}
	// The captures the Cast names decide what is owed; with none named, every offer is.
	if err := ok(`{"captures":["cap-2"],"held_check":[{"story":"texture:docC","verdict":"same"}]}`); err != nil {
		t.Fatalf("H3: cap-2's answer was refused: %v", err)
	}
	if err := ok(`{"held_check":[{"story":"texture:docA","verdict":"same"}]}`); err == nil {
		t.Fatal("H3: a Cast naming no captures owes every offer; it was accepted with docB and docC missing")
	}
	// Nothing offered, nothing owed; a nil offer (a resumed run) owes nothing.
	if err := requireHeldVerdicts(&openingOffer{ByCapture: map[string][]string{}}, `{"captures":["cap-1"]}`); err != nil {
		t.Fatalf("H3: nothing was offered, yet: %v", err)
	}
	if err := requireHeldVerdicts(nil, `{"captures":["cap-1"]}`); err != nil {
		t.Fatalf("H3: a nil offer owes nothing: %v", err)
	}
}

func TestStoryChoiceLineCarriesPageAndCandidates(t *testing.T) {
	d := choiceDetail{Window: openingWindow, InWindow: 312, LeftOut: 0, Offered: []string{"docA", "docB"}, Verdicts: map[string]string{"docA": "same"}}
	got := managementStoryChoiceLine("run-9", "open", "", 312, 0, d)
	want := "management story choice run=run-9 choice=open story=- opening_shown=312 verb_shown=0 candidates_shown=312 window=8h page_in_window=312 page_left_out=0 offered=docA,docB verdicts=docA:same,docB:none"
	if got != want {
		t.Fatalf("H4: line = %q\nwant        %q", got, want)
	}
	got = managementStoryChoiceLine("run-9", "feed", "doc-4", 50, 25, choiceDetail{Window: openingWindow, InWindow: 400, LeftOut: 77})
	want = "management story choice run=run-9 choice=feed story=doc-4 opening_shown=50 verb_shown=25 candidates_shown=75 window=8h page_in_window=400 page_left_out=77 offered=- verdicts=-"
	if got != want {
		t.Fatalf("H4: line = %q\nwant        %q", got, want)
	}
}

func TestPromptNamesWhatThePageHolds(t *testing.T) {
	prompt := managementTaskPrompt
	for _, want := range []string{"8 hours", "headlines", "left_out", "held_stories", "held_check", "same", "follow_up", "different"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("T1: the prompt does not say %q: %s", want, prompt)
		}
	}
	if strings.Contains(prompt, "newest page of the story library") || strings.Contains(prompt, `choir.Stories(50, "")`) {
		t.Errorf("T1: the prompt still promises the newest 50: %s", prompt)
	}
	// Every key the prompt and the overlay name is a key the page carries.
	page := openingPage{Stories: []openingStory{{}}}
	raw, _ := json.Marshal(page)
	held, _ := json.Marshal(openingHeldSection{Captures: []openingHeldCapture{{HeldStories: []heldCandidate{{}}}}})
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
	for _, key := range []string{"held_stories", "capture_id", "lede"} {
		if !strings.Contains(string(held), `"`+key+`"`) || !strings.Contains(overlay, key) {
			t.Errorf("T1: held section / overlay disagree on %q: %s", key, held)
		}
	}
	for _, v := range []string{"held_check", "same", "follow_up", "different"} {
		if !strings.Contains(overlay, v) {
			t.Errorf("T1: the overlay does not state the answer shape (%s)", v)
		}
	}
}

// The whole opening, over a real store: every story of the window is on the
// page, there is no newest-50 fallback, and the held section names the stories
// that may cover a capture.
func TestTaskOpeningHoldsTheWholeWindowAndOffersHeldStories(t *testing.T) {
	f, _, rec := openingFixture(t, "owner-open-window", "Harbor strike begins at Rotterdam port")
	for i := 0; i < 260; i++ {
		openTestStory(t, f.s, f.owner, f.computer, fmt.Sprintf("Unrelated story number %03d about a harbour of nothing", i))
	}
	covering := openTestStory(t, f.s, f.owner, f.computer, "Rotterdam port workers begin a harbor strike")
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
	if !strings.Contains(o.text, "Held stories that may already cover") || !strings.Contains(o.text, covering) {
		t.Fatalf("H1: the covering story %s is not offered: %.1200s", covering, o.text[strings.Index(o.text, "Held stories")-200:])
	}
	if o.offer == nil || len(o.offer.ByCapture["cap-o1"]) == 0 || o.offer.ByCapture["cap-o1"][0] != covering {
		t.Fatalf("H1: offer = %+v", o.offer)
	}
	// Building marks nothing; commit records the offer for the decision's line.
	if f.rt.openingShownCount(rec.RunID) != 0 {
		t.Fatal("building recorded the page")
	}
	o.commit()
	if got := f.rt.openingShownCount(rec.RunID); got != page.Shown {
		t.Fatalf("opening_shown = %d, want %d", got, page.Shown)
	}
	if offer := f.rt.openingOfferOf(rec.RunID); offer == nil || offer.ByCapture["cap-o1"][0] != covering {
		t.Fatalf("the run's offer was not kept: %+v", offer)
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
func TestTaskOpenNeedsAVerdictForEachOfferedHeldStory(t *testing.T) {
	f := newDelegationFixture(t, "owner-held-verdict")
	ctx := context.Background()
	covering := openTestStory(t, f.s, f.owner, f.computer, "Rotterdam port workers begin a harbor strike")
	seedArrival(t, f.s, f.owner, f.computer, "rss:h1", []types.CaptureArrival{{CaptureID: "cap-h1", Title: "Harbor strike begins at Rotterdam port", URL: "https://h.example/1"}})
	seedArrival(t, f.s, f.owner, f.computer, "rss:h2", []types.CaptureArrival{{CaptureID: "cap-h2", Title: "Harbor strike begins at Rotterdam port", URL: "https://h.example/2"}})
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 {
		t.Fatalf("queued = %+v", queued)
	}
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	cell := func(check string) string {
		return "var ids []string\nfor _, r := range choir.Records() { if r.Captures != nil { for _, c := range r.Captures.Items { ids = append(ids, c.CaptureID) } } }\n" +
			`choir.Cast("texture:new", "Harbor strike begins", map[string]any{"captures": ids, "publish": "auto"` + check + `, "hypotheses": []any{map[string]any{"statement": "the strike lasts a week", "probability": 0.5}}}, map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"})
choir.Done("opened")`
	}
	run := func(task types.ManagementTask, cell string) *types.RunRecord {
		t.Helper()
		f.model.mu.Lock()
		f.model.taskCell = cell
		f.model.mu.Unlock()
		rec, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, task.TaskInbox)
		if err != nil || rec == nil {
			t.Fatalf("start %s: %v, %v", task.TaskID, rec, err)
		}
		waitForStoredRunTerminalState(t, f.s, rec.RunID, 60*time.Second)
		eventually(t, func() bool { return f.task(task.TaskID).State == types.ManagementTaskClosed })
		return rec
	}
	run(queued[0], cell(""))
	if out := f.model.lastOutput(); !strings.Contains(out, "held_check") || !strings.Contains(out, covering) {
		t.Fatalf("H3: an open with no verdict for the offered story was not refused naming it: %s", out)
	}
	for _, id := range []string{"cap-h1", "cap-h2"} {
		if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, id); d.Kind != "" {
			t.Fatalf("H3: the refused cell disposed of %s: %+v", id, d)
		}
	}
	rec2 := run(queued[1], cell(`, "held_check": []any{map[string]any{"story": "texture:`+covering+`", "verdict": "different"}}`))
	opened := 0
	for _, id := range []string{"cap-h1", "cap-h2"} {
		if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, id); d.Kind == types.DispositionOpen {
			opened++
		}
	}
	if opened != 1 {
		t.Fatalf("H3: the open with a verdict did not commit (%d opened): %s", opened, f.model.lastOutput())
	}
	line := ""
	for _, l := range strings.Split(logs.String(), "\n") {
		if strings.Contains(l, "management story choice run="+rec2.RunID) && strings.Contains(l, "choice=open") {
			line = l
		}
	}
	if line == "" || strings.Contains(line, "story=- ") || !strings.Contains(line, "window=8h") || !strings.Contains(line, "offered="+covering) || !strings.Contains(line, "verdicts="+covering+":different") || !strings.Contains(line, "page_left_out=0") {
		t.Fatalf("H4: the committed open's line = %q", line)
	}
}

// Review round 1 (seat 2, b0296a55d). Failure list, written before the code:
//  R1  the held section and the offer depend on the records section fitting:
//      a task whose records are too large for the bound gets no offer, so its
//      open is unchecked.
//  R2  with an offer, a held_check that is present but not a list of
//      {story, verdict}, or a Cast naming a capture the task does not lease,
//      commits unchecked.
//  R3  a headline in a script written without spaces is one token, so even an
//      identical title cannot match.
//  R7  one huge headline drops the whole page and the held section.
func TestHeldOfferDoesNotDependOnTheRecordsFitting(t *testing.T) {
	restore := managementTaskOpeningSectionTokens
	t.Cleanup(func() { managementTaskOpeningSectionTokens = restore })
	filler := strings.Repeat("zqx wvu tsr ", 600)
	f, _, rec := openingFixture(t, "owner-open-norecords", "Harbor strike begins at Rotterdam port "+filler)
	covering := openTestStory(t, f.s, f.owner, f.computer, "Rotterdam port workers begin a harbor strike")
	managementTaskOpeningSectionTokens = 1500
	o := f.rt.managementTaskOpening(context.Background(), rec)
	if o == nil || strings.Contains(o.text, "Your records:") {
		t.Fatalf("R1: the records were meant not to fit: %v", o)
	}
	if o.offer == nil || len(o.offer.ByCapture["cap-o1"]) == 0 || o.offer.ByCapture["cap-o1"][0] != covering || !strings.Contains(o.text, covering) {
		t.Fatalf("R1: no offer when the records do not fit: %+v", o.offer)
	}
}

func TestHeldCheckMalformedOrForeignCaptureIsRefused(t *testing.T) {
	offer := &openingOffer{ByCapture: map[string][]string{"cap-1": {"docA"}}, Leased: []string{"cap-1", "cap-2"}}
	for name, packet := range map[string]string{
		"string":      `{"captures":["cap-1"],"held_check":"same"}`,
		"object":      `{"captures":["cap-1"],"held_check":{"story":"docA","verdict":"same"}}`,
		"bad entries": `{"captures":["cap-1"],"held_check":[1,2]}`,
	} {
		err := requireHeldVerdicts(offer, packet)
		if err == nil || !strings.Contains(err.Error(), "cap-1") || !strings.Contains(err.Error(), "same|follow_up|different") {
			t.Errorf("R2: %s held_check was accepted or the error does not name the captures and the shape: %v", name, err)
		}
	}
	err := requireHeldVerdicts(offer, `{"captures":["cap-9"],"held_check":[{"story":"docA","verdict":"same"}]}`)
	if err == nil || !strings.Contains(err.Error(), "cap-9") || !strings.Contains(err.Error(), "cap-2") {
		t.Errorf("R2: a capture the task does not lease was accepted: %v", err)
	}
	// A leased capture with no candidate may be named; null or an empty list is no answer, not malformed.
	if err := requireHeldVerdicts(offer, `{"captures":["cap-2"]}`); err != nil {
		t.Errorf("R2: a capture with nothing offered was refused: %v", err)
	}
	// Nothing offered at all: a malformed field is not this check's business.
	if err := requireHeldVerdicts(&openingOffer{ByCapture: map[string][]string{}, Leased: []string{"cap-1"}}, `{"captures":["cap-1"],"held_check":"x"}`); err != nil {
		t.Errorf("R2: refused with nothing offered: %v", err)
	}
}

func TestHeldStoriesMatchHeadlinesWrittenWithoutSpaces(t *testing.T) {
	now := time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)
	pool := []store.StorySummary{
		sumAt(0, "東京で大規模な地震が発生、津波警報を発令", "lede", 2*time.Hour, now),
		sumAt(1, "นายกรัฐมนตรีประกาศยุบสภาผู้แทนราษฎร", "lede", 3*time.Hour, now),
		sumAt(2, "新型コロナワクチンの追加接種を開始", "lede", 4*time.Hour, now),
	}
	idx := newHeldIndex(pool, now)
	if c := idx.find("東京で大規模な地震が発生、津波警報を発令", 3); len(c) == 0 || c[0].DocID != pool[0].DocID {
		t.Errorf("R3: an identical Japanese title was not offered first: %+v", c)
	}
	if c := idx.find("東京で大規模な地震が発生 津波警報", 3); len(c) == 0 || c[0].DocID != pool[0].DocID {
		t.Errorf("R3: a near-identical Japanese title was not offered: %+v", c)
	}
	if c := idx.find("นายกรัฐมนตรีประกาศยุบสภาผู้แทนราษฎร", 3); len(c) == 0 || c[0].DocID != pool[1].DocID {
		t.Errorf("R3: an identical Thai title was not offered: %+v", c)
	}
	if c := idx.find("今日の天気は晴れのち曇りでしょう", 3); len(c) != 0 {
		t.Errorf("R3: an unrelated Japanese title was offered %+v", c)
	}
}

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
