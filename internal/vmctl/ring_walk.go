package vmctl

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Ring-ordered deploys and automatic code rollback
// (docs/design/ring-ordered-deploys-2026-11-03.md; progressive delivery
// slices S5 and S6, owner rulings 2026-11-03: "rollout is the deploy", no
// switches, the paper refreshes automatically and last).
//
// A deploy that moves the guest release starts a walk. Computers outside the
// open ring are pinned to the last release that finished a walk (the manager
// asks ReleasePin at every cold boot and in every release rule), the deploy
// refresh refuses them, and each ring opens only when the ring before it is
// clean: its time floor passed and its members were seen healthy on the
// release. A guard breach pins every computer back, enrolls a planned return
// refresh for the computers that ran the release, and stops the walk. There is
// no switch: the walk runs on every deploy that moves the guest release.

type ringID string

const (
	ringR0 ringID = "R0" // proof: d@e
	ringR1 ringID = "R1" // test: class test
	ringR2 ringID = "R2" // the owner's computers, by id
	ringR3 ringID = "R3" // every other computer, a new one included
	ringR4 ringID = "R4" // public_platform
	ringR5 ringID = "R5" // the paper, last
)

var ringOrder = []ringID{ringR0, ringR1, ringR2, ringR3, ringR4, ringR5}

// Ring membership is by computer id, except R1 (design 3.1, review 340 F1).
// Changing a membership is a commit.
const (
	ringProofComputer = "computer-59c902c5d46caecc0a180a8557164e72" // d@e
	ringPaperComputer = "computer-1355b83bced1613eb048ee28d66688a8" // the paper
)

var ringOwnerComputers = map[string]bool{
	"computer-03335285269bdba4f94377e56879f9e6": true, // the owner's computer
}

// ringForComputer is the ring of a computer. class is the live class
// (WarmnessClassForOwnership), used only for R1 (test) and R4 (platform).
func ringForComputer(computerID string, class WarmnessClass) ringID {
	switch {
	case computerID == ringProofComputer:
		return ringR0
	case computerID == ringPaperComputer:
		return ringR5
	case ringOwnerComputers[computerID]:
		return ringR2
	case class == WarmnessClassTest:
		return ringR1
	case class == WarmnessClassPublicPlatform:
		return ringR4
	}
	return ringR3
}

func ringIndex(r ringID) int {
	for i, x := range ringOrder {
		if x == r {
			return i
		}
	}
	return len(ringOrder)
}

// ringSoak is a ring's soak (design 3.2): a time floor and an exposure floor.
type ringSoak struct {
	Floor       time.Duration
	MinExposure int
}

var ringSoaks = map[ringID]ringSoak{
	ringR0: {30 * time.Minute, 1},
	ringR1: {2 * time.Hour, 3},
	ringR2: {4 * time.Hour, 1},
	ringR3: {4 * time.Hour, 2},
	ringR4: {2 * time.Hour, 1},
	ringR5: {1 * time.Hour, 1},
}

// Guards (design 5.3 and 9.2; each has a known-bad fixture in
// testdata/ring_walk).
const (
	ringGuardReadiness = "readiness"
	ringGuardWedge     = "appender_wedge"
	ringGuardBoot      = "boot_success"
	ringGuardCrash     = "crash_restart"

	// ringReadinessInitialDelay: a guest is unreachable for minutes after a
	// boot (the paper, F1); readiness counts only after this long on the
	// release.
	ringReadinessInitialDelay = 5 * time.Minute
	ringReadinessStrikes      = 3
	ringWedgeStrikes          = 2

	// ringReenrollGap: an eligible member still off the release, with no
	// pending entry and no failed refresh to this target (its entry was
	// skipped while it was held or not running), is enrolled again at most
	// this often. A failed refresh is never retried by the walk.
	ringReenrollGap = 10 * time.Minute
)

const (
	ringWalkWalking    = "walking"
	ringWalkComplete   = "complete"
	ringWalkRolledBack = "rolled_back"
	ringWalkSuperseded = "superseded"

	ringOutcomeSoaking           = "soaking"
	ringOutcomeClean             = "clean"
	ringOutcomeBreached          = "breached"
	ringOutcomeSkippedEmpty      = "skipped_empty"
	ringOutcomeSkippedNoExposure = "skipped_no_exposure"
	ringOutcomeSuperseded        = "superseded"

	ringEventBaseline         = "baseline"
	ringEventWalkStarted      = "walk_started"
	ringEventWalkSuperseded   = "walk_superseded"
	ringEventRingOpened       = "ring_opened"
	ringEventRingClosed       = "ring_closed"
	ringEventRolledBack       = "rolled_back"
	ringEventRollbackComplete = "rollback_complete"
	ringEventWalkComplete     = "walk_complete"

	ringHistorySchema = "choir.ring_walk_history.v1"
	ringStateSchema   = 1
)

