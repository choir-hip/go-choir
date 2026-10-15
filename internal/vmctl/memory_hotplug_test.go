package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// Memory hotplug lease controller. The failure list (F1-F20) is in
// docs/reports/memory-hotplug-controller-design-2026-10-14.md section 5,
// written before this code; each test names the items it pins.

// fakeMemoryManager is a VM manager with a virtio-mem device per VM. The
// guest plugs instantly and unplugs down to floor (memory it cannot free).
type fakeMemoryManager struct {
	*rssVMManager
	hmu         sync.Mutex
	dev         map[string]*MemoryHotplugStatus
	floor       map[string]int
	patches     []string
	statusCalls int
	statusErr   map[string]error
	setErr      error
}

func newFakeMemoryManager() *fakeMemoryManager {
	return &fakeMemoryManager{
		rssVMManager: &rssVMManager{processControlVMManager: newProcessControlVMManager(), rss: map[string]uint64{}},
		dev:          map[string]*MemoryHotplugStatus{},
		floor:        map[string]int{},
		statusErr:    map[string]error{},
	}
}

func (m *fakeMemoryManager) addDevice(vmID string, base, total int) {
	m.hmu.Lock()
	defer m.hmu.Unlock()
	m.dev[vmID] = &MemoryHotplugStatus{BaseMiB: base, TotalMiB: total, BlockMiB: 2, SlotMiB: 128}
}

func (m *fakeMemoryManager) MemoryHotplugStatus(vmID string) (MemoryHotplugStatus, error) {
	m.hmu.Lock()
	defer m.hmu.Unlock()
	m.statusCalls++
	if err := m.statusErr[vmID]; err != nil {
		return MemoryHotplugStatus{}, err
	}
	st := m.dev[vmID]
	if st == nil {
		return MemoryHotplugStatus{}, ErrMemoryHotplugUnavailable
	}
	if st.PluggedMiB < st.RequestedMiB {
		st.PluggedMiB = st.RequestedMiB
	} else if st.PluggedMiB > st.RequestedMiB {
		st.PluggedMiB = max(st.RequestedMiB, m.floor[vmID])
	}
	return *st, nil
}

func (m *fakeMemoryManager) SetMemoryHotplugRequestedMiB(vmID string, mib int) error {
	m.hmu.Lock()
	defer m.hmu.Unlock()
	m.patches = append(m.patches, vmID+"="+itoa(mib))
	if m.setErr != nil {
		return m.setErr
	}
	st := m.dev[vmID]
	if st == nil {
		return ErrMemoryHotplugUnavailable
	}
	st.RequestedMiB = mib
	return nil
}

func (m *fakeMemoryManager) patchLog() []string {
	m.hmu.Lock()
	defer m.hmu.Unlock()
	return append([]string(nil), m.patches...)
}

func itoa(n int) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

type memoryFixture struct {
	reg   *OwnershipRegistry
	mgr   *fakeMemoryManager
	now   *time.Time
	mu    sync.Mutex
	avail uint64
	psi   float64
	h     *Handler
}

func newMemoryFixture(t *testing.T, mutate func(*MemoryHotplugConfig)) *memoryFixture {
	t.Helper()
	mgr := newFakeMemoryManager()
	reg, now := hostCapacityRegistry(t, mgr)
	f := &memoryFixture{reg: reg, mgr: mgr, now: now, avail: 16 * gib}
	reg.SetPressureReclaimConfig(PressureReclaimConfig{MinMemoryAvailableBytes: 4 * gib})
	reg.SetHostAdmissionConfig(measuredAdmission())
	reg.setPressureSamplerForTest(func(PressureReclaimConfig) HostPressureSample {
		f.mu.Lock()
		defer f.mu.Unlock()
		return HostPressureSample{MemoryTotalBytes: 32 * gib, MemoryAvailableBytes: f.avail, MemorySomeAvg10: f.psi}
	})
	cfg := MemoryHotplugConfig{Enabled: true, Only: []string{"computer-memory-m", "computer-memory-n"}, RegionMiB: 4096, LeaseMaxMiB: 2048}
	if mutate != nil {
		mutate(&cfg)
	}
	reg.SetMemoryHotplugConfig(cfg)
	f.h = NewHandler(reg)
	return f
}

