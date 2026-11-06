// Package actgate is the act gate and the circuit breaker of the Bayesian
// trust system (modular architecture plan MA7.8, trust draft 5 items 1 and 2,
// BT4).
//
// The act gate: a material act (impact class I2 and up) whose action class is
// not autonomous for the acting desk goes to a gate instead of executing.
// Autonomy is the trust draft's rule 4.3,
//
//	autonomous(s, k)  iff  LCB_05(s, k) >= tau(impact(k))
//
// read from the derived trust view (a snapshot the runtime pushes in,
// Observe), with no open breaker and with at least one record of the desk in
// the class. Thin evidence buys nothing: no view, a stale view, no record, or
// a bound below tau is gated. I4 is never autonomous for a third-party desk.
//
// The circuit breaker: a fidelity event (E3) or a platform event (E7) drops
// the class to gated at once, before the posterior is recomputed (Trip), and
// ends the desk's leases in the class through the lease policy's hook
// (lease.Service.RevokeOnFidelity). The drop holds until a trust view shows
// the event (a row of the grant with LastFidelityAt at or after the trip);
// from then the view's own breaker flag governs, read on every row of the
// act's grant whatever its impact, and autonomy returns only through new
// positive evidence (trust.Row.Breaker).
//
// The gate decides; it never reads the store and never writes. Where a gated
// act goes (the owner tap) and how a trip is recorded on the tape are the
// runtime's (internal/agentcore/act_gate.go).
package actgate

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TrustMaxAge is how old a trust snapshot may be and still be read; an older
// one is no view, and no view is gated.
const TrustMaxAge = 15 * time.Minute

// Decisions.
const (
	// NotMaterial: the act's class is below I2; the gate does not apply.
	NotMaterial = "not_material"
	// Autonomous: the desk's record in the class clears tau.
	Autonomous = "autonomous"
	// Gated: the act goes to the gate instead of executing.
	Gated = "gated"
)

// Wild is the class of a trip that covers every class of the desk (a forged
// frame, a platform event with no class).
var Wild = trust.ActionClass{Grant: "*", Impact: types.ImpactI4}

// IsWild reports whether c covers every class.
func IsWild(c trust.ActionClass) bool { return c.Grant == "" || c.Grant == "*" }

// Material reports whether an act of class c is material (I2 and up).
func Material(c trust.ActionClass) bool {
	r, ok := c.Impact.Rank()
	return ok && r >= 2
}

// Verdict is the gate's answer for one act, kept with the gated act on the
// tape so the owner sees why it waits.
type Verdict struct {
	Decision string            `json:"decision"`
	Holder   string            `json:"holder"`
	Class    trust.ActionClass `json:"class"`
	// LCB and N are the weakest lower bound and the fewest items over the
	// desk's rows in the class; Tau is the class's threshold.
	LCB     float64   `json:"lcb,omitempty"`
	N       int       `json:"n,omitempty"`
	Tau     float64   `json:"tau,omitempty"`
	Breaker bool      `json:"breaker,omitempty"`
	AsOf    time.Time `json:"as_of,omitempty"`
	Reason  string    `json:"reason"`
}

// Revoker is the lease policy's breaker hook (lease.Service.RevokeOnFidelity).
type Revoker interface {
	RevokeOnFidelity(ctx context.Context, desk string, class trust.ActionClass, at time.Time) int
}

type snapshot struct {
	at   time.Time
	rows []trust.Row
}

type markKey struct {
	holder string
	class  trust.ActionClass
}

// Gate holds the latest trust snapshot per owner and the breaker's drops.
type Gate struct {
	mu      sync.Mutex
	now     func() time.Time
	snaps   map[string]*snapshot
	marks   map[markKey]time.Time
	revoker Revoker
}

// New returns a gate whose trips call r (nil: no lease service).
func New(r Revoker) *Gate {
	return &Gate{now: time.Now, snaps: map[string]*snapshot{}, marks: map[markKey]time.Time{}, revoker: r}
}

// SetClock replaces the clock (tests).
func (g *Gate) SetClock(now func() time.Time) { g.now = now }

// Canon is the name a desk's drops are kept under: the desk without an app
// prefix or an instance suffix ("app:notes:x" and "notes" are "notes").
func Canon(name string) string {
	name = strings.TrimPrefix(strings.TrimSpace(name), "app:")
	head, _, _ := strings.Cut(name, ":")
	return head
}

