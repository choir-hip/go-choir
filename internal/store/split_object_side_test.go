package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Object side of the header/body split (design section 5): fingerprints stay
// byte-identical, the vocabulary migration and its revert keep the derived
// header columns and the bodies exact, the run-list readers return the same
// records as a full read, and the projector writes both tables.

// presplitFingerprintsSQL is the pre-split ListObjectGraphFingerprints og
// half, frozen.
const presplitFingerprintsSQL = `
		SELECT canonical_id, object_kind, version_id, content_hash, COALESCE(body, ''), metadata,
		       updated_at, tombstone, superseded_by
		  FROM og_objects
		 ORDER BY canonical_id ASC`

func fingerprintFixture() []objectgraph.Object {
	at := time.Date(2026, 11, 1, 3, 4, 5, 0, time.UTC)
	mk := func(id, kind, body, meta string, tomb bool, sup string) objectgraph.Object {
		o := objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.ObjectKind(kind), OwnerID: "o", ComputerID: "c",
			VersionID: "v", ContentHash: "h-" + id, Metadata: []byte(meta), CreatedAt: at, UpdatedAt: at.Add(time.Minute),
			Tombstone: tomb, SupersededBy: sup}
		if body != "\x00" {
			o.Body = []byte(body)
		}
		return o
	}
	return []objectgraph.Object{
		mk("obj:z", "choir.run", `{"a":1}`, `{"run_id":"r1"}`, false, ""),
		mk("obj:a", "choir.run", "\x00", `{}`, true, "obj:z"),
		mk("obj:m", "choir.commitment_record", `{"directive":"x"}`, `{}`, false, ""),
		mk("obj:e", "choir.event", `{"payload":{"phase":"p"}}`, `{"kind":"tool.result"}`, false, ""),
		mk("obj:t", "choir.source_ref", "not json", `{"k":"v"}`, false, ""),
		mk("obj:empty", "choir.agent", "", `{}`, false, ""),
	}
}

func fingerprintLines(t *testing.T, fps []ObjectGraphFingerprint) []string {
	t.Helper()
	var out []string
	for _, fp := range fps {
		if fp.Table == "" {
			out = append(out, fmt.Sprintf("%+v", fp))
		}
	}
	return out
}

