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

	"github.com/yusefmosiah/go-choir/internal/outlet"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
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
	eventFlushBudget   = 2 * time.Minute
	eventCallTimeout   = 30 * time.Second
)

const (
	// eventChunkSize is the most captures one Jev call resolves; chunks of
	// a flush run in order, so an event minted in one chunk is a candidate
	// in the next.
	eventChunkSize        = 12
	eventShortlistRecent  = 30
	eventShortlistOverlap = 10
	// eventRecentHorizon bounds the recency slots; eventOverlapHorizon
	// bounds the title-overlap slots, which reach uncorroborated events a
	// slow second report would otherwise miss (panel B2).
	eventRecentHorizon  = 72 * time.Hour
	eventOverlapHorizon = 14 * 24 * time.Hour
	// eventJoinP joins an event as a counted member; eventTentativeP up to
	// it attaches the capture as a tentative member, which never counts
	// toward the event's outlets and never mints a rival event (panel B1).
	eventJoinP      = 0.85
	eventTentativeP = 0.5
	// eventStateMaxBytes keeps a call under Jev's default context (above
	// about 32 KB, 1.14 runs a slower, less accurate long-context mode).
	eventStateMaxBytes = 24 * 1024
	eventSummaryRunes  = 400
	eventRecentTitles  = 2
	// eventIndexLoad bounds the rebuild read.
	eventIndexLoadEvents      = 10000
	eventIndexLoadMemberships = 20000
)

// eventInput is one capture to resolve.
type eventInput struct {
	Item     sourcegraph.CapturedItem
	SourceID string
	At       time.Time
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
}

func (v *eventView) corroborated() bool { return len(v.origins) >= store.EventMinOrigins }

func (v *eventView) apply(m store.EventMembership) {
	if m.CapturedAt.After(v.last) {
		v.last = m.CapturedAt
	}
	if m.Tentative {
		v.tentative++
		return
	}
	v.members++
	v.titles = append(v.titles, m.Title)
	v.captures = append(v.captures, m.CaptureID)
	if origin := m.CountedOrigin(); origin != "" {
		v.outlets[m.Outlet] = true
		v.origins[origin] = true
	}
}

func (v *eventView) summary() eventSummary {
	s := eventSummary{EventID: v.ev.EventID, Headline: v.ev.Headline, Origins: len(v.origins), Members: v.members,
		Tentative: v.tentative, Corroborated: v.corroborated(), FirstSeen: v.ev.FirstSeen, LastSeen: v.last,
		Titles: append([]string(nil), v.titles...), CaptureIDs: append([]string(nil), v.captures...)}
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
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if err := idx.loadLocked(ctx, rt.store, ownerID, computerID); err != nil {
		return report, err
	}
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
	for len(pending) > 0 {
		n := eventChunkSize
		if n > len(pending) {
			n = len(pending)
		}
		chunk := pending[:n]
		pending = pending[n:]
		rt.resolveEventChunk(ctx, idx, ownerID, computerID, chunk, &report)
	}
	return report, nil
}

