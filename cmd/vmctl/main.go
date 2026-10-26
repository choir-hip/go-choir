package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yusefmosiah/go-choir/internal/computertiers"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbcred"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"github.com/yusefmosiah/go-choir/internal/vmmanager"
)

func main() {
	port := server.PortFromEnv("VMCTL_PORT", "8083")

	// The autoputer URL base is where VM-backed autoputer runtimes are
	// reachable. In host-process mode this is the local autoputer.
	// In production with Firecracker, vmctl will return per-VM URLs.
	autoputerURLBase := envOr("VMCTL_AUTOPUTER_URL_BASE", "http://127.0.0.1:8085")

	registry := vmctl.NewOwnershipRegistry(autoputerURLBase)
	if ownershipPath := os.Getenv("VMCTL_OWNERSHIP_PATH"); ownershipPath != "" {
		if err := registry.SetPersistencePath(ownershipPath); err != nil {
			log.Fatalf("vmctl: load ownership registry: %v", err)
		}
		log.Printf("vmctl: ownership persistence enabled (%s)", ownershipPath)
	} else if stateDir := os.Getenv("VM_STATE_DIR"); stateDir != "" {
		ownershipPath = filepath.Join(stateDir, "ownerships.json")
		if err := registry.SetPersistencePath(ownershipPath); err != nil {
			log.Fatalf("vmctl: load ownership registry: %v", err)
		}
		log.Printf("vmctl: ownership persistence enabled (%s)", ownershipPath)
	}

	// Configure the gateway URL for issuing autoputer credentials to VM guests.
	// When Firecracker VMs are active, each guest autoputer needs a token to
	// authenticate to the host-side gateway for provider access.
	if gwURL := os.Getenv("VMCTL_GATEWAY_URL"); gwURL != "" {
		registry.SetGatewayURL(gwURL)
		log.Printf("vmctl: gateway URL configured for VM token issuance")
	}
	if corpusdURL := os.Getenv("VMCTL_CORPUSD_URL"); corpusdURL != "" {
		registry.SetCorpusdURL(corpusdURL)
		log.Printf("vmctl: corpusd URL configured for guest event credentials")

		// Retained-aware recovery admission: durable typed conditions gate
		// every realization start before resource allocation, and blocked
		// computers enqueue the deduplicated checkpoint/repair job.
		jobsURL := strings.TrimSpace(os.Getenv("VMCTL_RECOVERY_JOBS_URL"))
		if jobsURL == "" {
			jobsURL = strings.TrimSpace(corpusdURL)
		}
		conditionsPath := strings.TrimSpace(os.Getenv("VMCTL_RECOVERY_CONDITIONS_PATH"))
		if conditionsPath == "" {
			stateDir := strings.TrimSpace(os.Getenv("VM_STATE_DIR"))
			if ownershipPath := strings.TrimSpace(os.Getenv("VMCTL_OWNERSHIP_PATH")); ownershipPath != "" {
				stateDir = filepath.Dir(ownershipPath)
			}
			if stateDir != "" {
				conditionsPath = filepath.Join(stateDir, "recovery-conditions.json")
			}
		}
		if conditionsPath == "" {
			log.Printf("vmctl: recovery admission disabled (no VM_STATE_DIR/VMCTL_RECOVERY_CONDITIONS_PATH)")
		} else if err := registry.ConfigureRecoveryAdmission(vmctl.RecoveryAdmissionConfig{
			StatePath:  conditionsPath,
			CorpusdURL: strings.TrimSpace(corpusdURL),
			JobsURL:    jobsURL,
		}); err != nil {
			log.Fatalf("vmctl: configure recovery admission: %v", err)
		} else {
			log.Printf("vmctl: recovery admission configured (conditions=%s jobs=%s)", conditionsPath, jobsURL)
		}
	}

	// Configure idle timeout for automatic VM lifecycle management.
	// After this duration of inactivity, VMs transition to hibernated
	// state (VAL-VM-008, VAL-CROSS-116).
	idleSweeperEnabled := false
	idleSweepInterval := time.Minute
	if v := os.Getenv("VMCTL_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			registry.SetIdleTimeout(d)
			idleSweeperEnabled = true
			log.Printf("vmctl: idle timeout set to %s", d)
		}
	}
	// Density D3a: a guest whose /health does not report live_capsules keeps
	// the legacy idle rule, however short VMCTL_IDLE_TIMEOUT is.
	registry.SetLegacyGuestIdleTimeout(legacyGuestIdleTimeoutFromEnv())
	if v := os.Getenv("VMCTL_IDLE_SWEEP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			idleSweepInterval = d
		}
	}
	if cfg, ok := pressureReclaimConfigFromEnv(); ok {
		registry.SetPressureReclaimConfig(cfg)
		idleSweeperEnabled = true
		log.Printf("vmctl: pressure reclaim mode=%s signal=%s min_idle=%s max_candidates=%d", cfg.Mode, cfg.Signal, cfg.MinIdle, cfg.MaxCandidates)
	}
	admission := hostAdmissionConfigFromEnv()
	registry.SetHostAdmissionConfig(admission)
	log.Printf("vmctl: host admission mode=%s default_need_mib=%d min_need_mib=%d headroom_mib=%d settle=%s make_room=%s make_room_max_victims=%d make_room_settle=%s",
		registry.HostAdmissionSummary().Mode, admission.DefaultNeedBytes>>20, admission.MinNeedBytes>>20, admission.HeadroomBytes>>20, admission.Settle,
		admission.MakeRoom, admission.MakeRoomMaxVictims, admission.MakeRoomSettleWait)
	// Density D6: guests return cold page cache to the host. Off unless
	// VMCTL_GUEST_CACHE_RETURN is exactly observe or active. The idle trigger
	// has its own loop (started below); the switch does not start the
	// maintenance sweep (review 333).
	cacheReturn := guestCacheReturnConfigFromEnv()
	registry.SetGuestCacheReturnConfig(cacheReturn)
	log.Printf("vmctl: guest cache return mode=%s only=%s triggers=%s scope=%s idle=%s pressure_idle=%s pressure_budget=%s",
		cacheReturn.Mode, strings.Join(cacheReturn.Only, ","), strings.Join(cacheReturn.Triggers, ","), cacheReturn.Scope,
		cacheReturn.Idle, cacheReturn.PressureIdle, cacheReturn.PressureBudget)
	if cfg, ok := retentionPruneConfigFromEnv(); ok {
		registry.SetRetentionPruneConfig(cfg)
		idleSweeperEnabled = true
		log.Printf("vmctl: retention prune mode=%s ephemeral_domains=%s max_deletes=%d max_bytes=%d test_idle_mode=%s test_idle_min_age=%s quarantine_mode=%s quarantine_min_age=%s selector_named=%d", cfg.Mode, strings.Join(cfg.EphemeralEmailDomains, ","), cfg.MaxDeletes, cfg.MaxBytes, cfg.TestIdleMode, cfg.TestIdleMinAge, cfg.QuarantineMode, cfg.QuarantineMinAge, len(cfg.SelectorNamed))
	}
	if cfg, ok := retentionShadowPruneConfigFromEnv(); ok {
		registry.SetRetentionShadowPruneConfig(cfg)
		log.Printf("vmctl: retention shadow prune mode=dry-run requested_mode=%s ephemeral_domains=%s ephemeral_user_prefixes=%s max_deletes=%d max_bytes=%d", cfg.Mode, strings.Join(cfg.EphemeralEmailDomains, ","), strings.Join(cfg.EphemeralUserIDPrefixes, ","), cfg.MaxDeletes, cfg.MaxBytes)
	}
	warmnessPolicy, err := warmnessPolicyConfigFromEnv()
	if err != nil {
		log.Fatalf("vmctl: computer tiers: %v", err)
	}
	registry.SetWarmnessPolicyConfig(warmnessPolicy)
	log.Printf("vmctl: warmness policy primary_keepalive_mode=%s paid_user_count=%d platform_computer_count=%d test_computer_count=%d", warmnessPolicy.PrimaryKeepaliveMode, len(warmnessPolicy.PaidUserIDs), len(warmnessPolicy.PlatformComputerIDs), len(warmnessPolicy.TestComputerIDs))

	// Check if Firecracker is available on this host.
	// If so, create a VM manager for real Firecracker lifecycle management
	// and wire it to the ownership registry so that VM boot/stop/resume
	// operations are delegated to real Firecracker VMs (VAL-VM-010).
	// If not, vmctl can still run in host-process mode for local development,
	// Deployed vmctl always has Firecracker; keep the manager in scope for
	// recover_current's filesystem-safe quarantine/staging operations.
	var mgr *vmmanager.Manager
	if vmmanager.IsFirecrackerAvailable() {
		mgrCfg := vmmanager.LoadConfigFromEnv()
		if err := mgrCfg.Validate(); err != nil {
			log.Fatalf("vmctl: Firecracker config validation failed: %v", err)
		}
		mgr = vmmanager.NewManager(mgrCfg)
		mgr.Start()
		// Guests outlive vmctl by default: a vmctl exit must not take every
		// computer with it (vmctl 360 review Phase 0 step 7).
		if envBool("VMCTL_STOP_MANAGED_ON_EXIT", false) {
			defer mgr.Stop()
		} else {
			defer mgr.StopHealthChecks()
			log.Printf("vmctl: managed VMs will be left running on process exit for reattach")
		}

		// Wire the manager to the registry via an adapter that
		// translates between the vmctl and vmmanager interfaces.
		registry.SetVMManager(&vmManagerAdapter{mgr: mgr})

		log.Printf("vmctl: Firecracker VM manager started (kernel=%s rootfs=%s)", mgrCfg.KernelImagePath, mgrCfg.RootfsPath)
	} else {
		log.Fatal("vmctl: Firecracker is mandatory; host-process autoputer mode is forbidden")
	}
	handler := vmctl.NewHandler(registry)
	handler.SetColdRecoveryStorage(mgr)
	if corpusdURL := strings.TrimSpace(os.Getenv("VMCTL_CORPUSD_URL")); corpusdURL != "" {
		handler.SetColdRecoveryHeadReader(vmctl.HTTPRecoveryHeadReader{BaseURL: corpusdURL})
		handler.SetColdRecoveryVerifier(vmctl.HTTPRecoveryVerifier{
			GuestURLFor: func(computerID string) (string, error) {
				ownership := registry.GetOwnershipByComputerID(computerID)
				if ownership == nil {
					return "", fmt.Errorf("computer ownership not found")
				}
				return ownership.ComputerURL, nil
			},
		})
		log.Printf("vmctl: recover_current corpusd head and guest replay authorities configured")
	}
	handler.RequireRouteAuthority()
	if dsn := strings.TrimSpace(os.Getenv("VMCTL_ROUTE_DSN")); dsn != "" {
		// The host's Dolt user and password (dbcred; never logged).
		dsn, err := dbcred.ApplyDSN(dsn)
		if err != nil {
			log.Fatalf("vmctl: ComputerVersion route database credential: %v", err)
		}
		routeDB, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("vmctl: open ComputerVersion route database: %v", err)
		}
		defer func() { _ = routeDB.Close() }()
		artifactVerifier := computerversion.NewLocalArtifactContentVerifier(os.Getenv("VMCTL_ARTIFACTS_ROOT"))
		inputs := computerversion.NewSQLInputCatalog(routeDB, artifactVerifier)
		ledger := routeledger.NewSQLLedger(routeDB)
		// A slow or stalled route database must not crash-loop vmctl: on
		// 2026-10-15 the old log.Fatalf on the first ping failure exited every
		// 30 s, hit start-limit-hit after 10 restarts and left the host with no
		// VM lifecycle until a manual reset-failed. Retry in-process with
		// capped backoff, each attempt bounded at 30 s; the route authority
		// stays unset (and routing fails closed) until the database answers.
		// SIGTERM ends the wait so a unit stop is not delayed.
		waitCtx, stopWait := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		err = retryWithBackoff(waitCtx, retryPolicy{
			AttemptTimeout: 30 * time.Second,
			Initial:        time.Second,
			Max:            30 * time.Second,
		}, "ComputerVersion route database", func(ctx context.Context) error {
			if err := routeDB.PingContext(ctx); err != nil {
				return fmt.Errorf("ping: %w", err)
			}
			if err := inputs.EnsureSchema(ctx); err != nil {
				return fmt.Errorf("initialize immutable input catalog: %w", err)
			}
			if err := ledger.EnsureSchema(ctx); err != nil {
				return fmt.Errorf("initialize route ledger: %w", err)
			}
			return nil
		})
		stopWait()
		if err != nil {
			log.Printf("vmctl: stopped while waiting for the ComputerVersion route database: %v", err)
			return
		}
		authority, err := vmctl.NewRouteAuthority(ledger, inputs)
		if err != nil {
			log.Fatalf("vmctl: initialize ComputerVersion route authority: %v", err)
		}
		handler.SetRouteAuthority(authority)
		log.Printf("vmctl: ComputerVersion route authority configured on the corpusd world-wire SQL server")
	} else {
		log.Printf("vmctl: ComputerVersion route authority unavailable (VMCTL_ROUTE_DSN is not configured)")
	}
	// S3: drop memory snapshots of computers that are not hibernated (a
	// stopped computer cold boots) and interrupted captures, before any
	// computer is started or woken.
	registry.SweepOrphanSnapshots()
	// Density D1: make-room's hibernates take the idle sweep's route guard.
	registry.SetMakeRoomRouteGuard(handler.AuthorizeComputerVersionRoute)
	if reattached := registry.ReattachManagedVMs(context.Background(), handler.AuthorizeComputerVersionRoute); reattached > 0 {
		log.Printf("vmctl: route-authorized reattach adopted %d managed VM(s)", reattached)
	}
	if idleSweeperEnabled {
		registry.StartIdleSweeper(context.Background(), idleSweepInterval, handler.AuthorizeComputerVersionRoute)
		log.Printf("vmctl: route-gated idle sweeper interval set to %s", idleSweepInterval)
	}
	if cacheReturn.NeedsIdleLoop() {
		registry.StartGuestCacheReturnLoop(context.Background(), idleSweepInterval)
		log.Printf("vmctl: guest cache return idle loop interval set to %s", idleSweepInterval)
	}
	// Deferred deploy refreshes are retried until each computer takes the
	// update (docs/problems/always-busy-computer-never-updates-2026-10-15.md).
	// The pending set lives beside the ownership state so a vmctl restart
	// keeps it.
	{
		pendingPath := ""
		stateDir := strings.TrimSpace(os.Getenv("VM_STATE_DIR"))
		if ownershipPath := strings.TrimSpace(os.Getenv("VMCTL_OWNERSHIP_PATH")); ownershipPath != "" {
			stateDir = filepath.Dir(ownershipPath)
		}
		if stateDir != "" {
			pendingPath = filepath.Join(stateDir, "deploy-refresh-pending.json")
		}
		if err := handler.SetDeployRefreshPendingPath(pendingPath); err != nil {
			log.Printf("vmctl: deploy refresh pending set not loaded (starting empty): %v", err)
		}
		// D2 S4: a computer resumed from a snapshot onto an older guest release
		// is updated at its next idle moment, once its user has been away this
		// long (default 2m).
		behindIdle := vmctl.DefaultBehindRefreshIdle
		if v := strings.TrimSpace(os.Getenv("VMCTL_BEHIND_REFRESH_IDLE")); v != "" {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				behindIdle = d
			} else {
				log.Printf("vmctl: VMCTL_BEHIND_REFRESH_IDLE=%q is not a positive duration; using %s", v, behindIdle)
			}
		}
		handler.SetBehindRefresh(behindIdle, nil)
		handler.StartDeployRefreshRetry(context.Background(), vmctl.DeployRefreshRetryInterval)
		log.Printf("vmctl: deploy refresh retry every %s (pending=%q)", vmctl.DeployRefreshRetryInterval, pendingPath)
	}
	// The booted-vs-activated kernel lag goes to the journal at start and
	// daily (docs/problems/host-kernel-216-days-unpatched-2026-10-15.md).
	vmctl.StartHostBootStateLog(context.Background(), vmctl.HostBootStateLogInterval, log.Printf)
	// Store compaction on host disk pressure (docs/problems/node-b-disk-
	// fills-from-the-paper-data-image-2026-10-22.md): off unless
	// VMCTL_STORE_COMPACT_PRESSURE is observe or on. The attempts record
	// lives beside the ownership state so a vmctl restart keeps the gap.
	{
		stateDir := strings.TrimSpace(os.Getenv("VM_STATE_DIR"))
		if ownershipPath := strings.TrimSpace(os.Getenv("VMCTL_OWNERSHIP_PATH")); ownershipPath != "" {
			stateDir = filepath.Dir(ownershipPath)
		}
		cfg := vmctl.StoreCompactPressureConfigFromEnv(os.Getenv)
		attemptsPath := vmctl.StoreCompactAttemptsPath(stateDir)
		if err := handler.SetStoreCompactPressure(cfg, attemptsPath); err != nil {
			log.Printf("vmctl: store compaction pressure refuses every pressure compaction until the next timer or by-hand compaction rewrites the record: %v", err)
		}
		handler.StartStoreCompactPressure(context.Background())
		log.Printf("vmctl: store compaction pressure mode=%s interval=%s host_free_below=%d fill_fraction=%.2f min_image=%d min_gap=%s attempts=%q",
			cfg.Mode, cfg.Interval, cfg.HostFreeBelow, cfg.FillFraction, cfg.MinImageBytes, cfg.MinGap, attemptsPath)
	}
	// The pressure monitor samples host pressure and VM memory; under the
	// sustained signal it also runs the gated reclaim. Off with reclaim off.
	registry.StartPressureMonitor(context.Background(), handler.AuthorizeComputerVersionRoute)
	startUniversalWirePlatformComputer(registry, handler.AuthorizeComputerVersionRoute)

	s := server.NewServer("vmctl", port)
	vmctl.RegisterRoutes(s, handler)

	if socketPath := strings.TrimSpace(os.Getenv("VMCTL_AUTOPUTER_PROXY_SOCK")); socketPath != "" {
		if err := s.SetUnixSocket(socketPath); err != nil {
			log.Fatalf("vmctl: set unix socket %s: %v", socketPath, err)
		}
		log.Printf("vmctl: autoputer proxy UDS listener on %s", socketPath)
	}

	log.Printf("vmctl: ownership registry initialized (computer_url_base=%s)", autoputerURLBase)
	s.Start()
}