func (f *memoryFixture) addComputer(vmID, computerID, guestIP string) *VMOwnership {
	own := &VMOwnership{VMID: vmID, ComputerID: computerID, UserID: "user-" + computerID, DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateActive, ComputerURL: "http://" + guestIP + ":8085"}
	addOwnership(f.reg, own)
	f.mgr.addDevice(vmID, 4096, 4096)
	return own
}

func (f *memoryFixture) set(avail uint64, psi float64) {
	f.mu.Lock()
	f.avail, f.psi = avail, psi
	f.mu.Unlock()
}

func (f *memoryFixture) advance(d time.Duration) { *f.now = f.now.Add(d) }

func (f *memoryFixture) leaseBytes(vmID string) uint64 {
	led := &f.reg.admissionLedger
	led.mu.Lock()
	defer led.mu.Unlock()
	led.initLocked()
	return led.leases[vmID]
}

func reqStatus(err error) int {
	var reqErr *MemoryRequestError
	if errors.As(err, &reqErr) {
		return reqErr.Status
	}
	return 0
}

// Grow, observe, release, shrink: the whole lease, with the reservation
// held until the unplug completes.
func TestMemoryLeaseGrowThenReleaseShrinksToBase(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")

	view, err := f.reg.RequestMemory(own, 4096+1024, 0, MemoryLeaseHolderHost)
	if err != nil {
		t.Fatalf("grow: %v", err)
	}
	if view.RequestedMiB != 5120 || view.BaseMiB != 4096 || view.Lease == nil || view.Lease.Holder != "host" {
		t.Fatalf("grow view = %+v", view)
	}
	if got := f.mgr.patchLog(); len(got) != 1 || got[0] != "vm-m=1024" {
		t.Fatalf("patches = %v", got)
	}
	if f.leaseBytes("vm-m") != 1*gib {
		t.Fatalf("reservation = %d, want 1 GiB", f.leaseBytes("vm-m"))
	}
	if v := f.reg.WaitMemory(context.Background(), own, time.Second); v.Phase != MemoryPhaseLeased || v.CurrentMiB != 5120 || v.PluggedMiB != 1024 {
		t.Fatalf("after plug = %+v", v)
	}
	// The default TTL applies.
	if exp, _ := time.Parse(time.RFC3339, view.Lease.ExpiresAt); !exp.Equal(f.now.Add(10 * time.Minute)) {
		t.Fatalf("expires = %s, want now+10m", view.Lease.ExpiresAt)
	}

	// Release: target at the base.
	view, err = f.reg.RequestMemory(own, 4096, 0, MemoryLeaseHolderHost)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if view.Lease != nil || view.EndReason != "released" {
		t.Fatalf("release view = %+v", view)
	}
	f.reg.memoryTick()
	v, _ := f.reg.MemoryView("vm-m")
	if v.Phase != MemoryPhaseBase || v.CurrentMiB != 4096 || f.leaseBytes("vm-m") != 0 {
		t.Fatalf("after shrink = %+v reservation %d", v, f.leaseBytes("vm-m"))
	}
	c := f.reg.MemorySummary().Counters
	if c.Grants != 1 || c.Releases != 1 || c.ShrinksCompleted != 1 {
		t.Fatalf("counters = %+v", c)
	}
}

