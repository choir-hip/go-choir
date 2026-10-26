package vmctl

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// Admission by measured use, with in-flight reservations (density quick
// tier, lever L1; docs/reports/vm-density-limits-2026-10-14.md finding 6).
//
// Before: one MemAvailable sample against floor + the full 8 GiB guest
// (12.67 GiB on Node B for a computer that idles at 1.2 GiB), and nothing
// recorded the starts already in flight. Now a start needs
//
//	MemAvailable >= floor + headroom + reserved + need
//
// where need is the computer's last measured peak RSS (a conservative
// default when unknown, never under a minimum or over the guest), and
// reserved is what the starts admitted before it have not yet taken: a
// failed start releases at once, a finished one keeps need minus its
// measured RSS for a settle window. The check and the reservation happen
// under one lock, so concurrent wakes cannot all pass the same reading.
// Protected computers are admitted as before, but reserve.
//
// Admission does not reserve a running VM's growth (RES-15). A reservation
// covers need minus measured RSS only until the settle window ends or the
// VM's RSS reaches need; after that the VM is just part of MemAvailable. A
// VM that later grows past its last measured peak (a long agent run, a
// page-cache fill) is invisible to the next start's check, so admission
// bounds starts, not total host use. The backstops are pressure reclaim and
// the memory leases, not this check.
//
// Rollback: VMCTL_ADMISSION_MODE=guest-size restores the old rule exactly.

const (
	HostAdmissionModeMeasured  = "measured"
	HostAdmissionModeGuestSize = "guest-size"

	admissionRecentDecisions = 20
	// memoryPeakSaveDelta bounds how often a measurement rewrites the
	// ownership file.
	memoryPeakSaveDelta = 64 << 20
)

// HostAdmissionConfig controls the start admission check. It applies only
// while pressure reclaim is active, as before.
type HostAdmissionConfig struct {
	Mode string
	// DefaultNeedBytes is the estimate for a computer never measured.
	DefaultNeedBytes uint64
	// MinNeedBytes is the smallest estimate for a measured computer.
	MinNeedBytes uint64
	// HeadroomBytes is a shared burst pool kept above the floor.
	HeadroomBytes uint64
	// Settle is how long a finished start keeps reserving what its VM has
	// not yet taken (a snapshot wake grows for about 5 minutes).
	Settle time.Duration
	// InFlightTTL ends a reservation whose start never reported back.
	InFlightTTL time.Duration

	// MakeRoom (density D1, host_admission_make_room.go): active lets a
	// refused start hibernate idle eligible computers and then admit; off
	// is the rollback (refuse, as before).
	MakeRoom string
	// MakeRoomMaxVictims bounds the computers one start may hibernate.
	MakeRoomMaxVictims int
	// MakeRoomSettleWait bounds how long a start waits, after each
	// hibernate, for the freed memory to show in MemAvailable.
	MakeRoomSettleWait time.Duration
	// MakeRoomPollInterval is how often that wait re-samples.
	MakeRoomPollInterval time.Duration
}

func DefaultHostAdmissionConfig() HostAdmissionConfig {
	return HostAdmissionConfig{
		Mode:             HostAdmissionModeMeasured,
		DefaultNeedBytes: 2 << 30,
		MinNeedBytes:     1 << 30,
		HeadroomBytes:    1 << 30,
		Settle:           5 * time.Minute,
		InFlightTTL:      15 * time.Minute,

		MakeRoom:             HostAdmissionMakeRoomActive,
		MakeRoomMaxVictims:   defaultMakeRoomMaxVictims,
		MakeRoomSettleWait:   defaultMakeRoomSettleWait,
		MakeRoomPollInterval: defaultMakeRoomPollInterval,
	}
}

