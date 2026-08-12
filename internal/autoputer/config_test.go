package autoputer

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	_ = os.Unsetenv("AUTOPUTER_PORT")
	_ = os.Unsetenv("AUTOPUTER_ID")
	_ = os.Unsetenv("RUNTIME_STORE_PATH")

	cfg := LoadConfig()

	if cfg.Port != "8085" {
		t.Errorf("expected default port 8085, got %q", cfg.Port)
	}
	if cfg.ComputerID != "autoputer-dev" {
		t.Errorf("expected default computer_id autoputer-dev, got %q", cfg.ComputerID)
	}
	if cfg.StorePath != "" {
		t.Errorf("expected empty default store_path, got %q", cfg.StorePath)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	_ = os.Setenv("AUTOPUTER_PORT", "9090")
	_ = os.Setenv("AUTOPUTER_ID", "custom-autoputer-42")
	defer func() { _ = os.Unsetenv("AUTOPUTER_PORT") }()
	defer func() { _ = os.Unsetenv("AUTOPUTER_ID") }()

	cfg := LoadConfig()

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %q", cfg.Port)
	}
	if cfg.ComputerID != "custom-autoputer-42" {
		t.Errorf("expected computer_id custom-autoputer-42, got %q", cfg.ComputerID)
	}
}
