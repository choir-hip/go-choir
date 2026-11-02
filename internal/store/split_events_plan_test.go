package store

// Slice B tests of the header/body split (docs/design/store-header-body-split-2026-11-01.md
// sections 4 and 7): the plans of the choir_events side, retire selection
// equivalence against the frozen old layout, the partition migration, and the
// keepExisting body skip.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxplan"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// rawEventRow writes one choir.event row (header and body) at tape position
// (tape, op) through the production writer. The event kind is the metadata's
// "kind" when it has one.
func rawEventRow(t testing.TB, w choirEventWriter, tape uint64, op int, canonicalID, owner, body, meta string, at time.Time, stream int64) {
	t.Helper()
	var m struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal([]byte(meta), &m)
	row := choirEventRow{
		canonicalID: canonicalID, objectKind: "choir.event", ownerID: owner, contentHash: "h",
		body: []byte(body), metadata: meta, createdAt: at, updatedAt: at, streamSeq: stream, eventKind: m.Kind,
	}
	if err := insertChoirEventRow(context.Background(), w, tape, op, row, false); err != nil {
		t.Fatal(err)
	}
}

func newPartitionDB(t *testing.T) *sql.DB {
	t.Helper()
	db := dbxtest.SQLite(t)
	if _, err := db.Write.Exec(telemetryPartitionDDL); err != nil {
		t.Fatal(err)
	}
	return db.Write
}

// splitEventCorpus is the rows the equivalence and plan tests share: event
// kinds, phases from the payload and from the metadata, bodies that are not
// JSON, and times across several slices.
type splitEvent struct {
	id, kind, body, meta string
	at                   time.Time
}

func splitEventCorpus() []splitEvent {
	var out []splitEvent
	kinds := []string{"tool.invoked", "desk.cell", "run.progress", "tool.result"}
	bodies := []string{`{"payload":{}}`, `{"payload":{"phase":"tool_loop"}}`, `not json`, `{"payload":{"phase":""}}`, `{"payload":{"phase":7}}`, ``}
	metaPhases := []string{``, `,"phase":"mp"`, `,"phase":""`}
	n := 0
	for ki, kind := range kinds {
		for bi, body := range bodies {
			for mi, mp := range metaPhases {
				for day := 0; day < 3; day++ {
					id := fmt.Sprintf("ev-%d-%d-%d-%d", ki, bi, mi, day)
					out = append(out, splitEvent{
						id: id, kind: kind, body: body,
						meta: fmt.Sprintf(`{"kind":%q,"event_id":%q%s}`, kind, id, mp),
						at:   retireBase.Add(time.Duration(day)*24*time.Hour + time.Duration(n)*time.Second),
					})
					n++
				}
			}
		}
	}
	return out
}

// seedSplitEvents writes the corpus to w in the new layout.
func seedSplitEvents(t testing.TB, w choirEventWriter, tape0 uint64) {
	t.Helper()
	for i, e := range splitEventCorpus() {
		rawEventRow(t, w, tape0+uint64(i), 0, e.id, "owner-1", e.body, e.meta, e.at, int64(i+1))
	}
}

