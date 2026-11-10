package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/jevrecord"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Intake labels (L2a A1 / R2-3 / R2-4,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). When the
// computer's intake.labels setting is on, a poll's new captures are each
// labelled by Jev against a shortlist of the computer's stories and the
// poll's other new captures, after the captures are written and before the
// arrival record is minted. One request per capture carries two questions:
// which candidate reports the same event (or new), and which story shares
// the topic (or none). Choices are short aliases (c1…, n1…) mapped back
// here. Labels run in parallel within a per-poll budget; a failure or
// timeout leaves the capture unlabelled, and labels never gate arrival.
// Each label is its own choir.capture_label object, never part of the
// arrival record.

// judgmentScopeIntakeLabels is the gateway rate scope of intake labels: its
// own bucket, so a busy poll never starves the support judge.
const judgmentScopeIntakeLabels = "intake_labels"

// judgmentScopeOpedGate is the gateway scope of the paper's op-ed gate.
const judgmentScopeOpedGate = "oped_gate"

// judgmentScopeInteractive is the gateway scope of the latency-critical
// judgments (a desk's question, the prompt router, the disclosure gate): the
// share a bulk burst cannot take.
const judgmentScopeInteractive = "interactive"

// ScopedCandidateJudge is a CandidateJudge that can draw on a named gateway
// rate bucket. A judge without it is called through Judge.
type ScopedCandidateJudge interface {
	JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error)
}

// Budgets. Vars so a test can shorten them.
var (
	// intakeLabelPrepBudget bounds a flush's reads before its calls (the
	// story index and the corrections). There is no deadline across the
	// calls: each gets its own intakeLabelCallTimeout, started when it is
	// about to be sent, and only a stop cancels the rest. Our measured
	// latency is flat in question count (docs/reference/system1/
	// our-usage.md), so 5 s is not expected to bite at 12 questions; the
	// shadow comparison is where that is checked.
	intakeLabelPrepBudget  = 15 * time.Second
	intakeLabelCallTimeout = 5 * time.Second
)

const (
	// intakeFlushCaptures is the most captures one flush takes from the
	// queue (the event layer's chunk size); a call carries at most
	// intakeCallCaptures of them, and the byte guard below can split a call
	// earlier (docs/design/jev-batched-questions-2026-11-10.md, "Intake
	// flush").
	intakeFlushCaptures = 12
	// intakeCallCaptures is the most captures one call carries (two
	// questions each, so 12). The flush holds 12 captures, but accuracy
	// above 12 questions a call is unmeasured and the old code carried at
	// most 8; further calls follow in sequence. Raise it only after the
	// shadow comparison of docs/design/jev-batched-questions-2026-11-10.md
	// measures accuracy at the larger size.
	intakeCallCaptures = 6
	// intakeBatchCaptures is the count handed to labelPoll's grouping.
	intakeBatchCaptures = intakeCallCaptures
	// Owner rule 2026-11-10: never call Jev concurrently, put many questions
	// in single calls. The flush is a group commit with no timer: the first
	// capture is asked at once, and captures that arrive while a call is in
	// flight go out together in the next, so a burst batches by itself and a
	// quiet moment costs one call. Compiled constants, not settings.
	// intakeRequestMaxBytes is the most a call's state and questions may
	// carry: under OpenRouter's ~32,000-byte line past which Jev runs a
	// slower, less accurate long-context mode. A group of captures that
	// would cross it is split, never trimmed.
	intakeRequestMaxBytes = 30000
	// intakeChunkStateBytes is the most the stories of one call may take (the
	// state's story entries plus each story's two option names in a
	// question); the stories are cut into chunks by bytes, not by a count, so
	// a quiet window fits one call. About half the guard is left for the
	// captures' own state and questions.
	intakeChunkStateBytes     = 15000
	intakeShortlistOverlap    = 10
	intakeShortlistHorizon    = 72 * time.Hour
	intakeOverrideExamples    = 12
	intakeSummaryRunes        = 600
	intakeSiblingSummaryRunes = 200
	// intakeNoCandidatesModel marks a label written without a call because
	// there was nothing to compare the capture with.
	intakeNoCandidatesModel = "none"
)

// intakeLibrary is the per-poll view of the computer's stories: the
// newest within the horizon, the rest within it for title overlap, and a
// cache of the summaries read.
type intakeLibrary struct {
	rt       *Runtime
	recent   []types.Document
	rest     []types.Document
	mu       sync.Mutex
	summary  map[string]store.StorySummary
	examples []store.CaptureLabelOverride
	// stats are the poll's call counts, set by labelPoll for the poll's one
	// log line.
	stats intakePollStats
}

// intakePollStats are counts only: the calls a poll's labels made, their
// questions and the largest request.
type intakePollStats struct{ calls, questions, maxBytes int }

func (lib *intakeLibrary) summaryOf(ctx context.Context, doc types.Document) store.StorySummary {
	lib.mu.Lock()
	cached, ok := lib.summary[doc.DocID]
	lib.mu.Unlock()
	if ok {
		return cached
	}
	// Every candidate comes from the story index (X9), so a miss is a story
	// the index no longer lists: the document's own fields stand in.
	return store.StorySummary{DocID: doc.DocID, TrajectoryID: doc.TrajectoryID, Headline: doc.Title, UpdatedAt: doc.UpdatedAt, RevisionID: doc.CurrentRevisionID}
}

// intakeShortlistSplit splits stories (any order) into those updated within
// the shortlist window (newest first, at most intakeShortlistCeiling) and the
// rest within the overlap horizon.
func intakeShortlistSplit(docs []types.Document, now time.Time) (recent, rest []types.Document) {
	live := make([]types.Document, 0, len(docs))
	for _, d := range docs {
		if now.Sub(d.UpdatedAt) <= intakeShortlistHorizon {
			live = append(live, d)
		}
	}
	sort.SliceStable(live, func(i, j int) bool { return live[i].UpdatedAt.After(live[j].UpdatedAt) })
	in := 0
	for in < len(live) && in < intakeShortlistCeiling && (in < intakeShortlistFloor || now.Sub(live[in].UpdatedAt) <= intakeShortlistWindow) {
		in++
	}
	return live[:in], live[in:]
}

var intakeStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "after": true, "over": true, "says": true, "into": true,
	"that": true, "this": true, "are": true, "was": true, "has": true, "have": true, "its": true, "les": true, "des": true,
	"une": true, "los": true, "las": true, "del": true, "por": true, "con": true, "que": true, "der": true, "die": true,
	"das": true, "und": true, "mit": true, "von": true, "para": true, "com": true, "uma": true,
}

// titleTokens are a title's lower-cased words of three or more runes.
func titleTokens(title string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		if len([]rune(w)) >= 3 && !intakeStopwords[w] {
			out[w] = true
		}
	}
	return out
}

// intakeOverlapPicks returns up to n of rest that share the most title
// tokens with title, at least one; ties keep rest's (newest first) order.
func intakeOverlapPicks(rest []types.Document, title string, n int) []types.Document {
	want := titleTokens(title)
	if len(want) == 0 || n <= 0 {
		return nil
	}
	type scored struct {
		doc   types.Document
		score int
	}
	var hits []scored
	for _, d := range rest {
		score := 0
		for w := range titleTokens(d.Title) {
			if want[w] {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, scored{d, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]types.Document, 0, n)
	for _, h := range hits {
		if len(out) == n {
			break
		}
		out = append(out, h.doc)
	}
	return out
}

func intakeClip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > n {
		return strings.TrimSpace(string(runes[:n])) + "…"
	}
	return s
}

// intakeTicket is one enqueue's wait handle: its channel closes when every
// capture of the enqueue is resolved (labelled, failed, panicked and
// recovered, or abandoned at stop).
type intakeTicket struct {
	left atomic.Int32
	done chan struct{}
}

func newIntakeTicket(n int) *intakeTicket {
	t := &intakeTicket{done: make(chan struct{})}
	t.left.Store(int32(n))
	if n == 0 {
		close(t.done)
	}
	return t
}

// resolve marks one capture resolved.
func (t *intakeTicket) resolve() {
	if t.left.Add(-1) == 0 {
		close(t.done)
	}
}

// intakeHeld is a capture waiting for its flush, with the source that
// captured it and when it was held.
type intakeHeld struct {
	item     sourcegraph.CapturedItem
	sourceID string
	at       time.Time
	ticket   *intakeTicket
}

// resolveIntakeHeld marks every capture of a flush resolved.
func resolveIntakeHeld(held []intakeHeld) {
	for _, h := range held {
		if h.ticket != nil {
			h.ticket.resolve()
		}
	}
}

type intakeLabelQueue struct {
	owner, computer string
	items           []intakeHeld
}

// intakeLabelLayer is the runtime's intake-label queues, one per computer,
// drained by ONE flusher goroutine per runtime: a runtime may be handed
// polls for more than one computer (nothing in HandleSourcePollDue checks
// the computer against the runtime's configured one), and the owner's rule
// is that Jev is never called concurrently, so the flusher drains the
// queues in turn. It is a group commit: the flusher runs whenever anything
// is held and no call is in flight, so the first capture is asked at once
// and captures that arrive during a call go out together in the next.
// The queues are in memory only: a capture still held at a crash or a
// bounded stop is left unlabelled, exactly as a capture whose label call
// failed (nothing retries a capture; the next poll sees it as already
// captured). No boot step resumes it.
type intakeLabelLayer struct {
	mu      sync.Mutex
	queues  map[string]*intakeLabelQueue
	stopped bool
	running bool // the one flusher goroutine is alive
	ctx     context.Context
	cancel  context.CancelFunc
	// flushes counts the flusher goroutine, so a graceful stop can wait for
	// it (bounded).
	flushes sync.WaitGroup
}

// intakeFlushTestHook, when set by a test, runs at the named stage of a
// flush ("flusher": in the flusher goroutine before a flush; "labelHeld":
// inside the flush) so a test can make that stage panic or block. Nil in
// production.
var intakeFlushTestHook func(stage string)

// enqueueIntakeLabels holds a poll's new captures for the computer's next
// flush and returns a channel closed when all of them are resolved. The
// caller waits on it (and on its own ctx) before announcing the captures,
// so labels exist before the wake; it must hold no lock while waiting. A
// refused enqueue (stopped, or no judge) returns an already closed channel.
func (rt *Runtime) enqueueIntakeLabels(ownerID, computerID, sourceID string, captured []sourcegraph.CapturedItem) <-chan struct{} {
	if rt == nil || rt.candidateJudge == nil || rt.store == nil {
		return newIntakeTicket(0).done
	}
	var valid []sourcegraph.CapturedItem
	for _, c := range captured {
		if strings.TrimSpace(c.CaptureID) != "" {
			valid = append(valid, c)
		}
	}
	ticket := newIntakeTicket(len(valid))
	if len(valid) == 0 {
		return ticket.done
	}
	now := time.Now().UTC()
	key := eventKey(ownerID, computerID)
	l := &rt.intakeLabels
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		for range valid {
			ticket.resolve()
		}
		return ticket.done
	}
	if l.queues == nil {
		l.queues = map[string]*intakeLabelQueue{}
	}
	q := l.queues[key]
	if q == nil {
		q = &intakeLabelQueue{owner: ownerID, computer: computerID}
		l.queues[key] = q
	}
	for _, c := range valid {
		q.items = append(q.items, intakeHeld{item: c, sourceID: sourceID, at: now, ticket: ticket})
	}
	if !l.running {
		rt.startIntakeFlusherLocked()
	}
	return ticket.done
}

