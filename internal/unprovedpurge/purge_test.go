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

	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

type world struct {
	t        *testing.T
	auth     *auth.Store
	maild    *sql.DB
	platform *sql.DB
	root     string
	owners   map[string]bool
}

func openSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(on)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func exec(t *testing.T, db *sql.DB, q string, a ...any) {
	t.Helper()
	if _, err := db.Exec(q, a...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// newWorld seeds a proved account, four unproved accounts that must be refused (a computer owner, an
// owner-key holder, the bootstrap admin, one with platform rows) and one unproved account that goes.
func newWorld(t *testing.T) *world {
	t.Helper()
	dir := t.TempDir()
	store, err := auth.OpenStore(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	w := &world{t: t, auth: store, root: filepath.Join(dir, "mail"), owners: map[string]bool{}}
	w.maild = openSQLite(t, filepath.Join(dir, "mail.db"))
	exec(t, w.maild, `CREATE TABLE email_owner_addresses (id text primary key, owner_id text not null, address text)`)
	exec(t, w.maild, `CREATE TABLE email_aliases (id text primary key, target_type text not null, target_id text not null)`)
	exec(t, w.maild, `CREATE TABLE digest_subscriptions (id text primary key, owner_id text not null)`)
	w.platform = openSQLite(t, filepath.Join(dir, "platform.db"))
	exec(t, w.platform, `CREATE TABLE events (id text primary key, owner_id text not null)`)

	now := time.Now().UTC()
	user := func(id, email string, proved bool) {
		if _, err := store.CreateUser(id, email); err != nil {
			t.Fatal(err)
		}
		if !proved {
			exec(t, store.DB(), `UPDATE users SET email_verified_at = NULL WHERE id = ?`, id)
		}
		if err := store.CreateCredential(&auth.Credential{ID: "c-" + id, UserID: id, PublicKey: []byte("k"), AttestationType: "none", Transport: "[]", AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		exec(t, store.DB(), `INSERT INTO refresh_sessions (id, user_id, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`, "rs-"+id, id, "h-"+id, now, now.Add(time.Hour))
		exec(t, w.maild, `INSERT INTO email_owner_addresses (id, owner_id, address) VALUES (?, ?, ?)`, "a-"+id, id, id+"@choir.news")
		exec(t, w.maild, `INSERT INTO email_aliases (id, target_type, target_id) VALUES (?, 'user', ?)`, "al-"+id, id)
		exec(t, w.maild, `INSERT INTO digest_subscriptions (id, owner_id) VALUES (?, ?)`, "d-"+id, id)
		if err := os.MkdirAll(filepath.Join(w.root, "users", id), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(w.root, "users", id, "mailbox.db"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	user("proved", "proved@example.com", true)
	user("goes", "Goes@Example.com", false)
	user("computer", "computer@example.com", false)
	user("rooted", "rooted@example.com", false)
	user("platformed", "platformed@example.com", false)
	user(auth.BootstrapAdminAPIKeyUserID, "bootstrap@choir.local", false)
	w.owners["computer"] = true
	exec(t, store.DB(), `INSERT INTO owner_root_keys (owner_id, created_at) VALUES ('rooted', CURRENT_TIMESTAMP)`)
	exec(t, w.platform, `INSERT INTO events (id, owner_id) VALUES ('e1', 'platformed')`)
	exec(t, store.DB(), `INSERT INTO registration_waitlist (email, created_at, last_seen_at) VALUES ('goes@example.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), ('other@example.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	return w
}

func (w *world) services() []Service {
	return []Service{
		{Name: "auth", DB: w.auth.DB(), Kind: dbx.SQLite, Keys: AuthKeys},
		{Name: "maild", DB: w.maild, Kind: dbx.SQLite, Keys: MaildKeys, MailboxRoot: w.root},
		{Name: "platform", DB: w.platform, Kind: dbx.SQLite, Keys: PlatformKeys},
	}
}

func (w *world) owner(ctx context.Context) (map[string]bool, error) { return w.owners, nil }

func rowsFor(t *testing.T, db *sql.DB, q string, a ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, a...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (w *world) footprint(id string) []int {
	return []int{
		rowsFor(w.t, w.auth.DB(), `SELECT COUNT(*) FROM users WHERE id = ?`, id),
		rowsFor(w.t, w.auth.DB(), `SELECT COUNT(*) FROM credentials WHERE user_id = ?`, id),
		rowsFor(w.t, w.auth.DB(), `SELECT COUNT(*) FROM refresh_sessions WHERE user_id = ?`, id),
		rowsFor(w.t, w.maild, `SELECT COUNT(*) FROM email_owner_addresses WHERE owner_id = ?`, id),
		rowsFor(w.t, w.maild, `SELECT COUNT(*) FROM email_aliases WHERE target_id = ?`, id),
		rowsFor(w.t, w.maild, `SELECT COUNT(*) FROM digest_subscriptions WHERE owner_id = ?`, id),
		rowsFor(w.t, w.platform, `SELECT COUNT(*) FROM events WHERE owner_id = ?`, id),
	}
}

func equal(a, b []int) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDryRunChangesNothingAndCountsExactly(t *testing.T) {
	w := newWorld(t)
	before := map[string][]int{}
	for _, id := range []string{"proved", "goes", "computer", "rooted", "platformed", auth.BootstrapAdminAPIKeyUserID} {
		before[id] = w.footprint(id)
	}
	plan, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Purge) != 1 || plan.Purge[0].ID != "goes" {
		t.Fatalf("purge list = %+v, want only the unproved account that owns nothing", plan.Purge)
	}
	reasons := map[string]string{}
	for _, r := range plan.Refused {
		reasons[r.IDHash] = r.Reason
	}
	for id, want := range map[string]string{"computer": "computer", "rooted": "owner key", auth.BootstrapAdminAPIKeyUserID: "bootstrap", "platformed": "events"} {
		if !strings.Contains(reasons[Hash(id)], want) {
			t.Errorf("%s refused as %q, want it to mention %q", id, reasons[Hash(id)], want)
		}
	}
	if plan.Mailboxes != 1 {
		t.Errorf("mailbox directories = %d, want 1", plan.Mailboxes)
	}
	want := map[string]int64{"auth.users.id": 1, "auth.credentials.user_id": 1, "auth.refresh_sessions.user_id": 1, "auth.registration_waitlist.email": 1, "maild.email_owner_addresses.owner_id": 1, "maild.email_aliases.target_id": 1, "maild.digest_subscriptions.owner_id": 1, "platform.events.owner_id": 0}
	for _, c := range plan.Counts {
		if n, ok := want[c.Service+"."+c.Table+"."+c.Column]; ok && c.Rows != n {
			t.Errorf("%s.%s.%s = %d rows, want %d", c.Service, c.Table, c.Column, c.Rows, n)
		}
	}
	for id, b := range before {
		if !equal(b, w.footprint(id)) {
			t.Errorf("the dry run changed %s", id)
		}
	}
	if plan.Digest != Digest([]string{"goes"}) {
		t.Errorf("digest = %s", plan.Digest)
	}
}

func TestRealRunNeedsTheDigestAndDeletesOnlyTheListed(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	keep := map[string][]int{}
	for _, id := range []string{"proved", "computer", "rooted", "platformed", auth.BootstrapAdminAPIKeyUserID} {
		keep[id] = w.footprint(id)
	}
	// No digest, a wrong digest: nothing goes.
	for _, d := range []string{"", "deadbeef"} {
		if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), d); err == nil {
			t.Fatalf("a run with digest %q was accepted", d)
		}
	}
	if got := w.footprint("goes"); !equal(got, []int{1, 1, 1, 1, 1, 1, 0}) {
		t.Fatalf("a refused run changed the account: %v", got)
	}
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest); err != nil {
		t.Fatal(err)
	}
	if got := w.footprint("goes"); !equal(got, []int{0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("the purged account left rows behind: %v", got)
	}
	if rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM registration_waitlist WHERE email = 'goes@example.com'`) != 0 || rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM registration_waitlist WHERE email = 'other@example.com'`) != 1 {
		t.Fatal("the waitlist entry of the purged address went, or another's did not stay")
	}
	if _, err := os.Stat(filepath.Join(w.root, "users", "goes")); !os.IsNotExist(err) {
		t.Fatal("the mailbox directory of the purged account is still there")
	}
	for id, b := range keep {
		if !equal(b, w.footprint(id)) {
			t.Errorf("%s was touched: %v, then %v", id, b, w.footprint(id))
		}
		if _, err := os.Stat(filepath.Join(w.root, "users", id)); err != nil {
			t.Errorf("the mailbox of %s is gone", id)
		}
	}
	if rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM users WHERE id = 'proved'`) != 1 {
		t.Fatal("the proved account is gone")
	}
}

// A table keyed by a user that the registry does not name stops the run: it is never a table left behind.
func TestAnUnregisteredUserKeyedTableStopsTheRun(t *testing.T) {
	w := newWorld(t)
	exec(t, w.platform, `CREATE TABLE brand_new_things (id text primary key, owner_id text not null)`)
	if _, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, w.services()); !errors.Is(err, ErrUnregistered) || !strings.Contains(err.Error(), "brand_new_things.owner_id") {
		t.Fatalf("dry run err = %v", err)
	}
	if _, err := Apply(context.Background(), w.auth.DB(), w.owner, w.services(), Digest([]string{"goes"})); !errors.Is(err, ErrUnregistered) {
		t.Fatalf("real run err = %v", err)
	}
	if got := w.footprint("goes"); !equal(got, []int{1, 1, 1, 1, 1, 1, 0}) {
		t.Fatalf("a stopped run changed the account: %v", got)
	}
}

// An account proved between the dry run and the real run is no longer on the list, so the digest is no longer right.
func TestAnAccountProvedBetweenTheRunsIsNotDeleted(t *testing.T) {
	w := newWorld(t)
	plan, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	exec(t, w.auth.DB(), `UPDATE users SET email_verified_at = CURRENT_TIMESTAMP WHERE id = 'goes'`)
	if _, err := Apply(context.Background(), w.auth.DB(), w.owner, w.services(), plan.Digest); err == nil {
		t.Fatal("the run went ahead on a list that had changed")
	}
	if got := w.footprint("goes"); !equal(got, []int{1, 1, 1, 1, 1, 1, 0}) {
		t.Fatalf("the account that was proved lost rows: %v", got)
	}
}

// vmctl being unreachable is not "owns nothing".
func TestWithoutTheComputerOwnersNothingIsPlanned(t *testing.T) {
	w := newWorld(t)
	_, err := BuildPlan(context.Background(), w.auth.DB(), func(context.Context) (map[string]bool, error) { return nil, errors.New("vmctl down") }, w.services())
	if err == nil {
		t.Fatal("a plan was made without knowing who owns a computer")
	}
}
