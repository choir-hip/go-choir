package lease

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The lease policy (modular architecture plan MA7.4, trust draft 5.6).
//
// Inside the ceiling, a lease's duration comes from the holder's posterior
// lower credible bound in the lease's action class. Below the bound a lease is
// per-task: it ends with its run, and a crash closes it. At or above it, with
// enough evidence and no open breaker, a lease is durable: it lives a stated
// period that grows with the bound, the holder's later runs attach to it when
// they ask (the custodian does not decide again), and at its expiry it renews
// without asking while the record still holds. A fidelity event revokes the
// class's leases at once (RevokeOnFidelity, the hook the act gate's circuit
// breaker MA7.8 calls; ObserveTrust, the breaker read from the trust view).
//
// Scope never widens: a durable lease covers exactly what was asked, which
// decide already held inside the manifest ceiling, and the ceiling is
// re-checked at every use and every renewal. Only a desk's own ask is shaped
// by the policy: the first-party grants the platform mints (texture.search,
// texture.brief, the capsule store copy) and the custodian-ruled by-ref grants
// (mail.read) stay per-task, as their custodians' rules are written for one
// task. Every durable lease ends at boot like every other lease.
//
// The policy reads a snapshot of the trust view the runtime pushes in
// (ObserveTrust), never the store: the reader gate asks the service from
// inside store reads. A missing, stale or malformed posterior is no posterior,
// and no posterior is per-task.

// Policy numbers. They are the starting policy of plan section 18 item 1 (what
// lower bound earns a durable lease, how long durable is); the replay of MA7.9
// re-sets them.
const (
	// DurableFloor is the lowest lower bound a durable lease needs, whatever
	// the class's own tau (I0 and I1 never gate an act, so their tau says
	// nothing about how long a lease may run). The bar is the I2 tau.
	DurableFloor = 0.80
	// DurableMinN is the fewest evidence items a durable lease needs in the
	// class.
	DurableMinN = 20
	// DurableMin and DurableMax bound a durable lease's life per issue or
	// renewal: DurableMin at the floor, DurableMax at a bound of 0.99 and up.
	DurableMin = 2 * time.Hour
	DurableMax = 24 * time.Hour
	// TrustMaxAge is how old a trust snapshot may be and still be read.
	TrustMaxAge = 15 * time.Minute
)

// The action classes of a lease's scope (the impact table's grant rows).
const (
	ClassReadKind     = "desk.read.kind"
	ClassReadCaptures = "desk.captures.read"
)

// ReasonFidelity ends the leases the circuit breaker revokes.
const ReasonFidelity = "revoked by the circuit breaker: a fidelity event in the lease's class"

// Basis records why a lease got the life it has: the class the policy read,
// the weakest lower bound it found, and the reason in a sentence.
type Basis struct {
	Durable bool                `json:"durable"`
	Classes []trust.ActionClass `json:"classes,omitempty"`
	LCB     float64             `json:"lcb,omitempty"`
	N       int                 `json:"n,omitempty"`
	Bar     float64             `json:"bar,omitempty"`
	AsOf    time.Time           `json:"as_of,omitempty"`
	Reason  string              `json:"reason"`
}

// decision is the policy's answer for one ask or renewal.
type decision struct {
	durable bool
	life    time.Duration
	basis   Basis
}

// trustSnapshot is the last trust view the runtime pushed.
type trustSnapshot struct {
	at   time.Time // service clock when observed
	rows map[trustKey]trust.Row
}

type trustKey struct {
	s trust.Subject
	c trust.ActionClass
}

// fidelityKey marks a fidelity event the hook reported for a holder in a
// class, until a snapshot shows it.
type fidelityKey struct {
	holder string
	class  trust.ActionClass
}

// leaseClasses are the action classes a scope falls in.
func leaseClasses(s Scope) []trust.ActionClass {
	var out []trust.ActionClass
	add := func(grant string) {
		imp, ok := trust.GrantImpact(grant)
		if !ok {
			imp = types.ImpactI4 // an unclassed grant reads the strictest class
		}
		c := trust.ActionClass{Grant: grant, Impact: imp}
		for _, have := range out {
			if have == c {
				return
			}
		}
		out = append(out, c)
	}
	if len(s.Kinds) > 0 {
		add(ClassReadKind)
	}
	if len(s.CaptureIDs) > 0 || len(s.CaptureLabels) > 0 {
		add(ClassReadCaptures)
	}
	for _, g := range s.Grants {
		add(g)
	}
	return out
}

