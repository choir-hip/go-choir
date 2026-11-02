package objectgraph

// The derived-key marks and the validator's columns at the object-graph layer
// (docs/design/derived-key-marks-2026-11-01.md, "Ways it can fail"; the
// store-level cases drive the real Open, finalize, replay and rebase in
// internal/store/derived_key_marks_test.go).

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

func bindTestHead(store *SQLStore, head *string) {
	store.derived.BindMarkHead(func(context.Context, DerivedKeyTx) (string, error) { return *head, nil })
}

func markRow(t *testing.T, db *sql.DB, family string) (identity, head string, ok bool) {
	t.Helper()
	err := db.QueryRow(`SELECT owner_id, content_hash FROM `+DerivedKeysTable+` WHERE canonical_id = '' AND key_name = ?`, family).Scan(&identity, &head)
	if err == sql.ErrNoRows {
		return "", "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return identity, head, true
}

func seedEveryFamily(t *testing.T, store *SQLStore, n int, body func(kind ObjectKind, i int) string) {
	t.Helper()
	ctx := context.Background()
	at := time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		for _, k := range []struct {
			kind ObjectKind
			meta map[string]any
		}{
			{testRunKind, map[string]any{"agent_id": fmt.Sprintf("a%d", i%3)}},
			{testSourceRefKind, map[string]any{"doc_id": fmt.Sprintf("d%d", i%3)}},
			{ActorWakeOutboxKind, map[string]any{"projected": "false"}},
			{commitmentRecordKind, map[string]any{}},
		} {
			id, err := BuildCanonicalID(k.kind, "o1", fmt.Sprintf("k-%04d", i))
			if err != nil {
				t.Fatal(err)
			}
			if err := store.PutObject(ctx, keyedObj{id: id, kind: k.kind, meta: k.meta, body: body(k.kind, i), at: at.Add(time.Duration(i) * time.Second)}.object(t)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// Case 7 (the pass's half): a pass deletes the family's mark before it reads
// a page, so a crash in the pass leaves no mark, and stamps it when it
// publishes.
func TestPassDeletesTheMarkWhenItStartsAndStampsWhenItPublishes(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	head := "head-1"
	bindTestHead(store, &head)
	seedEveryFamily(t, store, 3, func(ObjectKind, int) string { return `{"addressee":"x"}` })
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	for _, fam := range derivedFamilies {
		identity, h, ok := markRow(t, db, fam.Name)
		if !ok || h != "head-1" || identity != fam.markIdentity() {
			t.Fatalf("after the pass %s mark = %q %q %t", fam.Name, identity, h, ok)
		}
	}
	var seenDuring bool
	store.derived.beforePublish = once(KeyRunAgentID, func() {
		_, _, seenDuring = markRow(t, db, KeyRunAgentID)
	})
	head = "head-2"
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	if seenDuring {
		t.Fatal("the run mark was present while its pass ran")
	}
	if _, h, ok := markRow(t, db, KeyRunAgentID); !ok || h != "head-2" {
		t.Fatalf("after the second pass the run mark names %q (%t)", h, ok)
	}
}

// Case 10: the mark is never an answer, and this binary's pass never treats
// it as a stray.
func TestMarkIsNeverAReadAnswerOrAStray(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	head := "head-1"
	bindTestHead(store, &head)
	seedEveryFamily(t, store, 2, func(ObjectKind, int) string { return `{"addressee":""}` })
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	for _, fam := range derivedFamilies {
		if _, _, ok := markRow(t, db, fam.Name); !ok {
			t.Fatalf("premise: no %s mark", fam.Name)
		}
	}
	// The values a mark row carries in its other columns.
	for _, v := range []string{"", "head-1", derivedFamilies[0].markIdentity()} {
		ids, err := pageIDs(t, store, testRunKind, "agent_id", v, 50)
		if err != nil || len(ids) != 0 {
			t.Fatalf("agent_id %q answers %v, %v", v, ids, err)
		}
	}
	var lines []string
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{Logf: func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }}); err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		if strings.Contains(l, "outside") || strings.Contains(l, "orphans dropped") && !strings.Contains(l, "0 orphans dropped") {
			t.Fatalf("the pass saw the mark as a stray or an orphan: %s", l)
		}
	}
	if _, _, ok := markRow(t, db, KeyCommitmentAddressee); !ok {
		t.Fatal("the second pass lost the addressee mark")
	}
	ids, err := bodyIDs(t, store, "o1", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if id == derivedKeyMarkID {
			t.Fatal("the addressee read answered the mark")
		}
	}
}

// Case 11: a binary from before the marks (G1) runs its pass after a
// rollback; its stray phase, which has no canonical_id ” exclusion, finds
// the mark and drops it as an orphan. This is G1's query text and reconcile.
func TestOlderPassDropsTheMarkAsAnOrphan(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	head := "head-1"
	bindTestHead(store, &head)
	seedEveryFamily(t, store, 1, func(ObjectKind, int) string { return `{}` })
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	fam := derivedFamilyByName(KeyRunAgentID)
	lo, hi := derivedKeyRange(fam.Kind)
	var strays []string
	rows, err := db.Query(`SELECT canonical_id FROM `+DerivedKeysTable+` WHERE key_name = ? AND (canonical_id <= ? OR canonical_id >= ?)`, fam.Name, lo, hi)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		strays = append(strays, id)
	}
	rows.Close()
	if len(strays) != 1 || strays[0] != derivedKeyMarkID {
		t.Fatalf("G1's stray query finds %q, want the mark", strays)
	}
	if _, _, dropped, _, err := store.validateIDs(ctx, fam, strays); err != nil || dropped != 1 {
		t.Fatalf("G1's reconcile dropped %d (%v), want the mark", dropped, err)
	}
	if _, _, ok := markRow(t, db, KeyRunAgentID); ok {
		t.Fatal("the mark survived G1's stray reconcile")
	}
}

