package vmctl

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// D2 S5 item 1, host half: no hibernate (idle sweep, make-room, pressure,
// operator) of a computer whose deploy refresh is in flight. The refresh holds
// the planned-restart marker window; a snapshot taken inside it would capture
// the marker (design 5.3).
//
// Failure modes, written first:
//   - a hibernate runs between the marker write and the kill;
//   - the guard stays up after the refresh ends, so the computer can never
//     hibernate again;
//   - the operator endpoint reports success for a refused hibernate.

func TestHibernateSkipsAComputerWhoseDeployRefreshIsInFlight(t *testing.T) {
	h, mgr, _ := newDeployRefreshFixture(t, "refreshing")
	release, ok := h.deployRefreshState().begin("refreshing", "primary")
	if !ok {
		t.Fatal("setup: could not take the in-flight slot")
	}

	// the idle sweep's entry point
	err := h.registry.HibernateVMForDesktop("refreshing", "primary")
	if !errors.Is(err, errHibernateGuestBusy) {
		t.Fatalf("hibernate during a deploy refresh = %v; want errHibernateGuestBusy (a skip, never the kill path)", err)
	}
	if len(mgr.hibernates) != 0 {
		t.Fatalf("the manager hibernated a computer mid-refresh: %v", mgr.hibernates)
	}

	// the operator endpoint
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/hibernate", strings.NewReader(`{"user_id":"refreshing","desktop_id":"primary"}`))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleHibernate(rec, req)
	if rec.Code != http.StatusConflict || len(mgr.hibernates) != 0 {
		t.Fatalf("operator hibernate during a refresh: %d %s hibernates=%v; want 409 and no hibernate", rec.Code, rec.Body.String(), mgr.hibernates)
	}

	// the guard ends with the refresh
	release()
	if err := h.registry.HibernateVMForDesktop("refreshing", "primary"); err != nil {
		t.Fatalf("hibernate after the refresh ended: %v", err)
	}
	if len(mgr.hibernates) != 1 {
		t.Fatalf("hibernates = %v; want one", mgr.hibernates)
	}
}
