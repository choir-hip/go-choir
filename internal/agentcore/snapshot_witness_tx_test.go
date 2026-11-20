package agentcore

import (
	"context"
	"database/sql"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// S1 (docs/design/snapshot-without-scratch-copy-2026-11-20.md): the witness the
// capture records, computed inside the transaction whose pages it streams, is
// the one a restore recomputes with LiveStoreWitness on the reassembled file.
func TestLiveStoreWitnessTxEqualsLiveStoreWitness(t *testing.T) {
	ctx := context.Background()
	store := openIndexStore(t)
	insertIndexRow(t, store, "c", "kept", 45000, "desk", "finalized", "[]")
	insertIndexRow(t, store, "c", "other", 46000, string(computerevent.EventProjectionBatchRecorded), "finalized", "[]")
	want, err := LiveStoreWitness(ctx, store.ReadDB())
	if err != nil {
		t.Fatal(err)
	}
	tx, err := store.ReadDB().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	got, err := LiveStoreWitnessTx(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != want.ContentRoot || got.SchemaHash != want.SchemaHash {
		t.Fatalf("witness in the transaction %s/%s, on the store %s/%s", got.ContentRoot, got.SchemaHash, want.ContentRoot, want.SchemaHash)
	}
	if _, err := LiveStoreWitnessTx(ctx, nil); err == nil {
		t.Fatal("a nil transaction was accepted")
	}
}
