package types

import (
	"strconv"
	"strings"
	"time"
)

// Commitment records (precommitment records): the durable unit of the
// commitment ledger. A record is a typed prediction committed before the
// evidence that resolves it, then scored on a verifiable outcome. The
// accumulated log is the agent's world model and supervision surface.
// Source: docs/precommitment-records.md ("Record schema").
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
	// DiscrepancyAnswered: a desk supplied the requested answer. It is an
	// arrival-class resolution, not a scored material verdict.
	DiscrepancyAnswered DiscrepancyClass = "answered"
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
	// Rule, when present, makes the precommit rule-settled: the runtime
	// answers it from receipts (commitment_rules.go). Engineering's
	// precommits carry one; older bodies have none.
	Rule *PrecommitRule `json:"rule,omitempty"`
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
	ScorerModelID string `json:"scorer_model_id"`
	// ScorerAgentID names the scorer ("scorer:<name>" for an independent
	// calibration scorer). Self marks the speaker's own score, written
	// with the act before any settlement; its model is the record's
	// provenance model (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md).
	ScorerAgentID string             `json:"scorer_agent_id,omitempty"`
	Self          bool               `json:"self,omitempty"`
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
	// ActingTask and ActingRunID name the management task and its run when
	// a task authored a record management sends (management delegation
	// R2-1, §4.2: the delegated sender). The host derives both from the
	// run; the store requires them on every record a task scope commits.
	ActingTask  string `json:"acting_task,omitempty"`
	ActingRunID string `json:"acting_run_id,omitempty"`
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

	// EmitterKind, ComputerID and SubjectRefs are reserved for records that
	// will be scored across computers (docs/precommitment-records.md section 4.6):
	// who spoke (an agent, the owner, an edition or an outside
	// source), on which computer, and what the record is about. Topic
	// scopes are derived from SubjectRefs when reputation is queried.
	EmitterKind CommitmentEmitterKind `json:"emitter_kind,omitempty"`
	ComputerID  string                `json:"computer_id,omitempty"`
	SubjectRefs map[string]string     `json:"subject_refs,omitempty"`

	// Inbox addressing (Cut 1, definitions/choir-reeng-cut1-authority
	// "Inboxes and sequence"). Inbox is the addressee inbox
	// ("texture:<doc>", "research:<doc>", "management:<owner>",
	// "engineering:<doc>") and SenderInbox the inbox the sender reads. The
	// store stamps InboxSeq (next number in Inbox), ConversationID (from the
	// unordered inbox pair) and Round (the open round, 0 outside any round)
	// at commit; a caller leaves them zero, or names the round it answers.
	Inbox          string `json:"inbox,omitempty"`
	SenderInbox    string `json:"sender_inbox,omitempty"`
	InboxSeq       int64  `json:"inbox_seq,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Round          int    `json:"round,omitempty"`
	// Final marks a Report that closes its round. Only the round's
	// recipient (or a system closure) may send one.
	Final bool `json:"final,omitempty"`
	// Cast and Report are the typed work bodies (decisions §2-§3): a Cast
	// opens, revises and withdraws hypotheses; a Report tags findings by
	// hypothesis id and settles or drops them. The schema may still change
	// until Cut 2's freeze.
	Cast   *CommitmentCast   `json:"cast,omitempty"`
	Report *CommitmentReport `json:"report,omitempty"`
	// Support is the body of the support-judgement records (Cut 2 2a (v)):
	// a Texture commit's list of pairs to management, or management's
	// failing verdicts to the Texture.
	Support *CommitmentSupport `json:"support,omitempty"`
	// Captures is the body of a capture arrival record (Cut 2 2b): the
	// system record a source poll that wrote new captures sends
	// management:<owner>, naming them.
	Captures *CommitmentCaptures `json:"captures,omitempty"`
	// Disposition is the structured body of management's choir.Feed and
	// choir.Decline Notes (L2a R2-5): which captures it fed into the story
	// (feed, to texture:<doc>) or declined (decline, to source:<id>), and
	// why.
	Disposition *CommitmentDisposition `json:"disposition,omitempty"`
	// TaskOutcome is the body of a management task's final Report
	// (management delegation R2-8), minted by the host from the task inbox
	// to management:<owner>.
	TaskOutcome *CommitmentTaskOutcome `json:"task_outcome,omitempty"`
	// Job is the body of a capsule job's end record
	// (docs/design/capsule-long-exec-2026-10-15.md): the system record the
	// runtime sends engineering:<assignment> from job:<id> when a job it
	// started ends.
	Job *CommitmentJob `json:"job,omitempty"`
	// Calibration is the body of an agent calibration record (level 2,
	// docs/archive/reports/choir-mcp-precommitments-2026-10-14.md): a claim an agent
	// wrote through /api/precommit (Kind precommit) or one scorer's
	// settlement of it (Kind resolve). Desk packs exclude these records in
	// their query.
	Calibration *CommitmentCalibration `json:"calibration,omitempty"`
}

// CastHypothesis is one hypothesis a Cast opens. The store assigns ID
// (H1, H2, ... per conversation); a caller leaves it empty. A general
// hypothesis (decisions §3.1) names its seed in General and the
// operational meaning of its key word in Meaning.
//
// Kind says how it settles (L3-1, docs/precommitment-records.md §5.5): an
// assertion by later corroboration, a forecast by an event on or before
// ResolvesBy (RFC 3339, at most HypothesisForecastHorizon after the
// commit). Criterion is the check that settles either kind. A hypothesis
// without Kind is legacy: never backfilled, outside the forecast cohort.
// The three fields are a compatible extension of the Cut 2 freeze
// (docs/reports/reeng-cut2-freeze-receipt-2026-10-13.md).
type CastHypothesis struct {
	ID          string  `json:"id,omitempty"`
	Statement   string  `json:"statement"`
	Probability float64 `json:"probability"`
	Stakes      string  `json:"stakes,omitempty"`
	Crux        string  `json:"crux,omitempty"`
	General     string  `json:"general,omitempty"`
	Meaning     string  `json:"meaning,omitempty"`
	Kind        string  `json:"kind,omitempty"`
	Criterion   string  `json:"criterion,omitempty"`
	ResolvesBy  string  `json:"resolves_by,omitempty"`
}

// Hypothesis kinds (CastHypothesis.Kind), the vocabulary of
// CalibrationClaimAssertion and CalibrationClaimForecast.
const (
	HypothesisKindAssertion = "assertion"
	HypothesisKindForecast  = "forecast"
)

// HypothesisForecastHorizon bounds a forecast's ResolvesBy after its
// commit.
const HypothesisForecastHorizon = 7 * 24 * time.Hour

// CastUncertainty is an open question with no hypothesis yet, weighted by
// how much it matters.
type CastUncertainty struct {
	Question string `json:"question"`
	Stakes   string `json:"stakes,omitempty"`
}

// CastRevision revises an open hypothesis the same sender opened. Empty
// fields and a nil Probability leave the prior value.
type CastRevision struct {
	ID          string   `json:"id"`
	Statement   string   `json:"statement,omitempty"`
	Probability *float64 `json:"probability,omitempty"`
	Stakes      string   `json:"stakes,omitempty"`
	Crux        string   `json:"crux,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}

