package pkgrelease

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

func shellRecord(owner, track, name string) Record {
	return Record{
		Package: Package{Name: name}, Track: track, Publisher: Publisher{OwnerID: owner},
		Manifest: Manifest{Class: string(apppackage.ClassProvider), Level: string(apppackage.LevelPrivileged)},
	}
}

// The install gate: a T2 record installs only for the owner of its own track,
// for a package the base image provides; behavioral, never by name.
func TestRequireOwnTrackShell(t *testing.T) {
	if err := RequireOwnTrackShell(shellRecord("o1", "o1", "menu-default"), "o1"); err != nil {
		t.Fatalf("own-track base update refused: %v", err)
	}
	other := func(class apppackage.Class, level apppackage.Level) Record {
		return Record{Package: Package{Name: "x"}, Track: "o2", Publisher: Publisher{OwnerID: "o2"}, Manifest: Manifest{Class: string(class), Level: string(level)}}
	}
	for name, tc := range map[string]struct {
		rec   Record
		owner string
	}{
		"another owner installs":     {shellRecord("o2", "o2", "menu-default"), "o1"},
		"forged track":               {shellRecord("o2", "o1", "menu-default"), "o1"},
		"not a base package":         {shellRecord("o1", "o1", "my-menu"), "o1"},
		"no installing owner":        {shellRecord("o1", "o1", "menu-default"), ""},
		"provider class alone":       {other(apppackage.ClassProvider, apppackage.LevelDefault), "o1"},
		"a window app with ui.shell": {other(apppackage.ClassApp, apppackage.LevelPrivileged), "o1"},
	} {
		if ReasonOf(RequireOwnTrackShell(tc.rec, tc.owner)) != ReasonShellNotYours {
			t.Errorf("%s: not refused", name)
		}
	}
	// A T1 release is not the gate's business.
	if err := RequireOwnTrackShell(other(apppackage.ClassApp, apppackage.LevelDefault), "o1"); err != nil {
		t.Errorf("T1 refused by the T2 gate: %v", err)
	}
}

// owner_listed is a real listing state: head-eligible, offered to the owner's
// track, never public.
func TestOwnerListedState(t *testing.T) {
	if !HeadEligible(StateOwnerListed) {
		t.Error("owner_listed must be head-eligible, or the next draft's parent is wrong")
	}
	l := Listing{State: StateOwnerListed, Track: "o1"}
	if !l.VisibleTo("o1") || l.VisibleTo("o2") || l.VisibleTo("") {
		t.Error("owner_listed must be visible to its owner alone")
	}
	installed := TrackRelease{ReleaseID: "a", Package: "p", Track: "o1", Seq: 1}
	newer := NewerReleases(installed, []TrackRelease{{ReleaseID: "b", Package: "p", Track: "o1", Seq: 2, State: StateOwnerListed, ListedAt: time.Now().Add(-48 * time.Hour)}})
	if len(newer) != 1 || !newer[0].Soaked(time.Now()) {
		t.Errorf("owner_listed update not offered or never soaks: %+v", newer)
	}
}

// The review: T2 is refused unless the caller holds the by-decision gate.
func TestReviewPlatformOwnShell(t *testing.T) {
	raw := []byte(`{"provides":[{"service":"contextMenu.renderer"}],"capabilities":[{"grant":"ui.shell"}]}`)
	if ReasonOf(reviewPlatform(raw, false)) != ReasonPlatformReviewRequired {
		t.Error("T2 admitted without the gate")
	}
	if err := reviewPlatform(raw, true); err != nil {
		t.Errorf("T2 refused with the gate: %v", err)
	}
}
