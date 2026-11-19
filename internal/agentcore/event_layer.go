package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
	"github.com/yusefmosiah/go-choir/internal/jevrecord"
	"github.com/yusefmosiah/go-choir/internal/outlet"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The event layer's V1 (docs/reports/event-layer-at-ingestion-design-
// 2026-10-16.md; owner §24-§25). New captures from every source poll are
// queued and resolved together by Jev onto world events: no desk runs and
// no generative model. An event accumulates captures across outlets, so
// corroboration exists before any story does. V1 is a shadow: nothing
// reads the events except the event view; production still wakes per
// arrival until V3.
//
// Restarts: the queue and the index are memory only. A crash loses the
// queued captures, which stay visibly unresolved (no membership); nothing
// resolves them at boot. The index is rebuilt from the store on first use,
// which is a read, not a resumed obligation.

// Bounds. Vars so a test can shorten them.
var (
	eventFlushInterval = 60 * time.Second
	eventCallTimeout   = 30 * time.Second
	// eventNeighboursTimeout bounds the one neighbours request of a flush; its
	// answer is a shortlist, never a reason to hold the flush.
	eventNeighboursTimeout = 40 * time.Second
)

const (
	eventShortlistRecent = 30
	// eventShortlistVector is how many events a capture's own question offers
	// beyond the recent ones: the events whose member captures are nearest to it
	// by vector (station V5; it replaced the title-overlap slots, which a
	// reworded report of a known event could not reach, and the corroborated
	// overlap slot, which a later report of a developed story needed once about
	// 30 newer events had passed: the vector neighbours reach both).
	eventShortlistVector = 3
	// eventNeighbourLimit is how many nearest captures corpusd is asked for
	// each queued capture; they are mapped to events and the first
	// eventShortlistVector distinct events are offered.
	eventNeighbourLimit = 12
	// eventNeighbourBatch is the most captures one neighbours request names
	// (corpusd's bound, platform.captureNeighboursMaxIDs); a flush of more is
	// sent in the next request.
	eventNeighbourBatch = 5000
	// eventLinkChoices bounds the after_ choices one capture's question
	// offers: the known events sharing the most words with the capture.
	eventLinkChoices = 3
	// eventRecentHorizon bounds the recency slot; eventVectorHorizon bounds how
	// far back a capture's vector neighbours are searched, which reaches the
	// uncorroborated events a slow second report would otherwise miss (panel B2).
	eventRecentHorizon = 72 * time.Hour
	eventVectorHorizon = 14 * 24 * time.Hour
	// eventJoinP joins an event as a counted member; eventTentativeP up to
	// it attaches the capture as a tentative member, which never counts
	// toward the event's outlets and never mints a rival event (panel B1).
	// eventJoinP was 0.85; on staging every join from 0.85 to 0.92 was a
	// topic merge that faked a corroboration (docs/problems/
	// event-layer-false-merges-at-marginal-p-2026-10-16.md).
	eventJoinP      = 0.92
	eventTentativeP = 0.5
	// A call is bounded by the vendor's token limits (jevbudget), not by the
	// 24 KiB guard it had, which came from OpenRouter's "about 32,000 bytes"
	// long-context line (a design choice of 2026-11-12, see the Delivery rules in CLAUDE.md).
	eventSummaryRunes = 400
	eventRecentTitles = 2
	// eventIndexLoad bounds the rebuild read.
	eventIndexLoadEvents      = 10000
	eventIndexLoadMemberships = 20000
)

// eventInput is one capture to resolve.
type eventInput struct {
	Item     sourcegraph.CapturedItem
	SourceID string
	At       time.Time
	// Research is true for a page a research desk read (choir.Read) or polled,
	// not one a source schedule captured.
	Research bool
}

// eventResolveReport is what one resolution did.
type eventResolveReport struct {
	Calls        int
	Joined       int
	Tentative    int
	Minted       int
	Unresolved   int
	Corroborated []string
	CostUSD      float64
	// Created is each event's members this resolution wrote, by event id.
	Created map[string][]string
	// Left are the inputs that ended without a membership.
	Left []eventInput
	// vec is the flush's vector neighbours (nil: none were asked for).
	vec *eventVectors
	// Queued is the questions the flush asked (each waiting capture once:
	// not a duplicate, not one that already had a membership); ResearchReads
	// the ones a research desk read or polled.
	Queued, ResearchReads int
	// ShortRecent is the recent slot's size. vectorEvents are the distinct
	// events the captures' vector neighbours led to across the flush's calls,
	// vectorOnlyEvts those the recent slot did not already offer.
	ShortRecent    int
	vectorEvents   map[string]bool
	vectorOnlyEvts map[string]bool
}

