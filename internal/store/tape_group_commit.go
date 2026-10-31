package store

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Tape group commit (docs/design/tape-group-commit-2026-10-31.md). With the
// computer setting tape.group_commit on, acknowledged projection writes whose
// ops are all unconditional upserts share one projection_batch_recorded tape
// event with the writes that arrived while an earlier one was being
// committed, and with the telemetry the sealer still holds. Every member
// returns only after the shared event's CAS and local finalize: no
// acknowledged write is answered before it is durable.
//
// Leader and followers, no timer: a lone writer commits at once, as before.
// Lock order is flushMu (the sealer's) -> appender mutex, the same order the
// sealer's own flush uses; the leader appends with the barrier exemption
// because it has itself done what the barrier does.

const (
	// groupMaxOps and groupMaxBytes cap the members' part of one group; one
	// write over the cap goes alone.
	groupMaxOps   = 256
	groupMaxBytes = 1 << 20
	// groupCommitTimeout bounds one shared append, like the sealer's.
	groupCommitTimeout = 30 * time.Second
	// telemetryGroupWindow is the sealer's window while group commit is on:
	// telemetry rides the next acknowledged write, so the window only matters
	// when no acknowledged write comes for that long.
	telemetryGroupWindow = 10 * time.Second
)

// groupCommit is the per-store group state. The zero value is off.
type groupCommit struct {
	on atomic.Bool

	mu      sync.Mutex // leaf: protects pending
	pending []*groupMember

	token chan struct{} // one leader at a time; created lazily
	once  sync.Once

	groups          atomic.Int64
	members         atomic.Int64
	ops             atomic.Int64
	membersHist     [opsHistogramBuckets]atomic.Int64
	telemetryRidden atomic.Int64
	refusedSplits   atomic.Int64
	offSealFailures atomic.Int64
	unknownOutcome  atomic.Int64
	cancelled       atomic.Int64
	waitNanosTotal  atomic.Int64
	waitNanosMax    atomic.Int64
}

type groupMember struct {
	ops   []computerevent.ProjectionOp
	bytes int
	start time.Time
	taken bool // under groupCommit.mu
	err   error
	done  chan struct{}
}

func (g *groupCommit) leaderToken() chan struct{} {
	g.once.Do(func() { g.token = make(chan struct{}, 1) })
	return g.token
}

// SetTapeGroupCommit turns group commit on or off at once. Off sends every
// later write down the direct path; a group already being committed finishes.
func (s *Store) SetTapeGroupCommit(on bool) {
	if s == nil {
		return
	}
	if s.tapeGroup.on.Swap(on) != on {
		// The run-memory tail cache is kept only while on; a stretch off
		// wrote entries it did not see (run_memory_batch.go).
		s.runMem.mu.Lock()
		s.runMem.tails = nil
		s.runMem.mu.Unlock()
		if !on {
			// Off must stop the lossy window now: seal what is queued (the
			// records keep their 10 s deadline otherwise).
			if sl := s.telemetry.Load(); sl != nil {
				before := s.telemetryStats.recordsDropped.Load()
				sl.barrier(sealBarrier)
				if lost := s.telemetryStats.recordsDropped.Load() - before; lost > 0 {
					s.tapeGroup.offSealFailures.Add(1)
					log.Printf("store: tape.group_commit off: sealing the queue lost %d record(s); their runs are marked damaged", lost)
				}
				sl.mu.Lock()
				if len(sl.queue) > 0 && !sl.stopped {
					sl.armTimerLocked()
				}
				sl.mu.Unlock()
			}
		}
	}
}

// TapeGroupCommitOn reports whether acknowledged writes are being grouped.
func (s *Store) TapeGroupCommitOn() bool { return s != nil && s.tapeGroup.on.Load() }

// groupableOps reports whether every op is an unconditional upsert by key:
// only those can share an event without one member's guard failing another
// (design F4). Anything else, and any op kind added later, goes direct.
func groupableOps(ops []computerevent.ProjectionOp) bool {
	if len(ops) == 0 {
		return false
	}
	for _, op := range ops {
		switch op.Kind {
		case computerevent.ProjectionOpObject, computerevent.ProjectionOpObjectEdge, computerevent.ProjectionOpRunMemoryEntry:
		default:
			return false
		}
	}
	return true
}

func opsBytes(ops []computerevent.ProjectionOp) int {
	n := 0
	for _, op := range ops {
		n += len(op.Body) + len(op.CanonicalID) + 64
	}
	return n
}

