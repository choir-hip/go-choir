package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type projectedDesktopState struct {
	OwnerID            string                     `json:"owner_id"`
	DesktopID          string                     `json:"desktop_id"`
	Windows            []types.WindowState        `json:"windows"`
	ActiveWindowID     string                     `json:"active_window_id"`
	UpdatedAt          time.Time                  `json:"updated_at"`
	CreatedBySessionID string                     `json:"created_by_session_id,omitempty"`
	Sessions           []projectedSessionIdentity `json:"sessions,omitempty"`
}

// projectedSessionIdentity is the witnessed session row. Presence fields
// (last_input_at, driver_until, visibility_state, is_driver) are not tape
// payloads and must not appear here.
type projectedSessionIdentity struct {
	SessionID       string `json:"session_id"`
	DeviceID        string `json:"device_id,omitempty"`
	ViewportProfile string `json:"viewport_profile,omitempty"`
}

func projectBatch(ctx context.Context, tx *sql.Tx, batch computerevent.ProjectionBatch) error {
	return projectBatchWithReplayTextureBootstrap(ctx, tx, batch, false, strings.TrimSpace(batch.ComputerID))
}

func projectBatchForReplay(ctx context.Context, tx *sql.Tx, batch computerevent.ProjectionBatch) error {
	return projectBatchWithReplayTextureBootstrap(ctx, tx, batch, true, strings.TrimSpace(batch.ComputerID))
}

func projectBatchWithReplayTextureBootstrap(ctx context.Context, tx *sql.Tx, batch computerevent.ProjectionBatch, allowReplayTextureBootstrap bool, replayComputerID string) error {
	if tx == nil {
		return fmt.Errorf("computer event projection: missing transaction")
	}
	if err := batch.Validate(); err != nil {
		return err
	}
	computerID := strings.TrimSpace(batch.ComputerID)
	if computerID == "" {
		computerID = replayComputerID
	}
	for i, op := range batch.Ops {
		if err := projectOp(ctx, tx, op, allowReplayTextureBootstrap, replayComputerID, computerID); err != nil {
			return fmt.Errorf("computer event projection: op %d: %w", i, err)
		}
	}
	return nil
}

func projectOp(ctx context.Context, tx *sql.Tx, op computerevent.ProjectionOp, allowReplayTextureBootstrap bool, replayComputerID, computerID string) error {
	switch strings.TrimSpace(op.Kind) {
	case computerevent.ProjectionOpDesktopState:
		return projectDesktopState(ctx, tx, op.Body, computerID)
	case computerevent.ProjectionOpObject:
		return projectObject(ctx, tx, op.Body)
	case computerevent.ProjectionOpObjectEdge:
		return projectObjectEdge(ctx, tx, op.Body)
	case computerevent.ProjectionOpRunMemoryEntry:
		return projectRunMemoryEntry(ctx, tx, op.Body)
	case computerevent.ProjectionOpSelfDevelopmentStartIntent:
		return projectSelfDevelopmentStartIntent(ctx, tx, op.Body)
	case computerevent.ProjectionOpSelfDevelopmentOperation:
		return projectSelfDevelopmentOperation(ctx, tx, op.Body)
	case computerevent.ProjectionOpTextureAgentMutation:
		return projectTextureAgentMutation(ctx, tx, op.Body, allowReplayTextureBootstrap, replayComputerID)
	case computerevent.ProjectionOpTextureDocumentAlias:
		return projectTextureDocumentAlias(ctx, tx, op.Body, computerID)
	case computerevent.ProjectionOpTextureDocumentAliasDelete:
		return projectTextureDocumentAliasDelete(ctx, tx, op.Body, computerID)
	default:
		return fmt.Errorf("%w: kind %q", computerevent.ErrProjectionBatchInvalid, op.Kind)
	}
}

