package dbcred

// Failure modes (written before the code;
// docs/problems/loopback-is-a-credential-2026-10-14.md):
//
//  1. no credential configured: the DSN is returned unchanged (every guest,
//     local development, and the "users" rollout stage).
//  2. a credential is configured but its file is missing or empty: an error,
//     never a silent fall back to the DSN's passwordless root.
//  3. the password leaks: no error message contains it.
//  4. the DSN's parameters (parseTime, multiStatements, clientFoundRows,
//     database, address) are lost in the rewrite.
//  5. a password with DSN metacharacters (@ / : ?) corrupts the DSN.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

const baseDSN = "root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true&clientFoundRows=true"

func writeSecret(t *testing.T, dir, name, value string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func clearEnv(t *testing.T) {
	t.Setenv(EnvUser, "")
	t.Setenv(EnvPasswordFile, "")
	t.Setenv("CREDENTIALS_DIRECTORY", "")
}

func TestApplyDSNUnchangedWithoutCredential(t *testing.T) {
	clearEnv(t)
	got, err := ApplyDSN(baseDSN)
	if err != nil || got != baseDSN {
		t.Fatalf("ApplyDSN = %q, %v; want the DSN unchanged", got, err)
	}
}

func TestApplyDSNUsesSystemdCredentialAndKeepsParameters(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	const secret = "p@ss/w:rd?with&meta=chars"
	writeSecret(t, dir, CredentialName, secret+"\n")
	t.Setenv("CREDENTIALS_DIRECTORY", dir)
	t.Setenv(EnvUser, "choir")
	got, err := ApplyDSN(baseDSN)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("rewritten DSN does not parse: %v", err)
	}
	if cfg.User != "choir" || cfg.Passwd != secret {
		t.Fatalf("user/password not applied: user=%q", cfg.User)
	}
	if cfg.Addr != "127.0.0.1:13306" || cfg.DBName != "platform" || !cfg.ParseTime || !cfg.MultiStatements || !cfg.ClientFoundRows {
		t.Fatalf("parameters lost: %+v", cfg)
	}
}

func TestApplyDSNFailsClosedOnMissingOrEmptyPassword(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv(EnvUser, "choir")
	t.Setenv(EnvPasswordFile, filepath.Join(dir, "absent"))
	if _, err := ApplyDSN(baseDSN); err == nil {
		t.Fatal("missing password file did not fail")
	}
	t.Setenv(EnvPasswordFile, writeSecret(t, dir, "empty", "  \n"))
	if _, err := ApplyDSN(baseDSN); err == nil {
		t.Fatal("empty password file did not fail")
	}
	// A user without any password source is a configuration error too.
	t.Setenv(EnvPasswordFile, "")
	if _, err := ApplyDSN(baseDSN); err == nil {
		t.Fatal("user without a password source did not fail")
	}
}

func TestApplyDSNErrorsNeverContainPassword(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	const secret = "never-print-this-secret-0123456789"
	t.Setenv(EnvPasswordFile, writeSecret(t, dir, "pw", secret))
	_, err := ApplyDSN("this is not a dsn :: ((")
	if err == nil {
		t.Fatal("bad DSN accepted")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked the password: %v", err)
	}
}

func TestApplyDSNPasswordWithoutUserKeepsDSNUser(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv(EnvPasswordFile, writeSecret(t, dir, "pw", "rootsecret-rootsecret"))
	got, err := ApplyDSN(baseDSN)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := mysql.ParseDSN(got)
	if cfg.User != "root" || cfg.Passwd != "rootsecret-rootsecret" {
		t.Fatalf("user=%q, want root with the password", cfg.User)
	}
}
