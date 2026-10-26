package vmctl

// D6 S2: the idle trigger for the guest's page cache return act
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.2, 4.5,
// 4.6; docs/problems/density-station-d-failure-modes-2026-10-26.md D6).
//
// A guest frees clean page cache only under its own memory pressure, so a
// computer that grew its cache once keeps it for ever and its host RSS never
// falls. The guest can return its cold cache (internal/guestmem, route
// POST /internal/runtime/memory/cache-return); free page reporting, already
// on in every guest, hands the freed pages to the host. This file is the
// host half of the idle trigger: after the idle sweep's hibernate decisions
// of a tick it asks selected, idle, unprotected computers to do it once per
// idle period.
//
// Everything here is a measurement or a request. The guest's report is the
// guest's claim: it is recorded, never used for a decision. Only host
// measurements (the Firecracker process's RSS, the host's MemAvailable) are
// read after the act, and only to record them. The act is not work in the
// CLAUDE.md "Restarts End Work" sense: it opens no obligation, and nothing
// here is persisted, so a vmctl restart resumes and replays nothing.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/rssfall"
)

// The switch. Only the exact words enable; anything else is off.
const (
	GuestCacheReturnOff     = "off"
	GuestCacheReturnObserve = "observe"
	GuestCacheReturnActive  = "active"
)

const (
	// cacheReturnMaxInFlight bounds acts host-wide; one per VM besides.
	cacheReturnMaxInFlight = 2
	// cacheReturnHTTPTimeout bounds one call to a guest.
	cacheReturnHTTPTimeout = 10 * time.Second
	// cacheReturnRearm: a computer the sweep never hibernates is acted on
	// again this long after its last act, whatever its activity.
	cacheReturnRearm = 30 * time.Minute
	// cacheReturnRingSize is the size of admission.recent.
	cacheReturnRingSize = 20

	cacheReturnSettleCap = rssfall.DefaultCap

	cacheReturnPath = "/internal/runtime/memory/cache-return"
)

// GuestCacheReturnConfig is the switch and its selector, triggers and
// bounds. The code default is off, nobody selected.
type GuestCacheReturnConfig struct {
	Mode string
	// Only selects computers: VM ids, computer ids, class:test,
	// class:primary, class:premium, or *. Empty selects nobody.
	Only []string
	// Triggers is a subset of idle, hibernate, pressure.
	Triggers []string
	// Scope is the guest cgroup the act writes: daemon or root.
	Scope string
	// Idle is how long a computer is idle before the idle trigger acts
	// (1m to 9m). PressureIdle and PressureBudget are the pressure
	// trigger's (S4).
	Idle           time.Duration
	PressureIdle   time.Duration
	PressureBudget time.Duration
}

// DefaultGuestCacheReturnConfig is the code default: off.
func DefaultGuestCacheReturnConfig() GuestCacheReturnConfig {
	return GuestCacheReturnConfig{
		Mode:           GuestCacheReturnOff,
		Triggers:       []string{"idle"},
		Scope:          "daemon",
		Idle:           3 * time.Minute,
		PressureIdle:   time.Minute,
		PressureBudget: 12 * time.Second,
	}
}

// NormalizeGuestCacheReturnMode turns the exact words observe and active
// into themselves and everything else, a typo or a capital included, into
// off.
func NormalizeGuestCacheReturnMode(mode string) string {
	switch mode {
	case GuestCacheReturnObserve, GuestCacheReturnActive:
		return mode
	}
	return GuestCacheReturnOff
}

// TriggerOn reports whether a trigger is selected.
func (c GuestCacheReturnConfig) TriggerOn(trigger string) bool {
	for _, t := range c.Triggers {
		if t == trigger {
			return true
		}
	}
	return false
}

// normalized fills zero durations and a missing scope with the defaults.
func (c GuestCacheReturnConfig) normalized() GuestCacheReturnConfig {
	def := DefaultGuestCacheReturnConfig()
	c.Mode = NormalizeGuestCacheReturnMode(c.Mode)
	if c.Scope != "daemon" && c.Scope != "root" {
		c.Scope = def.Scope
	}
	if c.Idle <= 0 {
		c.Idle = def.Idle
	}
	if c.PressureBudget <= 0 {
		c.PressureBudget = def.PressureBudget
	}
	return c
}

