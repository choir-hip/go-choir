package store

// The stream_seq lease: the highest stream_seq this computer may hand out,
// written ahead on the tape in blocks
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, round 3).
//
// The allocator never gives out a number above a lease that has passed the
// head CAS, so on every path except migration the lease is at or above every
// number published: stored, queued for the sealer, or put on the bus by an
// append that failed. A trusted lease seeds a restart with one primary-key
// read and a bounded cross-check; anything else takes the exact seed, a full
// paged maximum over the event history plus an unconditional gap, once.
//
// The lease is one og_objects object (choir.stream_seq_lease, external-key
// identity) written through the projection interceptor, so the tape is the
// single authority and the local row is its projection, which replay
// rebuilds. There is no new table, column or index.
//
// Lock order: flushMu -> appender mu -> eventMu (-> the engine mutex inside
// SQL). eventMu is never held across an append: the seeder and the refill run
// on their own goroutines, with eventMu released, and take it only to publish
// their result. A waiting allocator releases eventMu while it waits.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The constants table of the design (section 8). Variables so a test can
// shrink a block of 65,536 to eight; production never assigns them.
var (
	// streamSeqLeaseBlock is N: the numbers one lease write reserves.
	streamSeqLeaseBlock int64 = 65536
	// streamSeqExactGap is G: added unconditionally to an exact seed, over the
	// numbers a lease-unaware process published and never stored.
	streamSeqExactGap int64 = 1 << 20
	// streamSeqLeaseCeiling is C: a self-consistent lease above the
	// cross-check by more than C is forged or corrupt; it caps what a row
	// behind the writer fence could burn.
	streamSeqLeaseCeiling int64 = 1 << 40
	// leaseReadyDeadline bounds an allocation's wait for a refill: the
	// sealer's seal timeout.
	leaseReadyDeadline = 30 * time.Second
	// leaseSeedingDeadlineFloor and leaseSeedingMeasured set the seeding
	// deadline: the larger of the floor and three times the measured exact
	// seed at paper scale (section 2).
	leaseSeedingDeadlineFloor = 10 * time.Minute
	leaseSeedingMeasured      = leaseExactSeedMeasured
	// leaseRefillBackoff is the least wait between attempts after a failed
	// seed or refill.
	leaseRefillBackoff = time.Second
	// leaseScanPage is the exact seed's page size.
	leaseScanPage = 5000
	// leaseSeedBackoffCap bounds the doubling of the seeder's retry backoff.
	leaseSeedBackoffCap = 30 * time.Second
)

const (
	// leaseExactSeedMeasured is the measured time of the exact seed (test 37,
	// recorded in the design record, section "Build record"): 665 ms for
	// 150,000 events warm on an M1, about 4.4 us an event. The paper's real
	// figure is unmeasured until its first refresh reports seed_ms; the
	// seeding deadline is the larger of 10 minutes and three times this.
	leaseExactSeedMeasured = 665 * time.Millisecond
	// leaseSeedingFactor multiplies the measured seed into the deadline.
	leaseSeedingFactor = 3
	// leaseOwnerID and leaseIdentityKey name the singleton lease object.
	leaseOwnerID     = "__system__"
	leaseIdentityKey = "stream_seq_lease"
	// leaseWriterEpoch is the epoch a binary that writes the row stamps; a
	// row below leaseMinWriterEpoch is not trusted.
	leaseWriterEpoch    uint64 = 3
	leaseMinWriterEpoch uint64 = 3
	// leaseMaxBlock bounds a row's own block field, so a forged block cannot
	// overflow the consistency arithmetic.
	leaseMaxBlock int64 = 1 << 32
	// leaseBasisExact and leaseBasisLease are the two ways a process seeds.
	leaseBasisExact = "exact"
	leaseBasisLease = "lease"
)

// leaseFixedTime is the lease row's created_at and updated_at: a pure function
// of the row's fields, so a base and a replay agree, and old enough to stay out
// of the newest-objects windows.
var leaseFixedTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// leaseSeedingDeadline is how long a first allocation may wait for the seed.
func leaseSeedingDeadline() time.Duration {
	return max(leaseSeedingDeadlineFloor, leaseSeedingFactor*leaseSeedingMeasured)
}

// leaseRow is the lease object's body and metadata (identical JSON).
type leaseRow struct {
	// Reserved is R: the highest stream_seq the computer may hand out.
	Reserved int64 `json:"reserved_through"`
	// Allocated is A: the writer's maxStreamSeq when it wrote the row.
	Allocated int64 `json:"allocated_through"`
	// Block is N: the block size the writer used.
	Block int64 `json:"block"`
	// WriterEpoch is the store schema epoch of the writing binary.
	WriterEpoch uint64 `json:"writer_epoch"`
	// Basis says how the writing process seeded: "exact" or "lease".
	Basis string `json:"basis"`
	// HeadSequence is the local projection head sequence at write
	// (diagnostic).
	HeadSequence uint64 `json:"head_sequence"`
}

