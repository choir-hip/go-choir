package auth

import (
	"errors"
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// The operator mark decides who is not counted against the cap, who is not
// given the small shape and who may run self-development, so the migration
// that adds it is guarded (review of 2026-11-24, finding 7):
//  1. the backfill marking an address that is not on the legacy domain set;
//  2. the backfill running again on a later open, marking a row made since;
//  3. a row created after the migration being marked by its address.
func TestMigrateOperatorMarksLegacyDomainsOnceAndNeverAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, created_at DATETIME NOT NULL)`,
		`CREATE TABLE credentials (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, public_key BLOB NOT NULL DEFAULT x'', attestation_type TEXT NOT NULL DEFAULT 'none', transport TEXT NOT NULL DEFAULT '[]', sign_count INTEGER NOT NULL DEFAULT 0, aaguid BLOB NOT NULL DEFAULT x'', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO users VALUES ('a', 'ci1@example.com', '2026-11-01 00:00:00+00:00'), ('b', 'ci2@Example.TEST', '2026-11-01 00:00:00+00:00'), ('c', 'person@mosiah.org', '2026-11-01 00:00:00+00:00'), ('d', 'x@notexample.com', '2026-11-01 00:00:00+00:00'), ('e', 'unfinished@example.com', '2026-11-01 00:00:00+00:00')`,
		`INSERT INTO credentials (id, user_id) VALUES ('ca', 'a'), ('cb', 'b'), ('cc', 'c')`,
	} {
		if _, err := legacy.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	_ = legacy.Close()

	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	// e is an unfinished sign-up on a legacy domain: a stranger's until it finishes.
	want := map[string]bool{"a": true, "b": true, "c": false, "d": false, "e": false}
	for id, marked := range want {
		if got, err := store.IsOperatorAccount(id); err != nil || got != marked {
			t.Fatalf("after the migration %s operator = %v, %v; want %v", id, got, err, marked)
		}
	}
	// A row made after the migration is not marked by its address.
	if _, err := store.CreateUser("late", "late@example.com"); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.IsOperatorAccount("late"); got {
		t.Fatal("a row created after the migration was marked by its address")
	}
	_ = store.Close()

	// Opening again does not run the backfill again.
	again, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got, _ := again.IsOperatorAccount("late"); got {
		t.Fatal("a later open marked a later row")
	}
	if got, _ := again.IsOperatorAccount("c"); got {
		t.Fatal("a later open marked a person")
	}
}

// Review finding 1: an address typed in any spelling of its domain finds the
// account at every lookup (register, sign-in, email sign-in, recovery), not
// only the spelling it was stored under.
func TestEverySpellingOfAnAddressFindsTheAccount(t *testing.T) {
	h, _ := testHandlerEnv(t)
	if rec := registerBegin(t, h, "Ada@Example.com"); rec.Code != http.StatusOK {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	u, err := h.store.GetUserByEmail("Ada@example.com")
	if err != nil {
		t.Fatalf("stored spelling not found: %v", err)
	}
	if err := h.store.CreateCredential(&Credential{
		ID: "cred-ada", UserID: u.ID, PublicKey: []byte("k"), AttestationType: "none",
		Transport: `["internal"]`, AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{"Ada@Example.com", "Ada@example.com", "Ada@EXAMPLE.COM", " Ada@Example.com ", "Ada@example.COM"} {
		got, err := h.store.GetUserByEmail(spelling)
		if err != nil || got.ID != u.ID {
			t.Fatalf("%q: %v", spelling, err)
		}
		req := httptest.NewRequest(http.MethodPost, "/auth/login/begin", bytes.NewBufferString(`{"email":"`+spelling+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.HandleLoginBegin(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("login begin %q: user not found", spelling)
		}
		if rec := registerBegin(t, h, spelling); rec.Code != http.StatusConflict {
			t.Fatalf("register again %q: %d, want 409 (already registered)", spelling, rec.Code)
		}
	}
	// A row stored by an older build under a mixed-case domain is found too.
	if _, err := h.store.DB().Exec(`INSERT INTO users (id, email, created_at) VALUES ('old', 'Bob@Example.COM', ?)`, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{"Bob@Example.COM", "Bob@example.com", " Bob@EXAMPLE.com"} {
		if got, err := h.store.GetUserByEmail(spelling); err != nil || got.ID != "old" {
			t.Fatalf("old row %q: %v", spelling, err)
		}
	}
	// The local part is case-sensitive, as sign-in has always matched it.
	if _, err := h.store.GetUserByEmail("bob@example.com"); err == nil {
		t.Fatal("the local part must keep its case")
	}
}

// Review finding 8: a sign-up that resumes refreshes its row, so the sweep
// does not take it mid-ceremony; and a finish whose row is gone fails cleanly.
func TestResumedSignUpIsNotSweptAndAGoneRowFailsCleanly(t *testing.T) {
	h, _ := testHandlerEnv(t)
	if _, err := h.store.CreateUser("resume", "resume@mosiah.org"); err != nil {
		t.Fatal(err)
	}
	backdateUser(t, h, "resume", 5*24*time.Hour)
	if rec := registerBegin(t, h, "resume@mosiah.org"); rec.Code != http.StatusOK {
		t.Fatalf("resume: %d", rec.Code)
	}
	n, err := h.store.SweepAbandonedUsers(t.Context(), time.Now().UTC().Add(-AbandonedUserRetention))
	if err != nil || n != 0 || !userExists(h, "resume") {
		t.Fatalf("swept a row whose ceremony just began: n=%d err=%v", n, err)
	}
	err = h.store.CreateCredentialWithinCap(&Credential{
		ID: "c-gone", UserID: "not-a-user", PublicKey: []byte("k"), AttestationType: "none",
		Transport: `["internal"]`, AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC(),
	}, 5)
	if err == nil || !errors.Is(err, ErrUserGone) {
		t.Fatalf("a finish for a deleted row = %v, want ErrUserGone", err)
	}
}

// Review finding 2: an unfinished sign-up that an operator begun (marked) and
// never finished is not an exemption for the next person to resume it.
func TestResumingAnUnfinishedOperatorRowWithoutTheSecretLosesTheMark(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 1
	h.config.OperatorSecret = testOperatorSecret
	addPasskey(t, h, "p1", "one@mosiah.org") // the cap is full
	if rec := registerBeginAs(t, h, "ops@example.com", testOperatorSecret); rec.Code != http.StatusOK {
		t.Fatalf("operator begin: %d", rec.Code)
	}
	u, _ := h.store.GetUserByEmail("ops@example.com")
	if marked, _ := h.store.IsOperatorAccount(u.ID); !marked {
		t.Fatal("the operator's begin did not mark the row")
	}
	// A stranger types the same address, with no secret, at a full cap.
	if rec := registerBegin(t, h, "ops@example.com"); rec.Code != http.StatusForbidden {
		t.Fatalf("a stranger resuming the operator's unfinished row: %d, want 403", rec.Code)
	}
	if marked, _ := h.store.IsOperatorAccount(u.ID); marked {
		t.Fatal("the unfinished row kept its operator mark after a stranger's begin")
	}
}
