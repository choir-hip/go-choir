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
// changed deletes nothing. Stop go-choir-auth and go-choir-maild for the run (see the doc); backups
// (restic snapshots on node-a) keep what they held until they are pruned.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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

func main() {
	var (
		authDB     = flag.String("auth-db", "", "path of the auth SQLite database (required)")
		maildDB    = flag.String("maild-db", "", "path of the mail routing SQLite database (required with --maild-root)")
		maildRoot  = flag.String("maild-root", "", "mail storage root: users/<id> and attachments/outbound/<id> are removed there (required with --maild-db)")
		pgControl  = flag.String("pg-control-dsn", "", "the PostgreSQL control database DSN (required; the password comes from the usual credential)")
		pgGraph    = flag.String("pg-graph-dsn", "", "the PostgreSQL graph database DSN (required)")
		vmctlURL   = flag.String("vmctl-url", "", "vmctl's host-internal URL (required: who owns a computer)")
		tokenFile  = flag.String("internal-token-file", "", "the host-internal token file vmctl's list needs (else CHOIR_INTERNAL_TOKEN_FILE or the credentials directory)")
		emptyOK    = flag.Bool("vmctl-empty-ok", false, "accept an empty vmctl registry as 'nobody owns a computer' (refused by default: an unloaded registry looks the same)")
		allowMiss  = flag.String("allow-missing-service", "", "comma list of services (maild, control, graph) allowed to be absent; the run says so loudly")
		apply      = flag.Bool("apply", false, "delete (needs --digest and --receipt)")
		digest     = flag.String("digest", "", "the digest the dry run printed")
		receiptTo  = flag.String("receipt", "", "append a deletion receipt (JSON line) to this file; required with --apply")
		receiptKey = flag.String("receipt-key-file", "", "optional: a key file; the receipt then carries a keyed hash of each address so a later \"was my account deleted\" can be answered")
	)
	flag.Parse()
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
	if *tokenFile != "" {
		os.Setenv(hostauth.EnvTokenFile, *tokenFile)
	}
	for _, m := range sortedKeys(missing) {
		fmt.Fprintf(os.Stderr, "purge-unproved: WARNING: the %s service is not part of this run (--allow-missing-service): its rows and files are NOT deleted and its Proof checks do NOT run\n", m)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	open := func(path string, immediate bool) *sql.DB {
		if st, err := os.Stat(path); err != nil || st.IsDir() {
			fatal("%s is not an existing database file (a mistyped path would make an empty one)", path)
		}
		dsn := path + "?_pragma=busy_timeout(60000)&_pragma=foreign_keys(on)&_pragma=secure_delete(on)"
		if immediate {
			dsn += "&_txlock=immediate"
		}
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			fatal("open %s: %v", path, err)
		}
		db.SetMaxOpenConns(1)
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
	for _, p := range []struct{ name, dsn string }{{"control", *pgControl}, {"graph", *pgGraph}} {
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
		services = append(services, unprovedpurge.Service{Name: p.name, DB: db, Kind: dbx.Postgres, Keys: unprovedpurge.PlatformKeys, CoreAny: platformCore()})
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
			print(plan, "DRY RUN (nothing was changed)")
		}
		if err != nil {
			fatal("%v", err)
		}
		return
	}
	res, err := unprovedpurge.Apply(ctx, auth, owners, services, *digest)
	if res != nil && res.Plan != nil {
		print(res.Plan, "APPLY")
		for _, s := range res.Services {
			state := "COMMITTED"
			if !s.Committed {
				state = "NOT committed: " + fmt.Sprint(s.Err)
			}
			fmt.Printf("service %s: %s\n", s.Name, state)
		}
		fmt.Printf("mail directories removed: %d\n", res.MailDirs)
	}
	if res != nil && res.Plan != nil {
		if rerr := writeReceipt(*receiptTo, *receiptKey, res, err); rerr != nil {
			fmt.Fprintf(os.Stderr, "purge-unproved: the receipt could not be written: %v\n", rerr)
		}
	}
	if err != nil {
		fatal("%v", err)
	}
}

func platformCore() []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range unprovedpurge.PlatformKeys {
		if !seen[k.Table] {
			seen[k.Table] = true
			out = append(out, k.Table)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// writeReceipt appends one JSON line: when, the digest, the services and what each did, the row counts, and the
// full hash of each purged id (and, with a key file, a keyed hash of each address).
func writeReceipt(path, keyFile string, res *unprovedpurge.Result, runErr error) error {
	type svc struct {
		Name      string `json:"name"`
		Committed bool   `json:"committed"`
		Error     string `json:"error,omitempty"`
	}
	rec := struct {
		At       string           `json:"at"`
		Digest   string           `json:"digest"`
		Error    string           `json:"error,omitempty"`
		Services []svc            `json:"services"`
		Counts   map[string]int64 `json:"rows"`
		IDs      []string         `json:"id_sha256"`
		Emails   []string         `json:"email_hmac_sha256,omitempty"`
	}{At: time.Now().UTC().Format(time.RFC3339), Digest: res.Plan.Digest, Counts: map[string]int64{}}
	if runErr != nil {
		rec.Error = runErr.Error()
	}
	for _, s := range res.Services {
		e := ""
		if s.Err != nil {
			e = s.Err.Error()
		}
		rec.Services = append(rec.Services, svc{s.Name, s.Committed, e})
	}
	for _, c := range res.Plan.Counts {
		rec.Counts[c.Service+"."+c.Table+"."+c.Column] = c.Rows
	}
	var key []byte
	if keyFile != "" {
		k, err := os.ReadFile(keyFile)
		if err != nil {
			return err
		}
		key = k
	}
	for _, c := range res.Plan.Purge {
		h := sha256.Sum256([]byte(c.ID))
		rec.IDs = append(rec.IDs, hex.EncodeToString(h[:]))
		if key != nil {
			rec.Emails = append(rec.Emails, unprovedpurge.EmailMAC(key, c.Email))
		}
	}
	sort.Strings(rec.IDs)
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func print(p *unprovedpurge.Plan, mode string) {
	fmt.Printf("purge-unproved: %s\n", mode)
	fmt.Printf("services: %s\n", strings.Join(p.Services, ", "))
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
		fmt.Printf("  %s.%s.%s %d\n", c.Service, c.Table, c.Column, c.Rows)
	}
	fmt.Printf("mail directories: %d\n", p.Mailboxes)
	fmt.Printf("digest (over the sorted accounts with their state, the services and the counts; order does not matter): %s\n", p.Digest)
}

func usage(msg string) {
	fmt.Fprintf(os.Stderr, "purge-unproved: %s\n", msg)
	os.Exit(2)
}

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "purge-unproved: "+f+"\n", a...)
	os.Exit(1)
}