// vmManagerAdapter adapts the vmmanager.Manager to the vmctl.VMManager
// interface. This adapter translates between the vmctl ownership types
// and the vmmanager VM lifecycle types.
type vmManagerAdapter struct {
	mgr *vmmanager.Manager
}

func (a *vmManagerAdapter) ReserveBootEpoch(vmID string, minimum int64) (int64, error) {
	return a.mgr.ReserveBootEpoch(vmID, minimum)
}

func (a *vmManagerAdapter) BootVM(cfg vmctl.VMManagerConfig) (*vmctl.VMInstanceInfo, error) {
	inst, err := a.mgr.BootVM(toManagerVMConfig(cfg))
	if err != nil {
		return nil, err
	}
	return toVMInstanceInfo(inst), nil
}

func toManagerVMConfig(cfg vmctl.VMManagerConfig) vmmanager.VMConfig {
	return vmmanager.VMConfig{
		VMID:                       cfg.VMID,
		ComputerID:                 cfg.ComputerID,
		RealizationID:              cfg.RealizationID,
		Epoch:                      cfg.Epoch,
		KernelImagePath:            cfg.KernelImagePath,
		InitrdPath:                 cfg.InitrdPath,
		RootfsPath:                 cfg.RootfsPath,
		StoreDiskPath:              cfg.StoreDiskPath,
		KernelParams:               cfg.KernelParams,
		GuestPort:                  cfg.GuestPort,
		MachineCPUCount:            cfg.MachineCPUCount,
		MachineMemSizeMib:          cfg.MachineMemSizeMib,
		PersistentDir:              cfg.PersistentDir,
		GatewayToken:               cfg.GatewayToken,
		ComputerCredentialEnvelope: cfg.ComputerCredentialEnvelope,
		ComputerKind:               cfg.ComputerKind,
		OwnerID:                    cfg.OwnerID,
		DesktopID:                  cfg.DesktopID,
		MaintenanceHold:            cfg.MaintenanceHold,
		RecoveryReplayOnly:         cfg.RecoveryReplayOnly,
		BootKind:                   cfg.BootKind,
		OfflineMaintenance:         toManagerOfflineMaintenance(cfg.OfflineMaintenance),
		PostMaintenanceCredential:  cfg.PostMaintenanceCredential,
		WarmnessClass:              cfg.WarmnessClass,
		NewComputer:                cfg.NewComputer,
	}
}

