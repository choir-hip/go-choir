package vmmanager

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// The automatic checkpoint cadence per computer (X1 slice 3,
// docs/design/apply-checkpoint-without-replay-2026-10-26.md, F6; review 329).
// The guest compiles the cadence off. The computers this selector names run it,
// with the host's values:
//
//	VM_CHECKPOINT_CADENCE_ONLY          computer ids (computer_select.go); empty: none
//	VM_CHECKPOINT_CADENCE_EVERY_EVENTS  events between checkpoints, default 2500
//	VM_CHECKPOINT_CADENCE_EVERY         time between checkpoints, default 60m
//
// The live extract a bind makes holds the computer's appends for a time that
// is unmeasured on the paper (R2). So the deploy names one computer, the
// operator reads last_bind_millis in its checkpoint listing, and only then
// widens the list. Name computer ids only; never "*" or class: (the paper is
// a computer id like any other, and stays off until measured).
//
// The selection is two kernel boot arguments, so neither the guest nor a
// capsule can set it; the guest boot script validates both before it copies
// them into the daemon's environment file. Boot arguments are in the
// launch-config hash, so a newly selected computer cold boots once.

const (
	defaultCheckpointCadenceEveryEvents = 2500
	defaultCheckpointCadenceEvery       = "60m"
)

// loadCheckpointCadenceFromEnv reads the selector and its values.
func loadCheckpointCadenceFromEnv(cfg *ManagerConfig) {
	cfg.CheckpointCadenceOnly = parseComputerSelector(os.Getenv("VM_CHECKPOINT_CADENCE_ONLY"))
	cfg.CheckpointCadenceEveryEvents = defaultCheckpointCadenceEveryEvents
	if raw := strings.TrimSpace(os.Getenv("VM_CHECKPOINT_CADENCE_EVERY_EVENTS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			cfg.CheckpointCadenceEveryEvents = n
		} else {
			cfg.CheckpointCadenceEveryEvents = -1 // refused at launch, never defaulted silently
		}
	}
	cfg.CheckpointCadenceEvery = defaultCheckpointCadenceEvery
	if raw := strings.TrimSpace(os.Getenv("VM_CHECKPOINT_CADENCE_EVERY")); raw != "" {
		cfg.CheckpointCadenceEvery = raw
	}
}

// checkpointCadenceKernelParams is the boot arguments for cfg's launch: nil for
// a computer the list does not name, and nil when the values give no usable
// trigger (a selection must never reach the guest with a bad value; the guest
// would then run its own off default and read as selected).
func (m *Manager) checkpointCadenceKernelParams(cfg VMConfig) []string {
	if !selectsComputer(m.cfg.CheckpointCadenceOnly, cfg) {
		return nil
	}
	// A value that is set but bad refuses the whole selection: half a
	// configuration is not what the operator asked for.
	var params []string
	switch n := m.cfg.CheckpointCadenceEveryEvents; {
	case n < 0:
		return nil
	case n > 0:
		params = append(params, fmt.Sprintf("choir.checkpoint_every_events=%d", n))
	}
	if every := strings.TrimSpace(m.cfg.CheckpointCadenceEvery); every != "" {
		d, err := time.ParseDuration(every)
		if err != nil || d < 0 || strings.ContainsAny(every, " \t") || strings.Trim(every, "0123456789smh") != "" {
			return nil
		}
		if d > 0 {
			params = append(params, "choir.checkpoint_every="+every)
		}
	}
	return params
}
