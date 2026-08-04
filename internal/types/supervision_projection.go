package types

import "encoding/json"

// SupervisionProjectionEntry is one owner-readable, event-derived semantic
// record. Body is the exact validated mutation body, not a Trace transcript.
type SupervisionProjectionEntry struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`
	Status       string          `json:"status,omitempty"`
	Body         json.RawMessage `json:"body"`
	ArtifactRefs []string        `json:"artifact_refs,omitempty"`
	EvidenceRefs []string        `json:"evidence_refs,omitempty"`
}

// SupervisionProjectionControl is the bounded owner-orientation surface. Every
// listed record retains its exact artifact and evidence references. Omitted
// records are counted rather than silently treated as absent.
type SupervisionProjectionControl struct {
	Intent         *SupervisionProjectionEntry  `json:"intent,omitempty"`
	LatestDelta    *SupervisionProjectionEntry  `json:"latest_delta,omitempty"`
	Belief         *SupervisionProjectionEntry  `json:"belief,omitempty"`
	Messages       []SupervisionProjectionEntry `json:"messages"`
	Obligations    []SupervisionProjectionEntry `json:"obligations"`
	Blockers       []SupervisionProjectionEntry `json:"blockers"`
	Dissent        []SupervisionProjectionEntry `json:"dissent"`
	Decisions      []SupervisionProjectionEntry `json:"decisions"`
	Rebase         []SupervisionProjectionEntry `json:"rebase"`
	Settlement     *SupervisionProjectionEntry  `json:"settlement,omitempty"`
	OverflowCount  int                          `json:"overflow_count"`
	AttentionCount int                          `json:"attention_count"`
}

// SupervisionProjectionSnapshot is the canonical, owner-private read model
// reduced from acknowledged supervision transactions. It deliberately exposes
// no raw run Trace or role-sequence requirement.
type SupervisionProjectionSnapshot struct {
	Schema                     string                       `json:"schema"`
	OwnerID                    string                       `json:"owner_id"`
	ComputerID                 string                       `json:"computer_id"`
	TrajectoryID               string                       `json:"trajectory_id"`
	CanonicalEventHead         string                       `json:"canonical_event_head"`
	ObservedCanonicalEventHead string                       `json:"observed_canonical_event_head"`
	SnapshotCursor             int64                        `json:"snapshot_cursor"`
	LifecycleVersion           int64                        `json:"lifecycle_version"`
	IntentRevisionID           string                       `json:"intent_revision_id,omitempty"`
	ArtifactHeadRevisionID     string                       `json:"artifact_head_revision_id,omitempty"`
	SettlementProposalID       string                       `json:"settlement_proposal_id,omitempty"`
	Settled                    bool                         `json:"settled"`
	Archived                   bool                         `json:"archived"`
	Control                    SupervisionProjectionControl `json:"control"`
	Attempts                   []SupervisionProjectionEntry `json:"attempts"`
	Results                    []SupervisionProjectionEntry `json:"results"`
	Dispositions               []SupervisionProjectionEntry `json:"dispositions"`
	Findings                   []SupervisionProjectionEntry `json:"findings"`
	Reconciliations            []SupervisionProjectionEntry `json:"reconciliations"`
}