// toManagerOfflineMaintenance maps vmctl's unbootable sentinel to the
// manager's, so a hook that could not restore its backup stops the boot.
func toManagerOfflineMaintenance(hook func(string) error) func(string) error {
	if hook == nil {
		return nil
	}
	return func(dataImagePath string) error {
		err := hook(dataImagePath)
		if errors.Is(err, vmctl.ErrOfflineMaintenanceUnbootable) {
			return fmt.Errorf("%w: %v", vmmanager.ErrOfflineMaintenanceUnbootable, err)
		}
		return err
	}
}

func toVMInstanceInfo(inst *vmmanager.VMInstance) *vmctl.VMInstanceInfo {
	if inst == nil {
		return nil
	}
	return &vmctl.VMInstanceInfo{
		HostURL:         inst.HostURL,
		Epoch:           inst.Config.Epoch,
		Healthy:         inst.Healthy,
		State:           string(inst.State),
		StartedAt:       inst.StartedAt,
		LastHealthCheck: inst.LastHealthCheck,
		LastHealthyAt:   inst.LastHealthyAt,
		GuestRelease:    inst.Config.GuestRelease,
		Behind:          inst.Behind,
	}
}

func (a *vmManagerAdapter) StopVM(vmID string) error {
	return a.mgr.StopVM(vmID)
}