// resolveEventChunk asks Jev once about chunk and writes the result. The
// caller holds idx.mu.
func (rt *Runtime) resolveEventChunk(ctx context.Context, idx *eventIndex, ownerID, computerID string, chunk []eventInput, report *eventResolveReport) {
	now := time.Now().UTC()
	tokens := map[string]bool{}
	for _, in := range chunk {
		for w := range titleTokens(in.Item.Title) {
			tokens[w] = true
		}
	}
	candidates := eventShortlist(idx, tokens, now)
	stateRaw, questions, alias := eventQuestion(chunk, candidates, eventSummaryRunes)
	// The bound counts the questions too: each repeats every candidate's
	// criteria, so they grow with chunk x candidates (code review, Devin
	// B2). Split the chunk first; a single capture over the bound drops its
	// summary, then its weakest candidates.
	if eventRequestBytes(stateRaw, questions) > eventStateMaxBytes && len(chunk) > 1 {
		half := len(chunk) / 2
		rt.resolveEventChunk(ctx, idx, ownerID, computerID, chunk[:half], report)
		rt.resolveEventChunk(ctx, idx, ownerID, computerID, chunk[half:], report)
		return
	}
	for summary := eventSummaryRunes; eventRequestBytes(stateRaw, questions) > eventStateMaxBytes; {
		if summary > 0 {
			summary = 0
		} else if len(candidates) > 0 {
			candidates = candidates[:len(candidates)-1]
		} else {
			break
		}
		stateRaw, questions, alias = eventQuestion(chunk, candidates, summary)
	}
	raw, err := callJudgeWithin(ctx, rt.candidateJudge, judgmentScopeIntakeLabels, eventCallTimeout, stateRaw, questions)
	report.Calls++
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
				}
			}
			continue
		}
		target, opened := counted, ""
		if target == nil {
			first := chunk[members[0]]
			ev := store.WorldEvent{OwnerID: ownerID, ComputerID: computerID, EventID: eventID(computerID, first.Item.CaptureID),
				Headline: strings.TrimSpace(first.Item.Title), OpenedBy: first.Item.CaptureID, OpenedOutlet: outlet.Of(first.Item.URL), FirstSeen: first.At}
			stored, created, err := rt.store.PutWorldEvent(ctx, ev)
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
			if !wasCorroborated && target.corroborated() {
				report.Corroborated = append(report.Corroborated, target.ev.EventID)
				log.Printf("event layer: %s corroborated by %d origins (%s)", target.ev.EventID, len(target.origins), strings.Join(target.summary().Outlets, ", "))
			}
		}
	}
}

// eventRequestBytes is what one call sends upstream: the state and every
// question.
func eventRequestBytes(state json.RawMessage, questions map[string]json.RawMessage) int {
	n := len(state)
	for _, q := range questions {
		n += len(q)
	}
	return n
}

