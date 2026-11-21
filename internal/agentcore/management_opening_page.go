package agentcore

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The story page a management task is handed, and the held stories offered
// for its arrivals (docs/evidence/duplicate-stories-on-the-paper-2026-11-20.md,
// options 1 and 2; the failure is the paper opening a second story on an event
// it already holds, one published story in five).
//
// The page is every story updated in the last openingWindow, as headlines
// only, built to fit the section bound: stories are added newest first until
// the bound is reached, so the page fits by construction, and when the window
// holds more than fits the page says how many it left out and how to read
// them. There is no fallback to a count of stories.
//
// Beside it, for each arrival, the top few held stories of the last 72 hours
// that share its words, with a line of lead, for the task to judge. The task
// answers for each one in the spec of its texture:new Cast (held_check); the
// host refuses an open that leaves one unanswered, never an open whose answer
// is "same": the decision stays the model's.

const (
	// openingWindowHours is the window the prompt, the page and the log name.
	openingWindowHours = 8
	openingWindow      = openingWindowHours * time.Hour
	// heldHorizon is how far back a held story may be offered. A story
	// is held from the first version; two thirds of the paper's duplicates
	// are 1.1 to 59 hours apart (evidence section 2).
	heldHorizon = 72 * time.Hour
	// heldPerCapture is the top few held stories offered for one arrival.
	heldPerCapture = 3
	// heldLedeRunes is the line of lead a candidate carries.
	heldLedeRunes = 160
	// heldMinShared and heldMinOverlap are the title-word floor: two shared
	// stems, and at least this share of the lighter title's weight.
	heldMinShared  = 2
	heldMinOverlap = 0.4
)

const (
	openingStoriesHeading = "Stories updated in the last 8 hours, headlines only:"
	openingHeldHeading    = "Held stories that may already cover these captures (the last 72 hours; answer each in held_check if you open):"
)

// openingStory is one headline of the page.
type openingStory struct {
	Inbox    string `json:"inbox"`
	Headline string `json:"headline"`
	// Age is the time since the story's last version.
	Age     string `json:"age"`
	Version int    `json:"v"`
}

// openingPage is the story page of a task's opening.
type openingPage struct {
	AsOf    string `json:"as_of"`
	Window  string `json:"window"`
	Shown   int    `json:"shown"`
	LeftOut int    `json:"left_out,omitempty"`
	Note    string `json:"note,omitempty"`
	// Next is the Before of choir.Stories for the stories left out.
	Next    string         `json:"next,omitempty"`
	Stories []openingStory `json:"stories"`
}

// openingPageInfo is what the decision's log line says of the page.
type openingPageInfo struct {
	Window   time.Duration
	InWindow int
	Shown    int
	LeftOut  int
}

func shortDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	return d.String()
}

