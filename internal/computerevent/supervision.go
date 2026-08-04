package computerevent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const (
	SupervisionSchemaV1       = "choir.supervision_transaction.v1"
	SupervisionReducerV1      = "supervision/v1"
	SupervisionDigestRecipeV1 = "sha256:choir.canonical-json/v1:logical-private-artifacts-without-command_digest-and-transaction_id"
)

type SupervisionActor struct {
	ActorID      string `json:"actor_id"`
	Role         string `json:"role"`
	AuthorityRef string `json:"authority_ref"`
}

type SupervisionExpected struct {
	CanonicalEventHead     *string `json:"canonical_event_head"`
	LifecycleVersion       *uint64 `json:"lifecycle_version"`
	IntentRevisionID       *string `json:"intent_revision_id"`
	ArtifactHeadRevisionID *string `json:"artifact_head_revision_id"`
	SettlementProposalID   *string `json:"settlement_proposal_id"`
}

type SupervisionObservedBase struct {
	CanonicalEventHead     string `json:"canonical_event_head"`
	IntentRevisionID       string `json:"intent_revision_id"`
	ArtifactHeadRevisionID string `json:"artifact_head_revision_id"`
}

type SupervisionMutation struct {
	Kind string          `json:"kind"`
	Body json.RawMessage `json:"body"`
}

// ReferencedArtifact binds a mutation-named artifact to its authenticated pin
// receipt. The appender independently verifies the receipt before the
// transaction may reach the canonical tape.
type ReferencedArtifact struct {
	Ref                    string  `json:"ref"`
	ArtifactDigest         string  `json:"artifact_digest"`
	PlaintextDigest        string  `json:"plaintext_digest"`
	LogicalPlaintextDigest string  `json:"logical_plaintext_digest,omitempty"`
	MediaType              string  `json:"media_type"`
	BindingID              string  `json:"binding_id"`
	PinReceipt             Receipt `json:"pin_receipt"`
}

type SupervisionTransaction struct {
	Schema              string                   `json:"schema"`
	Reducer             string                   `json:"reducer"`
	DigestRecipe        string                   `json:"digest_recipe"`
	TransactionID       string                   `json:"transaction_id"`
	TransactionClass    string                   `json:"transaction_class"`
	OwnerID             string                   `json:"owner_id"`
	ComputerID          string                   `json:"computer_id"`
	TrajectoryID        string                   `json:"trajectory_id"`
	CommandID           string                   `json:"command_id"`
	CommandDigest       string                   `json:"command_digest"`
	Actor               SupervisionActor         `json:"actor"`
	Expected            SupervisionExpected      `json:"expected"`
	ObservedBase        *SupervisionObservedBase `json:"observed_base"`
	Mutations           []SupervisionMutation    `json:"mutations"`
	ReferencedArtifacts []ReferencedArtifact     `json:"referenced_artifacts"`
}

type supervisionTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type affectedTarget struct {
	Kind                  string `json:"kind"`
	ID                    string `json:"id"`
	PriorIntentRevisionID string `json:"prior_intent_revision_id"`
	StateDigest           string `json:"state_digest"`
	RebaseObligationID    string `json:"rebase_obligation_id"`
}

