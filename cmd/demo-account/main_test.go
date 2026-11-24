package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

func TestCredentialWritesAFileThatVerifiesAndPrintsThePasswordOnce(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cred")
	var so, se bytes.Buffer
	if run([]string{"credential", "--user", "reviewer", "--out", out}, &so, &se, nil) != 0 {
		t.Fatal(se.String())
	}
	pw := strings.TrimSpace(so.String())
	if len(pw) != 32 {
		t.Fatalf("password length %d", len(pw))
	}
	data, _ := os.ReadFile(out)
	if strings.Contains(string(data), pw) {
		t.Fatal("the file holds the password")
	}
	if info, _ := os.Stat(out); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}
	// A second run rotates: a different password, a different file.
	var so2 bytes.Buffer
	run([]string{"credential", "--user", "reviewer", "--out", out}, &so2, &se, nil)
	data2, _ := os.ReadFile(out)
	if so2.String() == so.String() || bytes.Equal(data, data2) {
		t.Fatal("rotation changed nothing")
	}
}

func TestResetRevokesKeysAndSessionsAndAsksVmctlToRemoveOnlyTheDemoComputer(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")
	store, err := auth.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := store.CreateUser("demo-uid", "throwaway@example.invalid")
	o, _ := store.CreateUser("other-uid", "other@example.invalid")
	ctx := context.Background()
	_, _, _ = store.CreateAPIKey(ctx, u.ID, "a", []string{"read:runtime"}, nil)
	_, _, _ = store.CreateAPIKey(ctx, u.ID, "b", []string{"read:runtime"}, nil)
	otherKey, _, _ := store.CreateAPIKey(ctx, o.ID, "c", []string{"read:runtime"}, nil)
	_ = store.Close()
	idFile := filepath.Join(dir, "id")
	_ = os.WriteFile(idFile, []byte(u.ID+"\n"), 0o600)

	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/remove" {
			t.Errorf("path %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"status":"removed"}`))
	}))
	defer srv.Close()
	var so, se bytes.Buffer
	if code := run([]string{"reset", "--id-file", idFile, "--auth-db", dbPath, "--vmctl", srv.URL}, &so, &se, srv.Client()); code != 0 {
		t.Fatalf("%d %s", code, se.String())
	}
	if got["user_id"] != u.ID {
		t.Fatalf("vmctl was asked about %v", got["user_id"])
	}
	store, _ = auth.OpenStore(dbPath)
	defer func() { _ = store.Close() }()
	for _, k := range mustKeys(t, store, u.ID) {
		if k.RevokedAt == nil {
			t.Fatal("a demo key survived")
		}
	}
	for _, k := range mustKeys(t, store, o.ID) {
		if k.ID == otherKey && k.RevokedAt != nil {
			t.Fatal("another account's key was revoked")
		}
	}
}

func mustKeys(t *testing.T, s *auth.Store, id string) []auth.APIKey {
	t.Helper()
	k, err := s.ListAPIKeys(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// One step turns the reviewer account off: the credential file goes (no new
// sign-in) and every key and session the account holds is revoked (no phone
// stays in). It touches the one account in the id file, never its computer,
// and never another account.
func TestOffRemovesTheCredentialAndRevokesEveryKeyOfTheDemoAccountOnly(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")
	store, err := auth.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := store.CreateUser("demo-uid", "throwaway@example.invalid")
	o, _ := store.CreateUser("other-uid", "other@example.invalid")
	ctx := context.Background()
	_, _, _ = store.CreateAPIKey(ctx, u.ID, "a", []string{"read:runtime"}, nil)
	otherKey, _, _ := store.CreateAPIKey(ctx, o.ID, "c", []string{"read:runtime"}, nil)
	_ = store.Close()
	idFile := filepath.Join(dir, "id")
	credFile := filepath.Join(dir, "cred")
	_ = os.WriteFile(idFile, []byte(u.ID+"\n"), 0o600)
	_ = os.WriteFile(credFile, []byte("x\n"), 0o600)

	var so, se bytes.Buffer
	if code := run([]string{"off", "--id-file", idFile, "--out", credFile, "--auth-db", dbPath}, &so, &se, nil); code != 0 {
		t.Fatalf("%d %s", code, se.String())
	}
	if _, err := os.Stat(credFile); !os.IsNotExist(err) {
		t.Fatal("the credential file is still there")
	}
	if _, err := os.Stat(idFile); err != nil {
		t.Fatal("the id file must stay: the account stays protected")
	}
	store, _ = auth.OpenStore(dbPath)
	defer func() { _ = store.Close() }()
	for _, k := range mustKeys(t, store, u.ID) {
		if k.RevokedAt == nil {
			t.Fatal("a demo key survived")
		}
	}
	for _, k := range mustKeys(t, store, o.ID) {
		if k.ID == otherKey && k.RevokedAt != nil {
			t.Fatal("another account's key was revoked")
		}
	}
	// Idempotent: a second run, with the file already gone, still succeeds.
	if code := run([]string{"off", "--id-file", idFile, "--out", credFile, "--auth-db", dbPath}, &so, &se, nil); code != 0 {
		t.Fatalf("second run %d %s", code, se.String())
	}
}

func TestOffWithoutAnIdFileRefusesAndLeavesTheCredential(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "cred")
	_ = os.WriteFile(credFile, []byte("x\n"), 0o600)
	var so, se bytes.Buffer
	if code := run([]string{"off", "--id-file", filepath.Join(dir, "missing"), "--out", credFile}, &so, &se, nil); code == 0 {
		t.Fatal("off ran without an account id")
	}
	if _, err := os.Stat(credFile); err != nil {
		t.Fatal("the credential was removed although the keys could not be")
	}
}
