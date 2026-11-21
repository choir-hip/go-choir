package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// A story's research trajectory (docs/design/semantic-layer-2026-11-20.md
// section 3, the owner's rulings of 2026-11-21: captures join existing
// trajectories with their accumulated context).
//
// What the code keeps today. A story has one research agent inbox
// (research:<doc>) and one conversation with its Texture
// (texture:<doc> to research:<doc>); every Texture Cast to research opens a
// new round and wakes a NEW run with a fresh session and a fresh corpus brief
// (research_brief.go builds it from the Cast's objective alone). The pages an
// earlier run read survive only as captures in corpusd and, since the search
// leads landed, as leads marked tried with the capture they made and the
// conversations whose searches returned them. Nothing carried them into the
// next run's first message; a fed capture reached the story as a record and
// nothing asked that it be read before research began again.
//
// What this file adds, all host-side and no model call:
//
//   - storyResearchContext: for a research run on a story, the pages earlier
//     runs on that story read (leads of its conversation), the captures the
//     story holds (opened with or fed), the outlets those hold, and, through
//     the story's succeeds ref, the same for the story it continues. It is
//     appended to the run's first message beside the corpus brief. The
//     read-first rule (search_decision.go) takes the same pages as HELD pages
//     of the run's root session, in FRONT of the corpus brief's (noteStoryHeld):
//     the run tries what the story already holds, newest fed capture first,
//     before its first web search. The rule demands the first five of that one
//     search zero (story pages, then the brief's), the same five it demands of
//     any search; the section lists up to 40 so the model sees the rest, and
//     is not required to read them. Pages whose URL is not known (a capture
//     the store did not return) are listed but not held: the rule keys a page
//     by its URL. Owner, 2026-11-21: "the more that we send a capture to
//     existing trajectories with accumulated context, the better" and "we
//     should be reading much more and searching less".
//   - requireFedCapturesRead: a Texture's Cast to research for a story that
//     already had research is refused while captures fed since its last
//     research Cast are not yet read by it. The same shape as the need test's
//     holds_unread: the reason and the capture ids to read.

const (
	// storyContextMaxPages bounds the pages one story's section lists; the
	// section also states how many it left out.
	storyContextMaxPages = 40
	// storyContextMaxOutlets bounds the outlets listed.
	storyContextMaxOutlets = 20
	// storyMaterialMaxRecords is how many of a story's newest inbox records its
	// material reads.
	storyMaterialMaxRecords = 300
	// storyContextMaxPredecessorPages is the share of the section a predecessor
	// story's pages take (a story's own take storyContextMaxPages).
	storyContextMaxPredecessorPages = 20
	// storyContextReadTimeout bounds each capture read of the context,
	// storyContextReadTotal the capture reads of one story and
	// storyContextLeadTimeout its one lead read; storyContextTotal bounds the
	// whole section, the successor's part included: the context never delays a
	// research start by more than storyContextTotal, and what it did not reach
	// in time is left out (the lead read logs a timeout).
	storyContextReadTimeout = 2 * time.Second
	storyContextReadTotal   = 6 * time.Second
	storyContextLeadTimeout = 3 * time.Second
	storyContextTotal       = 10 * time.Second
	// storyContextURLRunes clips a page's URL.
	storyContextURLRunes = 200
	// storyContextMaxFed bounds the fed captures the section names.
	storyContextMaxFed = 10
	// storyContextTitleRunes clips a provider's title.
	storyContextTitleRunes = 120
)

// storyHeldPage is one page a story already holds, in the shape of the
// read-first lane's yaegikernel.HeldPage (URL, Title, CaptureID).
type storyHeldPage struct {
	URL       string
	Title     string
	CaptureID string
}

// storyMaterial is what a story holds: the captures it was opened with or fed,
// the ones fed since its last research Cast, and whether it had research.
type storyMaterial struct {
	Captures     []string
	FedSince     []string
	HasResearch  bool
	LastResearch time.Time
}