// eventSummary is an event as the index holds it.
type eventSummary struct {
	EventID      string    `json:"event_id"`
	Headline     string    `json:"headline"`
	Outlets      []string  `json:"outlets"`
	Origins      int       `json:"origins"`
	Members      int       `json:"members"`
	Tentative    int       `json:"tentative"`
	Corroborated bool      `json:"corroborated"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	Titles       []string  `json:"titles"`
	CaptureIDs   []string  `json:"capture_ids"`
	// Follows is the earlier event this one is a later development of.
	Follows string `json:"follows,omitempty"`
}

type eventView struct {
	ev        store.WorldEvent
	titles    []string
	captures  []string
	outlets   map[string]bool
	origins   map[string]bool
	members   int
	tentative int
	last      time.Time
	// memberships are the event's members, counted and tentative, in
	// resolution order: an event's captures record names them (§15).
	memberships []store.EventMembership
	// agedWoken is set once the event's aged official wake was minted
	// this boot; the record id keeps a later boot's retry to a replay.
	agedWoken bool
	// corroboratedAt is when the membership that made it corroborated
	// was resolved. settled is set once its records were checked against
	// the ledger and cleared by every new member (event_wakes.go).
	corroboratedAt time.Time
	settled        bool
}

func (v *eventView) corroborated() bool { return len(v.origins) >= store.EventMinOrigins }

func (v *eventView) apply(m store.EventMembership) {
	if m.CapturedAt.After(v.last) {
		v.last = m.CapturedAt
	}
	v.memberships = append(v.memberships, m)
	v.settled = false
	if m.Tentative {
		v.tentative++
		return
	}
	v.members++
	v.titles = append(v.titles, m.Title)
	v.captures = append(v.captures, m.CaptureID)
	if origin := m.CountedOrigin(); origin != "" {
		was := v.corroborated()
		v.outlets[m.Outlet] = true
		v.origins[origin] = true
		if !was && v.corroborated() {
			v.corroboratedAt = m.ResolvedAt
			if v.corroboratedAt.IsZero() {
				v.corroboratedAt = m.CapturedAt
			}
		}
	}
}

func (v *eventView) summary() eventSummary {
	s := eventSummary{EventID: v.ev.EventID, Headline: v.ev.Headline, Origins: len(v.origins), Members: v.members,
		Tentative: v.tentative, Corroborated: v.corroborated(), FirstSeen: v.ev.FirstSeen, LastSeen: v.last,
		Titles: append([]string(nil), v.titles...), CaptureIDs: append([]string(nil), v.captures...), Follows: v.ev.Follows}
	for o := range v.outlets {
		s.Outlets = append(s.Outlets, o)
	}
	sort.Strings(s.Outlets)
	return s
}

type eventIndex struct {
	mu        sync.Mutex // held for a whole resolution: chunks and flushes run in order
	loaded    bool
	events    map[string]*eventView
	byCapture map[string]string
}

type eventQueue struct {
	items    []eventInput
	timer    *time.Timer
	inFlight bool // one flush at a time drains the queue, in order
}

// eventLayer is the runtime's event-layer state.
type eventLayer struct {
	mu      sync.Mutex
	indexes map[string]*eventIndex
	queues  map[string]*eventQueue
	stopped bool
	ctx     context.Context
	cancel  context.CancelFunc
	// lastSweep rate-limits the held-capture sweep per computer; tally is
	// what the last one counted, for management's Stories page.
	lastSweep map[string]time.Time
	tally     map[string]yaegikernel.HeldCaptures
}

func eventKey(ownerID, computerID string) string { return ownerID + "\x00" + computerID }

func (rt *Runtime) eventIndexFor(ownerID, computerID string) *eventIndex {
	rt.events.mu.Lock()
	defer rt.events.mu.Unlock()
	if rt.events.indexes == nil {
		rt.events.indexes = map[string]*eventIndex{}
	}
	key := eventKey(ownerID, computerID)
	idx := rt.events.indexes[key]
	if idx == nil {
		idx = &eventIndex{events: map[string]*eventView{}, byCapture: map[string]string{}}
		rt.events.indexes[key] = idx
	}
	return idx
}

// loadLocked rebuilds the index from the store once. The caller holds
// idx.mu.
func (idx *eventIndex) loadLocked(ctx context.Context, s *store.Store, ownerID, computerID string) error {
	if idx.loaded {
		return nil
	}
	ctx = withKindReader(ctx, "choir.world_event")
	events, err := s.RecentWorldEvents(ctx, ownerID, computerID, eventIndexLoadEvents)
	if err != nil {
		return fmt.Errorf("event layer: load events: %w", err)
	}
	memberships, err := s.RecentEventMemberships(ctx, ownerID, computerID, eventIndexLoadMemberships)
	if err != nil {
		return fmt.Errorf("event layer: load memberships: %w", err)
	}
	for _, ev := range events {
		idx.events[ev.EventID] = &eventView{ev: ev, outlets: map[string]bool{}, origins: map[string]bool{}, last: ev.FirstSeen}
	}
	for _, m := range memberships {
		idx.byCapture[m.CaptureID] = m.EventID
		if v := idx.events[m.EventID]; v != nil {
			v.apply(m)
		}
	}
	// An event whose memberships fell past the load cap still counts its
	// opener's outlet, so a slow second report corroborates it (code
	// review, Gemini). The opener's wire origin is not on the event; the
	// outlet stands in for it.
	for _, v := range idx.events {
		if v.members == 0 && v.ev.OpenedOutlet != "" {
			v.members = 1
			v.outlets[v.ev.OpenedOutlet] = true
			v.origins[v.ev.OpenedOutlet] = true
		}
	}
	idx.loaded = true
	return nil
}

// eventSummary returns one event as the index holds it.
func (rt *Runtime) eventSummary(ctx context.Context, ownerID, computerID, eventID string) (eventSummary, bool) {
	if rt == nil || rt.store == nil {
		return eventSummary{}, false
	}
	idx := rt.eventIndexFor(ownerID, computerID)
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if err := idx.loadLocked(ctx, rt.store, ownerID, computerID); err != nil {
		log.Printf("%v", err)
		return eventSummary{}, false
	}
	v := idx.events[eventID]
	if v == nil {
		return eventSummary{}, false
	}
	return v.summary(), true
}

// eventID is a minted event's identity: its opening capture, so a retried
// mint is the same event.
func eventID(computerID, openingCaptureID string) string {
	sum := sha256.Sum256([]byte(computerID + "\x00" + openingCaptureID))
	return "ev-" + hex.EncodeToString(sum[:10])
}

var wireTag = regexp.MustCompile(`(?i)\((reuters|ap|afp|bloomberg|dpa|upi|ansa|efe|xinhua|kyodo|yonhap|pti|ians|tass|anadolu)\)|^\s*(reuters|afp|ap)\s+[-—–]`)

// wireOrigin names the wire service a capture's lead carries
// ("wire:reuters"), or "" for original reporting. Two outlets carrying one
// wire story are one origin (panel B3).
func wireOrigin(texts ...string) string {
	for _, t := range texts {
		head := []rune(t)
		if len(head) > 300 {
			head = head[:300]
		}
		if m := wireTag.FindStringSubmatch(string(head)); m != nil {
			name := strings.ToLower(firstNonEmpty(m[1], m[2]))
			if name == "ap" {
				name = "associated-press"
			}
			return "wire:" + name
		}
	}
	return ""
}

// resolveEvents resolves inputs onto events, in chunks, in order. A capture
// that already has a membership is skipped, so a retry makes no call. A
// failed call leaves its chunk's captures unresolved.
func (rt *Runtime) resolveEvents(ctx context.Context, ownerID, computerID string, inputs []eventInput) (eventResolveReport, error) {
	var report eventResolveReport
	if rt == nil || rt.store == nil {
		return report, fmt.Errorf("event layer: no store")
	}
	judge := rt.candidateJudge
	if judge == nil {
		return report, fmt.Errorf("event layer: no Jev judge")
	}
	idx := rt.eventIndexFor(ownerID, computerID)
	// The vector shortlist is fetched before the index is held: the request
	// can take its whole timeout, and the management Stories read waits on
	// idx.mu (eventSummary). It is one request for every waiting capture
	// (never the embedder, never a reason to stop: an answer that is not there
	// is named in the journal line and the flush goes on with the recent events).
	idx.mu.Lock()
	err := idx.loadLocked(ctx, rt.store, ownerID, computerID)
	waiting := eventPendingLocked(idx, inputs)
	idx.mu.Unlock()
	if err != nil {
		return report, err
	}
	report.vec = rt.eventNeighbours(ctx, ownerID, computerID, waiting)
	idx.mu.Lock()
	defer idx.mu.Unlock()
	pending := eventPendingLocked(idx, inputs)
	report.Queued = len(pending)
	seen := map[string]bool{}
	for _, in := range pending {
		seen[strings.TrimSpace(in.Item.CaptureID)] = true
		if in.Research {
			report.ResearchReads++
		}
	}
	// The fewest calls: the longest run of waiting captures whose one request
	// fits the vendor's limits (jevbudget: 255 choices a question, 64k tokens a
	// request, 32k state plus a question); a split names its limit in the log.
	var chunks [][]eventInput
	if len(pending) > 0 {
		now := time.Now().UTC()
		bounds := [][2]int{}
		for lo := 0; lo < len(pending); {
			hi := len(pending)
			sub, err := planJevCalls("event_layer", hi-lo, func(a, b int) (json.RawMessage, map[string]json.RawMessage, error) {
				state, questions, _ := eventBuildChunk(idx, report.vec, pending[lo+a:lo+b], now)
				return state, questions, nil
			})
			if err != nil || len(sub.Ranges) == 0 {
				bounds = append(bounds, [2]int{lo, hi})
				lo = hi
				continue
			}
			// Only the first range is taken: the next is planned after this
			// one, against the events it will have minted.
			first := sub.Ranges[0]
			bounds = append(bounds, [2]int{lo + first[0], lo + first[1]})
			lo += first[1]
		}
		for _, b := range bounds {
			chunks = append(chunks, pending[b[0]:b[1]])
		}
	}
	// One call for every waiting capture; a split (past the choices or the
	// token limits) is rare and asks the next chunk after the previous one
	// committed, so it sees the events that chunk minted.
	// Each call has its own deadline, so a long flush never starves the chunks
	// behind it. A chunk whose call fails is asked again and halved
	// (resolveEventChunkAt, jev_failure.go); what still fails leaves its
	// captures in report.Left (an "unresolved" wake record, not retried again).
	// The retries, their backoff and the halves run while resolveEvents holds
	// idx.mu, so a failing flush holds the index for up to eight calls a
	// chunk (a dial failure ends a chunk's calls at once, jevTransportDown); only eventSummary readers wait on it, and the work is the flush's
	// own.
	for _, chunk := range chunks {
		// Each call has its own deadline (scaled to its size); a flush has none, so
		// retries and halves never starve the chunks behind it.
		chunkCtx, cancel := context.WithCancel(ctx)
		rt.resolveEventChunk(chunkCtx, idx, ownerID, computerID, chunk, &report)
		cancel()
	}
	for _, in := range inputs {
		id := strings.TrimSpace(in.Item.CaptureID)
		if id != "" && seen[id] && idx.byCapture[id] == "" {
			report.Left = append(report.Left, in)
		}
	}
	return report, nil
}

// eventCandidates are the known events a chunk's questions offer: the recent
// events every question offers, and each capture's own vector neighbours.
type eventCandidates struct {
	shared []*eventView
	own    [][]*eventView // by capture of the chunk
	// reached is every event the vector neighbours led to, among them those the
	// recent slot already offered (which the own lists leave out).
	reached map[*eventView]bool
}

// of is the events capture j's question offers, shared ones first.
func (c *eventCandidates) of(j int) []*eventView {
	out := append([]*eventView(nil), c.shared...)
	if j < len(c.own) {
		out = append(out, c.own[j]...)
	}
	return out
}

// union is every event any question offers, shared ones first, each once.
func (c *eventCandidates) union() []*eventView {
	out := append([]*eventView(nil), c.shared...)
	seen := map[*eventView]bool{}
	for _, v := range out {
		seen[v] = true
	}
	for _, own := range c.own {
		for _, v := range own {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// dropLast removes the weakest candidate of a one-capture chunk: its last
// vector neighbour, else the last recent event. It reports whether one was left.
func (c *eventCandidates) dropLast() bool {
	for j := len(c.own) - 1; j >= 0; j-- {
		if n := len(c.own[j]); n > 0 {
			c.own[j] = c.own[j][:n-1]
			return true
		}
	}
	if n := len(c.shared); n > 0 {
		c.shared = c.shared[:n-1]
		return true
	}
	return false
}

// eventPendingLocked is the inputs still waiting for a membership, each capture
// once. The caller holds idx.mu.
func eventPendingLocked(idx *eventIndex, inputs []eventInput) []eventInput {
	var pending []eventInput
	seen := map[string]bool{}
	for _, in := range inputs {
		id := strings.TrimSpace(in.Item.CaptureID)
		if id == "" || seen[id] || idx.byCapture[id] != "" {
			continue
		}
		seen[id] = true
		if in.At.IsZero() {
			in.At = time.Now().UTC()
		}
		pending = append(pending, in)
	}
	return pending
}

// eventShortlistFor is the known events a chunk's questions offer.
func eventShortlistFor(idx *eventIndex, vec *eventVectors, chunk []eventInput, now time.Time) *eventCandidates {
	c := &eventCandidates{shared: eventShortlist(idx, now), own: make([][]*eventView, len(chunk)), reached: map[*eventView]bool{}}
	offered := map[*eventView]bool{}
	for _, v := range c.shared {
		offered[v] = true
	}
	for j, in := range chunk {
		picked := map[*eventView]bool{}
		for _, nid := range vec.nearOf(strings.TrimSpace(in.Item.CaptureID)) {
			if len(c.own[j]) >= eventShortlistVector {
				break
			}
			v := idx.events[idx.byCapture[nid]]
			// A neighbour with no membership (a capture no flush has resolved,
			// a sibling of this flush) or whose event is past the index's load
			// caps offers nothing, and is counted; an event already offered to
			// every question, or to this one, is not offered twice.
			if v == nil {
				if vec != nil {
					vec.unmapped[nid] = true
				}
				continue
			}
			c.reached[v] = true
			if offered[v] || picked[v] {
				continue
			}
			picked[v] = true
			c.own[j] = append(c.own[j], v)
		}
	}
	return c
}

// eventBuildChunkState is eventBuildChunk's state and questions, for naming the limit.
func eventBuildChunkState(idx *eventIndex, vec *eventVectors, chunk []eventInput, now time.Time) (json.RawMessage, map[string]json.RawMessage) {
	state, questions, _ := eventBuildChunk(idx, vec, chunk, now)
	return state, questions
}

// eventBuildChunk is the request for chunk against the index as it stands,
// with no bound applied.
func eventBuildChunk(idx *eventIndex, vec *eventVectors, chunk []eventInput, now time.Time) (json.RawMessage, map[string]json.RawMessage, eventAliases) {
	return eventQuestion(chunk, eventShortlistFor(idx, vec, chunk, now), eventSummaryRunes)
}

// eventPrepareChunk builds the call for chunk against the index as it stands.
// split is true when the request is over the token budget and the chunk must be
// halved first (the halves are prepared one after the other, each seeing the
// previous half's events).
func eventPrepareChunk(idx *eventIndex, vec *eventVectors, chunk []eventInput, now time.Time) (state json.RawMessage, questions map[string]json.RawMessage, alias eventAliases, cands *eventCandidates, split bool) {
	cands = eventShortlistFor(idx, vec, chunk, now)
	state, questions, alias = eventQuestion(chunk, cands, eventSummaryRunes)
	// The bound counts the questions too: each repeats every candidate's
	// criteria, so they grow with chunk x candidates (code review, Devin
	// B2). Split the chunk first; a single capture over the bound drops its
	// summary, then its weakest candidates.
	if !jevbudget.Fits(state, questions) && len(chunk) > 1 {
		return nil, nil, alias, cands, true
	}
	for summary := eventSummaryRunes; !jevbudget.Fits(state, questions); {
		if summary > 0 {
			summary = 0
		} else if !cands.dropLast() {
			break
		}
		state, questions, alias = eventQuestion(chunk, cands, summary)
	}
	return state, questions, alias, cands, false
}

func (rt *Runtime) askEventChunk(ctx context.Context, chunk []eventInput, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	meta := jevrecord.Meta{Site: "event_layer", Scope: judgmentScopeIntakeLabels,
		Threshold: fmt.Sprintf("join=%.2f tentative=%.2f", eventJoinP, eventTentativeP)}
	if len(chunk) > 0 {
		meta.DecisionID = "event_chunk:" + strings.TrimSpace(chunk[0].Item.CaptureID)
	}
	// The deadline scales with the request: 30 s plus 1 s per thousand
	// estimated tokens (a deadline is not a call limit).
	timeout := eventCallTimeout + time.Duration(jevbudget.Measure(state, questions).Request/1000)*time.Second
	return rt.judgeCall(ctx, meta, timeout, state, questions)
}

// resolveEventChunk asks Jev once about chunk and writes the result. The
// caller holds idx.mu.
func (rt *Runtime) resolveEventChunk(ctx context.Context, idx *eventIndex, ownerID, computerID string, chunk []eventInput, report *eventResolveReport) {
	rt.resolveEventChunkAt(ctx, idx, ownerID, computerID, chunk, report, 0, 0)
}

// resolveEventChunkAt is resolveEventChunk for a chunk that sits depth halvings
// below the flush's call and has been tried retries more times: a failed call
// is handled as intake's is (jev_failure.go): a rate refusal backs off, other
// failures are asked once more and then halved to a bounded depth, except a
// dial failure, which fails at once.
func (rt *Runtime) resolveEventChunkAt(ctx context.Context, idx *eventIndex, ownerID, computerID string, chunk []eventInput, report *eventResolveReport, depth, retries int) {
	now := time.Now().UTC()
	stateRaw, questions, alias, cands, split := eventPrepareChunk(idx, report.vec, chunk, now)
	if split {
		half := len(chunk) / 2
		log.Printf("jev split: site=event_layer items=%d calls=2 forced_by=map[%s:1]", len(chunk), jevbudget.Limit(eventBuildChunkState(idx, report.vec, chunk, now)))
		rt.resolveEventChunkAt(ctx, idx, ownerID, computerID, chunk[:half], report, depth, retries)
		rt.resolveEventChunkAt(ctx, idx, ownerID, computerID, chunk[half:], report, depth, retries)
		return
	}
	report.noteShortlist(cands)
	raw, err := rt.askEventChunk(ctx, chunk, stateRaw, questions)
	report.Calls++
	if err != nil && ctx.Err() == nil {
		switch act, forcedBy := jevNextStep(err, retries, depth, len(chunk) > 1); act {
		case jevRetry:
			log.Printf("event layer: call of %d captures failed (%v); asking again after a backoff", len(chunk), err)
			select {
			case <-time.After(intakeRetryBackoff * time.Duration(retries+1)):
			case <-ctx.Done():
			}
			rt.resolveEventChunkAt(ctx, idx, ownerID, computerID, chunk, report, depth, retries+1)
			return
		case jevHalve:
			half := len(chunk) / 2
			log.Printf("jev split: site=event_layer items=%d calls=2 forced_by=map[%s:1] error=%q", len(chunk), forcedBy, err.Error())
			rt.resolveEventChunkAt(ctx, idx, ownerID, computerID, chunk[:half], report, depth+1, 1)
			rt.resolveEventChunkAt(ctx, idx, ownerID, computerID, chunk[half:], report, depth+1, 1)
			return
		}
	}
	if err != nil {
		report.Unresolved += len(chunk)
		log.Printf("event layer: %d captures unresolved: %v", len(chunk), err)
		return
	}
	var dist struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Choice        string             `json:"choice"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
		Usage jevUsage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &dist); err != nil {
		report.Unresolved += len(chunk)
		log.Printf("event layer: decode Jev's answer: %v", err)
		return
	}
	report.CostUSD += dist.Usage.Cost
	model := firstNonEmpty(dist.Model, candidateJudgmentModel)
	share := dist.Usage.Cost / float64(len(chunk))

	// Each answered capture: a counted or tentative match to a known
	// event, or a sibling union. An unanswered capture stays unresolved;
	// it is never minted (code review, Gemini).
	type decision struct {
		answered  bool
		event     *eventView
		tentative bool
		p         float64
		// follows is the known event this capture is a later development
		// of (an after_ answer); it only matters if the capture mints.
		follows  *eventView
		followsP float64
	}
	decisions := make([]decision, len(chunk))
	parent := make([]int, len(chunk))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			i = parent[i]
		}
		return i
	}
	for j := range chunk {
		a, ok := dist.Answers[fmt.Sprintf("k%d", j+1)]
		if !ok {
			continue
		}
		choice := strings.TrimSpace(a.Choice)
		p := a.Probabilities[choice]
		decisions[j] = decision{answered: true, p: p}
		switch {
		case alias.after[choice] != nil && p >= eventTentativeP:
			decisions[j].follows, decisions[j].followsP = alias.after[choice], p
		case alias.events[choice] != nil && p >= eventJoinP:
			decisions[j].event = alias.events[choice]
		case alias.events[choice] != nil && p >= eventTentativeP:
			decisions[j].event, decisions[j].tentative = alias.events[choice], true
		case alias.captures[choice] > 0 && alias.captures[choice]-1 != j && p >= eventJoinP:
			ri, rj := find(alias.captures[choice]-1), find(j)
			if ri < rj {
				parent[rj] = ri
			} else {
				parent[ri] = rj
			}
		}
	}
	// The formation fold, per sibling component of answered captures: the
	// best counted match takes the whole component; else the best
	// tentative match takes it tentatively (a sibling of a tentative member
	// never mints a rival event; code review, Devin B1); else one event is
	// minted, opened by the component's earliest capture.
	groups := map[int][]int{}
	for j := range chunk {
		if !decisions[j].answered {
			report.Unresolved++
			log.Printf("event layer: %s unresolved: Jev's answer omitted it", chunk[j].Item.CaptureID)
			continue
		}
		groups[find(j)] = append(groups[find(j)], j)
	}
	roots := make([]int, 0, len(groups))
	for r := range groups {
		roots = append(roots, r)
	}
	sort.Ints(roots)
	for _, root := range roots {
		members := groups[root]
		var counted, tentative *eventView
		bestCounted, bestTentative := -1.0, -1.0
		for _, j := range members {
			d := decisions[j]
			switch {
			case d.event != nil && !d.tentative && d.p > bestCounted:
				counted, bestCounted = d.event, d.p
			case d.event != nil && d.tentative && d.p > bestTentative:
				tentative, bestTentative = d.event, d.p
			}
		}
		if counted == nil && tentative != nil {
			for _, j := range members {
				ok, created := rt.writeMembership(ctx, idx, tentative, ownerID, computerID, chunk[j], true, false, decisions[j].p, model, share)
				switch {
				case !ok:
					report.Unresolved++
				case created:
					report.Tentative++
					report.noteCreated(tentative.ev.EventID, chunk[j].Item.CaptureID)
				}
			}
			continue
		}
		target, opened := counted, ""
		if target == nil {
			first := chunk[members[0]]
			ev := store.WorldEvent{OwnerID: ownerID, ComputerID: computerID, EventID: eventID(computerID, first.Item.CaptureID),
				Headline: strings.TrimSpace(first.Item.Title), OpenedBy: first.Item.CaptureID, OpenedOutlet: outlet.Of(first.Item.URL), FirstSeen: first.At}
			// A later development links to the earlier event; the
			// strongest answer in the component names it.
			best := -1.0
			for _, j := range members {
				if f := decisions[j].follows; f != nil && decisions[j].followsP > best {
					ev.Follows, best = f.ev.EventID, decisions[j].followsP
				}
			}
			stored, created, err := rt.store.PutWorldEvent(withKindCustodian(ctx, "choir.world_event"), ev)
			if err != nil {
				report.Unresolved += len(members)
				log.Printf("event layer: mint event for %s: %v", first.Item.CaptureID, err)
				continue
			}
			target = idx.events[stored.EventID]
			if target == nil {
				target = &eventView{ev: stored, outlets: map[string]bool{}, origins: map[string]bool{}, last: stored.FirstSeen}
				idx.events[stored.EventID] = target
			}
			opened = first.Item.CaptureID
			if created {
				report.Minted++
			}
		}
		for _, j := range members {
			wasCorroborated := target.corroborated()
			ok, created := rt.writeMembership(ctx, idx, target, ownerID, computerID, chunk[j], false, opened == chunk[j].Item.CaptureID, decisions[j].p, model, share)
			switch {
			case !ok:
				report.Unresolved++
			case created && opened != chunk[j].Item.CaptureID:
				report.Joined++
			}
			if ok && created {
				report.noteCreated(target.ev.EventID, chunk[j].Item.CaptureID)
			}
			if !wasCorroborated && target.corroborated() {
				report.Corroborated = append(report.Corroborated, target.ev.EventID)
				log.Printf("event layer: %s corroborated by %d origins (%s)", target.ev.EventID, len(target.origins), strings.Join(target.summary().Outlets, ", "))
			}
		}
	}
}

