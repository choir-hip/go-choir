package agentprofile

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// The vocabulary is the desk registry (MA2.5, F1). Pinned against the closed
// list it replaced.
func TestCanonicalAndIsLiveEqualTheClosedList(t *testing.T) {
	canonical := []string{"management", "engineering", "research", "texture", "conductor", "mail", "management_task", "processor", "reconciler", "verifier"}
	for _, n := range canonical {
		if got, err := Canonical(" " + n + " "); err != nil || got != n {
			t.Errorf("Canonical(%q) = %q, %v", n, got, err)
		}
	}
	if got, _ := Canonical("verifier-multimodal"); got != "verifier_multimodal" {
		t.Errorf("hyphenated multimodal verifier = %q", got)
	}
	live := map[string]bool{"management": true, "engineering": true, "research": true, "texture": true, "conductor": true, "mail": true, "verifier": true, "verifier_multimodal": true}
	for _, n := range append(canonical, "verifier_multimodal") {
		if IsLive(n) != live[n] {
			t.Errorf("IsLive(%q) = %v", n, IsLive(n))
		}
	}
	// The research child is a role of the manifest, not a profile; the V1
	// spellings and unknown names refuse.
	for _, n := range []string{"research_child", "super", "co-super", "nobody", "", "app:nobody"} {
		if _, err := Canonical(n); err == nil {
			t.Errorf("Canonical(%q) accepted", n)
		}
	}
}

func TestRegisteredDeskBecomesAProfileAndRetiredOneRefuses(t *testing.T) {
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:profiletest", Verbs: []string{"Cast"}, Taint: deskmanifest.TaintUntrustedInput, UID: 62501, WorkerMaxBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	if _, err := Canonical("app:profiletest"); err != nil || !IsLive("app:profiletest") {
		t.Fatalf("a registered desk is not a live profile: %v", err)
	}
	if err := deskmanifest.RetireApp("app:profiletest"); err != nil {
		t.Fatal(err)
	}
	if _, err := Canonical("app:profiletest"); err == nil || IsLive("app:profiletest") {
		t.Fatal("a retired desk still resolves")
	}
}
