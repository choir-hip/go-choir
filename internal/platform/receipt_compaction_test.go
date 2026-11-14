package platform

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The tests name the failure modes of receipt_compaction.go (C1..C8).

func noSleepCompactor(store *Store) *ReceiptCompactor {
	c := NewReceiptCompactor(store)
	c.sleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	return c
}

type storedReceiptRow struct {
	key    string
	seq    uint64
	stored string
}

func storedReceipts(t *testing.T, store *Store, computerID string) []storedReceiptRow {
	t.Helper()
	rows, err := store.db.Query(`SELECT idempotency_key, sequence, event_head_receipt_json FROM computer_event_append_receipts WHERE computer_id=? ORDER BY sequence`, computerID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []storedReceiptRow
	for rows.Next() {
		var r storedReceiptRow
		if err := rows.Scan(&r.key, &r.seq, &r.stored); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func runCompaction(t *testing.T, c *ReceiptCompactor) []ReceiptCompactionPass {
	t.Helper()
	passes, err := c.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("compaction pass: %v", err)
	}
	return passes
}

func verifyReplayReceipts(t *testing.T, d *drillComputer, page []computerevent.DurableEvent) {
	t.Helper()
	verifier := computerevent.EventHeadReceiptVerifier{Keys: platformTestKeyResolver{key: d.service.signingKey.Public}}
	for _, record := range page {
		if err := verifier.VerifyEventHeadReceipt(context.Background(), record.Receipt, record.Request); err != nil {
			t.Fatalf("sequence %d: replayed receipt does not verify: %v", record.Request.Event.Sequence, err)
		}
	}
}

// C2, C5, C8: the backfill rewrites a real CAS chain compactly, and replay,
// the idempotent retry, receipt verification and bundle export read it
// exactly as they read the full chain. New appends are stored compact.
func TestReceiptCompactionIsInvisibleToEveryReader(t *testing.T) {
	ctx := context.Background()
	d := newDrillComputer(t, "computer-59c902c5d46caecc0a180a8557164e72")
	d.appendEvent(computerevent.EventGenesisImported, "", nil)
	d.appendMixed(8, "before")
	baseline, err := d.artifacts.EventsPage(ctx, d.id, 0, 100)
	if err != nil || len(baseline) != 9 {
		t.Fatalf("baseline replay: %d, %v", len(baseline), err)
	}
	full := storedReceipts(t, d.store, d.id)
	fullExport := t.TempDir()
	exportWholeTape(t, d, fullExport)

	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
	passes := runCompaction(t, noSleepCompactor(d.store))
	if len(passes) != 1 || passes[0].Rewritten != 9 || passes[0].Kept != 0 || !passes[0].Done || passes[0].BytesAfter*2 > passes[0].BytesBefore {
		t.Fatalf("pass = %+v, want 9 rewritten to under half the bytes", passes)
	}
	compact := storedReceipts(t, d.store, d.id)
	for i, r := range compact {
		if !computerevent.IsCompactHeadReceipt(r.stored) || len(r.stored) >= len(full[i].stored) {
			t.Fatalf("sequence %d stored %d bytes, compact=%v", r.seq, len(r.stored), computerevent.IsCompactHeadReceipt(r.stored))
		}
	}

	// Replay pages, whole and paged, equal the full-JSON baseline and verify.
	page, err := d.artifacts.EventsPage(ctx, d.id, 0, 100)
	if err != nil || !reflect.DeepEqual(page, baseline) {
		t.Fatalf("replay of the compact chain differs from the full one: %v", err)
	}
	verifyReplayReceipts(t, d, page)
	if tail, err := d.artifacts.EventsPage(ctx, d.id, 4, 3); err != nil || !reflect.DeepEqual(tail, baseline[4:7]) {
		t.Fatalf("paged replay differs: %v", err)
	}
	// The idempotent retry returns the original signed bytes.
	for _, r := range full {
		receipt, found, err := existingComputerEventReceipt(ctx, d.store.db, d.id, r.key, baseline[r.seq-1].Request.Event.RequestCommitment)
		if err != nil || !found {
			t.Fatalf("retry lookup %s: %v %v", r.key, found, err)
		}
		raw, _ := receipt.CanonicalBytes()
		if string(raw) != r.stored {
			t.Fatalf("sequence %d: retry receipt differs from the stored original", r.seq)
		}
	}
	// A bundle carries the canonical bytes whatever the stored form.
	compactExport := t.TempDir()
	exportWholeTape(t, d, compactExport)
	if a, b := readBundleReceipts(t, fullExport), readBundleReceipts(t, compactExport); !reflect.DeepEqual(a, b) {
		t.Fatal("the bundle of the compact chain carries other receipts than the full one")
	}

	// C5: progress is durable: a second pass reads nothing it already did.
	if again := runCompaction(t, noSleepCompactor(d.store)); again[0].Scanned != 0 || !again[0].Done {
		t.Fatalf("second pass = %+v, want nothing scanned", again)
	}
	// New appends are stored compact at the append and still replay.
	d.appendMixed(3, "after")
	after := storedReceipts(t, d.store, d.id)
	for _, r := range after[9:] {
		if !computerevent.IsCompactHeadReceipt(r.stored) {
			t.Fatalf("new append at %d stored full under an active policy", r.seq)
		}
	}
	page, err = d.artifacts.EventsPage(ctx, d.id, 0, 100)
	if err != nil || len(page) != 12 || !reflect.DeepEqual(page[:9], baseline) {
		t.Fatalf("replay after compact appends: %d, %v", len(page), err)
	}
	verifyReplayReceipts(t, d, page)
	if third := runCompaction(t, noSleepCompactor(d.store)); third[0].Scanned != 3 || third[0].Rewritten != 0 || third[0].Kept != 3 {
		t.Fatalf("pass over compact appends = %+v, want 3 scanned and kept", third)
	}
}

func exportWholeTape(t *testing.T, c *drillComputer, out string) {
	t.Helper()
	if _, err := bundle.Export(context.Background(), c.store.db, c.root, bundle.ExportOptions{ComputerID: c.id, Out: out, FromSeq: 0, Producer: bundle.Producer{Host: "drill", Tool: "test"}}); err != nil {
		t.Fatalf("export: %v", err)
	}
}

func readBundleReceipts(t *testing.T, dir string) []bundle.ReceiptRow {
	t.Helper()
	var out []bundle.ReceiptRow
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "receipts.jsonl" {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 1<<20), 1<<24)
		for scanner.Scan() {
			var row bundle.ReceiptRow
			if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
				return err
			}
			out = append(out, row)
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatalf("no receipts in bundle %s", dir)
	}
	return out
}

