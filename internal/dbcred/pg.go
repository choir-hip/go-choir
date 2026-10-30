package dbcred

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// PostgreSQL credential, the successor to the Dolt one above
// (docs/design/data-system-build-sheet-2026-10-30.md, decision 11):
//
//	CHOIR_PG_PASSWORD_FILE  the password file, else
//	$CREDENTIALS_DIRECTORY/choir-pg-password (systemd LoadCredential=)
//
// The DSN carries host, port, database and user; the password comes from
// here. With no source configured the DSN is used unchanged (peer or trust
// auth on a unix socket, and tests). A configured but missing or empty
// password is an error. The password is never logged or put in an error.
const (
	EnvPGPasswordFile = "CHOIR_PG_PASSWORD_FILE"
	PGCredentialName  = "choir-pg-password"
)

// ApplyPGDSN returns dsn with the configured password applied. dsn may be a
// postgres:// URL or a libpq key=value string.
func ApplyPGDSN(dsn string) (string, error) {
	path := strings.TrimSpace(os.Getenv(EnvPGPasswordFile))
	if path == "" {
		if dir := strings.TrimSpace(os.Getenv("CREDENTIALS_DIRECTORY")); dir != "" {
			candidate := filepath.Join(dir, PGCredentialName)
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}
	if path == "" {
		return dsn, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("dbcred: read password file %s: %w", path, err)
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("dbcred: password file %s is empty", path)
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("dbcred: parse DSN: invalid DSN")
		}
		user := "choir"
		if u.User != nil && u.User.Username() != "" {
			user = u.User.Username()
		}
		u.User = url.UserPassword(user, password)
		return u.String(), nil
	}
	fields := strings.Fields(dsn)
	kept := fields[:0]
	for _, f := range fields {
		if !strings.HasPrefix(f, "password=") {
			kept = append(kept, f)
		}
	}
	return strings.Join(append(kept, "password='"+strings.ReplaceAll(strings.ReplaceAll(password, `\`, `\\`), `'`, `\'`)+"'"), " "), nil
}
