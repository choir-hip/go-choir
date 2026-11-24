package unprovedpurge

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/maild"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }

// realServices opens the schemas the services themselves create (auth and maild SQLite, the platform's two
// PostgreSQL databases through Store.Bootstrap), not tables written by hand for the test.
func realServices(t *testing.T) []Service {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	as, err := auth.OpenStore(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = as.Close() })
	ms, err := maild.OpenStore(filepath.Join(dir, "mail.db"), filepath.Join(dir, "mailroot"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ms.Close() })
	if err := ms.EnsureSchema(&maild.Config{DBPath: filepath.Join(dir, "mail.db"), StorageRoot: filepath.Join(dir, "mailroot"), PrimaryDomain: "choir.news", RootOwnerID: "root"}); err != nil {
		t.Fatalf("maild schema: %v", err)
	}
	cdsn, gdsn := dbxtest.PostgresDSN(t), dbxtest.PostgresDSN(t)
	ps, err := platform.OpenStore(ctx, cdsn, gdsn)
	if err != nil {
		t.Fatalf("platform bootstrap: %v", err)
	}
	t.Cleanup(func() { _ = ps.Close() })
	open := func(dsn string) *sql.DB {
		db, err := dbx.OpenPostgres(dsn, dbx.PGOptions{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		return db
	}
	mdb, err := sql.Open("sqlite", filepath.Join(dir, "mail.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mdb.Close() })
	return []Service{
		{Name: "auth", DB: as.DB(), Kind: dbx.SQLite, Keys: AuthKeys, CoreAny: []string{"users"}},
		{Name: "maild", DB: mdb, Kind: dbx.SQLite, Keys: MaildKeys, CoreAny: []string{"email_aliases", "maild_migrations"}},
		{Name: "control", DB: open(cdsn), Kind: dbx.Postgres, Keys: PlatformKeys, CoreAny: ControlCore},
		{Name: "graph", DB: open(gdsn), Kind: dbx.Postgres, Keys: PlatformKeys, CoreAny: GraphCore},
	}
}

// Every table the real schemas create is either keyed, or reviewed as not naming a user: a run against the real
// databases is not stopped by an unregistered column (the run of the dry run was the first to find
// proposal_delivery_records.target_id, which a hand-made table had hidden).
func TestTheRealSchemasHaveNoUnregisteredUserColumn(t *testing.T) {
	ctx := context.Background()
	svcs := realServices(t)
	for i := range svcs {
		if _, err := svcs[i].present(ctx); err != nil {
			t.Fatalf("%s: %v", svcs[i].Name, err)
		}
	}
	// and a whole dry run over the empty real databases.
	owners := func(context.Context) (map[string]bool, error) { return map[string]bool{"__any__": true}, nil }
	if _, err := BuildPlan(ctx, svcs[0].DB, owners, svcs); err != nil {
		t.Fatalf("dry run over the real schemas: %v", err)
	}
}

// The control and graph databases are told apart: either one given for the other is refused.
func TestTheControlAndGraphDatabasesCannotStandInForEachOther(t *testing.T) {
	ctx := context.Background()
	svcs := realServices(t)
	control, graph := svcs[2], svcs[3]
	for _, bad := range []Service{
		{Name: "graph", DB: control.DB, Kind: dbx.Postgres, Keys: PlatformKeys, CoreAny: GraphCore},
		{Name: "control", DB: graph.DB, Kind: dbx.Postgres, Keys: PlatformKeys, CoreAny: ControlCore},
	} {
		if _, err := bad.present(ctx); !errors.Is(err, ErrIncomplete) {
			t.Fatalf("%s given the other database: err = %v, want ErrIncomplete", bad.Name, err)
		}
	}
}

// A delivery record refuses an account named as its owner, and one named as its target when the target is a user;
// a target that is a proposal refuses nobody.
func TestARealDeliveryRecordRefusesTheOwnerAndAUserTarget(t *testing.T) {
	ctx := context.Background()
	svcs := realServices(t)
	graph := svcs[3]
	now := time.Now().UTC()
	for _, r := range [][]any{{"d1", "someone", "publication_version_proposal", "p1"}, {"d2", "someone", "user", "a-user"}} {
		if _, err := graph.DB.Exec(`INSERT INTO proposal_delivery_records (delivery_id, proposal_id, target_owner_id, target_kind, target_id, delivery_state, created_at, updated_at) VALUES ($1, 'p', $2, $3, $4, 'sent', $5, $5)`, r[0], r[1], r[2], r[3], now); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := graph.present(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hits := map[string]map[string]bool{}
	for _, k := range keys {
		if k.Table == "proposal_delivery_records" {
			h, err := graph.proofHits(ctx, k, []string{"p1", "a-user", "someone"})
			if err != nil {
				t.Fatal(err)
			}
			hits[k.Column] = h
		}
	}
	if !hits["target_owner_id"]["someone"] || hits["target_id"]["p1"] || !hits["target_id"]["a-user"] {
		t.Fatalf("hits = %v: the owner and a user target refuse; a proposal target does not", hits)
	}
}

