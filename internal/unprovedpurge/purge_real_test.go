package unprovedpurge

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// A graph row of an account's delivery refuses it through target_owner_id; the target_id (a proposal) does not.
func TestARealDeliveryRecordRefusesTheOwnerAndNotTheTargetId(t *testing.T) {
	ctx := context.Background()
	svcs := realServices(t)
	graph := svcs[3]
	now := time.Now().UTC()
	if _, err := graph.DB.Exec(`INSERT INTO proposal_delivery_records (delivery_id, proposal_id, target_owner_id, target_kind, target_id, delivery_state, created_at, updated_at) VALUES ('d1', 'p1', 'someone', 'publication_version_proposal', 'goes', 'sent', $1, $1)`, now); err != nil {
		t.Fatal(err)
	}
	keys, err := graph.present(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, k := range keys {
		if k.Table == "proposal_delivery_records" && k.Column == "target_owner_id" {
			found = true
			hits, err := graph.proofHits(ctx, k, []string{"goes", "someone"})
			if err != nil {
				t.Fatal(err)
			}
			if hits["goes"] || !hits["someone"] {
				t.Fatalf("hits = %v: the owner refuses, the target id does not", hits)
			}
		}
	}
	if !found {
		t.Fatal("target_owner_id is not keyed")
	}
	if !strings.Contains(strings.Join(sortedNames(notUserKeyed), ","), "proposal_delivery_records.target_id") {
		t.Fatal("target_id is not reviewed")
	}
}

func sortedNames(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