func holderMatch(holder string, ids []string) bool {
	h := Canon(holder)
	for _, id := range ids {
		if Canon(id) == h && h != "" {
			return true
		}
	}
	return false
}

// Observe takes a fresh trust view of ownerID's records as the gate's
// snapshot, and settles the drops the view now shows: a drop ends once a row
// of the holder in its class (any class for a wild drop) carries a fidelity
// event at or after it, since that row's breaker flag then governs.
func (g *Gate) Observe(ownerID string, rows []trust.Row) {
	cp := append([]trust.Row(nil), rows...)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.snaps[ownerID] = &snapshot{at: g.now(), rows: cp}
	for k, at := range g.marks {
		for _, r := range cp {
			if r.Subject.Kind != trust.SubjectAgent || Canon(r.Subject.ID) != k.holder {
				continue
			}
			if (IsWild(k.class) || r.Class.Grant == k.class.Grant) && !r.LastFidelityAt.Before(at) {
				delete(g.marks, k)
				break
			}
		}
	}
}

// Trip is the circuit breaker: the desk's class (every class for Wild) drops
// to gated at once, and its leases in the class end through the lease
// policy's hook. It returns how many leases ended.
func (g *Gate) Trip(ctx context.Context, desk string, class trust.ActionClass, at time.Time) int {
	h := Canon(desk)
	if h == "" {
		return 0
	}
	if IsWild(class) {
		class = Wild
	}
	g.mu.Lock()
	k := markKey{holder: h, class: class}
	if prev, ok := g.marks[k]; !ok || at.After(prev) {
		g.marks[k] = at
	}
	g.mu.Unlock()
	n := 0
	if g.revoker != nil {
		n = g.revoker.RevokeOnFidelity(ctx, desk, class, at)
	}
	log.Printf("act gate: the circuit breaker dropped desk %s in %s to gated at %s; %d lease(s) revoked", desk, class, at.UTC().Format(time.RFC3339Nano), n)
	return n
}

// Dropped reports whether the breaker holds a drop for the desk in class.
func (g *Gate) Dropped(desk string, class trust.ActionClass) (time.Time, bool) {
	h := Canon(desk)
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, at := range g.marks {
		if k.holder == h && (IsWild(k.class) || k.class.Grant == class.Grant) {
			return at, true
		}
	}
	return time.Time{}, false
}

// Decide answers one act of class by the desk going by the names holders
// (its manifest name, its agent id), on model, for ownerID. thirdParty marks
// a desk whose origin is not first-party: I4 is never autonomous for it.
func (g *Gate) Decide(ownerID string, holders []string, model string, class trust.ActionClass, thirdParty bool) Verdict {
	name := ""
	for _, h := range holders {
		if strings.TrimSpace(h) != "" {
			name = strings.TrimSpace(h)
			break
		}
	}
	v := Verdict{Holder: name, Class: class}
	gated := func(format string, args ...any) Verdict {
		v.Decision = Gated
		v.Reason = fmt.Sprintf(format, args...)
		return v
	}
	if !Material(class) {
		v.Decision = NotMaterial
		v.Reason = fmt.Sprintf("%s is below I2", class)
		return v
	}
	tau, ok := trust.Tau(class.Impact)
	if !ok {
		return gated("%s has no autonomy threshold", class)
	}
	v.Tau = tau
	if thirdParty && class.Impact == types.ImpactI4 {
		return gated("I4 is never autonomous for a desk that is not first-party")
	}
	for _, h := range holders {
		if at, dropped := g.Dropped(h, class); dropped {
			v.Breaker = true
			return gated("the circuit breaker dropped %s at %s, and no trust view shows the event yet", class, at.UTC().Format(time.RFC3339))
		}
	}
	g.mu.Lock()
	snap := g.snaps[ownerID]
	g.mu.Unlock()
	if snap == nil {
		return gated("no trust view has been read")
	}
	v.AsOf = snap.at
	age := g.now().Sub(snap.at)
	if age < 0 || age > TrustMaxAge {
		return gated("the trust view is %s old, outside [0, %s]", age.Round(time.Second), TrustMaxAge)
	}
	ids := aliases(holders, model)
	lcb, n, agent := math.Inf(1), math.MaxInt, false
	// The breaker reads every row of the grant, whatever its impact: fidelity
	// evidence is filed at the grant's table class, and an act may sit in a
	// class its sender raised.
	for _, r := range snap.rows {
		if r.Class.Grant == class.Grant && ids[r.Subject] && r.Breaker {
			v.Breaker = true
			return gated("the breaker is open in %s (a fidelity or platform event at %s with no positive evidence since)", r.Class, r.LastFidelityAt.UTC().Format(time.RFC3339))
		}
	}
	for _, r := range snap.rows {
		if r.Class != class || !ids[r.Subject] {
			continue
		}
		agent = agent || r.Subject.Kind == trust.SubjectAgent
		if math.IsNaN(r.LCB) || r.LCB < 0 || r.LCB > 1 {
			return gated("the lower bound in %s is not a probability (%v)", class, r.LCB)
		}
		if r.LCB < lcb {
			lcb = r.LCB
		}
		if r.N < n {
			n = r.N
		}
	}
	if !agent {
		return gated("the desk has no record in %s", class)
	}
	v.LCB, v.N = lcb, n
	if lcb < tau {
		return gated("lower bound %.3f over %d items is below tau %.2f for %s", lcb, n, tau, class.Impact)
	}
	v.Decision = Autonomous
	v.Reason = fmt.Sprintf("lower bound %.3f over %d items is at or above tau %.2f for %s", lcb, n, tau, class.Impact)
	return v
}

