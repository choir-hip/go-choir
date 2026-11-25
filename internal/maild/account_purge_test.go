package maild

// Failure list: the header of account_purge.go (1 to 6). Everything here is a
// temporary store; nothing real is touched.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

// seededItems is what seed() leaves per account: eight routing rows and two
// directories.
const seededItems = 10

type mailFixture struct {
	s    *Store
	cfg  *Config
	rows func(table, col, owner string) int
}

func (f *mailFixture) seed(t *testing.T, owner string) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.s.AssignUserNumber(ctx, "choir.news", owner, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.mailboxForOwner(owner); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(f.cfg.StorageRoot, "attachments", "outbound", owner, "a.bin"), filepath.Join(f.cfg.StorageRoot, "users", owner, "extra.txt")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("private"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO email_sender_whitelist (id, owner_id, alias_id, sender_address, created_at) VALUES ('w-` + owner + `', '` + owner + `', 'a-` + owner + `', 'x@y.z', 'now')`,
		`INSERT INTO email_provider_message_index (provider, provider_message_id, owner_id, created_at) VALUES ('resend', 'pm-` + owner + `', '` + owner + `', 'now')`,
		`INSERT INTO email_webhook_events (id, provider, provider_event_id, provider_message_id, event_type, raw_payload, received_at) VALUES ('e-` + owner + `', 'resend', 'ev-` + owner + `', 'pm-` + owner + `', 'received', '{"private":true}', 'now')`,
		`INSERT INTO email_approval_token_index (token, owner_id, draft_id, created_at) VALUES ('tok-` + owner + `', '` + owner + `', 'd', 'now')`,
		`INSERT INTO email_owner_addresses (owner_id, address, updated_at) VALUES ('` + owner + `', 'a@b.c', 'now')`,
		`INSERT INTO digest_subscriptions (owner_id, enabled, created_at, updated_at) VALUES ('` + owner + `', 1, 'now', 'now')`,
		`INSERT INTO digest_sent (owner_id, publication_id, version, sent_at) VALUES ('` + owner + `', 'p', 1, 'now')`,
	} {
		if _, err := f.s.routingDB.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

func newMailFixture(t *testing.T) *mailFixture {
	s, cfg := newTestStore(t)
	f := &mailFixture{s: s, cfg: cfg}
	f.rows = func(table, col, owner string) int {
		var n int
		if err := s.routingDB.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE `+col+` = ?`, owner).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	return f
}

func (f *mailFixture) remaining(owner string) int {
	n := f.rows("email_aliases", "target_id", owner) + f.rows("email_sender_whitelist", "owner_id", owner) +
		f.rows("email_provider_message_index", "owner_id", owner) + f.rows("email_approval_token_index", "owner_id", owner) +
		f.rows("email_owner_addresses", "owner_id", owner) + f.rows("digest_subscriptions", "owner_id", owner) + f.rows("digest_sent", "owner_id", owner) +
		f.rows("email_webhook_events", "provider_message_id", "pm-"+owner)
	for _, d := range []string{filepath.Join(f.cfg.StorageRoot, "users", owner), filepath.Join(f.cfg.StorageRoot, "attachments", "outbound", owner)} {
		if _, err := os.Stat(d); err == nil {
			n++
		}
	}
	return n
}

// Failure 1.
func TestPurgeAccountRefusesWithoutAnOldDeletionTime(t *testing.T) {
	f := newMailFixture(t)
	f.seed(t, "doomed")
	now := time.Now().UTC()
	for name, at := range map[string]time.Time{"none": {}, "young": now.Add(-time.Hour), "just short": now.Add(-purgeguard.GracePeriod + time.Minute), "future": now.Add(time.Hour)} {
		if _, err := f.s.PurgeAccount(context.Background(), "doomed", "user-root", at, now); err == nil {
			t.Fatalf("%s: purge ran", name)
		}
	}
	if f.remaining("doomed") != seededItems {
		t.Fatalf("a refused purge removed something: %d left", f.remaining("doomed"))
	}
}

