package proxy

import (
	"os"
	"testing"
)

func TestLoadConfigDerivesPublicKeyPathFromAuthKeyPath(t *testing.T) {
	origProxyKey := os.Getenv("PROXY_AUTH_PUBLIC_KEY_PATH")
	origAuthKey := os.Getenv("AUTH_JWT_PRIVATE_KEY_PATH")
	_ = os.Unsetenv("PROXY_AUTH_PUBLIC_KEY_PATH")
	_ = os.Setenv("AUTH_JWT_PRIVATE_KEY_PATH", "/tmp/shared/auth-signing-key")
	defer func() {
		if origProxyKey == "" {
			_ = os.Unsetenv("PROXY_AUTH_PUBLIC_KEY_PATH")
		} else {
			_ = os.Setenv("PROXY_AUTH_PUBLIC_KEY_PATH", origProxyKey)
		}
		if origAuthKey == "" {
			_ = os.Unsetenv("AUTH_JWT_PRIVATE_KEY_PATH")
		} else {
			_ = os.Setenv("AUTH_JWT_PRIVATE_KEY_PATH", origAuthKey)
		}
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.AuthPublicKeyPath != "/tmp/shared/auth-signing-key.pub" {
		t.Errorf("AuthPublicKeyPath: got %q, want %q", cfg.AuthPublicKeyPath, "/tmp/shared/auth-signing-key.pub")
	}
}