func normalizeHostAdmissionConfig(cfg HostAdmissionConfig) HostAdmissionConfig {
	def := DefaultHostAdmissionConfig()
	switch strings.TrimSpace(strings.ToLower(cfg.Mode)) {
	case HostAdmissionModeGuestSize, "guest_size", "legacy", "full":
		cfg.Mode = HostAdmissionModeGuestSize
	default:
		cfg.Mode = HostAdmissionModeMeasured
	}
	if cfg.DefaultNeedBytes == 0 {
		cfg.DefaultNeedBytes = def.DefaultNeedBytes
	}
	if cfg.MinNeedBytes == 0 {
		cfg.MinNeedBytes = def.MinNeedBytes
	}
	if cfg.Settle <= 0 {
		cfg.Settle = def.Settle
	}
	if cfg.InFlightTTL <= 0 {
		cfg.InFlightTTL = def.InFlightTTL
	}
	switch strings.TrimSpace(strings.ToLower(cfg.MakeRoom)) {
	case HostAdmissionMakeRoomOff, "false", "0", "disabled", "refuse":
		cfg.MakeRoom = HostAdmissionMakeRoomOff
	default:
		cfg.MakeRoom = HostAdmissionMakeRoomActive
	}
	if cfg.MakeRoomMaxVictims <= 0 {
		cfg.MakeRoomMaxVictims = def.MakeRoomMaxVictims
	}
	if cfg.MakeRoomSettleWait <= 0 {
		cfg.MakeRoomSettleWait = def.MakeRoomSettleWait
	}
	if cfg.MakeRoomPollInterval <= 0 {
		cfg.MakeRoomPollInterval = def.MakeRoomPollInterval
	}
	return cfg
}

// SetHostAdmissionConfig configures start admission.
func (r *OwnershipRegistry) SetHostAdmissionConfig(cfg HostAdmissionConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hostAdmission = normalizeHostAdmissionConfig(cfg)
}

// vmMemoryReader is implemented by the Firecracker manager: resident
// memory of each live VM's Firecracker processes.
type vmMemoryReader interface {
	VMResidentBytes() map[string]uint64
}

type vmMemoryObservation struct {
	bytes uint64
	at    time.Time
}

type admissionReservation struct {
	id         uint64
	vmID       string
	computerID string
	class      WarmnessClass
	need       uint64
	source     string
	startedAt  time.Time
	settledAt  time.Time
}

// hostMemoryLedger holds reservations and VM memory measurements. Its mutex
// may be taken while holding OwnershipRegistry.mu, never the reverse.
type hostMemoryLedger struct {
	mu           sync.Mutex
	nextID       uint64
	reservations map[uint64]*admissionReservation
	rss          map[string]vmMemoryObservation
	peak         map[string]uint64
	peakReset    map[string]bool
	recent       []HostAdmissionDecision
	// leases are memory hotplug leases (memory_hotplug.go): bytes plugged
	// or requested above a computer's base, held at full size until the
	// unplug completes, so starts and grows see each other.
	leases map[string]uint64
}

func (l *hostMemoryLedger) initLocked() {
	if l.reservations == nil {
		l.reservations = map[uint64]*admissionReservation{}
		l.rss = map[string]vmMemoryObservation{}
		l.peak = map[string]uint64{}
		l.peakReset = map[string]bool{}
	}
	if l.leases == nil {
		l.leases = map[string]uint64{}
	}
}

// outstandingLocked expires finished reservations and returns what the
// rest still hold: need minus the VM's RSS measured since the start.
func (l *hostMemoryLedger) outstandingLocked(now time.Time, cfg HostAdmissionConfig) (uint64, []AdmissionReservationView) {
	l.initLocked()
	var total uint64
	views := make([]AdmissionReservationView, 0, len(l.reservations))
	for id, res := range l.reservations {
		settled := !res.settledAt.IsZero()
		if settled && now.Sub(res.settledAt) > cfg.Settle {
			delete(l.reservations, id)
			continue
		}
		if !settled && now.Sub(res.startedAt) > cfg.InFlightTTL {
			log.Printf("vmctl: admission reservation for VM %s expired in flight after %s", res.vmID, now.Sub(res.startedAt).Round(time.Second))
			delete(l.reservations, id)
			continue
		}
		remaining := res.need
		if obs, ok := l.rss[res.vmID]; ok && !obs.at.Before(res.startedAt) {
			if obs.bytes >= res.need {
				remaining = 0
			} else {
				remaining = res.need - obs.bytes
			}
		}
		total += remaining
		view := AdmissionReservationView{
			ID:               res.id,
			VMID:             res.vmID,
			ComputerID:       res.computerID,
			Class:            string(res.class),
			NeedBytes:        res.need,
			NeedSource:       res.source,
			OutstandingBytes: remaining,
			InFlight:         !settled,
			StartedAt:        res.startedAt.UTC().Format(time.RFC3339),
		}
		if settled {
			view.SettledAt = res.settledAt.UTC().Format(time.RFC3339)
		}
		views = append(views, view)
	}
	for vmID, bytes := range l.leases {
		if bytes == 0 {
			continue
		}
		total += bytes
		views = append(views, AdmissionReservationView{
			VMID:             vmID,
			NeedBytes:        bytes,
			NeedSource:       memoryLeaseNeedSource,
			OutstandingBytes: bytes,
		})
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].ID != views[j].ID {
			return views[i].ID < views[j].ID
		}
		return views[i].VMID < views[j].VMID
	})
	return total, views
}

