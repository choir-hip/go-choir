package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"github.com/yusefmosiah/go-choir/internal/vmmanager"
)

func main() {
	port := server.PortFromEnv("VMCTL_PORT", "8083")

	// The sandbox URL base is where VM-backed sandbox runtimes are
	// reachable. In host-process mode this is the local sandbox.
	// In production with Firecracker, vmctl will return per-VM URLs.
	sandboxURLBase := envOr("VMCTL_SANDBOX_URL_BASE", "http://127.0.0.1:8085")

	registry := vmctl.NewOwnershipRegistry(sandboxURLBase)
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

	// Configure the gateway URL for issuing sandbox credentials to VM guests.
	// When Firecracker VMs are active, each guest sandbox needs a token to
	// authenticate to the host-side gateway for provider access.
	if gwURL := os.Getenv("VMCTL_GATEWAY_URL"); gwURL != "" {
		registry.SetGatewayURL(gwURL)
		log.Printf("vmctl: gateway URL configured for VM token issuance")
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
	if v := os.Getenv("VMCTL_IDLE_SWEEP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			idleSweepInterval = d
		}
	}
	if cfg, ok := pressureReclaimConfigFromEnv(); ok {
		registry.SetPressureReclaimConfig(cfg)
		idleSweeperEnabled = true
		log.Printf("vmctl: pressure reclaim mode=%s min_idle=%s max_candidates=%d", cfg.Mode, cfg.MinIdle, cfg.MaxCandidates)
	}
	if cfg, ok := retentionPruneConfigFromEnv(); ok {
		registry.SetRetentionPruneConfig(cfg)
		idleSweeperEnabled = true
		log.Printf("vmctl: retention prune mode=%s ephemeral_domains=%s max_deletes=%d max_bytes=%d", cfg.Mode, strings.Join(cfg.EphemeralEmailDomains, ","), cfg.MaxDeletes, cfg.MaxBytes)
	}
	if cfg, ok := retentionShadowPruneConfigFromEnv(); ok {
		registry.SetRetentionShadowPruneConfig(cfg)
		log.Printf("vmctl: retention shadow prune mode=dry-run requested_mode=%s ephemeral_domains=%s ephemeral_user_prefixes=%s max_deletes=%d max_bytes=%d", cfg.Mode, strings.Join(cfg.EphemeralEmailDomains, ","), strings.Join(cfg.EphemeralUserIDPrefixes, ","), cfg.MaxDeletes, cfg.MaxBytes)
	}
	warmnessPolicy := warmnessPolicyConfigFromEnv()
	registry.SetWarmnessPolicyConfig(warmnessPolicy)
	log.Printf("vmctl: warmness policy primary_keepalive_mode=%s always_on_user_count=%d", warmnessPolicy.PrimaryKeepaliveMode, len(warmnessPolicy.AlwaysOnUserIDs))
	if profile, ok := workerImageProfileFromEnv("VM_PLAYWRIGHT"); ok {
		registry.SetWorkerImageProfile("worker-playwright", profile)
		log.Printf("vmctl: worker-playwright image profile configured")
	}

	// Check if Firecracker is available on this host.
	// If so, create a VM manager for real Firecracker lifecycle management
	// and wire it to the ownership registry so that VM boot/stop/resume
	// operations are delegated to real Firecracker VMs (VAL-VM-010).
	// If not, vmctl can still run in host-process mode for local development,
	// but deployed environments should disable that fallback explicitly.
	if vmmanager.IsFirecrackerAvailable() {
		mgrCfg := vmmanager.LoadConfigFromEnv()
		if err := mgrCfg.Validate(); err != nil {
			log.Fatalf("vmctl: Firecracker config validation failed: %v", err)
		}
		mgr := vmmanager.NewManager(mgrCfg)
		mgr.Start()
		if envBool("VMCTL_STOP_MANAGED_ON_EXIT", true) {
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
		if !vmmanager.HostProcessFallbackEnabled() {
			log.Fatal("vmctl: Firecracker not available and host-process fallback is disabled")
		}
		log.Printf("vmctl: Firecracker not available, using host-process sandbox mode")
	}
	handler := vmctl.NewHandler(registry)
	handler.RequireRouteAuthority()
	if dsn := strings.TrimSpace(os.Getenv("VMCTL_ROUTE_DSN")); dsn != "" {
		routeDB, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("vmctl: open ComputerVersion route database: %v", err)
		}
		defer func() { _ = routeDB.Close() }()
		pingCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := routeDB.PingContext(pingCtx); err != nil {
			log.Fatalf("vmctl: ping ComputerVersion route database: %v", err)
		}
		artifactVerifier := computerversion.NewLocalArtifactContentVerifier(os.Getenv("VMCTL_ARTIFACTS_ROOT"))
		inputs := computerversion.NewSQLInputCatalog(routeDB, artifactVerifier)
		handler.SetImmutableArtifactOpener(artifactVerifier)
		if err := inputs.EnsureSchema(pingCtx); err != nil {
			log.Fatalf("vmctl: initialize immutable input catalog: %v", err)
		}
		ledger := routeledger.NewSQLLedger(routeDB, computerversion.VerifySQLInputsInTransition)
		if err := ledger.EnsureSchema(pingCtx); err != nil {
			log.Fatalf("vmctl: initialize ComputerVersion route ledger: %v", err)
		}
		authority, err := vmctl.NewRouteAuthority(ledger, inputs, ledger)
		if err != nil {
			log.Fatalf("vmctl: initialize ComputerVersion route authority: %v", err)
		}
		handler.SetRouteAuthority(authority)
		if blobRoot := strings.TrimSpace(os.Getenv("VMCTL_BASE_BLOB_ROOT")); blobRoot != "" {
			blobs, err := blob.OpenStore(blobRoot)
			if err != nil {
				log.Fatalf("vmctl: open immutable Base blob store: %v", err)
			}
			stateRoot := strings.TrimSpace(os.Getenv("VM_STATE_DIR"))
			materializer := computerversion.ProductionMaterializer{
				Inputs:    inputs,
				Artifacts: artifactVerifier,
				Blobs:     blobs,
				Disk:      diskinstantiation.Ext4Backend{WorkRoot: stateRoot},
				DiskPlan: diskinstantiation.Plan{
					DeviceID:     "data",
					LogicalBytes: 32 << 30,
					Filesystem:   diskinstantiation.FilesystemContract{Type: diskinstantiation.FilesystemExt4, Label: "choir-data", BlockSizeBytes: 4096},
					Allocation:   diskinstantiation.AllocationContract{Mode: diskinstantiation.AllocationSparse, MaxAllocatedBytes: 2 << 30, MinimumAvailableBytes: 2 << 30},
				},
				Launcher: vmctl.NewVMConstructionLauncher(registry, nil),
			}
			handler.SetConstructionService(materializer, computerversion.CapabilityManifest{
				Materializer: computerversion.ProductionMaterializerName,
				Substrate:    computerversion.VMManagerSubstrateFirecracker,
				Supported:    []computerversion.ObservationKind{computerversion.ObservationFileManifest, computerversion.ObservationBlobSet, computerversion.ObservationVMStateManifest},
			})
			log.Printf("vmctl: production ComputerVersion constructor configured")
		}
		log.Printf("vmctl: ComputerVersion route authority configured on the corpusd world-wire SQL server")
	} else {
		log.Printf("vmctl: ComputerVersion route authority unavailable (VMCTL_ROUTE_DSN is not configured)")
	}
	if reattached := registry.ReattachManagedVMs(context.Background(), handler.AuthorizeComputerVersionRoute); reattached > 0 {
		log.Printf("vmctl: route-authorized reattach adopted %d managed VM(s)", reattached)
	}
	if idleSweeperEnabled {
		registry.StartIdleSweeper(context.Background(), idleSweepInterval, handler.AuthorizeComputerVersionRoute)
		log.Printf("vmctl: route-gated idle sweeper interval set to %s", idleSweepInterval)
	}
	startUniversalWirePlatformComputer(registry, handler.AuthorizeComputerVersionRoute)
	if dir := strings.TrimSpace(os.Getenv("VMCTL_SANDBOX_PACKAGE_DIR")); dir != "" {
		handler.SetSandboxRuntimePackageDir(dir)
		log.Printf("vmctl: sandbox runtime package directory configured (%s)", dir)
	}

	s := server.NewServer("vmctl", port)
	vmctl.RegisterRoutes(s, handler)

	if socketPath := strings.TrimSpace(os.Getenv("VMCTL_SANDBOX_PROXY_SOCK")); socketPath != "" {
		if err := s.SetUnixSocket(socketPath); err != nil {
			log.Fatalf("vmctl: set unix socket %s: %v", socketPath, err)
		}
		log.Printf("vmctl: sandbox proxy UDS listener on %s", socketPath)
	}

	log.Printf("vmctl: ownership registry initialized (sandbox_url_base=%s)", sandboxURLBase)
	s.Start()
}

