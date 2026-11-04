package maild

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The Library shows a publisher by numbered address. Failure modes:
//
//	P1  the address is not the canonical zero-padded form (owner 000, user 7
//	    as 007), or names another domain than the primary one;
//	P2  an account with no number (or a disabled alias) gets an entry, or
//	    an unknown id breaks the call;
//	P3  a user request, an unmarked caller, or too many ids reaches the
//	    lookup or answers anything but a refusal.
func TestHandlePublicAddresses(t *testing.T) {
	s, cfg := newTestStore(t)
	ctx := context.Background()
	h := NewHandler(cfg, s)
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		if _, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, id, t0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.routingDB.ExecContext(ctx, `UPDATE email_aliases SET disabled_at = ? WHERE target_id = 'b'`, "2026-11-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, PublicAddressesPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Caller", "true")
		w := httptest.NewRecorder()
		h.HandlePublicAddresses(w, req)
		return w
	}
	w := post(`{"owner_ids":["` + cfg.RootOwnerID + `","a","g","b","nobody"]}`)
	var out struct {
		Addresses map[string]string `json:"addresses"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	domain := strings.ToLower(cfg.PrimaryDomain)
	want := map[string]string{cfg.RootOwnerID: "000@" + domain, "a": "001@" + domain, "g": "007@" + domain}
	if len(out.Addresses) != len(want) {
		t.Fatalf("addresses = %v, want %v (a disabled alias and an unknown id get none)", out.Addresses, want)
	}
	for id, addr := range want {
		if out.Addresses[id] != addr {
			t.Fatalf("%s = %q, want %q", id, out.Addresses[id], addr)
		}
	}
	if strings.Contains(w.Body.String(), "@example") && !strings.Contains(domain, "example") {
		t.Fatalf("an email leaked: %s", w.Body.String())
	}

	// P3.
	req := httptest.NewRequest(http.MethodPost, PublicAddressesPath, strings.NewReader(`{"owner_ids":["a"]}`))
	req.Header.Set("X-Internal-Caller", "true")
	req.Header.Set("X-Authenticated-User", "someone")
	rec := httptest.NewRecorder()
	h.HandlePublicAddresses(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a user request = %d, want 403", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, PublicAddressesPath, strings.NewReader(`{"owner_ids":["a"]}`))
	rec = httptest.NewRecorder()
	h.HandlePublicAddresses(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("an unmarked caller = %d, want 403", rec.Code)
	}
	many := make([]string, 101)
	for i := range many {
		many[i] = `"x"`
	}
	if w := post(`{"owner_ids":[` + strings.Join(many, ",") + `]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("101 ids = %d, want 400", w.Code)
	}
	req = httptest.NewRequest(http.MethodGet, PublicAddressesPath, nil)
	rec = httptest.NewRecorder()
	h.HandlePublicAddresses(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d", rec.Code)
	}
}

// Every account has a numbered address, test accounts included (owner,
// 2026-11-04). A test or ephemeral-domain account is numbered from the test
// range at assignment, never from the user sequence; an account that already
// has a number keeps it (the backfill never renumbers); and the lookup
// renders a test number as its plain digits.
func TestTestAccountsGetTestRangeNumbersAndKeepThem(t *testing.T) {
	s, cfg := newTestStore(t)
	ctx := context.Background()
	h := NewHandler(cfg, s)
	user, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "real", t0)
	if err != nil || user.Number != 1 {
		t.Fatalf("real user = %+v %v", user, err)
	}
	a, err := s.AssignTestUserNumber(ctx, cfg.PrimaryDomain, "tester", t0)
	if err != nil || !a.Created || a.Number < TestNumberStart || a.Number >= TestNumberEnd {
		t.Fatalf("test account = %+v %v, want a number in [%d,%d)", a, err, TestNumberStart, TestNumberEnd)
	}
	again, err := s.AssignTestUserNumber(ctx, cfg.PrimaryDomain, "tester", t0)
	if err != nil || again.Created || again.Number != a.Number {
		t.Fatalf("a second assignment = %+v %v, want the same number kept", again, err)
	}
	// An account that already holds a user number is not renumbered into the
	// test range, and a real one is not moved by the test path.
	kept, err := s.AssignTestUserNumber(ctx, cfg.PrimaryDomain, "real", t0)
	if err != nil || kept.Created || kept.Number != 1 {
		t.Fatalf("an existing number = %+v %v, want 1 kept", kept, err)
	}
	next, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "real-two", t0)
	if err != nil || next.Number != 2 {
		t.Fatalf("the user sequence moved: %+v %v", next, err)
	}

	// Through the route.
	req := assignRequest(`{"test_owner_ids":["tester-two"]}`)
	w := httptest.NewRecorder()
	h.HandleAuthAssignNumbers(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"assigned":1`) {
		t.Fatalf("route = %d %s", w.Code, w.Body.String())
	}
	post := httptest.NewRequest(http.MethodPost, PublicAddressesPath, strings.NewReader(`{"owner_ids":["tester","real"]}`))
	post.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandlePublicAddresses(rec, post)
	var out struct {
		Addresses map[string]string `json:"addresses"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("lookup = %d %s", rec.Code, rec.Body.String())
	}
	domain := strings.ToLower(cfg.PrimaryDomain)
	if out.Addresses["tester"] != fmt.Sprintf("%d@%s", a.Number, domain) || out.Addresses["real"] != "001@"+domain {
		t.Fatalf("addresses = %v", out.Addresses)
	}
}
