package selfdev

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const (
	StateRequested        = "requested"
	StateExecuting        = "executing"
	StateFrozen           = "frozen"
	StateVerified         = "verified"
	StateAwaitingApproval = "awaiting_approval"
	StateAccepted         = "accepted"
	StateMaterializing    = "materializing"
	StateApplied          = "applied"
	StateRejected         = "rejected"
	StateRollbackPending  = "rollback_pending"
	StateRolledBack       = "rolled_back"
	StateFailed           = "failed"
	StateDegraded         = "degraded"
)

var (
	ErrConflict          = errors.New("self-development operation conflict")
	ErrInvalidTransition = errors.New("invalid self-development operation transition")
)

type DBProvider interface {
	DB() *sql.DB
}

type HeadReader interface {
	Head(context.Context, string) (*computerevent.Head, error)
}

type Operation struct {
	OperationID            string   `json:"operation_id"`
	RequestCommitment      string   `json:"request_commitment"`
	ComputerID             string   `json:"computer_id"`
	TrajectoryID           string   `json:"trajectory_id"`
	CapsuleID              string   `json:"capsule_id,omitempty"`
	BaseHead               string   `json:"base_head"`
	PromptArtifactRef      string   `json:"prompt_artifact_ref"`
	BundleDigest           string   `json:"bundle_digest,omitempty"`
	ReleaseDigest          string   `json:"release_digest,omitempty"`
	CodeRef                string   `json:"code_ref,omitempty"`
	ArtifactProgramRef     string   `json:"artifact_program_ref,omitempty"`
	VerifierRefs           []string `json:"verifier_refs"`
	DecisionActor          string   `json:"decision_actor,omitempty"`
	DecisionEvent          string   `json:"decision_event,omitempty"`
	DesiredHead            string   `json:"desired_head"`
	EffectiveHead          string   `json:"effective_head"`
	MaterializationReceipt string   `json:"materialization_receipt,omitempty"`
	CheckpointRef          string   `json:"checkpoint_ref,omitempty"`
	RouteCertificate       string   `json:"route_certificate,omitempty"`
	RouteGeneration        *uint64  `json:"route_generation,omitempty"`
	RouteReceipt           string   `json:"route_receipt,omitempty"`
	ModeReceipt            string   `json:"mode_receipt,omitempty"`
	LifecycleReceipt       string   `json:"lifecycle_receipt,omitempty"`
	State                  string   `json:"state"`
	TerminalError          string   `json:"error,omitempty"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
}

type StartRequest struct {
	ComputerID        string `json:"computer_id"`
	IdempotencyKey    string `json:"idempotency_key"`
	PromptArtifactRef string `json:"prompt_artifact_ref"`
	OperationID       string `json:"-"`
	TrajectoryID      string `json:"-"`
	BaseHead          string `json:"-"`
	RequestCommitment string `json:"-"`
}

type RollbackStartRequest struct {
	ComputerID        string
	IdempotencyKey    string
	RequestCommitment string
	RollbackEvent     string
	DecisionActor     string
	CurrentDesired    string
	CurrentEffective  string
	Target            Operation
	RouteGeneration   uint64
}

type BaselineRequest struct {
	ComputerID             string
	IdempotencyKey         string
	EventHead              string
	StateCommitment        string
	ReleaseDigest          string
	CodeRef                string
	ArtifactProgramRef     string
	VerifierRefs           []string
	MaterializationReceipt string
	CheckpointRef          string
	RouteReceipt           string
	RouteGeneration        uint64
}

type Store struct {
	db    *sql.DB
	heads HeadReader
	now   func() time.Time
}

func NewStore(db DBProvider, heads HeadReader) (*Store, error) {
	if db == nil || db.DB() == nil || heads == nil {
		return nil, fmt.Errorf("self-development operations: embedded store and event projection are required")
	}
	return &Store{db: db.DB(), heads: heads, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Store) Start(ctx context.Context, request StartRequest) (Operation, error) {
	request.ComputerID = strings.TrimSpace(request.ComputerID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.PromptArtifactRef = strings.TrimSpace(request.PromptArtifactRef)
	if request.ComputerID == "" || request.IdempotencyKey == "" || !isArtifactRef(request.PromptArtifactRef) {
		return Operation{}, fmt.Errorf("self-development operation: complete target, idempotency key, and prompt artifact are required")
	}
	commitment := strings.TrimSpace(request.RequestCommitment)
	if commitment == "" {
		var err error
		commitment, err = startRequestCommitment(request)
		if err != nil {
			return Operation{}, err
		}
	}
	if existing, found, err := s.byIdempotency(ctx, request.ComputerID, request.IdempotencyKey); err != nil || found {
		if found && existing.RequestCommitment != commitment {
			return Operation{}, fmt.Errorf("%w: idempotency commitment changed", ErrConflict)
		}
		return existing, err
	}
	head, err := s.heads.Head(ctx, request.ComputerID)
	if err != nil {
		return Operation{}, err
	}
	if head == nil {
		return Operation{}, fmt.Errorf("self-development operation: GenesisImported is required")
	}
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		operationID = "selfdev-" + uuid.NewString()
	}
	trajectoryID := strings.TrimSpace(request.TrajectoryID)
	if trajectoryID == "" {
		trajectoryID = "trajectory-" + uuid.NewString()
	}
	baseHead := strings.TrimSpace(request.BaseHead)
	if baseHead == "" {
		baseHead = head.CanonicalEventHead
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	operation := Operation{
		OperationID: operationID, RequestCommitment: commitment, ComputerID: request.ComputerID,
		TrajectoryID: trajectoryID, BaseHead: baseHead, PromptArtifactRef: request.PromptArtifactRef,
		VerifierRefs: []string{}, DesiredHead: head.DesiredEventHead, EffectiveHead: head.EffectiveEventHead,
		State: StateRequested, CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, verifier_refs_json, desired_head, effective_head, state, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, '[]', ?, ?, ?, ?, ?)`, operation.OperationID, operation.ComputerID, request.IdempotencyKey, operation.RequestCommitment, operation.TrajectoryID, operation.BaseHead, operation.PromptArtifactRef, operation.DesiredHead, operation.EffectiveHead, operation.State, now, now)
	if err != nil {
		if retry, found, readErr := s.byIdempotency(ctx, request.ComputerID, request.IdempotencyKey); readErr == nil && found {
			if retry.RequestCommitment != commitment {
				return Operation{}, fmt.Errorf("%w: idempotency commitment changed", ErrConflict)
			}
			return retry, nil
		}
		return Operation{}, fmt.Errorf("self-development operation: persist: %w", err)
	}
	return operation, nil
}