type projectionImportedBody struct {
	ImportRef                     string          `json:"import_ref"`
	ImportDigest                  string          `json:"import_digest"`
	ImportArtifactPlaintextDigest string          `json:"import_artifact_plaintext_digest"`
	SourceDoltCommit              string          `json:"source_dolt_commit"`
	SourceProjectionDigest        string          `json:"source_projection_digest"`
	LegacyLifecycleWatermark      uint64          `json:"legacy_lifecycle_watermark"`
	ObjectCount                   uint64          `json:"object_count"`
	EdgeCount                     uint64          `json:"edge_count"`
	RefusalCount                  uint64          `json:"refusal_count"`
	QuiescenceReceiptRef          string          `json:"quiescence_receipt_ref"`
	DrainReceiptRefs              []string        `json:"drain_receipt_refs"`
	Manifest                      json.RawMessage `json:"manifest"`
}
type trajectoryStartedBody struct {
	TrajectoryKind       string            `json:"trajectory_kind"`
	SubjectRefs          map[string]string `json:"subject_refs"`
	IntentRevisionID     string            `json:"intent_revision_id"`
	ArtifactID           string            `json:"artifact_id"`
	ArtifactRevisionID   string            `json:"artifact_revision_id"`
	TextureActorID       string            `json:"texture_actor_id"`
	InitialAssignmentIDs []string          `json:"initial_assignment_ids"`
	Objective            string            `json:"objective"`
}
type intentRevisedBody struct {
	IntentRevisionID       string           `json:"intent_revision_id"`
	ParentIntentRevisionID *string          `json:"parent_intent_revision_id"`
	Intent                 string           `json:"intent"`
	Material               bool             `json:"material"`
	AffectedTargets        []affectedTarget `json:"affected_targets"`
}
type textureRevisionBody struct {
	ArtifactID               string          `json:"artifact_id"`
	RevisionID               string          `json:"revision_id"`
	Title                    string          `json:"title"`
	ParentRevisionID         *string         `json:"parent_revision_id"`
	Content                  string          `json:"content"`
	SourceGraph              json.RawMessage `json:"source_graph"`
	Metadata                 json.RawMessage `json:"metadata"`
	MetadataDigest           string          `json:"metadata_digest"`
	NarrativeKind            string          `json:"narrative_kind"`
	FulfillsIntentRevisionID string          `json:"fulfills_intent_revision_id"`
}
type actorMessageBody struct {
	MessageID          string  `json:"message_id"`
	FromActorID        string  `json:"from_actor_id"`
	ToRole             string  `json:"to_role"`
	ToActorID          *string `json:"to_actor_id"`
	ChannelID          string  `json:"channel_id"`
	PayloadArtifactRef string  `json:"payload_artifact_ref"`
	Material           bool    `json:"material"`
}
type researcherPacketBody struct {
	PacketID               string   `json:"packet_id"`
	ResearcherID           string   `json:"researcher_id"`
	ObligationID           string   `json:"obligation_id"`
	PacketArtifactRef      string   `json:"packet_artifact_ref"`
	SourceArtifactRefs     []string `json:"source_artifact_refs"`
	UncertaintyArtifactRef string   `json:"uncertainty_artifact_ref"`
	ConflictRefs           []string `json:"conflict_refs"`
}
type assignmentOpenedBody struct {
	AssignmentID          string                  `json:"assignment_id"`
	AssignedActorID       string                  `json:"assigned_actor_id"`
	AssignedRole          string                  `json:"assigned_role"`
	ParentDecisionID      string                  `json:"parent_decision_id"`
	IntentRevisionID      string                  `json:"intent_revision_id"`
	ObservedBase          SupervisionObservedBase `json:"observed_base"`
	ScopeDigest           string                  `json:"scope_digest"`
	CapabilityDigest      string                  `json:"capability_digest"`
	PolicyDigest          string                  `json:"policy_digest"`
	ObligationIDs         []string                `json:"obligation_ids"`
	IdempotencyCommitment string                  `json:"idempotency_commitment"`
}
type attemptStartedBody struct {
	AssignmentID      string                  `json:"assignment_id"`
	AttemptID         string                  `json:"attempt_id"`
	AttemptKind       string                  `json:"attempt_kind"`
	Ordinal           uint64                  `json:"ordinal"`
	PriorAttemptID    *string                 `json:"prior_attempt_id"`
	RunID             string                  `json:"run_id"`
	ObservedBase      SupervisionObservedBase `json:"observed_base"`
	RuntimeReceiptRef string                  `json:"runtime_receipt_ref"`
}
type attemptResultBody struct {
	AssignmentID               string                  `json:"assignment_id"`
	AttemptID                  string                  `json:"attempt_id"`
	ResultID                   string                  `json:"result_id"`
	Outcome                    string                  `json:"outcome"`
	ResultArtifactRef          string                  `json:"result_artifact_ref"`
	EvidenceRefs               []string                `json:"evidence_refs"`
	ObservedBase               SupervisionObservedBase `json:"observed_base"`
	DeliveredAfterCancellation bool                    `json:"delivered_after_cancellation"`
}
type superBeliefBody struct {
	BeliefID               string   `json:"belief_id"`
	SupersedesBeliefID     *string  `json:"supersedes_belief_id"`
	BeliefArtifactRef      string   `json:"belief_artifact_ref"`
	UncertaintyArtifactRef string   `json:"uncertainty_artifact_ref"`
	EvidenceRefs           []string `json:"evidence_refs"`
}
type superFindingBody struct {
	FindingID                   string            `json:"finding_id"`
	Fingerprint                 string            `json:"fingerprint"`
	Invariant                   string            `json:"invariant"`
	Subject                     supervisionTarget `json:"subject"`
	Severity                    string            `json:"severity"`
	State                       string            `json:"state"`
	EvidenceRefs                []string          `json:"evidence_refs"`
	ExpectedResponseArtifactRef string            `json:"expected_response_artifact_ref"`
}
type dissentBody struct {
	DissentID         string            `json:"dissent_id"`
	Subject           supervisionTarget `json:"subject"`
	StanceArtifactRef string            `json:"stance_artifact_ref"`
	EvidenceRefs      []string          `json:"evidence_refs"`
}
type reconciliationBody struct {
	ReconciliationID   string   `json:"reconciliation_id"`
	FindingIDs         []string `json:"finding_ids"`
	DissentIDs         []string `json:"dissent_ids"`
	AssignmentIDs      []string `json:"assignment_ids"`
	ObligationIDs      []string `json:"obligation_ids"`
	DispositionIDs     []string `json:"disposition_ids"`
	SummaryArtifactRef string   `json:"summary_artifact_ref"`
}
type superDecisionBody struct {
	DecisionID          string   `json:"decision_id"`
	OptionsArtifactRef  string   `json:"options_artifact_ref"`
	SelectedOptionID    string   `json:"selected_option_id"`
	ProposalArtifactRef string   `json:"proposal_artifact_ref"`
	EvidenceRefs        []string `json:"evidence_refs"`
	DissentIDs          []string `json:"dissent_ids"`
	ReservedAuthority   string   `json:"reserved_authority"`
}
type ownerDecisionBody struct {
	DecisionID               string  `json:"decision_id"`
	ProposalID               string  `json:"proposal_id"`
	OwnerActorID             string  `json:"owner_actor_id"`
	DecisionArtifactRef      string  `json:"decision_artifact_ref"`
	ScopeDigest              string  `json:"scope_digest"`
	Decision                 *string `json:"decision,omitempty"`
	SettlementSnapshotDigest *string `json:"settlement_snapshot_digest,omitempty"`
}
type assignmentCancelledBody struct {
	AssignmentID      string   `json:"assignment_id"`
	ReasonArtifactRef string   `json:"reason_artifact_ref"`
	ActiveAttemptIDs  []string `json:"active_attempt_ids"`
}
type dispositionBody struct {
	DispositionID            string            `json:"disposition_id"`
	Target                   supervisionTarget `json:"target"`
	PriorDispositionID       *string           `json:"prior_disposition_id"`
	Value                    string            `json:"value"`
	RationaleArtifactRef     string            `json:"rationale_artifact_ref"`
	EvidenceRefs             []string          `json:"evidence_refs"`
	CompensationObligationID *string           `json:"compensation_obligation_id"`
}
type settlementProposedBody struct {
	ProposalID                string   `json:"proposal_id"`
	CanonicalEventHead        string   `json:"canonical_event_head"`
	IntentRevisionID          string   `json:"intent_revision_id"`
	ArtifactHeadRevisionID    string   `json:"artifact_head_revision_id"`
	AssignmentIDs             []string `json:"assignment_ids"`
	AttemptIDs                []string `json:"attempt_ids"`
	ResultIDs                 []string `json:"result_ids"`
	UpdateIDs                 []string `json:"update_ids"`
	DispositionIDs            []string `json:"disposition_ids"`
	FindingIDs                []string `json:"finding_ids"`
	DissentIDs                []string `json:"dissent_ids"`
	RebaseObligationIDs       []string `json:"rebase_obligation_ids"`
	CompensationObligationIDs []string `json:"compensation_obligation_ids"`
	EvidenceRefs              []string `json:"evidence_refs"`
	OwnerAttentionIDs         []string `json:"owner_attention_ids"`
	SnapshotDigest            string   `json:"snapshot_digest"`
}
type trajectorySettledBody struct {
	SettlementID          string `json:"settlement_id"`
	ProposalID            string `json:"proposal_id"`
	OwnerDecisionID       string `json:"owner_decision_id"`
	SettlementArtifactRef string `json:"settlement_artifact_ref"`
	SnapshotDigest        string `json:"snapshot_digest"`
}
type artifactArchivedBody struct {
	ArtifactID        string `json:"artifact_id"`
	HeadRevisionID    string `json:"head_revision_id"`
	ReasonArtifactRef string `json:"reason_artifact_ref"`
}

