package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Test helpers for the stream_seq lease tests.

// leaseCanonicalID is the lease object's canonical id: kind, owner and the
// stable suffix of the identity key.
func leaseCanonicalID(t testing.TB) string {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.KindStreamSeqLease, "__system__", objectgraph.StableSuffixFromKey("stream_seq_lease"))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// readLeaseQuiet reads the durable row without a *testing.T, for goroutines.
func readLeaseQuiet(s *Store) (leaseRow, bool) {
	row, found, err := s.readLease(context.Background())
	if err != nil {
		return leaseRow{}, false
	}
	return row, found
}

// maxStreamSeqForTest is the allocator's in-memory high-water mark.
func (s *Store) maxStreamSeqForTest() int64 {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	return s.maxStreamSeq
}

// skewedCAS reports a platform head one event ahead of the local projection
// while skew is true: the state of a boot before replay has caught up.
type skewedCAS struct {
	computerevent.HeadCAS
	skew *atomic.Bool
}

func (c skewedCAS) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	head, err := c.HeadCAS.Head(ctx, computerID)
	if err != nil || head == nil || !c.skew.Load() {
		return head, err
	}
	ahead := *head
	ahead.Sequence++
	ahead.CanonicalEventHead = strings.Repeat("e", 64)
	return &ahead, nil
}

func (h *tapeHarness) setPlatformHeadSkew(skew *atomic.Bool) {
	h.t.Helper()
	h.platformSkew = skew
	h.bind()
}

// finalizeFailingProjection fails the local commit of a lease batch the
// platform already accepted, while fail is true: a crash between the CAS and
// the local commit. Other batches commit normally.
type finalizeFailingProjection struct {
	computerevent.ProjectionStore
	store *Store
	fail  *atomic.Bool
}

func (p finalizeFailingProjection) Finalize(ctx context.Context, computerID, digest string, receipt computerevent.Receipt) error {
	return p.ProjectionStore.Finalize(ctx, computerID, digest, receipt)
}

func (p finalizeFailingProjection) FinalizeBatch(ctx context.Context, computerID, digest string, receipt computerevent.Receipt, batch *computerevent.ProjectionBatch) error {
	if p.fail.Load() && batch != nil {
		for _, op := range batch.Ops {
			if strings.Contains(op.CanonicalID, "choir.stream_seq_lease") {
				return errors.New("injected crash between the CAS and the local commit")
			}
		}
	}
	return p.store.FinalizeBatch(ctx, computerID, digest, receipt, batch)
}

// DryRunProjectionBatch keeps the wrapped dry run.
func (p finalizeFailingProjection) DryRunProjectionBatch(ctx context.Context, computerID string, batch *computerevent.ProjectionBatch) error {
	if runner, ok := p.ProjectionStore.(computerevent.BatchDryRunner); ok {
		return runner.DryRunProjectionBatch(ctx, computerID, batch)
	}
	return p.store.DryRunProjectionBatch(ctx, computerID, batch)
}

// tapePayloads serves the harness tape's pinned payloads to a replay.
type tapePayloads struct{ tape *recordingTape }

func (r tapePayloads) FetchPayload(_ context.Context, _ string, digest string) ([]byte, error) {
	r.tape.mu.Lock()
	defer r.tape.mu.Unlock()
	if payload, ok := r.tape.payloads[digest]; ok {
		return payload, nil
	}
	return nil, fmt.Errorf("artifact %s is not pinned", digest)
}

func (h *tapeHarness) setFinalizeFail(fail *atomic.Bool) {
	h.t.Helper()
	h.finalizeFail = fail
	h.bind()
}

// waitLeaseReady nudges the seeder (a seeder in its retry backoff starts only
// on an allocation or StartStreamSeqSeed) until the lease is ready, with a
// generous bound. Tests that make a failure condition clear call it instead of
// racing the seeder's doubling backoff against a shortened seeding deadline:
// the deadline is the subject of the test that shortens it, and the backoff
// step can outlast a deadline sized for the first attempt on a loaded runner.
func waitLeaseReady(t testing.TB, s *Store) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		s.StartStreamSeqSeed()
		if s.StreamSeqLeaseStats().State == "ready" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the lease never became ready: %+v", s.StreamSeqLeaseStats())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
