package yaegikernel

import (
	"strings"
	"testing"
)

// choir.ReadMail is the first-party Mail reader's alone (drill P4): no
// built-in desk and no other app desk has it on its surface or can call it.
func TestReadMailIsTheMailReaderDrillsAlone(t *testing.T) {
	registerSurfaceProbeApp()
	for _, role := range []string{"management", "research", "texture", "engineering", "app:surfaceprobe", "app:mail-reader-drill"} {
		has := strings.Contains(DeskSurface(role, "", false), "choir.ReadMail(")
		if want := role == "app:mail-reader-drill"; has != want {
			t.Errorf("role %s: choir.ReadMail on surface = %v, want %v", role, has, want)
		}
	}
	other := &ChoirScope{desk: "app:surfaceprobe"}
	if _, err := other.ReadMail("msg-a"); err == nil {
		t.Error("a desk without the ReadMail verb called it")
	}
}