// startIntakeFlusherLocked starts the runtime's one flusher. The caller
// holds l.mu, l.running is false and the layer is not stopped.
func (rt *Runtime) startIntakeFlusherLocked() {
	l := &rt.intakeLabels
	if l.ctx == nil {
		l.ctx, l.cancel = context.WithCancel(context.Background())
	}
	l.running = true
	ctx := l.ctx
	l.flushes.Add(1)
	go func() {
		defer l.flushes.Done()
		defer func() {
			// A panic must not kill the process or leave the layer marked
			// running: that flush's captures stay unlabelled (their wait
			// handles are closed by the flush's own defer) and the flusher
			// restarts if anything is still held.
			if r := recover(); r != nil {
				log.Printf("intake labels: flusher panicked: %v; that flush's captures stay unlabelled", r)
				l.mu.Lock()
				l.running = false
				if !l.stopped && l.anyHeldLocked() {
					rt.startIntakeFlusherLocked()
				}
				l.mu.Unlock()
			}
		}()
		rt.runIntakeFlusher(ctx)
	}()
}

func (l *intakeLabelLayer) anyHeldLocked() bool {
	for _, q := range l.queues {
		if len(q.items) > 0 {
			return true
		}
	}
	return false
}

// runIntakeFlusher drains the computers' queues one flush at a time, the
// queue whose first capture waited longest first, then exits.
func (rt *Runtime) runIntakeFlusher(ctx context.Context) {
	l := &rt.intakeLabels
	for {
		l.mu.Lock()
		var next *intakeLabelQueue
		if !l.stopped {
			for _, q := range l.queues {
				if len(q.items) > 0 && (next == nil || q.items[0].at.Before(next.items[0].at)) {
					next = q
				}
			}
		}
		if next == nil {
			l.running = false
			l.mu.Unlock()
			return
		}
		n := min(len(next.items), intakeFlushCaptures)
		held := append([]intakeHeld(nil), next.items[:n]...)
		next.items = append([]intakeHeld(nil), next.items[n:]...)
		owner, computer := next.owner, next.computer
		l.mu.Unlock()
		func() {
			// Every exit path, a panic included, resolves the flush's
			// captures so no poll waits on a flush that is gone.
			defer resolveIntakeHeld(held)
			if hook := intakeFlushTestHook; hook != nil {
				hook("flusher")
			}
			rt.labelHeld(ctx, owner, computer, held)
		}()
	}
}

// intakeStopGrace is how long Stop waits, after the drain deadline cancelled
// the flush, for cancelled calls to return before the store may close.
const intakeStopGrace = 2 * time.Second

// stopIntakeLabels is the graceful stop: it refuses new captures, lets a
// running flush finish, flushes once what is held, all bounded by the
// runtime's drain deadline, then cancels what is left and waits (briefly)
// for it to return, so nothing touches the store as it closes. Every wait
// handle is closed. Nothing is persisted and nothing runs at boot, so a
// crash still drops held captures exactly as a failed label does. Called
// from Stop before the store closes.
func (rt *Runtime) stopIntakeLabels() {
	l := &rt.intakeLabels
	l.mu.Lock()
	already := l.stopped
	l.stopped = true
	type heldQueue struct {
		owner, computer string
		items           []intakeHeld
	}
	var rest []heldQueue
	for _, q := range l.queues {
		if len(q.items) > 0 && !already {
			rest = append(rest, heldQueue{q.owner, q.computer, q.items})
		}
		q.items = nil
	}
	if l.ctx == nil {
		l.ctx, l.cancel = context.WithCancel(context.Background())
	}
	l.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), rt.drainDeadline())
	defer cancel()
	// At the bound, whatever still runs is cancelled.
	stop := context.AfterFunc(ctx, l.cancel)
	defer stop()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		l.flushes.Wait()
		for _, h := range rest {
			for len(h.items) > 0 && ctx.Err() == nil {
				n := min(len(h.items), intakeFlushCaptures)
				func() {
					defer resolveIntakeHeld(h.items[:n])
					rt.labelHeld(ctx, h.owner, h.computer, h.items[:n])
				}()
				h.items = h.items[n:]
			}
			resolveIntakeHeld(h.items) // abandoned at the bound
		}
	}()
	select {
	case <-finished:
	case <-ctx.Done():
		log.Printf("intake labels: stop: drain deadline reached; cancelling, held captures stay unlabelled")
		l.cancel()
		select {
		case <-finished:
		case <-time.After(intakeStopGrace):
			log.Printf("intake labels: stop: the flush did not return within %v of its cancellation", intakeStopGrace)
		}
	}
	l.cancel()
}

