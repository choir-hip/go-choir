package platform

import (
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

const (
	indexTestComputer      = "computer-1355b83bced1613eb048ee28d66688a8" // the paper's shape: applied events early, others late
	indexTestOtherComputer = "computer-59c902c5d46caecc0a180a8557164e72" // none of the applied kinds at all
)

// loadReceiptRows fills the table the way the paper's is shaped: 99.2% of the
// rows projection batches, the rest spread over the other kinds, a genesis
// first, the applied and rollback kinds only early in the first computer's
// tape and absent from the second's.
func loadReceiptRows(t *testing.T, s *Store, computerID string, rows int, withApplied bool) {
	t.Helper()
	insert := func(kind string, from, to, step int) {
		_, err := s.db.Exec(`INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at)
			SELECT ?, 'k-' || ? || g, repeat('a', 32) || md5('c' || ? || g), g, ?, repeat('b', 32) || md5('d' || ? || g), repeat('c', 32) || md5('p' || ? || g), 'r-' || ? || g, '{}', now()
			FROM generate_series(?::bigint, ?::bigint, ?::bigint) g`, computerID, computerID, computerID, kind, computerID, computerID, computerID, from, to, step)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Batches everywhere, then the other kinds overwrite their sequences:
	// build the table by kind so no sequence repeats.
	others := []computerevent.EventKind{computerevent.EventArtifactProduced, computerevent.EventFileRootCommitted, computerevent.EventKeyRevoked}
	insert(string(computerevent.EventGenesisImported), 1, 1, 1)
	for i, kind := range others {
		insert(string(kind), 125+i, rows, 375) // about 0.8% of the rows in all
	}
	if withApplied {
		for i, kind := range []computerevent.EventKind{computerevent.EventMaterializationApplied, computerevent.EventRollbackRequested, computerevent.EventRollbackApplied} {
			insert(string(kind), 1000+i, 1000+i, 1)
		}
	}
	// Every sequence not taken is a batch.
	if _, err := s.db.Exec(`INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at)
		SELECT ?, 'k-' || ? || g, repeat('a', 32) || md5('c' || ? || g), g, 'projection_batch_recorded', repeat('b', 32) || md5('d' || ? || g), repeat('c', 32) || md5('p' || ? || g), 'r-' || ? || g, '{}', now()
		FROM generate_series(1::bigint, ?::bigint) g WHERE NOT EXISTS (SELECT 1 FROM computer_event_append_receipts r WHERE r.computer_id = ? AND r.sequence = g)`,
		computerID, computerID, computerID, computerID, computerID, computerID, rows, computerID); err != nil {
		t.Fatal(err)
	}
}

// kindIndexState rebuilds the table's kind indexes into one of the two states
// a release can meet: "full" (the old index only, as the live host has today)
// or "partial" (the partial index only, after the swap).
func kindIndexState(t *testing.T, s *Store, state string) {
	t.Helper()
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS ` + receiptKindOldIdx,
		`DROP INDEX IF EXISTS ` + receiptKindPartialIdx,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	switch state {
	case "full":
		if _, err := s.db.Exec(`CREATE INDEX ` + receiptKindOldIdx + ` ON computer_event_append_receipts (computer_id, event_kind, sequence)`); err != nil {
			t.Fatal(err)
		}
	case "partial":
		if _, err := s.db.Exec(`CREATE INDEX ` + receiptKindPartialIdx + ` ON computer_event_append_receipts (computer_id, event_kind, sequence) WHERE ` + receiptKindNotBatchSQL); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("state %q", state)
	}
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
	ctx := t.Context()
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

// I1 (literal): the predicate the readers spell out is the one the partial
// index has, and it names a kind no reader looks up.
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

// I1: every reader of event_kind seeks a kind index, never a sequential scan
// and never a walk of the computer's chain, in custom and generic plans, with
// the old full index alone and with the partial one alone, on two computers
// with the paper's density of other kinds (one of them without the applied
// kinds at all).
func TestReceiptKindReadersUseAKindIndex(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	loadReceiptRows(t, s, indexTestComputer, 200000, true)
	loadReceiptRows(t, s, indexTestOtherComputer, 200000, false)
	var nonBatch, total int
	if err := s.db.QueryRow(`SELECT count(*) FILTER (WHERE `+receiptKindNotBatchSQL+`), count(*) FROM computer_event_append_receipts`).Scan(&nonBatch, &total); err != nil {
		t.Fatal(err)
	}
	if density := float64(nonBatch) / float64(total); density < 0.006 || density > 0.012 {
		t.Fatalf("fixture density of other kinds = %.4f, want about 0.008", density)
	}
	for _, state := range []string{"full", "partial"} {
		kindIndexState(t, s, state)
		active := map[string]string{"full": receiptKindOldIdx, "partial": receiptKindPartialIdx}[state]
		for _, computerID := range []string{indexTestComputer, indexTestOtherComputer} {
			cases := []struct {
				name  string
				query string
				args  []any
			}{
				{"event_replay credential epoch", receiptKeyRevokedCountSQL, []any{computerID, 150000, string(computerevent.EventKeyRevoked)}},
				{"verifier_key_pin and checkpoints genesis", receiptFirstOfKindSQL, []any{computerID, string(computerevent.EventGenesisImported)}},
				{"clean_rebase latest applied", receiptLatestAppliedSQL, []any{
					computerID, string(computerevent.EventMaterializationApplied),
					computerID, string(computerevent.EventRollbackRequested),
					computerID, string(computerevent.EventRollbackApplied)}},
			}
			for _, tc := range cases {
				for _, generic := range []bool{false, true} {
					plan := explain(t, s, tc.query, generic, tc.args...)
					t.Logf("%s state=%s generic=%v:\n%s", tc.name, state, generic, plan)
					if strings.Contains(plan, "Seq Scan") || !strings.Contains(plan, active) || strings.Contains(plan, "computer_event_append_sequence_uq") {
						t.Errorf("%s state=%s generic=%v does not seek %s:\n%s", tc.name, state, generic, active, plan)
					}
				}
			}
		}
	}
	// Why the readers spell the predicate out: with the partial index alone, a
	// generic plan of the bare query has only a parameter for the kind and
	// cannot use it.
	bare := `SELECT sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_kind=? ORDER BY sequence LIMIT 1`
	if plan := explain(t, s, bare, true, indexTestComputer, string(computerevent.EventGenesisImported)); strings.Contains(plan, receiptKindPartialIdx) {
		t.Errorf("a generic plan without the predicate used the partial index; the readers may not need it:\n%s", plan)
	}
}

// The scan lints below cover the sources that can name the table: Go, shell,
// Python and nix files under internal, cmd, scripts and nix.
func sourceFiles(t *testing.T, visit func(path string, raw []byte)) {
	t.Helper()
	root := filepath.Join("..", "..")
	var scanned int
	for _, dir := range []string{"internal", "cmd", "scripts", "nix"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			name := d.Name()
			if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".md") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			visit(path, raw)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 200 {
		t.Fatalf("scanned %d files; the scan is vacuous", scanned)
	}
}

// I7: no source filters the receipt table by receipt id, so its index has no
// reader.
func TestNoReaderLooksUpAReceiptByID(t *testing.T) {
	byID := regexp.MustCompile(`event_head_receipt_id\s*(=|IN\b|LIKE\b|ANY\b)`)
	sourceFiles(t, func(path string, raw []byte) {
		if loc := byID.FindIndex(raw); loc != nil {
			t.Errorf("%s filters on event_head_receipt_id (%q); computer_event_append_receipt_id_idx has no reader to keep", path, raw[loc[0]:loc[1]])
		}
	})
}

// The head lookups read one row through the unique (computer, sequence) key,
// which no migration touches.
func TestHeadReceiptLookupsUseTheSequenceIndex(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	loadReceiptRows(t, s, indexTestComputer, 50000, true)
	if _, err := s.db.Exec(`ANALYZE computer_event_append_receipts`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{receiptAtSequenceSQL, receiptDigestAtSequenceSQL} {
		for _, generic := range []bool{false, true} {
			plan := explain(t, s, query, generic, indexTestComputer, 25000)
			if !strings.Contains(plan, "computer_event_append_sequence_uq") || strings.Contains(plan, "Seq Scan") {
				t.Errorf("generic=%v: %s\n%s", generic, query, plan)
			}
		}
	}
}

// I6: a freshly bootstrapped database has neither old index; it has the
// partial one, with the definition the readers and the supervised swap use,
// because its table has no rows.
func TestBootstrapCreatesThePartialIndexOnAFreshDatabase(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	idx := receiptIndexNames(t, s)
	if idx[receiptKindOldIdx] || idx[receiptIDOldIdx] {
		t.Errorf("Bootstrap left an old index on the table: %v", idx)
	}
	if !idx[receiptKindPartialIdx] || !idx["computer_event_append_sequence_uq"] {
		t.Fatalf("the partial and the unique sequence indexes are expected: %v", idx)
	}
	var def string
	if err := s.db.QueryRow(`SELECT pg_get_indexdef(?::regclass)`, receiptKindPartialIdx).Scan(&def); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(def, "(computer_id, event_kind, sequence)") || !strings.Contains(def, "projection_batch_recorded") {
		t.Fatalf("definition = %s", def)
	}
}

// Bootstrap never builds an index on a table that has rows: that is the
// supervised swap's job, concurrently and watched.
func TestBootstrapNeverBuildsAnIndexOnATableWithRows(t *testing.T) {
	ctx := t.Context()
	s, _ := openTestPlatformStore(t)
	loadReceiptRows(t, s, indexTestComputer, 100, true)
	if _, err := s.db.Exec(`DROP INDEX ` + receiptKindPartialIdx); err != nil {
		t.Fatal(err)
	}
	if err := s.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if idx := receiptIndexNames(t, s); idx[receiptKindPartialIdx] || idx[receiptKindOldIdx] || idx[receiptIDOldIdx] {
		t.Fatalf("Bootstrap built an index on a table with rows: %v", idx)
	}
}

// Two processes that boot a fresh database together build the index once and
// neither fails.
func TestBootstrapOfAFreshDatabaseIsNotRacedByASecondProcess(t *testing.T) {
	ctx := t.Context()
	s, _ := openTestPlatformStore(t)
	if _, err := s.db.Exec(`DROP INDEX ` + receiptKindPartialIdx); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { errs <- s.ensureFreshReceiptIndex(ctx) }()
	}
	for i := 0; i < 8; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if !receiptIndexNames(t, s)[receiptKindPartialIdx] {
		t.Fatal("no process built the index")
	}
}
