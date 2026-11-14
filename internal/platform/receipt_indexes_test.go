package platform

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

const indexTestComputer = "computer-1355b83bced1613eb048ee28d66688a8"

// loadReceiptRows fills the table the way the paper's is shaped: almost every
// row a projection batch, a few rows of the kinds the readers look up.
func loadReceiptRows(t *testing.T, s *Store, computerID string, batches int) {
	t.Helper()
	insert := func(kind string, from, to int) {
		_, err := s.db.Exec(`INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at)
			SELECT ?, 'k-' || ? || g, repeat('a', 32) || md5('c' || ? || g), g, ?, repeat('b', 32) || md5('d' || ? || g), repeat('c', 32) || md5('p' || ? || g), 'r-' || ? || g, '{}', now()
			FROM generate_series(?::bigint, ?::bigint) g`, computerID, computerID, computerID, kind, computerID, computerID, computerID, from, to)
		if err != nil {
			t.Fatal(err)
		}
	}
	insert(string(computerevent.EventGenesisImported), 1, 1)
	insert(string(computerevent.EventProjectionBatchRecorded), 2, batches)
	// A few other kinds late in the tape, among the batches.
	for i, kind := range []computerevent.EventKind{computerevent.EventMaterializationApplied, computerevent.EventKeyRevoked, computerevent.EventRollbackRequested, computerevent.EventRollbackApplied} {
		seq := batches + 1 + i
		insert(string(kind), seq, seq)
	}
	insert(string(computerevent.EventProjectionBatchRecorded), batches+5, batches+500)
	if _, err := s.db.Exec(`ANALYZE computer_event_append_receipts`); err != nil {
		t.Fatal(err)
	}
}