// F7: rounding to the block, the per-computer max, and renewal.
func TestMemoryLeaseMaxRoundingAndRenewal(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	if _, err := f.reg.RequestMemory(own, 4096+2049, 0, "host"); reqStatus(err) != http.StatusUnprocessableEntity {
		t.Fatalf("above the 2 GiB lease max = %v", err)
	}
	view, err := f.reg.RequestMemory(own, 4096+511, 60*time.Hour, "host")
	if err != nil || view.RequestedMiB != 4096+512 {
		t.Fatalf("unaligned grow = %+v %v, want rounded to 512", view, err)
	}
	if exp, _ := time.Parse(time.RFC3339, view.Lease.ExpiresAt); !exp.Equal(f.now.Add(30 * time.Minute)) {
		t.Fatalf("ttl not capped at 30m: %s", view.Lease.ExpiresAt)
	}
	// Renewal at the same size: no admission, a new expiry.
	f.set(1*gib, 0) // would refuse a grow
	f.advance(5 * time.Minute)
	view, err = f.reg.RequestMemory(own, 4096+512, time.Minute, "host")
	if err != nil {
		t.Fatalf("renewal under pressure refused: %v", err)
	}
	if exp, _ := time.Parse(time.RFC3339, view.Lease.ExpiresAt); !exp.Equal(f.now.Add(time.Minute)) {
		t.Fatalf("renewal expiry = %s", view.Lease.ExpiresAt)
	}
	// A larger target is a new grow and is admitted again.
	if _, err := f.reg.RequestMemory(own, 4096+1024, 0, "host"); reqStatus(err) != http.StatusServiceUnavailable {
		t.Fatalf("grow under pressure = %v", err)
	}
	if c := f.reg.MemorySummary().Counters; c.Renewals != 1 || c.Grants != 1 {
		t.Fatalf("counters = %+v", c)
	}
}

// F5: refused grows under PSI, short memory, or a missing sample; nothing
// is reserved or patched.
func TestMemoryGrowRefusedUnderHostPressure(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	for name, tc := range map[string]struct {
		avail uint64
		psi   float64
	}{
		"psi above 1.0":    {avail: 16 * gib, psi: 1.5},
		"below floor+head": {avail: 5 * gib}, // floor 4 + headroom 1 + lease 1
		"no sample":        {avail: 0},
	} {
		f.set(tc.avail, tc.psi)
		if _, err := f.reg.RequestMemory(own, 4096+1024, 0, "host"); reqStatus(err) != http.StatusServiceUnavailable {
			t.Fatalf("%s: grow = %v, want 503 refusal", name, err)
		}
	}
	if len(f.mgr.patchLog()) != 0 || f.leaseBytes("vm-m") != 0 {
		t.Fatalf("a refused grow patched %v or reserved %d", f.mgr.patchLog(), f.leaseBytes("vm-m"))
	}
	if c := f.reg.MemorySummary().Counters; c.Refusals != 3 {
		t.Fatalf("refusals = %d", c.Refusals)
	}
	recent := f.reg.HostAdmissionSummary().Recent
	if len(recent) == 0 || recent[len(recent)-1].Mode != "memory_grow" || recent[len(recent)-1].Decision != "refuse" {
		t.Fatalf("decision not recorded: %+v", recent)
	}
}

// F6: a lease is a reservation that the next grow and every start see.
func TestMemoryLeasesReserveAgainstEachOtherAndStarts(t *testing.T) {
	f := newMemoryFixture(t, nil)
	a := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	b := f.addComputer("vm-n", "computer-memory-n", "10.200.3.6")
	// floor 4 + headroom 1 + 2 GiB lease = 7 GiB; 8.5 GiB fits one, not two.
	f.set(8*gib+gib/2, 0)
	if _, err := f.reg.RequestMemory(a, 4096+2048, 0, "host"); err != nil {
		t.Fatalf("first lease: %v", err)
	}
	if _, err := f.reg.RequestMemory(b, 4096+2048, 0, "host"); reqStatus(err) != http.StatusServiceUnavailable {
		t.Fatalf("second lease passed the same reading: %v", err)
	}
	sum := f.reg.HostAdmissionSummary()
	if sum.ReservedBytes != 2*gib {
		t.Fatalf("start admission reserved = %d, want the 2 GiB lease", sum.ReservedBytes)
	}
	found := false
	for _, r := range sum.Reservations {
		if r.VMID == "vm-m" && r.NeedSource == memoryLeaseNeedSource {
			found = true
		}
	}
	if !found {
		t.Fatalf("lease missing from health reservations: %+v", sum.Reservations)
	}
}

