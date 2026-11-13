package agentcore

import "time"

// restartRelease selects which request-scoped protections to release.
type restartRelease uint8

const (
	relBlock       restartRelease = 1 << iota // the producer's post-cancel restart block
	relRefreshHold                            // new engineering waiting for the refresh
	relTelemetry                              // the host's synchronous telemetry hold
	relThaw                                   // capsules this request paused
)

// releaseRestartProtections is the ONE place the protections of a coming kill
// are released (TestRestartProtectionsAreReleasedOnlyThroughTheOneFunction
// fails if any other file calls the individual releases). Rule 2 of the
// marker's owner: while a marker that could still plan a boot is on disk, it
// releases nothing and returns false, whatever the caller asks. Every refusal
// and error branch of the planned-restart handler, the withdrawal and the
// restore cleanup call it and nothing else.
//
// The check and the release are not one atomic step, and need not be: the
// protections are the host request's, a host request holds plannedRestartReqMu
// for its whole run, and the only writer that can land a marker between the
// check and the release is a guest's own (platform_update, a self-development
// apply or restore), which then protects its own restart by its own means.
// Doing the release under the marker mutex instead would take the telemetry
// and capsule locks inside a leaf lock and invert the documented order.
//
// telemetryLocked is true when the caller already holds hostRestartTelemetryMu.
func (rt *Runtime) releaseRestartProtections(storePath string, rel restartRelease, telemetryLocked bool) bool {
	if rt == nil || plannedRestartMarkerPending(storePath, time.Now()) {
		return false
	}
	if rel&relBlock != 0 {
		rt.snapshots.clearRestartBlock()
	}
	if rel&relRefreshHold != 0 {
		rt.clearRefreshHold()
	}
	if rel&relTelemetry != 0 {
		if telemetryLocked {
			rt.releaseHostRestartTelemetryLocked()
		} else {
			rt.hostRestartTelemetryMu.Lock()
			rt.releaseHostRestartTelemetryLocked()
			rt.hostRestartTelemetryMu.Unlock()
		}
	}
	if rel&relThaw != 0 {
		rt.unpauseAllCapsules()
	}
	return true
}
