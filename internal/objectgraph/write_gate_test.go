package objectgraph

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// Failure modes (docs/problems/object-graph-registration-and-anchor-gaps-2026-10-14.md):
// F2 an unregistered write reaches the interceptor (the event tape); F4 a
// tombstone of a historic kind is refused; F5 one write method is
// ungated.

func gateObject(t *testing.T, kind ObjectKind, key string) Object {
	t.Helper()
	id, err := BuildCanonicalID(kind, "user:gate", StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	meta := json.RawMessage(`{}`)
	return Object{CanonicalID: id, ObjectKind: kind, OwnerID: "user:gate", ContentHash: ContentHash(kind, nil, meta), Metadata: meta, CreatedAt: now, UpdatedAt: now}
}

func gateEdge(t *testing.T, from, to string, kind EdgeKind) Edge {
	t.Helper()
	id, err := BuildEdgeID(from, to, kind, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	return Edge{EdgeID: id, FromID: from, ToID: to, Kind: kind, Metadata: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}
}

func TestCheckWriteRefusesUnregisteredKinds(t *testing.T) {
	good := gateObject(t, "choir.citation_check", "a")
	bad := gateObject(t, "choir.not_a_kind", "b")
	if err := CheckWrite([]Object{good}, []Edge{gateEdge(t, good.CanonicalID, good.CanonicalID, "commitment_record_packet")}); err != nil {
		t.Fatalf("registered kinds refused: %v", err)
	}
	if err := CheckWrite([]Object{good, bad}, nil); !errors.Is(err, ErrUnregisteredKind) {
		t.Fatalf("unregistered object kind: err = %v, want ErrUnregisteredKind", err)
	}
	if err := CheckWrite(nil, []Edge{gateEdge(t, good.CanonicalID, good.CanonicalID, "not_an_edge")}); !errors.Is(err, ErrUnregisteredKind) {
		t.Fatalf("unregistered edge kind: err = %v, want ErrUnregisteredKind", err)
	}
	// F4: retiring a historic object or edge of an unregistered kind passes.
	retired := gateObject(t, "choir.universal_wire_story_cluster", "c")
	retired.Tombstone = true
	retiredEdge := gateEdge(t, retired.CanonicalID, retired.CanonicalID, "not_an_edge")
	retiredEdge.Tombstone = true
	if err := CheckWrite([]Object{retired}, []Edge{retiredEdge}); err != nil {
		t.Fatalf("tombstone of a retired kind refused: %v", err)
	}
}

// The two citation relation types platform.Service sets become edge kinds
// at publication_graph.go (EdgeKind(cite.RelationType)); a literal scan
// cannot see them, so they are pinned here (F1).
func TestCitationRelationEdgeKindsAreRegistered(t *testing.T) {
	for _, kind := range []EdgeKind{"references", "is_version_of"} {
		if _, err := DefaultRegistry().LookupEdge(kind); err != nil {
			t.Fatalf("citation relation %q: %v", kind, err)
		}
	}
}

func TestSQLStoreGatesEveryWriteBeforeTheInterceptor(t *testing.T) {
	ctx := context.Background()
	store := NewSQLStore(openTestDB(t))
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	intercepted := 0
	store.SetMutationInterceptor(func(context.Context, []Object, []Edge) error {
		intercepted++
		return nil
	})
	bad := gateObject(t, "choir.not_a_kind", "x")
	good := gateObject(t, "choir.support_verdict", "y")
	badEdge := gateEdge(t, good.CanonicalID, good.CanonicalID, "not_an_edge")
	writes := map[string]func() error{
		"PutObject":           func() error { return store.PutObject(ctx, bad) },
		"PutBatch object":     func() error { return store.PutBatch(ctx, Batch{Objects: []Object{good, bad}}) },
		"PutBatch edge":       func() error { return store.PutBatch(ctx, Batch{Objects: []Object{good}, Edges: []Edge{badEdge}}) },
		"PutBatchConditional": func() error { return store.PutBatchConditional(ctx, nil, Batch{Objects: []Object{bad}}) },
		"PutEdge":             func() error { return store.PutEdge(ctx, badEdge) },
		"Service.CreateObject": func() error {
			_, err := NewService(Config{Durable: store}).CreateObject(ctx, CreateObjectRequest{Kind: "choir.not_a_kind", OwnerID: "user:gate", IdentityKey: "k"})
			return err
		},
	}
	for name, write := range writes {
		err := write()
		if name == "Service.CreateObject" {
			if err == nil {
				t.Errorf("%s: unregistered kind accepted", name)
			}
			continue
		}
		if !errors.Is(err, ErrUnregisteredKind) {
			t.Errorf("%s: err = %v, want ErrUnregisteredKind", name, err)
		}
	}
	if intercepted != 0 {
		t.Fatalf("interceptor saw %d refused writes; the gate must run before the event tape", intercepted)
	}
	if err := store.PutBatch(ctx, Batch{Objects: []Object{good}}); err != nil {
		t.Fatalf("registered write refused: %v", err)
	}
	if intercepted != 1 {
		t.Fatalf("registered write reached the interceptor %d times, want 1", intercepted)
	}
}

func TestSQLStoreDirectSQLWriteRefusedLeavesNoRow(t *testing.T) {
	ctx := context.Background()
	store := NewSQLStore(openTestDB(t))
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	bad := gateObject(t, "choir.not_a_kind", "x")
	if err := store.PutObject(ctx, bad); !errors.Is(err, ErrUnregisteredKind) {
		t.Fatalf("PutObject: err = %v", err)
	}
	if _, err := store.GetObject(ctx, bad.CanonicalID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("refused object was stored: err = %v", err)
	}
	// Tombstones of retired kinds still write (F4).
	retired := gateObject(t, "choir.universal_wire_story_cluster", "r")
	retired.Tombstone = true
	if err := store.PutObject(ctx, retired); err != nil {
		t.Fatalf("tombstone of a retired kind refused: %v", err)
	}
}
