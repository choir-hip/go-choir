package store

// telemetrySealer is the phase 1 window sealer. Phase 0 carries only this
// placeholder so the instrumentation and the read-class API compile and are
// exercised before any queue exists.
type telemetrySealer struct{}

func (sl *telemetrySealer) snapshotState() (enabled, synchronous bool, windowMS, depth int) {
	return false, false, 0, 0
}

func (sl *telemetrySealer) barrier(sealReason) {}
