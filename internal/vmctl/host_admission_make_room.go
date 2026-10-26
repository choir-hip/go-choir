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
// sample, one victim at a time: after each hibernate the shortfall is
// recomputed from a fresh sample and the plan is re-read from the live
// registry, and a victim is probed busy only just before its own hibernate
// (review 302). It is refused only when the eligible computers together
// could not cover the shortfall, the cap per start was reached, or the freed
// memory never showed up, and the refusal says why.
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
// A guest that answers busy at the hibernate fence is left running and counted
// as a skip (errHibernateGuestBusy), never killed.
//
// Fails closed: off unless VMCTL_ADMISSION_MAKE_ROOM is exactly active, on,
// enabled, true or 1 (nix/node-b.nix sets active). Rollback:
// VMCTL_ADMISSION_MAKE_ROOM=off refuses as before.

const (
	HostAdmissionMakeRoomActive = "active"
	HostAdmissionMakeRoomOff    = "off"

	// stoppedByMakeRoom is the StoppedBy of a computer hibernated to make
	// room for another's start. A request wakes it like any hibernate.
	stoppedByMakeRoom = "make_room"

	// The env overrides are capped where they are read (review 302 F5): a
	// mistyped bound cannot hibernate a host's worth of computers or hold a
	// start for an hour.
	MaxMakeRoomVictims    = 10
	MaxMakeRoomSettleWait = 60 * time.Second

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
	// skippedBy is the reason each skipped computer was skipped for, so a
	// start that also tried one counts it once.
	skippedBy map[string]string
	pending   uint64
}