// ringReleasePair is the single-image release and the layered one (the
// layered ring computers launch the second; layered_ring.go).
type ringReleasePair struct {
	Single        string `json:"single"`
	SingleCommit  string `json:"single_commit,omitempty"`
	Layered       string `json:"layered,omitempty"`
	LayeredCommit string `json:"layered_commit,omitempty"`
}

// sameIDs compares release ids only: a commit read late never restarts a walk.
func (p ringReleasePair) sameIDs(q ringReleasePair) bool {
	return p.Single == q.Single && p.Layered == q.Layered
}

func (p ringReleasePair) has(release string) bool {
	return release != "" && (release == p.Single || release == p.Layered)
}

// releaseFor is the release of the pair a computer launches: the layered one
// for a layered ring computer when the pair has it.
func (p ringReleasePair) releaseFor(layered bool) string {
	if layered && p.Layered != "" {
		return p.Layered
	}
	return p.Single
}

func (p ringReleasePair) commitFor(layered bool) string {
	if layered && p.Layered != "" {
		return p.LayeredCommit
	}
	return p.SingleCommit
}

// ringHealth is one /health sample of a guest.
type ringHealth struct {
	Answered bool   `json:"answered"`
	Status   int    `json:"status,omitempty"`
	Wedged   bool   `json:"wedged,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// parseRingHealth reads a guest /health answer: readiness is status 200; the
// appender wedge is projection_repair_required in the body
// (internal/autoputer/health_wedge.go).
func parseRingHealth(status int, body []byte) ringHealth {
	h := ringHealth{Answered: true, Status: status}
	var parsed map[string]json.RawMessage
	if json.Unmarshal(body, &parsed) == nil {
		if raw, ok := parsed["projection_repair_required"]; ok && string(raw) != "null" {
			h.Wedged = true
			var w struct {
				FirstError string `json:"first_error"`
			}
			_ = json.Unmarshal(raw, &w)
			h.Detail = "projection_repair_required: " + w.FirstError
		}
	}
	return h
}

// ringMemberObs is what a tick knows of one computer.
type ringMemberObs struct {
	ComputerID string
	UserID     string
	DesktopID  string
	VMID       string
	Ring       ringID
	// Active: running, the only state a refresh reaches.
	Active    bool
	Held      bool
	Divergent bool
	// Layered: a layered ring computer (it launches the layered release).
	Layered bool
	// Release is the guest release its running VM was launched from.
	Release string
	// Health is nil when the computer was not sampled.
	Health  *ringHealth
	Crashes []CrashRecord
	// Pending: the pending set holds an entry for this computer.
	// FailedTarget: the target of its last failed deploy refresh, if any.
	Pending      bool
	FailedTarget string
}

type ringMember struct {
	ComputerID    string    `json:"computer_id"`
	Enrolled      bool      `json:"enrolled,omitempty"`
	WakeRequested bool      `json:"wake_requested,omitempty"`
	EnrolledAt    time.Time `json:"enrolled_at,omitempty"`
	LastOffTarget time.Time `json:"last_off_target,omitempty"`
	FirstOnTarget time.Time `json:"first_on_target,omitempty"`
	ExposedAt     time.Time `json:"exposed_at,omitempty"`
	Samples       int       `json:"samples"`
	Healthy       int       `json:"healthy"`
	ReadyStrikes  int       `json:"ready_strikes,omitempty"`
	WedgeStrikes  int       `json:"wedge_strikes,omitempty"`
	LastStatus    int       `json:"last_status,omitempty"`
	LastDetail    string    `json:"last_detail,omitempty"`
	LastRelease   string    `json:"last_release,omitempty"`
}

type ringBreach struct {
	ComputerID string    `json:"computer_id"`
	Guard      string    `json:"guard"`
	Detail     string    `json:"detail,omitempty"`
	At         time.Time `json:"at"`
}

type ringProgress struct {
	Ring     ringID                 `json:"ring"`
	OpenedAt time.Time              `json:"opened_at"`
	ClosedAt time.Time              `json:"closed_at,omitempty"`
	Outcome  string                 `json:"outcome"`
	Waiting  string                 `json:"waiting,omitempty"`
	Members  map[string]*ringMember `json:"members"`
	Breach   *ringBreach            `json:"breach,omitempty"`
}

type ringWalk struct {
	ID        string          `json:"id"`
	StartedAt time.Time       `json:"started_at"`
	From      ringReleasePair `json:"from"`
	To        ringReleasePair `json:"to"`
	// PredictionIDs are the precommitment ids the release carries, for the
	// settlement the ring walk feeds (the calibration bridge,
	// docs/design/calibration-seed-2026-11-03.md). Empty until releases carry
	// them; settlement is not built here.
	PredictionIDs []string        `json:"prediction_ids"`
	Status        string          `json:"status"`
	Current       int             `json:"current"`
	Rings         []*ringProgress `json:"rings"`
	EndedAt       time.Time       `json:"ended_at,omitempty"`
	// Returning lists the computers a rollback enrolled for a return; the
	// rollback is complete once none of them runs the target.
	Returning       []string `json:"returning,omitempty"`
	ReturnCompleted bool     `json:"return_completed,omitempty"`
}

type ringWalkState struct {
	Schema int `json:"schema"`
	// Baseline is the last release that finished a walk (at first start, the
	// release vmctl found current). Pins name it.
	Baseline ringReleasePair `json:"baseline"`
	Walk     *ringWalk       `json:"walk,omitempty"`
}

// ringEvent is one history line.
type ringEvent struct {
	Schema        string             `json:"schema"`
	At            time.Time          `json:"at"`
	WalkID        string             `json:"walk_id,omitempty"`
	Event         string             `json:"event"`
	Ring          ringID             `json:"ring,omitempty"`
	Release       *ringEventRelease  `json:"release,omitempty"`
	Computers     []ringEventMember  `json:"computers,omitempty"`
	Soak          *ringEventSoak     `json:"soak,omitempty"`
	Outcome       string             `json:"outcome,omitempty"`
	Breach        *ringBreach        `json:"breach,omitempty"`
	PredictionIDs []string           `json:"prediction_ids,omitempty"`
	Detail        string             `json:"detail,omitempty"`
	Actions       []ringActionRecord `json:"actions,omitempty"`
}

type ringEventRelease struct {
	From ringReleasePair `json:"from"`
	To   ringReleasePair `json:"to"`
}

type ringEventMember struct {
	ComputerID    string    `json:"computer_id"`
	Release       string    `json:"release,omitempty"`
	FirstOnTarget time.Time `json:"first_on_target,omitempty"`
	ExposedAt     time.Time `json:"exposed_at,omitempty"`
	Samples       int       `json:"samples"`
	Healthy       int       `json:"healthy"`
	LastStatus    int       `json:"last_status,omitempty"`
	LastDetail    string    `json:"last_detail,omitempty"`
}

type ringEventSoak struct {
	Floor       string    `json:"floor"`
	MinExposure int       `json:"min_exposure"`
	OpenedAt    time.Time `json:"opened_at"`
	ClosedAt    time.Time `json:"closed_at,omitempty"`
	Exposed     int       `json:"exposed"`
	Required    int       `json:"required"`
}

const (
	ringActionEnroll = "enroll"
	ringActionForget = "forget"
	ringActionWake   = "wake"
)

// ringAction is a pending-set change a step asks for.
type ringAction struct {
	Kind       string
	ComputerID string
	UserID     string
	DesktopID  string
	VMID       string
	Target     string
	Reason     string
}

type ringActionRecord struct {
	Kind       string `json:"kind"`
	ComputerID string `json:"computer_id"`
	Target     string `json:"target,omitempty"`
}

func (w *ringWalk) release() *ringEventRelease {
	return &ringEventRelease{From: w.From, To: w.To}
}

func (w *ringWalk) event(at time.Time, kind string) ringEvent {
	return ringEvent{Schema: ringHistorySchema, At: at, WalkID: w.ID, Event: kind, Release: w.release(), PredictionIDs: w.predictionSlot()}
}

func (w *ringWalk) predictionSlot() []string {
	if w.PredictionIDs == nil {
		return []string{}
	}
	return w.PredictionIDs
}

func (w *ringWalk) open() *ringProgress {
	if w.Current < len(w.Rings) {
		return w.Rings[w.Current]
	}
	return nil
}

// ringOpened reports whether ring r has opened in this walk.
func (w *ringWalk) ringOpened(r ringID) bool {
	return ringIndex(r) < len(w.Rings)
}

// observeRelease reads the current release pair. A pair with no single
// release (legacy, unresolved) changes nothing.
func (s *ringWalkState) observeRelease(cur ringReleasePair, now time.Time) []ringEvent {
	if cur.Single == "" {
		return nil
	}
	s.Schema = ringStateSchema
	if s.Baseline.Single == "" {
		s.Baseline = cur
		return []ringEvent{{Schema: ringHistorySchema, At: now, Event: ringEventBaseline, Release: &ringEventRelease{To: cur}, Detail: "first observation: the current release is the baseline; no walk"}}
	}
	var events []ringEvent
	if w := s.Walk; w != nil && (w.Status == ringWalkWalking || w.Status == ringWalkRolledBack) {
		if w.To.sameIDs(cur) {
			w.To = cur
			return nil
		}
		walking := w.Status == ringWalkWalking
		w.Status, w.EndedAt = ringWalkSuperseded, now
		if p := w.open(); p != nil && p.ClosedAt.IsZero() && walking {
			p.Outcome, p.ClosedAt = ringOutcomeSuperseded, now
		}
		ev := w.event(now, ringEventWalkSuperseded)
		ev.Detail = "release " + cur.Single + " replaced the walk's target"
		events = append(events, ev)
		s.Walk = nil
	} else if w != nil && w.To.sameIDs(cur) {
		return nil
	}
	if cur.sameIDs(s.Baseline) {
		s.Walk = nil
		return events
	}
	w := &ringWalk{
		ID:            fmt.Sprintf("walk-%s-%d", shortRelease(cur.Single), now.Unix()),
		StartedAt:     now,
		From:          s.Baseline,
		To:            cur,
		PredictionIDs: []string{},
		Status:        ringWalkWalking,
	}
	s.Walk = w
	events = append(events, w.event(now, ringEventWalkStarted))
	events = append(events, w.openRing(now))
	return events
}

func shortRelease(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (w *ringWalk) openRing(now time.Time) ringEvent {
	r := ringOrder[len(w.Rings)]
	w.Rings = append(w.Rings, &ringProgress{Ring: r, OpenedAt: now, Outcome: ringOutcomeSoaking, Members: map[string]*ringMember{}})
	w.Current = len(w.Rings) - 1
	soak := ringSoaks[r]
	ev := w.event(now, ringEventRingOpened)
	ev.Ring = r
	ev.Soak = &ringEventSoak{Floor: soak.Floor.String(), MinExposure: soak.MinExposure, OpenedAt: now}
	return ev
}

// pinFor is the release pair a cold boot of the computer launches from now;
// empty means the current release.
func (s *ringWalkState) pinFor(computerID string, ring ringID) (single, layered string) {
	w := s.Walk
	if w == nil {
		return "", ""
	}
	switch w.Status {
	case ringWalkWalking:
		if w.ringOpened(ring) {
			return "", ""
		}
		return w.From.Single, w.From.Layered
	case ringWalkRolledBack:
		return w.From.Single, w.From.Layered
	}
	return "", ""
}

// gate answers "" when a deploy refresh of an active computer in ring to
// target may run, else the reason it is refused.
func (s *ringWalkState) gate(ring ringID, target string, layered bool) string {
	w := s.Walk
	if w == nil {
		return ""
	}
	fromCommit := w.From.commitFor(layered)
	switch w.Status {
	case ringWalkWalking:
		if w.ringOpened(ring) || (target != "" && target == fromCommit) {
			return ""
		}
		open := w.open()
		if open == nil {
			return "ring " + string(ring) + " waits: ring walk " + w.ID + " has no open ring"
		}
		return fmt.Sprintf("ring %s waits: ring walk %s is at %s (opened %s); it refreshes this computer when its ring opens", ring, w.ID, open.Ring, open.OpenedAt.UTC().Format(time.RFC3339))
	case ringWalkRolledBack:
		if target != "" && target == fromCommit {
			return ""
		}
		open := w.open()
		if open == nil {
			return "ring walk " + w.ID + " rolled release " + w.To.Single + " back"
		}
		return fmt.Sprintf("ring walk %s rolled release %s back at %s; this computer stays on %s until the next release", w.ID, w.To.Single, open.Ring, w.From.Single)
	}
	return ""
}

// step evaluates one tick. obs lists every computer vmctl knows.
func (s *ringWalkState) step(now time.Time, obs []ringMemberObs) ([]ringEvent, []ringAction) {
	w := s.Walk
	if w == nil {
		return nil, nil
	}
	switch w.Status {
	case ringWalkRolledBack:
		return w.stepReturn(now, obs), nil
	case ringWalkWalking:
	default:
		return nil, nil
	}
	// No computer observed at all (a registry not yet loaded, a failed read):
	// nothing is known, so nothing advances.
	if len(obs) == 0 {
		return nil, nil
	}
	var events []ringEvent
	var actions []ringAction
	for w.Status == ringWalkWalking {
		p := w.open()
		if p == nil {
			return events, actions
		}
		var members []ringMemberObs
		for _, o := range obs {
			if o.Ring == p.Ring {
				members = append(members, o)
			}
		}
		for _, o := range members {
			m := p.Members[o.ComputerID]
			if m == nil {
				m = &ringMember{ComputerID: o.ComputerID}
				p.Members[o.ComputerID] = m
			}
			m.LastRelease = o.Release
			onTarget := o.Active && o.Release != "" && o.Release == w.To.releaseFor(o.Layered)
			// R0 opens with the deploy, and CI refreshes it; the walk enrolls an
			// R0 computer the deploy did not reach only after the grace.
			graced := p.Ring == ringR0 && now.Sub(p.OpenedAt) < ringR0EnrollGrace
			target := w.To.commitFor(o.Layered)
			if !onTarget {
				m.LastOffTarget = now
			}
			reenroll := m.Enrolled && !o.Pending && o.FailedTarget != target && now.Sub(m.EnrolledAt) >= ringReenrollGap
			if o.Active && !o.Held && !o.Divergent && (!m.Enrolled || reenroll) && !o.Pending && o.FailedTarget != target && !graced && !onTarget {
				m.Enrolled, m.EnrolledAt = true, now
				actions = append(actions, ringAction{Kind: ringActionEnroll, ComputerID: o.ComputerID, UserID: o.UserID, DesktopID: o.DesktopID, VMID: o.VMID, Target: target, Reason: "ring walk " + w.ID + " opened " + string(p.Ring)})
			}
			// The proof ring needs d@e seen on the release; d@e is usually
			// hibernated between uses, so the walk wakes it once (a wake is
			// what any request does: a snapshot resume or a cold boot onto
			// the open ring's release, never a restart of running work).
			if p.Ring == ringR0 && !graced && !o.Active && !o.Held && !m.WakeRequested && m.ExposedAt.IsZero() {
				m.WakeRequested = true
				actions = append(actions, ringAction{Kind: ringActionWake, ComputerID: o.ComputerID, UserID: o.UserID, DesktopID: o.DesktopID, VMID: o.VMID, Reason: "ring walk " + w.ID + ": R0 needs d@e on the release"})
			}
			if onTarget && m.FirstOnTarget.IsZero() {
				m.FirstOnTarget = now
			}
			if breach := m.evaluate(now, o, onTarget, p.OpenedAt, p.Ring); breach != nil {
				events = append(events, w.rollback(now, p, breach, obs, &actions)...)
				return events, actions
			}
		}
		if len(members) == 0 {
			if p.Ring == ringR0 {
				// The proof ring never passes unseen, even when d@e is not
				// in the registry (a transient absence, or an id drift).
				p.Waiting = "no proof computer observed: R0 holds until d@e runs the release and is seen healthy"
				return events, actions
			}
			events = append(events, w.closeRing(now, p, ringOutcomeSkippedEmpty, "no computer in this ring"))
			ev, done := w.advance(now, s)
			events = append(events, ev...)
			if done {
				return events, actions
			}
			continue
		}
		soak := ringSoaks[p.Ring]
		if now.Sub(p.OpenedAt) < soak.Floor {
			p.Waiting = "time floor until " + p.OpenedAt.Add(soak.Floor).UTC().Format(time.RFC3339)
			return events, actions
		}
		// Exposure: at least min(floor count, eligible) members seen healthy
		// on the release, where eligible is every active, non-held,
		// non-divergent member plus any member seen healthy that has since
		// gone idle. A busy member that has not refreshed yet does not hold
		// the ring: its pending entry still refreshes it once its ring is open.
		required, exposed, eligible := 0, 0, 0
		var missing []string
		for _, o := range members {
			m := p.Members[o.ComputerID]
			isRequired := o.Active && !o.Held && !o.Divergent
			if isRequired {
				required++
			}
			if !m.ExposedAt.IsZero() {
				exposed++
			}
			if isRequired || !m.ExposedAt.IsZero() {
				eligible++
			}
			if isRequired && m.ExposedAt.IsZero() {
				missing = append(missing, o.ComputerID)
			}
		}
		if required == 0 && exposed > 0 {
			// Every member was seen healthy on the release and has since
			// gone idle (hibernated): that is exposure.
			events = append(events, w.closeRing(now, p, ringOutcomeClean, ""))
		} else if required == 0 {
			if p.Ring == ringR0 {
				p.Waiting = "no active member: the proof ring holds until d@e runs the release and is seen healthy"
				return events, actions
			}
			events = append(events, w.closeRing(now, p, ringOutcomeSkippedNoExposure, "every member was down for the whole time floor"))
		} else {
			need := min(soak.MinExposure, eligible)
			if exposed < need {
				sort.Strings(missing)
				p.Waiting = fmt.Sprintf("exposure: %d of %d seen healthy on the release; waiting for %s", exposed, need, strings.Join(missing, ","))
				return events, actions
			}
			events = append(events, w.closeRing(now, p, ringOutcomeClean, ""))
		}
		ev, done := w.advance(now, s)
		events = append(events, ev...)
		if done {
			return events, actions
		}
	}
	return events, actions
}

// evaluate applies the guards to one member's sample. A breach is returned.
func (m *ringMember) evaluate(now time.Time, o ringMemberObs, onTarget bool, openedAt time.Time, ring ringID) *ringBreach {
	// A held or divergent member is not walked; its crashes are not the
	// release's. The boot guard counts only a member the walk or the deploy
	// asked onto the release (enrolled, seen on it, or R0, which CI
	// refreshes).
	if o.Held || o.Divergent {
		m.ReadyStrikes, m.WedgeStrikes = 0, 0
		return nil
	}
	bootScoped := m.Enrolled || !m.FirstOnTarget.IsZero() || ring == ringR0
	for _, c := range o.Crashes {
		at, err := time.Parse(time.RFC3339Nano, c.At)
		if err != nil {
			continue
		}
		if c.Kind == crashKindBootFailed && bootScoped && !at.Before(openedAt) {
			return &ringBreach{ComputerID: o.ComputerID, Guard: ringGuardBoot, Detail: c.Cause, At: now}
		}
		// A crash after the last tick that saw the member off the release
		// (or after the ring opened) belongs to the release: a crash in the
		// seconds after the refresh boot is not missed.
		since := openedAt
		if m.LastOffTarget.After(since) {
			since = m.LastOffTarget
		}
		if c.Kind != crashKindBootFailed && !m.FirstOnTarget.IsZero() && !at.Before(since) {
			return &ringBreach{ComputerID: o.ComputerID, Guard: ringGuardCrash, Detail: c.Kind + ": " + c.Cause, At: now}
		}
	}
	if !onTarget || o.Health == nil {
		m.ReadyStrikes, m.WedgeStrikes = 0, 0
		return nil
	}
	h := o.Health
	m.Samples++
	m.LastStatus, m.LastDetail = h.Status, h.Detail
	ready := h.Answered && h.Status == 200
	if ready {
		m.ReadyStrikes = 0
	} else if now.Sub(m.FirstOnTarget) >= ringReadinessInitialDelay {
		m.ReadyStrikes++
	}
	if h.Wedged {
		m.WedgeStrikes++
	} else {
		m.WedgeStrikes = 0
	}
	if ready && !h.Wedged {
		m.Healthy++
		if m.ExposedAt.IsZero() {
			m.ExposedAt = now
		}
	}
	switch {
	case m.WedgeStrikes >= ringWedgeStrikes:
		return &ringBreach{ComputerID: o.ComputerID, Guard: ringGuardWedge, Detail: h.Detail, At: now}
	case m.ReadyStrikes >= ringReadinessStrikes:
		detail := h.Detail
		if h.Answered {
			detail = fmt.Sprintf("/health answered %d", h.Status)
		}
		return &ringBreach{ComputerID: o.ComputerID, Guard: ringGuardReadiness, Detail: detail, At: now}
	}
	return nil
}

func (w *ringWalk) closeRing(now time.Time, p *ringProgress, outcome, detail string) ringEvent {
	p.Outcome, p.ClosedAt, p.Waiting = outcome, now, ""
	soak := ringSoaks[p.Ring]
	ev := w.event(now, ringEventRingClosed)
	ev.Ring, ev.Outcome, ev.Breach, ev.Detail = p.Ring, outcome, p.Breach, detail
	ev.Soak = &ringEventSoak{Floor: soak.Floor.String(), MinExposure: soak.MinExposure, OpenedAt: p.OpenedAt, ClosedAt: now}
	ids := make([]string, 0, len(p.Members))
	for id := range p.Members {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		m := p.Members[id]
		if !m.ExposedAt.IsZero() {
			ev.Soak.Exposed++
		}
		ev.Computers = append(ev.Computers, ringEventMember{ComputerID: id, Release: m.LastRelease, FirstOnTarget: m.FirstOnTarget, ExposedAt: m.ExposedAt, Samples: m.Samples, Healthy: m.Healthy, LastStatus: m.LastStatus, LastDetail: m.LastDetail})
	}
	ev.Soak.Required = len(ids)
	return ev
}

// advance opens the next ring, or completes the walk after the last one.
func (w *ringWalk) advance(now time.Time, s *ringWalkState) ([]ringEvent, bool) {
	if len(w.Rings) >= len(ringOrder) {
		w.Status, w.EndedAt = ringWalkComplete, now
		s.Baseline = w.To
		return []ringEvent{w.event(now, ringEventWalkComplete)}, true
	}
	return []ringEvent{w.openRing(now)}, false
}

// rollback stops the walk at p: every computer is pinned to from (pinFor),
// the computers of the opened rings that run the target are enrolled for a
// planned return refresh, and forward entries of the others are forgotten.
func (w *ringWalk) rollback(now time.Time, p *ringProgress, breach *ringBreach, obs []ringMemberObs, actions *[]ringAction) []ringEvent {
	p.Breach = breach
	events := []ringEvent{w.closeRing(now, p, ringOutcomeBreached, "")}
	w.Status, w.EndedAt = ringWalkRolledBack, now
	ev := w.event(now, ringEventRolledBack)
	ev.Ring, ev.Breach = p.Ring, breach
	for _, o := range obs {
		if !w.ringOpened(o.Ring) {
			continue
		}
		if o.Active && o.Release != "" && o.Release == w.To.releaseFor(o.Layered) && o.Release != w.From.releaseFor(o.Layered) {
			a := ringAction{Kind: ringActionEnroll, ComputerID: o.ComputerID, UserID: o.UserID, DesktopID: o.DesktopID, VMID: o.VMID, Target: w.From.commitFor(o.Layered), Reason: "ring walk " + w.ID + " rolled back at " + string(p.Ring)}
			*actions = append(*actions, a)
			w.Returning = append(w.Returning, o.ComputerID)
			ev.Actions = append(ev.Actions, ringActionRecord{Kind: a.Kind, ComputerID: a.ComputerID, Target: a.Target})
			continue
		}
		a := ringAction{Kind: ringActionForget, ComputerID: o.ComputerID, UserID: o.UserID, DesktopID: o.DesktopID}
		*actions = append(*actions, a)
	}
	ev.Detail = fmt.Sprintf("every computer pinned to %s; %d return refresh(es) enrolled", w.From.Single, len(w.Returning))
	return append(events, ev)
}

// stepReturn watches a rolled-back walk's returns; it evaluates no guard and
// enrolls nothing, so a rollback cannot loop.
func (w *ringWalk) stepReturn(now time.Time, obs []ringMemberObs) []ringEvent {
	if w.ReturnCompleted {
		return nil
	}
	byID := map[string]ringMemberObs{}
	for _, o := range obs {
		byID[o.ComputerID] = o
	}
	for _, id := range w.Returning {
		o, ok := byID[id]
		if ok && o.Active && o.Release != "" && o.Release == w.To.releaseFor(o.Layered) && o.Release != w.From.releaseFor(o.Layered) {
			return nil
		}
	}
	w.ReturnCompleted = true
	ev := w.event(now, ringEventRollbackComplete)
	if p := w.open(); p != nil {
		ev.Ring = p.Ring
	}
	for _, id := range w.Returning {
		o := byID[id]
		ev.Computers = append(ev.Computers, ringEventMember{ComputerID: id, Release: o.Release})
	}
	ev.Detail = "no returning computer runs the rolled-back release"
	return []ringEvent{ev}
}

// ringWalkRunner holds the state, its file and the history file. Its mutex is
// never held while calling the registry, the manager or a guest.
type ringWalkRunner struct {
	mu          sync.Mutex
	path        string
	historyPath string
	state       ringWalkState
	// rings caches each known computer's ring, refreshed every tick, so the
	// manager's pin question needs no registry lock.
	rings map[string]ringID
}

func newRingWalkRunner() *ringWalkRunner {
	return &ringWalkRunner{rings: map[string]ringID{}}
}

// ringHistoryMaxBytes bounds the history file; past it the file is rotated
// once to .1.
const ringHistoryMaxBytes = 4 << 20

func (r *ringWalkRunner) load(path, historyPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.path, r.historyPath = strings.TrimSpace(path), strings.TrimSpace(historyPath)
	if r.path == "" {
		log.Printf("vmctl: ring walk: no state path; the walk is not durable (a vmctl restart forgets it and its pins)")
		return nil
	}
	raw, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ring walk: read %s: %w", r.path, err)
	}
	var st ringWalkState
	if err := json.Unmarshal(raw, &st); err != nil {
		// A torn or empty file: the last good copy keeps the walk and its
		// pins (saveLocked keeps it beside the file).
		bak, berr := os.ReadFile(r.path + ".bak")
		if berr != nil || json.Unmarshal(bak, &st) != nil {
			return fmt.Errorf("ring walk: decode %s (and no readable %s.bak): %w", r.path, r.path, err)
		}
		log.Printf("vmctl: ring walk: %s did not decode (%v); loaded the last good copy %s.bak", r.path, err, r.path)
	}
	r.state = st
	return nil
}

func (r *ringWalkRunner) saveLocked() {
	if r.path == "" {
		return
	}
	raw, err := json.MarshalIndent(r.state, "", "  ")
	if err == nil {
		// Keep the previous good file as .bak (load reads it when the file
		// does not decode).
		if prev, perr := os.ReadFile(r.path); perr == nil && json.Valid(prev) && len(prev) > 0 {
			_ = writeFileDurably(r.path+".bak", prev)
		}
		err = writeFileDurably(r.path, raw)
	}
	if err != nil {
		log.Printf("vmctl: ring walk: save %s: %v", r.path, err)
	}
}

func (r *ringWalkRunner) appendHistoryLocked(events []ringEvent) {
	for _, ev := range events {
		log.Printf("vmctl: ring walk %s: %s %s %s %s", ev.WalkID, ev.Event, ev.Ring, ev.Outcome, ev.Detail)
	}
	if r.historyPath == "" || len(events) == 0 {
		return
	}
	if info, err := os.Stat(r.historyPath); err == nil && info.Size() > ringHistoryMaxBytes {
		_ = os.Rename(r.historyPath, r.historyPath+".1")
	}
	f, err := os.OpenFile(r.historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("vmctl: ring walk: history %s: %v", r.historyPath, err)
		return
	}
	defer f.Close()
	for _, ev := range events {
		line, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		_, _ = f.Write(append(line, '\n'))
	}
	_ = f.Sync()
}

// observe records a release observation, persisting any change.
func (r *ringWalkRunner) observe(cur ringReleasePair, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if events := r.state.observeRelease(cur, now); len(events) > 0 {
		r.saveLocked()
		r.appendHistoryLocked(events)
	}
}

// ringOfLocked is a computer's ring and whether vmctl has observed it. The
// rings by id need no observation.
func (r *ringWalkRunner) ringOfLocked(computerID string) (ringID, bool) {
	if ring, ok := r.rings[computerID]; ok {
		return ring, true
	}
	ring := ringForComputer(computerID, "")
	return ring, ring != ringR3
}

// pin is the manager's ReleasePin.
func (r *ringWalkRunner) pin(computerID string) (single, layered string) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return "", ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ring, known := r.ringOfLocked(computerID)
	if !known {
		// Not yet observed (created since the last tick): its class is
		// unknown, so it is held on from while anything walks, never let
		// ahead of its ring.
		if w := r.state.Walk; w != nil && (w.Status == ringWalkWalking || w.Status == ringWalkRolledBack) {
			return w.From.Single, w.From.Layered
		}
		return "", ""
	}
	return r.state.pinFor(computerID, ring)
}

// pinnedReleases are the releases pins can name now (retention keeps them).
func (r *ringWalkRunner) pinnedReleases() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	w := r.state.Walk
	if w == nil || (w.Status != ringWalkWalking && w.Status != ringWalkRolledBack) {
		return nil
	}
	var out []string
	for _, id := range []string{w.From.Single, w.From.Layered} {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func (r *ringWalkRunner) gate(computerID string, class WarmnessClass, target string, layered bool) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ring := ringForComputer(computerID, class)
	return r.state.gate(ring, target, layered)
}

// ringCommit is the commit of the release a computer of ring is held on, ""
// when it is not held.
func (r *ringWalkRunner) ringCommit(computerID string, class WarmnessClass, layered bool) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	single, _ := r.state.pinFor(computerID, ringForComputer(computerID, class))
	if single == "" {
		return ""
	}
	return r.state.Walk.From.commitFor(layered)
}

// stepWith runs one tick over obs, persisting and recording what changed.
func (r *ringWalkRunner) stepWith(now time.Time, obs []ringMemberObs) []ringAction {
	r.mu.Lock()
	defer r.mu.Unlock()
	rings := make(map[string]ringID, len(obs))
	for _, o := range obs {
		rings[o.ComputerID] = o.Ring
	}
	r.rings = rings
	before, _ := json.Marshal(r.state)
	events, actions := r.state.step(now, obs)
	if after, _ := json.Marshal(r.state); string(before) != string(after) {
		r.saveLocked()
	}
	r.appendHistoryLocked(events)
	return actions
}

// sampleTargets lists the computers whose health a tick reads: active members
// of the open ring running the target. Nil when nothing is walking.
func (r *ringWalkRunner) sampleTargets() (ring ringID, to ringReleasePair, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w := r.state.Walk
	if w == nil || w.Status != ringWalkWalking {
		return "", ringReleasePair{}, false
	}
	p := w.open()
	if p == nil {
		return "", ringReleasePair{}, false
	}
	return p.Ring, w.To, true
}

// ringWalkStatus is GET /internal/vmctl/ring-walk.
type ringWalkStatus struct {
	State   ringWalkState     `json:"state"`
	History []json.RawMessage `json:"history"`
}

func (r *ringWalkRunner) status(tail int) ringWalkStatus {
	// A deep copy taken under the lock: the walk's maps are written by every
	// tick, and the caller marshals after the lock is released.
	r.mu.Lock()
	raw, _ := json.Marshal(r.state)
	path := r.historyPath
	r.mu.Unlock()
	st := ringWalkStatus{History: []json.RawMessage{}}
	_ = json.Unmarshal(raw, &st.State)
	if path == "" {
		return st
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	for _, l := range lines {
		if strings.TrimSpace(l) != "" && json.Valid([]byte(l)) {
			st.History = append(st.History, json.RawMessage(l))
		}
	}
	return st
}

// summary is the walk as CI sees it, nil when nothing walks.
func (r *ringWalkRunner) summary() *deployRefreshRingWalk {
	r.mu.Lock()
	defer r.mu.Unlock()
	w := r.state.Walk
	if w == nil || (w.Status != ringWalkWalking && w.Status != ringWalkRolledBack) {
		return nil
	}
	out := &deployRefreshRingWalk{ID: w.ID, Status: w.Status, ProofComputer: ringProofComputer}
	if p := w.open(); p != nil {
		out.OpenRing = string(p.Ring)
	}
	return out
}
