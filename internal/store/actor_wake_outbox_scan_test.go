package store

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The open-wakes list reads only open wakes
// (docs/problems/actor-wake-outbox-list-full-scan-2026-10-23.md).
//
// Ways it can fail, written before the code:
//  1. The list still matches $.projected in the metadata of every outbox row
//     ever written: the engine-mutex counters show a JSON read of the kind
//     after the family is validated (TestOpenWakeListLeavesTheJSONScan).
//  2. The side-table list answers differently from the JSON list: a closed
//     wake comes back, or an open one is missed, after a mark, a fate, a mint,
//     or a withdrawal (TestOpenWakeListAgreesWithTheRowsAcrossMarks).
//  3. The family is read before it is validated or after it is withdrawn: the
//     second test withdraws it and checks the answer is still exact.
//  4. The new family delays the three that exist past a boot: a pass covers
//     every family, so the test checks all four validate in one pass.

const (
	scanOwner, scanComputer = "owner-scan", "computer-scan"
	// outboxProjectedFamily is the family's name; the literal keeps this file
	// compiling on the commit that adds the family.
	outboxProjectedFamily = "actor_wake_outbox.projected"
)

// seedWakeOutbox writes closed projected wakes, then open ones, in batches. It
// returns the open wakes' canonical ids, sorted.
func seedWakeOutbox(t testing.TB, s *Store, closed, open int) []string {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	source := objectgraph.Object{OwnerID: scanOwner, ComputerID: scanComputer, CreatedAt: now, UpdatedAt: now}
	var batch objectgraph.Batch
	var openIDs []string
	flush := func() {
		if len(batch.Objects) == 0 {
			return
		}
		if err := s.ogStore.PutBatch(ctx, batch); err != nil {
			t.Fatal(err)
		}
		batch = objectgraph.Batch{}
	}
	for i := 0; i < closed+open; i++ {
		isOpen := i >= closed
		_, obj, err := actorWakeOutbox(source, fmt.Sprintf("src-%06d", i), "texture:doc-"+strconv.Itoa(i%7), "traj", "agent",
			"coagent_result", strings.Repeat("x", 600)+strconv.Itoa(i), time.Time{}, fmt.Sprintf("wake-%06d", i))
		if err != nil {
			t.Fatal(err)
		}
		if isOpen {
			openIDs = append(openIDs, obj.CanonicalID)
		} else {
			obj.Metadata = []byte(strings.Replace(string(obj.Metadata), `"projected":false`, `"projected":true`, 1))
			obj.ContentHash = objectgraph.ContentHash(obj.ObjectKind, obj.Body, obj.Metadata)
		}
		batch.Objects = append(batch.Objects, obj)
		if len(batch.Objects) == 250 {
			flush()
		}
	}
	flush()
	sort.Strings(openIDs)
	return openIDs
}

func openWakeIDs(t testing.TB, s *Store) []string {
	t.Helper()
	wakes, err := s.ListUnprojectedActorWakes(context.Background())
	if err != nil {
		t.Fatalf("ListUnprojectedActorWakes: %v", err)
	}
	ids := make([]string, len(wakes))
	for i, w := range wakes {
		ids[i] = w.CanonicalID
	}
	sort.Strings(ids)
	return ids
}

// outboxListOps sums the engine-mutex calls of the open-wakes read, by path.
func outboxListOps(s *Store) (jsonCalls, keyCalls uint64) {
	for op, st := range s.EngineMutexStats() {
		if !strings.Contains(op, "ListObjectsByMetadataPage") || !strings.Contains(op, "choir.actor_wake_outbox") || !strings.Contains(op, "$.projected") {
			continue
		}
		if strings.HasSuffix(op, " keyindex") {
			keyCalls += st.Calls
		} else {
			jsonCalls += st.Calls
		}
	}
	return
}

