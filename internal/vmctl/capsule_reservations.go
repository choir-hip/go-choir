package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// Live build reservations (station S4,
// docs/design/capsule-heavy-work-scheduling-2026-11-03.md, points 1, 3, 5).
//
// Measured admission charges a running computer at its settled resident
// memory, so the growth a capsule build adds inside the guest's ceiling was
// admitted by no one: four arm builds could add about 12 GB against 17 GB
// available and nothing refused it. A guest now reports its live build-class
// capsules here, and the ledger charges each such computer the larger of
// what it holds now and its idle footprint (its settled RSS) plus its live
// build reservations, clamped to what its ceiling can still grow. That
// outstanding amount is part of the shared reserved term every start and
// every build spawn is admitted against.
//
// A build spawn is admitted here, at placement, on the same rule a start is
// (MemAvailable >= floor + headroom + reserved + this build's increment), or
// refused with a visible reason; its jobs then never ask. Protected
// computers (the paper, the owner's computer) are always admitted, and
// their reservations still count against everyone else.
//
// A guest re-reports its whole live set every few seconds while it holds
// any; a set not reported for the in-flight TTL expires, and a computer that
// stops loses its set at the sampler's next pass. The reply carries the host
// facts (MemAvailable over the floor, PSI memory averages) the guest's
// capsule executor reads before it starts a build-class job.
//
// The reservations are the guest's claim: bounded per computer by its own
// machine size, never trusted for anything but this charge. Nothing here is
// persisted: a vmctl restart forgets the sets and the guests' next reports
// restore them.

const (
	capsuleReservationsPath = "/internal/vmctl/computers/capsule-reservations"

	// maxCapsuleReservations bounds one computer's live set.
	maxCapsuleReservations = 16

	// hostPressureHighAvg10 is the PSI memory some avg10 over which the
	// host reads as under memory pressure for deferring heavy work; avg60
	// uses the pressure reclaim's sustained threshold.
	hostPressureHighAvg10 = 10.0

	// Priorities as a guest reads them.
	CapsulePriorityProtected = "protected"
	CapsulePriorityBatch     = "batch"
)

// CapsuleReservation is one live build-class capsule's host reservation.
type CapsuleReservation struct {
	CapsuleID string `json:"capsule_id"`
	Bytes     uint64 `json:"bytes"`
}

// CapsuleReservationReport is a guest's report: its whole live set, and at
// most one new build spawn to admit.
type CapsuleReservationReport struct {
	ComputerID   string               `json:"computer_id"`
	Reservations []CapsuleReservation `json:"reservations"`
	Admit        *CapsuleReservation  `json:"admit,omitempty"`
}

// HostMemoryFacts is what the host publishes to a guest for its heavy work.
type HostMemoryFacts struct {
	SampledAt         string `json:"sampled_at"`
	MemAvailableBytes uint64 `json:"mem_available_bytes"`
	FloorBytes        uint64 `json:"floor_bytes"`
	// ReservedOthersBytes is what starts in flight and other computers'
	// live builds still hold; HeadroomBytes is MemAvailable less the floor
	// and that, so a guest never counts memory promised to another.
	ReservedOthersBytes uint64  `json:"reserved_others_bytes"`
	HeadroomBytes       uint64  `json:"headroom_bytes"`
	MemorySomeAvg10     float64 `json:"memory_some_avg10"`
	MemorySomeAvg60     float64 `json:"memory_some_avg60"`
	PressureHigh        bool    `json:"pressure_high"`
	// Priority is protected (the paper, the owner's computer: never
	// deferred) or batch.
	Priority string `json:"priority"`
}

