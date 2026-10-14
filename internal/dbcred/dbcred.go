// Package dbcred applies the host's Dolt SQL credential to a MySQL DSN.
//
// Node B's platform (Store A, :13306) and corpus (Store B, :13307) Dolt
// servers accepted root with no password from any local process
// (docs/problems/loopback-is-a-credential-2026-10-14.md). The repair gives
// each server an application user with a password and privileges on its one
// database. The DSNs in configuration keep their shape (address, database,
// parameters); the user and password come from here:
//
//	CHOIR_DOLT_USER           the SQL user to connect as (e.g. "choir")
//	CHOIR_DOLT_PASSWORD_FILE  the password file, else
//	$CREDENTIALS_DIRECTORY/choir-dolt-password (systemd LoadCredential=)
//
// With neither set, the DSN is used unchanged. With either set, a missing
// or empty password is an error: a service never silently falls back to the
// DSN's passwordless root. The password is never logged or put in an error.
package dbcred

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const (
	EnvUser         = "CHOIR_DOLT_USER"
	EnvPasswordFile = "CHOIR_DOLT_PASSWORD_FILE"
	CredentialName  = "choir-dolt-password"
)

// ApplyDSN returns dsn with the configured user and password applied.
func ApplyDSN(dsn string) (string, error) {
	user := strings.TrimSpace(os.Getenv(EnvUser))
	path := strings.TrimSpace(os.Getenv(EnvPasswordFile))
	if path == "" {
		if dir := strings.TrimSpace(os.Getenv("CREDENTIALS_DIRECTORY")); dir != "" {
			candidate := filepath.Join(dir, CredentialName)
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}
	if user == "" && path == "" {
		return dsn, nil
	}
	if path == "" {
		return "", fmt.Errorf("dbcred: %s=%q is set but no password source (%s or $CREDENTIALS_DIRECTORY/%s)", EnvUser, user, EnvPasswordFile, CredentialName)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("dbcred: read password file %s: %w", path, err)
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("dbcred: password file %s is empty", path)
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		// The DSN itself has no password in it on this path's inputs, but
		// keep the message generic either way.
		return "", fmt.Errorf("dbcred: parse DSN: invalid DSN")
	}
	if user != "" {
		cfg.User = user
	}
	cfg.Passwd = password
	return cfg.FormatDSN(), nil
}