// storyDocOfDeskAgent is the story a Texture desk agent serves: its agent id
// is texture:<doc>. The read marks and the refusal both derive it here.
func storyDocOfDeskAgent(agentID string) string {
	return strings.TrimPrefix(strings.TrimSpace(agentID), deskmanifest.DocumentCustodianName()+":")
}

func storyRecordTime(rec types.CommitmentRecord) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(rec.Provenance.CommittedAt))
	return t
}

// storyMaterialOf reads the story's own inbox (the captures it was opened with
// and fed, each record's time) and its conversation with research (the times
// of its Texture's research Casts).
func (rt *Runtime) storyMaterialOf(ctx context.Context, ownerID, computerID, doc string) (storyMaterial, error) {
	var m storyMaterial
	inbox := deskmanifest.DocumentInbox(doc)
	// Only the newest storyMaterialMaxRecords records are read: a story with
	// hundreds of records must not delay a research start. Older captures are
	// older material, and the fed-since set is always recent.
	var after int64
	if head, err := rt.store.InboxHead(ctx, ownerID, computerID, inbox); err == nil && head > storyMaterialMaxRecords {
		after = head - storyMaterialMaxRecords
	}
	recs, err := rt.store.ListInboxRecords(ctx, ownerID, computerID, inbox, after, 0)
	if err != nil {
		return m, err
	}
	conv, err := rt.store.ConversationRecords(ctx, ownerID, computerID, types.InboxConversationID(inbox, deskmanifest.ResearchInbox(doc)))
	if err != nil {
		return m, err
	}
	for _, rec := range conv {
		if rec.IsCast() && strings.TrimSpace(rec.SenderInbox) == inbox {
			m.HasResearch = true
			if t := storyRecordTime(rec); t.After(m.LastResearch) {
				m.LastResearch = t
			}
		}
	}
	seen := map[string]bool{}
	for _, rec := range recs {
		var ids []string
		switch {
		case rec.Disposition != nil && rec.Disposition.Kind == types.DispositionFeed:
			ids = rec.Disposition.Captures
		case rec.IsCast() && rec.Cast != nil && strings.HasPrefix(rec.SenderInbox, deskmanifest.AdmitterName()+":"):
			ids = rec.Cast.Captures
		}
		for _, id := range ids {
			if id = strings.TrimSpace(id); id == "" || seen[id] {
				continue
			}
			seen[id] = true
			m.Captures = append(m.Captures, id)
			if m.HasResearch && storyRecordTime(rec).After(m.LastResearch) {
				m.FedSince = append(m.FedSince, id)
			}
		}
	}
	return m, nil
}

// noteStoryCaptureRead records that a story's Texture asked to read a capture
// (an attempt counts: a read the gate refused is not owed again).
func (rt *Runtime) noteStoryCaptureRead(ownerID, computerID, doc, captureID string) {
	doc, captureID = strings.TrimSpace(doc), strings.TrimSpace(captureID)
	if rt == nil || doc == "" || captureID == "" {
		return
	}
	key := ownerID + "\x00" + computerID + "\x00" + doc
	v, _ := rt.storyReads.LoadOrStore(key, &syncSet{})
	v.(*syncSet).add(captureID)
}

func (rt *Runtime) storyCaptureWasRead(ownerID, computerID, doc, captureID string) bool {
	v, ok := rt.storyReads.Load(ownerID + "\x00" + computerID + "\x00" + doc)
	return ok && v.(*syncSet).has(strings.TrimSpace(captureID))
}

// storyResearchRefusalLimit bounds how often one set of unread fed captures
// refuses a story's research Cast. A refusal is a correctness check, not a
// switch, but a Texture that is told and never reads would be woken again by
// the fed record it left unread and refused again for ever: the refusal after
// the limit is released (and logged), so the worst case for one fed set is this
// many refused attempts.
const storyResearchRefusalLimit = 3

