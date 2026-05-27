package maild

import (
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("MAILD_PORT", "")
	t.Setenv("MAILD_STORAGE_ROOT", "")
	t.Setenv("MAILD_DB_PATH", "")
	t.Setenv("MAILD_PRIMARY_DOMAIN", "")
	t.Setenv("MAILD_ROOT_OWNER_ID", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("Port = %q, want %q", cfg.Port, DefaultPort)
	}
	if cfg.PrimaryDomain != DefaultPrimaryDomain {
		t.Fatalf("PrimaryDomain = %q, want %q", cfg.PrimaryDomain, DefaultPrimaryDomain)
	}
	if cfg.RootOwnerID != DefaultRootOwnerID {
		t.Fatalf("RootOwnerID = %q, want %q", cfg.RootOwnerID, DefaultRootOwnerID)
	}
	if cfg.ResendBaseURL != DefaultResendBaseURL {
		t.Fatalf("ResendBaseURL = %q, want %q", cfg.ResendBaseURL, DefaultResendBaseURL)
	}
	if cfg.DBPath != filepath.Join(DefaultLocalDir, "mail.db") {
		t.Fatalf("DBPath = %q", cfg.DBPath)
	}
	if cfg.ProviderMaxBytes != DefaultProviderMaxBody {
		t.Fatalf("ProviderMaxBytes = %d, want %d", cfg.ProviderMaxBytes, DefaultProviderMaxBody)
	}
	if cfg.APIMaxBytes != DefaultAPIMaxBody {
		t.Fatalf("APIMaxBytes = %d, want %d", cfg.APIMaxBytes, DefaultAPIMaxBody)
	}
}

func TestConfigEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Port:             DefaultPort,
		DBPath:           filepath.Join(dir, "nested", "mail.db"),
		StorageRoot:      filepath.Join(dir, "mail"),
		PrimaryDomain:    DefaultPrimaryDomain,
		RootOwnerID:      DefaultRootOwnerID,
		ResendBaseURL:    DefaultResendBaseURL,
		WebhookMaxBytes:  DefaultWebhookMaxBody,
		APIMaxBytes:      DefaultAPIMaxBody,
		ProviderMaxBytes: DefaultProviderMaxBody,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
}