// CastWithdrawal withdraws an open hypothesis the same sender opened.
type CastWithdrawal struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

// CommitmentCast is the typed body of a cast directive. The objective
// stays on Directive.Objective.
//
// Captures and Publish are the opening-Cast fields a management Cast to a
// Texture may carry (Cut 2 pre-flight review, A6'): with Hypotheses they are
// everything a story is opened with, and they enter the freeze receipt.
// Captures lists capture ids Texture may cite at once (its frame shows them
// as the Cast record's sources); Publish is "" or CastPublishAuto, which
// publishes every committed revision of the document (2b). Texture reads
// them in its frame and never writes them.
type CommitmentCast struct {
	Hypotheses    []CastHypothesis  `json:"hypotheses,omitempty"`
	Uncertainties []CastUncertainty `json:"uncertainties,omitempty"`
	Revisions     []CastRevision    `json:"revisions,omitempty"`
	Withdrawals   []CastWithdrawal  `json:"withdrawals,omitempty"`
	Captures      []string          `json:"captures,omitempty"`
	Publish       string            `json:"publish,omitempty"`
}

// CastPublishAuto is the one publish mode an opening Cast may set.
const CastPublishAuto = "auto"

// ReportClaim is one claim a Report makes, with the sources it rests on.
type ReportClaim struct {
	Text    string   `json:"text"`
	Sources []string `json:"sources,omitempty"`
}