// storyAge is the age text of a story: minutes up to three hours, then hours,
// then days.
func storyAge(updated, now time.Time) string {
	d := now.Sub(updated)
	if d < 0 {
		d = 0
	}
	switch {
	case d < 3*time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
}

func leftOutNote(left int, window time.Duration, oldest string) string {
	return fmt.Sprintf("%d stories updated in the last %s are not listed: the page is bounded, and the oldest listed is %s old. Read them, with ledes, with choir.Stories(50, next) (the next above); a story you cannot find there is older than the window.", left, shortDuration(window), oldest)
}

// buildOpeningPage builds the page from the story index (newest first, any
// age): every story updated within window as headlines, the newest first,
// stopping where the next one would take the page over budgetTokens. It makes
// the page fit by construction and says what it left out.
func buildOpeningPage(sums []store.StorySummary, now time.Time, window time.Duration, budgetTokens int) (openingPage, openingPageInfo) {
	now = now.UTC()
	var in []store.StorySummary
	for _, s := range sums {
		if now.Sub(s.UpdatedAt) <= window {
			in = append(in, s)
		}
	}
	page := openingPage{AsOf: now.Format(time.RFC3339), Window: shortDuration(window), Stories: []openingStory{}}
	info := openingPageInfo{Window: window, InWindow: len(in)}
	if len(in) == 0 {
		page.Note = fmt.Sprintf("no story was updated in the last %s; older stories are read with choir.Stories(50, \"\").", shortDuration(window))
		return page, info
	}
	entry := func(s store.StorySummary) openingStory {
		return openingStory{Inbox: deskmanifest.DocumentInbox(s.DocID), Headline: strings.Join(strings.Fields(s.Headline), " "), Age: storyAge(s.UpdatedAt, now), Version: s.Version}
	}
	// The page's own bytes beside its stories, at the worst case: the note,
	// the cursor and the count of a page that left everything out.
	probe := page
	probe.Shown, probe.LeftOut = len(in), len(in)
	probe.Note = leftOutNote(len(in), window, "99h")
	probe.Next = storiesCursor(in[0].UpdatedAt, in[0].DocID)
	base, _ := json.Marshal(probe)
	bytes := len(base)
	n := 0
	for _, s := range in {
		raw, _ := json.Marshal(entry(s))
		if estimateTextTokens(strings.Repeat("x", bytes+len(raw)+1)) > budgetTokens {
			break
		}
		bytes += len(raw) + 1
		n++
	}
	for _, s := range in[:n] {
		page.Stories = append(page.Stories, entry(s))
	}
	page.Shown, info.Shown = n, n
	if left := len(in) - n; left > 0 {
		info.LeftOut, page.LeftOut = left, left
		oldest := "0m"
		if n > 0 {
			oldest = page.Stories[n-1].Age
			page.Next = storiesCursor(in[n-1].UpdatedAt, in[n-1].DocID)
		}
		page.Note = leftOutNote(left, window, oldest)
	}
	return page, info
}

// heldCandidate is a held story that may already cover an arrival.
type heldCandidate struct {
	DocID    string `json:"-"`
	Inbox    string `json:"inbox"`
	Headline string `json:"headline"`
	Age      string `json:"age"`
	Lede     string `json:"lede"`
}

// heldIndex is the title-word index of the held stories of the last 72 hours:
// the floor of the search, which needs no model call, no Jev call and no read
// outside the story index management already reads. A story's words are its
// headline's, lower-cased, without stop words, cut to five letters ("sanctions"
// and "sanctioned" meet); a word is weighted by how few held stories carry it.
type heldIndex struct {
	now     time.Time
	stories []heldStory
	weight  map[string]float64
}

type heldStory struct {
	sum    store.StorySummary
	stems  map[string]bool
	weight float64
}

func titleStems(title string) map[string]bool {
	out := map[string]bool{}
	for w := range titleTokens(title) {
		if r := []rune(w); len(r) > 5 {
			w = string(r[:5])
		}
		out[w] = true
	}
	return out
}

func newHeldIndex(sums []store.StorySummary, now time.Time) *heldIndex {
	idx := &heldIndex{now: now.UTC(), weight: map[string]float64{}}
	df := map[string]int{}
	for _, s := range sums {
		if now.Sub(s.UpdatedAt) > heldHorizon {
			continue
		}
		st := heldStory{sum: s, stems: titleStems(s.Headline)}
		for w := range st.stems {
			df[w]++
		}
		idx.stories = append(idx.stories, st)
	}
	n := float64(len(idx.stories))
	for w, c := range df {
		idx.weight[w] = math.Log(1 + n/float64(c))
	}
	for i := range idx.stories {
		for w := range idx.stories[i].stems {
			idx.stories[i].weight += idx.weight[w]
		}
	}
	return idx
}

// find returns the top n held stories sharing words with title: at least
// heldMinShared stems, and at least heldMinOverlap of the lighter title's
// weight, best first (ties keep the newest first).
func (idx *heldIndex) find(title string, n int) []heldCandidate {
	want := titleStems(title)
	if len(want) == 0 || n <= 0 {
		return nil
	}
	var wantWeight float64
	for w := range want {
		wantWeight += idx.weight[w]
	}
	type hit struct {
		i     int
		score float64
	}
	var hits []hit
	for i, st := range idx.stories {
		shared, got := 0, 0.0
		for w := range want {
			if st.stems[w] {
				shared++
				got += idx.weight[w]
			}
		}
		if shared < heldMinShared {
			continue
		}
		lighter := math.Min(wantWeight, st.weight)
		if lighter <= 0 || got/lighter < heldMinOverlap {
			continue
		}
		hits = append(hits, hit{i, got / lighter})
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	var out []heldCandidate
	for _, h := range hits {
		if len(out) == n {
			break
		}
		s := idx.stories[h.i].sum
		out = append(out, heldCandidate{DocID: s.DocID, Inbox: deskmanifest.DocumentInbox(s.DocID),
			Headline: strings.Join(strings.Fields(s.Headline), " "), Age: storyAge(s.UpdatedAt, idx.now), Lede: intakeClip(s.Lede, heldLedeRunes)})
	}
	return out
}

// openingHeldCapture is one arrival and the held stories that may cover it.
type openingHeldCapture struct {
	CaptureID   string          `json:"capture_id"`
	Title       string          `json:"title"`
	HeldStories []heldCandidate `json:"held_stories"`
}

// openingHeldSection is the held stories offered, by arrival.
type openingHeldSection struct {
	Captures []openingHeldCapture `json:"captures"`
	Note     string               `json:"note,omitempty"`
}

// openingOffer is what a task's opening offered, kept for the run: the page's
// size and the held stories offered for each capture (document ids), which the
// task's open must answer and the decision's log line names.
type openingOffer struct {
	Window     time.Duration
	InWindow   int
	Shown      int
	LeftOut    int
	ByCapture  map[string][]string
	HeldUnlist int
}

// buildHeldSection offers, for each arrival with a candidate, its top few held
// stories, within budgetTokens (an arrival whose stories do not fit is said to
// be left out, and nothing is owed for it).
func buildHeldSection(arrivals []types.CaptureArrival, idx *heldIndex, budgetTokens int) (*openingHeldSection, map[string][]string, int) {
	section := &openingHeldSection{Captures: []openingHeldCapture{}}
	offers := map[string][]string{}
	seen := map[string]bool{}
	bytes, unlisted := 64, 0
	for _, a := range arrivals {
		id := strings.TrimSpace(a.CaptureID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		cands := idx.find(a.Title, heldPerCapture)
		if len(cands) == 0 {
			continue
		}
		c := openingHeldCapture{CaptureID: id, Title: strings.Join(strings.Fields(a.Title), " "), HeldStories: cands}
		raw, _ := json.Marshal(c)
		if estimateTextTokens(strings.Repeat("x", bytes+len(raw)+400)) > budgetTokens {
			unlisted++
			continue
		}
		bytes += len(raw) + 1
		section.Captures = append(section.Captures, c)
		for _, cand := range cands {
			offers[id] = append(offers[id], cand.DocID)
		}
	}
	if unlisted > 0 {
		section.Note = fmt.Sprintf("%d captures with held stories are not listed here (the section is bounded); nothing is owed for them.", unlisted)
	}
	if len(section.Captures) == 0 && unlisted == 0 {
		return nil, offers, 0
	}
	return section, offers, unlisted
}

// heldVerdicts are the three answers for an offered held story: the same
// happening, a follow-up that adds a real development, a different happening.
var heldVerdicts = map[string]bool{"same": true, "follow_up": true, "different": true}

type heldCheckEntry struct {
	Story   string `json:"story"`
	Verdict string `json:"verdict"`
}

// heldPacket is the part of a texture:new Cast's spec the held check reads.
type heldPacket struct {
	Captures  []string         `json:"captures"`
	HeldCheck []heldCheckEntry `json:"held_check"`
}

func parseHeldPacket(packet string) (heldPacket, bool) {
	var p heldPacket
	packet = strings.TrimSpace(packet)
	if packet == "" || json.Unmarshal([]byte(packet), &p) != nil {
		return p, false
	}
	return p, true
}

func docOfStoryRef(ref string) string {
	return strings.TrimPrefix(strings.TrimSpace(ref), deskmanifest.DocumentCustodianName()+":")
}

// offeredFor lists the held stories offered for captures (in the captures'
// order, once each); with no captures named, every offer, by capture id.
func (o *openingOffer) offeredFor(captures []string) []string {
	if o == nil || len(o.ByCapture) == 0 {
		return nil
	}
	if len(captures) == 0 {
		for id := range o.ByCapture {
			captures = append(captures, id)
		}
		sort.Strings(captures)
	}
	var out []string
	seen := map[string]bool{}
	for _, id := range captures {
		for _, doc := range o.ByCapture[strings.TrimSpace(id)] {
			if !seen[doc] {
				seen[doc] = true
				out = append(out, doc)
			}
		}
	}
	return out
}

// requireHeldVerdicts refuses a texture:new Cast that leaves a held story
// offered for its captures without a verdict, or gives one outside same,
// follow_up and different. It never refuses an open for what the verdict is.
func requireHeldVerdicts(offer *openingOffer, packet string) error {
	p, ok := parseHeldPacket(packet)
	if !ok || offer == nil {
		return nil
	}
	owed := offer.offeredFor(p.Captures)
	if len(owed) == 0 {
		return nil
	}
	answered := map[string]bool{}
	for _, e := range p.HeldCheck {
		if !heldVerdicts[strings.TrimSpace(e.Verdict)] {
			return fmt.Errorf("held_check verdict %q for %s is not one of same, follow_up, different", e.Verdict, strings.TrimSpace(e.Story))
		}
		answered[docOfStoryRef(e.Story)] = true
	}
	var missing []string
	for _, doc := range owed {
		if !answered[doc] {
			missing = append(missing, deskmanifest.DocumentInbox(doc))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("a Cast to %s must answer, for each held story offered for its captures, whether it is the same happening, a follow_up or different: add to the spec \"held_check\": [{\"story\": \"%s\", \"verdict\": \"same|follow_up|different\"}] for %s (nothing was sent; read the held stories you were handed, then cast again)", types.StoryOpenInbox(), deskmanifest.DocumentInbox("<doc>"), strings.Join(missing, ", "))
	}
	return nil
}

// choiceDetail is what a decision's log line adds to the run's counts.
type choiceDetail struct {
	Window   time.Duration
	InWindow int
	LeftOut  int
	Offered  []string
	// Verdicts is the answers by document id; nil when the act is not asked
	// for any (a feed).
	Verdicts map[string]string
}

func (d choiceDetail) offeredText() string {
	if len(d.Offered) == 0 {
		return "-"
	}
	return strings.Join(d.Offered, ",")
}

func (d choiceDetail) verdictsText() string {
	if len(d.Offered) == 0 || d.Verdicts == nil {
		return "-"
	}
	parts := make([]string, 0, len(d.Offered))
	for _, doc := range d.Offered {
		v := d.Verdicts[doc]
		if v == "" {
			v = "none"
		}
		parts = append(parts, doc+":"+v)
	}
	return strings.Join(parts, ",")
}

// openingOfferOf is the offer the run's opening carried, nil when it had none.
func (rt *Runtime) openingOfferOf(runID string) *openingOffer {
	if rt == nil {
		return nil
	}
	if v, ok := rt.openingOffers.Load(strings.TrimSpace(runID)); ok {
		o, _ := v.(*openingOffer)
		return o
	}
	return nil
}

// detailOfDecision reads a decision's detail from the run's offer and the
// captures the act names (an open's spec, a feed's captures).
func (rt *Runtime) detailOfDecision(runID string, in stagedDecision) choiceDetail {
	offer := rt.openingOfferOf(runID)
	d := choiceDetail{}
	if offer == nil {
		return d
	}
	d.Window, d.InWindow, d.LeftOut = offer.Window, offer.InWindow, offer.LeftOut
	captures := in.captures
	if in.packet != "" {
		if p, ok := parseHeldPacket(in.packet); ok {
			captures = p.Captures
			d.Verdicts = map[string]string{}
			for _, e := range p.HeldCheck {
				d.Verdicts[docOfStoryRef(e.Story)] = strings.TrimSpace(e.Verdict)
			}
		}
	}
	d.Offered = offer.offeredFor(captures)
	return d
}

// stagedDecision is the part of a staged act the log line reads.
type stagedDecision struct {
	captures []string
	packet   string
}
