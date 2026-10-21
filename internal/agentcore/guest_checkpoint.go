package agentcore

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// The guest's Dolt checkpoint cadence at the runtime seam (ARCH-13,
// docs/problems/guest-checkpoint-on-the-write-path-2026-10-21.md). The
// committer lives in internal/store; this file is the flag.

// configureGuestCheckpoint applies the flag at boot: the env value
// CHOIR_GUEST_CHECKPOINT=debounced or the owner's computer setting
// store.debounced_checkpoint. With neither, the store keeps count mode.
func (rt *Runtime) configureGuestCheckpoint(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	cfg := store.CheckpointConfigFromEnv()
	if !cfg.Debounced {
		ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
		cfg.Debounced = rt.computerSettingOn(ctx, ownerID, rt.cfg.ComputerID, store.SettingDebouncedCheckpoint)
	}
	if !cfg.Debounced {
		return
	}
	rt.store.ConfigureCheckpoint(cfg)
	stats := rt.store.CheckpointStats()
	log.Printf("runtime: guest checkpoint debounced (window %dms, ops bound %d)", stats.DebounceMS, stats.MaxOps)
}

// applyGuestCheckpointSetting is the owner's setting change at runtime. Off
// returns to counted inline commits; the env flag, when set, keeps it on.
func (rt *Runtime) applyGuestCheckpointSetting(value bool) {
	if rt == nil || rt.store == nil {
		return
	}
	cfg := store.CheckpointConfigFromEnv()
	cfg.Debounced = cfg.Debounced || value
	rt.store.ConfigureCheckpoint(cfg)
}