// GuestCacheReturnValues are the raw settings (the environment's strings).
type GuestCacheReturnValues struct {
	Mode           string
	Only           string
	Triggers       string
	Scope          string
	Idle           string
	PressureIdle   string
	PressureBudget string
}

// GuestCacheReturnConfigFromValues parses and clamps. Malformed values are
// the defaults; an unset trigger list is the default (idle), a set list with
// no valid word selects no trigger.
func GuestCacheReturnConfigFromValues(v GuestCacheReturnValues) GuestCacheReturnConfig {
	cfg := DefaultGuestCacheReturnConfig()
	cfg.Mode = NormalizeGuestCacheReturnMode(v.Mode)
	for _, tok := range strings.Split(v.Only, ",") {
		if tok = strings.TrimSpace(tok); tok != "" {
			cfg.Only = append(cfg.Only, tok)
		}
	}
	if strings.TrimSpace(v.Triggers) != "" {
		cfg.Triggers = nil
		for _, tok := range strings.Split(v.Triggers, ",") {
			switch tok = strings.TrimSpace(tok); tok {
			case "idle", "hibernate", "pressure":
				cfg.Triggers = append(cfg.Triggers, tok)
			}
		}
	}
	if v.Scope == "root" {
		cfg.Scope = "root"
	}
	clamp := func(raw string, dst *time.Duration, lo, hi time.Duration) {
		d, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			return
		}
		*dst = min(max(d, lo), hi)
	}
	clamp(v.Idle, &cfg.Idle, time.Minute, 9*time.Minute)
	clamp(v.PressureIdle, &cfg.PressureIdle, 0, 9*time.Minute)
	clamp(v.PressureBudget, &cfg.PressureBudget, 2*time.Second, 30*time.Second)
	return cfg
}

// SetGuestCacheReturnConfig configures the trigger.
func (r *OwnershipRegistry) SetGuestCacheReturnConfig(cfg GuestCacheReturnConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheReturnCfg = cfg.normalized()
}

func (r *OwnershipRegistry) guestCacheReturnConfig() GuestCacheReturnConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg := r.cacheReturnCfg
	if cfg.Mode == "" && cfg.Scope == "" {
		return DefaultGuestCacheReturnConfig()
	}
	return cfg.normalized()
}

// --- the candidate rule ---------------------------------------------------

// cacheReturnCandidateInput is everything the candidate rule reads, so it
// is a pure function of its input.
type cacheReturnCandidateInput struct {
	Now    time.Time
	Config GuestCacheReturnConfig
	Policy WarmnessPolicyConfig
	// SweepHibernates: the idle sweep hibernates unprotected computers (an
	// idle timeout is set). Protected classes are never hibernated by it.
	SweepHibernates bool
	// Ownerships carry activity folded in (cloneWithActivityLocked).
	Ownerships []*VMOwnership
	// IdleFloor is the idle time a computer needs for this trigger.
	IdleFloor time.Duration
	// Limit stops the (probing) pass once this many candidates are found.
	Limit int

	Claimed     func(*VMOwnership) bool
	Uncaptured  func(vmID string) bool
	InFlight    func(vmID string) bool
	LastAct     func(vmID string) (time.Time, bool)
	Unsupported func(vmID string, epoch int64) bool
	// Busy is the guest busy probe: the last rule, asked only of computers
	// that passed every other one.
	Busy func(*VMOwnership) bool
}

// cacheReturnSelected reports whether the selector names the computer.
func cacheReturnSelected(own *VMOwnership, only []string, class WarmnessClass) bool {
	for _, tok := range only {
		switch {
		case tok == "*",
			tok == own.VMID,
			own.ComputerID != "" && tok == own.ComputerID,
			tok == "class:test" && class == WarmnessClassTest,
			tok == "class:primary" && class == WarmnessClassPrimary,
			tok == "class:premium" && class == WarmnessClassPremiumAlwaysOn:
			return true
		}
	}
	return false
}

