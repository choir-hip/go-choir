package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const ProjectionImportV1Schema = "choir.supervision_projection_import.v1"

type ProjectionImportV1 struct {
	Schema                   string                         `json:"schema"`
	ReducerVersion           string                         `json:"reducer_version"`
	OwnerID                  string                         `json:"owner_id"`
	ComputerID               string                         `json:"computer_id"`
	SourceDoltCommit         string                         `json:"source_dolt_commit"`
	SourceProjectionDigest   string                         `json:"source_projection_digest"`
	LegacyLifecycleWatermark uint64                         `json:"legacy_lifecycle_watermark"`
	CutoverBarrier           ProjectionImportCutoverBarrier `json:"cutover_barrier"`
	Objects                  []ProjectionImportObject       `json:"objects"`
	Edges                    []ProjectionImportEdge         `json:"edges"`
	ExplicitRefusals         []ProjectionImportRefusal      `json:"explicit_refusals"`
	Snapshot                 ProjectionImportSnapshot       `json:"snapshot"`
	ProjectionDigest         string                         `json:"projection_digest"`
}

type ProjectionImportCutoverBarrier struct {
	WritesDisabledAt     time.Time `json:"writes_disabled_at"`
	AdmissionWatermark   uint64    `json:"admission_watermark"`
	ActiveRunIDs         []string  `json:"active_run_ids"`
	ActiveAttemptIDs     []string  `json:"active_attempt_ids"`
	PendingDeliveryIDs   []string  `json:"pending_delivery_ids"`
	SlotClaimIDs         []string  `json:"slot_claim_ids"`
	AgentMutationIDs     []string  `json:"agent_mutation_ids"`
	CommandReceiptIDs    []string  `json:"command_receipt_ids"`
	DrainReceiptRefs     []string  `json:"drain_receipt_refs"`
	QuiescenceReceiptRef string    `json:"quiescence_receipt_ref"`
}

type ProjectionImportObject struct {
	Kind        string          `json:"kind"`
	CanonicalID string          `json:"canonical_id"`
	ContentHash string          `json:"content_hash"`
	Body        json.RawMessage `json:"body"`
}

type ProjectionImportEdge struct {
	EdgeID      string `json:"edge_id"`
	FromID      string `json:"from_id"`
	ToID        string `json:"to_id"`
	Kind        string `json:"kind"`
	ContentHash string `json:"content_hash"`
}

type ProjectionImportRefusal struct {
	LegacyKind  string `json:"legacy_kind"`
	LegacyID    string `json:"legacy_id"`
	Reason      string `json:"reason"`
	ContentHash string `json:"content_hash"`
}

// ProjectionImportSnapshot is the complete legacy state which the canonical
// projection_imported mutation materializes. Lifecycle events are deliberately
// absent: they are audit-only refused inputs, never canonical replay input.
type ProjectionImportSnapshot struct {
	Trajectory       types.TrajectoryRecord           `json:"trajectory"`
	IntentRevisionID string                           `json:"intent_revision_id"`
	WorkItems        []types.WorkItemRecord           `json:"work_items"`
	Agents           []types.AgentRecord              `json:"agents"`
	Updates          []types.CoagentSourcePacket      `json:"updates"`
	Document         types.Document                   `json:"document"`
	HeadRevision     types.Revision                   `json:"head_revision"`
	SourceEntities   []TextureSourceEntityGraphRecord `json:"source_entities"`
	SourceRefs       []TextureSourceRefGraphRecord    `json:"source_refs"`
}

type ProjectionImportBuildOptions struct {
	// These receipts must name already-pinned immutable quiescence evidence.
	// The appender does not synthesize them, preventing an unpinned import_ref.
	QuiescenceReceiptRef string
	DrainReceiptRefs     []string
	SourceDoltCommit     string
	WritesDisabledAt     time.Time
}

// LegacyProjectionImportBarrier excludes every legacy lifecycle writer while
// its holder snapshots and imports one trajectory. It owns trajectoryMu rather
// than reporting a merely advisory quiescence observation.
//
// The barrier is intentionally released only by Release. Callers must defer
// Release immediately after acquisition.
type LegacyProjectionImportBarrier struct {
	store        *Store
	ownerID      string
	computerID   string
	trajectoryID string
	acquiredAt   time.Time
	released     bool
}

// ProjectionImportBarrierState is the observed, gated lifecycle state used to
// bind import evidence. It is sampled while legacy lifecycle mutation is
// excluded, never synthesized by the importer.
type ProjectionImportBarrierState struct {
	OwnerID            string
	ComputerID         string
	TrajectoryID       string
	WritesDisabledAt   time.Time
	AdmissionWatermark uint64
}

// AcquireLegacyProjectionImportBarrier serializes legacy write admission with
// import construction. lifecycle mutation paths take trajectoryMu before their
// durable admission/update, so once this returns the observed watermark cannot
// advance until Release.
func (s *Store) AcquireLegacyProjectionImportBarrier(ctx context.Context, ownerID, computerID, trajectoryID string) (*LegacyProjectionImportBarrier, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return nil, err
	}
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		return nil, fmt.Errorf("projection import barrier: trajectory_id is required")
	}
	s.trajectoryMu.Lock()
	if err := ctx.Err(); err != nil {
		s.trajectoryMu.Unlock()
		return nil, err
	}
	return &LegacyProjectionImportBarrier{
		store: s, ownerID: ownerID, computerID: computerID, trajectoryID: trajectoryID, acquiredAt: time.Now().UTC(),
	}, nil
}