func TestSplitEventsPlans(t *testing.T) {
	s, _ := vocabUpcastOpenStore(t, "split-events-plans")
	part := newPartitionDB(t)
	seedSplitEvents(t, s.db, 1)
	seedSplitEvents(t, part, 1)

	for name, db := range map[string]*sql.DB{"store": s.db, "partition": part} {
		// Header-only: the retire selections of every phase mode, with and
		// without the cited set, the emitter's probes, the seeds.
		for _, phase := range []string{"", computerevent.ObjectRetirePhaseNone, computerevent.ObjectRetirePhaseSome} {
			for _, keep := range []string{"", computerevent.ObjectRetireKeepRunAcceptance} {
				p := retireP([]string{"tool.invoked", "desk.cell"}, retireBase, retireBase.Add(24*time.Hour), phase)
				p.KeepCited = keep
				from, before, err := p.Slice()
				if err != nil {
					t.Fatal(err)
				}
				cited := `["a","b"]`
				where, args := retireTableWhere(p, from, before, &cited)
				label := fmt.Sprintf("%s retireTableWhere phase=%q keep=%q", name, phase, keep)
				dbxplan.HeaderOnly(t, db, "idx_choir_events_kind_updated_phase", `SELECT ev_id FROM `+choirEventsTable+` WHERE `+where, args...)
				dbxplan.HeaderOnly(t, db, "idx_choir_events_kind_updated_phase", `SELECT 1 FROM `+choirEventsTable+` WHERE `+where+` LIMIT 1`, args...)
				pw, pargs, err := partitionRetireWhere(p, from, before, []string{"a", "b"})
				if err != nil {
					t.Fatal(err)
				}
				dbxplan.HeaderOnly(t, db, "idx_choir_events_kind_updated_phase", `SELECT ev_id FROM `+choirEventsTable+` WHERE `+pw, pargs...)
				_ = label
			}
		}
		oldest := dbxplan.HeaderOnly(t, db, "idx_choir_events_updated", `SELECT MIN(updated_at) FROM `+choirEventsTable+` WHERE updated_at < ?`, retireBase)
		_ = oldest
		seedQueries := []struct {
			label, index string
			sql          func() (string, []any)
		}{
			{"RunMaxSeq", "idx_choir_events_run_seq", func() (string, []any) { return eventTableRunMaxSeqSQL("run-1") }},
			{"NewestStream", "sqlite_autoindex_choir_events_1", eventTableNewestStreamSQL},
			{"NextOwner", "idx_choir_events_owner_stream", func() (string, []any) { return eventTableNextOwnerSQL("owner-0") }},
			{"OwnerMaxStream", "idx_choir_events_owner_stream", func() (string, []any) { return eventTableOwnerMaxStreamSQL("owner-1") }},
		}
		for _, q := range seedQueries {
			query, args := q.sql()
			dbxplan.HeaderOnly(t, db, q.index, query, args...)
		}
		dbxplan.HeaderOnly(t, db, "", eventTableAnyRowSQL)
		dbxplan.HeaderOnly(t, db, "sqlite_autoindex_choir_events_1", `SELECT ev_id FROM choir_events WHERE tape_seq > ?`, 10)

		// Body by rowid: the four union halves and the partition reader.
		halves := []struct {
			label string
			sql   func() (string, []any)
		}{
			{"RunAfter", func() (string, []any) { return eventTableRunAfterSQL("run-1", 0, 10) }},
			{"OwnerAfter", func() (string, []any) { return eventTableOwnerAfterSQL("owner-1", 0, 10) }},
			{"OwnerNewest", func() (string, []any) { return eventTableOwnerNewestSQL("owner-1", 10) }},
			{"TrajectoryNewest", func() (string, []any) { return eventTableTrajectoryNewestSQL("traj-1", 10) }},
		}
		for _, h := range halves {
			query, args := h.sql()
			plan := dbxplan.BodyByRowid(t, db, "b", query, args...)
			if strings.Contains(strings.ToUpper(plan), "TEMP B-TREE") {
				t.Errorf("%s %s sorts:\n%s", name, h.label, plan)
			}
		}
		dbxplan.BodyByRowid(t, db, "b", readStoredRowsSQL(""))
		dbxplan.BodyByRowid(t, db, "b", readStoredRowsSQL(`WHERE COALESCE(CASE WHEN json_valid(metadata) THEN CAST(json_extract(metadata, '$.event_id') AS TEXT) END, '') IN (SELECT value FROM json_each(?))`), `["x"]`)
	}
	// The cited-set read joins the acceptances' bodies by rowid, og_objects outer.
	dbxplan.BodyByRowid(t, s.db, "rb", citedEventIDsSQL)
}

// oldRetireWhere is the pre-split retire predicate over the old choir_events
// (the phase evaluated over the row's body and metadata), verbatim.
func oldRetireWhere(p computerevent.ObjectRetireProjection, from, before time.Time, cited *string) (string, []any) {
	marks, kindArgs := retireKindPlaceholders(p)
	phase := ""
	switch p.Phase {
	case computerevent.ObjectRetirePhaseNone:
		phase = ` AND ` + objectgraph.ChoirEventPhaseSQLFor("body", "metadata") + ` = ''`
	case computerevent.ObjectRetirePhaseSome:
		phase = ` AND ` + objectgraph.ChoirEventPhaseSQLFor("body", "metadata") + ` <> ''`
	}
	keep, keepArgs := "", []any(nil)
	if p.KeepCited == computerevent.ObjectRetireKeepRunAcceptance {
		if cited != nil {
			keep = ` AND COALESCE(CASE WHEN json_valid(metadata) THEN CAST(json_extract(metadata, '$.event_id') AS TEXT) END, '') NOT IN (SELECT value FROM json_each(?))`
			keepArgs = []any{*cited}
		} else {
			keep = ` AND COALESCE(CASE WHEN json_valid(metadata) THEN CAST(json_extract(metadata, '$.event_id') AS TEXT) END, '') NOT IN (
		SELECT CAST(json_extract(e.value, '$.event_id') AS TEXT)
		FROM og_objects ra, json_each(CASE WHEN json_valid(CAST(ra.body AS TEXT)) THEN CAST(ra.body AS TEXT) ELSE '{}' END, '$.evidence_refs') e
		WHERE ra.object_kind = '` + string(ogKindRunAccept) + `' AND json_extract(e.value, '$.event_id') IS NOT NULL)`
		}
	}
	where := `updated_at >= ? AND updated_at < ? AND event_kind IN (` + marks + `)` + phase + keep
	args := append(append([]any{from, before}, kindArgs...), keepArgs...)
	return where, args
}

