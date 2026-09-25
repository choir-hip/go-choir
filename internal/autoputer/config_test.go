package autoputer

import (
	"testing"
)

func TestLoadConfigUsesStableComputerIdentity(t *testing.T) {
	t.Setenv("AUTOPUTER_ID", "candidate-fleet-realization")
	t.Setenv("CHOIR_COMPUTER_ID", "computer-stable")

	cfg := LoadConfig()

	if cfg.ComputerID != "computer-stable" {
		t.Errorf("expected stable computer_id computer-stable, got %q", cfg.ComputerID)
	}
}
