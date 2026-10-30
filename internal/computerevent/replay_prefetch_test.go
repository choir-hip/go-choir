package computerevent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Failure modes of the replay prefetch: payloads still fetched one at a time;
// batches applied out of event order; a prefetched payload served without its
// digest check; a payload the prefetch could not fetch dropped instead of
// fetched in line; a page's prefetched payloads left behind after the replay.

type pagedMemoryCAS struct{ *memoryCAS }

// orderedBatchProjection advances its head on every event and records the
// batches it applies, in order.
type orderedBatchProjection struct {
	memoryProjection
	applied []string
}

func (p *orderedBatchProjection) FinalizeBatch(ctx context.Context, computerID, digest string, receipt Receipt, batch *ProjectionBatch) error {
	if batch != nil {
		p.applied = append(p.applied, batch.EventID)
	}
	return p.memoryProjection.Finalize(ctx, computerID, digest, receipt)
}

func (c pagedMemoryCAS) EventsPage(ctx context.Context, computerID string, after uint64, size int) ([]DurableEvent, error) {
	records, err := c.Events(ctx, computerID, after)
	if len(records) > size {
		records = records[:size]
	}
	return records, err
}

type slowPayloadReader struct {
	payloads          map[string][]byte
	corrupt           string
	failOnce          sync.Map
	inFlight, maxSeen atomic.Int32
	fetches           atomic.Int32
}

func (r *slowPayloadReader) FetchPayload(_ context.Context, _, digest string) ([]byte, error) {
	r.fetches.Add(1)
	n := r.inFlight.Add(1)
	defer r.inFlight.Add(-1)
	for {
		seen := r.maxSeen.Load()
		if n <= seen || r.maxSeen.CompareAndSwap(seen, n) {
			break
		}
	}
	time.Sleep(5 * time.Millisecond)
	if _, failed := r.failOnce.LoadOrStore(digest, true); !failed && digest == r.corrupt {
		return nil, errors.New("injected first-fetch failure")
	}
	raw, ok := r.payloads[digest]
	if !ok {
		return nil, errors.New("payload absent")
	}
	if digest == r.corrupt && r.payloads[""] != nil {
		return r.payloads[""], nil
	}
	return raw, nil
}

func appendTestBatches(t *testing.T, count int) (*memoryCAS, memoryPinner, ReceiptVerifier, map[string][]byte, []string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	cas := &memoryCAS{signer: signer}
	pinner := memoryPinner{signer: signer}
	verifier := EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}}
	appender, err := NewComputerEventAppender(testComputerID, pinner, &orderedBatchProjection{}, cas, verifier)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(ctx, genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	payloads := map[string][]byte{}
	var order []string
	for range count {
		event := testEvent(t, nil, EventProjectionBatchRecorded)
		event.PrivacyClass = "owner"
		payload := testBatchPayloadFor(t, event.EventID)
		_, digest, err := appender.AppendNewPayload(ctx, event, TransitionInput{}, payload, ProjectionBatchMediaType, "owner")
		if err != nil {
			t.Fatal(err)
		}
		payloads[digest] = payload
		order = append(order, event.EventID)
	}
	return cas, pinner, verifier, payloads, order
}

func TestReplayFetchesAPagesPayloadsConcurrentlyAndAppliesThemInOrder(t *testing.T) {
	cas, pinner, verifier, payloads, order := appendTestBatches(t, 40)
	reader := &slowPayloadReader{payloads: payloads}
	for digest := range payloads {
		reader.corrupt = digest // its first fetch fails: the replay must fetch it again in line
		break
	}
	projection := &orderedBatchProjection{}
	replayer, err := NewComputerEventAppender(testComputerID, pinner, projection, cas, verifier)
	if err != nil {
		t.Fatal(err)
	}
	replayer.SetPayloadResolver(reader, nil)
	if err := replayer.Reconstruct(context.Background(), pagedMemoryCAS{cas}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(projection.applied, order) {
		t.Fatalf("applied %d batches out of order or incomplete, want %d in append order", len(projection.applied), len(order))
	}
	if got := reader.maxSeen.Load(); got < 2 {
		t.Fatalf("at most %d payload fetches were in flight, want a concurrent prefetch", got)
	}
	if got := reader.fetches.Load(); got != int32(len(payloads))+1 {
		t.Fatalf("fetched payloads %d times, want %d (one each, plus one in-line retry)", got, len(payloads)+1)
	}
	if replayer.replayPrefetched != nil {
		t.Fatalf("replay left %d prefetched payloads behind", len(replayer.replayPrefetched))
	}
}

func TestReplayRefusesAPrefetchedPayloadWhoseDigestDisagrees(t *testing.T) {
	cas, pinner, verifier, payloads, _ := appendTestBatches(t, 4)
	reader := &slowPayloadReader{payloads: payloads}
	for digest := range payloads {
		reader.corrupt = digest
		break
	}
	reader.failOnce.Store(reader.corrupt, true) // serve the wrong bytes from the first fetch on
	payloads[""] = []byte(`{"not":"the pinned payload"}`)
	replayer, err := NewComputerEventAppender(testComputerID, pinner, &orderedBatchProjection{}, cas, verifier)
	if err != nil {
		t.Fatal(err)
	}
	replayer.SetPayloadResolver(reader, nil)
	if err := replayer.Reconstruct(context.Background(), pagedMemoryCAS{cas}); !errors.Is(err, ErrPayloadDigestMismatch) {
		t.Fatalf("replay with a corrupt payload returned %v, want a digest mismatch", err)
	}
}
