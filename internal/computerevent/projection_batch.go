package computerevent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// ProjectorVersionV1 is the first complete-tape projector. It is distinct
	// from ReducerVersion (head algebra). Replay must bind this version so
	// restored rows do not depend on whichever binary happens to run.
	ProjectorVersionV1 = 1
	ProjectionBatchV1  = 1

	// ProjectorVersionV2 adds reducer-backed snapshots for runtime/control
	// tables that were historically written outside the event tape. V1 remains
	// readable so existing desktop/OG events stay replayable.
	ProjectorVersionV2 = 2
	ProjectionBatchV2  = 2

	ProjectionOpDesktopState               = "desktop_state_recorded"
	ProjectionOpObject                     = "object_recorded"
	ProjectionOpObjectEdge                 = "object_edge_recorded"
	ProjectionOpRunMemoryEntry             = "run_memory_entry_recorded"
	ProjectionOpSelfDevelopmentStartIntent = "self_development_start_intent_recorded"
	ProjectionOpSelfDevelopmentOperation   = "self_development_operation_recorded"
	ProjectionOpTextureAgentMutation       = "texture_agent_mutation_recorded"
	ProjectionOpTextureDocumentAlias       = "texture_document_alias_recorded"
	ProjectionOpTextureDocumentAliasDelete = "texture_document_alias_delete_recorded"
	ProjectionOpMediaProgress              = "media_progress_recorded"
	ProjectionOpMediaRecent                = "media_recent_recorded"
	ProjectionOpMediaRecentDelete          = "media_recent_delete_recorded"
	ProjectionOpUserPreference             = "user_preference_recorded"
	// ProjectionOpObjectRetire removes aged rows of named event kinds from the
	// working store; the rows stay on the sealed tape where they were recorded
	// (docs/design/object-graph-redesign-2026-10-30.md section 11.3).
	ProjectionOpObjectRetire = "object_retire_recorded"
	ProjectionBatchMediaType = "application/vnd.choir.projection-batch+json"
)

var (
	ErrProjectionBatchInvalid = errors.New("computer event projection batch: invalid")
	ErrProjectionPoison       = errors.New("computer event projection: poison event cannot wedge the computer")
	ErrSQLDuringResolve       = errors.New("computer event projection: SQL executor used during payload resolve")
	ErrProjectionPresence     = errors.New("computer event projection: session presence is not a tape payload")
)

var projectionOpKinds = map[string]struct{}{
	ProjectionOpDesktopState:               {},
	ProjectionOpObject:                     {},
	ProjectionOpObjectEdge:                 {},
	ProjectionOpRunMemoryEntry:             {},
	ProjectionOpSelfDevelopmentStartIntent: {},
	ProjectionOpSelfDevelopmentOperation:   {},
	ProjectionOpTextureAgentMutation:       {},
	ProjectionOpTextureDocumentAlias:       {},
	ProjectionOpTextureDocumentAliasDelete: {},
	ProjectionOpMediaProgress:              {},
	ProjectionOpMediaRecent:                {},
	ProjectionOpMediaRecentDelete:          {},
	ProjectionOpUserPreference:             {},
	ProjectionOpObjectRetire:               {},
}

func isProjectionV2OnlyKind(kind string) bool {
	switch kind {
	case ProjectionOpRunMemoryEntry, ProjectionOpSelfDevelopmentStartIntent, ProjectionOpSelfDevelopmentOperation, ProjectionOpTextureAgentMutation, ProjectionOpTextureDocumentAlias, ProjectionOpTextureDocumentAliasDelete,
		ProjectionOpMediaProgress, ProjectionOpMediaRecent, ProjectionOpMediaRecentDelete, ProjectionOpUserPreference, ProjectionOpObjectRetire:
		return true
	default:
		return false
	}
}

// ProjectionOp is one mutation inside an atomic batch. Intermediate heads
// between ops in the same batch are not restore targets; the whole batch is.
type ProjectionOp struct {
	Kind        string          `json:"kind"`
	CanonicalID string          `json:"canonical_id,omitempty"`
	Table       string          `json:"table,omitempty"`
	Body        json.RawMessage `json:"body,omitempty"`
}