func projectDesktopState(ctx context.Context, tx *sql.Tx, body json.RawMessage, computerID string) error {
	var state projectedDesktopState
	if err := json.Unmarshal(body, &state); err != nil {
		return fmt.Errorf("decode desktop state: %w", err)
	}
	ownerID := strings.TrimSpace(state.OwnerID)
	desktopID := normalizeDesktopID(state.DesktopID)
	if ownerID == "" {
		return fmt.Errorf("desktop state owner is required")
	}
	computerID = strings.TrimSpace(computerID)
	now := state.UpdatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sessionID := strings.TrimSpace(state.CreatedBySessionID)
	if sessionID == "" {
		sessionID = "projected"
	}
	windowsJSON, err := json.Marshal(state.Windows)
	if err != nil {
		return fmt.Errorf("marshal desktop windows: %w", err)
	}
	stamp := now.Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO desktop_workspaces (owner_id, computer_id, desktop_id, windows_json, active_window, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   windows_json = VALUES(windows_json),
		   active_window = VALUES(active_window),
		   updated_at = VALUES(updated_at)`,
		ownerID, computerID, desktopID, string(windowsJSON), state.ActiveWindowID, stamp,
	); err != nil {
		return fmt.Errorf("project desktop workspace: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_app_instances WHERE owner_id = ? AND (computer_id = ? OR computer_id = '') AND desktop_id = ?`, ownerID, computerID, desktopID); err != nil {
		return fmt.Errorf("replace projected app instances: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_window_placements WHERE owner_id = ? AND (computer_id = ? OR computer_id = '') AND desktop_id = ?`, ownerID, computerID, desktopID); err != nil {
		return fmt.Errorf("replace projected placements: %w", err)
	}
	for i, win := range state.Windows {
		appInstanceID := strings.TrimSpace(win.WindowID)
		if appInstanceID == "" {
			continue
		}
		stackRank := win.ZIndex
		if stackRank <= 0 {
			stackRank = i + 1
		}
		appContextJSON, err := json.Marshal(win.AppContext)
		if err != nil {
			return fmt.Errorf("marshal desktop app context: %w", err)
		}
		if len(appContextJSON) == 0 || string(appContextJSON) == "null" {
			appContextJSON = []byte("{}")
		}
		lifecycle := "open"
		if win.Mode == types.WindowMinimized {
			lifecycle = "minimized"
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO desktop_app_instances (
				owner_id, computer_id, desktop_id, app_instance_id, app_id, title, app_context_json,
				lifecycle, shared_stack_rank, last_used_at, created_by_session_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ownerID, computerID, desktopID, appInstanceID, strings.TrimSpace(win.AppID), strings.TrimSpace(win.Title),
			string(appContextJSON), lifecycle, stackRank, stamp, sessionID, stamp, stamp,
		); err != nil {
			return fmt.Errorf("insert projected app instance: %w", err)
		}
		restoredGeometryJSON := ""
		if win.RestoredGeometry != nil {
			raw, err := json.Marshal(win.RestoredGeometry)
			if err != nil {
				return fmt.Errorf("marshal restored geometry: %w", err)
			}
			restoredGeometryJSON = string(raw)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO desktop_window_placements (
				owner_id, computer_id, desktop_id, session_id, app_instance_id,
				x, y, width, height, mode, local_z_index, local_focused,
				restored_geometry_json, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ownerID, computerID, desktopID, sessionID, appInstanceID,
			win.Geometry.X, win.Geometry.Y, win.Geometry.Width, win.Geometry.Height,
			string(win.Mode), stackRank, win.WindowID == state.ActiveWindowID,
			restoredGeometryJSON, stamp,
		); err != nil {
			return fmt.Errorf("insert projected placement: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_sessions WHERE owner_id = ? AND desktop_id = ?`, ownerID, desktopID); err != nil {
		return fmt.Errorf("replace projected session identities: %w", err)
	}
	for _, session := range state.Sessions {
		sessionID := strings.TrimSpace(session.SessionID)
		if sessionID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO desktop_sessions (
				owner_id, computer_id, desktop_id, session_id, device_id, viewport_profile,
				visibility_state, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)`,
			ownerID, computerID, desktopID, sessionID, strings.TrimSpace(session.DeviceID),
			strings.TrimSpace(session.ViewportProfile), stamp, stamp,
		); err != nil {
			return fmt.Errorf("project session identity: %w", err)
		}
	}
	return nil
}

func projectObject(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var obj objectgraph.Object
	if err := json.Unmarshal(body, &obj); err != nil {
		return fmt.Errorf("decode object: %w", err)
	}
	if strings.TrimSpace(obj.CanonicalID) == "" || strings.TrimSpace(string(obj.ObjectKind)) == "" {
		return fmt.Errorf("object identity is required")
	}
	metadata := string(obj.Metadata)
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	created := obj.CreatedAt.UTC()
	updated := obj.UpdatedAt.UTC()
	if created.IsZero() {
		created = time.Now().UTC()
	}
	if updated.IsZero() {
		updated = created
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			object_kind = VALUES(object_kind),
			owner_id = VALUES(owner_id),
			computer_id = VALUES(computer_id),
			version_id = VALUES(version_id),
			content_hash = VALUES(content_hash),
			body = VALUES(body),
			metadata = VALUES(metadata),
			updated_at = VALUES(updated_at),
			tombstone = VALUES(tombstone),
			superseded_by = VALUES(superseded_by)`,
		obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
		obj.VersionID, obj.ContentHash, obj.Body, metadata,
		created, updated, obj.Tombstone, obj.SupersededBy); err != nil {
		return fmt.Errorf("project object: %w", err)
	}
	return nil
}

func projectObjectEdge(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var edge objectgraph.Edge
	if err := json.Unmarshal(body, &edge); err != nil {
		return fmt.Errorf("decode object edge: %w", err)
	}
	if strings.TrimSpace(edge.EdgeID) == "" || strings.TrimSpace(edge.FromID) == "" || strings.TrimSpace(edge.ToID) == "" {
		return fmt.Errorf("edge identity is required")
	}
	metadata := string(edge.Metadata)
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	created := edge.CreatedAt.UTC()
	if created.IsZero() {
		created = time.Now().UTC()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO og_edges
		(edge_id, from_id, to_id, kind, metadata, created_at, tombstone)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			from_id = VALUES(from_id),
			to_id = VALUES(to_id),
			kind = VALUES(kind),
			metadata = VALUES(metadata),
			tombstone = VALUES(tombstone)`,
		edge.EdgeID, edge.FromID, edge.ToID, string(edge.Kind), metadata, created, edge.Tombstone); err != nil {
		return fmt.Errorf("project object edge: %w", err)
	}
	return nil
}

func projectRunMemoryEntry(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var entry computerevent.RunMemoryEntryProjection
	if err := json.Unmarshal(body, &entry); err != nil {
		return fmt.Errorf("decode run memory entry: %w", err)
	}
	if strings.TrimSpace(entry.EntryID) == "" || strings.TrimSpace(entry.RunID) == "" || entry.Seq <= 0 || strings.TrimSpace(entry.Kind) == "" {
		return fmt.Errorf("run memory entry identity is required")
	}
	if strings.TrimSpace(entry.DetailsJSON) == "" {
		entry.DetailsJSON = "{}"
	}
	var existing computerevent.RunMemoryEntryProjection
	err := tx.QueryRowContext(ctx, `SELECT
		entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind,
		role, message_json, summary, first_kept_entry_id, tokens_before,
		reason, model, details_json, created_at
		FROM run_memory_entries WHERE entry_id=? FOR UPDATE`, entry.EntryID).Scan(
		&existing.EntryID, &existing.RunID, &existing.OwnerID, &existing.AgentID, &existing.ParentEntryID,
		&existing.Seq, &existing.Kind, &existing.Role, &existing.MessageJSON, &existing.Summary,
		&existing.FirstKeptEntryID, &existing.TokensBefore, &existing.Reason, &existing.Model,
		&existing.DetailsJSON, &existing.CreatedAt)
	if err == nil {
		if strings.TrimSpace(existing.DetailsJSON) == "" {
			existing.DetailsJSON = "{}"
		}
		if existing != entry {
			return fmt.Errorf("%w: run memory entry %s changed", computerevent.ErrProjectionMismatch, entry.EntryID)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read run memory entry: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO run_memory_entries (
		entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind,
		role, message_json, summary, first_kept_entry_id, tokens_before,
		reason, model, details_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.EntryID, entry.RunID, entry.OwnerID, entry.AgentID, entry.ParentEntryID, entry.Seq,
		entry.Kind, entry.Role, entry.MessageJSON, entry.Summary, entry.FirstKeptEntryID,
		entry.TokensBefore, entry.Reason, entry.Model, entry.DetailsJSON, entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("project run memory entry: %w", err)
	}
	return nil
}

func projectSelfDevelopmentStartIntent(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var intent computerevent.SelfDevelopmentStartIntentProjection
	if err := json.Unmarshal(body, &intent); err != nil {
		return fmt.Errorf("decode self-development start intent: %w", err)
	}
	if strings.TrimSpace(intent.ComputerID) == "" || strings.TrimSpace(intent.IdempotencyKey) == "" || !computerevent.IsSHA256(intent.RequestCommitment) {
		return fmt.Errorf("self-development start intent identity is required")
	}
	var storedCommitment, storedCreatedAt string
	err := tx.QueryRowContext(ctx, `SELECT request_commitment, created_at FROM self_development_start_intents WHERE computer_id=? AND idempotency_key=? FOR UPDATE`, intent.ComputerID, intent.IdempotencyKey).Scan(&storedCommitment, &storedCreatedAt)
	if err == nil {
		if storedCommitment != intent.RequestCommitment || storedCreatedAt != intent.CreatedAt {
			return fmt.Errorf("%w: start intent %s changed", computerevent.ErrProjectionMismatch, intent.IdempotencyKey)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read self-development start intent: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO self_development_start_intents (computer_id, idempotency_key, request_commitment, created_at)
		VALUES (?, ?, ?, ?)`, intent.ComputerID, intent.IdempotencyKey, intent.RequestCommitment, intent.CreatedAt)
	if err != nil {
		return fmt.Errorf("project self-development start intent: %w", err)
	}
	return nil
}

func projectSelfDevelopmentOperation(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var operation computerevent.SelfDevelopmentOperationProjection
	if err := json.Unmarshal(body, &operation); err != nil {
		return fmt.Errorf("decode self-development operation: %w", err)
	}
	if strings.TrimSpace(operation.OperationID) == "" || strings.TrimSpace(operation.ComputerID) == "" || strings.TrimSpace(operation.IdempotencyKey) == "" || strings.TrimSpace(operation.State) == "" {
		return fmt.Errorf("self-development operation identity is required")
	}
	if strings.TrimSpace(operation.VerifierRefsJSON) == "" {
		operation.VerifierRefsJSON = "[]"
	}
	var existing computerevent.SelfDevelopmentOperationProjection
	var routeGeneration sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT
		operation_id, idempotency_key, request_commitment, computer_id, trajectory_id, capsule_id,
		base_head, prompt_artifact_ref, bundle_digest, release_digest, code_ref, artifact_program_ref,
		verifier_refs_json, decision_actor, decision_event, decision_receipt, desired_head, effective_head,
		materialization_receipt, checkpoint_ref, route_certificate, route_generation, route_receipt,
		mode_receipt, lifecycle_receipt, state, terminal_error, created_at, updated_at
		FROM self_development_operations WHERE operation_id=? FOR UPDATE`, operation.OperationID).Scan(
		&existing.OperationID, &existing.IdempotencyKey, &existing.RequestCommitment, &existing.ComputerID,
		&existing.TrajectoryID, &existing.CapsuleID, &existing.BaseHead, &existing.PromptArtifactRef,
		&existing.BundleDigest, &existing.ReleaseDigest, &existing.CodeRef, &existing.ArtifactProgramRef,
		&existing.VerifierRefsJSON, &existing.DecisionActor, &existing.DecisionEvent, &existing.DecisionReceipt,
		&existing.DesiredHead, &existing.EffectiveHead, &existing.MaterializationReceipt, &existing.CheckpointRef,
		&existing.RouteCertificate, &routeGeneration, &existing.RouteReceipt, &existing.ModeReceipt,
		&existing.LifecycleReceipt, &existing.State, &existing.TerminalError, &existing.CreatedAt, &existing.UpdatedAt)
	if err == nil {
		if routeGeneration.Valid {
			value := uint64(routeGeneration.Int64)
			existing.RouteGeneration = &value
		}
		if strings.TrimSpace(existing.VerifierRefsJSON) == "" {
			existing.VerifierRefsJSON = "[]"
		}
		if existing.ComputerID != operation.ComputerID || existing.IdempotencyKey != operation.IdempotencyKey || existing.RequestCommitment != operation.RequestCommitment {
			return fmt.Errorf("%w: operation %s identity changed", computerevent.ErrProjectionMismatch, operation.OperationID)
		}
		if strings.TrimSpace(operation.ExpectedState) != "" {
			if existing.State != operation.ExpectedState {
				return fmt.Errorf("%w: operation %s state is %s, expected %s", computerevent.ErrProjectionMismatch, operation.OperationID, existing.State, operation.ExpectedState)
			}
		} else {
			if existing.State != operation.State || !sameSelfDevelopmentOperationProjection(existing, operation) {
				return fmt.Errorf("%w: operation %s create snapshot changed", computerevent.ErrProjectionMismatch, operation.OperationID)
			}
			return nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read self-development operation: %w", err)
	} else if strings.TrimSpace(operation.ExpectedState) != "" {
		return fmt.Errorf("%w: operation %s disappeared", computerevent.ErrProjectionMismatch, operation.OperationID)
	}
	var idempotentOperationID, idempotentCommitment string
	idempotencyErr := tx.QueryRowContext(ctx, `SELECT operation_id, request_commitment FROM self_development_operations WHERE computer_id=? AND idempotency_key=? FOR UPDATE`, operation.ComputerID, operation.IdempotencyKey).Scan(&idempotentOperationID, &idempotentCommitment)
	if idempotencyErr == nil && (idempotentOperationID != operation.OperationID || idempotentCommitment != operation.RequestCommitment) {
		return fmt.Errorf("%w: operation idempotency binding changed", computerevent.ErrProjectionMismatch)
	}
	if idempotencyErr != nil && !errors.Is(idempotencyErr, sql.ErrNoRows) {
		return fmt.Errorf("read self-development operation idempotency: %w", idempotencyErr)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO self_development_operations (
		operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, capsule_id,
		base_head, prompt_artifact_ref, bundle_digest, release_digest, code_ref, artifact_program_ref,
		verifier_refs_json, decision_actor, decision_event, decision_receipt, desired_head, effective_head,
		materialization_receipt, checkpoint_ref, route_certificate, route_generation, route_receipt,
		mode_receipt, lifecycle_receipt, state, terminal_error, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		computer_id=VALUES(computer_id), idempotency_key=VALUES(idempotency_key), request_commitment=VALUES(request_commitment),
		trajectory_id=VALUES(trajectory_id), capsule_id=VALUES(capsule_id), base_head=VALUES(base_head),
		prompt_artifact_ref=VALUES(prompt_artifact_ref), bundle_digest=VALUES(bundle_digest), release_digest=VALUES(release_digest),
		code_ref=VALUES(code_ref), artifact_program_ref=VALUES(artifact_program_ref), verifier_refs_json=VALUES(verifier_refs_json),
		decision_actor=VALUES(decision_actor), decision_event=VALUES(decision_event), decision_receipt=VALUES(decision_receipt),
		desired_head=VALUES(desired_head), effective_head=VALUES(effective_head), materialization_receipt=VALUES(materialization_receipt),
		checkpoint_ref=VALUES(checkpoint_ref), route_certificate=VALUES(route_certificate), route_generation=VALUES(route_generation),
		route_receipt=VALUES(route_receipt), mode_receipt=VALUES(mode_receipt), lifecycle_receipt=VALUES(lifecycle_receipt),
		state=VALUES(state), terminal_error=VALUES(terminal_error), created_at=VALUES(created_at), updated_at=VALUES(updated_at)`,
		operation.OperationID, operation.ComputerID, operation.IdempotencyKey, operation.RequestCommitment,
		operation.TrajectoryID, operation.CapsuleID, operation.BaseHead, operation.PromptArtifactRef,
		operation.BundleDigest, operation.ReleaseDigest, operation.CodeRef, operation.ArtifactProgramRef,
		operation.VerifierRefsJSON, operation.DecisionActor, operation.DecisionEvent, operation.DecisionReceipt,
		operation.DesiredHead, operation.EffectiveHead, operation.MaterializationReceipt, operation.CheckpointRef,
		operation.RouteCertificate, operation.RouteGeneration, operation.RouteReceipt, operation.ModeReceipt,
		operation.LifecycleReceipt, operation.State, operation.TerminalError, operation.CreatedAt, operation.UpdatedAt)
	if err != nil {
		return fmt.Errorf("project self-development operation: %w", err)
	}
	return nil
}

func sameSelfDevelopmentOperationProjection(left, right computerevent.SelfDevelopmentOperationProjection) bool {
	if (left.RouteGeneration == nil) != (right.RouteGeneration == nil) {
		return false
	}
	if left.RouteGeneration != nil && *left.RouteGeneration != *right.RouteGeneration {
		return false
	}
	left.RouteGeneration = nil
	right.RouteGeneration = nil
	left.ExpectedState = ""
	right.ExpectedState = ""
	return reflect.DeepEqual(left, right)
}

func projectTextureAgentMutation(ctx context.Context, tx *sql.Tx, body json.RawMessage, allowReplayTextureBootstrap bool, replayComputerID string) error {
	var mutation computerevent.TextureAgentMutationProjection
	if err := json.Unmarshal(body, &mutation); err != nil {
		return fmt.Errorf("decode Texture agent mutation: %w", err)
	}
	if strings.TrimSpace(mutation.DocID) == "" || strings.TrimSpace(mutation.RunID) == "" || strings.TrimSpace(mutation.OwnerID) == "" {
		return fmt.Errorf("Texture agent mutation identity is required")
	}
	var existing computerevent.TextureAgentMutationProjection
	var completedAt sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at
		FROM texture_agent_mutations WHERE owner_id=? AND computer_id=? AND doc_id=? AND loop_id=? FOR UPDATE`,
		mutation.OwnerID, mutation.ComputerID, mutation.DocID, mutation.RunID).Scan(
		&existing.DocID, &existing.RunID, &existing.OwnerID, &existing.ComputerID, &existing.State,
		&existing.ScheduledMessageSeq, &existing.RevisionID, &existing.CreatedAt, &completedAt)
	if err == nil {
		if completedAt.Valid {
			existing.CompletedAt = &completedAt.String
		}
		if mutation.CreateOnly || (len(mutation.ExpectedStates) == 0 && mutation.RequireRevision == nil) {
			if !sameTextureAgentMutationProjection(existing, mutation) {
				return fmt.Errorf("%w: Texture mutation %s/%s snapshot changed", computerevent.ErrProjectionMismatch, mutation.DocID, mutation.RunID)
			}
			return nil
		}
		if len(mutation.ExpectedStates) > 0 {
			matched := false
			for _, expected := range mutation.ExpectedStates {
				if existing.State == expected {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("%w: Texture mutation state is %s", computerevent.ErrProjectionMismatch, existing.State)
			}
		}
		if mutation.RequireRevision != nil {
			hasRevision := strings.TrimSpace(existing.RevisionID) != ""
			if *mutation.RequireRevision != hasRevision {
				return fmt.Errorf("%w: Texture mutation revision presence changed", computerevent.ErrProjectionMismatch)
			}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read Texture agent mutation: %w", err)
	} else if len(mutation.ExpectedStates) > 0 || mutation.RequireRevision != nil {
		mutationComputerID := strings.TrimSpace(mutation.ComputerID)
		replayCompatibleSeed := allowReplayTextureBootstrap &&
			(mutationComputerID == "" || mutationComputerID == strings.TrimSpace(replayComputerID))
		if !replayCompatibleSeed {
			return fmt.Errorf("%w: Texture mutation disappeared", computerevent.ErrProjectionMismatch)
		}
		// A pre-cutover residue import can omit a row that was already
		// referenced by a later guarded transition. The full snapshot is the
		// only durable state witness available at this point in replay. Accept
		// only the empty legacy scope or this replay's computer scope; live
		// finalization never enables the branch and still fails closed.
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO texture_agent_mutations (
		doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		state=VALUES(state), scheduled_message_seq=VALUES(scheduled_message_seq), revision_id=VALUES(revision_id),
		created_at=VALUES(created_at), completed_at=VALUES(completed_at)`,
		mutation.DocID, mutation.RunID, mutation.OwnerID, mutation.ComputerID, mutation.State,
		mutation.ScheduledMessageSeq, mutation.RevisionID, mutation.CreatedAt, mutation.CompletedAt)
	if err != nil {
		return fmt.Errorf("project Texture agent mutation: %w", err)
	}
	return nil
}

func sameTextureAgentMutationProjection(left, right computerevent.TextureAgentMutationProjection) bool {
	if (left.CompletedAt == nil) != (right.CompletedAt == nil) {
		return false
	}
	if left.CompletedAt != nil && *left.CompletedAt != *right.CompletedAt {
		return false
	}
	left.CompletedAt = nil
	right.CompletedAt = nil
	left.ExpectedStates = nil
	right.ExpectedStates = nil
	left.RequireRevision = nil
	right.RequireRevision = nil
	left.CreateOnly = false
	right.CreateOnly = false
	return reflect.DeepEqual(left, right)
}

func projectTextureDocumentAlias(ctx context.Context, tx *sql.Tx, body json.RawMessage, computerID string) error {
	var alias computerevent.TextureDocumentAliasProjection
	if err := json.Unmarshal(body, &alias); err != nil {
		return fmt.Errorf("decode Texture document alias: %w", err)
	}
	ownerID := strings.TrimSpace(alias.OwnerID)
	targetComputerID := strings.TrimSpace(alias.ComputerID)
	if targetComputerID == "" {
		targetComputerID = strings.TrimSpace(computerID)
	}
	sourcePath := strings.TrimSpace(alias.SourcePath)
	docID := strings.TrimSpace(alias.DocID)
	if ownerID == "" || sourcePath == "" || docID == "" {
		return fmt.Errorf("Texture document alias identity (owner_id, source_path, doc_id) is required")
	}
	now := time.Now().UTC()
	createdAt := now
	if alias.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, alias.CreatedAt); err == nil {
			createdAt = t
		}
	}
	updatedAt := now
	if alias.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, alias.UpdatedAt); err == nil {
			updatedAt = t
		}
	}
	if targetComputerID != "" {
		_, _ = tx.ExecContext(ctx, `DELETE FROM texture_document_aliases WHERE owner_id=? AND computer_id='' AND source_path=?`, ownerID, sourcePath)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO texture_document_aliases (
			owner_id, computer_id, source_path, doc_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			doc_id=VALUES(doc_id),
			updated_at=VALUES(updated_at)`,
		ownerID, targetComputerID, sourcePath, docID, createdAt, updatedAt)
	if err != nil {
		return fmt.Errorf("project Texture document alias: %w", err)
	}
	return nil
}

func projectTextureDocumentAliasDelete(ctx context.Context, tx *sql.Tx, body json.RawMessage, computerID string) error {
	var alias computerevent.TextureDocumentAliasProjection
	if err := json.Unmarshal(body, &alias); err != nil {
		return fmt.Errorf("decode Texture document alias delete: %w", err)
	}
	ownerID := strings.TrimSpace(alias.OwnerID)
	targetComputerID := strings.TrimSpace(alias.ComputerID)
	if targetComputerID == "" {
		targetComputerID = strings.TrimSpace(computerID)
	}
	sourcePath := strings.TrimSpace(alias.SourcePath)
	if ownerID == "" || sourcePath == "" {
		return fmt.Errorf("Texture document alias delete identity (owner_id, source_path) is required")
	}
	_, err := tx.ExecContext(ctx, `
		DELETE FROM texture_document_aliases
		WHERE owner_id=? AND (computer_id=? OR computer_id='') AND source_path=?`,
		ownerID, targetComputerID, sourcePath)
	if err != nil {
		return fmt.Errorf("project Texture document alias delete: %w", err)
	}
	return nil
}
