package auth

import (
	"testing"
	"time"
)
func TestConfigValidationRejectsEmptyRPID(t *testing.T) {
	cfg := &Config{
		Port:              "8081",
		DBPath:            "/tmp/test.db",
		RPID:              "", // empty
		RPOrigins:         []string{"https://example.com"},
		JWTPrivateKeyPath: "/tmp/key",
		AccessTokenTTL:    5 * time.Minute,
		RefreshTokenTTL:   720 * time.Hour,
		CookieSecure:      true,
	}
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for empty RPID, got nil")
	}
}

func TestConfigValidationRejectsEmptyRPOrigins(t *testing.T) {
	cfg := &Config{
		Port:              "8081",
		DBPath:            "/tmp/test.db",
		RPID:              "example.com",
		RPOrigins:         []string{},
		JWTPrivateKeyPath: "/tmp/key",
		AccessTokenTTL:    5 * time.Minute,
		RefreshTokenTTL:   720 * time.Hour,
		CookieSecure:      true,
	}
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for empty RPOrigins, got nil")
	}
}

func TestConfigValidationRejectsNegativeTTLs(t *testing.T) {
	cfg := &Config{
		Port:              "8081",
		DBPath:            "/tmp/test.db",
		RPID:              "example.com",
		RPOrigins:         []string{"https://example.com"},
		JWTPrivateKeyPath: "/tmp/key",
		AccessTokenTTL:    -1 * time.Minute,
		RefreshTokenTTL:   720 * time.Hour,
		CookieSecure:      true,
	}
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for negative AccessTokenTTL, got nil")
	}

	cfg.AccessTokenTTL = 5 * time.Minute
	cfg.RefreshTokenTTL = -1 * time.Hour
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for negative RefreshTokenTTL, got nil")
	}
}
