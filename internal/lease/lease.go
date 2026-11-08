// Package lease is the lease service (modular architecture plan MA3.4, R20,
// section 8.5).
//
// Authority has two layers. The desk manifest names a CEILING: the most a desk
// may ever request (deskmanifest.Desk.LeaseKinds, LeaseCaptures). A LEASE is
// what the desk holds now: a custodian-issued record on the tape that lets one
// desk, in one run, read a kind or a capture it does not keep, for a stated
// purpose. A desk asks through a request record that is itself a scored
// precommitment (types.RequestLease: p_success, weight, settlement as a Cast
// carries them). The service decides by its rules and writes the lease; the
// owner sees every lease and revokes any; an uninstall, a yank or a retirement
// of the desk revokes its leases; and the run's end ends them.
//
// A lease is per-task unless the trust policy (MA7.4, policy.go) finds the
// holder's record strong in the lease's class: a per-task lease's lifetime is
// the run it was asked for; a durable one lives a stated period, serves the
// holder's runs that ask, and renews without asking while the record holds. A
// crash restart never resumes a run, so the boot pass ends every lease still
// open (EndAll), durable ones included, along with the runs the crash closed.
// A planned restart's one resume asks for its leases again.
//
// The reader gate (objectgraph.checkReadKind) and the capture frame handler
// ask the service through objectgraph.LeaseAuthority. The service re-checks the
// ceiling on every use, so a lease record written outside the service (a
// forged or stale one) never reads beyond what the manifest allows.
package lease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// States of a lease record.
const (
	StateActive  = "active"
	StateEnded   = "ended"
	StateRevoked = "revoked"
)

// End reasons.
const (
	ReasonRunEnded = "run ended"
	ReasonRestart  = "interrupted by a restart"
	ReasonExpired  = "expired"
	ReasonOwner    = "revoked by the owner"
	ReasonRetired  = "desk retired (uninstall, yank or retirement)"
)

// MaxDuration bounds a lease's own expiry. A per-task lease also ends with its
// run, whichever comes first.
const MaxDuration = time.Hour

// Scope is what a lease covers. A lease is a set of kinds, of captures by id,
// of captures by label, or a mix.
type Scope struct {
	Kinds         []string `json:"kinds,omitempty"`
	CaptureIDs    []string `json:"capture_ids,omitempty"`
	CaptureLabels []string `json:"capture_labels,omitempty"`
	// Grants name first-party typed grants (GrantTextureSearch): a guarded
	// service API the custodian owns, not a read of a kind. A grant lease
	// opens no kind to the reader gate; the custodian's service checks it
	// (objectgraph.LeaseAdmitsGrant) and then reads as the custodian.
	Grants []string `json:"grants,omitempty"`
	// Refs name the objects of a ref-scoped grant (mail.read: email refs):
	// the grant serves those refs and no other (MA4.7). Only a grant lease
	// carries refs, and only for a grant the table marks by-ref.
	Refs []string `json:"refs,omitempty"`
}

func (s Scope) empty() bool {
	return len(s.Kinds) == 0 && len(s.CaptureIDs) == 0 && len(s.CaptureLabels) == 0 && len(s.Grants) == 0
}

