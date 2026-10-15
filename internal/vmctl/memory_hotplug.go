package vmctl

// Memory hotplug leases (docs/reports/memory-hotplug-controller-design-2026-10-14.md).
//
// A computer vmctl selects boots with a virtio-mem region beside its base
// memory (vmmanager/memory_hotplug.go). A grow is a lease: a target total,
// a holder and an expiry. Grows are admitted against measured host memory
// and PSI, held as reservations in the start admission ledger until the
// unplug completes, and shrunk back to base on release, expiry, host
// pressure, or a vmctl restart (no lease survives one). The guest is asked
// to unplug; a guest that cannot is retried and reported, never forced.
//
// Rollback: unset VM_MEMORY_HOTPLUG (new boots get no device; routes 404).

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	memoryLeaseNeedSource = "memory_lease"

	MemoryPhaseBase          = "base"
	MemoryPhaseGrowing       = "growing"
	MemoryPhaseLeased        = "leased"
	MemoryPhaseShrinking     = "shrinking"
	MemoryPhaseShrinkStalled = "shrink_stalled"

	MemoryLeaseHolderHost  = "host"
	MemoryLeaseHolderGuest = "guest"

	memoryWaitPoll = 250 * time.Millisecond
	memoryWaitMax  = 60 * time.Second
)

// Errors from the memory manager; the vmctl adapter maps vmmanager's.
var (
	ErrMemoryHotplugUnavailable = errors.New("memory hotplug unavailable")
	ErrMemoryHotplugBusy        = errors.New("memory hotplug: VM operation in progress")
	ErrMemoryHotplugRejected    = errors.New("memory hotplug request rejected")
)

// MemoryHotplugStatus is a running VM's base memory and hotplug device.
type MemoryHotplugStatus struct {
	BaseMiB      int
	TotalMiB     int
	BlockMiB     int
	SlotMiB      int
	PluggedMiB   int
	RequestedMiB int
}

// memoryHotplugManager is implemented by the Firecracker manager adapter.
// The contract is backend neutral: Cloud Hypervisor's vm.resize takes the
// total (base + requested).
type memoryHotplugManager interface {
	MemoryHotplugStatus(vmID string) (MemoryHotplugStatus, error)
	SetMemoryHotplugRequestedMiB(vmID string, requestedMiB int) error
}

// MemoryHotplugConfig selects computers and bounds their leases.
type MemoryHotplugConfig struct {
	Enabled bool
	// Only lists VM ids, computer ids, owner:<user_id>, class:test, or *.
	// Empty selects no computer.
	Only []string
	// RegionMiB is the hotplug region a selected computer boots with.
	RegionMiB int
	// LeaseMaxMiB is the most one computer may hold above its base.
	LeaseMaxMiB int
	DefaultTTL  time.Duration
	MaxTTL      time.Duration
	// GrowMaxPSI refuses a grow while host memory PSI some avg10 is above it.
	GrowMaxPSI       float64
	ShrinkRetry      time.Duration
	ShrinkStallAfter time.Duration
	TickInterval     time.Duration
	ScanInterval     time.Duration
}

func DefaultMemoryHotplugConfig() MemoryHotplugConfig {
	return MemoryHotplugConfig{
		RegionMiB:        4096,
		LeaseMaxMiB:      2048,
		DefaultTTL:       10 * time.Minute,
		MaxTTL:           30 * time.Minute,
		GrowMaxPSI:       1.0,
		ShrinkRetry:      15 * time.Second,
		ShrinkStallAfter: 2 * time.Minute,
		TickInterval:     5 * time.Second,
		ScanInterval:     time.Minute,
	}
}

func normalizeMemoryHotplugConfig(cfg MemoryHotplugConfig) MemoryHotplugConfig {
	def := DefaultMemoryHotplugConfig()
	if cfg.RegionMiB <= 0 {
		cfg.RegionMiB = def.RegionMiB
	}
	if cfg.LeaseMaxMiB <= 0 {
		cfg.LeaseMaxMiB = def.LeaseMaxMiB
	}
	if cfg.LeaseMaxMiB > cfg.RegionMiB {
		cfg.LeaseMaxMiB = cfg.RegionMiB
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = def.DefaultTTL
	}
	if cfg.MaxTTL <= 0 {
		cfg.MaxTTL = def.MaxTTL
	}
	if cfg.DefaultTTL > cfg.MaxTTL {
		cfg.DefaultTTL = cfg.MaxTTL
	}
	if cfg.GrowMaxPSI <= 0 {
		cfg.GrowMaxPSI = def.GrowMaxPSI
	}
	if cfg.ShrinkRetry <= 0 {
		cfg.ShrinkRetry = def.ShrinkRetry
	}
	if cfg.ShrinkStallAfter <= 0 {
		cfg.ShrinkStallAfter = def.ShrinkStallAfter
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = def.TickInterval
	}
	if cfg.ScanInterval <= 0 {
		cfg.ScanInterval = def.ScanInterval
	}
	return cfg
}