func receiptIndexNames(t *testing.T, s *Store) map[string]bool {
	t.Helper()
	rows, err := s.db.Query(`SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = 'computer_event_append_receipts'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[n] = true
	}
	return out
}

// explain plans a statement with the arguments given, as a custom plan (the
// planner sees the values) or a generic one (it sees only parameters), the two
// ways a prepared statement of the driver runs.
func explain(t *testing.T, s *Store, query string, generic bool, args ...any) string {
	t.Helper()
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// The pooled connection goes back clean: no prepared statement, no plan mode.
	defer func() {
		_, _ = conn.ExecContext(ctx, `DEALLOCATE q`)
		_, _ = conn.ExecContext(ctx, `RESET plan_cache_mode`)
	}()
	mode := "force_custom_plan"
	if generic {
		mode = "force_generic_plan"
	}
	if _, err := conn.ExecContext(ctx, `SET plan_cache_mode = `+mode); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `SET enable_seqscan = on`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `PREPARE q AS `+dbx.Rebind(query)); err != nil {
		t.Fatalf("prepare %q: %v", query, err)
	}
	literals := make([]string, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case string:
			literals[i] = "'" + strings.ReplaceAll(v, "'", "''") + "'"
		default:
			literals[i] = fmt.Sprint(v)
		}
	}
	rows, err := conn.QueryContext(ctx, `EXPLAIN EXECUTE q(`+strings.Join(literals, ", ")+`)`)
	if err != nil {
		t.Fatalf("explain %q: %v", query, err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(line + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return plan.String()
}

// I1 (literal): the predicate the readers spell out is the one the index has,
// and it names a kind no reader looks up.
func TestReceiptKindPredicateIsABatchKind(t *testing.T) {
	if want := fmt.Sprintf("event_kind <> '%s'", computerevent.EventProjectionBatchRecorded); receiptKindNotBatchSQL != want {
		t.Fatalf("predicate = %q, want %q", receiptKindNotBatchSQL, want)
	}
	for _, kind := range []computerevent.EventKind{computerevent.EventKeyRevoked, computerevent.EventGenesisImported, computerevent.EventMaterializationApplied, computerevent.EventRollbackRequested, computerevent.EventRollbackApplied} {
		if kind == computerevent.EventProjectionBatchRecorded {
			t.Fatalf("a reader looks up %s, which the partial index leaves out", kind)
		}
	}
}

// I1: every reader of event_kind uses an index, never a sequential scan, with
// the partial index in place of the full one, for custom and generic plans.
func TestReceiptKindReadersUseThePartialIndex(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	idx := receiptIndexNames(t, s)
	if !idx[receiptKindPartialIdx] || idx[receiptKindOldIdx] {
		t.Fatalf("indexes = %v, want the partial one and not the full one", idx)
	}
	loadReceiptRows(t, s, indexTestComputer, 300000)
	cases := []struct {
		name       string
		query      string
		args       []any
		wantKindIx bool // the partial index itself serves the query
	}{
		{"event_replay credential epoch", receiptKeyRevokedCountSQL, []any{indexTestComputer, 200000, string(computerevent.EventKeyRevoked)}, true},
		{"verifier_key_pin and checkpoints genesis", receiptFirstOfKindSQL, []any{indexTestComputer, string(computerevent.EventGenesisImported)}, true},
		{"clean_rebase latest applied", receiptLatestAppliedSQL, []any{indexTestComputer, string(computerevent.EventMaterializationApplied), string(computerevent.EventRollbackRequested), string(computerevent.EventRollbackApplied)}, false},
	}
	for _, tc := range cases {
		for _, generic := range []bool{false, true} {
			plan := explain(t, s, tc.query, generic, tc.args...)
			t.Logf("%s generic=%v:\n%s", tc.name, generic, plan)
			if strings.Contains(plan, "Seq Scan") || !strings.Contains(plan, "Index") {
				t.Errorf("%s generic=%v does not use an index:\n%s", tc.name, generic, plan)
			}
			if tc.wantKindIx && !strings.Contains(plan, receiptKindPartialIdx) {
				t.Errorf("%s generic=%v does not use %s:\n%s", tc.name, generic, receiptKindPartialIdx, plan)
			}
		}
	}
	// Why the readers spell the predicate out: without it a generic plan has
	// only a parameter for the kind and cannot use the partial index.
	bare := `SELECT sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_kind=? ORDER BY sequence LIMIT 1`
	if plan := explain(t, s, bare, true, indexTestComputer, string(computerevent.EventGenesisImported)); strings.Contains(plan, receiptKindPartialIdx) {
		t.Errorf("a generic plan without the predicate used the partial index; the readers may not need it:\n%s", plan)
	}
}

// I2, I3, I4, I5: the migration builds the partial index, drops the full one
// only afterwards, repairs an interrupted build, and is idempotent.
func TestEnsureReceiptIndexesMigratesALiveTable(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestPlatformStore(t)
	loadReceiptRows(t, s, indexTestComputer, 20000)
	// The state of the live host: the full index, no partial one.
	if _, err := s.db.Exec(`DROP INDEX ` + receiptKindPartialIdx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE INDEX ` + receiptKindOldIdx + ` ON computer_event_append_receipts (computer_id, event_kind, sequence)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE INDEX ` + receiptIDOldIdx + ` ON computer_event_append_receipts (computer_id, event_head_receipt_id)`); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureReceiptIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	idx := receiptIndexNames(t, s)
	if !idx[receiptKindPartialIdx] || idx[receiptKindOldIdx] || idx[receiptIDOldIdx] {
		t.Fatalf("after the migration: %v", idx)
	}
	// Smaller: only the rows of other kinds are in it.
	var partial, full int64
	if err := s.db.QueryRow(`SELECT pg_relation_size(?::regclass)`, receiptKindPartialIdx).Scan(&partial); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE INDEX probe_full ON computer_event_append_receipts (computer_id, event_kind, sequence)`); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT pg_relation_size('probe_full'::regclass)`).Scan(&full); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP INDEX probe_full`); err != nil {
		t.Fatal(err)
	}
	if partial*10 > full {
		t.Fatalf("partial index %d bytes, full %d: not far smaller", partial, full)
	}
	// Idempotent.
	if err := s.ensureReceiptIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	// An interrupted build leaves an invalid index; the next run rebuilds it,
	// and the full index is not dropped before a valid one exists.
	if _, err := s.db.Exec(`CREATE INDEX ` + receiptKindOldIdx + ` ON computer_event_append_receipts (computer_id, event_kind, sequence)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE pg_index SET indisvalid = false WHERE indexrelid = ?::regclass`, receiptKindPartialIdx); err != nil {
		t.Skipf("cannot mark an index invalid here: %v", err)
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if exists, valid, err := indexState(ctx, conn, receiptKindPartialIdx); err != nil || !exists || valid {
		t.Fatalf("setup: exists=%v valid=%v err=%v", exists, valid, err)
	}
	conn.Close()
	if err := s.ensureReceiptIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	conn, err = s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if exists, valid, err := indexState(ctx, conn, receiptKindPartialIdx); err != nil || !exists || !valid {
		t.Fatalf("after repair: exists=%v valid=%v err=%v", exists, valid, err)
	}
	if idx := receiptIndexNames(t, s); idx[receiptKindOldIdx] {
		t.Fatalf("the full index survived: %v", idx)
	}
}

// I5: a second runner that cannot take the lock does nothing.
func TestEnsureReceiptIndexesYieldsToARunningMigration(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestPlatformStore(t)
	if _, err := s.db.Exec(`DROP INDEX ` + receiptKindPartialIdx); err != nil {
		t.Fatal(err)
	}
	holder, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if _, err := holder.ExecContext(ctx, `SELECT pg_advisory_lock(?)`, receiptIndexLockKey); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureReceiptIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	if idx := receiptIndexNames(t, s); idx[receiptKindPartialIdx] {
		t.Fatal("a second runner built the index while another held the lock")
	}
}

// I7: no source outside tests filters the receipt table by receipt id, so its
// index has no reader.
func TestNoReaderLooksUpAReceiptByID(t *testing.T) {
	byID := regexp.MustCompile(`event_head_receipt_id\s*(=|IN\b|LIKE\b|ANY\b)`)
	root := filepath.Join("..", "..")
	var scanned int
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			if loc := byID.FindIndex(raw); loc != nil {
				t.Errorf("%s filters on event_head_receipt_id (%q); computer_event_append_receipt_id_idx is dropped", path, raw[loc[0]:loc[1]])
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned %d files; the scan is vacuous", scanned)
	}
}

// The head lookups read one row through the unique (computer, sequence) key
// with the receipt id index gone.
func TestHeadReceiptLookupsUseTheSequenceIndex(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	if idx := receiptIndexNames(t, s); idx[receiptIDOldIdx] {
		t.Fatalf("the receipt id index is still there: %v", idx)
	}
	loadReceiptRows(t, s, indexTestComputer, 50000)
	for _, query := range []string{receiptAtSequenceSQL, receiptDigestAtSequenceSQL} {
		for _, generic := range []bool{false, true} {
			plan := explain(t, s, query, generic, indexTestComputer, 25000)
			if !strings.Contains(plan, "computer_event_append_sequence_uq") || strings.Contains(plan, "Seq Scan") {
				t.Errorf("generic=%v: %s\n%s", generic, query, plan)
			}
		}
	}
}
