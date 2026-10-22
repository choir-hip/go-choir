package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The telemetry sealer (docs/reports/telemetry-batching-design-2026-10-20.md,
// phase 1). Standalone Store.AppendEvent calls of the ten batched kinds are
// queued and sealed into one ordinary projection_batch_recorded tape event
// holding N choir.event ops, instead of one acknowledged write each. Every
// acknowledged write first seals and waits for the pending window (the append
// barrier), so a crash loses the newest telemetry of a run and never a
// semantic event or a hole in the middle.
//
// Lock discipline, all load-bearing:
//   - a record is enqueued INSIDE Store.eventMu, after seq and stream_seq are
//     allocated, so enqueue order equals allocation order;
//   - every flush runs OUTSIDE eventMu: the projection store's Prepare and
//     Finalize take eventMu under the appender mutex, so a flush under eventMu
//     would deadlock;
//   - the sealer's own mutex mu is a leaf (nothing is called while holding it)
//     and the sealer takes no other store lock;
//   - flushMu serialises flushes; the barrier runs before the appender mutex.

const (
	// telemetryMaxBatchOps and telemetryMaxBatchBytes cap one sealed batch.
	telemetryMaxBatchOps   = 64
	telemetryMaxBatchBytes = 256 << 10
	// telemetryQueueCapacity is the hard cap on pending plus in-flight plus
	// reserved records. The reseed gap below depends on it.
	telemetryQueueCapacity = 256
	// telemetrySeqGap is added to the stored maximum of stream_seq and of a
	// run's seq when a process seeds them, so a restart cannot reissue a number
	// the previous process published and lost: capacity + largest batch, rounded
	// up (design 6.1).
	telemetrySeqGap = 512

	telemetryDefaultWindow = time.Second
	telemetryMinWindow     = 250 * time.Millisecond
	telemetryMaxWindow     = 2 * time.Second
	telemetrySealTimeout   = 30 * time.Second
)

// TelemetryBatchConfig is the sealer's runtime configuration.
type TelemetryBatchConfig struct {
	Enabled bool
	// Window is W, the deadline measured from the first unsealed record. Zero
	// means the 1 s default; other values are clamped to 250 ms to 2 s.
	Window time.Duration
}

// TelemetryBatchConfigFromEnv reads CHOIR_TELEMETRY_BATCH (on|off, default
// off) and CHOIR_TELEMETRY_BATCH_WINDOW_MS.
func TelemetryBatchConfigFromEnv() TelemetryBatchConfig {
	cfg := TelemetryBatchConfig{}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CHOIR_TELEMETRY_BATCH"))) {
	case "on", "1", "true", "yes":
		cfg.Enabled = true
	}
	if ms, err := strconv.Atoi(strings.TrimSpace(os.Getenv("CHOIR_TELEMETRY_BATCH_WINDOW_MS"))); err == nil && ms > 0 {
		cfg.Window = time.Duration(ms) * time.Millisecond
	}
	return cfg
}

func clampTelemetryWindow(w time.Duration) time.Duration {
	switch {
	case w <= 0:
		return telemetryDefaultWindow
	case w < telemetryMinWindow:
		return telemetryMinWindow
	case w > telemetryMaxWindow:
		return telemetryMaxWindow
	}
	return w
}

// telemetryRecord is one queued runtime event. Its fields are unexported and
// it is built only by newTelemetryRecord, which refuses any kind outside the
// batched set: nothing outside this package, and nothing inside it except
// that constructor, can put an op in the queue (design 3.4 layer 3).
type telemetryRecord struct {
	kind      types.EventKind
	runID     string
	ownerID   string
	seq       int64
	streamSeq int64
	object    objectgraph.Object
	op        computerevent.ProjectionOp
	size      int
	n         int64 // enqueue order, assigned by the sealer
}

