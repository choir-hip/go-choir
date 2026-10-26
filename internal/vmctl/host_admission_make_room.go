package vmctl

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// Make-room admission (density station D1;
// docs/problems/density-station-d-failure-modes-2026-10-26.md, D1).
//
// Before: a start that did not fit was refused (host_memory_pressure, 503)
// and nothing was reclaimed until the host fell under its floor, so a
// person stopped an idle computer by hand (b@c, 2026-10-26 11:49, the
// fourth recurrence). Now a refused start hibernates idle eligible
// computers, lowest class first, longest idle first, and admits on a fresh
// sample. It is refused only when the eligible computers together could not
// cover the shortfall, or the freed memory never showed up, and the refusal
// says why.
//
// One decider: make-room runs under makeRoomMu, so a second refused start
// waits and re-admits on a fresh sample (seeing the first start's
// reservation and the first victim's freed memory) before choosing anything.
// Every hibernate goes through hibernateVMForDesktopWithReason, which takes
// the victim's lifecycle claim, so a race with the idle sweep or pressure
// reclaim cannot hibernate one computer twice.
//
// A victim is never working (the guest's own busy probe, fail closed),
// never held, never protected (premium_always_on, critical_protected,
// public_platform), never of a higher class than the starter, never mid
// lifecycle, and idle by the pressure-reclaim rule.
//
// Rollback: VMCTL_ADMISSION_MAKE_ROOM=off refuses as before.

const (
	HostAdmissionMakeRoomActive = "active"
	HostAdmissionMakeRoomOff    = "off"

	// stoppedByMakeRoom is the StoppedBy of a computer hibernated to make
	// room for another's start. A request wakes it like any hibernate.
	stoppedByMakeRoom = "make_room"

	defaultMakeRoomMaxVictims   = 3
	defaultMakeRoomSettleWait   = 10 * time.Second
	defaultMakeRoomPollInterval = 250 * time.Millisecond
)

// errMakeRoomSuperseded: while the start waited to make room, another
// request registered the same computer; the caller starts over.
var errMakeRoomSuperseded = errors.New("make-room superseded: the computer was registered meanwhile")

// SetMakeRoomRouteGuard authorizes make-room's hibernates (the idle
// sweep's guard). Without one make-room hibernates nothing.
func (r *OwnershipRegistry) SetMakeRoomRouteGuard(guard ComputerVersionRouteGuard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.makeRoomGuard = guard
}

// makeRoomEnabledLocked reports whether a refused start may make room.
// Caller holds r.mu (read).
func (r *OwnershipRegistry) makeRoomEnabledLocked() bool {
	if r.vmManager == nil || r.makeRoomGuard == nil {
		return false
	}
	if normalizePressureReclaimConfig(r.pressureReclaim).Mode != PressureReclaimModeActive {
		return false
	}
	acfg := normalizeHostAdmissionConfig(r.hostAdmission)
	return acfg.Mode == HostAdmissionModeMeasured && acfg.MakeRoom == HostAdmissionMakeRoomActive
}

func makeRoomRefusal(err error) bool {
	refusal := RecoveryRefusalFrom(err)
	return refusal != nil && refusal.Kind == RecoveryRefusalHostMemoryPressure
}

type makeRoomCandidate struct {
	own      *VMOwnership
	class    WarmnessClass
	priority int
	idle     time.Duration
	rss      uint64
}

// makeRoomPlan is the ranked victims and why every other running computer
// was skipped. pending is memory a hibernate already in flight will free.
type makeRoomPlan struct {
	candidates []makeRoomCandidate
	skipped    map[string]int
	pending    uint64
}

func (p *makeRoomPlan) skip(reason string) {
	if p.skipped == nil {
		p.skipped = map[string]int{}
	}
	p.skipped[reason]++
}

func (p *makeRoomPlan) skippedText() string {
	if len(p.skipped) == 0 {
		return "no other computer is running"
	}
	reasons := make([]string, 0, len(p.skipped))
	for reason := range p.skipped {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		parts = append(parts, fmt.Sprintf("%s=%d", reason, p.skipped[reason]))
	}
	return "skipped " + strings.Join(parts, " ")
}

