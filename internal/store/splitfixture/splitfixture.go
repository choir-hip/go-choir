// Package splitfixture builds a pre-split store file and its split-layout
// copy with raw SQL, for the tests of the header/body split's witness and
// differential (docs/design/store-header-body-split-2026-11-01.md). It
// depends on no store writer: the old file executes the frozen pre-split
// DDL (internal/store/testdata/presplit/schema.sql) and the split file
// executes the objectgraph header and body DDL plus the split choir_events
// DDL, then copies the rows the way the migration does.
package splitfixture

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// SplitEventsDDL is the split choir_events layout (design section 2).
const SplitEventsDDL = `
CREATE TABLE choir_events (
	ev_id INTEGER PRIMARY KEY,
	tape_seq INTEGER NOT NULL,
	op_index INTEGER NOT NULL,
	canonical_id TEXT NOT NULL,
	object_kind TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	version_id TEXT NOT NULL DEFAULT '',
	content_hash TEXT NOT NULL,
	metadata TEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone INTEGER NOT NULL DEFAULT 0,
	superseded_by TEXT NOT NULL DEFAULT '',
	run_id TEXT NOT NULL DEFAULT '',
	seq INTEGER NOT NULL DEFAULT 0,
	stream_seq INTEGER NOT NULL DEFAULT 0,
	trajectory_id TEXT NOT NULL DEFAULT '',
	event_kind TEXT NOT NULL DEFAULT '',
	event_phase TEXT NOT NULL DEFAULT '',
	UNIQUE (tape_seq, op_index)
);
CREATE TABLE choir_event_bodies (
	ev_id INTEGER PRIMARY KEY,
	body  BLOB NOT NULL
);
CREATE TRIGGER choir_events_drop_body AFTER DELETE ON choir_events
BEGIN DELETE FROM choir_event_bodies WHERE ev_id = OLD.ev_id; END;
CREATE INDEX idx_choir_events_kind_updated_phase ON choir_events(event_kind, updated_at, event_phase);
`

// OpenRW opens path for writing with the store's DSN.
func OpenRW(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func presplitSchema() (string, error) {
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "testdata", "presplit", "schema.sql"))
	return string(b), err
}

const ogInsert = `INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
	VALUES (?, ?, 'o1', 'c1', 'v1', ?, ?, ?, ?, ?, ?, '')`

const evInsert = `INSERT INTO choir_events (tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind)
	VALUES (?, ?, ?, 'choir.event', 'o1', 'c1', 'v1', ?, ?, ?, ?, ?, 0, '', 'r1', ?, ?, 't1', ?)`

// Build writes dir/state.db.presplit (old layout) and dir/state.db (split
// layout) holding the same rows, and returns both paths. Row times are
// 2026-10-29 and 2026-10-30.
func Build(dir string) (oldPath, newPath string, err error) {
	oldPath = filepath.Join(dir, "state.db.presplit")
	newPath = filepath.Join(dir, "state.db")
	schema, err := presplitSchema()
	if err != nil {
		return "", "", err
	}
	odb, err := OpenRW(oldPath)
	if err != nil {
		return "", "", err
	}
	defer odb.Close()
	if _, err := odb.Exec(schema); err != nil {
		return "", "", fmt.Errorf("presplit schema: %w", err)
	}
	if err := fill(odb); err != nil {
		return "", "", err
	}
	if _, err := odb.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return "", "", err
	}

	ndb, err := OpenRW(newPath)
	if err != nil {
		return "", "", err
	}
	defer ndb.Close()
	if _, err := ndb.Exec(`ATTACH DATABASE ? AS old`, oldPath); err != nil {
		return "", "", err
	}
	// Every table except the two split ones, from the old file's own schema.
	rows, err := ndb.Query(`SELECT name, sql FROM old.sqlite_master WHERE type = 'table' AND sql IS NOT NULL AND name NOT LIKE 'sqlite\_%' ESCAPE '\' AND name NOT IN ('og_objects', 'choir_events')`)
	if err != nil {
		return "", "", err
	}
	type ddl struct{ name, sql string }
	var tables []ddl
	for rows.Next() {
		var d ddl
		if err := rows.Scan(&d.name, &d.sql); err != nil {
			rows.Close()
			return "", "", err
		}
		tables = append(tables, d)
	}
	rows.Close()
	for _, d := range tables {
		if _, err := ndb.Exec(d.sql); err != nil {
			return "", "", fmt.Errorf("create %s: %w", d.name, err)
		}
		if _, err := ndb.Exec(`INSERT INTO main.` + d.name + ` SELECT * FROM old.` + d.name); err != nil {
			return "", "", fmt.Errorf("copy %s: %w", d.name, err)
		}
	}
	claim, phase := objectgraph.BodyDerivedSQL("o.object_kind", "o.body", "o.metadata")
	stmts := []string{
		objectgraph.ObjectHeaderTableDDL(),
		objectgraph.ObjectBodyTableDDL,
		SplitEventsDDL,
		`INSERT INTO main.og_objects (obj_id, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata, created_at, updated_at, tombstone, superseded_by, og_rec_claim, event_phase)
			SELECT rowid, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata, created_at, updated_at, tombstone, superseded_by, ` + claim + `, ` + phase + ` FROM old.og_objects o`,
		`INSERT INTO main.og_bodies SELECT rowid, body FROM old.og_objects`,
		`INSERT INTO main.choir_events (ev_id, tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata, created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind, event_phase)
			SELECT rowid, tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata, created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind, ` + objectgraph.ChoirEventPhaseSQLFor("body", "metadata") + ` FROM old.choir_events`,
		`INSERT INTO main.choir_event_bodies SELECT rowid, body FROM old.choir_events`,
	}
	for _, s := range stmts {
		if _, err := ndb.Exec(s); err != nil {
			return "", "", fmt.Errorf("split copy: %w", err)
		}
	}
	for _, s := range objectgraph.ObjectIndexDDL() {
		if _, err := ndb.Exec(s); err != nil {
			return "", "", fmt.Errorf("index: %w", err)
		}
	}
	if _, err := ndb.Exec(`DETACH DATABASE old`); err != nil {
		return "", "", err
	}
	if _, err := ndb.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return "", "", err
	}
	return oldPath, newPath, nil
}