// newTelemetryRecord builds the choir.event object and op for a batched kind,
// exactly as the acknowledged path (AppendEventOG) would, or returns an error.
func newTelemetryRecord(rec *types.EventRecord) (telemetryRecord, error) {
	if rec == nil || !rec.Kind.Batchable() {
		kind := types.EventKind("")
		if rec != nil {
			kind = rec.Kind
		}
		return telemetryRecord{}, fmt.Errorf("telemetry: event kind %q is not batched", kind)
	}
	now := rec.Timestamp
	if now.IsZero() {
		now = time.Now().UTC()
	}
	obj, err := buildEventObject(rec, now)
	if err != nil {
		return telemetryRecord{}, err
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return telemetryRecord{}, fmt.Errorf("telemetry: marshal event object: %w", err)
	}
	return telemetryRecord{
		kind: rec.Kind, runID: rec.RunID, ownerID: rec.OwnerID, seq: rec.Seq, streamSeq: rec.StreamSeq,
		object: obj,
		op:     computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: obj.CanonicalID, Body: body},
		size:   len(body),
	}, nil
}

type telemetrySealer struct {
	s *Store

	mu            sync.Mutex // leaf; protects everything below
	enabled       bool
	synchronous   bool // syncHolds > 0
	syncHolds     int  // planned-restart holders of synchronous mode
	stopped       bool
	window        time.Duration
	queue         []telemetryRecord
	queueBytes    int
	inflightRecs  []telemetryRecord
	reserved      int
	enqueuedN     int64 // records ever enqueued
	sealedThrough int64 // highest enqueue number whose seal finished (success or not)
	timer         *time.Timer
	timerGen      int64
	lastLog       time.Time

	flushMu sync.Mutex // one flusher at a time

	bootMu      sync.Mutex
	watermarked bool
}

// ConfigureTelemetryBatching applies the flag and window. The first time it is
// enabled it creates the sealer and installs the append barrier on the bound
// appender; a process that never enables it runs the old code. Disabling
// drains the queue and returns to acknowledged writes.
func (s *Store) ConfigureTelemetryBatching(cfg TelemetryBatchConfig) error {
	sl := s.telemetry.Load()
	if sl == nil {
		if !cfg.Enabled {
			return nil
		}
		if s.projectionTape == nil {
			return fmt.Errorf("telemetry batching requires the projection tape to be bound")
		}
		sl = &telemetrySealer{s: s, window: clampTelemetryWindow(cfg.Window)}
		if !s.telemetry.CompareAndSwap(nil, sl) {
			sl = s.telemetry.Load()
		}
		s.projectionTape.appender.SetAppendBarrier(sl.appendBarrier)
	}
	sl.mu.Lock()
	sl.window = clampTelemetryWindow(cfg.Window)
	was := sl.enabled
	sl.enabled = cfg.Enabled
	sl.mu.Unlock()
	if was && !cfg.Enabled {
		sl.drain()
	}
	return nil
}

