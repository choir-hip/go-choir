package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The write gate refuses new writes of unregistered kinds on the computer's
// store, but replay of a tape that already carries one still projects it
// (H-OG1, failure mode F3 in
// docs/archive/problems/object-graph-registration-and-anchor-gaps-2026-10-14.md).
func TestObjectGraphWriteGateRefusesNewWritesButReplayProjectsHistory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	kind := objectgraph.ObjectKind("choir.universal_wire_story_cluster") // retired, unregistered (H-OG2)
	id, err := objectgraph.BuildCanonicalID(kind, "replay-owner", objectgraph.StableSuffixFromKey("historic"))
	if err != nil {
		t.Fatal(err)
	}
	obj := objectgraph.Object{CanonicalID: id, ObjectKind: kind, OwnerID: "replay-owner", ContentHash: "sha256:historic", Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}

	if err := s.ogStore.PutObject(ctx, obj); !errors.Is(err, objectgraph.ErrUnregisteredKind) {
		t.Fatalf("new write of an unregistered kind: err = %v, want ErrUnregisteredKind", err)
	}

	body, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := projectObject(ctx, tx, body, nil, nil); err != nil {
		_ = tx.Rollback()
		t.Fatalf("replay projection of a historic unregistered kind: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got, err := s.ogStore.GetObject(ctx, id)
	if err != nil {
		t.Fatalf("projected historic object not readable: %v", err)
	}
	if got.ObjectKind != kind {
		t.Fatalf("projected kind = %q", got.ObjectKind)
	}
}
