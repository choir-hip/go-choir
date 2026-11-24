package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The public Pulse summary says whether sign-up is open and nothing of the
// host's live memory, disk, computers or room: those figures are the owner's,
// read on a platform-operator route (the wake refusals already keep them from
// a stranger). Failure modes: the public route carrying a figure; the owner
// route open to anyone, to an email session, or to an account that is not the
// platform operator; the owner route answering without the figures.

func fullCapacitySummary() vmctl.PulseSummary {
	return vmctl.PulseSummary{
		Status: "ok",
		Capacity: vmctl.PulseCapacitySummary{
			Memory:    vmctl.PulseCapacityMemory{Available: true, TotalBytes: 34359738368, AvailableBytes: 21474836480, FloorBytes: 5033164800},
			Disk:      vmctl.PulseCapacityDisk{Available: true, AvailableBytes: 164595924992, FloorBytes: 34359738368},
			Computers: vmctl.PulseCapacityComputers{Running: 7, Hibernated: 11, RunningGuestMemoryMiB: 28672},
			Room:      vmctl.PulseCapacityRoom{Available: true, ReserveBytes: 107374182400, FootprintBytes: 9663676416, Accounts: 17, Promised: 3, HeldBytes: 31337, Places: 4, Open: true, SignUpsLast24h: 2},
		},
		Storage: vmctl.PulseStorageSummary{
			VMStateBytesByClass:         map[string]int64{"real": 777000111},
			VMStateBytesTotal:           888000222,
			VMStateFilesystem:           vmctl.PulseFilesystemSummary{Path: "/var/lib/go-choir/vm-state", TotalBytes: 476000333, AvailableBytes: 154000444, UsedBytes: 322000555},
			NixStoreFilesystem:          vmctl.PulseFilesystemSummary{Path: "/nix", TotalBytes: 999000666, AvailableBytes: 123000777},
			ManualRecoverySnapshotCount: 2,
		},
	}
}

// pulseCapacityServer is a proxy over a vmctl that serves the full summary and
// a corpusd that says whether the account is the platform operator.
func pulseCapacityServer(t *testing.T, operator bool) (get func(path string, session bool) *httptest.ResponseRecorder) {
	t.Helper()
	vmctlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(fullCapacitySummary())
	}))
	t.Cleanup(vmctlSrv.Close)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == platform.PlatformOperatorCheckRoute {
			_, _ = fmt.Fprintf(w, `{"operator":%v}`, operator)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(corpusd.Close)
	autoputer := httptest.NewServer(http.NewServeMux())
	t.Cleanup(autoputer.Close)
	h, priv := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	h.vmctlClient = vmctl.NewClient(vmctlSrv.URL)
	return func(path string, session bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "https://choir.news"+path, nil)
		if session {
			req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
		}
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w
	}
}

func TestPublicPulseCarriesNoCapacityFigures(t *testing.T) {
	get := pulseCapacityServer(t, false)
	w := get("/api/pulse/summary", false)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var capacity map[string]map[string]any
	if err := json.Unmarshal(got["capacity"], &capacity); err != nil {
		t.Fatalf("capacity: %v (%s)", err, got["capacity"])
	}
	if len(capacity) != 1 || len(capacity["room"]) != 2 || capacity["room"]["open"] != true || capacity["room"]["available"] != true {
		t.Fatalf("public capacity = %s, want only room {available, open}", got["capacity"])
	}
	for _, figure := range []string{"34359738368", "21474836480", "164595924992", "107374182400", "9663676416", "28672", "31337", "777000111", "888000222", "476000333", "154000444", "322000555", "999000666", "123000777", "vm_state_filesystem", "vm_state_bytes_total", "nix_store_filesystem"} {
		if strings.Contains(w.Body.String(), figure) {
			t.Fatalf("the public summary carries the host figure %s", figure)
		}
	}
}

func TestOwnerCapacityRouteIsThePlatformOperatorsAndCarriesTheFigures(t *testing.T) {
	// Anonymous: no.
	get := pulseCapacityServer(t, true)
	if w := get(platformOperatorCapacityPath, false); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d", w.Code)
	}
	// The operator, signed in with a passkey: the figures.
	w := get(platformOperatorCapacityPath, true)
	if w.Code != http.StatusOK {
		t.Fatalf("operator = %d %s", w.Code, w.Body.String())
	}
	var got vmctl.PulseOperatorReading
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	full := fullCapacitySummary()
	if got.PulseCapacitySummary != full.Capacity || got.Storage.VMStateBytesTotal != full.Storage.VMStateBytesTotal || got.Storage.VMStateFilesystem != full.Storage.VMStateFilesystem {
		t.Fatalf("the owner route answered %+v, want the full capacity and storage", got)
	}
	// A signed-in account that is not the operator: no.
	getOther := pulseCapacityServer(t, false)
	if w := getOther(platformOperatorCapacityPath, true); w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "34359738368") {
		t.Fatalf("a non-operator = %d %s", w.Code, w.Body.String())
	}
}