func (a *vmManagerAdapter) HibernateVM(vmID string) error {
	return a.mgr.HibernateVM(vmID)
}

// HibernateVMWithCacheReturn is HibernateVM with the D6 cache return step.
func (a *vmManagerAdapter) HibernateVMWithCacheReturn(vmID string, opt vmctl.HibernateCacheReturn) error {
	return a.mgr.HibernateVMWithOptions(vmID, vmmanager.HibernateOptions{
		CacheReturn: &vmmanager.CacheReturnOption{Scope: opt.Scope, DryRun: opt.DryRun},
	})
}

func (a *vmManagerAdapter) ResumeVM(vmID string) (*vmctl.VMInstanceInfo, error) {
	inst, err := a.mgr.ResumeVM(vmID)
	if err != nil {
		return nil, err
	}
	return toVMInstanceInfo(inst), nil
}

func (a *vmManagerAdapter) ReattachVM(vmID, hostURL string, epoch int64) (*vmctl.VMInstanceInfo, error) {
	inst, err := a.mgr.ReattachVM(vmID, hostURL, epoch)
	if err != nil {
		return nil, err
	}
	return toVMInstanceInfo(inst), nil
}

func (a *vmManagerAdapter) ReattachVMWithConfig(vmID, hostURL string, epoch int64, cfg vmctl.VMManagerConfig) (*vmctl.VMInstanceInfo, error) {
	inst, err := a.mgr.ReattachVMWithConfig(vmID, hostURL, epoch, toManagerVMConfig(cfg))
	if err != nil {
		return nil, err
	}
	return toVMInstanceInfo(inst), nil
}

