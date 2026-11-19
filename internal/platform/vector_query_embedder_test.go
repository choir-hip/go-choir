package platform

// Failure list for the query embedder (docs/problems/search-waits-four-seconds-
// behind-the-embed-worker-2026-11-19.md), written before the code:
//
//  F1. A ranked search's query embed goes to the same embedder as the worker's
//      passages, so while the worker holds that embedder the query waits behind
//      it (4 s deadline, vector half "unavailable"). Wanted: with a query
//      embedder set, the query input goes there and the bulk embedder receives
//      no query input, even while the worker is blocked inside the bulk one.
//  F2. With no query embedder set (tests, a developer's machine) the one
//      embedder must still serve both: unchanged behaviour, pinned here.
//  F3. A query embedder that is down or too slow must leave the vector half
//      "unavailable" within the deadline, and the query must NOT fall back to
//      the bulk embedder (that puts it behind the bulk work again).
//  F4. A query longer than a query slot's context makes the server answer
//      "input (N tokens) is larger than the max context size" (or "too large to
//      process"); the search must answer unavailable, never return an error to
//      the caller and never hang, and the ranked search must answer from BM25.
//  F5. Documents (the worker, the import) must keep using the bulk embedder and
//      never the query one.
//
// Race shape avoided: every seam (SetEmbedder, SetQueryEmbedder, the deadline
// variable) is set before any goroutine of the code under test starts. The one
// test that shortens captureVectorDeadline leaves a goroutine behind on
// purpose (searchVectorsBounded stops waiting at the deadline), so its slow
// embedder is a gate: the test releases it and waits until that embed call has
// returned, in a defer that runs before the t.Cleanup that restores the
// variable. The only reader of the variable besides that goroutine's caller is
// the caller itself.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
)

// seenQuery reports whether an embedder was given any query input.
func seenQuery(f *embedder.Fake) bool {
	for _, in := range f.Seen() {
		if strings.Contains(in, "query: ") {
			return true
		}
	}
	return false
}