// F1, F4: disabled, not selected, not active, no device.
func TestMemoryRequestRefusedOutsideTheSelection(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	other := &VMOwnership{VMID: "vm-x", ComputerID: "computer-x", UserID: "u-x", DesktopID: PrimaryDesktopID, State: VMStateActive}
	addOwnership(f.reg, other)
	f.mgr.addDevice("vm-x", 4096, 4096)
	if _, err := f.reg.RequestMemory(other, 5120, 0, "host"); reqStatus(err) != http.StatusConflict {
		t.Fatalf("unselected = %v, want 409", err)
	}
	f.mgr.hmu.Lock()
	delete(f.mgr.dev, "vm-m")
	f.mgr.hmu.Unlock()
	if _, err := f.reg.RequestMemory(own, 5120, 0, "host"); reqStatus(err) != http.StatusConflict {
		t.Fatalf("no device = %v, want 409", err)
	}
	f.mgr.hmu.Lock()
	f.mgr.statusErr["vm-m"] = ErrMemoryHotplugBusy
	f.mgr.hmu.Unlock()
	if _, err := f.reg.RequestMemory(own, 5120, 0, "host"); reqStatus(err) != http.StatusServiceUnavailable {
		t.Fatalf("busy = %v, want 503", err)
	}
	hib := *own
	hib.State = VMStateHibernated
	if _, err := f.reg.RequestMemory(&hib, 5120, 0, "host"); reqStatus(err) != http.StatusConflict {
		t.Fatalf("hibernated = %v, want 409", err)
	}
	f.reg.SetMemoryHotplugConfig(MemoryHotplugConfig{Enabled: false, Only: []string{"*"}})
	if _, err := f.reg.RequestMemory(own, 5120, 0, "host"); reqStatus(err) != http.StatusNotFound {
		t.Fatalf("disabled = %v, want 404", err)
	}
}

// F4: Firecracker refuses the PATCH (guest driver not bound): the
// reservation is rolled back.
func TestMemoryGrowRollsBackWhenFirecrackerRefuses(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	f.mgr.setErr = errors.New("memory hotplug request rejected: Device is not active")
	f.mgr.setErr = errors.Join(ErrMemoryHotplugRejected, f.mgr.setErr)
	if _, err := f.reg.RequestMemory(own, 5120, 0, "host"); reqStatus(err) != http.StatusConflict {
		t.Fatalf("refused PATCH = %v, want 409", err)
	}
	if f.leaseBytes("vm-m") != 0 {
		t.Fatalf("reservation kept after a refused PATCH: %d", f.leaseBytes("vm-m"))
	}
	if v, _ := f.reg.MemoryView("vm-m"); v.Lease != nil || !strings.Contains(v.LastError, "not active") {
		t.Fatalf("view = %+v", v)
	}
}

// F8: an expired lease shrinks to base without anyone releasing it.
func TestMemoryLeaseExpiryShrinksToBase(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	if _, err := f.reg.RequestMemory(own, 5120, time.Minute, "guest"); err != nil {
		t.Fatal(err)
	}
	f.reg.memoryTick()
	if v, _ := f.reg.MemoryView("vm-m"); v.Phase != MemoryPhaseLeased {
		t.Fatalf("before expiry = %+v", v)
	}
	f.advance(61 * time.Second)
	f.reg.memoryTick()
	if v, _ := f.reg.MemoryView("vm-m"); v.Lease != nil || v.RequestedMiB != 4096 {
		t.Fatalf("expiry did not request base: %+v", v)
	}
	f.reg.memoryTick() // the guest unplugs asynchronously
	v, _ := f.reg.MemoryView("vm-m")
	if v.Lease != nil || v.EndReason != "expired" || v.Phase != MemoryPhaseBase || f.leaseBytes("vm-m") != 0 {
		t.Fatalf("after expiry = %+v reservation %d", v, f.leaseBytes("vm-m"))
	}
	if c := f.reg.MemorySummary().Counters; c.Expiries != 1 {
		t.Fatalf("expiries = %d", c.Expiries)
	}
}

