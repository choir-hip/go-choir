package dbx_test

import (
	"context"
	"os"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }

func TestRebind(t *testing.T) {
	cases := map[string]string{
		`SELECT 1`:                                       `SELECT 1`,
		`SELECT * FROM t WHERE a = ? AND b = ?`:          `SELECT * FROM t WHERE a = $1 AND b = $2`,
		`SELECT '?' , ? FROM t`:                          `SELECT '?' , $1 FROM t`,
		`SELECT 'it''s ?', ?`:                            `SELECT 'it''s ?', $1`,
		`SELECT "we?ird", ? -- why?` + "\n" + `, ?`:      `SELECT "we?ird", $1 -- why?` + "\n" + `, $2`,
		`SELECT /* ? */ ?`:                               `SELECT /* ? */ $1`,
		`DO $$ BEGIN PERFORM '?'; END $$; SELECT ?`:      `DO $$ BEGIN PERFORM '?'; END $$; SELECT $1`,
		`INSERT INTO t (a) VALUES (?) ON CONFLICT (a) DO UPDATE SET a = excluded.a`: `INSERT INTO t (a) VALUES ($1) ON CONFLICT (a) DO UPDATE SET a = excluded.a`,
	}
	for in, want := range cases {
		if got := dbx.Rebind(in); got != want {
			t.Errorf("Rebind(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

// The upsert and insert-ignore forms the build sheet prescribes run the same
// on both engines; failure modes covered: placeholder rebinding inside a
// transaction, ON CONFLICT on both, unique violation classification,
// introspection on both.
func TestEnginesAgree(t *testing.T) {
	ctx := context.Background()
	sq := dbxtest.SQLite(t)
	pg := dbxtest.Postgres(t)
	run := func(t *testing.T, kind dbx.Kind, exec func(string, ...any) error, q dbx.Queryer) {
		if err := exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT NOT NULL, n BIGINT NOT NULL DEFAULT 0)`); err != nil {
			t.Fatalf("%s create: %v", kind, err)
		}
		up := `INSERT INTO kv (k, v, n) VALUES (?, ?, ?) ON CONFLICT (k) DO UPDATE SET v = excluded.v, n = kv.n + excluded.n`
		for i := 0; i < 3; i++ {
			if err := exec(up, "a", "x", 1); err != nil {
				t.Fatalf("%s upsert: %v", kind, err)
			}
		}
		if err := exec(`INSERT INTO kv (k, v) VALUES (?, ?) ON CONFLICT DO NOTHING`, "a", "ignored"); err != nil {
			t.Fatalf("%s insert ignore: %v", kind, err)
		}
		var v string
		var n int64
		if err := q.QueryRowContext(ctx, `SELECT v, n FROM kv WHERE k = ?`, "a").Scan(&v, &n); err != nil {
			t.Fatalf("%s select: %v", kind, err)
		}
		if v != "x" || n != 3 {
			t.Fatalf("%s got v=%q n=%d, want x 3", kind, v, n)
		}
		err := exec(`INSERT INTO kv (k, v) VALUES (?, ?)`, "a", "dup")
		if !dbx.IsUniqueViolation(err) {
			t.Fatalf("%s duplicate insert: want unique violation, got %v", kind, err)
		}
		ok, err := dbx.TableExists(ctx, q, kind, "kv")
		if err != nil || !ok {
			t.Fatalf("%s TableExists = %v, %v", kind, ok, err)
		}
		ok, err = dbx.ColumnExists(ctx, q, kind, "kv", "n")
		if err != nil || !ok {
			t.Fatalf("%s ColumnExists = %v, %v", kind, ok, err)
		}
		ok, err = dbx.ColumnExists(ctx, q, kind, "kv", "missing")
		if err != nil || ok {
			t.Fatalf("%s ColumnExists(missing) = %v, %v", kind, ok, err)
		}
		tables, err := dbx.Tables(ctx, q, kind)
		if err != nil || len(tables) != 1 || tables[0] != "kv" {
			t.Fatalf("%s Tables = %v, %v", kind, tables, err)
		}
	}
	t.Run("sqlite", func(t *testing.T) {
		run(t, dbx.SQLite, func(q string, a ...any) error { _, err := sq.Write.ExecContext(ctx, q, a...); return err }, sq.Read)
	})
	t.Run("postgres", func(t *testing.T) {
		run(t, dbx.Postgres, func(q string, a ...any) error { _, err := pg.ExecContext(ctx, q, a...); return err }, pg)
	})
}