// TelemetryBatchingEnabled reports whether new batched-kind events are being
// queued right now.
func (s *Store) TelemetryBatchingEnabled() bool {
	sl := s.telemetry.Load()
	if sl == nil {
		return false
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.enabled && !sl.synchronous && !sl.stopped
}

// TelemetryBatchingOn reports whether batching is switched on, whether or not
// a planned restart holds synchronous mode right now.
func (s *Store) TelemetryBatchingOn() bool {
	sl := s.telemetry.Load()
	if sl == nil {
		return false
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.enabled && !sl.stopped
}

// TelemetryBatchingConfigured reports whether the sealer exists in this
// process, which is when a crash can have lost a telemetry window.
func (s *Store) TelemetryBatchingConfigured() bool { return s.telemetry.Load() != nil }

// DrainTelemetry seals everything queued and waits, bounded by ctx. It is the
// shutdown and planned-restart drain; with no sealer it is a no-op.
func (s *Store) DrainTelemetry(ctx context.Context) error {
	sl := s.telemetry.Load()
	if sl == nil {
		return nil
	}
	lostBefore := s.telemetryStats.recordsDropped.Load()
	done := make(chan struct{})
	go func() {
		sl.drain()
		close(done)
	}()
	select {
	case <-done:
		if lost := s.telemetryStats.recordsDropped.Load() - lostBefore; lost > 0 {
			return fmt.Errorf("telemetry drain lost %d records", lost)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("telemetry drain: %w", ctx.Err())
	}
}

// SetTelemetrySynchronous takes (on) or releases (off) one hold on
// synchronous mode: while any hold is taken, new batched-kind events take the
// acknowledged path. Taking a hold drains first. The planned-restart paths use
// it so nothing new queues behind the drain; a guest apply and the host's
// request can hold it at once, and the first release must not reopen the
// queue for the other. A release with no hold taken does nothing.
func (s *Store) SetTelemetrySynchronous(ctx context.Context, on bool) error {
	sl := s.telemetry.Load()
	if sl == nil {
		return nil
	}
	sl.mu.Lock()
	if on {
		sl.syncHolds++
	} else if sl.syncHolds > 0 {
		sl.syncHolds--
	}
	sl.synchronous = sl.syncHolds > 0
	sl.mu.Unlock()
	if on {
		return s.DrainTelemetry(ctx)
	}
	return nil
}

// closeTelemetry drains and stops the sealer. It reports whether this call
// stopped it, so a Close that then fails closed can resume it.
func (s *Store) closeTelemetry() bool {
	sl := s.telemetry.Load()
	if sl == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.DrainTelemetry(ctx)
	sl.mu.Lock()
	was := sl.stopped
	sl.stopped = true
	sl.stopTimerLocked()
	sl.mu.Unlock()
	return !was
}

// resumeTelemetry undoes closeTelemetry for a store a Close left open.
// While stopped, records were written synchronously; what the drain did
// not seal gets its deadline again.
func (s *Store) resumeTelemetry() {
	sl := s.telemetry.Load()
	if sl == nil {
		return
	}
	sl.mu.Lock()
	sl.stopped = false
	if len(sl.queue) > 0 {
		sl.armTimerLocked()
	}
	sl.mu.Unlock()
}

// ---- test seams (unexported) -------------------------------------------------

func (sl *telemetrySealer) setWindow(w time.Duration) {
	sl.mu.Lock()
	sl.window = w
	sl.mu.Unlock()
}

func (sl *telemetrySealer) depth() int {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return len(sl.queue) + len(sl.inflightRecs)
}

// discardForCrash drops the queue without sealing it: the process died.
func (sl *telemetrySealer) discardForCrash() {
	sl.mu.Lock()
	sl.queue, sl.queueBytes = nil, 0
	sl.stopped = true
	sl.stopTimerLocked()
	sl.mu.Unlock()
}

// ---- state -------------------------------------------------------------------

func (sl *telemetrySealer) snapshotState() (enabled, synchronous bool, windowMS, depth int) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.enabled, sl.synchronous, int(sl.window / time.Millisecond), len(sl.queue) + len(sl.inflightRecs)
}

// seqFloor is the highest seq the queue holds for one (run, owner): a reseed
// may not take a number at or below it (design 6.1, "floor from the queue").
func (sl *telemetrySealer) seqFloor(key eventSeqKey) int64 {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	var floor int64
	for _, list := range [][]telemetryRecord{sl.queue, sl.inflightRecs} {
		for _, rec := range list {
			if rec.runID == key.runID && rec.ownerID == key.ownerID && rec.seq > floor {
				floor = rec.seq
			}
		}
	}
	return floor
}

// accepting reports whether new records would be queued right now.
func (sl *telemetrySealer) accepting() bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.enabled && !sl.synchronous && !sl.stopped
}

// reserve claims one queue slot before the caller takes eventMu. It returns
// false when records are not being queued (off, synchronous, stopped). At the
// cap it flushes, outside every store lock, and retries: backpressure, never a
// silent drop.
func (sl *telemetrySealer) reserve() bool {
	for {
		sl.mu.Lock()
		if !sl.enabled || sl.synchronous || sl.stopped {
			sl.mu.Unlock()
			return false
		}
		if len(sl.queue)+len(sl.inflightRecs)+sl.reserved < telemetryQueueCapacity {
			sl.reserved++
			sl.mu.Unlock()
			return true
		}
		target := sl.enqueuedN
		progressed := len(sl.queue)+len(sl.inflightRecs) > 0
		sl.mu.Unlock()
		sl.s.telemetryStats.capacityFlushes.Add(1)
		sl.flushThrough(sealCapacity, target)
		if !progressed {
			time.Sleep(time.Millisecond) // only reservations hold the slots; let them enqueue
		}
	}
}

func (sl *telemetrySealer) release() {
	sl.mu.Lock()
	if sl.reserved > 0 {
		sl.reserved--
	}
	sl.mu.Unlock()
}

// enqueue turns a reservation into a queued record. It is called with
// Store.eventMu held and takes only sl.mu.
func (sl *telemetrySealer) enqueue(rec telemetryRecord) (trigger sealReason, triggered bool) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if sl.reserved > 0 {
		sl.reserved--
	}
	sl.enqueuedN++
	rec.n = sl.enqueuedN
	wasEmpty := len(sl.queue) == 0
	sl.queue = append(sl.queue, rec)
	sl.queueBytes += rec.size
	sl.s.telemetryStats.recordsEnqueued.Add(1)
	if wasEmpty {
		sl.armTimerLocked()
	}
	switch {
	case sl.queueBytes >= telemetryMaxBatchBytes:
		return sealBytes, true
	case len(sl.queue) >= telemetryMaxBatchOps:
		return sealCount, true
	}
	return 0, false
}