// consistent reports whether the row checks against its own lineage: the
// epoch, A <= R <= A + 3N/2. A refill writes R = max(reserved, A) + N with
// reserved - A < N/2 at its trigger, so R - A < 3N/2; a seeder writes
// R = A + N.
func (r leaseRow) consistent() bool {
	if r.WriterEpoch < leaseMinWriterEpoch || r.Block <= 0 || r.Block > leaseMaxBlock {
		return false
	}
	if r.Basis != leaseBasisExact && r.Basis != leaseBasisLease {
		return false
	}
	return r.Allocated >= 0 && r.Allocated <= r.Reserved && r.Reserved <= r.Allocated+3*r.Block/2
}

// leaseState is the allocator's phase.
type leaseState int

const (
	leaseUnseeded leaseState = iota
	leaseSeeding
	leaseReady
)

func (l leaseState) String() string {
	switch l {
	case leaseSeeding:
		return "seeding"
	case leaseReady:
		return "ready"
	default:
		return "unseeded"
	}
}

// leaseAllocator is the lease state of a Store. Every field is under
// Store.eventMu, and Reopen leaves them alone.
type leaseAllocator struct {
	state          leaseState
	reserved       int64 // the durable R this process knows
	refillInFlight bool
	changed        chan struct{} // closed and replaced on every change
	refillFailed   int64
	// seedFailed counts failed seed attempts since the process started;
	// seedStreak the consecutive ones, which set the retry backoff.
	seedFailed int64
	seedStreak int64
	tombstoned int64 // tombstoned lease rows met by a seeder
	// undecodable is the number of event rows the last exact seed could not
	// read a stream_seq from (metadata without one and a body that is not an
	// event record); lastUndecodable is the scan's running figure.
	undecodable     int64
	lastUndecodable atomic.Int64
	// workspace is the Dolt workspace directory the lease reads its marker
	// from, snapshotted under eventMu: Open sets it, Reopen (after Close
	// drained the lease work) refreshes it.
	workspace string
	// closingCh is closed, without a lock, when Close begins: every parked
	// waiter selects on it. Guarded by ctxMu.
	closingCh     chan struct{}
	lastErr       string
	seedBasis     string
	untrustReason string
	trusted       bool
	seedMS        int64
	crossCheckMax int64
	// allocatedThrough is the A of the row this process last wrote.
	allocatedThrough int64
	// onTape is whether the last lease row this process wrote went through a
	// bound projection tape (a row written before any tape was bound is an
	// og row only, which replay does not reproduce). Written under eventMu;
	// atomic so a reader that must not wait on the allocator (the event-table
	// gate's status) reads it lock-free.
	onTape          atomic.Bool
	seedNotBefore   time.Time
	refillNotBefore time.Time
	// ctx is the lifetime of the lease goroutines; Close cancels it and waits
	// for workers to drain. closing refuses new work until Reopen (or a failed
	// Close) resumes.
	ctxMu   sync.Mutex // guards ctx and cancel; never held with eventMu waiting
	ctx     context.Context
	cancel  context.CancelFunc
	workers atomic.Int32
	closing atomic.Bool
	obs     atomic.Pointer[StreamSeqLeaseStats]
}

// StreamSeqLeaseStats is the lease block of GET /api/runtime/observability.
type StreamSeqLeaseStats struct {
	State            string `json:"state"`
	ReservedThrough  int64  `json:"reserved_through"`
	AllocatedThrough int64  `json:"allocated_through"`
	Trusted          bool   `json:"trusted"`
	// UntrustReason is no_lease, receipt, marker, inconsistent, ceiling or
	// violated; empty when the lease was trusted.
	UntrustReason string `json:"untrust_reason"`
	// SeededFrom is exact or lease; empty before the first seed.
	SeededFrom     string `json:"seeded_from"`
	SeedMS         int64  `json:"seed_ms"`
	CrossCheckMax  int64  `json:"cross_check_max"`
	RefillFailed   int64  `json:"refill_failed"`
	LastLeaseError string `json:"last_lease_error"`
	MarkerPresent  bool   `json:"marker_present"`
	// SeedFailed counts failed seed attempts (each is a rescan of the event
	// history when the seed is exact); LeaseTombstoned counts tombstoned
	// lease rows the seeder met (a tombstone untrusts the lease).
	SeedFailed      int64 `json:"seed_failed"`
	LeaseTombstoned int64 `json:"lease_tombstoned"`
	// SeedUndecodableRows counts event rows the exact seed could not read a
	// stream_seq from, skipped and logged (a desk-forged or corrupt row).
	SeedUndecodableRows int64 `json:"seed_undecodable_rows"`
	// OnTape: the last lease row this process wrote went through a bound
	// projection tape. The event-table gate needs it with state ready.
	OnTape bool `json:"on_tape"`
}

// StreamSeqLeaseStats reports the lease without waiting on the allocator: the
// snapshot is published on every state change (and once at Open), marker
// included.
func (s *Store) StreamSeqLeaseStats() StreamSeqLeaseStats {
	st := StreamSeqLeaseStats{State: leaseUnseeded.String()}
	if s == nil {
		return st
	}
	if snap := s.lease.obs.Load(); snap != nil {
		st = *snap
	}
	return st
}

