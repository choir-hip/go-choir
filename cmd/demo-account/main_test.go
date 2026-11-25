package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
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
	configuredID(t, idFile)

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
	configuredID(t, idFile)
	line, _ := auth.FormatDemoCredential("reviewer", "a test password that is only a test")
	_ = os.WriteFile(credFile, []byte(line), 0o600)

	configured(t, credFile)
	var so, se bytes.Buffer
	if code := run([]string{"off", "--id-file", idFile, "--credential", credFile, "--auth-db", dbPath}, &so, &se, nil); code != 0 {
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
	if code := run([]string{"off", "--id-file", idFile, "--credential", credFile, "--auth-db", dbPath, "--already-off"}, &so, &se, nil); code != 0 {
		t.Fatalf("second run %d %s", code, se.String())
	}
}

func TestOffWithoutAnIdFileRefusesAndLeavesTheCredential(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "cred")
	_ = os.WriteFile(credFile, []byte("x\n"), 0o600)
	configured(t, credFile)
	var so, se bytes.Buffer
	configuredID(t, filepath.Join(dir, "missing"))
	if code := run([]string{"off", "--id-file", filepath.Join(dir, "missing"), "--credential", credFile}, &so, &se, nil); code == 0 {
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
	configuredID(t, idFile)
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
	configured(t, cred)
	missingDB := filepath.Join(dir, "typo", "auth.db")
	for name, args := range map[string][]string{
		"off, db typo":   {"off", "--id-file", good, "--credential", cred, "--auth-db", missingDB},
		"off, bad id":    {"off", "--id-file", bad, "--credential", cred, "--auth-db", dbPath},
		"off, swapped":   {"off", "--id-file", cred, "--credential", good, "--auth-db", dbPath},
		"reset, db typo": {"reset", "--id-file", good, "--auth-db", missingDB, "--vmctl", "http://127.0.0.1:1"},
		"reset, bad id":  {"reset", "--id-file", bad, "--auth-db", dbPath, "--vmctl", "http://127.0.0.1:1"},
	} {
		_ = os.WriteFile(cred, []byte("reviewer\nhash\n"), 0o600)
		for i, a := range args {
			if a == "--id-file" {
				configuredID(t, args[i+1])
			}
		}
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

// configuredID points the command's "the account auth is configured for" at a temp id file.
func configuredID(t *testing.T, path string) {
	t.Helper()
	old := idFilePath
	idFilePath = path
	t.Cleanup(func() { idFilePath = old })
}

// configured points the command's "the credential auth reads" at a temp path for one test.
func configured(t *testing.T, path string) {
	t.Helper()
	old := credentialPath
	credentialPath = path
	t.Cleanup(func() { credentialPath = old })
}

func seed(t *testing.T) (dir, dbPath, idFile string) {
	dir = t.TempDir()
	dbPath = filepath.Join(dir, "auth.db")
	store, _ := auth.OpenStore(dbPath)
	u, _ := store.CreateUser("demo-uid", "throwaway@example.invalid")
	_ = store.Close()
	idFile = filepath.Join(dir, "id")
	_ = os.WriteFile(idFile, []byte(u.ID+"\n"), 0o600)
	configuredID(t, idFile)
	return
}

// N0: off must remove the credential file auth reads, or fail. A path that does not exist
// is not "already off" unless the operator says so, and a file that is not a credential
// (the id file, the signing key) is never deleted.
func TestOffRemovesOnlyTheConfiguredCredentialAndFailsOtherwise(t *testing.T) {
	dir, dbPath, idFile := seed(t)
	var so, se bytes.Buffer
	cred := filepath.Join(dir, "demo", "demo-credential")
	_ = os.MkdirAll(filepath.Dir(cred), 0o700)
	configured(t, cred)
	run([]string{"credential", "--user", "reviewer", "--out", cred}, &so, &se, nil)
	// A stale credential at an old path validates but is not the configured one: off refuses
	// it, with or without --already-off, and the configured file is left alone.
	stale := filepath.Join(dir, "demo-credential")
	run([]string{"credential", "--user", "reviewer", "--out", stale}, &so, &se, nil)
	for _, extra := range [][]string{nil, {"--already-off"}} {
		args := append([]string{"off", "--id-file", idFile, "--credential", stale, "--auth-db", dbPath}, extra...)
		if code := run(args, &so, &se, nil); code == 0 {
			t.Fatalf("off accepted a path that is not the configured credential %v", extra)
		}
	}
	if _, err := os.Stat(cred); err != nil {
		t.Fatal("the configured credential was touched by a refused off")
	}
	// The configured one: removed and proved absent; the stale old-path file is removed too, and said.
	oldStale := filepath.Join(dir, "old-demo-credential")
	staleCredentialPath = oldStale
	t.Cleanup(func() { staleCredentialPath = "" })
	run([]string{"credential", "--user", "reviewer", "--out", oldStale}, &so, &se, nil)
	so.Reset()
	if code := run([]string{"off", "--id-file", idFile, "--credential", cred, "--auth-db", dbPath}, &so, &se, nil); code != 0 {
		t.Fatalf("off of the configured credential failed: %s", se.String())
	}
	if _, err := os.Stat(cred); err == nil {
		t.Fatal("the configured credential is still there")
	}
	if _, err := os.Stat(oldStale); err == nil || !strings.Contains(so.String(), "stale") {
		t.Fatalf("the stale old-path file must be removed and said: %q", so.String())
	}
	// A second run says so explicitly, and still proves the configured path absent.
	if code := run([]string{"off", "--id-file", idFile, "--credential", cred, "--auth-db", dbPath}, &so, &se, nil); code == 0 {
		t.Fatal("a second off without --already-off must fail")
	}
	if code := run([]string{"off", "--id-file", idFile, "--credential", cred, "--auth-db", dbPath, "--already-off"}, &so, &se, nil); code != 0 {
		t.Fatalf("--already-off on the configured path must succeed: %s", se.String())
	}
	// A file that is not a credential, at the configured path, is never deleted.
	_ = os.WriteFile(cred, []byte("not a credential\n"), 0o600)
	if code := run([]string{"off", "--id-file", idFile, "--credential", cred, "--auth-db", dbPath}, &so, &se, nil); code == 0 {
		t.Fatal("off removed a file that is not a credential")
	}
	if _, err := os.Stat(cred); err != nil {
		t.Fatal("a non-credential was deleted")
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

// D1: the path the command removes must be the one nix gives the auth service.
func TestTheCredentialPathIsTheOneNixConfiguresForAuth(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "nix", "node-b.nix"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	dir := regexp.MustCompile(`(?m)^\s*authSigningDir\s*=\s*"([^"]+)";`).FindStringSubmatch(src)
	set := regexp.MustCompile(`"AUTH_DEMO_CREDENTIAL_FILE=([^"]+)"`).FindStringSubmatch(src)
	if dir == nil || set == nil {
		t.Fatal("nix/node-b.nix no longer sets AUTH_DEMO_CREDENTIAL_FILE from authSigningDir; update this test with it")
	}
	got := strings.ReplaceAll(set[1], "${authSigningDir}", dir[1])
	if strings.Contains(got, "${") {
		t.Fatalf("the nix value %q uses a variable this test does not resolve", set[1])
	}
	if auth.DemoCredentialPath != got {
		t.Fatalf("demo-account off removes %s but nix configures auth to read %s", auth.DemoCredentialPath, got)
	}
}

// D2: the stale old-path file is checked before it is removed, and a failed unlink is not success.
func TestOffChecksTheStaleFileAndReportsAFailedUnlink(t *testing.T) {
	dir, dbPath, idFile := seed(t)
	var so, se bytes.Buffer
	cred := filepath.Join(dir, "demo", "demo-credential")
	_ = os.MkdirAll(filepath.Dir(cred), 0o700)
	configured(t, cred)
	// Not a credential at the stale path: kept, loud, non-zero (the real one is already off).
	stale := filepath.Join(dir, "stale-credential")
	staleCredentialPath = stale
	t.Cleanup(func() { staleCredentialPath = "" })
	run([]string{"credential", "--user", "reviewer", "--out", cred}, &so, &se, nil)
	_ = os.WriteFile(stale, []byte("something else\n"), 0o600)
	if code := run([]string{"off", "--id-file", idFile, "--credential", cred, "--auth-db", dbPath}, &so, &se, nil); code == 0 {
		t.Fatal("off exited 0 with a non-credential at the stale path")
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatal("a file that is not a credential was removed")
	}
	// A real stale credential that cannot be unlinked: non-zero.
	if os.Geteuid() == 0 {
		t.Skip("root can unlink anything")
	}
	locked := filepath.Join(dir, "locked")
	_ = os.MkdirAll(locked, 0o700)
	stale2 := filepath.Join(locked, "old")
	staleCredentialPath = stale2
	run([]string{"credential", "--user", "reviewer", "--out", cred}, &so, &se, nil)
	run([]string{"credential", "--user", "reviewer", "--out", stale2}, &so, &se, nil)
	_ = os.Chmod(locked, 0o500)
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	se.Reset()
	if code := run([]string{"off", "--id-file", idFile, "--credential", cred, "--auth-db", dbPath}, &so, &se, nil); code == 0 {
		t.Fatal("off exited 0 although the stale credential could not be removed")
	}
	if !strings.Contains(se.String(), "stale") {
		t.Fatalf("the failure must name the stale file: %q", se.String())
	}
}

// B: the id file is the one auth is configured for. Another account's id file is refused by
// off and reset, so neither can touch another account's keys or computer.
func TestOffAndResetRefuseAnotherAccountsIdFile(t *testing.T) {
	dir, dbPath, idFile := seed(t) // configured: the demo account's id file
	store, _ := auth.OpenStore(dbPath)
	o, _ := store.CreateUser("other-uid", "other@example.invalid")
	otherKey, _, _ := store.CreateAPIKey(context.Background(), o.ID, "c", []string{"read:runtime"}, nil)
	_ = store.Close()
	otherID := filepath.Join(dir, "other-id")
	_ = os.WriteFile(otherID, []byte(o.ID+"\n"), 0o600)
	cred := filepath.Join(dir, "cred")
	configured(t, cred)
	var so, se bytes.Buffer
	run([]string{"credential", "--user", "reviewer", "--out", cred}, &so, &se, nil)
	_ = idFile
	for name, args := range map[string][]string{
		"off":   {"off", "--id-file", otherID, "--credential", cred, "--auth-db", dbPath},
		"reset": {"reset", "--id-file", otherID, "--auth-db", dbPath, "--vmctl", "http://127.0.0.1:1"},
	} {
		if code := run(args, &so, &se, nil); code == 0 {
			t.Errorf("%s accepted another account's id file", name)
		}
	}
	if _, err := os.Stat(cred); err != nil {
		t.Error("the demo credential was removed by a refused off")
	}
	store, _ = auth.OpenStore(dbPath)
	defer func() { _ = store.Close() }()
	for _, k := range mustKeys(t, store, o.ID) {
		if k.ID == otherKey && k.RevokedAt != nil {
			t.Error("another account's key was revoked")
		}
	}
}