// writeMembership writes one capture's membership. ok is false when the
// write failed; created is false when the capture already had one (a
// retry), which is not a failure.
func (rt *Runtime) writeMembership(ctx context.Context, idx *eventIndex, target *eventView, ownerID, computerID string, in eventInput, tentative, opened bool, p float64, model string, cost float64) (ok, created bool) {
	m := store.EventMembership{OwnerID: ownerID, ComputerID: computerID, CaptureID: in.Item.CaptureID, EventID: target.ev.EventID,
		Opened: opened, Tentative: tentative, P: p, Outlet: outlet.Of(in.Item.URL), Origin: wireOrigin(in.Item.Summary, in.Item.Title),
		URL: in.Item.URL, Title: strings.TrimSpace(in.Item.Title), SourceID: in.SourceID, CapturedAt: in.At, Model: model, CostUSD: cost}
	stored, created, err := rt.store.PutEventMembership(withKindCustodian(ctx, "choir.event_membership"), m)
	if err != nil {
		log.Printf("event layer: membership of %s: %v", in.Item.CaptureID, err)
		return false, false
	}
	idx.byCapture[stored.CaptureID] = stored.EventID
	if created {
		if v := idx.events[stored.EventID]; v != nil {
			v.apply(stored)
		}
	}
	return true, created
}