// gateEmbedder stands for a bulk embedder that is busy with the worker: a call
// blocks until release is closed or its context ends.
type gateEmbedder struct {
	inner   embedder.Embedder
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGateEmbedder(inner embedder.Embedder) *gateEmbedder {
	return &gateEmbedder{inner: inner, entered: make(chan struct{}), release: make(chan struct{})}
}

func (g *gateEmbedder) Health(ctx context.Context) error { return g.inner.Health(ctx) }
func (g *gateEmbedder) Embed(ctx context.Context, in []string) ([][]float32, error) {
	g.once.Do(func() { close(g.entered) })
	select {
	case <-g.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return g.inner.Embed(ctx, in)
}

func TestVectorQueryEmbedGoesToTheQueryEmbedderEvenWhileTheWorkerHoldsTheBulkOne(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.pass(t) // vectors exist, embedded on the plain bulk fake
	f.story(t, "doc-b", 1, "Bridge closes", storyText("bridge"), time.Time{})
	bulkFake := f.emb
	gate := newGateEmbedder(bulkFake)
	queryFake := &embedder.Fake{}
	f.store.SetEmbedder(gate)
	f.store.SetQueryEmbedder(queryFake)

	passDone := make(chan struct{})
	go func() {
		defer close(passDone)
		_, _ = f.svc.RunVectorPass(context.Background())
	}()
	t.Cleanup(func() { // after the pass has stopped; release first so it can
		select {
		case <-gate.release:
		default:
			close(gate.release)
		}
		<-passDone
	})
	select {
	case <-gate.entered:
	case <-time.After(30 * time.Second):
		t.Fatal("the worker never reached the bulk embedder")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	res, err := f.store.SearchVectors(ctx, VectorSearchRequest{Query: "harbor"})
	if err != nil || res.Unavailable != "" || len(res.Stories) == 0 {
		t.Fatalf("search while the worker held the bulk embedder: %+v %v (after %v)", res, err, time.Since(start))
	}
	if !seenQuery(queryFake) {
		t.Fatalf("the query embedder was not asked: %v", queryFake.Seen())
	}
	if seenQuery(bulkFake) {
		t.Fatalf("the bulk embedder was given a query: %v", bulkFake.Seen())
	}
	close(gate.release)
	<-passDone
	if seenQuery(bulkFake) {
		t.Fatalf("the bulk embedder was given a query: %v", bulkFake.Seen())
	}
	// F5: the worker's documents went to the bulk embedder, none to the query one.
	for _, in := range queryFake.Seen() {
		if strings.HasPrefix(in, "title: ") {
			t.Fatalf("the query embedder was given a document: %q", in)
		}
	}
}

func TestVectorQueryEmbedWithNoQueryEmbedderUsesTheOneEmbedder(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.pass(t)
	res, err := f.store.SearchVectors(context.Background(), VectorSearchRequest{Query: "harbor"})
	if err != nil || res.Unavailable != "" || len(res.Stories) == 0 {
		t.Fatalf("one embedder: %+v %v", res, err)
	}
	if !seenQuery(f.emb) {
		t.Fatalf("the one embedder did not serve the query: %v", f.emb.Seen())
	}
	// A nil query embedder set explicitly means the same.
	f.store.SetQueryEmbedder(nil)
	n := f.emb.Calls.Load()
	if _, err := f.store.SearchVectors(context.Background(), VectorSearchRequest{Query: "harbor"}); err != nil || f.emb.Calls.Load() != n+1 {
		t.Fatalf("nil query embedder: calls %d -> %d, err %v", n, f.emb.Calls.Load(), err)
	}
}

func TestVectorQueryEmbedderDownLeavesTheVectorHalfUnavailableAndDoesNotFallBack(t *testing.T) {
	f := newRankFixture(t)
	f.disagree(t)
	bulkFake := f.emb
	queryFake := &embedder.Fake{Override: markerVector}
	f.store.SetQueryEmbedder(queryFake)
	f.pass(t)
	seen := len(bulkFake.Seen())
	queryFake.Down.Store(true)

	start := time.Now()
	resp := f.rank(t, "zorblax")
	if took := time.Since(start); took > captureVectorDeadline {
		t.Fatalf("the search took %v with the query embedder down", took)
	}
	r := resp.Ranked
	if r == nil || r.VectorAnswered || r.VectorUnavailable == "" {
		t.Fatalf("ranking info %+v", r)
	}
	if seenQuery(bulkFake) || len(bulkFake.Seen()) != seen {
		t.Fatalf("the query fell back to the bulk embedder: %v", bulkFake.Seen()[seen:])
	}
}

// slowEmbedder does not look at its context: a query call blocks on release,
// and returned counts the calls that have come back.
type slowEmbedder struct {
	inner    embedder.Embedder
	release  chan struct{}
	entered  chan struct{}
	once     sync.Once
	returned sync.WaitGroup
}

func (e *slowEmbedder) Health(ctx context.Context) error { return e.inner.Health(ctx) }
func (e *slowEmbedder) Embed(ctx context.Context, in []string) ([][]float32, error) {
	e.returned.Add(1)
	defer e.returned.Done()
	e.once.Do(func() { close(e.entered) })
	<-e.release
	return e.inner.Embed(ctx, in)
}

func TestVectorQueryEmbedderThatDoesNotAnswerInTimeIsUnavailableAtTheDeadlineAndNotRetriedOnTheBulkOne(t *testing.T) {
	f := newRankFixture(t)
	f.disagree(t)
	bulkFake := f.emb
	f.pass(t) // documents, on the bulk embedder, before any seam changes
	slow := &slowEmbedder{inner: &embedder.Fake{Override: markerVector}, release: make(chan struct{}), entered: make(chan struct{})}
	f.store.SetQueryEmbedder(slow)
	old := captureVectorDeadline
	captureVectorDeadline = 80 * time.Millisecond
	t.Cleanup(func() { captureVectorDeadline = old })
	// Runs when the test function returns, before the Cleanup above: the call
	// the search left behind is released and has returned before the variable
	// is restored.
	defer func() {
		close(slow.release)
		slow.returned.Wait()
	}()
	seen := len(bulkFake.Seen())
	start := time.Now()
	resp := f.rank(t, "zorblax")
	if took := time.Since(start); took > 400*time.Millisecond {
		t.Fatalf("the search waited %v with an 80 ms deadline", took)
	}
	select {
	case <-slow.entered:
	default:
		t.Fatal("the query embedder was never asked")
	}
	if r := resp.Ranked; r == nil || r.VectorAnswered || !strings.Contains(r.VectorUnavailable, "within") {
		t.Fatalf("ranking info %+v", r)
	}
	if len(bulkFake.Seen()) != seen {
		t.Fatalf("the query fell back to the bulk embedder: %v", bulkFake.Seen()[seen:])
	}
}

func TestVectorQueryLongerThanAQuerySlotIsUnavailableNeverAnErrorOrAHang(t *testing.T) {
	const tooLong = `{"error":{"message":"input (900 tokens) is larger than the max context size (512 tokens). skipping"}}`
	const tooLargeBatch = `{"error":{"message":"input (900 tokens) is too large to process. increase the physical batch size (current batch size: 512)"}}`
	for _, msg := range []string{tooLong, tooLargeBatch} {
		f := newRankFixture(t)
		f.disagree(t)
		f.pass(t)
		bulkSeen := len(f.emb.Seen())
		long := strings.Repeat("zorblax ", 200)
		query := httpEmbedder(t, func(in []string) (int, string) {
			for _, x := range in {
				if len(x) > 800 {
					return 500, msg
				}
			}
			return 200, ""
		})
		f.store.SetQueryEmbedder(query)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := f.store.SearchVectors(ctx, VectorSearchRequest{Query: long})
		cancel()
		if err != nil || res.Unavailable == "" || len(res.Stories) != 0 || len(res.Captures) != 0 {
			t.Fatalf("%s: long query: %+v %v", msg, res, err)
		}
		// The ranked search answers from BM25 and says so.
		resp := f.rank(t, long)
		if r := resp.Ranked; r == nil || r.VectorAnswered || r.VectorUnavailable == "" {
			t.Fatalf("%s: ranking info %+v", msg, r)
		}
		// A short query still answers on the same unit.
		if res, err := f.store.SearchVectors(context.Background(), VectorSearchRequest{Query: "zorblax"}); err != nil || res.Unavailable != "" {
			t.Fatalf("%s: short query: %+v %v", msg, res, err)
		}
		if len(f.emb.Seen()) != bulkSeen {
			t.Fatalf("%s: the long query was retried on the bulk embedder: %v", msg, f.emb.Seen()[bulkSeen:])
		}
	}
}
