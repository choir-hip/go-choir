package decisionpolicy

const (
	ReducerVersionV1                = "decisionpolicy-consensus-v1"
	ReceiptKindPolicySelection      = "PolicySelectionReceipt"
	ReceiptKindQualifiedConsensus   = "QualifiedConsensusReceipt"
	AuthorityPrefix                 = "qualified-consensus:"
	PolicyIDReversibleSelfDevV1     = "reversible-selfdev-v1"
	PolicyDigestReversibleSelfDevV1 = "c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7"
	EffectClassReversible           = "reversible_computer_local"
	EffectClassIrreversible         = "irreversible_external"
	HumanSeatAbsent                 = "absent"
	HumanSeatRequired               = "required"
	HumanSeatOptional               = "optional"
	HumanSeatPresent                = "present"
	HumanSeatNotRequired            = "not_required"
	VoteAccept                      = "accept"
	VoteReject                      = "reject"
	VoteAbstain                     = "abstain"
	ActuatorTrustedOutbox           = "trusted_outbox"
	AttestationPrefix               = "choir-ballot-attestation-v1:"
)

type Policy struct {
	PolicyID              string            `json:"policy_id"`
	Actuator              string            `json:"actuator"`
	AdmissibleEvidence    []string          `json:"admissible_evidence"`
	BlastRadius           []string          `json:"blast_radius"`
	BlastRadiusOut        []string          `json:"blast_radius_out"`
	Budget                PolicyBudget      `json:"budget"`
	Capabilities          []string          `json:"capabilities"`
	ConsequenceReceipt    []string          `json:"consequence_receipt"`
	Dissent               PolicyDissent     `json:"dissent"`
	EffectClass           string            `json:"effect_class"`
	EligibleSeats         []PolicySeat      `json:"eligible_seats"`
	Expiry                string            `json:"expiry"`
	ForbiddenCapabilities []string          `json:"forbidden_capabilities"`
	HumanSeat             string            `json:"human_seat"`
	InadmissibleEvidence  []string          `json:"inadmissible_evidence"`
	IndependenceDomains   []string          `json:"independence_domains"`
	OwnerRevocation       bool              `json:"owner_revocation"`
	Privacy               string            `json:"privacy"`
	Quorum                PolicyQuorum      `json:"quorum"`
	Recovery              string            `json:"recovery"`
	Recusal               string            `json:"recusal"`
	Replacement           PolicyReplacement `json:"replacement"`
	Scope                 string            `json:"scope"`
	SubjectBinding        []string          `json:"subject_binding"`
	Timeout               PolicyTimeout     `json:"timeout"`
}

type PolicyBudget struct {
	Currency                int `json:"currency"`
	ExternalSends           int `json:"external_sends"`
	PromotionsOfThisSubject int `json:"promotions_of_this_subject"`
}

type PolicyDissent struct {
	PolicyBlockingUnresolved string `json:"policy_blocking_unresolved"`
	RecordedDissentAllowed   bool   `json:"recorded_dissent_allowed"`
}

type PolicySeat struct {
	CountsTowardQuorum bool     `json:"counts_toward_quorum"`
	Eligibility        string   `json:"eligibility"`
	IndependenceDomain string   `json:"independence_domain"`
	Kind               string   `json:"kind"`
	Profile            string   `json:"profile"`
	RecusedFrom        []string `json:"recused_from"`
	SeatID             string   `json:"seat_id"`
}

type PolicyQuorum struct {
	AbstentionCountsAgainstQuorum bool                    `json:"abstention_counts_against_quorum"`
	GlobalAcceptMinimum           int                     `json:"global_accept_minimum"`
	PerDomain                     map[string]DomainQuorum `json:"per_domain"`
	Weighting                     string                  `json:"weighting"`
}

type DomainQuorum struct {
	AcceptMinimum   int  `json:"accept_minimum"`
	RequiredPresent bool `json:"required_present"`
}

type PolicyReplacement struct {
	Bench                        []string `json:"bench"`
	EmptyBenchOnRequiredSeatLoss string   `json:"empty_bench_on_required_seat_loss"`
}

type PolicyTimeout struct {
	Clock          string `json:"clock"`
	DecisionWindow string `json:"decision_window"`
}