func (l *hostMemoryLedger) recordLocked(d HostAdmissionDecision) {
	l.recent = append(l.recent, d)
	if len(l.recent) > admissionRecentDecisions {
		l.recent = append([]HostAdmissionDecision(nil), l.recent[len(l.recent)-admissionRecentDecisions:]...)
	}
}

// HostAdmissionDecision is one admission decision with its inputs.
type HostAdmissionDecision struct {
	At             string `json:"at"`
	Decision       string `json:"decision"`
	Mode           string `json:"mode"`
	VMID           string `json:"vm_id,omitempty"`
	ComputerID     string `json:"-"` // journal only; health shows VM ids
	Class          string `json:"class,omitempty"`
	NeedBytes      uint64 `json:"need_bytes"`
	NeedSource     string `json:"need_source"`
	AvailableBytes uint64 `json:"available_bytes"`
	ReservedBytes  uint64 `json:"reserved_bytes"`
	StartsInFlight int    `json:"starts_in_flight"`
	FloorBytes     uint64 `json:"floor_bytes"`
	HeadroomBytes  uint64 `json:"headroom_bytes"`
	// MadeRoomVMIDs are the computers a make-room decision hibernated;
	// Reason explains a make-room refusal (host_admission_make_room.go).
	MadeRoomVMIDs []string `json:"made_room_vm_ids,omitempty"`
	Reason        string   `json:"reason,omitempty"`
}

// AdmissionReservationView is one reservation as health shows it.
type AdmissionReservationView struct {
	ID               uint64 `json:"id"`
	VMID             string `json:"vm_id"`
	ComputerID       string `json:"-"` // journal only
	Class            string `json:"class,omitempty"`
	NeedBytes        uint64 `json:"need_bytes"`
	NeedSource       string `json:"need_source"`
	OutstandingBytes uint64 `json:"outstanding_bytes"`
	InFlight         bool   `json:"in_flight"`
	StartedAt        string `json:"started_at"`
	SettledAt        string `json:"settled_at,omitempty"`
}

// HostAdmissionSummary is the admission section of vmctl health.
type HostAdmissionSummary struct {
	Mode             string                     `json:"mode"`
	Enabled          bool                       `json:"enabled"`
	DefaultNeedBytes uint64                     `json:"default_need_bytes,omitempty"`
	MinNeedBytes     uint64                     `json:"min_need_bytes,omitempty"`
	HeadroomBytes    uint64                     `json:"headroom_bytes,omitempty"`
	SettleSeconds    int64                      `json:"settle_seconds,omitempty"`
	MakeRoom         string                     `json:"make_room,omitempty"`
	ReservedBytes    uint64                     `json:"reserved_bytes"`
	Reservations     []AdmissionReservationView `json:"reservations,omitempty"`
	Recent           []HostAdmissionDecision    `json:"recent,omitempty"`
}

// HostAdmissionSummary reports the ledger and recent decisions. It never
// samples the host.
func (r *OwnershipRegistry) HostAdmissionSummary() HostAdmissionSummary {
	r.mu.RLock()
	acfg := normalizeHostAdmissionConfig(r.hostAdmission)
	enabled := normalizePressureReclaimConfig(r.pressureReclaim).Mode == PressureReclaimModeActive
	r.mu.RUnlock()
	led := &r.admissionLedger
	led.mu.Lock()
	defer led.mu.Unlock()
	reserved, views := led.outstandingLocked(r.now(), acfg)
	out := HostAdmissionSummary{
		Mode:          acfg.Mode,
		Enabled:       enabled,
		ReservedBytes: reserved,
		Reservations:  views,
		Recent:        append([]HostAdmissionDecision(nil), led.recent...),
	}
	if acfg.Mode == HostAdmissionModeMeasured {
		out.DefaultNeedBytes = acfg.DefaultNeedBytes
		out.MinNeedBytes = acfg.MinNeedBytes
		out.HeadroomBytes = acfg.HeadroomBytes
		out.SettleSeconds = int64(acfg.Settle.Seconds())
		out.MakeRoom = acfg.MakeRoom
	}
	return out
}

