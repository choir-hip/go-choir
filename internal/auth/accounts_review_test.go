package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Review follow-ups to the account switcher.

// 1: a renewal that passed the gate and then waited on the rotation lock must
// not write cookies after a switch committed meanwhile.
func TestAccountsRenewalRechecksGateUnderRotationLock(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	rs := refreshRowFor(t, h, []*http.Cookie{j[RefreshTokenCookieName]})
	user, _ := h.store.GetUserByID(b.ID)
	// A switch to A lands while the renewal is already past its first gate.
	if rec := switchTo(h, j.clone(), a.ID); rec.Code != http.StatusOK {
		t.Fatalf("switch = %d", rec.Code)
	}
	rec := httptest.NewRecorder()
	err := h.rotateRefreshSession(rec, j.req(http.MethodGet, "/auth/session", nil), rs, user)
	if err == nil || !isAccountSwitched(err) {
		t.Fatalf("rotate after switch = %v, want account switched", err)
	}
	if len(setCookies(rec)) != 0 {
		t.Fatalf("refused rotation wrote cookies %v", setCookies(rec))
	}
}

// 2: renewal keeps the set alive for the idle sweep.
func TestAccountsRotationTouchesSetLastUsed(t *testing.T) {
	h, j, _, _ := twoAccounts(t)
	set := mustSet(t, h, j)
	old := time.Now().UTC().Add(-20 * 24 * time.Hour)
	if _, err := h.store.DB().Exec("UPDATE account_set SET last_used_at = ?", old); err != nil {
		t.Fatal(err)
	}
	delete(j, AccessTokenCookieName)
	if sr := sessionOf(t, h, j); sr.User == nil {
		t.Fatal("renewal failed")
	}
	got, _ := h.store.GetAccountSetByHandleHash(set.HandleHash)
	if !got.LastUsedAt.After(old.Add(time.Hour)) {
		t.Fatalf("last_used_at not touched by rotation: %v", got.LastUsedAt)
	}
}

// 4: a presented (possibly planted) handle is never kept.
func TestAccountsSignInRotatesTheSetHandle(t *testing.T) {
	h, j, a, _ := twoAccounts(t)
	before := j[AccountsCookieName].Value
	signIn(t, h, j, a)
	after := j[AccountsCookieName].Value
	if after == before {
		t.Fatal("set handle was reused across a sign-in")
	}
	if _, err := h.store.GetAccountSetByHandleHash(accountsHandleHash(before)); err == nil {
		t.Fatal("old handle still names the set")
	}
	if ms, _ := h.store.ListAccountMembers(mustSet(t, h, j).ID); len(ms) != 2 {
		t.Fatalf("members = %d", len(ms))
	}
	// A planted handle naming someone else's set is not joined.
	attacker := jar{}
	signIn(t, h, attacker, mkUser(t, h, "uatk"))
	attackerHandle := attacker[AccountsCookieName].Value
	victim := jar{AccountsCookieName: attacker[AccountsCookieName]}
	signIn(t, h, victim, mkUser(t, h, "uvic"))
	if victim[AccountsCookieName].Value == attackerHandle {
		t.Fatal("victim kept the attacker's handle")
	}
}

// 5: the list never deletes a member that has a live session, and keeps the
// active pointer on a member.
func TestAccountsListDeleteIsConditionalAndKeepsActive(t *testing.T) {
	h, j, a, b := twoAccounts(t) // b active
	set := mustSet(t, h, j)
	mb, _ := h.store.GetAccountMember(set.ID, b.ID)
	_ = h.store.DeleteRefreshSessionByID(mb.RefreshSessionID) // active member dead
	// A is revived-live (it already is); the list must delete only B.
	got := listAccounts(t, h, j)
	if len(got) != 2 {
		t.Fatalf("list = %+v", got)
	}
	s2, err := h.store.GetAccountSetByHandleHash(set.HandleHash)
	if err != nil {
		t.Fatal(err)
	}
	if s2.ActiveUserID != a.ID {
		t.Fatalf("active after removing dead active member = %q", s2.ActiveUserID)
	}
	if _, err := h.store.GetAccountMember(set.ID, a.ID); err != nil {
		t.Fatal("live member removed")
	}
	if _, err := h.store.GetAccountMember(set.ID, b.ID); err == nil {
		t.Fatal("dead member kept")
	}
	// Revived between the read and the delete: the conditional delete spares it.
	h2, j2, a2, _ := twoAccounts(t)
	set2 := mustSet(t, h2, j2)
	listing, _ := h2.store.ListAccountSet(set2)
	if len(listing) != 2 {
		t.Fatal("setup")
	}
	if n := h2.store.deleteDeadMember(set2.ID, a2.ID); n != 0 {
		t.Fatalf("deleteDeadMember removed a live member (%d)", n)
	}
}

// 6: signing out the active member that hands over clears the dead pair.
func TestAccountsSignoutActiveWithNextClearsAuthCookies(t *testing.T) {
	h, j, _, b := twoAccounts(t)
	rec := call(h, h.HandleAccountSignout, j.clone(), http.MethodPost, "/auth/accounts/signout", map[string]string{"user_id": b.ID})
	cleared := map[string]bool{}
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			cleared[c.Name] = true
		}
	}
	if !cleared[AccessTokenCookieName] || !cleared[RefreshTokenCookieName] {
		t.Fatalf("active signout with next left the signed-out pair: %v", setCookies(rec))
	}
	if cleared[AccountsCookieName] {
		t.Fatal("set cookie cleared though members remain")
	}
}

// 7: an all-devices logout leaves no dangling active pointer or empty set.
func TestAccountsAllDevicesLogoutRepairsSets(t *testing.T) {
	h, j, a, _ := twoAccounts(t) // b active
	set := mustSet(t, h, j)
	rec := call(h, h.HandleLogout, j.clone(), http.MethodPost, "/auth/logout?all_devices=true", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout = %d", rec.Code)
	}
	s2, err := h.store.GetAccountSetByHandleHash(set.HandleHash)
	if err != nil {
		t.Fatal(err)
	}
	if s2.ActiveUserID != a.ID {
		t.Fatalf("active = %q, want remaining member", s2.ActiveUserID)
	}
	// Last member gone: the set goes too.
	h2 := acctEnv(t)
	sj := jar{}
	signIn(t, h2, sj, mkUser(t, h2, "usolo"))
	call(h2, h2.HandleLogout, sj.clone(), http.MethodPost, "/auth/logout?all_devices=true", nil)
	var n int
	_ = h2.store.DB().QueryRow("SELECT COUNT(*) FROM account_set").Scan(&n)
	if n != 0 {
		t.Fatalf("empty sets left = %d", n)
	}
}
