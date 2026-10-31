package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Selftest for the staging harness helpers (no network beyond httptest): the
// inbox reader picks the newest unseen sign-in mail from inbox or quarantine,
// extracts its code, ignores older and seen mail, and sends only the Bearer
// key; accessMethod reads the method claim from the jar.
func TestE2EInboxNewestCode(t *testing.T) {
	var sawAuth, sawCookie bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") == "Bearer k-1"
		sawCookie = sawCookie || r.Header.Get("Cookie") != ""
		switch {
		case r.URL.Path == "/api/email/messages" && r.URL.Query().Get("folder") == "inbox":
			json.NewEncoder(w).Encode(map[string]any{"messages": []map[string]string{
				{"id": "old", "subject": "Your Choir sign-in code", "created_at": "2026-10-31T01:00:00Z"},
				{"id": "seen", "subject": "Your Choir sign-in code", "created_at": "2026-10-31T03:00:00Z"},
			}})
		case r.URL.Path == "/api/email/messages":
			json.NewEncoder(w).Encode(map[string]any{"messages": []map[string]string{
				{"id": "new", "subject": "Your Choir sign-in code", "created_at": "2026-10-31T02:00:00Z"},
				{"id": "other", "subject": "hello", "created_at": "2026-10-31T09:00:00Z"},
			}})
		case r.URL.Path == "/api/email/messages/new":
			json.NewEncoder(w).Encode(map[string]string{"text_body": "Your code: 12345678\n\nOr open"})
		case r.URL.Path == "/api/email/messages/old":
			json.NewEncoder(w).Encode(map[string]string{"text_body": "Your code: 11111111"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := &e2eInbox{backend: srv.URL, key: "k-1", http: srv.Client()}
	code, err := m.newestCode("Your Choir sign-in code", map[string]bool{"seen": true})
	if err != nil || code != "12345678" {
		t.Fatalf("newestCode = %q, %v; want 12345678 (the newest unseen is the quarantine one)", code, err)
	}
	if !sawAuth || sawCookie {
		t.Fatalf("inbox reader must send only the Bearer key (auth=%v cookie=%v)", sawAuth, sawCookie)
	}
	if code, _ := m.newestCode("Your Choir sign-in code", map[string]bool{"seen": true, "new": true, "old": true}); code != "" {
		t.Fatalf("all seen: code = %q, want none", code)
	}
}

func TestE2EAccessMethodReadsTheJar(t *testing.T) {
	claims, _ := json.Marshal(map[string]string{"am": "email_code"})
	token := "h." + base64.RawURLEncoding.EncodeToString(claims) + ".s"
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "choir_access", Value: token, Path: "/"})
	}))
	defer backend.Close()
	s := mustDesktopSession(t, backend.URL)
	if _, b := s.get(t, "/x"); b == nil {
		t.Fatal("get failed")
	}
	if got := accessMethod(s, backend.URL); got != "email_code" {
		t.Fatalf("accessMethod = %q", got)
	}
	if strings.Contains(accessMethod(s, "http://other.invalid"), "email") {
		t.Fatal("read a cookie for another host")
	}
}