// vmManagerAdapter adapts the vmmanager.Manager to the vmctl.VMManager
// interface. This adapter translates between the vmctl ownership types
// and the vmmanager VM lifecycle types.
type vmManagerAdapter struct {
	mgr *vmmanager.Manager
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
		VMID:              cfg.VMID,
		KernelImagePath:   cfg.KernelImagePath,
		InitrdPath:        cfg.InitrdPath,
		RootfsPath:        cfg.RootfsPath,
		StoreDiskPath:     cfg.StoreDiskPath,
		KernelParams:      cfg.KernelParams,
		GuestPort:         cfg.GuestPort,
		MachineCPUCount:   cfg.MachineCPUCount,
		MachineMemSizeMib: cfg.MachineMemSizeMib,
		PersistentDir:     cfg.PersistentDir,
		SourceVMID:        cfg.SourceVMID,
		DataDevicePath:    cfg.DataDevicePath,
		GatewayToken:      cfg.GatewayToken,
		ComputerKind:      cfg.ComputerKind,
		OwnerID:           cfg.OwnerID,
		DesktopID:         cfg.DesktopID,
		WorkerID:          cfg.WorkerID,
		CandidateID:       cfg.CandidateID,
		CodeRef:           cfg.CodeRef,
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
	}
}

func workerImageProfileFromEnv(prefix string) (vmctl.VMImageProfile, bool) {
	profile := vmctl.VMImageProfile{
		KernelImagePath: strings.TrimSpace(os.Getenv(prefix + "_KERNEL_IMAGE")),
		InitrdPath:      strings.TrimSpace(os.Getenv(prefix + "_INITRD_IMAGE")),
		RootfsPath:      strings.TrimSpace(os.Getenv(prefix + "_ROOTFS_IMAGE")),
		StoreDiskPath:   strings.TrimSpace(os.Getenv(prefix + "_STORE_DISK_IMAGE")),
	}
	if paramsFile := strings.TrimSpace(os.Getenv(prefix + "_KERNEL_PARAMS_FILE")); paramsFile != "" {
		if data, err := os.ReadFile(paramsFile); err == nil {
			profile.KernelParams = strings.TrimSpace(string(data))
		} else {
			log.Printf("vmctl: could not read %s kernel params file %s: %v", prefix, paramsFile, err)
		}
	}
	if params := strings.TrimSpace(os.Getenv(prefix + "_KERNEL_PARAMS")); params != "" {
		profile.KernelParams = params
	}
	ok := profile.KernelImagePath != "" ||
		profile.InitrdPath != "" ||
		profile.RootfsPath != "" ||
		profile.StoreDiskPath != "" ||
		profile.KernelParams != ""
	if ok {
		if missing := profile.MissingRequiredFields(); len(missing) > 0 {
			log.Printf("vmctl: incomplete %s worker image profile ignored; missing %s", prefix, strings.Join(missing, ", "))
			return vmctl.VMImageProfile{}, false
		}
	}
	return profile, ok
}

