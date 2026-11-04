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
// docs/design/capsule-heavy-work-scheduling-2026-11-03.md, points 1, 3, 5),
// held per job since
// docs/problems/build-capsule-holds-peak-while-agent-thinks-2026-11-04.md:
// a build capsule holds only its baseline (what its idle processes use, as
// its guest measures it), and each job a lease for its need while it runs.
//
// Measured admission charges a running computer at its settled resident
// memory, so the growth a capsule build adds inside the guest's ceiling was
// admitted by no one: four arm builds could add about 12 GB against 17 GB
// available and nothing refused it. A guest now reports its live build-class
// capsules here, and the ledger charges each such computer the larger of
// what it holds now and its idle footprint (buildIdle) plus its live
// build reservations, clamped to what its ceiling can still grow. That
// outstanding amount is part of the shared reserved term every start and
// every build spawn is admitted against.
//
// A build spawn's baseline and a job's lease are admitted here, on the same
// rule a start is (MemAvailable >= floor + headroom + reserved + this
// increment), or refused with a visible reason and the refusal as data
// (CapsuleMemoryWait: need, room, holders). A job lease also waits while
// the host is under memory pressure. Protected computers (the paper, the
// owner's computer) are always admitted, and their reservations still
// count against everyone else.
//
// A guest re-reports its whole live set every few seconds while it holds
// any; a set not reported for the in-flight TTL expires, a set reported
// from an earlier realization (epoch) of its VM is dropped, and a computer
// that stops loses its set at the sampler's next pass. The reply still
// carries the host facts (MemAvailable over the floor, PSI memory
// averages) that a guest from before per-job leases reads before it starts
// a build-class job.
//
// The reservations are the guest's claim: bounded per computer by its own
// machine size, never trusted for anything but this charge. Nothing here is
// persisted: a vmctl restart forgets the sets and the guests' next reports
// restore them.

const (
	capsuleReservationsPath = "/internal/vmctl/computers/capsule-reservations"

	// maxCapsuleReservations bounds one computer's live set: capsules and
	// their job leases (one job runs at a time in a capsule).
	maxCapsuleReservations = 32

	// hostPressureHighAvg10 is the PSI memory some avg10 over which the
	// host reads as under memory pressure for deferring heavy work; avg60
	// uses the pressure reclaim's sustained threshold.
	hostPressureHighAvg10 = 10.0

	// Priorities as a guest reads them.
	CapsulePriorityProtected = "protected"
	CapsulePriorityBatch     = "batch"
)

// CapsuleReservation is one live build-class capsule's baseline, or, with
// JobID, the lease of the job running in it.
type CapsuleReservation struct {
	CapsuleID string `json:"capsule_id"`
	JobID     string `json:"job_id,omitempty"`
	Bytes     uint64 `json:"bytes"`
}

// reservationKey is a set entry: a capsule's baseline (job "") or a job's
// lease.
type reservationKey struct{ capsule, job string }

func (c CapsuleReservation) key() reservationKey { return reservationKey{c.CapsuleID, c.JobID} }

// CapsuleReservationReport is a guest's report: its whole live set, and at
// most one new capsule baseline or job lease to admit.
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
	// live builds still hold; BurstPoolBytes is the shared burst pool every
	// admission keeps above the floor (VMCTL admission headroom; 0 when the
	// spawn gate is unchecked or the computer is protected); HeadroomBytes is MemAvailable less the
	// floor, the pool and that, so a guest never counts memory promised to
	// another, and a job reads the room a build spawn is admitted against
	// (docs/problems/working-guest-cache-held-while-builds-wait-2026-11-03.md).
	ReservedOthersBytes uint64 `json:"reserved_others_bytes"`
	BurstPoolBytes      uint64 `json:"burst_pool_bytes"`
	HeadroomBytes       uint64 `json:"headroom_bytes"`
	// OwnReservedBytes is what the ledger still holds for this computer's
	// own capsules and jobs. Only a guest from before per-job leases reads
	// it (its job credit, 67f9ec664); delete once no such guest runs.
	OwnReservedBytes uint64  `json:"own_reserved_bytes,omitempty"`
	MemorySomeAvg10  float64 `json:"memory_some_avg10"`
	MemorySomeAvg60  float64 `json:"memory_some_avg60"`
	PressureHigh     bool    `json:"pressure_high"`
	// Priority is protected (the paper, the owner's computer: never
	// deferred) or batch.
	Priority string `json:"priority"`
}