func (s *Store) publishLeaseObsLocked() {
	// marker_present is read here, with the state it describes, so the
	// snapshot agrees with itself. Presence that cannot be decided reads as
	// present, the answer that closes.
	markerPresent := false
	if s.lease.workspace != "" {
		markerPresent, _ = storeschema.LeaseUntrusted(s.lease.workspace)
	}
	s.lease.obs.Store(&StreamSeqLeaseStats{
		State:            s.lease.state.String(),
		ReservedThrough:  s.lease.reserved,
		AllocatedThrough: s.lease.allocatedThrough,
		Trusted:          s.lease.trusted,
		UntrustReason:    s.lease.untrustReason,
		SeededFrom:       s.lease.seedBasis,
		SeedMS:           s.lease.seedMS,
		CrossCheckMax:    s.lease.crossCheckMax,
		RefillFailed:     s.lease.refillFailed,
		LastLeaseError:   s.lease.lastErr,
		SeedFailed:       s.lease.seedFailed,
		LeaseTombstoned:  s.lease.tombstoned,

		SeedUndecodableRows: s.lease.undecodable,
		MarkerPresent:       markerPresent,
	})
}

// leaseChangedLocked returns the channel that closes on the next state change.
func (s *Store) leaseChangedLocked() chan struct{} {
	if s.lease.changed == nil {
		s.lease.changed = make(chan struct{})
	}
	return s.lease.changed
}

// leaseBroadcastLocked wakes every waiter and arms a fresh channel.
func (s *Store) leaseBroadcastLocked() {
	if s.lease.changed != nil {
		close(s.lease.changed)
	}
	s.lease.changed = make(chan struct{})
}

// leaseWorkContextLocked is the context of a lease goroutine: cancelled by
// Close, carrying the capability that admits a lease write and the store's
// own writer.
func (s *Store) leaseWorkContextLocked() context.Context {
	s.lease.ctxMu.Lock()
	if s.lease.ctx == nil || s.lease.ctx.Err() != nil {
		s.lease.ctx, s.lease.cancel = context.WithCancel(context.Background())
	}
	s.lease.ctxMu.Unlock()
	return objectgraph.WithWriter(objectgraph.WithStreamSeqLeaseCapability(s.lease.ctx), objectgraph.WriterStore)
}

var (
	errLeaseClosing  = errors.New("append event: stream_seq lease: the store is closing")
	errLeaseDeadline = errors.New("deadline passed")
	// errLeaseTombstoned marks a lease row that is retired: untrusted, logged
	// and counted by the seeder, never a failure to seed.
	errLeaseTombstoned = errors.New("the lease row is tombstoned")
)

// allocateStreamSeqLocked gives rec the next store-wide stream_seq, always at
// or below a lease that has passed the head CAS. The caller holds eventMu; it
// is held again on return, but released while the allocation waits for a seed
// or a refill. On any error no number is taken.
func (s *Store) allocateStreamSeqLocked(ctx context.Context, rec *types.EventRecord, folded bool) (took bool, err error) {
	var seedingDeadline, readyDeadline time.Time
	for {
		if s.lease.closing.Load() {
			return false, errLeaseClosing
		}
		switch s.lease.state {
		case leaseUnseeded:
			s.startSeederLocked()
			if seedingDeadline.IsZero() {
				seedingDeadline = time.Now().Add(leaseSeedingDeadline())
			}
			if err := s.leaseBlockedLocked(ctx, seedingDeadline, s.lease.seedNotBefore, "seed"); err != nil {
				return false, err
			}
		case leaseSeeding:
			if seedingDeadline.IsZero() {
				seedingDeadline = time.Now().Add(leaseSeedingDeadline())
			}
			if err := s.leaseBlockedLocked(ctx, seedingDeadline, time.Time{}, "seed"); err != nil {
				return false, err
			}
		default: // leaseReady
			if folded && rec.Seq != 0 && rec.StreamSeq != 0 {
				return false, nil // numbered by another caller while this one waited
			}
			next := s.maxStreamSeq + 1
			if next > s.lease.reserved {
				s.startRefillLocked()
				if readyDeadline.IsZero() {
					readyDeadline = time.Now().Add(leaseReadyDeadline)
				}
				if err := s.leaseBlockedLocked(ctx, readyDeadline, s.lease.refillNotBefore, "refill"); err != nil {
					return false, err
				}
				continue
			}
			s.maxStreamSeq = next
			rec.StreamSeq = next
			if s.lease.reserved-next < streamSeqLeaseBlock/2 {
				s.startRefillLocked()
			}
			return true, nil
		}
	}
}

