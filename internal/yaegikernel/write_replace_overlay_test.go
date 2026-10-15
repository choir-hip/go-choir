package yaegikernel

import (
	"os"
	"testing"
)

// Opt-in check on a real overlayfs (Linux, privileged setup, run as a
// non-root uid), the shape of an engineering capsule's source tree: a
// root-owned 0444 file in the lower layer under a 0777 directory, merged. Set
//
//	CHOIR_OVERLAY_MERGED_FILE  the file's path in the merged mount
//	CHOIR_OVERLAY_LOWER_FILE   the same file in the lower layer
//
// It is skipped otherwise; CI and workstations do not mount overlays.
func TestWriteFileReplacingOnAnOverlayLowerFile(t *testing.T) {
	merged := os.Getenv("CHOIR_OVERLAY_MERGED_FILE")
	lower := os.Getenv("CHOIR_OVERLAY_LOWER_FILE")
	if merged == "" || lower == "" {
		t.Skip("CHOIR_OVERLAY_MERGED_FILE / CHOIR_OVERLAY_LOWER_FILE not set")
	}
	skipIfRoot(t)
	if err := os.WriteFile(merged, []byte("x"), 0o644); err == nil {
		t.Fatal("premise: a plain write over the lower 0444 file succeeded")
	}
	if err := WriteFileReplacing(merged, []byte("edited through the overlay\n"), 0o644, false); err != nil {
		t.Fatalf("replace write on the overlay: %v", err)
	}
	if got, err := os.ReadFile(merged); err != nil || string(got) != "edited through the overlay\n" {
		t.Fatalf("merged file = %q, %v", got, err)
	}
	if got, err := os.ReadFile(lower); err != nil || string(got) == "edited through the overlay\n" {
		t.Fatalf("lower layer was modified: %q, %v", got, err)
	}
}
