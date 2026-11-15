package agentcore

// SCAFFOLD, replaced by the code commit of lane/restore-act: the names the
// tests of the restore act reach, doing nothing, so that the tests fail on
// behavior and not on a missing symbol.

// restoreActApplyHook, when set, is called after each tail event a restore
// act applies, with the number applied so far. It is a seam for tests that
// hold a replay mid-way.
var restoreActApplyHook func(applied int)

// restoreActFreeBytes is the free-space reading the disk floor uses; the
// production value is freeBytesOnVolume.
var restoreActFreeBytes = freeBytesOnVolume

// RepairRestoreAtBoot is the boot's pass over what a restore left behind.
// It returns the act ids it closed.
func RepairRestoreAtBoot(storePath, updaterRoot string) []string { return nil }

// RestoreActEstimate is what the acceptance answer says about the work ahead.
type RestoreActEstimate struct {
	TailEvents       uint64  `json:"tail_events"`
	ReplayRate       float64 `json:"replay_rate"`
	ReplayRateSource string  `json:"replay_rate_source"`
	EstimatedSeconds float64 `json:"estimated_seconds"`
}

func estimateRestore(tail uint64, measuredRate float64, measured bool) RestoreActEstimate {
	return RestoreActEstimate{}
}