func (a *vmManagerAdapter) RecoverVM(vmID string, cfg vmctl.VMManagerConfig) (*vmctl.VMInstanceInfo, error) {
	mcfg := toManagerVMConfig(cfg)
	if mcfg.BootKind == "" {
		mcfg.BootKind = "recover"
	}
	inst, err := a.mgr.RecoverVMWithConfig(vmID, mcfg)
	if err != nil {
		return nil, err
	}
	return toVMInstanceInfo(inst), nil
}

func (a *vmManagerAdapter) RefreshVM(vmID string, cfg vmctl.VMManagerConfig) (*vmctl.VMInstanceInfo, error) {
	mcfg := toManagerVMConfig(cfg)
	if mcfg.BootKind == "" {
		mcfg.BootKind = "refresh"
	}
	inst, err := a.mgr.RefreshVMWithConfig(vmID, mcfg)
	if err != nil {
		return nil, err
	}
	return toVMInstanceInfo(inst), nil
}

func (a *vmManagerAdapter) DestroyVMState(vmID string) error {
	return a.mgr.DestroyVMState(vmID)
}

func (a *vmManagerAdapter) GetVM(vmID string) *vmctl.VMInstanceInfo {
	inst := a.mgr.GetVM(vmID)
	return toVMInstanceInfo(inst)
}

// LiveFirecrackerVMs, ReapUnmanagedVM and SetVMOOMScoreAdj implement the
// vmctl host-capacity process controller.
func (a *vmManagerAdapter) LiveFirecrackerVMs() map[string][]int { return a.mgr.LiveFirecrackerVMs() }

func (a *vmManagerAdapter) ReapUnmanagedVM(vmID string) error { return a.mgr.ReapUnmanagedVM(vmID) }

func (a *vmManagerAdapter) SetVMOOMScoreAdj(vmID string, adj int) error {
	return a.mgr.SetVMOOMScoreAdj(vmID, adj)
}

// VMResidentBytes implements vmctl's VM memory reader (measured admission).
func (a *vmManagerAdapter) VMResidentBytes() map[string]uint64 { return a.mgr.VMResidentBytes() }

// GuestMemoryStats serves vmctl's read-only balloon statistics route
// (internal/vmctl/balloon_stats.go).
func (a *vmManagerAdapter) GuestMemoryStats(vmID string) (vmctl.GuestMemoryStats, error) {
	st, err := a.mgr.GuestMemoryStats(vmID)
	if err != nil {
		var se *vmmanager.GuestMemoryStatsError
		if errors.As(err, &se) {
			return vmctl.GuestMemoryStats{}, &vmctl.GuestMemoryStatsError{VMID: se.VMID, Reason: se.Reason, Detail: se.Detail}
		}
		return vmctl.GuestMemoryStats{}, err
	}
	s := st.Stats
	return vmctl.GuestMemoryStats{
		VMID: st.VMID, StatsPollingIntervalS: st.StatsPollingIntervalS, ObservedAt: st.ObservedAt,
		Stats: vmctl.BalloonStatistics{
			TargetPages: s.TargetPages, ActualPages: s.ActualPages, TargetMiB: s.TargetMiB, ActualMiB: s.ActualMiB,
			SwapIn: s.SwapIn, SwapOut: s.SwapOut, MajorFaults: s.MajorFaults, MinorFaults: s.MinorFaults,
			FreeMemory: s.FreeMemory, TotalMemory: s.TotalMemory, AvailableMemory: s.AvailableMemory,
			DiskCaches: s.DiskCaches, HugetlbAllocations: s.HugetlbAllocations, HugetlbFailures: s.HugetlbFailures,
		},
	}, nil
}

func (a *vmManagerAdapter) CheckHealth(vmID string) (bool, error) {
	return a.mgr.CheckHealth(vmID)
}

func (a *vmManagerAdapter) ReadGatewayToken(vmID string) (string, error) {
	return a.mgr.ReadGatewayToken(vmID)
}