// unreadFedCaptures is the captures fed to a story since its last research
// Cast that its Texture has not read, nil when there is no earlier research,
// when none is unread, when the refusal limit for this set is spent, or when
// the store could not be read (the gate then fails open, because a store blip
// must not block a story, and says so in a "story research gate skipped" line
// that the status script counts). A non-empty result is a refusal: it is
// counted, logged and told the model.
func (rt *Runtime) unreadFedCaptures(ctx context.Context, ownerID, computerID, doc, runID string) []string {
	m, err := rt.storyMaterialOf(ctx, ownerID, computerID, doc)
	if err != nil {
		log.Print(storyResearchGateSkippedLine(runID, doc, err))
		return nil
	}
	if !m.HasResearch {
		return nil
	}
	var unread []string
	for _, id := range m.FedSince {
		if !rt.storyCaptureWasRead(ownerID, computerID, doc, id) {
			unread = append(unread, id)
		}
	}
	if len(unread) == 0 {
		return nil
	}
	key := ownerID + "\x00" + computerID + "\x00" + doc + "\x00" + strings.Join(unread, ",")
	v, _ := rt.storyRefusals.LoadOrStore(key, new(atomic.Int32))
	if n := v.(*atomic.Int32).Add(1); n > storyResearchRefusalLimit {
		log.Printf("story research gate released run=%s story=%s unread=%d refused=%d", runID, doc, len(unread), n-1)
		return nil
	}
	log.Print(storyResearchRefusedLine(runID, doc, len(unread)))
	return unread
}

func storyResearchGateSkippedLine(runID, story string, err error) string {
	return fmt.Sprintf("story research gate skipped run=%s story=%s err=%q", runID, story, err.Error())
}

func unreadFedMessage(unread []string) string {
	return fmt.Sprintf("research for this story was last asked before %d captures were fed to it that you have not read: read what you were fed before you research again (choir.ReadCapture on each of %s), revise from them, and cast research only for what they leave open", len(unread), strings.Join(unread, ", "))
}

// requireFedCapturesRead refuses a story Texture's Cast to research while
// captures fed since its last research Cast are unread by it. A story with no
// earlier research is never refused (its opening captures are its first
// material, and the open path's Texture reads them as it writes V1).
func (rt *Runtime) requireFedCapturesRead(ctx context.Context, ownerID, computerID, doc, runID string) error {
	unread := rt.unreadFedCaptures(ctx, ownerID, computerID, doc, runID)
	if len(unread) == 0 {
		return nil
	}
	return fmt.Errorf("%s (nothing was sent)", unreadFedMessage(unread))
}

func storyResearchRefusedLine(runID, story string, unread int) string {
	return fmt.Sprintf("story research refused run=%s story=%s unread=%d", runID, story, unread)
}

