package unprovedpurge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// newWorld seeds a proved account, and unproved accounts: one that goes, and ones that must be refused (a computer
// owner, an owner-key holder, the bootstrap admin, one with platform rows, one with route-ledger rows).
func newWorld(t *testing.T) *world {
	t.Helper()
	dir := t.TempDir()
	store, err := auth.OpenStore(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	w := &world{t: t, auth: store, root: filepath.Join(dir, "mail"), owners: map[string]bool{"__any__": true}}
	w.maild = openSQLite(t, filepath.Join(dir, "mail.db"))
	exec(t, w.maild, `CREATE TABLE email_owner_addresses (id text primary key, owner_id text not null, address text)`)
	exec(t, w.maild, `CREATE TABLE email_aliases (id text primary key, target_type text not null, target_id text not null)`)
	exec(t, w.maild, `CREATE TABLE digest_subscriptions (id text primary key, owner_id text not null)`)
	exec(t, w.maild, `CREATE TABLE email_provider_message_index (provider text not null, provider_message_id text not null, owner_id text not null)`)
	exec(t, w.maild, `CREATE TABLE email_webhook_events (id text primary key, provider text not null, provider_message_id text, raw_payload text not null)`)
	w.platform = openSQLite(t, filepath.Join(dir, "platform.db"))
	exec(t, w.platform, `CREATE TABLE events (id text primary key, owner_id text not null)`)
	exec(t, w.platform, `CREATE TABLE computer_version_route_slots (route_slot_id text primary key, current_code_ref text)`)

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
		exec(t, w.maild, `INSERT INTO email_provider_message_index (provider, provider_message_id, owner_id) VALUES ('p', ?, ?)`, "m-"+id, id)
		exec(t, w.maild, `INSERT INTO email_webhook_events (id, provider, provider_message_id, raw_payload) VALUES (?, 'p', ?, ?)`, "w-"+id, "m-"+id, `{"to":"`+id+`@choir.news"}`)
		for _, d := range mailDirs(w.root, id) {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(d, "f"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Join(w.root, "users"), 0o755); err != nil {
		t.Fatal(err)
	}
	user("proved", "proved@example.com", true)
	user("goes", "Goes@Example.com", false)
	user("computer", "computer@example.com", false)
	user("rooted", "rooted@example.com", false)
	user("platformed", "platformed@example.com", false)
	user("ledgered", "ledgered@example.com", false)
	user(auth.BootstrapAdminAPIKeyUserID, "bootstrap@choir.local", false)
	w.owners["computer"] = true
	exec(t, store.DB(), `INSERT INTO owner_root_keys (owner_id, created_at) VALUES ('rooted', CURRENT_TIMESTAMP)`)
	exec(t, w.platform, `INSERT INTO events (id, owner_id) VALUES ('e1', 'platformed')`)
	exec(t, w.platform, `INSERT INTO computer_version_route_slots (route_slot_id, current_code_ref) VALUES ('computer:ledgered:c1', 'x')`)
	exec(t, store.DB(), `INSERT INTO registration_waitlist (email, created_at, last_seen_at) VALUES ('goes@example.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), ('other@example.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	return w
}

func (w *world) services() []Service {
	return []Service{
		{Name: "auth", DB: w.auth.DB(), Kind: dbx.SQLite, Keys: AuthKeys, CoreAny: []string{"users"}},
		{Name: "maild", DB: w.maild, Kind: dbx.SQLite, Keys: MaildKeys, MailboxRoot: w.root, CoreAny: []string{"email_aliases"}},
		{Name: "platform", DB: w.platform, Kind: dbx.SQLite, Keys: PlatformKeys, CoreAny: []string{"events"}},
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
	dirs := 0
	for _, d := range mailDirs(w.root, id) {
		if _, err := os.Lstat(d); err == nil {
			dirs++
		}
	}
	return []int{
		rowsFor(w.t, w.auth.DB(), `SELECT COUNT(*) FROM users WHERE id = ?`, id),
		rowsFor(w.t, w.auth.DB(), `SELECT COUNT(*) FROM credentials WHERE user_id = ?`, id),
		rowsFor(w.t, w.auth.DB(), `SELECT COUNT(*) FROM refresh_sessions WHERE user_id = ?`, id),
		rowsFor(w.t, w.maild, `SELECT COUNT(*) FROM email_owner_addresses WHERE owner_id = ?`, id),
		rowsFor(w.t, w.maild, `SELECT COUNT(*) FROM email_aliases WHERE target_id = ?`, id),
		rowsFor(w.t, w.maild, `SELECT COUNT(*) FROM digest_subscriptions WHERE owner_id = ?`, id),
		rowsFor(w.t, w.platform, `SELECT COUNT(*) FROM events WHERE owner_id = ?`, id),
		dirs,
		rowsFor(w.t, w.maild, `SELECT COUNT(*) FROM email_webhook_events WHERE provider_message_id = ?`, "m-"+id),
	}
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var whole = []int{1, 1, 1, 1, 1, 1, 0, 2, 1}
var gone = []int{0, 0, 0, 0, 0, 0, 0, 0, 0}

func TestDryRunChangesNothingAndCountsExactly(t *testing.T) {
	w := newWorld(t)
	before := map[string][]int{}
	for _, id := range []string{"proved", "goes", "computer", "rooted", "platformed", "ledgered", auth.BootstrapAdminAPIKeyUserID} {
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
	for id, want := range map[string]string{"computer": "computer", "rooted": "owner key", auth.BootstrapAdminAPIKeyUserID: "bootstrap", "platformed": "events", "ledgered": "computer_version_route_slots"} {
		if !strings.Contains(reasons[Hash(id)], want) {
			t.Errorf("%s refused as %q, want it to mention %q", id, reasons[Hash(id)], want)
		}
	}
	if plan.Mailboxes != 2 {
		t.Errorf("mail directories = %d, want 2 (mailbox and outbound attachments)", plan.Mailboxes)
	}
	// The exact expected list: a table missing from the plan fails here, not silently.
	want := map[string]int64{
		"auth.credentials.user_id": 1, "auth.refresh_sessions.user_id": 1, "auth.challenge_state.user_id": 0, "auth.api_keys.user_id": 0,
		"auth.pending_registrations.email": 0, "auth.registration_waitlist.email": 1, "auth.users.id": 1, "auth.account_set (repaired)": 0,
		"maild.email_owner_addresses.owner_id": 1, "maild.email_aliases.target_id": 1, "maild.digest_subscriptions.owner_id": 1,
		"platform.events.owner_id": 0, "platform.computer_version_route_slots.route_slot_id": 0,
	}
	seen := map[string]bool{}
	for _, c := range plan.Counts {
		k := c.Service + "." + c.Table + "." + c.Column
		if c.Table == "account_set" {
			k = "auth.account_set (repaired)"
		}
		seen[k] = true
		if n, ok := want[k]; ok && c.Rows != n {
			t.Errorf("%s = %d rows, want %d", k, c.Rows, n)
		}
	}
	for k := range want {
		if !seen[k] {
			t.Errorf("the plan lists no %s", k)
		}
	}
	for id, b := range before {
		if !equal(b, w.footprint(id)) {
			t.Errorf("the dry run changed %s", id)
		}
	}
}

func TestRealRunNeedsTheDigestAndDeletesOnlyTheListed(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	keep := map[string][]int{}
	for _, id := range []string{"proved", "computer", "rooted", "platformed", "ledgered", auth.BootstrapAdminAPIKeyUserID} {
		keep[id] = w.footprint(id)
	}
	for _, d := range []string{"", "deadbeef"} {
		if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), d, nil); err == nil {
			t.Fatalf("a run with digest %q was accepted", d)
		}
	}
	if got := w.footprint("goes"); !equal(got, whole) {
		t.Fatalf("a refused run changed the account: %v", got)
	}
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	res, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Services) != 3 || !res.Services[len(res.Services)-1].Committed || res.Services[len(res.Services)-1].Name != "auth" {
		t.Fatalf("services = %+v, want maild and platform, then auth last, all committed", res.Services)
	}
	if got := w.footprint("goes"); !equal(got, gone) {
		t.Fatalf("the purged account left rows or directories behind: %v", got)
	}
	if rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM registration_waitlist WHERE email = 'goes@example.com'`) != 0 || rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM registration_waitlist WHERE email = 'other@example.com'`) != 1 {
		t.Fatal("the waitlist entry of the purged address went, or another's did not stay")
	}
	for id, b := range keep {
		if !equal(b, w.footprint(id)) {
			t.Errorf("%s was touched: %v, then %v", id, b, w.footprint(id))
		}
	}
}

// The seat's scenario: an account proves between the plan and the lock. Nothing of it may be deleted, in any
// service, and the answer says so.
func TestAnAccountProvedAfterThePlanLosesNothingInAnyService(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	afterPlanHook = func() {
		exec(t, w.auth.DB(), `UPDATE users SET email_verified_at = CURRENT_TIMESTAMP WHERE id = 'goes'`)
	}
	t.Cleanup(func() { afterPlanHook = nil })
	res, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, nil)
	if !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged", err)
	}
	if got := w.footprint("goes"); !equal(got, whole) {
		t.Fatalf("an account that proved itself lost rows or directories (maild committed before auth checked?): %v", got)
	}
	for _, s := range res.Services {
		if s.Committed {
			t.Fatalf("service %s committed before the check", s.Name)
		}
	}
}

// A passkey added after the plan is a change too, though the address is still unproved.
func TestAnAccountThatChangedAfterThePlanIsNotDeleted(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	plan, _ := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	afterPlanHook = func() {
		exec(t, w.auth.DB(), `INSERT INTO credentials (id, user_id, public_key, attestation_type, transport, sign_count, aaguid, flags, name, created_at) VALUES ('c2', 'goes', x'01', 'none', '[]', 0, x'00', '{}', '', CURRENT_TIMESTAMP)`)
	}
	t.Cleanup(func() { afterPlanHook = nil })
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, nil); !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged", err)
	}
	if got := w.footprint("goes"); got[3] != 1 || got[7] != 2 {
		t.Fatalf("the account lost mail rows or directories: %v", got)
	}
}

// A table keyed by a user that the registry does not name stops the run, by name and by suffix.
func TestAnUnregisteredUserKeyedTableStopsTheRun(t *testing.T) {
	for _, col := range []string{"owner_id", "source_owner_id", "submitter_id", "target_owner_id", "requested_by", "account_id", "mailbox_owner_id"} {
		w := newWorld(t)
		exec(t, w.platform, fmt.Sprintf(`CREATE TABLE brand_new_things (id text primary key, %s text not null)`, col))
		if _, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, w.services()); !errors.Is(err, ErrUnregistered) || !strings.Contains(err.Error(), "brand_new_things."+col) {
			t.Fatalf("%s: dry run err = %v", col, err)
		}
		if _, err := Apply(context.Background(), w.auth.DB(), w.owner, w.services(), Digest([]string{"goes"}), nil); !errors.Is(err, ErrUnregistered) {
			t.Fatalf("%s: real run err = %v", col, err)
		}
		if got := w.footprint("goes"); !equal(got, whole) {
			t.Fatalf("%s: a stopped run changed the account: %v", col, got)
		}
	}
}

// The composite route slot "computer:<owner>:<computer>" refuses the owner; so do the tables the first list missed.
func TestRouteLedgerProposalAndUnwrapRowsRefuseTheAccount(t *testing.T) {
	w := newWorld(t)
	exec(t, w.platform, `CREATE TABLE publication_version_proposals (id text primary key, source_owner_id text, submitter_id text)`)
	exec(t, w.platform, `CREATE TABLE proposal_delivery_records (id text primary key, target_owner_id text, target_kind text, target_id text)`)
	exec(t, w.platform, `CREATE TABLE computer_key_unwrap_requests (request_id text primary key, requested_by text)`)
	exec(t, w.platform, `INSERT INTO publication_version_proposals VALUES ('p1', 'someone', 'goes')`)
	plan, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Purge {
		if c.ID == "goes" {
			t.Fatal("an account that submitted a proposal was listed for purge")
		}
	}
}

// An address a surviving account holds in another case is that account's: its recovery links, pending sign-up and waitlist entry stay.
func TestACaseVariantOfAKeptAddressIsLeftAlone(t *testing.T) {
	w := newWorld(t)
	// "proved@example.com" is the kept account's; an unproved account holds "Proved@Example.com".
	exec(t, w.auth.DB(), `INSERT INTO users (id, email, created_at) VALUES ('variant', 'Proved@Example.com', CURRENT_TIMESTAMP)`)
	exec(t, w.auth.DB(), `INSERT INTO registration_waitlist (email, created_at, last_seen_at) VALUES ('proved@example.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	exec(t, w.auth.DB(), `INSERT INTO pending_registrations (challenge, email, user_id, session_data, created_at, expires_at) VALUES ('ch', 'proved@example.com', 'proved', '{}', CURRENT_TIMESTAMP, datetime('now', '+5 minutes'))`)
	ctx := context.Background()
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	var variant bool
	for _, c := range plan.Purge {
		variant = variant || c.ID == "variant"
	}
	if !variant {
		t.Fatal("the unproved case variant was not planned for purge")
	}
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, nil); err != nil {
		t.Fatal(err)
	}
	if rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM registration_waitlist WHERE email = 'proved@example.com'`) != 1 {
		t.Fatal("the waitlist entry of the kept account's address went with a case variant")
	}
	if rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM pending_registrations WHERE challenge = 'ch'`) != 1 {
		t.Fatal("a pending sign-up of the kept account went with a case variant")
	}
}

func TestUnsafeIdsAreRefusedAndNeverReachAPath(t *testing.T) {
	for _, id := range []string{"", ".", "..", "../x", "a/b", `a\b`, ".hidden", "a b", "x;y", strings.Repeat("a", 200)} {
		if SafeID(id) {
			t.Errorf("%q was taken for a safe id", id)
		}
	}
	for _, id := range []string{"3f2a9c1e-0b7d-4a55-9d3e-1c2b3a4d5e6f", "u_1", "choir-bootstrap-admin"} {
		if !SafeID(id) {
			t.Errorf("%q was refused", id)
		}
	}
	w := newWorld(t)
	for _, bad := range []string{"..", ".", "", "../../etc"} {
		exec(t, w.auth.DB(), `PRAGMA foreign_keys = OFF`)
		exec(t, w.auth.DB(), `INSERT INTO users (id, email, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, bad, "bad"+fmt.Sprint(len(bad))+"@example.com")
	}
	plan, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Purge {
		if !SafeID(c.ID) {
			t.Fatalf("an unsafe id %q is on the purge list", c.ID)
		}
	}
	if _, err := Apply(context.Background(), w.auth.DB(), w.owner, w.services(), plan.Digest, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.root, "users")); err != nil {
		t.Fatal("the users directory of the mail root is gone")
	}
	if got := w.footprint("proved"); !equal(got, whole) {
		t.Fatalf("a kept mailbox was removed: %v", got)
	}
}

func TestASymlinkedMailRootIsRefused(t *testing.T) {
	w := newWorld(t)
	target := t.TempDir()
	if err := os.RemoveAll(filepath.Join(w.root, "users")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(w.root, "users")); err != nil {
		t.Skip("no symlinks here")
	}
	plan, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), w.auth.DB(), w.owner, w.services(), plan.Digest, nil); err == nil {
		t.Fatal("the run went ahead with users/ a symlink")
	}
	if rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM users WHERE id = 'goes'`) != 1 {
		t.Fatal("an account was deleted although the mail root was refused")
	}
}

// A browser's account set shared with a kept account stays whole: the kept member becomes the active one; an empty set goes.
func TestAnAccountSetIsRepairedNotLeftWithoutAnActiveMember(t *testing.T) {
	w := newWorld(t)
	db := w.auth.DB()
	exec(t, db, `INSERT INTO account_set (id, handle_hash, active_user_id, created_at, last_used_at) VALUES ('s1', 'h1', 'goes', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), ('s2', 'h2', 'goes', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	exec(t, db, `INSERT INTO account_set_member (set_id, user_id, refresh_session_id, added_at) VALUES ('s1', 'proved', 'rs-proved', CURRENT_TIMESTAMP), ('s1', 'goes', 'rs-goes', datetime('now', '+1 minute')), ('s2', 'goes', 'rs-goes', CURRENT_TIMESTAMP)`)
	plan, err := BuildPlan(context.Background(), db, w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), db, w.owner, w.services(), plan.Digest, nil); err != nil {
		t.Fatal(err)
	}
	var active string
	if err := db.QueryRow(`SELECT active_user_id FROM account_set WHERE id = 's1'`).Scan(&active); err != nil || active != "proved" {
		t.Fatalf("the shared set's active member = %q (%v), want the kept account", active, err)
	}
	if rowsFor(t, db, `SELECT COUNT(*) FROM account_set WHERE id = 's2'`) != 0 {
		t.Fatal("a set left with no member was kept")
	}
}

// vmctl being unreachable is not "owns nothing".
func TestWithoutTheComputerOwnersNothingIsPlannedOrApplied(t *testing.T) {
	w := newWorld(t)
	down := func(context.Context) (map[string]bool, error) { return nil, errors.New("vmctl down") }
	if _, err := BuildPlan(context.Background(), w.auth.DB(), down, w.services()); err == nil {
		t.Fatal("a plan was made without knowing who owns a computer")
	}
	if _, err := Apply(context.Background(), w.auth.DB(), down, w.services(), Digest([]string{"goes"}), nil); err == nil {
		t.Fatal("a run was applied without knowing who owns a computer")
	}
	if got := w.footprint("goes"); !equal(got, whole) {
		t.Fatalf("a refused run changed the account: %v", got)
	}
}

// A database that is not the one it is named for (a mistyped path makes an empty file) is not a service.
func TestAnEmptyDatabaseIsNotAService(t *testing.T) {
	w := newWorld(t)
	empty := openSQLite(t, filepath.Join(t.TempDir(), "typo.db"))
	svcs := w.services()
	svcs[1].DB = empty
	if _, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, svcs); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("err = %v, want ErrIncomplete", err)
	}
}

// More ids than one statement holds: the chunks are all deleted.
func TestManyAccountsAreAllDeleted(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 950; i++ {
		id := fmt.Sprintf("bulk-%d", i)
		exec(t, w.auth.DB(), `INSERT INTO users (id, email, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, id, id+"@example.com")
		exec(t, w.maild, `INSERT INTO email_owner_addresses (id, owner_id) VALUES (?, ?)`, "a-"+id, id)
	}
	plan, err := BuildPlan(context.Background(), w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Purge) != 951 {
		t.Fatalf("planned %d, want 951", len(plan.Purge))
	}
	if _, err := Apply(context.Background(), w.auth.DB(), w.owner, w.services(), plan.Digest, nil); err != nil {
		t.Fatal(err)
	}
	if n := rowsFor(t, w.maild, `SELECT COUNT(*) FROM email_owner_addresses WHERE owner_id LIKE 'bulk-%'`); n != 0 {
		t.Fatalf("%d bulk rows left", n)
	}
	if n := rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM users WHERE id LIKE 'bulk-%'`); n != 0 {
		t.Fatalf("%d bulk users left", n)
	}
}

// The webhook log of the account's messages goes with it (its payload names the addresses); another account's stays.
func TestTheWebhookEventsOfTheAccountsMessagesGoAndOthersStay(t *testing.T) {
	ctx := context.Background()
	w := newWorld(t)
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, nil); err != nil {
		t.Fatal(err)
	}
	if n := rowsFor(t, w.maild, `SELECT COUNT(*) FROM email_webhook_events WHERE id = 'w-goes'`); n != 0 {
		t.Fatalf("the purged account's webhook event stayed (%d)", n)
	}
	if n := rowsFor(t, w.maild, `SELECT COUNT(*) FROM email_webhook_events WHERE id = 'w-proved'`); n != 1 {
		t.Fatalf("a kept account's webhook event went (%d)", n)
	}
}

// Only directories that existed are counted: the dry run's number and the real run's are the same.
func TestTheMailDirectoryCountIsOnlyWhatExisted(t *testing.T) {
	ctx := context.Background()
	w := newWorld(t)
	if err := os.RemoveAll(filepath.Join(w.root, "attachments")); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	res, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.MailDirs != plan.Mailboxes || res.MailDirs != 1 {
		t.Fatalf("removed %d, planned %d, want 1 (only users/goes existed)", res.MailDirs, plan.Mailboxes)
	}
}

// A root given with a trailing slash or a dot is cleaned before the checks.
func TestAMailRootWithATrailingSlashIsCleaned(t *testing.T) {
	w := newWorld(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(w.root, link); err != nil {
		t.Fatal(err)
	}
	if err := checkMailRoot(link + "/"); err == nil {
		t.Fatal("a symlinked root with a trailing slash was accepted")
	}
	if err := checkMailRoot(w.root + "/./"); err != nil {
		t.Fatalf("a real root with a trailing slash: %v", err)
	}
}

type recordingJournal struct {
	stages []string
	failAt string
	seen   func(stage string)
}

func (j *recordingJournal) Entry(stage string, res *Result) error {
	if j.seen != nil {
		j.seen(stage)
	}
	j.stages = append(j.stages, stage)
	if stage == j.failAt {
		return errors.New("disk full")
	}
	return nil
}

// The journal is written BEFORE each irreversible step: when a step's entry cannot be written, that step has not run.
func TestTheJournalIsWrittenBeforeEachStepAndAFailureStopsThere(t *testing.T) {
	ctx := context.Background()
	for _, failAt := range []string{"intent", "before-commit:maild", "before-mail-dirs", "before-commit:auth"} {
		w := newWorld(t)
		plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
		if err != nil {
			t.Fatal(err)
		}
		j := &recordingJournal{failAt: failAt}
		res, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, j)
		if err == nil || !strings.Contains(err.Error(), "journal") {
			t.Fatalf("%s: err = %v, want a journal error", failAt, err)
		}
		committed := map[string]bool{}
		for _, s := range res.Services {
			committed[s.Name] = s.Committed
		}
		switch failAt {
		case "intent", "before-commit:maild":
			if len(committed) != 0 || !equal(w.footprint("goes"), whole) {
				t.Fatalf("%s: something was deleted before its entry was written (%v, %v)", failAt, committed, w.footprint("goes"))
			}
		case "before-mail-dirs":
			if f := w.footprint("goes"); f[7] != 2 {
				t.Fatalf("%s: the mail directories went before their entry (%v)", failAt, f)
			}
		case "before-commit:auth":
			if rowsFor(t, w.auth.DB(), `SELECT COUNT(*) FROM users WHERE id = 'goes'`) != 1 {
				t.Fatalf("%s: auth committed before its entry was written", failAt)
			}
		}
	}
}

// A whole run writes intent, a line before every commit, and done, in that order, and a stop writes why.
func TestTheJournalOfAWholeRunAndOfAStop(t *testing.T) {
	ctx := context.Background()
	w := newWorld(t)
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	j := &recordingJournal{}
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, j); err != nil {
		t.Fatal(err)
	}
	want := []string{"intent", "before-commit:maild", "after-commit:maild", "before-commit:platform", "after-commit:platform", "before-mail-dirs", "after-mail-dirs", "before-commit:auth", "after-commit:auth", "done"}
	if strings.Join(j.stages, "|") != strings.Join(want, "|") {
		t.Fatalf("stages = %v, want %v", j.stages, want)
	}
	w2 := newWorld(t)
	plan2, _ := BuildPlan(ctx, w2.auth.DB(), w2.owner, w2.services())
	j2 := &recordingJournal{}
	afterPlanHook = func() {
		exec(t, w2.auth.DB(), `UPDATE users SET email_verified_at = CURRENT_TIMESTAMP WHERE id = 'goes'`)
	}
	defer func() { afterPlanHook = nil }()
	if _, err := Apply(ctx, w2.auth.DB(), w2.owner, w2.services(), plan2.Digest, j2); err == nil {
		t.Fatal("a changed account was purged")
	}
	if len(j2.stages) != 0 {
		t.Fatalf("a run that stopped before any step wrote %v", j2.stages)
	}
}

func TestTheFileJournalIsValidatedBeforeAnythingAndRecordsEachServiceAndTheMailDirs(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenJournal(filepath.Join(dir, "no-such-dir", "r.jsonl"), ""); err == nil {
		t.Fatal("an unwritable receipt path was accepted")
	}
	if _, err := OpenJournal(filepath.Join(dir, "r.jsonl"), filepath.Join(dir, "no-key")); err == nil {
		t.Fatal("a missing key file was accepted")
	}
	key := filepath.Join(dir, "key")
	if err := os.WriteFile(key, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "r.jsonl")
	j, err := OpenJournal(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	ctx := context.Background()
	w := newWorld(t)
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, j); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	text := string(b)
	for _, want := range []string{`"stage":"intent"`, `"stage":"done"`, `"mail_dirs":2`, `"status":"committed"`, `"status":"pending"`, `"email_hmac_sha256"`, `"name":"maild"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("the receipt lacks %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, "goes@") || strings.Contains(text, "Goes@") {
		t.Fatal("the receipt holds an address")
	}
	if prev, err := LastIncomplete(path); err != nil || prev != "" {
		t.Fatalf("a finished run reads as incomplete: %q %v", prev, err)
	}
	// a run that died after its intent line
	dead := filepath.Join(dir, "dead.jsonl")
	if err := os.WriteFile(dead, []byte(`{"at":"t","stage":"before-commit:auth","digest":"d","services":[{"name":"maild","status":"committed"}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if prev, _ := LastIncomplete(dead); !strings.Contains(prev, "before-commit:auth") {
		t.Fatalf("an interrupted run was not reported: %q", prev)
	}
}

// A dry run cannot write: the database is opened read-only.
func TestADryRunOpensTheDatabasesReadOnly(t *testing.T) {
	w := newWorld(t)
	path := filepath.Join(t.TempDir(), "x.db")
	seed, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	exec(t, seed, `CREATE TABLE t (a int)`)
	_ = seed.Close()
	_ = w
	ro, err := OpenSQLiteFile(path, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err := ro.Exec(`INSERT INTO t VALUES (1)`); err == nil {
		t.Fatal("a dry run's database accepted a write")
	}
	rw, err := OpenSQLiteFile(path, true, true)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	if _, err := rw.Exec(`INSERT INTO t VALUES (1)`); err != nil {
		t.Fatalf("a real run's database refused a write: %v", err)
	}
	if _, err := OpenSQLiteFile(path+".missing", false, false); err == nil {
		t.Fatal("a missing file was opened")
	}
}

// The repair picks a proved member with a live session, as auth's repairSetTx picks a live one.
func TestTheSetRepairPrefersAProvedMemberWithALiveSession(t *testing.T) {
	ctx := context.Background()
	w := newWorld(t)
	db := w.auth.DB()
	now := time.Now().UTC()
	exec(t, db, `INSERT INTO account_set (id, handle_hash, active_user_id, created_at, last_used_at) VALUES ('s1', 'hh1', 'goes', ?, ?)`, now, now)
	exec(t, db, `INSERT INTO refresh_sessions (id, user_id, token_hash, created_at, expires_at) VALUES ('rs-dead', 'proved', 'hd', ?, ?)`, now, now.Add(-time.Hour))
	exec(t, db, `INSERT INTO account_set_member (set_id, user_id, refresh_session_id, added_at) VALUES ('s1', 'goes', 'rs-goes', ?), ('s1', 'computer', 'rs-computer', ?), ('s1', 'proved', 'rs-dead', ?)`, now, now.Add(time.Second), now.Add(-time.Second))
	plan, err := BuildPlan(ctx, db, w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, db, w.owner, w.services(), plan.Digest, nil); err != nil {
		t.Fatal(err)
	}
	var active string
	if err := db.QueryRow(`SELECT active_user_id FROM account_set WHERE id = 's1'`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != "proved" {
		t.Fatalf("active = %q: the proved member (dead session) comes before the newer unproved one", active)
	}
}

// The webhook log is matched on the provider AND the message id: the same id under another provider is another
// account's event and stays.
func TestAWebhookEventOfAnotherProviderWithTheSameMessageIdStays(t *testing.T) {
	ctx := context.Background()
	w := newWorld(t)
	exec(t, w.maild, `INSERT INTO email_webhook_events (id, provider, provider_message_id, raw_payload) VALUES ('w-other-provider', 'other', 'm-goes', '{"x":1}')`)
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, nil); err != nil {
		t.Fatal(err)
	}
	if n := rowsFor(t, w.maild, `SELECT COUNT(*) FROM email_webhook_events WHERE id = 'w-goes'`); n != 0 {
		t.Fatalf("the account's own event stayed (%d)", n)
	}
	if n := rowsFor(t, w.maild, `SELECT COUNT(*) FROM email_webhook_events WHERE id = 'w-other-provider'`); n != 1 {
		t.Fatalf("an event of another provider went (%d)", n)
	}
}

// A plan with nobody in it still opens and closes the receipt, so a run that died after its commit is closed by the rerun.
func TestAnEmptyPlanWritesIntentAndDone(t *testing.T) {
	ctx := context.Background()
	w := newWorld(t)
	exec(t, w.auth.DB(), `UPDATE users SET email_verified_at = CURRENT_TIMESTAMP WHERE id = 'goes'`)
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	j := &recordingJournal{}
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, j); err != nil {
		t.Fatal(err)
	}
	if strings.Join(j.stages, "|") != "intent|done" {
		t.Fatalf("stages = %v", j.stages)
	}
}

// The receipt names accounts the way the dry run prints them.
func TestTheJournalHashesIdsAsTheDryRunPrintsThem(t *testing.T) {
	ctx := context.Background()
	w := newWorld(t)
	path := filepath.Join(t.TempDir(), "r.jsonl")
	j, err := OpenJournal(path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	plan, err := BuildPlan(ctx, w.auth.DB(), w.owner, w.services())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, w.auth.DB(), w.owner, w.services(), plan.Digest, j); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"`+Hash("goes")+`"`) {
		t.Fatalf("the receipt does not carry %s (what the dry run prints):\n%s", Hash("goes"), b)
	}
}

func TestReadingAReceiptThatIsNewEmptyOrTornDoesNotStopTheRun(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh.jsonl")
	if prev, err := LastIncomplete(fresh); err != nil || prev != "" {
		t.Fatalf("missing: %q %v", prev, err)
	}
	j, err := OpenJournal(fresh, "") // creates the empty file, as the command does
	if err != nil {
		t.Fatal(err)
	}
	j.Close()
	if prev, err := LastIncomplete(fresh); err != nil || prev != "" {
		t.Fatalf("empty: %q %v", prev, err)
	}
	torn := filepath.Join(dir, "torn.jsonl")
	if err := os.WriteFile(torn, []byte(`{"at":"t","stage":"before-commit:auth","digest":"d"}`+"\n"+`{"at":"t","sta`), 0o600); err != nil {
		t.Fatal(err)
	}
	prev, err := LastIncomplete(torn)
	if err != nil || !strings.Contains(prev, "before-commit:auth") || !strings.Contains(prev, "cut short") {
		t.Fatalf("torn: %q %v", prev, err)
	}
	blank := filepath.Join(dir, "blank.jsonl")
	if err := os.WriteFile(blank, []byte("\n  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if prev, err := LastIncomplete(blank); err != nil || prev != "" {
		t.Fatalf("blank: %q %v", prev, err)
	}
}