var supervisionBodyTypes = map[string]reflect.Type{
	"projection_imported": reflect.TypeFor[projectionImportedBody](), "trajectory_started": reflect.TypeFor[trajectoryStartedBody](),
	"intent_revised": reflect.TypeFor[intentRevisedBody](), "texture_revision": reflect.TypeFor[textureRevisionBody](),
	"actor_message_recorded": reflect.TypeFor[actorMessageBody](), "researcher_packet_recorded": reflect.TypeFor[researcherPacketBody](),
	"assignment_opened": reflect.TypeFor[assignmentOpenedBody](), "attempt_started": reflect.TypeFor[attemptStartedBody](),
	"attempt_result": reflect.TypeFor[attemptResultBody](), "super_belief_recorded": reflect.TypeFor[superBeliefBody](),
	"super_finding_recorded": reflect.TypeFor[superFindingBody](), "dissent_recorded": reflect.TypeFor[dissentBody](),
	"super_reconciliation_recorded": reflect.TypeFor[reconciliationBody](), "super_decision_proposed": reflect.TypeFor[superDecisionBody](),
	"owner_decision_recorded": reflect.TypeFor[ownerDecisionBody](), "assignment_cancelled": reflect.TypeFor[assignmentCancelledBody](),
	"disposition_recorded": reflect.TypeFor[dispositionBody](), "settlement_proposed": reflect.TypeFor[settlementProposedBody](),
	"trajectory_settled": reflect.TypeFor[trajectorySettledBody](), "artifact_archived": reflect.TypeFor[artifactArchivedBody](),
}

