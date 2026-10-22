// Package vmmanager provides Firecracker VM lifecycle management.
// See manager.go for the main implementation.
package vmmanager

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// LoadConfigFromEnv creates a ManagerConfig from VMCTL_* and VM_*
// environment variables. This is used by the vmctl service to configure
// the VM manager on Node B.
func LoadConfigFromEnv() ManagerConfig {
	cfg := DefaultManagerConfig()

	if v := os.Getenv("VM_FIRECRACKER_BIN"); v != "" {
		cfg.FirecrackerBinPath = v
	}
	if v := os.Getenv("VM_MKFS_EXT4_BIN"); v != "" {
		cfg.MkfsExt4Path = v
	}

	if v := os.Getenv("VM_KERNEL_IMAGE"); v != "" {
		cfg.KernelImagePath = v
	}

	if v := os.Getenv("VM_INITRD_IMAGE"); v != "" {
		cfg.InitrdPath = v
	}

	if v := os.Getenv("VM_ROOTFS_IMAGE"); v != "" {
		cfg.RootfsPath = v
	}

	if v := os.Getenv("VM_STORE_DISK_IMAGE"); v != "" {
		cfg.StoreDiskPath = v
	}

	if v := os.Getenv("VM_KERNEL_PARAMS"); v != "" {
		cfg.KernelParams = strings.TrimSpace(v)
	}
	if v := os.Getenv("VM_KERNEL_PARAMS_FILE"); v != "" {
		if data, err := os.ReadFile(v); err == nil {
			cfg.KernelParams = strings.TrimSpace(string(data))
		}
	}

	if v := os.Getenv("VM_GUEST_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.GuestPort = n
		}
	}

	if v := os.Getenv("VM_HOST_BASE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.HostBasePort = n
		}
	}

	if v := os.Getenv("VM_CPU_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MachineCPUCount = n
		}
	}

	if v := os.Getenv("VM_MEM_MIB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MachineMemSizeMib = n
		}
	}

	if v := os.Getenv("VM_STATE_DIR"); v != "" {
		cfg.StateDir = v
	}

	if v := os.Getenv("VM_HEALTH_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HealthCheckInterval = d
		}
	}

	if v := os.Getenv("VM_HEALTH_CHECK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HealthCheckTimeout = d
		}
	}

	if v := os.Getenv("VM_BOOT_READY_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.BootReadyTimeout = d
		}
	}

	if v := os.Getenv("VM_REPLAY_STALL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReplayStallTimeout = d
		}
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv("VM_BALLOON_FREE_PAGE_REPORTING"))) {
	case "1", "true", "on", "yes":
		cfg.BalloonFreePageReporting = true
	}
	// Guest data drive discard (data_discard.go). Needs Firecracker >= 1.17.0;
	// on an older binary the flag is ignored rather than failing every boot.
	cfg.DataDiscard = dataDiscardEnabledFromEnv(envFlagOn("VM_DATA_DISCARD"), cfg.FirecrackerBinPath)
	if envFlagOn("VM_DATA_DISCARD") && !cfg.DataDiscard {
		log.Printf("vmmanager: VM_DATA_DISCARD is on but the Firecracker binary does not report 1.17.0 or newer; data drive discard stays off for the pinned build")
	}
	cfg.DataDiscardOnly = parseComputerSelector(os.Getenv("VM_DATA_DISCARD_ONLY"))
	loadGuestReclaimFromEnv(&cfg)
	cfg.CapsuleTunnelEnforceOnly = parseComputerSelector(os.Getenv("VM_CAPSULE_TUNNEL_ENFORCE_ONLY"))
	cfg.BalloonStatsOnly = parseComputerSelector(os.Getenv("VM_BALLOON_STATS_ONLY"))
	if v := os.Getenv("VM_BALLOON_STATS_POLLING_INTERVAL_S"); v != "" {
		// Firecracker's field is a u16: refuse out-of-range values rather
		// than fail every boot.
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 && n <= 65535 {
			cfg.BalloonStatsPollingIntervalS = n
		}
	}
	// S3 snapshot hibernate (snapshot.go). Default kill: rollback is unsetting
	// VM_HIBERNATE_MODE.
	if v := strings.TrimSpace(os.Getenv("VM_HIBERNATE_MODE")); v != "" {
		cfg.HibernateMode = strings.ToLower(v)
	}
	if v := os.Getenv("VM_SNAPSHOT_BUDGET_GIB"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			cfg.SnapshotBudgetGiB = n
		}
	}
	// L3a Diff snapshots (snapshot_diff.go): full unless set to diff, and
	// then only for the computers VM_SNAPSHOT_DIFF_ONLY names.
	cfg.SnapshotType = normalizeSnapshotTypeConfig(os.Getenv("VM_SNAPSHOT_TYPE"))
	cfg.SnapshotDiffOnly = parseComputerSelector(os.Getenv("VM_SNAPSHOT_DIFF_ONLY"))
	if v := os.Getenv("VM_SNAPSHOT_MAX_AGE"); v != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil && d > 0 {
			cfg.SnapshotMaxAge = d
		}
	}

	// Snapshot vault (snapshot_vault.go). Off unless the dir is set; an
	// unusable value is kept as given so the probe refuses every capture
	// instead of the vault silently turning off.
	cfg.SnapshotVaultDir = strings.TrimSpace(os.Getenv("VM_SNAPSHOT_VAULT_DIR"))
	for _, id := range strings.Split(os.Getenv("VM_SNAPSHOT_VAULT_ONLY"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			cfg.SnapshotVaultOnly = append(cfg.SnapshotVaultOnly, id)
		}
	}
	cfg.SnapshotVaultAcceptAnyCrypt = envFlagOn("VM_SNAPSHOT_VAULT_ACCEPT_ANY_CRYPT")

	cfg.Jailer = loadJailerConfigFromEnv(cfg.FirecrackerBinPath)
	cfg.FirecrackerNext = loadFirecrackerNextFromEnv(cfg.FirecrackerBinPath, envFlagOn("VM_DATA_DISCARD"))
	cfg.ClaudeDelegate = loadClaudeDelegateConfig(os.Getenv)
	cfg.VisualCapture = loadVisualCaptureConfig(os.Getenv)
	cfg.CapsuleMirrors = loadCapsuleMirrorConfig(os.Getenv)
	cfg.HostFetch = loadHostFetchConfig(os.Getenv)
	cfg.EventTable = loadEventTableConfig(os.Getenv)

	// virtio-mem hotplug (memory_hotplug.go). Off by default; vmctl also
	// selects each computer, so this flag alone adds no device.
	cfg.MemoryHotplug = envFlagOn("VM_MEMORY_HOTPLUG")
	for name, dst := range map[string]*int{
		"VM_MEMORY_HOTPLUG_BLOCK_MIB": &cfg.MemoryHotplugBlockMiB,
		"VM_MEMORY_HOTPLUG_SLOT_MIB":  &cfg.MemoryHotplugSlotMiB,
	} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				*dst = n
			}
		}
	}

	if v := strings.TrimSpace(os.Getenv("VMCTL_GUEST_DNS_PORT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 65536 {
			cfg.GuestDNSPort = n
		} else {
			log.Printf("vmmanager: ignoring VMCTL_GUEST_DNS_PORT=%q: not a port", v)
		}
	}

	if v := os.Getenv("VM_SOURCE_SEED_REPO"); v != "" {
		cfg.SourceSeedRepoPath = strings.TrimSpace(v)
	} else if _, err := os.Stat("/opt/go-choir/.git"); err == nil {
		// Node B convention: the deployed source checkout lives at
		// /opt/go-choir (synced by the deploy pipeline). Seeding
		// files/Source/platform from it is what makes the engineering desk's
		// assignment preflight reachable on a virgin computer.
		cfg.SourceSeedRepoPath = "/opt/go-choir"
	}

	return cfg
}