// CapsuleReservationResult is the reply. Admitted, Reason and Wait answer
// Admit.
type CapsuleReservationResult struct {
	Admitted *bool  `json:"admitted,omitempty"`
	Reason   string `json:"reason,omitempty"`
	// Wait is a refusal as data, for the guest to show its desks.
	Wait  *CapsuleMemoryWait `json:"wait,omitempty"`
	Facts HostMemoryFacts    `json:"facts"`
}

// CapsuleMemoryWait is a refused admission: what it needs, the room the
// host has for it (MemAvailable less the floor, the burst pool and what is
// promised), and who holds what is promised. A wait is true contention:
// the memory is promised to a start in flight or a running job, or the
// host is under memory pressure.
type CapsuleMemoryWait struct {
	NeedBytes      uint64                `json:"need_bytes"`
	RoomBytes      uint64                `json:"room_bytes"`
	AvailableBytes uint64                `json:"available_bytes"`
	FloorBytes     uint64                `json:"floor_bytes"`
	PoolBytes      uint64                `json:"pool_bytes"`
	ReservedBytes  uint64                `json:"reserved_bytes"`
	PressureHigh   bool                  `json:"pressure_high,omitempty"`
	Holders        []CapsuleMemoryHolder `json:"holders,omitempty"`
}

// CapsuleMemoryHolder is one holder of promised host memory: a start in
// flight, a job lease, or capsule baselines not yet resident. Another
// computer's holder carries its kind, class and bytes, never its ids.
type CapsuleMemoryHolder struct {
	Kind      string `json:"kind"` // start, job, capsule
	Own       bool   `json:"own,omitempty"`
	Class     string `json:"class,omitempty"`
	CapsuleID string `json:"capsule_id,omitempty"`
	JobID     string `json:"job_id,omitempty"`
	Bytes     uint64 `json:"bytes"`
}

// String names the holder for a reason text.
func (h CapsuleMemoryHolder) String() string {
	who := "another computer"
	if h.Own {
		who = "this computer"
	}
	if h.Class != "" && !h.Own {
		who = "another " + h.Class + " computer"
	}
	switch h.Kind {
	case "start":
		return fmt.Sprintf("a start in flight on %s (%d MiB)", who, h.Bytes>>20)
	case "job":
		if h.Own {
			return fmt.Sprintf("job %s in capsule %s on this computer (%d MiB)", h.JobID, h.CapsuleID, h.Bytes>>20)
		}
		return fmt.Sprintf("a running job on %s (%d MiB)", who, h.Bytes>>20)
	}
	return fmt.Sprintf("capsules not yet resident on %s (%d MiB)", who, h.Bytes>>20)
}

// buildReservationSet is one computer's live build capsules and job leases
// in the ledger.
type buildReservationSet struct {
	vmID       string
	computerID string
	// epoch is the realization the set was reported from: a set from an
	// earlier boot of the VM is dropped, never inherited.
	epoch int64
	class WarmnessClass
	// idle is the footprint the builds are charged on top of, fixed when
	// the set is created (buildIdle); idleSource says how it was taken.
	idle       uint64
	idleSource string
	// ceiling is the guest's machine memory: the set never charges past it.
	ceiling    uint64
	entries    map[reservationKey]uint64
	reportedAt time.Time
}