// MemoryHotplugConfigFromEnv reads VM_MEMORY_HOTPLUG* and VMCTL_MEMORY_*.
func MemoryHotplugConfigFromEnv() MemoryHotplugConfig {
	cfg := DefaultMemoryHotplugConfig()
	switch strings.ToLower(strings.TrimSpace(os.Getenv("VM_MEMORY_HOTPLUG"))) {
	case "1", "true", "on", "yes":
		cfg.Enabled = true
	}
	for _, id := range strings.Split(os.Getenv("VM_MEMORY_HOTPLUG_ONLY"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			cfg.Only = append(cfg.Only, id)
		}
	}
	for name, dst := range map[string]*int{
		"VM_MEMORY_HOTPLUG_TOTAL_MIB": &cfg.RegionMiB,
		"VMCTL_MEMORY_LEASE_MAX_MIB":  &cfg.LeaseMaxMiB,
	} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				*dst = n
			}
		}
	}
	for name, dst := range map[string]*time.Duration{
		"VMCTL_MEMORY_LEASE_TTL":     &cfg.DefaultTTL,
		"VMCTL_MEMORY_LEASE_MAX_TTL": &cfg.MaxTTL,
	} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				*dst = d
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv("VMCTL_MEMORY_GROW_MAX_PSI")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.GrowMaxPSI = f
		}
	}
	return normalizeMemoryHotplugConfig(cfg)
}

// SetMemoryHotplugConfig configures selection and leases.
func (r *OwnershipRegistry) SetMemoryHotplugConfig(cfg MemoryHotplugConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memoryHotplug = normalizeMemoryHotplugConfig(cfg)
}

func (r *OwnershipRegistry) memoryHotplugConfig() MemoryHotplugConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return normalizeMemoryHotplugConfig(r.memoryHotplug)
}

// memoryHotplugMiBWith is the selection: the region a computer boots with,
// or 0. Platform computers are never selected.
func memoryHotplugMiBWith(own *VMOwnership, cfg MemoryHotplugConfig, policy WarmnessPolicyConfig) int {
	if own == nil || !cfg.Enabled || isPlatformOwnership(own) {
		return 0
	}
	cfg = normalizeMemoryHotplugConfig(cfg)
	for _, token := range cfg.Only {
		token = strings.TrimSpace(token)
		switch {
		case token == "":
		case token == "*",
			token == own.VMID,
			own.ComputerID != "" && token == own.ComputerID,
			strings.HasPrefix(token, "owner:") && own.UserID != "" && strings.TrimPrefix(token, "owner:") == own.UserID,
			token == "class:test" && warmnessClassForOwnership(own, policy) == WarmnessClassTest:
			return cfg.RegionMiB
		}
	}
	return 0
}

// memoryHotplugMiBFor is memoryHotplugMiBWith for a caller not holding r.mu.
func (r *OwnershipRegistry) memoryHotplugMiBFor(own *VMOwnership) int {
	r.mu.RLock()
	cfg, policy := r.memoryHotplug, r.warmnessPolicy
	r.mu.RUnlock()
	return memoryHotplugMiBWith(own, cfg, policy)
}

// --- controller state ---------------------------------------------------

type memoryLease struct {
	targetMiB  int // total guest memory
	hotplugMiB int // above the base
	holder     string
	grantedAt  time.Time
	expiresAt  time.Time
}

type memoryVMState struct {
	vmID        string
	computerID  string
	status      MemoryHotplugStatus
	observedAt  time.Time
	lease       *memoryLease
	phase       string
	shrinkSince time.Time
	lastPatch   time.Time
	stallNoted  bool
	lastError   string
	endReason   string
}

// MemoryHotplugCounters are lifetime counts since vmctl started.
type MemoryHotplugCounters struct {
	Grants           uint64 `json:"grants"`
	Renewals         uint64 `json:"renewals"`
	Refusals         uint64 `json:"refusals"`
	Releases         uint64 `json:"releases"`
	Expiries         uint64 `json:"expiries"`
	Revocations      uint64 `json:"revocations"`
	OrphansReclaimed uint64 `json:"orphans_reclaimed"`
	ShrinksCompleted uint64 `json:"shrinks_completed"`
	ShrinkStalls     uint64 `json:"shrink_stalls"`
}

