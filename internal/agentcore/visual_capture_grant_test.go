package agentcore

import "testing"

// The overlays teach capture and review only on a computer the host granted;
// a runtime with no executor, or one whose grant is absent, grants nothing.
func TestVisualCaptureIsNotGrantedWithoutTheHostsGrant(t *testing.T) {
	var none *Runtime
	if none.visualCaptureGranted() {
		t.Fatal("a nil runtime reports visual capture granted")
	}
	runtime, _ := testRuntime(t)
	if runtime.visualCaptureGranted() {
		t.Fatal("a runtime with no capsule executor reports visual capture granted")
	}
}
