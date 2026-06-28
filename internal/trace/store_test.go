package trace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// newTestStore opens a fresh in-memory SQLite trace store for each test.
func newTestStore(t *testing.T) *SQLStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open trace store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// newFileTestStore opens a file-backed SQLite trace store, exercising the same
// DDL path on a persistent backend.
func newFileTestStore(t *testing.T) *SQLStore {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "go-choir-m20-trace-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(path)
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("open file trace store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_ = os.Remove(path)
	})
	return s
}

func sampleEvent(id, runID, kind string, ts time.Time) Event {
	return Event{
		ID:        id,
		RunID:     runID,
		EventType: kind,
		Actor:     "agent-1",
		OwnerID:   "user-alice",
		Payload:   json.RawMessage(`{}`),
		CreatedAt: ts.UTC(),
	}
}

func TestAppendAndGetRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		open func(t *testing.T) *SQLStore
	}{
		{"memory", newTestStore},
		{"file", newFileTestStore},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.open(t)
			ctx := context.Background()
			ts := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
			ev := sampleEvent("evt-1", "run-1", "tool.invoked", ts)
			ev.Tool = "edit_texture"
			ev.Payload = json.RawMessage(`{"tool":"edit_texture","call_id":"c1","args":{"doc_id":"d1"}}`)
			if err := s.Append(ctx, &ev); err != nil {
				t.Fatalf("Append: %v", err)
			}

			got, err := s.Get(ctx, "evt-1")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.ID != "evt-1" || got.RunID != "run-1" || got.EventType != "tool.invoked" {
				t.Fatalf("unexpected event: %+v", got)
			}
			if got.Tool != "edit_texture" {
				t.Fatalf("tool mismatch: got %q want edit_texture", got.Tool)
			}
			if got.Actor != "agent-1" || got.OwnerID != "user-alice" {
				t.Fatalf("actor/owner mismatch: %+v", got)
			}
			if !got.CreatedAt.Equal(ts) {
				t.Fatalf("created_at mismatch: got %v want %v", got.CreatedAt, ts)
			}
			if string(got.Payload) != string(ev.Payload) {
				t.Fatalf("payload mismatch: got %q want %q", got.Payload, ev.Payload)
			}
		})
	}
}

func TestGetNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, err := s.Get(ctx, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAppendRejectsEmptyIDAndType(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.Append(ctx, &Event{EventType: "loop.started", CreatedAt: time.Now()}); err == nil {
		t.Fatalf("expected error for empty id")
	}
	if err := s.Append(ctx, &Event{ID: "x", CreatedAt: time.Now()}); err == nil {
		t.Fatalf("expected error for empty event_type")
	}
	if err := s.Append(ctx, nil); err == nil {
		t.Fatalf("expected error for nil event")
	}
}

func TestAppendDefaultsCreatedAtAndPayload(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	before := time.Now().Add(-time.Second)
	ev := Event{ID: "e-default", RunID: "run-1", EventType: "loop.started"}
	if err := s.Append(ctx, &ev); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := s.Get(ctx, "e-default")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CreatedAt.Before(before) {
		t.Fatalf("created_at not defaulted: %v", got.CreatedAt)
	}
	if string(got.Payload) != "{}" {
		t.Fatalf("payload not defaulted: %q", got.Payload)
	}
}

func TestListByRunOrdersBySeqAscending(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	// Insert out of seq order to verify ordering.
	for _, tc := range []struct {
		id  string
		seq int64
	}{
		{"e3", 3},
		{"e1", 1},
		{"e2", 2},
	} {
		ev := sampleEvent(tc.id, "run-1", "loop.progress", base.Add(time.Duration(tc.seq)*time.Minute))
		ev.Seq = tc.seq
		if err := s.Append(ctx, &ev); err != nil {
			t.Fatalf("Append %s: %v", tc.id, err)
		}
	}
	// Distractor in another run.
	other := sampleEvent("other", "run-2", "loop.started", base)
	if err := s.Append(ctx, &other); err != nil {
		t.Fatalf("Append other: %v", err)
	}

	got, err := s.ListByRun(ctx, "run-1", 0)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}
	if got[0].ID != "e1" || got[1].ID != "e2" || got[2].ID != "e3" {
		t.Fatalf("seq order mismatch: %s %s %s", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestListByOwnerOrdersByCreatedAtDesc(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	for i, id := range []string{"old", "mid", "new"} {
		ev := sampleEvent(id, "run-"+id, "loop.started", base.Add(time.Duration(i)*time.Hour))
		if err := s.Append(ctx, &ev); err != nil {
			t.Fatalf("Append %s: %v", id, err)
		}
	}
	// Distractor owned by another user.
	bob := sampleEvent("bob-1", "run-bob", "loop.started", base)
	bob.OwnerID = "user-bob"
	if err := s.Append(ctx, &bob); err != nil {
		t.Fatalf("Append bob: %v", err)
	}

	got, err := s.ListByOwner(ctx, "user-alice", 0)
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}
	if got[0].ID != "new" || got[2].ID != "old" {
		t.Fatalf("owner order mismatch: %s %s %s", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestListByTrajectory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	for i, id := range []string{"t1", "t2"} {
		ev := sampleEvent(id, "run-"+id, "loop.started", base.Add(time.Duration(i)*time.Minute))
		ev.TrajectoryID = "traj-A"
		if err := s.Append(ctx, &ev); err != nil {
			t.Fatalf("Append %s: %v", id, err)
		}
	}
	distractor := sampleEvent("t3", "run-t3", "loop.started", base)
	distractor.TrajectoryID = "traj-B"
	if err := s.Append(ctx, &distractor); err != nil {
		t.Fatalf("Append distractor: %v", err)
	}

	got, err := s.ListByTrajectory(ctx, "user-alice", "traj-A", 0)
	if err != nil {
		t.Fatalf("ListByTrajectory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].ID != "t1" || got[1].ID != "t2" {
		t.Fatalf("trajectory order mismatch: %s %s", got[0].ID, got[1].ID)
	}
}

func TestFromEventRecordExtractsToolAndParent(t *testing.T) {
	ts := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	rec := &types.EventRecord{
		EventID:      "ev-orig",
		Seq:          7,
		StreamSeq:    42,
		Timestamp:    ts,
		RunID:        "run-7",
		AgentID:      "agent-texture",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-1",
		Kind:         types.EventToolInvoked,
		Phase:        "execution",
		Payload:      json.RawMessage(`{"tool":"edit_texture","call_id":"c1","parent_event_id":"ev-parent"}`),
	}
	ev := FromEventRecord(rec)
	if ev.ID != "ev-orig" || ev.RunID != "run-7" || ev.EventType != "tool.invoked" {
		t.Fatalf("projection mismatch: %+v", ev)
	}
	if ev.Tool != "edit_texture" {
		t.Fatalf("tool not extracted: %q", ev.Tool)
	}
	if ev.ParentID != "ev-parent" {
		t.Fatalf("parent_id not extracted: %q", ev.ParentID)
	}
	if ev.Seq != 7 || ev.StreamSeq != 42 {
		t.Fatalf("seq mismatch: %d/%d", ev.Seq, ev.StreamSeq)
	}
	if ev.Actor != "agent-texture" || ev.OwnerID != "user-alice" || ev.TrajectoryID != "traj-1" {
		t.Fatalf("identity mismatch: %+v", ev)
	}
	if !ev.CreatedAt.Equal(ts.UTC()) {
		t.Fatalf("created_at mismatch: %v", ev.CreatedAt)
	}
	// Source record must be unchanged.
	if rec.EventID != "ev-orig" || string(rec.Payload) != `{"tool":"edit_texture","call_id":"c1","parent_event_id":"ev-parent"}` {
		t.Fatalf("source record mutated: %+v", rec)
	}
}

func TestFromEventRecordHandlesEmptyAndMalformedPayload(t *testing.T) {
	rec := &types.EventRecord{
		EventID:   "ev-empty",
		RunID:     "run-1",
		Kind:      types.EventRunStarted,
		Timestamp: time.Now(),
	}
	ev := FromEventRecord(rec)
	if ev.Tool != "" || ev.ParentID != "" {
		t.Fatalf("expected empty tool/parent, got %q/%q", ev.Tool, ev.ParentID)
	}
	if string(ev.Payload) != "{}" {
		t.Fatalf("expected defaulted payload, got %q", ev.Payload)
	}

	rec2 := &types.EventRecord{
		EventID:   "ev-bad",
		Kind:      types.EventRunStarted,
		Payload:   json.RawMessage(`{not-json`),
		Timestamp: time.Now(),
	}
	ev2 := FromEventRecord(rec2)
	if ev2.Tool != "" || ev2.ParentID != "" {
		t.Fatalf("expected empty tool/parent for malformed payload, got %q/%q", ev2.Tool, ev2.ParentID)
	}
}

func TestAppendPersistsParentChainForQuery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	parent := sampleEvent("ev-parent", "run-1", "tool.invoked", base)
	parent.Tool = "edit_texture"
	if err := s.Append(ctx, &parent); err != nil {
		t.Fatalf("Append parent: %v", err)
	}
	child := sampleEvent("ev-child", "run-1", "tool.result", base.Add(time.Second))
	child.ParentID = "ev-parent"
	if err := s.Append(ctx, &child); err != nil {
		t.Fatalf("Append child: %v", err)
	}

	got, err := s.Get(ctx, "ev-child")
	if err != nil {
		t.Fatalf("Get child: %v", err)
	}
	if got.ParentID != "ev-parent" {
		t.Fatalf("parent_id not persisted: %q", got.ParentID)
	}
}

func TestNewDoltStoreRejectsNilDB(t *testing.T) {
	if _, err := NewDoltStore(nil); err == nil {
		t.Fatalf("expected error for nil db")
	}
}

func TestParseTraceTimeFormats(t *testing.T) {
	cases := []string{
		"2026-06-27T12:00:00Z",
		"2026-06-27T12:00:00.123456789Z",
		"2026-06-27 12:00:00",
		"2026-06-27 12:00:00.123456789",
		"2026-06-27T12:00:00",
	}
	for _, raw := range cases {
		ts, err := parseTraceTime(raw)
		if err != nil {
			t.Fatalf("parseTraceTime(%q): %v", raw, err)
		}
		wantYear, wantMonth, wantDay := 2026, time.June, 27
		if ts.Year() != wantYear || ts.Month() != wantMonth || ts.Day() != wantDay {
			t.Fatalf("parseTraceTime(%q) date mismatch: %v", raw, ts)
		}
	}
	if _, err := parseTraceTime(""); err == nil {
		t.Fatalf("expected error for empty timestamp")
	}
	if _, err := parseTraceTime("not-a-time"); err == nil {
		t.Fatalf("expected error for invalid timestamp")
	}
}

func TestAppendDuplicateIDFails(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ev := sampleEvent("dup", "run-1", "loop.started", time.Now())
	if err := s.Append(ctx, &ev); err != nil {
		t.Fatalf("first Append: %v", err)
	}
	err := s.Append(ctx, &ev)
	if err == nil || !strings.Contains(err.Error(), "insert event") {
		t.Fatalf("expected insert error on duplicate id, got %v", err)
	}
}