// appendGrouped writes ops as one member of a group and returns that group's
// outcome for this member.
func (t *projectionTape) appendGrouped(ctx context.Context, ops []computerevent.ProjectionOp) error {
	g := t.group
	m := &groupMember{ops: ops, bytes: opsBytes(ops), start: time.Now(), done: make(chan struct{})}
	g.mu.Lock()
	g.pending = append(g.pending, m)
	g.mu.Unlock()
	token := g.leaderToken()
	for {
		select {
		case <-m.done:
			return m.err
		case token <- struct{}{}:
			t.leadGroup()
			<-token
			select {
			case <-m.done:
				return m.err
			default:
				// Not taken (the caps cut the group short): lead again.
			}
		case <-ctx.Done():
			g.mu.Lock()
			if !m.taken {
				for i, p := range g.pending {
					if p == m {
						g.pending = append(g.pending[:i:i], g.pending[i+1:]...)
						break
					}
				}
				g.mu.Unlock()
				g.cancelled.Add(1)
				return ctx.Err() // nothing of it was written
			}
			g.mu.Unlock()
			<-m.done // taken: its outcome is the group's
			return m.err
		}
	}
}

// takeGroup removes the next group from the pending list, oldest first, up to
// the caps. The first member is always taken, whatever its size.
func (g *groupCommit) takeGroup() []*groupMember {
	g.mu.Lock()
	defer g.mu.Unlock()
	n, ops, bytes := 0, 0, 0
	for n < len(g.pending) {
		m := g.pending[n]
		if n > 0 && (ops+len(m.ops) > groupMaxOps || bytes+m.bytes > groupMaxBytes) {
			break
		}
		ops += len(m.ops)
		bytes += m.bytes
		m.taken = true
		n++
	}
	taken := append([]*groupMember(nil), g.pending[:n]...)
	g.pending = append(g.pending[:0:0], g.pending[n:]...)
	return taken
}

// leadGroup commits one group. It is called holding the leader token.
func (t *projectionTape) leadGroup() {
	g := t.group
	members := g.takeGroup()
	if len(members) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(computerevent.WithBarrierExemption(context.Background()), groupCommitTimeout)
	defer cancel()

	var memberOps []computerevent.ProjectionOp
	for _, m := range members {
		memberOps = append(memberOps, m.ops...)
	}
	commit := func(teleOps []computerevent.ProjectionOp) error {
		all := make([]computerevent.ProjectionOp, 0, len(teleOps)+len(memberOps))
		all = append(all, teleOps...)
		all = append(all, memberOps...)
		return t.appendDirect(ctx, all, projectionAuthority)
	}
	var err error
	ridden := 0
	if sl := t.telemetryQueue(); sl != nil {
		ridden, err = sl.sealRiding(commit)
	} else {
		err = commit(nil)
	}

	switch {
	case err == nil:
		g.finishMembers(members, nil)
	case errors.Is(err, computerevent.ErrBatchRefused):
		// Nothing was pinned. The riding telemetry was sealed on its own by
		// sealRiding; each member is now written alone, in arrival order,
		// and gets its own answer (F2).
		g.refusedSplits.Add(1)
		for _, m := range members {
			m.err = t.appendDirect(ctx, m.ops, projectionAuthority)
			g.finishMember(m)
		}
	default:
		// Outcome unknown: never retried (F3).
		g.unknownOutcome.Add(1)
		g.finishMembers(members, err)
	}
	g.groups.Add(1)
	g.members.Add(int64(len(members)))
	g.ops.Add(int64(len(memberOps)))
	g.membersHist[opsBucket(len(members))].Add(1)
	g.telemetryRidden.Add(int64(ridden))
}

func (g *groupCommit) finishMembers(members []*groupMember, err error) {
	for _, m := range members {
		m.err = err
		g.finishMember(m)
	}
}

func (g *groupCommit) finishMember(m *groupMember) {
	g.recordWait(time.Since(m.start))
	close(m.done)
}

func (g *groupCommit) recordWait(d time.Duration) {
	g.waitNanosTotal.Add(int64(d))
	for {
		cur := g.waitNanosMax.Load()
		if int64(d) <= cur || g.waitNanosMax.CompareAndSwap(cur, int64(d)) {
			return
		}
	}
}