// admissionTicket is an admitted start's reservation. finish reports how
// the start ended; it is safe on a nil ticket.
type admissionTicket struct {
	reg    *OwnershipRegistry
	id     uint64
	vmID   string
	need   uint64
	source string
}

func (t *admissionTicket) finish(started bool) {
	if t == nil || t.reg == nil {
		return
	}
	led := &t.reg.admissionLedger
	led.mu.Lock()
	defer led.mu.Unlock()
	led.initLocked()
	res := led.reservations[t.id]
	if res == nil {
		return
	}
	if !started {
		delete(led.reservations, t.id)
		return
	}
	res.settledAt = t.reg.now()
	// The next measurement starts this start's own peak.
	led.peakReset[res.vmID] = true
}

// estimateAdmissionNeed is the memory a start of own is expected to take.
func (r *OwnershipRegistry) estimateAdmissionNeed(own *VMOwnership) (uint64, string) {
	r.mu.RLock()
	acfg := normalizeHostAdmissionConfig(r.hostAdmission)
	r.mu.RUnlock()
	return estimateNeedWith(own, acfg)
}

func estimateNeedWith(own *VMOwnership, acfg HostAdmissionConfig) (uint64, string) {
	_, memMib := machineShapeForOwnership(own)
	guest := uint64(memMib) << 20
	need, source := acfg.DefaultNeedBytes, "default"
	if own != nil && own.MemoryPeakRSSBytes > 0 {
		need, source = own.MemoryPeakRSSBytes, "measured"
		if need < acfg.MinNeedBytes {
			need = acfg.MinNeedBytes
		}
	}
	if guest > 0 && need > guest {
		need = guest
	}
	return need, source
}

// admitHostMemory refuses a start that would add a VM to a host whose
// available memory, less what starts in flight still need, would fall
// below the floor. Protected computers are always admitted. Only active
// pressure reclaim enables it. A nil ticket with a nil error means no
// reservation was taken.
//
// A refused start makes room when make-room is on (density D1): it
// hibernates idle eligible computers and admits on a fresh sample, and is
// refused only when nothing eligible can make the room. The caller must not
// hold r.mu.
func (r *OwnershipRegistry) admitHostMemory(own *VMOwnership, class WarmnessClass) (*admissionTicket, error) {
	r.mu.RLock()
	cfg, acfg, sampler := r.pressureReclaim, r.hostAdmission, r.pressureSampler
	mayMakeRoom := r.makeRoomEnabledLocked()
	r.mu.RUnlock()
	ticket, err := r.admitHostMemoryWith(own, class, cfg, acfg, sampler)
	if err == nil || !mayMakeRoom || !makeRoomRefusal(err) {
		return ticket, err
	}
	return r.makeRoomAndAdmit(own, class, nil)
}

// admitHostMemoryLocked is admitHostMemory for a caller holding r.mu.
func (r *OwnershipRegistry) admitHostMemoryLocked(own *VMOwnership, class WarmnessClass) (*admissionTicket, error) {
	return r.admitHostMemoryWith(own, class, r.pressureReclaim, r.hostAdmission, r.pressureSampler)
}

func (r *OwnershipRegistry) admitHostMemoryWith(own *VMOwnership, class WarmnessClass, cfg PressureReclaimConfig, acfg HostAdmissionConfig, sampler hostPressureSampler) (*admissionTicket, error) {
	cfg = normalizePressureReclaimConfig(cfg)
	if cfg.Mode != PressureReclaimModeActive {
		return nil, nil
	}
	acfg = normalizeHostAdmissionConfig(acfg)
	if sampler == nil {
		sampler = sampleHostPressure
	}
	if acfg.Mode == HostAdmissionModeGuestSize {
		return nil, r.admitGuestSize(own, class, cfg, sampler)
	}

	d, ticket := r.attemptMeasuredAdmission(own, class, cfg, acfg, sampler, true)
	logAdmissionDecision(d)
	if ticket != nil {
		return ticket, nil
	}
	return nil, hostMemoryRefusal(d, "")
}

