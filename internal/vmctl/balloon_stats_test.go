package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// RES-10 step 2: the host-only read of guest balloon statistics. Failure
// list in docs/problems/balloon-stats-unread-no-emergency-inflation-2026-10-21.md,
// "Failure modes before code" (items 8, 10, 11, 13).

type fakeBalloonStatsManager struct {
	*rssVMManager
	smu   sync.Mutex
	stats map[string]GuestMemoryStats
	errs  map[string]error
	delay map[string]time.Duration
	reads []string
}

func (m *fakeBalloonStatsManager) GuestMemoryStats(vmID string) (GuestMemoryStats, error) {
	m.smu.Lock()
	m.reads = append(m.reads, vmID)
	st, ok := m.stats[vmID]
	err := m.errs[vmID]
	d := m.delay[vmID]
	m.smu.Unlock()
	if d > 0 {
		time.Sleep(d)
	}
	if err != nil {
		return GuestMemoryStats{}, err
	}
	if !ok {
		return GuestMemoryStats{}, &GuestMemoryStatsError{VMID: vmID, Reason: "not_running"}
	}
	return st, nil
}

func u64(v uint64) *uint64 { return &v }

func balloonStatsFixture(t *testing.T) (*Handler, *fakeBalloonStatsManager) {
	t.Helper()
	mgr := &fakeBalloonStatsManager{
		rssVMManager: newRSSVMManager(),
		stats:        map[string]GuestMemoryStats{},
		errs:         map[string]error{},
		delay:        map[string]time.Duration{},
	}
	reg, _ := hostCapacityRegistry(t, mgr)
	for _, own := range []*VMOwnership{
		{VMID: "vm-s", ComputerID: "computer-test-s", UserID: "qa-s", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive},
		{VMID: "vm-u", ComputerID: "computer-test-u", UserID: "qa-u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive},
	} {
		addOwnership(reg, own)
	}
	mgr.stats["vm-s"] = GuestMemoryStats{
		VMID: "vm-s", StatsPollingIntervalS: 5, ObservedAt: time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC),
		Stats: BalloonStatistics{AvailableMemory: u64(6012954214), TotalMemory: u64(8589934592), DiskCaches: u64(4831838208)},
	}
	mgr.errs["vm-u"] = &GuestMemoryStatsError{VMID: "vm-u", Reason: "stats_disabled", Detail: "balloon booted with stats_polling_interval_s 0"}
	return NewHandler(reg), mgr
}

type balloonStatsBody struct {
	VMs []struct {
		VMID       string            `json:"vm_id"`
		ComputerID string            `json:"computer_id"`
		Reading    *GuestMemoryStats `json:"reading"`
		Reason     string            `json:"reason"`
		Detail     string            `json:"detail"`
	} `json:"vms"`
	Units string `json:"units"`
}

func decodeBalloonStats(t *testing.T, raw string) balloonStatsBody {
	t.Helper()
	var body balloonStatsBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	return body
}

// 10, 11: host only, GET only; no guest address and no credential-less
// loopback reaches it.
func TestBalloonStatsRouteHostOnly(t *testing.T) {
	const token = "vmctl-balloon-test-token-0123456789abcdef0123456789"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	h, mgr := balloonStatsFixture(t)
	const path = "/internal/vmctl/memory/balloon"
	if rec := serveThroughHostauth(h.HandleBalloonStats, http.MethodGet, path, "127.0.0.1:5000", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("loopback without credential = %d", rec.Code)
	}
	if rec := serveThroughHostauth(h.HandleBalloonStats, http.MethodGet, path, "10.200.3.2:41000", token, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("guest address = %d", rec.Code)
	}
	if rec := serveThroughHostauth(h.HandleBalloonStats, http.MethodPost, path, "127.0.0.1:5000", token, map[string]any{"vm_id": "vm-s"}); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", rec.Code)
	}
	if len(mgr.reads) != 0 {
		t.Fatalf("refused requests read stats: %v", mgr.reads)
	}
	if rec := serveThroughHostauth(h.HandleBalloonStats, http.MethodGet, path, "127.0.0.1:5000", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("host with credential = %d %s", rec.Code, rec.Body.String())
	}
}