// overflowPages lists og_bodies' overflow pages.
func overflowPages(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	rows, err := db.Query(`SELECT pageno FROM dbstat WHERE name = 'og_bodies' AND pagetype = 'overflow'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var p int64
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

// Case 12: the three metadata families validate without reading any body
// overflow page. Every metadata-family row has a body that overflows, and
// every og_bodies overflow page is then zeroed: a pass that read one would
// fail with "database disk image is malformed". The commitment records have
// small bodies (their family derives from the body). With the header/body
// split every header column reads with the bodies' overflow zeroed.
func TestMetadataFamiliesValidateWithoutReadingBodyOverflow(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	big := `{"text":"` + strings.Repeat("z", 20000) + `"}`
	seedEveryFamily(t, store, 30, func(kind ObjectKind, i int) string {
		if kind == commitmentRecordKind {
			return mustJSON(map[string]any{"addressee": fmt.Sprintf("a%d", i%4)})
		}
		return big
	})
	pages := overflowPages(t, db)
	if len(pages) < 90 {
		t.Fatalf("premise: %d overflow pages, want at least one per big body", len(pages))
	}
	var pageSize int
	if err := db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	for _, p := range pages {
		if _, err := db.Exec(`UPDATE sqlite_dbpage SET data = zeroblob(?) WHERE pgno = ?`, pageSize, p); err != nil {
			t.Fatalf("zero page %d: %v", p, err)
		}
	}
	runID, _ := BuildCanonicalID(testRunKind, "o1", "k-0000")
	var updated string
	if err := db.QueryRow(`SELECT updated_at FROM og_objects WHERE canonical_id = ?`, runID).Scan(&updated); err != nil {
		t.Fatalf("a header column does not read with the body overflow zeroed: %v", err)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 7}); err != nil {
		t.Fatalf("a pass read a body overflow page: %v", err)
	}
	for _, fam := range derivedFamilies {
		if !store.derived.Validated(fam.Name) {
			t.Fatalf("%s not validated", fam.Name)
		}
	}
	// The read pool takes the same path.
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 7, ReadDB: db}); err != nil {
		t.Fatalf("a read-pool pass read a body overflow page: %v", err)
	}
}

// Case 13: a stale updated_at on a key row trips a metadata family's read no
// more (it is not part of that answer, and the validator no longer compares
// it, so a trip would loop), and still trips the ordered addressee family.
func TestTripwireComparesUpdatedAtOnlyForTheOrderedFamily(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	seedEveryFamily(t, store, 2, func(kind ObjectKind, i int) string {
		return mustJSON(map[string]any{"addressee": "a"})
	})
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE `+DerivedKeysTable+` SET updated_at = ? WHERE canonical_id <> ''`, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	ids, err := pageIDs(t, store, testRunKind, "agent_id", "a0", 50)
	if err != nil || len(ids) != 1 {
		t.Fatalf("run read = %v, %v", ids, err)
	}
	if !store.derived.Validated(KeyRunAgentID) || store.derived.Mismatches(KeyRunAgentID) != 0 {
		t.Fatal("a stale updated_at withdrew the run family")
	}
	if _, err := bodyIDs(t, store, "o1", "a", 50); err != nil {
		t.Fatal(err)
	}
	if store.derived.Validated(KeyCommitmentAddressee) || store.derived.Mismatches(KeyCommitmentAddressee) == 0 {
		t.Fatal("a stale updated_at did not withdraw the ordered addressee family")
	}
	// And the validator repairs the ordered family's updated_at.
	var lines []string
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{OnlyNeeded: true, Logf: func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, KeyCommitmentAddressee+": validated 2 rows") || !strings.Contains(joined, "(2 re-derived") {
		t.Fatalf("the addressee pass did not repair both rows:\n%s", joined)
	}
	if strings.Contains(joined, KeyRunAgentID+": validation pass starts") {
		t.Fatalf("OnlyNeeded re-ran a validated family:\n%s", joined)
	}
}

// Case 15 (the probe's half): the open's malformed-row probe reads the
// partial index of malformed rows only, whatever the family's size.
func TestBadRowProbeUsesThePartialIndex(t *testing.T) {
	db, store := openDerivedStore(t)
	seedEveryFamily(t, store, 50, func(ObjectKind, int) string { return `{"addressee":"a"}` })
	plan := explainPlan(t, db, derivedKeyBadProbeSQL, KeyRunAgentID)
	if !strings.Contains(plan, derivedKeyBadIndex) || strings.Contains(plan, "SCAN") && !strings.Contains(plan, "INDEX") {
		t.Fatalf("the probe does not search the partial index:\n%s", plan)
	}
	var one int
	if err := db.QueryRow(derivedKeyBadProbeSQL, KeyRunAgentID).Scan(&one); err != sql.ErrNoRows {
		t.Fatalf("a clean family's probe = %d, %v", one, err)
	}
	if err := store.PutObject(context.Background(), keyedObj{id: "run-bad", kind: testRunKind, meta: map[string]any{}, body: `{}`, at: time.Now()}.object(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE ` + DerivedKeysTable + ` SET derive_ok = 0 WHERE canonical_id = 'run-bad'`); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(derivedKeyBadProbeSQL, KeyRunAgentID).Scan(&one); err != nil {
		t.Fatalf("the probe missed a malformed row: %v", err)
	}
}

// Case 14 (the failure half): when the tripwire's durable delete fails, the
// family stays withdrawn in memory and the read still answers.
func TestTripwireWhoseMarkDeleteFailsStaysWithdrawn(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	head := "head-1"
	bindTestHead(store, &head)
	at := time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)
	k := keyedObj{id: "run-1", kind: testRunKind, meta: map[string]any{"agent_id": "a"}, body: `{}`, at: at}
	if err := store.PutObject(ctx, k.object(t)); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	k.meta = map[string]any{"agent_id": "b"}
	rawOldBinaryPut(t, db, k.object(t))
	broken, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	_ = broken.Close()
	store.derived.markDB.Store(broken)
	// The stale key row answers "a"; its object now says "b": the tripwire
	// fires and the JSON path answers (nothing).
	ids, err := pageIDs(t, store, testRunKind, "agent_id", "a", 10)
	if err != nil || len(ids) != 0 {
		t.Fatalf("the read after the tripwire = %v, %v", ids, err)
	}
	if store.derived.Mismatches(KeyRunAgentID) == 0 {
		t.Fatal("premise: the tripwire did not fire")
	}
	if store.derived.Validated(KeyRunAgentID) {
		t.Fatal("a failed mark delete left the family validated")
	}
}
