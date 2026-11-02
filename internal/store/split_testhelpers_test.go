package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Test seeds for the split layout (og_objects header + og_bodies). The
// pre-split tests inserted one 12-column og_objects row; these write the same
// row through the production primitive, objectgraph.WriteObjectTx, so the
// derived header columns hold what a real writer would store.

func testBytes(v any) []byte {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		return x
	case json.RawMessage:
		return x
	case string:
		return []byte(x)
	}
	panic(fmt.Sprintf("testBytes: unsupported %T", v))
}

func testTime(v any) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	return time.Now().UTC()
}

// drillSeedErr writes one object row (tombstone false, no supersession).
func drillSeedErr(ctx context.Context, s *Store, id, kind, owner, computer, version, hash, body, meta, created, updated any) (any, error) {
	_, err := objectgraph.WriteObjectTx(ctx, s.db, objectgraph.Object{
		CanonicalID: fmt.Sprint(id), ObjectKind: objectgraph.ObjectKind(fmt.Sprint(kind)), OwnerID: fmt.Sprint(owner), ComputerID: fmt.Sprint(computer),
		VersionID: fmt.Sprint(version), ContentHash: fmt.Sprint(hash), Body: testBytes(body), Metadata: testBytes(meta),
		CreatedAt: testTime(created), UpdatedAt: testTime(updated),
	})
	return nil, err
}

func drillSeed(t *testing.T, s *Store, id, kind, owner, computer, version, hash, body, meta, created, updated any) {
	t.Helper()
	if _, err := drillSeedErr(context.Background(), s, id, kind, owner, computer, version, hash, body, meta, created, updated); err != nil {
		t.Fatalf("drill seed: %v", err)
	}
}