// attemptMeasuredAdmission is one measured admission decision: a sample
// and the ledger read under the ledger lock, and a reservation when it
// admits. record adds the decision to the recent list; make-room's
// re-samples do not, it records its own outcome once.
func (r *OwnershipRegistry) attemptMeasuredAdmission(own *VMOwnership, class WarmnessClass, cfg PressureReclaimConfig, acfg HostAdmissionConfig, sampler hostPressureSampler, record bool) (HostAdmissionDecision, *admissionTicket) {
	var vmID, computerID string
	if own != nil {
		vmID, computerID = own.VMID, own.ComputerID
	}
	need, source := estimateNeedWith(own, acfg)
	protected := protectedWarmnessClass(class)

	led := &r.admissionLedger
	led.mu.Lock()
	// Sample inside the lock: every RSS measurement the ledger subtracts
	// was taken before this MemAvailable reading.
	sample := sampler(cfg)
	now := r.now()
	reserved, views := led.outstandingLocked(now, acfg)
	floor := memoryFloorBytes(cfg, sample.MemoryTotalBytes)
	inFlight := 0
	for _, v := range views {
		if v.InFlight {
			inFlight++
		}
	}
	d := HostAdmissionDecision{
		At:             now.UTC().Format(time.RFC3339),
		Mode:           acfg.Mode,
		VMID:           vmID,
		ComputerID:     computerID,
		Class:          string(class),
		NeedBytes:      need,
		NeedSource:     source,
		AvailableBytes: sample.MemoryAvailableBytes,
		ReservedBytes:  reserved,
		StartsInFlight: inFlight,
		FloorBytes:     floor,
		HeadroomBytes:  acfg.HeadroomBytes,
	}
	required := floor + acfg.HeadroomBytes + reserved + need
	switch {
	case protected:
		d.Decision = "admit_protected"
	case sample.MemoryAvailableBytes == 0:
		d.Decision = "admit_unknown_sample" // never refuse on a missing sample
	case sample.MemoryAvailableBytes >= required:
		d.Decision = "admit"
	default:
		d.Decision = "refuse"
	}
	if record {
		led.recordLocked(d)
	}
	var ticket *admissionTicket
	if d.Decision != "refuse" {
		led.nextID++
		led.reservations[led.nextID] = &admissionReservation{
			id: led.nextID, vmID: vmID, computerID: computerID, class: class,
			need: need, source: source, startedAt: now,
		}
		ticket = &admissionTicket{reg: r, id: led.nextID, vmID: vmID, need: need, source: source}
	}
	led.mu.Unlock()
	return d, ticket
}

// admissionShortfall is how far a refused decision's sample was under its
// requirement.
func admissionShortfall(d HostAdmissionDecision) uint64 {
	required := d.FloorBytes + d.HeadroomBytes + d.ReservedBytes + d.NeedBytes
	if d.AvailableBytes >= required {
		return 0
	}
	return required - d.AvailableBytes
}

// hostMemoryRefusal is the refusal for a refused measured decision; detail,
// when set, is make-room's account of why it could not make the room.
func hostMemoryRefusal(d HostAdmissionDecision, detail string) *RecoveryRefusal {
	reason := fmt.Sprintf("host memory available %d MiB; %d MiB reserved for starts in flight; this computer needs %d MiB (%s) above the %d MiB floor and %d MiB headroom",
		d.AvailableBytes>>20, d.ReservedBytes>>20, d.NeedBytes>>20, d.NeedSource, d.FloorBytes>>20, d.HeadroomBytes>>20)
	if detail != "" {
		reason += "; " + detail
	}
	return &RecoveryRefusal{
		ComputerID:        d.ComputerID,
		Kind:              RecoveryRefusalHostMemoryPressure,
		Reason:            reason,
		RetryAfterSeconds: hostMemoryRetryAfterSeconds,
	}
}

