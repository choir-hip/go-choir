package agentcore

import (
	"context"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Per-computer compute-saving switches (FP0 direction 2026-10-24,
// docs/evidence/switched-off-inventory-2026-10-24.md rows E2 and E3). Both
// were guest-image env vars, all or nothing. Each is now also an owner's
// computer setting, on one computer at a time and live, no deploy:
//
//	research.run_memory_elision  E2, CHOIR_RUN_MEMORY_ELISION
//	research.search_caps         E3, CHOIR_DESK_SEARCH_MAX_RESULTS / _SNIPPET_CHARS
//
// A switch is on when the setting is stored true OR the env var turns it on:
// the env stays the fleet switch. The flags live on the Runtime, one runtime
// per computer, set at boot from the store (anything but a stored true is
// off, a read error included) and by the owner's POST at runtime. They
// default off; off is the rollback. The settings route is the owner act
// (HandleComputerSettings); choir.ProductAPI cannot reach it.

// configureResearchSwitchSettings reads both settings at boot. Anything but a
// stored true is off.
func (rt *Runtime) configureResearchSwitchSettings(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	rt.runMemoryElisionSetting.Store(rt.computerSettingOn(ctx, ownerID, rt.cfg.ComputerID, store.SettingRunMemoryElision))
	rt.searchCapsSetting.Store(rt.computerSettingOn(ctx, ownerID, rt.cfg.ComputerID, store.SettingSearchCaps))
}

// applyResearchSwitchSetting is the owner's change at runtime; a key that is
// not one of the two does nothing. It is idempotent, so a same-value POST
// re-applies it.
func (rt *Runtime) applyResearchSwitchSetting(key string, value bool) {
	if rt == nil {
		return
	}
	switch key {
	case store.SettingRunMemoryElision:
		rt.runMemoryElisionSetting.Store(value)
	case store.SettingSearchCaps:
		rt.searchCapsSetting.Store(value)
	}
}

func (rt *Runtime) runMemoryElisionSettingOn() bool {
	return rt != nil && rt.runMemoryElisionSetting.Load()
}

func (rt *Runtime) searchCapsSettingOn() bool {
	return rt != nil && rt.searchCapsSetting.Load()
}

// runMemoryElisionPolicy is the elision policy for one run: the computer's
// setting or the fleet env, read when the run's memory manager is built, so
// the owner's change reaches the next run.
func (rt *Runtime) runMemoryElisionPolicy(rec *types.RunRecord) runMemoryElision {
	return newRunMemoryElision(rec, rt.runMemoryElisionSettingOn())
}