// aliases are the subjects a desk's record may be filed under: each name, its
// app-less form and its canonical desk, and desk@model for each.
func aliases(holders []string, model string) map[trust.Subject]bool {
	out := map[trust.Subject]bool{}
	for _, h := range holders {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		for _, id := range []string{h, strings.TrimPrefix(h, "app:"), Canon(h), "app:" + Canon(h)} {
			if id == "" || id == "app:" {
				continue
			}
			out[trust.Subject{Kind: trust.SubjectAgent, ID: id}] = true
			if model != "" {
				out[trust.Subject{Kind: trust.SubjectAgentModel, ID: id + "@" + model}] = true
			}
		}
	}
	return out
}

// TripClasses are the classes a trip is recorded in as fidelity evidence for
// the trust view: the class itself, or for a wild trip every impact-table
// grant of class I1 and up (the classes an act can fall in; I0 carries no
// evidence).
func TripClasses(class trust.ActionClass) []trust.ActionClass {
	if !IsWild(class) {
		return []trust.ActionClass{class}
	}
	var out []trust.ActionClass
	for _, g := range trust.ImpactTable().Grants {
		if g.Family {
			continue
		}
		if r, ok := g.Impact.Rank(); ok && r >= 1 {
			out = append(out, trust.ActionClass{Grant: g.Grant, Impact: g.Impact})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Grant < out[j].Grant })
	return out
}

// Counters, keyed "decision|impact|enforcement" (enforced for a desk the gate
// holds, observed for a first-party desk it counts and lets pass), read by
// GET /api/runtime/observability.
var (
	countMu sync.Mutex
	counts  = map[string]uint64{}
	logged  = map[string]bool{}
)

// Note counts one decision.
func Note(v Verdict, enforcement string) {
	key := fmt.Sprintf("%s|%s|%s", v.Decision, v.Class.Impact, enforcement)
	countMu.Lock()
	counts[key]++
	first := !logged[key]
	logged[key] = true
	countMu.Unlock()
	if first && v.Decision == Gated {
		log.Printf("act gate: %s (%s): desk %s in %s: %s", key, enforcement, v.Holder, v.Class, v.Reason)
	}
}

// NoteTrip counts one breaker trip, keyed "trip|cause".
func NoteTrip(cause string) {
	countMu.Lock()
	counts["trip|"+cause]++
	countMu.Unlock()
}

// Counts returns the counters.
func Counts() map[string]uint64 {
	countMu.Lock()
	defer countMu.Unlock()
	out := make(map[string]uint64, len(counts))
	for k, v := range counts {
		out[k] = v
	}
	return out
}

// ResetCounts clears the counters (tests).
func ResetCounts() {
	countMu.Lock()
	counts = map[string]uint64{}
	logged = map[string]bool{}
	countMu.Unlock()
}
