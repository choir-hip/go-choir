package capsulejob

import (
	"strings"
	"testing"
)

// A job's declared memory need (docs/reports/memory-hotplug-controller-
// design-2026-10-14.md section 9, J13): absent is no need; out of range is
// refused at the spec, so the cell and the reducer refuse it alike.
func TestSpecMemoryNeedBounds(t *testing.T) {
	base := Spec{ID: "job-1", Argv: []string{"go", "build"}}
	if spec, err := base.Normalize(); err != nil || spec.MemoryMiB != 0 {
		t.Fatalf("no need: %+v %v", spec, err)
	}
	ok := base
	ok.MemoryMiB = 768
	if spec, err := ok.Normalize(); err != nil || spec.MemoryMiB != 768 {
		t.Fatalf("768 MiB: %+v %v", spec, err)
	}
	ok.MemoryMiB = MaxMemoryMiB
	if _, err := ok.Normalize(); err != nil {
		t.Fatalf("max: %v", err)
	}
	for _, bad := range []int{-1, MaxMemoryMiB + 1} {
		s := base
		s.MemoryMiB = bad
		if _, err := s.Normalize(); err == nil || !strings.Contains(err.Error(), "memory") {
			t.Fatalf("memory_mib %d: %v, want a refusal naming memory", bad, err)
		}
	}
}
