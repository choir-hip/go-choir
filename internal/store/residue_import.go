package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	ErrResidueImportUnbound = errors.New("store: residue import requires a bound projection tape")
	ErrResidueImportSplit   = errors.New("store: desktop and OG residue must be imported together")
)

// ResidueImportResult is the observed snapshot that was appended. It is not
// an eligibility receipt and does not reclassify EmptyUntilSupported.
type ResidueImportResult struct {
	Desktops         int
	Sessions         int
	Objects          int
	Edges            int
	RunMemoryEntries int
	StartIntents     int
	Operations       int
	TextureMutations int
	TextureAliases   int
	Appended         bool
}

// ImportResidueSnapshot appends one projection_batch_recorded event whose
// payload is the current VM-local residue. That event is "state as of now,"
// not fabricated history of prior heads. Presence fields are omitted.
// Live staging must not call this until current main is deployed. This method
// does not reclassify replay eligibility.
func (s *Store) ImportResidueSnapshot(ctx context.Context) (ResidueImportResult, error) {
	return s.ImportResidueSnapshotForOwner(ctx, "")
}

// ImportResidueSnapshotForOwner appends one owner- and computer-scoped
// projection snapshot for legacy rows that predate reducer-backed writers.
// The unscoped wrapper remains only for package tests and local migration
// tooling; the authenticated product route always supplies ownerID.
func (s *Store) ImportResidueSnapshotForOwner(ctx context.Context, ownerID string) (ResidueImportResult, error) {
	if s == nil || s.projectionTape == nil {
		return ResidueImportResult{}, ErrResidueImportUnbound
	}
	ownerID = strings.TrimSpace(ownerID)
	computerID := s.projectionTape.computerID
	desktops, err := s.snapshotResidueDesktops(ctx, ownerID, computerID)
	if err != nil {
		return ResidueImportResult{}, err
	}
	objects, err := s.snapshotResidueObjects(ctx, ownerID, computerID)
	if err != nil {
		return ResidueImportResult{}, err
	}
	edges, err := s.snapshotResidueEdges(ctx, ownerID, computerID)
	if err != nil {
		return ResidueImportResult{}, err
	}
	runtimeOps, counts, err := s.snapshotResidueRuntime(ctx, ownerID, computerID, objects)
	if err != nil {
		return ResidueImportResult{}, err
	}
	aliasOps, err := s.snapshotResidueAliases(ctx, ownerID, computerID)
	if err != nil {
		return ResidueImportResult{}, err
	}
	sessionCount := 0
	for _, desktop := range desktops {
		sessionCount += len(desktop.Sessions)
	}
	result := ResidueImportResult{
		Desktops: len(desktops), Sessions: sessionCount, Objects: len(objects), Edges: len(edges),
		RunMemoryEntries: counts.runMemory, StartIntents: counts.startIntents,
		Operations: counts.operations, TextureMutations: counts.textureMutations,
		TextureAliases: len(aliasOps),
	}
	desktopResidue := result.Desktops > 0 || result.Sessions > 0
	ogResidue := result.Objects > 0 || result.Edges > 0
	if desktopResidue != ogResidue {
		return result, ErrResidueImportSplit
	}
	if !desktopResidue && !ogResidue && len(runtimeOps) == 0 && len(aliasOps) == 0 {
		return result, nil
	}
	ops := make([]computerevent.ProjectionOp, 0, result.Desktops+result.Objects+result.Edges+len(runtimeOps)+len(aliasOps))
	for _, desktop := range desktops {
		body, err := json.Marshal(desktop)
		if err != nil {
			return result, fmt.Errorf("store: marshal residue desktop: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpDesktopState, Body: body})
	}
	for _, obj := range objects {
		body, err := json.Marshal(obj)
		if err != nil {
			return result, fmt.Errorf("store: marshal residue object: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: obj.CanonicalID, Body: body})
	}
	for _, edge := range edges {
		body, err := json.Marshal(edge)
		if err != nil {
			return result, fmt.Errorf("store: marshal residue edge: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObjectEdge, CanonicalID: edge.EdgeID, Body: body})
	}
	ops = append(ops, runtimeOps...)
	ops = append(ops, aliasOps...)
	if err := s.projectionTape.appendOps(ctx, ops); err != nil {
		return result, err
	}
	result.Appended = true
	return result, nil
}

type desktopKey struct {
	ownerID   string
	desktopID string
}

type residueRuntimeCounts struct {
	runMemory        int
	startIntents     int
	operations       int
	textureMutations int
}

func (s *Store) snapshotResidueDesktops(ctx context.Context, ownerID, computerID string) ([]projectedDesktopState, error) {
	keys := map[desktopKey]struct{}{}
	if err := s.collectDesktopKeys(ctx, `SELECT DISTINCT owner_id, desktop_id FROM desktop_workspaces`, ownerID, computerID, keys); err != nil {
		return nil, err
	}
	if err := s.collectDesktopKeys(ctx, `SELECT DISTINCT owner_id, desktop_id FROM desktop_app_instances`, ownerID, computerID, keys); err != nil {
		return nil, err
	}
	if err := s.collectDesktopKeys(ctx, `SELECT DISTINCT owner_id, desktop_id FROM desktop_window_placements`, ownerID, computerID, keys); err != nil {
		return nil, err
	}
	if err := s.collectDesktopKeys(ctx, `SELECT DISTINCT owner_id, desktop_id FROM desktop_sessions`, ownerID, computerID, keys); err != nil {
		return nil, err
	}
	sessions, err := s.snapshotResidueSessionIdentities(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	out := make([]projectedDesktopState, 0, len(keys))
	for key := range keys {
		state, err := s.GetDesktopStateForSession(ctx, key.ownerID, key.desktopID, "projected")
		if err != nil {
			return nil, err
		}
		projected := desktopStateProjection(state, key.desktopID, "projected", state.UpdatedAt)
		projected.Sessions = sessions[key]
		out = append(out, projected)
	}
	return out, nil
}

func (s *Store) collectDesktopKeys(ctx context.Context, query, ownerID, computerID string, keys map[desktopKey]struct{}) error {
	args := []any{}
	ownerID = strings.TrimSpace(ownerID)
	computerID = strings.TrimSpace(computerID)
	if ownerID != "" && computerID != "" {
		query += ` WHERE owner_id = ? AND (computer_id = ? OR computer_id = '')`
		args = append(args, ownerID, computerID)
	} else if ownerID != "" {
		query += ` WHERE owner_id = ?`
		args = append(args, ownerID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("store: list residue desktops: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var ownerID, desktopID string
		if err := rows.Scan(&ownerID, &desktopID); err != nil {
			return fmt.Errorf("store: scan residue desktop: %w", err)
		}
		ownerID = strings.TrimSpace(ownerID)
		desktopID = normalizeDesktopID(desktopID)
		if ownerID == "" {
			continue
		}
		keys[desktopKey{ownerID: ownerID, desktopID: desktopID}] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate residue desktops: %w", err)
	}
	return nil
}

func (s *Store) snapshotResidueSessionIdentities(ctx context.Context, ownerID, computerID string) (map[desktopKey][]projectedSessionIdentity, error) {
	query := `SELECT owner_id, desktop_id, session_id, device_id, viewport_profile FROM desktop_sessions`
	args := []any{}
	ownerID = strings.TrimSpace(ownerID)
	computerID = strings.TrimSpace(computerID)
	if ownerID != "" && computerID != "" {
		query += ` WHERE owner_id = ? AND (computer_id = ? OR computer_id = '')`
		args = append(args, ownerID, computerID)
	} else if ownerID != "" {
		query += ` WHERE owner_id = ?`
		args = append(args, ownerID)
	}
	query += ` ORDER BY owner_id, desktop_id, session_id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list residue sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[desktopKey][]projectedSessionIdentity{}
	for rows.Next() {
		var ownerID, desktopID, sessionID, deviceID, viewport string
		if err := rows.Scan(&ownerID, &desktopID, &sessionID, &deviceID, &viewport); err != nil {
			return nil, fmt.Errorf("store: scan residue session: %w", err)
		}
		key := desktopKey{ownerID: strings.TrimSpace(ownerID), desktopID: normalizeDesktopID(desktopID)}
		out[key] = append(out[key], projectedSessionIdentity{
			SessionID:       strings.TrimSpace(sessionID),
			DeviceID:        strings.TrimSpace(deviceID),
			ViewportProfile: strings.TrimSpace(viewport),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate residue sessions: %w", err)
	}
	return out, nil
}

func (s *Store) snapshotResidueObjects(ctx context.Context, ownerID, computerID string) ([]objectgraph.Object, error) {
	query := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects`
	args := []any{}
	if strings.TrimSpace(ownerID) != "" {
		query += ` WHERE owner_id = ? AND computer_id = ?`
		args = append(args, strings.TrimSpace(ownerID), strings.TrimSpace(computerID))
	}
	query += ` ORDER BY canonical_id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list residue objects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []objectgraph.Object
	for rows.Next() {
		obj, err := scanResidueObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate residue objects: %w", err)
	}
	return out, nil
}

func (s *Store) snapshotResidueEdges(ctx context.Context, ownerID, computerID string) ([]objectgraph.Edge, error) {
	query := `SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone FROM og_edges`
	args := []any{}
	if strings.TrimSpace(ownerID) != "" {
		query += ` WHERE EXISTS (SELECT 1 FROM og_objects o WHERE o.owner_id = ? AND o.computer_id = ? AND (o.canonical_id = og_edges.from_id OR o.canonical_id = og_edges.to_id))`
		args = append(args, strings.TrimSpace(ownerID), strings.TrimSpace(computerID))
	}
	query += ` ORDER BY edge_id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list residue edges: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []objectgraph.Edge
	for rows.Next() {
		edge, err := scanResidueEdge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate residue edges: %w", err)
	}
	return out, nil
}

type residueRunScope struct {
	ownerID string
	runID   string
}

// residueRunScopes joins legacy SQL memory rows to the current canonical run
// authority. Production runs are object-graph records; the SQL runs table is
// retained only as a compatibility source for rows created before that cutover.
func (s *Store) residueRunScopes(ctx context.Context, ownerID, computerID string, objects []objectgraph.Object) ([]residueRunScope, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	scopes := make(map[residueRunScope]struct{})
	add := func(candidateOwnerID, runID string) {
		candidateOwnerID, runID = strings.TrimSpace(candidateOwnerID), strings.TrimSpace(runID)
		if candidateOwnerID == "" || runID == "" {
			return
		}
		if ownerID != "" && candidateOwnerID != ownerID {
			return
		}
		scopes[residueRunScope{ownerID: candidateOwnerID, runID: runID}] = struct{}{}
	}
	addRunObject := func(obj objectgraph.Object) error {
		if obj.Tombstone || obj.ObjectKind != ogKindRun {
			return nil
		}
		var rec types.RunRecord
		if err := json.Unmarshal(obj.Body, &rec); err != nil {
			return fmt.Errorf("store: decode residue run body: %w", err)
		}
		var metadata struct {
			RunID      string `json:"run_id"`
			ComputerID string `json:"computer_id"`
		}
		if err := json.Unmarshal(obj.Metadata, &metadata); err != nil {
			return fmt.Errorf("store: decode residue run metadata: %w", err)
		}
		if strings.TrimSpace(rec.RunID) == "" {
			rec.RunID = metadata.RunID
		}
		if strings.TrimSpace(rec.ComputerID) == "" {
			rec.ComputerID = metadata.ComputerID
		}
		if strings.TrimSpace(rec.ComputerID) == "" {
			rec.ComputerID = obj.ComputerID
		}
		if strings.TrimSpace(rec.ComputerID) != computerID {
			return nil
		}
		objectOwnerID, bodyOwnerID := strings.TrimSpace(obj.OwnerID), strings.TrimSpace(rec.OwnerID)
		if objectOwnerID != "" && bodyOwnerID != "" && objectOwnerID != bodyOwnerID {
			return fmt.Errorf("store: residue run owner mismatch: object=%q body=%q", objectOwnerID, bodyOwnerID)
		}
		if bodyOwnerID == "" {
			bodyOwnerID = objectOwnerID
		}
		add(bodyOwnerID, rec.RunID)
		return nil
	}

	// Lifecycle objects are already scoped by snapshotResidueObjects. Their
	// storage computer_id is authoritative when present.
	for _, obj := range objects {
		if strings.TrimSpace(obj.ComputerID) != computerID {
			continue
		}
		if err := addRunObject(obj); err != nil {
			return nil, err
		}
	}

	// CreateRunOG stores the run's computer identity in the canonical body and
	// metadata, while the object-graph computer_id column remains empty. Query
	// the canonical metadata index instead of treating that storage column as
	// the run's scope; otherwise its durable run_memory_entries are omitted.
	canonicalRuns, err := s.ogListAllByMetadata(ctx, ogKindRun, "computer_id", computerID)
	if err != nil {
		return nil, fmt.Errorf("store: list canonical residue runs: %w", err)
	}
	for _, obj := range canonicalRuns {
		if err := addRunObject(obj); err != nil {
			return nil, err
		}
	}

	legacyQuery := `SELECT loop_id, owner_id FROM runs WHERE computer_id=?`
	legacyArgs := []any{strings.TrimSpace(computerID)}
	if strings.TrimSpace(ownerID) != "" {
		legacyQuery += ` AND owner_id=?`
		legacyArgs = append(legacyArgs, strings.TrimSpace(ownerID))
	}
	rows, err := s.db.QueryContext(ctx, legacyQuery, legacyArgs...)
	if err != nil {
		return nil, fmt.Errorf("store: list legacy residue runs: %w", err)
	}
	for rows.Next() {
		var runID, runOwnerID string
		if err := rows.Scan(&runID, &runOwnerID); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("store: scan legacy residue run: %w", err)
		}
		add(runOwnerID, runID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("store: iterate legacy residue runs: %w", err)
	}
	_ = rows.Close()

	out := make([]residueRunScope, 0, len(scopes))
	for scope := range scopes {
		out = append(out, scope)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ownerID != out[j].ownerID {
			return out[i].ownerID < out[j].ownerID
		}
		return out[i].runID < out[j].runID
	})
	return out, nil
}

func (s *Store) snapshotResidueRuntime(ctx context.Context, ownerID, computerID string, objects []objectgraph.Object) ([]computerevent.ProjectionOp, residueRuntimeCounts, error) {
	var counts residueRuntimeCounts
	ops := make([]computerevent.ProjectionOp, 0)

	runScopes, err := s.residueRunScopes(ctx, ownerID, computerID, objects)
	if err != nil {
		return nil, counts, err
	}
	var rows *sql.Rows
	if len(runScopes) > 0 {
		clauses := make([]string, len(runScopes))
		runMemoryArgs := make([]any, 0, len(runScopes)*2)
		for i, scope := range runScopes {
			clauses[i] = `(e.owner_id=? AND e.loop_id=?)`
			runMemoryArgs = append(runMemoryArgs, scope.ownerID, scope.runID)
		}
		runMemoryQuery := `SELECT e.entry_id, e.loop_id, e.owner_id, e.agent_id, e.parent_entry_id, e.seq,
			e.kind, e.role, e.message_json, e.summary, e.first_kept_entry_id, e.tokens_before,
			e.reason, e.model, e.details_json, e.created_at
			FROM run_memory_entries e WHERE ` + strings.Join(clauses, ` OR `) + `
			ORDER BY e.owner_id, e.loop_id, e.seq`
		rows, err = s.db.QueryContext(ctx, runMemoryQuery, runMemoryArgs...)
		if err != nil {
			return nil, counts, fmt.Errorf("store: list residue run memory: %w", err)
		}
		for rows.Next() {
			var entry computerevent.RunMemoryEntryProjection
			if err := rows.Scan(&entry.EntryID, &entry.RunID, &entry.OwnerID, &entry.AgentID, &entry.ParentEntryID,
				&entry.Seq, &entry.Kind, &entry.Role, &entry.MessageJSON, &entry.Summary, &entry.FirstKeptEntryID,
				&entry.TokensBefore, &entry.Reason, &entry.Model, &entry.DetailsJSON, &entry.CreatedAt); err != nil {
				_ = rows.Close()
				return nil, counts, fmt.Errorf("store: scan residue run memory: %w", err)
			}
			body, err := json.Marshal(entry)
			if err != nil {
				_ = rows.Close()
				return nil, counts, fmt.Errorf("store: marshal residue run memory: %w", err)
			}
			ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpRunMemoryEntry, CanonicalID: entry.EntryID, Body: body})
			counts.runMemory++
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: iterate residue run memory: %w", err)
		}
		_ = rows.Close()
	}
	startQuery := `SELECT computer_id, idempotency_key, request_commitment, created_at FROM self_development_start_intents WHERE computer_id=?`
	rows, err = s.db.QueryContext(ctx, startQuery, strings.TrimSpace(computerID))
	if err != nil {
		return nil, counts, fmt.Errorf("store: list residue start intents: %w", err)
	}
	for rows.Next() {
		var intent computerevent.SelfDevelopmentStartIntentProjection
		if err := rows.Scan(&intent.ComputerID, &intent.IdempotencyKey, &intent.RequestCommitment, &intent.CreatedAt); err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: scan residue start intent: %w", err)
		}
		body, err := json.Marshal(intent)
		if err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: marshal residue start intent: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpSelfDevelopmentStartIntent, CanonicalID: intent.ComputerID + "\x00" + intent.IdempotencyKey, Body: body})
		counts.startIntents++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, counts, fmt.Errorf("store: iterate residue start intents: %w", err)
	}
	_ = rows.Close()

	operationQuery := `SELECT operation_id, idempotency_key, request_commitment, computer_id, trajectory_id, capsule_id,
		base_head, prompt_artifact_ref, bundle_digest, release_digest, code_ref, artifact_program_ref,
		verifier_refs_json, decision_actor, decision_event, decision_receipt, desired_head, effective_head,
		materialization_receipt, checkpoint_ref, route_certificate, route_generation, route_receipt,
		mode_receipt, lifecycle_receipt, state, terminal_error, created_at, updated_at
		FROM self_development_operations WHERE computer_id=? ORDER BY created_at, operation_id`
	rows, err = s.db.QueryContext(ctx, operationQuery, strings.TrimSpace(computerID))
	if err != nil {
		return nil, counts, fmt.Errorf("store: list residue self-development operations: %w", err)
	}
	for rows.Next() {
		var operation computerevent.SelfDevelopmentOperationProjection
		var routeGeneration sql.NullInt64
		if err := rows.Scan(&operation.OperationID, &operation.IdempotencyKey, &operation.RequestCommitment, &operation.ComputerID,
			&operation.TrajectoryID, &operation.CapsuleID, &operation.BaseHead, &operation.PromptArtifactRef,
			&operation.BundleDigest, &operation.ReleaseDigest, &operation.CodeRef, &operation.ArtifactProgramRef,
			&operation.VerifierRefsJSON, &operation.DecisionActor, &operation.DecisionEvent, &operation.DecisionReceipt,
			&operation.DesiredHead, &operation.EffectiveHead, &operation.MaterializationReceipt, &operation.CheckpointRef,
			&operation.RouteCertificate, &routeGeneration, &operation.RouteReceipt, &operation.ModeReceipt,
			&operation.LifecycleReceipt, &operation.State, &operation.TerminalError, &operation.CreatedAt, &operation.UpdatedAt); err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: scan residue self-development operation: %w", err)
		}
		if routeGeneration.Valid {
			value := uint64(routeGeneration.Int64)
			operation.RouteGeneration = &value
		}
		body, err := json.Marshal(operation)
		if err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: marshal residue self-development operation: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpSelfDevelopmentOperation, CanonicalID: operation.OperationID, Body: body})
		counts.operations++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, counts, fmt.Errorf("store: iterate residue self-development operations: %w", err)
	}
	_ = rows.Close()

	mutationQuery := `SELECT doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at
		FROM texture_agent_mutations WHERE (computer_id=? OR computer_id='')`
	mutationArgs := []any{strings.TrimSpace(computerID)}
	if strings.TrimSpace(ownerID) != "" {
		mutationQuery += ` AND owner_id=?`
		mutationArgs = append(mutationArgs, strings.TrimSpace(ownerID))
	}
	mutationQuery += ` ORDER BY owner_id, doc_id, loop_id`
	rows, err = s.db.QueryContext(ctx, mutationQuery, mutationArgs...)
	if err != nil {
		return nil, counts, fmt.Errorf("store: list residue Texture mutations: %w", err)
	}
	for rows.Next() {
		var mutation computerevent.TextureAgentMutationProjection
		var completedAt sql.NullString
		if err := rows.Scan(&mutation.DocID, &mutation.RunID, &mutation.OwnerID, &mutation.ComputerID, &mutation.State,
			&mutation.ScheduledMessageSeq, &mutation.RevisionID, &mutation.CreatedAt, &completedAt); err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: scan residue Texture mutation: %w", err)
		}
		if completedAt.Valid {
			mutation.CompletedAt = &completedAt.String
		}
		body, err := json.Marshal(mutation)
		if err != nil {
			_ = rows.Close()
			return nil, counts, fmt.Errorf("store: marshal residue Texture mutation: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpTextureAgentMutation,
			CanonicalID: mutation.OwnerID + "\x00" + mutation.ComputerID + "\x00" + mutation.DocID + "\x00" + mutation.RunID, Body: body})
		counts.textureMutations++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, counts, fmt.Errorf("store: iterate residue Texture mutations: %w", err)
	}
	_ = rows.Close()
	return ops, counts, nil
}

