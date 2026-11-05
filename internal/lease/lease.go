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
// Until the trust policy (MA7.4) every lease is per-task: its lifetime is the
// run it was asked for. A crash restart never resumes a run, so the boot pass
// ends every lease still open (EndAll) along with the runs the crash closed.
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
	s := fmt.Sprintf("%s may read %s in run %s, for: %s (%s)", r.Holder, strings.Join(what, " and "), r.HolderRunID, r.Purpose, r.State)
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

	mu     sync.Mutex
	live   map[string]*Record // lease id -> live lease
	seq    int
	remove func()
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
	rec := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    requestID,
		Kind:        types.CommitmentKindPrecommit,
		Discrepancy: types.DiscrepancyUnresolved,
		Precommit: &types.CommitmentPrecommit{
			Question:     claim,
			Distribution: map[string]float64{"yes": pYes, "no": 1 - pYes},
			Resolver:     kernelServiceAddressee,
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
		Addressee:    kernelServiceAddressee,
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
	now := s.now()
	r := Record{
		LeaseID: "lease-" + hex.EncodeToString(sum[6:12]), OwnerID: req.OwnerID, ComputerID: req.ComputerID,
		Holder: deskName, HolderWriter: string(holder), HolderRunID: req.RunID, Assignment: req.Assignment,
		Custodians: custodians, Scope: req.Scope, Purpose: req.Purpose, RequestRef: requestID,
		NotBefore: now, IssuedAt: now, State: StateActive,
	}
	if req.Duration > 0 {
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
	s.mu.Unlock()
	return r, nil
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
		for _, g := range req.Scope.Grants {
			if err := grantAllowed(desk.Name, g); err != nil {
				return "", "", nil, err
			}
			c, _ := grantCustodian(g)
			custodians = append(custodians, string(c))
		}
		sort.Strings(custodians)
		return desk.Name, holder, custodians, nil
	}
	if len(req.Scope.Grants) > 0 {
		return "", "", nil, refuse("a first-party grant is issued by its custodian, never asked for")
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
	return Scope{Kinds: clean(s.Kinds), CaptureIDs: clean(s.CaptureIDs), CaptureLabels: clean(s.CaptureLabels), Grants: clean(s.Grants)}
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
	return strings.Join(parts, " ")
}

// liveFor returns the live leases of holder in runID that are inside their
// window, and ends any whose expiry passed.
func (s *Service) liveFor(holder objectgraph.Writer, runID string) []*Record {
	now := s.now()
	var out, expired []*Record
	s.mu.Lock()
	for _, r := range s.live {
		if r.HolderWriter != string(holder) || r.HolderRunID != runID {
			continue
		}
		if !r.ExpiresAt.IsZero() && !now.Before(r.ExpiresAt) {
			expired = append(expired, r)
			continue
		}
		if now.Before(r.NotBefore) {
			continue
		}
		out = append(out, r)
	}
	s.mu.Unlock()
	for _, r := range expired {
		s.end(context.Background(), r.LeaseID, StateEnded, ReasonExpired)
	}
	return out
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
			if k == string(kind) && withinCeiling(desk, Scope{Kinds: []string{k}}) == nil {
				s.noteUse(r.LeaseID)
				return true
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
		if match {
			s.noteUse(r.LeaseID)
			return true
		}
	}
	return false
}

// AdmitsGrant implements objectgraph.LeaseAuthority: it reports the id of a
// live lease of holder in runID that names grant, and counts the use. The grant
// table is re-checked on every use, as the ceiling is for kinds.
func (s *Service) AdmitsGrant(holder objectgraph.Writer, runID, grant string) (string, bool) {
	for _, r := range s.liveFor(holder, runID) {
		desk, ok := deskOf(r)
		if !ok || grantAllowed(desk.Name, grant) != nil {
			continue
		}
		for _, g := range r.Scope.Grants {
			if g == grant {
				s.noteUse(r.LeaseID)
				return r.LeaseID, true
			}
		}
	}
	return "", false
}

func (s *Service) noteUse(id string) {
	s.mu.Lock()
	if r := s.live[id]; r != nil {
		r.Uses++
	}
	s.mu.Unlock()
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
	done := *r
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

// EndRun ends the leases of a run that finished. It returns how many.
func (s *Service) EndRun(ctx context.Context, runID, reason string) int {
	n := 0
	for _, id := range s.matching(func(r *Record) bool { return r.HolderRunID == runID }) {
		if s.end(ctx, id, StateEnded, reason) {
			n++
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
			out = append(out, *r)
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
			recs[i] = *r
		}
	}
	s.mu.Unlock()
	sort.Slice(recs, func(i, j int) bool { return recs[i].IssuedAt.After(recs[j].IssuedAt) })
	return recs, nil
}

var _ objectgraph.LeaseAuthority = (*Service)(nil)
