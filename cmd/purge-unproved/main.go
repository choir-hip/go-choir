// Command purge-unproved deletes the accounts whose address was never proved. An operator runs it by
// hand; nothing else does (no service, boot step or timer). It is black class: it deletes accounts
// for good, and docs/ship/purge-unproved.md lists every table it touches and the key of each.
//
//	purge-unproved --auth-db FILE [--maild-db FILE --maild-root DIR] [--pg-dsn DSN ...] --vmctl-url URL
//	               [--apply --digest HEX]
//
// With no --apply it only reads: it prints, per service and table, the rows that would go, the
// accounts it refuses (hashed) and why, and the digest of the list. The real run (--apply) needs that
// digest and recomputes the plan; if the list differs it deletes nothing. It refuses to run without
// --vmctl-url: whether an account owns or ever owned a computer is not guessed.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/dbcred"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/unprovedpurge"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	var (
		authDB    = flag.String("auth-db", "", "path of the auth SQLite database (required)")
		maildDB   = flag.String("maild-db", "", "path of the mail routing SQLite database")
		maildRoot = flag.String("maild-root", "", "mail storage root (users/<id> mailboxes are removed there)")
		vmctlURL  = flag.String("vmctl-url", "", "vmctl's host-internal URL (required: who owns a computer)")
		apply     = flag.Bool("apply", false, "delete (needs --digest)")
		digest    = flag.String("digest", "", "the digest the dry run printed")
		pg        multi
	)
	flag.Var(&pg, "pg-dsn", "a PostgreSQL DSN (control, graph); repeatable; the password comes from the usual credential")
	flag.Parse()
	if *authDB == "" || *vmctlURL == "" {
		fmt.Fprintln(os.Stderr, "purge-unproved: --auth-db and --vmctl-url are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	open := func(path string) *sql.DB {
		db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(60000)&_pragma=foreign_keys(on)&_txlock=immediate")
		if err != nil {
			fatal("open %s: %v", path, err)
		}
		db.SetMaxOpenConns(1)
		return db
	}
	auth := open(*authDB)
	defer auth.Close()
	services := []unprovedpurge.Service{{Name: "auth", DB: auth, Kind: dbx.SQLite, Keys: unprovedpurge.AuthKeys}}
	if *maildDB != "" {
		m := open(*maildDB)
		defer m.Close()
		services = append(services, unprovedpurge.Service{Name: "maild", DB: m, Kind: dbx.SQLite, Keys: unprovedpurge.MaildKeys, MailboxRoot: *maildRoot})
	}
	for i, dsn := range pg {
		full, err := dbcred.ApplyPGDSN(dsn)
		if err != nil {
			fatal("postgres credential: %v", err)
		}
		db, err := dbx.OpenPostgres(full, dbx.PGOptions{})
		if err != nil {
			fatal("open postgres %d: %v", i+1, err)
		}
		defer db.Close()
		services = append(services, unprovedpurge.Service{Name: fmt.Sprintf("postgres-%d", i+1), DB: db, Kind: dbx.Postgres, Keys: unprovedpurge.PlatformKeys})
	}
	client := vmctl.NewClient(*vmctlURL)
	owners := func(ctx context.Context) (map[string]bool, error) {
		list, err := client.ListOwnershipsContext(ctx)
		if err != nil {
			return nil, err
		}
		out := map[string]bool{}
		for _, o := range list {
			out[o.UserID] = true
		}
		return out, nil
	}

	var plan *unprovedpurge.Plan
	var err error
	if *apply {
		plan, err = unprovedpurge.Apply(ctx, auth, owners, services, *digest)
	} else {
		plan, err = unprovedpurge.BuildPlan(ctx, auth, owners, services)
	}
	if plan != nil {
		print(plan, *apply && err == nil)
	}
	if err != nil {
		fatal("%v", err)
	}
}

func print(p *unprovedpurge.Plan, applied bool) {
	mode := "DRY RUN (nothing was changed)"
	if applied {
		mode = "APPLIED"
	}
	fmt.Printf("purge-unproved: %s\n", mode)
	fmt.Printf("accounts to purge: %d\n", len(p.Purge))
	hashes := make([]string, 0, len(p.Purge))
	for _, c := range p.Purge {
		hashes = append(hashes, unprovedpurge.Hash(c.ID))
	}
	sort.Strings(hashes)
	for _, h := range hashes {
		fmt.Printf("  purge %s\n", h)
	}
	fmt.Printf("accounts refused: %d\n", len(p.Refused))
	for _, r := range p.Refused {
		fmt.Printf("  refused %s: %s\n", r.IDHash, r.Reason)
	}
	fmt.Println("rows per table (service.table.column):")
	for _, c := range p.Counts {
		verb := "delete"
		if c.Clear {
			verb = "clear"
		}
		fmt.Printf("  %-8s %s.%s.%s %d\n", verb, c.Service, c.Table, c.Column, c.Rows)
	}
	fmt.Printf("mailbox directories: %d\n", p.Mailboxes)
	fmt.Printf("digest: %s\n", p.Digest)
}

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "purge-unproved: "+f+"\n", a...)
	os.Exit(1)
}