// C3: a dry run counts what it would rewrite and writes nothing, and the
// append path keeps writing full JSON for a dry-run computer.
func TestReceiptCompactionDryRunWritesNothing(t *testing.T) {
	d := newDrillComputer(t, "computer-1355b83bced1613eb048ee28d66688a8")
	d.appendEvent(computerevent.EventGenesisImported, "", nil)
	d.appendMixed(5, "dry")
	before := storedReceipts(t, d.store, d.id)
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactDryRun}})
	passes := runCompaction(t, noSleepCompactor(d.store))
	if passes[0].Rewritten != 6 || passes[0].BytesAfter >= passes[0].BytesBefore {
		t.Fatalf("dry run = %+v, want 6 counted as rewritable", passes[0])
	}
	d.appendMixed(2, "dry-after")
	after := storedReceipts(t, d.store, d.id)
	if !reflect.DeepEqual(after[:6], before) {
		t.Fatal("a dry run changed stored receipts")
	}
	for _, r := range after {
		if computerevent.IsCompactHeadReceipt(r.stored) {
			t.Fatalf("sequence %d stored compact under dry-run", r.seq)
		}
	}
	var mode string
	var rewritten int64
	if err := d.store.db.QueryRow(`SELECT mode, rows_rewritten FROM computer_event_receipt_compaction WHERE computer_id=?`, d.id).Scan(&mode, &rewritten); err != nil || mode != ReceiptCompactDryRun || rewritten != 6 {
		t.Fatalf("dry-run progress = %q %d %v", mode, rewritten, err)
	}
}

// C6: expand writes back the exact original bytes (the rollback), and a mode
// change starts the walk over.
func TestReceiptCompactionExpandRestoresTheOriginal(t *testing.T) {
	d := newDrillComputer(t, "computer-59c902c5d46caecc0a180a8557164e72")
	d.appendEvent(computerevent.EventGenesisImported, "", nil)
	d.appendMixed(4, "x")
	original := storedReceipts(t, d.store, d.id)
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
	runCompaction(t, noSleepCompactor(d.store))
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactExpand}})
	passes := runCompaction(t, noSleepCompactor(d.store))
	if passes[0].Rewritten != 5 {
		t.Fatalf("expand = %+v, want 5 rewritten", passes[0])
	}
	if got := storedReceipts(t, d.store, d.id); !reflect.DeepEqual(got, original) {
		t.Fatal("expand did not restore the original stored bytes")
	}
	// Expand also stops compact writes.
	d.appendMixed(1, "y")
	if got := storedReceipts(t, d.store, d.id); computerevent.IsCompactHeadReceipt(got[5].stored) {
		t.Fatal("an append under expand was stored compact")
	}
}

