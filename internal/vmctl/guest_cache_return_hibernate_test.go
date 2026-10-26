package vmctl

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// D6 S3, vmctl's half: which hibernates ask the manager for a cache return
// before the snapshot (docs/design/density-d6-page-cache-return-2026-10-26.md
// section 4.3; docs/problems/density-station-d-failure-modes-2026-10-26.md
// D6, M12, M15). The manager's side is in internal/vmmanager.
//
// Ways it can fail:
//   - M15: with the switch off, or the hibernate trigger not listed, or the
//     computer not selected, the manager is asked for the cache return;
//   - M12: the paper (critical_protected) or a public_platform computer gets
//     one;
//   - observe asks for a real return, or active for a dry run;
//   - a manager that predates the option is called with it;
//   - the option changes how a busy guest or a manager error is handled.

type optsHibernateManager struct {
	*processControlVMManager
	mu       sync.Mutex
	plain    []string
	withOpts []HibernateCacheReturn
	optsVM   []string
	err      error
}

func (m *optsHibernateManager) HibernateVM(vmID string) error {
	m.mu.Lock()
	m.plain = append(m.plain, vmID)
	m.mu.Unlock()
	return nil
}

func (m *optsHibernateManager) HibernateVMWithCacheReturn(vmID string, opt HibernateCacheReturn) error {
	m.mu.Lock()
	m.withOpts = append(m.withOpts, opt)
	m.optsVM = append(m.optsVM, vmID)
	err := m.err
	m.mu.Unlock()
	return err
}

type busyErr struct{}

func (busyErr) Error() string            { return "guest busy" }
func (busyErr) HibernateGuestBusy() bool { return true }

func hibernateRegistry(t *testing.T, cfg GuestCacheReturnConfig, mgr VMManager) *OwnershipRegistry {
	t.Helper()
	reg, _ := hostCapacityRegistry(t, mgr)
	reg.SetGuestCacheReturnConfig(cfg)
	return reg
}

func hibernateCfg(mode string, triggers ...string) GuestCacheReturnConfig {
	return GuestCacheReturnConfig{Mode: mode, Only: []string{"*"}, Triggers: triggers, Scope: "daemon"}
}

func activeComputer(reg *OwnershipRegistry, vmID string) *VMOwnership {
	own := &VMOwnership{VMID: vmID, ComputerID: "comp-" + vmID, UserID: "user-" + vmID, DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateActive, ComputerURL: "http://10.0.0.9:8085", LastActiveAt: time.Now()}
	addOwnership(reg, own)
	return own
}

func newOptsManager() *optsHibernateManager {
	return &optsHibernateManager{processControlVMManager: newProcessControlVMManager()}
}

func TestHibernateAsksForACacheReturnWhenSwitchSelectorAndTriggerAllowIt(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cfg    GuestCacheReturnConfig
		scope  string
		dryRun bool
	}{
		{"active", hibernateCfg(GuestCacheReturnActive, "idle", "hibernate"), "daemon", false},
		{"observe is a dry run", hibernateCfg(GuestCacheReturnObserve, "hibernate"), "daemon", true},
		{"root scope", func() GuestCacheReturnConfig {
			c := hibernateCfg(GuestCacheReturnActive, "hibernate")
			c.Scope = "root"
			return c
		}(), "root", false},
	} {
		mgr := newOptsManager()
		reg := hibernateRegistry(t, tc.cfg, mgr)
		own := activeComputer(reg, "vm-a")
		if err := reg.hibernateVMForDesktopWithReason(own.UserID, own.DesktopID, "idle"); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(mgr.plain) != 0 || len(mgr.withOpts) != 1 || mgr.optsVM[0] != "vm-a" ||
			mgr.withOpts[0].Scope != tc.scope || mgr.withOpts[0].DryRun != tc.dryRun {
			t.Fatalf("%s: plain=%v opts=%+v, want one option call scope=%s dry_run=%v", tc.name, mgr.plain, mgr.withOpts, tc.scope, tc.dryRun)
		}
		if got := reg.GetOwnership(own.UserID); got == nil || got.State != VMStateHibernated {
			t.Fatalf("%s: the computer is not hibernated: %+v", tc.name, got)
		}
	}
}

