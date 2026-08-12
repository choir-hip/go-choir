package autoputer

import (
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("AUTOPUTER_PORT", "")
	t.Setenv("AUTOPUTER_ID", "")
	t.Setenv("CHOIR_COMPUTER_ID", "")
	t.Setenv("RUNTIME_STORE_PATH", "")

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

func TestLoadConfigFromVMFallbackEnv(t *testing.T) {
	t.Setenv("AUTOPUTER_PORT", "9090")
	t.Setenv("AUTOPUTER_ID", "custom-autoputer-42")
	t.Setenv("CHOIR_COMPUTER_ID", "")

	cfg := LoadConfig()

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %q", cfg.Port)
	}
	if cfg.ComputerID != "custom-autoputer-42" {
		t.Errorf("expected computer_id custom-autoputer-42, got %q", cfg.ComputerID)
	}
}

func TestLoadConfigUsesStableComputerIdentity(t *testing.T) {
	t.Setenv("AUTOPUTER_ID", "candidate-fleet-realization")
	t.Setenv("CHOIR_COMPUTER_ID", "computer-stable")

	cfg := LoadConfig()

	if cfg.ComputerID != "computer-stable" {
		t.Errorf("expected stable computer_id computer-stable, got %q", cfg.ComputerID)
	}
}
