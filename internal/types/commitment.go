package types

import "strings"

// Commitment records (precommitment records): the durable unit of the
// commitment ledger. A record is a typed prediction committed before the
// evidence that resolves it, then scored on a verifiable outcome. The
// accumulated log is the agent's world model and supervision surface.
// Source: docs/Precommitment Records — Engineering Memo.md ("Record schema").
//
// The record persists as an objectgraph object (kind
// choir.commitment_record), never a third store. Scalar scores are derived
// views over the rich body; the body is kept so later scorers can rescore.

// DiscrepancyClass is the outcome-of-record verdict for a commitment.
type DiscrepancyClass string

const (
	// DiscrepancyConfirmed: the prediction held.
	DiscrepancyConfirmed DiscrepancyClass = "confirmed"
	// DiscrepancyQualified: partially right; an assumption was off but the
	// hypothesis direction held.
	DiscrepancyQualified DiscrepancyClass = "qualified"
	// DiscrepancyContradicted: the prediction failed.
	DiscrepancyContradicted DiscrepancyClass = "contradicted"
	// DiscrepancyUnresolved: the outcome never became verifiable.
	DiscrepancyUnresolved DiscrepancyClass = "unresolved"
)

// CommitmentRecordKind is the adjudicated act taxonomy (record-native
// messaging, 2026-10-01 panel): precommit, report, resolve, disagreement, or
// directive. Written at mint; legacy records without the field derive it in
// RecordKind — disagreement/resolve/precommit from the typed sub-object,
// directive/report from the RecordID kind segment (legacy minted IDs carry
// the intent kind between cell identity and local id).
type CommitmentRecordKind string

const (
	CommitmentKindPrecommit    CommitmentRecordKind = "precommit"
	CommitmentKindReport       CommitmentRecordKind = "report"
	CommitmentKindResolve      CommitmentRecordKind = "resolve"
	CommitmentKindDisagreement CommitmentRecordKind = "disagreement"
	CommitmentKindDirective    CommitmentRecordKind = "directive"
)

// CommitmentDirectiveSubtype is the closed directive routing enum —
// note, escalate, cast, retract. Never free text: the reducer and
// projections route on it, and open-ended subtypes would smuggle acts past
// the scored taxonomy.
type CommitmentDirectiveSubtype string

const (
	CommitmentDirectiveNote     CommitmentDirectiveSubtype = "note"
	CommitmentDirectiveEscalate CommitmentDirectiveSubtype = "escalate"
	CommitmentDirectiveCast     CommitmentDirectiveSubtype = "cast"
	CommitmentDirectiveRetract  CommitmentDirectiveSubtype = "retract"
)

// CommitmentDirective is the typed operational-act body: a directive's full
// content lives on the record (never the retired envelope). Body carries the
// note/escalate text; Objective the cast objective; TargetRef the retract
// target. Subtype is required.
type CommitmentDirective struct {
	Subtype   CommitmentDirectiveSubtype `json:"subtype"`
	Body      string                     `json:"body,omitempty"`
	Objective string                     `json:"objective,omitempty"`
	TargetRef string                     `json:"target_ref,omitempty"`
}