// noteCreated records a membership this resolution wrote.
func (r *eventResolveReport) noteCreated(eventID, captureID string) {
	if r.Created == nil {
		r.Created = map[string][]string{}
	}
	r.Created[eventID] = append(r.Created[eventID], captureID)
}

type eventAliases struct {
	events   map[string]*eventView
	after    map[string]*eventView // "after_eN" -> the event a capture would follow
	captures map[string]int        // alias -> index+1
}

// eventShortlist is the recent slot every question offers: the newest events
// within the recency horizon, at most eventShortlistRecent. The events a
// capture's vector neighbours reach are added per capture (eventShortlistFor).
func eventShortlist(idx *eventIndex, now time.Time) []*eventView {
	var live []*eventView
	for _, v := range idx.events {
		if now.Sub(v.last) <= eventRecentHorizon {
			live = append(live, v)
		}
	}
	sort.Slice(live, func(i, j int) bool {
		if !live[i].last.Equal(live[j].last) {
			return live[i].last.After(live[j].last)
		}
		return live[i].ev.EventID < live[j].ev.EventID
	})
	if len(live) > eventShortlistRecent {
		live = live[:eventShortlistRecent]
	}
	return live
}

// eventVectors is a flush's vector neighbours: for each queued capture the
// nearest other captures of the computer, best first, as corpusd gave them.
// They are mapped to events when a call is built, so a membership an earlier
// call of the flush wrote is seen by the next.
type eventVectors struct {
	near map[string][]string
	// Unavailable is why no neighbours came ("" when corpusd answered).
	Unavailable string
	// Rows is the neighbour rows returned. Unembedded are waiting captures
	// with no vector yet (the case a title-overlap slot used to cover);
	// Excluded those corpusd will never embed or may not return (private,
	// tombstoned, superseded, not found); Cut those a deadline did not reach.
	Rows, Unembedded, Excluded, Cut int
	// State is ok, approximate or partial; "unavailable" when Unavailable is set.
	State string
	// unmapped are the neighbour captures that mapped to no event in the
	// index (no membership yet, a sibling of the flush, or a membership or
	// event past the index's load caps), counted once each.
	unmapped map[string]bool
}