// holderIDs are the agent subject ids a holder's record may be filed under:
// the manifest name and an app desk's bare slug.
func holderIDs(names ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		for _, id := range []string{n, strings.TrimPrefix(n, "app:")} {
			if id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// ObserveTrust takes a fresh trust view of ownerID's records (its rows) as the
// policy's snapshot for that owner's leases, and fires the breaker: a row
// whose breaker is set revokes the holder's durable leases of that owner in
// the class and the per-task ones issued at or before the fidelity event. It
// never reads the store.
func (s *Service) ObserveTrust(ctx context.Context, ownerID string, rows []trust.Row) int {
	snap := &trustSnapshot{at: s.now(), rows: make(map[trustKey]trust.Row, len(rows))}
	for _, r := range rows {
		snap.rows[trustKey{r.Subject, r.Class}] = r
	}
	s.mu.Lock()
	if s.trust == nil {
		s.trust = map[string]*trustSnapshot{}
	}
	s.trust[ownerID] = snap
	// A mark is settled once the snapshot shows an event at or after it, in
	// a row of the holder under any of its names.
	for k, at := range s.fidelity {
		for _, r := range rows {
			if r.Subject.Kind == trust.SubjectAgent && canonHolder(r.Subject.ID) == k.holder && (r.Class == k.class || wildClass(k.class)) && !r.LastFidelityAt.Before(at) {
				delete(s.fidelity, k)
				break
			}
		}
	}
	s.mu.Unlock()
	n := 0
	for _, r := range rows {
		if !r.Breaker || r.Subject.Kind != trust.SubjectAgent {
			continue
		}
		event := r.LastFidelityAt
		ids := holderIDs(r.Subject.ID)
		for _, id := range s.matching(func(l *Record) bool {
			if l.OwnerID != ownerID || !holderIs(l.Holder, ids) || !classHit(l.Scope, r.Class) {
				return false
			}
			return l.Durable || event.IsZero() || !l.IssuedAt.After(event)
		}) {
			if s.end(ctx, id, StateRevoked, ReasonFidelity) {
				n++
			}
		}
	}
	return n
}

// RevokeOnFidelity is the circuit breaker's hook (MA7.8 calls it at the frame
// check): every live lease of desk in class ends at once, durable or
// per-task, and until a trust snapshot shows the event the policy issues the
// holder no durable lease in that class. A class whose grant is "*" or empty
// is every class (a platform event). It returns how many leases it ended.
func (s *Service) RevokeOnFidelity(ctx context.Context, desk string, class trust.ActionClass, at time.Time) int {
	ids := holderIDs(desk)
	s.mu.Lock()
	if s.fidelity == nil {
		s.fidelity = map[fidelityKey]time.Time{}
	}
	k := fidelityKey{holder: canonHolder(desk), class: class}
	if prev, ok := s.fidelity[k]; !ok || at.After(prev) {
		s.fidelity[k] = at
	}
	s.mu.Unlock()
	n := 0
	for _, id := range s.matching(func(l *Record) bool { return holderIs(l.Holder, ids) && classHit(l.Scope, class) }) {
		if s.end(ctx, id, StateRevoked, ReasonFidelity) {
			n++
		}
	}
	return n
}

// canonHolder is the name a holder's marks are kept under: an app desk's
// bare slug.
func canonHolder(name string) string { return strings.TrimPrefix(strings.TrimSpace(name), "app:") }

func holderIs(holder string, ids []string) bool {
	h := canonHolder(holder)
	for _, id := range ids {
		if canonHolder(id) == h {
			return true
		}
	}
	return false
}

func wildClass(c trust.ActionClass) bool { return c.Grant == "" || c.Grant == "*" }

// classHit reports whether a lease of scope falls in class.
func classHit(scope Scope, class trust.ActionClass) bool {
	if wildClass(class) {
		return true
	}
	for _, c := range leaseClasses(scope) {
		if c.Grant == class.Grant {
			return true
		}
	}
	return false
}

// decidePolicy answers a desk's ask or a renewal: durable, and for how long,
// or per-task. desks are the names the holder goes by; model is the run's.
func (s *Service) decidePolicy(ownerID string, desks []string, model string, scope Scope) decision {
	classes := leaseClasses(scope)
	perTask := func(format string, args ...any) decision {
		return decision{basis: Basis{Classes: classes, Reason: "per-task: " + fmt.Sprintf(format, args...)}}
	}
	if len(classes) == 0 {
		return perTask("the scope names no action class")
	}
	now := s.now()
	s.mu.Lock()
	snap := s.trust[ownerID]
	marks := map[fidelityKey]time.Time{}
	for k, v := range s.fidelity {
		marks[k] = v
	}
	s.mu.Unlock()
	if snap == nil {
		return perTask("no trust view has been read")
	}
	age := now.Sub(snap.at)
	if age < 0 || age > TrustMaxAge {
		return perTask("the trust view is %s old, outside [0, %s]", age.Round(time.Second), TrustMaxAge)
	}
	ids := holderIDs(desks...)
	lcb, n, bar := math.Inf(1), math.MaxInt, DurableFloor
	for _, c := range classes {
		if tau, ok := trust.Tau(c.Impact); ok && tau > bar {
			bar = tau
		}
		var found []trust.Row
		for _, id := range ids {
			if r, ok := snap.rows[trustKey{trust.Subject{Kind: trust.SubjectAgent, ID: id}, c}]; ok {
				found = append(found, r)
			}
			if model != "" {
				if r, ok := snap.rows[trustKey{trust.Subject{Kind: trust.SubjectAgentModel, ID: id + "@" + model}, c}]; ok {
					found = append(found, r)
				}
			}
		}
		for k, at := range marks {
			if (k.class == c || wildClass(k.class)) && holderIs(k.holder, ids) {
				return perTask("a fidelity event in %s at %s is not yet in the trust view", k.class, at.Format(time.RFC3339))
			}
		}
		agent := false
		for _, r := range found {
			agent = agent || r.Subject.Kind == trust.SubjectAgent
			if r.Breaker {
				return perTask("the breaker is open in %s", c)
			}
			if math.IsNaN(r.LCB) || math.IsInf(r.LCB, 0) || r.LCB < 0 || r.LCB > 1 {
				return perTask("the lower bound in %s is not a probability (%v)", c, r.LCB)
			}
			if r.LCB < lcb {
				lcb = r.LCB
			}
			if r.N < n {
				n = r.N
			}
		}
		if !agent {
			return perTask("no record in %s", c)
		}
	}
	if n < DurableMinN {
		return perTask("%d items, fewer than %d", n, DurableMinN)
	}
	if lcb < bar {
		return perTask("lower bound %.3f below %.2f", lcb, bar)
	}
	// The life grows linearly with the bound from DurableMin at the bar to
	// DurableMax at 0.99.
	frac := 1.0
	if top := 0.99; bar < top {
		frac = math.Min(1, math.Max(0, (lcb-bar)/(top-bar)))
	}
	life := DurableMin + time.Duration(frac*float64(DurableMax-DurableMin))
	return decision{durable: true, life: life, basis: Basis{Durable: true, Classes: classes, LCB: lcb, N: n, Bar: bar, AsOf: snap.at,
		Reason: fmt.Sprintf("durable: lower bound %.3f at or above %.2f over %d items; %s per issue, renewed while it holds", lcb, bar, n, life.Round(time.Minute))}}
}

// covers reports whether have covers every member of want.
func covers(have, want Scope) bool {
	sub := func(h, w []string) bool {
		set := map[string]bool{}
		for _, v := range h {
			set[v] = true
		}
		for _, v := range w {
			if !set[v] {
				return false
			}
		}
		return true
	}
	return sub(have.Kinds, want.Kinds) && sub(have.CaptureIDs, want.CaptureIDs) && sub(have.CaptureLabels, want.CaptureLabels) &&
		sub(have.Grants, want.Grants) && sub(have.Refs, want.Refs)
}

// attachDurable attaches runID to a live durable lease of the same owner,
// computer and holder that covers scope and is inside its window. It returns
// the lease after the attach.
func (s *Service) attachDurable(ctx context.Context, ownerID, computerID string, holderWriter, runID string, scope Scope) (Record, bool) {
	now := s.now()
	s.mu.Lock()
	var hit *Record
	for _, r := range s.live {
		if !r.Durable || r.OwnerID != ownerID || r.ComputerID != computerID || r.HolderWriter != holderWriter {
			continue
		}
		if now.Before(r.NotBefore) || r.ExpiresAt.IsZero() || !now.Before(r.ExpiresAt) || !covers(r.Scope, scope) {
			continue
		}
		if hit == nil || r.LeaseID < hit.LeaseID {
			hit = r
		}
	}
	if hit == nil {
		s.mu.Unlock()
		return Record{}, false
	}
	if !containsString(hit.Runs, runID) {
		hit.Runs = append(hit.Runs, runID)
	}
	out := *hit
	out.Runs = append([]string(nil), hit.Runs...)
	s.mu.Unlock()
	if err := s.p.PutLease(context.WithoutCancel(ctx), out); err != nil {
		// The attach is in the live index; the tape keeps the earlier runs.
		// The boot pass ends the lease either way.
		log.Printf("lease: record the attach of run %s to %s: %v", runID, out.LeaseID, err)
	}
	return out, true
}

// renewOrEnd handles a durable lease past its expiry: renew it without asking
// while the record still earns it and the ceiling still holds, else end it as
// expired.
func (s *Service) renewOrEnd(ctx context.Context, id string) {
	s.mu.Lock()
	r := s.live[id]
	if r == nil {
		s.mu.Unlock()
		return
	}
	owner, desk, model, scope := r.OwnerID, r.Holder, r.HolderModel, r.Scope
	s.mu.Unlock()
	d, ok := lookupDesk(desk)
	if ok && withinCeiling(d, scope) == nil {
		if dec := s.decidePolicy(owner, []string{desk, d.Name}, model, scope); dec.durable {
			now := s.now()
			s.mu.Lock()
			r = s.live[id]
			if r != nil && !now.Before(r.ExpiresAt) {
				life := dec.life
				if r.AskedLife > 0 && r.AskedLife < life {
					life = r.AskedLife
				}
				r.NotBefore, r.ExpiresAt, r.Renewals = now, now.Add(life), r.Renewals+1
				b := dec.basis
				r.Basis = &b
				out := *r
				out.Runs = append([]string(nil), r.Runs...)
				s.mu.Unlock()
				if err := s.p.PutLease(context.WithoutCancel(ctx), out); err != nil {
					log.Printf("lease: record the renewal of %s: %v", id, err)
				}
				return
			}
			// Gone (revoked or ended meanwhile), or renewed by another caller.
			s.mu.Unlock()
			return
		}
	}
	s.end(ctx, id, StateEnded, ReasonExpired)
}

// Sweep renews or ends every live lease past its expiry. The runtime calls it
// when it pushes a trust view, so an idle durable lease does not sit past its
// expiry.
func (s *Service) Sweep(ctx context.Context) {
	now := s.now()
	var durable, expired []string
	s.mu.Lock()
	for id, r := range s.live {
		if r.ExpiresAt.IsZero() || now.Before(r.ExpiresAt) {
			continue
		}
		if r.Durable {
			durable = append(durable, id)
		} else {
			expired = append(expired, id)
		}
	}
	s.mu.Unlock()
	for _, id := range durable {
		s.renewOrEnd(ctx, id)
	}
	for _, id := range expired {
		s.end(ctx, id, StateEnded, ReasonExpired)
	}
}

// TrustAge is how old the policy's trust snapshot of ownerID is; ok is false
// when none has been read.
func (s *Service) TrustAge(ownerID string) (time.Duration, bool) {
	s.mu.Lock()
	snap := s.trust[ownerID]
	s.mu.Unlock()
	if snap == nil {
		return 0, false
	}
	return s.now().Sub(snap.at), true
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// PolicySentence says the lease policy in words for the Library's block (L1):
// how a desk's grants are lent inside the ceiling the release lists. It is
// built from the policy numbers above, so the words cannot drift from them.
func PolicySentence() string {
	return fmt.Sprintf("A desk's permissions are lent one task at a time and never go beyond what the app lists here. "+
		"A desk with a strong record for that kind of act (at least %d settled outcomes, with a lower bound of %d%%) "+
		"may hold one for %d to %d hours, renewed while its record holds. A broken commitment ends the loan at once, "+
		"and you can end any loan yourself.",
		DurableMinN, int(math.Round(DurableFloor*100)), int(DurableMin/time.Hour), int(DurableMax/time.Hour))
}