// holders attributes the set's outstanding to its job leases first, then to
// its capsules' baselines. own reveals the ids.
func (s *buildReservationSet) holders(out uint64, own bool) []CapsuleMemoryHolder {
	keys := make([]reservationKey, 0, len(s.entries))
	for k := range s.entries {
		if k.job != "" {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if s.entries[keys[i]] != s.entries[keys[j]] {
			return s.entries[keys[i]] > s.entries[keys[j]]
		}
		return keys[i].capsule+"/"+keys[i].job < keys[j].capsule+"/"+keys[j].job
	})
	var hs []CapsuleMemoryHolder
	for _, k := range keys {
		if out == 0 {
			break
		}
		b := min(s.entries[k], out)
		out -= b
		h := CapsuleMemoryHolder{Kind: "job", Own: own, Class: string(s.class), Bytes: b}
		if own {
			h.CapsuleID, h.JobID = k.capsule, k.job
		}
		hs = append(hs, h)
	}
	if out > 0 {
		hs = append(hs, CapsuleMemoryHolder{Kind: "capsule", Own: own, Class: string(s.class), Bytes: out})
	}
	return hs
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
			IdleBytes: set.idle, IdleSource: set.idleSource,
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

// buildIdleSettledMaxAge bounds how long a settled RSS is trusted as a
// build's idle baseline; an older one is re-measured (the RSS before the
// capsule) rather than trusted.
const buildIdleSettledMaxAge = 6 * time.Hour

// settledIdle is a computer's D4 settled RSS with the realization it was
// measured on and the one it runs now.
type settledIdle struct {
	bytes   uint64
	epoch   int64
	at      string
	current int64
}

// fresh: measured on the realization the computer runs now (a stamped
// epoch; an unstamped one is from an older vmctl and could be from any
// boot), within buildIdleSettledMaxAge.
func (s settledIdle) fresh(now time.Time) bool {
	if s.bytes == 0 || s.epoch == 0 || s.epoch != s.current {
		return false
	}
	at, err := time.Parse(time.RFC3339, s.at)
	if err != nil {
		return false
	}
	age := now.Sub(at)
	return age >= 0 && age <= buildIdleSettledMaxAge
}

// buildIdle is the idle baseline a computer's builds are charged on top of
// (docs/problems/capsule-reservation-idle-baseline-and-job-double-gate-2026-11-03.md,
// problem 1). A settled RSS from an earlier realization (a deploy refresh
// onto a new release) or older than the bound under-reserves a build by
// whatever the computer's idle has grown since, so:
//   - with the RSS before any capsule known (preCapsule), it is the
//     baseline, raised to a fresh settled RSS when that is larger;
//   - without it (the first report already carries live builds, as after a
//     vmctl restart), a fresh settled RSS is the baseline, and a stale one
//     is only trusted as max(settled, rss), whose rss may hold build
//     growth: conservative, never under; with no reading at all it is
//     unmeasured (0), which charges the builds in full.
func buildIdle(settled settledIdle, rss uint64, preCapsule bool, now time.Time) (uint64, string) {
	fresh := settled.fresh(now)
	switch {
	case preCapsule && rss > 0:
		if fresh && settled.bytes > rss {
			return settled.bytes, "settled"
		}
		return rss, "pre_capsule_rss"
	case fresh:
		return settled.bytes, "settled"
	case rss == 0:
		// No reading yet (just after a vmctl restart) and no settle to
		// trust: unmeasured, so the builds are charged in full until the
		// first reading sets the baseline (review r1, devin F2).
		return 0, "unmeasured"
	default:
		return max(settled.bytes, rss), "max_settled_rss"
	}
}

func validReservationID(id string, optional bool) bool {
	if id == "" {
		return optional
	}
	return strings.TrimSpace(id) == id && len(id) <= 128 && !strings.ContainsAny(id, "\x00\n\r")
}

func validCapsuleReservation(c CapsuleReservation) bool {
	return validReservationID(c.CapsuleID, false) && validReservationID(c.JobID, true) && c.Bytes > 0
}

// memoryHoldersLocked is every holder of promised memory, largest first;
// vmID's own are named. Callers hold l.mu and have just run
// outstandingLocked.
func (l *hostMemoryLedger) memoryHoldersLocked(vmID string, views []AdmissionReservationView) []CapsuleMemoryHolder {
	var hs []CapsuleMemoryHolder
	for _, v := range views {
		if v.OutstandingBytes == 0 {
			continue
		}
		if v.NeedSource != "capsule_build" {
			hs = append(hs, CapsuleMemoryHolder{Kind: "start", Own: v.VMID == vmID, Class: v.Class, Bytes: v.OutstandingBytes})
			continue
		}
		if set := l.builds[v.VMID]; set != nil {
			hs = append(hs, set.holders(v.OutstandingBytes, v.VMID == vmID)...)
		}
	}
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].Bytes > hs[j].Bytes })
	return hs
}