// storyPages gathers the pages a story holds, at most max: the captures it was
// opened with or fed (newest first, their URLs read from the capture store,
// private ones left out) and the leads its conversation's searches returned and
// a run read. The budget is shared: leads take at most half of it while the
// story has captures to show, so pages read earlier never push out what the
// story was just fed.
func (rt *Runtime) storyPages(ctx context.Context, ownerID, computerID, doc string, max int) (pages []storyHeldPage, held int, fed []string) {
	var captures any
	if rt.researchDeps != nil {
		captures = rt.researchDeps.Captures
	}
	var leads []storyHeldPage
	if lr, ok := captures.(researchtools.SearchLeadPageReader); ok {
		start := time.Now()
		callCtx, cancel := context.WithTimeout(ctx, storyContextLeadTimeout)
		got, err := lr.SearchLeadPages(callCtx, ownerID, computerID, doc, max)
		timedOut := callCtx.Err() == context.DeadlineExceeded
		cancel()
		if err != nil {
			// A timeout drops the story's read pages from the section: it is
			// logged as one, so a read that grows past its bound is seen.
			log.Printf("story research context: lead pages failed story=%s elapsed=%s timeout=%t: %v", doc, time.Since(start).Round(time.Millisecond), timedOut, err)
		}
		for _, p := range got {
			if p.CaptureID != "" {
				leads = append(leads, storyHeldPage{URL: p.URL, Title: p.Title, CaptureID: p.CaptureID})
			}
		}
	}
	m, err := rt.storyMaterialOf(ctx, ownerID, computerID, doc)
	if err != nil {
		return takePages(nil, leads, max), len(leads), nil
	}
	cr, _ := captures.(interface {
		ReadCapture(ctx context.Context, captureID string, fromBlock, maxChars int) (json.RawMessage, error)
	})
	readCtx, cancelReads := context.WithTimeout(ctx, storyContextReadTotal)
	defer cancelReads()
	// Newest first, and only as many as the share allows: the pages the story
	// holds beyond that are found with choir.SearchWireCorpus.
	slots := max - len(leads)
	if half := max / 2; slots < half {
		slots = half
	}
	var own []storyHeldPage
	for i := len(m.Captures) - 1; i >= 0 && len(own) < slots; i-- {
		id := m.Captures[i]
		page := storyHeldPage{CaptureID: id}
		if cr != nil && readCtx.Err() == nil {
			callCtx, cancel := context.WithTimeout(readCtx, storyContextReadTimeout)
			raw, err := cr.ReadCapture(callCtx, id, 0, 1)
			cancel()
			if err == nil {
				var meta struct {
					URL    string   `json:"url"`
					Labels []string `json:"labels"`
				}
				if json.Unmarshal(raw, &meta) == nil {
					if types.NewLabelSet(meta.Labels...).OwnerPrivate() {
						continue
					}
					page.URL = meta.URL
				}
			}
		}
		own = append(own, page)
	}
	pages = takePages(own, leads, max)
	return pages, len(pages), m.FedSince
}

// takePages is the story's own captures, then as many leads as the budget has
// left, each capture once.
func takePages(own, leads []storyHeldPage, max int) []storyHeldPage {
	var out []storyHeldPage
	seen := map[string]bool{}
	for _, p := range own {
		if !seen[p.CaptureID] && len(out) < max {
			seen[p.CaptureID] = true
			out = append(out, p)
		}
	}
	room := max - len(out)
	var fill []storyHeldPage
	for _, p := range leads {
		if !seen[p.CaptureID] && len(fill) < room {
			seen[p.CaptureID] = true
			fill = append(fill, p)
		}
	}
	// The story's own captures, the newest fed among them, come first.
	return append(out, fill...)
}

func outletOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// storyResearchContext is the section a research run on a story gets.
type storyResearchContext struct {
	text string
	// held are the pages the section lists, with a known URL, the story's own
	// first and then its predecessor's: the run's search zero (noteStoryHeld).
	held []yaegikernel.HeldPage
}

func heldOf(pages []storyHeldPage) []yaegikernel.HeldPage {
	var out []yaegikernel.HeldPage
	for _, p := range pages {
		if strings.TrimSpace(p.URL) != "" && p.CaptureID != "" {
			out = append(out, yaegikernel.HeldPage{URL: p.URL, Title: p.Title, CaptureID: p.CaptureID})
		}
	}
	return out
}

func (c *storyResearchContext) openingText() string {
	if c == nil || strings.TrimSpace(c.text) == "" {
		return ""
	}
	return "\n\n" + frameOpeningSection(worldReadLabels, c.text)
}

func clipTitle(s string) string {
	return intakeClip(strings.Join(strings.Fields(s), " "), storyContextTitleRunes)
}