// leaseBlockedLocked waits for a lease change, giving up at the deadline or
// when ctx is done. It returns an error only for a deadline already passed or
// a cancelled ctx, checked before the wait; after the wait the caller
// re-evaluates the state, so a lease that landed at the deadline still serves.
// wakeAt, when in the future, also ends the wait (a backoff expiring).
func (s *Store) leaseBlockedLocked(ctx context.Context, deadline, wakeAt time.Time, what string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("append event: stream_seq lease: %w", err)
	}
	now := time.Now()
	if !now.Before(deadline) {
		last := s.lease.lastErr
		if last == "" {
			last = "none"
		}
		return fmt.Errorf("append event: stream_seq lease: no %s within the deadline (state %s, reserved %d, last lease error: %s): %w",
			what, s.lease.state, s.lease.reserved, last, errLeaseDeadline)
	}
	until := deadline
	if wakeAt.After(now) && wakeAt.Before(until) {
		until = wakeAt
	}
	changed := s.leaseChangedLocked()
	closing := s.leaseClosingChan()
	s.eventMu.Unlock()
	timer := time.NewTimer(time.Until(until))
	select {
	case <-changed:
	case <-closing: // Close began: the loop head returns the closing error
	case <-timer.C:
	case <-ctx.Done():
	}
	timer.Stop()
	s.eventMu.Lock()
	return nil
}

