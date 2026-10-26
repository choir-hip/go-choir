package vmctl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// D2 S3: GET /internal/vmctl/guest-releases (what release retention may not
// delete) and the release fields in vmctl health.

type fakeGuestReleaseManager struct {
	*fakeMemoryManager
	answer  string
	current string
	byVM    map[string]GuestReleaseVMStatus
}

func (m *fakeGuestReleaseManager) GuestReleases() (json.RawMessage, error) {
	return json.RawMessage(m.answer), nil
}

func (m *fakeGuestReleaseManager) CurrentGuestReleaseInfo() (string, string) { return m.current, "" }

func (m *fakeGuestReleaseManager) GuestReleaseHealth() (string, map[string]GuestReleaseVMStatus) {
	return m.current, m.byVM
}

func TestGuestReleasesEndpoint(t *testing.T) {
	mgr := &fakeGuestReleaseManager{
		fakeMemoryManager: newFakeMemoryManager(),
		answer:            `{"retain":true,"current":"c","complete":true,"in_use":[{"id":"a","running":["vm-a"],"snapshots":[]}]}`,
	}
	reg, _ := hostCapacityRegistry(t, mgr)
	h := NewHandler(reg)

	rec := serveThroughHostauth(h.HandleGuestReleases, http.MethodGet, "/internal/vmctl/guest-releases", "127.0.0.1:5000", "", nil)
	if rec.Code != http.StatusOK || rec.Body.String() == "" {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Retain   bool `json:"retain"`
		Complete bool `json:"complete"`
		InUse    []struct {
			ID      string   `json:"id"`
			Running []string `json:"running"`
		} `json:"in_use"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || !got.Retain || !got.Complete || len(got.InUse) != 1 || got.InUse[0].Running[0] != "vm-a" {
		t.Fatalf("answer = %s (%v)", rec.Body.String(), err)
	}

	// Internal callers only; GET only.
	if rec := serveThroughHostauth(h.HandleGuestReleases, http.MethodGet, "/internal/vmctl/guest-releases", "203.0.113.9:5000", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("external caller = %d, want 403", rec.Code)
	}
	if rec := serveThroughHostauth(h.HandleGuestReleases, http.MethodPost, "/internal/vmctl/guest-releases", "127.0.0.1:5000", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rec.Code)
	}

	// A manager without the reader answers 404, which retention reads as "an
	// older vmctl: nothing in use".
	reg2, _ := hostCapacityRegistry(t, newFakeMemoryManager())
	if rec := serveThroughHostauth(NewHandler(reg2).HandleGuestReleases, http.MethodGet, "/internal/vmctl/guest-releases", "127.0.0.1:5000", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("without a reader = %d, want 404", rec.Code)
	}
}

func TestHealthCarriesGuestReleases(t *testing.T) {
	mgr := &fakeGuestReleaseManager{
		fakeMemoryManager: newFakeMemoryManager(),
		current:           "r2",
		byVM: map[string]GuestReleaseVMStatus{
			"vm-old": {GuestRelease: "r1", Behind: true},
			"vm-new": {GuestRelease: "r2"},
		},
	}
	reg, _ := hostCapacityRegistry(t, mgr)
	addOwnership(reg, &VMOwnership{VMID: "vm-old", ComputerID: "computer-test-old", UserID: "qa-o", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	addOwnership(reg, &VMOwnership{VMID: "vm-new", ComputerID: "computer-test-new", UserID: "qa-n", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	addOwnership(reg, &VMOwnership{VMID: "vm-legacy", ComputerID: "computer-test-legacy", UserID: "qa-l", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	srv := httptest.NewServer(http.HandlerFunc(NewHandler(reg).HandleHealth))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var h struct {
		Current string `json:"guest_release_current"`
		Compute []struct {
			ComputerID   string `json:"computer_id"`
			GuestRelease string `json:"guest_release"`
			Behind       bool   `json:"behind"`
		} `json:"guest_releases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h.Current != "r2" || len(h.Compute) != 2 {
		t.Fatalf("health = %+v; want current r2 and the two release-bound computers", h)
	}
	for _, c := range h.Compute {
		if c.ComputerID == "computer-test-old" && (c.GuestRelease != "r1" || !c.Behind) {
			t.Fatalf("old computer = %+v", c)
		}
		if c.ComputerID == "computer-test-new" && (c.GuestRelease != "r2" || c.Behind) {
			t.Fatalf("new computer = %+v", c)
		}
	}
}
