// Command purge-unproved deletes the accounts whose address was never proved. An operator runs it by
// hand; nothing else does (no service, boot step or timer). It is black class: it deletes accounts
// for good, and docs/ship/purge-unproved.md lists every table it touches and the key of each.
//
//	purge-unproved --auth-db FILE --maild-db FILE --maild-root DIR \
//	               --pg-control-dsn DSN --pg-graph-dsn DSN --vmctl-url URL [--internal-token-file FILE]
//	purge-unproved ... --apply --digest HEX --receipt FILE
//
// With no --apply it only reads: it prints, per service and table, the rows that would go, the
// accounts it refuses (hashed) and why, and the digest of the list. The real run (--apply) needs that
// digest and a receipt path, recomputes the plan, takes auth's write lock, and if any listed account
// changed deletes nothing. Stop go-choir-auth and go-choir-maild before the first --apply and keep them stopped until a rerun has committed (see the doc); backups
// (restic snapshots on node-a) keep what they held until they are pruned.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/dbcred"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/unprovedpurge"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// exit is how usage and fatal stop a run: run recovers it, so every deferred close still happens and a test can
// drive the whole command.
type exit struct{ code int }

func run(args []string, out, errw io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(exit)
			if !ok {
				panic(r)
			}
			code = e.code
		}
	}()
	stderr = errw
	fs := flag.NewFlagSet("purge-unproved", flag.ContinueOnError)
	fs.SetOutput(errw)
	var (
		authDB     = fs.String("auth-db", "", "path of the auth SQLite database (required)")
		maildDB    = fs.String("maild-db", "", "path of the mail routing SQLite database (required with --maild-root)")
		maildRoot  = fs.String("maild-root", "", "mail storage root: users/<id> and attachments/outbound/<id> are removed there (required with --maild-db)")
		pgControl  = fs.String("pg-control-dsn", "", "the PostgreSQL control database DSN (required; the password comes from the usual credential)")
		pgGraph    = fs.String("pg-graph-dsn", "", "the PostgreSQL graph database DSN (required)")
		vmctlURL   = fs.String("vmctl-url", "", "vmctl's host-internal URL (required: who owns a computer)")
		tokenFile  = fs.String("internal-token-file", "", "the host-internal token file vmctl's list needs (else CHOIR_INTERNAL_TOKEN_FILE or the credentials directory)")
		emptyOK    = fs.Bool("vmctl-empty-ok", false, "accept an empty vmctl registry as 'nobody owns a computer' (refused by default: an unloaded registry looks the same)")
		allowMiss  = fs.String("allow-missing-service", "", "comma list of services (maild, control, graph) allowed to be absent; the run says so loudly")
		apply      = fs.Bool("apply", false, "delete (needs --digest and --receipt)")
		digest     = fs.String("digest", "", "the digest the dry run printed")
		receiptTo  = fs.String("receipt", "", "append a deletion receipt (JSON line) to this file; required with --apply")
		receiptKey = fs.String("receipt-key-file", "", "optional: a key file; the receipt then carries a keyed hash of each address so a later \"was my account deleted\" can be answered")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	missing := map[string]bool{}
	for _, m := range strings.Split(*allowMiss, ",") {
		if m = strings.TrimSpace(m); m != "" {
			missing[m] = true
		}
	}
	if *authDB == "" || *vmctlURL == "" {
		usage("--auth-db and --vmctl-url are required")
	}
	if (*maildDB == "") != (*maildRoot == "") {
		usage("--maild-db and --maild-root go together: a mail database with no root leaves every mailbox on disk")
	}
	need := func(name, v string) {
		if v == "" && !missing[name] {
			usage("--" + map[string]string{"maild": "maild-db and --maild-root", "control": "pg-control-dsn", "graph": "pg-graph-dsn"}[name] + " is required (or --allow-missing-service " + name + ", which is printed loudly)")
		}
	}
	need("maild", *maildDB)
	need("control", *pgControl)
	need("graph", *pgGraph)
	if *apply && (*digest == "" || *receiptTo == "") {
		usage("--apply needs --digest and --receipt")
	}
	if *pgControl != "" && *pgControl == *pgGraph {
		usage("--pg-control-dsn and --pg-graph-dsn are the same: the graph database's tables would be skipped (give each its own)")
	}
	if *maildRoot != "" {
		*maildRoot = filepath.Clean(*maildRoot)
	}
	// What the last run left is read BEFORE the receipt is opened (opening creates the file); then the receipt
	// and its key are checked and opened, before anything is deleted.
	if *receiptTo != "" {
		if prev, err := unprovedpurge.LastIncomplete(*receiptTo); err != nil {
			fatal("%v", err)
		} else if prev != "" {
			fmt.Fprintf(errw, "purge-unproved: WARNING: %s\n", prev)
		}
	}
	var journal *unprovedpurge.FileJournal
	if *apply {
		j, err := unprovedpurge.OpenJournal(*receiptTo, *receiptKey)
		if err != nil {
			fatal("%v (nothing was changed)", err)
		}
		journal = j
		defer j.Close()
	}
	if *tokenFile != "" {
		os.Setenv(hostauth.EnvTokenFile, *tokenFile)
	}
	for _, m := range sortedKeys(missing) {
		fmt.Fprintf(errw, "purge-unproved: WARNING: the %s service is not part of this run (--allow-missing-service): its rows and files are NOT deleted and its Proof checks do NOT run\n", m)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	open := func(path string, immediate bool) *sql.DB {
		db, err := unprovedpurge.OpenSQLiteFile(path, *apply, immediate)
		if err != nil {
			fatal("%v", err)
		}
		return db
	}
	auth := open(*authDB, true)
	defer auth.Close()
	services := []unprovedpurge.Service{{Name: "auth", DB: auth, Kind: dbx.SQLite, Keys: unprovedpurge.AuthKeys, CoreAny: []string{"users"}}}
	if *maildDB != "" {
		m := open(*maildDB, false)
		defer m.Close()
		services = append(services, unprovedpurge.Service{Name: "maild", DB: m, Kind: dbx.SQLite, Keys: unprovedpurge.MaildKeys, MailboxRoot: *maildRoot, CoreAny: []string{"email_aliases", "maild_migrations"}})
	}
	for _, p := range []struct {
		name, dsn string
		core      []string
	}{{"control", *pgControl, unprovedpurge.ControlCore}, {"graph", *pgGraph, unprovedpurge.GraphCore}} {
		if p.dsn == "" {
			continue
		}
		full, err := dbcred.ApplyPGDSN(p.dsn)
		if err != nil {
			fatal("postgres credential: %v", err)
		}
		db, err := dbx.OpenPostgres(full, dbx.PGOptions{})
		if err != nil {
			fatal("open postgres %s: %v", p.name, err)
		}
		defer db.Close()
		services = append(services, unprovedpurge.Service{Name: p.name, DB: db, Kind: dbx.Postgres, Keys: unprovedpurge.PlatformKeys, CoreAny: p.core})
	}
	client := vmctl.NewClient(*vmctlURL)
	owners := func(ctx context.Context) (map[string]bool, error) {
		list, err := client.ListOwnershipsContext(ctx)
		if err != nil {
			return nil, err
		}
		out := map[string]bool{}
		for _, o := range list {
			if id := strings.TrimSpace(o.UserID); id != "" {
				out[id] = true
			}
		}
		if len(out) == 0 && !*emptyOK {
			return nil, fmt.Errorf("vmctl answered an empty registry: an unloaded registry looks the same as one with no computers; check vmctl's state directory, or pass --vmctl-empty-ok if there really are none")
		}
		return out, nil
	}

	if !*apply {
		plan, err := unprovedpurge.BuildPlan(ctx, auth, owners, services)
		if plan != nil {
			print(out, plan, "DRY RUN (nothing was changed)")
		}
		if err != nil {
			fatal("%v", err)
		}
		return 0
	}
	res, err := unprovedpurge.Apply(ctx, auth, owners, services, *digest, journal)
	if res != nil && res.Plan != nil {
		print(out, res.Plan, "APPLY")
		for _, s := range res.Services {
			state := "COMMITTED"
			if !s.Committed {
				state = "NOT committed: " + fmt.Sprint(s.Err)
			}
			fmt.Fprintf(out, "service %s: %s\n", s.Name, state)
		}
		fmt.Fprintf(out, "mail directories removed: %d\n", res.MailDirs)
	}
	if err != nil {
		fatal("%v", err)
	}
	return 0
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func print(w io.Writer, p *unprovedpurge.Plan, mode string) {
	fmt.Fprintf(w, "purge-unproved: %s\n", mode)
	fmt.Fprintf(w, "services: %s\n", strings.Join(p.Services, ", "))
	fmt.Fprintf(w, "accounts to purge: %d\n", len(p.Purge))
	hashes := make([]string, 0, len(p.Purge))
	for _, c := range p.Purge {
		hashes = append(hashes, unprovedpurge.Hash(c.ID))
	}
	sort.Strings(hashes)
	for _, h := range hashes {
		fmt.Fprintf(w, "  purge %s\n", h)
	}
	fmt.Fprintf(w, "accounts refused: %d\n", len(p.Refused))
	for _, r := range p.Refused {
		fmt.Fprintf(w, "  refused %s: %s\n", r.IDHash, r.Reason)
	}
	fmt.Fprintln(w, "rows per table (service.table.column):")
	for _, c := range p.Counts {
		fmt.Fprintf(w, "  %s.%s.%s %d\n", c.Service, c.Table, c.Column, c.Rows)
	}
	fmt.Fprintf(w, "mail directories: %d\n", p.Mailboxes)
	fmt.Fprintf(w, "digest (over the sorted accounts with their state, the services and the counts; order does not matter): %s\n", p.Digest)
}

var stderr io.Writer = os.Stderr

func usage(msg string) {
	fmt.Fprintf(stderr, "purge-unproved: %s\n", msg)
	panic(exit{2})
}

func fatal(f string, a ...any) {
	fmt.Fprintf(stderr, "purge-unproved: "+f+"\n", a...)
	panic(exit{1})
}