// BootTimeline implements the vmctl bootTimelineReader capability: it returns
// the VM's merged host+guest boot receipt as raw JSON for the internal
// endpoint to stream verbatim.
func (a *vmManagerAdapter) BootTimeline(vmID string) (json.RawMessage, error) {
	tl, err := a.mgr.BootTimelineForVM(vmID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(tl)
}

// ResumeVMFromSnapshot implements the vmctl snapshotResumer capability (S3):
// it wakes a hibernated VM from its memory snapshot, delivering the fresh
// credential issueCredential mints, or reports why it could not so vmctl
// cold boots instead.
func (a *vmManagerAdapter) ResumeVMFromSnapshot(vmID string, cfg vmctl.VMManagerConfig, issueCredential func() (string, error)) (*vmctl.VMInstanceInfo, error) {
	inst, err := a.mgr.ResumeVMFromSnapshot(vmID, toManagerVMConfig(cfg), issueCredential)
	if err != nil {
		return nil, err
	}
	return toVMInstanceInfo(inst), nil
}

// DiscardSnapshot implements the vmctl snapshotDiscarder capability.
func (a *vmManagerAdapter) DiscardSnapshot(vmID, reason string) bool {
	return a.mgr.DiscardSnapshot(vmID, reason)
}

// SweepOrphanSnapshots implements the vmctl snapshotDiscarder capability.
func (a *vmManagerAdapter) SweepOrphanSnapshots(keep func(vmID string) bool) []string {
	return a.mgr.SweepOrphanSnapshots(keep)
}

// GuestReleases implements the vmctl guestReleaseReader capability: the
// releases live VMs run and unconsumed snapshots need (D2 retention).
func (a *vmManagerAdapter) GuestReleases() (json.RawMessage, error) {
	return json.Marshal(a.mgr.GuestReleaseUsage())
}

// GuestReleaseHealth implements the vmctl guestReleaseReader capability.
// CurrentGuestReleaseInfo implements the vmctl guestReleaseReader capability.
func (a *vmManagerAdapter) CurrentGuestReleaseInfo() (string, string) {
	return a.mgr.CurrentGuestReleaseInfo()
}

func (a *vmManagerAdapter) GuestReleaseHealth() (string, map[string]vmctl.GuestReleaseVMStatus) {
	current, states := a.mgr.GuestReleaseHealth()
	out := make(map[string]vmctl.GuestReleaseVMStatus, len(states))
	for vmID, st := range states {
		out[vmID] = vmctl.GuestReleaseVMStatus{GuestRelease: st.GuestRelease, Behind: st.Behind}
	}
	return current, out
}

// HibernateTimeline implements the vmctl hibernateTimelineReader capability.
func (a *vmManagerAdapter) HibernateTimeline(vmID string) (json.RawMessage, error) {
	tl, err := a.mgr.HibernateTimelineForVM(vmID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(tl)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// defaultLegacyGuestIdleTimeout is today's idle timeout, 30 minutes.
const defaultLegacyGuestIdleTimeout = 30 * time.Minute

// legacyGuestIdleTimeoutFromEnv reads VMCTL_IDLE_TIMEOUT_LEGACY_GUEST: how
// long a guest that does not report live_capsules must be idle before the
// idle sweep, pressure reclaim or make-room hibernates it (density D3a).
// Unset or malformed is 30 minutes, so lowering VMCTL_IDLE_TIMEOUT never
// lowers it for a guest that could be holding a parked capsule. A value
// below VMCTL_IDLE_TIMEOUT is raised to it: the legacy window protects a
// guest from the short timer and is never shorter than that timer.
func legacyGuestIdleTimeoutFromEnv() time.Duration {
	d := defaultLegacyGuestIdleTimeout
	if v := os.Getenv("VMCTL_IDLE_TIMEOUT_LEGACY_GUEST"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil && parsed > 0 {
			d = parsed
		}
	}
	if v := os.Getenv("VMCTL_IDLE_TIMEOUT"); v != "" {
		if idle, err := time.ParseDuration(v); err == nil && idle > d {
			d = idle
		}
	}
	return d
}

func pressureReclaimConfigFromEnv() (vmctl.PressureReclaimConfig, bool) {
	mode := os.Getenv("VMCTL_PRESSURE_RECLAIM_MODE")
	if strings.TrimSpace(mode) == "" {
		return vmctl.PressureReclaimConfig{}, false
	}
	cfg := vmctl.DefaultPressureReclaimConfig()
	cfg.Mode = mode
	if v := os.Getenv("VMCTL_PRESSURE_RECLAIM_MIN_IDLE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.MinIdle = d
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_MIB"); v != "" {
		if mib, err := strconv.ParseUint(v, 10, 64); err == nil {
			cfg.MinMemoryAvailableBytes = mib * 1024 * 1024
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_PERCENT"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MinMemoryAvailablePercent = pct
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_MIN_STATE_DIR_AVAILABLE_MIB"); v != "" {
		if mib, err := strconv.ParseUint(v, 10, 64); err == nil {
			cfg.MinStateDirAvailableBytes = mib * 1024 * 1024
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_MIN_STATE_DIR_AVAILABLE_PERCENT"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MinStateDirAvailablePercent = pct
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_MAX_MEMORY_SOME_AVG10"); v != "" {
		if avg, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MaxMemorySomeAvg10 = avg
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_MAX_CPU_SOME_AVG10"); v != "" {
		if avg, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MaxCPUSomeAvg10 = avg
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_MAX_IO_SOME_AVG10"); v != "" {
		if avg, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MaxIOSomeAvg10 = avg
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_RECLAIM_MAX_CANDIDATES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxCandidates = n
		}
	}
	if v := os.Getenv("VMCTL_STALE_STATE_MIN_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.StaleStateMinAge = d
		}
	}
	if v := os.Getenv("VMCTL_STALE_STATE_MAX_DELETES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxStateDeletes = n
		}
	}
	if v := os.Getenv("VM_STATE_DIR"); v != "" {
		cfg.StateDir = v
	}
	// Sustained signal (default) or instant (rollback: today's single
	// avg10 sample decides, up to MaxCandidates per pass).
	if v := os.Getenv("VMCTL_PRESSURE_RECLAIM_SIGNAL"); v != "" {
		cfg.Signal = v
	}
	if v := os.Getenv("VMCTL_PRESSURE_MAX_MEMORY_SOME_AVG60"); v != "" {
		if avg, err := strconv.ParseFloat(v, 64); err == nil && avg > 0 {
			cfg.MaxMemorySomeAvg60 = avg
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_SUSTAINED_SAMPLES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SustainedSamples = n
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_SAMPLE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.SampleInterval = d
		}
	}
	if v := os.Getenv("VMCTL_PRESSURE_RECLAIM_COOLDOWN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.Cooldown = d
		}
	}
	return cfg, true
}

// hostAdmissionConfigFromEnv reads start admission. Measured (default)
// admits by each computer's last measured memory with reservations for
// starts in flight; guest-size is the rollback (floor + the full guest).
func hostAdmissionConfigFromEnv() vmctl.HostAdmissionConfig {
	cfg := vmctl.DefaultHostAdmissionConfig()
	if v := os.Getenv("VMCTL_ADMISSION_MODE"); v != "" {
		cfg.Mode = v
	}
	mib := func(key string, dst *uint64) {
		if v := os.Getenv(key); v != "" {
			if n, err := strconv.ParseUint(v, 10, 64); err == nil {
				*dst = n * 1024 * 1024
			}
		}
	}
	mib("VMCTL_ADMISSION_DEFAULT_NEED_MIB", &cfg.DefaultNeedBytes)
	mib("VMCTL_ADMISSION_MIN_NEED_MIB", &cfg.MinNeedBytes)
	mib("VMCTL_ADMISSION_HEADROOM_MIB", &cfg.HeadroomBytes)
	if v := os.Getenv("VMCTL_ADMISSION_SETTLE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.Settle = d
		}
	}
	if v := os.Getenv("VMCTL_ADMISSION_INFLIGHT_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.InFlightTTL = d
		}
	}
	// Density D4: peak (default) charges a start the last peak RSS; wake
	// charges the RSS its last start settled at. Unknown values mean peak.
	if strings.EqualFold(strings.TrimSpace(os.Getenv("VMCTL_ADMISSION_NEED")), vmctl.HostAdmissionNeedWake) {
		cfg.NeedMode = vmctl.HostAdmissionNeedWake
	} else {
		cfg.NeedMode = vmctl.HostAdmissionNeedPeak
	}
	// Density D1: a refused start makes room (active) or is refused as
	// before (off, the default and the rollback). Only an exact enabling
	// word turns it on; the bounds are capped (review 302 F3, F5).
	if v := os.Getenv("VMCTL_ADMISSION_MAKE_ROOM"); v != "" {
		cfg.MakeRoom = vmctl.NormalizeMakeRoomMode(v)
	}
	if v := os.Getenv("VMCTL_ADMISSION_MAKE_ROOM_MAX_VICTIMS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MakeRoomMaxVictims = min(n, vmctl.MaxMakeRoomVictims)
		}
	}
	if v := os.Getenv("VMCTL_ADMISSION_MAKE_ROOM_SETTLE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.MakeRoomSettleWait = min(d, vmctl.MaxMakeRoomSettleWait)
		}
	}
	return cfg
}

