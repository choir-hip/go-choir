package vmctl

import (
	"context"
	"errors"
	"log"
	"time"
)

// Sustained pressure reclaim (density quick tier, lever L2;
// docs/problems/pressure-reclaim-wedged-a-user-computer-2026-10-14.md).
//
// A pressure monitor samples the host every SampleInterval. A sample
// qualifies when memory is under the floor AND PSI memory some avg60 is over
// its threshold (or the state dir is under its floor). A reclaim needs
// SustainedSamples qualifying samples in a row. A sample taken while any
// hibernate writes its snapshot, or inside the cool-down after one ends,
// never counts: our own write is not evidence of a shortage. At most one
// reclaim runs at a time and it hibernates one computer.

const pressureRecentObservations = 8

// pressureWindow is the monitor's state. Guarded by OwnershipRegistry.mu.
type pressureWindow struct {
	streak               int
	lastAt               time.Time
	lastSample           HostPressureSample
	recent               []PressureObservation
	hibernatesInFlight   int
	lastHibernateEndedAt time.Time
	reclaimInFlight      bool
	lastReclaimAt        time.Time
	lastHoldLog          string
	lastHoldLogAt        time.Time
}

// PressureObservation is one monitor sample as the sustained signal judged it.
type PressureObservation struct {
	At                   string  `json:"at"`
	MemoryAvailableBytes uint64  `json:"memory_available_bytes,omitempty"`
	MemoryFloorBytes     uint64  `json:"memory_floor_bytes,omitempty"`
	MemorySomeAvg10      float64 `json:"memory_some_avg10"`
	MemorySomeAvg60      float64 `json:"memory_some_avg60"`
	StateDirPressure     bool    `json:"state_dir_pressure,omitempty"`
	Qualifies            bool    `json:"qualifies"`
	// Suppressed names why a qualifying sample did not count:
	// hibernate_in_flight or cooldown.
	Suppressed string `json:"suppressed,omitempty"`
}

// SustainedPressureState is the monitor's view, reported in vmctl health.
type SustainedPressureState struct {
	Signal                   string                `json:"signal"`
	RequiredSamples          int                   `json:"required_samples"`
	Streak                   int                   `json:"streak"`
	Sustained                bool                  `json:"sustained"`
	Stale                    bool                  `json:"stale,omitempty"`
	LastSampleAt             string                `json:"last_sample_at,omitempty"`
	HibernatesInFlight       int                   `json:"hibernates_in_flight"`
	ReclaimInFlight          bool                  `json:"reclaim_in_flight"`
	CooldownRemainingSeconds int64                 `json:"cooldown_remaining_seconds,omitempty"`
	LastReclaimAt            string                `json:"last_reclaim_at,omitempty"`
	Hold                     string                `json:"hold,omitempty"`
	Recent                   []PressureObservation `json:"recent,omitempty"`
}

// recordPressureSample adds one monitor sample to the window.
func (r *OwnershipRegistry) recordPressureSample(sample HostPressureSample) PressureObservation {
	r.mu.Lock()
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	annotatePressure(&sample, cfg)
	now := r.now()
	w := &r.pressureWin
	obs := PressureObservation{
		At:                   now.UTC().Format(time.RFC3339),
		MemoryAvailableBytes: sample.MemoryAvailableBytes,
		MemoryFloorBytes:     sample.MemoryFloorBytes,
		MemorySomeAvg10:      sample.MemorySomeAvg10,
		MemorySomeAvg60:      sample.MemorySomeAvg60,
		StateDirPressure:     sample.StateDirPressure,
		Qualifies:            sample.ReclaimPressure,
	}
	switch {
	case w.hibernatesInFlight > 0:
		obs.Suppressed = "hibernate_in_flight"
	case !w.lastHibernateEndedAt.IsZero() && !now.After(w.lastHibernateEndedAt.Add(cfg.Cooldown)):
		obs.Suppressed = "cooldown"
	}
	counts := obs.Qualifies && obs.Suppressed == ""
	contiguous := !w.lastAt.IsZero() && now.Sub(w.lastAt) <= 2*cfg.SampleInterval
	switch {
	case counts && contiguous && w.streak > 0:
		w.streak++
	case counts:
		w.streak = 1
	default:
		w.streak = 0
	}
	w.lastAt = now
	w.lastSample = sample
	w.recent = append(w.recent, obs)
	if len(w.recent) > pressureRecentObservations {
		w.recent = append([]PressureObservation(nil), w.recent[len(w.recent)-pressureRecentObservations:]...)
	}
	streak := w.streak
	r.mu.Unlock()

	if cfg.Signal == PressureReclaimSignalSustained && obs.Qualifies {
		log.Printf("vmctl: pressure sample qualifies signal=sustained streak=%d/%d suppressed=%s mem_available_mib=%d floor_mib=%d some_avg10=%.2f some_avg60=%.2f state_dir_pressure=%v",
			streak, cfg.SustainedSamples, firstNonEmpty(obs.Suppressed, "none"), obs.MemoryAvailableBytes>>20, obs.MemoryFloorBytes>>20,
			obs.MemorySomeAvg10, obs.MemorySomeAvg60, obs.StateDirPressure)
	}
	return obs
}

