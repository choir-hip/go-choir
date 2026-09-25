package vmctl

import (
	"testing"
)

func TestInteractiveMachineShapeRejectsBadValues(t *testing.T) {
	for _, v := range []string{"0", "-4", "huge", "4.5"} {
		t.Setenv(interactiveVMCPUCountEnv, v)
		t.Setenv(interactiveVMMemSizeMibEnv, v)
		cpu, mem := interactiveMachineShape()
		if cpu != interactiveVMCPUCount || mem != interactiveVMMemSizeMib {
			t.Fatalf("shape for %q = (%d, %d), want defaults", v, cpu, mem)
		}
	}
}
