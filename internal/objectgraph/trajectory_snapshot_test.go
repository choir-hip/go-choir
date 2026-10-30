package objectgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6: the paper's
// engine mutex was held 416 ms per ReadObjectSnapshotFiltered call, a read of
// the metadata of every lifecycle object in the computer to keep one
// trajectory's rows. ReadTrajectorySnapshot reads them through the
// og_meta_trajectory_id index. Failure modes:
//  1. the refs query scans og_objects instead of seeking the index;
//  2. a row of another trajectory, owner or computer is returned, or a row
//     of this one is lost;
//  3. the followed objects (named by canonical id) are not fetched, or a
//     follow round repeats forever;
//  4. the order differs from the scan's (canonical id), which callers rely
//     on for ties;
//  5. with no trajectory id, the read pretends to succeed.

func TestTrajectorySnapshotSeeksTheTrajectoryIndex(t *testing.T) {
	db := openTestDB(t)
	store := NewSQLStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	for i := 0; i < 20; i++ {
		putTestObject(t, store, fmt.Sprintf("ev-%02d", i), "choir.lifecycle_event", map[string]any{"trajectory_id": fmt.Sprintf("t-%d", i%4)}, `{}`, at)
	}
	query, args := trajectoryRefsSQL("choir.lifecycle_event", "t-1")
	assertIndexedPlan(t, explainPlan(t, db, query, args...), "idx_og_objects_og_meta_trajectory_id(_recent)?", false)
}

func TestTrajectorySnapshotKeepsOneTrajectoryAndFollows(t *testing.T) {
	db := openTestDB(t)
	store := NewSQLStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	put := func(id string, kind ObjectKind, owner, computer string, meta map[string]any, body string) {
		t.Helper()
		raw, _ := json.Marshal(meta)
		obj := Object{CanonicalID: id, ObjectKind: kind, OwnerID: owner, ComputerID: computer, VersionID: "v",
			ContentHash: "h-" + id, Body: []byte(body), Metadata: raw, CreatedAt: at, UpdatedAt: at}
		if err := store.PutObject(ctx, obj); err != nil {
			t.Fatalf("put %s: %v", id, err)
		}
	}
	put("z-run", "choir.run", "owner", "computer", map[string]any{"trajectory_id": "t1"}, `{"n":"z"}`)
	put("a-run", "choir.run", "owner", "computer", map[string]any{"trajectory_id": "t1"}, `{"n":"a"}`)
	put("m-ev", "choir.lifecycle_event", "owner", "computer", map[string]any{"trajectory_id": "t1"}, `{}`)
	put("x-ev-dropped", "choir.lifecycle_event", "owner", "computer", map[string]any{"trajectory_id": "t1", "drop": true}, `{}`)
	put("other-traj", "choir.run", "owner", "computer", map[string]any{"trajectory_id": "t2"}, `{}`)
	put("other-computer", "choir.run", "owner", "computer-2", map[string]any{"trajectory_id": "t1"}, `{}`)
	put("other-owner", "choir.run", "owner-2", "computer", map[string]any{"trajectory_id": "t1"}, `{}`)
	put("doc-1", "choir.texture_document", "owner", "computer", map[string]any{"doc_id": "d1"}, `{"next":"rev-1"}`)
	put("rev-1", "choir.texture_revision", "owner", "computer", map[string]any{"doc_id": "d1"}, `{}`)

	keep := func(_ ObjectKind, metadata json.RawMessage) bool {
		var m struct{ Drop bool }
		_ = json.Unmarshal(metadata, &m)
		return !m.Drop
	}
	rounds := 0
	follow := func(have []Object) []string {
		rounds++
		ids := map[string]bool{}
		for _, o := range have {
			ids[o.CanonicalID] = true
		}
		switch {
		case !ids["doc-1"]:
			return []string{"doc-1"}
		case !ids["rev-1"]:
			return []string{"rev-1", "doc-1"}
		}
		return []string{"doc-1"} // already fetched: must end the rounds
	}
	objs, ok, err := store.ReadTrajectorySnapshot(ctx, "owner", "computer", "t1", []ObjectKind{"choir.run", "choir.lifecycle_event"}, keep, follow)
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}
	var got []string
	for _, o := range objs {
		got = append(got, o.CanonicalID)
	}
	want := []string{"a-run", "doc-1", "m-ev", "rev-1", "z-run"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("objects = %v, want %v (canonical order)", got, want)
	}
	if rounds > 4 {
		t.Fatalf("follow ran %d rounds", rounds)
	}

	if _, ok, err := store.ReadTrajectorySnapshot(ctx, "owner", "computer", "", []ObjectKind{"choir.run"}, keep, follow); ok || err != nil {
		t.Fatalf("empty trajectory: ok=%v err=%v; want ok=false so the caller scans", ok, err)
	}
}