// memoryController holds the lease state. opMu serializes requests and
// ticks (they call Firecracker); mu guards the cache that the list reads.
// Lock order: opMu, then r.mu (briefly, never held across), then mu, then
// the admission ledger's mutex.
type memoryController struct {
	opMu     sync.Mutex
	mu       sync.Mutex
	vms      map[string]*memoryVMState
	counters MemoryHotplugCounters
	lastScan time.Time
}

func (c *memoryController) stateLocked(vmID, computerID string) *memoryVMState {
	if c.vms == nil {
		c.vms = map[string]*memoryVMState{}
	}
	st := c.vms[vmID]
	if st == nil {
		st = &memoryVMState{vmID: vmID, computerID: computerID, phase: MemoryPhaseBase}
		c.vms[vmID] = st
	}
	if computerID != "" {
		st.computerID = computerID
	}
	return st
}

// MemoryLeaseView is a lease as the API shows it.
type MemoryLeaseView struct {
	TargetMiB int    `json:"target_mib"`
	Holder    string `json:"holder"`
	GrantedAt string `json:"granted_at"`
	ExpiresAt string `json:"expires_at"`
}

// MemoryHotplugView is one computer's memory as the API shows it. Sizes
// are totals (base + hotplug) except PluggedMiB and RegionMiB.
type MemoryHotplugView struct {
	VMID         string           `json:"vm_id"`
	ComputerID   string           `json:"computer_id,omitempty"`
	BaseMiB      int              `json:"base_mib"`
	RegionMiB    int              `json:"region_mib"`
	MaxMiB       int              `json:"max_mib"`
	RequestedMiB int              `json:"requested_mib"`
	PluggedMiB   int              `json:"plugged_mib"`
	CurrentMiB   int              `json:"current_mib"`
	Phase        string           `json:"phase"`
	Lease        *MemoryLeaseView `json:"lease,omitempty"`
	ObservedAt   string           `json:"observed_at,omitempty"`
	LastError    string           `json:"last_error,omitempty"`
	EndReason    string           `json:"end_reason,omitempty"`
	RSSBytes     uint64           `json:"rss_bytes,omitempty"`
}

// MemoryHotplugSummary is GET /internal/vmctl/memory without a computer.
type MemoryHotplugSummary struct {
	Enabled  bool                  `json:"enabled"`
	Counters MemoryHotplugCounters `json:"counters"`
	VMs      []MemoryHotplugView   `json:"vms"`
}

func (st *memoryVMState) view(cfg MemoryHotplugConfig) MemoryHotplugView {
	v := MemoryHotplugView{
		VMID:         st.vmID,
		ComputerID:   st.computerID,
		BaseMiB:      st.status.BaseMiB,
		RegionMiB:    st.status.TotalMiB,
		MaxMiB:       st.status.BaseMiB + min(cfg.LeaseMaxMiB, st.status.TotalMiB),
		RequestedMiB: st.status.BaseMiB + st.status.RequestedMiB,
		PluggedMiB:   st.status.PluggedMiB,
		CurrentMiB:   st.status.BaseMiB + st.status.PluggedMiB,
		Phase:        st.phase,
		LastError:    st.lastError,
		EndReason:    st.endReason,
	}
	if !st.observedAt.IsZero() {
		v.ObservedAt = st.observedAt.UTC().Format(time.RFC3339)
	}
	if st.lease != nil {
		v.Lease = &MemoryLeaseView{
			TargetMiB: st.lease.targetMiB,
			Holder:    st.lease.holder,
			GrantedAt: st.lease.grantedAt.UTC().Format(time.RFC3339),
			ExpiresAt: st.lease.expiresAt.UTC().Format(time.RFC3339),
		}
	}
	return v
}

// MemoryRequestError is a refused or failed memory request with its HTTP
// status.
type MemoryRequestError struct {
	Status int
	Reason string
}

func (e *MemoryRequestError) Error() string { return e.Reason }

func memoryErr(status int, format string, args ...any) error {
	return &MemoryRequestError{Status: status, Reason: fmt.Sprintf(format, args...)}
}

func (r *OwnershipRegistry) memoryManager() (memoryHotplugManager, bool) {
	r.mu.RLock()
	mgr := r.vmManager
	r.mu.RUnlock()
	hm, ok := mgr.(memoryHotplugManager)
	return hm, ok && hm != nil
}

