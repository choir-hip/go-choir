package trace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func seedChain(t *testing.T, s *SQLStore) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	events := []struct {
		id        string
		kind      string
		parent    string
		offsetMin int
	}{
		{"root", "tool.invoked", "", 0},
		{"mid", "tool.result", "root", 1},
		{"leaf", "tool.invoked", "mid", 2},
	}
	for _, e := range events {
		ev := sampleEvent(e.id, "run-1", e.kind, base.Add(time.Duration(e.offsetMin)*time.Minute))
		ev.ParentID = e.parent
		if err := s.Append(ctx, &ev); err != nil {
			t.Fatalf("Append %s: %v", e.id, err)
		}
	}
}

func TestParentChainWalksRootFirst(t *testing.T) {
	s := newTestStore(t)
	seedChain(t, s)
	q := NewQueries(s)
	ctx := context.Background()

	chain, err := q.ParentChain(ctx, "leaf", 0)
	if err != nil {
		t.Fatalf("ParentChain: %v", err)
	}
	if len(chain) != 3 {
		t.Fatalf("expected 3 events, got %d", len(chain))
	}
	if chain[0].ID != "root" || chain[1].ID != "mid" || chain[2].ID != "leaf" {
		t.Fatalf("chain order mismatch: %s %s %s", chain[0].ID, chain[1].ID, chain[2].ID)
	}
}

func TestParentChainNoParentReturnsSingle(t *testing.T) {
	s := newTestStore(t)
	seedChain(t, s)
	q := NewQueries(s)
	ctx := context.Background()

	chain, err := q.ParentChain(ctx, "root", 0)
	if err != nil {
		t.Fatalf("ParentChain: %v", err)
	}
	if len(chain) != 1 || chain[0].ID != "root" {
		t.Fatalf("expected single root event, got %+v", chain)
	}
}

func TestParentChainNotFound(t *testing.T) {
	s := newTestStore(t)
	q := NewQueries(s)
	ctx := context.Background()

	_, err := q.ParentChain(ctx, "missing", 0)
	if err == nil {
		t.Fatalf("expected error for missing event")
	}
}

func TestParentChainCycleDetected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Now().UTC()
	a := sampleEvent("a", "run-1", "tool.invoked", base)
	a.ParentID = "b"
	b := sampleEvent("b", "run-1", "tool.invoked", base.Add(time.Second))
	b.ParentID = "a"
	if err := s.Append(ctx, &a); err != nil {
		t.Fatalf("Append a: %v", err)
	}
	if err := s.Append(ctx, &b); err != nil {
		t.Fatalf("Append b: %v", err)
	}
	q := NewQueries(s)
	_, err := q.ParentChain(ctx, "a", 64)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestParentChainMissingParentStopsGracefully(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Now().UTC()
	ev := sampleEvent("orphan", "run-1", "tool.invoked", base)
	ev.ParentID = "ghost"
	if err := s.Append(ctx, &ev); err != nil {
		t.Fatalf("Append: %v", err)
	}
	q := NewQueries(s)
	chain, err := q.ParentChain(ctx, "orphan", 0)
	if err != nil {
		t.Fatalf("ParentChain: %v", err)
	}
	if len(chain) != 1 || chain[0].ID != "orphan" {
		t.Fatalf("expected single event with missing parent, got %+v", chain)
	}
}

func TestHTTPHandlerListByRun(t *testing.T) {
	s := newTestStore(t)
	seedChain(t, s)
	h := NewHTTPHandler(s, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/trace/events?run_id=run-1&owner_id=user-alice&limit=10", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rec.Code, rec.Body.String())
	}
	var resp traceEventListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(resp.Events))
	}
}