var supervisionClassRules = map[string]struct {
	roles, kinds []string
	single       bool
	required     map[string]int
}{
	"projection_import":     {[]string{"runtime"}, []string{"projection_imported"}, true, nil},
	"open_trajectory":       {[]string{"texture", "owner"}, []string{"trajectory_started", "intent_revised", "texture_revision"}, false, map[string]int{"trajectory_started": 1, "intent_revised": 1, "texture_revision": 1}},
	"revise_intent":         {[]string{"texture", "owner"}, []string{"intent_revised", "disposition_recorded"}, false, map[string]int{"intent_revised": 1}},
	"revise_artifact":       {[]string{"texture", "owner"}, []string{"texture_revision"}, true, nil},
	"record_message":        {[]string{"texture", "super"}, []string{"actor_message_recorded"}, true, nil},
	"record_research":       {[]string{"researcher"}, []string{"researcher_packet_recorded"}, true, nil},
	"open_assignment":       {[]string{"super"}, []string{"assignment_opened"}, false, nil},
	"start_attempt":         {[]string{"super", "runtime"}, []string{"attempt_started"}, true, nil},
	"return_result":         {[]string{"cosuper"}, []string{"attempt_result"}, true, nil},
	"record_belief":         {[]string{"super"}, []string{"super_belief_recorded"}, true, nil},
	"record_finding":        {[]string{"super"}, []string{"super_finding_recorded"}, true, nil},
	"record_dissent":        {[]string{"super", "researcher", "cosuper", "verifier"}, []string{"dissent_recorded"}, true, nil},
	"record_reconciliation": {[]string{"super"}, []string{"super_reconciliation_recorded"}, true, nil},
	"propose_decision":      {[]string{"super"}, []string{"super_decision_proposed"}, true, nil},
	"record_owner_decision": {[]string{"owner"}, []string{"owner_decision_recorded"}, true, nil},
	"cancel_assignment":     {[]string{"super"}, []string{"assignment_cancelled"}, true, nil},
	"record_disposition":    {[]string{"super"}, []string{"disposition_recorded"}, false, nil},
	"propose_settlement":    {[]string{"super"}, []string{"settlement_proposed"}, true, nil},
	"settle_trajectory":     {[]string{"owner"}, []string{"trajectory_settled"}, true, nil},
	"archive_artifact":      {[]string{"texture", "owner"}, []string{"artifact_archived"}, true, nil},
}

