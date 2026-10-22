package store

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// opsHistogramBounds are the inclusive upper bounds of the ops-per-tape-event
// buckets; the last bucket is open ("65+").
var opsHistogramBounds = [...]int{1, 2, 4, 8, 16, 32, 64}

const opsHistogramBuckets = len(opsHistogramBounds) + 1

func opsBucket(n int) int {
	for i, bound := range opsHistogramBounds {
		if n <= bound {
			return i
		}
	}
	return len(opsHistogramBounds)
}

// opsBucketLabel names bucket i: "1", "2", "3-4", "5-8", ..., "65+".
func opsBucketLabel(i int) string {
	if i >= len(opsHistogramBounds) {
		return strconv.Itoa(opsHistogramBounds[len(opsHistogramBounds)-1]+1) + "+"
	}
	hi := opsHistogramBounds[i]
	if i == 0 {
		return strconv.Itoa(hi)
	}
	lo := opsHistogramBounds[i-1] + 1
	if lo == hi {
		return strconv.Itoa(hi)
	}
	return strconv.Itoa(lo) + "-" + strconv.Itoa(hi)
}

// sealReason is why the sealer sealed a batch (design 4.3).
type sealReason int

const (
	sealDeadline sealReason = iota
	sealCount
	sealBytes
	sealBarrier
	sealRead
	sealShutdown
	sealCapacity
	sealReasons
)

var sealReasonNames = [sealReasons]string{"deadline", "count", "bytes", "barrier", "read", "shutdown", "capacity"}

// telemetryCounters is the store's batching instrumentation: what the code
// does today (phase 0: how much of the tape is batchable telemetry, how big
// tape events are) and what the sealer does when it is on (phase 1). Counters
// never gate behaviour. The zero value is ready to use.
type telemetryCounters struct {
	mu                    sync.Mutex
	appendedByKind        map[string]int64
	payloadBytesByKind    map[string]int64
	sinkByKind            [eventSinks]map[string]int64
	tapeEventsByAuthority map[string]int64
	tapeOpsByKind         map[string]int64

	appendedBatchable    atomic.Int64
	appendedAcknowledged atomic.Int64
	tapeEvents           atomic.Int64
	tapeOps              atomic.Int64
	tapeOpsHist          [opsHistogramBuckets]atomic.Int64
	readsConsistent      atomic.Int64
	readsRelaxed         atomic.Int64

	recordsEnqueued       atomic.Int64
	batchesSealed         atomic.Int64
	opsSealed             atomic.Int64
	sealedOpsHist         [opsHistogramBuckets]atomic.Int64
	sealReasonCount       [sealReasons]atomic.Int64
	barrierWaits          atomic.Int64
	classViolations       atomic.Int64
	recordsDropped        atomic.Int64
	poisonDropped         atomic.Int64
	sealFailures          atomic.Int64
	unknownOutcomeLoss    atomic.Int64
	splitToSingles        atomic.Int64
	capacityFlushes       atomic.Int64
	watermarkWrites       atomic.Int64
	sealLatencyNanosTotal atomic.Int64
	sealLatencyNanosMax   atomic.Int64
}

// eventSink is where an event the store accepted was written (R7 S0,
// docs/design/trace-segment-log-2026-10-22.md section 9). Today every event
// goes to the tape, whether as its own acknowledged tape event or as a record
// the sealer queues for one. The trace log is S4's; the dropped sink is the
// log's drop at disk-critical, also S4's. Both read zero until then.
type eventSink int

const (
	sinkTape eventSink = iota
	sinkTraceLog
	sinkDropped
	eventSinks
)

var eventSinkNames = [eventSinks]string{"tape", "trace_log", "dropped"}

// recordSink counts one event of kind that reached sink. It is called only
// after the write (or the sealer's enqueue) succeeded, so appended minus the
// sinks is the number of appends that failed.
func (c *telemetryCounters) recordSink(kind types.EventKind, sink eventSink) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.sinkByKind[sink] == nil {
		c.sinkByKind[sink] = map[string]int64{}
	}
	c.sinkByKind[sink][string(kind)]++
	c.mu.Unlock()
}

// recordAppend counts one AppendEvent call by kind and adds the size of its
// payload. The stored line is the payload plus a fixed envelope of about
// 0.3 KB (ids, kind, timestamps), so the payload mean is the line mean to
// within that.
func (c *telemetryCounters) recordAppend(kind types.EventKind, payloadBytes int) {
	if c == nil {
		return
	}
	if kind.Batchable() {
		c.appendedBatchable.Add(1)
	} else {
		c.appendedAcknowledged.Add(1)
	}
	c.mu.Lock()
	if c.appendedByKind == nil {
		c.appendedByKind = map[string]int64{}
	}
	c.appendedByKind[string(kind)]++
	if c.payloadBytesByKind == nil {
		c.payloadBytesByKind = map[string]int64{}
	}
	c.payloadBytesByKind[string(kind)] += int64(payloadBytes)
	c.mu.Unlock()
}