// RunMemoryEntryProjection is the canonical row snapshot for a run memory
// entry. JSON strings mirror the SQL LONGTEXT columns exactly; the projector
// never reconstructs provider-facing content from an alternate representation.
type RunMemoryEntryProjection struct {
	EntryID          string `json:"entry_id"`
	RunID            string `json:"loop_id"`
	OwnerID          string `json:"owner_id"`
	AgentID          string `json:"agent_id"`
	ParentEntryID    string `json:"parent_entry_id"`
	Seq              int64  `json:"seq"`
	Kind             string `json:"kind"`
	Role             string `json:"role"`
	MessageJSON      string `json:"message_json"`
	Summary          string `json:"summary"`
	FirstKeptEntryID string `json:"first_kept_entry_id"`
	TokensBefore     int    `json:"tokens_before"`
	Reason           string `json:"reason"`
	Model            string `json:"model"`
	DetailsJSON      string `json:"details_json"`
	CreatedAt        string `json:"created_at"`
}

// SelfDevelopmentStartIntentProjection is the canonical row snapshot for a
// start-intent idempotency binding.
type SelfDevelopmentStartIntentProjection struct {
	ComputerID        string `json:"computer_id"`
	IdempotencyKey    string `json:"idempotency_key"`
	RequestCommitment string `json:"request_commitment"`
	CreatedAt         string `json:"created_at"`
}