// fill writes the same rows into an old-layout file with raw SQL.
func fill(db *sql.DB) error {
	now := "2026-10-30 12:00:00+00:00"
	day := "2026-10-29 12:00:00+00:00"
	type og struct {
		id, kind, hash string
		body           any
		meta, at       string
		tomb           int
	}
	objs := []og{
		{"doc-1", "texture.doc", "h1", []byte("hello body"), `{"title":"a"}`, now, 0},
		{"doc-2", "texture.doc", "h2", nil, `{"title":"b"}`, day, 1},
		{"rec-1", "choir.commitment_record", "h3", []byte(`{"directive":""}`), `{"inbox":"x","inbox_seq":1}`, now, 0},
		{"rec-2", "choir.commitment_record", "h4", []byte(`{"directive":"do"}`), `{"inbox":"x","inbox_seq":2}`, now, 0},
		{"ev-1", "choir.event", "h5", []byte(`{"payload":{"phase":"final"}}`), `{"kind":"tool.result"}`, now, 0},
		{"ev-2", "choir.event", "h6", []byte(`{"payload":{}}`), `{"kind":"tool.result","phase":"x"}`, day, 0},
		{"ev-3", "choir.event", "h7", []byte(`not json`), `{"kind":"model.delta"}`, day, 0},
		{"ev-4", "choir.event", "h8", []byte(``), `{"kind":7}`, now, 0},
	}
	for _, o := range objs {
		if _, err := db.Exec(ogInsert, o.id, o.kind, o.hash, o.body, o.meta, o.at, o.at, o.tomb); err != nil {
			return fmt.Errorf("insert %s: %w", o.id, err)
		}
	}
	events := []struct {
		pos, idx  int
		id, kind  string
		body, at  string
		phaseMeta string
	}{
		{1, 0, "e1", "tool.result", `{"payload":{"phase":"final"}}`, now, `{"kind":"tool.result"}`},
		{1, 1, "e2", "tool.result", `{"payload":{}}`, day, `{"kind":"tool.result","phase":"x"}`},
		{2, 0, "e3", "model.delta", `plain`, day, `{"kind":"model.delta"}`},
		{3, 0, "e4", "", `{}`, now, `{}`},
	}
	for i, e := range events {
		if _, err := db.Exec(evInsert, e.pos, e.idx, e.id, "eh"+e.id, []byte(e.body), e.phaseMeta, e.at, e.at, i, 10+i, e.kind); err != nil {
			return fmt.Errorf("insert event %s: %w", e.id, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO og_key_index (canonical_id, key_name, key_value, owner_id, updated_at, content_hash) VALUES ('doc-1','k','v','o1',?, 'h1')`, now); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES ('c1', 3, 'h', 'h', 'h', 's', 's', 1, 0, ?)`, now); err != nil {
		return err
	}
	return nil
}
