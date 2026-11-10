package gateway

import (
	"hash/fnv"
	"log"
	"strconv"
	"sync"
	"time"
)

// Devin hourly pacing (docs/evidence/devin-quota-capacity-2026-11-10.md,
// options (a) and (b)).
//
// Devin's free quota is one account-wide window per fixed clock hour. The
// gateway used to run every thread flat out until Devin answered 429, then
// send the whole load to the paid tail until the reset. The pacer instead
// counts the input tokens Devin has served this clock hour and, when the
// count runs ahead of an even spend, answers a deterministic share of
// THREADS with a quota refusal flagged Shed, so the guest moves those
// threads to the next pool member for the run. The rest keep Devin to the end
// of the hour. The share is chosen by thread hash, the idiom the pool's
// spread uses (internal/modelpolicy spreadGroups: FNV-1a over thread+"|..."),
// so a thread stays on one side of the line all hour and its prompt cache is
// not rebuilt.
//
// State is in memory only. A gateway restart relearns: the ceiling returns to
// devinCeilingStart and a trip in the hour the pacer first saw partway does
// not shrink it (the count is partial).

const (
	// devinCeilingStart is the starting ceiling in Devin input tokens per
	// clock hour. Source: the evidence doc's two-point fit. Hour 10 of
	// 2026-11-10 completed clean at 42.70M gateway input tokens; hour 11
	// tripped at 43.54M with the owner's local runs also on the account.
	// 40M sits under the clean hour, so the first hour does not trip.
	devinCeilingStart = 40_000_000
	// devinCeilingFloor stops one bad hour (the owner's local runs spent the
	// account; hours 07 and 08 tripped at 18.6M and 13.1M) from collapsing the
	// ceiling toward zero. 13.1M x 0.95 = 12.4M; the floor is below that.
	devinCeilingFloor = 10_000_000
	// devinCeilingCap bounds the slow raise at 1.5x the start.
	devinCeilingCap = 60_000_000
	// devinTripLearnFactor: on a trip, ceiling = min(ceiling, counted x this).
	devinTripLearnFactor = 0.95
	// The ceiling rises by 1/devinCeilingRaiseDiv (5%) after an hour that
	// ended without a trip and either made the pacer shed (demand above the
	// paced rate: the ceiling is the limit) or spent at least
	// devinRaiseUsage of the budget. The pacer holds spend under the budget,
	// so a threshold on the ceiling itself could never be met. A wrong low
	// ceiling costs paid calls every hour; a wrong high one costs one trip,
	// which then teaches the ceiling.
	devinCeilingRaiseDiv = 20
	devinRaiseUsage      = 0.90
	// A quota 429 teaches the ceiling only when the gateway's own count is at
	// least this share of the ceiling (below it the owner's local use, not
	// the gateway, spent the account), and not in the first devinTripSettle
	// of an hour (a lagging reset or a call in flight across the boundary).
	devinTripMinShare = 0.5
	devinTripSettle   = 2 * time.Minute
	// devinReserveFraction is the share of the ceiling the gateway never
	// spends: the owner's local CLI reviews use the same account (option b).
	devinReserveFraction = 0.10
	// The shed share is a feedback controller on what can be observed: the
	// served Devin tokens per minute over the trailing devinRateWindow against
	// the budget left per minute left in the hour. Evaluated at most every
	// devinShedEvalEvery, on admit: over the allowed rate, the share rises by
	// devinShedStep (to at most devinShedMax); under devinShedLowFraction of it,
	// the share falls by devinShedStep for every devinShedFallEvery since it
	// last moved (so a stale share decays in one admit after a gap). At the
	// budget it is 1 (the reserve). Shed sets are nested by hash.
	devinShedEvalEvery   = 30 * time.Second
	devinShedFallEvery   = 60 * time.Second
	devinRateWindow      = 5 * time.Minute
	devinShedStep        = 0.10
	devinShedMax         = 0.95
	devinShedLowFraction = 0.7
	// devinCarryShare is the part of an hour's final share the next
	// contiguous hour starts with, and only when that hour finished at least
	// devinCarryMinUse of its budget (demand that did not use the budget is
	// not demand to shed).
	devinCarryShare  = 0.5
	devinCarryMinUse = 0.8
	// devinShedRetry is the RetryAt a shed answer carries. Guests that do not
	// read the Shed flag treat it as a reset time and hold Devin until then;
	// it is long enough to survive guest clock skew (a reset not after the
	// guest's now is read as unknown and held two minutes) and short enough
	// to cost little when it is held.
	devinShedRetry = 10 * time.Second
)