// SustainedPressureState reports the monitor's window without sampling.
func (r *OwnershipRegistry) SustainedPressureState() SustainedPressureState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sustainedPressureStateLocked()
}

func (r *OwnershipRegistry) sustainedPressureStateLocked() SustainedPressureState {
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	now := r.now()
	w := &r.pressureWin
	st := SustainedPressureState{
		Signal:             cfg.Signal,
		RequiredSamples:    cfg.SustainedSamples,
		Streak:             w.streak,
		HibernatesInFlight: w.hibernatesInFlight,
		ReclaimInFlight:    w.reclaimInFlight,
		Recent:             append([]PressureObservation(nil), w.recent...),
	}
	if !w.lastAt.IsZero() {
		st.LastSampleAt = w.lastAt.UTC().Format(time.RFC3339)
	}
	if !w.lastReclaimAt.IsZero() {
		st.LastReclaimAt = w.lastReclaimAt.UTC().Format(time.RFC3339)
	}
	st.Stale = w.lastAt.IsZero() || now.Sub(w.lastAt) > 2*cfg.SampleInterval
	st.Sustained = !st.Stale && w.streak >= cfg.SustainedSamples
	if !w.lastHibernateEndedAt.IsZero() {
		if remaining := w.lastHibernateEndedAt.Add(cfg.Cooldown).Sub(now); remaining > 0 {
			st.CooldownRemainingSeconds = int64(remaining.Round(time.Second).Seconds())
		}
	}
	switch {
	case w.reclaimInFlight:
		st.Hold = "reclaim_in_flight"
	case w.hibernatesInFlight > 0:
		st.Hold = "hibernate_in_flight"
	case st.CooldownRemainingSeconds > 0:
		st.Hold = "cooldown"
	}
	return st
}

// beginSustainedReclaim claims the single reclaim slot when the window is
// sustained and nothing else is writing a snapshot.
func (r *OwnershipRegistry) beginSustainedReclaim() (SustainedPressureState, HostPressureSample, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.sustainedPressureStateLocked()
	if !st.Sustained || st.Hold != "" {
		return st, r.pressureWin.lastSample, false
	}
	r.pressureWin.reclaimInFlight = true
	return st, r.pressureWin.lastSample, true
}

func (r *OwnershipRegistry) endSustainedReclaim(hibernated bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pressureWin.reclaimInFlight = false
	if hibernated {
		r.pressureWin.lastReclaimAt = r.now()
	}
}

// noteHibernateStartLocked and noteHibernateEndLocked bracket every
// hibernate (idle, pressure, operator): its snapshot write is our own
// pressure. Caller holds r.mu.
func (r *OwnershipRegistry) noteHibernateStartLocked() {
	r.pressureWin.hibernatesInFlight++
}

func (r *OwnershipRegistry) noteHibernateEndLocked() {
	if r.pressureWin.hibernatesInFlight > 0 {
		r.pressureWin.hibernatesInFlight--
	}
	r.pressureWin.lastHibernateEndedAt = r.now()
}