func TestHTTPHandlerListRequiresRunID(t *testing.T) {
	s := newTestStore(t)
	h := NewHTTPHandler(s, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/trace/events?owner_id=user-alice", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHTTPHandlerListRequiresAuth(t *testing.T) {
	s := newTestStore(t)
	h := NewHTTPHandler(s, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/trace/events?run_id=run-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHTTPHandlerListOwnerScopedNotFound(t *testing.T) {
	s := newTestStore(t)
	seedChain(t, s)
	// Bob asks for alice's run; the run's events are owned by alice.
	h := NewHTTPHandler(s, func(*http.Request) string { return "user-bob" })

	req := httptest.NewRequest(http.MethodGet, "/api/trace/events?run_id=run-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access, got %d", rec.Code)
	}
}

func TestHTTPHandlerListByRunFiltersMixedOwnerEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	alice := sampleEvent("alice-root", "run-mixed", "tool.invoked", base)
	alice.Seq = 1
	if err := s.Append(ctx, &alice); err != nil {
		t.Fatalf("Append alice: %v", err)
	}
	bob := sampleEvent("bob-leaf", "run-mixed", "tool.result", base.Add(time.Minute))
	bob.OwnerID = "user-bob"
	bob.Seq = 2
	if err := s.Append(ctx, &bob); err != nil {
		t.Fatalf("Append bob: %v", err)
	}

	h := NewHTTPHandler(s, func(*http.Request) string { return "user-alice" })
	req := httptest.NewRequest(http.MethodGet, "/api/trace/events?run_id=run-mixed", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rec.Code, rec.Body.String())
	}
	var resp traceEventListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Events) != 1 || resp.Events[0].ID != "alice-root" {
		t.Fatalf("mixed owner list leaked or omitted events: %+v", resp.Events)
	}
}

func TestHTTPHandlerSingleWithChain(t *testing.T) {
	s := newTestStore(t)
	seedChain(t, s)
	h := NewHTTPHandler(s, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/trace/events/leaf?owner_id=user-alice", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rec.Code, rec.Body.String())
	}
	var resp traceEventDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Event == nil || resp.Event.ID != "leaf" {
		t.Fatalf("event mismatch: %+v", resp.Event)
	}
	if len(resp.ParentChain) != 3 {
		t.Fatalf("expected 3 chain events, got %d", len(resp.ParentChain))
	}
	if resp.ParentChain[0].ID != "root" || resp.ParentChain[2].ID != "leaf" {
		t.Fatalf("chain order mismatch: %+v", resp.ParentChain)
	}
}

func TestHTTPHandlerSingleParentChainOmitsCrossOwnerParent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	parent := sampleEvent("bob-parent", "run-cross-parent", "tool.invoked", base)
	parent.OwnerID = "user-bob"
	if err := s.Append(ctx, &parent); err != nil {
		t.Fatalf("Append parent: %v", err)
	}
	child := sampleEvent("alice-child", "run-cross-parent", "tool.result", base.Add(time.Minute))
	child.ParentID = parent.ID
	if err := s.Append(ctx, &child); err != nil {
		t.Fatalf("Append child: %v", err)
	}

	h := NewHTTPHandler(s, func(*http.Request) string { return "user-alice" })
	req := httptest.NewRequest(http.MethodGet, "/api/trace/events/alice-child", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rec.Code, rec.Body.String())
	}
	var resp traceEventDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.ParentChain) != 1 || resp.ParentChain[0].ID != "alice-child" {
		t.Fatalf("cross-owner parent leaked in chain: %+v", resp.ParentChain)
	}
}

func TestHTTPHandlerSingleNotFound(t *testing.T) {
	s := newTestStore(t)
	h := NewHTTPHandler(s, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/trace/events/ghost?owner_id=user-alice", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHTTPHandlerSingleOwnerScopedNotFound(t *testing.T) {
	s := newTestStore(t)
	seedChain(t, s)
	h := NewHTTPHandler(s, func(*http.Request) string { return "user-bob" })

	req := httptest.NewRequest(http.MethodGet, "/api/trace/events/leaf", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access, got %d", rec.Code)
	}
}

func TestHTTPHandlerRejectsNonGet(t *testing.T) {
	s := newTestStore(t)
	h := NewHTTPHandler(s, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/trace/events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHTTPHandlerLimitClampedToMax(t *testing.T) {
	s := newTestStore(t)
	seedChain(t, s)
	h := NewHTTPHandler(s, nil)
	h.maxLimit = 2

	req := httptest.NewRequest(http.MethodGet, "/api/trace/events?run_id=run-1&owner_id=user-alice&limit=100", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var resp traceEventListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("expected clamped to 2, got %d", len(resp.Events))
	}
}