// labelHeld is the flush: it labels the held captures, in the order held, and
// reports how many were labelled and how many were left unlabelled. Its calls
// run one after another and it returns within the flush budget whatever Jev
// does.
func (rt *Runtime) labelHeld(ctx context.Context, ownerID, computerID string, held []intakeHeld) (labelled, unlabelled int) {
	if len(held) == 0 {
		return 0, 0
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("intake labels: flush of %d captures panicked: %v; they stay unlabelled", len(held), r)
			labelled, unlabelled = 0, len(held)
		}
	}()
	if hook := intakeFlushTestHook; hook != nil {
		hook("labelHeld")
	}
	items := make([]sourcegraph.CapturedItem, len(held))
	for i, h := range held {
		items[i] = h.item
	}
	sourceID := held[0].sourceID
	waited := time.Since(held[0].at)
	judge := rt.candidateJudge
	if judge == nil || rt.store == nil {
		return 0, len(items)
	}
	// The labeler is management's: it reads the story index Texture
	// publishes, never Texture's documents or revisions (X9).
	ctx = withKindReader(ctx, "choir.capture_label")
	prep, cancelPrep := context.WithTimeout(ctx, intakeLabelPrepBudget)
	defer cancelPrep()
	lib := &intakeLibrary{rt: rt, summary: map[string]store.StorySummary{}}
	if err := rt.store.PublishStoryIndex(prep, ownerID, computerID); err != nil {
		log.Printf("intake labels: flush: publish story index: %v; labelling against siblings only", err)
	} else if sums, err := rt.store.ListStoryIndex(prep, ownerID, computerID, time.Time{}, "", 0); err != nil {
		log.Printf("intake labels: flush: list stories: %v; labelling against siblings only", err)
	} else {
		docs := make([]types.Document, 0, len(sums))
		for _, sum := range sums {
			lib.summary[sum.DocID] = sum
			docs = append(docs, types.Document{DocID: sum.DocID, TrajectoryID: sum.TrajectoryID, Title: sum.Headline, UpdatedAt: sum.UpdatedAt, CurrentRevisionID: sum.RevisionID})
		}
		lib.recent, lib.rest = intakeShortlistSplit(docs, time.Now().UTC())
	}
	if examples, err := rt.store.RecentCaptureLabelOverrides(prep, ownerID, computerID, intakeOverrideExamples); err != nil {
		log.Printf("intake labels: flush: read corrections: %v", err)
	} else {
		lib.examples = examples
	}

	results := rt.labelPoll(ctx, lib, ownerID, computerID, sourceID, items, intakeBatchCaptures)
	for i, r := range results {
		r.Value.SourceID = held[i].sourceID
		if r.Err == nil {
			// In the poll's order; a label that cannot be written is
			// unlabelled like one Jev did not answer.
			_, r.Err = rt.store.AppendCaptureLabel(withKindCustodian(ctx, "choir.capture_label"), r.Value)
		}
		if r.Err != nil {
			// A capture a stop cancelled before it was asked is left
			// unlabelled without a line.
			if ctx.Err() == nil {
				log.Printf("intake labels: flush: %s unlabelled: %v", items[i].CaptureID, r.Err)
			}
			continue
		}
		labelled++
	}
	if ctx.Err() != nil {
		notAsked := 0
		for _, r := range results {
			if r.Err != nil && errors.Is(r.Err, context.Canceled) {
				notAsked++
			}
		}
		log.Printf("intake labels: stopped during the flush: %d of %d captures not asked or unanswered", notAsked, len(items))
	}
	// One line per flush, counts only: the per-capture lines above stop when
	// the budget ends, and a slow Jev would otherwise leave no trace.
	log.Printf("intake labels: flush captures=%d calls=%d questions=%d bytes_max=%d waited_ms=%d unlabelled=%d cancelled=%t",
		len(items), lib.stats.calls, lib.stats.questions, lib.stats.maxBytes, waited.Milliseconds(), len(items)-labelled, ctx.Err() != nil)
	return labelled, len(items) - labelled
}

// intakeCaptureState is what Jev reads of one capture.
func intakeCaptureState(c sourcegraph.CapturedItem, summaryRunes int) map[string]any {
	state := map[string]any{"title": strings.TrimSpace(c.Title)}
	for k, v := range map[string]string{"summary": intakeClip(c.Summary, summaryRunes), "outlet": c.Outlet, "language": c.Language, "script": c.Script, "url": c.URL, "published_at": c.PublishedAt} {
		if v = strings.TrimSpace(v); v != "" {
			state[k] = v
		}
	}
	return state
}

// intakePoll is the poll's captures against the poll's candidate stories:
// the stories listed once, under stable handles (c1, c2, … newest first), and
// the captures under stable handles (n1, n2, … in poll order), so a question
// names its capture and its choices by handle and the story list lives in the
// call's state, not in every question.
type intakePoll struct {
	items []sourcegraph.CapturedItem
	// handles[i] is items[i]'s handle (n<i+1>).
	handles []string
	// stories lists every candidate of the poll in handle order: the stories
	// updated within the window, then the title-overlap picks of the poll's
	// captures that are not already in the window.
	stories []intakeStory
	// views[i] is what items[i] is compared with.
	views []intakeView
	// cuts[k] is the end of chunk k in stories (see intakeChunkStateBytes).
	cuts []int
}

// intakeLabelResult is one capture's label, or why it has none.
type intakeLabelResult struct {
	Value store.CaptureLabel
	Err   error
}

type intakeStory struct {
	handle string
	sum    store.StorySummary
	entry  string
}

// intakeView is one capture's candidates and siblings.
type intakeView struct {
	// candidate marks the story handles this capture is compared with: the
	// window, plus its own overlap picks.
	candidate map[string]bool
	// order lists those handles in the capture's own order: the window
	// newest first, then its own overlap picks in its own score order. The
	// first option is the one Jev leans toward, so each question offers its
	// capture's options in this order, whatever order the shared state lists
	// the poll's union of stories in.
	order []string
	// siblings are the handles of the poll's other captures.
	siblings []string
}