func scanResidueObject(row interface{ Scan(...any) error }) (objectgraph.Object, error) {
	var obj objectgraph.Object
	var metadata string
	if err := row.Scan(
		&obj.CanonicalID, &obj.ObjectKind, &obj.OwnerID, &obj.ComputerID,
		&obj.VersionID, &obj.ContentHash, &obj.Body, &metadata,
		&obj.CreatedAt, &obj.UpdatedAt, &obj.Tombstone, &obj.SupersededBy,
	); err != nil {
		return objectgraph.Object{}, fmt.Errorf("store: scan residue object: %w", err)
	}
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	obj.Metadata = json.RawMessage(metadata)
	obj.CreatedAt = obj.CreatedAt.UTC()
	obj.UpdatedAt = obj.UpdatedAt.UTC()
	return obj, nil
}

func scanResidueEdge(row interface{ Scan(...any) error }) (objectgraph.Edge, error) {
	var edge objectgraph.Edge
	var metadata string
	if err := row.Scan(
		&edge.EdgeID, &edge.FromID, &edge.ToID, &edge.Kind, &metadata,
		&edge.CreatedAt, &edge.Tombstone,
	); err != nil {
		return objectgraph.Edge{}, fmt.Errorf("store: scan residue edge: %w", err)
	}
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	edge.Metadata = json.RawMessage(metadata)
	edge.CreatedAt = edge.CreatedAt.UTC()
	return edge, nil
}