// guestCacheReturnConfigFromEnv reads the D6 switch and its bounds. Only the
// exact words observe and active enable it; rollback is off.
func guestCacheReturnConfigFromEnv() vmctl.GuestCacheReturnConfig {
	return vmctl.GuestCacheReturnConfigFromValues(vmctl.GuestCacheReturnValues{
		Mode:           os.Getenv("VMCTL_GUEST_CACHE_RETURN"),
		Only:           os.Getenv("VMCTL_GUEST_CACHE_RETURN_ONLY"),
		Triggers:       os.Getenv("VMCTL_GUEST_CACHE_RETURN_TRIGGERS"),
		Scope:          os.Getenv("VMCTL_GUEST_CACHE_RETURN_SCOPE"),
		Idle:           os.Getenv("VMCTL_GUEST_CACHE_RETURN_IDLE"),
		PressureIdle:   os.Getenv("VMCTL_GUEST_CACHE_RETURN_PRESSURE_IDLE"),
		PressureBudget: os.Getenv("VMCTL_GUEST_CACHE_RETURN_PRESSURE_BUDGET"),
	})
}

func retentionPruneConfigFromEnv() (vmctl.RetentionPruneConfig, bool) {
	mode := os.Getenv("VMCTL_RETENTION_PRUNE_MODE")
	if strings.TrimSpace(mode) == "" {
		return vmctl.RetentionPruneConfig{}, false
	}
	cfg := vmctl.DefaultRetentionPruneConfig()
	cfg.Mode = mode
	if v := os.Getenv("VM_STATE_DIR"); v != "" {
		cfg.StateDir = v
	}
	if v := os.Getenv("VMCTL_RETENTION_STATE_DIR"); v != "" {
		cfg.StateDir = v
	}
	cfg.AuthDBPath = strings.TrimSpace(os.Getenv("VMCTL_RETENTION_AUTH_DB_PATH"))
	cfg.EphemeralEmailDomains = splitEnvList(os.Getenv("VMCTL_RETENTION_EPHEMERAL_EMAIL_DOMAINS"))
	cfg.EphemeralUserIDPrefixes = splitEnvList(os.Getenv("VMCTL_RETENTION_EPHEMERAL_USER_PREFIXES"))
	if v := os.Getenv("VMCTL_RETENTION_ORPHAN_MIN_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.OrphanMinAge = d
		}
	}
	if v := os.Getenv("VMCTL_RETENTION_EPHEMERAL_MIN_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.EphemeralMinAge = d
		}
	}
	if v := os.Getenv("VMCTL_RETENTION_MAX_DELETES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxDeletes = n
		}
	}
	if v := os.Getenv("VMCTL_RETENTION_MAX_BYTES_MIB"); v != "" {
		if mib, err := strconv.ParseInt(v, 10, 64); err == nil && mib > 0 {
			cfg.MaxBytes = mib * 1024 * 1024
		}
	}
	retentionRulesFromEnv(&cfg, "VMCTL_RETENTION_")
	return cfg, true
}

// retentionRulesFromEnv reads the test_class_idle and quarantine_age_out
// switches (default off) and their minimum ages under prefix, and the ids
// every per-computer selector names (VM_*_ONLY, VMCTL_*_ONLY,
// CHOIR_*_COMPUTERS, CHOIR_*_ONLY), which neither rule touches.
func retentionRulesFromEnv(cfg *vmctl.RetentionPruneConfig, prefix string) {
	cfg.TestIdleMode = os.Getenv(prefix + "TEST_IDLE_MODE")
	cfg.QuarantineMode = os.Getenv(prefix + "QUARANTINE_MODE")
	if v := os.Getenv(prefix + "TEST_IDLE_MIN_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.TestIdleMinAge = d
		}
	}
	if v := os.Getenv(prefix + "QUARANTINE_MIN_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.QuarantineMinAge = d
		}
	}
	cfg.SelectorNamed = vmctl.RetentionSelectorNamedFromEnviron(os.Environ())
}