func (a *vmManagerAdapter) StopVM(vmID string) error {
	return a.mgr.StopVM(vmID)
}

func (a *vmManagerAdapter) HibernateVM(vmID string) error {
	return a.mgr.HibernateVM(vmID)
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
	inst, err := a.mgr.RecoverVMWithConfig(vmID, toManagerVMConfig(cfg))
	if err != nil {
		return nil, err
	}
	return toVMInstanceInfo(inst), nil
}

func (a *vmManagerAdapter) RefreshVM(vmID string, cfg vmctl.VMManagerConfig) (*vmctl.VMInstanceInfo, error) {
	inst, err := a.mgr.RefreshVMWithConfig(vmID, toManagerVMConfig(cfg))
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

func (a *vmManagerAdapter) CheckHealth(vmID string) (bool, error) {
	return a.mgr.CheckHealth(vmID)
}

func (a *vmManagerAdapter) ReadGatewayToken(vmID string) (string, error) {
	return a.mgr.ReadGatewayToken(vmID)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
	return cfg, true
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
	return cfg, true
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

func warmnessPolicyConfigFromEnv() vmctl.WarmnessPolicyConfig {
	cfg := vmctl.DefaultWarmnessPolicyConfig()
	cfg.PrimaryKeepaliveMode = envOr("VMCTL_PRIMARY_KEEPALIVE_MODE", vmctl.PrimaryKeepaliveModeUnderCapacity)
	cfg.AlwaysOnUserIDs = map[string]bool{}
	for _, raw := range strings.Split(os.Getenv("VMCTL_ALWAYS_ON_USER_IDS"), ",") {
		userID := strings.TrimSpace(raw)
		if userID != "" {
			cfg.AlwaysOnUserIDs[userID] = true
		}
	}
	return cfg
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
		if err := registry.EnsureUniversalWirePlatformComputer(ctx); err != nil {
			log.Printf("vmctl: universal wire platform computer: %v", err)
		}
	}()
}
