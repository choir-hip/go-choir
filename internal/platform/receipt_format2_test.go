package platform

import (
	"context"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Rows stored while the column was jsonb read back in PostgreSQL's spelling
// (docs/problems/old-receipts-do-not-compact-2026-11-14.md). Every reader
// expands format 2, the compact form of that spelling, but under codec 1 the
// backfill does not write it, and it does not expand a format-2 row back to
// full text either: the release before the writer must find only rows it can
// read, and a revert of the writer's release must leave them readable.
func TestFormat2RowsAreReadByEveryReaderAndNotWrittenByTheBackfill(t *testing.T) {
	ctx := context.Background()
	d := newDrillComputer(t, "computer-59c902c5d46caecc0a180a8557164e72")
	d.appendEvent(computerevent.EventGenesisImported, "", nil)
	d.appendMixed(6, "jsonb")
	// PostgreSQL's own jsonb text is the old rows' spelling.
	if _, err := d.store.db.Exec(`UPDATE computer_event_append_receipts SET event_head_receipt_json = event_head_receipt_json::jsonb::text WHERE computer_id=?`, d.id); err != nil {
		t.Fatal(err)
	}
	old := storedReceipts(t, d.store, d.id)
	baseline, err := d.artifacts.EventsPage(ctx, d.id, 0, 100)
	if err != nil || len(baseline) != 7 {
		t.Fatalf("baseline replay: %d, %v", len(baseline), err)
	}
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
	// Codec 1: the jsonb-spelled rows are kept as they are.
	if p := runCompaction(t, noSleepCompactor(d.store)); p[0].Rewritten != 0 || p[0].Kept != 7 || !reflect.DeepEqual(storedReceipts(t, d.store, d.id), old) {
		t.Fatalf("pass = %+v: the backfill wrote a jsonb-spelled row", p[0])
	}
	// What the writer's release will have stored: format 2, made here by the
	// codec's allowance.
	rows, err := d.store.db.Query(`SELECT `+appendReceiptRowColumns+` FROM computer_event_append_receipts WHERE computer_id=? ORDER BY sequence`, d.id)
	if err != nil {
		t.Fatal(err)
	}
	type update struct {
		seq     uint64
		compact string
	}
	var updates []update
	for rows.Next() {
		row, stored, err := scanAppendReceipt(rows)
		if err != nil {
			t.Fatal(err)
		}
		compact, ok := computerevent.CompactStoredHeadReceipt(row, []byte(stored), true)
		if !ok {
			t.Fatalf("sequence %d: format 2 refused", row.Sequence)
		}
		updates = append(updates, update{row.Sequence, compact})
	}
	rows.Close()
	for _, u := range updates {
		if _, err := d.store.db.Exec(`UPDATE computer_event_append_receipts SET event_head_receipt_json=? WHERE computer_id=? AND sequence=?`, u.compact, d.id, u.seq); err != nil {
			t.Fatal(err)
		}
	}
	// Every reader reads them.
	page, err := d.artifacts.EventsPage(ctx, d.id, 0, 100)
	if err != nil || !reflect.DeepEqual(page, baseline) {
		t.Fatalf("replay of format-2 rows differs: %v", err)
	}
	verifyReplayReceipts(t, d, page)
	for _, r := range old {
		receipt, found, err := existingComputerEventReceipt(ctx, d.store.db, d.id, r.key, baseline[r.seq-1].Request.Event.RequestCommitment)
		if err != nil || !found {
			t.Fatalf("retry lookup %s: %v %v", r.key, found, err)
		}
		if raw, _ := receipt.CanonicalBytes(); len(raw) == 0 {
			t.Fatal("empty retry receipt")
		}
	}
	// A pass under codec 1 keeps them compact.
	if _, err := d.store.db.Exec(`DELETE FROM computer_event_receipt_compaction WHERE computer_id=?`, d.id); err != nil {
		t.Fatal(err)
	}
	if p := runCompaction(t, noSleepCompactor(d.store)); p[0].Rewritten != 0 || p[0].Kept != 7 {
		t.Fatalf("pass over format-2 rows = %+v, want all kept", p[0])
	}
	for _, r := range storedReceipts(t, d.store, d.id) {
		if !computerevent.IsCompactHeadReceipt(r.stored) {
			t.Fatalf("sequence %d was expanded by an active pass", r.seq)
		}
	}
	// The rollback gives back the stored bytes exactly.
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactExpand}})
	if exp := runCompaction(t, noSleepCompactor(d.store)); exp[0].Rewritten != 7 {
		t.Fatalf("expand = %+v", exp[0])
	}
	if got := storedReceipts(t, d.store, d.id); !reflect.DeepEqual(got, old) {
		t.Fatal("expand did not restore the jsonb-spelled bytes")
	}
}
