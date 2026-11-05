package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// EvidenceClass is a class of trust evidence (trust draft 3.1).
type EvidenceClass string

const (
	E1 EvidenceClass = "E1" // outcome: accuracy, score 1
	E2 EvidenceClass = "E2" // coverage: a foreseeable consequence left out, score 3
	E3 EvidenceClass = "E3" // fidelity: declared versus used, from the frame log
	E4 EvidenceClass = "E4" // refusal: the frame check refused a call outside the grants
	E5 EvidenceClass = "E5" // peer: a request or sub-call accepted or contradicted
	E6 EvidenceClass = "E6" // owner: approve, reject, revert, uninstall
	E7 EvidenceClass = "E7" // platform: yank, forced rollback, blocking finding
	E8 EvidenceClass = "E8" // cost: estimate versus metered spend
)

// SubjectKind says what a track record belongs to (trust draft 2).
type SubjectKind string

const (
	SubjectAgent      SubjectKind = "agent"       // a desk
	SubjectAgentModel SubjectKind = "agent_model" // a desk on one model (desk@model)
	SubjectModel      SubjectKind = "model"
	SubjectPackage    SubjectKind = "package"
	SubjectPublisher  SubjectKind = "publisher"
	SubjectScorer     SubjectKind = "scorer"
	SubjectReviewer   SubjectKind = "reviewer"
)

// Subject is who a track record is about.
type Subject struct {
	Kind SubjectKind `json:"kind"`
	ID   string      `json:"id"`
}

// SubjectInfo places a desk in the package and publisher hierarchy.
type SubjectInfo struct {
	Package   string
	Publisher string
	// Surface is the package version's behaviour-surface hash (MA7.3).
	Surface string
}

// SubjectResolver maps a desk to its package and publisher. Nil means no
// hierarchy is known: no package, publisher or independence check applies.
type SubjectResolver func(desk string) SubjectInfo

// Exclusion reasons (trust draft 3.1, "Excluded, always").
const (
	ExcludedInterrupted    = "interrupted"             // work a crash closed
	ExcludedSelfFulfilling = "self_fulfilling"         // the outcome is the emitter's own later act
	ExcludedSamePublisher  = "same_publisher_scorer"   // settled by a scorer of the subject's publisher
	ExcludedSameFamily     = "same_family_scorer"      // an independent judge of the same model family
	ExcludedSelfScored     = "self_scored"             // a self-score never settles
	ExcludedWeightZero     = "weight_zero"             // the Cast states no weight
	ExcludedFreezeHash     = "freeze_hash_mismatch"    // the record's frozen procedure fails its hash
	ExcludedReferenceHash  = "reference_hash_mismatch" // the frozen reference fails its hash or names another procedure
	ExcludedZeroWeight     = "zero_weight"             // frozen weight is zero (inert, or irrelevant)
)

// Naming origins for a foreseeable consequence (plan 10.1).
const (
	NamedByReference = "reference_predictor"
	NamedByCritic    = "critic_panel"
	NamedByPeer      = "peer_record"
)

// MinNamingProbability is the materiality floor a peer's naming must carry.
const MinNamingProbability = 0.05

// FrozenConsequence is one consequence of the reference set C*, or one that a
// reference predictor, the critic panel or another desk's scored record named
// before the commitment. Its weight is w = PRef * impact * relevance.
type FrozenConsequence struct {
	Label     string            `json:"label"`
	PRef      float64           `json:"p_ref"`
	Impact    types.ImpactClass `json:"impact"`
	Relevance float64           `json:"relevance"`
	NamedBy   string            `json:"named_by"`
	NamedAt   string            `json:"named_at"`
	// For a peer naming: the desk that named it, whether its record is itself
	// scored under the proper rule, and the probability it carried.
	NamerDesk   string  `json:"namer_desk,omitempty"`
	NamerScored bool    `json:"namer_scored,omitempty"`
	NamerP      float64 `json:"namer_p,omitempty"`
}

// W is the consequence's frozen weight.
func (c FrozenConsequence) W() float64 { return c.PRef * ImpactNumber(c.Impact) * c.Relevance }

// FrozenReference is what is frozen with a record at commit time and hashed
// with it (plan 10.1): the record-level P_ref and relevance, the weights of
// its typed questions, the reference consequence set, and the hash of the
// record's materiality procedure it was frozen against. Nothing that happens
// later changes any of it.
type FrozenReference struct {
	RecordID      string  `json:"record_id"`
	ProcedureHash string  `json:"procedure_hash"`
	CommittedAt   string  `json:"committed_at"`
	PRef          float64 `json:"p_ref"`
	Relevance     float64 `json:"relevance"`
	// QuestionWeights are the frozen weights of the record's typed questions,
	// index-aligned; empty means equal.
	QuestionWeights []float64           `json:"question_weights,omitempty"`
	Consequences    []FrozenConsequence `json:"consequences,omitempty"`
	Hash            string              `json:"hash"`
}

type referenceHashInput FrozenReference

