package store

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

// The capsule's store copy (S3 failure modes 1, 2, 4, 6, 7,
// docs/design/capsule-overlay-computer-state-2026-11-03.md).

func TestSnapshotToIsOneFileWithTheSidecarAndOpens(t *testing.T) {
	s, _ := compactTestStore(t, 5, 20, 200)
	dir := t.TempDir()
	receipt, err := s.SnapshotTo(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Sequence != 5 || receipt.Head != compactTestHead(5) || receipt.ComputerID != compactTestComputer {
		t.Fatalf("receipt head = %+v", receipt)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "state.db,state.texture" {
		t.Fatalf("copy dir holds %v, want state.db and state.texture only", names)
	}
	side, _ := os.ReadDir(filepath.Join(dir, "state.texture"))
	for _, e := range side {
		switch e.Name() {
		case storeschema.FloorFile, storeschema.File, storeschema.LeaseUntrustedFile, vocabReportFileName, depositUpcastLedgerFileName:
		default:
			t.Fatalf("sidecar carried %s", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "state.texture", storeschema.File)); err != nil {
		t.Fatalf("schema receipt not carried: %v", err)
	}
	// The copy opens as a store (the in-capsule autoputer's path).
	copyStore, err := Open(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatalf("open the copy: %v", err)
	}
	var rows int
	if err := copyStore.ReadDB().QueryRow(`SELECT COUNT(*) FROM compact_probe`).Scan(&rows); err != nil || rows != 20 {
		t.Fatalf("copy rows = %d (%v), want 20", rows, err)
	}
	_ = copyStore.Close()
}

func TestSnapshotToLeavesTheLiveStoreUnchangedAndWritable(t *testing.T) {
	s, _ := compactTestStore(t, 3, 5, 0)
	if _, err := s.SnapshotTo(context.Background(), filepath.Dir(s.StatePath())); err == nil {
		t.Fatal("a snapshot into the live store's directory was accepted")
	}
	dir := t.TempDir()
	if _, err := s.SnapshotTo(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	compactTestAppend(t, s, 4)
	var seq uint64
	if err := s.ReadDB().QueryRow(`SELECT sequence FROM computer_event_projection_heads`).Scan(&seq); err != nil || seq != 4 {
		t.Fatalf("live head after copy = %d (%v), want 4", seq, err)
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(filepath.Join(dir, "state.db"), dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.QueryRow(`SELECT sequence FROM computer_event_projection_heads`).Scan(&seq); err != nil || seq != 3 {
		t.Fatalf("copy head = %d (%v), want 3: the copy is one instant", seq, err)
	}
}

// A secret-shaped value is blanked in the copy, and its bytes are in no
// file of the copy (secure_delete and the second VACUUM), while the live
// store keeps it.
func TestSnapshotToBlanksSecretShapedValuesAndNoBytesSurvive(t *testing.T) {
	s, _ := compactTestStore(t, 2, 3, 0)
	secret := "ghp_" + strings.Repeat("Z7q", 8)
	if _, err := s.db.Exec(`INSERT INTO compact_probe VALUES (900, ?)`, "note: "+secret+" end"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO compact_probe VALUES (901, ?)`, []byte("blob "+secret)); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	receipt, err := s.SnapshotTo(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Blanked["compact_probe.payload"] != 2 || receipt.ScannedBytes == 0 {
		t.Fatalf("receipt = %+v", receipt)
	}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		raw, _ := os.ReadFile(path)
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("the secret survives in %s", path)
		}
		return nil
	})
	var live string
	if err := s.ReadDB().QueryRow(`SELECT payload FROM compact_probe WHERE id = 900`).Scan(&live); err != nil || !strings.Contains(live, secret) {
		t.Fatalf("the live store was changed: %q %v", live, err)
	}
}

// Every column of the guest store whose name looks like a credential is
// ruled on here, with its reason. A new one fails until it is ruled on
// (S3 failure mode 4). The S0 inventory found none holding a credential.
func TestStoreColumnsWithCredentialShapedNamesAreRuledOn(t *testing.T) {
	s, _ := trimTestOpen(t)
	rulings := map[string]string{
		"store_meta.key":                                              "the store's own metadata key names",
		"run_memory_entries.tokens_before":                            "a count of model tokens",
		"user_preferences.preference_key":                             "a preference's name",
		"desktop_sessions.session_id":                                 "a layout key the client picks (X-Choir-Session), not an authenticator",
		"desktop_app_instances.created_by_session_id":                 "the same layout key",
		"desktop_window_placements.session_id":                        "the same layout key",
		"computer_event_projection_heads.credential_revocation_epoch": "an epoch number",
		"computer_event_index.idempotency_key":                        "a request's idempotency name",
		"computer_event_index.next_credential_revocation_epoch":       "an epoch number",
		"self_development_start_intents.idempotency_key":              "a request's idempotency name",
		"self_development_operations.idempotency_key":                 "a request's idempotency name",
		"og_key_index.key_name":                                       "an object-graph index key's name",
		"og_key_index.key_value":                                      "an object-graph index key's value (object ids and kinds)",
	}
	shape := regexp.MustCompile(`(?i)token|secret|key|password|cookie|session|nonce|credential`)
	rows, err := s.ReadDB().Query(`SELECT m.name, p.name FROM sqlite_master m, pragma_table_info(m.name) p WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var unruled []string
	matched := 0
	for rows.Next() {
		var table, col string
		if err := rows.Scan(&table, &col); err != nil {
			t.Fatal(err)
		}
		name := table + "." + col
		if shape.MatchString(col) {
			matched++
			if rulings[name] == "" {
				unruled = append(unruled, name)
			}
		}
	}
	if matched < len(rulings)/2 {
		t.Fatalf("the scan matched %d columns; the schema query is not reading the store", matched)
	}
	if len(unruled) > 0 {
		sort.Strings(unruled)
		t.Fatalf("credential-shaped columns with no ruling (say what each holds, and blank it in SnapshotTo if it is a credential): %v", unruled)
	}
}

// X6 (MA3.4): the store copy is a lease. A copy taken for a lease holds the
// objects of the kinds the lease names and none of the others; a nil kinds
// copies every kind.
func TestSnapshotToKindsKeepsOnlyTheLeasedKinds(t *testing.T) {
	s, _ := compactTestStore(t, 3, 5, 0)
	ctx := context.Background()
	if err := s.PutComputerSetting(ctx, ComputerSetting{OwnerID: "owner-x6", ComputerID: "computer-x6", Key: SettingIntakeLabels, Value: false}); err != nil {
		t.Fatal(err)
	}
	if err := s.Leases().PutLease(ctx, lease.Record{LeaseID: "lease-x6", OwnerID: "owner-x6", ComputerID: "computer-x6", Holder: "engineering", HolderRunID: "run:x6", State: lease.StateActive}); err != nil {
		t.Fatal(err)
	}
	count := func(dir, kind string) int {
		t.Helper()
		db, err := sql.Open("sqlite", dbx.SQLiteDSN(filepath.Join(dir, "state.db"), dbx.SQLiteOptions{}, false))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM og_objects WHERE object_kind = ?`, kind).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	whole := t.TempDir()
	if _, err := s.SnapshotTo(ctx, whole); err != nil {
		t.Fatal(err)
	}
	if count(whole, "choir.lease") != 1 || count(whole, "choir.computer_setting") != 1 {
		t.Fatalf("the whole copy lost a kind: lease=%d setting=%d", count(whole, "choir.lease"), count(whole, "choir.computer_setting"))
	}
	leased := t.TempDir()
	receipt, err := s.SnapshotToKinds(ctx, leased, []string{"choir.computer_setting"})
	if err != nil {
		t.Fatal(err)
	}
	if count(leased, "choir.computer_setting") != 1 {
		t.Fatal("the leased kind is missing from the copy")
	}
	if count(leased, "choir.lease") != 0 || receipt.Withheld["choir.lease"] != 1 {
		t.Fatalf("an unleased kind is in the copy: lease=%d withheld=%v", count(leased, "choir.lease"), receipt.Withheld)
	}
	// The live store is untouched.
	if _, err := s.Leases().ListLeases(ctx, "owner-x6", "computer-x6"); err != nil {
		t.Fatal(err)
	}
	live, _ := s.Leases().ListLeases(ctx, "owner-x6", "computer-x6")
	if len(live) != 1 {
		t.Fatalf("the live store lost its lease: %d", len(live))
	}
}

// insertGenericRow inserts one row into table, filling every NOT NULL column
// that has no default with a value of its declared type. It returns false when
// the table cannot take a generic row.
func insertGenericRow(t *testing.T, db *sql.DB, table string, overrides map[string]any) {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + sqlIdent(table) + `)`)
	if err != nil {
		t.Fatal(err)
	}
	var cols []string
	var vals []any
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if v, ok := overrides[name]; ok {
			cols, vals = append(cols, sqlIdent(name)), append(vals, v)
			continue
		}
		if notnull == 0 && pk == 0 || dflt.Valid {
			continue
		}
		if strings.Contains(strings.ToUpper(typ), "INT") {
			if pk == 1 && strings.EqualFold(typ, "INTEGER") {
				continue // rowid alias
			}
			cols, vals = append(cols, sqlIdent(name)), append(vals, 1)
		} else if strings.Contains(strings.ToUpper(typ), "BLOB") {
			cols, vals = append(cols, sqlIdent(name)), append(vals, []byte("secret-body"))
		} else if strings.Contains(strings.ToUpper(typ), "DATETIME") {
			cols, vals = append(cols, sqlIdent(name)), append(vals, "2026-01-01T00:00:00Z")
		} else {
			cols, vals = append(cols, sqlIdent(name)), append(vals, "secret-"+name)
		}
	}
	rows.Close()
	marks := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
	if len(cols) == 0 {
		if _, err := db.Exec(`INSERT INTO ` + sqlIdent(table) + ` DEFAULT VALUES`); err != nil {
			t.Fatalf("seed %s: %v", table, err)
		}
		return
	}
	if _, err := db.Exec(`INSERT INTO `+sqlIdent(table)+` (`+strings.Join(cols, ",")+`) VALUES (`+marks+`)`, vals...); err != nil {
		t.Fatalf("seed %s: %v", table, err)
	}
}

// B1: the copy is a VACUUM INTO of the whole state.db, so the lease decides
// every table, not only the object graph. By default no content-bearing table
// of a withheld kind, no unmapped table and no store_meta secret has a row; a
// lease that names a kind keeps that kind's tables.
func TestSnapshotToKindsDecidesEveryTable(t *testing.T) {
	s, _ := compactTestStore(t, 3, 5, 0)
	ctx := context.Background()
	if err := s.Leases().PutLease(ctx, lease.Record{LeaseID: "lease-b1", OwnerID: "o", ComputerID: "c", Holder: "engineering", HolderRunID: "r", State: lease.StateActive}); err != nil {
		t.Fatal(err)
	}
	for table := range snapshotTableKinds {
		insertGenericRow(t, s.db, table, nil)
	}
	if _, err := s.db.Exec(`CREATE TABLE zz_unmapped (secret TEXT)`); err != nil {
		t.Fatal(err)
	}
	insertGenericRow(t, s.db, "zz_unmapped", nil)
	if _, err := s.db.Exec(`INSERT INTO store_meta (key, value) VALUES ('owner_secret', 'x')`); err != nil {
		t.Fatal(err)
	}
	insertGenericRow(t, s.db, "choir_events", map[string]any{"object_kind": "choir.texture_revision"})
	if _, err := s.db.Exec(`INSERT INTO choir_event_bodies (ev_id, body) SELECT ev_id, x'01' FROM choir_events`); err != nil {
		t.Fatal(err)
	}

	count := func(db *sql.DB, table string) int {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + sqlIdent(table)).Scan(&n); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		return n
	}
	open := func(dir string) *sql.DB {
		db, err := sql.Open("sqlite", dbx.SQLiteDSN(filepath.Join(dir, "state.db"), dbx.SQLiteOptions{}, false))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}
	assertCopy := func(name string, kinds []string) {
		dir := t.TempDir()
		if _, err := s.SnapshotToKinds(ctx, dir, kinds); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		leased := map[string]bool{}
		for _, k := range kinds {
			leased[k] = true
		}
		db := open(dir)
		names, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
		if err != nil {
			t.Fatal(err)
		}
		var tables []string
		for names.Next() {
			var n string
			names.Scan(&n)
			tables = append(tables, n)
		}
		names.Close()
		for _, table := range tables {
			n := count(db, table)
			kind, mapped := snapshotTableKinds[table]
			switch {
			case table == "store_meta":
				var other int
				db.QueryRow(`SELECT COUNT(*) FROM store_meta WHERE key <> 'schema_version'`).Scan(&other)
				if other != 0 {
					t.Errorf("%s: store_meta kept %d non-schema rows", name, other)
				}
			case table == "zz_unmapped":
				if n != 0 {
					t.Errorf("%s: an unmapped table kept %d rows (the disposition must fail closed)", name, n)
				}
			case mapped && !leased[kind]:
				if n != 0 {
					t.Errorf("%s: %s (kind %s, not leased) kept %d rows", name, table, kind, n)
				}
			case mapped && leased[kind]:
				if n == 0 {
					t.Errorf("%s: %s (kind %s, leased) lost its rows", name, table, kind)
				}
			case table == "choir_events" || table == "choir_event_bodies":
				if n != 0 && !leased["choir.texture_revision"] {
					t.Errorf("%s: the tape kept %d rows of an unleased kind in %s", name, n, table)
				}
			case table == "og_objects":
				var lk int
				db.QueryRow(`SELECT COUNT(*) FROM og_objects WHERE object_kind = 'choir.lease'`).Scan(&lk)
				if lk != 0 {
					t.Errorf("%s: the lease kind is in the copy", name)
				}
			}
		}
		// The copy opens as a store (the in-capsule autoputer's path).
		copyStore, err := Open(filepath.Join(dir, "state"))
		if err != nil {
			t.Fatalf("%s: open the copy: %v", name, err)
		}
		_ = copyStore.Close()
	}
	assertCopy("default", lease.CapsuleStoreKinds())
	assertCopy("nothing leased", []string{"choir.run"})
	withTexture := append(lease.CapsuleStoreKinds(), "choir.texture_revision")
	assertCopy("texture revisions leased", withTexture)
}
