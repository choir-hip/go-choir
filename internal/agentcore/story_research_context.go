package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// What a story holds, for the research run on it (docs/design/semantic-layer-2026-11-20.md
// sections 3 and 4). A story has one research agent inbox (research:<doc>) and one
// conversation with its Texture (texture:<doc> to research:<doc>); every Texture Cast to
// research opens a new round and wakes a NEW run with a fresh session. This file reads
// what the story holds from its inboxes (storyMaterialOf: the captures it was opened with
// or fed, and which were fed since its last research Cast), keeps the read bounded to the
// newest records of each inbox (newestInboxRecords), and gates a Texture's Cast to research
// while captures fed since its last research Cast are unread (unreadFedCaptures: the same
// shape as the need test's reason and the capture ids to read). What a research run is
// GIVEN is research_digest.go's.

const (
	// storyContextMaxPages bounds the pages one story's digest section lists; the
	// section also states how many it left out.
	storyContextMaxPages = 40
	// storyMaterialMaxRecords is how many of a story's newest Texture-inbox records its
	// material and its Reports are read from; storyCastWindow how many of the newest
	// records of its research inbox (the Texture's Casts to research). A story with
	// hundreds of records must not make a run's start slower the older the story gets:
	// the read is by (inbox, seq) from a cursor, so it is bounded at the index.
	storyMaterialMaxRecords = 300
	storyCastWindow         = 64
	// storyContextLeadTimeout bounds the one lead read.
	storyContextLeadTimeout = 3 * time.Second
	// storyContextURLRunes clips a page's URL; storyContextTitleRunes a provider's title.
	storyContextURLRunes   = 200
	storyContextTitleRunes = 120
)

// storyHeldPage is one page a story already holds (URL, Title, CaptureID).
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
	// WindowCut says the story's inbox holds older records than the newest
	// storyMaterialMaxRecords this read looked at: Captures is then a floor.
	WindowCut bool
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

// newestInboxRecords reads the newest window records of an inbox (by seq, from a cursor,
// so the read is bounded at the index) and says whether older records were left unread.
func (rt *Runtime) newestInboxRecords(ctx context.Context, ownerID, computerID, inbox string, window int64) (recs []types.CommitmentRecord, cut bool, err error) {
	head, err := rt.store.InboxHead(ctx, ownerID, computerID, inbox)
	if err != nil {
		return nil, false, err
	}
	var after int64
	if head > window {
		after, cut = head-window, true
	}
	recs, err = rt.store.ListInboxRecords(ctx, ownerID, computerID, inbox, after, 0)
	return recs, cut, err
}

// storyMaterialOf reads the story's own inbox (the captures it was opened with and fed,
// each record's time) and its Texture's Casts to research (their times): the newest
// storyMaterialMaxRecords and storyCastWindow records of each. Older captures are older
// material, the fed-since set is always recent, and a Cast older than the window is not
// the last one unless the window holds none (then the story is read as it was before
// research: HasResearch false).
func (rt *Runtime) storyMaterialOf(ctx context.Context, ownerID, computerID, doc string) (storyMaterial, error) {
	var m storyMaterial
	inbox := deskmanifest.DocumentInbox(doc)
	recs, cut, err := rt.newestInboxRecords(ctx, ownerID, computerID, inbox, storyMaterialMaxRecords)
	if err != nil {
		return m, err
	}
	m.WindowCut = cut
	casts, _, err := rt.newestInboxRecords(ctx, ownerID, computerID, deskmanifest.ResearchInbox(doc), storyCastWindow)
	if err != nil {
		return m, err
	}
	for _, rec := range casts {
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

func storyResearchRefusedLine(runID, story string, unread int) string {
	return fmt.Sprintf("story research refused run=%s story=%s unread=%d", runID, story, unread)
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

func clipTitle(s string) string {
	return intakeClip(strings.Join(strings.Fields(s), " "), storyContextTitleRunes)
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