func (s *Store) GetByIdempotency(ctx context.Context, computerID, idempotencyKey string) (Operation, bool, error) {
	return s.byIdempotency(ctx, strings.TrimSpace(computerID), strings.TrimSpace(idempotencyKey))
}

func (s *Store) Get(ctx context.Context, computerID, operationID string) (Operation, error) {
	return scanOperation(s.db.QueryRowContext(ctx, operationSelect+` WHERE computer_id=? AND operation_id=?`, strings.TrimSpace(computerID), strings.TrimSpace(operationID)))
}

func (s *Store) GetByTrajectory(ctx context.Context, computerID, trajectoryID string) (Operation, error) {
	return scanOperation(s.db.QueryRowContext(ctx, operationSelect+` WHERE computer_id=? AND trajectory_id=?`, strings.TrimSpace(computerID), strings.TrimSpace(trajectoryID)))
}

func (s *Store) GetByEffectiveHead(ctx context.Context, computerID, effectiveHead string) (Operation, error) {
	return scanOperation(s.db.QueryRowContext(ctx, operationSelect+` WHERE computer_id=? AND effective_head=? AND state IN (?,?) ORDER BY updated_at DESC LIMIT 1`, strings.TrimSpace(computerID), strings.TrimSpace(effectiveHead), StateApplied, StateRolledBack))
}

