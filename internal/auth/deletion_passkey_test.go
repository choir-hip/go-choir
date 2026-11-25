package auth

// An address held by a pending deletion is held: nobody else registers it and
// no passkey is added to its account during the 72 hours, by the passkey
// sign-up routes (register/mail, register/begin, register/finish, the app's
// register/finish) or by the store.

import (
	"net/http"
	"testing"
	"time"
)

func pendingDeletionAccount(t *testing.T, h *Handler, email string) *User {
	t.Helper()
	if rec := createAccount(t, h, email, newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatalf("setup: %s", rec.Body.String())
	}
	user, err := h.store.GetUserByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.store.RequestAccountDeletion(user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return user
}

func TestAnAddressHeldByAPendingDeletionCannotBeRegisteredOrMailed(t *testing.T) {
	h := appEnv(t)
	rm := &fakeRecoveryMailer{}
	h.recoveryMailer = rm
	h.recoveryMailDispatch = func(f func()) { f() }
	user := pendingDeletionAccount(t, h, "held-pending@example.com")
	mailsBefore := len(testMailer(h).mails())

	fields, rec := proofFields(t, h, "held-pending@example.com")
	if rec.Code != http.StatusOK || fields == nil {
		t.Fatalf("register/mail = %d; it answers like any address", rec.Code)
	}
	if len(testMailer(h).mails()) != mailsBefore {
		t.Fatal("a registration code was mailed for a held address")
	}
	if len(rm.mails()) != 0 {
		t.Fatalf("a recovery link was mailed for an account whose deletion is pending (%d)", len(rm.mails()))
	}
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields); rec.Code == http.StatusOK {
		t.Fatal("register/begin answered 200 for an address held by a pending deletion")
	}
	creds, _ := h.store.GetCredentialsByUserID(user.ID)
	if len(creds) != 1 {
		t.Fatalf("credentials = %d, want 1", len(creds))
	}
}

func TestARecoveryFinishAddsNoPasskeyToAPendingDeletion(t *testing.T) {
	h := appEnv(t)
	user := pendingDeletionAccount(t, h, "recover-pending@example.com")
	begin := saveRecoveryChallenge(t, h, user, ChallengeSourceRecovery)
	body := rawJSON(newSoftPasskey(t).create(t, begin, testOrigin))
	if rec := postJSON(h.HandleRegisterFinish, body); rec.Code == http.StatusOK {
		t.Fatalf("web register/finish added a passkey to a pending deletion: %s", rec.Body.String())
	}
	begin = saveRecoveryChallenge(t, h, user, ChallengeSourceRecovery)
	body = rawJSON(newSoftPasskey(t).create(t, begin, testOrigin))
	rec := postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, body))
	if rec.Code == http.StatusOK {
		t.Fatalf("app register/finish minted a key for a pending deletion: %s", rec.Body.String())
	}
	if creds, _ := h.store.GetCredentialsByUserID(user.ID); len(creds) != 1 {
		t.Fatalf("credentials = %d, want 1", len(creds))
	}
}

func TestTheStoreAddsNoRecoveryPasskeyAndMintsNoScopedKeyForAPendingDeletion(t *testing.T) {
	h := appEnv(t)
	user := pendingDeletionAccount(t, h, "store-pending@example.com")
	c := &Credential{ID: "late-credential", UserID: user.ID, PublicKey: []byte("k"), AAGUID: []byte{}, Transport: "[]", CreatedAt: time.Now().UTC()}
	if err := h.store.AddRecoveryCredential(c); err != ErrDeletionPending {
		t.Fatalf("AddRecoveryCredential = %v, want ErrDeletionPending", err)
	}
	if creds, _ := h.store.GetCredentialsByUserID(user.ID); len(creds) != 1 {
		t.Fatalf("credentials = %d, want the one that was there", len(creds))
	}
	if _, _, err := h.store.CreateComputerScopedAPIKey(t.Context(), user.ID, "late", []string{"read"}, "", nil); err != ErrDeletionPending {
		t.Fatalf("CreateComputerScopedAPIKey = %v, want ErrDeletionPending", err)
	}
}
