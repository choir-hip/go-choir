package objectgraph

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// The pass compares on the read pool and takes the write connection only to
// repair. On the paper (2026-10-30) a pass that compared on the write
// connection ran at 47 rows a second: each 200-row page waited about a second
// for the busy writer, and the duty-cycle pause then slept three times that.
// Ways the read-pool pass could fail:
//  1. a page that needs no repair still takes the write connection, so the
//     pass stalls behind a busy writer (here: a writer that never lets go);
//  2. a missing, stale or orphan key row seen on the read pool is not
//     repaired, because the repair never runs or runs on the read handle;
//  3. a family is published validated while a repair it needed failed;
//  4. the stray-id scan or the row count still runs on the write connection.
func TestValidatorComparesOnTheReadPool(t *testing.T) {
	ctx := context.Background()
	write, read := openTestDBs(t)
	store := NewSQLStore(write)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	var all []keyedObj
	for i := 0; i < 300; i++ {
		all = append(all,
			keyedObj{id: fmt.Sprintf("run-%03d", i), kind: testRunKind, meta: map[string]any{"agent_id": fmt.Sprintf("a%d", i%6)}, body: `{}`, at: at},
			keyedObj{id: fmt.Sprintf("rec-%03d", i), kind: commitmentRecordKind, meta: map[string]any{}, body: mustJSON(map[string]any{"addressee": fmt.Sprintf("a%d", i%6)}), at: at})
	}
	for _, k := range all {
		if err := store.PutObject(ctx, k.object(t)); err != nil {
			t.Fatal(err)
		}
	}

	// 1 and 4: a consistent store validates while the write connection is
	// held for the whole pass.
	conn, err := write.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	held, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	passCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	err = store.ValidateDerivedKeys(passCtx, DerivedKeyValidation{PageSize: 32, ReadDB: read})
	cancel()
	_ = held.Rollback()
	_ = conn.Close()
	if err != nil {
		t.Fatalf("a pass with nothing to repair waited for the write connection: %v", err)
	}
	for _, name := range []string{KeyRunAgentID, KeyCommitmentAddressee} {
		if !store.derived.Validated(name) {
			t.Fatalf("family %s not validated after the read-pool pass", name)
		}
	}

	// 2 and 3: an older binary's writes and a deleted row are repaired
	// through the write connection.
	changed := all[0]
	changed.meta = map[string]any{"agent_id": "moved"}
	changed.at = at.Add(time.Hour)
	rawOldBinaryPut(t, write, changed.object(t))
	rawOldBinaryPut(t, write, keyedObj{id: "run-new", kind: testRunKind, meta: map[string]any{"agent_id": "a1"}, body: `{}`, at: at.Add(2 * time.Hour)}.object(t))
	if _, err := write.Exec(`DELETE FROM og_objects WHERE canonical_id = ?`, all[3].object(t).CanonicalID); err != nil {
		t.Fatal(err)
	}
	forward := NewSQLStore(write)
	if err := forward.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := forward.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 32, ReadDB: read}); err != nil {
		t.Fatal(err)
	}
	assertSideMatchesTruth(t, write)
	got, err := pageIDs(t, forward, testRunKind, "agent_id", "moved", 8)
	if err != nil || len(got) != 1 || !forward.derived.Validated(KeyRunAgentID) {
		t.Fatalf("read after the repairing pass = %v, %v (validated %t)", got, err, forward.derived.Validated(KeyRunAgentID))
	}
}