// SelfDevelopmentOperationProjection is the canonical row snapshot for one
// self-development operation. ExpectedState is a live CAS precondition and is
// omitted from imported snapshots; it is not a persisted table column.
type SelfDevelopmentOperationProjection struct {
	OperationID            string  `json:"operation_id"`
	IdempotencyKey         string  `json:"idempotency_key"`
	RequestCommitment      string  `json:"request_commitment"`
	ComputerID             string  `json:"computer_id"`
	TrajectoryID           string  `json:"trajectory_id"`
	CapsuleID              string  `json:"capsule_id"`
	BaseHead               string  `json:"base_head"`
	PromptArtifactRef      string  `json:"prompt_artifact_ref"`
	BundleDigest           string  `json:"bundle_digest"`
	ReleaseDigest          string  `json:"release_digest"`
	CodeRef                string  `json:"code_ref"`
	ArtifactProgramRef     string  `json:"artifact_program_ref"`
	VerifierRefsJSON       string  `json:"verifier_refs_json"`
	DecisionActor          string  `json:"decision_actor"`
	DecisionEvent          string  `json:"decision_event"`
	DecisionReceipt        string  `json:"decision_receipt"`
	DesiredHead            string  `json:"desired_head"`
	EffectiveHead          string  `json:"effective_head"`
	MaterializationReceipt string  `json:"materialization_receipt"`
	CheckpointRef          string  `json:"checkpoint_ref"`
	RouteCertificate       string  `json:"route_certificate"`
	RouteGeneration        *uint64 `json:"route_generation,omitempty"`
	RouteReceipt           string  `json:"route_receipt"`
	ModeReceipt            string  `json:"mode_receipt"`
	LifecycleReceipt       string  `json:"lifecycle_receipt"`
	State                  string  `json:"state"`
	TerminalError          string  `json:"terminal_error"`
	// LastIntentError is the most recent non-terminal intent refusal. It is
	// omitted while empty, so an event written before the field existed and
	// an event with no stamp are the same bytes and both project an empty
	// column (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md).
	LastIntentError string `json:"last_intent_error,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	ExpectedState   string `json:"expected_state,omitempty"`
}

// TextureAgentMutationProjection is the canonical row snapshot for Texture's
// lifecycle/idempotency row. Expected* fields are live transition guards and
// are not table columns.
type TextureAgentMutationProjection struct {
	DocID               string   `json:"doc_id"`
	RunID               string   `json:"loop_id"`
	OwnerID             string   `json:"owner_id"`
	ComputerID          string   `json:"computer_id"`
	State               string   `json:"state"`
	ScheduledMessageSeq int64    `json:"scheduled_message_seq"`
	RevisionID          string   `json:"revision_id"`
	CreatedAt           string   `json:"created_at"`
	CompletedAt         *string  `json:"completed_at,omitempty"`
	ExpectedStates      []string `json:"expected_states,omitempty"`
	RequireRevision     *bool    `json:"require_revision,omitempty"`
	CreateOnly          bool     `json:"create_only,omitempty"`
}

// ProjectionBatch is the durable unit Project applies inside Finalize's SQL
// transaction. Payload bytes are already resolved; this value is SQL-only.
type ProjectionBatch struct {
	Version          int            `json:"version"`
	ProjectorVersion int            `json:"projector_version"`
	ComputerID       string         `json:"computer_id"`
	EventID          string         `json:"event_id"`
	EventDigest      string         `json:"event_digest"`
	Ops              []ProjectionOp `json:"ops"`
}

// ObjectRetireKindEvent is the only object kind a retirement may name for
// now: a retirement of any other class needs its own design (it can drop
// state a reader needs, where an aged event line is a trace).
const ObjectRetireKindEvent = "choir.event"

// Phase filters of an ObjectRetireProjection, on the event payload's phase
// (the store keeps the phase in the payload, not in EventRecord.Phase).
const (
	// ObjectRetirePhaseAny retires a matching event whatever its phase.
	ObjectRetirePhaseAny = ""
	// ObjectRetirePhaseNone retires only an event with no phase.
	ObjectRetirePhaseNone = "none"
	// ObjectRetirePhaseSome retires only an event with a phase.
	ObjectRetirePhaseSome = "some"
)

// ObjectRetireMaxSlice is the longest time slice one retire op may name. The
// slice bounds the rows an op reads and removes (the apply holds the event
// lock), and the op is as deterministic as the slice is exact.
const ObjectRetireMaxSlice = 24 * time.Hour

// ObjectRetireProjection is the body of a ProjectionOpObjectRetire op: remove
// every row of ObjectKind, from the working store, whose event kind is in
// EventKinds, whose updated_at is in the half-open slice
// [UpdatedFrom, UpdatedBefore), and whose payload phase matches Phase. The
// slice is in the body, never the wall clock at apply, so live apply and
// replay remove the same rows at the same tape position.
type ObjectRetireProjection struct {
	ObjectKind    string   `json:"object_kind"`
	EventKinds    []string `json:"event_kinds"`
	UpdatedFrom   string   `json:"updated_from"`
	UpdatedBefore string   `json:"updated_before"`
	Phase         string   `json:"phase,omitempty"`
	Reason        string   `json:"reason,omitempty"`
}

// DecodeObjectRetire parses and validates a retire body.
func DecodeObjectRetire(body json.RawMessage) (ObjectRetireProjection, error) {
	var p ObjectRetireProjection
	if err := json.Unmarshal(body, &p); err != nil {
		return ObjectRetireProjection{}, fmt.Errorf("decode object retire: %w", err)
	}
	if err := p.Validate(); err != nil {
		return ObjectRetireProjection{}, err
	}
	return p, nil
}

// Validate refuses an empty kind list, a missing, zero or unparseable bound,
// a slice that is empty, reversed or longer than ObjectRetireMaxSlice, an
// unknown phase filter and any object kind but choir.event.
func (p ObjectRetireProjection) Validate() error {
	if p.ObjectKind != ObjectRetireKindEvent {
		return fmt.Errorf("object retire: object_kind %q is not %q", p.ObjectKind, ObjectRetireKindEvent)
	}
	if len(p.EventKinds) == 0 {
		return fmt.Errorf("object retire: event_kinds is empty")
	}
	for _, k := range p.EventKinds {
		if strings.TrimSpace(k) == "" || k != strings.TrimSpace(k) {
			return fmt.Errorf("object retire: event kind %q is blank or padded", k)
		}
	}
	if _, _, err := p.Slice(); err != nil {
		return err
	}
	switch p.Phase {
	case ObjectRetirePhaseAny, ObjectRetirePhaseNone, ObjectRetirePhaseSome:
	default:
		return fmt.Errorf("object retire: unknown phase filter %q", p.Phase)
	}
	return nil
}

// Slice is the half-open [from, before) slice as UTC times; neither is zero,
// from is before before, and the slice is at most ObjectRetireMaxSlice.
func (p ObjectRetireProjection) Slice() (from, before time.Time, err error) {
	parse := func(name, raw string) (time.Time, error) {
		t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
		if err != nil {
			return time.Time{}, fmt.Errorf("object retire: %s: %w", name, err)
		}
		if t.IsZero() || t.Year() < 2000 {
			return time.Time{}, fmt.Errorf("object retire: %s is the zero time", name)
		}
		return t.UTC(), nil
	}
	if from, err = parse("updated_from", p.UpdatedFrom); err != nil {
		return time.Time{}, time.Time{}, err
	}
	if before, err = parse("updated_before", p.UpdatedBefore); err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !from.Before(before) {
		return time.Time{}, time.Time{}, fmt.Errorf("object retire: updated_from is not before updated_before")
	}
	if before.Sub(from) > ObjectRetireMaxSlice {
		return time.Time{}, time.Time{}, fmt.Errorf("object retire: the slice is longer than %s", ObjectRetireMaxSlice)
	}
	return from, before, nil
}

func (b ProjectionBatch) Validate() error {
	validVersion := (b.Version == ProjectionBatchV1 && b.ProjectorVersion == ProjectorVersionV1) ||
		(b.Version == ProjectionBatchV2 && b.ProjectorVersion == ProjectorVersionV2)
	if !validVersion {
		return fmt.Errorf("%w: unsupported version", ErrProjectionBatchInvalid)
	}
	if strings.TrimSpace(b.ComputerID) == "" || strings.TrimSpace(b.EventID) == "" {
		return fmt.Errorf("%w: computer/event identity required", ErrProjectionBatchInvalid)
	}
	if digest := strings.TrimSpace(b.EventDigest); digest != "" && !IsSHA256(digest) {
		return fmt.Errorf("%w: event digest", ErrProjectionBatchInvalid)
	}
	if len(b.Ops) == 0 {
		return fmt.Errorf("%w: empty batch", ErrProjectionBatchInvalid)
	}
	for i, op := range b.Ops {
		kind := strings.TrimSpace(op.Kind)
		if kind == "" {
			return fmt.Errorf("%w: op %d missing kind", ErrProjectionBatchInvalid, i)
		}
		if _, ok := projectionOpKinds[kind]; !ok {
			return fmt.Errorf("%w: op %d unknown kind %q", ErrProjectionBatchInvalid, i, kind)
		}
		if b.Version == ProjectionBatchV1 && isProjectionV2OnlyKind(kind) {
			return fmt.Errorf("%w: op %d kind %q requires projection batch v2", ErrProjectionBatchInvalid, i, kind)
		}
		table := strings.TrimSpace(op.Table)
		if table == "desktop_sessions" || table == "desktop_session_presence" {
			return fmt.Errorf("%w: %s", ErrProjectionPresence, table)
		}
		if err := rejectProjectionPresence(op.Body); err != nil {
			return err
		}
		if kind == ProjectionOpObjectRetire {
			if _, err := DecodeObjectRetire(op.Body); err != nil {
				return fmt.Errorf("%w: op %d: %v", ErrProjectionBatchInvalid, i, err)
			}
		}
	}
	return nil
}

// DecodeProjectionBatch parses already-resolved plaintext. Callers must
// ResolvePayloads before this and before BeginTx.
func DecodeProjectionBatch(plaintext []byte) (ProjectionBatch, error) {
	var batch ProjectionBatch
	if err := json.Unmarshal(plaintext, &batch); err != nil {
		return ProjectionBatch{}, fmt.Errorf("%w: decode: %v", ErrProjectionBatchInvalid, err)
	}
	if err := batch.Validate(); err != nil {
		return ProjectionBatch{}, err
	}
	return batch, nil
}

func rejectProjectionPresence(body json.RawMessage) error {
	if len(body) == 0 {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"last_input_at", "driver_until", "visibility_state", "is_driver"} {
		if _, ok := obj[key]; ok {
			return fmt.Errorf("%w: field %s", ErrProjectionPresence, key)
		}
	}
	return nil
}

// SQLExecutor is the transaction-scoped writer Project uses. ResolvePayloads
// must never receive one.
type SQLExecutor interface {
	Exec(query string, args ...any) error
}

// GuardNoSQL is a resolver-phase executor that fails closed if SQL is attempted.
type GuardNoSQL struct{}

func (GuardNoSQL) Exec(string, ...any) error { return ErrSQLDuringResolve }

// ClassifyProjectionFailure distinguishes decrypt/resolve failures (must happen
// before CAS) from post-CAS Project failures that would otherwise wedge the
// computer via ErrNeedsProjectionRepair retrying the same crash.
func ClassifyProjectionFailure(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPayloadResolverRequired) || errors.Is(err, ErrPayloadDigestMismatch) ||
		errors.Is(err, ErrPayloadPrivacyMismatch) || errors.Is(err, ErrPayloadSQLBeforeResolve) ||
		errors.Is(err, ErrSQLDuringResolve) {
		return fmt.Errorf("%w: %w", ErrPayloadSQLBeforeResolve, err)
	}
	return fmt.Errorf("%w: %w", ErrProjectionPoison, err)
}

// TextureDocumentAliasProjection is the canonical row snapshot for a Texture
// document file-path alias.
type TextureDocumentAliasProjection struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	SourcePath string `json:"source_path"`
	DocID      string `json:"doc_id"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// MediaProgressProjection is the canonical row snapshot for one media_progress
// row. Every field mirrors its SQL column verbatim (UpdatedAt is the DATETIME
// text, MediaIdentityHash the stored key), so the projector never derives a
// value and a seeded legacy row replays byte-for-byte
// (docs/problems/stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24.md).
type MediaProgressProjection struct {
	OwnerID           string  `json:"owner_id"`
	MediaKind         string  `json:"media_kind"`
	MediaIdentityHash string  `json:"media_identity_hash"`
	MediaIdentity     string  `json:"media_identity"`
	PositionSeconds   float64 `json:"position_seconds"`
	DurationSeconds   float64 `json:"duration_seconds"`
	PlaybackRate      float64 `json:"playback_rate"`
	UpdatedByDevice   string  `json:"updated_by_device"`
	UpdatedAt         string  `json:"updated_at"`
}

// MediaRecentProjection is the canonical row snapshot for one media_recents
// row, columns verbatim.
type MediaRecentProjection struct {
	OwnerID           string `json:"owner_id"`
	MediaKind         string `json:"media_kind"`
	MediaIdentityHash string `json:"media_identity_hash"`
	MediaIdentity     string `json:"media_identity"`
	Title             string `json:"title"`
	FileName          string `json:"file_name"`
	FilePath          string `json:"file_path"`
	SourceURL         string `json:"source_url"`
	MediaType         string `json:"media_type"`
	ContentID         string `json:"content_id"`
	OpenedAt          string `json:"opened_at"`
}

// MediaRecentDeleteProjection names one media_recents row by its primary
// key. The prune that keeps the newest recents per (owner, media_kind) is
// this op, so a replay removes exactly the rows the live store removed.
type MediaRecentDeleteProjection struct {
	OwnerID           string `json:"owner_id"`
	MediaKind         string `json:"media_kind"`
	MediaIdentityHash string `json:"media_identity_hash"`
}

// UserPreferenceProjection is the canonical row snapshot for one
// user_preferences row. ValueJSON is the stored LONGTEXT, not a re-encoding.
type UserPreferenceProjection struct {
	OwnerID       string `json:"owner_id"`
	PreferenceKey string `json:"preference_key"`
	ValueJSON     string `json:"value_json"`
	UpdatedAt     string `json:"updated_at"`
}
