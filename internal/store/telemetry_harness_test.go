package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// recordingTape is a fake platform for the telemetry tests. It pins and
// CASes like liveTapePinner / liveTapeCAS and records, in CAS order, every
// event the appender sent together with the payload it pinned. The
// injection hooks make a platform outage or a refusal scriptable.
type recordingTape struct {
	mu       sync.Mutex
	signer   computerevent.SigningKey
	payloads map[string][]byte
	records  []tapeRecord
	// pinErr, when set and non-nil for a payload, fails the pin (before the
	// CAS, so nothing is appended).
	pinErr func(payload []byte) error
	// casErr, when set and non-nil for a request, fails the CAS after the pin.
	casErr func(request computerevent.CASRequest) error
	pins   int
	// leaseDigests are the payload digests of projection batches that carry
	// only the stream_seq lease object. The lease is store bookkeeping, not
	// part of the workload a test scripts: its batches are recorded apart
	// (leaseRecords, leaseSnapshot) so the shape of the workload's tape stays
	// the one the telemetry tests were written against.
	leaseDigests map[string]bool
	leaseRecords []tapeRecord
}

// tapeRecord is one event the platform accepted.
type tapeRecord struct {
	Sequence       uint64
	Kind           computerevent.EventKind
	Authority      string
	IdempotencyKey string
	ActorProfile   string
	PrivacyClass   string
	PayloadDigest  string
}

func newRecordingTape(t *testing.T) *recordingTape {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &recordingTape{
		signer:   computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey},
		payloads: map[string][]byte{},
	}
}

type recordingPinner struct {
	liveTapePinner
	tape *recordingTape
}

func (p recordingPinner) PinNonPrivatePayload(ctx context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (computerevent.PinResult, error) {
	p.tape.mu.Lock()
	hook := p.tape.pinErr
	p.tape.mu.Unlock()
	if hook != nil {
		if err := hook(payload); err != nil {
			return computerevent.PinResult{}, err
		}
	}
	result, err := p.liveTapePinner.PinNonPrivatePayload(ctx, computerID, payload, mediaType, privacyClass, pinIntentCommitment)
	if err == nil {
		p.tape.mu.Lock()
		p.tape.payloads[result.ArtifactDigest] = append([]byte(nil), payload...)
		p.tape.pins++
		if isLeaseOnlyBatch(payload) {
			if p.tape.leaseDigests == nil {
				p.tape.leaseDigests = map[string]bool{}
			}
			p.tape.leaseDigests[result.ArtifactDigest] = true
		}
		p.tape.mu.Unlock()
	}
	return result, err
}

type recordingCAS struct {
	liveTapeCAS
	tape *recordingTape
}

func (c recordingCAS) CompareAndSwap(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	c.tape.mu.Lock()
	hook := c.tape.casErr
	c.tape.mu.Unlock()
	if hook != nil {
		if err := hook(request); err != nil {
			return computerevent.Receipt{}, err
		}
	}
	receipt, err := c.liveTapeCAS.CompareAndSwap(ctx, request)
	if err == nil {
		e := request.Event
		c.tape.mu.Lock()
		record := tapeRecord{
			Sequence: e.Sequence, Kind: e.EventKind, Authority: e.AuthorityRef,
			IdempotencyKey: e.IdempotencyKey, ActorProfile: e.ActorProfile, PrivacyClass: e.PrivacyClass,
			PayloadDigest: e.PayloadCommitment,
		}
		if c.tape.leaseDigests[e.PayloadCommitment] {
			c.tape.leaseRecords = append(c.tape.leaseRecords, record)
		} else {
			c.tape.records = append(c.tape.records, record)
		}
		c.tape.mu.Unlock()
	}
	return receipt, err
}

// snapshot returns the accepted events in CAS order.
func (r *recordingTape) snapshot() []tapeRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]tapeRecord(nil), r.records...)
}

// leaseSnapshot returns the accepted lease-only batches in CAS order.
func (r *recordingTape) leaseSnapshot() []tapeRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]tapeRecord(nil), r.leaseRecords...)
}

// isLeaseOnlyBatch reports whether a pinned payload is a projection batch of
// the stream_seq lease object alone.
func isLeaseOnlyBatch(payload []byte) bool {
	batch, err := computerevent.DecodeProjectionBatch(payload)
	if err != nil || len(batch.Ops) == 0 {
		return false
	}
	for _, op := range batch.Ops {
		if !strings.Contains(op.CanonicalID, "choir.stream_seq_lease") {
			return false
		}
	}
	return true
}

// batchOps decodes the projection batch an event pinned.
func (r *recordingTape) batchOps(t *testing.T, rec tapeRecord) []computerevent.ProjectionOp {
	t.Helper()
	r.mu.Lock()
	payload, ok := r.payloads[rec.PayloadDigest]
	r.mu.Unlock()
	if !ok {
		t.Fatalf("no pinned payload for event sequence %d", rec.Sequence)
	}
	batch, err := computerevent.DecodeProjectionBatch(payload)
	if err != nil {
		t.Fatalf("decode projection batch of sequence %d: %v", rec.Sequence, err)
	}
	return batch.Ops
}

// tapeHarness is a store bound to a recording tape, with the files needed to
// reopen it (a "reboot") under the same platform.
type tapeHarness struct {
	t          *testing.T
	path       string
	computerID string
	tape       *recordingTape
	store      *Store
	appender   *computerevent.ComputerEventAppender
	// dryRunHook, when set, is consulted by the projection's dry run: a
	// non-nil error refuses the batch before anything is pinned.
	dryRunHook func(batch *computerevent.ProjectionBatch) error
	// platformSkew, when set and true, makes the platform report a head one
	// event ahead of the local projection (stream_seq lease tests).
	platformSkew *atomic.Bool
	// finalizeFail, when set and true, makes the local projection fail every
	// finalize after the platform accepted the event: a crash between the CAS
	// and the local commit.
	finalizeFail *atomic.Bool
}