// writeMembership writes one capture's membership. ok is false when the
// write failed; created is false when the capture already had one (a
// retry), which is not a failure.
func (rt *Runtime) writeMembership(ctx context.Context, idx *eventIndex, target *eventView, ownerID, computerID string, in eventInput, tentative, opened bool, p float64, model string, cost float64) (ok, created bool) {
	m := store.EventMembership{OwnerID: ownerID, ComputerID: computerID, CaptureID: in.Item.CaptureID, EventID: target.ev.EventID,
		Opened: opened, Tentative: tentative, P: p, Outlet: outlet.Of(in.Item.URL), Origin: wireOrigin(in.Item.Summary, in.Item.Title),
		URL: in.Item.URL, Title: strings.TrimSpace(in.Item.Title), SourceID: in.SourceID, CapturedAt: in.At, Model: model, CostUSD: cost}
	stored, created, err := rt.store.PutEventMembership(ctx, m)
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

type eventAliases struct {
	events   map[string]*eventView
	captures map[string]int // alias -> index+1
}

// eventShortlist is the candidate events for a chunk: the newest within the
// recency horizon, then uncorroborated ones within the overlap horizon that
// share title words with the chunk.
func eventShortlist(idx *eventIndex, tokens map[string]bool, now time.Time) []*eventView {
	var live []*eventView
	for _, v := range idx.events {
		if now.Sub(v.last) <= eventOverlapHorizon {
			live = append(live, v)
		}
	}
	sort.Slice(live, func(i, j int) bool {
		if !live[i].last.Equal(live[j].last) {
			return live[i].last.After(live[j].last)
		}
		return live[i].ev.EventID < live[j].ev.EventID
	})
	var out []*eventView
	picked := map[*eventView]bool{}
	for _, v := range live {
		if len(out) >= eventShortlistRecent || now.Sub(v.last) > eventRecentHorizon {
			break
		}
		out = append(out, v)
		picked[v] = true
	}
	type scored struct {
		v     *eventView
		score int
	}
	var overlap []scored
	for _, v := range live {
		if picked[v] || v.corroborated() {
			continue
		}
		words := titleTokens(v.ev.Headline + " " + strings.Join(lastN(v.titles, eventRecentTitles), " "))
		score := 0
		for w := range words {
			if tokens[w] {
				score++
			}
		}
		if score > 0 {
			overlap = append(overlap, scored{v, score})
		}
	}
	sort.SliceStable(overlap, func(i, j int) bool { return overlap[i].score > overlap[j].score })
	for i := 0; i < len(overlap) && i < eventShortlistOverlap; i++ {
		out = append(out, overlap[i].v)
	}
	return out
}

func lastN(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// eventQuestion builds one call: the candidate events, the chunk's
// captures, and one choice question per capture.
func eventQuestion(chunk []eventInput, candidates []*eventView, summaryRunes int) (json.RawMessage, map[string]json.RawMessage, eventAliases) {
	alias := eventAliases{events: map[string]*eventView{}, captures: map[string]int{}}
	events := map[string]any{}
	var eventChoices []string
	for i, v := range candidates {
		a := fmt.Sprintf("e%d", i+1)
		alias.events[a] = v
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
		eventChoices = append(eventChoices, a)
	}
	captures := map[string]any{}
	for j, in := range chunk {
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
		"task":     "Each capture in state.captures is a news item just fetched, in any language. For each, decide whether it reports the same real-world event as a known event in state.events or as another capture in state.captures, or a new event. The same event means the same occurrence or its direct development (a later announcement, reaction or outcome of it). The same topic, the same actors or the same ongoing story in a different occurrence is a new event.",
		"captures": captures,
	}
	if len(events) > 0 {
		state["events"] = events
	}
	questions := map[string]json.RawMessage{}
	for j := range chunk {
		a := fmt.Sprintf("k%d", j+1)
		criteria := map[string]string{"new": "No known event and no other capture reports this occurrence."}
		choices := append([]string(nil), eventChoices...)
		for _, e := range eventChoices {
			criteria[e] = "state.events." + e + " reports the same occurrence as state.captures." + a + "."
		}
		for i := range chunk {
			if i == j {
				continue
			}
			other := fmt.Sprintf("k%d", i+1)
			criteria[other] = "state.captures." + other + " reports the same occurrence, and no known event does."
			choices = append(choices, other)
		}
		choices = append(choices, "new")
		raw, _ := json.Marshal(map[string]any{
			"type":         "choice",
			"instructions": "Which known event or other capture reports the same real-world occurrence as state.captures." + a + "? Prefer a known event over another capture when both match.",
			"criteria":     criteria,
			"choices":      choices,
		})
		questions[a] = raw
	}
	stateRaw, _ := json.Marshal(state)
	return stateRaw, questions, alias
}

// enqueueEventResolution queues a poll's new captures for the next flush,
// when intake.events is on. A flush runs at eventChunkSize queued captures
// or eventFlushInterval after the first, whichever comes first, so captures
// from many sources share calls. One flush at a time drains a computer's
// queue, in order; captures queued meanwhile are drained by the same flush.
func (rt *Runtime) enqueueEventResolution(ctx context.Context, ownerID, computerID, sourceID string, captured []sourcegraph.CapturedItem) {
	if rt == nil || len(captured) == 0 || !rt.computerSettingOn(ctx, ownerID, computerID, store.SettingIntakeEvents) {
		return
	}
	now := time.Now().UTC()
	key := eventKey(ownerID, computerID)
	rt.events.mu.Lock()
	defer rt.events.mu.Unlock()
	if rt.events.stopped {
		return
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
			q.items = append(q.items, eventInput{Item: c, SourceID: sourceID, At: now})
		}
	}
	if q.inFlight || len(q.items) == 0 {
		return
	}
	if len(q.items) >= eventChunkSize {
		rt.startEventFlushLocked(ownerID, computerID, q)
		return
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
		ctx, cancel := context.WithTimeout(layerCtx, eventFlushBudget)
		report, err := rt.resolveEvents(ctx, ownerID, computerID, items)
		cancel()
		if err != nil {
			log.Printf("event layer: %d captures unresolved: %v", len(items), err)
			continue
		}
		log.Printf("event layer: resolved %d captures in %d calls: %d joined, %d tentative, %d new events, %d unresolved, %d corroborated, $%.5f",
			len(items), report.Calls, report.Joined, report.Tentative, report.Minted, report.Unresolved, len(report.Corroborated), report.CostUSD)
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