// newIntakePoll builds the poll's handles and each capture's candidates: the
// stories updated within the window, then the rest by title overlap, as the
// per-capture path chose them.
func newIntakePoll(ctx context.Context, lib *intakeLibrary, items []sourcegraph.CapturedItem) (*intakePoll, error) {
	p := &intakePoll{items: items, handles: make([]string, len(items)), views: make([]intakeView, len(items))}
	byDoc := map[string]string{}
	addStory := func(doc types.Document) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if h, ok := byDoc[doc.DocID]; ok {
			return h, nil
		}
		sum := lib.summaryOf(ctx, doc)
		n := len(p.stories)
		h := fmt.Sprintf("c%d", n+1)
		// The entry starts with exactly its handle, the option name Jev is
		// offered: one hop from the option to its story.
		entry := h + ": " + sum.Headline + " (" + sum.UpdatedAt.UTC().Format(time.RFC3339) + ")"
		if sum.Lede != "" && n < intakeLedeCandidates {
			entry += " — " + sum.Lede
		}
		byDoc[doc.DocID] = h
		p.stories = append(p.stories, intakeStory{handle: h, sum: sum, entry: entry})
		return h, nil
	}
	recent := make([]string, 0, len(lib.recent))
	for _, doc := range lib.recent {
		h, err := addStory(doc)
		if err != nil {
			return nil, err
		}
		recent = append(recent, h)
	}
	for i, item := range items {
		p.handles[i] = fmt.Sprintf("n%d", i+1)
		v := intakeView{candidate: map[string]bool{}}
		for _, h := range recent {
			v.candidate[h] = true
			v.order = append(v.order, h)
		}
		for _, doc := range intakeOverlapPicks(lib.rest, item.Title, intakeShortlistOverlap) {
			h, err := addStory(doc)
			if err != nil {
				return nil, err
			}
			if !v.candidate[h] {
				v.candidate[h] = true
				v.order = append(v.order, h)
			}
		}
		p.views[i] = v
	}
	// The stories are cut into chunks by bytes, never by a count: a chunk
	// closes when the next story would take it past intakeChunkStateBytes.
	size := 0
	for n, s := range p.stories {
		cost := len(s.entry) + 2*(len(s.handle)+8)
		if size > 0 && size+cost > intakeChunkStateBytes {
			p.cuts = append(p.cuts, n)
			size = 0
		}
		size += cost
	}
	if len(p.stories) > 0 {
		p.cuts = append(p.cuts, len(p.stories))
	}
	for i := range items {
		for j := range items {
			if j != i {
				p.views[i].siblings = append(p.views[i].siblings, p.handles[j])
			}
		}
	}
	return p, nil
}

// candidatesOf lists a capture's candidate stories in its own order.
func (p *intakePoll) candidatesOf(i int) []intakeStory {
	var out []intakeStory
	for _, h := range p.views[i].order {
		out = append(out, p.stories[storyIndex(h)])
	}
	return out
}

// storyIndex is the position of a story handle (c<n>) in intakePoll.stories.
func storyIndex(handle string) int {
	var n int
	_, _ = fmt.Sscanf(handle, "c%d", &n)
	return n - 1
}

// chunkCount is how many chunks of at most intakeChunkStateBytes of stories the
// poll's candidates make (one, even with none: the siblings' call).
func (p *intakePoll) chunkCount() int {
	return max(1, len(p.cuts))
}

func (p *intakePoll) chunk(k int) []intakeStory {
	if k >= len(p.cuts) {
		return nil
	}
	lo := 0
	if k > 0 {
		lo = p.cuts[k-1]
	}
	return p.stories[lo:p.cuts[k]]
}

// intakeAsk is one capture's questions in one call: the choices offered and
// the questions' names.
type intakeAsk struct {
	capture      int
	eventName    string
	topicName    string
	eventChoices []string
	storyChoices []string
}

// intakeCall is one Jev call: a chunk's stories (the state, sent once) and
// the event and topic questions of a group of captures.
type intakeCall struct {
	chunk int
	// first is the first capture's id, the record's DecisionID; each
	// question joins to its own capture through decisions.
	first     string
	asks      []intakeAsk
	state     json.RawMessage
	questions map[string]json.RawMessage
	decisions map[string]string
}

func (c intakeCall) bytes() int {
	n := len(c.state)
	for _, q := range c.questions {
		n += len(q)
	}
	return n
}

// buildCall builds chunk k's call for the captures in members.
func (rt *Runtime) buildIntakeCall(lib *intakeLibrary, p *intakePoll, k int, members []int) (intakeCall, error) {
	stories := p.chunk(k)
	var storyState []string
	for _, s := range stories {
		storyState = append(storyState, s.entry)
	}
	captures := map[string]any{}
	if k == 0 {
		// The siblings ride the first chunk's calls only.
		for j, other := range p.items {
			captures[p.handles[j]] = intakeCaptureState(sourcegraph.CapturedItem{Title: other.Title, Summary: other.Summary, Outlet: other.Outlet, Language: other.Language}, intakeSiblingSummaryRunes)
		}
	}
	state := map[string]any{
		"task":     "News captures have arrived (state.captures). Each question is about one of them: which story in state.stories or other new capture reports the same real-world event, and which story shares its topic. Titles and summaries may be in any language; read them all.",
		"captures": captures,
	}
	if len(storyState) > 0 {
		state["stories"] = storyState
	}
	if len(lib.examples) > 0 {
		var corrections []map[string]string
		for _, o := range lib.examples {
			corrections = append(corrections, map[string]string{"capture": o.CaptureTitle, "label_said": o.JevSaid, "editor_did": overrideDisposition(o), "reason": o.Reason})
		}
		state["corrections"] = corrections
	}
	call := intakeCall{chunk: k, first: p.items[members[0]].CaptureID, questions: map[string]json.RawMessage{}, decisions: map[string]string{}}
	for _, i := range members {
		captures[p.handles[i]] = intakeCaptureState(p.items[i], intakeSummaryRunes)
		ask := intakeAsk{capture: i, eventName: "event_" + p.handles[i]}
		inChunk := map[string]bool{}
		for _, s := range stories {
			inChunk[s.handle] = true
		}
		for _, h := range p.views[i].order {
			if inChunk[h] {
				ask.storyChoices = append(ask.storyChoices, h)
			}
		}
		var siblingChoices []string
		if k == 0 {
			siblingChoices = p.views[i].siblings
		}
		// Choice order is as it was: the stories newest first, then the other
		// captures, then new (and none last for the topic). The first option
		// is what Jev leans toward, so the order is not changed here.
		ask.eventChoices = append(append(append([]string(nil), ask.storyChoices...), siblingChoices...), store.CaptureLabelEventNew)
		// The option set is the keys of criteria (the vendor's schema has no
		// choices field): a handle has a null description, read by name
		// alone, and the state entry carries what it means.
		event, err := json.Marshal(map[string]any{
			"type":         "choice",
			"instructions": intakeEventInstructions(p.handles[i], len(ask.storyChoices) > 0, len(siblingChoices) > 0, len(lib.examples) > 0),
			"criteria":     intakeCriteria(ask.eventChoices, map[string]string{store.CaptureLabelEventNew: "No story and no other new capture reports this event; a shared topic or shared actors alone do not count."}),
		})
		if err != nil {
			return intakeCall{}, err
		}
		call.questions[ask.eventName] = event
		call.decisions[ask.eventName] = p.items[i].CaptureID
		if len(ask.storyChoices) > 0 {
			ask.topicName = "topic_" + p.handles[i]
			topic, err := json.Marshal(map[string]any{
				"type":         "choice",
				"instructions": "Which story in state.stories shares state.captures." + p.handles[i] + "'s topic: the same ongoing subject (a war, an election, a company, a trial), even when it is a different event? The options are the story handles (c1, c2, … each at the start of its entry in state.stories) and none; answer none when no story does.",
				"criteria":     intakeCriteria(append(append([]string(nil), ask.storyChoices...), "none"), map[string]string{"none": "No story shares the capture's topic."}),
			})
			if err != nil {
				return intakeCall{}, err
			}
			call.questions[ask.topicName] = topic
			call.decisions[ask.topicName] = p.items[i].CaptureID
		}
		call.asks = append(call.asks, ask)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return intakeCall{}, err
	}
	call.state = raw
	return call, nil
}

