package vmmanager

// Free page reporting through the virtio balloon
// (docs/archive/reports/lr-guest-memory-return-2026-10-14.md). Without it the host
// keeps every page a guest has touched until the guest stops; with it the
// guest kernel reports freed blocks (2 MiB by default) and Firecracker drops
// them with MADV_DONTNEED, so host RSS follows guest use.
//
// The balloon is never inflated (amount_mib 0): this is reporting only, not
// a host-driven squeeze. Field names are Firecracker v1.15.1's
// BalloonDeviceConfig (src/vmm/src/vmm_config/balloon.rs), which denies
// unknown fields; free_page_hinting (a developer preview with a documented
// corruption race) is never sent.

// firecrackerBalloonConfig returns the "balloon" object for cfg's
// Firecracker --config-file, or nil when free page reporting is off.
// Statistics are on only for a computer VM_BALLOON_STATS_ONLY names
// (balloon_stats.go); every other computer gets exactly the pre-RES-10
// object, so its snapshot launch-config hash does not move.
func (m *Manager) firecrackerBalloonConfig(cfg VMConfig) map[string]interface{} {
	if !m.cfg.BalloonFreePageReporting {
		return nil
	}
	return map[string]interface{}{
		"amount_mib":               0,
		"deflate_on_oom":           true,
		"stats_polling_interval_s": m.balloonStatsIntervalS(cfg),
		"free_page_reporting":      true,
	}
}