// State proves that the held barrier still protects a quiescent source and
// returns its actual lifecycle admission watermark.
func (b *LegacyProjectionImportBarrier) State(ctx context.Context) (ProjectionImportBarrierState, error) {
	if b == nil || b.store == nil || b.released {
		return ProjectionImportBarrierState{}, fmt.Errorf("projection import barrier is not held")
	}
	snapshot, err := b.store.GetLifecycleSnapshot(ctx, b.ownerID, b.computerID, b.trajectoryID)
	if err != nil {
		return ProjectionImportBarrierState{}, err
	}
	if err := b.store.validateProjectionImportQuiescence(ctx, snapshot); err != nil {
		return ProjectionImportBarrierState{}, err
	}
	return ProjectionImportBarrierState{
		OwnerID: b.ownerID, ComputerID: b.computerID, TrajectoryID: b.trajectoryID,
		WritesDisabledAt: b.acquiredAt, AdmissionWatermark: uint64(snapshot.Watermark),
	}, nil
}

// Release admits legacy lifecycle writers again. It is safe to call repeatedly
// so callers can defer it across every import failure path.
func (b *LegacyProjectionImportBarrier) Release() {
	if b == nil || b.store == nil || b.released {
		return
	}
	b.released = true
	b.store.trajectoryMu.Unlock()
}

// BuildProjectionImportV1 snapshots one legacy Texture lifecycle trajectory
// only after every mutable execution surface is empty. It does not append: the
// caller must pin this canonical manifest and pass that exact ref to NewProjectionImportTransaction.
func (s *Store) BuildProjectionImportV1(ctx context.Context, ownerID, computerID, trajectoryID string, options ProjectionImportBuildOptions) (ProjectionImportV1, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return ProjectionImportV1{}, err
	}
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		return ProjectionImportV1{}, fmt.Errorf("projection import: trajectory_id is required")
	}
	if _, err := s.GetSupervisionProjectionSnapshot(ctx, ownerID, computerID, trajectoryID); err == nil {
		return ProjectionImportV1{}, fmt.Errorf("projection import: trajectory is already imported")
	} else if err != nil && err != ErrNotFound {
		return ProjectionImportV1{}, err
	}
	if _, err := computerevent.ParseArtifactRef(options.QuiescenceReceiptRef); err != nil && !computerevent.IsSupervisionArtifactPlaceholder(options.QuiescenceReceiptRef) {
		return ProjectionImportV1{}, fmt.Errorf("projection import: quiescence receipt: %w", err)
	}
	if len(options.DrainReceiptRefs) == 0 {
		return ProjectionImportV1{}, fmt.Errorf("projection import: drain receipts are required")
	}
	for _, ref := range options.DrainReceiptRefs {
		if _, err := computerevent.ParseArtifactRef(ref); err != nil && !computerevent.IsSupervisionArtifactPlaceholder(ref) {
			return ProjectionImportV1{}, fmt.Errorf("projection import: drain receipt: %w", err)
		}
	}
	if options.WritesDisabledAt.IsZero() {
		return ProjectionImportV1{}, fmt.Errorf("projection import: writes_disabled_at is required")
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return ProjectionImportV1{}, err
	}
	if err := s.validateProjectionImportQuiescence(ctx, snapshot); err != nil {
		return ProjectionImportV1{}, err
	}
	sourceEntities, err := s.ListTextureSourceEntities(ctx, ownerID)
	if err != nil {
		return ProjectionImportV1{}, err
	}
	sourceRefs, err := s.ListTextureSourceRefsForRevision(ctx, ownerID, snapshot.Document.DocID, snapshot.HeadRevision.RevisionID)
	if err != nil {
		return ProjectionImportV1{}, err
	}
	scopedRefs := sourceRefs[:0]
	for _, ref := range sourceRefs {
		if ref.ComputerID == "" || ref.ComputerID == computerID {
			scopedRefs = append(scopedRefs, ref)
		}
	}
	sourceRefs = append([]TextureSourceRefGraphRecord(nil), scopedRefs...)
	entityIDs := make(map[string]struct{}, len(sourceRefs))
	for _, ref := range sourceRefs {
		entityIDs[ref.SourceEntityCanonicalID] = struct{}{}
	}
	filterEntities := sourceEntities[:0]
	for _, entity := range sourceEntities {
		if _, referenced := entityIDs[entity.CanonicalID]; referenced && (entity.ComputerID == "" || entity.ComputerID == computerID) {
			filterEntities = append(filterEntities, entity)
		}
	}
	sourceEntities = append([]TextureSourceEntityGraphRecord(nil), filterEntities...)
	intentRevisionID := strings.TrimSpace(snapshot.Trajectory.SubjectRefs["intent_revision_id"])
	if intentRevisionID == "" {
		intentRevisionID = snapshot.HeadRevision.RevisionID
	}
	importSnapshot := ProjectionImportSnapshot{Trajectory: snapshot.Trajectory, IntentRevisionID: intentRevisionID, WorkItems: append([]types.WorkItemRecord(nil), snapshot.WorkItems...), Agents: append([]types.AgentRecord(nil), snapshot.Agents...), Updates: append([]types.CoagentSourcePacket(nil), snapshot.Updates...), Document: snapshot.Document, HeadRevision: snapshot.HeadRevision, SourceEntities: sourceEntities, SourceRefs: append([]TextureSourceRefGraphRecord(nil), sourceRefs...)}
	sortProjectionImportSnapshot(&importSnapshot)
	sourceDigest, err := projectionImportDigest(importSnapshot)
	if err != nil {
		return ProjectionImportV1{}, err
	}
	commit := strings.TrimSpace(options.SourceDoltCommit)
	if commit == "" {
		if err := s.textureHandle().QueryRowContext(ctx, "SELECT DOLT_HASHOF('HEAD')").Scan(&commit); err != nil {
			return ProjectionImportV1{}, fmt.Errorf("projection import: source Dolt commit: %w", err)
		}
	}
	if commit == "" {
		return ProjectionImportV1{}, fmt.Errorf("projection import: source Dolt commit is empty")
	}
	manifest := ProjectionImportV1{Schema: ProjectionImportV1Schema, ReducerVersion: computerevent.SupervisionReducerV1, OwnerID: ownerID, ComputerID: computerID, SourceDoltCommit: commit, SourceProjectionDigest: sourceDigest, LegacyLifecycleWatermark: uint64(snapshot.Watermark), CutoverBarrier: ProjectionImportCutoverBarrier{WritesDisabledAt: options.WritesDisabledAt.UTC(), AdmissionWatermark: uint64(snapshot.Watermark), ActiveRunIDs: []string{}, ActiveAttemptIDs: []string{}, PendingDeliveryIDs: []string{}, SlotClaimIDs: []string{}, AgentMutationIDs: []string{}, CommandReceiptIDs: []string{}, DrainReceiptRefs: append([]string(nil), options.DrainReceiptRefs...), QuiescenceReceiptRef: options.QuiescenceReceiptRef}, Snapshot: importSnapshot}
	manifest.Objects, manifest.Edges, err = buildProjectionImportObjects(importSnapshot)
	if err != nil {
		return ProjectionImportV1{}, err
	}
	for _, event := range snapshot.Events {
		refusal := ProjectionImportRefusal{LegacyKind: "lifecycle_event", LegacyID: event.EventID, Reason: "derived_recovery_state"}
		refusal.ContentHash, err = projectionImportDigest(refusal)
		if err != nil {
			return ProjectionImportV1{}, err
		}
		manifest.ExplicitRefusals = append(manifest.ExplicitRefusals, refusal)
	}
	sort.Slice(manifest.ExplicitRefusals, func(i, j int) bool {
		return manifest.ExplicitRefusals[i].LegacyID < manifest.ExplicitRefusals[j].LegacyID
	})
	manifest.ProjectionDigest, err = projectionImportManifestDigest(manifest)
	if err != nil {
		return ProjectionImportV1{}, err
	}
	return manifest, nil
}