// recordTapeEvent counts one accepted projection-batch tape event.
func (c *telemetryCounters) recordTapeEvent(authority string, ops []computerevent.ProjectionOp) {
	if c == nil {
		return
	}
	c.tapeEvents.Add(1)
	c.tapeOps.Add(int64(len(ops)))
	c.tapeOpsHist[opsBucket(len(ops))].Add(1)
	c.mu.Lock()
	if c.tapeEventsByAuthority == nil {
		c.tapeEventsByAuthority = map[string]int64{}
		c.tapeOpsByKind = map[string]int64{}
	}
	c.tapeEventsByAuthority[authority]++
	for _, op := range ops {
		c.tapeOpsByKind[string(op.Kind)]++
	}
	c.mu.Unlock()
}

func (c *telemetryCounters) recordSealLatency(d time.Duration) {
	n := d.Nanoseconds()
	c.sealLatencyNanosTotal.Add(n)
	for {
		cur := c.sealLatencyNanosMax.Load()
		if n <= cur || c.sealLatencyNanosMax.CompareAndSwap(cur, n) {
			return
		}
	}
}

// BatchingStats is the sealer half of TelemetryStats.
type BatchingStats struct {
	// Enabled is the flag as the sealer sees it now; WindowMS is W.
	Enabled     bool `json:"enabled"`
	Synchronous bool `json:"synchronous"`
	WindowMS    int  `json:"window_ms"`
	// QueueDepth is pending plus in-flight ops right now.
	QueueDepth int `json:"queue_depth"`

	RecordsEnqueued int64 `json:"records_enqueued"`
	BatchesSealed   int64 `json:"batches_sealed"`
	OpsSealed       int64 `json:"ops_sealed"`
	// SealedOpsPerBatch is the realised batch size histogram (bucket label to
	// count); MeanOpsPerBatch is its mean, the k of design section 7.
	SealedOpsPerBatch map[string]int64 `json:"sealed_ops_per_batch"`
	MeanOpsPerBatch   float64          `json:"mean_ops_per_batch"`
	SealReasons       map[string]int64 `json:"seal_reasons"`

	BarrierWaits       int64 `json:"barrier_waits"`
	ClassViolations    int64 `json:"telemetry_class_violation"`
	RecordsDropped     int64 `json:"records_dropped"`
	PoisonDropped      int64 `json:"poison_dropped"`
	SealFailures       int64 `json:"seal_failures"`
	UnknownOutcomeLoss int64 `json:"unknown_outcome_records_lost"`
	SplitToSingles     int64 `json:"split_to_singles"`
	CapacityFlushes    int64 `json:"capacity_flushes"`
	BootWatermarks     int64 `json:"boot_watermark_writes"`

	SealLatencyMeanMS float64 `json:"seal_latency_mean_ms"`
	SealLatencyMaxMS  float64 `json:"seal_latency_max_ms"`
}

// TelemetryStats is the batching instrumentation snapshot served on the
// guest observability endpoint. Phase 0 reads it to price batching, with the
// flag off: t of design section 7 is EventsAppendedBatchable / TapeEvents
// (each standalone AppendEvent is one tape event), and OpsPerTapeEvent is the
// baseline batch size. With the flag on the sealer counters give the realised
// k. It is read-only; nothing branches on it.
type TelemetryStats struct {
	EventsAppendedByKind map[string]int64 `json:"events_appended_by_kind"`
	// EventPayloadBytesByKind is the summed payload size of those appends;
	// MeanEventPayloadBytesByKind is that sum over the count, the mean line
	// size the trace log's sizing needs (R7 S0). Counted at the same place as
	// EventsAppendedByKind, so the two always divide.
	EventPayloadBytesByKind     map[string]int64   `json:"event_payload_bytes_by_kind"`
	MeanEventPayloadBytesByKind map[string]float64 `json:"mean_event_payload_bytes_by_kind"`
	// EventsBySink is sink name ("tape", "trace_log", "dropped") to kind to the
	// number of events that reached it. All three sinks are always present.
	EventsBySink               map[string]map[string]int64 `json:"events_by_sink"`
	EventsAppendedBatchable    int64                       `json:"events_appended_batchable"`
	EventsAppendedAcknowledged int64                       `json:"events_appended_acknowledged"`
	// BatchableShare is batchable / all standalone AppendEvent calls.
	BatchableShare float64 `json:"batchable_share"`

	// Tape events and ops as the appender accepted them, whatever wrote them.
	TapeEvents            int64            `json:"tape_events"`
	TapeOps               int64            `json:"tape_ops"`
	OpsPerTapeEvent       map[string]int64 `json:"ops_per_tape_event"`
	TapeEventsByAuthority map[string]int64 `json:"tape_events_by_authority"`
	TapeOpsByKind         map[string]int64 `json:"tape_ops_by_kind"`

	ReadsConsistent int64 `json:"reads_consistent"`
	ReadsRelaxed    int64 `json:"reads_relaxed"`

	Batching BatchingStats `json:"batching"`
}