func (v *eventVectors) nearOf(captureID string) []string {
	if v == nil {
		return nil
	}
	return v.near[captureID]
}

// eventNeighbours asks corpusd, in one request, for the nearest held captures
// of every waiting capture. It never fails the flush: a corpusd without the
// route, a refusal, a timeout or vectors that are not ready is named in
// Unavailable and the flush goes on without the vector slot.
func (rt *Runtime) eventNeighbours(ctx context.Context, ownerID, computerID string, pending []eventInput) *eventVectors {
	vec := &eventVectors{near: map[string][]string{}, unmapped: map[string]bool{}}
	if len(pending) == 0 {
		return vec
	}
	var source researchtools.CaptureNeighbourer
	if rt.researchDeps != nil {
		source, _ = rt.researchDeps.Captures.(researchtools.CaptureNeighbourer)
	}
	if source == nil {
		vec.Unavailable = "no capture store answers neighbours"
		return vec
	}
	ids := make([]string, 0, len(pending))
	for _, in := range pending {
		ids = append(ids, strings.TrimSpace(in.Item.CaptureID))
	}
	since := time.Now().UTC().Add(-eventVectorHorizon).Format("2006-01-02")
	for lo := 0; lo < len(ids); lo += eventNeighbourBatch {
		callCtx, cancel := context.WithTimeout(ctx, eventNeighboursTimeout)
		ans, err := source.CaptureNeighbours(callCtx, researchtools.CaptureNeighboursQuery{OwnerID: ownerID, ComputerID: computerID,
			CaptureIDs: ids[lo:min(lo+eventNeighbourBatch, len(ids))], Since: since, Limit: eventNeighbourLimit})
		cancel()
		switch {
		case err != nil:
			vec.Unavailable = "the neighbours request failed: " + err.Error()
		case ans.Unavailable != "":
			vec.Unavailable = ans.Unavailable
		}
		if vec.Unavailable != "" {
			break
		}
		for id, list := range ans.Neighbours {
			for _, n := range list {
				vec.near[id] = append(vec.near[id], n.CaptureID)
				vec.Rows++
			}
		}
		vec.Unembedded += ans.Unembedded
		vec.Excluded += ans.Excluded
		vec.Cut += ans.Cut
		vec.State = worseVectorState(vec.State, ans.State)
	}
	if vec.Cut > 0 {
		vec.State = "partial"
	}
	if vec.Unavailable != "" {
		log.Printf("event layer: vector slot unavailable: %s", vec.Unavailable)
		vec.near, vec.Rows, vec.Unembedded, vec.Excluded, vec.Cut, vec.State = map[string][]string{}, 0, 0, 0, 0, "unavailable"
	}
	return vec
}