// hourPacer counts Devin input tokens in the current clock hour and decides
// which threads Devin may serve.
type hourPacer struct {
	now  func() time.Time
	logf func(format string, args ...any)

	mu       sync.Mutex
	since    time.Time // when the pacer first saw time; before it, an hour is partial
	hour     time.Time // start of the current clock hour (UTC)
	tokens   int64
	unshed   int64 // tokens of calls that cannot be shed: no thread key, or NoShed
	shedEver bool  // the share was above zero at some point this hour
	admitted int64
	shed     int64
	trips    int
	ceiling  int64
	share    float64
	lastEval time.Time   // the controller's last evaluation
	lastMove time.Time   // when the share last changed (or the hour began)
	lastHold time.Time   // the last evaluation that did not find the rate low
	events   []paceEvent // served tokens, trailing devinRateWindow
	prev     closedHour  // the hour before this one, for a late 429
}

// paceEvent is one served call's tokens.
type paceEvent struct {
	at     time.Time
	tokens int64
}

// closedHour is what a late 429 for a call dispatched in the previous hour
// needs to learn from that hour's count.
type closedHour struct {
	hour     time.Time
	tokens   int64
	complete bool
}

func newHourPacer(now func() time.Time, logf func(string, ...any)) *hourPacer {
	if logf == nil {
		logf = log.Printf
	}
	t := now()
	return &hourPacer{now: now, logf: logf, since: t, hour: t.UTC().Truncate(time.Hour), lastMove: t.UTC().Truncate(time.Hour), lastHold: t.UTC().Truncate(time.Hour), ceiling: devinCeilingStart}
}

type pacerSnapshot struct {
	tokens, ceiling int64
	share           float64
	trips           int
}

func (p *hourPacer) snapshot() pacerSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rollLocked(p.now())
	return pacerSnapshot{tokens: p.tokens, ceiling: p.ceiling, share: p.share, trips: p.trips}
}

func (p *hourPacer) budgetLocked() float64 {
	return float64(p.ceiling) * (1 - devinReserveFraction)
}

// rollLocked moves to the clock hour of now, closing the one that ended:
// one log line, and the slow raise of the ceiling.
func (p *hourPacer) rollLocked(now time.Time) {
	hs := now.UTC().Truncate(time.Hour)
	if !hs.After(p.hour) {
		return
	}
	complete := !p.since.After(p.hour)
	raised := ""
	if complete && p.trips == 0 && (p.shedEver || float64(p.tokens) >= devinRaiseUsage*p.budgetLocked()) && p.ceiling < devinCeilingCap {
		next := p.ceiling + p.ceiling/devinCeilingRaiseDiv
		if next > devinCeilingCap {
			next = devinCeilingCap
		}
		raised = formatCeilingChange(p.ceiling, next, "clean hour: paced, or spent >=90% of the budget")
		p.ceiling = next
	}
	total := p.admitted + p.shed
	p.logf("gateway: devin pacing hour=%s tokens=%d ceiling=%d budget=%d shed_share=%.0f%% shed_calls=%d of %d trips=%d complete=%t unsheddable_tokens=%d",
		p.hour.Format("2006-01-02T15:04Z"), p.tokens, p.ceiling, int64(p.budgetLocked()), p.share*100, p.shed, total, p.trips, complete, p.unshed)
	if raised != "" {
		p.logf("%s", raised)
	}
	carry := 0.0
	if hs.Sub(p.hour) == time.Hour && float64(p.tokens) >= devinCarryMinUse*p.budgetLocked() {
		carry = p.share * devinCarryShare
	}
	p.prev = closedHour{hour: p.hour, tokens: p.tokens, complete: complete}
	p.events = nil
	p.hour, p.tokens, p.unshed, p.admitted, p.shed, p.trips, p.share, p.lastEval, p.lastMove, p.lastHold = hs, 0, 0, 0, 0, 0, carry, time.Time{}, hs, hs
	p.shedEver = carry > 0
}

func formatCeilingChange(from, to int64, why string) string {
	return "gateway: devin pacing ceiling " + itoa64(from) + " -> " + itoa64(to) + " (" + why + ")"
}

// record adds a finished call's input tokens to this hour's count. The usage
// is known only when the call returns (a streamed call at its end), so
// in-flight calls are not in the count; the burst allowance and the reserve
// absorb them. A call that failed returns no usage and counts nothing.
func (p *hourPacer) record(inputTokens int, sheddable bool) {
	if inputTokens <= 0 {
		return
	}
	p.mu.Lock()
	p.rollLocked(p.now())
	now := p.now()
	p.tokens += int64(inputTokens)
	p.events = append(p.events, paceEvent{at: now, tokens: int64(inputTokens)})
	if !sheddable {
		p.unshed += int64(inputTokens)
	}
	p.mu.Unlock()
}