// loadJailerConfigFromEnv reads the VMM jailer flag (slice 1). Rollback is
// VMCTL_JAILER=off (the default): new launches are direct again.
//
//	VMCTL_JAILER=off|on             run each Firecracker under the jailer
//	VMCTL_JAILER_ONLY=a,b           only these VM ids, computer ids, class:test or *
//	VM_JAILER_BIN                   jailer binary (default: beside firecracker)
//	VMCTL_JAILER_CHROOT_BASE        default <VM_STATE_DIR>/.jail
//	VMCTL_JAILER_UID_BASE/_COUNT    per-VM uid range (900000 + 4096)
//	VMCTL_JAILER_CGROUP_PARENT      VMs' parent cgroup (default: derived)
//	VMCTL_JAILER_PIDS_MAX           pids.max per VM (default 128; 0 omits)
//	VMCTL_JAILER_CPU_WEIGHT         cpu.weight per VM (default 100; 0 omits)
//	VMCTL_VMM_SPECULATION_BARRIERS  also force barriers on direct launches
func loadJailerConfigFromEnv(firecrackerBin string) JailerConfig {
	j := defaultJailerConfig()
	j.Enabled = envFlagOn("VMCTL_JAILER")
	j.Only = parseComputerSelector(os.Getenv("VMCTL_JAILER_ONLY"))
	j.BinPath = "jailer"
	if dir := filepath.Dir(firecrackerBin); dir != "." && dir != "" {
		j.BinPath = filepath.Join(dir, "jailer")
	}
	if v := strings.TrimSpace(os.Getenv("VM_JAILER_BIN")); v != "" {
		j.BinPath = v
	}
	if v := strings.TrimSpace(os.Getenv("VMCTL_JAILER_CHROOT_BASE")); v != "" {
		j.ChrootBase = filepath.Clean(v)
	}
	if v := strings.Trim(strings.TrimSpace(os.Getenv("VMCTL_JAILER_CGROUP_PARENT")), "/"); v != "" {
		j.CgroupParent = filepath.Clean(v)
	}
	for name, dst := range map[string]*int{
		"VMCTL_JAILER_UID_BASE":   &j.UIDBase,
		"VMCTL_JAILER_UID_COUNT":  &j.UIDCount,
		"VMCTL_JAILER_PIDS_MAX":   &j.PidsMax,
		"VMCTL_JAILER_CPU_WEIGHT": &j.CPUWeight,
	} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				*dst = n
			}
		}
	}
	j.SpeculationDirect = envFlagOn("VMCTL_VMM_SPECULATION_BARRIERS")
	return j
}