// StartStreamSeqSeed starts the seeder if the lease is unseeded, and returns
// at once. It is the eager seed: autoputer calls it right after replay and the
// vocabulary fence, before the runtime serves, so a computer with no lease
// pays the exact seed while event appends wait rather than fail. Calling it
// again, or after the seed has started, does nothing.
func (s *Store) StartStreamSeqSeed() {
	if s == nil {
		return
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	s.startSeederLocked()
}

// startSeederLocked starts the single seeder goroutine. It does nothing unless
// the state is unseeded, the store is not closing and the last failure's
// backoff has passed.
func (s *Store) startSeederLocked() {
	if s.lease.closing.Load() || s.lease.state != leaseUnseeded || time.Now().Before(s.lease.seedNotBefore) {
		return
	}
	ctx := s.leaseWorkContextLocked()
	workspace := s.lease.workspace
	s.lease.state = leaseSeeding
	s.lease.workers.Add(1)
	s.publishLeaseObsLocked()
	s.leaseBroadcastLocked()
	go s.runSeeder(ctx, workspace)
}

// leaseSeedResult is what the seeder computed before it wrote the lease.
type leaseSeedResult struct {
	seed          int64
	basis         string
	trusted       bool
	untrustReason string
	crossMax      int64
	undecodable   int64
	// onTape: the seed's lease row was written with the projection tape bound.
	onTape bool
}

// runSeeder computes the seed with eventMu released, lands a fresh lease block
// on the tape, and only then lets allocation begin. On failure the state goes
// back to unseeded and the waiters retry or fail at their deadline.
func (s *Store) runSeeder(ctx context.Context, workspace string) {
	started := time.Now()
	res, err := s.computeLeaseSeed(ctx, workspace)
	var row leaseRow
	if err == nil {
		row = leaseRow{Reserved: res.seed + streamSeqLeaseBlock, Allocated: res.seed, Block: streamSeqLeaseBlock,
			WriterEpoch: leaseWriterEpoch, Basis: res.basis}
		res.onTape = s.projectionTape != nil
		err = s.appendLease(ctx, "seed", &row)
	}
	if err == nil && res.basis == leaseBasisExact {
		// The exact seed's lease has passed the head CAS: the marker's work is
		// done. A failure to clear leaves the marker, which only costs one more
		// exact seed next boot.
		if clearErr := storeschema.ClearLeaseUntrusted(workspace); clearErr != nil {
			log.Printf("store: stream_seq lease: clear untrust marker: %v", clearErr)
		}
	}
	took := time.Since(started)

	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	s.lease.workers.Add(-1)
	if err != nil {
		s.lease.state = leaseUnseeded
		s.lease.seedFailed++
		s.lease.seedStreak++
		// Mirror the refill's backoff, doubling per consecutive failure up to
		// 30 s: a failing seed rescans the event history, and an allocation
		// arriving every few milliseconds must not start a rescan each time.
		backoff := leaseRefillBackoff
		for i := int64(1); i < s.lease.seedStreak && backoff < leaseSeedBackoffCap; i++ {
			backoff *= 2
		}
		backoff = min(backoff, max(leaseSeedBackoffCap, leaseRefillBackoff))
		s.lease.lastErr = fmt.Sprintf("seed attempt %d failed: %v", s.lease.seedFailed, err)
		s.lease.seedNotBefore = time.Now().Add(backoff)
		log.Printf("store: stream_seq lease: seed attempt %d failed after %s (next in %s): %v", s.lease.seedFailed, took.Round(time.Millisecond), backoff, err)
	} else {
		s.completeSeedLocked(res, row, took)
	}
	s.publishLeaseObsLocked()
	s.leaseBroadcastLocked()
}

// completeSeedLocked is the one place a seed raises maxStreamSeq: the lease
// block has passed the head CAS, so numbers up to row.Reserved are leased.
func (s *Store) completeSeedLocked(res leaseSeedResult, row leaseRow, took time.Duration) {
	s.maxStreamSeq = res.seed
	s.lease.reserved = row.Reserved
	s.lease.allocatedThrough = row.Allocated
	s.lease.state = leaseReady
	s.lease.seedStreak = 0
	s.lease.seedBasis = res.basis
	s.lease.trusted = res.trusted
	s.lease.untrustReason = res.untrustReason
	s.lease.crossCheckMax = res.crossMax
	s.lease.undecodable = res.undecodable
	s.lease.onTape.Store(res.onTape)
	s.lease.seedMS = took.Milliseconds()
	// lastErr is deliberately not cleared: it is the last error, evidence of a
	// failure that has since healed.
	log.Printf("store: stream_seq lease: seeded from %s (trusted=%t reason=%q) seed %d reserved through %d in %s",
		res.basis, res.trusted, res.untrustReason, res.seed, row.Reserved, took.Round(time.Millisecond))
}

// computeLeaseSeed decides whether the lease is trusted and returns the seed.
// It runs with eventMu released; allocation waits meanwhile, so no
// stream_seq is minted or appended during the scan.
func (s *Store) computeLeaseSeed(ctx context.Context, workspace string) (leaseSeedResult, error) {
	// The seeder's precondition: the local head is the platform head, so
	// replay cannot add events during the scan and the lease append, which
	// requires equal heads, lands on the state that was scanned.
	if tape := s.projectionTape; tape != nil {
		if err := tape.appender.HeadsAligned(ctx); err != nil {
			return leaseSeedResult{}, fmt.Errorf("stream_seq seed: %w", err)
		}
	}
	row, found, err := s.readLease(ctx)
	if errors.Is(err, errLeaseTombstoned) {
		// A retired lease is not a lease: the zero row fails consistent() and
		// the exact seed runs. Say so, and count it.
		log.Printf("store: stream_seq lease: the lease row is tombstoned; it is untrusted and the exact seed runs")
		s.eventMu.Lock()
		s.lease.tombstoned++
		s.eventMu.Unlock()
		err = nil
	}
	if err != nil {
		return leaseSeedResult{}, fmt.Errorf("stream_seq seed: read lease: %w", err)
	}
	// One presence read: anything but a clean absence reads as present.
	markerPresent, _ := storeschema.LeaseUntrusted(workspace)
	markerReason := ""
	if markerPresent {
		markerReason, _, _ = storeschema.LeaseUntrustedReason(workspace)
	}

	untrust := ""
	switch {
	case !found:
		untrust = "no_lease"
	case markerPresent:
		untrust = "marker"
		if markerReason == "receipt" {
			untrust = "receipt"
		}
	case !row.consistent():
		untrust = "inconsistent"
	}

	res := leaseSeedResult{}
	if untrust == "" {
		cross, err := s.leaseCrossCheck(ctx)
		if err != nil {
			return leaseSeedResult{}, fmt.Errorf("stream_seq seed: cross-check: %w", err)
		}
		res.crossMax = cross
		switch {
		case cross > row.Reserved:
			untrust = "violated"
			log.Printf("store: stream_seq lease violated: the lease reserves through %d and the store holds %d; taking the exact seed", row.Reserved, cross)
		case row.Reserved > cross+streamSeqLeaseCeiling:
			untrust = "ceiling"
		}
	}
	if untrust == "" {
		res.seed, res.basis, res.trusted = row.Reserved, leaseBasisLease, true
		return res, nil
	}

	started := time.Now()
	ogMax, tableMax, err := s.exactStreamSeqMax(ctx)
	if err != nil {
		return leaseSeedResult{}, err
	}
	exact := max(ogMax, tableMax)
	res.undecodable = s.lease.lastUndecodable.Load()
	// A legitimate lease covers the numbers its lineage published and never
	// stored; the exact maximum covers anything an older binary stored. A row
	// that fails its own checks, or sits above the ceiling, is ignored.
	var carried int64
	switch {
	case !found:
	case !row.consistent():
		log.Printf("store: stream_seq lease ignored: R=%d is not self-consistent (A=%d block=%d epoch=%d basis=%q)",
			row.Reserved, row.Allocated, row.Block, row.WriterEpoch, row.Basis)
	case row.Reserved > exact+streamSeqLeaseCeiling:
		log.Printf("store: stream_seq lease ignored: R=%d is above the exact maximum %d by more than the ceiling %d", row.Reserved, exact, streamSeqLeaseCeiling)
	default:
		carried = row.Reserved
	}
	res.seed = max(exact, carried) + streamSeqExactGap
	res.basis, res.trusted, res.untrustReason = leaseBasisExact, false, untrust
	log.Printf("store: stream_seq exact seed: %s; og max %d, table max %d, lease %d, seed %d, took %s",
		untrust, ogMax, tableMax, carried, res.seed, time.Since(started).Round(time.Millisecond))
	return res, nil
}

// leaseCrossCheck reads the newest events by the bounded windows (the og
// objects among the newest streamSeedWindows, and the newest 500 rows of
// choir_events by tape position). It is a staleness check, not a seed: a
// result above the lease makes the lease untrusted, and a result below it (a
// window that holds no event reads 0) says nothing.
func (s *Store) leaseCrossCheck(ctx context.Context) (int64, error) {
	graph := s.ogStore
	if graph == nil {
		return 0, fmt.Errorf("object graph not initialized")
	}
	var maxSeq int64
	for _, window := range streamSeedWindows {
		headers, err := graph.ListNewestObjectHeaders(ctx, window)
		if err != nil {
			return 0, err
		}
		found := false
		for _, h := range headers {
			if h.Kind != ogKindEvent {
				continue
			}
			var meta struct {
				StreamSeq *int64 `json:"stream_seq"`
			}
			if err := json.Unmarshal(h.Metadata, &meta); err != nil || meta.StreamSeq == nil {
				continue
			}
			found = true
			maxSeq = max(maxSeq, *meta.StreamSeq)
		}
		if found || len(headers) < window {
			break
		}
	}
	tableStream, err := s.eventTableNewestStreamSeq(ctx)
	if err != nil {
		return 0, err
	}
	return max(maxSeq, tableStream), nil
}

// exactStreamSeqMax is the exact seed's two halves: the maximum stream_seq of
// the event kind in og_objects and in choir_events.
func (s *Store) exactStreamSeqMax(ctx context.Context) (ogMax, tableMax int64, err error) {
	ogMax, err = s.exactOGStreamSeqMax(ctx)
	if err != nil {
		return 0, 0, err
	}
	tableMax, err = s.exactTableStreamSeqMax(ctx)
	if err != nil {
		return 0, 0, err
	}
	return ogMax, tableMax, nil
}

// exactOGStreamSeqMax scans the whole event kind of og_objects, metadata only,
// a page at a time with the engine mutex released between pages. A row whose
// metadata has no stream_seq has its body decoded. The walk is the primary-key
// range of the kind's ids (a plan with no sort: measured, the keyset page by
// the kind predicate costs the kind once per page). A row of the kind outside
// that range makes the walk fall back to the kind-predicate pages, so the
// answer is exact whatever ids the rows carry.
func (s *Store) exactOGStreamSeqMax(ctx context.Context) (int64, error) {
	graph := s.ogStore
	if graph == nil {
		return 0, fmt.Errorf("append event: seed stream_seq: object graph not initialized")
	}
	outside, err := graph.KindHasRowsOutsideIDRange(ctx, ogKindEvent)
	if err != nil {
		return 0, fmt.Errorf("append event: seed stream_seq (exact scan): %w", err)
	}
	var maxSeq, undecodable int64
	defer func() { s.lease.lastUndecodable.Store(undecodable) }()
	take := func(row objectgraph.KindMetadataRow) error {
		var meta struct {
			StreamSeq *int64 `json:"stream_seq"`
		}
		if err := json.Unmarshal(row.Metadata, &meta); err == nil && meta.StreamSeq != nil {
			maxSeq = max(maxSeq, *meta.StreamSeq)
			return nil
		}
		obj, err := graph.GetObject(ctx, row.CanonicalID)
		if err != nil {
			return fmt.Errorf("append event: seed stream_seq (exact scan): read %s: %w", row.CanonicalID, err)
		}
		var ev types.EventRecord
		if err := ogDecode(obj, &ev); err != nil {
			// An event body this binary cannot decode carries no number it can
			// see; the metadata path covers every event this binary or its
			// predecessors wrote. Count it and say so: a skipped row is a
			// number the seed did not see (a desk-forged or corrupt row).
			undecodable++
			log.Printf("store: stream_seq exact seed: undecodable event row %s skipped: %v", row.CanonicalID, err)
			return nil
		}
		maxSeq = max(maxSeq, ev.StreamSeq)
		return nil
	}
	after := ""
	for page := 1; ; page++ {
		var rows []objectgraph.KindMetadataRow
		var next string
		var scanned int
		if outside {
			rows, err = graph.ListKindMetadataPage(ctx, ogKindEvent, after, leaseScanPage)
			scanned = len(rows)
			if len(rows) > 0 {
				next = rows[len(rows)-1].CanonicalID
			}
		} else {
			rows, next, scanned, err = graph.ListKindIDRangePage(ctx, ogKindEvent, after, leaseScanPage)
		}
		if err != nil {
			return 0, fmt.Errorf("append event: seed stream_seq (exact scan): %w", err)
		}
		for _, row := range rows {
			if err := take(row); err != nil {
				return 0, err
			}
		}
		if hook := s.leaseScanHook; hook != nil {
			hook(page)
		}
		if scanned < leaseScanPage {
			return maxSeq, nil
		}
		after = next
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}
}

// exactTableStreamSeqMax is the exact maximum over choir_events by one reverse
// seek per owner, owners listed by skip-seeks on the same index.
func (s *Store) exactTableStreamSeqMax(ctx context.Context) (int64, error) {
	var maxSeq int64
	ownerMax := func(owner string) error {
		query, args := eventTableOwnerMaxStreamSQL(owner)
		return s.eventTableDB("store.event_table.seed_owner_max", func(db *sql.DB) error {
			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var v int64
				if err := rows.Scan(&v); err != nil {
					return err
				}
				maxSeq = max(maxSeq, v)
			}
			return rows.Err()
		})
	}
	nextOwner := func(after string) (string, bool, error) {
		query, args := eventTableNextOwnerSQL(after)
		var owner string
		found := false
		err := s.eventTableDB("store.event_table.seed_next_owner", func(db *sql.DB) error {
			rows, err := db.QueryContext(ctx, query, args...)
			if err != nil {
				return err
			}
			defer rows.Close()
			if rows.Next() {
				found = true
				if err := rows.Scan(&owner); err != nil {
					return err
				}
			}
			return rows.Err()
		})
		return owner, found, err
	}
	fail := func(err error) (int64, error) {
		return 0, fmt.Errorf("append event: seed stream_seq (exact %s scan): %w", choirEventsTable, err)
	}
	// The empty owner sorts before every other; a row cannot name it through
	// the object write path, but the seed does not rely on that.
	if err := ownerMax(""); err != nil {
		return fail(err)
	}
	after := ""
	for {
		owner, found, err := nextOwner(after)
		if err != nil {
			return fail(err)
		}
		if !found {
			return maxSeq, nil
		}
		if err := ownerMax(owner); err != nil {
			return fail(err)
		}
		after = owner
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}
}