// trip learns from a quota 429 for a call dispatched at dispatched:
// ceiling = min(ceiling, counted x 0.95). The count is the one of the hour
// the call was dispatched in (a late 429 for a call from the previous hour
// is that hour's, not the new hour's near-zero count). Nothing is learned
// when the gateway's count was under half the ceiling (the owner's local use
// spent the account), in an hour the pacer saw only part of, or for a call
// dispatched in the first two minutes of its hour.
func (p *hourPacer) trip(dispatched time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rollLocked(p.now())
	dh := dispatched.UTC().Truncate(time.Hour)
	var tokens int64
	var complete bool
	switch {
	case dh.Equal(p.hour):
		if dispatched.Sub(p.hour) < devinTripSettle || p.now().Sub(p.hour) < devinTripSettle {
			p.logf("gateway: devin pacing trip in the first 2 minutes of the hour (counted %d); attributed to nothing, ceiling stays %d", p.tokens, p.ceiling)
			return
		}
		p.trips++
		tokens, complete = p.tokens, !p.since.After(p.hour)
	case dh.Equal(p.prev.hour):
		tokens, complete = p.prev.tokens, p.prev.complete
	default:
		p.logf("gateway: devin pacing trip not learned: its call was dispatched in %s, no longer kept", dh.Format("15:04Z"))
		return
	}
	if !complete {
		p.logf("gateway: devin pacing trip in a partly observed hour (counted %d); ceiling stays %d", tokens, p.ceiling)
		return
	}
	if float64(tokens) < devinTripMinShare*float64(p.ceiling) {
		p.logf("gateway: devin pacing trip not learned: count too low to be the gateway's doing (counted %d of ceiling %d)", tokens, p.ceiling)
		return
	}
	learned := int64(float64(tokens) * devinTripLearnFactor)
	if learned < devinCeilingFloor {
		learned = devinCeilingFloor
	}
	if learned < p.ceiling {
		p.logf("%s", formatCeilingChange(p.ceiling, learned, "quota 429 after "+itoa64(tokens)+" counted"))
		p.ceiling = learned
	}
}

// admit reports whether the thread may use Devin now: it is shed when its
// hash falls below the hour's share. The share is steered by feedback (see
// devinShedEvalEvery): it cannot stick, because if every sheddable call is
// shed the served rate falls to the unsheddable rate, which is under the
// allowed rate (the share falls) or over it (nothing more can be done).
func (p *hourPacer) admit(thread string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	p.rollLocked(now)
	budget := p.budgetLocked()
	if float64(p.tokens) >= budget {
		p.share = 1 // the reserve: what is left belongs to the local CLI
	} else if p.lastEval.IsZero() || now.Sub(p.lastEval) >= devinShedEvalEvery {
		p.lastEval = now
		elapsed := now.Sub(p.hour)
		// The rate is measured since the share last moved (at most the
		// trailing window): older traffic was served under another share and
		// would wind the controller up. Under a minute of data it waits.
		start := now.Add(-devinRateWindow)
		if p.lastMove.After(start) {
			start = p.lastMove
		}
		if now.Sub(start) < devinShedFallEvery {
			return !p.shedLocked(thread)
		}
		cut, kept, served := start, p.events[:0], int64(0)
		for _, e := range p.events {
			if e.at.After(cut) {
				kept, served = append(kept, e), served+e.tokens
			}
		}
		p.events = kept
		rate := float64(served) / now.Sub(start).Minutes()
		allowed := (budget - float64(p.tokens)) / max(1, (time.Hour-elapsed).Minutes())
		switch {
		case rate > allowed:
			if p.share < devinShedMax {
				p.share = min(devinShedMax, p.share+devinShedStep)
			}
			p.lastMove, p.lastHold = now, now
		case rate < devinShedLowFraction*allowed:
			// One step per 60 s the rate has been low, so a quiet gap (no
			// admits, hence no evaluations) decays a stale share at once,
			// while one low reading in a steady hour costs one step.
			since := p.lastMove
			if p.lastHold.After(since) {
				since = p.lastHold
			}
			if n := int(now.Sub(since) / devinShedFallEvery); n >= 1 {
				p.share, p.lastMove, p.lastHold = max(0, p.share-float64(n)*devinShedStep), now, now
			}
		default:
			p.lastHold = now
		}
	}
	return !p.shedLocked(thread)
}

// shedLocked counts the decision for thread under the current share.
func (p *hourPacer) shedLocked(thread string) bool {
	if p.share > 0 {
		p.shedEver = true
	}
	shed := p.share > 0 && float64(threadHash(thread))/float64(1<<32) < p.share
	if shed {
		p.shed++
	} else {
		p.admitted++
	}
	return shed
}

// threadHash is FNV-1a over the thread and a pacing salt, the idiom
// internal/modelpolicy uses for its spread (fnv32(thread+"|spread")), with
// its own salt so the shed set is independent of the spread order.
func threadHash(thread string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(thread + "|pace"))
	return h.Sum32()
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