func retentionShadowPruneConfigFromEnv() (vmctl.RetentionPruneConfig, bool) {
	mode := os.Getenv("VMCTL_RETENTION_SHADOW_PRUNE_MODE")
	if strings.TrimSpace(mode) == "" {
		return vmctl.RetentionPruneConfig{}, false
	}
	cfg := vmctl.DefaultRetentionPruneConfig()
	cfg.Mode = mode
	if v := os.Getenv("VM_STATE_DIR"); v != "" {
		cfg.StateDir = v
	}
	if v := os.Getenv("VMCTL_RETENTION_SHADOW_STATE_DIR"); v != "" {
		cfg.StateDir = v
	}
	cfg.AuthDBPath = strings.TrimSpace(os.Getenv("VMCTL_RETENTION_SHADOW_AUTH_DB_PATH"))
	cfg.EphemeralEmailDomains = splitEnvList(os.Getenv("VMCTL_RETENTION_SHADOW_EPHEMERAL_EMAIL_DOMAINS"))
	cfg.EphemeralUserIDPrefixes = splitEnvList(os.Getenv("VMCTL_RETENTION_SHADOW_EPHEMERAL_USER_PREFIXES"))
	if v := os.Getenv("VMCTL_RETENTION_SHADOW_ORPHAN_MIN_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.OrphanMinAge = d
		}
	}
	if v := os.Getenv("VMCTL_RETENTION_SHADOW_EPHEMERAL_MIN_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.EphemeralMinAge = d
		}
	}
	if v := os.Getenv("VMCTL_RETENTION_SHADOW_MAX_DELETES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxDeletes = n
		}
	}
	if v := os.Getenv("VMCTL_RETENTION_SHADOW_MAX_BYTES_MIB"); v != "" {
		if mib, err := strconv.ParseInt(v, 10, 64); err == nil && mib > 0 {
			cfg.MaxBytes = mib * 1024 * 1024
		}
	}
	retentionRulesFromEnv(&cfg, "VMCTL_RETENTION_SHADOW_")
	return cfg, true
}

func splitEnvList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// warmnessPolicyConfigFromEnv reads the keepalive mode from the environment
// and the computer tiers from the embedded tier file
// (internal/computertiers). The tiers are not configurable on the host: a
// tier changes by a reviewed commit, and a malformed file fails vmctl's
// start rather than dropping every computer to the free tier.
func warmnessPolicyConfigFromEnv() (vmctl.WarmnessPolicyConfig, error) {
	cfg := vmctl.DefaultWarmnessPolicyConfig()
	cfg.PrimaryKeepaliveMode = envOr("VMCTL_PRIMARY_KEEPALIVE_MODE", vmctl.PrimaryKeepaliveModeUnderCapacity)
	tiers, err := computertiers.Load()
	if err != nil {
		return cfg, err
	}
	for _, id := range tiers.PaidUserIDs() {
		cfg.PaidUserIDs[id] = true
	}
	for _, id := range tiers.PlatformComputerIDs() {
		cfg.PlatformComputerIDs[id] = true
	}
	cfg.TestComputerIDs = map[string]bool{}
	for _, id := range tiers.TestComputerIDs() {
		cfg.TestComputerIDs[id] = true
	}
	return cfg, nil
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch v {
	case "0", "false", "FALSE", "no", "NO":
		return false
	case "1", "true", "TRUE", "yes", "YES":
		return true
	default:
		return fallback
	}
}

// platformEnsureRetryInterval spaces the startup ensure's retries after a
// transient refusal.
const platformEnsureRetryInterval = 15 * time.Second

func startUniversalWirePlatformComputer(registry *vmctl.OwnershipRegistry, guard vmctl.ComputerVersionRouteGuard) {
	if !envBool("VMCTL_PLATFORM_WIRE_ENABLED", false) {
		return
	}
	go func() {
		timeout := 10 * time.Minute
		if raw := strings.TrimSpace(os.Getenv("VMCTL_PLATFORM_WIRE_BOOT_TIMEOUT")); raw != "" {
			if d, err := time.ParseDuration(raw); err == nil && d > 0 {
				timeout = d
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if guard == nil {
			log.Printf("vmctl: universal wire platform computer refused: ComputerVersion route guard is unavailable")
			return
		}
		if err := guard(ctx, vmctl.UniversalWirePlatformOwnerID, vmctl.UniversalWirePlatformDesktopID); err != nil {
			log.Printf("vmctl: universal wire platform computer refused: %v", err)
			return
		}
		// A refusal that spawned nothing (an interrupted start's close still
		// open, a live guest still reattaching, host memory pressure, another
		// start in progress) clears by itself: retry it inside the timeout
		// instead of leaving the platform computer down until a request
		// (wake-during-deploy-vmctl-restart-orphans-the-vm-2026-10-22.md,
		// review round 2, F3). Any other error is final, as before.
		for {
			err := registry.EnsureUniversalWirePlatformComputer(ctx)
			if err == nil {
				return
			}
			if !vmctl.IsTransientStartRefusal(err) {
				log.Printf("vmctl: universal wire platform computer: %v", err)
				return
			}
			log.Printf("vmctl: universal wire platform computer refused for now, retrying in %s: %v", platformEnsureRetryInterval, err)
			select {
			case <-ctx.Done():
				log.Printf("vmctl: universal wire platform computer: gave up after %s: %v", timeout, err)
				return
			case <-time.After(platformEnsureRetryInterval):
			}
		}
	}()
}
