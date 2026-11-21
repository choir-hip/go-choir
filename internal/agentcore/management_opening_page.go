package agentcore

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The story page a management task is handed, and the candidate stories
// offered for its arrivals (docs/design/semantic-layer-2026-11-20.md, landing
// A; the failure is the paper opening a second story on a main point it
// already holds, one published story in five).
//
// The page is every story updated in the last openingWindow, as headlines
// only, built to fit the section bound: stories are added newest first until
// the bound is reached, so the page fits by construction, and when the window
// holds more than fits the page says how many it left out and how to read
// them. There is no fallback to a count of stories.
//
// Beside it, for each arrival, the stories its nearest captures already
// belong to (found by vectors) and the stories whose research searches
// returned its page (found by search leads), at any age, each with its main
// point. Never by title words. The task answers for each candidate in the
// spec of its texture:new Cast (judgment); a capture that serves a held story
// is fed to it, so the host refuses an open that answers "same".

const (
	// openingWindowHours is the window the prompt, the page and the log name.
	openingWindowHours = 8
	openingWindow      = openingWindowHours * time.Hour
	// candidateHorizon is how far back a capture's neighbours are read (the
	// event layer's horizon). A story is offered at any age; recency only
	// ranks (design section 9: six of eleven "same" pairs are 25 to 54 hours
	// apart).
	candidateHorizon = 14 * 24 * time.Hour
	// candidateNeighbourLimit is the nearest captures asked for each arrival,
	// in one CaptureNeighbours request for the whole task.
	candidateNeighbourLimit = 12
	// candidateNeighboursTimeout bounds that request; on failure the page alone
	// is shown, with a note, never a refused open.
	candidateNeighboursTimeout = 8 * time.Second
	// candidatesPerCapture is the most candidate stories offered for one arrival.
	candidatesPerCapture = 12
	// candidatePointRunes clips a main point (or the lede that stands in).
	candidatePointRunes = 240
	// openingHeadlineRunes clips a headline of the page, so one oversized
	// headline cannot take the page's bound.
	openingHeadlineRunes = 300
	// mainPointMinRunes and mainPointMaxRunes bound the one sentence a story's
	// opening Cast records as its main point.
	mainPointMinRunes = 20
	mainPointMaxRunes = 240
)