// hookedProjection is the store as the appender's projection, with a scriptable
// dry-run refusal. Every other method is the store's own.
type hookedProjection struct {
	*Store
	hook func() func(batch *computerevent.ProjectionBatch) error
}

func (p hookedProjection) DryRunProjectionBatch(ctx context.Context, computerID string, batch *computerevent.ProjectionBatch) error {
	if fn := p.hook(); fn != nil {
		if err := fn(batch); err != nil {
			return err
		}
	}
	return p.Store.DryRunProjectionBatch(ctx, computerID, batch)
}

func newTapeHarness(t *testing.T) *tapeHarness {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	h := &tapeHarness{t: t, path: path, computerID: "computer-telemetry", tape: newRecordingTape(t)}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h.store = s
	prepareGenesis(t, s, h.computerID, "genesis-telemetry")
	h.bind()
	t.Cleanup(func() { _ = h.store.Close() })
	return h
}

// bind attaches a fresh appender over the current store to the shared tape.
func (h *tapeHarness) bind() {
	h.t.Helper()
	var cas computerevent.HeadCAS = recordingCAS{liveTapeCAS: liveTapeCAS{signer: h.tape.signer, projection: h.store}, tape: h.tape}
	if h.platformSkew != nil {
		cas = skewedCAS{HeadCAS: cas, skew: h.platformSkew}
	}
	var projection computerevent.ProjectionStore = hookedProjection{Store: h.store, hook: func() func(*computerevent.ProjectionBatch) error { return h.dryRunHook }}
	if h.finalizeFail != nil {
		projection = finalizeFailingProjection{ProjectionStore: projection, store: h.store, fail: h.finalizeFail}
	}
	appender, err := computerevent.NewComputerEventAppender(
		h.computerID,
		recordingPinner{liveTapePinner: liveTapePinner{signer: h.tape.signer}, tape: h.tape},
		projection,
		cas,
		liveTapeVerifier{},
	)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := h.store.BindProjectionTape(h.computerID, appender); err != nil {
		h.t.Fatal(err)
	}
	h.appender = appender
}

// reopen closes the store and opens the same files again under the same
// platform: a restart. Whatever the old process had only in memory is gone.
func (h *tapeHarness) reopen() {
	h.t.Helper()
	h.abandon()
	s, err := Open(h.path)
	if err != nil {
		h.t.Fatal(err)
	}
	h.store = s
	h.bind()
}

func (h *tapeHarness) event(id, run, owner string, kind types.EventKind, at time.Time) *types.EventRecord {
	return &types.EventRecord{
		EventID: id, RunID: run, OwnerID: owner, TrajectoryID: "trajectory-" + run, Timestamp: at,
		Kind: kind, Payload: json.RawMessage(fmt.Sprintf(`{"id":%q}`, id)),
	}
}

var telemetryBase = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

// abandon drops the store the way a crash does: whatever the sealer still
// holds is discarded, nothing is flushed on purpose.
func (h *tapeHarness) abandon() {
	h.t.Helper()
	if sl := h.store.telemetry.Load(); sl != nil {
		sl.discardForCrash()
	}
	_ = h.store.Close()
}

// enableBatching turns the sealer on with a test window (not clamped, so a
// test can use a window well under the production floor of 250 ms).
func (h *tapeHarness) enableBatching(window time.Duration) *telemetrySealer {
	h.t.Helper()
	if err := h.store.ConfigureTelemetryBatching(TelemetryBatchConfig{Enabled: true, Window: window}); err != nil {
		h.t.Fatal(err)
	}
	sl := h.store.telemetry.Load()
	sl.setWindow(window)
	return sl
}

// shapeEntry is one tape event reduced to what the ordering tests read: its
// authority and the events it carries, each as "kind:event_id" (run memory as
// "run_memory:entry_id").
type shapeEntry struct {
	Authority string
	Items     []string
}

func (h *tapeHarness) shape() []shapeEntry {
	h.t.Helper()
	var out []shapeEntry
	for _, rec := range h.tape.snapshot() {
		e := shapeEntry{Authority: rec.Authority}
		for _, op := range h.tape.batchOps(h.t, rec) {
			switch op.Kind {
			case computerevent.ProjectionOpObject:
				var obj objectgraph.Object
				if err := json.Unmarshal(op.Body, &obj); err != nil {
					h.t.Fatal(err)
				}
				var ev types.EventRecord
				if err := json.Unmarshal(obj.Body, &ev); err != nil {
					h.t.Fatal(err)
				}
				e.Items = append(e.Items, string(ev.Kind)+":"+ev.EventID)
			case computerevent.ProjectionOpRunMemoryEntry:
				e.Items = append(e.Items, "run_memory:"+op.CanonicalID)
			default:
				e.Items = append(e.Items, string(op.Kind))
			}
		}
		out = append(out, e)
	}
	return out
}

// storedEvents reads the events of one run through the sealing read path.
func (h *tapeHarness) storedEvents(runID string) []types.EventRecord {
	h.t.Helper()
	events, err := h.store.ListEventsAfter(context.Background(), runID, 0, 1000)
	if err != nil {
		h.t.Fatal(err)
	}
	return events
}

func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}