// intakeCriteria is a choice question's criteria: the options in the order
// given, each a key, with its description or null (a choice without one is
// read by its name alone). The vendor reads the option set from these keys,
// and the first option is the one Jev leans toward, so the order is written
// out here and not left to a Go map's sorted keys.
func intakeCriteria(options []string, described map[string]string) json.RawMessage {
	var b strings.Builder
	b.WriteByte('{')
	for n, option := range options {
		if n > 0 {
			b.WriteByte(',')
		}
		name, _ := json.Marshal(option)
		b.Write(name)
		b.WriteByte(':')
		if text, ok := described[option]; ok {
			desc, _ := json.Marshal(text)
			b.Write(desc)
		} else {
			b.WriteString("null")
		}
	}
	b.WriteByte('}')
	return json.RawMessage(b.String())
}

// intakeEventInstructions is the event question's text. It names the capture
// by handle and the candidates by their kinds; the candidates themselves are
// in state.stories and state.captures, once for the call.
func intakeEventInstructions(handle string, stories, siblings, corrections bool) string {
	var entries []string
	if stories {
		entries = append(entries, "a story (c1, c2, … in state.stories)")
	}
	if siblings {
		entries = append(entries, "another new capture (n1, n2, … in state.captures, never "+handle+" itself)")
	}
	text := "Which entry reports the same real-world event as state.captures." + handle + "?"
	if len(entries) > 0 {
		text += " The entries are " + strings.Join(entries, " and ") + "; an entry reports the same event when it covers the same occurrence or its direct development (a later announcement, reaction or outcome of that occurrence), in any language."
	}
	if stories && siblings {
		text += " Prefer a story over a new capture when both match."
	}
	text += " The options are handles: each story's handle (c1, c2, …) is at the start of its entry in state.stories, each other capture's handle (n1, n2, …) is its key in state.captures, and new. The same topic or the same actors in a different event is new: answer new when no entry reports this event."
	if corrections {
		text += " state.corrections are earlier labels the editor corrected, with the reason."
	}
	return text
}

// planIntakeCalls lists the poll's calls, chunk by chunk: a chunk's captures
// (those with a candidate in it, and in the first chunk those with siblings)
// in groups of at most batch captures, and a group is closed early when
// adding a capture would take the request over intakeRequestMaxBytes, so a
// capture is never dropped, only moved to the next group of the same chunk.
func (rt *Runtime) planIntakeCalls(lib *intakeLibrary, p *intakePoll, batch int, live []int) ([]intakeCall, error) {
	var calls []intakeCall
	for k := 0; k < p.chunkCount(); k++ {
		inChunk := map[string]bool{}
		for _, s := range p.chunk(k) {
			inChunk[s.handle] = true
		}
		var needs []int
		for _, i := range live {
			asked := k == 0 && len(p.views[i].siblings) > 0
			for h := range p.views[i].candidate {
				if inChunk[h] {
					asked = true
					break
				}
			}
			if asked {
				needs = append(needs, i)
			}
		}
		var group []int
		var built intakeCall
		flush := func() {
			if len(group) > 0 {
				calls = append(calls, built)
			}
			group, built = nil, intakeCall{}
		}
		for _, i := range needs {
			trial := append(append([]int(nil), group...), i)
			next, err := rt.buildIntakeCall(lib, p, k, trial)
			if err != nil {
				return nil, err
			}
			if len(group) > 0 && (len(trial) > max(batch, 1) || next.bytes() > intakeRequestMaxBytes) {
				flush()
				if next, err = rt.buildIntakeCall(lib, p, k, []int{i}); err != nil {
					return nil, err
				}
				trial = []int{i}
			}
			group, built = trial, next
		}
		flush()
	}
	return calls, nil
}

// intakeCallAnswer is one capture's answer to one call, or why it has none.
type intakeCallAnswer struct {
	answer intakeAnswer
	err    error
}