func mapManagerMemoryErr(err error) error {
	switch {
	case errors.Is(err, ErrMemoryHotplugBusy):
		return memoryErr(http.StatusServiceUnavailable, "%v", err)
	case errors.Is(err, ErrMemoryHotplugUnavailable), errors.Is(err, ErrMemoryHotplugRejected):
		return memoryErr(http.StatusConflict, "%v", err)
	default:
		return memoryErr(http.StatusBadGateway, "%v", err)
	}
}

// RequestMemory sets a computer's total memory: above its base it grows
// under a lease of ttl (0: the default; capped), at or below its base it
// releases. holder is "host" or "guest".
func (r *OwnershipRegistry) RequestMemory(own *VMOwnership, targetMiB int, ttl time.Duration, holder string) (MemoryHotplugView, error) {
	cfg := r.memoryHotplugConfig()
	if !cfg.Enabled {
		return MemoryHotplugView{}, memoryErr(http.StatusNotFound, "memory hotplug is disabled")
	}
	if own == nil {
		return MemoryHotplugView{}, memoryErr(http.StatusNotFound, "computer not found")
	}
	if targetMiB < 0 {
		return MemoryHotplugView{}, memoryErr(http.StatusBadRequest, "target_mib must not be negative")
	}
	if r.memoryHotplugMiBFor(own) == 0 {
		return MemoryHotplugView{}, memoryErr(http.StatusConflict, "computer %s is not selected for memory hotplug", own.ComputerID)
	}
	if own.State != VMStateActive {
		return MemoryHotplugView{}, memoryErr(http.StatusConflict, "computer %s is %s, not active", own.ComputerID, own.State)
	}
	mgr, ok := r.memoryManager()
	if !ok {
		return MemoryHotplugView{}, memoryErr(http.StatusServiceUnavailable, "no memory hotplug manager")
	}
	if ttl <= 0 {
		ttl = cfg.DefaultTTL
	}
	if ttl > cfg.MaxTTL {
		ttl = cfg.MaxTTL
	}

	ctl := &r.memoryCtl
	ctl.opMu.Lock()
	defer ctl.opMu.Unlock()

	status, err := mgr.MemoryHotplugStatus(own.VMID)
	if err != nil {
		return MemoryHotplugView{}, mapManagerMemoryErr(err)
	}
	now := r.now()
	ctl.mu.Lock()
	st := ctl.stateLocked(own.VMID, own.ComputerID)
	st.status, st.observedAt = status, now
	var current *memoryLease
	if st.lease != nil {
		cp := *st.lease
		current = &cp
	}
	ctl.mu.Unlock()

	if targetMiB <= status.BaseMiB {
		return r.releaseMemoryLease(mgr, own, cfg, "released")
	}
	maxMiB := status.BaseMiB + min(cfg.LeaseMaxMiB, status.TotalMiB)
	if targetMiB > maxMiB {
		r.noteMemoryRefusal()
		return MemoryHotplugView{}, memoryErr(http.StatusUnprocessableEntity, "target %d MiB exceeds this computer's maximum %d MiB (base %d + lease max %d)", targetMiB, maxMiB, status.BaseMiB, min(cfg.LeaseMaxMiB, status.TotalMiB))
	}
	block := status.BlockMiB
	if block <= 0 {
		block = 2
	}
	hotplug := targetMiB - status.BaseMiB
	if rem := hotplug % block; rem != 0 {
		hotplug += block - rem
	}
	if hotplug > status.TotalMiB {
		hotplug = status.TotalMiB - status.TotalMiB%block
	}
	target := status.BaseMiB + hotplug

	renewal := current != nil && hotplug <= current.hotplugMiB
	if !renewal {
		prev := uint64(0)
		if current != nil {
			prev = uint64(current.hotplugMiB) << 20
		}
		if err := r.admitMemoryGrow(own, uint64(hotplug)<<20, prev); err != nil {
			r.noteMemoryRefusal()
			return MemoryHotplugView{}, err
		}
	} else {
		// A smaller or equal renewal keeps the reservation at least what is
		// still plugged; the tick lowers it as the guest unplugs.
		r.setLeaseReservation(own.VMID, uint64(max(hotplug, status.PluggedMiB))<<20)
	}
	if err := mgr.SetMemoryHotplugRequestedMiB(own.VMID, hotplug); err != nil {
		// Roll the reservation back to what the previous lease held.
		prev := 0
		if current != nil {
			prev = max(current.hotplugMiB, status.PluggedMiB)
		} else {
			prev = status.PluggedMiB
		}
		r.setLeaseReservation(own.VMID, uint64(prev)<<20)
		ctl.mu.Lock()
		st.lastError = err.Error()
		ctl.mu.Unlock()
		return MemoryHotplugView{}, mapManagerMemoryErr(err)
	}
	ctl.mu.Lock()
	lease := &memoryLease{targetMiB: target, hotplugMiB: hotplug, holder: holder, grantedAt: now, expiresAt: now.Add(ttl)}
	if current != nil {
		lease.grantedAt = current.grantedAt
	}
	st.lease = lease
	st.status.RequestedMiB = hotplug
	st.lastError, st.endReason = "", ""
	st.shrinkSince, st.stallNoted = time.Time{}, false
	st.phase = leasedPhase(st.status)
	if renewal {
		ctl.counters.Renewals++
	} else {
		ctl.counters.Grants++
	}
	view := st.view(cfg)
	ctl.mu.Unlock()
	log.Printf("vmctl: memory lease vm=%s computer=%s holder=%s target_mib=%d hotplug_mib=%d expires=%s renewal=%t",
		own.VMID, own.ComputerID, holder, target, hotplug, lease.expiresAt.UTC().Format(time.RFC3339), renewal)
	return view, nil
}

