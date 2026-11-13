package agentcore

// ClearPlannedRestartMarker removes whatever marker is on disk. It exists for
// tests only: no non-test code may clear a marker unconditionally (the boot's
// consume aside); production clears by reason (clearPlannedRestartMarkerIfReason).
func ClearPlannedRestartMarker(storePath string) error {
	plannedRestartMarkerMu.Lock()
	defer plannedRestartMarkerMu.Unlock()
	return clearPlannedRestartMarkerLocked(storePath)
}
