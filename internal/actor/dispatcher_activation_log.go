package actor

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// Per-activation observation (docs/problems/paper-dispatcher-twelve-slots-full-ninety-due-2026-11-20.md,
// "Next step" 1): how long each activation held its slot, how long the agent
// waited before it, and which updates it folded. Nothing here is read by a
// scheduling decision.
//
// Volume: an activation over activationLogOver logs its own line, so the tail
// is never sampled away; the rest are counted into one summary line every
// activationSummaryEvery, by agent kind and by update kind.
//
// Not logged: the time a handler spent blocked on the store. The dispatcher
// calls the handler as one opaque HandleUpdate and the store belongs to the
// handler's package, so the dispatcher cannot know it without a hook inside
// the store. The line does carry the handler's wall time per update kind and
// the actor log's own commit time, which separates a slow turn from a slow
// log.

// activationLogOver is the wall time above which an activation logs its own
// line. A var so a test can lower it; nothing sets it in production.
var activationLogOver = time.Second

// activationSummaryEvery is how often the summary of the faster activations
// is written.
const activationSummaryEvery = time.Minute

// handledUpdate is one update's handler wall time within an activation.
type handledUpdate struct {
	kind string
	dur  time.Duration
}

// activationRecord is what one activation did, filled in as it runs.
type activationRecord struct {
	agentID      string
	started      time.Time
	pending      []Update
	incorporated []string
	handled      []handledUpdate
	commit       time.Duration
	skipCause    string        // why the agent was passed over before this activation, or ""
	slotWait     time.Duration // since the first pass that skipped it, 0 when none did
}

// agentKindOf is the kind of an agent id: the scoped id's last segment up to
// the first ':' ("texture:doc-..." is "texture"). Never the rest of the id.
func agentKindOf(agentID string) string {
	name := actorLogName(agentID)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	if name == "" {
		return "unknown"
	}
	return name
}

// peekSkip reads, without consuming, why agentID was passed over and for how
// long as of now. The wait line consumes the note afterwards.
func (d *Dispatcher) peekSkip(agentID string, now time.Time) (string, time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	note, ok := d.skips[agentID]
	if !ok {
		return "", 0
	}
	return note.cause, now.Sub(note.since)
}

// finishActivation writes the activation's line (over activationLogOver) or
// counts it for the summary.
func (d *Dispatcher) finishActivation(rec activationRecord) {
	now := time.Now()
	dur := now.Sub(rec.started)
	if dur > activationLogOver {
		log.Print(activationDoneLine(rec, dur, now))
		return
	}
	d.stats.add(agentKindOf(rec.agentID), dur, rec.handled)
}

// activationDoneLine is the line of one completed activation. due_age is how
// long the oldest due update had waited when the activation started; slot_wait
// is how long the agent was passed over (cause) before it started.
func activationDoneLine(rec activationRecord, dur time.Duration, now time.Time) string {
	kinds := make(map[string]string, len(rec.pending))
	for _, u := range rec.pending {
		kinds[u.UpdateID] = u.Kind
	}
	counts := map[string]int{}
	for _, id := range rec.incorporated {
		counts[kinds[id]]++
	}
	var oldest time.Time
	for _, u := range rec.pending {
		due := u.CreatedAt
		if u.NotBefore.After(due) {
			due = u.NotBefore
		}
		if due.After(rec.started) {
			continue
		}
		if oldest.IsZero() || due.Before(oldest) {
			oldest = due
		}
	}
	var dueAge time.Duration
	if !oldest.IsZero() {
		dueAge = rec.started.Sub(oldest)
	}
	cause := rec.skipCause
	if cause == "" {
		cause = "none"
	}
	var slowest handledUpdate
	for _, h := range rec.handled {
		if h.dur > slowest.dur {
			slowest = h
		}
	}
	incorporated := countsString(counts)
	if incorporated == "" {
		incorporated = "none"
	}
	return fmt.Sprintf("dispatcher: activation done agent=%s kind=%s dur=%s slot_wait=%s slot_wait_cause=%s due_age=%s commit=%s updates=%d incorporated=%d by_kind=%s slowest=%s:%s",
		actorLogName(rec.agentID), agentKindOf(rec.agentID), dur.Round(time.Millisecond), rec.slotWait.Round(time.Millisecond), cause,
		dueAge.Round(time.Millisecond), rec.commit.Round(time.Millisecond), len(rec.pending), len(rec.incorporated), incorporated,
		orDash(slowest.kind), slowest.dur.Round(time.Millisecond))
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// countsString renders kind counts most first: "inbox_record:3,coagent_result:1".
func countsString(counts map[string]int) string {
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool {
		if counts[kinds[i]] != counts[kinds[j]] {
			return counts[kinds[i]] > counts[kinds[j]]
		}
		return kinds[i] < kinds[j]
	})
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s:%d", k, counts[k]))
	}
	return strings.Join(parts, ",")
}

// activationStats holds the durations of the activations too fast to log
// individually, until the next summary.
type activationStats struct {
	mu      sync.Mutex
	since   time.Time
	byAgent map[string][]time.Duration
	byKind  map[string][]time.Duration
}

func (s *activationStats) add(agentKind string, dur time.Duration, handled []handledUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.since.IsZero() {
		s.since = time.Now()
	}
	if s.byAgent == nil {
		s.byAgent = map[string][]time.Duration{}
		s.byKind = map[string][]time.Duration{}
	}
	s.byAgent[agentKind] = append(s.byAgent[agentKind], dur)
	for _, h := range handled {
		s.byKind[h.kind] = append(s.byKind[h.kind], h.dur)
	}
}

// flushIfDue writes the summary when every has passed since the first sample.
func (s *activationStats) flushIfDue(now time.Time, every time.Duration) {
	s.mu.Lock()
	if s.since.IsZero() || now.Sub(s.since) < every {
		s.mu.Unlock()
		return
	}
	line := s.takeLocked(now)
	s.mu.Unlock()
	log.Print(line)
}

// takeLocked renders and clears the samples. Callers hold s.mu.
func (s *activationStats) takeLocked(now time.Time) string {
	line := activationSummaryLine(now.Sub(s.since), s.byAgent, s.byKind)
	s.since, s.byAgent, s.byKind = time.Time{}, nil, nil
	return line
}

// activationSummaryLine: count, p50, p90 and max for each agent kind's
// activations and each update kind's handler time.
func activationSummaryLine(window time.Duration, byAgent, byKind map[string][]time.Duration) string {
	total := 0
	for _, v := range byAgent {
		total += len(v)
	}
	return fmt.Sprintf("dispatcher: activation summary window=%s activations=%d agent_kinds=[%s] update_kinds=[%s]",
		window.Round(time.Second), total, quantileGroups(byAgent), quantileGroups(byKind))
}

func quantileGroups(groups map[string][]time.Duration) string {
	names := make([]string, 0, len(groups))
	for k := range groups {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, k := range names {
		v := append([]time.Duration(nil), groups[k]...)
		sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
		parts = append(parts, fmt.Sprintf("%s n=%d p50=%s p90=%s max=%s", k, len(v),
			quantile(v, 0.5).Round(time.Millisecond), quantile(v, 0.9).Round(time.Millisecond), v[len(v)-1].Round(time.Millisecond)))
	}
	return strings.Join(parts, "; ")
}

// quantile is the nearest-rank quantile of sorted v (non-empty).
func quantile(v []time.Duration, q float64) time.Duration {
	i := int(q*float64(len(v))+0.999999) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(v) {
		i = len(v) - 1
	}
	return v[i]
}