// readLease reads the lease object by one primary-key seek. found is true for
// a row that exists even if it does not parse (the zero row then fails
// consistent()), so a corrupt row is untrusted, not absent.
func (s *Store) readLease(ctx context.Context) (leaseRow, bool, error) {
	if s.ogStore == nil {
		return leaseRow{}, false, fmt.Errorf("object graph not initialized")
	}
	id, err := objectgraph.BuildCanonicalID(objectgraph.KindStreamSeqLease, leaseOwnerID, objectgraph.StableSuffixFromKey(leaseIdentityKey))
	if err != nil {
		return leaseRow{}, false, err
	}
	obj, err := s.ogStore.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return leaseRow{}, false, nil
	}
	if err != nil {
		return leaseRow{}, false, err
	}
	if obj.Tombstone || obj.ObjectKind != objectgraph.KindStreamSeqLease {
		return leaseRow{}, true, errLeaseTombstoned
	}
	var row leaseRow
	if err := json.Unmarshal(obj.Metadata, &row); err != nil {
		return leaseRow{}, true, nil
	}
	return row, true, nil
}

// writeLease writes the row through the object graph's write path, which takes
// it to the tape when the store is bound. It declares the store as the writer
// and carries the capability; the fixed timestamps make the object a pure
// function of the row. The returned error is the append's: a lease that did
// not pass the head CAS and the local finalize is not a lease.
func (s *Store) writeLease(ctx context.Context, row leaseRow) error {
	if s.og == nil {
		return fmt.Errorf("store: object graph not initialized")
	}
	raw, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("store: marshal stream_seq lease: %w", err)
	}
	normalized, err := objectgraph.NormalizeMetadata(raw)
	if err != nil {
		return err
	}
	ctx = objectgraph.WithWriter(objectgraph.WithStreamSeqLeaseCapability(ctx), objectgraph.WriterStore)
	_, err = s.og.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:        objectgraph.KindStreamSeqLease,
		OwnerID:     leaseOwnerID,
		IdentityKey: leaseIdentityKey,
		Body:        normalized,
		Metadata:    normalized,
		Now:         leaseFixedTime,
	})
	return err
}