// 13: a reading carries numbers; a reason carries none.
func TestBalloonStatsRouteReadingsAndReasons(t *testing.T) {
	h, _ := balloonStatsFixture(t)
	rec := serveThroughHostauth(h.HandleBalloonStats, http.MethodGet, "/internal/vmctl/memory/balloon", "127.0.0.1:5000", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	body := decodeBalloonStats(t, rec.Body.String())
	if len(body.VMs) != 2 || body.VMs[0].VMID != "vm-s" || body.VMs[1].VMID != "vm-u" {
		t.Fatalf("vms = %+v, want vm-s and vm-u in order", body.VMs)
	}
	s, u := body.VMs[0], body.VMs[1]
	if s.ComputerID != "computer-test-s" || s.Reason != "" || s.Reading == nil || s.Reading.Stats.AvailableMemory == nil || *s.Reading.Stats.AvailableMemory != 6012954214 {
		t.Fatalf("selected reading = %+v", s)
	}
	if u.Reason != "stats_disabled" || u.Reading != nil || !strings.Contains(u.Detail, "interval") {
		t.Fatalf("unselected = %+v, want reason only", u)
	}
	if !strings.Contains(rec.Body.String(), `"vm_id":"vm-u","computer_id":"computer-test-u","reason":"stats_disabled"`) {
		t.Fatalf("reason entry carries more than the reason: %s", rec.Body.String())
	}
	if !strings.Contains(body.Units, "bytes") {
		t.Fatalf("units = %q", body.Units)
	}
}

func TestBalloonStatsRouteOneComputer(t *testing.T) {
	h, mgr := balloonStatsFixture(t)
	rec := serveThroughHostauth(h.HandleBalloonStats, http.MethodGet, "/internal/vmctl/memory/balloon?computer_id=computer-test-s", "127.0.0.1:5000", "", nil)
	body := decodeBalloonStats(t, rec.Body.String())
	if rec.Code != http.StatusOK || len(body.VMs) != 1 || body.VMs[0].VMID != "vm-s" {
		t.Fatalf("one computer = %d %s", rec.Code, rec.Body.String())
	}
	if len(mgr.reads) != 1 {
		t.Fatalf("reads = %v, want only vm-s", mgr.reads)
	}
	rec = serveThroughHostauth(h.HandleBalloonStats, http.MethodGet, "/internal/vmctl/memory/balloon?vm_id=vm-nope", "127.0.0.1:5000", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown vm = %d", rec.Code)
	}
}

// 8: the all-VM read fans out, so one stuck VMM costs the request its own
// bound, not the sum.
func TestBalloonStatsRouteFansOut(t *testing.T) {
	h, mgr := balloonStatsFixture(t)
	mgr.delay["vm-s"] = 300 * time.Millisecond
	mgr.delay["vm-u"] = 300 * time.Millisecond
	start := time.Now()
	rec := serveThroughHostauth(h.HandleBalloonStats, http.MethodGet, "/internal/vmctl/memory/balloon", "127.0.0.1:5000", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if el := time.Since(start); el > 550*time.Millisecond {
		t.Fatalf("two 300 ms reads took %v; they must run in parallel", el)
	}
}

// A manager without the reader answers a reason per computer.
func TestBalloonStatsRouteWithoutReader(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newRSSVMManager())
	addOwnership(reg, &VMOwnership{VMID: "vm-x", ComputerID: "computer-test-x", UserID: "qa-x", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	rec := serveThroughHostauth(NewHandler(reg).HandleBalloonStats, http.MethodGet, "/internal/vmctl/memory/balloon", "127.0.0.1:5000", "", nil)
	body := decodeBalloonStats(t, rec.Body.String())
	if rec.Code != http.StatusOK || len(body.VMs) != 1 || body.VMs[0].Reason != "unsupported" || body.VMs[0].Reading != nil {
		t.Fatalf("without reader = %d %s", rec.Code, rec.Body.String())
	}
}

func newRSSVMManager() *rssVMManager {
	return &rssVMManager{processControlVMManager: newProcessControlVMManager(), rss: map[string]uint64{}}
}

func serveThroughHostauth(h http.HandlerFunc, method, path, remote, token string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.RemoteAddr = remote
	if token != "" {
		req.Header.Set(hostauth.Header, token)
	}
	rec := httptest.NewRecorder()
	hostauth.Middleware(h).ServeHTTP(rec, req)
	return rec
}

// The VM list shows the latest RSS sample and the base size of an active
// computer, from the admission ledger, with no Firecracker call.
func TestListShowsCurrentMemory(t *testing.T) {
	reg, now := hostCapacityRegistry(t, newRSSVMManager())
	addOwnership(reg, &VMOwnership{VMID: "vm-m", ComputerID: "computer-memory-m", UserID: "user-m", DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateActive, ComputerURL: "http://10.200.3.2:8085"})
	reg.admissionLedger.mu.Lock()
	reg.admissionLedger.initLocked()
	reg.admissionLedger.rss["vm-m"] = vmMemoryObservation{bytes: 1500 << 20, at: *now}
	reg.admissionLedger.mu.Unlock()
	list, err := NewHandler(reg).listOwnershipResponses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got *ownershipResponse
	for i := range list {
		if list[i].VMID == "vm-m" {
			got = &list[i]
		}
	}
	if got == nil || got.MemoryBaseMiB <= 0 || got.MemoryRSSBytes != 1500<<20 {
		t.Fatalf("list memory fields = %+v", got)
	}
}