func leasedPhase(st MemoryHotplugStatus) string {
	if st.PluggedMiB >= st.RequestedMiB {
		return MemoryPhaseLeased
	}
	return MemoryPhaseGrowing
}

// releaseMemoryLease ends a computer's lease and asks for base. Caller
// holds opMu.
func (r *OwnershipRegistry) releaseMemoryLease(mgr memoryHotplugManager, own *VMOwnership, cfg MemoryHotplugConfig, reason string) (MemoryHotplugView, error) {
	ctl := &r.memoryCtl
	now := r.now()
	ctl.mu.Lock()
	st := ctl.stateLocked(own.VMID, own.ComputerID)
	had := st.lease != nil
	ctl.mu.Unlock()
	err := mgr.SetMemoryHotplugRequestedMiB(own.VMID, 0)
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	if had {
		switch reason {
		case "expired":
			ctl.counters.Expiries++
		case "revoked_host_pressure":
			ctl.counters.Revocations++
		case "orphan":
			ctl.counters.OrphansReclaimed++
		default:
			ctl.counters.Releases++
		}
	} else if reason == "orphan" {
		ctl.counters.OrphansReclaimed++
	}
	st.lease = nil
	st.endReason = reason
	st.lastPatch = now
	if err != nil {
		st.lastError = err.Error()
	} else {
		st.status.RequestedMiB = 0
		st.lastError = ""
	}
	if st.status.PluggedMiB > 0 {
		if st.shrinkSince.IsZero() {
			st.shrinkSince = now
		}
		if st.phase != MemoryPhaseShrinkStalled {
			st.phase = MemoryPhaseShrinking
		}
	} else if st.status.RequestedMiB == 0 {
		st.phase = MemoryPhaseBase
	} else {
		st.phase = MemoryPhaseShrinking
	}
	log.Printf("vmctl: memory lease ended vm=%s computer=%s reason=%s plugged_mib=%d err=%v", own.VMID, own.ComputerID, reason, st.status.PluggedMiB, err)
	view := st.view(cfg)
	if err != nil {
		return view, mapManagerMemoryErr(err)
	}
	return view, nil
}

func (r *OwnershipRegistry) noteMemoryRefusal() {
	r.memoryCtl.mu.Lock()
	r.memoryCtl.counters.Refusals++
	r.memoryCtl.mu.Unlock()
}

// setLeaseReservation sets (or clears, at 0) a computer's lease bytes in
// the admission ledger.
func (r *OwnershipRegistry) setLeaseReservation(vmID string, bytes uint64) {
	led := &r.admissionLedger
	led.mu.Lock()
	defer led.mu.Unlock()
	led.initLocked()
	if bytes == 0 {
		delete(led.leases, vmID)
		return
	}
	led.leases[vmID] = bytes
}