// appendLease stamps the head sequence, runs the test hook, writes the row and
// logs the cost. phase is "seed" or "refill".
func (s *Store) appendLease(ctx context.Context, phase string, row *leaseRow) error {
	if tape := s.projectionTape; tape != nil {
		if head, err := s.Head(ctx, tape.computerID); err == nil && head != nil {
			row.HeadSequence = head.Sequence
		}
	}
	if hook := s.leaseWriteHook; hook != nil {
		if err := hook(phase, row.Reserved); err != nil {
			return err
		}
	}
	if err := s.writeLease(ctx, *row); err != nil {
		return err
	}
	if hook := s.leaseWriteHook; hook != nil && phase == "refill" {
		// The lease has passed the CAS; this is the failure after it (a
		// finalize failure in production).
		if err := hook("refill_after", row.Reserved); err != nil {
			return err
		}
	}
	return nil
}

// startRefillLocked starts the single refill goroutine: the lease for the next
// block, written while the current block still serves. It does nothing while a
// refill is in flight, during the backoff after a failure, or while the store
// is closing.
func (s *Store) startRefillLocked() {
	if s.lease.closing.Load() || s.lease.state != leaseReady || s.lease.refillInFlight || time.Now().Before(s.lease.refillNotBefore) {
		return
	}
	target := max(s.lease.reserved, s.maxStreamSeq) + streamSeqLeaseBlock
	allocated := s.maxStreamSeq
	ctx := s.leaseWorkContextLocked()
	s.lease.refillInFlight = true
	s.lease.workers.Add(1)
	go s.runRefill(ctx, target, allocated)
}