// planMakeRoom ranks the running computers a start of starter (class
// starterClass) may hibernate. It does not probe guests.
func (r *OwnershipRegistry) planMakeRoom(starter *VMOwnership, starterClass WarmnessClass, acfg HostAdmissionConfig) makeRoomPlan {
	var plan makeRoomPlan
	starterPriority := warmnessPriority(starterClass)
	var starterVM, starterKey string
	if starter != nil {
		starterVM = starter.VMID
		starterKey = ownershipKey(starter.UserID, normalizeDesktopID(starter.DesktopID))
	}
	r.mu.RLock()
	pcfg := normalizePressureReclaimConfig(r.pressureReclaim)
	policy := normalizeWarmnessPolicyConfig(r.warmnessPolicy)
	now := r.now()
	led := &r.admissionLedger
	led.mu.Lock()
	led.initLocked()
	residentOf := func(own *VMOwnership) uint64 {
		if obs, ok := led.rss[own.VMID]; ok && obs.bytes > 0 {
			return obs.bytes
		}
		return own.MemoryPeakRSSBytes
	}
	for key, own := range r.ownerships {
		if own == nil || own.VMID == starterVM || key == starterKey {
			continue
		}
		if holder := r.lifecycle[key]; holder == lifecycleHibernate {
			// Already being hibernated (the idle sweep, pressure reclaim,
			// an operator): its memory is on the way.
			plan.pending += residentOf(own)
			continue
		} else if holder != "" && (own.State == VMStateActive || own.State == VMStateDegraded) {
			plan.skip("lifecycle")
			continue
		}
		if own.State != VMStateActive {
			continue
		}
		class := warmnessClassForOwnership(own, policy)
		idle := time.Duration(0)
		if !own.LastActiveAt.IsZero() {
			idle = now.Sub(own.LastActiveAt)
		}
		switch {
		case own.IsHeld():
			plan.skip("held")
			continue
		case protectedWarmnessClass(class):
			plan.skip("protected")
			continue
		case warmnessPriority(class) > starterPriority:
			plan.skip("higher_class")
			continue
		case own.LastActiveAt.IsZero():
			plan.skip("unknown_last_active")
			continue
		}
		// The pressure-reclaim idle rule: a test computer yields to a real
		// one however recently used; a test start waits out another test
		// computer's settle window; everything else must be idle MinIdle.
		minIdle := pcfg.MinIdle
		if class == WarmnessClassTest {
			minIdle = 0
			if starterClass == WarmnessClassTest {
				minIdle = acfg.Settle
			}
		}
		if idle < minIdle {
			plan.skip("recent_activity")
			continue
		}
		plan.candidates = append(plan.candidates, makeRoomCandidate{
			own: cloneOwnership(own), class: class, priority: warmnessPriority(class), idle: idle, rss: residentOf(own),
		})
	}
	led.mu.Unlock()
	r.mu.RUnlock()
	sort.SliceStable(plan.candidates, func(i, j int) bool {
		left, right := plan.candidates[i], plan.candidates[j]
		if left.priority != right.priority {
			return left.priority < right.priority
		}
		if left.idle != right.idle {
			return left.idle > right.idle
		}
		return left.own.VMID < right.own.VMID
	})
	return plan
}