func (t SupervisionTransaction) Validate() error {
	if t.Schema != SupervisionSchemaV1 || t.Reducer != SupervisionReducerV1 || t.DigestRecipe != SupervisionDigestRecipeV1 {
		return fmt.Errorf("supervision transaction: unsupported schema, reducer, or digest recipe")
	}
	for name, value := range map[string]string{"transaction_id": t.TransactionID, "owner_id": t.OwnerID, "computer_id": t.ComputerID, "trajectory_id": t.TrajectoryID, "command_id": t.CommandID, "actor_id": t.Actor.ActorID, "authority_ref": t.Actor.AuthorityRef} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("supervision transaction: %s is required", name)
		}
	}
	if !IsSHA256(t.CommandDigest) {
		return fmt.Errorf("supervision transaction: invalid command_digest")
	}
	if t.Expected.CanonicalEventHead != nil && !IsSHA256(*t.Expected.CanonicalEventHead) {
		return fmt.Errorf("supervision transaction: invalid expected canonical_event_head")
	}
	if t.ObservedBase != nil {
		if err := validateRequired(reflect.ValueOf(*t.ObservedBase), "observed_base"); err != nil {
			return err
		}
	}
	if err := validateReferencedArtifacts(t.ReferencedArtifacts); err != nil {
		return err
	}
	rule, ok := supervisionClassRules[t.TransactionClass]
	if !ok {
		return fmt.Errorf("supervision transaction: unknown transaction_class %q", t.TransactionClass)
	}
	if !contains(rule.roles, t.Actor.Role) {
		return fmt.Errorf("supervision transaction: role %q cannot authorize %q", t.Actor.Role, t.TransactionClass)
	}
	if len(t.Mutations) == 0 || len(t.Mutations) > 64 || (rule.single && len(t.Mutations) != 1) {
		return fmt.Errorf("supervision transaction: invalid mutation count for %q", t.TransactionClass)
	}
	counts := make(map[string]int, len(t.Mutations))
	for i, mutation := range t.Mutations {
		if !contains(rule.kinds, mutation.Kind) {
			return fmt.Errorf("supervision transaction: mutation %q is forbidden for %q", mutation.Kind, t.TransactionClass)
		}
		bodyType, ok := supervisionBodyTypes[mutation.Kind]
		if !ok {
			return fmt.Errorf("supervision transaction: unknown mutation kind %q", mutation.Kind)
		}
		body := reflect.New(bodyType)
		decoder := json.NewDecoder(bytes.NewReader(mutation.Body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(body.Interface()); err != nil {
			return fmt.Errorf("supervision transaction: mutation %d %s: %w", i, mutation.Kind, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return fmt.Errorf("supervision transaction: mutation %d has trailing JSON", i)
		}
		if err := validateRequired(body.Elem(), mutation.Kind); err != nil {
			return err
		}
		if err := validateMutationSemantics(mutation.Kind, body.Elem().Interface()); err != nil {
			return err
		}
		counts[mutation.Kind]++
	}
	for kind, minimum := range rule.required {
		if counts[kind] < minimum {
			return fmt.Errorf("supervision transaction: %q requires %s", t.TransactionClass, kind)
		}
		if kind != "assignment_opened" && counts[kind] != minimum {
			return fmt.Errorf("supervision transaction: %q permits exactly one %s", t.TransactionClass, kind)
		}
	}
	digest, err := t.ComputeCommandDigest()
	if err != nil {
		return err
	}
	if digest != t.CommandDigest {
		return fmt.Errorf("supervision transaction: command_digest mismatch")
	}
	return nil
}

func (t SupervisionTransaction) CanonicalBytes() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return CanonicalJSON(t)
}
func (t SupervisionTransaction) ComputeCommandDigest() (string, error) {
	normalized := t
	normalized.ReferencedArtifacts = append([]ReferencedArtifact(nil), t.ReferencedArtifacts...)
	normalized.Mutations = append([]SupervisionMutation(nil), t.Mutations...)
	refReplacements := make(map[string]string, len(t.ReferencedArtifacts))
	seenBindings := make(map[string]struct{}, len(t.ReferencedArtifacts))
	for index := range normalized.ReferencedArtifacts {
		artifact := &normalized.ReferencedArtifacts[index]
		logicalPlaintextDigest := artifact.LogicalPlaintextDigest
		if logicalPlaintextDigest == "" {
			logicalPlaintextDigest = artifact.PlaintextDigest
		}
		if strings.TrimSpace(artifact.BindingID) == "" || strings.TrimSpace(artifact.MediaType) == "" || !IsSHA256(artifact.PlaintextDigest) || !IsSHA256(logicalPlaintextDigest) {
			return "", fmt.Errorf("supervision transaction: logical artifact identity is incomplete")
		}
		if _, exists := seenBindings[artifact.BindingID]; exists {
			return "", fmt.Errorf("supervision transaction: duplicate artifact binding %q", artifact.BindingID)
		}
		seenBindings[artifact.BindingID] = struct{}{}
		placeholder := SupervisionArtifactPlaceholder(artifact.BindingID)
		if artifact.Ref != "" {
			refReplacements[artifact.Ref] = placeholder
		}
		refReplacements[placeholder] = placeholder
		artifact.Ref = placeholder
		artifact.ArtifactDigest = ZeroHead
		refReplacements[artifact.PlaintextDigest] = logicalPlaintextDigest
		refReplacements[logicalPlaintextDigest] = logicalPlaintextDigest
		artifact.PlaintextDigest = logicalPlaintextDigest
		artifact.LogicalPlaintextDigest = ""
		artifact.PinReceipt = Receipt{}
	}
	for index := range normalized.Mutations {
		body, err := replaceSupervisionArtifactRefs(normalized.Mutations[index].Body, refReplacements)
		if err != nil {
			return "", fmt.Errorf("supervision transaction: normalize mutation artifacts: %w", err)
		}
		normalized.Mutations[index].Body = body
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return "", err
	}
	delete(value, "command_digest")
	delete(value, "transaction_id")
	canonical, err := CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	return DigestBytes(canonical), nil
}

// ValidateLogical validates a reservation-stage transaction whose private
// artifacts still use stable binding placeholders. It exercises the same
// closed class/body/reference rules as Validate without requiring pin receipts
// or content addresses that cannot exist before reservation.
func (t SupervisionTransaction) ValidateLogical() error {
	synthetic := t
	synthetic.Mutations = append([]SupervisionMutation(nil), t.Mutations...)
	synthetic.ReferencedArtifacts = append([]ReferencedArtifact(nil), t.ReferencedArtifacts...)
	replacements := make(map[string]string, len(synthetic.ReferencedArtifacts))
	seenBindings := make(map[string]struct{}, len(synthetic.ReferencedArtifacts))
	for index := range synthetic.ReferencedArtifacts {
		artifact := &synthetic.ReferencedArtifacts[index]
		logicalDigest := artifact.LogicalPlaintextDigest
		if logicalDigest == "" {
			logicalDigest = artifact.PlaintextDigest
		}
		if strings.TrimSpace(artifact.BindingID) == "" || strings.TrimSpace(artifact.MediaType) == "" ||
			!IsSHA256(artifact.PlaintextDigest) || !IsSHA256(logicalDigest) ||
			(artifact.Ref != "" && artifact.Ref != SupervisionArtifactPlaceholder(artifact.BindingID)) {
			return fmt.Errorf("supervision transaction: invalid logical referenced artifact")
		}
		if _, duplicate := seenBindings[artifact.BindingID]; duplicate {
			return fmt.Errorf("supervision transaction: duplicate artifact binding %q", artifact.BindingID)
		}
		seenBindings[artifact.BindingID] = struct{}{}
		placeholder := SupervisionArtifactPlaceholder(artifact.BindingID)
		digest := DigestBytes([]byte(artifact.BindingID + "\x00" + logicalDigest))
		ref := "artifact:sha256:" + digest
		replacements[placeholder] = ref
		artifact.Ref = ref
		artifact.ArtifactDigest = digest
		artifact.LogicalPlaintextDigest = logicalDigest
		artifact.PinReceipt = Receipt{ReceiptKind: "PinReceipt"}
	}
	for index := range synthetic.Mutations {
		body, err := replaceSupervisionArtifactRefs(synthetic.Mutations[index].Body, replacements)
		if err != nil {
			return fmt.Errorf("supervision transaction: validate logical mutation artifacts: %w", err)
		}
		synthetic.Mutations[index].Body = body
	}
	return synthetic.Validate()
}

// SupervisionArtifactPlaceholder is the stable logical reference used while a
// command is reserved. It is replaced by the content-addressed private ref only
// after reservation, and is never accepted by the final transaction validator.
func SupervisionArtifactPlaceholder(bindingID string) string {
	return "choir:supervision-artifact:" + DigestBytes([]byte(strings.TrimSpace(bindingID)))
}

func IsSupervisionArtifactPlaceholder(value string) bool {
	const prefix = "choir:supervision-artifact:"
	return strings.HasPrefix(value, prefix) && IsSHA256(strings.TrimPrefix(value, prefix))
}

func replaceSupervisionArtifactRefs(raw json.RawMessage, replacements map[string]string) (json.RawMessage, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	value = replaceSupervisionArtifactRefValue(value, replacements)
	return CanonicalJSON(value)
}

func replaceSupervisionArtifactRefValue(value any, replacements map[string]string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = replaceSupervisionArtifactRefValue(child, replacements)
		}
	case []any:
		for index, child := range typed {
			typed[index] = replaceSupervisionArtifactRefValue(child, replacements)
		}
	case string:
		if replacement, ok := replacements[typed]; ok {
			return replacement
		}
	}
	return value
}
func ValidateSupervisionEventBinding(event Event, transaction SupervisionTransaction) error {
	if err := transaction.Validate(); err != nil {
		return err
	}
	if event.EventKind != EventSupervisionTransaction ||
		event.ComputerID != transaction.ComputerID ||
		event.TrajectoryID != transaction.TrajectoryID ||
		event.IdempotencyKey != transaction.CommandID ||
		event.PayloadCommitment != transaction.CommandDigest ||
		event.ActorProfile != transaction.Actor.Role ||
		event.AuthorityRef != transaction.Actor.AuthorityRef ||
		event.PrivacyClass != "private" {
		return fmt.Errorf("computer event: supervision event binding mismatch")
	}
	if !IsSHA256(event.DecisionRef) {
		return fmt.Errorf("computer event: supervision artifact commitment is missing")
	}
	transactionRef, err := ArtifactRefFromDigest(event.DecisionRef)
	if err != nil {
		return err
	}
	for _, rawRef := range event.InputArtifactRefs {
		if rawRef == transactionRef.String() {
			return nil
		}
	}
	return fmt.Errorf("computer event: supervision artifact reference is missing")
}