func TestHibernateAsksForNothingWhenSwitchTriggerOrSelectorSayNo(t *testing.T) {
	notSelected := hibernateCfg(GuestCacheReturnActive, "hibernate")
	notSelected.Only = []string{"someone-else"}
	nobody := hibernateCfg(GuestCacheReturnActive, "hibernate")
	nobody.Only = nil
	for name, cfg := range map[string]GuestCacheReturnConfig{
		"off":                 hibernateCfg(GuestCacheReturnOff, "hibernate"),
		"a typo of active":    hibernateCfg("actve", "hibernate"),
		"trigger not listed":  hibernateCfg(GuestCacheReturnActive, "idle", "pressure"),
		"no trigger at all":   hibernateCfg(GuestCacheReturnActive),
		"computer not chosen": notSelected,
		"nobody chosen":       nobody,
	} {
		mgr := newOptsManager()
		reg := hibernateRegistry(t, cfg, mgr)
		own := activeComputer(reg, "vm-a")
		if err := reg.hibernateVMForDesktopWithReason(own.UserID, own.DesktopID, "idle"); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(mgr.withOpts) != 0 || len(mgr.plain) != 1 {
			t.Fatalf("%s: plain=%v opts=%+v, want one plain hibernate", name, mgr.plain, mgr.withOpts)
		}
	}
}

// M12: the paper and a public platform computer never get the act.
func TestHibernateOfProtectedPlatformComputersAsksForNoCacheReturn(t *testing.T) {
	for _, class := range []WarmnessClass{WarmnessClassCriticalProtected, WarmnessClassPublicPlatform} {
		mgr := newOptsManager()
		reg := hibernateRegistry(t, hibernateCfg(GuestCacheReturnActive, "hibernate"), mgr)
		own := activeComputer(reg, "vm-platform")
		reg.mu.Lock()
		own.WarmnessClass = class
		reg.mu.Unlock()
		if err := reg.hibernateVMForDesktopWithReason(own.UserID, own.DesktopID, "idle"); err != nil {
			t.Fatalf("%s: %v", class, err)
		}
		if len(mgr.withOpts) != 0 || len(mgr.plain) != 1 {
			t.Fatalf("%s: plain=%v opts=%+v, want a plain hibernate", class, mgr.plain, mgr.withOpts)
		}
	}
}

// A manager that predates the option is called as before.
func TestHibernateWithAManagerWithoutTheOptionIsPlain(t *testing.T) {
	mgr := newProcessControlVMManager()
	reg := hibernateRegistry(t, hibernateCfg(GuestCacheReturnActive, "hibernate"), mgr)
	own := activeComputer(reg, "vm-a")
	if err := reg.hibernateVMForDesktopWithReason(own.UserID, own.DesktopID, "idle"); err != nil {
		t.Fatal(err)
	}
	if len(mgr.hibernates) != 1 || mgr.hibernates[0] != "vm-a" {
		t.Fatalf("hibernates = %v", mgr.hibernates)
	}
}

// The option does not change how a busy guest or a failure is handled.
func TestHibernateWithTheOptionKeepsBusyAndErrorHandling(t *testing.T) {
	mgr := newOptsManager()
	mgr.err = busyErr{}
	reg := hibernateRegistry(t, hibernateCfg(GuestCacheReturnActive, "hibernate"), mgr)
	own := activeComputer(reg, "vm-a")
	err := reg.hibernateVMForDesktopWithReason(own.UserID, own.DesktopID, "idle")
	if !errors.Is(err, errHibernateGuestBusy) {
		t.Fatalf("err = %v, want errHibernateGuestBusy", err)
	}
	if got := reg.GetOwnership(own.UserID); got.State != VMStateActive {
		t.Fatalf("a busy guest was marked %s", got.State)
	}
}