// admitMemoryGrow admits a lease of leaseBytes for own (replacing prevBytes
// it already holds) and reserves it, under the ledger lock so concurrent
// grows and starts cannot pass one reading. A grow is optional: a missing
// host sample refuses.
func (r *OwnershipRegistry) admitMemoryGrow(own *VMOwnership, leaseBytes, prevBytes uint64) error {
	r.mu.RLock()
	pcfg, acfg, sampler, hcfg := r.pressureReclaim, r.hostAdmission, r.pressureSampler, r.memoryHotplug
	r.mu.RUnlock()
	pcfg = normalizePressureReclaimConfig(pcfg)
	acfg = normalizeHostAdmissionConfig(acfg)
	hcfg = normalizeMemoryHotplugConfig(hcfg)
	if pcfg.MinMemoryAvailableBytes == 0 && pcfg.MinMemoryAvailablePercent == 0 {
		def := DefaultPressureReclaimConfig()
		pcfg.MinMemoryAvailableBytes, pcfg.MinMemoryAvailablePercent = def.MinMemoryAvailableBytes, def.MinMemoryAvailablePercent
	}
	if sampler == nil {
		sampler = sampleHostPressure
	}

	led := &r.admissionLedger
	led.mu.Lock()
	defer led.mu.Unlock()
	led.initLocked()
	sample := sampler(pcfg)
	now := r.now()
	reserved, _ := led.outstandingLocked(now, acfg)
	if cur := led.leases[own.VMID]; cur > 0 && reserved >= cur {
		reserved -= cur
	}
	floor := memoryFloorBytes(pcfg, sample.MemoryTotalBytes)
	d := HostAdmissionDecision{
		At:             now.UTC().Format(time.RFC3339),
		Mode:           "memory_grow",
		VMID:           own.VMID,
		ComputerID:     own.ComputerID,
		NeedBytes:      leaseBytes,
		NeedSource:     memoryLeaseNeedSource,
		AvailableBytes: sample.MemoryAvailableBytes,
		ReservedBytes:  reserved,
		FloorBytes:     floor,
		HeadroomBytes:  acfg.HeadroomBytes,
	}
	required := floor + acfg.HeadroomBytes + reserved + leaseBytes
	var refusal error
	switch {
	case sample.MemoryAvailableBytes == 0:
		refusal = memoryErr(http.StatusServiceUnavailable, "host memory sample unavailable (%s); a grow is not admitted blind", sample.ObservationError)
	case sample.MemorySomeAvg10 > hcfg.GrowMaxPSI:
		refusal = memoryErr(http.StatusServiceUnavailable, "host memory PSI some avg10 %.2f is above %.2f", sample.MemorySomeAvg10, hcfg.GrowMaxPSI)
	case sample.MemoryAvailableBytes < required:
		refusal = memoryErr(http.StatusServiceUnavailable, "host memory available %d MiB; a %d MiB lease needs %d MiB (floor %d + headroom %d + reserved %d + lease)",
			sample.MemoryAvailableBytes>>20, leaseBytes>>20, required>>20, floor>>20, acfg.HeadroomBytes>>20, reserved>>20)
	}
	if refusal != nil {
		d.Decision = "refuse"
	} else {
		d.Decision = "admit"
		led.leases[own.VMID] = max(leaseBytes, prevBytes)
	}
	led.recordLocked(d)
	logAdmissionDecision(d)
	return refusal
}

// MemoryView returns the cached view of one computer, without calling
// Firecracker. ok is false when the controller has never observed it.
func (r *OwnershipRegistry) MemoryView(vmID string) (MemoryHotplugView, bool) {
	cfg := r.memoryHotplugConfig()
	ctl := &r.memoryCtl
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	st := ctl.vms[vmID]
	if st == nil {
		return MemoryHotplugView{}, false
	}
	return st.view(cfg), true
}

// MemorySummary is every observed computer plus the counters.
func (r *OwnershipRegistry) MemorySummary() MemoryHotplugSummary {
	cfg := r.memoryHotplugConfig()
	ctl := &r.memoryCtl
	ctl.mu.Lock()
	out := MemoryHotplugSummary{Enabled: cfg.Enabled, Counters: ctl.counters, VMs: make([]MemoryHotplugView, 0, len(ctl.vms))}
	for _, st := range ctl.vms {
		out.VMs = append(out.VMs, st.view(cfg))
	}
	ctl.mu.Unlock()
	rss := r.latestVMResidentBytes()
	for i := range out.VMs {
		out.VMs[i].RSSBytes = rss[out.VMs[i].VMID]
	}
	sort.Slice(out.VMs, func(i, j int) bool { return out.VMs[i].VMID < out.VMs[j].VMID })
	return out
}

// latestVMResidentBytes is the admission ledger's latest RSS sample per VM.
func (r *OwnershipRegistry) latestVMResidentBytes() map[string]uint64 {
	led := &r.admissionLedger
	led.mu.Lock()
	defer led.mu.Unlock()
	out := make(map[string]uint64, len(led.rss))
	for vmID, obs := range led.rss {
		out[vmID] = obs.bytes
	}
	return out
}