func (p *makeRoomPlan) skip(reason, vmID string) {
	if p.skipped == nil {
		p.skipped = map[string]int{}
		p.skippedBy = map[string]string{}
	}
	p.skipped[reason]++
	p.skippedBy[vmID] = reason
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
			plan.skip("lifecycle", own.VMID)
			continue
		}
		if own.State != VMStateActive {
			continue
		}
		class := warmnessClassForOwnership(own, policy)
		idle := time.Duration(0)
		if !own.LastActiveAt.IsZero() {
			lastActive := own.LastActiveAt
			if busyAt := r.guestLastBusyAt[own.VMID]; busyAt.After(lastActive) {
				lastActive = busyAt // work is activity (density D3, PR-09)
			}
			idle = now.Sub(lastActive)
		}
		switch {
		case own.IsHeld():
			plan.skip("held", own.VMID)
			continue
		case protectedWarmnessClass(class):
			plan.skip("protected", own.VMID)
			continue
		case warmnessPriority(class) > starterPriority:
			plan.skip("higher_class", own.VMID)
			continue
		case own.LastActiveAt.IsZero():
			plan.skip("unknown_last_active", own.VMID)
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
			plan.skip("recent_activity", own.VMID)
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
// make-room slot, re-admit on a fresh sample; else hibernate one victim at a
// time, re-planning from the live registry and a fresh sample after each,
// until a fresh sample admits, the per-start cap is reached, or nothing
// eligible is left (review 302 F1, F2). stillNeeded, when set, is asked
// before each hibernate; false ends with errMakeRoomSuperseded. A cancelled
// ctx ends it too. Either way, a start that already hibernated victims
// records a decision naming them (F4). The caller must not hold r.mu.
func (r *OwnershipRegistry) makeRoomAndAdmit(ctx context.Context, own *VMOwnership, class WarmnessClass, stillNeeded func() bool) (*admissionTicket, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("make-room cancelled: %w", err)
	}
	if stillNeeded != nil && !stillNeeded() {
		return nil, errMakeRoomSuperseded
	}

	// A start that waited for another's make-room may fit now.
	first, ticket := r.attemptMeasuredAdmission(own, class, cfg, acfg, sampler, false)
	if ticket != nil {
		r.recordMakeRoom(first, "admit", nil, "")
		return ticket, nil
	}
	cur := first
	shortfall := admissionShortfall(cur)
	hold := r.hibernationHoldReason(r.guestBusy(), r.now)
	var made []string
	// tried is why each computer this start looked at was passed over (or
	// that it was hibernated); one entry per computer, so a computer probed
	// twice is counted once.
	tried := map[string]string{}
	abandon := func(cause error, why string) (*admissionTicket, error) {
		if len(made) > 0 {
			r.recordMakeRoom(first, "abandon_make_room", made, why)
		}
		return nil, cause
	}
	// pick is the first ranked candidate not yet tried that passes the route
	// guard and a fresh busy probe now.
	pick := func(plan *makeRoomPlan) *makeRoomCandidate {
		for i := range plan.candidates {
			c := &plan.candidates[i]
			if _, done := tried[c.own.VMID]; done {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			if !authorizeLifecycleRoute(ctx, guard, c.own.UserID, c.own.DesktopID) {
				tried[c.own.VMID] = "route"
				continue
			}
			if reason := hold(c.own); reason != "" {
				tried[c.own.VMID] = reason
				continue
			}
			return c
		}
		return nil
	}
	skippedText := func(plan *makeRoomPlan) string {
		merged := makeRoomPlan{skipped: map[string]int{}}
		for reason, n := range plan.skipped {
			merged.skipped[reason] += n
		}
		// A computer this start tried and the latest plan skips is one
		// computer: the reason it was tried for replaces the plan's.
		for vmID, reason := range tried {
			if reason == "hibernated" {
				continue
			}
			if prev, ok := plan.skippedBy[vmID]; ok {
				if merged.skipped[prev]--; merged.skipped[prev] <= 0 {
					delete(merged.skipped, prev)
				}
			}
			merged.skipped[reason]++
		}
		return merged.skippedText()
	}

	plan := r.planMakeRoom(own, class, acfg)
	// Another path's in-flight hibernate (the idle sweep, reclaim) is not
	// this start's room: its memory may never show. When it alone would
	// cover the shortfall, wait for it before evicting anyone else.
	if plan.pending > 0 && plan.pending >= shortfall {
		d, ticket, err := r.awaitAdmission(ctx, own, class, cfg, acfg, sampler)
		if err != nil {
			return abandon(fmt.Errorf("make-room cancelled: %w", err), "cancelled")
		}
		if ticket != nil {
			r.recordMakeRoom(first, "admit_after_make_room", nil, "")
			return ticket, nil
		}
		cur, shortfall = d, admissionShortfall(d)
		plan = r.planMakeRoom(own, class, acfg)
	}
	// Refuse before hibernating anyone when the eligible computers
	// themselves (pending excluded) could not cover the shortfall within
	// the per-start cap.
	var cover uint64
	eligible := 0
	for i := range plan.candidates {
		if eligible >= acfg.MakeRoomMaxVictims || cover >= shortfall {
			break
		}
		c := &plan.candidates[i]
		if _, done := tried[c.own.VMID]; done {
			continue
		}
		if ctx.Err() != nil {
			return abandon(fmt.Errorf("make-room cancelled: %w", ctx.Err()), "cancelled")
		}
		if !authorizeLifecycleRoute(ctx, guard, c.own.UserID, c.own.DesktopID) {
			tried[c.own.VMID] = "route"
			continue
		}
		if reason := hold(c.own); reason != "" {
			tried[c.own.VMID] = reason
			continue
		}
		cover += c.rss
		eligible++
	}
	if cover < shortfall {
		// A start is refused on a fresh sample only: another path's
		// hibernate may have landed while this one planned.
		var ticket *admissionTicket
		if cur, ticket = r.attemptMeasuredAdmission(own, class, cfg, acfg, sampler, false); ticket != nil {
			r.recordMakeRoom(first, "admit_after_make_room", nil, "")
			return ticket, nil
		}
		shortfall = admissionShortfall(cur)
		// These probes only sized the plan; the loop below re-probes each
		// victim just before its hibernate.
		detail := noRoomDetail(shortfall, cover, eligible, acfg, plan.pending, skippedText(&plan))
		return nil, r.refuseMakeRoom(first, cur, nil, detail)
	}
	for k := range tried {
		delete(tried, k)
	}

	lastWaitTimedOut := false
	for len(made) < acfg.MakeRoomMaxVictims {
		if err := ctx.Err(); err != nil {
			return abandon(fmt.Errorf("make-room cancelled: %w", err), "cancelled")
		}
		if stillNeeded != nil && !stillNeeded() {
			return abandon(errMakeRoomSuperseded, "superseded: the computer was registered meanwhile")
		}
		var ticket *admissionTicket
		if cur, ticket = r.attemptMeasuredAdmission(own, class, cfg, acfg, sampler, false); ticket != nil {
			r.recordMakeRoom(first, "admit_after_make_room", made, "")
			return ticket, nil
		}
		shortfall = admissionShortfall(cur)
		// F1: the live registry, read now, decides who is still eligible;
		// the busy probe runs just before the hibernate, not at plan time.
		plan = r.planMakeRoom(own, class, acfg)
		v := pick(&plan)
		if v == nil {
			break
		}
		// pick's last probe can outlast the request: never hibernate a
		// computer for one that went away meanwhile (review 312 Low 2).
		if err := ctx.Err(); err != nil {
			return abandon(fmt.Errorf("make-room cancelled: %w", err), "cancelled")
		}
		log.Printf("vmctl: make-room decision=hibernate for_vm=%s for_computer=%s short_mib=%d victim_vm=%s victim_computer=%s class=%s idle_s=%d rss_mib=%d",
			first.VMID, first.ComputerID, shortfall>>20, v.own.VMID, v.own.ComputerID, v.class, int64(v.idle.Seconds()), v.rss>>20)
		if err := r.hibernateVMForDesktopWithReason(v.own.UserID, v.own.DesktopID, stoppedByMakeRoom); err != nil {
			// The guest answered busy (left running), or another path took
			// it (the sweep, reclaim, a visit). Neither is room; the next
			// victim is tried on a fresh plan.
			reason := "hibernate_failed"
			if errors.Is(err, errHibernateGuestBusy) {
				reason = "hibernate_busy"
			}
			tried[v.own.VMID] = reason
			log.Printf("vmctl: make-room hibernate of VM %s skipped (%s): %v", v.own.VMID, reason, err)
			continue
		}
		tried[v.own.VMID] = "hibernated"
		made = append(made, v.own.VMID)
		d, ticket, err := r.awaitAdmission(ctx, own, class, cfg, acfg, sampler)
		if err != nil {
			return abandon(fmt.Errorf("make-room cancelled: %w", err), "cancelled")
		}
		if ticket != nil {
			r.recordMakeRoom(first, "admit_after_make_room", made, "")
			return ticket, nil
		}
		cur, lastWaitTimedOut = d, true
	}
	// A cancelled start is abandoned, not given a last attempt that could
	// admit and boot a request nobody waits for: pick stops on a cancelled
	// ctx, which is not the plan running out of victims (review 310 Low 1).
	if err := ctx.Err(); err != nil {
		return abandon(fmt.Errorf("make-room cancelled: %w", err), "cancelled")
	}
	cur, ticket = r.attemptMeasuredAdmission(own, class, cfg, acfg, sampler, false)
	if ticket != nil {
		r.recordMakeRoom(first, "admit_after_make_room", made, "")
		return ticket, nil
	}
	shortfall = admissionShortfall(cur)
	if len(made) == 0 {
		return nil, r.refuseMakeRoom(first, cur, nil, noRoomDetail(shortfall, 0, 0, acfg, plan.pending, skippedText(&plan)))
	}
	var detail string
	switch {
	case len(made) >= acfg.MakeRoomMaxVictims:
		detail = fmt.Sprintf("short %d MiB after hibernating [%s]: the cap of %d per start is reached", shortfall>>20, strings.Join(made, " "), acfg.MakeRoomMaxVictims)
	case lastWaitTimedOut && len(tried) == len(made):
		detail = fmt.Sprintf("short %d MiB; made room by hibernating [%s] but memory did not show within %s",
			shortfall>>20, strings.Join(made, " "), acfg.MakeRoomSettleWait)
	default:
		detail = fmt.Sprintf("short %d MiB after hibernating [%s]; no further eligible computer: %s", shortfall>>20, strings.Join(made, " "), skippedText(&plan))
	}
	return nil, r.refuseMakeRoom(first, cur, made, detail)
}

func noRoomDetail(shortfall, cover uint64, eligible int, acfg HostAdmissionConfig, pending uint64, skipped string) string {
	return fmt.Sprintf("short %d MiB; no computer can make room: %d MiB from %d eligible (at most %d per start), %d MiB already being hibernated by other paths; %s",
		shortfall>>20, cover>>20, eligible, acfg.MakeRoomMaxVictims, pending>>20, skipped)
}

// awaitAdmission re-admits on fresh samples until one admits, the settle
// wait ends, or ctx is cancelled. It returns the last refused decision.
func (r *OwnershipRegistry) awaitAdmission(ctx context.Context, own *VMOwnership, class WarmnessClass, cfg PressureReclaimConfig, acfg HostAdmissionConfig, sampler hostPressureSampler) (HostAdmissionDecision, *admissionTicket, error) {
	deadline := time.Now().Add(acfg.MakeRoomSettleWait)
	for {
		d, ticket := r.attemptMeasuredAdmission(own, class, cfg, acfg, sampler, false)
		if ticket != nil {
			return d, ticket, nil
		}
		if !time.Now().Before(deadline) {
			return d, nil, nil
		}
		timer := time.NewTimer(acfg.MakeRoomPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return d, nil, ctx.Err()
		case <-timer.C:
		}
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

// refuseMakeRoom records the refusal with the start's first sample and
// words it with the latest, the one the detail's shortfall comes from.
func (r *OwnershipRegistry) refuseMakeRoom(first, latest HostAdmissionDecision, made []string, detail string) error {
	r.recordMakeRoom(first, "refuse_no_room", made, detail)
	return hostMemoryRefusal(latest, detail)
}

// stillNeededLocked reports that no request has registered the computer at
// key. Caller holds r.mu.
func stillNeededLocked(r *OwnershipRegistry, key string) bool {
	_, registered := r.ownerships[key]
	_, pending := r.pendingWaiters[key]
	return !registered && !pending
}