// admitGuestSize is the rule before 2026-10-14, kept as the rollback: one
// sample against floor + the full interactive guest, no reservation.
func (r *OwnershipRegistry) admitGuestSize(own *VMOwnership, class WarmnessClass, cfg PressureReclaimConfig, sampler hostPressureSampler) error {
	if protectedWarmnessClass(class) {
		return nil
	}
	sample := sampler(cfg)
	if sample.MemoryAvailableBytes == 0 {
		return nil // unknown; never refuse on a missing sample
	}
	floor := memoryFloorBytes(cfg, sample.MemoryTotalBytes)
	_, memMib := interactiveMachineShape()
	need := uint64(memMib) * 1024 * 1024
	d := HostAdmissionDecision{
		At:             r.now().UTC().Format(time.RFC3339),
		Decision:       "admit",
		Mode:           HostAdmissionModeGuestSize,
		Class:          string(class),
		NeedBytes:      need,
		NeedSource:     "guest_size",
		AvailableBytes: sample.MemoryAvailableBytes,
		FloorBytes:     floor,
	}
	if own != nil {
		d.VMID, d.ComputerID = own.VMID, own.ComputerID
	}
	if sample.MemoryAvailableBytes < floor+need {
		d.Decision = "refuse"
	}
	led := &r.admissionLedger
	led.mu.Lock()
	led.recordLocked(d)
	led.mu.Unlock()
	logAdmissionDecision(d)
	if d.Decision == "admit" {
		return nil
	}
	return &RecoveryRefusal{
		Kind: RecoveryRefusalHostMemoryPressure,
		Reason: fmt.Sprintf("host memory available %d MiB; a new computer needs %d MiB above the %d MiB floor",
			sample.MemoryAvailableBytes>>20, need>>20, floor>>20),
		RetryAfterSeconds: hostMemoryRetryAfterSeconds,
	}
}

func logAdmissionDecision(d HostAdmissionDecision) {
	log.Printf("vmctl: admission decision=%s mode=%s vm=%s computer=%s class=%s need_mib=%d need_source=%s available_mib=%d reserved_mib=%d starts_in_flight=%d floor_mib=%d headroom_mib=%d",
		d.Decision, d.Mode, d.VMID, d.ComputerID, d.Class, d.NeedBytes>>20, d.NeedSource, d.AvailableBytes>>20,
		d.ReservedBytes>>20, d.StartsInFlight, d.FloorBytes>>20, d.HeadroomBytes>>20)
}

// sampleVMMemory reads each live VM's resident memory, keeps the latest
// reading for reservations, and records each computer's peak for this
// start on its ownership.
func (r *OwnershipRegistry) sampleVMMemory() {
	r.mu.RLock()
	mgr := r.vmManager
	r.mu.RUnlock()
	reader, ok := mgr.(vmMemoryReader)
	if !ok || reader == nil {
		return
	}
	readings := reader.VMResidentBytes()
	now := r.now()

	led := &r.admissionLedger
	led.mu.Lock()
	led.initLocked()
	peaks := make(map[string]uint64, len(readings))
	for vmID, bytes := range readings {
		led.rss[vmID] = vmMemoryObservation{bytes: bytes, at: now}
		peak, seen := led.peak[vmID]
		if !seen || led.peakReset[vmID] || bytes > peak {
			peak = bytes
		}
		delete(led.peakReset, vmID)
		led.peak[vmID] = peak
		peaks[vmID] = peak
	}
	for vmID := range led.rss {
		if _, live := readings[vmID]; !live {
			// The VM stopped: its next start measures afresh.
			delete(led.rss, vmID)
			delete(led.peak, vmID)
		}
	}
	led.mu.Unlock()

	if len(peaks) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := false
	stamp := now.UTC().Format(time.RFC3339)
	for vmID, peak := range peaks {
		own := r.vmByID[vmID]
		if own == nil {
			continue
		}
		old := own.MemoryPeakRSSBytes
		own.MemoryPeakRSSBytes = peak
		own.MemoryMeasuredAt = stamp
		if old == 0 || absDiff(old, peak) >= memoryPeakSaveDelta {
			changed = true
		}
	}
	if changed {
		r.saveLocked()
	}
}

func absDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}