// labelPoll labels items against lib and returns each capture's label or why
// it has none, in the poll's order. It asks one call per candidate chunk and
// group of at most batch captures (the questions of a group share the chunk's
// stories, sent once); a capture's answer is read from its own questions, so
// one capture's bad answer, or one failed call, leaves the others' labels.
func (rt *Runtime) labelPoll(ctx context.Context, lib *intakeLibrary, ownerID, computerID, sourceID string, items []sourcegraph.CapturedItem, batch int) []intakeLabelResult {
	out := make([]intakeLabelResult, len(items))
	p, err := newIntakePoll(ctx, lib, items)
	if err != nil {
		for i := range out {
			out[i].Err = err
		}
		return out
	}
	var live []int
	for i, capture := range items {
		label := store.CaptureLabel{OwnerID: ownerID, ComputerID: computerID, CaptureID: capture.CaptureID, SourceID: sourceID,
			Title: strings.TrimSpace(capture.Title), Outlet: capture.Outlet, Language: capture.Language, Script: capture.Script}
		for _, s := range p.candidatesOf(i) {
			label.CandidateDocIDs = append(label.CandidateDocIDs, s.sum.DocID)
		}
		for _, h := range p.views[i].siblings {
			label.SiblingCaptureIDs = append(label.SiblingCaptureIDs, p.items[siblingIndex(h)].CaptureID)
		}
		out[i].Value = label
		if len(label.CandidateDocIDs) == 0 && len(p.views[i].siblings) == 0 {
			// Nothing to compare the capture with.
			out[i].Value.Event, out[i].Value.EventP, out[i].Value.Model = store.CaptureLabelEventNew, 1, intakeNoCandidatesModel
			continue
		}
		live = append(live, i)
	}
	calls, err := rt.planIntakeCalls(lib, p, batch, live)
	if err != nil {
		for _, i := range live {
			out[i].Err = err
		}
		return out
	}
	lib.stats = intakePollStats{calls: len(calls)}
	for _, c := range calls {
		lib.stats.questions += len(c.questions)
		lib.stats.maxBytes = max(lib.stats.maxBytes, c.bytes())
	}
	// answers[i][k] is capture i's answer to chunk k's call.
	answers := make([][]intakeCallAnswer, len(items))
	for _, i := range live {
		answers[i] = make([]intakeCallAnswer, p.chunkCount())
		for k := range answers[i] {
			answers[i][k].err = errIntakeNotAsked
		}
	}
	// One call after another, never concurrently: Jev answers a call's
	// questions in parallel, so the way to go faster is a bigger call.
	for _, c := range calls {
		var ans []intakeCallAnswer
		err := ctx.Err()
		if err == nil {
			ans, err = rt.askIntakeCall(ctx, lib, c)
		}
		for a, ask := range c.asks {
			if err != nil {
				answers[ask.capture][c.chunk] = intakeCallAnswer{err: err}
			} else {
				answers[ask.capture][c.chunk] = ans[a]
			}
		}
	}
	for _, i := range live {
		out[i] = rt.mergeIntakeAnswers(p, i, out[i].Value, answers[i])
	}
	return out
}

var errIntakeNotAsked = fmt.Errorf("intake: the capture was not asked")

func siblingIndex(handle string) int {
	var n int
	_, _ = fmt.Sscanf(handle, "n%d", &n)
	return n - 1
}

// mergeIntakeAnswers merges a capture's chunk answers, in chunk order, into
// its label: the story with the highest event probability wins, a sibling
// only when no story matched, and new only when no chunk named anything.
func (rt *Runtime) mergeIntakeAnswers(p *intakePoll, i int, label store.CaptureLabel, answers []intakeCallAnswer) intakeLabelResult {
	storyOf := map[string]store.StorySummary{}
	for _, s := range p.stories {
		storyOf[s.handle] = s.sum
	}
	siblingOf := map[string]string{}
	for _, h := range p.views[i].siblings {
		siblingOf[h] = p.items[siblingIndex(h)].CaptureID
	}
	var event, topic intakeAnswer
	newP, haveNew := 1.0, false
	for _, r := range answers {
		if r.err == errIntakeNotAsked {
			continue // no candidate of the capture is in this chunk
		}
		if r.err != nil {
			// A capture that missed part of the window is unlabelled, not
			// labelled "new" on part of the evidence.
			return intakeLabelResult{Value: label, Err: r.err}
		}
		a := r.answer
		label.Model = firstNonEmpty(label.Model, a.model)
		label.InputTokens, label.OutputTokens, label.CostUSD = label.InputTokens+a.inputTokens, label.OutputTokens+a.outputTokens, label.CostUSD+a.cost
		switch {
		case a.event == store.CaptureLabelEventNew:
			haveNew = true
			newP = min(newP, a.eventP)
		case storyOf[a.event].DocID != "":
			if event.event == "" || storyOf[event.event].DocID == "" || a.eventP > event.eventP {
				event = a
			}
		case siblingOf[a.event] != "":
			if event.event == "" || (storyOf[event.event].DocID == "" && a.eventP > event.eventP) {
				event = a
			}
		default:
			return intakeLabelResult{Value: label, Err: fmt.Errorf("Jev chose %q, which is not a choice", a.event)}
		}
		if storyOf[a.topic].DocID != "" && (topic.topic == "" || a.topicP > topic.topicP) {
			topic = a
		}
	}
	switch choice := event.event; {
	case choice == "" && haveNew:
		label.Event, label.EventP = store.CaptureLabelEventNew, newP
	case storyOf[choice].DocID != "":
		label.Event, label.EventDocID, label.EventHeadline, label.EventP = store.CaptureLabelEventStory, storyOf[choice].DocID, storyOf[choice].Headline, event.eventP
	case siblingOf[choice] != "":
		label.Event, label.EventCaptureID, label.EventP = store.CaptureLabelEventSibling, siblingOf[choice], event.eventP
	default:
		return intakeLabelResult{Value: label, Err: fmt.Errorf("Jev gave no event answer")}
	}
	if sum := storyOf[topic.topic]; sum.DocID != "" {
		label.TopicDocID, label.TopicHeadline, label.TopicP = sum.DocID, sum.Headline, topic.topicP
	}
	label.Model = firstNonEmpty(label.Model, candidateJudgmentModel)
	return intakeLabelResult{Value: label}
}