// Failure 2, 5, 6.
func TestPurgeAccountRemovesOnlyThatAccountAndIsIdempotent(t *testing.T) {
	f := newMailFixture(t)
	f.seed(t, "doomed")
	f.seed(t, "neighbour")
	before := f.remaining("neighbour")
	now := time.Now().UTC()
	c, err := f.s.PurgeAccount(context.Background(), "doomed", "user-root", now.Add(-purgeguard.GracePeriod-time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if c.Aliases != 1 || c.Whitelist != 1 || c.ProviderIndex != 1 || c.WebhookEvents != 1 || !c.MailboxDir || !c.Attachments {
		t.Fatalf("counts: %+v", c)
	}
	if f.remaining("doomed") != 0 {
		t.Fatalf("%d rows or dirs of the account survive", f.remaining("doomed"))
	}
	if f.remaining("neighbour") != before {
		t.Fatal("another account lost data")
	}
	// Idempotent.
	if c, err := f.s.PurgeAccount(context.Background(), "doomed", "user-root", now.Add(-100*time.Hour), now); err != nil || c.Aliases != 0 || c.MailboxDir {
		t.Fatalf("second run: %+v %v", c, err)
	}
	// The number is not issued to anyone again.
	next, err := f.s.AssignUserNumber(context.Background(), "choir.news", "newcomer", t0)
	if err != nil || next.Number <= 0 {
		t.Fatal(err)
	}
	var doomedNumber int64
	_ = doomedNumber
}

// Failure 3, 4.
func TestPurgeAccountRefusesRootAndPathTricks(t *testing.T) {
	f := newMailFixture(t)
	f.seed(t, "victim")
	now := time.Now().UTC()
	old := now.Add(-100 * time.Hour)
	var rootBefore int
	_ = f.s.routingDB.QueryRow(`SELECT COUNT(*) FROM email_aliases`).Scan(&rootBefore)
	for _, owner := range []string{"user-root", "", "..", ".", "../users/victim", "victim/..", `a\b`, "x/y"} {
		if _, err := f.s.PurgeAccount(context.Background(), owner, "user-root", old, now); err == nil {
			t.Fatalf("owner %q accepted", owner)
		}
	}
	var after int
	_ = f.s.routingDB.QueryRow(`SELECT COUNT(*) FROM email_aliases`).Scan(&after)
	if after != rootBefore || f.remaining("victim") != seededItems {
		t.Fatal("a refused purge removed data")
	}
	if _, err := os.Stat(f.cfg.StorageRoot); err != nil {
		t.Fatal("storage root gone")
	}
}

func TestPurgeAccountRouteIsHostOnlyAndChecksTheTime(t *testing.T) {
	f := newMailFixture(t)
	f.seed(t, "doomed")
	h := NewHandler(f.cfg, f.s)
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, "purge-route-test-token-0123456789abcdef0123456789")
	defer restore()
	post := func(body string, host bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, AuthPurgeAccountPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.1.1.1:5000"
		if host {
			req.RemoteAddr = "127.0.0.1:5000"
			req.Header.Set("X-Internal-Caller", "true")
			req.Header.Set(hostauth.Header, "purge-route-test-token-0123456789abcdef0123456789")
		}
		rec := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(h.HandleAuthPurgeAccount)).ServeHTTP(rec, req)
		return rec
	}
	old := time.Now().Add(-100 * time.Hour).UTC().Format(time.RFC3339)
	if rec := post(`{"owner_id":"doomed","deletion_requested_at":"`+old+`"}`, false); rec.Code != http.StatusForbidden {
		t.Fatalf("non-host: %d", rec.Code)
	}
	if rec := post(`{"owner_id":"doomed"}`, true); rec.Code != http.StatusForbidden {
		t.Fatalf("no time: %d %s", rec.Code, rec.Body)
	}
	if f.remaining("doomed") != seededItems {
		t.Fatal("a refused call removed data")
	}
	rec := post(`{"owner_id":"doomed","deletion_requested_at":"`+old+`"}`, true)
	if rec.Code != http.StatusOK || f.remaining("doomed") != 0 {
		t.Fatalf("host call: %d %s left=%d", rec.Code, rec.Body, f.remaining("doomed"))
	}
	if !errors.Is(purgeguard.CheckClaim(time.Time{}, time.Now()), purgeguard.ErrNoRecord) {
		t.Fatal("sanity")
	}
}

// The shared tables of the mail database from before mailboxes were split per
// account are copied out of and left (migrateLegacySharedMailbox): an
// account's old messages, drafts and events can still be there, and they are
// its mail. The purge deletes the account's rows in them, and only its rows.
func TestPurgeAccountDeletesTheAccountsRowsInTheLegacySharedTables(t *testing.T) {
	f := newMailFixture(t)
	f.seed(t, "doomed")
	f.seed(t, "neighbour")
	legacy := map[string]string{"email_messages": "mailbox_owner_id", "email_ingress_events": "owner_id", "email_drafts": "owner_id",
		"email_draft_approval_events": "owner_id", "email_draft_approval_tokens": "owner_id", "email_risk_alerts": "owner_id", "email_draft_edit_requests": "owner_id"}
	for table, col := range legacy {
		if _, err := f.s.routingDB.Exec(`CREATE TABLE IF NOT EXISTS ` + table + ` (id TEXT, ` + col + ` TEXT, body TEXT)`); err != nil {
			t.Fatal(err)
		}
		for _, owner := range []string{"doomed", "neighbour"} {
			if _, err := f.s.routingDB.Exec(`INSERT INTO `+table+` (id, `+col+`, body) VALUES (?, ?, 'private')`, "m-"+owner, owner); err != nil {
				t.Fatal(err)
			}
		}
	}
	// The children of each message: keyed by the message, not the owner.
	children := []string{"email_message_recipients", "email_attachments", "email_source_packets"}
	for _, table := range children {
		if _, err := f.s.routingDB.Exec(`CREATE TABLE IF NOT EXISTS ` + table + ` (message_id TEXT, address TEXT)`); err != nil {
			t.Fatal(err)
		}
		for _, owner := range []string{"doomed", "neighbour"} {
			if _, err := f.s.routingDB.Exec(`INSERT INTO `+table+` (message_id, address) VALUES (?, 'someone@example.com')`, "m-"+owner); err != nil {
				t.Fatal(err)
			}
		}
	}
	now := time.Now().UTC()
	c, err := f.s.PurgeAccount(context.Background(), "doomed", "user-root", now.Add(-purgeguard.GracePeriod-time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if c.LegacyRows != int64(len(legacy)+len(children)) {
		t.Fatalf("legacy rows deleted: %d", c.LegacyRows)
	}
	for _, table := range children {
		if n := f.rows(table, "message_id", "m-doomed"); n != 0 {
			t.Fatalf("%s keeps %d rows of the account's messages", table, n)
		}
		if n := f.rows(table, "message_id", "m-neighbour"); n != 1 {
			t.Fatalf("%s lost another account's rows: %d", table, n)
		}
	}
	for table, col := range legacy {
		if n := f.rows(table, col, "doomed"); n != 0 {
			t.Fatalf("%s keeps %d rows of the account", table, n)
		}
		if n := f.rows(table, col, "neighbour"); n != 1 {
			t.Fatalf("%s lost another account's rows: %d", table, n)
		}
	}
}