// WaitMemory polls a computer until its plug or unplug settles or wait
// passes, and returns the latest view.
func (r *OwnershipRegistry) WaitMemory(ctx context.Context, own *VMOwnership, wait time.Duration) MemoryHotplugView {
	if wait > memoryWaitMax {
		wait = memoryWaitMax
	}
	deadline := time.Now().Add(wait)
	for {
		view, _ := r.ObserveMemory(own)
		settled := view.PluggedMiB == view.RequestedMiB-view.BaseMiB
		if settled || wait <= 0 || time.Now().After(deadline) {
			return view
		}
		select {
		case <-ctx.Done():
			return view
		case <-time.After(memoryWaitPoll):
		}
	}
}

// ObserveMemory reads one computer's hotplug status from its Firecracker
// (taking opMu) and returns the refreshed view; an error means the
// computer has no reachable device (or is busy).
func (r *OwnershipRegistry) ObserveMemory(own *VMOwnership) (MemoryHotplugView, error) {
	cfg := r.memoryHotplugConfig()
	ctl := &r.memoryCtl
	mgr, ok := r.memoryManager()
	if !ok {
		return MemoryHotplugView{}, memoryErr(http.StatusServiceUnavailable, "no memory hotplug manager")
	}
	ctl.opMu.Lock()
	status, err := mgr.MemoryHotplugStatus(own.VMID)
	ctl.opMu.Unlock()
	if err != nil {
		view, _ := r.MemoryView(own.VMID)
		return view, mapManagerMemoryErr(err)
	}
	ctl.mu.Lock()
	st := ctl.stateLocked(own.VMID, own.ComputerID)
	st.status, st.observedAt = status, r.now()
	r.updatePhaseLocked(st, cfg)
	ctl.mu.Unlock()
	view, _ := r.MemoryView(own.VMID)
	return view, nil
}

// updatePhaseLocked derives the phase from the lease and the last status.
// Caller holds memoryCtl.mu.
func (r *OwnershipRegistry) updatePhaseLocked(st *memoryVMState, cfg MemoryHotplugConfig) {
	now := r.now()
	if st.lease != nil {
		st.phase = leasedPhase(st.status)
		return
	}
	if st.status.PluggedMiB == 0 && st.status.RequestedMiB == 0 {
		if st.phase == MemoryPhaseShrinking || st.phase == MemoryPhaseShrinkStalled {
			r.memoryCtl.counters.ShrinksCompleted++
		}
		st.phase = MemoryPhaseBase
		st.shrinkSince, st.stallNoted = time.Time{}, false
		return
	}
	if st.shrinkSince.IsZero() {
		st.shrinkSince = now
	}
	if now.Sub(st.shrinkSince) >= cfg.ShrinkStallAfter {
		if !st.stallNoted {
			r.memoryCtl.counters.ShrinkStalls++
			st.stallNoted = true
			log.Printf("vmctl: memory shrink stalled vm=%s computer=%s plugged_mib=%d for %s; retrying", st.vmID, st.computerID, st.status.PluggedMiB, now.Sub(st.shrinkSince).Round(time.Second))
		}
		st.phase = MemoryPhaseShrinkStalled
		return
	}
	st.phase = MemoryPhaseShrinking
}