func selectIDs(t *testing.T, db *sql.DB, query string, args ...any) []string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func TestRetireSelectsSameRowsAsOldPredicate(t *testing.T) {
	// Old layout: the frozen whole-store schema, rows inserted with the old
	// statement.
	schema, err := os.ReadFile(filepath.Join("testdata", "presplit", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	oldDB := dbxtest.SQLite(t).Write
	if _, err := oldDB.Exec(string(schema)); err != nil {
		t.Fatalf("frozen schema: %v", err)
	}
	newStore, _ := vocabUpcastOpenStore(t, "retire-equivalence")
	newDB := newStore.db
	part := newPartitionDB(t)

	corpus := splitEventCorpus()
	for i, e := range corpus {
		var m struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal([]byte(e.meta), &m)
		if _, err := oldDB.Exec(`INSERT INTO choir_events (tape_seq, op_index, canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at, stream_seq, event_kind)
			VALUES (?, 0, ?, 'choir.event', 'owner-1', 'h', ?, ?, ?, ?, ?, ?)`, i+1, e.id, []byte(e.body), e.meta, e.at, e.at, i+1, m.Kind); err != nil {
			t.Fatal(err)
		}
	}
	seedSplitEvents(t, newDB, 1)
	seedSplitEvents(t, part, 1)

	// Run acceptances cite some events: old (body in the row) and new (body
	// in og_bodies).
	cites := []string{corpus[3].id, corpus[40].id, corpus[77].id, corpus[100].id}
	var refs []map[string]string
	for _, c := range cites {
		refs = append(refs, map[string]string{"event_id": c})
	}
	acceptBody, _ := json.Marshal(map[string]any{"evidence_refs": refs})
	for i, body := range [][]byte{acceptBody, []byte(`not json`)} {
		id := fmt.Sprintf("obj:run_acceptance:%d", i)
		if _, err := oldDB.Exec(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at)
			VALUES (?, ?, 'owner-1', 'h', ?, '{}', ?, ?)`, id, string(ogKindRunAccept), body, retireBase, retireBase); err != nil {
			t.Fatal(err)
		}
		var objID int64
		if err := newDB.QueryRow(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, content_hash, metadata, created_at, updated_at)
			VALUES (?, ?, 'owner-1', 'h', '{}', ?, ?) RETURNING obj_id`, id, string(ogKindRunAccept), retireBase, retireBase).Scan(&objID); err != nil {
			t.Fatal(err)
		}
		if _, err := newDB.Exec(`INSERT INTO og_bodies (obj_id, body) VALUES (?, ?)`, objID, body); err != nil {
			t.Fatal(err)
		}
	}
	citedJSON := func() *string {
		raw, _ := json.Marshal(cites)
		s := string(raw)
		return &s
	}

	all := []string{"tool.invoked", "desk.cell", "run.progress", "tool.result"}
	slices := []struct {
		kinds        []string
		from, before time.Time
	}{
		{[]string{"tool.invoked"}, retireBase, retireBase.Add(24 * time.Hour)},
		{all, retireBase, retireBase.Add(24 * time.Hour)},
		{all, retireBase.Add(24 * time.Hour), retireBase.Add(48 * time.Hour)},
		{all, retireBase.Add(48 * time.Hour), retireBase.Add(72 * time.Hour)},
		{[]string{"run.progress", "desk.cell"}, retireBase.Add(12 * time.Hour), retireBase.Add(36 * time.Hour)},
		{[]string{"nonexistent"}, retireBase, retireBase.Add(24 * time.Hour)},
	}
	checked := 0
	for _, sl := range slices {
		for _, phase := range []string{"", computerevent.ObjectRetirePhaseNone, computerevent.ObjectRetirePhaseSome} {
			for _, keep := range []string{"", computerevent.ObjectRetireKeepRunAcceptance} {
				for _, mode := range []string{"read-og", "cited-param"} {
					p := retireP(sl.kinds, sl.from, sl.before, phase)
					p.KeepCited = keep
					from, before, err := p.Slice()
					if err != nil {
						t.Fatal(err)
					}
					var cited *string
					if mode == "cited-param" {
						cited = citedJSON()
					}
					ow, oargs := oldRetireWhere(p, from, before, cited)
					nw, nargs := retireTableWhere(p, from, before, cited)
					want := selectIDs(t, oldDB, `SELECT canonical_id FROM choir_events WHERE `+ow, oargs...)
					got := selectIDs(t, newDB, `SELECT canonical_id FROM choir_events WHERE `+nw, nargs...)
					label := fmt.Sprintf("kinds=%v phase=%q keep=%q %s", sl.kinds, phase, keep, mode)
					if !reflect.DeepEqual(want, got) {
						t.Fatalf("%s: old selects %d rows, new %d", label, len(want), len(got))
					}
					// The partition predicate selects the same rows from its own cited list.
					if mode == "cited-param" {
						pw, pargs, err := partitionRetireWhere(p, from, before, cites)
						if err != nil {
							t.Fatal(err)
						}
						if pgot := selectIDs(t, part, `SELECT canonical_id FROM choir_events WHERE `+pw, pargs...); !reflect.DeepEqual(want, pgot) {
							t.Fatalf("%s: partition selects %d rows, old %d", label, len(pgot), len(want))
						}
					}
					if len(want) > 0 {
						checked++
					}
				}
			}
		}
	}
	if checked < 10 {
		t.Fatalf("only %d non-empty selections compared; the corpus is too thin", checked)
	}
	// event_phase is the old expression over the stored body and metadata.
	oldPhases := map[string]string{}
	rows, err := oldDB.Query(`SELECT canonical_id, ` + objectgraph.ChoirEventPhaseSQLFor("body", "metadata") + ` FROM choir_events`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, ph string
		if err := rows.Scan(&id, &ph); err != nil {
			t.Fatal(err)
		}
		oldPhases[id] = ph
	}
	rows.Close()
	rows, err = newDB.Query(`SELECT canonical_id, event_phase FROM choir_events`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id, ph string
		if err := rows.Scan(&id, &ph); err != nil {
			t.Fatal(err)
		}
		if oldPhases[id] != ph {
			t.Errorf("%s: event_phase %q, old expression %q", id, ph, oldPhases[id])
		}
		n++
	}
	if n != len(corpus) {
		t.Fatalf("new rows %d, want %d", n, len(corpus))
	}
}

type partitionRow struct {
	Tape, Op                                                    int64
	CanonicalID, Kind, Owner, Computer, Version, Hash, Metadata string
	Body                                                        string
	Created, Updated                                            string
	Tombstone                                                   int64
	Superseded, Run                                             string
	Seq, Stream                                                 int64
	Trajectory, EventKind                                       string
}

func TestPartitionSplitMigration(t *testing.T) {
	ctx := context.Background()
	ddl, err := os.ReadFile(filepath.Join("testdata", "presplit", "partition.sql"))
	if err != nil {
		t.Fatal(err)
	}
	db := dbxtest.SQLite(t).Write
	if _, err := db.Exec(string(ddl)); err != nil {
		t.Fatal(err)
	}
	corpus := splitEventCorpus()
	for i, e := range corpus {
		var m struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal([]byte(e.meta), &m)
		// Spread the tape positions and skip some, so rowid != tape_seq.
		if _, err := db.Exec(`INSERT INTO choir_events (tape_seq, op_index, canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at, run_id, seq, stream_seq, trajectory_id, event_kind)
			VALUES (?, ?, ?, 'choir.event', 'owner-1', 'h', ?, ?, ?, ?, 'run-1', ?, ?, 'traj-1', ?)`,
			100+i*3, i%2, e.id, []byte(e.body), e.meta, e.at, e.at, i+1, i+1, m.Kind); err != nil {
			t.Fatal(err)
		}
	}
	// A hole in the rowids.
	if _, err := db.Exec(`DELETE FROM choir_events WHERE canonical_id IN (?, ?)`, corpus[5].id, corpus[9].id); err != nil {
		t.Fatal(err)
	}
	oldRows := func() []partitionRow {
		rs, err := db.Query(`SELECT tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata, CAST(body AS TEXT),
			CAST(created_at AS TEXT), CAST(updated_at AS TEXT), tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind FROM choir_events ORDER BY tape_seq, op_index`)
		if err != nil {
			t.Fatal(err)
		}
		defer rs.Close()
		return scanPartitionRows(t, rs)
	}
	before := oldRows()
	oldPhase := map[string]string{}
	oldRowid := map[string]int64{}
	rs, err := db.Query(`SELECT canonical_id, rowid, ` + objectgraph.ChoirEventPhaseSQLFor("body", "metadata") + ` FROM choir_events`)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var id, ph string
		var rid int64
		if err := rs.Scan(&id, &rid, &ph); err != nil {
			t.Fatal(err)
		}
		oldPhase[id], oldRowid[id] = ph, rid
	}
	rs.Close()

	// A failure inside the transaction at every step leaves the file old.
	t.Cleanup(func() { partitionSplitHook = nil })
	for _, failAt := range []string{"rename", "drop-index", "create", "copy-headers", "copy-bodies", "drop-old"} {
		partitionSplitHook = func(step string) error {
			if step == failAt {
				return fmt.Errorf("injected crash after %s", step)
			}
			return nil
		}
		if err := migratePartitionSplit(ctx, db); err == nil {
			t.Fatalf("injected failure at %s did not fail the migration", failAt)
		}
		var bodyCols, bodyTables, presplit int
		if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM pragma_table_info('choir_events') WHERE name='body'),
			(SELECT COUNT(*) FROM sqlite_master WHERE name='choir_event_bodies'),
			(SELECT COUNT(*) FROM sqlite_master WHERE name='choir_events_presplit')`).Scan(&bodyCols, &bodyTables, &presplit); err != nil {
			t.Fatal(err)
		}
		if bodyCols != 1 || bodyTables != 0 || presplit != 0 {
			t.Fatalf("after a failure at %s the file is not old: body col=%d body table=%d presplit=%d", failAt, bodyCols, bodyTables, presplit)
		}
		if got := oldRows(); !reflect.DeepEqual(got, before) {
			t.Fatalf("after a failure at %s the rows changed", failAt)
		}
	}
	partitionSplitHook = nil
	if err := migratePartitionSplit(ctx, db); err != nil {
		t.Fatal(err)
	}
	// The DDL after the migration is a no-op, and a second migration too.
	if _, err := db.Exec(telemetryPartitionDDL); err != nil {
		t.Fatal(err)
	}
	if err := migratePartitionSplit(ctx, db); err != nil {
		t.Fatal(err)
	}
	rs, err = db.Query(`SELECT h.tape_seq, h.op_index, h.canonical_id, h.object_kind, h.owner_id, h.computer_id, h.version_id, h.content_hash, h.metadata, CAST(b.body AS TEXT),
		CAST(h.created_at AS TEXT), CAST(h.updated_at AS TEXT), h.tombstone, h.superseded_by, h.run_id, h.seq, h.stream_seq, h.trajectory_id, h.event_kind
		FROM choir_events h JOIN choir_event_bodies b ON b.ev_id = h.ev_id ORDER BY h.tape_seq, h.op_index`)
	if err != nil {
		t.Fatal(err)
	}
	after := scanPartitionRows(t, rs)
	rs.Close()
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("rows differ after the migration (%d before, %d after)", len(before), len(after))
	}
	rs, err = db.Query(`SELECT canonical_id, ev_id, event_phase FROM choir_events`)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var id, ph string
		var ev int64
		if err := rs.Scan(&id, &ev, &ph); err != nil {
			t.Fatal(err)
		}
		if ph != oldPhase[id] || ev != oldRowid[id] {
			t.Errorf("%s: ev_id=%d phase=%q; old rowid=%d phase=%q", id, ev, ph, oldRowid[id], oldPhase[id])
		}
	}
	rs.Close()
	var orphans, presplit int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM choir_event_bodies b LEFT JOIN choir_events h ON h.ev_id=b.ev_id WHERE h.ev_id IS NULL),
		(SELECT COUNT(*) FROM sqlite_master WHERE name='choir_events_presplit')`).Scan(&orphans, &presplit); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 || presplit != 0 {
		t.Fatalf("orphan bodies=%d presplit tables=%d", orphans, presplit)
	}
	// The reader the fingerprints use sees the same rows through the join.
	stored, err := readStoredRows(ctx, db, "")
	if err != nil || len(stored) != len(before) {
		t.Fatalf("readStoredRows after the split: %d rows, err=%v; want %d", len(stored), err, len(before))
	}
}