func envFlagOn(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

// Validate checks that the manager configuration is usable for
// launching Firecracker VMs on Node B.
func (c ManagerConfig) Validate() error {
	if c.KernelImagePath == "" {
		return fmt.Errorf("VM_KERNEL_IMAGE is required for Firecracker VM management")
	}
	if c.StateDir == "" {
		return fmt.Errorf("VM_STATE_DIR is required for Firecracker VM management")
	}
	return nil
}

// IsFirecrackerAvailable returns true if the Firecracker binary is
// present on the system (i.e., we are on Node B with KVM).
// On macOS or environments without Firecracker, this returns false
// and the vmctl service falls back to host-process autoputer mode.
func IsFirecrackerAvailable() bool {
	bin := os.Getenv("VM_FIRECRACKER_BIN")
	if bin == "" {
		bin = "firecracker"
	}

	// Check if the binary exists and is executable.
	_, err := os.Stat(bin)
	if err != nil {
		// Try PATH lookup.
		if _, pathErr := findInPath(bin); pathErr != nil {
			return false
		}
	}
	return true
}

func findInPath(name string) (string, error) {
	path := os.Getenv("PATH")
	if path == "" {
		return "", fmt.Errorf("no PATH")
	}

	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == ':' {
			candidate := path[start:i] + "/" + name
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
			start = i + 1
		}
	}
	return "", fmt.Errorf("%s not found in PATH", name)
}