// ReportFinding bears on one or more hypothesis ids, or lies outside every
// listed hypothesis; exactly one of the two.
type ReportFinding struct {
	Text    string   `json:"text"`
	BearsOn []string `json:"bears_on,omitempty"`
	Outside bool     `json:"outside,omitempty"`
}

// HypothesisSettlement settles an open hypothesis with an outcome.
type HypothesisSettlement struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
}

// HypothesisDrop drops an open hypothesis with a reason.
type HypothesisDrop struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// CommitmentReport is the typed Report body that replaces raw packet JSON
// in Prediction.Hypothesis. Whether it closes its round is the record's
// Final flag; StopReason says why the recipient stopped and is required on
// a final Report.
type CommitmentReport struct {
	Claims     []ReportClaim          `json:"claims,omitempty"`
	Findings   []ReportFinding        `json:"findings,omitempty"`
	Settled    []HypothesisSettlement `json:"settled,omitempty"`
	Dropped    []HypothesisDrop       `json:"dropped,omitempty"`
	StopReason string                 `json:"stop_reason,omitempty"`
}

// CommitmentEmitterKind names who emitted a record. An agent is a desk on a
// computer; the owner is the person behind the computer, whose acts are
// not agent speech; an edition is a computer publishing claims; a source is
// an outside outlet whose claims are observations, never precommitments.
type CommitmentEmitterKind string

const (
	CommitmentEmitterAgent   CommitmentEmitterKind = "agent"
	CommitmentEmitterOwner   CommitmentEmitterKind = "owner"
	CommitmentEmitterEdition CommitmentEmitterKind = "edition"
	CommitmentEmitterSource  CommitmentEmitterKind = "source"
	// CommitmentEmitterSystem is the store or boot speaking for no desk: a
	// round closure after a crash. System records never wake anyone.
	CommitmentEmitterSystem CommitmentEmitterKind = "system"
)

// Round closure reasons and their record-id slugs (D5).
const (
	ClosureReasonInterruptedByRestart = "interrupted_by_restart"
	closureSystemAgentID              = "system:closure"
)

// IsCast reports whether the record is a cast directive.
func (rec CommitmentRecord) IsCast() bool {
	return rec.RecordKind() == CommitmentKindDirective && rec.Directive != nil &&
		rec.Directive.Subtype == CommitmentDirectiveCast
}

// RoundClosureRecordID is the deterministic closure id
// "<conversation_id>:r<round>:<reason-slug>"; interrupted_by_restart
// slugs to "restart".
func RoundClosureRecordID(conversationID string, round int, reason string) string {
	slug := "restart"
	if reason != ClosureReasonInterruptedByRestart {
		slug = strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				return r
			case r >= 'A' && r <= 'Z':
				return r + ('a' - 'A')
			}
			return '-'
		}, strings.TrimSpace(reason))
	}
	return strings.TrimSpace(conversationID) + ":r" + strconv.Itoa(round) + ":" + slug
}

// NewRoundClosureRecord builds the system Report that closes an open round:
// final, sent on behalf of the round's recipient and addressed to its
// sender (the caster), with the deterministic id above. The store refuses
// it unless that round is the conversation's open round.
func NewRoundClosureRecord(casterInbox, recipientInbox string, round int, reason, committedAt string) CommitmentRecord {
	conversationID := InboxConversationID(casterInbox, recipientInbox)
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    RoundClosureRecordID(conversationID, round, reason),
		Kind:        CommitmentKindReport,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: closureSystemAgentID, CommittedAt: committedAt},
		Observation: CommitmentObservation{Excerpt: reason, ObservedAt: committedAt},
		EmitterKind: CommitmentEmitterSystem,
		Inbox:       strings.TrimSpace(casterInbox), SenderInbox: strings.TrimSpace(recipientInbox),
		ConversationID: conversationID, Round: round, Final: true,
		Report: &CommitmentReport{StopReason: reason},
	}
}

// InboxConversationID derives the conversation id from the two inboxes.
// The pair is unordered so a Cast and the Reports answering it share one
// conversation; who cast a round is on the conversation head.
func InboxConversationID(a, b string) string {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if b < a {
		a, b = b, a
	}
	return "conv:" + a + "|" + b
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