// memoryTick runs one controller pass: lease expiry, revocation under host
// pressure, shrink retries, reservations, and (every ScanInterval) a scan
// of selected computers for memory plugged without a lease.
func (r *OwnershipRegistry) memoryTick() {
	cfg := r.memoryHotplugConfig()
	mgr, ok := r.memoryManager()
	if !ok {
		return
	}
	ctl := &r.memoryCtl
	ctl.opMu.Lock()
	defer ctl.opMu.Unlock()
	now := r.now()

	byVM := map[string]*VMOwnership{}
	for _, own := range r.ListOwnerships() {
		if own != nil && own.State == VMStateActive {
			byVM[own.VMID] = own
		}
	}

	ctl.mu.Lock()
	scan := now.Sub(ctl.lastScan) >= cfg.ScanInterval
	if scan {
		ctl.lastScan = now
	}
	ids := make([]string, 0, len(ctl.vms))
	leases := 0
	for id, st := range ctl.vms {
		ids = append(ids, id)
		if st.lease != nil {
			leases++
		}
	}
	ctl.mu.Unlock()
	if scan && cfg.Enabled {
		known := map[string]bool{}
		for _, id := range ids {
			known[id] = true
		}
		for vmID, own := range byVM {
			if !known[vmID] && r.memoryHotplugMiBFor(own) > 0 {
				ids = append(ids, vmID)
			}
		}
	}

	revoke := false
	if leases > 0 {
		revoke = r.hostBelowMemoryFloor()
	}

	sort.Strings(ids)
	for _, vmID := range ids {
		own := byVM[vmID]
		if own == nil {
			r.dropMemoryState(vmID)
			continue
		}
		status, err := mgr.MemoryHotplugStatus(vmID)
		if err != nil {
			if errors.Is(err, ErrMemoryHotplugBusy) {
				continue
			}
			if errors.Is(err, ErrMemoryHotplugUnavailable) {
				r.dropMemoryState(vmID)
				continue
			}
			ctl.mu.Lock()
			if st := ctl.vms[vmID]; st != nil {
				st.lastError = err.Error()
			}
			ctl.mu.Unlock()
			continue
		}
		ctl.mu.Lock()
		st := ctl.stateLocked(vmID, own.ComputerID)
		st.status, st.observedAt = status, now
		var endReason string
		switch {
		case st.lease != nil && !now.Before(st.lease.expiresAt):
			endReason = "expired"
		case st.lease != nil && revoke:
			endReason = "revoked_host_pressure"
		case st.lease == nil && status.RequestedMiB > 0 && st.phase == MemoryPhaseBase:
			// Plugged or requested with no lease this vmctl granted: a lease
			// from before a restart. Restarts end work.
			endReason = "orphan"
		}
		ctl.mu.Unlock()
		if endReason != "" {
			_, _ = r.releaseMemoryLease(mgr, own, cfg, endReason)
		}

		ctl.mu.Lock()
		st = ctl.stateLocked(vmID, own.ComputerID)
		retry := st.lease == nil && (st.status.PluggedMiB > 0 || st.status.RequestedMiB > 0) && now.Sub(st.lastPatch) >= cfg.ShrinkRetry
		ctl.mu.Unlock()
		if retry {
			// Re-raise the request so the guest driver tries again.
			err := mgr.SetMemoryHotplugRequestedMiB(vmID, 0)
			ctl.mu.Lock()
			st.lastPatch = now
			if err != nil {
				st.lastError = err.Error()
			}
			ctl.mu.Unlock()
		}

		ctl.mu.Lock()
		r.updatePhaseLocked(st, cfg)
		reserve := 0
		if st.lease != nil {
			reserve = max(st.lease.hotplugMiB, st.status.PluggedMiB)
		} else {
			reserve = max(st.status.PluggedMiB, st.status.RequestedMiB)
		}
		ctl.mu.Unlock()
		r.setLeaseReservation(vmID, uint64(reserve)<<20)
	}
}

// dropMemoryState forgets a computer that is no longer running (its
// memory went with its Firecracker) and releases its reservation.
func (r *OwnershipRegistry) dropMemoryState(vmID string) {
	ctl := &r.memoryCtl
	ctl.mu.Lock()
	if st := ctl.vms[vmID]; st != nil && st.lease != nil {
		log.Printf("vmctl: memory lease vm=%s ended: the computer is no longer running", vmID)
	}
	delete(ctl.vms, vmID)
	ctl.mu.Unlock()
	r.setLeaseReservation(vmID, 0)
}

// hostBelowMemoryFloor reports whether host MemAvailable is under the
// reclaim floor: leases end before a bystander is hibernated.
func (r *OwnershipRegistry) hostBelowMemoryFloor() bool {
	r.mu.RLock()
	pcfg, sampler := r.pressureReclaim, r.pressureSampler
	r.mu.RUnlock()
	pcfg = normalizePressureReclaimConfig(pcfg)
	if pcfg.MinMemoryAvailableBytes == 0 && pcfg.MinMemoryAvailablePercent == 0 {
		def := DefaultPressureReclaimConfig()
		pcfg.MinMemoryAvailableBytes, pcfg.MinMemoryAvailablePercent = def.MinMemoryAvailableBytes, def.MinMemoryAvailablePercent
	}
	if sampler == nil {
		sampler = sampleHostPressure
	}
	sample := sampler(pcfg)
	if sample.MemoryAvailableBytes == 0 {
		return false
	}
	return sample.MemoryAvailableBytes < memoryFloorBytes(pcfg, sample.MemoryTotalBytes)
}

// StartMemoryHotplugController runs the lease controller until ctx ends.
// With hotplug disabled it still runs, so leases granted before a flag
// flip end on their TTL; with no manager it does nothing.
func (r *OwnershipRegistry) StartMemoryHotplugController(ctx context.Context) {
	cfg := r.memoryHotplugConfig()
	go func() {
		ticker := time.NewTicker(cfg.TickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.memoryTick()
			}
		}
	}()
}
