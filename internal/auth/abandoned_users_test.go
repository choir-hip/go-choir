package auth

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// A stranger who begins a sign-up and never finishes the passkey leaves a
// users row (register/begin writes it, 10 a minute per address). Nothing
// removed it (docs/design/account-create-delete-2026-11-23.md section D; 172
// such rows on Node B, the oldest from May). Failure modes guarded, written
// before the code:
//  1. an abandoned row older than a day staying forever;
//  2. the sweep deleting a person who holds a passkey;
//  3. the sweep deleting a row younger than a day (a ceremony in progress);
//  4. the sweep deleting an account that holds a key or a session though it
//     has no passkey (the bootstrap admin is one);

func backdateUser(t *testing.T, h *Handler, id string, age time.Duration) {
	t.Helper()
	if _, err := h.store.DB().Exec("UPDATE users SET created_at = ? WHERE id = ?", time.Now().UTC().Add(-age), id); err != nil {
		t.Fatalf("backdate %s: %v", id, err)
	}
}

func userExists(h *Handler, id string) bool {
	_, err := h.store.GetUserByID(id)
	return err == nil
}

func TestSweepAbandonedUsersRemovesOnlyOldRowsWithNothingBehindThem(t *testing.T) {
	h, _ := testHandlerEnv(t)
	ctx := context.Background()

	if _, err := h.store.CreateUser("old-abandoned", "old@mosiah.org"); err != nil {
		t.Fatal(err)
	}
	backdateUser(t, h, "old-abandoned", 3*24*time.Hour)

	if _, err := h.store.CreateUser("fresh-abandoned", "fresh@mosiah.org"); err != nil {
		t.Fatal(err)
	}
	backdateUser(t, h, "fresh-abandoned", 2*time.Hour)

	addPasskey(t, h, "old-with-passkey", "kept@mosiah.org")
	backdateUser(t, h, "old-with-passkey", 90*24*time.Hour)

	if _, err := h.store.CreateUser("old-with-key", "key@mosiah.org"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.store.CreateAPIKey(ctx, "old-with-key", "k", []string{"owner:acts"}, nil); err != nil {
		t.Fatal(err)
	}
	backdateUser(t, h, "old-with-key", 90*24*time.Hour)

	if _, err := h.store.CreateUser(BootstrapAdminAPIKeyUserID, "bootstrap-admin@mosiah.org"); err != nil {
		t.Fatal(err)
	}
	backdateUser(t, h, BootstrapAdminAPIKeyUserID, 90*24*time.Hour)

	n, err := h.store.SweepAbandonedUsers(ctx, time.Now().UTC().Add(-AbandonedUserRetention))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("swept %d rows, want exactly the one old abandoned row", n)
	}
	if userExists(h, "old-abandoned") {
		t.Fatal("an abandoned row older than a day survived the sweep")
	}
	for _, id := range []string{"fresh-abandoned", "old-with-passkey", "old-with-key", BootstrapAdminAPIKeyUserID} {
		if !userExists(h, id) {
			t.Fatalf("the sweep deleted %s", id)
		}
	}
}

func TestSweepAbandonedUsersFreesTheEmailForANewAttempt(t *testing.T) {
	h, _ := testHandlerEnv(t)
	if rec := registerBegin(t, h, "again@mosiah.org"); rec.Code != http.StatusOK {
		t.Fatalf("first begin: %d %s", rec.Code, rec.Body.String())
	}
	u, err := h.store.GetUserByEmail("again@mosiah.org")
	if err != nil {
		t.Fatal(err)
	}
	backdateUser(t, h, u.ID, 2*24*time.Hour)
	if _, err := h.store.SweepAbandonedUsers(context.Background(), time.Now().UTC().Add(-AbandonedUserRetention)); err != nil {
		t.Fatal(err)
	}
	if rec := registerBegin(t, h, "again@mosiah.org"); rec.Code != http.StatusOK {
		t.Fatalf("a begin after the sweep: %d %s", rec.Code, rec.Body.String())
	}
}

// Failure modes 4 and 5, completed (review of 2026-11-24): a session or an
// account-set membership with no passkey keeps the row, and so does a
// ceremony still live (a returning person's old row, begun a minute ago).
func TestSweepKeepsRowsWithASessionAMembershipOrALiveChallenge(t *testing.T) {
	h, _ := testHandlerEnv(t)
	ctx := context.Background()
	db := h.store.DB()
	for _, id := range []string{"with-session", "with-member", "with-challenge", "with-expired-challenge"} {
		if _, err := h.store.CreateUser(id, id+"@mosiah.org"); err != nil {
			t.Fatal(err)
		}
		backdateUser(t, h, id, 90*24*time.Hour)
	}
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO refresh_sessions (id, user_id, token_hash, expires_at, created_at) VALUES ('s1', 'with-session', 'h', ?, ?)`, now.Add(time.Hour), now); err != nil {
		t.Fatalf("session fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO account_set (id, handle_hash, active_user_id, created_at, last_used_at) VALUES ('set1', 'hh', 'with-member', ?, ?)`, now, now); err != nil {
		t.Fatalf("account set fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO account_set_member (set_id, user_id, refresh_session_id, added_at) VALUES ('set1', 'with-member', 'rs', ?)`, now); err != nil {
		t.Fatalf("member fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO challenge_state (id, user_id, type, challenge, expires_at, created_at) VALUES ('c1', 'with-challenge', 'registration', 'x', ?, ?)`, now.Add(5*time.Minute), now); err != nil {
		t.Fatalf("challenge fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO challenge_state (id, user_id, type, challenge, expires_at, created_at) VALUES ('c2', 'with-expired-challenge', 'registration', 'x', ?, ?)`, now.Add(-time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("expired challenge fixture: %v", err)
	}
	if _, err := h.store.SweepAbandonedUsers(ctx, now.Add(-AbandonedUserRetention)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"with-session", "with-member", "with-challenge"} {
		if !userExists(h, id) {
			t.Fatalf("the sweep deleted %s", id)
		}
	}
	if userExists(h, "with-expired-challenge") {
		t.Fatal("a row whose only challenge expired should be swept")
	}
}