// F9: a guest that cannot unplug everything is retried, reported as
// stalled, keeps its reservation for what is still plugged, and completes
// when it can.
func TestMemoryPartialUnplugRetriesThenStallsThenCompletes(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	if _, err := f.reg.RequestMemory(own, 4096+1024, 0, "host"); err != nil {
		t.Fatal(err)
	}
	f.reg.memoryTick()
	f.mgr.hmu.Lock()
	f.mgr.floor["vm-m"] = 384
	f.mgr.hmu.Unlock()
	if _, err := f.reg.RequestMemory(own, 0, 0, "host"); err != nil {
		t.Fatal(err)
	}
	f.reg.memoryTick()
	v, _ := f.reg.MemoryView("vm-m")
	if v.Phase != MemoryPhaseShrinking || v.PluggedMiB != 384 || f.leaseBytes("vm-m") != 384<<20 {
		t.Fatalf("partial = %+v reservation %d", v, f.leaseBytes("vm-m"))
	}
	before := len(f.mgr.patchLog())
	for i := 0; i < 10; i++ {
		f.advance(15 * time.Second)
		f.reg.memoryTick()
	}
	if retries := len(f.mgr.patchLog()) - before; retries < 8 {
		t.Fatalf("retries = %d, want one per 15 s", retries)
	}
	v, _ = f.reg.MemoryView("vm-m")
	if v.Phase != MemoryPhaseShrinkStalled {
		t.Fatalf("after 150 s = %+v, want shrink_stalled", v)
	}
	if c := f.reg.MemorySummary().Counters; c.ShrinkStalls != 1 {
		t.Fatalf("stalls = %d, want counted once", c.ShrinkStalls)
	}
	f.mgr.hmu.Lock()
	f.mgr.floor["vm-m"] = 0
	f.mgr.hmu.Unlock()
	f.reg.memoryTick()
	v, _ = f.reg.MemoryView("vm-m")
	if v.Phase != MemoryPhaseBase || f.leaseBytes("vm-m") != 0 {
		t.Fatalf("after the guest freed memory = %+v reservation %d", v, f.leaseBytes("vm-m"))
	}
	if c := f.reg.MemorySummary().Counters; c.ShrinksCompleted != 1 {
		t.Fatalf("shrinks completed = %d", c.ShrinksCompleted)
	}
}

// F12: memory plugged with no lease (a lease from before a vmctl restart)
// is shrunk by the scan.
func TestMemoryOrphanAfterRestartIsReclaimed(t *testing.T) {
	f := newMemoryFixture(t, nil)
	f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	f.mgr.hmu.Lock()
	f.mgr.dev["vm-m"].RequestedMiB, f.mgr.dev["vm-m"].PluggedMiB = 1024, 1024
	f.mgr.hmu.Unlock()
	f.reg.memoryTick()
	if got := f.mgr.patchLog(); len(got) == 0 || got[0] != "vm-m=0" {
		t.Fatalf("orphan not shrunk: %v", got)
	}
	f.reg.memoryTick()
	v, _ := f.reg.MemoryView("vm-m")
	if v.EndReason != "orphan" || v.Phase != MemoryPhaseBase {
		t.Fatalf("orphan view = %+v", v)
	}
	if c := f.reg.MemorySummary().Counters; c.OrphansReclaimed != 1 {
		t.Fatalf("orphans = %d", c.OrphansReclaimed)
	}
}

// F16: host memory under the floor revokes leases.
func TestMemoryLeasesRevokedBelowHostFloor(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	if _, err := f.reg.RequestMemory(own, 5120, 0, "guest"); err != nil {
		t.Fatal(err)
	}
	f.set(3*gib, 0)
	f.reg.memoryTick()
	v, _ := f.reg.MemoryView("vm-m")
	if v.Lease != nil || v.EndReason != "revoked_host_pressure" {
		t.Fatalf("lease not revoked under the floor: %+v", v)
	}
	if c := f.reg.MemorySummary().Counters; c.Revocations != 1 {
		t.Fatalf("revocations = %d", c.Revocations)
	}
}