func TestFingerprintsByteIdenticalAcrossSplit(t *testing.T) {
	ctx := context.Background()
	fixture := fingerprintFixture()

	// The old layout: the frozen pre-split schema, the frozen query.
	schema, err := os.ReadFile(filepath.Join("testdata", "presplit", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	oldDB, err := sql.Open("sqlite", dbx.SQLiteDSN(filepath.Join(t.TempDir(), "old.db"), dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	defer oldDB.Close()
	if _, err := oldDB.Exec(string(schema)); err != nil {
		t.Fatalf("frozen schema: %v", err)
	}
	for _, o := range fixture {
		if _, err := oldDB.Exec(`INSERT INTO og_objects
			(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			o.CanonicalID, string(o.ObjectKind), o.OwnerID, o.ComputerID, o.VersionID, o.ContentHash, o.Body, string(o.Metadata),
			o.CreatedAt, o.UpdatedAt, o.Tombstone, o.SupersededBy); err != nil {
			t.Fatalf("old put %s: %v", o.CanonicalID, err)
		}
	}
	rows, err := oldDB.Query(presplitFingerprintsSQL)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for rows.Next() {
		var (
			fp        ObjectGraphFingerprint
			body      []byte
			metadata  string
			updatedAt any
		)
		if err := rows.Scan(&fp.CanonicalID, &fp.ObjectKind, &fp.VersionID, &fp.ContentHash, &body, &metadata, &updatedAt, &fp.Tombstone, &fp.SupersededBy); err != nil {
			t.Fatal(err)
		}
		bs := sha256.Sum256(body)
		ms := sha256.Sum256([]byte(metadata))
		fp.BodyDigest, fp.MetadataDigest = hex.EncodeToString(bs[:]), hex.EncodeToString(ms[:])
		fp.UpdatedAt = fmt.Sprint(updatedAt)
		want = append(want, fmt.Sprintf("%+v", fp))
	}
	rows.Close()

	// The same rows in the new layout.
	s := openTestStore(t)
	for _, o := range fixture {
		if _, err := objectgraph.WriteObjectTx(ctx, s.db, o); err != nil {
			t.Fatalf("new put %s: %v", o.CanonicalID, err)
		}
	}
	fps, err := s.ListObjectGraphFingerprints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := fingerprintLines(t, fps)
	if len(want) != len(fixture) || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("fingerprints differ across the split\n new: %v\n old: %v", got, want)
	}
}

// derivedColumnsAgree recomputes og_rec_claim and event_phase from the joined
// body for every row and compares them with the stored header columns.
func derivedColumnsAgree(t *testing.T, s *Store) {
	t.Helper()
	claim, phase := objectgraph.BodyDerivedSQL("h.object_kind", "b.body", "h.metadata")
	q := `SELECT h.canonical_id, quote(h.og_rec_claim)||typeof(h.og_rec_claim)||'|'||quote(h.event_phase)||typeof(h.event_phase),
		quote(` + claim + `)||typeof(` + claim + `)||'|'||quote(` + phase + `)||typeof(` + phase + `)
		FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id`
	rows, err := s.db.Query(q)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id, stored, recomputed string
		if err := rows.Scan(&id, &stored, &recomputed); err != nil {
			t.Fatal(err)
		}
		n++
		if stored != recomputed {
			t.Errorf("%s: stored derived %s, recomputed %s", id, stored, recomputed)
		}
	}
	if n == 0 {
		t.Fatal("no rows")
	}
}

func objectBodies(t *testing.T, s *Store) []string {
	t.Helper()
	rows, err := s.db.Query(`SELECT h.obj_id, h.canonical_id, h.content_hash, h.metadata, b.body FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id ORDER BY h.obj_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id int64
		var cid, hash, meta string
		var body []byte
		if err := rows.Scan(&id, &cid, &hash, &meta, &body); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%d|%s|%s|%s|%x", id, cid, hash, meta, body))
	}
	return out
}

// The vocabulary migration rewrites headers and bodies in the split layout:
// obj_id survives, the derived columns follow the rewritten body, and the
// revert restores header and body exactly.
func TestVocabMigrateRevertRoundTripOnSplitLayout(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	put := func(kind, owner, key, body, meta string) string {
		metaNorm, err := objectgraph.NormalizeMetadata([]byte(meta))
		if err != nil {
			t.Fatal(err)
		}
		hash := objectgraph.ContentHash(objectgraph.ObjectKind(kind), []byte(body), metaNorm)
		var suffix string
		if key != "" {
			suffix = objectgraph.StableSuffixFromKey(key)
		} else {
			suffix = objectgraph.StableSuffixFromContent(hash)
		}
		id, err := objectgraph.BuildCanonicalID(objectgraph.ObjectKind(kind), owner, suffix)
		if err != nil {
			t.Fatal(err)
		}
		drillSeed(t, s, id, kind, owner, "computer", "v1", hash, body, string(metaNorm), now, now)
		return id
	}
	put("choir.agent", "owner", "computer\x00co-super:impl", `{"agent_id":"co-super:impl","profile":"co-super"}`, `{"agent_id":"co-super:impl","profile":"co-super"}`)
	put("choir.event", "owner", "", `{"actor_profile":"co-super","payload":{"phase":"started"}}`, `{"actor_profile":"co-super","kind":"tool.result"}`)
	put("choir.commitment_record", "owner", "", `{"actor_profile":"co-super","directive":"build"}`, `{"actor_profile":"co-super"}`)
	derivedColumnsAgree(t, s)

	before := objectBodies(t, s)
	idsBefore := map[string]int64{}
	rows, err := s.db.Query(`SELECT canonical_id, obj_id FROM og_objects`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var c string
		var id int64
		_ = rows.Scan(&c, &id)
		idsBefore[c] = id
	}
	rows.Close()

	rep, err := s.MigrateVocabularyToV2(ctx)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(rep.OGObjects) == 0 {
		t.Fatal("nothing migrated")
	}
	if after := objectBodies(t, s); strings.Join(after, "\n") == strings.Join(before, "\n") {
		t.Fatal("the migration changed no header or body")
	}
	derivedColumnsAgree(t, s)
	for _, e := range rep.OGObjects {
		var id int64
		if err := s.db.QueryRow(`SELECT obj_id FROM og_objects WHERE canonical_id = ?`, e.NewCanonicalID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if id != idsBefore[e.OldCanonicalID] {
			t.Errorf("%s: obj_id %d after migration, %d before", e.NewCanonicalID, id, idsBefore[e.OldCanonicalID])
		}
	}
	var orphans int
	if err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM og_bodies WHERE obj_id NOT IN (SELECT obj_id FROM og_objects)) + (SELECT COUNT(*) FROM og_objects WHERE obj_id NOT IN (SELECT obj_id FROM og_bodies))`).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("orphans after migration: %d, %v", orphans, err)
	}

	if err := s.RevertVocabularyToV1(ctx, rep); err != nil {
		t.Fatalf("revert: %v", err)
	}
	derivedColumnsAgree(t, s)
	if after := objectBodies(t, s); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("revert is not byte-identical\n got %v\nwant %v", after, before)
	}
}

// The run-list readers go through ListObjectsByKindOwnerWhere (headers, keep,
// then bodies of the kept rows); they return the records a full read of
// every run returns, in the same order.
func TestRunListReadersMatchAFullRead(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	owner := "owner-split-lists"
	states := []types.RunState{types.RunPending, types.RunRunning, types.RunCompleted, types.RunPending, types.RunCompleted, types.RunRunning}
	var all []types.RunRecord
	for i, st := range states {
		rec := types.RunRecord{RunID: fmt.Sprintf("run-%d", i), AgentID: fmt.Sprintf("agent-%d", i), OwnerID: owner,
			ComputerID: "autoputer-1", AgentProfile: "management", AgentRole: "management", State: st,
			CreatedAt: now.Add(time.Duration(i) * time.Minute), UpdatedAt: now.Add(time.Duration(i) * time.Minute)}
		if err := s.CreateRunOG(ctx, rec); err != nil {
			t.Fatal(err)
		}
		all = append(all, rec)
	}
	got, err := s.ListRunsByOwnerStates(ctx, owner, "autoputer-1", []types.RunState{types.RunPending, types.RunRunning}, 16)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, r := range all {
		if r.State == types.RunPending || r.State == types.RunRunning {
			want = append(want, r.RunID)
		}
	}
	var ids []string
	for _, r := range got {
		ids = append(ids, r.RunID)
	}
	sort.Strings(ids)
	sort.Strings(want)
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("runs by state = %v, want %v", ids, want)
	}
	for _, r := range got {
		full, err := s.GetRun(ctx, r.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if full.State != r.State || full.AgentID != r.AgentID || !full.UpdatedAt.Equal(r.UpdatedAt) {
			t.Errorf("list record %+v differs from full read %+v", r, full)
		}
	}
}

// The replay projector writes the header and the body in one transaction.
func TestProjectObjectWritesHeaderAndBody(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	stmts := s.projectionStmts(ctx)
	for _, with := range []*projectStmts{nil, stmts} {
		id := "obj:choir.commitment_record:o:plain"
		if with != nil {
			id = "obj:choir.commitment_record:o:prepared"
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		raw := fmt.Sprintf(`{"canonical_id":%q,"object_kind":"choir.commitment_record","owner_id":"o","content_hash":"h","body":"eyJkaXJlY3RpdmUiOiJ4In0=","metadata":{},"created_at":%q,"updated_at":%q}`, id, at.Format(time.RFC3339), at.Format(time.RFC3339))
		if err := projectObject(ctx, tx, []byte(raw), nil, with); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		var body string
		var claim sql.NullInt64
		if err := s.db.QueryRow(`SELECT CAST(b.body AS TEXT), h.og_rec_claim FROM og_objects h JOIN og_bodies b ON b.obj_id = h.obj_id WHERE h.canonical_id = ?`, id).Scan(&body, &claim); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if body != `{"directive":"x"}` || !claim.Valid {
			t.Fatalf("%s: body %q claim %v", id, body, claim)
		}
	}
}