// ReportCapsuleReservations records a computer's live set of build capsule
// baselines and job leases, and admits its new one, if any.
//
// An admission is decided under the ledger lock with a host sample taken
// inside it, as a start's is, and an admitted lease joins the reserved term
// before the lock is released: two guests starting jobs in the same second
// are decided one after the other, and the second sees the first's lease.
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
	vmID := own.VMID
	epoch := own.Epoch
	settled := settledIdle{bytes: own.MemorySettledRSSBytes, epoch: own.MemorySettledEpoch, at: own.MemorySettledAt, current: own.Epoch}
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
	if set != nil && set.epoch != epoch {
		// Reported from an earlier realization of the VM: whatever its
		// capsules and jobs held died with that boot.
		log.Printf("vmctl: capsule reservations of VM %s dropped: reported from epoch %d, the VM now runs epoch %d", vmID, set.epoch, epoch)
		delete(led.builds, vmID)
		set = nil
	}
	if set == nil {
		// A set is created by the report that first carries a live or
		// admitted capsule: with none reported, the RSS now is the computer
		// before any capsule.
		set = &buildReservationSet{vmID: vmID, computerID: computerID, epoch: epoch}
		set.idle, set.idleSource = buildIdle(settled, rss, len(report.Reservations) == 0, now)
		led.builds[vmID] = set
	} else if set.idle == 0 && rss > 0 {
		// Created before the sampler's first reading of this VM (just
		// after a vmctl restart): the reading now may hold build growth,
		// so it only ever raises the baseline.
		set.idle, set.idleSource = buildIdle(settled, rss, false, now)
	}
	set.class, set.ceiling, set.reportedAt = class, ceiling, now
	set.entries = map[reservationKey]uint64{}
	clamp := func(b uint64) uint64 {
		if ceiling > 0 && b > ceiling {
			return ceiling
		}
		return b
	}
	for _, c := range report.Reservations {
		set.entries[c.key()] = clamp(c.Bytes)
	}

	reserved, views := led.outstandingLocked(now, acfg)
	floor := memoryFloorBytes(cfg, sample.MemoryTotalBytes)
	// checked: the spawn gate is in force, and with it the burst pool.
	checked := cfg.Mode == PressureReclaimModeActive && acfg.Mode == HostAdmissionModeMeasured
	pressure := hostPressureHigh(sample, cfg)
	// The pool is part of the room only where the gate keeps it: a
	// protected computer is always admitted (review r1, DeepSeek F2).
	pool := uint64(0)
	if checked && !protectedWarmnessClass(class) {
		pool = acfg.HeadroomBytes
	}
	var result CapsuleReservationResult
	if admit := report.Admit; admit != nil {
		if _, held := set.entries[admit.key()]; held {
			yes := true
			result.Admitted = &yes
		} else {
			bytes := clamp(admit.Bytes)
			before := set.outstanding(rss, 0)
			increment := set.outstanding(rss, bytes) - before
			job := admit.JobID != ""
			what, source := "build", "capsule_build"
			if job {
				what, source = "job", "capsule_job"
			}
			d := HostAdmissionDecision{
				At: now.UTC().Format(time.RFC3339), Mode: acfg.Mode, VMID: vmID, ComputerID: computerID,
				Class: string(class), NeedBytes: increment, NeedSource: source,
				AvailableBytes: sample.MemoryAvailableBytes, ReservedBytes: reserved,
				FloorBytes: floor, HeadroomBytes: acfg.HeadroomBytes,
			}
			required := floor + acfg.HeadroomBytes + reserved + increment
			switch {
			case !checked:
				d.Decision = "admit_" + what + "_unchecked" // start admission is off too
			case protectedWarmnessClass(class):
				d.Decision = "admit_" + what + "_protected"
			case sample.MemoryAvailableBytes == 0:
				d.Decision = "admit_" + what + "_unknown_sample"
			case job && pressure:
				// A job is where memory is used: under host memory
				// pressure it waits even when the arithmetic fits.
				d.Decision = "refuse_job"
				d.Reason = fmt.Sprintf("the host is under memory pressure (PSI some avg10 %.1f%%, avg60 %.1f%%)", sample.MemorySomeAvg10, sample.MemorySomeAvg60)
			case sample.MemoryAvailableBytes >= required:
				d.Decision = "admit_" + what
			default:
				d.Decision = "refuse_" + what
				d.Reason = fmt.Sprintf("host memory available %d MiB; %d MiB reserved for starts in flight and running jobs; this %s needs %d MiB above the %d MiB floor and %d MiB headroom",
					d.AvailableBytes>>20, d.ReservedBytes>>20, what, d.NeedBytes>>20, d.FloorBytes>>20, d.HeadroomBytes>>20)
			}
			admitted := !strings.HasPrefix(d.Decision, "refuse_")
			if !admitted {
				wait := &CapsuleMemoryWait{NeedBytes: increment, AvailableBytes: sample.MemoryAvailableBytes, FloorBytes: floor,
					PoolBytes: pool, ReservedBytes: reserved, PressureHigh: pressure, Holders: led.memoryHoldersLocked(vmID, views)}
				if spare := floor + pool + reserved; sample.MemoryAvailableBytes > spare {
					wait.RoomBytes = sample.MemoryAvailableBytes - spare
				}
				if len(wait.Holders) > 0 {
					d.Reason += "; the largest holder is " + wait.Holders[0].String()
				}
				result.Wait = wait
			}
			led.recordLocked(d)
			logAdmissionDecision(d)
			result.Admitted = &admitted
			if admitted {
				set.entries[admit.key()] = bytes
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
	// capsules and jobs) is not promised to another (review r1, deepseek).
	mine := uint64(0)
	for _, v := range views {
		if v.VMID == vmID && v.NeedSource != "capsule_build" {
			mine += v.OutstandingBytes
		}
	}
	ownBuilds := uint64(0)
	if len(set.entries) > 0 {
		ownBuilds = set.outstanding(rss, 0)
		mine += ownBuilds
	}
	others := uint64(0)
	if reserved > mine {
		others = reserved - mine
	}
	facts := HostMemoryFacts{
		SampledAt: now.UTC().Format(time.RFC3339Nano), MemAvailableBytes: sample.MemoryAvailableBytes,
		FloorBytes: floor, ReservedOthersBytes: others, BurstPoolBytes: pool, OwnReservedBytes: ownBuilds,
		MemorySomeAvg10: sample.MemorySomeAvg10, MemorySomeAvg60: sample.MemorySomeAvg60,
		PressureHigh: pressure, Priority: capsulePriority(class),
	}
	if spare := floor + pool + others; sample.MemoryAvailableBytes > spare {
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