// C8: the tape sweep reads payload pins from a compact receipt as from a
// full one, so a compacted live chain's files are never listed as orphans.
func TestReceiptCompactionTapeSweepKeepsCompactChainFiles(t *testing.T) {
	f := newTapeFixture(t)
	f.live("computer-live")
	live := f.chain("computer-live", 4, tapeAged)
	orphan := f.put(tapeNSPayload, []byte("orphan payload"), tapeAged)
	f.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{"computer-live": ReceiptCompactActive}})
	if passes := runCompaction(t, noSleepCompactor(f.store)); passes[0].Rewritten != 4 {
		t.Fatalf("compaction = %+v", passes[0])
	}
	items := f.manifestItems(f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true}))
	orphans := listedDigests(items, TapeSweepOrphans)
	if len(orphans) != 1 || !orphans[orphan] {
		t.Fatalf("orphans = %v, want only the orphan payload", orphans)
	}
	for ns, digests := range live.files() {
		for _, d := range digests {
			if orphans[d] {
				t.Fatalf("compact chain file %s/%s listed as orphan", ns, d)
			}
		}
	}
}

// C7: only exact computer ids and known modes are accepted.
func TestParseReceiptCompactComputers(t *testing.T) {
	p, err := ParseReceiptCompactComputers(" computer-a=active, computer-b=dry-run ,computer-c,computer-d=expand,")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"computer-a": "active", "computer-b": "dry-run", "computer-c": "active", "computer-d": "expand"}
	if !reflect.DeepEqual(p.Computers, want) {
		t.Fatalf("parsed %v", p.Computers)
	}
	if p, err := ParseReceiptCompactComputers(""); err != nil || len(p.Computers) != 0 {
		t.Fatalf("empty = %v, %v", p, err)
	}
	for _, bad := range []string{"computer-a=on", "vm-a", "*", "computer-a=active=x"} {
		if _, err := ParseReceiptCompactComputers(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

// C1: every non-test reader of event_head_receipt_json goes through
// appendReceiptRowColumns and the expand helpers; a new raw reader of the
// column fails here.
func TestAppendReceiptJSONReadersExpand(t *testing.T) {
	allowed := map[string]bool{
		"internal/platform/computer_events.go":    true, // DDL, the insert, the column list
		"internal/platform/receipt_compaction.go": true, // the column list and the backfill
		"internal/bundle/store.go":                true, // receiptColumns; readReceipts expands
		"internal/bundle/bundle.go":               true, // the bundle line's field tag
	}
	root := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if strings.Contains(string(raw), "event_head_receipt_json") && !allowed[rel] {
				t.Errorf("%s names event_head_receipt_json: select appendReceiptRowColumns and expand with scanExpandedAppendReceipt (receipt_compaction.go)", rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Rows written while the column was jsonb read back in PostgreSQL's spelling,
// not the canonical bytes (docs/problems/old-receipts-do-not-compact-2026-11-14.md).
// The backfill had already walked past them under the canonical codec and kept
// them, so a pass saved by that codec must start over, and the rewrite must give
// back exactly the stored bytes (C2, C5, C6, C8, C9).
func TestReceiptCompactionRevisitsRowsAnOlderCodecKept(t *testing.T) {
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
	// The pass the canonical codec saved: through the head, every row kept.
	if _, err := d.store.db.Exec(`INSERT INTO computer_event_receipt_compaction (computer_id, mode, through_sequence, rows_rewritten, rows_kept, bytes_before, bytes_after, updated_at) VALUES (?, 'active', 7, 0, 7, 1, 1, now())`, d.id); err != nil {
		t.Fatal(err)
	}
	// Chunks the sweep stored for this computer must survive: the rewrite is lossless (C9).
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
	passes := runCompaction(t, noSleepCompactor(d.store))
	if len(passes) != 1 || passes[0].Rewritten != 7 || passes[0].Kept != 0 || !passes[0].Done {
		t.Fatalf("pass = %+v, want all 7 jsonb-spelled rows rewritten", passes)
	}
	now := storedReceipts(t, d.store, d.id)
	for i, r := range now {
		if !computerevent.IsCompactHeadReceipt(r.stored) || len(r.stored)*2 > len(old[i].stored) {
			t.Fatalf("sequence %d not compact: %d bytes from %d", r.seq, len(r.stored), len(old[i].stored))
		}
	}
	page, err := d.artifacts.EventsPage(ctx, d.id, 0, 100)
	if err != nil || !reflect.DeepEqual(page, baseline) {
		t.Fatalf("replay of the rewritten chain differs: %v", err)
	}
	verifyReplayReceipts(t, d, page)
	// A second pass reads nothing: the codec of the saved pass is current.
	if again := runCompaction(t, noSleepCompactor(d.store)); again[0].Scanned != 0 || !again[0].Done {
		t.Fatalf("second pass = %+v, want nothing scanned", again)
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