// DecodeSupervisionTransaction accepts only one complete, schema-valid
// transaction document. Reconstruction additionally compares its canonical
// bytes to the decrypted artifact before using it.
func DecodeSupervisionTransaction(raw []byte) (SupervisionTransaction, error) {
	var transaction SupervisionTransaction
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&transaction); err != nil {
		return transaction, fmt.Errorf("supervision transaction: decode: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return transaction, fmt.Errorf("supervision transaction: trailing JSON")
	}
	if err := transaction.Validate(); err != nil {
		return transaction, err
	}
	return transaction, nil
}

func DecodeSupervisionMutationBody[T any](mutation SupervisionMutation) (T, error) {
	var body T
	decoder := json.NewDecoder(bytes.NewReader(mutation.Body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return body, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return body, fmt.Errorf("trailing JSON")
	}
	return body, nil
}

func validateRequired(value reflect.Value, path string) error {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		return validateRequired(value.Elem(), path)
	}
	switch value.Kind() {
	case reflect.Struct:
		typ := value.Type()
		for i := range value.NumField() {
			name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
			if err := validateRequired(value.Field(i), path+"."+name); err != nil {
				return err
			}
		}
	case reflect.String:
		text := value.String()
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("supervision transaction: %s is required", path)
		}
		name := path[strings.LastIndex(path, ".")+1:]
		if strings.HasSuffix(name, "_digest") || strings.HasSuffix(name, "_head") || name == "fingerprint" || name == "idempotency_commitment" {
			if !IsSHA256(text) {
				return fmt.Errorf("supervision transaction: %s must be a SHA256 digest", path)
			}
		}
		if strings.Contains(name, "artifact_ref") || strings.Contains(name, "receipt_ref") || name == "import_ref" || name == "source_graph_ref" {
			if _, err := ParseArtifactRef(text); err != nil {
				return fmt.Errorf("supervision transaction: %s: %w", path, err)
			}
		}
	case reflect.Slice:
		seen := map[string]struct{}{}
		for i := range value.Len() {
			item := value.Index(i)
			if err := validateRequired(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
			if item.Kind() == reflect.String {
				text := item.String()
				if _, ok := seen[text]; ok {
					return fmt.Errorf("supervision transaction: %s contains duplicate %q", path, text)
				}
				seen[text] = struct{}{}
			}
		}
	}
	return nil
}

