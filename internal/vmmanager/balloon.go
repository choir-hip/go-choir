package vmmanager

// Free page reporting through the virtio balloon
// (docs/reports/lr-guest-memory-return-2026-10-14.md). Without it the host
// keeps every page a guest has touched until the guest stops; with it the
// guest kernel reports freed blocks (2 MiB by default) and Firecracker drops
// them with MADV_DONTNEED, so host RSS follows guest use.
//
// The balloon is never inflated (amount_mib 0): this is reporting only, not
// a host-driven squeeze. Field names are Firecracker v1.15.1's
// BalloonDeviceConfig (src/vmm/src/vmm_config/balloon.rs), which denies
// unknown fields; free_page_hinting (a developer preview with a documented
// corruption race) is never sent.

// firecrackerBalloonConfig returns the "balloon" object for the Firecracker
// --config-file, or nil when free page reporting is off.
func firecrackerBalloonConfig(cfg ManagerConfig) map[string]interface{} {
	if !cfg.BalloonFreePageReporting {
		return nil
	}
	return map[string]interface{}{
		"amount_mib":               0,
		"deflate_on_oom":           true,
		"stats_polling_interval_s": cfg.BalloonStatsPollingIntervalS,
		"free_page_reporting":      true,
	}
}
