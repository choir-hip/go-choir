package vmctl

import (
	"testing"
)

func TestInteractiveMachineShapeDefaults(t *testing.T) {
	t.Setenv(interactiveVMCPUCountEnv, "")
	t.Setenv(interactiveVMMemSizeMibEnv, "")
	cpu, mem := interactiveMachineShape()
	if cpu != interactiveVMCPUCount || mem != interactiveVMMemSizeMib {
		t.Fatalf("shape = (%d, %d), want defaults (%d, %d)", cpu, mem, interactiveVMCPUCount, interactiveVMMemSizeMib)
	}
}

func TestInteractiveMachineShapeEnvOverride(t *testing.T) {
	t.Setenv(interactiveVMCPUCountEnv, "4")
	t.Setenv(interactiveVMMemSizeMibEnv, "8192")
	cpu, mem := interactiveMachineShape()
	if cpu != 4 || mem != 8192 {
		t.Fatalf("shape = (%d, %d), want (4, 8192)", cpu, mem)
	}
}

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
