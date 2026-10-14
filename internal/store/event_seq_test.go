package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// AppendEvent's per-run Seq is an in-memory high-water mark (C5). Ways it
// can fail, written before the code:
//
//  1. Two runs, or two owners on one run id, share a counter.
//  2. A reopened store restarts a run's Seq at 1 (the mark is not seeded
//     from the store) and repeats numbers a catch-up reader has seen.
//  3. Concurrent appends to one run take the same number or skip one.
//  4. A failed write burns its number, where the per-append read used to
//     hand it out again.
func TestAppendEventSeqHighWaterMark(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	ctx := context.Background()
	now := time.Now().UTC()
	appendTo := func(s *Store, run, owner, id string) int64 {
		t.Helper()
		rec := &types.EventRecord{EventID: id, RunID: run, OwnerID: owner, Timestamp: now,
			Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
		if err := s.AppendEvent(ctx, rec); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
		return rec.Seq
	}

	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// 1. Separate counters per run and per owner.
	for i := 1; i <= 3; i++ {
		if got := appendTo(s1, "run-a", "alice", fmt.Sprintf("a-%d", i)); got != int64(i) {
			t.Fatalf("run-a alice append %d: seq %d", i, got)
		}
		if got := appendTo(s1, "run-b", "alice", fmt.Sprintf("b-%d", i)); got != int64(i) {
			t.Fatalf("run-b alice append %d: seq %d", i, got)
		}
	}
	if got := appendTo(s1, "run-a", "bob", "a-bob-1"); got != 1 {
		t.Fatalf("run-a bob first append: seq %d, want 1", got)
	}

	// 3. Concurrent appends to one run: unique and gapless.
	var wg sync.WaitGroup
	seqs := make([]int64, 16)
	for i := range seqs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := &types.EventRecord{EventID: fmt.Sprintf("c-%d", i), RunID: "run-c", OwnerID: "alice", Timestamp: now,
				Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
			if err := s1.AppendEvent(ctx, rec); err != nil {
				t.Errorf("concurrent append %d: %v", i, err)
			}
			seqs[i] = rec.Seq
		}(i)
	}
	wg.Wait()
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	for i, seq := range seqs {
		if seq != int64(i+1) {
			t.Fatalf("concurrent seqs = %v, want 1..%d", seqs, len(seqs))
		}
	}

	// 4. A failed write hands its number out again.
	refuse := errors.New("refused for the test")
	s1.ogStore.SetWriteValidator(func(context.Context, []objectgraph.Object, []objectgraph.Edge) error { return refuse })
	rec := &types.EventRecord{EventID: "a-fail", RunID: "run-a", OwnerID: "alice", Timestamp: now,
		Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
	if err := s1.AppendEvent(ctx, rec); !errors.Is(err, refuse) {
		t.Fatalf("refused append: %v", err)
	}
	s1.ogStore.SetWriteValidator(s1.vocabWriteGuard)
	if got := appendTo(s1, "run-a", "alice", "a-4"); got != 4 {
		t.Fatalf("append after a failed write: seq %d, want 4", got)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	// 2. A reopened store continues each run from its durable maximum.
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	if got := appendTo(s2, "run-a", "alice", "a-5"); got != 5 {
		t.Fatalf("run-a after reopen: seq %d, want 5", got)
	}
	if got := appendTo(s2, "run-c", "alice", "c-after"); got != int64(len(seqs)+1) {
		t.Fatalf("run-c after reopen: seq %d, want %d", got, len(seqs)+1)
	}
	if got := appendTo(s2, "run-a", "bob", "a-bob-2"); got != 2 {
		t.Fatalf("run-a bob after reopen: seq %d, want 2", got)
	}
	events, err := s2.ListEventsAfter(ctx, "run-a", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, ev := range events {
		key := fmt.Sprintf("%s/%d", ev.OwnerID, ev.Seq)
		if seen[key] {
			t.Fatalf("duplicate seq %s in %+v", key, events)
		}
		seen[key] = true
	}
}