func copyCounts(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// TelemetryStats returns a snapshot of the batching instrumentation. It is
// safe to call at any time and does not touch the tape.
func (s *Store) TelemetryStats() TelemetryStats {
	c := &s.telemetryStats
	c.mu.Lock()
	out := TelemetryStats{
		EventsAppendedByKind:    copyCounts(c.appendedByKind),
		EventPayloadBytesByKind: copyCounts(c.payloadBytesByKind),
		TapeEventsByAuthority:   copyCounts(c.tapeEventsByAuthority),
		TapeOpsByKind:           copyCounts(c.tapeOpsByKind),
		EventsBySink:            map[string]map[string]int64{},
	}
	out.MeanEventPayloadBytesByKind = make(map[string]float64, len(c.payloadBytesByKind))
	for kind, bytes := range c.payloadBytesByKind {
		if n := c.appendedByKind[kind]; n > 0 {
			out.MeanEventPayloadBytesByKind[kind] = float64(bytes) / float64(n)
		}
	}
	for sink := range c.sinkByKind {
		out.EventsBySink[eventSinkNames[sink]] = copyCounts(c.sinkByKind[sink])
	}
	c.mu.Unlock()
	out.EventsAppendedBatchable = c.appendedBatchable.Load()
	out.EventsAppendedAcknowledged = c.appendedAcknowledged.Load()
	if total := out.EventsAppendedBatchable + out.EventsAppendedAcknowledged; total > 0 {
		out.BatchableShare = float64(out.EventsAppendedBatchable) / float64(total)
	}
	out.TapeEvents = c.tapeEvents.Load()
	out.TapeOps = c.tapeOps.Load()
	out.OpsPerTapeEvent = map[string]int64{}
	for i := range c.tapeOpsHist {
		if n := c.tapeOpsHist[i].Load(); n > 0 {
			out.OpsPerTapeEvent[opsBucketLabel(i)] = n
		}
	}
	out.ReadsConsistent = c.readsConsistent.Load()
	out.ReadsRelaxed = c.readsRelaxed.Load()

	b := &out.Batching
	b.SealedOpsPerBatch = map[string]int64{}
	b.SealReasons = map[string]int64{}
	if sl := s.telemetry.Load(); sl != nil {
		b.Enabled, b.Synchronous, b.WindowMS, b.QueueDepth = sl.snapshotState()
	}
	b.RecordsEnqueued = c.recordsEnqueued.Load()
	b.BatchesSealed = c.batchesSealed.Load()
	b.OpsSealed = c.opsSealed.Load()
	for i := range c.sealedOpsHist {
		if n := c.sealedOpsHist[i].Load(); n > 0 {
			b.SealedOpsPerBatch[opsBucketLabel(i)] = n
		}
	}
	if b.BatchesSealed > 0 {
		b.MeanOpsPerBatch = float64(b.OpsSealed) / float64(b.BatchesSealed)
		b.SealLatencyMeanMS = float64(c.sealLatencyNanosTotal.Load()) / float64(b.BatchesSealed) / 1e6
	}
	b.SealLatencyMaxMS = float64(c.sealLatencyNanosMax.Load()) / 1e6
	for i := range c.sealReasonCount {
		if n := c.sealReasonCount[i].Load(); n > 0 {
			b.SealReasons[sealReasonNames[i]] = n
		}
	}
	b.BarrierWaits = c.barrierWaits.Load()
	b.ClassViolations = c.classViolations.Load()
	b.RecordsDropped = c.recordsDropped.Load()
	b.PoisonDropped = c.poisonDropped.Load()
	b.SealFailures = c.sealFailures.Load()
	b.UnknownOutcomeLoss = c.unknownOutcomeLoss.Load()
	b.SplitToSingles = c.splitToSingles.Load()
	b.CapacityFlushes = c.capacityFlushes.Load()
	b.BootWatermarks = c.watermarkWrites.Load()
	return out
}
