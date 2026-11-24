package auth

import (
	"net/http"
	"testing"
)

// The invariant (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md):
// no account is ever bound to an address that its holder has not proved, in the
// same ceremony; an account whose address is not proved mints nothing.

// A registration that names an address and carries no proof of the inbox is
// refused, and nothing exists for the address afterwards.
func TestRegisterBeginWithoutProofIsRefusedAndLeavesNothing(t *testing.T) {
	h := appEnv(t)
	rec := postJSON(h.HandleRegisterBegin, map[string]string{"email": "stranger@example.com"})
	if rec.Code == http.StatusOK {
		t.Fatalf("register/begin answered 200 with no proof: %s", rec.Body.String())
	}
	if _, err := h.store.GetUserByEmail("stranger@example.com"); err == nil {
		t.Fatal("a user row exists for an address nobody proved")
	}
}

// An account whose address was never proved (a row from before this rule)
// cannot sign in on the app route or mint a key.
func TestAccountWithAnUnprovedAddressCannotSignInOrMintAKey(t *testing.T) {
	h := appEnv(t)
	p := newSoftPasskey(t)
	if rec := createAccount(t, h, "legacy@example.com", p, testOrigin); rec.Code != 200 {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := h.store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE email = ?`, "legacy@example.com"); err != nil {
		t.Fatal(err)
	}
	user, _ := h.store.GetUserByEmail("legacy@example.com")
	before := countKeys(t, h, user.ID)
	begin := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, p.assert(t, begin, testOrigin)))
	if rec.Code == http.StatusOK {
		t.Fatalf("an unproved account signed in on the app route: %s", rec.Body.String())
	}
	if countKeys(t, h, user.ID) != before {
		t.Fatal("a key was minted for an unproved account")
	}
}
