package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Written before the code (CLAUDE.md, Testing). How credential resolution and
// `choir login` (docs/design/cli-agent-onboarding-2026-11-03.md) could fail:
//
//  R1 the order is wrong (file flag, $CHOIR_API_KEY, $CHOIR_API_KEY_FILE,
//     credentials file).
//  R2 a credentials file with a loose mode is read.
//  R3 a key from the credentials file is sent to another host.
//  R4 the credentials file's host is not used when no host is given.
//  R5 choir mcp resolves differently from the CLI.
//  L1 login writes nothing, or writes with a loose mode.
//  L2 login does not wait through pending and slow_down.
//  L3 denied or expired still writes a file, or exits 0.
//  L4 re-login leaves the old key live.
//  L5 logout leaves the key live or the file behind.
//  M1 mcp install writes a config that does not run this binary's mcp, or
//     clobbers the host's other servers.

func isolateCredentials(t *testing.T) string {
	t.Helper()
	step := loginPollStep
	loginPollStep = 0
	t.Cleanup(func() { loginPollStep = step })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(apiKeyEnvVar, "")
	t.Setenv("CHOIR_API_KEY_FILE", "")
	t.Setenv(hostEnvVar, "")
	return filepath.Join(dir, "choir", "credentials")
}

func writeCreds(t *testing.T, path string, c storedCredentials, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(c)
	if err := os.WriteFile(path, raw, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func writeKeyFile(t *testing.T, key string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(p, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func resolveClient(t *testing.T, args ...string) (*client, error) {
	t.Helper()
	return newClient(flag.NewFlagSet("t", flag.ContinueOnError), args, io.Discard, io.Discard)
}

// R1, R4.
func TestCredentialResolutionOrder(t *testing.T) {
	credPath := isolateCredentials(t)
	writeCreds(t, credPath, storedCredentials{Host: "https://staging.example", APIKey: "choir_sk_creds", KeyID: "ak_c"}, 0o600)

	c, err := resolveClient(t)
	if err != nil || c.apiKey != "choir_sk_creds" || c.host != "https://staging.example" {
		t.Fatalf("credentials file: %+v %v", c, err)
	}
	t.Setenv("CHOIR_API_KEY_FILE", writeKeyFile(t, "choir_sk_envfile"))
	if c, err = resolveClient(t); err != nil || c.apiKey != "choir_sk_envfile" {
		t.Fatalf("$CHOIR_API_KEY_FILE: %+v %v", c, err)
	}
	if c.host != defaultHost {
		t.Fatalf("a key not from the credentials file uses the default host, got %q", c.host)
	}
	t.Setenv(apiKeyEnvVar, "choir_sk_env")
	if c, err = resolveClient(t); err != nil || c.apiKey != "choir_sk_env" {
		t.Fatalf("$CHOIR_API_KEY: %+v %v", c, err)
	}
	if c, err = resolveClient(t, "--api-key-file", writeKeyFile(t, "choir_sk_flag")); err != nil || c.apiKey != "choir_sk_flag" {
		t.Fatalf("--api-key-file: %+v %v", c, err)
	}
}

// R2.
func TestCredentialsFileModeEnforced(t *testing.T) {
	credPath := isolateCredentials(t)
	writeCreds(t, credPath, storedCredentials{Host: defaultHost, APIKey: "choir_sk_loose"}, 0o644)
	_, err := resolveClient(t)
	if err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("loose credentials file: err %v", err)
	}
}

// R3.
func TestCredentialsKeyBoundToItsHost(t *testing.T) {
	credPath := isolateCredentials(t)
	writeCreds(t, credPath, storedCredentials{Host: "https://choir.news", APIKey: "choir_sk_creds"}, 0o600)
	if _, err := resolveClient(t, "--host", "https://evil.example"); err == nil || !strings.Contains(err.Error(), "choir login --host") {
		t.Fatalf("--host elsewhere: err %v", err)
	}
	t.Setenv(hostEnvVar, "https://evil.example")
	if _, err := resolveClient(t); err == nil {
		t.Fatal("$CHOIR_HOST elsewhere: no error")
	}
	// The same host, spelled with a trailing slash, is fine.
	t.Setenv(hostEnvVar, "https://choir.news/")
	if c, err := resolveClient(t); err != nil || c.apiKey != "choir_sk_creds" {
		t.Fatalf("same host: %+v %v", c, err)
	}
}

// No key anywhere names choir login.
func TestNoKeyNamesChoirLogin(t *testing.T) {
	isolateCredentials(t)
	_, err := resolveClient(t)
	if err == nil || !strings.Contains(err.Error(), "choir login") {
		t.Fatalf("err %v", err)
	}
}

// R5: choir mcp uses the credentials file with no flags or env.
func TestMCPUsesCredentialsFile(t *testing.T) {
	credPath := isolateCredentials(t)
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"operations":[]}`))
	}))
	t.Cleanup(srv.Close)
	writeCreds(t, credPath, storedCredentials{Host: srv.URL, APIKey: "choir_sk_mcpcreds"}, 0o600)
	cliStdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"self_dev_list","arguments":{"computer":"c1"}}}` + "\n")
	t.Cleanup(func() { cliStdin = os.Stdin })
	var out, errOut bytes.Buffer
	if code := run([]string{"mcp"}, &out, &errOut); code != 0 {
		t.Fatalf("mcp exit %d: %s", code, errOut.String())
	}
	if auth != "Bearer choir_sk_mcpcreds" {
		t.Fatalf("mcp sent %q", auth)
	}
}

// fakeDeviceServer answers the device endpoints. states is the sequence of
// poll answers ("pending", "slow_down", "approved", "denied", "expired").
type fakeDeviceServer struct {
	mu      sync.Mutex
	states  []string
	polls   int
	begun   map[string]any
	revoked []string
	keyN    int
}

func (f *fakeDeviceServer) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/begin", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewDecoder(r.Body).Decode(&f.begun)
		if !strings.HasPrefix(r.UserAgent(), "choir-cli") {
			t.Errorf("user agent %q", r.UserAgent())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "choir_dc_secret", "user_code": "BCDF-GHJK",
			"verification_uri": "http://x/device", "verification_uri_complete": "http://x/device?code=BCDF-GHJK",
			"expires_in": 600, "interval": 0,
		})
	})
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["device_code"] != "choir_dc_secret" {
			t.Errorf("polled with %q", body["device_code"])
		}
		st := f.states[min(f.polls, len(f.states)-1)]
		f.polls++
		switch st {
		case "approved":
			f.keyN++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"api_key": "choir_sk_new" + string(rune('0'+f.keyN)), "key_id": "ak_new" + string(rune('0'+f.keyN)), "label": "choir login: t", "scopes": []string{"admin", "owner:acts"}})
		default:
			code := map[string]string{"pending": "authorization_pending", "slow_down": "slow_down", "denied": "access_denied", "expired": "expired_token"}[st]
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
		}
	})
	mux.HandleFunc("DELETE /auth/api-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.revoked = append(f.revoked, r.PathValue("id")+" by "+r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func runLoginAgainst(t *testing.T, f *fakeDeviceServer, args ...string) (int, string, string) {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	var out, errOut bytes.Buffer
	code := run(append([]string{"login", "--host", srv.URL, "--label", "t"}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

// L1, L2.
func TestLoginWaitsThenWritesCredentials(t *testing.T) {
	credPath := isolateCredentials(t)
	f := &fakeDeviceServer{states: []string{"pending", "slow_down", "pending", "approved"}}
	code, out, errOut := runLoginAgainst(t, f)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if f.polls != 4 || f.begun["label"] != "t" {
		t.Fatalf("polls %d begun %v", f.polls, f.begun)
	}
	if !strings.Contains(errOut, "BCDF-GHJK") || !strings.Contains(errOut, "http://x/device?code=BCDF-GHJK") {
		t.Fatalf("stderr does not show the code and link: %s", errOut)
	}
	if strings.Contains(out+errOut, "choir_sk_new") {
		t.Fatal("the secret was printed")
	}
	info, err := os.Stat(credPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials file: %v %v", info, err)
	}
	dirInfo, _ := os.Stat(filepath.Dir(credPath))
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("credentials dir mode %v", dirInfo.Mode().Perm())
	}
	creds, err := loadCredentials()
	if err != nil || creds.APIKey != "choir_sk_new1" || creds.KeyID != "ak_new1" || !strings.HasPrefix(creds.Host, "http://127.0.0.1") {
		t.Fatalf("creds %+v %v", creds, err)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil || res["key_id"] != "ak_new1" || res["credentials"] != credPath {
		t.Fatalf("stdout %q", out)
	}

	// L4: a second login revokes the first key with itself.
	f.states, f.polls = []string{"approved"}, 0
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	creds.Host = srv.URL
	writeCreds(t, credPath, *creds, 0o600)
	var o2, e2 bytes.Buffer
	if code := run([]string{"login", "--host", srv.URL}, &o2, &e2); code != 0 {
		t.Fatalf("second login %d: %s", code, e2.String())
	}
	if len(f.revoked) != 1 || f.revoked[0] != "ak_new1 by Bearer choir_sk_new1" {
		t.Fatalf("revoked %v", f.revoked)
	}
	if c2, _ := loadCredentials(); c2.KeyID != "ak_new2" {
		t.Fatalf("after re-login %+v", c2)
	}

	// L5: logout revokes the key with itself and removes the file.
	var o3, e3 bytes.Buffer
	if code := run([]string{"logout"}, &o3, &e3); code != 0 {
		t.Fatalf("logout %d: %s", code, e3.String())
	}
	if len(f.revoked) != 2 || f.revoked[1] != "ak_new2 by Bearer choir_sk_new2" {
		t.Fatalf("revoked %v", f.revoked)
	}
	if _, err := os.Stat(credPath); !os.IsNotExist(err) {
		t.Fatalf("credentials file still there: %v", err)
	}
}

// L3.
func TestLoginDeniedOrExpiredWritesNothing(t *testing.T) {
	for _, st := range []string{"denied", "expired"} {
		t.Run(st, func(t *testing.T) {
			credPath := isolateCredentials(t)
			code, _, errOut := runLoginAgainst(t, &fakeDeviceServer{states: []string{"pending", st}})
			if code != 1 {
				t.Fatalf("exit %d, want 1: %s", code, errOut)
			}
			if _, err := os.Stat(credPath); !os.IsNotExist(err) {
				t.Fatal("a credentials file was written")
			}
		})
	}
}

// M1.
func TestMCPInstallCursorMergesAndPrints(t *testing.T) {
	isolateCredentials(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".cursor", "mcp.json")
	_ = os.MkdirAll(filepath.Dir(cfg), 0o755)
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"other":{"command":"x"}},"keep":true}`), 0o644)

	var out, errOut bytes.Buffer
	if code := run([]string{"mcp", "install", "cursor", "--print"}, &out, &errOut); code != 0 {
		t.Fatalf("print %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"mcp"`) {
		t.Fatalf("print output %s", out.String())
	}
	if raw, _ := os.ReadFile(cfg); strings.Contains(string(raw), "choir") {
		t.Fatal("--print wrote the file")
	}

	out.Reset()
	if code := run([]string{"mcp", "install", "cursor"}, &out, &errOut); code != 0 {
		t.Fatalf("install %d: %s", code, errOut.String())
	}
	var doc map[string]any
	raw, _ := os.ReadFile(cfg)
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	servers := doc["mcpServers"].(map[string]any)
	choir := servers["choir"].(map[string]any)
	exe, _ := os.Executable()
	if choir["command"] != exe || strings.Join(toStrings(choir["args"]), " ") != "mcp" || servers["other"] == nil || doc["keep"] != true {
		t.Fatalf("merged config %s", raw)
	}
	if code := run([]string{"mcp", "install", "nosuchhost"}, &out, &errOut); code != 2 {
		t.Fatalf("unknown host exit %d", code)
	}
}

func TestMCPInstallCodexAppendsOnce(t *testing.T) {
	isolateCredentials(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	cfg := filepath.Join(home, ".codex", "config.toml")
	_ = os.MkdirAll(filepath.Dir(cfg), 0o755)
	_ = os.WriteFile(cfg, []byte("model = \"x\"\n"), 0o644)
	for i := 0; i < 2; i++ {
		var out, errOut bytes.Buffer
		if code := run([]string{"mcp", "install", "codex"}, &out, &errOut); code != 0 {
			t.Fatalf("install %d: %s", code, errOut.String())
		}
	}
	raw, _ := os.ReadFile(cfg)
	if strings.Count(string(raw), "[mcp_servers.choir]") != 1 || !strings.HasPrefix(string(raw), "model = \"x\"\n") {
		t.Fatalf("codex config:\n%s", raw)
	}
}

func toStrings(v any) []string {
	raw, _ := v.([]any)
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		out = append(out, s.(string))
	}
	return out
}