// sealRiding is the sealer's side of a group: holding flushMu, it seals in
// order every queued chunk but the newest, then hands the newest (at most one
// batch's caps) to commit, which writes it ahead of the members' ops in one
// ordinary event. It returns how many records rode. On a refusal the riding
// records are sealed on their own before it returns, so the members' singles
// that follow still land after them (F2, F5).
func (sl *telemetrySealer) sealRiding(commit func(teleOps []computerevent.ProjectionOp) error) (int, error) {
	sl.flushMu.Lock()
	defer sl.flushMu.Unlock()
	var ride []telemetryRecord
	for {
		sl.mu.Lock()
		n, bytes := 0, 0
		for n < len(sl.queue) && n < telemetryMaxBatchOps {
			if n > 0 && bytes+sl.queue[n].size > telemetryMaxBatchBytes {
				break
			}
			bytes += sl.queue[n].size
			n++
		}
		chunk := append([]telemetryRecord(nil), sl.queue[:n]...)
		sl.queue = append([]telemetryRecord(nil), sl.queue[n:]...)
		sl.queueBytes -= bytes
		sl.inflightRecs = chunk
		last := len(sl.queue) == 0
		if last {
			sl.stopTimerLocked()
		}
		sl.mu.Unlock()
		if last {
			ride = chunk
			break
		}
		sl.seal(chunk, sealBarrier)
		sl.finishInflight(chunk)
	}
	defer sl.finishInflight(ride)

	teleOps := make([]computerevent.ProjectionOp, 0, len(ride))
	for _, rec := range ride {
		if err := recheckTelemetryOp(rec.op); err != nil {
			// A class violation never rides: the sealer's own path writes the
			// chunk as acknowledged singles and counts it.
			sl.seal(ride, sealBarrier)
			ride, teleOps = nil, nil
			break
		}
		teleOps = append(teleOps, rec.op)
	}
	started := time.Now()
	err := commit(teleOps)
	switch {
	case len(ride) == 0:
	case err == nil:
		sl.sealed(len(ride), sealRide, started)
	case errors.Is(err, computerevent.ErrBatchRefused):
		sl.seal(ride, sealBarrier)
	default:
		stats := &sl.s.telemetryStats
		stats.sealFailures.Add(1)
		stats.unknownOutcomeLoss.Add(int64(len(ride)))
		stats.recordsDropped.Add(int64(len(ride)))
		sl.noteDropped(ride...)
		sl.logFailure("riding seal failed, outcome unknown, not retried", err)
	}
	return len(ride), err
}

// finishInflight clears the in-flight chunk and advances sealedThrough past
// it, as flushThrough does after each seal.
func (sl *telemetrySealer) finishInflight(chunk []telemetryRecord) {
	sl.mu.Lock()
	sl.inflightRecs = nil
	if len(chunk) > 0 && chunk[len(chunk)-1].n > sl.sealedThrough {
		sl.sealedThrough = chunk[len(chunk)-1].n
	}
	sl.mu.Unlock()
}

// GroupCommitStats is the group-commit part of the observability snapshot.
type GroupCommitStats struct {
	Enabled         bool             `json:"enabled"`
	Groups          int64            `json:"groups"`
	Members         int64            `json:"members"`
	Ops             int64            `json:"ops"`
	MembersPerGroup map[string]int64 `json:"members_per_group"`
	MeanMembers     float64          `json:"mean_members_per_group"`
	TelemetryRidden int64            `json:"telemetry_ridden"`
	RefusedSplits   int64            `json:"refused_splits"`
	OffSealFailures int64            `json:"off_seal_failures"`
	UnknownOutcome  int64            `json:"unknown_outcome"`
	Cancelled       int64            `json:"cancelled"`
	WaitMeanMS      float64          `json:"wait_ms_mean"`
	WaitMaxMS       float64          `json:"wait_ms_max"`
}

func (s *Store) groupCommitStats() GroupCommitStats {
	g := &s.tapeGroup
	out := GroupCommitStats{
		Enabled:         g.on.Load(),
		Groups:          g.groups.Load(),
		Members:         g.members.Load(),
		Ops:             g.ops.Load(),
		MembersPerGroup: map[string]int64{},
		TelemetryRidden: g.telemetryRidden.Load(),
		RefusedSplits:   g.refusedSplits.Load(),
		OffSealFailures: g.offSealFailures.Load(),
		UnknownOutcome:  g.unknownOutcome.Load(),
		Cancelled:       g.cancelled.Load(),
		WaitMaxMS:       float64(g.waitNanosMax.Load()) / 1e6,
	}
	for i := range g.membersHist {
		if n := g.membersHist[i].Load(); n > 0 {
			out.MembersPerGroup[opsBucketLabel(i)] = n
		}
	}
	if out.Groups > 0 {
		out.MeanMembers = float64(out.Members) / float64(out.Groups)
	}
	if out.Members > 0 {
		out.WaitMeanMS = float64(g.waitNanosTotal.Load()) / float64(out.Members) / 1e6
	}
	return out
}