// runRefill appends the next block. On any error, including one after the CAS
// such as a finalize failure, the in-memory R is left alone: a number above
// the old R is never handed out on a lease this process cannot show. R never
// decreases.
func (s *Store) runRefill(ctx context.Context, target, allocated int64) {
	row := leaseRow{Reserved: target, Allocated: allocated, Block: streamSeqLeaseBlock,
		WriterEpoch: leaseWriterEpoch, Basis: leaseBasisLease}
	started := time.Now()
	onTape := s.projectionTape != nil
	err := s.appendLease(ctx, "refill", &row)

	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	s.lease.refillInFlight = false
	s.lease.workers.Add(-1)
	if err != nil {
		s.lease.refillFailed++
		s.lease.lastErr = err.Error()
		s.lease.refillNotBefore = time.Now().Add(leaseRefillBackoff)
		log.Printf("store: stream_seq lease: refill to %d failed after %s: %v", target, time.Since(started).Round(time.Millisecond), err)
	} else {
		s.lease.reserved = max(s.lease.reserved, target)
		s.lease.allocatedThrough = allocated
		s.lease.onTape.Store(onTape)
	}
	s.publishLeaseObsLocked()
	s.leaseBroadcastLocked()
}

// cancelLeaseWork refuses new lease work and cancels the seeder and the
// refill in flight. It takes no store lock: Close calls it first, while a
// commit may hold eventMu (the inline-commit fence), and must not wait behind
// it. The lease refuses new work until resumeLeaseWork (Reopen, or a Close that
// failed and left the store open).
func (s *Store) cancelLeaseWork() {
	s.lease.closing.Store(true)
	s.lease.ctxMu.Lock()
	if s.lease.closingCh == nil {
		s.lease.closingCh = make(chan struct{})
	}
	select {
	case <-s.lease.closingCh:
	default:
		close(s.lease.closingCh) // wakes every parked waiter, with no eventMu
	}
	if s.lease.cancel != nil {
		s.lease.cancel()
	}
	s.lease.ctxMu.Unlock()
}

// leaseClosingChan is the channel cancelLeaseWork closes.
func (s *Store) leaseClosingChan() <-chan struct{} {
	s.lease.ctxMu.Lock()
	defer s.lease.ctxMu.Unlock()
	if s.lease.closingCh == nil {
		s.lease.closingCh = make(chan struct{})
	}
	return s.lease.closingCh
}

// drainLeaseWork waits, until deadline, for the lease goroutines to end, and
// wakes any allocation waiting on the lease so it sees the store is closing.
// Close calls it after the commit-lock fence and before it pins the pools: a
// goroutine still mid-statement then fails the pin closed, as any statement in
// flight does.
func (s *Store) drainLeaseWork(deadline time.Time) {
	for {
		if s.eventMu.TryLock() {
			s.leaseBroadcastLocked()
			s.eventMu.Unlock()
		}
		if s.lease.workers.Load() == 0 || !time.Now().Before(deadline) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// leaseResumeHook is a test seam: it runs in resumeLeaseWork once the lease
// may start work again. Always nil in production.
var leaseResumeHook func(s *Store)

// resumeLeaseWork lets the lease start work again.
func (s *Store) resumeLeaseWork() {
	s.lease.ctxMu.Lock()
	s.lease.ctx, s.lease.cancel = nil, nil
	s.lease.closingCh = nil
	s.lease.ctxMu.Unlock()
	s.lease.closing.Store(false)
	if hook := leaseResumeHook; hook != nil {
		hook(s)
	}
}

// refreshLeaseWorkspace re-reads the workspace after Reopen adopted another
// one, and republishes the snapshot (marker_present describes the new
// workspace). It takes eventMu, which resumeLeaseWork must not: a failed Close
// resumes while an inline commit may still hold eventMu across the fence.
func (s *Store) refreshLeaseWorkspace() {
	s.eventMu.Lock()
	s.lease.workspace = s.texturePath
	s.publishLeaseObsLocked()
	s.eventMu.Unlock()
}

// initLeaseObs sets the workspace the lease reads its marker from and
// publishes the first snapshot. Open calls it before the store is shared.
func (s *Store) initLeaseObs() {
	s.eventMu.Lock()
	s.lease.workspace = s.texturePath
	s.publishLeaseObsLocked()
	s.eventMu.Unlock()
}

// leaseSwapReason names why a marker was written, for the marker body.
const (
	leaseMarkerReceipt = "receipt"
	leaseMarkerReplace = "replace"
)

// priorReceiptUntrustReason decides, from the receipt Open read before it
// rewrote it, whether the workspace's lease is distrusted. A prior receipt
// below the epoch that writes leases, one that cannot be read, or one that is
// absent on a workspace that already holds data, means a binary that does not
// know the lease may have allocated past it. An absent receipt on a workspace
// with no data is a fresh store.
func priorReceiptUntrustReason(found bool, version uint64, readErr error, hadData bool) string {
	switch {
	case readErr != nil:
		return leaseMarkerReceipt
	case !found && hadData:
		return leaseMarkerReceipt
	case found && version < leaseMinWriterEpoch:
		return leaseMarkerReceipt
	}
	return ""
}

// workspaceHoldsData reports whether a workspace directory exists and holds
// anything: an empty or absent directory is a store about to be created.
func workspaceHoldsData(workspace string) bool {
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return !errors.Is(err, os.ErrNotExist) // unreadable: assume data
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".stream-seq-lease-untrusted-") {
			return true
		}
	}
	return false
}
