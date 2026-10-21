//go:build !linux

package yaegikernel

// runDeskCellProbes: confinement is Linux-only (the guest is Linux); a dev
// host reports that it probed nothing, which the judge refuses.
func runDeskCellProbes(allowedRoot, tmp string, reads []string) DeskCellSelfTestProbes {
	return DeskCellSelfTestProbes{"unsupported": "ERROR: desk cell probes need Linux"}
}

func landlockABI() int { return 0 }
