package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Failure modes (problem record failure mode 12 and Evidence 5): a
// retrieval call is not counted; a miss is counted as an error; the journal
// gets a line per primary-key read; /health does not carry the counters.

func TestRetrievalMetricsCountTimeAndExposeOnHealth(t *testing.T) {
	store, root := openTestPlatformStore(t)
	var mu sync.Mutex
	var lines []string
	store.retrieval.logf = func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, fmt.Sprintf(format, args...))
	}
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	if _, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: objectgraph.WebCaptureObjectKind, OwnerID: "o", ComputerID: "c", Limit: 5}); err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if _, err := og.GetObject(ctx, "obj:choir.web_capture:bm9uZQ:missing"); err == nil {
		t.Fatal("GetObject of a missing id: want ErrNotFound")
	}
	if _, err := og.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", "/pub/none"); err == nil {
		t.Fatal("GetObjectByMetadata of a missing route: want ErrNotFound")
	}

	snap := store.RetrievalSnapshot()
	list := snap.Ops[retrievalListObjects]
	if list.Calls != 1 || list.Errors != 0 {
		t.Fatalf("list_objects counters = %+v, want 1 call, 0 errors", list)
	}
	if got := snap.Ops[retrievalGetObject]; got.Calls != 1 || got.Errors != 0 {
		t.Fatalf("get_object counters = %+v: a miss is a success, not an error", got)
	}
	if got := snap.Ops[retrievalGetByMetadata]; got.Calls != 1 || got.Errors != 0 {
		t.Fatalf("get_by_metadata counters = %+v", got)
	}
	var total int64
	for _, n := range list.Buckets {
		total += n
	}
	if total != 1 {
		t.Fatalf("list_objects buckets sum to %d, want 1", total)
	}

	mu.Lock()
	joined := strings.Join(lines, "\n")
	mu.Unlock()
	if !strings.Contains(joined, "corpusd retrieval op=list_objects") {
		t.Fatalf("no journal line for list_objects: %q", joined)
	}
	if strings.Contains(joined, "op=get_object ") {
		t.Fatalf("a fast primary-key read must not write a journal line: %q", joined)
	}

	h := NewHandler(NewService(store, root+"/artifacts", ""))
	rec := httptest.NewRecorder()
	h.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body struct {
		Retrieval *RetrievalSnapshot `json:"retrieval"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if body.Retrieval == nil || body.Retrieval.Ops[retrievalListObjects].Calls != 1 {
		t.Fatalf("health retrieval block = %+v, want list_objects counted", body.Retrieval)
	}
}

func TestRetrievalMetricsQuantilesFromBuckets(t *testing.T) {
	m := &RetrievalMetrics{logf: func(string, ...any) {}}
	for i := 0; i < 90; i++ {
		m.record("op", "", 3*time.Millisecond, 1, nil)
	}
	for i := 0; i < 10; i++ {
		m.record("op", "", 700*time.Millisecond, 1, nil)
	}
	got := m.Snapshot().Ops["op"]
	if got.P50MS != 5 {
		t.Fatalf("p50 bound = %v, want 5", got.P50MS)
	}
	if got.P95MS != 1000 {
		t.Fatalf("p95 bound = %v, want 1000", got.P95MS)
	}
	if got.Buckets["le_5ms"] != 90 || got.Buckets["le_1000ms"] != 10 {
		t.Fatalf("buckets = %v", got.Buckets)
	}
}