// worseVectorState folds corpusd's state (exact, approximate, partial) into
// the flush's (ok, approximate, partial), keeping the worst.
func worseVectorState(have, got string) string {
	rank := map[string]int{"": 0, "ok": 1, "approximate": 2, "partial": 3}
	if got == "exact" || got == "" {
		got = "ok"
	}
	if rank[got] > rank[have] {
		return got
	}
	return have
}

// noteShortlist adds one call's candidates to the flush's shortlist counts.
func (r *eventResolveReport) noteShortlist(c *eventCandidates) {
	if c == nil {
		return
	}
	if n := len(c.shared); n > r.ShortRecent {
		r.ShortRecent = n
	}
	if r.vectorEvents == nil {
		r.vectorEvents, r.vectorOnlyEvts = map[string]bool{}, map[string]bool{}
	}
	for v := range c.reached {
		r.vectorEvents[v.ev.EventID] = true
	}
	for _, own := range c.own {
		for _, v := range own {
			r.vectorOnlyEvts[v.ev.EventID] = true // an own list never repeats a recent event
		}
	}
}

func lastN(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// eventDefinition is what an event is, for Jev's task and questions and for
// the blind judges (scripts/event-blind-sample.py carries it verbatim; a
// test holds them together). Owner ruling 2026-10-20: one event is one
// incident, one place, one time; later developments are new linked events.
// The earlier wording ("the same occurrence or its direct development")
// made a developing story one event, which no threshold could undo
// (docs/problems/event-layer-false-merges-at-marginal-p-2026-10-16.md).
const eventDefinition = "One event is one incident, in one place, at one time: the occurrence itself, however many outlets report it and however its details are updated. A later development of it (a reaction, a response, an investigation, an outcome), another incident of the same story, place or actors, a live blog or roundup spanning several incidents, and an opinion or analysis piece about a topic are each a different event."

// linkTokens are a title's words for choosing link candidates: titleTokens
// with a plural s trimmed, so houthi and houthis meet.
func linkTokens(text string) map[string]bool {
	out := map[string]bool{}
	for w := range titleTokens(text) {
		if len(w) > 4 && strings.HasSuffix(w, "s") {
			w = strings.TrimSuffix(w, "s")
		}
		out[w] = true
	}
	return out
}

// eventLinkCandidates are the known events one capture may be a later
// development of: up to eventLinkChoices sharing the most words with it,
// at least one, ties in candidate order.
func eventLinkCandidates(in eventInput, candidates []*eventView, aliasOf map[*eventView]string) []string {
	want := linkTokens(in.Item.Title + " " + in.Item.Summary)
	type scored struct {
		alias string
		score int
	}
	var hits []scored
	for _, v := range candidates {
		score := 0
		for w := range linkTokens(v.ev.Headline + " " + strings.Join(lastN(v.titles, eventRecentTitles), " ")) {
			if want[w] {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, scored{aliasOf[v], score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	var out []string
	for i := 0; i < len(hits) && i < eventLinkChoices; i++ {
		out = append(out, hits[i].alias)
	}
	return out
}

// eventQuestion builds one call: the candidate events, the chunk's captures,
// and one choice question per capture, which offers the recent events and its
// own vector neighbours' events.
func eventQuestion(chunk []eventInput, cands *eventCandidates, summaryRunes int) (json.RawMessage, map[string]json.RawMessage, eventAliases) {
	candidates := cands.union()
	alias := eventAliases{events: map[string]*eventView{}, after: map[string]*eventView{}, captures: map[string]int{}}
	aliasOf := map[*eventView]string{}
	events := map[string]any{}
	for i, v := range candidates {
		a := fmt.Sprintf("e%d", i+1)
		alias.events[a] = v
		aliasOf[v] = a
		entry := map[string]any{"headline": intakeClip(v.ev.Headline, 200), "last_seen": v.last.UTC().Format(time.RFC3339)}
		var recent []string
		for _, t := range lastN(v.titles, eventRecentTitles) {
			if t != v.ev.Headline {
				recent = append(recent, intakeClip(t, 160))
			}
		}
		if len(recent) > 0 {
			entry["recent_titles"] = recent
		}
		events[a] = entry
	}
	captures := map[string]any{}
	titles := make([]string, len(chunk))
	for j, in := range chunk {
		titles[j] = in.Item.Title
		a := fmt.Sprintf("k%d", j+1)
		alias.captures[a] = j + 1
		c := map[string]any{"title": intakeClip(in.Item.Title, 200)}
		if summaryRunes > 0 {
			if s := intakeClip(in.Item.Summary, summaryRunes); strings.TrimSpace(s) != "" {
				c["summary"] = s
			}
		}
		if o := outlet.Of(in.Item.URL); o != "" {
			c["outlet"] = o
		}
		if in.Item.Language != "" {
			c["language"] = in.Item.Language
		}
		captures[a] = c
	}
	state := map[string]any{
		"task":     "Each capture in state.captures is a news item just fetched, in any language. For each, decide whether it reports the same real-world event as a known event in state.events or as another capture in state.captures, or a new event. " + eventDefinition + " A new event that is a later development of a known event can name it with that event's after_ choice, when one is offered.",
		"captures": captures,
	}
	if len(events) > 0 {
		state["events"] = events
	}
	questions := map[string]json.RawMessage{}
	for j := range chunk {
		a := fmt.Sprintf("k%d", j+1)
		// The options are handles, read by name alone (a null description, as
		// intake's are): a sentence under every option of every question put
		// quadratically many tokens in the request, so a flush of a few dozen
		// captures crossed the 64k limit and split.
		criteria := map[string]any{"new": "No known event and no other capture reports this incident."}
		offered := cands.of(j)
		var choices []string
		for _, v := range offered {
			e := aliasOf[v]
			choices = append(choices, e)
			criteria[e] = nil
		}
		for _, e := range eventLinkCandidates(chunk[j], offered, aliasOf) {
			after := "after_" + e
			alias.after[after] = alias.events[e]
			criteria[after] = "A later development of state.events." + e + ", not that incident itself."
			choices = append(choices, after)
		}
		for i := range chunk {
			if i == j {
				continue
			}
			other := fmt.Sprintf("k%d", i+1)
			criteria[other] = nil
			choices = append(choices, other)
		}
		choices = append(choices, "new")
		raw, _ := json.Marshal(map[string]any{
			"type":         "choice",
			"instructions": eventDefinition + " Which known event or other capture reports the same incident as state.captures." + a + "? Prefer a known event over another capture when both match. The options are handles: e1, e2, … are the known events in state.events, k1, k2, … the other captures in state.captures, and new. Choose an after_ choice only for a later development of a known event.",
			"criteria":     criteria,
			"choices":      choices,
		})
		questions[a] = raw
	}
	stateRaw, _ := json.Marshal(state)
	return stateRaw, questions, alias
}

// enqueueEventResolution queues a poll's new captures for the next flush;
// the event layer has no setting. A flush runs eventFlushInterval after the first
// capture, so captures from many sources share one call; no count of queued
// captures starts a flush early. One flush at a time drains a computer's
// queue, in order; captures queued meanwhile are drained by the same flush.
//
// It reports whether it queued the captures.
func (rt *Runtime) enqueueEventResolution(ctx context.Context, ownerID, computerID, sourceID string, captured []sourcegraph.CapturedItem) bool {
	return rt.enqueueEventCaptures(ctx, ownerID, computerID, sourceID, captured, false)
}

// enqueueEventCaptures is enqueueEventResolution for captures a source
// schedule made (research false) or a research desk's read or poll made
// (research true: the flush's journal line counts them apart).
func (rt *Runtime) enqueueEventCaptures(ctx context.Context, ownerID, computerID, sourceID string, captured []sourcegraph.CapturedItem, research bool) bool {
	if rt == nil || len(captured) == 0 {
		return false
	}
	now := time.Now().UTC()
	key := eventKey(ownerID, computerID)
	rt.events.mu.Lock()
	defer rt.events.mu.Unlock()
	if rt.events.stopped {
		return false
	}
	if rt.events.queues == nil {
		rt.events.queues = map[string]*eventQueue{}
	}
	q := rt.events.queues[key]
	if q == nil {
		q = &eventQueue{}
		rt.events.queues[key] = q
	}
	for _, c := range captured {
		if strings.TrimSpace(c.CaptureID) != "" {
			q.items = append(q.items, eventInput{Item: c, SourceID: sourceID, At: now, Research: research})
		}
	}
	if q.inFlight || len(q.items) == 0 {
		return true
	}
	if q.timer == nil {
		q.timer = time.AfterFunc(eventFlushInterval, func() {
			rt.events.mu.Lock()
			defer rt.events.mu.Unlock()
			q.timer = nil
			if !rt.events.stopped && !q.inFlight && len(q.items) > 0 {
				rt.startEventFlushLocked(ownerID, computerID, q)
			}
		})
	}
	return true
}

// startEventFlushLocked starts the computer's one flush. The caller holds
// rt.events.mu, and the runtime is not stopped, so the WaitGroup add never
// races Stop's wait.
func (rt *Runtime) startEventFlushLocked(ownerID, computerID string, q *eventQueue) {
	if rt.events.ctx == nil {
		rt.events.ctx, rt.events.cancel = context.WithCancel(context.Background())
	}
	if q.timer != nil {
		q.timer.Stop()
		q.timer = nil
	}
	q.inFlight = true
	layerCtx := rt.events.ctx
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		rt.drainEventQueue(layerCtx, ownerID, computerID, q)
	}()
}

// drainEventQueue resolves the queue until it is empty or the runtime
// stops.
func (rt *Runtime) drainEventQueue(layerCtx context.Context, ownerID, computerID string, q *eventQueue) {
	for {
		rt.events.mu.Lock()
		if rt.events.stopped || len(q.items) == 0 {
			q.inFlight = false
			rt.events.mu.Unlock()
			return
		}
		items := q.items
		q.items = nil
		rt.events.mu.Unlock()
		// No deadline across the flush: each chunk has its own (resolveEvents).
		report, err := rt.resolveEvents(layerCtx, ownerID, computerID, items)
		if layerCtx.Err() == nil {
			rt.mintEventWakes(layerCtx, ownerID, computerID, items, report, err)
		}
		if err != nil {
			log.Printf("event layer: %d captures unresolved: %v", len(items), err)
			continue
		}
		log.Print(eventFlushLine(report))
	}
}

// flushEventQueue starts a flush of one computer's queue now, unless one is
// running.
func (rt *Runtime) flushEventQueue(ownerID, computerID string) {
	rt.events.mu.Lock()
	defer rt.events.mu.Unlock()
	q := rt.events.queues[eventKey(ownerID, computerID)]
	if rt.events.stopped || q == nil || q.inFlight || len(q.items) == 0 {
		return
	}
	rt.startEventFlushLocked(ownerID, computerID, q)
}

// stopEventLayer stops the timers, cancels a running flush, and refuses new
// work. Queued captures are dropped and stay visibly unresolved; nothing
// resolves them at the next boot.
func (rt *Runtime) stopEventLayer() {
	rt.events.mu.Lock()
	defer rt.events.mu.Unlock()
	rt.events.stopped = true
	for _, q := range rt.events.queues {
		if q.timer != nil {
			q.timer.Stop()
			q.timer = nil
		}
		q.items = nil
	}
	if rt.events.cancel != nil {
		rt.events.cancel()
	}
}

// eventFlushLine is the journal line of one flush, the same fields every time
// (the station V5 acceptance reads them; scripts/status-corpus-reuse): the
// questions the flush asked (queued: each waiting capture once) and how many
// were research's pages, the Jev calls, the shortlist by slot (recent: the
// slot's size; vector: the events the captures' neighbours led to; vector_new:
// those the recent slot did not already offer), vector=<neighbour rows>|
// unavailable with vector_state=ok|approximate|partial|unavailable, the
// captures with no neighbours by reason (missing_unembedded: no vector yet;
// missing_excluded: private, tombstoned, superseded or unknown; a deadline cut
// is partial, not missing; a capture whose vector the embedder refused is
// counted by corpusd as refused=, which this line does not carry, so it is in
// neither missing field), the neighbours that mapped to no event
// (vector_unmapped), and what became of the questions.
func eventFlushLine(r eventResolveReport) string {
	vector, state := "unavailable", "unavailable"
	var unembedded, excluded, unmapped int
	if r.vec != nil && r.vec.Unavailable == "" {
		vector, state = fmt.Sprintf("%d", r.vec.Rows), firstNonEmpty(r.vec.State, "ok")
		unembedded, excluded, unmapped = r.vec.Unembedded, r.vec.Excluded, len(r.vec.unmapped)
	}
	return fmt.Sprintf("event layer: resolved %d captures in %d calls: op=event_flush queued=%d research_reads=%d calls=%d shortlist_recent=%d shortlist_vector=%d shortlist_vector_new=%d vector=%s vector_state=%s missing_unembedded=%d missing_excluded=%d vector_unmapped=%d joined=%d minted=%d tentative=%d unresolved=%d corroborated=%d cost_usd=%.5f",
		r.Queued, r.Calls, r.Queued, r.ResearchReads, r.Calls, r.ShortRecent, len(r.vectorEvents), len(r.vectorOnlyEvts), vector, state, unembedded, excluded, unmapped,
		r.Joined, r.Minted, r.Tentative, r.Unresolved, len(r.Corroborated), r.CostUSD)
}