func (s *Store) ListByStates(ctx context.Context, computerID string, states ...string) ([]Operation, error) {
	if len(states) == 0 {
		return []Operation{}, nil
	}
	placeholders := make([]string, len(states))
	args := make([]any, 0, len(states)+1)
	args = append(args, strings.TrimSpace(computerID))
	for index, state := range states {
		placeholders[index] = "?"
		args = append(args, strings.TrimSpace(state))
	}
	rows, err := s.db.QueryContext(ctx, operationSelect+` WHERE computer_id=? AND state IN (`+strings.Join(placeholders, ",")+`) ORDER BY created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	operations := make([]Operation, 0)
	for rows.Next() {
		operation, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}

func (s *Store) RecordAppliedBaseline(ctx context.Context, request BaselineRequest) (Operation, error) {
	if request.ComputerID == "" || request.IdempotencyKey == "" || !computerevent.IsSHA256(request.EventHead) ||
		!computerevent.IsSHA256(request.StateCommitment) || !computerevent.IsSHA256(request.ReleaseDigest) ||
		request.CodeRef == "" || request.ArtifactProgramRef == "" || len(request.VerifierRefs) == 0 ||
		!computerevent.IsSHA256(request.VerifierRefs[0]) || !computerevent.IsSHA256(request.MaterializationReceipt) ||
		request.CheckpointRef == "" || request.RouteReceipt == "" || request.RouteGeneration == 0 {
		return Operation{}, fmt.Errorf("self-development baseline: complete immutable bindings are required")
	}
	if existing, found, err := s.byIdempotency(ctx, request.ComputerID, request.IdempotencyKey); err != nil {
		return Operation{}, err
	} else if found {
		if existing.EffectiveHead != request.EventHead || existing.ReleaseDigest != request.ReleaseDigest ||
			existing.CodeRef != request.CodeRef || existing.ArtifactProgramRef != request.ArtifactProgramRef ||
			existing.MaterializationReceipt != request.MaterializationReceipt || existing.CheckpointRef != request.CheckpointRef ||
			len(existing.VerifierRefs) != len(request.VerifierRefs) || existing.VerifierRefs[0] != request.VerifierRefs[0] ||
			existing.RouteReceipt != request.RouteReceipt || existing.RouteGeneration == nil || *existing.RouteGeneration != request.RouteGeneration {
			return Operation{}, fmt.Errorf("%w: genesis baseline binding changed", ErrConflict)
		}
		return existing, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	operation := Operation{
		OperationID: "genesis-" + uuid.NewString(), RequestCommitment: request.StateCommitment, ComputerID: request.ComputerID,
		TrajectoryID: "trajectory-genesis-" + uuid.NewString(), BaseHead: request.EventHead,
		PromptArtifactRef: request.CheckpointRef, BundleDigest: request.ReleaseDigest, ReleaseDigest: request.ReleaseDigest,
		CodeRef: request.CodeRef, ArtifactProgramRef: request.ArtifactProgramRef, VerifierRefs: append([]string(nil), request.VerifierRefs...),
		DecisionActor: "external-owner-genesis", DecisionEvent: request.EventHead,
		DesiredHead: request.EventHead, EffectiveHead: request.EventHead, MaterializationReceipt: request.MaterializationReceipt,
		CheckpointRef: request.CheckpointRef, RouteReceipt: request.RouteReceipt, RouteGeneration: &request.RouteGeneration,
		State: StateApplied, CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
	}
	verifiers, _ := json.Marshal(operation.VerifierRefs)
	_, err := s.db.ExecContext(ctx, `INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, bundle_digest, release_digest, code_ref, artifact_program_ref, verifier_refs_json, decision_actor, decision_event, desired_head, effective_head, materialization_receipt, checkpoint_ref, route_receipt, route_generation, state, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		operation.OperationID, operation.ComputerID, request.IdempotencyKey, operation.RequestCommitment, operation.TrajectoryID, operation.BaseHead, operation.PromptArtifactRef, operation.BundleDigest, operation.ReleaseDigest, operation.CodeRef, operation.ArtifactProgramRef, string(verifiers), operation.DecisionActor, operation.DecisionEvent, operation.DesiredHead, operation.EffectiveHead, operation.MaterializationReceipt, operation.CheckpointRef, operation.RouteReceipt, request.RouteGeneration, operation.State, now, now)
	if err != nil {
		return Operation{}, err
	}
	return operation, nil
}