// describeStoryPages writes one story's held pages and outlets.
func describeStoryPages(b *strings.Builder, pages []storyHeldPage) {
	outlets := map[string]int{}
	for _, p := range pages {
		if o := outletOf(p.URL); o != "" {
			outlets[o]++
		}
	}
	shown := pages
	if len(shown) > storyContextMaxPages {
		shown = shown[:storyContextMaxPages]
	}
	for _, p := range shown {
		fmt.Fprintf(b, "- capture_id %s", p.CaptureID)
		if p.URL != "" {
			fmt.Fprintf(b, " %s", intakeClip(p.URL, storyContextURLRunes))
		}
		if t := clipTitle(p.Title); t != "" {
			fmt.Fprintf(b, " (%s)", t)
		}
		b.WriteString("\n")
	}
	if len(pages) > len(shown) {
		fmt.Fprintf(b, "(%d more pages are held and not listed; choir.SearchWireCorpus finds them)\n", len(pages)-len(shown))
	}
	type oc struct {
		name string
		n    int
	}
	var list []oc
	for o, n := range outlets {
		list = append(list, oc{o, n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].name < list[j].name
	})
	if len(list) > 0 {
		b.WriteString("Outlets held:")
		for i, o := range list {
			if i == storyContextMaxOutlets {
				fmt.Fprintf(b, " (+%d more)", len(list)-i)
				break
			}
			fmt.Fprintf(b, " %s (%d)", o.name, o.n)
		}
		b.WriteString("\n")
	}
}

// storyResearchContextOpening builds the section for a story's research run,
// or nil when the run is no story's or the story holds nothing yet. It marks
// nothing and calls no model.
func (rt *Runtime) storyResearchContextOpening(ctx context.Context, rec *types.RunRecord) *storyResearchContext {
	if rt == nil || rt.store == nil || rec == nil || !deskmanifest.IsResearchInbox(agentIDForRun(rec)) || !deskmanifest.SpawnsResearch(agentProfileForRun(rec)) {
		return nil
	}
	doc := strings.TrimSpace(channelIDForRun(rec))
	if doc == "" || strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.ComputerID) == "" {
		return nil
	}
	ctx = withDeskReader(ctx, rec.AgentProfile)
	ctx, cancelTotal := context.WithTimeout(ctx, storyContextTotal)
	defer cancelTotal()
	pages, _, fed := rt.storyPages(ctx, rec.OwnerID, rec.ComputerID, doc, storyContextMaxPages)
	var b strings.Builder
	found := false
	held := heldOf(pages)
	mainPoint, succeeds := rt.store.StoryPointOf(ctx, rec.OwnerID, rec.ComputerID, doc)
	if len(pages) > 0 {
		b.WriteString("Story research so far. This story's earlier research runs read, and the story holds, these pages; read one with choir.ReadCapture(capture_id) before you search the web for what it already answers.\n")
		if mainPoint != "" {
			fmt.Fprintf(&b, "The story's main point: %s\n", intakeClip(mainPoint, mainPointMaxRunes))
		}
		describeStoryPages(&b, pages)
		found = true
		if len(fed) > 0 {
			shown := fed
			if len(shown) > storyContextMaxFed {
				shown = shown[:storyContextMaxFed]
			}
			fmt.Fprintf(&b, "Fed since the last research round: %s", strings.Join(shown, ", "))
			if len(fed) > len(shown) {
				fmt.Fprintf(&b, " (+%d more)", len(fed)-len(shown))
			}
			b.WriteString(".\n")
		}
	}
	if succeeds != "" {
		prior, _, _ := rt.storyPages(ctx, rec.OwnerID, rec.ComputerID, succeeds, storyContextMaxPredecessorPages)
		if len(prior) > 0 {
			pm, _ := rt.store.StoryPointOf(ctx, rec.OwnerID, rec.ComputerID, succeeds)
			b.WriteString("\nThis story succeeds an earlier one (its main beat moved on); the earlier story's research read and holds these pages, so start from them.\n")
			if pm != "" {
				fmt.Fprintf(&b, "The earlier story's main point: %s\n", intakeClip(pm, mainPointMaxRunes))
			}
			describeStoryPages(&b, prior)
			held = append(held, heldOf(prior)...)
			found = true
		}
	}
	if !found {
		return nil
	}
	return &storyResearchContext{text: strings.TrimSpace(b.String()), held: held}
}

// syncSet is a set of ids safe for concurrent use.
type syncSet struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (s *syncSet) add(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids == nil {
		s.ids = map[string]bool{}
	}
	s.ids[id] = true
}

func (s *syncSet) has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ids[id]
}
