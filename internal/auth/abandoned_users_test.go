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