// reclaimSustained hibernates at most one eligible computer when the
// window is sustained, logging the inputs it decided on.
func (r *OwnershipRegistry) reclaimSustained(ctx context.Context, guard ComputerVersionRouteGuard, cfg PressureReclaimConfig) int {
	if cfg.Mode != PressureReclaimModeActive {
		return 0
	}
	st, sample, ok := r.beginSustainedReclaim()
	if !ok {
		if st.Sustained {
			r.logPressureHold(st.Hold, st, sample, cfg)
		}
		return 0
	}
	hibernated := false
	defer func() { r.endSustainedReclaim(hibernated) }()

	for _, candidate := range r.pressureReclaimActionCandidates() {
		if candidate.own == nil || candidate.public.Protected || !authorizeLifecycleRoute(ctx, guard, candidate.own.UserID, candidate.own.DesktopID) {
			continue
		}
		log.Printf("vmctl: pressure reclaim decision=reclaim signal=sustained streak=%d/%d mem_available_mib=%d floor_mib=%d some_avg10=%.2f some_avg60=%.2f state_dir_pressure=%v vm=%s computer=%s class=%s idle_s=%d",
			st.Streak, st.RequiredSamples, sample.MemoryAvailableBytes>>20, sample.MemoryFloorBytes>>20,
			sample.MemorySomeAvg10, sample.MemorySomeAvg60, sample.StateDirPressure,
			candidate.own.VMID, candidate.own.ComputerID, candidate.public.WarmnessClass, candidate.public.IdleSeconds)
		r.mu.Lock()
		r.pressureWin.streak = 0
		r.mu.Unlock()
		if err := r.hibernateVMForDesktopWithReason(candidate.own.UserID, candidate.own.DesktopID, "pressure"); err != nil {
			if errors.Is(err, errHibernateGuestBusy) {
				log.Printf("vmctl: pressure reclaim of VM %s skipped: the guest is working", candidate.own.VMID)
			} else {
				log.Printf("vmctl: pressure reclaim of VM %s failed: %v", candidate.own.VMID, err)
			}
			return 0
		}
		hibernated = true
		return 1
	}
	r.logPressureHold("no_eligible_computer", st, sample, cfg)
	return 0
}

// logPressureHold logs a sustained shortage that did not reclaim, once per
// reason change or minute, so the journal explains it without flooding.
func (r *OwnershipRegistry) logPressureHold(reason string, st SustainedPressureState, sample HostPressureSample, cfg PressureReclaimConfig) {
	now := r.now()
	r.mu.Lock()
	w := &r.pressureWin
	if w.lastHoldLog == reason && now.Sub(w.lastHoldLogAt) < time.Minute {
		r.mu.Unlock()
		return
	}
	w.lastHoldLog, w.lastHoldLogAt = reason, now
	r.mu.Unlock()
	log.Printf("vmctl: pressure reclaim decision=hold reason=%s signal=sustained streak=%d/%d mem_available_mib=%d floor_mib=%d some_avg10=%.2f some_avg60=%.2f hibernates_in_flight=%d cooldown_remaining_s=%d",
		reason, st.Streak, cfg.SustainedSamples, sample.MemoryAvailableBytes>>20, sample.MemoryFloorBytes>>20,
		sample.MemorySomeAvg10, sample.MemorySomeAvg60, st.HibernatesInFlight, st.CooldownRemainingSeconds)
}

// pressureMonitorTick samples host pressure and VM memory once and, under
// the sustained signal in active mode, runs the gated reclaim. It never
// starts, wakes or resumes a computer.
func (r *OwnershipRegistry) pressureMonitorTick(ctx context.Context, guard ComputerVersionRouteGuard) {
	r.mu.RLock()
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	sampler := r.pressureSampler
	r.mu.RUnlock()
	if cfg.Mode == PressureReclaimModeOff {
		return
	}
	if sampler == nil {
		sampler = sampleHostPressure
	}
	r.recordPressureSample(sampler(cfg))
	r.sampleVMMemory()
	if cfg.Mode == PressureReclaimModeActive && cfg.Signal == PressureReclaimSignalSustained {
		if n := r.ReclaimPressureVMs(ctx, guard); n > 0 {
			log.Printf("vmctl: pressure reclaim hibernated %d VM(s)", n)
		}
	}
}

// StartPressureMonitor runs the pressure monitor every SampleInterval. It
// is off when pressure reclaim is off.
func (r *OwnershipRegistry) StartPressureMonitor(ctx context.Context, guard ComputerVersionRouteGuard) {
	r.mu.RLock()
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	r.mu.RUnlock()
	if cfg.Mode == PressureReclaimModeOff {
		return
	}
	log.Printf("vmctl: pressure monitor every %s signal=%s samples=%d avg60>%.2f cooldown=%s", cfg.SampleInterval, cfg.Signal, cfg.SustainedSamples, cfg.MaxMemorySomeAvg60, cfg.Cooldown)
	go func() {
		ticker := time.NewTicker(cfg.SampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.pressureMonitorTick(ctx, guard)
			}
		}
	}()
}