// Digest recomputes the reference's hash over everything but Hash.
func (r FrozenReference) Digest() string {
	in := referenceHashInput(r)
	in.Hash = ""
	raw, _ := json.Marshal(in)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// NewFrozenReference freezes a reference for rec.
func NewFrozenReference(rec types.CommitmentRecord, committedAt string, pRef, relevance float64, cons []FrozenConsequence, questionWeights []float64) FrozenReference {
	r := FrozenReference{RecordID: rec.RecordID, CommittedAt: committedAt, PRef: pRef, Relevance: relevance,
		QuestionWeights: questionWeights, Consequences: cons}
	if rec.MaterialityProcedure != nil {
		r.ProcedureHash = rec.MaterialityProcedure.Hash
	}
	r.Hash = r.Digest()
	return r
}

// FrameObservation is the frame log's view of one run: the grants the record
// declared against the grants the trace shows it used (E3).
type FrameObservation struct {
	Desk     string
	RecordID string
	Declared []string
	Used     []string
	At       string
}

// Refusal is a call the frame check refused outside the desk's grants (E4).
type Refusal struct {
	Desk  string
	Grant string
	At    string
}

// OwnerAct is an owner act on a desk's work: approve, reject, revert or
// uninstall (E6).
type OwnerAct struct {
	Desk  string
	Act   string
	Class ActionClass
	At    string
}

// Platform event kinds (E7).
const (
	PlatformYank            = "yank"
	PlatformForcedRollback  = "forced_rollback"
	PlatformBlockingFinding = "blocking_finding"
)

// PlatformEvent is a platform act against a package and its publisher (E7).
// Classes empty means every class the subjects hold.
type PlatformEvent struct {
	Package   string
	Publisher string
	Kind      string
	Classes   []ActionClass
	At        string
}

// CostObservation pairs a committed cost estimate with the metered spend (E8).
type CostObservation struct {
	Desk      string
	Class     ActionClass
	Estimated float64
	Metered   float64
	At        string
}

// Inputs is everything a view is derived from. The view is a pure function of
// them; none is written back.
type Inputs struct {
	// Records are commitment records: requests, sub-call Casts and their
	// settlements, precommits and their resolves.
	Records []types.CommitmentRecord
	// References are the frozen references by record id.
	References map[string]FrozenReference
	// Materialized lists, by record id, the consequences the record's frozen
	// procedure measured as material when its window closed.
	Materialized map[string][]string
	// Realized is the realized-materiality class the frozen procedure
	// measured, by record id (the importance-judgment score only).
	Realized map[string]types.ImpactClass

	Frames    []FrameObservation
	Refusals  []Refusal
	OwnerActs []OwnerAct
	Platform  []PlatformEvent
	Costs     []CostObservation

	Resolver SubjectResolver
	// SelfFulfilling lets the custodian flag a record whose outcome is the
	// emitter's own later act, beyond the SubjectRefs outcome_emitter rule.
	SelfFulfilling func(types.CommitmentRecord) bool
	// BaseRates is the reference success rate per action class (default 0.5).
	BaseRates map[ActionClass]float64
	// Prior supplies the prior pseudo-counts of a subject in a class (MA7.3);
	// nil means the platform base.
	Prior PriorFunc
	// SubjectSince, when set, starts a subject's own evidence at the given
	// time (a new version's record starts at its install; what came before
	// reaches it only through its prior, discounted by change).
	SubjectSince func(Subject) time.Time
}

// PriorFunc returns (alpha0, beta0).
type PriorFunc func(s Subject, c ActionClass) (float64, float64)

// PlatformBase is the base prior per class: deliberately low for I3 and I4.
func PlatformBase(c ActionClass) (float64, float64) {
	switch c.Impact {
	case types.ImpactI3:
		return 1, 2
	case types.ImpactI4:
		return 1, 4
	}
	return 1, 1
}

// Item is one piece of extracted evidence about a desk (or directly about a
// package or publisher), before it is folded into any posterior.
type Item struct {
	Class    EvidenceClass
	RecordID string
	Desk     string
	Model    string
	Direct   []Subject
	Action   ActionClass
	At       time.Time
	// W is the frozen weight. Q in [0,1] is the settled quality when Skill is
	// false; with Skill, D in [-1,1] is the unclipped skill against the base
	// rate: the alpha increment is max(D,0) and the beta increment max(-D,0).
	W     float64
	Q     float64
	D     float64
	Skill bool
	// M is the evidence-strength multiplier of negative evidence.
	M float64
	// Settler is who settled it.
	Settler string
}

// QPos and QNeg are the unit increments of the item.
func (i Item) QPos() float64 {
	if i.Skill {
		return maxF(i.D, 0)
	}
	return i.Q
}
func (i Item) QNeg() float64 {
	if i.Skill {
		return maxF(-i.D, 0)
	}
	return 1 - i.Q
}

// Failure reports an item that counts as a failure of its subject.
func (i Item) Failure() bool {
	if i.Skill {
		return i.D < 0
	}
	return i.Q < 0.5
}

// Excluded is evidence that was seen and does not count.
type Excluded struct {
	RecordID string `json:"record_id"`
	Desk     string `json:"desk,omitempty"`
	Reason   string `json:"reason"`
}

// Surprise is a consequence that turned out material and that nobody named
// before the commitment: recorded against no one.
type Surprise struct {
	RecordID       string `json:"record_id"`
	Label          string `json:"label"`
	CountedAgainst string `json:"counted_against"`
}

// JudgmentStat is score 2 for one subject: the mean ranked probability score
// of its impact-class forecasts against what the frozen procedure measured.
// It is shown beside the posterior and moves nothing.
type JudgmentStat struct {
	N       int     `json:"n"`
	MeanRPS float64 `json:"mean_rps"`
	// Half is the half-width of an approximate 95% interval.
	Half float64 `json:"half"`
}

// DeskOf is the desk of an agent id ("texture:doc-1" is "texture"); host
// services ("system:...") are no desk.
func DeskOf(agentID string) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || strings.HasPrefix(agentID, "system:") {
		return ""
	}
	d, _, _ := strings.Cut(agentID, ":")
	return d
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