// afterEnqueue runs after eventMu is released: a count or byte trigger starts
// a flush on its own goroutine so the caller returns at enqueue.
func (sl *telemetrySealer) afterEnqueue(reason sealReason, triggered bool) {
	if !triggered {
		return
	}
	sl.mu.Lock()
	target := sl.enqueuedN
	sl.mu.Unlock()
	go sl.flushThrough(reason, target)
}

// armTimerLocked starts the deadline for a queue that just became non-empty.
// It is measured from this first record and later records do not move it.
func (sl *telemetrySealer) armTimerLocked() {
	sl.stopTimerLocked()
	gen := sl.timerGen
	sl.timer = time.AfterFunc(sl.window, func() {
		sl.mu.Lock()
		if gen != sl.timerGen || sl.stopped {
			sl.mu.Unlock()
			return
		}
		target := sl.enqueuedN
		sl.mu.Unlock()
		sl.flushThrough(sealDeadline, target)
	})
}

func (sl *telemetrySealer) stopTimerLocked() {
	sl.timerGen++
	if sl.timer != nil {
		sl.timer.Stop()
		sl.timer = nil
	}
}

// appendBarrier is installed on the appender: every acknowledged write calls
// it before taking the appender mutex.
func (sl *telemetrySealer) appendBarrier(context.Context) { sl.barrier(sealBarrier) }

// barrier seals and waits for every record enqueued before the call. With
// nothing queued or in flight it returns at once.
func (sl *telemetrySealer) barrier(reason sealReason) {
	sl.mu.Lock()
	idle := len(sl.queue) == 0 && len(sl.inflightRecs) == 0
	target := sl.enqueuedN
	sl.mu.Unlock()
	if idle {
		return
	}
	sl.s.telemetryStats.barrierWaits.Add(1)
	sl.flushThrough(reason, target)
}

