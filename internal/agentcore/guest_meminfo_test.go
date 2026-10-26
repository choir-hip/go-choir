package agentcore

import "testing"

// The guest observability route reads MemTotal. Failure modes: kB
// read as bytes, a missing MemTotal reported as zero memory, a malformed
// line aborting the parse.
func TestParseGuestMeminfo(t *testing.T) {
	got := parseGuestMeminfo("MemTotal:        4031484 kB\nMemFree:  100 kB\nbogus line\nMemAvailable:    3000000 kB\nHugePages_Total: 0\n")
	if got == nil || got.MemTotalBytes != 4031484<<10 || got.MemAvailableBytes != 3000000<<10 || got.MemFreeBytes != 100<<10 {
		t.Fatalf("parsed = %+v", got)
	}
	if parseGuestMeminfo("MemFree: 100 kB\n") != nil {
		t.Fatal("no MemTotal must yield nil")
	}
}