// TypedQuestion is one checkable clause of a prediction: a prompt plus the
// committing agent's stated probability distribution over its answers.
type TypedQuestion struct {
	Question      string             `json:"question"`
	Answers       []string           `json:"answers,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	// Weight is the materiality weight of this clause (likelihood x impact x
	// relevance on the governance scale); omission is penalized by weight.
	Weight float64 `json:"weight,omitempty"`
}

// CommitmentPrecommit is the frozen, machine-scoreable prediction staged by
// choir.Precommit. Distribution maps each possible answer to the probability
// committed before evidence arrives; Resolver names the desk that may resolve
// it. The objectgraph body deliberately accepts future JSON fields so readers
// from either side of an additive rollout can retain the record.
type CommitmentPrecommit struct {
	Question     string             `json:"question"`
	Distribution map[string]float64 `json:"distribution"`
	Resolver     string             `json:"resolver"`
}

// CommitmentResolve is the evidence-bearing result staged by choir.Resolve.
// It closes the linked commitment append-only; no score belongs to a resolver
// verdict.
type CommitmentResolve struct {
	Verdict      string   `json:"verdict"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// CommitmentDisagreement preserves a scorer's differing verdict separately
// from the resolver's result. It is supervision signal, not acting context.
type CommitmentDisagreement struct {
	CommitmentID    string   `json:"commitment_id"`
	ScorerVerdict   string   `json:"scorer_verdict"`
	ResolverVerdict string   `json:"resolver_verdict"`
	EvidenceRefs    []string `json:"evidence_refs,omitempty"`
}

// CommitmentPrediction is the required frozen prediction: the hypothesis,
// the assumptions it rests on, competing alternatives, and typed questions
// with the agent's probabilities — all frozen at commit time.
type CommitmentPrediction struct {
	Hypothesis   string          `json:"hypothesis"`
	Assumptions  []string        `json:"assumptions,omitempty"`
	Alternatives []string        `json:"alternatives,omitempty"`
	Questions    []TypedQuestion `json:"questions,omitempty"`
	// CommittedAt is when the prediction froze; MUST precede Observation.
	CommittedAt string `json:"committed_at"`
}

// CommitmentObservation is the resolving evidence: a compact excerpt plus a
// reference to the full source.
type CommitmentObservation struct {
	Excerpt string `json:"excerpt"`
	// SourceRef names the full source — a channel-message canonical id, an
	// event id, or an evidence node the body resolves against.
	SourceRef  string `json:"source_ref"`
	ObservedAt string `json:"observed_at"`
}

// CommitmentScore is one scorer's verdict: its answers to the typed
// questions, the scorer's model identity, and whether it disagrees with a
// sibling scorer. Multiple scores on one record are preserved; disagreement
// is signal, not collapsed.
type CommitmentScore struct {
	ScorerModelID string             `json:"scorer_model_id"`
	Answers       map[string]string  `json:"answers,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	// Disagreement flags a scorer verdict that conflicts with another score
	// on the same record.
	Disagreement bool   `json:"disagreement,omitempty"`
	ScoredAt     string `json:"scored_at"`
}

// CommitmentProvenance binds the record to who committed it and when:
// the agent, its model, the context the prediction was made under, and the
// commit/resolve timestamps. Required.
type CommitmentProvenance struct {
	AgentID     string `json:"agent_id"`
	ModelID     string `json:"model_id"`
	ContextRef  string `json:"context_ref,omitempty"`
	CommittedAt string `json:"committed_at"`
	ResolvedAt  string `json:"resolved_at,omitempty"`
}

// CommitmentRevision is the optional post-score model update: which
// assumptions weakened or strengthened and the revised hypothesis. Written
// only when the score flags a surprise.
type CommitmentRevision struct {
	Weakened     []string `json:"weakened,omitempty"`
	Strengthened []string `json:"strengthened,omitempty"`
	Hypothesis   string   `json:"hypothesis,omitempty"`
	RevisedAt    string   `json:"revised_at,omitempty"`
}

// CommitmentAction records the intended action and declared capabilities for
// a material action, so procedural fidelity can check declared vs used.
type CommitmentAction struct {
	Intent               string   `json:"intent"`
	DeclaredCapabilities []string `json:"declared_capabilities,omitempty"`
}

// CommitmentConsequence is one predicted/observed side effect. Prediction
// and observation are paired so omission and overclaim are both measurable.
type CommitmentConsequence struct {
	Label     string  `json:"label"`
	Predicted bool    `json:"predicted"`
	Observed  bool    `json:"observed"`
	Weight    float64 `json:"weight,omitempty"`
	Detail    string  `json:"detail,omitempty"`
}

// CommitmentRecord is the full precommitment record body persisted on the
// object graph. Five fields are required (Prediction, Observation, Scores,
// Discrepancy, Provenance); the rest are optional.
type CommitmentRecord struct {
	// SchemaID pins the body schema version for rescore-forward-compat.
	SchemaID string `json:"schema_id"`
	// RecordID is the commitment's identity; it is also the OG object's
	// canonical-id suffix.
	RecordID string `json:"record_id"`

	Prediction   CommitmentPrediction    `json:"prediction"`
	Observation  CommitmentObservation   `json:"observation"`
	Scores       []CommitmentScore       `json:"scores"`
	Discrepancy  DiscrepancyClass        `json:"discrepancy"`
	Provenance   CommitmentProvenance    `json:"provenance"`
	Revision     CommitmentRevision      `json:"revision,omitempty"`
	Action       *CommitmentAction       `json:"action,omitempty"`
	Consequences []CommitmentConsequence `json:"consequences,omitempty"`

	// Precommit, Resolve, and Disagreement are the additive typed semantic-act
	// schema on choir.commitment_record. Older bodies have these nil and retain
	// their Prediction.Hypothesis string unchanged. A typed Resolve may target
	// that legacy record, but it remains unscoreable because no distribution is
	// inferred or rewritten from free text.
	Precommit    *CommitmentPrecommit    `json:"precommit,omitempty"`
	Resolve      *CommitmentResolve      `json:"resolve,omitempty"`
	Disagreement *CommitmentDisagreement `json:"disagreement,omitempty"`

	// Kind is the explicit record taxonomy (record-native cutover); it
	// duplicates what the typed sub-objects imply so routing and projection
	// classify without decoding intent names out of the RecordID string.
	Kind CommitmentRecordKind `json:"kind,omitempty"`
	// Directive is the typed operational-act body for Kind==directive
	// (note/escalate/cast/retract). Directives are operational records —
	// excluded from claim accrual and materiality projection.
	Directive *CommitmentDirective `json:"directive,omitempty"`

	// Addressee is the desk/actor this act is addressed to (StagedIntent
	// ToDesk, falling back to ResolverID). It is the ledger-side analogue of
	// the packet envelope's target_agent_id for pending/evidence queries —
	// typed here so a ledger reader can scope records to the desk without a
	// packet envelope. Proposed vocabulary for R5a to ratify/rename.
	Addressee string `json:"addressee,omitempty"`
	// EvidenceRefs are the typed evidence/source references the act carries
	// (StagedIntent EvidenceRefs + ExecutionRefs, plus packet source URIs for
	// packet-bodied reports). Each ref is the raw typed/URL/execution string
	// the evidence resolver materializes into a source entity — kept verbatim
	// (not the packet's richer Source structure) so the record stays additive
	// and never becomes a packet-envelope duplicate.
	EvidenceRefs []string `json:"evidence_refs,omitempty"`

	// ParentID / ChildIDs carry nested-commitment provenance edges;
	// RelatedIDs links records; RetrievedByIDs records later decisions that
	// pulled this record into context.
	ParentID       string   `json:"parent_id,omitempty"`
	ChildIDs       []string `json:"child_ids,omitempty"`
	RelatedIDs     []string `json:"related_ids,omitempty"`
	RetrievedByIDs []string `json:"retrieved_by_ids,omitempty"`
}

// CommitmentRecordSchemaV1 is the schema version stamped into SchemaID.
const CommitmentRecordSchemaV1 = "commitment_record.v1"

// RecordKind returns the record's taxonomy. Explicit Kind wins; legacy
// bodies classify by typed sub-object (disagreement/resolve/precommit) and
// finally by the legacy RecordID kind segment — minted IDs embed
// cell:kind:local, so the second-to-last colon segment is the legacy kind.
// Unknown/empty bodies classify as report only as a readable default for
// the unmarked legacy claim records minted before the taxonomy existed.
func (rec CommitmentRecord) RecordKind() CommitmentRecordKind {
	if rec.Kind != "" {
		return rec.Kind
	}
	switch {
	case rec.Disagreement != nil:
		return CommitmentKindDisagreement
	case rec.Resolve != nil:
		return CommitmentKindResolve
	case rec.Precommit != nil:
		return CommitmentKindPrecommit
	case rec.Directive != nil:
		return CommitmentKindDirective
	}
	parts := strings.Split(rec.RecordID, ":")
	if len(parts) >= 2 {
		switch parts[len(parts)-2] {
		case "note", "escalate", "cast", "cancel", "retract":
			return CommitmentKindDirective
		case "resolve":
			return CommitmentKindResolve
		case "disagreement":
			return CommitmentKindDisagreement
		case "precommit", "ask":
			return CommitmentKindPrecommit
		}
	}
	return CommitmentKindReport
}

// IsDirectiveRecord reports whether the record is an operational act —
// excluded from claim accrual and materiality projection. Directives carry
// no scored claim; accruing them as open claims was the projection-pollution
// defect the explicit taxonomy repairs.
func (rec CommitmentRecord) IsDirectiveRecord() bool {
	return rec.RecordKind() == CommitmentKindDirective
}

// IsValidDirectiveSubtype reports whether the subtype is one of the closed
// routing enum members.
func IsValidDirectiveSubtype(subtype CommitmentDirectiveSubtype) bool {
	switch subtype {
	case CommitmentDirectiveNote, CommitmentDirectiveEscalate,
		CommitmentDirectiveCast, CommitmentDirectiveRetract:
		return true
	}
	return false
}