// cacheReturnCandidates returns the computers to act on, longest idle first.
// A computer is a candidate when every rule holds: selected; not the paper
// (critical_protected), a public_platform computer or a held one; has a
// ComputerURL (defense in depth: the shared probe already answers busy for
// none); running and active under no lifecycle claim; idle at least
// IdleFloor; D4 has captured its settled RSS (no open or awaiting
// reservation); not in flight; not unsupported at this boot; not acted on
// since its last activity (a computer the sweep never hibernates: again
// after 30 minutes); and the guest probe answers idle.
func cacheReturnCandidates(in cacheReturnCandidateInput) []*VMOwnership {
	type ranked struct {
		own  *VMOwnership
		idle time.Duration
	}
	var pool []ranked
	for _, own := range in.Ownerships {
		if own == nil || own.State != VMStateActive || own.IsHeld() || isPlatformOwnership(own) {
			continue
		}
		class := warmnessClassForOwnership(own, in.Policy)
		if class == WarmnessClassCriticalProtected || class == WarmnessClassPublicPlatform {
			continue
		}
		if strings.TrimSpace(own.ComputerURL) == "" {
			continue
		}
		if !cacheReturnSelected(own, in.Config.Only, class) {
			continue
		}
		if own.LastActiveAt.IsZero() {
			continue
		}
		idle := in.Now.Sub(own.LastActiveAt)
		if idle < in.IdleFloor {
			continue
		}
		if in.Claimed != nil && in.Claimed(own) {
			continue
		}
		if in.InFlight != nil && in.InFlight(own.VMID) {
			continue
		}
		if in.Unsupported != nil && in.Unsupported(own.VMID, own.Epoch) {
			continue
		}
		if last, acted := lastActOf(in, own.VMID); acted && !last.Before(own.LastActiveAt) {
			// Acted in this idle period. A computer the sweep never
			// hibernates keeps its cache growing, so it is re-armed.
			neverHibernated := warmnessClassProtected(class) || !in.SweepHibernates
			if !neverHibernated || in.Now.Sub(last) < cacheReturnRearm {
				continue
			}
		}
		if in.Uncaptured != nil && in.Uncaptured(own.VMID) {
			continue
		}
		pool = append(pool, ranked{own, idle})
	}
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].idle != pool[j].idle {
			return pool[i].idle > pool[j].idle
		}
		return pool[i].own.VMID < pool[j].own.VMID
	})
	var out []*VMOwnership
	for _, c := range pool {
		if in.Limit > 0 && len(out) >= in.Limit {
			break
		}
		if in.Busy != nil && in.Busy(c.own) {
			continue
		}
		out = append(out, c.own)
	}
	return out
}

func lastActOf(in cacheReturnCandidateInput, vmID string) (time.Time, bool) {
	if in.LastAct == nil {
		return time.Time{}, false
	}
	return in.LastAct(vmID)
}

// --- state ----------------------------------------------------------------

// cacheReturnRSS is a Firecracker process's resident memory, in bytes.
type cacheReturnRSS struct {
	Total uint64
	Anon  uint64
	File  uint64
}

// CacheReturnRecord is one act, as health and the journal show it. The
// guest_* fields are the guest's claim; the rest are host measurements.
type CacheReturnRecord struct {
	At         string `json:"at"`
	VMID       string `json:"vm_id"`
	ComputerID string `json:"computer_id,omitempty"`
	Trigger    string `json:"trigger"`
	Mode       string `json:"mode"`
	Scope      string `json:"scope"`
	DryRun     bool   `json:"dry_run,omitempty"`
	// Outcome: the guest's word on a 200, else busy, in_progress,
	// not_fenced, unsupported, host_timeout, error, bad_report or http_NNN.
	Outcome string `json:"outcome"`
	Method  string `json:"method,omitempty"`

	GuestTargetMiB    uint64 `json:"guest_target_mib"`
	GuestReclaimedMiB uint64 `json:"guest_reclaimed_mib"`
	GuestSteps        int    `json:"guest_steps,omitempty"`
	GuestMS           int64  `json:"guest_ms,omitempty"`

	RSSBeforeMiB       uint64 `json:"rss_before_mib"`
	RSSAfterMiB        uint64 `json:"rss_after_mib"`
	RSSAnonBeforeMiB   uint64 `json:"rss_anon_before_mib"`
	RSSAnonAfterMiB    uint64 `json:"rss_anon_after_mib"`
	RSSFileBeforeMiB   uint64 `json:"rss_file_before_mib"`
	RSSFileAfterMiB    uint64 `json:"rss_file_after_mib"`
	HostAvailBeforeMiB uint64 `json:"host_avail_before_mib"`
	HostAvailAfterMiB  uint64 `json:"host_avail_after_mib"`
	SettleMS           int64  `json:"settle_ms"`
	SettleExit         string `json:"settle_exit"`
	MS                 int64  `json:"ms"`
}