// makeRoomAndAdmit is a refused start's second chance: under the one
// make-room slot, re-admit on a fresh sample; else hibernate victims one at
// a time until a fresh sample admits. stillNeeded, when set, is asked before
// any hibernate; false ends with errMakeRoomSuperseded. The caller must not
// hold r.mu.
func (r *OwnershipRegistry) makeRoomAndAdmit(own *VMOwnership, class WarmnessClass, stillNeeded func() bool) (*admissionTicket, error) {
	r.makeRoomMu.Lock()
	defer r.makeRoomMu.Unlock()

	r.mu.RLock()
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	acfg := normalizeHostAdmissionConfig(r.hostAdmission)
	sampler := r.pressureSampler
	guard := r.makeRoomGuard
	r.mu.RUnlock()
	if sampler == nil {
		sampler = sampleHostPressure
	}
	if stillNeeded != nil && !stillNeeded() {
		return nil, errMakeRoomSuperseded
	}

	// A start that waited for another's make-room may fit now.
	d, ticket := r.attemptMeasuredAdmission(own, class, cfg, acfg, sampler, false)
	if ticket != nil {
		r.recordMakeRoom(d, "admit", nil, "")
		return ticket, nil
	}
	shortfall := admissionShortfall(d)

	plan := r.planMakeRoom(own, class, acfg)
	busy := r.guestBusy()
	var victims []makeRoomCandidate
	cover := plan.pending
	for _, c := range plan.candidates {
		if len(victims) >= acfg.MakeRoomMaxVictims || cover >= shortfall {
			break
		}
		if busy(c.own) {
			plan.skip("busy")
			continue
		}
		if !authorizeLifecycleRoute(context.Background(), guard, c.own.UserID, c.own.DesktopID) {
			plan.skip("route")
			continue
		}
		victims = append(victims, c)
		cover += c.rss
	}
	if cover < shortfall {
		detail := fmt.Sprintf("short %d MiB; no computer can make room: %d MiB from %d eligible (at most %d per start), %d MiB already being hibernated; %s",
			shortfall>>20, (cover-plan.pending)>>20, len(victims), acfg.MakeRoomMaxVictims, plan.pending>>20, plan.skippedText())
		return nil, r.refuseMakeRoom(d, nil, detail)
	}

	var made []string
	if plan.pending > 0 {
		if ticket := r.awaitAdmission(own, class, cfg, acfg, sampler); ticket != nil {
			r.recordMakeRoom(d, "admit_after_make_room", nil, "")
			return ticket, nil
		}
	}
	for _, v := range victims {
		if stillNeeded != nil && !stillNeeded() {
			return nil, errMakeRoomSuperseded
		}
		log.Printf("vmctl: make-room decision=hibernate for_vm=%s for_computer=%s short_mib=%d victim_vm=%s victim_computer=%s class=%s idle_s=%d rss_mib=%d",
			d.VMID, d.ComputerID, shortfall>>20, v.own.VMID, v.own.ComputerID, v.class, int64(v.idle.Seconds()), v.rss>>20)
		if err := r.hibernateVMForDesktopWithReason(v.own.UserID, v.own.DesktopID, stoppedByMakeRoom); err != nil {
			// Another path took it (the sweep, reclaim, a visit); its memory
			// is either on the way or in use again. The fresh sample decides.
			log.Printf("vmctl: make-room hibernate of VM %s skipped: %v", v.own.VMID, err)
		} else {
			made = append(made, v.own.VMID)
		}
		if ticket := r.awaitAdmission(own, class, cfg, acfg, sampler); ticket != nil {
			r.recordMakeRoom(d, "admit_after_make_room", made, "")
			return ticket, nil
		}
	}
	detail := fmt.Sprintf("short %d MiB; made room by hibernating [%s] but memory did not show within %s",
		shortfall>>20, strings.Join(made, " "), acfg.MakeRoomSettleWait)
	return nil, r.refuseMakeRoom(d, made, detail)
}

// awaitAdmission re-admits on fresh samples until one admits or the settle
// wait ends.
func (r *OwnershipRegistry) awaitAdmission(own *VMOwnership, class WarmnessClass, cfg PressureReclaimConfig, acfg HostAdmissionConfig, sampler hostPressureSampler) *admissionTicket {
	deadline := time.Now().Add(acfg.MakeRoomSettleWait)
	for {
		if _, ticket := r.attemptMeasuredAdmission(own, class, cfg, acfg, sampler, false); ticket != nil {
			return ticket
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(acfg.MakeRoomPollInterval)
	}
}

// recordMakeRoom records make-room's outcome as one decision: the first
// refused sample's inputs, the decision, and the computers it hibernated.
func (r *OwnershipRegistry) recordMakeRoom(d HostAdmissionDecision, decision string, made []string, reason string) {
	d.Decision = decision
	d.MadeRoomVMIDs = append([]string(nil), made...)
	d.Reason = reason
	led := &r.admissionLedger
	led.mu.Lock()
	led.recordLocked(d)
	led.mu.Unlock()
	logAdmissionDecision(d)
	if len(made) > 0 || reason != "" {
		log.Printf("vmctl: make-room decision=%s vm=%s computer=%s hibernated=[%s] reason=%q", decision, d.VMID, d.ComputerID, strings.Join(made, " "), reason)
	}
}

func (r *OwnershipRegistry) refuseMakeRoom(d HostAdmissionDecision, made []string, detail string) error {
	r.recordMakeRoom(d, "refuse_no_room", made, detail)
	return hostMemoryRefusal(d, detail)
}

// stillNeededLocked reports that no request has registered the computer at
// key. Caller holds r.mu.
func stillNeededLocked(r *OwnershipRegistry, key string) bool {
	_, registered := r.ownerships[key]
	_, pending := r.pendingWaiters[key]
	return !registered && !pending
}
