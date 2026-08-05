package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type supervisionProjectionState struct {
	Schema                      string                                `json:"schema"`
	OwnerID                     string                                `json:"owner_id"`
	ComputerID                  string                                `json:"computer_id"`
	TrajectoryID                string                                `json:"trajectory_id"`
	TrajectoryKind              string                                `json:"trajectory_kind"`
	SubjectRefs                 map[string]string                     `json:"subject_refs"`
	LifecycleVersion            uint64                                `json:"lifecycle_version"`
	ProjectionCursor            uint64                                `json:"projection_cursor"`
	CanonicalSequence           uint64                                `json:"canonical_sequence"`
	CanonicalEventHead          string                                `json:"canonical_event_head"`
	IntentRevisionID            string                                `json:"intent_revision_id"`
	ArtifactID                  string                                `json:"artifact_id"`
	ArtifactHeadRevisionID      string                                `json:"artifact_head_revision_id"`
	SettlementProposalID        string                                `json:"settlement_proposal_id"`
	SettlementSnapshotDigest    string                                `json:"settlement_snapshot_digest"`
	Settled                     bool                                  `json:"settled"`
	Archived                    bool                                  `json:"archived"`
	Entities                    map[string]map[string]json.RawMessage `json:"entities"`
	Statuses                    map[string]map[string]string          `json:"statuses"`
	Dispositions                map[string]string                     `json:"dispositions"`
	StaleDispositions           map[string]bool                       `json:"stale_dispositions"`
	OwnerAttention              map[string]bool                       `json:"owner_attention"`
	ReferencedArtifacts         map[string]bool                       `json:"referenced_artifacts"`
	OpenRebaseObligations       map[string]bool                       `json:"open_rebase_obligations"`
	OpenCompensationObligations map[string]bool                       `json:"open_compensation_obligations"`
	OpenFindings                map[string]bool                       `json:"open_findings"`
	OpenDissents                map[string]bool                       `json:"open_dissents"`
	CreatedAt                   time.Time                             `json:"created_at"`
	UpdatedAt                   time.Time                             `json:"updated_at"`
}

type supervisionProjectionEvent struct {
	Schema           string                            `json:"schema"`
	TransactionID    string                            `json:"transaction_id"`
	TransactionClass string                            `json:"transaction_class"`
	CommandID        string                            `json:"command_id"`
	CommandDigest    string                            `json:"command_digest"`
	OwnerID          string                            `json:"owner_id"`
	ComputerID       string                            `json:"computer_id"`
	TrajectoryID     string                            `json:"trajectory_id"`
	Sequence         uint64                            `json:"sequence"`
	ProjectionCursor uint64                            `json:"projection_cursor"`
	MutationIndex    int                               `json:"mutation_index"`
	Mutation         computerevent.SupervisionMutation `json:"mutation"`
	CreatedAt        time.Time                         `json:"created_at"`
}