// drain seals everything queued so far.
func (sl *telemetrySealer) drain() {
	sl.mu.Lock()
	target := sl.enqueuedN
	sl.mu.Unlock()
	sl.flushThrough(sealShutdown, target)
}

// flushThrough seals records in enqueue order until every record numbered
// <= upTo has been handled. One flusher runs at a time, so batches are sealed
// in the order they were queued, and a caller that arrives while another
// flusher is mid-batch waits for it, then seals whatever remains of its own.
func (sl *telemetrySealer) flushThrough(reason sealReason, upTo int64) {
	sl.flushMu.Lock()
	defer sl.flushMu.Unlock()
	for {
		sl.mu.Lock()
		if sl.sealedThrough >= upTo || len(sl.queue) == 0 {
			sl.mu.Unlock()
			return
		}
		n, bytes := 0, 0
		for n < len(sl.queue) && n < telemetryMaxBatchOps {
			if n > 0 && bytes+sl.queue[n].size > telemetryMaxBatchBytes {
				break
			}
			bytes += sl.queue[n].size
			n++
		}
		chunk := make([]telemetryRecord, n)
		copy(chunk, sl.queue[:n])
		rest := make([]telemetryRecord, len(sl.queue)-n)
		copy(rest, sl.queue[n:])
		sl.queue = rest
		sl.queueBytes -= bytes
		sl.inflightRecs = chunk
		if len(sl.queue) == 0 {
			sl.stopTimerLocked()
		}
		sl.mu.Unlock()

		sl.seal(chunk, reason)

		sl.mu.Lock()
		sl.inflightRecs = nil
		sl.sealedThrough = chunk[len(chunk)-1].n
		sl.mu.Unlock()
	}
}

// recheck re-decodes every op and asserts the invariant the constructor
// already enforces: an object op, of kind choir.event, whose event kind is
// batched (design 3.4 layer 4), unstamped or stamped for choir_events. It is the independent second look.
func recheckTelemetryOp(op computerevent.ProjectionOp) error {
	if op.Kind != computerevent.ProjectionOpObject {
		return fmt.Errorf("op kind %q is not an object", op.Kind)
	}
	if op.Table != "" && op.Table != choirEventsTable {
		// A sealed op lands in og_objects or, stamped, in choir_events
		// (event-table design 2.2); nothing else.
		return fmt.Errorf("op table %q is not %q", op.Table, choirEventsTable)
	}
	var obj objectgraph.Object
	if err := json.Unmarshal(op.Body, &obj); err != nil {
		return fmt.Errorf("decode object: %w", err)
	}
	if obj.ObjectKind != ogKindEvent {
		return fmt.Errorf("object kind %q is not %q", obj.ObjectKind, ogKindEvent)
	}
	var ev types.EventRecord
	if err := json.Unmarshal(obj.Body, &ev); err != nil {
		return fmt.Errorf("decode event: %w", err)
	}
	if !ev.Kind.Batchable() {
		return fmt.Errorf("event kind %q is not batched", ev.Kind)
	}
	return nil
}