// CapsuleReservationResult is the reply. Admitted and Reason answer Admit.
type CapsuleReservationResult struct {
	Admitted *bool           `json:"admitted,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Facts    HostMemoryFacts `json:"facts"`
}

// buildReservationSet is one computer's live builds in the ledger.
type buildReservationSet struct {
	vmID       string
	computerID string
	class      WarmnessClass
	// idle is the footprint the builds are charged on top of: the settled
	// RSS, else the RSS when the set was first reported.
	idle uint64
	// ceiling is the guest's machine memory: the set never charges past it.
	ceiling    uint64
	entries    map[string]uint64
	reportedAt time.Time
}

func (s *buildReservationSet) total() uint64 {
	var sum uint64
	for _, b := range s.entries {
		sum += b
	}
	return sum
}

// outstanding is what the set still holds beyond the VM's latest reading:
// idle + builds - rss, never past what the ceiling can still grow.
func (s *buildReservationSet) outstanding(rss uint64, extra uint64) uint64 {
	builds := s.total() + extra
	if builds == 0 {
		return 0
	}
	charge := s.idle + builds
	if charge <= rss {
		return 0
	}
	out := charge - rss
	if s.ceiling > 0 {
		room := uint64(0)
		if s.ceiling > rss {
			room = s.ceiling - rss
		}
		if out > room {
			out = room
		}
	}
	return out
}

// buildOutstandingLocked expires sets not reported within the in-flight TTL
// and returns what the rest hold, per VM and as views. Callers hold l.mu.
func (l *hostMemoryLedger) buildOutstandingLocked(now time.Time, cfg HostAdmissionConfig) (uint64, []AdmissionReservationView) {
	l.initLocked()
	var total uint64
	var views []AdmissionReservationView
	for vmID, set := range l.builds {
		if now.Sub(set.reportedAt) > cfg.InFlightTTL {
			log.Printf("vmctl: capsule build reservations of VM %s expired: not reported for %s", vmID, now.Sub(set.reportedAt).Round(time.Second))
			delete(l.builds, vmID)
			continue
		}
		if len(set.entries) == 0 {
			continue
		}
		out := set.outstanding(l.rss[vmID].bytes, 0)
		total += out
		views = append(views, AdmissionReservationView{
			VMID: vmID, ComputerID: set.computerID, Class: string(set.class),
			NeedBytes: set.total(), NeedSource: "capsule_build", OutstandingBytes: out,
			StartedAt: set.reportedAt.UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].VMID < views[j].VMID })
	return total, views
}

// capsulePriority is how a guest reads its class.
func capsulePriority(class WarmnessClass) string {
	if protectedWarmnessClass(class) {
		return CapsulePriorityProtected
	}
	return CapsulePriorityBatch
}

func hostPressureHigh(sample HostPressureSample, cfg PressureReclaimConfig) bool {
	avg60 := cfg.MaxMemorySomeAvg60
	if avg60 <= 0 {
		avg60 = defaultMaxMemorySomeAvg60
	}
	return sample.MemorySomeAvg10 > hostPressureHighAvg10 || sample.MemorySomeAvg60 > avg60
}

func validCapsuleReservation(c CapsuleReservation) bool {
	id := strings.TrimSpace(c.CapsuleID)
	return id != "" && id == c.CapsuleID && len(id) <= 128 && !strings.ContainsAny(id, "\x00\n\r") && c.Bytes > 0
}

// ReportCapsuleReservations records a computer's live build set and admits
// its new build spawn, if any.
func (r *OwnershipRegistry) ReportCapsuleReservations(report CapsuleReservationReport) (CapsuleReservationResult, error) {
	computerID := strings.TrimSpace(report.ComputerID)
	if len(report.Reservations) > maxCapsuleReservations {
		return CapsuleReservationResult{}, fmt.Errorf("at most %d live capsule reservations", maxCapsuleReservations)
	}
	for _, c := range report.Reservations {
		if !validCapsuleReservation(c) {
			return CapsuleReservationResult{}, fmt.Errorf("invalid capsule reservation")
		}
	}
	if report.Admit != nil && !validCapsuleReservation(*report.Admit) {
		return CapsuleReservationResult{}, fmt.Errorf("invalid capsule reservation")
	}

	r.mu.RLock()
	own := r.byComputerIDLocked(computerID)
	if own == nil || own.VMID == "" {
		r.mu.RUnlock()
		return CapsuleReservationResult{}, fmt.Errorf("vmctl: no ownership for computer %s", computerID)
	}
	vmID, settled := own.VMID, own.MemorySettledRSSBytes
	_, memMib := machineShapeForOwnership(own)
	class := warmnessClassForOwnership(own, r.warmnessPolicy)
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	acfg := normalizeHostAdmissionConfig(r.hostAdmission)
	sampler := r.pressureSampler
	r.mu.RUnlock()
	if sampler == nil {
		sampler = sampleHostPressure
	}
	ceiling := uint64(memMib) << 20

	led := &r.admissionLedger
	led.mu.Lock()
	defer led.mu.Unlock()
	led.initLocked()
	// Sample inside the lock, as a start's admission does: every reading
	// the ledger subtracts was taken before this MemAvailable.
	sample := sampler(cfg)
	now := r.now()
	rss := led.rss[vmID].bytes

	set := led.builds[vmID]
	if set == nil {
		idle := settled
		if idle == 0 {
			idle = rss
		}
		set = &buildReservationSet{vmID: vmID, computerID: computerID, idle: idle}
		led.builds[vmID] = set
	} else if settled > 0 {
		set.idle = settled
	}
	set.class, set.ceiling, set.reportedAt = class, ceiling, now
	set.entries = map[string]uint64{}
	clamp := func(b uint64) uint64 {
		if ceiling > 0 && b > ceiling {
			return ceiling
		}
		return b
	}
	for _, c := range report.Reservations {
		set.entries[c.CapsuleID] = clamp(c.Bytes)
	}

	reserved, views := led.outstandingLocked(now, acfg)
	floor := memoryFloorBytes(cfg, sample.MemoryTotalBytes)
	var result CapsuleReservationResult
	if admit := report.Admit; admit != nil {
		if _, held := set.entries[admit.CapsuleID]; held {
			yes := true
			result.Admitted = &yes
		} else {
			bytes := clamp(admit.Bytes)
			before := set.outstanding(rss, 0)
			increment := set.outstanding(rss, bytes) - before
			d := HostAdmissionDecision{
				At: now.UTC().Format(time.RFC3339), Mode: acfg.Mode, VMID: vmID, ComputerID: computerID,
				Class: string(class), NeedBytes: increment, NeedSource: "capsule_build",
				AvailableBytes: sample.MemoryAvailableBytes, ReservedBytes: reserved,
				FloorBytes: floor, HeadroomBytes: acfg.HeadroomBytes,
			}
			required := floor + acfg.HeadroomBytes + reserved + increment
			switch {
			case cfg.Mode != PressureReclaimModeActive || acfg.Mode != HostAdmissionModeMeasured:
				d.Decision = "admit_build_unchecked" // start admission is off too
			case protectedWarmnessClass(class):
				d.Decision = "admit_build_protected"
			case sample.MemoryAvailableBytes == 0:
				d.Decision = "admit_build_unknown_sample"
			case sample.MemoryAvailableBytes >= required:
				d.Decision = "admit_build"
			default:
				d.Decision = "refuse_build"
				d.Reason = fmt.Sprintf("host memory available %d MiB; %d MiB reserved for starts in flight and live builds; this build needs %d MiB above the %d MiB floor and %d MiB headroom",
					d.AvailableBytes>>20, d.ReservedBytes>>20, d.NeedBytes>>20, d.FloorBytes>>20, d.HeadroomBytes>>20)
			}
			led.recordLocked(d)
			logAdmissionDecision(d)
			admitted := d.Decision != "refuse_build"
			result.Admitted = &admitted
			if admitted {
				set.entries[admit.CapsuleID] = bytes
				reserved += increment
			} else {
				result.Reason = d.Reason
			}
		}
	}
	if len(set.entries) == 0 {
		delete(led.builds, vmID)
	}
	// What this computer itself holds (its own start in flight and its
	// builds) is not promised to another (review r1, deepseek).
	mine := uint64(0)
	for _, v := range views {
		if v.VMID == vmID && v.NeedSource != "capsule_build" {
			mine += v.OutstandingBytes
		}
	}
	if len(set.entries) > 0 {
		mine += set.outstanding(rss, 0)
	}
	others := uint64(0)
	if reserved > mine {
		others = reserved - mine
	}
	facts := HostMemoryFacts{
		SampledAt: now.UTC().Format(time.RFC3339Nano), MemAvailableBytes: sample.MemoryAvailableBytes,
		FloorBytes: floor, ReservedOthersBytes: others,
		MemorySomeAvg10: sample.MemorySomeAvg10, MemorySomeAvg60: sample.MemorySomeAvg60,
		PressureHigh: hostPressureHigh(sample, cfg), Priority: capsulePriority(class),
	}
	if spare := floor + others; sample.MemoryAvailableBytes > spare {
		facts.HeadroomBytes = sample.MemoryAvailableBytes - spare
	}
	result.Facts = facts
	return result, nil
}

// HandleCapsuleReservations serves POST
// /internal/vmctl/computers/capsule-reservations: a host caller, or the
// guest whose tap address binds to exactly the computer it names (I7).
func (h *Handler) HandleCapsuleReservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if h == nil || h.registry == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ownership registry unavailable"})
		return
	}
	var request CapsuleReservationReport
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.ComputerID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid capsule reservation report"})
		return
	}
	if !isInternalCaller(r) {
		if err := h.bindRequestToGuestComputer(r, request.ComputerID); err != nil {
			writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "caller not bound to the reported computer: " + err.Error()})
			return
		}
	}
	result, err := h.registry.ReportCapsuleReservations(request)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "no ownership") {
			status = http.StatusNotFound
		}
		writeVMCTLJSON(w, status, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, result)
}

// ErrCapsuleReservationsUnsupported is a vmctl that does not serve the
// reservation report (an older release): the guest runs as before.
var ErrCapsuleReservationsUnsupported = fmt.Errorf("vmctl client: capsule reservations unsupported")

// ReportCapsuleReservations posts this computer's live build set (and a
// build spawn to admit) and returns the host's answer and facts.
func (c *Client) ReportCapsuleReservations(ctx context.Context, report CapsuleReservationReport) (CapsuleReservationResult, error) {
	var result CapsuleReservationResult
	if c == nil || c.httpClient == nil {
		return result, fmt.Errorf("vmctl client: not configured")
	}
	if report.Reservations == nil {
		report.Reservations = []CapsuleReservation{}
	}
	payload, err := json.Marshal(report)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+capsuleReservationsPath, bytes.NewReader(payload))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	// The guest's vmctl address is its tap peer, never a credential port:
	// MarkInternal sends no host credential there.
	hostauth.MarkInternal(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("vmctl client: capsule reservations: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		if !bytes.Contains(body, []byte("no ownership")) {
			return result, ErrCapsuleReservationsUnsupported
		}
		fallthrough
	default:
		return result, fmt.Errorf("vmctl client: capsule reservations returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("vmctl client: capsule reservations: decode: %w", err)
	}
	return result, nil
}