func (s *Store) snapshotResidueAliases(ctx context.Context, ownerID, computerID string) ([]computerevent.ProjectionOp, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	query := `SELECT owner_id, computer_id, source_path, doc_id, created_at, updated_at FROM texture_document_aliases`
	args := []any{}
	if ownerID != "" && computerID != "" {
		query += ` WHERE owner_id = ? AND (computer_id = ? OR computer_id = '')`
		args = append(args, ownerID, computerID)
	} else if ownerID != "" {
		query += ` WHERE owner_id = ?`
		args = append(args, ownerID)
	}
	query += ` ORDER BY source_path`
	rows, err := s.textureHandle().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list residue texture aliases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ops []computerevent.ProjectionOp
	for rows.Next() {
		var oID, cID, sourcePath, docID string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&oID, &cID, &sourcePath, &docID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("store: scan residue texture alias: %w", err)
		}
		if cID == "" {
			cID = computerID
		}
		proj := computerevent.TextureDocumentAliasProjection{
			OwnerID:    oID,
			ComputerID: cID,
			SourcePath: sourcePath,
			DocID:      docID,
			CreatedAt:  createdAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:  updatedAt.UTC().Format(time.RFC3339Nano),
		}
		body, err := json.Marshal(proj)
		if err != nil {
			return nil, fmt.Errorf("store: marshal residue texture alias: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{
			Kind: computerevent.ProjectionOpTextureDocumentAlias,
			Body: body,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate residue texture aliases: %w", err)
	}
	return ops, nil
}