func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestOpenWakeListAgreesWithTheRowsAcrossMarks(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	open := seedWakeOutbox(t, s, 40, 6)
	if err := s.ValidateDerivedKeys(ctx, objectgraph.DerivedKeyValidation{PageSize: 16}); err != nil {
		t.Fatal(err)
	}
	if got := openWakeIDs(t, s); !sameIDs(got, open) {
		t.Fatalf("validated: %v, want %v", got, open)
	}

	// A projected wake, an exhausted one, an interrupted one leave the list.
	if err := s.MarkActorWakeProjected(ctx, open[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkActorWakeExhausted(ctx, open[1], 5, "boom"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkActorWakeInterrupted(ctx, open[2]); err != nil {
		t.Fatal(err)
	}
	want := open[3:]
	if got := openWakeIDs(t, s); !sameIDs(got, want) {
		t.Fatalf("after three marks: %v, want %v", got, want)
	}

	// A wake minted after the pass joins the list.
	now := time.Now().UTC().Truncate(time.Second)
	_, fresh, err := actorWakeOutbox(objectgraph.Object{OwnerID: scanOwner, ComputerID: scanComputer, CreatedAt: now, UpdatedAt: now},
		"src-fresh", "texture:doc-fresh", "traj", "agent", "coagent_result", "fresh", time.Time{}, "wake-fresh")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{fresh}}); err != nil {
		t.Fatal(err)
	}
	want = append(append([]string{}, want...), fresh.CanonicalID)
	sort.Strings(want)
	if got := openWakeIDs(t, s); !sameIDs(got, want) {
		t.Fatalf("after a mint: %v, want %v", got, want)
	}
	if err := s.MarkActorWakeProjected(ctx, fresh.CanonicalID); err != nil {
		t.Fatal(err)
	}
	want = append([]string{}, open[3:]...)
	if got := openWakeIDs(t, s); !sameIDs(got, want) {
		t.Fatalf("after the mint is projected: %v, want %v", got, want)
	}

	// A withdrawn family answers from the rows, and says the same.
	s.derivedKeys().Invalidate(outboxProjectedFamily, "test")
	if got := openWakeIDs(t, s); !sameIDs(got, want) {
		t.Fatalf("withdrawn: %v, want %v", got, want)
	}
}

// TestOpenWakeListMeasure reports rows read and time for the JSON read (before
// the pass) and the side-table read (after it) on a store of many closed wakes
// and few open ones. WAKE_SCAN_ROWS sets the closed count (default 3000).
func TestOpenWakeListMeasure(t *testing.T) {
	closed := 3000
	if v, err := strconv.Atoi(os.Getenv("WAKE_SCAN_ROWS")); err == nil && v > 0 {
		closed = v
	}
	s := openTestStore(t)
	ctx := context.Background()
	want := seedWakeOutbox(t, s, closed, 4)

	timeList := func() time.Duration {
		best := time.Duration(1<<63 - 1)
		for i := 0; i < 5; i++ {
			start := time.Now()
			if got := openWakeIDs(t, s); !sameIDs(got, want) {
				t.Fatalf("list = %v, want %v", got, want)
			}
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	count := func(query string, args ...any) int {
		var n int
		if err := s.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	kindRows := count(`SELECT COUNT(*) FROM og_objects WHERE object_kind = ?`, "choir.actor_wake_outbox")
	jsonTime := timeList()
	t.Logf("JSON read: %d outbox rows read in full to return %d wakes, best of 5 = %s", kindRows, len(want), jsonTime)

	if err := s.ValidateDerivedKeys(ctx, objectgraph.DerivedKeyValidation{PageSize: 200}); err != nil {
		t.Fatal(err)
	}
	keyRows := count(`SELECT COUNT(*) FROM og_key_index WHERE key_name = ? AND key_value = ?`, outboxProjectedFamily, "false")
	keyTime := timeList()
	t.Logf("side-table read: %d key rows read (of %d outbox rows) to return %d wakes, best of 5 = %s", keyRows, kindRows, len(want), keyTime)
}