func (s *Store) validateProjectionImportQuiescence(ctx context.Context, snapshot types.LifecycleSnapshot) error {
	activeRuns, err := s.ListActiveLifecycleRunsByTrajectory(ctx, snapshot.Trajectory.OwnerID, snapshot.Trajectory.ComputerID, snapshot.Trajectory.TrajectoryID, 0)
	if err != nil {
		return err
	}
	if len(activeRuns) != 0 {
		return fmt.Errorf("projection import: active runs remain")
	}
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen {
			return fmt.Errorf("projection import: active attempts remain")
		}
	}
	for _, update := range snapshot.Updates {
		if update.Disposition == types.UpdatePending {
			return fmt.Errorf("projection import: pending deliveries remain")
		}
	}
	var slots, mutations int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM co_super_slots WHERE owner_id=? AND trajectory_id=?`, snapshot.Trajectory.OwnerID, snapshot.Trajectory.TrajectoryID).Scan(&slots); err != nil {
		return fmt.Errorf("projection import: query slot claims: %w", err)
	}
	if slots != 0 {
		return fmt.Errorf("projection import: slot claims remain")
	}
	if err := s.textureHandle().QueryRowContext(ctx, `SELECT COUNT(*) FROM texture_agent_mutations WHERE owner_id=? AND computer_id=? AND doc_id=? AND state IN ('pending','sleeping')`, snapshot.Trajectory.OwnerID, snapshot.Trajectory.ComputerID, snapshot.Document.DocID).Scan(&mutations); err != nil {
		return fmt.Errorf("projection import: query agent mutations: %w", err)
	}
	if mutations != 0 {
		return fmt.Errorf("projection import: agent mutations remain")
	}
	return nil
}

func NewProjectionImportTransaction(manifest ProjectionImportV1, importRef, commandID string) (computerevent.SupervisionTransaction, error) {
	if err := validateProjectionImportManifest(manifest); err != nil {
		return computerevent.SupervisionTransaction{}, err
	}
	if _, err := computerevent.ParseArtifactRef(importRef); err != nil && !computerevent.IsSupervisionArtifactPlaceholder(importRef) {
		return computerevent.SupervisionTransaction{}, fmt.Errorf("projection import: pinned import ref is invalid")
	}
	manifestPlaintextDigest, err := projectionImportDigest(manifest)
	if err != nil {
		return computerevent.SupervisionTransaction{}, err
	}
	body, err := json.Marshal(struct {
		ImportRef                     string             `json:"import_ref"`
		ImportDigest                  string             `json:"import_digest"`
		ImportArtifactPlaintextDigest string             `json:"import_artifact_plaintext_digest"`
		SourceDoltCommit              string             `json:"source_dolt_commit"`
		SourceProjectionDigest        string             `json:"source_projection_digest"`
		LegacyLifecycleWatermark      uint64             `json:"legacy_lifecycle_watermark"`
		ObjectCount                   uint64             `json:"object_count"`
		EdgeCount                     uint64             `json:"edge_count"`
		RefusalCount                  uint64             `json:"refusal_count"`
		QuiescenceReceiptRef          string             `json:"quiescence_receipt_ref"`
		DrainReceiptRefs              []string           `json:"drain_receipt_refs"`
		Manifest                      ProjectionImportV1 `json:"manifest"`
	}{
		ImportRef: importRef, ImportDigest: manifest.ProjectionDigest,
		ImportArtifactPlaintextDigest: manifestPlaintextDigest,
		SourceDoltCommit:              manifest.SourceDoltCommit, SourceProjectionDigest: manifest.SourceProjectionDigest,
		LegacyLifecycleWatermark: manifest.LegacyLifecycleWatermark,
		ObjectCount:              uint64(len(manifest.Objects)), EdgeCount: uint64(len(manifest.Edges)),
		RefusalCount:         uint64(len(manifest.ExplicitRefusals)),
		QuiescenceReceiptRef: manifest.CutoverBarrier.QuiescenceReceiptRef,
		DrainReceiptRefs:     manifest.CutoverBarrier.DrainReceiptRefs, Manifest: manifest,
	})
	if err != nil {
		return computerevent.SupervisionTransaction{}, err
	}
	transaction := computerevent.SupervisionTransaction{Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: commandID, TransactionClass: "projection_import", OwnerID: manifest.OwnerID, ComputerID: manifest.ComputerID, TrajectoryID: manifest.Snapshot.Trajectory.TrajectoryID, CommandID: commandID, Actor: computerevent.SupervisionActor{ActorID: "runtime", Role: "runtime", AuthorityRef: "authority:projection-import"}, Expected: computerevent.SupervisionExpected{}, Mutations: []computerevent.SupervisionMutation{{Kind: "projection_imported", Body: body}}}
	var errDigest error
	transaction.CommandDigest, errDigest = transaction.ComputeCommandDigest()
	return transaction, errDigest
}

func projectionImportDigest(value any) (string, error) {
	raw, err := computerevent.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(raw), nil
}
func projectionImportManifestDigest(manifest ProjectionImportV1) (string, error) {
	manifest.ProjectionDigest = ""
	return projectionImportDigest(manifest)
}

// BindProjectionImportManifest resolves reserved logical receipt placeholders
// and recomputes the manifest digest after their private content addresses exist.
func BindProjectionImportManifest(manifest ProjectionImportV1, replacements map[string]string) (ProjectionImportV1, map[string]string, error) {
	logicalProjectionDigest := manifest.ProjectionDigest
	quiescence, ok := replacements[manifest.CutoverBarrier.QuiescenceReceiptRef]
	if !ok {
		return ProjectionImportV1{}, nil, fmt.Errorf("projection import: quiescence receipt binding is missing")
	}
	manifest.CutoverBarrier.QuiescenceReceiptRef = quiescence
	for index, ref := range manifest.CutoverBarrier.DrainReceiptRefs {
		bound, ok := replacements[ref]
		if !ok {
			return ProjectionImportV1{}, nil, fmt.Errorf("projection import: drain receipt binding is missing")
		}
		manifest.CutoverBarrier.DrainReceiptRefs[index] = bound
	}
	digest, err := projectionImportManifestDigest(manifest)
	if err != nil {
		return ProjectionImportV1{}, nil, err
	}
	manifest.ProjectionDigest = digest
	return manifest, map[string]string{logicalProjectionDigest: digest}, nil
}
func validateProjectionImportManifest(manifest ProjectionImportV1) error {
	if manifest.Schema != ProjectionImportV1Schema || manifest.ReducerVersion != computerevent.SupervisionReducerV1 || manifest.OwnerID == "" || manifest.ComputerID == "" || manifest.SourceDoltCommit == "" || manifest.Snapshot.Trajectory.TrajectoryID == "" {
		return fmt.Errorf("projection import: incomplete manifest")
	}
	if len(manifest.Objects) == 0 || len(manifest.CutoverBarrier.DrainReceiptRefs) == 0 || manifest.CutoverBarrier.QuiescenceReceiptRef == "" {
		return fmt.Errorf("projection import: incomplete manifest admission evidence")
	}
	if len(manifest.CutoverBarrier.ActiveRunIDs) != 0 || len(manifest.CutoverBarrier.ActiveAttemptIDs) != 0 || len(manifest.CutoverBarrier.PendingDeliveryIDs) != 0 || len(manifest.CutoverBarrier.SlotClaimIDs) != 0 || len(manifest.CutoverBarrier.AgentMutationIDs) != 0 {
		return fmt.Errorf("projection import: manifest is not quiescent")
	}
	snapshot, err := validateProjectionImportSnapshot(manifest)
	if err != nil {
		return err
	}
	sourceDigest, err := projectionImportDigest(snapshot)
	if err != nil || sourceDigest != manifest.SourceProjectionDigest {
		return fmt.Errorf("projection import: source projection digest mismatch")
	}
	expectedObjects, expectedEdges, err := buildProjectionImportObjects(snapshot)
	if err != nil {
		return fmt.Errorf("projection import: rebuild canonical graph: %w", err)
	}
	if err := requireCanonicalProjectionImportValue("objects", manifest.Objects, expectedObjects); err != nil {
		return err
	}
	if err := requireCanonicalProjectionImportValue("edges", manifest.Edges, expectedEdges); err != nil {
		return err
	}
	digest, err := projectionImportManifestDigest(manifest)
	if err != nil || digest != manifest.ProjectionDigest {
		return fmt.Errorf("projection import: digest mismatch")
	}
	return nil
}

func sortProjectionImportSnapshot(snapshot *ProjectionImportSnapshot) {
	if snapshot.WorkItems == nil {
		snapshot.WorkItems = []types.WorkItemRecord{}
	}
	if snapshot.Agents == nil {
		snapshot.Agents = []types.AgentRecord{}
	}
	if snapshot.Updates == nil {
		snapshot.Updates = []types.CoagentSourcePacket{}
	}
	if snapshot.SourceEntities == nil {
		snapshot.SourceEntities = []TextureSourceEntityGraphRecord{}
	}
	if snapshot.SourceRefs == nil {
		snapshot.SourceRefs = []TextureSourceRefGraphRecord{}
	}
	sort.Slice(snapshot.WorkItems, func(i, j int) bool { return snapshot.WorkItems[i].WorkItemID < snapshot.WorkItems[j].WorkItemID })
	sort.Slice(snapshot.Agents, func(i, j int) bool { return snapshot.Agents[i].AgentID < snapshot.Agents[j].AgentID })
	sort.Slice(snapshot.Updates, func(i, j int) bool { return snapshot.Updates[i].UpdateID < snapshot.Updates[j].UpdateID })
	sort.Slice(snapshot.SourceEntities, func(i, j int) bool {
		return snapshot.SourceEntities[i].CanonicalID < snapshot.SourceEntities[j].CanonicalID
	})
	sort.Slice(snapshot.SourceRefs, func(i, j int) bool { return snapshot.SourceRefs[i].CanonicalID < snapshot.SourceRefs[j].CanonicalID })
}

func validateProjectionImportSnapshot(manifest ProjectionImportV1) (ProjectionImportSnapshot, error) {
	snapshot := manifest.Snapshot
	snapshot.WorkItems = append([]types.WorkItemRecord(nil), snapshot.WorkItems...)
	snapshot.Agents = append([]types.AgentRecord(nil), snapshot.Agents...)
	snapshot.Updates = append([]types.CoagentSourcePacket(nil), snapshot.Updates...)
	snapshot.SourceEntities = append([]TextureSourceEntityGraphRecord(nil), snapshot.SourceEntities...)
	snapshot.SourceRefs = append([]TextureSourceRefGraphRecord(nil), snapshot.SourceRefs...)
	sortProjectionImportSnapshot(&snapshot)
	ownerID, computerID := manifest.OwnerID, manifest.ComputerID
	trajectory := snapshot.Trajectory
	if trajectory.TrajectoryID == "" || trajectory.OwnerID != ownerID || trajectory.ComputerID != computerID {
		return ProjectionImportSnapshot{}, fmt.Errorf("projection import: trajectory scope mismatch")
	}
	if snapshot.IntentRevisionID == "" || snapshot.Document.DocID == "" || snapshot.HeadRevision.RevisionID == "" {
		return ProjectionImportSnapshot{}, fmt.Errorf("projection import: incomplete artifact linkage")
	}
	if snapshot.Document.OwnerID != ownerID || snapshot.Document.ComputerID != computerID || snapshot.Document.TrajectoryID != trajectory.TrajectoryID {
		return ProjectionImportSnapshot{}, fmt.Errorf("projection import: document scope mismatch")
	}
	if trajectory.SubjectRefs["doc_id"] != snapshot.Document.DocID {
		return ProjectionImportSnapshot{}, fmt.Errorf("projection import: trajectory document linkage mismatch")
	}
	if snapshot.Document.CurrentRevisionID != snapshot.HeadRevision.RevisionID || snapshot.IntentRevisionID != snapshot.HeadRevision.RevisionID {
		return ProjectionImportSnapshot{}, fmt.Errorf("projection import: revision linkage mismatch")
	}
	if snapshot.HeadRevision.DocID != snapshot.Document.DocID || snapshot.HeadRevision.OwnerID != ownerID || snapshot.HeadRevision.ComputerID != computerID || snapshot.HeadRevision.TrajectoryID != trajectory.TrajectoryID {
		return ProjectionImportSnapshot{}, fmt.Errorf("projection import: revision scope mismatch")
	}
	if trajectory.TerminalArtifactHeadRef != "" && trajectory.TerminalArtifactHeadRef != snapshot.HeadRevision.RevisionID {
		return ProjectionImportSnapshot{}, fmt.Errorf("projection import: terminal revision linkage mismatch")
	}

	ids := make(map[string]string, 3+len(snapshot.WorkItems)+len(snapshot.Agents)+len(snapshot.Updates)+len(snapshot.SourceEntities)+len(snapshot.SourceRefs))
	addID := func(kind, id string) error {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("projection import: %s canonical id is required", kind)
		}
		if previous, exists := ids[id]; exists {
			return fmt.Errorf("projection import: duplicate canonical id %q (%s and %s)", id, previous, kind)
		}
		ids[id] = kind
		return nil
	}
	for _, value := range []struct{ kind, id string }{
		{"trajectory", trajectory.TrajectoryID},
		{"document", snapshot.Document.DocID},
		{"revision", snapshot.HeadRevision.RevisionID},
	} {
		if err := addID(value.kind, value.id); err != nil {
			return ProjectionImportSnapshot{}, err
		}
	}
	workIDs := make(map[string]struct{}, len(snapshot.WorkItems))
	for _, work := range snapshot.WorkItems {
		if err := addID("assignment", work.WorkItemID); err != nil {
			return ProjectionImportSnapshot{}, err
		}
		if work.OwnerID != ownerID || work.ComputerID != computerID || work.TrajectoryID != trajectory.TrajectoryID {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: assignment %q scope mismatch", work.WorkItemID)
		}
		workIDs[work.WorkItemID] = struct{}{}
	}
	agentIDs := make(map[string]struct{}, len(snapshot.Agents))
	for _, agent := range snapshot.Agents {
		if err := addID("agent", agent.AgentID); err != nil {
			return ProjectionImportSnapshot{}, err
		}
		if agent.OwnerID != ownerID || agent.ComputerID != computerID {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: agent %q scope mismatch", agent.AgentID)
		}
		agentIDs[agent.AgentID] = struct{}{}
	}
	for _, work := range snapshot.WorkItems {
		if work.AssignedAgentID != "" {
			if _, exists := agentIDs[work.AssignedAgentID]; !exists {
				return ProjectionImportSnapshot{}, fmt.Errorf("projection import: assignment %q references unknown agent %q", work.WorkItemID, work.AssignedAgentID)
			}
		}
	}
	for _, update := range snapshot.Updates {
		if err := addID("update", update.UpdateID); err != nil {
			return ProjectionImportSnapshot{}, err
		}
		if update.OwnerID != ownerID || update.ComputerID != computerID || update.TrajectoryID != trajectory.TrajectoryID {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: update %q scope mismatch", update.UpdateID)
		}
		if _, exists := agentIDs[update.TargetAgentID]; update.TargetAgentID == "" || !exists {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: update %q references unknown target agent %q", update.UpdateID, update.TargetAgentID)
		}
		if update.WorkItemID != "" {
			if _, exists := workIDs[update.WorkItemID]; !exists {
				return ProjectionImportSnapshot{}, fmt.Errorf("projection import: update %q references unknown work item %q", update.UpdateID, update.WorkItemID)
			}
		}
	}
	entityIDs := make(map[string]TextureSourceEntityGraphRecord, len(snapshot.SourceEntities))
	for _, entity := range snapshot.SourceEntities {
		if err := addID("source_entity", entity.CanonicalID); err != nil {
			return ProjectionImportSnapshot{}, err
		}
		if entity.OwnerID != ownerID || (entity.ComputerID != "" && entity.ComputerID != computerID) {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: source entity %q scope mismatch", entity.CanonicalID)
		}
		if err := validateProjectionImportSourceEntity(entity); err != nil {
			return ProjectionImportSnapshot{}, err
		}
		entityIDs[entity.CanonicalID] = entity
	}
	entityReferences := make(map[string]int, len(entityIDs))
	for _, ref := range snapshot.SourceRefs {
		if err := addID("source_ref", ref.CanonicalID); err != nil {
			return ProjectionImportSnapshot{}, err
		}
		if ref.OwnerID != ownerID || (ref.ComputerID != "" && ref.ComputerID != computerID) || ref.DocID != snapshot.Document.DocID || ref.TextureRevisionID != snapshot.HeadRevision.RevisionID {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: source ref %q linkage mismatch", ref.CanonicalID)
		}
		entity, exists := entityIDs[ref.SourceEntityCanonicalID]
		if !exists || entity.VersionID != ref.SourceEntityVersionID {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: source ref %q references an unknown source entity version", ref.CanonicalID)
		}
		if entity.ComputerID != "" && ref.ComputerID != "" && entity.ComputerID != ref.ComputerID {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: source ref %q crosses computer scope", ref.CanonicalID)
		}
		if err := validateProjectionImportSourceRef(ref); err != nil {
			return ProjectionImportSnapshot{}, err
		}
		entityReferences[entity.CanonicalID]++
	}
	for canonicalID := range entityIDs {
		if entityReferences[canonicalID] == 0 {
			return ProjectionImportSnapshot{}, fmt.Errorf("projection import: source entity %q is not referenced by this projection", canonicalID)
		}
	}
	return snapshot, nil
}

func validateProjectionImportSourceEntity(entity TextureSourceEntityGraphRecord) error {
	versionID, contentHash, metadata, err := TextureSourceGraphVersionID(TextureSourceEntityObjectKind, entity.Body, entity.Metadata)
	if err != nil || versionID != entity.VersionID || contentHash != entity.ContentHash || !bytes.Equal(metadata, entity.Metadata) {
		return fmt.Errorf("projection import: source entity %q integrity mismatch", entity.CanonicalID)
	}
	if entity.ComputerID != "" {
		if err := validateLifecycleSourceEntityIdentity(entity); err != nil {
			return fmt.Errorf("projection import: source entity %q identity: %w", entity.CanonicalID, err)
		}
	}
	return nil
}

func validateProjectionImportSourceRef(ref TextureSourceRefGraphRecord) error {
	versionID, contentHash, metadata, err := TextureSourceGraphVersionID(TextureSourceRefObjectKind, sourceRefVersionBody(ref), ref.Metadata)
	if err != nil || versionID != ref.VersionID || contentHash != ref.ContentHash || !bytes.Equal(metadata, ref.Metadata) {
		return fmt.Errorf("projection import: source ref %q integrity mismatch", ref.CanonicalID)
	}
	if ref.ComputerID != "" {
		if err := validateLifecycleSourceRefIdentity(ref); err != nil {
			return fmt.Errorf("projection import: source ref %q identity: %w", ref.CanonicalID, err)
		}
	}
	return nil
}

func requireCanonicalProjectionImportValue(name string, actual, expected any) error {
	actualBytes, err := computerevent.CanonicalJSON(actual)
	if err != nil {
		return fmt.Errorf("projection import: canonical %s: %w", name, err)
	}
	expectedBytes, err := computerevent.CanonicalJSON(expected)
	if err != nil {
		return fmt.Errorf("projection import: rebuild canonical %s: %w", name, err)
	}
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("projection import: %s do not match typed snapshot", name)
	}
	return nil
}

func buildProjectionImportObjects(snapshot ProjectionImportSnapshot) ([]ProjectionImportObject, []ProjectionImportEdge, error) {
	objects := make([]ProjectionImportObject, 0, 2+len(snapshot.WorkItems)+len(snapshot.Agents)+len(snapshot.Updates)+len(snapshot.SourceEntities)+len(snapshot.SourceRefs))
	appendObject := func(kind, id string, body any) error {
		raw, err := computerevent.CanonicalJSON(body)
		if err != nil {
			return err
		}
		objects = append(objects, ProjectionImportObject{Kind: kind, CanonicalID: id, ContentHash: computerevent.DigestBytes(raw), Body: raw})
		return nil
	}
	if err := appendObject("trajectory", snapshot.Trajectory.TrajectoryID, snapshot.Trajectory); err != nil {
		return nil, nil, err
	}
	if err := appendObject("document", snapshot.Document.DocID, snapshot.Document); err != nil {
		return nil, nil, err
	}
	if err := appendObject("revision", snapshot.HeadRevision.RevisionID, snapshot.HeadRevision); err != nil {
		return nil, nil, err
	}
	for _, value := range snapshot.WorkItems {
		if err := appendObject("assignment", value.WorkItemID, value); err != nil {
			return nil, nil, err
		}
	}
	for _, value := range snapshot.Agents {
		if err := appendObject("agent", value.AgentID, value); err != nil {
			return nil, nil, err
		}
	}
	for _, value := range snapshot.Updates {
		if err := appendObject("update", value.UpdateID, value); err != nil {
			return nil, nil, err
		}
	}
	for _, value := range snapshot.SourceEntities {
		if err := appendObject("source_entity", value.CanonicalID, value); err != nil {
			return nil, nil, err
		}
	}
	for _, value := range snapshot.SourceRefs {
		if err := appendObject("source_ref", value.CanonicalID, value); err != nil {
			return nil, nil, err
		}
	}
	sort.Slice(objects, func(i, j int) bool {
		if objects[i].Kind != objects[j].Kind {
			return objects[i].Kind < objects[j].Kind
		}
		return objects[i].CanonicalID < objects[j].CanonicalID
	})
	edges := []ProjectionImportEdge{}
	add := func(kind, from, to string) error {
		hash, err := projectionImportDigest(struct{ Kind, From, To string }{kind, from, to})
		if err != nil {
			return err
		}
		edges = append(edges, ProjectionImportEdge{EdgeID: kind + ":" + from + ":" + to, FromID: from, ToID: to, Kind: kind, ContentHash: hash})
		return nil
	}
	if err := add("document_revision", snapshot.Document.DocID, snapshot.HeadRevision.RevisionID); err != nil {
		return nil, nil, err
	}
	if err := add("trajectory_artifact", snapshot.Trajectory.TrajectoryID, snapshot.Document.DocID); err != nil {
		return nil, nil, err
	}
	for _, work := range snapshot.WorkItems {
		if err := add("trajectory_assignment", snapshot.Trajectory.TrajectoryID, work.WorkItemID); err != nil {
			return nil, nil, err
		}
	}
	for _, ref := range snapshot.SourceRefs {
		if err := add("revision_source_ref", snapshot.HeadRevision.RevisionID, ref.CanonicalID); err != nil {
			return nil, nil, err
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].EdgeID < edges[j].EdgeID })
	return objects, edges, nil
}

func projectionImportFromMutationBody(body map[string]any) (ProjectionImportV1, error) {
	raw, ok := body["manifest"]
	if !ok {
		return ProjectionImportV1{}, fmt.Errorf("supervision projection: import manifest is missing")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return ProjectionImportV1{}, fmt.Errorf("supervision projection: encode import manifest: %w", err)
	}
	var manifest ProjectionImportV1
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return ProjectionImportV1{}, fmt.Errorf("supervision projection: decode import manifest: %w", err)
	}
	if err := validateProjectionImportManifest(manifest); err != nil {
		return ProjectionImportV1{}, err
	}
	return manifest, nil
}

func applyProjectionImportManifest(state *supervisionProjectionState, transaction computerevent.SupervisionTransaction, manifest ProjectionImportV1) error {
	snapshot := manifest.Snapshot
	if manifest.OwnerID != transaction.OwnerID || manifest.ComputerID != transaction.ComputerID ||
		snapshot.Trajectory.TrajectoryID != transaction.TrajectoryID {
		return fmt.Errorf("supervision projection: import scope mismatch")
	}
	if snapshot.Document.OwnerID != transaction.OwnerID || snapshot.Document.ComputerID != transaction.ComputerID ||
		snapshot.HeadRevision.OwnerID != transaction.OwnerID || snapshot.HeadRevision.ComputerID != transaction.ComputerID {
		return fmt.Errorf("supervision projection: import artifact scope mismatch")
	}
	state.TrajectoryKind = string(snapshot.Trajectory.Kind)
	state.SubjectRefs = cloneStringMap(snapshot.Trajectory.SubjectRefs)
	state.IntentRevisionID = snapshot.IntentRevisionID
	state.ArtifactID = snapshot.Document.DocID
	state.ArtifactHeadRevisionID = snapshot.Document.CurrentRevisionID
	if state.ArtifactHeadRevisionID != snapshot.HeadRevision.RevisionID || state.IntentRevisionID == "" {
		return fmt.Errorf("supervision projection: import semantic heads are incomplete")
	}
	state.Settled = snapshot.Trajectory.Status == types.TrajectorySettled
	state.Archived = snapshot.Document.ArchivedAt != nil
	state.CreatedAt = snapshot.Trajectory.CreatedAt
	if state.CreatedAt.IsZero() {
		state.CreatedAt = snapshot.Document.CreatedAt
	}
	for _, object := range manifest.Objects {
		if state.Entities["imported_"+object.Kind] == nil {
			state.Entities["imported_"+object.Kind] = map[string]json.RawMessage{}
		}
		state.Entities["imported_"+object.Kind][object.CanonicalID] = append(json.RawMessage(nil), object.Body...)
	}
	for _, work := range snapshot.WorkItems {
		setStatus(state, "assignment", work.WorkItemID, string(work.Status))
	}
	for _, update := range snapshot.Updates {
		setStatus(state, "update", update.UpdateID, string(update.Disposition))
	}
	return nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func supervisionImportedDerivedObjects(manifest ProjectionImportV1, sequence uint64) ([]objectgraph.Object, error) {
	snapshot := manifest.Snapshot
	objects := make([]objectgraph.Object, 0, 3+len(snapshot.WorkItems)+len(snapshot.Agents)+len(snapshot.Updates)+len(snapshot.SourceEntities)+len(snapshot.SourceRefs))
	add := func(kind objectgraph.ObjectKind, identity string, body any, created, updated time.Time, metadata map[string]any) error {
		object, err := lifecycleObject(kind, manifest.OwnerID, manifest.ComputerID, identity, body, metadata, created, updated)
		if err != nil {
			return err
		}
		objects = append(objects, object)
		return nil
	}
	if err := add(ogKindTrajectory, snapshot.Trajectory.TrajectoryID, snapshot.Trajectory, snapshot.Trajectory.CreatedAt, snapshot.Trajectory.UpdatedAt, lifecycleMetadata("trajectory_id", snapshot.Trajectory.TrajectoryID, manifest.ComputerID, snapshot.Trajectory.TrajectoryID, int64(sequence))); err != nil {
		return nil, err
	}
	if err := add(ogKindTexDoc, snapshot.Document.DocID, snapshot.Document, snapshot.Document.CreatedAt, snapshot.Document.UpdatedAt, lifecycleMetadata("doc_id", snapshot.Document.DocID, manifest.ComputerID, snapshot.Trajectory.TrajectoryID, int64(sequence))); err != nil {
		return nil, err
	}
	if err := add(ogKindTexRev, snapshot.HeadRevision.RevisionID, snapshot.HeadRevision, snapshot.HeadRevision.CreatedAt, snapshot.HeadRevision.CreatedAt, lifecycleMetadata("revision_id", snapshot.HeadRevision.RevisionID, manifest.ComputerID, snapshot.Trajectory.TrajectoryID, int64(sequence))); err != nil {
		return nil, err
	}
	for _, work := range snapshot.WorkItems {
		if err := add(ogKindWorkItem, work.WorkItemID, work, work.CreatedAt, work.UpdatedAt, lifecycleMetadata("work_item_id", work.WorkItemID, manifest.ComputerID, snapshot.Trajectory.TrajectoryID, int64(sequence))); err != nil {
			return nil, err
		}
	}
	for _, agent := range snapshot.Agents {
		if err := add(ogKindAgent, agent.AgentID, agent, agent.CreatedAt, agent.UpdatedAt, lifecycleMetadata("agent_id", agent.AgentID, manifest.ComputerID, snapshot.Trajectory.TrajectoryID, int64(sequence))); err != nil {
			return nil, err
		}
	}
	for _, update := range snapshot.Updates {
		if err := add(ogKindWorkerUpdate, update.UpdateID, update, update.CreatedAt, update.CreatedAt, lifecycleMetadata("update_id", update.UpdateID, manifest.ComputerID, snapshot.Trajectory.TrajectoryID, int64(sequence))); err != nil {
			return nil, err
		}
	}
	for _, entity := range snapshot.SourceEntities {
		if err := add(TextureSourceEntityObjectKind, entity.CanonicalID, entity, entity.CreatedAt, entity.CreatedAt, lifecycleMetadata("canonical_id", entity.CanonicalID, manifest.ComputerID, snapshot.Trajectory.TrajectoryID, int64(sequence))); err != nil {
			return nil, err
		}
	}
	for _, ref := range snapshot.SourceRefs {
		if err := add(TextureSourceRefObjectKind, ref.CanonicalID, ref, ref.CreatedAt, ref.CreatedAt, lifecycleMetadata("canonical_id", ref.CanonicalID, manifest.ComputerID, snapshot.Trajectory.TrajectoryID, int64(sequence))); err != nil {
			return nil, err
		}
	}
	return objects, nil
}