// JournalLine is the one journal line per act.
func (r CacheReturnRecord) JournalLine() string {
	return fmt.Sprintf("guest cache return VM %s trigger=%s mode=%s scope=%s outcome=%s dry_run=%t "+
		"guest_target_mib=%d guest_reclaimed_mib=%d rss_before_mib=%d rss_after_mib=%d "+
		"rss_anon_before_mib=%d rss_anon_after_mib=%d rss_file_before_mib=%d rss_file_after_mib=%d "+
		"host_avail_before_mib=%d host_avail_after_mib=%d settle_ms=%d settle_exit=%s ms=%d",
		r.VMID, r.Trigger, r.Mode, r.Scope, r.Outcome, r.DryRun,
		r.GuestTargetMiB, r.GuestReclaimedMiB, r.RSSBeforeMiB, r.RSSAfterMiB,
		r.RSSAnonBeforeMiB, r.RSSAnonAfterMiB, r.RSSFileBeforeMiB, r.RSSFileAfterMiB,
		r.HostAvailBeforeMiB, r.HostAvailAfterMiB, r.SettleMS, r.SettleExit, r.MS)
}

// GuestCacheReturnSummary is the health view: the switch and the recent
// acts. Memory only; it never probes a guest.
type GuestCacheReturnSummary struct {
	Mode     string              `json:"mode"`
	Only     []string            `json:"only,omitempty"`
	Triggers []string            `json:"triggers,omitempty"`
	Scope    string              `json:"scope"`
	Recent   []CacheReturnRecord `json:"recent,omitempty"`
	// LastAt is each computer's last act (last_cache_return_at).
	LastAt map[string]string `json:"last_cache_return_at,omitempty"`
}

// guestCacheReturnState is the trigger's memory: nothing here is persisted.
type guestCacheReturnState struct {
	mu          sync.Mutex
	inflight    map[string]bool
	lastAct     map[string]time.Time
	unsupported map[string]int64
	ring        []CacheReturnRecord
	lastAt      map[string]string
	wg          sync.WaitGroup

	// Seams (tests). rss reads a VM's resident memory; avail the host's
	// MemAvailable; sleep pauses the settle wait; httpTimeout bounds a call.
	rss         func(vmID string) (cacheReturnRSS, bool)
	avail       func() uint64
	sleep       func(time.Duration)
	httpTimeout time.Duration
}

func (s *guestCacheReturnState) inFlightCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.inflight)
}

func (s *guestCacheReturnState) isInFlight(vmID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inflight[vmID]
}

// begin takes an in-flight slot for vmID, or reports that none is free.
func (s *guestCacheReturnState) begin(vmID string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight == nil {
		s.inflight = map[string]bool{}
	}
	if s.inflight[vmID] || len(s.inflight) >= cacheReturnMaxInFlight {
		return false
	}
	s.inflight[vmID] = true
	if s.lastAct == nil {
		s.lastAct = map[string]time.Time{}
	}
	// Every attempt counts, so a failing guest is not asked every pass.
	s.lastAct[vmID] = now
	return true
}

func (s *guestCacheReturnState) end(vmID string) {
	s.mu.Lock()
	delete(s.inflight, vmID)
	s.mu.Unlock()
}

func (s *guestCacheReturnState) markUnsupported(vmID string, epoch int64) {
	s.mu.Lock()
	if s.unsupported == nil {
		s.unsupported = map[string]int64{}
	}
	s.unsupported[vmID] = epoch
	s.mu.Unlock()
}

func (s *guestCacheReturnState) isUnsupported(vmID string, epoch int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.unsupported[vmID]
	return ok && e == epoch
}

func (s *guestCacheReturnState) lastActAt(vmID string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.lastAct[vmID]
	return t, ok
}

func (s *guestCacheReturnState) record(rec CacheReturnRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ring = append(s.ring, rec)
	if len(s.ring) > cacheReturnRingSize {
		s.ring = append([]CacheReturnRecord(nil), s.ring[len(s.ring)-cacheReturnRingSize:]...)
	}
	if rec.VMID != "" {
		if s.lastAt == nil {
			s.lastAt = map[string]string{}
		}
		s.lastAt[rec.VMID] = rec.At
	}
}