func validateMutationSemantics(kind string, body any) error {
	switch value := body.(type) {
	case projectionImportedBody:
		if len(value.Manifest) == 0 {
			return fmt.Errorf("supervision transaction: projection import manifest is required")
		}
		var manifest any
		decoder := json.NewDecoder(bytes.NewReader(value.Manifest))
		decoder.UseNumber()
		if err := decoder.Decode(&manifest); err != nil {
			return fmt.Errorf("supervision transaction: projection import manifest: %w", err)
		}
		manifestMap, ok := manifest.(map[string]any)
		if !ok {
			return fmt.Errorf("supervision transaction: projection import manifest must be an object")
		}
		canonicalArtifact, err := CanonicalJSON(manifestMap)
		if err != nil || DigestBytes(canonicalArtifact) != value.ImportArtifactPlaintextDigest {
			return fmt.Errorf("supervision transaction: projection import plaintext digest does not bind manifest")
		}
		projectionDigest, ok := manifestMap["projection_digest"].(string)
		if !ok || projectionDigest != value.ImportDigest {
			return fmt.Errorf("supervision transaction: projection import digest does not bind manifest")
		}
		manifestMap["projection_digest"] = ""
		canonical, err := CanonicalJSON(manifestMap)
		if err != nil {
			return fmt.Errorf("supervision transaction: canonicalize projection import manifest: %w", err)
		}
		if DigestBytes(canonical) != value.ImportDigest {
			return fmt.Errorf("supervision transaction: projection import digest does not bind manifest")
		}
		if _, err := ParseArtifactRef(value.ImportRef); err != nil {
			return fmt.Errorf("supervision transaction: projection import ref is invalid")
		}
		if len(value.DrainReceiptRefs) == 0 {
			return fmt.Errorf("supervision transaction: projection import drain receipts are required")
		}
		if value.ObjectCount == 0 {
			return fmt.Errorf("supervision transaction: projection import objects are required")
		}
	case trajectoryStartedBody:
		if len(value.InitialAssignmentIDs) == 0 {
			return fmt.Errorf("supervision transaction: trajectory_started.initial_assignment_ids must not be empty")
		}
	case intentRevisedBody:
		if value.Material != (len(value.AffectedTargets) > 0) {
			return fmt.Errorf("supervision transaction: intent_revised materiality and affected_targets disagree")
		}
	case assignmentOpenedBody:
		if len(value.ObligationIDs) == 0 {
			return fmt.Errorf("supervision transaction: assignment_opened.obligation_ids must not be empty")
		}
		if !contains([]string{"cosuper", "researcher", "verifier"}, value.AssignedRole) {
			return fmt.Errorf("supervision transaction: invalid assignment_opened.assigned_role")
		}
	case attemptStartedBody:
		if value.Ordinal == 0 {
			return fmt.Errorf("supervision transaction: attempt_started.ordinal must be positive")
		}
		switch value.AttemptKind {
		case "initial":
			if value.Ordinal != 1 || value.PriorAttemptID != nil {
				return fmt.Errorf("supervision transaction: initial attempt must be ordinal one without prior_attempt_id")
			}
		case "retry":
			if value.Ordinal < 2 || value.PriorAttemptID == nil || strings.TrimSpace(*value.PriorAttemptID) == "" {
				return fmt.Errorf("supervision transaction: retry attempt needs ordinal and prior_attempt_id")
			}
		default:
			return fmt.Errorf("supervision transaction: invalid attempt_kind")
		}
	case textureRevisionBody:
		if !contains([]string{"owner_edit", "texture_synthesis", "semantic_rebase", "settlement"}, value.NarrativeKind) {
			return fmt.Errorf("supervision transaction: invalid narrative_kind")
		}
	case actorMessageBody:
		if !contains([]string{"owner", "texture", "super", "researcher", "cosuper", "verifier"}, value.ToRole) {
			return fmt.Errorf("supervision transaction: invalid to_role")
		}
	case attemptResultBody:
		if !contains([]string{"succeeded", "failed", "cancelled", "blocked"}, value.Outcome) {
			return fmt.Errorf("supervision transaction: invalid attempt outcome")
		}
	case superFindingBody:
		if !validSupervisionTarget(value.Subject) {
			return fmt.Errorf("supervision transaction: invalid finding subject")
		}
		if !contains([]string{"watch", "nudge_required", "blocked", "violation"}, value.Severity) || !contains([]string{"open", "resolved", "escalated"}, value.State) {
			return fmt.Errorf("supervision transaction: invalid finding state")
		}
	case dissentBody:
		if !validSupervisionTarget(value.Subject) {
			return fmt.Errorf("supervision transaction: invalid dissent subject")
		}
	case superDecisionBody:
		if !contains([]string{"none", "texture", "owner"}, value.ReservedAuthority) {
			return fmt.Errorf("supervision transaction: invalid reserved_authority")
		}
	case ownerDecisionBody:
		if (value.Decision == nil) != (value.SettlementSnapshotDigest == nil) {
			return fmt.Errorf("supervision transaction: settlement owner decision fields must be complete")
		}
		if value.Decision != nil && !contains([]string{"accept", "revise"}, *value.Decision) {
			return fmt.Errorf("supervision transaction: invalid owner settlement decision")
		}
	case dispositionBody:
		if !validDispositionTarget(value.Target) {
			return fmt.Errorf("supervision transaction: invalid disposition target")
		}
		if !contains([]string{"preserved", "invalidated", "superseded", "compensation_required", "cancelled", "late", "incorporated", "rejected"}, value.Value) {
			return fmt.Errorf("supervision transaction: invalid disposition")
		}
		if value.Value == "compensation_required" && value.CompensationObligationID == nil {
			return fmt.Errorf("supervision transaction: compensation_required needs compensation_obligation_id")
		}
		if value.Value != "compensation_required" && value.CompensationObligationID != nil {
			return fmt.Errorf("supervision transaction: compensation_obligation_id requires compensation_required")
		}
	}
	_ = kind
	return nil
}