// askIntakeCall makes one call and reads each of its captures' answers from
// that capture's own questions. The slice is in call.asks order. A capture
// whose answer is missing or off the menu carries its own error; the call's
// usage is split among the captures by question count.
func (rt *Runtime) askIntakeCall(ctx context.Context, lib *intakeLibrary, call intakeCall) ([]intakeCallAnswer, error) {
	meta := jevrecord.Meta{Site: "intake_labels", DecisionID: call.first, Scope: judgmentScopeIntakeLabels, QuestionDecisions: call.decisions}
	raw, err := rt.judgeCall(ctx, meta, intakeLabelCallTimeout, call.state, call.questions)
	if err != nil {
		return nil, err
	}
	var distribution struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Choice        string             `json:"choice"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
		Usage struct {
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
			Cost         float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &distribution); err != nil {
		return nil, fmt.Errorf("decode Jev's answer: %w", err)
	}
	total := len(call.questions)
	out := make([]intakeCallAnswer, len(call.asks))
	for a, ask := range call.asks {
		share := 1.0
		if total > 0 {
			share = float64(1+btoi(ask.topicName != "")) / float64(total)
		}
		event, ok := distribution.Answers[ask.eventName]
		if !ok {
			out[a].err = fmt.Errorf("Jev gave no event answer")
			continue
		}
		choice := strings.TrimSpace(event.Choice)
		if !intakeHas(ask.eventChoices, choice) {
			out[a].err = fmt.Errorf("Jev chose %q, which is not a choice", choice)
			continue
		}
		res := intakeAnswer{event: choice, eventP: event.Probabilities[choice], model: distribution.Model,
			inputTokens:  int(math.Round(float64(distribution.Usage.InputTokens) * share)),
			outputTokens: int(math.Round(float64(distribution.Usage.OutputTokens) * share)),
			cost:         distribution.Usage.Cost * share}
		if ask.topicName != "" {
			if topic, ok := distribution.Answers[ask.topicName]; ok {
				if t := strings.TrimSpace(topic.Choice); t != "" && t != "none" && intakeHas(ask.storyChoices, t) {
					res.topic, res.topicP = t, topic.Probabilities[t]
				}
			}
		}
		out[a].answer = res
	}
	return out, nil
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intakeHas(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Intake candidate windows (docs/problems/unsupported-cites-duplicates-and-
// churn-2026-11-10.md, D2): the shortlist is the stories updated within a
// window of time, not the newest N, because a busy hour pushed a story out
// of a count in under an hour and its later report opened a duplicate (the
// pairs opened 40 minutes to 6.5 hours after the first). A window longer than
// one call should carry is asked in chunks, never truncated.
const (
	// intakeShortlistWindow is how far back a story is a candidate. The
	// duplicates read opened 40 minutes to 6.5 hours after their first.
	intakeShortlistWindow = 8 * time.Hour
	// intakeShortlistCeiling backstops the window: past this many stories
	// (six chunks; seven with the overlap picks) the oldest fall out.
	intakeShortlistCeiling = 480
	// intakeShortlistFloor is the newest stories that are candidates whatever
	// their age, so a quiet stretch never shrinks the list below the old
	// newest-30 reach.
	intakeShortlistFloor = 30
	// Entries in a chunk are a headline and a time (a lede only on the newest
	// intakeLedeCandidates), so a story costs about 100 bytes and a chunk
	// holds on the order of 100; see intakeChunkStateBytes.
	intakeLedeCandidates = 30
)

// intakeAnswer is one chunk call's answer: the event choice and the topic
// choice with their probabilities, and the call's usage.
type intakeAnswer struct {
	event, topic              string
	eventP, topicP            float64
	model                     string
	inputTokens, outputTokens int
	cost                      float64
}

// judgeCall is the one way a call site asks Jev: the call, timed and bounded
// by callJudgeWithin, and its record, success or failure, through
// jevrecord.Observe. meta names the site and the decision the answer feeds;
// meta.Scope is the gateway scope.
func (rt *Runtime) judgeCall(ctx context.Context, meta jevrecord.Meta, timeout time.Duration, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	switch meta.Site {
	case "system1_verb", "subcall_judge", "desk_cast_judge", "inert_judge":
		// Their question text is an agent's, so the record keeps its digest
		// only.
		meta.NoText = true
	}
	return jevrecord.Observe(rt.jevRecorder(), meta, state, questions, func() (json.RawMessage, error) {
		return callJudgeWithin(ctx, rt.candidateJudge, meta.Scope, timeout, state, questions)
	})
}

// jevRecorder is the runtime's record of Jev calls: a rotated log beside the
// store, not the tape (a judgment is observation, and at thousands an hour
// it would outweigh the tape). Nil, and a no-op, when the store has no path.
func (rt *Runtime) jevRecorder() *jevrecord.Recorder {
	rt.jevRecOnce.Do(func() {
		if rt.store == nil || strings.TrimSpace(rt.store.StatePath()) == "" {
			return
		}
		rt.jevRec = jevrecord.NewRecorder(filepath.Join(filepath.Dir(rt.store.StatePath()), "jev-calls", "calls.jsonl"))
	})
	return rt.jevRec
}

// callJudgeWithin calls Jev on a rate scope ("" for the judge's default
// bucket) within timeout. It returns at the deadline even when the judge
// ignores it.
func callJudgeWithin(ctx context.Context, judge CandidateJudge, scope string, timeout time.Duration, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type result struct {
		raw json.RawMessage
		err error
	}
	done := make(chan result, 1)
	go func() {
		var r result
		defer func() {
			// A judge that panics fails its call, never the process.
			if p := recover(); p != nil {
				r = result{err: fmt.Errorf("judge panicked: %v", p)}
				done <- r
			}
		}()
		if scoped, ok := judge.(ScopedCandidateJudge); ok && scope != "" {
			r.raw, r.err = scoped.JudgeScoped(callCtx, scope, state, questions)
		} else {
			r.raw, r.err = judge.Judge(callCtx, state, questions)
		}
		done <- r
	}()
	select {
	case r := <-done:
		return r.raw, r.err
	case <-callCtx.Done():
		return nil, fmt.Errorf("Jev: %w", callCtx.Err())
	}
}