// seal writes one chunk to the tape. Failure classes (design 6.2):
//   - class violation: the whole chunk falls back to ordinary acknowledged
//     singles, so nothing is lost and nothing non-telemetry rides under the
//     telemetry authority;
//   - refused before anything was pinned (ErrBatchRefused): split to singles,
//     a record refused alone is dropped as poison;
//   - any other error: the outcome is unknown (the pin or the CAS may have
//     happened), so it is NOT retried, which could append a second event; the
//     unchanged RecoverPrepared settles the first at boot. Counted and logged.
func (sl *telemetrySealer) seal(chunk []telemetryRecord, reason sealReason) {
	tape := sl.s.projectionTape
	ctx, cancel := context.WithTimeout(computerevent.WithBarrierExemption(context.Background()), telemetrySealTimeout)
	defer cancel()
	stats := &sl.s.telemetryStats
	started := time.Now()

	for _, rec := range chunk {
		if err := recheckTelemetryOp(rec.op); err != nil {
			stats.classViolations.Add(1)
			log.Printf("telemetry: class violation at seal time (%v); writing %d records as acknowledged singles", err, len(chunk))
			for _, r := range chunk {
				if err := tape.appendOps(ctx, []computerevent.ProjectionOp{r.op}); err != nil {
					stats.recordsDropped.Add(1)
					sl.logFailure("class-violation single", err)
				}
			}
			return
		}
	}

	ops := make([]computerevent.ProjectionOp, len(chunk))
	for i, rec := range chunk {
		ops[i] = rec.op
	}
	err := tape.appendOpsAs(ctx, ops, telemetryProjectionAuthority)
	switch {
	case err == nil:
		sl.sealed(len(ops), reason, started)
	case errors.Is(err, computerevent.ErrBatchRefused):
		stats.splitToSingles.Add(1)
		sl.logFailure("batch refused, splitting", err)
		for _, rec := range chunk {
			single := tape.appendOpsAs(ctx, []computerevent.ProjectionOp{rec.op}, telemetryProjectionAuthority)
			switch {
			case single == nil:
				sl.sealed(1, reason, started)
			case errors.Is(single, computerevent.ErrBatchRefused):
				stats.poisonDropped.Add(1)
				stats.recordsDropped.Add(1)
				sl.logFailure("poison record dropped", single)
			default:
				stats.sealFailures.Add(1)
				stats.unknownOutcomeLoss.Add(1)
				stats.recordsDropped.Add(1)
				sl.logFailure("single seal failed, not retried", single)
			}
		}
	default:
		stats.sealFailures.Add(1)
		stats.unknownOutcomeLoss.Add(int64(len(chunk)))
		stats.recordsDropped.Add(int64(len(chunk)))
		sl.logFailure(fmt.Sprintf("seal of %d records failed, outcome unknown, not retried", len(chunk)), err)
	}
}

func (sl *telemetrySealer) sealed(ops int, reason sealReason, started time.Time) {
	stats := &sl.s.telemetryStats
	stats.batchesSealed.Add(1)
	stats.opsSealed.Add(int64(ops))
	stats.sealedOpsHist[opsBucket(ops)].Add(1)
	stats.sealReasonCount[reason].Add(1)
	stats.recordSealLatency(time.Since(started))
}

// logFailure rate-limits seal failure logging to one line per second so an
// outage cannot flood the guest log.
func (sl *telemetrySealer) logFailure(what string, err error) {
	sl.mu.Lock()
	now := time.Now()
	quiet := now.Sub(sl.lastLog) < time.Second
	if !quiet {
		sl.lastLog = now
	}
	sl.mu.Unlock()
	if !quiet {
		log.Printf("telemetry: %s: %v", what, err)
	}
}

// ---- boot watermark ------------------------------------------------------------

// ensureBootWatermark writes the boot's one acknowledged record before the
// first batched record is queued, so the reseeded stream_seq floor is durable
// and a second crash seeds above it (design 6.1, case 2). It returns false
// when the write failed; the caller then takes the acknowledged path.
func (s *Store) ensureBootWatermark(ctx context.Context, sl *telemetrySealer) bool {
	sl.bootMu.Lock()
	defer sl.bootMu.Unlock()
	if sl.watermarked {
		return true
	}
	rec := &types.EventRecord{
		EventID: uuid.NewString(), OwnerID: "__system__", Timestamp: time.Now().UTC(),
		Kind:    types.EventRuntimeHealth,
		Payload: json.RawMessage(`{"telemetry_boot_watermark":true}`),
	}
	if err := s.AppendEvent(context.WithoutCancel(ctx), rec); err != nil {
		sl.logFailure("boot watermark write failed; telemetry stays acknowledged until it lands", err)
		return false
	}
	sl.watermarked = true
	s.telemetryStats.watermarkWrites.Add(1)
	return true
}
