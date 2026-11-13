package vmctl

import (
	"net/http"
	"testing"
)

// A guest that was slow to answer the planned-restart request may still write
// its marker after the host gave up on it (the unwind of a cancelled snapshot
// capture makes that slower). The host's give-up path asks the guest to clear
// it, the way the failed-restart path already does; otherwise the guest keeps
// the refresh hold for up to 15 minutes, and a crash in that window boots as
// planned and resumes work.
// (docs/problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md, review 1.)

func TestDeployRefreshClearsAMarkerTheGuestMayHaveWrittenAfterTheHostGaveUp(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-slow-mark")
	fake.outcome, fake.detail = guestMarkUnavailable, "context deadline exceeded"
	code, resp := postDeployRefresh(t, h, `{"user_id":"owner-slow-mark","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || len(mgr.refreshes) != 0 {
		t.Fatalf("%d %+v refreshes=%v, want deferred and no restart", code, resp, mgr.refreshes)
	}
	if fake.clears != 1 {
		t.Fatalf("marker clears = %d, want 1: a marker written after the host gave up must not outlive the give-up", fake.clears)
	}
}

func TestRestartInPlaceClearsAMarkerTheGuestMayHaveWrittenBeforeTheMarkerlessRestart(t *testing.T) {
	h, mgr, fake, _ := currentFixture(t)
	fake.outcome, fake.detail = guestMarkUnavailable, "context deadline exceeded"
	code, resp := postRestartInPlace(t, h, "owner-up")
	if code != http.StatusOK || len(mgr.refreshes) != 1 || resp.PlannedRestart {
		t.Fatalf("%d %+v refreshes=%v, want one markerless restart", code, resp, mgr.refreshes)
	}
	if fake.clears != 1 {
		t.Fatalf("marker clears = %d, want 1: a late marker would turn the markerless boot into a planned one", fake.clears)
	}
}