// F15: a computer that stops drops its state and reservation.
func TestMemoryStateDroppedWhenComputerStops(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	if _, err := f.reg.RequestMemory(own, 5120, 0, "guest"); err != nil {
		t.Fatal(err)
	}
	f.reg.mu.Lock()
	f.reg.vmByID["vm-m"].State = VMStateHibernated
	f.reg.mu.Unlock()
	f.reg.memoryTick()
	if _, ok := f.reg.MemoryView("vm-m"); ok || f.leaseBytes("vm-m") != 0 {
		t.Fatalf("state or reservation kept for a stopped computer")
	}
}

// F1: the selection tokens; platform computers never.
func TestMemoryHotplugSelection(t *testing.T) {
	own := &VMOwnership{VMID: "vm-1", ComputerID: "c-1", UserID: "qa-user", DesktopID: PrimaryDesktopID}
	policy := WarmnessPolicyConfig{TestUserIDs: map[string]bool{"qa-user": true}}
	cfg := MemoryHotplugConfig{Enabled: true, RegionMiB: 2048}
	for token, want := range map[string]int{
		"":              0,
		"vm-1":          2048,
		"c-1":           2048,
		"owner:qa-user": 2048,
		"owner:other":   0,
		"class:test":    2048,
		"*":             2048,
		"vm-2":          0,
	} {
		cfg.Only = []string{token}
		if got := memoryHotplugMiBWith(own, cfg, normalizeWarmnessPolicyConfig(policy)); got != want {
			t.Fatalf("token %q = %d, want %d", token, got, want)
		}
	}
	cfg.Only = []string{"class:test"}
	if got := memoryHotplugMiBWith(own, cfg, WarmnessPolicyConfig{}); got != 0 {
		t.Fatal("class:test selected a computer not in the test tier")
	}
	cfg.Only = []string{"*"}
	cfg.Enabled = false
	if memoryHotplugMiBWith(own, cfg, policy) != 0 {
		t.Fatal("disabled flag selected a computer")
	}
	cfg.Enabled = true
	platform := &VMOwnership{VMID: "vm-p", UserID: UniversalWirePlatformOwnerID, DesktopID: UniversalWirePlatformDesktopID}
	if memoryHotplugMiBWith(platform, cfg, policy) != 0 {
		t.Fatal("a platform computer was selected")
	}
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

// F10, F11: the host route needs the transport and the credential; the
// guest route binds to the caller's own computer only.
func TestMemoryRoutesAuthorization(t *testing.T) {
	const token = "vmctl-memory-test-token-0123456789abcdef0123456789"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	f := newMemoryFixture(t, nil)
	f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	f.addComputer("vm-n", "computer-memory-n", "10.200.3.6")
	grow := map[string]any{"computer_id": "computer-memory-m", "target_mib": 5120}

	if rec := serveThroughHostauth(f.h.HandleMemory, http.MethodPost, "/internal/vmctl/memory", "127.0.0.1:5000", "", grow); rec.Code != http.StatusForbidden {
		t.Fatalf("loopback without credential = %d", rec.Code)
	}
	if rec := serveThroughHostauth(f.h.HandleMemory, http.MethodPost, "/internal/vmctl/memory", "10.200.3.2:41000", token, grow); rec.Code != http.StatusForbidden {
		t.Fatalf("guest address on the host route = %d", rec.Code)
	}
	rec := serveThroughHostauth(f.h.HandleMemory, http.MethodPost, "/internal/vmctl/memory", "127.0.0.1:5000", token, grow)
	if rec.Code != http.StatusOK {
		t.Fatalf("host with credential = %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveThroughHostauth(f.h.HandleMemory, http.MethodGet, "/internal/vmctl/memory", "127.0.0.1:5000", token, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"counters"`) {
		t.Fatalf("summary = %d %s", rec.Code, rec.Body.String())
	}

	guestGrow := map[string]any{"target_mib": 5120}
	if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodPost, "/internal/vmctl/memory/guest", "127.0.0.1:5000", token, guestGrow); rec.Code != http.StatusForbidden {
		t.Fatalf("loopback on the guest route = %d", rec.Code)
	}
	if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodPost, "/internal/vmctl/memory/guest", "10.200.9.9:41000", "", guestGrow); rec.Code != http.StatusForbidden {
		t.Fatalf("unbound guest = %d", rec.Code)
	}
	for _, body := range []map[string]any{
		{"target_mib": 5120, "computer_id": "computer-memory-m"},
		{"target_mib": 5120, "vm_id": "vm-m"},
	} {
		if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodPost, "/internal/vmctl/memory/guest", "10.200.3.6:41000", "", body); rec.Code != http.StatusForbidden {
			t.Fatalf("guest n naming computer m = %d", rec.Code)
		}
	}
	if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodPost, "/internal/vmctl/memory/guest", "10.200.3.6:41000", "", map[string]any{"target_mib": 5120, "ttl_seconds": 60, "wait_seconds": 1}); rec.Code != http.StatusOK {
		t.Fatalf("guest n on itself = %d %s", rec.Code, rec.Body.String())
	} else {
		var view MemoryHotplugView
		_ = json.Unmarshal(rec.Body.Bytes(), &view)
		if view.VMID != "vm-n" || view.Lease == nil || view.Lease.Holder != "guest" || view.PluggedMiB != 1024 {
			t.Fatalf("guest view = %+v", view)
		}
	}
	if v, _ := f.reg.MemoryView("vm-m"); v.Lease == nil || v.Lease.Holder != "host" {
		t.Fatalf("guest n changed computer m's lease: %+v", v)
	}
	if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodPost, "/internal/vmctl/memory/guest", "10.200.3.6:41000", "", map[string]any{"target_mib": 1, "extra": true}); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d", rec.Code)
	}
	if rec := serveThroughHostauth(f.h.HandleMemory, http.MethodGet, "/internal/vmctl/memory?computer_id=computer-memory-n", "127.0.0.1:5000", token, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"plugged_mib":1024`) {
		t.Fatalf("observe = %d %s", rec.Code, rec.Body.String())
	}
	f.mgr.hmu.Lock()
	delete(f.mgr.dev, "vm-n")
	f.mgr.hmu.Unlock()
	if rec := serveThroughHostauth(f.h.HandleMemory, http.MethodGet, "/internal/vmctl/memory?computer_id=computer-memory-n", "127.0.0.1:5000", token, nil); rec.Code != http.StatusConflict {
		t.Fatalf("observe without a device = %d %s", rec.Code, rec.Body.String())
	}
}

// F20 and the list field: current memory in the VM list, from the cache,
// with no Firecracker call.
func TestListShowsCurrentMemoryWithoutCallingFirecracker(t *testing.T) {
	f := newMemoryFixture(t, nil)
	own := f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	f.reg.admissionLedger.mu.Lock()
	f.reg.admissionLedger.initLocked()
	f.reg.admissionLedger.rss["vm-m"] = vmMemoryObservation{bytes: 1500 << 20, at: *f.now}
	f.reg.admissionLedger.mu.Unlock()
	if _, err := f.reg.RequestMemory(own, 5120, 0, "host"); err != nil {
		t.Fatal(err)
	}
	f.reg.memoryTick()
	f.mgr.hmu.Lock()
	calls := f.mgr.statusCalls
	f.mgr.hmu.Unlock()
	list, err := f.h.listOwnershipResponses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	f.mgr.hmu.Lock()
	after := f.mgr.statusCalls
	f.mgr.hmu.Unlock()
	if after != calls {
		t.Fatalf("the list called Firecracker %d times", after-calls)
	}
	var got *ownershipResponse
	for i := range list {
		if list[i].VMID == "vm-m" {
			got = &list[i]
		}
	}
	if got == nil || got.MemoryCurrentMiB != 5120 || got.MemoryBaseMiB != 4096 || got.MemoryPluggedMiB != 1024 ||
		got.MemoryRSSBytes != 1500<<20 || got.MemoryHotplugPhase != MemoryPhaseLeased || got.MemoryLeaseExpiresAt == "" || !got.MemoryHotplugSelected {
		t.Fatalf("list memory fields = %+v", got)
	}
}

// The guest helper: request, run, release; a refusal never runs fn.
func TestWithGuestMemoryLease(t *testing.T) {
	var mu sync.Mutex
	var targets []int
	refuse := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body memoryRequestBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		targets = append(targets, body.TargetMiB)
		deny := refuse && body.TargetMiB > 0
		mu.Unlock()
		if r.URL.Path != "/internal/vmctl/memory/guest" || body.VMID != "" || body.ComputerID != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if deny {
			writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "host memory PSI some avg10 2.00 is above 1.00"})
			return
		}
		writeVMCTLJSON(w, http.StatusOK, MemoryHotplugView{VMID: "vm-g", RequestedMiB: body.TargetMiB, CurrentMiB: body.TargetMiB})
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	ran := false
	ctx, cancel := context.WithCancel(context.Background())
	err := WithGuestMemoryLease(ctx, c, 5120, time.Minute, time.Second, func(_ context.Context, v *MemoryHotplugView) error {
		ran = true
		cancel() // a cancelled run still releases
		if v.CurrentMiB != 5120 {
			t.Errorf("lease view = %+v", v)
		}
		return nil
	})
	if err != nil || !ran {
		t.Fatalf("lease run = %v ran=%t", err, ran)
	}
	mu.Lock()
	if len(targets) != 2 || targets[0] != 5120 || targets[1] != 0 {
		t.Fatalf("requests = %v, want grow then release", targets)
	}
	refuse, targets = true, nil
	mu.Unlock()

	ran = false
	err = WithGuestMemoryLease(context.Background(), c, 5120, time.Minute, 0, func(context.Context, *MemoryHotplugView) error {
		ran = true
		return nil
	})
	var leaseErr *MemoryLeaseError
	if !errors.As(err, &leaseErr) || leaseErr.Status != http.StatusServiceUnavailable || ran {
		t.Fatalf("refused lease = %v ran=%t", err, ran)
	}
}

// The guest reads its own memory (its base, for the lease set's target)
// by the same tap identity; a loopback caller is refused.
func TestGuestMemoryRouteReadsOwnView(t *testing.T) {
	f := newMemoryFixture(t, nil)
	f.addComputer("vm-m", "computer-memory-m", "10.200.3.2")
	f.addComputer("vm-n", "computer-memory-n", "10.200.3.6")
	rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodGet, "/internal/vmctl/memory/guest", "10.200.3.6:41000", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest GET = %d %s", rec.Code, rec.Body.String())
	}
	var view MemoryHotplugView
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if view.VMID != "vm-n" || view.BaseMiB != 4096 || view.CurrentMiB != 4096 {
		t.Fatalf("guest view = %+v", view)
	}
	if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodGet, "/internal/vmctl/memory/guest", "127.0.0.1:5000", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("loopback guest GET = %d", rec.Code)
	}
	if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodGet, "/internal/vmctl/memory/guest", "10.200.9.9:41000", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("unbound guest GET = %d", rec.Code)
	}
	if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodDelete, "/internal/vmctl/memory/guest", "10.200.3.6:41000", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("guest DELETE = %d", rec.Code)
	}
	f.reg.SetMemoryHotplugConfig(MemoryHotplugConfig{})
	if rec := serveThroughHostauth(f.h.HandleGuestMemory, http.MethodGet, "/internal/vmctl/memory/guest", "10.200.3.6:41000", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("guest GET with hotplug off = %d", rec.Code)
	}
}