func scanPartitionRows(t *testing.T, rs *sql.Rows) []partitionRow {
	t.Helper()
	var out []partitionRow
	for rs.Next() {
		var r partitionRow
		if err := rs.Scan(&r.Tape, &r.Op, &r.CanonicalID, &r.Kind, &r.Owner, &r.Computer, &r.Version, &r.Hash, &r.Metadata, &r.Body,
			&r.Created, &r.Updated, &r.Tombstone, &r.Superseded, &r.Run, &r.Seq, &r.Stream, &r.Trajectory, &r.EventKind); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestKeepExistingSkipsBody(t *testing.T) {
	ctx := context.Background()
	for name, db := range map[string]*sql.DB{"store": func() *sql.DB { s, _ := vocabUpcastOpenStore(t, "keep-existing"); return s.db }(), "partition": newPartitionDB(t)} {
		at := retireBase
		first := choirEventRow{canonicalID: "ev-1", objectKind: "choir.event", ownerID: "o", contentHash: "h", body: []byte(`{"payload":{"phase":"a"}}`), metadata: `{"kind":"k"}`, createdAt: at, updatedAt: at, eventKind: "k"}
		if err := insertChoirEventRow(ctx, db, 7, 0, first, false); err != nil {
			t.Fatal(err)
		}
		var evID int64
		if err := db.QueryRow(`SELECT ev_id FROM choir_events WHERE tape_seq=7 AND op_index=0`).Scan(&evID); err != nil {
			t.Fatal(err)
		}
		// keepExisting with a different row: neither the header nor the body moves.
		second := first
		second.canonicalID, second.body, second.metadata = "ev-2", []byte(`{"payload":{"phase":"b"}}`), `{"kind":"k2","phase":"z"}`
		if err := insertChoirEventRow(ctx, db, 7, 0, second, true); err != nil {
			t.Fatal(err)
		}
		var id, body, phase string
		if err := db.QueryRow(`SELECT h.canonical_id, CAST(b.body AS TEXT), h.event_phase FROM choir_events h JOIN choir_event_bodies b ON b.ev_id=h.ev_id WHERE h.ev_id=?`, evID).Scan(&id, &body, &phase); err != nil {
			t.Fatal(err)
		}
		if id != "ev-1" || body != string(first.body) || phase != "a" {
			t.Fatalf("%s: keepExisting changed the row: %s %s %s", name, id, body, phase)
		}
		// Proof the body statement did not run: with the body removed by hand,
		// a keepExisting write leaves it absent.
		if _, err := db.Exec(`DELETE FROM choir_event_bodies WHERE ev_id=?`, evID); err != nil {
			t.Fatal(err)
		}
		if err := insertChoirEventRow(ctx, db, 7, 0, second, true); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM choir_event_bodies`).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s: keepExisting wrote a body (n=%d err=%v)", name, n, err)
		}
		// A plain upsert keeps ev_id, replaces the body and recomputes the phase.
		if err := insertChoirEventRow(ctx, db, 7, 0, second, false); err != nil {
			t.Fatal(err)
		}
		var evID2 int64
		if err := db.QueryRow(`SELECT h.ev_id, CAST(b.body AS TEXT), h.event_phase FROM choir_events h JOIN choir_event_bodies b ON b.ev_id=h.ev_id WHERE h.tape_seq=7`).Scan(&evID2, &body, &phase); err != nil {
			t.Fatal(err)
		}
		if evID2 != evID || body != string(second.body) || phase != "b" {
			t.Fatalf("%s: upsert: ev_id %d->%d body=%s phase=%s", name, evID, evID2, body, phase)
		}
		// The trigger drops the body with its header.
		if _, err := db.Exec(`DELETE FROM choir_events WHERE ev_id=?`, evID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM choir_event_bodies`).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s: header delete left %d bodies (err=%v)", name, n, err)
		}
	}
}
