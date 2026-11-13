package agentcore

import (
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// O1, deterministic: the guest's own marker is requested while the withdrawal
// holds the owner between its reason check and its delete. It must land after
// the delete, not between the two, and so survive.
func TestAGuestMarkerCannotLandBetweenTheWithdrawalsCheckAndItsDelete(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	if err := WritePlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "host", RequestedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	var guestDone atomic.Bool
	guestFinished := make(chan struct{})
	prev := plannedRestartMarkerCheckedHook
	plannedRestartMarkerCheckedHook = func() {
		go func() {
			_, _ = rt.markPlannedRestart("platform_update", "update-1")
			guestDone.Store(true)
			close(guestFinished)
		}()
		// Give the guest every chance to write while the check is open.
		time.Sleep(300 * time.Millisecond)
		if guestDone.Load() {
			t.Error("the guest's marker was written between the withdrawal's check and its delete")
		}
	}
	t.Cleanup(func() { plannedRestartMarkerCheckedHook = prev })

	code, body := withdraw(t, rt)
	if code != http.StatusOK {
		t.Fatalf("withdrawal: %d %s", code, body)
	}
	<-guestFinished
	raw, err := os.ReadFile(PlannedRestartMarkerPath(rt.cfg.StorePath))
	if err != nil || !strings.Contains(string(raw), "platform_update") {
		t.Fatalf("the guest's marker did not survive the withdrawal: %v %s", err, raw)
	}
}