// GuestCacheReturnSummary reports the switch and the recent acts.
func (r *OwnershipRegistry) GuestCacheReturnSummary() GuestCacheReturnSummary {
	cfg := r.guestCacheReturnConfig()
	out := GuestCacheReturnSummary{
		Mode: cfg.Mode, Only: append([]string(nil), cfg.Only...), Triggers: append([]string(nil), cfg.Triggers...), Scope: cfg.Scope,
	}
	s := &r.cacheReturn
	s.mu.Lock()
	defer s.mu.Unlock()
	out.Recent = append([]CacheReturnRecord(nil), s.ring...)
	if len(s.lastAt) > 0 {
		out.LastAt = make(map[string]string, len(s.lastAt))
		for k, v := range s.lastAt {
			out.LastAt[k] = v
		}
	}
	return out
}

// waitGuestCacheReturn waits for the acts in flight, up to d (tests).
func (r *OwnershipRegistry) waitGuestCacheReturn(d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		r.cacheReturn.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// --- the hibernate option (S3) --------------------------------------------

// HibernateCacheReturn asks the manager to return the guest's cold page
// cache between the fence and the snapshot of one hibernate. It is a
// per-call option, never part of a VM's configuration or launch.
type HibernateCacheReturn struct {
	Scope  string
	DryRun bool
}

// hibernateCacheReturnManager is implemented by a manager whose snapshot
// hibernate can take the option.
type hibernateCacheReturnManager interface {
	HibernateVMWithCacheReturn(vmID string, opt HibernateCacheReturn) error
}

// hibernateCacheReturnForLocked is the option for hibernating own, or nil: the
// switch is on, the hibernate trigger is listed, the selector names the
// computer, and it is not the paper or a public platform computer. The
// caller holds r.mu.
func (r *OwnershipRegistry) hibernateCacheReturnForLocked(own *VMOwnership) *HibernateCacheReturn {
	cfg := r.cacheReturnCfg
	if cfg.Mode == "" && cfg.Scope == "" {
		cfg = DefaultGuestCacheReturnConfig()
	}
	cfg = cfg.normalized()
	if own == nil || cfg.Mode == GuestCacheReturnOff || len(cfg.Only) == 0 || !cfg.TriggerOn("hibernate") {
		return nil
	}
	if isPlatformOwnership(own) {
		return nil
	}
	class := warmnessClassForOwnership(own, r.warmnessPolicy)
	if class == WarmnessClassCriticalProtected || class == WarmnessClassPublicPlatform {
		return nil
	}
	if !cacheReturnSelected(own, cfg.Only, class) {
		return nil
	}
	return &HibernateCacheReturn{Scope: cfg.Scope, DryRun: cfg.Mode == GuestCacheReturnObserve}
}

// --- the pass -------------------------------------------------------------

// runGuestCacheReturnPass is the idle trigger: one pass of the idle sweep's
// tick, after its hibernate decisions. It starts at most two acts host-wide
// in the background and returns how many it started. Off, an empty selector
// and a trigger list without idle return before reading anything. It never
// holds r.mu across a probe or a call.
func (r *OwnershipRegistry) runGuestCacheReturnPass(ctx context.Context) int {
	cfg := r.guestCacheReturnConfig()
	if cfg.Mode == GuestCacheReturnOff || len(cfg.Only) == 0 || !cfg.TriggerOn("idle") {
		return 0
	}
	st := &r.cacheReturn
	free := cacheReturnMaxInFlight - st.inFlightCount()
	if free <= 0 {
		return 0
	}
	now := r.now()

	r.mu.RLock()
	policy := r.warmnessPolicy
	acfg := normalizeHostAdmissionConfig(r.hostAdmission)
	sweepHibernates := r.idleTimeout > 0
	owns := make([]*VMOwnership, 0, len(r.ownerships))
	claimed := map[string]bool{}
	for key, own := range r.ownerships {
		owns = append(owns, r.cloneWithActivityLocked(own))
		if _, held := r.lifecycle[key]; held {
			claimed[own.VMID] = true
		}
	}
	r.mu.RUnlock()

	led := &r.admissionLedger
	picks := cacheReturnCandidates(cacheReturnCandidateInput{
		Now: now, Config: cfg, Policy: policy, SweepHibernates: sweepHibernates,
		Ownerships: owns, IdleFloor: cfg.Idle, Limit: free,
		Claimed: func(own *VMOwnership) bool { return claimed[own.VMID] },
		// D4 first: an act before the settled RSS is captured would make
		// D4 under-charge every later wake. An open reservation, or an
		// ended window still awaiting the sampler's capture, holds it off.
		Uncaptured: func(vmID string) bool {
			led.mu.Lock()
			defer led.mu.Unlock()
			led.outstandingLocked(now, acfg)
			if _, waiting := led.expired[vmID]; waiting {
				return true
			}
			for _, res := range led.reservations {
				if res.vmID == vmID {
					return true
				}
			}
			return false
		},
		InFlight:    st.isInFlight,
		LastAct:     st.lastActAt,
		Unsupported: st.isUnsupported,
		Busy:        r.guestBusy(),
	})
	started := 0
	for _, own := range picks {
		if !st.begin(own.VMID, now) {
			continue
		}
		started++
		st.wg.Add(1)
		go r.guestCacheReturnAct(ctx, own, "idle", cfg)
	}
	return started
}

// guestCacheReturnAct runs one act and records it.
func (r *OwnershipRegistry) guestCacheReturnAct(ctx context.Context, own *VMOwnership, trigger string, cfg GuestCacheReturnConfig) {
	st := &r.cacheReturn
	defer st.wg.Done()
	defer st.end(own.VMID)

	started := time.Now()
	dry := cfg.Mode == GuestCacheReturnObserve
	rec := CacheReturnRecord{
		At: r.now().UTC().Format(time.RFC3339), VMID: own.VMID, ComputerID: own.ComputerID,
		Trigger: trigger, Mode: "idle", Scope: cfg.Scope, DryRun: dry,
	}
	read := r.cacheReturnRSSReader(own.VMID)
	before, haveBefore := read()
	if haveBefore {
		rec.setRSSBefore(before)
	}
	rec.HostAvailBeforeMiB = st.hostAvail() >> 20

	status, report, err := st.call(ctx, own, cfg.Scope, dry)
	switch {
	case err != nil:
		rec.Outcome = err.Error()
	case status == http.StatusNotFound:
		// An old guest without the route: not again until it boots.
		st.markUnsupported(own.VMID, own.Epoch)
		rec.Outcome = "unsupported"
	case status == http.StatusConflict:
		rec.Outcome = report.Outcome
		if rec.Outcome == "" {
			rec.Outcome = "conflict"
		}
	case status != http.StatusOK:
		rec.Outcome = "http_" + strconv.Itoa(status)
	case report.Outcome == "":
		rec.Outcome = "bad_report"
	default:
		rec.Outcome, rec.Method = report.Outcome, report.Method
		rec.GuestTargetMiB, rec.GuestReclaimedMiB = report.TargetBytes>>20, report.ReclaimedBytes>>20
		rec.GuestSteps, rec.GuestMS = report.Steps, report.MS
	}

	// What the host sees afterwards. The guest's reclaimed bytes only size
	// how long to wait for the fall (free page reporting runs on a 2 second
	// delay); they decide nothing.
	rec.SettleExit = "none"
	after, haveAfter := before, haveBefore
	if haveBefore && status == http.StatusOK && !dry && (report.Outcome == "returned" || report.Outcome == "partial") {
		settle := waitRSSFall(read, st.sleepFn(), before, report.ReclaimedBytes, cacheReturnSettleCap)
		after, haveAfter = settle.RSS, settle.Exit != "gone"
		rec.SettleMS, rec.SettleExit = settle.MS, settle.Exit
	} else if got, ok := read(); ok {
		after, haveAfter = got, true
	}
	if haveAfter {
		rec.setRSSAfter(after)
	}
	rec.HostAvailAfterMiB = st.hostAvail() >> 20
	rec.MS = time.Since(started).Milliseconds()
	st.record(rec)
	log.Print("vmctl: " + rec.JournalLine())
}

func (rec *CacheReturnRecord) setRSSBefore(v cacheReturnRSS) {
	rec.RSSBeforeMiB, rec.RSSAnonBeforeMiB, rec.RSSFileBeforeMiB = v.Total>>20, v.Anon>>20, v.File>>20
}

func (rec *CacheReturnRecord) setRSSAfter(v cacheReturnRSS) {
	rec.RSSAfterMiB, rec.RSSAnonAfterMiB, rec.RSSFileAfterMiB = v.Total>>20, v.Anon>>20, v.File>>20
}

// cacheReturnGuestReport is the part of the guest's report vmctl records.
// It is a claim: nothing reads it to decide.
type cacheReturnGuestReport struct {
	Outcome        string `json:"outcome"`
	Method         string `json:"method"`
	TargetBytes    uint64 `json:"target_bytes"`
	ReclaimedBytes uint64 `json:"reclaimed_bytes"`
	Steps          int    `json:"steps"`
	MS             int64  `json:"ms"`
}

// call posts one act to the guest. The request carries no host credential:
// hostauth.MarkInternal sends none to a guest address. The error is the
// recorded outcome word.
func (s *guestCacheReturnState) call(ctx context.Context, own *VMOwnership, scope string, dryRun bool) (int, cacheReturnGuestReport, error) {
	var report cacheReturnGuestReport
	body, _ := json.Marshal(map[string]any{"mode": "idle", "scope": scope, "dry_run": dryRun})
	timeout := s.httpTimeout
	if timeout <= 0 {
		timeout = cacheReturnHTTPTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	base := strings.TrimRight(strings.TrimSpace(own.ComputerURL), "/")
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, base+cacheReturnPath, bytes.NewReader(body))
	if err != nil {
		return 0, report, errors.New("error")
	}
	req.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(cctx.Err(), context.DeadlineExceeded) {
			return 0, report, errors.New("host_timeout")
		}
		return 0, report, errors.New("error")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict {
		_ = json.Unmarshal(raw, &report)
	}
	return resp.StatusCode, report, nil
}

func (s *guestCacheReturnState) hostAvail() uint64 {
	if s.avail != nil {
		return s.avail()
	}
	if _, available, err := readMemInfo(); err == nil {
		return available
	}
	return 0
}

func (s *guestCacheReturnState) sleepFn() func(time.Duration) {
	if s.sleep != nil {
		return s.sleep
	}
	return time.Sleep
}

// cacheReturnRSSReader reads the VM's Firecracker process for the length of
// one act: the pids are resolved once, then each poll reads their status.
func (r *OwnershipRegistry) cacheReturnRSSReader(vmID string) func() (cacheReturnRSS, bool) {
	if r.cacheReturn.rss != nil {
		rss := r.cacheReturn.rss
		return func() (cacheReturnRSS, bool) { return rss(vmID) }
	}
	r.mu.RLock()
	mgr := r.vmManager
	r.mu.RUnlock()
	pc, ok := mgr.(vmProcessController)
	if !ok || pc == nil {
		return func() (cacheReturnRSS, bool) { return cacheReturnRSS{}, false }
	}
	pids := pc.LiveFirecrackerVMs()[vmID]
	return func() (cacheReturnRSS, bool) {
		var total cacheReturnRSS
		found := false
		for _, pid := range pids {
			raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
			if err != nil {
				continue
			}
			if v, ok := residentFromStatus(string(raw)); ok {
				total.Total += v.Total
				total.Anon += v.Anon
				total.File += v.File
				found = true
			}
		}
		return total, found
	}
}

// residentFromStatus reads VmRSS, RssAnon and RssFile (kB) from a
// /proc/<pid>/status. Total needs VmRSS; the split fields may be absent
// (an old kernel) and read as zero.
func residentFromStatus(status string) (cacheReturnRSS, bool) {
	var v cacheReturnRSS
	haveTotal := false
	for _, line := range strings.Split(status, "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		kib, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch name {
		case "VmRSS":
			v.Total, haveTotal = kib<<10, true
		case "RssAnon":
			v.Anon = kib << 10
		case "RssFile":
			v.File = kib << 10
		}
	}
	return v, haveTotal
}

// --- the settle wait ------------------------------------------------------

type cacheReturnSettle struct {
	RSS  cacheReturnRSS
	MS   int64
	Exit string // fell, cap, no_wait, gone
}

// waitRSSFall waits for the host to see a guest's return (internal/rssfall
// holds the rule, shared with the hibernate path in internal/vmmanager). It
// keeps the full reading (anon, file) the rule sees only the total of.
func waitRSSFall(read func() (cacheReturnRSS, bool), sleep func(time.Duration), rss0 cacheReturnRSS, reclaimed uint64, limit time.Duration) cacheReturnSettle {
	last := rss0
	res := rssfall.Wait(func() (uint64, bool) {
		cur, ok := read()
		if ok {
			last = cur
		}
		return cur.Total, ok
	}, sleep, rss0.Total, reclaimed, limit)
	out := cacheReturnSettle{RSS: last, MS: res.MS, Exit: res.Exit}
	if res.Exit == rssfall.ExitGone {
		out.RSS = rss0
	}
	return out
}
