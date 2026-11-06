//go:build race

package yaegikernel

const raceDetectorEnabled = true

// A test helper worker built with -race maps the race runtime's thread and
// shadow state beyond what a production worker maps, so at the calibrated
// overhead it can fail to start a thread ("pthread_create failed: Resource
// temporarily unavailable"; MA6.4 r3, 2 of 5 runs of
// TestGvisorDeskWorkersShareOneSandboxPerClass on the agents VM, at r2 too).
// Only the spawner in this test binary reads the overhead, so the headroom
// stays out of production. TestGvisorAddressCalibration skips under -race.
func init() { gvisorAddressOverheadBytes += 1 << 30 }