type EffectSubject struct {
	ComputerID               string `json:"computer_id"`
	OperationID              string `json:"operation_id"`
	BundleDigest             string `json:"bundle_digest"`
	DesiredEventHead         string `json:"desired_event_head"`
	EffectiveEventHead       string `json:"effective_event_head"`
	PendingTransitionRef     string `json:"pending_transition_ref"`
	DesiredStateCommitment   string `json:"desired_state_commitment"`
	EffectiveStateCommitment string `json:"effective_state_commitment"`
	Recipient                string `json:"recipient,omitempty"`
	PayloadDigest            string `json:"payload_digest,omitempty"`
	Actuator                 string `json:"actuator,omitempty"`
	AcceptanceInbox          string `json:"acceptance_inbox,omitempty"`
	OwnerRecovery            bool   `json:"owner_recovery,omitempty"`
	EffectClass              string `json:"effect_class,omitempty"`
	ExternalSends            int    `json:"external_sends,omitempty"`
}

type Seat struct {
	SeatID             string `json:"seat_id"`
	IndependenceDomain string `json:"independence_domain"`
	Kind               string `json:"kind"`
	EligibilityProof   string `json:"eligibility_proof"`
	Recused            bool   `json:"recused"`
}

type SeatManifest struct {
	Seats []Seat `json:"seats"`
}

type PolicySelectionReceipt struct {
	ReceiptKind        string `json:"receipt_kind"`
	PolicyDigest       string `json:"policy_digest"`
	SeatManifestDigest string `json:"seat_manifest_digest"`
	SubjectDigest      string `json:"subject_digest"`
	SelectedAtHead     string `json:"selected_at_head"`
	SelectedSequence   uint64 `json:"selected_sequence"`
	SelectionDigest    string `json:"selection_digest,omitempty"`
}

type BallotAttestation struct {
	BallotID               string `json:"ballot_id"`
	SeatID                 string `json:"seat_id"`
	EligibilityProofDigest string `json:"eligibility_proof_digest"`
	IndependenceDomain     string `json:"independence_domain"`
	PolicyDigest           string `json:"policy_digest"`
	SeatManifestDigest     string `json:"seat_manifest_digest"`
	SubjectDigest          string `json:"subject_digest"`
	PolicySelectionDigest  string `json:"policy_selection_digest"`
	Vote                   string `json:"vote"`
	WindowID               string `json:"window_id"`
	SignerProvenance       string `json:"signer_provenance"`
	Attestation            string `json:"attestation,omitempty"`
	BallotDigest           string `json:"ballot_digest,omitempty"`
}

type QuorumEvaluation struct {
	GlobalAccepts int            `json:"global_accepts"`
	DomainAccepts map[string]int `json:"domain_accepts"`
	Met           bool           `json:"met"`
}

type QualifiedConsensusReceipt struct {
	ReceiptKind         string              `json:"receipt_kind"`
	PolicyID            string              `json:"policy_id"`
	PolicyDigest        string              `json:"policy_digest"`
	SubjectDigest       string              `json:"subject_digest"`
	EligibleSeatsDigest string              `json:"eligible_seats_digest"`
	SelectionDigest     string              `json:"selection_digest"`
	Ballots             []BallotAttestation `json:"ballots"`
	QuorumEvaluation    QuorumEvaluation    `json:"quorum_evaluation"`
	DissentDisposition  string              `json:"dissent_disposition"`
	HumanSeatState      string              `json:"human_seat_state"`
	Window              DecisionWindow      `json:"window"`
	ReducerVersion      string              `json:"reducer_version"`
	ReceiptDigest       string              `json:"receipt_digest,omitempty"`
}

type DecisionWindow struct {
	StartedAt string `json:"started_at"`
	ExpiresAt string `json:"expires_at"`
}

type ConsensusInput struct {
	Policy    Policy
	Manifest  SeatManifest
	Subject   EffectSubject
	Selection PolicySelectionReceipt
	Ballots   []BallotAttestation
	Now       string
}

type ConsensusBundle struct {
	Manifest  SeatManifest           `json:"seat_manifest"`
	Subject   EffectSubject          `json:"subject"`
	Selection PolicySelectionReceipt `json:"selection"`
	Ballots   []BallotAttestation    `json:"ballots"`
	Now       string                 `json:"now,omitempty"`
}