// Record is the lease record on the tape (kind choir.lease).
type Record struct {
	LeaseID    string `json:"lease_id"`
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	// Holder is the desk, HolderWriter its writer, HolderRunID the run the
	// lease is for. The holder comes from the run record, never from a cell.
	// A disclosure grant (GrantTextureBrief) is for the custodian's run whose
	// messages it admits, so it ends with that run.
	Holder       string `json:"holder"`
	HolderWriter string `json:"holder_writer"`
	HolderRunID  string `json:"holder_run_id"`
	// Assignment names the engineering assignment of a capsule lease (X6).
	Assignment string `json:"assignment,omitempty"`
	// Custodians are the writers of the kinds the scope names: the desks and
	// services whose data the lease opens.
	Custodians []string `json:"custodians,omitempty"`
	Scope      Scope    `json:"scope"`
	Purpose    string   `json:"purpose"`
	// RequestRef is the request record, the precommitment the lease answers.
	RequestRef string     `json:"request_ref"`
	NotBefore  time.Time  `json:"not_before"`
	IssuedAt   time.Time  `json:"issued_at"`
	ExpiresAt  time.Time  `json:"expires_at,omitempty"`
	State      string     `json:"state"`
	EndReason  string     `json:"end_reason,omitempty"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	// Uses counts the reads served against the lease (the use log).
	Uses int `json:"uses,omitempty"`

	// The lease policy (MA7.4, policy.go). A durable lease is not ended by its
	// run: Runs are the runs of the holder that asked and were attached
	// (HolderRunID is the first), it lives to ExpiresAt, and it renews without
	// asking (Renewals) while the record earns it. A per-task lease has
	// Durable false and Runs empty. HolderModel is the model of the run that
	// asked, read again at renewal; AskedLife is the desk's own duration
	// bound, if it named one. Basis says why the lease got its life.
	Durable     bool          `json:"durable,omitempty"`
	Runs        []string      `json:"runs,omitempty"`
	HolderModel string        `json:"holder_model,omitempty"`
	AskedLife   time.Duration `json:"asked_life,omitempty"`
	Renewals    int           `json:"renewals,omitempty"`
	Basis       *Basis        `json:"basis,omitempty"`
}

// clone copies r with its own slices.
func (r Record) clone() Record {
	r.Runs = append([]string(nil), r.Runs...)
	if r.Basis != nil {
		b := *r.Basis
		r.Basis = &b
	}
	return r
}

// Sentence renders the lease for the owner's view.
func (r Record) Sentence() string {
	var what []string
	if len(r.Scope.Kinds) > 0 {
		what = append(what, "the kinds "+strings.Join(r.Scope.Kinds, ", "))
	}
	if len(r.Scope.CaptureIDs) > 0 {
		what = append(what, fmt.Sprintf("%d named capture(s)", len(r.Scope.CaptureIDs)))
	}
	if len(r.Scope.CaptureLabels) > 0 {
		what = append(what, "captures labelled "+strings.Join(r.Scope.CaptureLabels, ", "))
	}
	if len(r.Scope.Grants) > 0 {
		what = append(what, "the grants "+strings.Join(r.Scope.Grants, ", "))
	}
	if len(r.Scope.Refs) > 0 {
		what = append(what, fmt.Sprintf("%d named ref(s)", len(r.Scope.Refs)))
	}
	s := fmt.Sprintf("%s may read %s in run %s, for: %s (%s)", r.Holder, strings.Join(what, " and "), r.HolderRunID, r.Purpose, r.State)
	if r.Durable {
		s = fmt.Sprintf("%s may read %s in the runs that ask (%d so far) until %s, renewed %d time(s) on its record, for: %s (%s)",
			r.Holder, strings.Join(what, " and "), len(r.Runs), r.ExpiresAt.Format(time.RFC3339), r.Renewals, r.Purpose, r.State)
	}
	if r.EndReason != "" {
		s += ": " + r.EndReason
	}
	return s
}

// Persistence is how the service reaches the tape. internal/store implements
// it; the service keeps its own in-memory index of the live leases so the
// reader gate never reads the store from inside a store read.
type Persistence interface {
	PutLease(ctx context.Context, rec Record) error
	ListLeases(ctx context.Context, ownerID, computerID string) ([]Record, error)
	AppendRequestRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) (string, error)
}

// Service is the lease service of one computer.
type Service struct {
	p   Persistence
	now func() time.Time

	mu   sync.Mutex
	live map[string]*Record // lease id -> live lease
	// issueMu serialises custodian-ruled grant requests (IssueGrant), so a
	// rule that counts the run's live leases sees every earlier issue.
	issueMu sync.Mutex
	seq     int
	remove  func()

	// trust is the policy's snapshot of the trust view per owner and fidelity the
	// events the breaker hook reported that no snapshot shows yet (policy.go).
	// Both are guarded by mu.
	trust    map[string]*trustSnapshot // by owner
	fidelity map[fidelityKey]time.Time
}

// New returns a service over p. It revokes a desk's leases when the desk is
// retired (an uninstall, a yank the computer applied, a package disabled).
func New(p Persistence) *Service {
	s := &Service{p: p, now: func() time.Time { return time.Now().UTC() }, live: map[string]*Record{}}
	s.remove = deskmanifest.OnRetireApp(func(name string) { s.RevokeHolder(context.Background(), name, ReasonRetired) })
	return s
}

// Close unregisters the service's retirement observer.
func (s *Service) Close() {
	if s.remove != nil {
		s.remove()
	}
}

// SetClock replaces the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Load reads the active leases of the computer from the tape into the live
// index. It is the restart path: a lease a crash left active is ended by
// EndAll right after, never trusted.
func (s *Service) Load(ctx context.Context, ownerID, computerID string) error {
	recs, err := s.p.ListLeases(ctx, ownerID, computerID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range recs {
		if r.State == StateActive {
			r := r
			s.live[r.LeaseID] = &r
		}
	}
	return nil
}

// Request is a desk's ask. Desk, RunID, AgentID and ModelID come from the
// run's record in trusted code; Fields from the cell (parsed and validated
// here), as a Cast's do.
type Request struct {
	OwnerID    string
	ComputerID string
	Desk       string
	RunID      string
	AgentID    string
	ModelID    string
	Assignment string
	Origin     string
	Scope      Scope
	Purpose    string
	// Duration, if positive, bounds the lease beyond its run (at most
	// MaxDuration); zero is the run's lifetime.
	Duration time.Duration
	Fields   *types.RequestFields

	// capsule marks the platform's own store-copy lease (IssueCapsule): its
	// bound is the built-in kind table minus the withheld kinds, not a desk
	// ceiling. Unexported: only this package can ask.
	capsule bool
	// grant marks a platform-issued first-party grant lease (IssueTextureSearch):
	// its bound is the grant table (grants.go), not a desk ceiling.
	grant bool
	// addressee is the custodian a grant request is addressed to (IssueGrant);
	// empty is the lease service itself.
	addressee string
	// rule is the custodian's own rule for a grant request (IssueGrant): it
	// sees the live leases the holder's run already has and refuses or admits.
	// It runs after the request is on the tape, so a refused ask leaves its
	// record.
	rule func(held []Record) error
}

// ErrRefused wraps every refusal of a request.
var ErrRefused = errors.New("lease: request refused")

func refuse(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrRefused, fmt.Sprintf(format, args...))
}

// Request records the ask as a scored precommitment and, when the ceiling and
// the rules allow it, issues the lease. A refused request leaves its record on
// the tape (the ask is evidence for the requester's record) and returns
// ErrRefused.
func (s *Service) Request(ctx context.Context, req Request) (Record, error) {
	if strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.Desk) == "" {
		return Record{}, refuse("a lease is per-task: the request needs the desk and the run it is for")
	}
	if strings.TrimSpace(req.Purpose) == "" {
		return Record{}, refuse("a lease needs a stated purpose")
	}
	if err := types.ValidateRequestFields(types.RequestLease, req.Fields); err != nil {
		return Record{}, refuse("%v", err)
	}
	req.Scope = normalizeScope(req.Scope)
	if req.Scope.empty() {
		return Record{}, refuse("a lease needs a scope: kinds, capture ids or capture labels")
	}
	origin := req.Origin
	if origin == "" {
		origin = types.OriginDeskRun
	}
	s.mu.Lock()
	s.seq++
	local := fmt.Sprintf("%s:%d:%d", req.RunID, s.now().UnixNano(), s.seq)
	s.mu.Unlock()
	sum := sha256.Sum256([]byte(local + "\x00" + req.Purpose))
	requestID := req.RunID + ":lease_request:" + hex.EncodeToString(sum[:6])

	// The request is a precommit record, as an Ask is (ask is a subset of
	// precommit): its claim is that the lease serves its purpose, with the
	// requester's own probability and weight, and the lease service resolves
	// it. A directive would be excluded from scoring.
	pYes := *req.Fields.PSuccess
	claim := "the lease serves its stated purpose: " + req.Purpose
	addressee := kernelServiceAddressee
	if req.addressee != "" {
		addressee = req.addressee
	}
	rec := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    requestID,
		Kind:        types.CommitmentKindPrecommit,
		Discrepancy: types.DiscrepancyUnresolved,
		Precommit: &types.CommitmentPrecommit{
			Question:     claim,
			Distribution: map[string]float64{"yes": pYes, "no": 1 - pYes},
			Resolver:     addressee,
		},
		Provenance: types.CommitmentProvenance{
			AgentID: req.AgentID, ModelID: req.ModelID, Desk: req.Desk, RunID: req.RunID, Origin: origin,
			CommittedAt: s.now().Format(time.RFC3339Nano),
		},
		Prediction: types.CommitmentPrediction{
			Hypothesis: claim,
			Questions: []types.TypedQuestion{{
				Question: claim, Answers: []string{"yes", "no"},
				Probabilities: map[string]float64{"yes": pYes, "no": 1 - pYes}, Weight: *req.Fields.Weight,
			}},
			CommittedAt: s.now().Format(time.RFC3339Nano),
		},
		EvidenceRefs: []string{"scope:" + scopeSummary(req.Scope)},
		Addressee:    addressee,
	}
	if err := rec.StampRequest(types.RequestLease, req.Fields); err != nil {
		return Record{}, refuse("%v", err)
	}
	if req.AgentID == "" {
		rec.Provenance.AgentID = req.Desk
	}
	if _, err := s.p.AppendRequestRecord(ctx, req.OwnerID, req.ComputerID, rec); err != nil {
		return Record{}, fmt.Errorf("lease: record the request: %w", err)
	}

	deskName, holder, custodians, err := s.decide(req)
	if err != nil {
		return Record{}, err
	}
	if req.rule != nil {
		var held []Record
		for _, r := range s.liveFor(holder, req.RunID) {
			held = append(held, *r)
		}
		if err := req.rule(held); err != nil {
			if errors.Is(err, ErrRefused) {
				return Record{}, err
			}
			return Record{}, refuse("%v", err)
		}
	}
	// The lease policy shapes a desk's own ask; the platform's grants and
	// the capsule copy stay per-task (policy.go).
	var dec decision
	if !req.capsule && !req.grant {
		dec = s.decidePolicy(req.OwnerID, []string{req.Desk, deskName}, req.ModelID, req.Scope)
		if dec.durable {
			if got, ok := s.attachDurable(ctx, req.OwnerID, req.ComputerID, string(holder), req.RunID, req.Scope); ok {
				return got, nil
			}
		}
	}
	now := s.now()
	r := Record{
		LeaseID: "lease-" + hex.EncodeToString(sum[6:12]), OwnerID: req.OwnerID, ComputerID: req.ComputerID,
		Holder: deskName, HolderWriter: string(holder), HolderRunID: req.RunID, Assignment: req.Assignment,
		Custodians: custodians, Scope: req.Scope, Purpose: req.Purpose, RequestRef: requestID,
		NotBefore: now, IssuedAt: now, State: StateActive, HolderModel: req.ModelID,
	}
	if !req.capsule && !req.grant {
		b := dec.basis
		r.Basis = &b
	}
	switch {
	case dec.durable:
		life := dec.life
		if req.Duration > 0 && req.Duration < life {
			life, r.AskedLife = req.Duration, req.Duration
		}
		r.Durable, r.Runs, r.ExpiresAt = true, []string{req.RunID}, now.Add(life)
	case req.Duration > 0:
		d := req.Duration
		if d > MaxDuration {
			d = MaxDuration
		}
		r.ExpiresAt = now.Add(d)
	}
	if err := s.p.PutLease(ctx, r); err != nil {
		return Record{}, fmt.Errorf("lease: record the lease: %w", err)
	}
	s.mu.Lock()
	s.live[r.LeaseID] = &r
	out := r.clone()
	s.mu.Unlock()
	return out, nil
}

// kernelServiceAddressee is the request's addressee: the lease service.
const kernelServiceAddressee = "system:lease"

// decide applies the custodian's rules: the desk is registered, the scope is
// inside its ceiling, every kind is real and not the lease kind itself.
func (s *Service) decide(req Request) (string, objectgraph.Writer, []string, error) {
	desk, ok := lookupDesk(req.Desk)
	if !ok {
		return "", "", nil, refuse("desk %q is not registered", req.Desk)
	}
	holder, ok := objectgraph.DeskWriter(req.Desk)
	if !ok {
		return "", "", nil, refuse("desk %q has no writer", req.Desk)
	}
	if req.grant {
		if len(req.Scope.Kinds) > 0 || len(req.Scope.CaptureIDs) > 0 || len(req.Scope.CaptureLabels) > 0 {
			return "", "", nil, refuse("a grant lease names grants only")
		}
		var custodians []string
		byRef := false
		for _, g := range req.Scope.Grants {
			if err := grantAllowed(desk, g); err != nil {
				return "", "", nil, err
			}
			c, _ := grantCustodian(g)
			custodians = append(custodians, string(c))
			byRef = byRef || grantByRef(g)
		}
		// The by-ref shape is the lease layer's, not only the custodian's:
		// one grant, one ref, and the custodian's own rule decides it.
		if byRef && (len(req.Scope.Grants) != 1 || len(req.Scope.Refs) != 1) {
			return "", "", nil, refuse("a by-ref grant is asked for alone and names exactly one ref")
		}
		if byRef && req.rule == nil {
			return "", "", nil, refuse("a by-ref grant is issued only under its custodian's rule")
		}
		if !byRef && len(req.Scope.Refs) > 0 {
			return "", "", nil, refuse("refs scope only a by-ref grant")
		}
		sort.Strings(custodians)
		return desk.Name, holder, custodians, nil
	}
	if len(req.Scope.Grants) > 0 {
		return "", "", nil, refuse("a first-party grant is issued by its custodian, never asked for")
	}
	if len(req.Scope.Refs) > 0 {
		return "", "", nil, refuse("refs scope only a by-ref grant")
	}
	if req.capsule {
		allowed := map[string]bool{}
		for _, k := range CapsuleStoreKinds() {
			allowed[k] = true
		}
		for _, k := range req.Scope.Kinds {
			if !allowed[k] {
				return "", "", nil, refuse("kind %q is outside what a capsule store lease may name", k)
			}
		}
	} else if err := withinCeiling(desk, req.Scope); err != nil {
		return "", "", nil, err
	}
	seen := map[string]bool{}
	var custodians []string
	for _, k := range req.Scope.Kinds {
		if k == "choir.lease" {
			return "", "", nil, refuse("a lease does not open the lease kind")
		}
		c, known := objectgraph.CustodianOf(objectgraph.ObjectKind(k))
		if !known {
			return "", "", nil, refuse("kind %q is not registered", k)
		}
		if !seen[string(c)] {
			seen[string(c)] = true
			custodians = append(custodians, string(c))
		}
	}
	sort.Strings(custodians)
	return desk.Name, holder, custodians, nil
}

// lookupDesk resolves a desk by its manifest name, or by an app desk's bare
// slug (a run record names the profile either way).
func lookupDesk(name string) (deskmanifest.Desk, bool) {
	name = strings.TrimSpace(name)
	if d, ok := deskmanifest.Lookup(name); ok {
		return d, true
	}
	return deskmanifest.Lookup("app:" + name)
}

// withinCeiling is the ceiling check. It runs at issue and at every use.
func withinCeiling(desk deskmanifest.Desk, scope Scope) error {
	allowed := map[string]bool{}
	for _, k := range desk.LeaseKinds {
		allowed[k] = true
	}
	for _, k := range scope.Kinds {
		if !allowed[k] {
			return refuse("kind %q is outside the ceiling of desk %q (its manifest names %v)", k, desk.Name, desk.LeaseKinds)
		}
	}
	if (len(scope.CaptureIDs) > 0 || len(scope.CaptureLabels) > 0) && !desk.LeaseCaptures {
		return refuse("desk %q's ceiling does not name captures", desk.Name)
	}
	// A label lease names owner-private labels only: a ubiquitous label such as
	// world_untrusted would otherwise read as a wildcard.
	for _, l := range scope.CaptureLabels {
		if !strings.HasPrefix(l, types.LabelOwnerPrivatePrefix) {
			return refuse("capture label %q is not an owner-private label; a label lease names owner-private labels only", l)
		}
	}
	return nil
}

func normalizeScope(s Scope) Scope {
	clean := func(in []string) []string {
		seen := map[string]bool{}
		var out []string
		for _, v := range in {
			v = strings.TrimSpace(v)
			if v != "" && !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
		sort.Strings(out)
		return out
	}
	return Scope{Kinds: clean(s.Kinds), CaptureIDs: clean(s.CaptureIDs), CaptureLabels: clean(s.CaptureLabels), Grants: clean(s.Grants), Refs: clean(s.Refs)}
}

func scopeSummary(s Scope) string {
	var parts []string
	if len(s.Kinds) > 0 {
		parts = append(parts, "kinds="+strings.Join(s.Kinds, ","))
	}
	if len(s.CaptureIDs) > 0 {
		parts = append(parts, "captures="+strings.Join(s.CaptureIDs, ","))
	}
	if len(s.CaptureLabels) > 0 {
		parts = append(parts, "labels="+strings.Join(s.CaptureLabels, ","))
	}
	if len(s.Grants) > 0 {
		parts = append(parts, "grants="+strings.Join(s.Grants, ","))
	}
	if len(s.Refs) > 0 {
		parts = append(parts, "refs="+strings.Join(s.Refs, ","))
	}
	return strings.Join(parts, " ")
}

// liveFor returns the live leases of holder in runID that are inside their
// window: a per-task lease of that run, or a durable lease the run attached
// to. A per-task lease whose expiry passed is ended; a durable one is renewed
// or ended (renewOrEnd).
func (s *Service) liveFor(holder objectgraph.Writer, runID string) []*Record {
	now := s.now()
	var expired, due []string
	s.mu.Lock()
	for id, r := range s.live {
		if r.HolderWriter != string(holder) || !r.heldIn(runID) {
			continue
		}
		if !r.ExpiresAt.IsZero() && !now.Before(r.ExpiresAt) {
			if r.Durable {
				due = append(due, id)
			} else {
				expired = append(expired, id)
			}
		}
	}
	s.mu.Unlock()
	for _, id := range expired {
		s.end(context.Background(), id, StateEnded, ReasonExpired)
	}
	for _, id := range due {
		s.renewOrEnd(context.Background(), id)
	}
	if len(due) > 0 {
		now = s.now()
	}
	var out []*Record
	s.mu.Lock()
	for _, r := range s.live {
		if r.HolderWriter != string(holder) || !r.heldIn(runID) {
			continue
		}
		if !r.ExpiresAt.IsZero() && !now.Before(r.ExpiresAt) {
			continue
		}
		if now.Before(r.NotBefore) {
			continue
		}
		out = append(out, r)
	}
	s.mu.Unlock()
	return out
}

// heldIn reports whether runID holds r: the run a per-task lease is for, or a
// run attached to a durable lease. Callers hold mu.
func (r *Record) heldIn(runID string) bool {
	if r.Durable {
		return containsString(r.Runs, runID)
	}
	return r.HolderRunID == runID
}

// deskOf resolves the holder's registered desk for the ceiling re-check. A
// desk that is no longer registered (retired) holds nothing.
func deskOf(r *Record) (deskmanifest.Desk, bool) { return lookupDesk(r.Holder) }

// AdmitsKind implements objectgraph.LeaseAuthority.
func (s *Service) AdmitsKind(holder objectgraph.Writer, runID string, kind objectgraph.ObjectKind) bool {
	for _, r := range s.liveFor(holder, runID) {
		desk, ok := deskOf(r)
		if !ok {
			continue
		}
		for _, k := range r.Scope.Kinds {
			if k == string(kind) && withinCeiling(desk, Scope{Kinds: []string{k}}) == nil && s.noteUse(r.LeaseID) {
				return true
			}
		}
	}
	return false
}

// AdmitsKindForRecipient reports whether a lease held by the recipient desk
// holder, for ownerID, covers kind: the check a message sent to a package desk
// makes. The lease belongs to the recipient's run (or is durable and attached
// to runs), never to the sender's, so it is found by holder and owner and then
// judged like any use: inside its window, inside the desk's ceiling, held by a
// live run (a per-task lease ends with its run; a durable one is renewed or
// ended). An admitting check counts a use.
func (s *Service) AdmitsKindForRecipient(ownerID string, holder objectgraph.Writer, kind objectgraph.ObjectKind) bool {
	runs := map[string]bool{}
	s.mu.Lock()
	for _, r := range s.live {
		if r.OwnerID != ownerID || r.HolderWriter != string(holder) {
			continue
		}
		if r.Durable {
			for _, id := range r.Runs {
				runs[id] = true
			}
		} else if r.HolderRunID != "" {
			runs[r.HolderRunID] = true
		}
	}
	s.mu.Unlock()
	for runID := range runs {
		for _, r := range s.liveFor(holder, runID) {
			if r.OwnerID != ownerID {
				continue
			}
			desk, ok := deskOf(r)
			if !ok {
				continue
			}
			for _, k := range r.Scope.Kinds {
				if k == string(kind) && withinCeiling(desk, Scope{Kinds: []string{k}}) == nil && s.noteUse(r.LeaseID) {
					return true
				}
			}
		}
	}
	return false
}

// AdmitsCapture implements objectgraph.LeaseAuthority.
func (s *Service) AdmitsCapture(holder objectgraph.Writer, runID, captureID string, labels []string) bool {
	for _, r := range s.liveFor(holder, runID) {
		desk, ok := deskOf(r)
		if !ok || !desk.LeaseCaptures {
			continue
		}
		match := false
		for _, id := range r.Scope.CaptureIDs {
			if id == captureID && captureID != "" {
				match = true
			}
		}
		// By label: every owner-private label the record carries must be
		// covered by the lease (mirrors captureServedTo), so a capture that
		// carries one covered label and another is not admitted.
		if !match && len(r.Scope.CaptureLabels) > 0 {
			covered := map[string]bool{}
			for _, want := range r.Scope.CaptureLabels {
				covered[want] = true
			}
			private, all := 0, true
			for _, l := range labels {
				if strings.HasPrefix(l, types.LabelOwnerPrivatePrefix) {
					private++
					all = all && covered[l]
				}
			}
			match = private > 0 && all
		}
		if match && s.noteUse(r.LeaseID) {
			return true
		}
	}
	return false
}

// AdmitsGrant implements objectgraph.LeaseAuthority: it reports the id of a
// live lease of holder in runID that names grant, and counts the use. The grant
// table is re-checked on every use, as the ceiling is for kinds.
func (s *Service) AdmitsGrant(holder objectgraph.Writer, runID, grant string) (string, bool) {
	// A by-ref grant admits a named ref only (AdmitsGrantRef).
	if grantByRef(grant) {
		return "", false
	}
	return s.admitsGrant(holder, runID, "", grant, "")
}

// AdmitsGrantRef implements objectgraph.LeaseAuthority: AdmitsGrant for a
// by-ref grant, where the lease must be ownerID's and name ref.
func (s *Service) AdmitsGrantRef(holder objectgraph.Writer, runID, ownerID, grant, ref string) (string, bool) {
	if !grantByRef(grant) || strings.TrimSpace(ref) == "" || strings.TrimSpace(ownerID) == "" {
		return "", false
	}
	return s.admitsGrant(holder, runID, ownerID, grant, ref)
}

// admitsGrant finds a live lease of holder in runID naming grant; a non-empty
// ownerID binds it to that owner and a non-empty ref to that ref.
func (s *Service) admitsGrant(holder objectgraph.Writer, runID, ownerID, grant, ref string) (string, bool) {
	for _, r := range s.liveFor(holder, runID) {
		desk, ok := deskOf(r)
		if !ok || grantAllowed(desk, grant) != nil {
			continue
		}
		if ownerID != "" && r.OwnerID != ownerID {
			continue
		}
		named := false
		for _, g := range r.Scope.Grants {
			named = named || g == grant
		}
		if !named {
			continue
		}
		if ref != "" {
			covered := false
			for _, want := range r.Scope.Refs {
				covered = covered || want == ref
			}
			if !covered {
				continue
			}
		}
		if s.noteUse(r.LeaseID) {
			return r.LeaseID, true
		}
	}
	return "", false
}

// noteUse counts a use of a lease that is still live and reports whether it
// was. A revoke between liveFor and here wins: the admission is refused, so no
// read is admitted once a revoke has returned.
func (s *Service) noteUse(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.live[id]
	if r == nil {
		return false
	}
	r.Uses++
	return true
}

// end moves a live lease to a terminal state and writes it to the tape. The
// lease leaves the live index first, so the next read fails whatever the
// write does.
func (s *Service) end(ctx context.Context, id, state, reason string) bool {
	s.mu.Lock()
	r := s.live[id]
	if r == nil {
		s.mu.Unlock()
		return false
	}
	delete(s.live, id)
	done := r.clone()
	s.mu.Unlock()
	now := s.now()
	done.State, done.EndReason, done.EndedAt = state, reason, &now
	if err := s.p.PutLease(context.WithoutCancel(ctx), done); err != nil {
		log.Printf("lease: record the end of %s (%s): %v", id, reason, err)
	}
	return true
}

func (s *Service) matching(f func(*Record) bool) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id, r := range s.live {
		if f(r) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// EndRun ends the per-task leases of a run that finished and detaches the run
// from the durable leases it attached to (a durable lease outlives the run
// and lives to its expiry). It returns how many leases it ended.
func (s *Service) EndRun(ctx context.Context, runID, reason string) int {
	n := 0
	for _, id := range s.matching(func(r *Record) bool { return !r.Durable && r.HolderRunID == runID }) {
		if s.end(ctx, id, StateEnded, reason) {
			n++
		}
	}
	var detached []Record
	s.mu.Lock()
	for _, r := range s.live {
		if !r.Durable || !containsString(r.Runs, runID) {
			continue
		}
		kept := r.Runs[:0:0]
		for _, v := range r.Runs {
			if v != runID {
				kept = append(kept, v)
			}
		}
		r.Runs = kept
		detached = append(detached, r.clone())
	}
	s.mu.Unlock()
	for _, r := range detached {
		if err := s.p.PutLease(context.WithoutCancel(ctx), r); err != nil {
			log.Printf("lease: record run %s leaving %s: %v", runID, r.LeaseID, err)
		}
	}
	return n
}

// EndAll ends every live lease: the boot pass. Every lease is per-task, a
// crash restart closes the runs, and a lease is never resumed with a run.
func (s *Service) EndAll(ctx context.Context, reason string) int {
	n := 0
	for _, id := range s.matching(func(*Record) bool { return true }) {
		if s.end(ctx, id, StateEnded, reason) {
			n++
		}
	}
	return n
}

// End ends one live lease whose task finished before its run (a research
// Texture child's texture.search, MA4.6). It is not a revocation: the holder
// did nothing wrong.
func (s *Service) End(ctx context.Context, id, reason string) error {
	if !s.end(ctx, id, StateEnded, reason) {
		return fmt.Errorf("lease: %q is not a live lease", id)
	}
	return nil
}

// Revoke is the owner's act (and the breaker's, later): the lease ends at
// once.
func (s *Service) Revoke(ctx context.Context, id, reason string) error {
	if !s.end(ctx, id, StateRevoked, reason) {
		return fmt.Errorf("lease: %q is not a live lease", id)
	}
	return nil
}

// RevokeHolder revokes every live lease of a desk (uninstall, yank,
// retirement).
func (s *Service) RevokeHolder(ctx context.Context, desk, reason string) int {
	n := 0
	for _, id := range s.matching(func(r *Record) bool { return r.Holder == desk }) {
		if s.end(ctx, id, StateRevoked, reason) {
			n++
		}
	}
	return n
}

// Live lists the live leases of the owner, for the owner's view.
func (s *Service) Live(ownerID string) []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Record
	for _, r := range s.live {
		if ownerID == "" || r.OwnerID == ownerID {
			out = append(out, r.clone())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IssuedAt.Before(out[j].IssuedAt) })
	return out
}

// All lists every lease of the computer from the tape (live ones with the
// service's use counts), newest first, for the owner's view.
func (s *Service) All(ctx context.Context, ownerID, computerID string) ([]Record, error) {
	recs, err := s.p.ListLeases(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	for i := range recs {
		if r := s.live[recs[i].LeaseID]; r != nil {
			recs[i] = r.clone()
		}
	}
	s.mu.Unlock()
	sort.Slice(recs, func(i, j int) bool { return recs[i].IssuedAt.After(recs[j].IssuedAt) })
	return recs, nil
}

var _ objectgraph.LeaseAuthority = (*Service)(nil)
