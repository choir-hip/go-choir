package platform

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The term search's covering index and the two timers of SearchCaptures.
// Failure modes, written before the code:
//   C1. the index is missing after the migration, or not valid;
//   C2. an interrupted build leaves an invalid index that is never repaired,
//       or the repair is not a rebuild, or a valid index is rebuilt each time;
//   C3. the real statement does not read the postings from the index alone
//       (heap fetches come back), so the search stays slow;
//   C4. with the index the search answers differently (ids, order, matched)
//       from without it;
//   C5. the index is built in a way that blocks capture writes (not
//       CONCURRENTLY, or inside a transaction);
//   C7. the index is built on a table that was never vacuumed, so the scan
//       still fetches every posting's heap page (no gain until a vacuum);
//   C6. the two journal lines nest: the terms line contains the text search.

const coverFixtureCaptures = 6000

// coverFixture loads captures straight into the projection tables and
// og_objects (the same rows the writers produce, without the object graph's
// per-object cost), with a few common words and a tail of rare ones.
func coverFixture(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()
	db := store.corpus()
	exec := func(q string, args ...any) {
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("fixture: %v\n%s", err, q)
		}
	}
	exec(fmt.Sprintf(`INSERT INTO proj_capture_index (capture_id, owner_id, computer_id, url_key, canonical_key, url, canonical_url, host, source_id, language, title, fetched_at, updated_at)
		SELECT 'obj:choir.web_capture:o:' || lpad(g::text, 8, '0'), 'o', 'c', 'u' || g, 'k' || g, 'https://h.example/' || g, 'https://h.example/' || g, 'h.example', 's', 'en',
			'alpha' || (g %% 3) || ' beta' || (g %% 11) || ' gamma' || (g %% 101) || ' delta' || (g %% 997), now() - (g || ' minutes')::interval, now()
		FROM generate_series(1, %d) g`, coverFixtureCaptures))
	for _, col := range []struct{ prefix, mod string }{{"alpha", "3"}, {"beta", "11"}, {"gamma", "101"}, {"delta", "997"}} {
		exec(fmt.Sprintf(`INSERT INTO proj_capture_terms (owner_id, computer_id, term, capture_id, fetched_at)
			SELECT 'o', 'c', '%s' || (g %% %s), 'obj:choir.web_capture:o:' || lpad(g::text, 8, '0'), now() - (g || ' minutes')::interval FROM generate_series(1, %d) g`, col.prefix, col.mod, coverFixtureCaptures))
	}
	exec(fmt.Sprintf(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata, created_at, updated_at)
		SELECT capture_id, 'choir.web_capture', 'o', 'c', 'v', 'h' || capture_id, '{}', fetched_at, fetched_at FROM proj_capture_index LIMIT %d`, coverFixtureCaptures))
	store.retrievalState.captureIndexReady.Store(true)
	store.retrievalState.captureIndexComplete.Store(true)
}

func coverIndexState(t *testing.T, store *Store) (exists, valid bool) {
	t.Helper()
	var v *bool
	err := store.corpus().QueryRowContext(context.Background(), `SELECT i.indisvalid FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid WHERE c.relname = ?`, captureTermsCoverIdx).Scan(&v)
	if err != nil {
		return false, false
	}
	return true, v != nil && *v
}

func TestCaptureTermsCoverIndexIsBuiltByTheMigrationAndRepaired(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()
	if ex, va := coverIndexState(t, store); !ex || !va {
		t.Fatalf("after MigrateRetrieval: exists=%v valid=%v, want a valid index", ex, va)
	}
	if !strings.Contains(captureTermsCoverDDL, "CREATE INDEX CONCURRENTLY") || !strings.Contains(captureTermsCoverDDL, "INCLUDE (fetched_at)") {
		t.Fatalf("the build must be concurrent and cover fetched_at: %s", captureTermsCoverDDL)
	}
	// A valid index is left alone: its identity (oid) does not change.
	oid := func() (o uint32) {
		if err := store.corpus().QueryRowContext(ctx, `SELECT ?::regclass::oid`, captureTermsCoverIdx).Scan(&o); err != nil {
			t.Fatal(err)
		}
		return
	}
	before := oid()
	if err := store.ensureCaptureTermsCoverIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if oid() != before {
		t.Fatal("a valid index was rebuilt")
	}
	// An interrupted build leaves the index invalid (marked here as the
	// server marks it); the next call drops and rebuilds it.
	if _, err := store.corpus().ExecContext(ctx, `UPDATE pg_index SET indisvalid = false WHERE indexrelid = ?::regclass`, captureTermsCoverIdx); err != nil {
		t.Skipf("cannot mark an index invalid here: %v", err)
	}
	if ex, va := coverIndexState(t, store); !ex || va {
		t.Fatalf("setup: exists=%v valid=%v", ex, va)
	}
	if err := store.ensureCaptureTermsCoverIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if ex, va := coverIndexState(t, store); !ex || !va {
		t.Fatalf("after repair: exists=%v valid=%v, want a valid index", ex, va)
	}
	if oid() == before {
		t.Fatal("an invalid index was not rebuilt")
	}
	// A missing index is built.
	if _, err := store.corpus().ExecContext(ctx, `DROP INDEX `+captureTermsCoverIdx); err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateRetrieval(ctx); err != nil {
		t.Fatal(err)
	}
	if ex, va := coverIndexState(t, store); !ex || !va {
		t.Fatalf("after migration of a database without it: exists=%v valid=%v", ex, va)
	}
	// Without the projection tables there is nothing to index and no error.
	store.retrievalState.captureIndexReady.Store(false)
	if err := store.ensureCaptureTermsCoverIndex(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureTermSearchReadsPostingsFromTheCoveringIndexAndAnswersTheSame(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()
	// The state before the change: bulk-loaded rows, never vacuumed, no index.
	if _, err := store.corpus().ExecContext(ctx, `DROP INDEX `+captureTermsCoverIdx); err != nil {
		t.Fatal(err)
	}
	coverFixture(t, store)
	if err := store.ensureCaptureTermsCoverIndex(ctx); err != nil {
		t.Fatal(err)
	}
	reqs := []CaptureSearchRequest{
		{OwnerID: "o", ComputerID: "c", Words: []string{"alpha1", "beta4", "gamma7"}, Need: 2, Candidates: 40},
		{OwnerID: "o", ComputerID: "c", Words: []string{"alpha2", "delta33"}, Need: 1, Candidates: 25},
		{OwnerID: "o", ComputerID: "c", Words: []string{"gamma5", "beta5", "alpha0", "delta5"}, Need: 3, Candidates: 500},
		{OwnerID: "o", ComputerID: "c", Words: []string{"nothinghere"}, Need: 1},
	}
	type answer struct {
		ids     []string
		matched int
		capped  bool
	}
	ask := func() []answer {
		var out []answer
		for _, r := range reqs {
			resp, err := store.SearchCaptures(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			var a answer
			for _, o := range resp.Objects {
				a.ids = append(a.ids, o.CanonicalID)
			}
			a.matched, a.capped = resp.Matched, resp.Capped
			out = append(out, a)
		}
		return out
	}
	with := ask()
	if len(with[0].ids) == 0 || with[0].matched == 0 {
		t.Fatalf("fixture answers nothing: %+v", with[0])
	}

	// C3: the real statement is an index-only scan of the new index.
	query, args := captureTermQuery("o", "c", reqs[0].Words, reqs[0].Need)
	rows, err := store.corpus().QueryContext(ctx, "EXPLAIN (ANALYZE, COSTS OFF) "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	var plan []string
	for rows.Next() {
		var l string
		_ = rows.Scan(&l)
		plan = append(plan, l)
	}
	rows.Close()
	text := strings.Join(plan, "\n")
	if !strings.Contains(text, "Index Only Scan using "+captureTermsCoverIdx) || strings.Contains(text, "Bitmap Heap Scan") || strings.Contains(text, "Seq Scan") {
		t.Fatalf("the term search does not read from the covering index alone:\n%s", text)
	}
	if !strings.Contains(text, "Heap Fetches: 0") {
		t.Fatalf("index-only scan with heap fetches (not vacuumed, or the index does not cover):\n%s", text)
	}

	// C4: dropped, the search answers the same through the primary key.
	if _, err := store.corpus().ExecContext(ctx, `DROP INDEX `+captureTermsCoverIdx); err != nil {
		t.Fatal(err)
	}
	without := ask()
	if !reflect.DeepEqual(with, without) {
		t.Fatalf("answers differ with and without the index:\nwith    %+v\nwithout %+v", with, without)
	}
}

func TestSearchCaptureTimersDoNotNest(t *testing.T) {
	f := newFTFixture(t, true)
	now := time.Now().UTC()
	f.put(t, f.capture(t, "owner-a", "computer-a", "k1", "Harbor reopens", "https://example.com/a", "The harbor reopened after the storm.", now))
	f.store.indexCaptureText(context.Background(), nil)
	if err := f.store.ReconcileCaptureIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	const slow = 150 * time.Millisecond
	captureFullTextSearchHook = func() { time.Sleep(slow) }
	t.Cleanup(func() { captureFullTextSearchHook = nil })
	before := f.store.RetrievalSnapshot()
	if _, err := f.store.SearchCaptures(context.Background(), CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Query: "harbor", Words: []string{"harbor"}, Need: 1}); err != nil {
		t.Fatal(err)
	}
	after := f.store.RetrievalSnapshot()
	terms := after.Ops[retrievalCaptureSearch]
	text := after.Ops[retrievalFullTextSearch]
	if terms.Calls-before.Ops[retrievalCaptureSearch].Calls != 1 || text.Calls-before.Ops[retrievalFullTextSearch].Calls != 1 {
		t.Fatalf("each timer records once: terms %+v text %+v", terms, text)
	}
	if text.MaxMS < float64(slow.Milliseconds()) {
		t.Fatalf("the slow text half was not timed by its own line: %+v", text)
	}
	if terms.MaxMS >= float64(slow.Milliseconds()) {
		t.Fatalf("the terms line (%.0f ms) contains the text search (%.0f ms): the timers nest", terms.MaxMS, text.MaxMS)
	}
}
