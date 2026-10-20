package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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
		c.tape.records = append(c.tape.records, tapeRecord{
			Sequence: e.Sequence, Kind: e.EventKind, Authority: e.AuthorityRef,
			IdempotencyKey: e.IdempotencyKey, ActorProfile: e.ActorProfile, PrivacyClass: e.PrivacyClass,
			PayloadDigest: e.PayloadCommitment,
		})
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
	appender, err := computerevent.NewComputerEventAppender(
		h.computerID,
		recordingPinner{liveTapePinner: liveTapePinner{signer: h.tape.signer}, tape: h.tape},
		h.store,
		recordingCAS{liveTapeCAS: liveTapeCAS{signer: h.tape.signer, projection: h.store}, tape: h.tape},
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

// abandon drops the store the way a crash does: nothing is flushed on purpose.
func (h *tapeHarness) abandon() {
	h.t.Helper()
	_ = h.store.Close()
}