const (
	openingStoriesHeading   = "Stories updated in the last 8 hours, headlines only:"
	openingCandidateHeading = "Candidate stories for these captures (found by nearest captures and by search leads, at any age; answer each in judgment if you open):"
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
	return fmt.Sprintf("%d stories updated in the last %s are not listed: the page is bounded, and the oldest listed is %s old. Read them, with ledes, with choir.Stories(50, next); the cursor goes on past the oldest listed, so keep following the next of each page for the rest.", left, shortDuration(window), oldest)
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
		return openingStory{Inbox: deskmanifest.DocumentInbox(s.DocID), Headline: intakeClip(s.Headline, openingHeadlineRunes), Age: storyAge(s.UpdatedAt, now), Version: s.Version}
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

// candidateStory is a held story that may already hold an arrival's main point.
type candidateStory struct {
	DocID string `json:"-"`
	Inbox string `json:"inbox"`
	// Headline is the story's headline; MainPoint the one sentence its opening
	// recorded, or, for a story opened before main points, its lede with
	// PointIsLede set.
	Headline    string `json:"headline"`
	MainPoint   string `json:"main_point"`
	PointIsLede bool   `json:"point_is_lede,omitempty"`
	Age         string `json:"age"`
	Version     int    `json:"v"`
	// Held counts the arrival's nearest captures this story holds: the
	// trajectory with the most accumulated context on the matter has the most.
	Held int `json:"held,omitempty"`
	// FoundBy says how it was found: "near:<n>" (n nearest captures), "lead".
	FoundBy []string `json:"found_by"`
}

// arrivalLinks are the stories an arrival is linked to, as documents.
type arrivalLinks struct {
	// Near has one entry per nearest capture that belongs to a story, in
	// neighbour order, so a story repeats once for each neighbour it holds.
	Near []string
	// Leads are the stories whose research searches returned the arrival's page.
	Leads []string
}

// openingCandidateCapture is one arrival and the stories that may hold its main point.
type openingCandidateCapture struct {
	CaptureID  string           `json:"capture_id"`
	Title      string           `json:"title"`
	Candidates []candidateStory `json:"candidate_stories"`
}

// openingCandidateSection is the candidate stories offered, by arrival.
type openingCandidateSection struct {
	Captures []openingCandidateCapture `json:"captures"`
	Note     string                    `json:"note,omitempty"`
}

// candidateSources are how many candidates were offered by each source, for
// the decision's log line.
type candidateSources struct{ Near, Lead int }

// openingOffer is what a task's opening offered, kept for the run: the page's
// size and the candidate stories offered for each capture (document ids), which
// the task's open must answer and the decision's log line names.
type openingOffer struct {
	Window    time.Duration
	InWindow  int
	Shown     int
	LeftOut   int
	ByCapture map[string][]string
	// Found is how each offered candidate was found: capture id, then document.
	Found      map[string]map[string][]string
	HeldUnlist int
	// Leased are the captures the task leases (the arrivals of its records,
	// whether or not the records section fitted).
	Leased []string
}

func candidateOf(sum store.StorySummary, now time.Time, near int, lead bool) candidateStory {
	c := candidateStory{DocID: sum.DocID, Inbox: deskmanifest.DocumentInbox(sum.DocID),
		Headline: intakeClip(strings.Join(strings.Fields(sum.Headline), " "), openingHeadlineRunes),
		Age:      storyAge(sum.UpdatedAt, now), Version: sum.Version, Held: near}
	if mp := strings.TrimSpace(sum.MainPoint); mp != "" {
		c.MainPoint = intakeClip(mp, candidatePointRunes)
	} else {
		c.MainPoint, c.PointIsLede = intakeClip(sum.Lede, candidatePointRunes), true
	}
	if near > 0 {
		c.FoundBy = append(c.FoundBy, fmt.Sprintf("near:%d", near))
	}
	if lead {
		c.FoundBy = append(c.FoundBy, "lead")
	}
	return c
}

// buildCandidateSection offers, for each arrival with a candidate, up to
// candidatesPerCapture stories, within budgetTokens (an arrival whose stories
// do not fit is said to be left out, and nothing is owed for it). Candidates
// are ranked by how many of the arrival's nearest captures (and leads) the
// story holds, then by version (the context the trajectory has accumulated on
// the matter), then newest. Nothing here reads a title.
func buildCandidateSection(arrivals []types.CaptureArrival, links map[string]arrivalLinks, byDoc map[string]store.StorySummary, now time.Time, budgetTokens int) (*openingCandidateSection, *openingOffer, candidateSources, int) {
	section := &openingCandidateSection{Captures: []openingCandidateCapture{}}
	offer := &openingOffer{ByCapture: map[string][]string{}, Found: map[string]map[string][]string{}}
	var src candidateSources
	seen := map[string]bool{}
	bytes, unlisted := 64, 0
	for _, a := range arrivals {
		id := strings.TrimSpace(a.CaptureID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		l := links[id]
		near := map[string]int{}
		for _, doc := range l.Near {
			near[doc]++
		}
		lead := map[string]bool{}
		for _, doc := range l.Leads {
			lead[doc] = true
		}
		var cands []candidateStory
		for doc := range near {
			if sum, ok := byDoc[doc]; ok {
				cands = append(cands, candidateOf(sum, now, near[doc], lead[doc]))
			}
		}
		for doc := range lead {
			if sum, ok := byDoc[doc]; ok && near[doc] == 0 {
				cands = append(cands, candidateOf(sum, now, 0, true))
			}
		}
		sort.SliceStable(cands, func(i, j int) bool {
			si, sj := cands[i].Held+leadWeight(cands[i]), cands[j].Held+leadWeight(cands[j])
			if si != sj {
				return si > sj
			}
			if cands[i].Version != cands[j].Version {
				return cands[i].Version > cands[j].Version
			}
			if ui, uj := byDoc[cands[i].DocID].UpdatedAt, byDoc[cands[j].DocID].UpdatedAt; !ui.Equal(uj) {
				return ui.After(uj)
			}
			return cands[i].DocID < cands[j].DocID
		})
		if len(cands) > candidatesPerCapture {
			cands = cands[:candidatesPerCapture]
		}
		if len(cands) == 0 {
			continue
		}
		c := openingCandidateCapture{CaptureID: id, Title: intakeClip(a.Title, openingHeadlineRunes), Candidates: cands}
		raw, _ := json.Marshal(c)
		if estimateTextTokens(strings.Repeat("x", bytes+len(raw)+400)) > budgetTokens {
			unlisted++
			continue
		}
		bytes += len(raw) + 1
		section.Captures = append(section.Captures, c)
		offer.Found[id] = map[string][]string{}
		for _, cand := range cands {
			offer.ByCapture[id] = append(offer.ByCapture[id], cand.DocID)
			offer.Found[id][cand.DocID] = cand.FoundBy
			if cand.Held > 0 {
				src.Near++
			}
			if lead[cand.DocID] {
				src.Lead++
			}
		}
	}
	if unlisted > 0 {
		section.Note = fmt.Sprintf("%d captures with candidate stories are not listed here (the section is bounded); nothing is owed for them.", unlisted)
	}
	offer.HeldUnlist = unlisted
	if len(section.Captures) == 0 && unlisted == 0 {
		return nil, offer, src, 0
	}
	return section, offer, src, unlisted
}

func leadWeight(c candidateStory) int {
	for _, f := range c.FoundBy {
		if f == "lead" {
			return 1
		}
	}
	return 0
}

// The answers for an offered candidate. "same" is the main point of the held
// story: the capture is fed to it, so an open never answers it.
var judgmentAnswers = map[string]bool{"same": true, "next": true, "new": true}

type judgmentEntry struct {
	Story  string `json:"story"`
	Answer string `json:"answer"`
	Why    string `json:"why"`
}

// judgmentPacket is the part of a texture:new Cast's spec the judgment reads.
type judgmentPacket struct {
	Captures  []string        `json:"captures"`
	MainPoint string          `json:"main_point"`
	Succeeds  string          `json:"succeeds"`
	Judgment  json.RawMessage `json:"judgment"`
	// Entries and Bad are Judgment as read.
	Entries []judgmentEntry `json:"-"`
	Bad     bool            `json:"-"`
}

func parseJudgmentPacket(packet string) (judgmentPacket, bool) {
	var p judgmentPacket
	packet = strings.TrimSpace(packet)
	if packet == "" || json.Unmarshal([]byte(packet), &p) != nil {
		return p, false
	}
	if raw := strings.TrimSpace(string(p.Judgment)); raw != "" && raw != "null" {
		if json.Unmarshal(p.Judgment, &p.Entries) != nil {
			p.Bad = true
		}
	}
	return p, true
}

func docOfStoryRef(ref string) string {
	return strings.TrimPrefix(strings.TrimSpace(ref), deskmanifest.DocumentCustodianName()+":")
}

// offeredFor lists the candidate stories offered for captures (in the captures'
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

// judgmentShape is the spec fields an open carries.
func judgmentShape() string {
	return fmt.Sprintf(`"main_point": "<one sentence, %d to %d characters, at the level of the beat>", "judgment": [{"story": "%s", "answer": "next|new", "why": "<why>"}], "succeeds": "%s" (only with a next answer)`,
		mainPointMinRunes, mainPointMaxRunes, deskmanifest.DocumentInbox("<doc>"), deskmanifest.DocumentInbox("<doc>"))
}

// requireJudgment refuses a texture:new Cast of a task that has no main point
// of the right size, that leaves a candidate offered for its captures without
// an answer (same, next or new, with a reason), that answers "same" (feed the
// story instead), or whose succeeds does not match its one next answer. Each
// refusal is a correctness refusal naming the shape; nothing is sent. It
// refuses nothing for what the answer is beyond that: the lean is the prompt's.
func requireJudgment(offer *openingOffer, packet string) error {
	p, ok := parseJudgmentPacket(packet)
	if !ok {
		return nil
	}
	shape := judgmentShape()
	leased := ""
	if offer != nil {
		leased = strings.Join(offer.Leased, ", ")
		if len(offer.Leased) == 0 {
			var ids []string
			for id := range offer.ByCapture {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			leased = strings.Join(ids, ", ")
		}
	}
	if n := len([]rune(strings.TrimSpace(p.MainPoint))); n < mainPointMinRunes || n > mainPointMaxRunes {
		return fmt.Errorf("a Cast to %s records the story's main point: add to the spec %s; yours is %d characters (nothing was sent)", types.StoryOpenInbox(), shape, n)
	}
	if p.Bad {
		return fmt.Errorf("judgment must be a list of {story, answer, why} objects, as in %s; the captures you lease are %s (nothing was sent)", shape, leased)
	}
	if offer != nil && len(offer.Leased) > 0 {
		leasedSet := map[string]bool{}
		for _, id := range offer.Leased {
			leasedSet[id] = true
		}
		for _, id := range p.Captures {
			if id = strings.TrimSpace(id); id != "" && !leasedSet[id] {
				return fmt.Errorf("the Cast names capture %s, which this task does not lease; the captures you lease are %s: answer in the spec as in %s (nothing was sent)", id, leased, shape)
			}
		}
	}
	if offer != nil && len(offer.Leased) > 0 && len(nonEmpty(p.Captures)) == 0 {
		return fmt.Errorf("a Cast to %s names the captures it opens with, in the spec's captures (the captures you lease are %s) (nothing was sent)", types.StoryOpenInbox(), leased)
	}
	answers := map[string]string{}
	var nexts []string
	for _, e := range p.Entries {
		ans, doc := strings.TrimSpace(e.Answer), docOfStoryRef(e.Story)
		if !judgmentAnswers[ans] {
			return fmt.Errorf("judgment answer %q for %s is not one of same, next, new (nothing was sent)", e.Answer, strings.TrimSpace(e.Story))
		}
		if ans == "same" {
			return fmt.Errorf("you answered same for %s: a capture that serves a held story's main point is fed to it, not opened: choir.Feed(%q, [capture ids], reason, req) for the captures that serve it, and open only the rest; when it is unclear, feed (nothing was sent)", strings.TrimSpace(e.Story), doc)
		}
		if strings.TrimSpace(e.Why) == "" {
			return fmt.Errorf("judgment for %s needs a why: say in a sentence how its main point differs (nothing was sent)", strings.TrimSpace(e.Story))
		}
		if _, dup := answers[doc]; dup {
			return fmt.Errorf("judgment answers %s twice; answer each candidate once (nothing was sent)", strings.TrimSpace(e.Story))
		}
		answers[doc] = ans
		if ans == "next" {
			nexts = append(nexts, doc)
		}
	}
	if len(nexts) > 1 {
		return fmt.Errorf("a story succeeds one story: two next answers (%s) were given; answer next for the one whose main beat this moves on from and new for the others (nothing was sent)", strings.Join(nexts, ", "))
	}
	succeeds := docOfStoryRef(p.Succeeds)
	switch {
	case succeeds != "" && len(nexts) == 0:
		return fmt.Errorf("succeeds %s needs a next answer for it in judgment, as in %s (nothing was sent)", p.Succeeds, shape)
	case succeeds == "" && len(nexts) == 1:
		return fmt.Errorf("you answered next for %s: name it in the spec as \"succeeds\": %q (nothing was sent)", nexts[0], deskmanifest.DocumentInbox(nexts[0]))
	case succeeds != "" && succeeds != nexts[0]:
		return fmt.Errorf("succeeds %s is not the story you answered next for (%s) (nothing was sent)", p.Succeeds, nexts[0])
	}
	if succeeds != "" {
		offered := false
		for _, doc := range offer.offeredFor(p.Captures) {
			offered = offered || doc == succeeds
		}
		if !offered {
			return fmt.Errorf("succeeds %s is not a candidate story offered for your captures; a story succeeds only one you were handed (nothing was sent)", p.Succeeds)
		}
	}
	var missing []string
	for _, doc := range offer.offeredFor(p.Captures) {
		if answers[doc] == "" {
			missing = append(missing, deskmanifest.DocumentInbox(doc))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("a Cast to %s must answer, for each candidate story offered for its captures, whether the main beat moved on (next) or this is new matter: add to the spec %s for %s; if a capture serves a candidate's main point, feed it there instead, and when it is unclear, feed (nothing was sent; read the candidates you were handed, then cast again)", types.StoryOpenInbox(), shape, strings.Join(missing, ", "))
	}
	return nil
}

// choiceDetail is what a decision's log line adds to the run's counts.
type choiceDetail struct {
	Window   time.Duration
	InWindow int
	LeftOut  int
	Offered  []string
	// Answers is the judgment by document id; nil when the act is not asked
	// for any (a feed or a decline).
	Answers map[string]string
	// Sources counts the candidates offered for the decision's captures by
	// how they were found.
	Sources candidateSources
	// Captures is how many captures the act disposes of.
	Captures int
	// MainPoint is the characters of the opening's main point; Succeeds the
	// document the new story succeeds, "" for none.
	MainPoint int
	Succeeds  string
}

func (d choiceDetail) offeredText() string {
	if len(d.Offered) == 0 {
		return "-"
	}
	return strings.Join(d.Offered, ",")
}

func (d choiceDetail) answersText() string {
	if len(d.Offered) == 0 || d.Answers == nil {
		return "-"
	}
	parts := make([]string, 0, len(d.Offered))
	for _, doc := range d.Offered {
		v := d.Answers[doc]
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
// captures the act names (an open's spec, a feed's or a decline's captures).
func (rt *Runtime) detailOfDecision(runID string, in stagedDecision) choiceDetail {
	d := choiceDetail{Captures: len(in.captures)}
	captures := in.captures
	if in.packet != "" {
		if p, ok := parseJudgmentPacket(in.packet); ok {
			captures = p.Captures
			d.Captures = len(p.Captures)
			d.MainPoint = len([]rune(strings.TrimSpace(p.MainPoint)))
			d.Succeeds = docOfStoryRef(p.Succeeds)
			d.Answers = map[string]string{}
			for _, e := range p.Entries {
				d.Answers[docOfStoryRef(e.Story)] = strings.TrimSpace(e.Answer)
			}
		}
	}
	offer := rt.openingOfferOf(runID)
	if offer == nil {
		return d
	}
	d.Window, d.InWindow, d.LeftOut = offer.Window, offer.InWindow, offer.LeftOut
	d.Offered = offer.offeredFor(captures)
	for _, id := range captures {
		for doc, found := range offer.Found[strings.TrimSpace(id)] {
			_ = doc
			for _, f := range found {
				switch {
				case strings.HasPrefix(f, "near:"):
					d.Sources.Near++
				case f == "lead":
					d.Sources.Lead++
				}
			}
		}
	}
	return d
}

// stagedDecision is the part of a staged act the log line reads.
type stagedDecision struct {
	captures []string
	packet   string
}

func nonEmpty(ids []string) []string {
	var out []string
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}
