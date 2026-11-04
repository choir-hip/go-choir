package modelpolicy

import "testing"

// NormalizeRole reads the desk registry (MA2.5, F2); pinned against the closed
// list it replaced.
func TestNormalizeRoleEqualsTheClosedList(t *testing.T) {
	for _, n := range []string{"management", "engineering", "research", "conductor", "texture", "email", "management_task", "verifier", "verifier_multimodal"} {
		if got := NormalizeRole(" " + n + " "); got != n {
			t.Errorf("NormalizeRole(%q) = %q", n, got)
		}
	}
	if got := NormalizeRole("Verifier-Multimodal"); got != "verifier_multimodal" {
		t.Errorf("hyphenated verifier = %q", got)
	}
	for _, n := range []string{"research_child", "co-super", "", "nobody"} {
		if got := NormalizeRole(n); got != "" {
			t.Errorf("NormalizeRole(%q) = %q, want fail-closed", n, got)
		}
	}
}