// SupervisionProjectionSemanticDigest commits the replay-owned supervision
// state and canonical head while excluding storage timestamps. It is the
// stable comparison value for a private-tape rebuild, not a general object
// graph digest.
func (s *Store) SupervisionProjectionSemanticDigest(ctx context.Context, computerID string) (string, error) {
	if s == nil || s.db == nil || strings.TrimSpace(computerID) == "" {
		return "", fmt.Errorf("supervision projection: digest requires store and computer")
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	head, err := s.Head(ctx, computerID)
	if err != nil {
		return "", err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT canonical_id, body FROM og_objects WHERE computer_id=? AND object_kind=? AND tombstone=FALSE ORDER BY canonical_id`, computerID, ogKindSupervisionState)
	if err != nil {
		return "", fmt.Errorf("supervision projection: list semantic state: %w", err)
	}
	defer rows.Close()
	type semanticState struct {
		CanonicalID string                     `json:"canonical_id"`
		State       supervisionProjectionState `json:"state"`
	}
	states := make([]semanticState, 0)
	for rows.Next() {
		var item semanticState
		var raw []byte
		if err := rows.Scan(&item.CanonicalID, &raw); err != nil {
			return "", fmt.Errorf("supervision projection: scan semantic state: %w", err)
		}
		if err := json.Unmarshal(raw, &item.State); err != nil || item.State.ComputerID != computerID {
			return "", fmt.Errorf("supervision projection: invalid semantic state")
		}
		item.State.CreatedAt = time.Time{}
		item.State.UpdatedAt = time.Time{}
		states = append(states, item)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("supervision projection: iterate semantic state: %w", err)
	}
	canonical, err := computerevent.CanonicalJSON(struct {
		Head   *computerevent.Head `json:"head"`
		States []semanticState     `json:"states"`
	}{Head: head, States: states})
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

// SupervisionAttemptLineage is the complete, unbounded canonical attempt
// history for one assignment. It is intentionally separate from the bounded
// owner projection view used for UI rendering.
type SupervisionAttemptLineage struct {
	AttemptID    string
	AttemptKind  string
	Ordinal      uint64
	PriorAttempt string
	RunID        string
	Status       string
}

type SupervisionAssignmentLineage struct {
	AssignmentID    string
	AssignedActorID string
	AssignedRole    string
	Status          string
	Attempts        []SupervisionAttemptLineage
}

// SupervisionDeliveryEvent is one unbounded, canonically ordered projection
// record whose private artifact may need delivery to an actor. It carries the
// validated mutation body and binding identity, never a second semantic copy
// of the private payload.
type SupervisionDeliveryEvent struct {
	ID               string
	Kind             string
	TransactionID    string
	CommandID        string
	Sequence         uint64
	TrajectoryID     string
	ProjectionCursor uint64
	Body             json.RawMessage
	CreatedAt        time.Time
}

// GetSupervisionAssignmentLineage reads every canonical supervision mutation
// for an assignment; it never consults the bounded owner snapshot.
func (s *Store) GetSupervisionAssignmentLineage(ctx context.Context, ownerID, computerID, trajectoryID, assignmentID string) (SupervisionAssignmentLineage, error) {
	if s == nil || s.ogStore == nil {
		return SupervisionAssignmentLineage{}, fmt.Errorf("supervision lineage: store unavailable")
	}
	events, err := s.listSupervisionProjectionEvents(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return SupervisionAssignmentLineage{}, err
	}
	lineage := SupervisionAssignmentLineage{AssignmentID: assignmentID}
	attempts := make(map[string]*SupervisionAttemptLineage)
	for _, event := range events {
		var body map[string]any
		if err := json.Unmarshal(event.Mutation.Body, &body); err != nil {
			return SupervisionAssignmentLineage{}, fmt.Errorf("decode supervision lineage mutation: %w", err)
		}
		if bodyString(body, "assignment_id") != assignmentID {
			continue
		}
		switch event.Mutation.Kind {
		case "assignment_opened":
			lineage.AssignedActorID, lineage.AssignedRole, lineage.Status = bodyString(body, "assigned_actor_id"), bodyString(body, "assigned_role"), "open"
		case "attempt_started":
			attempt := &SupervisionAttemptLineage{AttemptID: bodyString(body, "attempt_id"), AttemptKind: bodyString(body, "attempt_kind"), Ordinal: bodyUint(body, "ordinal"), PriorAttempt: bodyNullableString(body, "prior_attempt_id"), RunID: bodyString(body, "run_id"), Status: "open"}
			attempts[attempt.AttemptID] = attempt
			lineage.Attempts = append(lineage.Attempts, *attempt)
		case "assignment_cancelled":
			lineage.Status = "cancelled"
			for _, attemptID := range bodyStrings(body, "active_attempt_ids") {
				if attempt, ok := attempts[attemptID]; ok {
					attempt.Status = "cancelled"
				}
			}
		case "attempt_result":
			if attempt, ok := attempts[bodyString(body, "attempt_id")]; ok {
				attempt.Status = "returned"
			}
		}
	}
	// Map updates above affect pointers; copy the final statuses into the
	// stable ordered slice before returning.
	for i := range lineage.Attempts {
		if attempt, ok := attempts[lineage.Attempts[i].AttemptID]; ok {
			lineage.Attempts[i] = *attempt
		}
	}
	if lineage.AssignedActorID == "" {
		return SupervisionAssignmentLineage{}, ErrNotFound
	}
	return lineage, nil
}

// ListSupervisionDeliveryEvents returns every canonical delivery and its
// acknowledgement in append order. The bounded owner projection is deliberately
// not used for delivery recovery.
func (s *Store) ListSupervisionDeliveryEvents(ctx context.Context, ownerID, computerID, trajectoryID string) ([]SupervisionDeliveryEvent, error) {
	events, err := s.listSupervisionProjectionEvents(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return nil, err
	}
	deliveries := make([]SupervisionDeliveryEvent, 0, len(events))
	for _, event := range events {
		switch event.Mutation.Kind {
		case "actor_message_recorded", "actor_message_acknowledged", "researcher_packet_recorded", "attempt_result":
		default:
			continue
		}
		body, err := supervisionBodyMap(event.Mutation.Body)
		if err != nil {
			return nil, fmt.Errorf("supervision delivery: decode %s body: %w", event.Mutation.Kind, err)
		}
		id, err := supervisionMutationID(event.Mutation.Kind, body)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, SupervisionDeliveryEvent{
			ID: id, Kind: event.Mutation.Kind, TransactionID: event.TransactionID,
			Sequence:  event.Sequence,
			CommandID: event.CommandID, TrajectoryID: event.TrajectoryID,
			ProjectionCursor: event.ProjectionCursor, Body: append(json.RawMessage(nil), event.Mutation.Body...),
			CreatedAt: event.CreatedAt,
		})
	}
	return deliveries, nil
}

func (s *Store) listSupervisionProjectionEvents(ctx context.Context, ownerID, computerID, trajectoryID string) ([]supervisionProjectionEvent, error) {
	if s == nil || s.ogStore == nil {
		return nil, fmt.Errorf("supervision events: store unavailable")
	}
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return nil, err
	}
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		return nil, fmt.Errorf("supervision events: trajectory_id is required")
	}
	events := make([]supervisionProjectionEvent, 0)
	after := ""
	for {
		page, err := s.ogStore.ListObjectsPage(ctx, string(ogKindSupervisionEvent), after, 1000)
		if err != nil {
			return nil, err
		}
		for _, obj := range page {
			if obj.OwnerID != ownerID || obj.ComputerID != computerID {
				continue
			}
			event, err := decodeLifecycleObject[supervisionProjectionEvent](obj)
			if err != nil {
				return nil, err
			}
			if event.TrajectoryID == trajectoryID {
				events = append(events, event)
			}
		}
		if len(page) < 1000 {
			break
		}
		after = page[len(page)-1].CanonicalID
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].ProjectionCursor != events[j].ProjectionCursor {
			return events[i].ProjectionCursor < events[j].ProjectionCursor
		}
		return events[i].MutationIndex < events[j].MutationIndex
	})
	return events, nil
}

func (s *Store) applySupervisionProjectionTx(ctx context.Context, tx *sql.Tx, sequence uint64, previousHead, eventDigest string, occurredAt time.Time, transaction computerevent.SupervisionTransaction) error {
	if err := transaction.Validate(); err != nil {
		return fmt.Errorf("supervision projection: %w", err)
	}
	stateID, err := lifecycleCanonicalID(ogKindSupervisionState, transaction.OwnerID, transaction.ComputerID, transaction.TrajectoryID)
	if err != nil {
		return err
	}
	state, _, exists, err := loadSupervisionStateTx(ctx, tx, stateID)
	if err != nil {
		return err
	}
	if err := validateSupervisionExpected(transaction, state, exists, previousHead); err != nil {
		return err
	}
	now := occurredAt
	if !exists {
		state = supervisionProjectionState{
			Schema: computerevent.SupervisionReducerV1, OwnerID: transaction.OwnerID, ComputerID: transaction.ComputerID,
			TrajectoryID: transaction.TrajectoryID, CanonicalEventHead: previousHead, Entities: map[string]map[string]json.RawMessage{}, Statuses: map[string]map[string]string{},
			Dispositions: map[string]string{}, StaleDispositions: map[string]bool{}, OwnerAttention: map[string]bool{}, ReferencedArtifacts: map[string]bool{},
			OpenRebaseObligations: map[string]bool{}, OpenCompensationObligations: map[string]bool{}, OpenFindings: map[string]bool{}, OpenDissents: map[string]bool{}, CreatedAt: now,
		}
	}
	if state.Dispositions == nil {
		state.Dispositions = map[string]string{}
	}
	if state.StaleDispositions == nil {
		state.StaleDispositions = map[string]bool{}
	}
	if state.OwnerAttention == nil {
		state.OwnerAttention = map[string]bool{}
	}
	supervisionRecordReferencedArtifacts(&state, transaction)

	if state.ProjectionCursor == 0 && state.LifecycleVersion > 0 {
		state.ProjectionCursor = uint64(supervisionEntityCount(state))
	}
	objects := make([]objectgraph.Object, 0, len(transaction.Mutations)+8)
	conditions := []objectgraph.ObjectCondition{{CanonicalID: stateID, Exists: exists}}
	for index, mutation := range transaction.Mutations {
		body, err := supervisionBodyMap(mutation.Body)
		if err != nil {
			return fmt.Errorf("supervision projection: %s body: %w", mutation.Kind, err)
		}
		if err := applySupervisionMutation(&state, transaction, mutation.Kind, body); err != nil {
			return err
		}
		entityID, err := supervisionMutationID(mutation.Kind, body)
		if err != nil {
			return err
		}
		if state.Entities[mutation.Kind] == nil {
			state.Entities[mutation.Kind] = map[string]json.RawMessage{}
		}
		if _, duplicate := state.Entities[mutation.Kind][entityID]; duplicate {
			return fmt.Errorf("supervision projection: duplicate %s %s", mutation.Kind, entityID)
		}
		state.Entities[mutation.Kind][entityID] = append(json.RawMessage(nil), mutation.Body...)

		cursor := state.ProjectionCursor + uint64(index+1)
		event := supervisionProjectionEvent{Schema: computerevent.SupervisionReducerV1, TransactionID: transaction.TransactionID, TransactionClass: transaction.TransactionClass, CommandID: transaction.CommandID, CommandDigest: transaction.CommandDigest, OwnerID: transaction.OwnerID, ComputerID: transaction.ComputerID, TrajectoryID: transaction.TrajectoryID, Sequence: sequence, ProjectionCursor: cursor, MutationIndex: index, Mutation: mutation, CreatedAt: now}
		eventKey := transaction.TransactionID + ":" + strconv.Itoa(index)
		eventObj, err := lifecycleObject(ogKindSupervisionEvent, transaction.OwnerID, transaction.ComputerID, eventKey, event, lifecycleMetadata("event_id", eventKey, transaction.ComputerID, transaction.TrajectoryID, int64(cursor)), now, now)
		if err != nil {
			return err
		}
		objects = append(objects, eventObj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID, Exists: false})
		lifecycleEvent := types.LifecycleEvent{Schema: computerevent.SupervisionSchemaV1, EventID: eventKey, OwnerID: transaction.OwnerID, ComputerID: transaction.ComputerID, TrajectoryID: transaction.TrajectoryID, WorkItemID: bodyString(body, "assignment_id"), UpdateID: bodyString(body, "update_id"), Kind: types.LifecycleEventKind(mutation.Kind), ReducerVersion: computerevent.SupervisionReducerV1, ReducerSeq: int64(cursor), CommandID: transaction.CommandID, CommandDigest: transaction.CommandDigest, ArtifactRefs: supervisionArtifactRefs(body), EvidenceRefs: bodyStrings(body, "evidence_refs"), CreatedAt: now}
		lifecycleEventObj, err := lifecycleObject(ogKindLifecycleEvent, transaction.OwnerID, transaction.ComputerID, eventKey, lifecycleEvent, lifecycleMetadata("event_id", eventKey, transaction.ComputerID, transaction.TrajectoryID, int64(cursor)), now, now)
		if err != nil {
			return err
		}
		objects = append(objects, lifecycleEventObj)
	}
	state.LifecycleVersion++
	state.ProjectionCursor += uint64(len(transaction.Mutations))
	state.CanonicalSequence = sequence
	state.CanonicalEventHead = eventDigest
	state.UpdatedAt = now
	stateObject, err := lifecycleObject(ogKindSupervisionState, state.OwnerID, state.ComputerID, state.TrajectoryID, state, lifecycleMetadata("trajectory_id", state.TrajectoryID, state.ComputerID, state.TrajectoryID, int64(state.ProjectionCursor)), state.CreatedAt, now)
	if err != nil {
		return err
	}
	stateObject.VersionID = strconv.FormatUint(state.LifecycleVersion, 10)
	objects = append(objects, stateObject)
	derived, err := supervisionDerivedObjects(state, transaction, sequence, now)
	if err != nil {
		return err
	}
	objects = append(objects, derived...)
	if err := s.ogStore.PutBatchConditionalTx(ctx, tx, conditions, objectgraph.Batch{Objects: objects}); err != nil {
		return fmt.Errorf("supervision projection: atomic finalize: %w", err)
	}
	return nil
}
func supervisionRecordReferencedArtifacts(state *supervisionProjectionState, transaction computerevent.SupervisionTransaction) {
	if state.ReferencedArtifacts == nil {
		state.ReferencedArtifacts = map[string]bool{}
	}
	for _, artifact := range transaction.ReferencedArtifacts {
		state.ReferencedArtifacts[artifact.Ref] = true
	}
}

func loadSupervisionStateTx(ctx context.Context, tx *sql.Tx, canonicalID string) (supervisionProjectionState, objectgraph.Object, bool, error) {
	var state supervisionProjectionState
	var obj objectgraph.Object
	var kind, rawBody, rawMetadata string
	err := tx.QueryRowContext(ctx, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id=? FOR UPDATE`, canonicalID).Scan(&obj.CanonicalID, &kind, &obj.OwnerID, &obj.ComputerID, &obj.VersionID, &obj.ContentHash, &rawBody, &rawMetadata, &obj.CreatedAt, &obj.UpdatedAt, &obj.Tombstone, &obj.SupersededBy)
	if errors.Is(err, sql.ErrNoRows) {
		return state, obj, false, nil
	}
	if err != nil {
		return state, obj, false, fmt.Errorf("supervision projection: load state: %w", err)
	}
	obj.ObjectKind = objectgraph.ObjectKind(kind)
	obj.Body = []byte(rawBody)
	obj.Metadata = json.RawMessage(rawMetadata)
	if err := json.Unmarshal(obj.Body, &state); err != nil {
		return state, obj, false, fmt.Errorf("supervision projection: decode state: %w", err)
	}
	return state, obj, true, nil
}

func validateSupervisionExpected(transaction computerevent.SupervisionTransaction, state supervisionProjectionState, exists bool, previousHead string) error {
	if !exists {
		if transaction.TransactionClass != "open_trajectory" && transaction.TransactionClass != "projection_import" {
			return fmt.Errorf("supervision projection: trajectory does not exist")
		}
		if transaction.Expected.CanonicalEventHead != nil || transaction.Expected.LifecycleVersion != nil || transaction.ObservedBase != nil {
			return fmt.Errorf("supervision projection: initial transaction must expect an absent projection")
		}
		return nil
	}
	if state.Settled && transaction.TransactionClass != "archive_artifact" {
		return fmt.Errorf("supervision projection: trajectory is settled")
	}
	if transaction.Expected.CanonicalEventHead == nil || *transaction.Expected.CanonicalEventHead != previousHead {
		return fmt.Errorf("supervision projection: stale canonical event head")
	}
	if transaction.TransactionClass == "revise_artifact" &&
		(transaction.Expected.LifecycleVersion == nil || transaction.Expected.IntentRevisionID == nil || transaction.Expected.ArtifactHeadRevisionID == nil) {
		return fmt.Errorf("supervision projection: revise_artifact requires complete semantic expectations")
	}
	allowStaleSemanticBase := transaction.TransactionClass == "return_result"
	if transaction.Expected.LifecycleVersion != nil && *transaction.Expected.LifecycleVersion != state.LifecycleVersion && !allowStaleSemanticBase {
		return fmt.Errorf("supervision projection: stale lifecycle version")
	}
	if transaction.Expected.IntentRevisionID != nil && *transaction.Expected.IntentRevisionID != state.IntentRevisionID && !allowStaleSemanticBase {
		return fmt.Errorf("supervision projection: stale intent revision")
	}
	if transaction.Expected.ArtifactHeadRevisionID != nil && *transaction.Expected.ArtifactHeadRevisionID != state.ArtifactHeadRevisionID && !allowStaleSemanticBase {
		return fmt.Errorf("supervision projection: stale artifact head")
	}
	expectedProposal := ""
	if transaction.Expected.SettlementProposalID != nil {
		expectedProposal = *transaction.Expected.SettlementProposalID
	}
	if expectedProposal != state.SettlementProposalID {
		return fmt.Errorf("supervision projection: stale settlement proposal")
	}
	return nil
}

func applySupervisionMutation(state *supervisionProjectionState, transaction computerevent.SupervisionTransaction, kind string, body map[string]any) error {
	if state.Dispositions == nil {
		state.Dispositions = map[string]string{}
	}
	if state.StaleDispositions == nil {
		state.StaleDispositions = map[string]bool{}
	}
	if state.OwnerAttention == nil {
		state.OwnerAttention = map[string]bool{}
	}
	if observed, ok := body["observed_base"].(map[string]any); ok &&
		kind != "attempt_result" && !supervisionObservedBaseMatches(state, observed) {
		return fmt.Errorf("supervision projection: %s observed base is stale", kind)
	}
	if state.SettlementProposalID != "" && kind != "owner_decision_recorded" && kind != "trajectory_settled" {
		state.SettlementProposalID = ""
		state.SettlementSnapshotDigest = ""
	}
	switch kind {
	case "projection_imported":
		if state.LifecycleVersion != 0 {
			return fmt.Errorf("supervision projection: import requires an absent projection")
		}
		manifest, err := projectionImportFromMutationBody(body)
		if err != nil {
			return err
		}
		if err := applyProjectionImportManifest(state, transaction, manifest); err != nil {
			return err
		}
	case "trajectory_started":
		if state.LifecycleVersion != 0 || state.IntentRevisionID != "" {
			return fmt.Errorf("supervision projection: trajectory already started")
		}
		state.TrajectoryKind = bodyString(body, "trajectory_kind")
		if refs, ok := body["subject_refs"].(map[string]any); ok {
			state.SubjectRefs = make(map[string]string, len(refs))
			for key, value := range refs {
				state.SubjectRefs[key], _ = value.(string)
			}
		}
		state.IntentRevisionID = bodyString(body, "intent_revision_id")
		state.ArtifactID = bodyString(body, "artifact_id")
		state.ArtifactHeadRevisionID = bodyString(body, "artifact_revision_id")
		for _, assignmentID := range bodyStrings(body, "initial_assignment_ids") {
			if statusValue(state, "assignment", assignmentID) != "" {
				return fmt.Errorf("supervision projection: duplicate initial assignment %s", assignmentID)
			}
			setStatus(state, "assignment", assignmentID, "open")
		}
	case "intent_revised":
		parent := bodyNullableString(body, "parent_intent_revision_id")
		initialIntent := transaction.TransactionClass == "open_trajectory" && state.LifecycleVersion == 0 && bodyString(body, "intent_revision_id") == state.IntentRevisionID
		if state.IntentRevisionID != "" && !initialIntent && parent != state.IntentRevisionID {
			return fmt.Errorf("supervision projection: intent parent is stale")
		}
		if material, _ := body["material"].(bool); material {
			for _, target := range bodyObjects(body, "affected_targets") {
				targetKind, targetID := bodyString(target, "kind"), bodyString(target, "id")
				if bodyString(target, "prior_intent_revision_id") != state.IntentRevisionID {
					return fmt.Errorf("supervision projection: material rebase target intent is stale")
				}
				expectedDigest, err := supervisionTargetStateDigest(*state, targetKind, targetID)
				if err != nil {
					return fmt.Errorf("supervision projection: material rebase target: %w", err)
				}
				if bodyString(target, "state_digest") != expectedDigest {
					return fmt.Errorf("supervision projection: material rebase target state digest is stale")
				}
				if supervisionCurrentDisposition(*state, targetKind, targetID) != "" {
					state.StaleDispositions[supervisionTargetKey(targetKind, targetID)] = true
				}
				obligation := bodyString(target, "rebase_obligation_id")
				if statusValue(state, "rebase_obligation", obligation) != "" {
					return fmt.Errorf("supervision projection: duplicate rebase obligation %s", obligation)
				}
				state.OpenRebaseObligations[obligation] = true
				setStatus(state, "rebase_obligation", obligation, "open")
			}
		}
		state.IntentRevisionID = bodyString(body, "intent_revision_id")
	case "texture_revision":
		if state.ArtifactID != "" && bodyString(body, "artifact_id") != state.ArtifactID {
			return fmt.Errorf("supervision projection: artifact identity changed")
		}
		revisionID := bodyString(body, "revision_id")
		parentRevisionID := bodyNullableString(body, "parent_revision_id")
		initialRevision := transaction.TransactionClass == "open_trajectory" && state.LifecycleVersion == 0 &&
			revisionID == state.ArtifactHeadRevisionID && parentRevisionID == ""
		if !initialRevision && (parentRevisionID != state.ArtifactHeadRevisionID || revisionID == state.ArtifactHeadRevisionID) {
			return fmt.Errorf("supervision projection: artifact revision parent is stale")
		}
		if bodyString(body, "fulfills_intent_revision_id") != state.IntentRevisionID {
			return fmt.Errorf("supervision projection: artifact revision intent is stale")
		}
		state.ArtifactHeadRevisionID = revisionID
	case "assignment_opened":
		id := bodyString(body, "assignment_id")
		if status := statusValue(state, "assignment", id); status != "" &&
			(status != "open" || state.Entities["assignment_opened"][id] != nil) {
			return fmt.Errorf("supervision projection: duplicate assignment %s", id)
		}
		if bodyString(body, "intent_revision_id") != state.IntentRevisionID {
			return fmt.Errorf("supervision projection: assignment intent is stale")
		}
		if state.Entities["super_decision_proposed"][bodyString(body, "parent_decision_id")] == nil {
			return fmt.Errorf("supervision projection: assignment parent Super decision is unknown")
		}
		setStatus(state, "assignment", id, "open")
	case "attempt_started":
		assignment := bodyString(body, "assignment_id")
		if statusValue(state, "assignment", assignment) != "open" {
			return fmt.Errorf("supervision projection: attempt assignment is not open")
		}
		id := bodyString(body, "attempt_id")
		if statusValue(state, "attempt", id) != "" {
			return fmt.Errorf("supervision projection: duplicate attempt %s", id)
		}
		kind, ordinal := bodyString(body, "attempt_kind"), bodyUint(body, "ordinal")
		prior := bodyNullableString(body, "prior_attempt_id")
		if kind == "initial" && (ordinal != 1 || prior != "" || hasAssignmentAttempt(state, assignment)) {
			return fmt.Errorf("supervision projection: initial attempt lineage is invalid")
		}
		if kind == "retry" {
			priorAssignment, ok := supervisionAttemptAssignment(state, prior)
			if ordinal < 2 || !ok || priorAssignment != assignment || supervisionAttemptOrdinal(state, prior)+1 != ordinal {
				return fmt.Errorf("supervision projection: retry attempt lineage is invalid")
			}
		}
		setStatus(state, "attempt", id, "open")
	case "attempt_result":
		assignment, attempt := bodyString(body, "assignment_id"), bodyString(body, "attempt_id")
		attemptAssignment, knownAttempt := supervisionAttemptAssignment(state, attempt)
		if !knownAttempt || attemptAssignment != assignment {
			return fmt.Errorf("supervision projection: result does not bind its assignment attempt")
		}
		attemptStatus := statusValue(state, "attempt", attempt)
		late, _ := body["delivered_after_cancellation"].(bool)
		if late != (attemptStatus == "cancelled") {
			return fmt.Errorf("supervision projection: result late flag does not match cancellation")
		}
		if attemptStatus != "open" && attemptStatus != "cancelled" {
			return fmt.Errorf("supervision projection: result attempt is not active")
		}
		id := bodyString(body, "result_id")
		if statusValue(state, "result", id) != "" {
			return fmt.Errorf("supervision projection: duplicate result %s", id)
		}
		setStatus(state, "attempt", attempt, "returned")
		if late {
			if late && supervisionCurrentDisposition(*state, "attempt", attempt) != "" {
				state.StaleDispositions[supervisionTargetKey("attempt", attempt)] = true
			}
			setStatus(state, "result", id, "late")
		} else {
			setStatus(state, "result", id, "returned")
		}
		if observed, ok := body["observed_base"].(map[string]any); ok && !supervisionObservedBaseMatches(state, observed) {
			state.OpenRebaseObligations["rebase:"+id] = true
			setStatus(state, "rebase_obligation", "rebase:"+id, "open")
		}
	case "researcher_packet_recorded":
		id := bodyString(body, "packet_id")
		if statusValue(state, "update", id) != "" {
			return fmt.Errorf("supervision projection: duplicate researcher update %s", id)
		}
		setStatus(state, "update", id, "pending")
	case "super_finding_recorded":
		id := bodyString(body, "finding_id")
		if statusValue(state, "finding", id) != "" {
			return fmt.Errorf("supervision projection: duplicate finding %s", id)
		}
		findingState := bodyString(body, "state")
		if findingState != "open" && findingState != "resolved" {
			return fmt.Errorf("supervision projection: finding state is invalid")
		}
		if findingState == "open" {
			state.OpenFindings[id] = true
		}
		setStatus(state, "finding", id, findingState)
	case "dissent_recorded":
		id := bodyString(body, "dissent_id")
		if statusValue(state, "dissent", id) != "" {
			return fmt.Errorf("supervision projection: duplicate dissent %s", id)
		}
		state.OpenDissents[id] = true
		setStatus(state, "dissent", id, "open")
	case "assignment_cancelled":
		id := bodyString(body, "assignment_id")
		if statusValue(state, "assignment", id) != "open" {
			return fmt.Errorf("supervision projection: assignment is not open")
		}
		active := bodyStrings(body, "active_attempt_ids")
		if !sameStringSet(active, supervisionOpenAssignmentAttempts(state, id)) {
			return fmt.Errorf("supervision projection: cancellation active attempts do not match assignment")
		}
		setStatus(state, "assignment", id, "cancelled")
		for _, attempt := range active {
			setStatus(state, "attempt", attempt, "cancelled")
		}
	case "disposition_recorded":
		target, _ := body["target"].(map[string]any)
		targetKind, targetID, value := bodyString(target, "kind"), bodyString(target, "id"), bodyString(body, "value")
		if !knownDispositionTarget(state, targetKind, targetID) {
			return fmt.Errorf("supervision projection: unknown disposition target %s %s", targetKind, targetID)
		}
		if !allowedDisposition(targetKind, value) {
			return fmt.Errorf("supervision projection: disposition %s is invalid for %s", value, targetKind)
		}
		prior := bodyNullableString(body, "prior_disposition_id")
		current := supervisionCurrentDisposition(*state, targetKind, targetID)
		if prior != current {
			return fmt.Errorf("supervision projection: disposition prior lineage is stale")
		}
		if value == "compensation_required" {
			obligation := bodyNullableString(body, "compensation_obligation_id")
			if obligation == "" || statusValue(state, "compensation_obligation", obligation) != "" {
				return fmt.Errorf("supervision projection: compensation obligation is invalid")
			}
			state.OpenCompensationObligations[obligation] = true
			setStatus(state, "compensation_obligation", obligation, "open")
		}
		setDisposition(state, targetKind, targetID, bodyString(body, "disposition_id"))
		delete(state.StaleDispositions, supervisionTargetKey(targetKind, targetID))
		if targetKind == "assignment" && statusValue(state, targetKind, targetID) != "cancelled" {
			setStatus(state, targetKind, targetID, "closed")
		}
		switch targetKind {
		case "rebase_obligation":
			delete(state.OpenRebaseObligations, targetID)
		case "compensation_obligation":
			delete(state.OpenCompensationObligations, targetID)
		case "finding":
			delete(state.OpenFindings, targetID)
		case "dissent":
			delete(state.OpenDissents, targetID)
		}
	case "super_decision_proposed":
		if bodyString(body, "reserved_authority") == "owner" {
			state.OwnerAttention[bodyString(body, "decision_id")] = true
		}
	case "owner_decision_recorded":
		delete(state.OwnerAttention, bodyString(body, "proposal_id"))
		if bodyString(body, "decision") == "" {
			break
		}
		if bodyString(body, "proposal_id") != state.SettlementProposalID ||
			bodyString(body, "settlement_snapshot_digest") != state.SettlementSnapshotDigest {
			return fmt.Errorf("supervision projection: owner settlement decision is stale")
		}
		switch bodyString(body, "decision") {
		case "accept":
		case "revise":
			state.SettlementProposalID = ""
			state.SettlementSnapshotDigest = ""
		default:
			return fmt.Errorf("supervision projection: owner settlement decision is invalid")
		}
	case "settlement_proposed":
		if err := state.settlementReady(); err != nil {
			return err
		}
		settlementHead := state.CanonicalEventHead
		if transaction.Expected.CanonicalEventHead != nil {
			settlementHead = *transaction.Expected.CanonicalEventHead
		}
		if bodyString(body, "canonical_event_head") != settlementHead ||
			bodyString(body, "intent_revision_id") != state.IntentRevisionID ||
			bodyString(body, "artifact_head_revision_id") != state.ArtifactHeadRevisionID {
			return fmt.Errorf("supervision projection: settlement semantic head is stale")
		}
		if err := exactSettlementEntitySets(*state, body); err != nil {
			return err
		}
		snapshotDigest, err := supervisionSettlementSnapshotDigest(*state)
		if err != nil {
			return err
		}
		evidenceRefs := bodyStrings(body, "evidence_refs")
		if len(evidenceRefs) == 0 {
			return fmt.Errorf("supervision projection: settlement evidence is empty")
		}
		retainedRefs := state.ReferencedArtifacts
		for _, ref := range evidenceRefs {
			if _, err := computerevent.ParseArtifactRef(ref); err != nil {
				return fmt.Errorf("supervision projection: settlement evidence ref is invalid")
			}
			if !retainedRefs[ref] {
				return fmt.Errorf("supervision projection: settlement evidence ref is not retained")
			}
		}
		if len(state.StaleDispositions) > 0 {
			return fmt.Errorf("supervision projection: stale reconciliation dispositions")
		}
		if len(state.OwnerAttention) > 0 {
			return fmt.Errorf("supervision projection: owner attention is unresolved")
		}
		if bodyString(body, "snapshot_digest") != snapshotDigest {
			return fmt.Errorf("supervision projection: settlement snapshot digest is stale")
		}
		state.SettlementProposalID = bodyString(body, "proposal_id")
		state.SettlementSnapshotDigest = snapshotDigest
	case "trajectory_settled":
		proposalID, snapshotDigest := bodyString(body, "proposal_id"), bodyString(body, "snapshot_digest")
		if state.SettlementProposalID == "" || proposalID != state.SettlementProposalID || snapshotDigest != state.SettlementSnapshotDigest {
			return fmt.Errorf("supervision projection: settlement proposal is stale")
		}
		decisionID := bodyString(body, "owner_decision_id")
		rawDecision, ok := state.Entities["owner_decision_recorded"][decisionID]
		if !ok {
			return fmt.Errorf("supervision projection: fresh owner settlement acceptance is missing")
		}
		decision, err := supervisionBodyMap(rawDecision)
		if err != nil || bodyString(decision, "decision") != "accept" ||
			bodyString(decision, "proposal_id") != proposalID ||
			bodyString(decision, "settlement_snapshot_digest") != snapshotDigest ||
			bodyString(decision, "owner_actor_id") != transaction.Actor.ActorID {
			return fmt.Errorf("supervision projection: fresh owner settlement acceptance is invalid")
		}
		if err := state.settlementReady(); err != nil {
			return err
		}
		state.Settled = true
		state.SettlementProposalID = ""
		state.SettlementSnapshotDigest = ""
	case "artifact_archived":
		if !state.Settled {
			return fmt.Errorf("supervision projection: archive requires settlement")
		}
		if bodyString(body, "artifact_id") != state.ArtifactID || bodyString(body, "head_revision_id") != state.ArtifactHeadRevisionID {
			return fmt.Errorf("supervision projection: archive head is stale")
		}
		state.Archived = true
	}
	return nil
}

func supervisionObservedBaseMatches(state *supervisionProjectionState, observed map[string]any) bool {
	return bodyString(observed, "canonical_event_head") == state.CanonicalEventHead &&
		bodyString(observed, "intent_revision_id") == state.IntentRevisionID &&
		bodyString(observed, "artifact_head_revision_id") == state.ArtifactHeadRevisionID
}

func (state supervisionProjectionState) settlementReady() error {
	for _, kind := range []string{"assignment", "attempt", "result", "update", "finding", "dissent", "rebase_obligation", "compensation_obligation"} {
		for id, status := range state.Statuses[kind] {
			if kind == "finding" && status == "resolved" {
				continue
			}
			if (kind == "assignment" || kind == "attempt") && status == "open" {
				return fmt.Errorf("supervision projection: %s %s is operationally open", kind, id)
			}
			if supervisionCurrentDisposition(state, kind, id) == "" {
				return fmt.Errorf("supervision projection: %s %s lacks disposition", kind, id)
			}
		}
	}
	if len(state.StaleDispositions) > 0 {
		return fmt.Errorf("supervision projection: stale reconciliation dispositions")
	}
	if len(state.OwnerAttention) > 0 {
		return fmt.Errorf("supervision projection: owner attention is unresolved")
	}
	if len(state.OpenRebaseObligations) > 0 || len(state.OpenCompensationObligations) > 0 || len(state.OpenFindings) > 0 || len(state.OpenDissents) > 0 {
		return fmt.Errorf("supervision projection: unresolved supervision obligations")
	}
	if state.IntentRevisionID == "" || state.ArtifactHeadRevisionID == "" {
		return fmt.Errorf("supervision projection: semantic heads are incomplete")
	}
	return nil
}
func supervisionDerivedObjects(state supervisionProjectionState, transaction computerevent.SupervisionTransaction, sequence uint64, now time.Time) ([]objectgraph.Object, error) {
	if transaction.TransactionClass == "projection_import" && len(transaction.Mutations) == 1 && transaction.Mutations[0].Kind == "projection_imported" {
		body, err := supervisionBodyMap(transaction.Mutations[0].Body)
		if err != nil {
			return nil, err
		}
		manifest, err := projectionImportFromMutationBody(body)
		if err != nil {
			return nil, err
		}
		return supervisionImportedDerivedObjects(manifest, sequence)
	}
	created := state.CreatedAt
	if created.IsZero() {
		created = now
	}
	trajectory := types.TrajectoryRecord{
		TrajectoryID: state.TrajectoryID, OwnerID: state.OwnerID, ComputerID: state.ComputerID,
		Kind: types.TrajectoryKind(state.TrajectoryKind), Status: types.TrajectoryLive,
		SubjectRefs:      state.SubjectRefs,
		LifecycleVersion: int64(state.LifecycleVersion), ReducerSeq: int64(state.ProjectionCursor),
		TerminalArtifactHeadRef: state.ArtifactHeadRevisionID, CreatedAt: created, UpdatedAt: now,
	}
	if state.Settled {
		trajectory.Status = types.TrajectorySettled
		trajectory.SettledAt = &now
	}
	trajectoryObj, err := lifecycleObject(ogKindTrajectory, state.OwnerID, state.ComputerID, state.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", state.TrajectoryID, state.ComputerID, state.TrajectoryID, int64(state.ProjectionCursor)), created, now)
	if err != nil {
		return nil, err
	}
	objects := []objectgraph.Object{trajectoryObj}
	for index, mutation := range transaction.Mutations {
		body, err := supervisionBodyMap(mutation.Body)
		if err != nil {
			return nil, err
		}
		cursor := state.ProjectionCursor - uint64(len(transaction.Mutations)) + uint64(index+1)
		switch mutation.Kind {
		case "trajectory_started":
			textureActorID := bodyString(body, "texture_actor_id")
			agent := types.AgentRecord{
				AgentID: textureActorID, OwnerID: state.OwnerID, ComputerID: state.ComputerID, SandboxID: state.ComputerID,
				Profile: "texture", Role: "texture", ChannelID: state.ArtifactID,
				LifecycleVersion: int64(state.LifecycleVersion), LastReducerSeq: int64(cursor), CreatedAt: created, UpdatedAt: now,
			}
			agentObj, err := lifecycleObject(ogKindAgent, state.OwnerID, state.ComputerID, textureActorID, agent, lifecycleMetadata("agent_id", textureActorID, state.ComputerID, state.TrajectoryID, int64(cursor)), created, now)
			if err != nil {
				return nil, err
			}
			objects = append(objects, agentObj)
			for _, assignmentID := range bodyStrings(body, "initial_assignment_ids") {
				work, ok := supervisionWorkItem(state, assignmentID, cursor, now)
				if !ok {
					continue
				}
				workObj, err := lifecycleObject(ogKindWorkItem, state.OwnerID, state.ComputerID, assignmentID, work, lifecycleMetadata("work_item_id", assignmentID, state.ComputerID, state.TrajectoryID, int64(cursor)), work.CreatedAt, now)
				if err != nil {
					return nil, err
				}
				objects = append(objects, workObj)
			}
		case "assignment_opened", "assignment_cancelled", "disposition_recorded":
			assignmentID := bodyString(body, "assignment_id")
			if mutation.Kind == "disposition_recorded" {
				target, _ := body["target"].(map[string]any)
				if bodyString(target, "kind") != "assignment" {
					continue
				}
				assignmentID = bodyString(target, "id")
			}
			work, ok := supervisionWorkItem(state, assignmentID, cursor, now)
			if !ok {
				continue
			}
			obj, err := lifecycleObject(ogKindWorkItem, state.OwnerID, state.ComputerID, assignmentID, work, lifecycleMetadata("work_item_id", assignmentID, state.ComputerID, state.TrajectoryID, int64(cursor)), work.CreatedAt, now)
			if err != nil {
				return nil, err
			}
			objects = append(objects, obj)
			if mutation.Kind == "assignment_opened" {
				agentID := bodyString(body, "assigned_actor_id")
				agent := types.AgentRecord{
					AgentID: agentID, OwnerID: state.OwnerID, ComputerID: state.ComputerID, SandboxID: state.ComputerID,
					Profile: bodyString(body, "assigned_role"), Role: bodyString(body, "assigned_role"), ChannelID: state.ArtifactID,
					LifecycleVersion: int64(state.LifecycleVersion), LastReducerSeq: int64(cursor), CreatedAt: work.CreatedAt, UpdatedAt: now,
				}
				agentObj, err := lifecycleObject(ogKindAgent, state.OwnerID, state.ComputerID, agentID, agent, lifecycleMetadata("agent_id", agentID, state.ComputerID, state.TrajectoryID, int64(cursor)), work.CreatedAt, now)
				if err != nil {
					return nil, err
				}
				objects = append(objects, agentObj)
			}
		case "texture_revision":
			documentID, revisionID := bodyString(body, "artifact_id"), bodyString(body, "revision_id")
			document := types.Document{DocID: documentID, OwnerID: state.OwnerID, ComputerID: state.ComputerID, TrajectoryID: state.TrajectoryID, Title: bodyString(body, "title"), CurrentRevisionID: revisionID, CreatedAt: created, UpdatedAt: now}
			documentObj, err := lifecycleObject(ogKindTexDoc, state.OwnerID, state.ComputerID, documentID, document, lifecycleMetadata("doc_id", documentID, state.ComputerID, state.TrajectoryID, int64(cursor)), created, now)
			if err != nil {
				return nil, err
			}
			authorKind := types.AuthorAppAgent
			if transaction.Actor.Role == "owner" {
				authorKind = types.AuthorUser
			}
			revisionMetadata, err := json.Marshal(body["metadata"])
			if err != nil {
				return nil, fmt.Errorf("supervision projection: revision metadata: %w", err)
			}
			bodyDoc, sourceEntities, citations, err := supervisionStructuredRevisionSourceGraph(body)
			if err != nil {
				return nil, err
			}
			parentRevisionID := bodyNullableString(body, "parent_revision_id")
			parentHash, version, err := supervisionRevisionLineage(state, parentRevisionID)
			if err != nil {
				return nil, err
			}
			provenance := json.RawMessage(`{}`)
			revision := types.Revision{
				RevisionID: revisionID, DocID: documentID, OwnerID: state.OwnerID, ComputerID: state.ComputerID,
				AuthorKind: authorKind, AuthorLabel: transaction.Actor.ActorID, VersionNumber: version,
				Content: bodyString(body, "content"), BodyDoc: bodyDoc, SourceEntities: sourceEntities, Citations: citations, Metadata: revisionMetadata,
				Provenance: provenance, ParentRevisionID: parentRevisionID, TrajectoryID: state.TrajectoryID,
				RevisionHash: types.ComputeStructuredRevisionHash(parentHash, bodyString(body, "content"), bodyDoc, sourceEntities, provenance), CreatedAt: now,
			}
			revisionObj, err := lifecycleObject(ogKindTexRev, state.OwnerID, state.ComputerID, revisionID, revision, lifecycleMetadata("revision_id", revisionID, state.ComputerID, state.TrajectoryID, int64(cursor)), now, now)
			if err != nil {
				return nil, err
			}
			objects = append(objects, documentObj, revisionObj)
		case "artifact_archived":
			documentID := bodyString(body, "artifact_id")
			document := types.Document{DocID: documentID, OwnerID: state.OwnerID, ComputerID: state.ComputerID, TrajectoryID: state.TrajectoryID, Title: supervisionDocumentTitle(state), CurrentRevisionID: state.ArtifactHeadRevisionID, CreatedAt: created, UpdatedAt: now, ArchivedAt: &now}
			documentObj, err := lifecycleObject(ogKindTexDoc, state.OwnerID, state.ComputerID, documentID, document, lifecycleMetadata("doc_id", documentID, state.ComputerID, state.TrajectoryID, int64(cursor)), created, now)
			if err != nil {
				return nil, err
			}
			objects = append(objects, documentObj)
		}
	}
	return objects, nil
}

func supervisionWorkItem(state supervisionProjectionState, assignmentID string, cursor uint64, now time.Time) (types.WorkItemRecord, bool) {
	body, authorityProfile, assignedActorID, objective := map[string]any(nil), "cosuper", "", "supervision assignment "+assignmentID
	if raw, ok := state.Entities["assignment_opened"][assignmentID]; ok {
		decoded, err := supervisionBodyMap(raw)
		if err != nil {
			return types.WorkItemRecord{}, false
		}
		body = decoded
		assignedActorID = bodyString(body, "assigned_actor_id")
	} else {
		for _, raw := range state.Entities["trajectory_started"] {
			decoded, err := supervisionBodyMap(raw)
			if err != nil || !containsStringValue(bodyStrings(decoded, "initial_assignment_ids"), assignmentID) {
				continue
			}
			body, authorityProfile = decoded, "texture"
			assignedActorID = bodyString(body, "texture_actor_id")
			if value := bodyString(body, "objective"); value != "" {
				objective = value
			}
			break
		}
		if body == nil {
			return types.WorkItemRecord{}, false
		}
	}
	status := types.WorkItemOpen
	switch state.Statuses["assignment"][assignmentID] {
	case "cancelled":
		status = types.WorkItemCancelled
	case "rejected":
		status = types.WorkItemRefused
	case "preserved", "incorporated", "superseded":
		status = types.WorkItemCompleted
	}
	return types.WorkItemRecord{
		WorkItemID: assignmentID, TrajectoryID: state.TrajectoryID, OwnerID: state.OwnerID, ComputerID: state.ComputerID,
		Objective: objective, AuthorityProfile: authorityProfile,
		ObjectiveFingerprint: bodyString(body, "scope_digest"), Status: status,
		LifecycleVersion: int64(state.LifecycleVersion), LastReducerSeq: int64(cursor),
		AssignedAgentID: assignedActorID, Details: body,
		CreatedAt: state.CreatedAt, UpdatedAt: now,
	}, true
}

func containsStringValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func bodyUint(body map[string]any, key string) uint64 {
	number, ok := body[key].(json.Number)
	if !ok {
		return 0
	}
	value, err := strconv.ParseUint(number.String(), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func supervisionTargetKey(kind, id string) string { return kind + ":" + id }

func setDisposition(state *supervisionProjectionState, kind, id, dispositionID string) {
	if state.Dispositions == nil {
		state.Dispositions = map[string]string{}
	}
	state.Dispositions[supervisionTargetKey(kind, id)] = dispositionID
}

func supervisionTargetStateDigest(state supervisionProjectionState, kind, id string) (string, error) {
	if kind == "belief" {
		raw := state.Entities["super_belief_recorded"][id]
		if raw == nil {
			return "", fmt.Errorf("unknown supervision belief")
		}
		body, err := supervisionBodyMap(raw)
		if err != nil {
			return "", err
		}
		canonical, err := computerevent.CanonicalJSON(body)
		if err != nil {
			return "", err
		}
		return computerevent.DigestBytes(canonical), nil
	}
	if kind == "artifact_premise" {
		if id != state.ArtifactID {
			return "", fmt.Errorf("unknown artifact premise")
		}
		canonical, err := computerevent.CanonicalJSON(map[string]string{"artifact_id": state.ArtifactID, "artifact_head_revision_id": state.ArtifactHeadRevisionID, "intent_revision_id": state.IntentRevisionID})
		if err != nil {
			return "", err
		}
		return computerevent.DigestBytes(canonical), nil
	}
	status := statusValue(&state, kind, id)
	if status == "" {
		return "", fmt.Errorf("unknown supervision target")
	}
	canonical, err := computerevent.CanonicalJSON(map[string]any{
		"kind": kind, "id": id, "status": status, "disposition_id": supervisionCurrentDisposition(state, kind, id),
	})
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

func supervisionAttemptAssignment(state *supervisionProjectionState, attemptID string) (string, bool) {
	raw := state.Entities["attempt_started"][attemptID]
	if raw == nil {
		return "", false
	}
	body, err := supervisionBodyMap(raw)
	if err != nil {
		return "", false
	}
	assignmentID := bodyString(body, "assignment_id")
	return assignmentID, assignmentID != ""
}

func supervisionAttemptOrdinal(state *supervisionProjectionState, attemptID string) uint64 {
	raw := state.Entities["attempt_started"][attemptID]
	if raw == nil {
		return 0
	}
	body, err := supervisionBodyMap(raw)
	if err != nil {
		return 0
	}
	return bodyUint(body, "ordinal")
}

func hasAssignmentAttempt(state *supervisionProjectionState, assignmentID string) bool {
	for attemptID := range state.Statuses["attempt"] {
		if assignment, ok := supervisionAttemptAssignment(state, attemptID); ok && assignment == assignmentID {
			return true
		}
	}
	return false
}

func supervisionOpenAssignmentAttempts(state *supervisionProjectionState, assignmentID string) []string {
	ids := make([]string, 0)
	for attemptID, status := range state.Statuses["attempt"] {
		if status != "open" {
			continue
		}
		if assignment, ok := supervisionAttemptAssignment(state, attemptID); ok && assignment == assignmentID {
			ids = append(ids, attemptID)
		}
	}
	sort.Strings(ids)
	return ids
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy, rightCopy := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	return slicesEqual(leftCopy, rightCopy)
}

func slicesEqual(left, right []string) bool {
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func knownDispositionTarget(state *supervisionProjectionState, kind, id string) bool {
	return containsString([]string{"assignment", "attempt", "result", "update", "finding", "dissent", "rebase_obligation", "compensation_obligation"}, kind) &&
		statusValue(state, kind, id) != ""
}

func allowedDisposition(kind, value string) bool {
	allowed := map[string][]string{
		"assignment":              {"preserved", "invalidated", "superseded", "compensation_required", "cancelled", "rejected"},
		"attempt":                 {"preserved", "invalidated", "superseded", "compensation_required", "cancelled", "rejected"},
		"result":                  {"preserved", "invalidated", "superseded", "compensation_required", "late", "incorporated", "rejected"},
		"update":                  {"preserved", "invalidated", "superseded", "compensation_required", "incorporated", "rejected"},
		"finding":                 {"preserved", "invalidated", "superseded", "compensation_required", "incorporated", "rejected"},
		"dissent":                 {"preserved", "invalidated", "superseded", "compensation_required", "incorporated", "rejected"},
		"rebase_obligation":       {"preserved", "invalidated", "superseded", "compensation_required"},
		"compensation_obligation": {"preserved", "incorporated", "rejected"},
	}
	return containsString(allowed[kind], value)
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func supervisionCurrentDisposition(state supervisionProjectionState, targetKind, targetID string) string {
	if state.Dispositions != nil {
		if dispositionID := state.Dispositions[supervisionTargetKey(targetKind, targetID)]; dispositionID != "" {
			return dispositionID
		}
	}
	leaves := make(map[string]bool)
	for dispositionID, raw := range state.Entities["disposition_recorded"] {
		body, err := supervisionBodyMap(raw)
		if err != nil {
			return ""
		}
		target, _ := body["target"].(map[string]any)
		if bodyString(target, "kind") != targetKind || bodyString(target, "id") != targetID {
			continue
		}
		leaves[dispositionID] = true
	}
	if len(leaves) == 0 {
		return ""
	}
	for _, raw := range state.Entities["disposition_recorded"] {
		body, err := supervisionBodyMap(raw)
		if err != nil {
			return ""
		}
		if prior := bodyNullableString(body, "prior_disposition_id"); prior != "" {
			delete(leaves, prior)
		}
	}
	if len(leaves) != 1 {
		return ""
	}
	for dispositionID := range leaves {
		return dispositionID
	}
	return ""
}

func exactSettlementEntitySets(state supervisionProjectionState, body map[string]any) error {
	expected := map[string][]string{
		"assignment_ids":              statusIDs(state, "assignment"),
		"attempt_ids":                 statusIDs(state, "attempt"),
		"result_ids":                  statusIDs(state, "result"),
		"update_ids":                  statusIDs(state, "update"),
		"disposition_ids":             entityIDs(state, "disposition_recorded"),
		"finding_ids":                 statusIDs(state, "finding"),
		"dissent_ids":                 statusIDs(state, "dissent"),
		"rebase_obligation_ids":       statusIDs(state, "rebase_obligation"),
		"compensation_obligation_ids": statusIDs(state, "compensation_obligation"),
		"owner_attention_ids":         {},
	}
	for field, ids := range expected {
		if !sameStringSet(bodyStrings(body, field), ids) {
			return fmt.Errorf("supervision projection: settlement %s does not match current entities", field)
		}
	}
	return nil
}

func statusIDs(state supervisionProjectionState, kind string) []string {
	ids := make([]string, 0, len(state.Statuses[kind]))
	for id := range state.Statuses[kind] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func entityIDs(state supervisionProjectionState, kind string) []string {
	ids := make([]string, 0, len(state.Entities[kind]))
	for id := range state.Entities[kind] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func supervisionEntityCount(state supervisionProjectionState) int {
	count := 0
	for _, entities := range state.Entities {
		count += len(entities)
	}
	return count
}

func supervisionSettlementSnapshotDigest(state supervisionProjectionState) (string, error) {
	canonical, err := computerevent.CanonicalJSON(state)
	if err != nil {
		return "", fmt.Errorf("supervision projection: canonical settlement snapshot: %w", err)
	}
	return computerevent.DigestBytes(canonical), nil
}

func supervisionBodyMap(raw json.RawMessage) (map[string]any, error) {
	var body map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return nil, err
	}
	return body, nil
}

func supervisionStructuredRevisionSourceGraph(body map[string]any) (json.RawMessage, json.RawMessage, json.RawMessage, error) {
	graph, ok := body["source_graph"].(map[string]any)
	if !ok {
		return nil, nil, nil, fmt.Errorf("supervision projection: texture revision source graph is invalid")
	}
	encode := func(key string) (json.RawMessage, error) {
		value, present := graph[key]
		if !present || value == nil {
			return nil, nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(raw), nil
	}
	bodyDoc, err := encode("body_doc")
	if err != nil {
		return nil, nil, nil, err
	}
	sourceEntities, err := encode("source_entities")
	if err != nil {
		return nil, nil, nil, err
	}
	citations, err := encode("citations")
	if err != nil {
		return nil, nil, nil, err
	}
	return bodyDoc, sourceEntities, citations, nil
}

func supervisionRevisionLineage(state supervisionProjectionState, revisionID string) (string, int, error) {
	if revisionID == "" {
		return "", 0, nil
	}
	raw := state.Entities["texture_revision"][revisionID]
	if raw == nil {
		return "", 0, fmt.Errorf("supervision projection: revision parent is unknown")
	}
	body, err := supervisionBodyMap(raw)
	if err != nil {
		return "", 0, err
	}
	parentHash, parentVersion, err := supervisionRevisionLineage(state, bodyNullableString(body, "parent_revision_id"))
	if err != nil {
		return "", 0, err
	}
	bodyDoc, sourceEntities, _, err := supervisionStructuredRevisionSourceGraph(body)
	if err != nil {
		return "", 0, err
	}
	return types.ComputeStructuredRevisionHash(parentHash, bodyString(body, "content"), bodyDoc, sourceEntities, json.RawMessage(`{}`)), parentVersion + 1, nil
}
func supervisionDocumentTitle(state supervisionProjectionState) string {
	raw := state.Entities["texture_revision"][state.ArtifactHeadRevisionID]
	if raw == nil {
		return state.ArtifactID
	}
	body, err := supervisionBodyMap(raw)
	if err != nil {
		return state.ArtifactID
	}
	title := bodyString(body, "title")
	if title == "" {
		return state.ArtifactID
	}
	return title
}

func bodyString(body map[string]any, key string) string { value, _ := body[key].(string); return value }
func bodyNullableString(body map[string]any, key string) string {
	if body[key] == nil {
		return ""
	}
	return bodyString(body, key)
}
func bodyStrings(body map[string]any, key string) []string {
	raw, _ := body[key].([]any)
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		if text, ok := value.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
func bodyObjects(body map[string]any, key string) []map[string]any {
	raw, _ := body[key].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, value := range raw {
		if obj, ok := value.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}
func setStatus(state *supervisionProjectionState, kind, id, status string) {
	if state.Statuses[kind] == nil {
		state.Statuses[kind] = map[string]string{}
	}
	state.Statuses[kind][id] = status
}
func statusValue(state *supervisionProjectionState, kind, id string) string {
	if state.Statuses[kind] == nil {
		return ""
	}
	return state.Statuses[kind][id]
}
func supervisionMutationID(kind string, body map[string]any) (string, error) {
	keys := map[string][]string{
		"projection_imported": {"import_digest"}, "trajectory_started": {"intent_revision_id"},
		"intent_revised": {"intent_revision_id"}, "texture_revision": {"revision_id"},
		"actor_message_recorded": {"message_id"}, "actor_message_acknowledged": {"message_id"}, "researcher_packet_recorded": {"packet_id"},
		"assignment_opened": {"assignment_id"}, "attempt_started": {"attempt_id"},
		"attempt_result": {"result_id"}, "super_belief_recorded": {"belief_id"},
		"super_finding_recorded": {"finding_id"}, "dissent_recorded": {"dissent_id"},
		"super_reconciliation_recorded": {"reconciliation_id"}, "super_decision_proposed": {"decision_id"},
		"owner_decision_recorded": {"decision_id"}, "assignment_cancelled": {"assignment_id"},
		"disposition_recorded": {"disposition_id"}, "settlement_proposed": {"proposal_id"},
		"trajectory_settled": {"settlement_id"}, "artifact_archived": {"artifact_id"},
	}
	for _, key := range keys[kind] {
		if value := bodyString(body, key); value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("supervision projection: %s identity is missing", kind)
}

func supervisionArtifactRefs(body map[string]any) []string {
	seen := map[string]struct{}{}
	for key, value := range body {
		if !strings.Contains(key, "artifact_ref") && !strings.Contains(key, "receipt_ref") && key != "import_ref" && key != "source_graph_ref" {
			continue
		}
		switch typed := value.(type) {
		case string:
			seen[typed] = struct{}{}
		case []any:
			for _, item := range typed {
				if text, ok := item.(string); ok {
					seen[text] = struct{}{}
				}
			}
		}
	}
	refs := make([]string, 0, len(seen))
	for ref := range seen {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}

const supervisionProjectionControlLimit = 8

// GetSupervisionProjectionSnapshot returns the bounded, owner-readable
// projection of a finalized supervision tape. It is a read model only: the
// exact validated mutation bodies and refs remain the drill-down source.
func (s *Store) GetSupervisionProjectionSnapshot(ctx context.Context, ownerID, computerID, trajectoryID string) (types.SupervisionProjectionSnapshot, error) {
	if s == nil || s.ogStore == nil {
		return types.SupervisionProjectionSnapshot{}, fmt.Errorf("supervision projection: store unavailable")
	}
	stateID, err := lifecycleCanonicalID(ogKindSupervisionState, ownerID, computerID, trajectoryID)
	if err != nil {
		return types.SupervisionProjectionSnapshot{}, err
	}
	object, err := s.ogStore.GetObject(ctx, stateID)
	if err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return types.SupervisionProjectionSnapshot{}, ErrNotFound
		}
		return types.SupervisionProjectionSnapshot{}, fmt.Errorf("supervision projection: load state: %w", err)
	}
	var state supervisionProjectionState
	if err := json.Unmarshal(object.Body, &state); err != nil {
		return types.SupervisionProjectionSnapshot{}, fmt.Errorf("supervision projection: decode state: %w", err)
	}
	if state.OwnerID != ownerID || state.ComputerID != computerID || state.TrajectoryID != trajectoryID {
		return types.SupervisionProjectionSnapshot{}, ErrNotFound
	}
	snapshot := projectSupervisionSnapshot(state)
	head, err := s.Head(ctx, computerID)
	if err != nil {
		return types.SupervisionProjectionSnapshot{}, fmt.Errorf("supervision projection: load canonical head: %w", err)
	}
	snapshot.CanonicalEventHead = head.CanonicalEventHead
	return snapshot, nil
}

func projectSupervisionSnapshot(state supervisionProjectionState) types.SupervisionProjectionSnapshot {
	snapshot := types.SupervisionProjectionSnapshot{
		Schema: computerevent.SupervisionSchemaV1, OwnerID: state.OwnerID, ComputerID: state.ComputerID,
		TrajectoryID: state.TrajectoryID, CanonicalEventHead: state.CanonicalEventHead, ObservedCanonicalEventHead: state.CanonicalEventHead,
		SnapshotCursor: int64(state.ProjectionCursor), LifecycleVersion: int64(state.LifecycleVersion),
		IntentRevisionID: state.IntentRevisionID, ArtifactHeadRevisionID: state.ArtifactHeadRevisionID,
		SettlementProposalID: state.SettlementProposalID, Settled: state.Settled, Archived: state.Archived,
		Attempts: []types.SupervisionProjectionEntry{}, Results: []types.SupervisionProjectionEntry{},
		Dispositions: []types.SupervisionProjectionEntry{}, Findings: []types.SupervisionProjectionEntry{},
		Reconciliations: []types.SupervisionProjectionEntry{},
		Control: types.SupervisionProjectionControl{
			Messages: []types.SupervisionProjectionEntry{}, Obligations: []types.SupervisionProjectionEntry{},
			Blockers: []types.SupervisionProjectionEntry{}, Dissent: []types.SupervisionProjectionEntry{},
			Decisions: []types.SupervisionProjectionEntry{}, Rebase: []types.SupervisionProjectionEntry{},
		},
	}
	byKind := func(kind string) []types.SupervisionProjectionEntry {
		records := make([]types.SupervisionProjectionEntry, 0, len(state.Entities[kind]))
		ids := make([]string, 0, len(state.Entities[kind]))
		for id := range state.Entities[kind] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			raw := append(json.RawMessage(nil), state.Entities[kind][id]...)
			body, err := supervisionBodyMap(raw)
			if err != nil {
				continue
			}
			records = append(records, types.SupervisionProjectionEntry{
				ID: id, Kind: kind, Status: supervisionEntryStatus(state, kind, id, body), Body: raw,
				ArtifactRefs: supervisionArtifactRefs(body), EvidenceRefs: bodyStrings(body, "evidence_refs"),
			})
		}
		return records
	}
	intentEntries := byKind("intent_revised")
	for _, entry := range intentEntries {
		if entry.ID == state.IntentRevisionID {
			entryCopy := entry
			snapshot.Control.Intent = &entryCopy
			if body, err := supervisionBodyMap(entry.Body); err == nil && bodyNullableString(body, "parent_intent_revision_id") != "" {
				deltaCopy := entry
				snapshot.Control.LatestDelta = &deltaCopy
			}
			break
		}
	}
	beliefs := byKind("super_belief_recorded")
	if belief := latestSupervisionBelief(beliefs); belief != nil {
		snapshot.Control.Belief = belief
	}
	assignments := byKind("assignment_opened")
	snapshot.Control.Obligations, snapshot.Control.OverflowCount = boundedSupervisionEntries(assignments, supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	snapshot.Attempts, snapshot.Control.OverflowCount = boundedSupervisionEntries(byKind("attempt_started"), supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	snapshot.Results, snapshot.Control.OverflowCount = boundedSupervisionEntries(byKind("attempt_result"), supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	snapshot.Dispositions, snapshot.Control.OverflowCount = boundedSupervisionEntries(byKind("disposition_recorded"), supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	snapshot.Findings, snapshot.Control.OverflowCount = boundedSupervisionEntries(byKind("super_finding_recorded"), supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	snapshot.Reconciliations, snapshot.Control.OverflowCount = boundedSupervisionEntries(byKind("super_reconciliation_recorded"), supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	snapshot.Control.Dissent, snapshot.Control.OverflowCount = boundedSupervisionEntries(byKind("dissent_recorded"), supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	decisions := append(byKind("super_decision_proposed"), byKind("owner_decision_recorded")...)
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].ID < decisions[j].ID })
	for _, decision := range decisions {
		body, err := supervisionBodyMap(decision.Body)
		if err == nil && bodyString(body, "reserved_authority") == "owner" {
			snapshot.Control.AttentionCount++
		}
	}
	snapshot.Control.Decisions, snapshot.Control.OverflowCount = boundedSupervisionEntries(decisions, supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	snapshot.Control.Blockers, snapshot.Control.Rebase = supervisionProjectionObligations(state)
	snapshot.Control.Blockers, snapshot.Control.OverflowCount = boundedSupervisionEntries(snapshot.Control.Blockers, supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	snapshot.Control.Rebase, snapshot.Control.OverflowCount = boundedSupervisionEntries(snapshot.Control.Rebase, supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	proposals := byKind("settlement_proposed")
	if state.SettlementProposalID != "" {
		for _, proposal := range proposals {
			if proposal.ID == state.SettlementProposalID {
				proposalCopy := proposal
				snapshot.Control.Settlement = &proposalCopy
				break
			}
		}
	}
	messages := byKind("actor_message_recorded")
	snapshot.Control.Messages, snapshot.Control.OverflowCount = boundedSupervisionEntries(messages, supervisionProjectionControlLimit, snapshot.Control.OverflowCount)
	for _, message := range messages {
		body, err := supervisionBodyMap(message.Body)
		if err == nil && bodyString(body, "to_role") == "owner" {
			snapshot.Control.AttentionCount++
		}
	}
	return snapshot
}

func boundedSupervisionEntries(entries []types.SupervisionProjectionEntry, limit, overflow int) ([]types.SupervisionProjectionEntry, int) {
	if len(entries) <= limit {
		return entries, overflow
	}
	return entries[:limit], overflow + len(entries) - limit
}

func supervisionEntryStatus(state supervisionProjectionState, kind, id string, body map[string]any) string {
	switch kind {
	case "assignment_opened":
		return statusValue(&state, "assignment", id)
	case "attempt_started":
		return statusValue(&state, "attempt", id)
	case "attempt_result":
		return statusValue(&state, "result", id)
	case "disposition_recorded":
		return bodyString(body, "value")
	case "super_finding_recorded":
		return bodyString(body, "state")
	}
	return ""
}

func latestSupervisionBelief(entries []types.SupervisionProjectionEntry) *types.SupervisionProjectionEntry {
	if len(entries) == 0 {
		return nil
	}
	superseded := make(map[string]bool, len(entries))
	for _, entry := range entries {
		body, err := supervisionBodyMap(entry.Body)
		if err == nil {
			if prior := bodyNullableString(body, "supersedes_belief_id"); prior != "" {
				superseded[prior] = true
			}
		}
	}
	for index := len(entries) - 1; index >= 0; index-- {
		if !superseded[entries[index].ID] {
			entry := entries[index]
			return &entry
		}
	}
	entry := entries[len(entries)-1]
	return &entry
}

func supervisionProjectionObligations(state supervisionProjectionState) ([]types.SupervisionProjectionEntry, []types.SupervisionProjectionEntry) {
	blockers := make([]types.SupervisionProjectionEntry, 0, len(state.OpenFindings)+len(state.OpenCompensationObligations))
	rebases := make([]types.SupervisionProjectionEntry, 0, len(state.OpenRebaseObligations))
	for id, open := range state.OpenFindings {
		if open {
			if raw := state.Entities["super_finding_recorded"][id]; raw != nil {
				body, err := supervisionBodyMap(raw)
				if err == nil {
					blockers = append(blockers, types.SupervisionProjectionEntry{ID: id, Kind: "super_finding_recorded", Status: "open", Body: append(json.RawMessage(nil), raw...), ArtifactRefs: supervisionArtifactRefs(body), EvidenceRefs: bodyStrings(body, "evidence_refs")})
				}
			}
		}
	}
	for id := range state.OpenCompensationObligations {
		raw, _ := json.Marshal(map[string]any{"compensation_obligation_id": id, "open": true})
		blockers = append(blockers, types.SupervisionProjectionEntry{ID: id, Kind: "compensation_obligation", Status: "open", Body: raw})
	}
	rebaseBodies := make(map[string]json.RawMessage, len(state.OpenRebaseObligations))
	for intentID, raw := range state.Entities["intent_revised"] {
		body, err := supervisionBodyMap(raw)
		if err != nil {
			continue
		}
		for _, target := range bodyObjects(body, "affected_targets") {
			rebaseID := bodyString(target, "rebase_obligation_id")
			if !state.OpenRebaseObligations[rebaseID] {
				continue
			}
			projected, err := json.Marshal(map[string]any{"intent_revision_id": intentID, "affected_target": target, "open": true})
			if err == nil {
				rebaseBodies[rebaseID] = projected
			}
		}
	}
	for id := range state.OpenRebaseObligations {
		raw := rebaseBodies[id]
		if raw == nil {
			raw, _ = json.Marshal(map[string]any{"rebase_obligation_id": id, "open": true})
		}
		rebases = append(rebases, types.SupervisionProjectionEntry{ID: id, Kind: "rebase_obligation", Status: "open", Body: raw})
	}
	sort.Slice(blockers, func(i, j int) bool { return blockers[i].ID < blockers[j].ID })
	sort.Slice(rebases, func(i, j int) bool { return rebases[i].ID < rebases[j].ID })
	return blockers, rebases
}