func validSupervisionTarget(target supervisionTarget) bool {
	return strings.TrimSpace(target.ID) != "" && contains([]string{
		"trajectory", "artifact", "assignment", "attempt", "result", "update",
		"belief", "finding", "dissent", "decision", "premise",
		"rebase_obligation", "compensation_obligation", "owner_attention",
	}, target.Kind)
}

func validDispositionTarget(target supervisionTarget) bool {
	return strings.TrimSpace(target.ID) != "" && contains([]string{
		"assignment", "attempt", "result", "update", "finding", "dissent",
		"rebase_obligation", "compensation_obligation",
	}, target.Kind)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateReferencedArtifacts(artifacts []ReferencedArtifact) error {
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		ref, err := ParseArtifactRef(artifact.Ref)
		if err != nil || ref.Digest().String() != artifact.ArtifactDigest || !IsSHA256(artifact.ArtifactDigest) ||
			!IsSHA256(artifact.PlaintextDigest) || (artifact.LogicalPlaintextDigest != "" && !IsSHA256(artifact.LogicalPlaintextDigest)) ||
			strings.TrimSpace(artifact.MediaType) == "" || strings.TrimSpace(artifact.BindingID) == "" ||
			artifact.PinReceipt.ReceiptKind != "PinReceipt" {
			return fmt.Errorf("supervision transaction: invalid referenced artifact")
		}
		if _, duplicate := seen[artifact.Ref]; duplicate {
			return fmt.Errorf("supervision transaction: duplicate referenced artifact")
		}
		seen[artifact.Ref] = struct{}{}
	}
	return nil
}
