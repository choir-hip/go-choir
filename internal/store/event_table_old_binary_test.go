package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// F5 (docs/archive/design/choir-event-table-f-c-2026-10-22.md section 5): what a
// pre-deploy-1 binary does with a stamped op. Written and run on main
// 6c60685cc before any F-c code: Validate passes an object op stamped
// Table "choir_events", and the projector ignores the stamp and upserts the
// row into og_objects. That is why deploy 1 is the rollback floor once any
// op is stamped (section 4.5).
func TestOldBinaryProjectsStampedOpIntoObjectGraph(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	at := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	rec := &types.EventRecord{EventID: "ev-old-1", RunID: "run-old", OwnerID: "owner-1", Seq: 1, StreamSeq: 1,
		Kind: types.EventToolResult, Timestamp: at, Payload: json.RawMessage(`{}`)}
	obj, err := buildEventObject(rec, at)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	batch := computerevent.ProjectionBatch{Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: "computer-old", EventID: "event-old",
		Ops: []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObject, CanonicalID: obj.CanonicalID, Table: "choir_events", Body: body}}}
	if err := batch.Validate(); err != nil {
		t.Fatalf("Validate refuses a stamped object op: %v", err)
	}
	// The old route: projectObject reads only op.Body.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := projectObject(ctx, tx, batch.Ops[0].Body, nil, nil); err != nil {
		t.Fatalf("the old projector refuses a stamped op: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertCount(t, s, `SELECT COUNT(*) FROM og_objects WHERE object_kind='choir.event'`, 1)
}
