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
	"github.com/yusefmosiah/go-choir/internal/hostauth"
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
	line, _ := auth.FormatDemoCredential("reviewer", "a test password that is only a test")
	_ = os.WriteFile(credFile, []byte(line), 0o600)

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
	// Idempotent when the operator says it is already off.
	if code := run([]string{"off", "--id-file", idFile, "--out", credFile, "--auth-db", dbPath, "--already-off"}, &so, &se, nil); code != 0 {
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

// Node B's vmctl enforces the host credential, and a root shell has none loaded: reset
// reads the token file itself and sends it (review, 2026-11-24).
func TestResetSendsTheInternalTokenFromItsFileToVmctl(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")
	store, _ := auth.OpenStore(dbPath)
	u, _ := store.CreateUser("demo-uid", "throwaway@example.invalid")
	_ = store.Close()
	idFile := filepath.Join(dir, "id")
	_ = os.WriteFile(idFile, []byte(u.ID+"\n"), 0o600)
	token := strings.Repeat("t0ken-", 12)
	tokenFile := filepath.Join(dir, "token")
	_ = os.WriteFile(tokenFile, []byte(token+"\n"), 0o600)
	var sent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Get(hostauth.Header)
		if sent != token {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"status":"removed"}`))
	}))
	defer srv.Close()
	tokenPorts[portOf(srv.URL)] = true
	var so, se bytes.Buffer
	if code := run([]string{"reset", "--id-file", idFile, "--auth-db", dbPath, "--vmctl", srv.URL, "--token-file", tokenFile}, &so, &se, srv.Client()); code != 0 {
		t.Fatalf("%d %s", code, se.String())
	}
	if sent != token {
		t.Fatal("the token was not sent")
	}
	if strings.Contains(so.String()+se.String(), token) {
		t.Fatal("the token is in the output")
	}
}

// off must fail loudly unless it proved the account's access is gone: a wrong database
// path or a wrong id removes nothing and exits non-zero.
func TestOffAndResetRefuseAWrongDatabaseOrAccountAndLeaveTheCredential(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")
	store, _ := auth.OpenStore(dbPath)
	u, _ := store.CreateUser("demo-uid", "throwaway@example.invalid")
	_ = store.Close()
	good := filepath.Join(dir, "id")
	_ = os.WriteFile(good, []byte(u.ID+"\n"), 0o600)
	bad := filepath.Join(dir, "badid")
	_ = os.WriteFile(bad, []byte("no-such-account\n"), 0o600)
	cred := filepath.Join(dir, "cred")
	missingDB := filepath.Join(dir, "typo", "auth.db")
	for name, args := range map[string][]string{
		"off, db typo":   {"off", "--id-file", good, "--out", cred, "--auth-db", missingDB},
		"off, bad id":    {"off", "--id-file", bad, "--out", cred, "--auth-db", dbPath},
		"off, swapped":   {"off", "--id-file", cred, "--out", good, "--auth-db", dbPath},
		"reset, db typo": {"reset", "--id-file", good, "--auth-db", missingDB, "--vmctl", "http://127.0.0.1:1"},
		"reset, bad id":  {"reset", "--id-file", bad, "--auth-db", dbPath, "--vmctl", "http://127.0.0.1:1"},
	} {
		_ = os.WriteFile(cred, []byte("reviewer\nhash\n"), 0o600)
		var so, se bytes.Buffer
		if code := run(args, &so, &se, nil); code == 0 {
			t.Errorf("%s: exited 0 (%s)", name, so.String())
		}
		if _, err := os.Stat(cred); err != nil && name != "off, swapped" {
			t.Errorf("%s: the credential was removed", name)
		}
		if _, err := os.Stat(filepath.Dir(missingDB)); err == nil {
			t.Errorf("%s: a database was created at the typo path", name)
		}
	}
}

func portOf(u string) string { return u[strings.LastIndex(u, ":")+1:] }

func init() { offSettle = 0 }

func seed(t *testing.T) (dir, dbPath, idFile string) {
	dir = t.TempDir()
	dbPath = filepath.Join(dir, "auth.db")
	store, _ := auth.OpenStore(dbPath)
	u, _ := store.CreateUser("demo-uid", "throwaway@example.invalid")
	_ = store.Close()
	idFile = filepath.Join(dir, "id")
	_ = os.WriteFile(idFile, []byte(u.ID+"\n"), 0o600)
	return
}

// N0: off must remove the credential file auth reads, or fail. A path that does not exist
// is not "already off" unless the operator says so, and a file that is not a credential
// (the id file, the signing key) is never deleted.
func TestOffRemovesOnlyARealCredentialAndFailsOtherwise(t *testing.T) {
	dir, dbPath, idFile := seed(t)
	var so, se bytes.Buffer
	missing := filepath.Join(dir, "old-path", "demo-credential")
	if code := run([]string{"off", "--id-file", idFile, "--out", missing, "--auth-db", dbPath}, &so, &se, nil); code == 0 {
		t.Fatal("off exited 0 for a credential path that does not exist")
	}
	if code := run([]string{"off", "--id-file", idFile, "--out", idFile, "--auth-db", dbPath}, &so, &se, nil); code == 0 {
		t.Fatal("off exited 0 for a file that is not a credential")
	}
	if _, err := os.Stat(idFile); err != nil {
		t.Fatal("off deleted a file that is not a credential")
	}
	if code := run([]string{"off", "--id-file", idFile, "--out", missing, "--auth-db", dbPath, "--already-off"}, &so, &se, nil); code != 0 {
		t.Fatalf("an explicit --already-off must succeed: %s", se.String())
	}
	cred := filepath.Join(dir, "cred")
	run([]string{"credential", "--user", "reviewer", "--out", cred}, &so, &se, nil)
	if code := run([]string{"off", "--id-file", idFile, "--out", cred, "--auth-db", dbPath}, &so, &se, nil); code != 0 {
		t.Fatalf("off of a real credential failed: %s", se.String())
	}
	if _, err := os.Stat(cred); err == nil {
		t.Fatal("the credential is still there")
	}
}

// N3: the token goes only to the vmctl port, and a redirect never carries it elsewhere.
func TestResetNeverSendsTheTokenToAnotherPortOrThroughARedirect(t *testing.T) {
	dir, dbPath, idFile := seed(t)
	token := strings.Repeat("t0ken-", 12)
	tokenFile := filepath.Join(dir, "token")
	_ = os.WriteFile(tokenFile, []byte(token+"\n"), 0o600)
	var elsewhere string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhere = r.Header.Get(hostauth.Header)
		_, _ = w.Write([]byte(`{"status":"removed"}`))
	}))
	defer other.Close()
	// 1: a port that is not vmctl's gets no token.
	var so, se bytes.Buffer
	run([]string{"reset", "--id-file", idFile, "--auth-db", dbPath, "--vmctl", other.URL, "--token-file", tokenFile}, &so, &se, other.Client())
	if elsewhere != "" {
		t.Fatal("the token was sent to a port that is not vmctl's")
	}
	// 2: a redirect from vmctl's port is not followed.
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusTemporaryRedirect)
	}))
	defer redirecting.Close()
	tokenPorts[portOf(redirecting.URL)] = true
	if code := run([]string{"reset", "--id-file", idFile, "--auth-db", dbPath, "--vmctl", redirecting.URL, "--token-file", tokenFile}, &so, &se, redirecting.Client()); code == 0 {
		t.Fatal("a redirected reset exited 0")
	}
	if elsewhere != "" {
		t.Fatal("the token followed a redirect")
	}
	// 3: an unreadable token file for vmctl's port fails before anything is sent.
	if code := run([]string{"reset", "--id-file", idFile, "--auth-db", dbPath, "--vmctl", redirecting.URL, "--token-file", filepath.Join(dir, "nope")}, &so, &se, redirecting.Client()); code == 0 || !strings.Contains(se.String(), "token") {
		t.Fatalf("an unreadable token file must be named: %q", se.String())
	}
}
