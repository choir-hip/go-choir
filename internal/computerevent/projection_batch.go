package computerevent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	ProjectionBatchMediaType               = "application/vnd.choir.projection-batch+json"
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
}

func isProjectionV2OnlyKind(kind string) bool {
	switch kind {
	case ProjectionOpRunMemoryEntry, ProjectionOpSelfDevelopmentStartIntent, ProjectionOpSelfDevelopmentOperation, ProjectionOpTextureAgentMutation, ProjectionOpTextureDocumentAlias, ProjectionOpTextureDocumentAliasDelete:
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
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
	ExpectedState          string  `json:"expected_state,omitempty"`
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
