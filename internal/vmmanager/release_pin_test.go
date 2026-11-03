package vmmanager

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Release pins (docs/design/ring-ordered-deploys-2026-11-03.md). Failure
// modes first:
//   P1 a pinned computer cold boots the current release (containment leak);
//   P2 a pinned computer reads as "behind" and the sweep refreshes it in a
//      loop onto the release it already runs;
//   P3 a pin to a release that does not load fails the boot;
//   P4 a layered ring computer pinned gets the single image while its layered
//      pin loads;
//   P5 retention deletes a pinned release (it is not reported in use);
//   P6 no pinner, or a pin of "", changes nothing.

const relPin = "44444444444444444444444444444444"

type fakePinner struct {
	single, layered map[string]string
	pinned          []string
}

func (f fakePinner) Pin(id string) (string, string) { return f.single[id], f.layered[id] }
func (f fakePinner) Pinned() []string               { return f.pinned }

func TestReleasePinHoldsComputerOnEarlierRelease(t *testing.T) {
	r := newRingFixture(t, []string{ringComputer})
	writeTestRelease(t, r.releases, relPin, defaultRelPolicy(3*time.Hour))
	// P6
	if cfg := bindFor(r.mgr, "computer-x"); cfg.GuestRelease != relR1 {
		t.Fatalf("no pinner: launch %q, want current %s", cfg.GuestRelease, relR1)
	}
	r.mgr.SetReleasePinner(fakePinner{single: map[string]string{"computer-x": relPin, ringComputer: relPin}, layered: map[string]string{}, pinned: []string{relPin}})
	// P1
	if cfg := bindFor(r.mgr, "computer-x"); cfg.GuestRelease != relPin || cfg.StoreDiskPath != filepath.Join(r.releases, relPin, "storedisk.erofs") {
		t.Fatalf("pinned computer launched %q (store %q), want %s", cfg.GuestRelease, cfg.StoreDiskPath, relPin)
	}
	if id, _ := r.mgr.GuestReleaseInfoFor("computer-x"); id != relPin {
		t.Fatalf("GuestReleaseInfoFor = %q, want the pin", id)
	}
	if cfg := bindFor(r.mgr, "computer-y"); cfg.GuestRelease != relR1 {
		t.Fatalf("an unpinned computer launched %q", cfg.GuestRelease)
	}
	// P4: the layered computer pinned to single only gets the single pin.
	if cfg := bindFor(r.mgr, ringComputer); cfg.GuestRelease != relPin || cfg.BaseDiskPath != "" {
		t.Fatalf("layered computer with a single pin launched %q base %q", cfg.GuestRelease, cfg.BaseDiskPath)
	}
	// P5
	var use GuestReleaseUsage
	raw, _ := json.Marshal(r.mgr.GuestReleaseUsage())
	_ = json.Unmarshal(raw, &use)
	found := false
	for _, e := range use.InUse {
		if e.ID == relPin && e.PinnedBy == "ring-walk" {
			found = true
		}
	}
	if !found || !strings.Contains(string(raw), `"pinned_by":"ring-walk"`) {
		t.Fatalf("pinned release not reported in use: %s", raw)
	}
	// P3: a pin that does not load falls back to the current release.
	r.mgr.SetReleasePinner(fakePinner{single: map[string]string{"computer-x": relR3}})
	if cfg := bindFor(r.mgr, "computer-x"); cfg.GuestRelease != relR1 {
		t.Fatalf("an unloadable pin launched %q, want the current %s", cfg.GuestRelease, relR1)
	}
	// P4: a layered pin for the layered computer.
	r.mgr.SetReleasePinner(fakePinner{single: map[string]string{ringComputer: relPin}, layered: map[string]string{ringComputer: relR2}})
	if cfg := bindFor(r.mgr, ringComputer); cfg.GuestRelease != relR2 || cfg.BaseDiskPath == "" {
		t.Fatalf("layered pin launched %q base %q, want layered %s", cfg.GuestRelease, cfg.BaseDiskPath, relR2)
	}
	r.mgr.SetReleasePinner(nil)
	if cfg := bindFor(r.mgr, "computer-x"); cfg.GuestRelease != relR1 {
		t.Fatalf("pinner removed: launch %q", cfg.GuestRelease)
	}
}

// P2: a running VM on the pinned release is not behind.
func TestReleasePinnedComputerNotBehind(t *testing.T) {
	r := newRingFixture(t, []string{ringComputer})
	r.inst.Config.GuestRelease = relR1
	r.inst.State = StateRunning
	r.point(t, relR3) // the link moved; relR3 has no directory, so the current is unresolved
	writeTestRelease(t, r.releases, relPin, defaultRelPolicy(time.Hour))
	r.point(t, relPin)
	_, byVM := r.mgr.GuestReleaseHealth()
	if st, ok := byVM[r.vmID]; !ok || !st.Behind {
		t.Fatalf("without a pin the VM on %s must be behind %s: %+v", relR1, relPin, byVM)
	}
	r.mgr.SetReleasePinner(fakePinner{single: map[string]string{ringComputer: relR1}, layered: map[string]string{ringComputer: relR1}})
	_, byVM = r.mgr.GuestReleaseHealth()
	if st := byVM[r.vmID]; st.Behind {
		t.Fatalf("a VM on its pinned release reads behind: %+v", st)
	}
}
