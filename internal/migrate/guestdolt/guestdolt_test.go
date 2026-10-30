package guestdolt

// Failure modes of the guest wrapper (the copier's own are in rowcopy):
//  1. A failure midway leaves a partial or final state file that boot would
//     then trust. (TestFailureLeavesNoStateFile: a unique index in the new
//     schema rejects duplicate source rows after earlier rows were written.)
//  2. The old workspace is changed by a failed or a successful run.
//     (the same test and TestMigrateSuccess compare the workspace tree)
//  3. A second boot re-migrates over live data. (TestSkips)
//  4. A WAL sidecar from the new store's handle survives into the renamed
//     file. (TestMigrateSuccess checks no -wal/-shm remain and the file
//     opens with its rows)
//  5. No report is written, so the operator cannot tell what happened.
//     (both tests read the report)

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/migrate/doltsrc"
)

const newSchema = `
CREATE TABLE notes (id INTEGER NOT NULL PRIMARY KEY, body TEXT NOT NULL, tag TEXT, made DATETIME);
CREATE TABLE brand_new (id INTEGER PRIMARY KEY)`

// legacy builds a workspace like a guest's: <dir>/state.texture, database texture.
func legacy(t *testing.T, dir string, dupTag bool) string {
	t.Helper()
	ctx := context.Background()
	ws := filepath.Join(dir, "state.texture")
	h, err := doltsrc.Create(ctx, ws, doltsrc.DatabaseName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.DB.ExecContext(ctx, `CREATE TABLE notes (id BIGINT NOT NULL PRIMARY KEY, body TEXT NOT NULL, tag VARCHAR(20) NULL, made DATETIME NULL)`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 50; i++ {
		tag := any("t" + string(rune('a'+i%5)))
		if i%4 == 0 {
			tag = nil
		}
		if dupTag && i == 45 {
			tag = "dup" // collides with row 40 below via the unique index
		}
		if dupTag && i == 40 {
			tag = "dup"
		}
		if _, err := h.DB.ExecContext(ctx, `INSERT INTO notes VALUES (?, ?, ?, '2026-07-08 09:10:11')`, i, "note ünï "+string(rune('a'+i%26)), tag); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	return ws
}

func openNewFor(schema string) OpenNew {
	return func(path string) (*dbx.SQLiteDB, error) {
		db, err := dbx.OpenSQLite(path, dbx.SQLiteOptions{})
		if err != nil {
			return nil, err
		}
		for _, stmt := range splitStmts(schema) {
			if _, err := db.Write.Exec(stmt); err != nil {
				db.Close()
				return nil, err
			}
		}
		return db, nil
	}
}

func splitStmts(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ';' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func tree(t *testing.T, root string) map[string]int64 {
	t.Helper()
	m := map[string]int64{}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(p) != "LOCK" {
			rel, _ := filepath.Rel(root, p)
			m[rel] = info.Size()
		}
		return nil
	})
	return m
}

func TestMigrateSuccess(t *testing.T) {
	dir := t.TempDir()
	ws := legacy(t, dir, false)
	before := tree(t, ws)
	sqlitePath := filepath.Join(dir, "state.db")

	rep, err := Migrate(context.Background(), ws, sqlitePath, openNewFor(newSchema))
	if err != nil || !rep.OK {
		t.Fatalf("Migrate: %v %+v", err, rep)
	}
	for _, side := range []string{"-wal", "-shm", ""} {
		if _, err := os.Stat(TempPath(sqlitePath) + side); err == nil {
			t.Errorf("temp file%s left behind", side)
		}
	}
	for _, side := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(sqlitePath + side); err == nil {
			t.Errorf("sidecar %s left on the final file", side)
		}
	}
	db, err := dbx.OpenSQLite(sqlitePath, dbx.SQLiteOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.Read.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&n); err != nil || n != 50 {
		t.Errorf("rows = %d %v", n, err)
	}
	raw, err := os.ReadFile(ReportPath(sqlitePath))
	if err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(raw, &back); err != nil || !back.OK || back.Copy == nil || len(back.Copy.Tables) != 1 {
		t.Errorf("report = %s", raw)
	}
	if got := back.Copy.Tables[0]; got.SourceDigest == "" || got.SourceDigest != got.TargetDigest {
		t.Errorf("digests = %+v", got)
	}
	after := tree(t, ws)
	if len(before) != len(after) {
		t.Errorf("workspace file set changed: %d -> %d files", len(before), len(after))
	}
}

func TestFailureLeavesNoStateFile(t *testing.T) {
	dir := t.TempDir()
	ws := legacy(t, dir, true)
	before := tree(t, ws)
	sqlitePath := filepath.Join(dir, "state.db")
	schema := newSchema + `;CREATE UNIQUE INDEX notes_tag ON notes(tag) WHERE tag = 'dup'`

	rep, err := Migrate(context.Background(), ws, sqlitePath, openNewFor(schema))
	if err == nil || rep.OK {
		t.Fatalf("expected a failure, got err=%v ok=%v", err, rep.OK)
	}
	for _, p := range []string{sqlitePath, TempPath(sqlitePath), TempPath(sqlitePath) + "-wal", TempPath(sqlitePath) + "-shm"} {
		if _, serr := os.Stat(p); !errors.Is(serr, os.ErrNotExist) {
			t.Errorf("%s exists after a failed migration", p)
		}
	}
	raw, rerr := os.ReadFile(ReportPath(sqlitePath))
	if rerr != nil {
		t.Fatalf("no report after failure: %v", rerr)
	}
	var back Report
	_ = json.Unmarshal(raw, &back)
	if back.OK || back.Error == "" {
		t.Errorf("failure report = %s", raw)
	}
	if after := tree(t, ws); len(before) != len(after) {
		t.Errorf("workspace changed by a failed run: %d -> %d files", len(before), len(after))
	}
	// A rerun with a good schema succeeds from scratch.
	rep, err = Migrate(context.Background(), ws, sqlitePath, openNewFor(newSchema))
	if err != nil || !rep.OK {
		t.Fatalf("rerun: %v", err)
	}
}

func TestSkips(t *testing.T) {
	dir := t.TempDir()
	sqlitePath := filepath.Join(dir, "state.db")
	rep, err := Migrate(context.Background(), filepath.Join(dir, "absent.texture"), sqlitePath, openNewFor(newSchema))
	if err != nil || !rep.Skipped {
		t.Errorf("no workspace: %v %+v", err, rep)
	}
	if _, serr := os.Stat(sqlitePath); serr == nil {
		t.Error("a skipped run created the state file")
	}
	ws := legacy(t, dir, false)
	if err := os.WriteFile(sqlitePath, []byte("live"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err = Migrate(context.Background(), ws, sqlitePath, openNewFor(newSchema))
	if err != nil || !rep.Skipped {
		t.Errorf("state exists: %v %+v", err, rep)
	}
	if b, _ := os.ReadFile(sqlitePath); string(b) != "live" {
		t.Error("a skipped run touched the existing state file")
	}
}