func (s *Store) StartRollback(ctx context.Context, request RollbackStartRequest) (Operation, error) {
	request.ComputerID, request.IdempotencyKey = strings.TrimSpace(request.ComputerID), strings.TrimSpace(request.IdempotencyKey)
	if request.ComputerID == "" || request.IdempotencyKey == "" || !computerevent.IsSHA256(request.RequestCommitment) || !computerevent.IsSHA256(request.RollbackEvent) ||
		!computerevent.IsSHA256(request.CurrentDesired) || !computerevent.IsSHA256(request.CurrentEffective) || request.RouteGeneration == 0 ||
		request.Target.ComputerID != request.ComputerID || (request.Target.State != StateApplied && request.Target.State != StateRolledBack) || !computerevent.IsSHA256(request.Target.EffectiveHead) ||
		!computerevent.IsSHA256(request.Target.BundleDigest) || !computerevent.IsSHA256(request.Target.ReleaseDigest) || request.Target.CodeRef == "" || request.Target.ArtifactProgramRef == "" ||
		len(request.Target.VerifierRefs) == 0 || !computerevent.IsSHA256(request.Target.VerifierRefs[0]) ||
		request.Target.MaterializationReceipt == "" || request.Target.CheckpointRef == "" || request.Target.RouteReceipt == "" {
		return Operation{}, fmt.Errorf("self-development rollback: complete current and prior applied bindings are required")
	}
	if existing, found, err := s.byIdempotency(ctx, request.ComputerID, request.IdempotencyKey); err != nil || found {
		if found && existing.RequestCommitment != request.RequestCommitment {
			return Operation{}, fmt.Errorf("%w: rollback idempotency commitment changed", ErrConflict)
		}
		return existing, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	operation := Operation{
		OperationID: "rollback-" + uuid.NewString(), RequestCommitment: request.RequestCommitment, ComputerID: request.ComputerID,
		TrajectoryID: "trajectory-rollback-" + uuid.NewString(), BaseHead: request.Target.EffectiveHead, PromptArtifactRef: request.Target.CheckpointRef,
		BundleDigest: request.Target.BundleDigest, ReleaseDigest: request.Target.ReleaseDigest,
		CodeRef: request.Target.CodeRef, ArtifactProgramRef: request.Target.ArtifactProgramRef,
		VerifierRefs:  append([]string(nil), request.Target.VerifierRefs...),
		DecisionActor: request.DecisionActor, DecisionEvent: request.RollbackEvent,
		DesiredHead: request.CurrentDesired, EffectiveHead: request.CurrentEffective,
		MaterializationReceipt: request.Target.MaterializationReceipt, CheckpointRef: request.Target.CheckpointRef,
		RouteReceipt: request.Target.RouteReceipt, RouteGeneration: &request.RouteGeneration, State: StateRollbackPending,
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
	}
	verifiers, _ := json.Marshal(operation.VerifierRefs)
	_, err := s.db.ExecContext(ctx, `INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, bundle_digest, release_digest, code_ref, artifact_program_ref, verifier_refs_json, decision_actor, decision_event, desired_head, effective_head, materialization_receipt, checkpoint_ref, route_receipt, route_generation, state, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		operation.OperationID, operation.ComputerID, request.IdempotencyKey, operation.RequestCommitment, operation.TrajectoryID, operation.BaseHead, operation.PromptArtifactRef, operation.BundleDigest, operation.ReleaseDigest, operation.CodeRef, operation.ArtifactProgramRef, string(verifiers), operation.DecisionActor, operation.DecisionEvent, operation.DesiredHead, operation.EffectiveHead, operation.MaterializationReceipt, operation.CheckpointRef, operation.RouteReceipt, request.RouteGeneration, operation.State, now, now)
	if err != nil {
		if replay, found, readErr := s.byIdempotency(ctx, request.ComputerID, request.IdempotencyKey); readErr == nil && found && replay.RequestCommitment == request.RequestCommitment {
			return replay, nil
		}
		return Operation{}, err
	}
	return operation, nil
}

func (s *Store) Transition(ctx context.Context, computerID, operationID, expectedState, nextState string, mutate func(*Operation) error) (Operation, error) {
	if !allowedTransition(expectedState, nextState) {
		return Operation{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, expectedState, nextState)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Operation{}, err
	}
	defer tx.Rollback()
	operation, err := scanOperation(tx.QueryRowContext(ctx, operationSelect+` WHERE computer_id=? AND operation_id=? FOR UPDATE`, computerID, operationID))
	if err != nil {
		return Operation{}, err
	}
	if operation.State != expectedState {
		return Operation{}, fmt.Errorf("%w: state is %s, expected %s", ErrConflict, operation.State, expectedState)
	}
	if mutate != nil {
		if err := mutate(&operation); err != nil {
			return Operation{}, err
		}
	}
	operation.State = nextState
	now := s.now().UTC().Truncate(time.Microsecond)
	operation.UpdatedAt = now.Format(time.RFC3339Nano)
	verifiers, err := json.Marshal(operation.VerifierRefs)
	if err != nil {
		return Operation{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE self_development_operations SET capsule_id=?, bundle_digest=?, release_digest=?, code_ref=?, artifact_program_ref=?, verifier_refs_json=?, decision_actor=?, decision_event=?, desired_head=?, effective_head=?, materialization_receipt=?, checkpoint_ref=?, route_certificate=?, route_generation=?, route_receipt=?, mode_receipt=?, lifecycle_receipt=?, state=?, terminal_error=?, updated_at=? WHERE computer_id=? AND operation_id=? AND state=?`, operation.CapsuleID, operation.BundleDigest, operation.ReleaseDigest, operation.CodeRef, operation.ArtifactProgramRef, string(verifiers), operation.DecisionActor, operation.DecisionEvent, operation.DesiredHead, operation.EffectiveHead, operation.MaterializationReceipt, operation.CheckpointRef, operation.RouteCertificate, operation.RouteGeneration, operation.RouteReceipt, operation.ModeReceipt, operation.LifecycleReceipt, operation.State, operation.TerminalError, now, strings.TrimSpace(computerID), strings.TrimSpace(operationID), expectedState)
	if err != nil {
		return Operation{}, err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return Operation{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return Operation{}, err
	}
	return operation, nil
}

func (s *Store) byIdempotency(ctx context.Context, computerID, idempotencyKey string) (Operation, bool, error) {
	operation, err := scanOperation(s.db.QueryRowContext(ctx, operationSelect+` WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey))
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, false, nil
	}
	return operation, err == nil, err
}

const operationSelect = `SELECT operation_id, request_commitment, computer_id, trajectory_id, capsule_id, base_head, prompt_artifact_ref, bundle_digest, release_digest, code_ref, artifact_program_ref, verifier_refs_json, decision_actor, decision_event, desired_head, effective_head, materialization_receipt, checkpoint_ref, route_certificate, route_generation, route_receipt, mode_receipt, lifecycle_receipt, state, terminal_error, created_at, updated_at FROM self_development_operations`

type rowScanner interface{ Scan(...any) error }

func scanOperation(row rowScanner) (Operation, error) {
	var operation Operation
	var verifiers string
	var routeGeneration sql.NullInt64
	var createdAt, updatedAt time.Time
	err := row.Scan(&operation.OperationID, &operation.RequestCommitment, &operation.ComputerID, &operation.TrajectoryID, &operation.CapsuleID, &operation.BaseHead, &operation.PromptArtifactRef, &operation.BundleDigest, &operation.ReleaseDigest, &operation.CodeRef, &operation.ArtifactProgramRef, &verifiers, &operation.DecisionActor, &operation.DecisionEvent, &operation.DesiredHead, &operation.EffectiveHead, &operation.MaterializationReceipt, &operation.CheckpointRef, &operation.RouteCertificate, &routeGeneration, &operation.RouteReceipt, &operation.ModeReceipt, &operation.LifecycleReceipt, &operation.State, &operation.TerminalError, &createdAt, &updatedAt)
	if err != nil {
		return Operation{}, err
	}
	if err := json.Unmarshal([]byte(verifiers), &operation.VerifierRefs); err != nil {
		return Operation{}, err
	}
	if operation.VerifierRefs == nil {
		operation.VerifierRefs = []string{}
	}
	if routeGeneration.Valid {
		value := uint64(routeGeneration.Int64)
		operation.RouteGeneration = &value
	}
	operation.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	operation.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return operation, nil
}

func startRequestCommitment(request StartRequest) (string, error) {
	canonical, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

func isArtifactRef(ref string) bool {
	if !strings.HasPrefix(ref, "artifact:sha256:") {
		return false
	}
	return computerevent.IsSHA256(strings.TrimPrefix(ref, "artifact:sha256:"))
}

func allowedTransition(from, to string) bool {
	switch from {
	case StateRequested:
		return to == StateExecuting || to == StateFailed
	case StateExecuting:
		return to == StateFrozen || to == StateFailed
	case StateFrozen:
		return to == StateVerified || to == StateFailed
	case StateVerified:
		return to == StateAwaitingApproval || to == StateFailed
	case StateAwaitingApproval:
		return to == StateAccepted || to == StateRejected || to == StateFailed
	case StateAccepted:
		return to == StateMaterializing || to == StateFailed
	case StateMaterializing:
		return to == StateApplied || to == StateFailed || to == StateDegraded
	case StateApplied:
		return to == StateRollbackPending || to == StateDegraded
	case StateRollbackPending:
		return to == StateRolledBack || to == StateFailed || to == StateDegraded
	default:
		return false
	}
}
