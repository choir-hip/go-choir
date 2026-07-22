// Package store provides durable runtime storage for the go-choir sandbox runtime.
//
// The store persists run records, agent records, channel messages, and event
// records using the same embedded Dolt workspace that owns Texture state, enabling
// stable run IDs, durable agent/channel identity, and restart-safe recovery
// (VAL-RUNTIME-003, VAL-RUNTIME-010).
//
// Design decisions:
//   - One embedded Dolt workspace per user computer owns both runtime/control
//     state and Texture/app state.
//   - Retired SQLite runtime files are inert evidence; serving startup never
//     reads or imports them.
//   - Event sequence numbers are per-task, enabling incremental cursors for
//     Trace projections and internal workflow verification.
//   - Runtime writes are serialized through the main embedded connection while
//     a shared-engine read handle keeps status reads observable during writes.
package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("record not found")

// ErrStaleDocumentHead is returned when a caller tries to create a revision
// against an older parent while the document head has already moved on.
var ErrStaleDocumentHead = errors.New("stale document head")

// ErrConcurrentStateChange is returned when an optimistic state transition
// loses a compare-and-set race against a newer stored record.
var ErrConcurrentStateChange = errors.New("concurrent state change")

// ErrLifecycleAuthorityRequired is returned when a legacy writer attempts to
// mutate state owned by the durable lifecycle reducer.
var ErrLifecycleAuthorityRequired = errors.New("durable lifecycle authority required")

// ErrInvalidTextureRevision is returned when a Texture revision write fails
// structured body/source validation before persistence.
var ErrInvalidTextureRevision = errors.New("invalid texture revision")

func sanitizeStoreText(value string) string {
	return strings.ToValidUTF8(value, "\uFFFD")
}

// Store wraps the embedded Dolt connection and provides persistence for run
// records, agent records, channel messages, event records, and Texture state.
type doltConnector interface {
	driver.Connector
	Close() error
}

type Store struct {
	db               *sql.DB
	readDB           *sql.DB
	path             string
	textureDB        *sql.DB
	texturePath      string
	doltConnector    doltConnector
	jsonPatchMu      sync.Mutex
	trajectoryMu     sync.Mutex
	textureRevMu     sync.Mutex
	doltCommitMu     sync.Mutex
	doltHistoryDirty bool
	workerUpdateMu   sync.Mutex
	channelMsgMu     sync.Mutex
	eventMu          sync.Mutex
	og               *objectgraph.Service
	ogStore          *objectgraph.DoltStore
	ogReadStore      *objectgraph.DoltStore
}

// DB returns the primary embedded Dolt *sql.DB connection used by this store.
// It is exposed so additive observability layers (e.g. the trace store from
// internal/trace) can wrap the same workspace without opening a second
// connection. The caller must not close the returned handle; the Store retains
// ownership and closes it on Store.Close.
func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// commitDoltCheckpoint makes the current VM-local working set addressable by
// Dolt history and AS OF queries. Dolt commits are database-wide, so callers
// must describe the boundary as a VM-state checkpoint rather than claiming the
// commit contains only one logical record.
func (s *Store) commitDoltCheckpoint(ctx context.Context, message string) error {
	if s == nil || s.textureHandle() == nil {
		return fmt.Errorf("runtime store: nil database")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "vm state checkpoint"
	}

	s.doltCommitMu.Lock()
	defer s.doltCommitMu.Unlock()
	if !s.doltHistoryDirty {
		return nil
	}

	if _, err := s.textureHandle().ExecContext(ctx, "CALL DOLT_COMMIT('-Am', ?)", message); err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			s.doltHistoryDirty = false
			return nil
		}
		return fmt.Errorf("runtime store: dolt checkpoint: %w", err)
	}
	s.doltHistoryDirty = false
	return nil
}

func (s *Store) markDoltHistoryDirty() {
	if s == nil {
		return
	}
	s.doltCommitMu.Lock()
	s.doltHistoryDirty = true
	s.doltCommitMu.Unlock()
}

// schemaDDL creates the runtime tables if they do not already exist.
const schemaDDL = `
CREATE TABLE IF NOT EXISTS agents (
	agent_id    VARCHAR(255) PRIMARY KEY,
	owner_id    VARCHAR(255) NOT NULL,
	sandbox_id  VARCHAR(255) NOT NULL,
	profile     VARCHAR(255) NOT NULL DEFAULT '',
	role        VARCHAR(255) NOT NULL DEFAULT '',
	channel_id  VARCHAR(255) NOT NULL DEFAULT '',
	created_at  DATETIME NOT NULL,
	updated_at  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
	loop_id     VARCHAR(255) PRIMARY KEY,
	agent_id    VARCHAR(255) NOT NULL DEFAULT '',
	channel_id  VARCHAR(255) NOT NULL DEFAULT '',
	requested_by_run_id VARCHAR(255) NOT NULL DEFAULT '',
	trajectory_id VARCHAR(255) NOT NULL DEFAULT '',
	agent_profile VARCHAR(255) NOT NULL DEFAULT '',
	agent_role VARCHAR(255) NOT NULL DEFAULT '',
	owner_id    VARCHAR(255) NOT NULL,
	sandbox_id  VARCHAR(255) NOT NULL,
	state       VARCHAR(64) NOT NULL,
	prompt      LONGTEXT NOT NULL DEFAULT '',
	result      LONGTEXT NOT NULL DEFAULT '',
	error       LONGTEXT NOT NULL DEFAULT '',
	created_at  DATETIME NOT NULL,
	updated_at  DATETIME NOT NULL,
	finished_at DATETIME,
	metadata_json LONGTEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS events (
	event_id   VARCHAR(255) NOT NULL,
	loop_id    VARCHAR(255) NOT NULL DEFAULT '',
	agent_id   VARCHAR(255) NOT NULL DEFAULT '',
	channel_id VARCHAR(255) NOT NULL DEFAULT '',
	owner_id   VARCHAR(255) NOT NULL DEFAULT '',
	trajectory_id VARCHAR(255) NOT NULL DEFAULT '',
	seq        BIGINT NOT NULL,
	stream_seq BIGINT NOT NULL DEFAULT 0,
	ts         DATETIME NOT NULL,
	kind       VARCHAR(255) NOT NULL,
	phase      VARCHAR(255) NOT NULL DEFAULT '',
	payload_json LONGTEXT NOT NULL DEFAULT '{}',
	PRIMARY KEY (event_id)
);

CREATE TABLE IF NOT EXISTS channel_messages (
	channel_id      VARCHAR(255) NOT NULL,
	seq             BIGINT NOT NULL,
	owner_id        VARCHAR(255) NOT NULL DEFAULT '',
	from_agent_id   VARCHAR(255) NOT NULL DEFAULT '',
	from_loop_id    VARCHAR(255) NOT NULL DEFAULT '',
	to_agent_id     VARCHAR(255) NOT NULL DEFAULT '',
	to_loop_id      VARCHAR(255) NOT NULL DEFAULT '',
	trajectory_id   VARCHAR(255) NOT NULL DEFAULT '',
	from_name       VARCHAR(255) NOT NULL DEFAULT '',
	role            VARCHAR(64) NOT NULL DEFAULT '',
	content         LONGTEXT NOT NULL,
	created_at      DATETIME NOT NULL,
	PRIMARY KEY (channel_id, seq)
);

CREATE TABLE IF NOT EXISTS inbox_deliveries (
	delivery_id          VARCHAR(255) PRIMARY KEY,
	owner_id             VARCHAR(255) NOT NULL DEFAULT '',
	to_agent_id          VARCHAR(255) NOT NULL DEFAULT '',
	to_loop_id           VARCHAR(255) NOT NULL DEFAULT '',
	from_agent_id        VARCHAR(255) NOT NULL DEFAULT '',
	from_loop_id         VARCHAR(255) NOT NULL DEFAULT '',
	channel_id           VARCHAR(255) NOT NULL DEFAULT '',
	role                 VARCHAR(64) NOT NULL DEFAULT '',
	content              LONGTEXT NOT NULL,
	trajectory_id        VARCHAR(255) NOT NULL DEFAULT '',
	created_at           DATETIME NOT NULL,
	delivered_to_loop_id VARCHAR(255) NOT NULL DEFAULT '',
	delivered_at         DATETIME
);

CREATE TABLE IF NOT EXISTS run_memory_entries (
	entry_id             VARCHAR(255) PRIMARY KEY,
	loop_id              VARCHAR(255) NOT NULL,
	owner_id             VARCHAR(255) NOT NULL DEFAULT '',
	agent_id             VARCHAR(255) NOT NULL DEFAULT '',
	parent_entry_id      VARCHAR(255) NOT NULL DEFAULT '',
	seq                  BIGINT NOT NULL,
	kind                 VARCHAR(64) NOT NULL,
	role                 VARCHAR(64) NOT NULL DEFAULT '',
	message_json         LONGTEXT NOT NULL DEFAULT '',
	summary              LONGTEXT NOT NULL DEFAULT '',
	first_kept_entry_id  VARCHAR(255) NOT NULL DEFAULT '',
	tokens_before        BIGINT NOT NULL DEFAULT 0,
	reason               LONGTEXT NOT NULL DEFAULT '',
	model                VARCHAR(255) NOT NULL DEFAULT '',
	details_json         LONGTEXT NOT NULL DEFAULT '{}',
	created_at           DATETIME NOT NULL,
	UNIQUE(loop_id, seq)
);


CREATE TABLE IF NOT EXISTS run_acceptances (
	acceptance_id        VARCHAR(255) PRIMARY KEY,
	target_mission_id    VARCHAR(255) NOT NULL DEFAULT '',
	source_prompt_or_objective LONGTEXT NOT NULL DEFAULT '',
	owner_id             VARCHAR(255) NOT NULL DEFAULT '',
	desktop_id           VARCHAR(255) NOT NULL DEFAULT '',
	trajectory_id        VARCHAR(255) NOT NULL DEFAULT '',
	loop_id              VARCHAR(255) NOT NULL DEFAULT '',
	authority_profile    VARCHAR(255) NOT NULL DEFAULT '',
	base_sha             VARCHAR(128) NOT NULL DEFAULT '',
	deployment_commit    VARCHAR(128) NOT NULL DEFAULT '',
	ci_run_id            VARCHAR(255) NOT NULL DEFAULT '',
	deploy_run_id        VARCHAR(255) NOT NULL DEFAULT '',
	staging_url          LONGTEXT NOT NULL DEFAULT '',
	health_commit        VARCHAR(128) NOT NULL DEFAULT '',
	acceptance_level     VARCHAR(64) NOT NULL DEFAULT '',
	vm_mode              VARCHAR(64) NOT NULL DEFAULT '',
	gateway_provider_evidence LONGTEXT NOT NULL DEFAULT '',
	state                VARCHAR(64) NOT NULL DEFAULT '',
	checkpoints_json     LONGTEXT NOT NULL DEFAULT '[]',
	invariant_checks_json LONGTEXT NOT NULL DEFAULT '[]',
	verifier_contracts_json LONGTEXT NOT NULL DEFAULT '[]',
	evidence_refs_json   LONGTEXT NOT NULL DEFAULT '[]',
	rollback_refs_json   LONGTEXT NOT NULL DEFAULT '[]',
	failure_residual_risks_json LONGTEXT NOT NULL DEFAULT '[]',
	created_at           DATETIME NOT NULL,
	updated_at           DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS run_continuations (
	continuation_id    VARCHAR(255) PRIMARY KEY,
	owner_id           VARCHAR(255) NOT NULL DEFAULT '',
	source_loop_id     VARCHAR(255) NOT NULL DEFAULT '',
	next_loop_id       VARCHAR(255) NOT NULL DEFAULT '',
	objective          LONGTEXT NOT NULL,
	reason             LONGTEXT NOT NULL DEFAULT '',
	authority_profile  VARCHAR(255) NOT NULL DEFAULT '',
	lease_seconds      BIGINT NOT NULL DEFAULT 0,
	status             VARCHAR(64) NOT NULL,
	details_json       LONGTEXT NOT NULL DEFAULT '{}',
	created_at         DATETIME NOT NULL,
	updated_at         DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS trajectories (
	trajectory_id        VARCHAR(255) PRIMARY KEY,
	owner_id             VARCHAR(255) NOT NULL DEFAULT '',
	kind                 VARCHAR(64) NOT NULL DEFAULT '',
	subject_refs_json    LONGTEXT NOT NULL DEFAULT '{}',
	status               VARCHAR(64) NOT NULL DEFAULT 'live',
	settlement_rule_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at           DATETIME NOT NULL,
	updated_at           DATETIME NOT NULL,
	settled_at           DATETIME
);

CREATE TABLE IF NOT EXISTS work_items (
	work_item_id          VARCHAR(255) PRIMARY KEY,
	trajectory_id         VARCHAR(255) NOT NULL DEFAULT '',
	owner_id              VARCHAR(255) NOT NULL DEFAULT '',
	objective             LONGTEXT NOT NULL,
	reason                LONGTEXT NOT NULL DEFAULT '',
	authority_profile     VARCHAR(255) NOT NULL DEFAULT '',
	step_budget           BIGINT NOT NULL DEFAULT 0,
	token_budget          BIGINT NOT NULL DEFAULT 0,
	objective_fingerprint VARCHAR(255) NOT NULL DEFAULT '',
	status                VARCHAR(64) NOT NULL DEFAULT 'open',
	assigned_agent_id     VARCHAR(255) NOT NULL DEFAULT '',
	created_by_loop_id    VARCHAR(255) NOT NULL DEFAULT '',
	details_json          LONGTEXT NOT NULL DEFAULT '{}',
	created_at            DATETIME NOT NULL,
	updated_at            DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS browser_sessions (
	session_id       VARCHAR(255) PRIMARY KEY,
	owner_id         VARCHAR(255) NOT NULL DEFAULT '',
	provider         VARCHAR(255) NOT NULL DEFAULT '',
	mode             VARCHAR(64) NOT NULL DEFAULT '',
	execution_scope  VARCHAR(64) NOT NULL DEFAULT '',
	backend_session_id VARCHAR(255) NOT NULL DEFAULT '',
	world_kind      VARCHAR(64) NOT NULL DEFAULT '',
	vm_id           VARCHAR(255) NOT NULL DEFAULT '',
	snapshot_id     VARCHAR(255) NOT NULL DEFAULT '',
	source_loop_id  VARCHAR(255) NOT NULL DEFAULT '',
	candidate_trace_id VARCHAR(255) NOT NULL DEFAULT '',
	state            VARCHAR(64) NOT NULL DEFAULT '',
	current_url      LONGTEXT NOT NULL DEFAULT '',
	title            LONGTEXT NOT NULL DEFAULT '',
	text_snapshot    LONGTEXT NOT NULL DEFAULT '',
	html_snapshot    LONGTEXT NOT NULL DEFAULT '',
	links_json       LONGTEXT NOT NULL DEFAULT '[]',
	screenshot_png_base64 LONGTEXT NOT NULL DEFAULT '',
	error            LONGTEXT NOT NULL DEFAULT '',
	created_at       DATETIME NOT NULL,
	updated_at       DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS worker_updates (
	owner_id          VARCHAR(255) NOT NULL DEFAULT '',
	update_id         VARCHAR(255) NOT NULL DEFAULT '',
	agent_id          VARCHAR(255) NOT NULL DEFAULT '',
	target_agent_id   VARCHAR(255) NOT NULL DEFAULT '',
	channel_id        VARCHAR(255) NOT NULL DEFAULT '',
	message_seq       BIGINT NOT NULL DEFAULT 0,
	trajectory_id     VARCHAR(255) NOT NULL DEFAULT '',
	role              VARCHAR(64) NOT NULL DEFAULT '',
	kind              VARCHAR(64) NOT NULL DEFAULT '',
	summary           LONGTEXT NOT NULL DEFAULT '',
	packet_json       LONGTEXT NOT NULL DEFAULT '{}',
	content           LONGTEXT NOT NULL DEFAULT '',
	created_at        DATETIME NOT NULL,
	delivered_to_loop_id VARCHAR(255) NOT NULL DEFAULT '',
	delivered_at      DATETIME,
	PRIMARY KEY (owner_id, update_id)
);

CREATE TABLE IF NOT EXISTS coagent_mailboxes (
	owner_id              VARCHAR(255) NOT NULL DEFAULT '',
	agent_id              VARCHAR(255) NOT NULL DEFAULT '',
	channel_id            VARCHAR(255) NOT NULL DEFAULT '',
	processed_message_seq BIGINT NOT NULL DEFAULT 0,
	updated_at            DATETIME NOT NULL,
	PRIMARY KEY (owner_id, agent_id)
);

CREATE TABLE IF NOT EXISTS co_super_slots (
	owner_id       VARCHAR(255) NOT NULL DEFAULT '',
	trajectory_id  VARCHAR(255) NOT NULL DEFAULT '',
	slot           VARCHAR(64) NOT NULL DEFAULT '',
	run_id         VARCHAR(255) NOT NULL DEFAULT '',
	agent_id       VARCHAR(255) NOT NULL DEFAULT '',
	requested_by_run_id VARCHAR(255) NOT NULL DEFAULT '',
	claimed_at     DATETIME NOT NULL,
	updated_at     DATETIME NOT NULL,
	PRIMARY KEY (owner_id, trajectory_id, slot)
);

CREATE TABLE IF NOT EXISTS media_progress (
	owner_id          VARCHAR(255) NOT NULL DEFAULT '',
	media_kind        VARCHAR(64) NOT NULL DEFAULT '',
	media_identity_hash VARCHAR(64) NOT NULL DEFAULT '',
	media_identity    LONGTEXT NOT NULL DEFAULT '',
	position_seconds  DOUBLE NOT NULL DEFAULT 0,
	duration_seconds  DOUBLE NOT NULL DEFAULT 0,
	playback_rate     DOUBLE NOT NULL DEFAULT 1,
	updated_by_device VARCHAR(255) NOT NULL DEFAULT '',
	updated_at        DATETIME NOT NULL,
	PRIMARY KEY (owner_id, media_kind, media_identity_hash)
);

CREATE TABLE IF NOT EXISTS media_recents (
	owner_id        VARCHAR(255) NOT NULL DEFAULT '',
	media_kind      VARCHAR(64) NOT NULL DEFAULT '',
	media_identity_hash VARCHAR(64) NOT NULL DEFAULT '',
	media_identity  LONGTEXT NOT NULL DEFAULT '',
	title           LONGTEXT NOT NULL DEFAULT '',
	file_name       LONGTEXT NOT NULL DEFAULT '',
	file_path       LONGTEXT NOT NULL DEFAULT '',
	source_url      LONGTEXT NOT NULL DEFAULT '',
	media_type      VARCHAR(255) NOT NULL DEFAULT '',
	content_id      VARCHAR(255) NOT NULL DEFAULT '',
	opened_at       DATETIME NOT NULL,
	PRIMARY KEY (owner_id, media_kind, media_identity_hash)
);

CREATE TABLE IF NOT EXISTS user_preferences (
	owner_id        VARCHAR(255) NOT NULL DEFAULT '',
	preference_key  VARCHAR(255) NOT NULL DEFAULT '',
	value_json      LONGTEXT NOT NULL DEFAULT '{}',
	updated_at      DATETIME NOT NULL,
	PRIMARY KEY (owner_id, preference_key)
);

CREATE INDEX IF NOT EXISTS idx_agents_owner_id ON agents(owner_id);
CREATE INDEX IF NOT EXISTS idx_agents_channel_id ON agents(channel_id);
CREATE INDEX IF NOT EXISTS idx_runs_owner_id ON runs(owner_id);
CREATE INDEX IF NOT EXISTS idx_runs_state ON runs(state);
CREATE INDEX IF NOT EXISTS idx_runs_sandbox_id ON runs(sandbox_id);
CREATE INDEX IF NOT EXISTS idx_runs_agent_id ON runs(agent_id);
CREATE INDEX IF NOT EXISTS idx_runs_owner_agent_state_updated ON runs(owner_id, agent_id, state, updated_at);
CREATE INDEX IF NOT EXISTS idx_runs_channel_id ON runs(channel_id);
CREATE INDEX IF NOT EXISTS idx_trajectories_owner_status ON trajectories(owner_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_work_items_trajectory_status ON work_items(trajectory_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_work_items_owner_status ON work_items(owner_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_work_items_fingerprint ON work_items(owner_id, trajectory_id, objective_fingerprint);
CREATE INDEX IF NOT EXISTS idx_events_loop_id_seq ON events(loop_id, seq);
CREATE INDEX IF NOT EXISTS idx_events_owner_id ON events(owner_id);
CREATE INDEX IF NOT EXISTS idx_events_agent_id ON events(agent_id);
CREATE INDEX IF NOT EXISTS idx_events_channel_id_ts ON events(channel_id, ts);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
CREATE INDEX IF NOT EXISTS idx_channel_messages_owner_id ON channel_messages(owner_id);
CREATE INDEX IF NOT EXISTS idx_channel_messages_created_at ON channel_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_channel_messages_to_agent_id ON channel_messages(to_agent_id);
CREATE INDEX IF NOT EXISTS idx_channel_messages_trajectory_id ON channel_messages(trajectory_id);
CREATE INDEX IF NOT EXISTS idx_inbox_deliveries_created_at ON inbox_deliveries(created_at);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_loop_seq ON run_memory_entries(loop_id, seq);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_owner_loop_seq ON run_memory_entries(owner_id, loop_id, seq);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_owner_agent_created ON run_memory_entries(owner_id, agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_parent ON run_memory_entries(parent_entry_id);
CREATE INDEX IF NOT EXISTS idx_run_acceptances_owner_updated ON run_acceptances(owner_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_acceptances_owner_trajectory ON run_acceptances(owner_id, trajectory_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_acceptances_owner_loop ON run_acceptances(owner_id, loop_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_acceptances_target_mission ON run_acceptances(target_mission_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_continuations_owner_status ON run_continuations(owner_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_continuations_source_loop ON run_continuations(source_loop_id);
CREATE INDEX IF NOT EXISTS idx_run_continuations_next_loop ON run_continuations(next_loop_id);
CREATE INDEX IF NOT EXISTS idx_browser_sessions_owner_updated ON browser_sessions(owner_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_worker_updates_channel_id ON worker_updates(channel_id, created_at);
CREATE INDEX IF NOT EXISTS idx_worker_updates_target_agent_id ON worker_updates(target_agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_worker_updates_trajectory_id ON worker_updates(trajectory_id, created_at);
CREATE INDEX IF NOT EXISTS idx_co_super_slots_run_id ON co_super_slots(run_id);
CREATE INDEX IF NOT EXISTS idx_co_super_slots_agent_id ON co_super_slots(owner_id, agent_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_media_progress_owner_kind_updated ON media_progress(owner_id, media_kind, updated_at);
CREATE INDEX IF NOT EXISTS idx_media_recents_owner_kind_opened ON media_recents(owner_id, media_kind, opened_at);
CREATE INDEX IF NOT EXISTS idx_user_preferences_owner_updated ON user_preferences(owner_id, updated_at);

CREATE TABLE IF NOT EXISTS desktop_state (
	owner_id       VARCHAR(255) PRIMARY KEY,
	windows_json   LONGTEXT NOT NULL DEFAULT '[]',
	active_window  VARCHAR(255) NOT NULL DEFAULT '',
	updated_at     DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS desktop_workspaces (
	owner_id       VARCHAR(255) NOT NULL,
	desktop_id     VARCHAR(255) NOT NULL,
	windows_json   LONGTEXT NOT NULL DEFAULT '[]',
	active_window  VARCHAR(255) NOT NULL DEFAULT '',
	updated_at     DATETIME NOT NULL,
	PRIMARY KEY (owner_id, desktop_id)
);

CREATE TABLE IF NOT EXISTS desktop_sessions (
	owner_id          VARCHAR(255) NOT NULL,
	desktop_id        VARCHAR(255) NOT NULL,
	session_id        VARCHAR(255) NOT NULL,
	device_id         VARCHAR(255) NOT NULL DEFAULT '',
	viewport_profile  VARCHAR(64) NOT NULL DEFAULT '',
	visibility_state  VARCHAR(64) NOT NULL DEFAULT '',
	last_input_at     DATETIME NULL,
	driver_until      DATETIME NULL,
	created_at        DATETIME NOT NULL,
	updated_at        DATETIME NOT NULL,
	PRIMARY KEY (owner_id, desktop_id, session_id)
);

CREATE TABLE IF NOT EXISTS desktop_app_instances (
	owner_id              VARCHAR(255) NOT NULL,
	desktop_id            VARCHAR(255) NOT NULL,
	app_instance_id       VARCHAR(255) NOT NULL,
	app_id                VARCHAR(255) NOT NULL,
	title                 LONGTEXT NOT NULL DEFAULT '',
	app_context_json      LONGTEXT NOT NULL DEFAULT '{}',
	lifecycle             VARCHAR(64) NOT NULL DEFAULT 'open',
	shared_stack_rank     BIGINT NOT NULL DEFAULT 0,
	last_used_at          DATETIME NOT NULL,
	created_by_session_id VARCHAR(255) NOT NULL DEFAULT '',
	created_at            DATETIME NOT NULL,
	updated_at            DATETIME NOT NULL,
	PRIMARY KEY (owner_id, desktop_id, app_instance_id)
);

CREATE TABLE IF NOT EXISTS desktop_window_placements (
	owner_id               VARCHAR(255) NOT NULL,
	desktop_id             VARCHAR(255) NOT NULL,
	session_id             VARCHAR(255) NOT NULL,
	app_instance_id        VARCHAR(255) NOT NULL,
	x                      INT NOT NULL DEFAULT 100,
	y                      INT NOT NULL DEFAULT 100,
	width                  INT NOT NULL DEFAULT 600,
	height                 INT NOT NULL DEFAULT 400,
	mode                   VARCHAR(64) NOT NULL DEFAULT 'normal',
	local_z_index          BIGINT NOT NULL DEFAULT 0,
	local_focused          BOOLEAN NOT NULL DEFAULT FALSE,
	restored_geometry_json LONGTEXT NOT NULL DEFAULT '',
	updated_at             DATETIME NOT NULL,
	PRIMARY KEY (owner_id, desktop_id, session_id, app_instance_id)
);

CREATE TABLE IF NOT EXISTS computer_event_projection_heads (
	computer_id                    VARCHAR(128) PRIMARY KEY,
	sequence                       BIGINT UNSIGNED NOT NULL,
	canonical_event_head           CHAR(64) NOT NULL,
	desired_event_head             CHAR(64) NOT NULL,
	effective_event_head           CHAR(64) NOT NULL,
	pending_transition_ref         CHAR(64) NULL,
	desired_state_commitment       CHAR(64) NOT NULL,
	effective_state_commitment     CHAR(64) NOT NULL,
	reducer_version                BIGINT UNSIGNED NOT NULL,
	credential_revocation_epoch    BIGINT UNSIGNED NOT NULL,
	updated_at                     DATETIME(6) NOT NULL
);

CREATE TABLE IF NOT EXISTS computer_event_index (
	event_digest                   CHAR(64) PRIMARY KEY,
	computer_id                    VARCHAR(128) NOT NULL,
	sequence                       BIGINT UNSIGNED NOT NULL,
	previous_head                  CHAR(64) NOT NULL,
	event_kind                     VARCHAR(64) NOT NULL,
	event_json                     LONGTEXT NOT NULL,
	event_artifact_digest          CHAR(64) NOT NULL,
	event_pin_receipt_digest       CHAR(64) NOT NULL,
	payload_pin_receipt_digests_json LONGTEXT NOT NULL,
	request_commitment             CHAR(64) NOT NULL,
	idempotency_key                VARCHAR(255) NOT NULL,
	status                         VARCHAR(32) NOT NULL,
	next_desired_event_head        CHAR(64) NOT NULL,
	next_effective_event_head      CHAR(64) NOT NULL,
	next_pending_transition_ref    CHAR(64) NULL,
	next_desired_state_commitment  CHAR(64) NOT NULL,
	next_effective_state_commitment CHAR(64) NOT NULL,
	next_reducer_version           BIGINT UNSIGNED NOT NULL,
	next_credential_revocation_epoch BIGINT UNSIGNED NOT NULL,
	target_state_commitment        CHAR(64) NULL,
	restored_prior_effective       BOOLEAN NOT NULL DEFAULT FALSE,
	event_head_receipt_json        LONGTEXT NULL,
	event_head_receipt_digest      CHAR(64) NULL,
	prepared_at                    DATETIME(6) NOT NULL,
	finalized_at                   DATETIME(6) NULL,
	UNIQUE(computer_id, sequence),
	UNIQUE(computer_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_computer_event_index_status ON computer_event_index(computer_id, status, sequence);

CREATE TABLE IF NOT EXISTS computer_effective_state (
	computer_id                    VARCHAR(128) PRIMARY KEY,
	canonical_event_head           CHAR(64) NOT NULL,
	desired_event_head             CHAR(64) NOT NULL,
	effective_event_head           CHAR(64) NOT NULL,
	desired_state_commitment       CHAR(64) NOT NULL,
	effective_state_commitment     CHAR(64) NOT NULL,
	pending_transition_ref         CHAR(64) NULL,
	reducer_version                BIGINT UNSIGNED NOT NULL,
	effective_code_ref             LONGTEXT NOT NULL,
	effective_artifact_program_ref LONGTEXT NOT NULL,
	embedded_state_ref             LONGTEXT NOT NULL,
	release_digest                 CHAR(64) NOT NULL,
	checkpoint_ref                 LONGTEXT NOT NULL,
	last_receipt_digest            CHAR(64) NOT NULL,
	updated_at                     DATETIME(6) NOT NULL
);

CREATE TABLE IF NOT EXISTS self_development_start_intents (
	computer_id        VARCHAR(128) NOT NULL,
	idempotency_key    VARCHAR(255) NOT NULL,
	request_commitment CHAR(64) NOT NULL,
	created_at         DATETIME(6) NOT NULL,
	PRIMARY KEY(computer_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS self_development_operations (
	operation_id                   VARCHAR(255) PRIMARY KEY,
	computer_id                    VARCHAR(128) NOT NULL,
	idempotency_key                VARCHAR(255) NOT NULL,
	request_commitment             CHAR(64) NOT NULL,
	trajectory_id                  VARCHAR(255) NOT NULL,
	capsule_id                     VARCHAR(255) NOT NULL DEFAULT '',
	base_head                      CHAR(64) NOT NULL,
	prompt_artifact_ref            LONGTEXT NOT NULL,
	bundle_digest                  CHAR(64) NOT NULL DEFAULT '',
	release_digest                  CHAR(64) NOT NULL DEFAULT '',
	code_ref                        VARCHAR(96) NOT NULL DEFAULT '',
	artifact_program_ref            VARCHAR(128) NOT NULL DEFAULT '',
	verifier_refs_json             LONGTEXT NOT NULL DEFAULT '[]',
	decision_actor                 VARCHAR(255) NOT NULL DEFAULT '',
	decision_event                 CHAR(64) NOT NULL DEFAULT '',
	decision_receipt               VARCHAR(255) NOT NULL DEFAULT '',
	desired_head                   CHAR(64) NOT NULL,
	effective_head                 CHAR(64) NOT NULL,
	materialization_receipt        LONGTEXT NOT NULL DEFAULT '',
	checkpoint_ref                 LONGTEXT NOT NULL DEFAULT '',
	route_certificate              LONGTEXT NOT NULL DEFAULT '',
	route_generation               BIGINT UNSIGNED NULL,
	route_receipt                   VARCHAR(128) NOT NULL DEFAULT '',
	mode_receipt                   LONGTEXT NOT NULL DEFAULT '',
	lifecycle_receipt              LONGTEXT NOT NULL DEFAULT '',
	state                          VARCHAR(32) NOT NULL,
	terminal_error                 LONGTEXT NOT NULL DEFAULT '',
	created_at                     DATETIME(6) NOT NULL,
	updated_at                     DATETIME(6) NOT NULL,
	UNIQUE(computer_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_self_development_operations_trajectory ON self_development_operations(trajectory_id);
CREATE INDEX IF NOT EXISTS idx_self_development_operations_state ON self_development_operations(computer_id, state, updated_at);
CREATE INDEX IF NOT EXISTS idx_self_development_operations_bundle ON self_development_operations(computer_id, bundle_digest);

CREATE INDEX IF NOT EXISTS idx_desktop_workspaces_owner_id ON desktop_workspaces(owner_id);
CREATE INDEX IF NOT EXISTS idx_desktop_sessions_driver ON desktop_sessions(owner_id, desktop_id, driver_until);
CREATE INDEX IF NOT EXISTS idx_desktop_app_instances_stack ON desktop_app_instances(owner_id, desktop_id, shared_stack_rank);
CREATE INDEX IF NOT EXISTS idx_desktop_window_placements_instance ON desktop_window_placements(owner_id, desktop_id, app_instance_id, updated_at);
`

// Open opens (or creates) the unified embedded Dolt workspace derived from
// dbPath and applies the runtime and texture schemas. A legacy SQLite file at
// dbPath is retained only as inert rollback evidence; serving startup never
// reads or imports it.
func Open(dbPath string) (*Store, error) {
	log.Printf("store: open phase=prepare-path status=starting")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("runtime store: create directory: %w", err)
	}

	freshStore := false
	if _, err := os.Stat(dbPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("runtime store: stat marker or legacy sqlite: %w", err)
		}
		freshStore = true
		_ = os.RemoveAll(deriveTextureWorkspacePath(dbPath))
	}
	log.Printf("store: open phase=prepare-path status=complete fresh=%t", freshStore)

	log.Printf("store: open phase=workspace-open status=starting")
	db, workspacePath, connector, err := openTextureWorkspaceDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("runtime store: open unified Dolt workspace: %w", err)
	}
	log.Printf("store: open phase=workspace-open status=complete")

	readDB := sql.OpenDB(connector)
	configureEmbeddedDoltDB(readDB)

	s := &Store{db: db, readDB: readDB, path: dbPath, texturePath: workspacePath, doltConnector: connector}
	log.Printf("store: open phase=runtime-schema status=starting")
	if err := s.bootstrap(); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("runtime store: bootstrap: %w", err)
	}
	log.Printf("store: open phase=runtime-schema status=complete")

	// Initialize object-graph persistence on the same private Dolt workspace.
	ogDoltStore := objectgraph.NewDoltStore(db)
	log.Printf("store: open phase=objectgraph-schema status=starting")
	if err := ogDoltStore.EnsureSchema(context.Background()); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("runtime store: bootstrap object graph: %w", err)
	}
	log.Printf("store: open phase=objectgraph-schema status=complete")
	s.ogStore = ogDoltStore
	// Create a read-only DoltStore using the read connection pool so OG
	// reads don't block during write transactions on the main connection.
	if readDB != nil {
		s.ogReadStore = objectgraph.NewDoltStore(readDB)
	}
	s.og = objectgraph.NewService(objectgraph.Config{
		Durable: ogDoltStore,
	})

	// Apply the texture schema to the embedded Dolt workspace.
	log.Printf("store: open phase=texture-schema status=starting")
	if err := s.EnsureTextureSchema(); err != nil {
		log.Printf("ERROR EnsureTextureSchema failed: %v", err)
		_ = s.Close()
		return nil, fmt.Errorf("runtime store: bootstrap texture: %w", err)
	}
	log.Printf("store: open phase=texture-schema status=complete")

	if freshStore {
		if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("runtime store: write marker: %w", err)
		}
	}

	return s, nil
}

// bootstrap applies the schema DDL to the database.
func (s *Store) bootstrap() error {
	_, err := s.db.Exec(schemaDDL)
	if err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	for _, migration := range []struct {
		table string
		name  string
		ddl   string
	}{
		{"runs", "agent_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"runs", "channel_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"runs", "requested_by_run_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"runs", "agent_profile", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"runs", "agent_role", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"runs", "trajectory_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"co_super_slots", "requested_by_run_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"events", "agent_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"events", "channel_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"events", "trajectory_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"events", "stream_seq", "BIGINT NOT NULL DEFAULT 0"},
		{"channel_messages", "to_agent_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"channel_messages", "to_loop_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"channel_messages", "trajectory_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"inbox_deliveries", "delivered_to_loop_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"inbox_deliveries", "delivered_at", "DATETIME"},
		{"browser_sessions", "html_snapshot", "LONGTEXT NOT NULL DEFAULT ''"},
		{"browser_sessions", "links_json", "LONGTEXT NOT NULL DEFAULT '[]'"},
		{"browser_sessions", "screenshot_png_base64", "LONGTEXT NOT NULL DEFAULT ''"},
		{"browser_sessions", "execution_scope", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"browser_sessions", "backend_session_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"browser_sessions", "world_kind", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"browser_sessions", "vm_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"browser_sessions", "snapshot_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"browser_sessions", "source_loop_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"browser_sessions", "candidate_trace_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"worker_updates", "kind", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"worker_updates", "summary", "LONGTEXT NOT NULL DEFAULT ''"},
		{"worker_updates", "packet_json", "LONGTEXT NOT NULL DEFAULT '{}'"},
		{"worker_updates", "delivered_to_loop_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"worker_updates", "delivered_at", "DATETIME"},
		{"self_development_operations", "route_receipt", "VARCHAR(128) NOT NULL DEFAULT ''"},
		{"self_development_operations", "release_digest", "CHAR(64) NOT NULL DEFAULT ''"},
		{"self_development_operations", "code_ref", "VARCHAR(96) NOT NULL DEFAULT ''"},
		{"self_development_operations", "artifact_program_ref", "VARCHAR(128) NOT NULL DEFAULT ''"},
		{"self_development_operations", "decision_receipt", "VARCHAR(255) NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn(migration.table, migration.name, migration.ddl); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_owner_stream_seq ON events(owner_id, stream_seq)`); err != nil {
		return fmt.Errorf("create idx_events_owner_stream_seq: %w", err)
	}
	// After ensureColumn so existing databases gain runs.trajectory_id first.
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_runs_trajectory_id ON runs(trajectory_id)`); err != nil {
		return fmt.Errorf("create idx_runs_trajectory_id: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_runs_requested_by_run_id ON runs(requested_by_run_id)`); err != nil {
		return fmt.Errorf("create idx_runs_requested_by_run_id: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_trajectory_stream_seq ON events(trajectory_id, stream_seq)`); err != nil {
		return fmt.Errorf("create idx_events_trajectory_stream_seq: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_inbox_deliveries_owner_target ON inbox_deliveries(owner_id, to_agent_id, delivered_at)`); err != nil {
		return fmt.Errorf("create idx_inbox_deliveries_owner_target: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_worker_updates_pending_target ON worker_updates(owner_id, target_agent_id, delivered_at, created_at)`); err != nil {
		return fmt.Errorf("create idx_worker_updates_pending_target: %w", err)
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_coagent_mailboxes_channel ON coagent_mailboxes(owner_id, channel_id)`); err != nil {
		return fmt.Errorf("create idx_coagent_mailboxes_channel: %w", err)
	}
	return s.backfillDerivedRuntimeState()
}

func (s *Store) backfillDerivedRuntimeState() error {
	if _, err := s.db.Exec(`UPDATE events SET stream_seq = seq WHERE stream_seq = 0`); err != nil {
		return fmt.Errorf("backfill events.stream_seq: %w", err)
	}
	if _, err := s.db.Exec(`
		INSERT INTO desktop_workspaces (owner_id, desktop_id, windows_json, active_window, updated_at)
		SELECT owner_id, 'primary', windows_json, active_window, updated_at
		  FROM desktop_state
		 WHERE NOT EXISTS (
			SELECT 1
			  FROM desktop_workspaces dw
			 WHERE dw.owner_id = desktop_state.owner_id
			   AND dw.desktop_id = 'primary'
		 )`); err != nil {
		return fmt.Errorf("migrate desktop_state to desktop_workspaces: %w", err)
	}
	return nil
}

func normalizeDesktopID(desktopID string) string {
	desktopID = strings.TrimSpace(desktopID)
	if desktopID == "" {
		return types.PrimaryDesktopID
	}
	return desktopID
}

func (s *Store) ensureColumn(table, name, ddl string) error {
	if err := validateIdentifier(table); err != nil {
		return err
	}
	if err := validateIdentifier(name); err != nil {
		return err
	}
	var count int
	if err := s.db.QueryRow(`
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = ?
  AND column_name = ?`,
		table,
		name,
	).Scan(&count); err != nil {
		return fmt.Errorf("information_schema.columns(%s.%s): %w", table, name, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, ddl)); err != nil {
		return fmt.Errorf("alter table %s add column %s: %w", table, name, err)
	}
	return nil
}

func validateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("empty SQL identifier")
	}
	for _, r := range name {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return fmt.Errorf("unsafe SQL identifier %q", name)
	}
	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	var err error
	if db := s.textureDB; db != nil {
		func() {
			defer func() {
				if r := recover(); r != nil && err == nil {
					err = fmt.Errorf("close texture workspace: %v", r)
				}
			}()
			if closeErr := db.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}()
	}
	if db := s.readDB; db != nil {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	if db := s.db; db != nil {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	if connector := s.doltConnector; connector != nil {
		if closeErr := connector.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}

func (s *Store) queryDB() *sql.DB {
	if s.readDB != nil {
		return s.readDB
	}
	return s.db
}

// TexturePath returns the filesystem path backing the embedded texture workspace.
func (s *Store) TexturePath() string {
	return s.texturePath
}

// UpsertAgent persists a durable agent record.
func (s *Store) UpsertAgent(ctx context.Context, rec types.AgentRecord) error {
	return s.UpsertAgentOG(ctx, rec)
}

// GetAgentByScope returns an agent by the complete durable identity tuple.
func (s *Store) GetAgentByScope(ctx context.Context, ownerID, computerID, agentID string) (types.AgentRecord, error) {
	return s.GetAgentByScopeOG(ctx, ownerID, computerID, agentID)
}

func runTrajectoryID(rec types.RunRecord) string {
	trajectoryID := strings.TrimSpace(rec.TrajectoryID)
	if trajectoryID != "" {
		return trajectoryID
	}
	legacyTrajectoryID, _ := rec.Metadata["trajectory_id"].(string)
	return strings.TrimSpace(legacyTrajectoryID)
}

func (s *Store) rejectActiveRunOnTerminalTrajectory(ctx context.Context, rec types.RunRecord, operation string) error {
	trajectoryID := runTrajectoryID(rec)
	if !rec.State.Active() || trajectoryID == "" {
		return nil
	}
	trajectory, err := s.GetTrajectoryOG(ctx, rec.OwnerID, trajectoryID)
	if err == nil {
		if trajectory.Status == types.TrajectorySettled || trajectory.Status == types.TrajectoryCancelled {
			return fmt.Errorf("%s run on terminal trajectory: %w", operation, ErrConcurrentStateChange)
		}
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// CreateRun inserts a new run record.
func (s *Store) CreateRun(ctx context.Context, rec types.RunRecord) error {
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	rec.TrajectoryID = runTrajectoryID(rec)

	if err := s.rejectActiveRunOnTerminalTrajectory(ctx, rec, "create"); err != nil {
		return err
	}
	return s.CreateRunOG(ctx, rec)
}

// GetRun returns the run with the given run ID.
func (s *Store) GetRun(ctx context.Context, runID string) (types.RunRecord, error) {
	return s.GetRunOG(ctx, runID)
}

// GetRunByOwner returns a run by its owner-scoped canonical identity.
func (s *Store) GetRunByOwner(ctx context.Context, ownerID, runID string) (types.RunRecord, error) {
	return s.GetRunByOwnerOG(ctx, ownerID, runID)
}

// UpdateRun updates an existing run record.
func (s *Store) UpdateRun(ctx context.Context, rec types.RunRecord) error {
	if !rec.State.Active() {
		return s.UpdateRunOG(ctx, rec)
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	rec.TrajectoryID = runTrajectoryID(rec)

	var existing types.RunRecord
	var err error
	if rec.OwnerID == "" {
		existing, err = s.GetRunOG(ctx, rec.RunID)
	} else {
		existing, err = s.GetRunByOwnerOG(ctx, rec.OwnerID, rec.RunID)
	}
	if err != nil {
		return err
	}
	if !existing.State.Active() {
		if err := s.rejectActiveRunOnTerminalTrajectory(ctx, rec, "reactivate"); err != nil {
			return err
		}
	}
	return s.UpdateRunOG(ctx, rec)
}

// UpdateRunAndMarkWorkerUpdatesDelivered updates a run and marks its waking
// update_coagent records for the run's agent delivered.
func (s *Store) UpdateRunAndMarkWorkerUpdatesDelivered(ctx context.Context, rec types.RunRecord, ownerID string, updateIDs []string) error {
	// Mark worker updates delivered BEFORE persisting the run's terminal
	// state. Otherwise a concurrent reader (e.g., waitForRuntimeRunTerminal)
	// can observe the run as completed before the updates are marked,
	// producing a false "not delivered" observation.
	if len(updateIDs) > 0 {
		if err := s.MarkWorkerUpdatesDelivered(ctx, ownerID, rec.AgentID, updateIDs, rec.RunID); err != nil {
			return err
		}
	}
	if err := s.UpdateRunOG(ctx, rec); err != nil {
		// Rollback: unmark the worker updates if the run update failed,
		// so the mailbox cursor doesn't consume them while the run
		// remains non-terminal. Use context.Background() because the
		// caller's context may be canceled, and the rollback must still
		// execute to preserve delivery atomicity.
		if len(updateIDs) > 0 {
			_ = s.unmarkWorkerUpdatesDelivered(context.Background(), ownerID, rec.AgentID, updateIDs)
		}
		return err
	}
	return nil
}

// unmarkWorkerUpdatesDelivered clears the delivery state on worker update
// records. Used as a compensating rollback when UpdateRunOG fails after
// MarkWorkerUpdatesDelivered succeeded.
func (s *Store) unmarkWorkerUpdatesDelivered(ctx context.Context, ownerID, targetAgentID string, updateIDs []string) error {
	s.workerUpdateMu.Lock()
	defer s.workerUpdateMu.Unlock()
	for _, id := range updateIDs {
		rec, err := s.GetWorkerUpdateOG(ctx, ownerID, id)
		if err != nil {
			continue
		}
		if rec.TargetAgentID != targetAgentID {
			continue
		}
		if rec.LifecycleVersion > 0 {
			continue
		}
		rec.DeliveredToRunID = ""
		rec.DeliveredAt = nil
		_ = s.CreateWorkerUpdateOG(ctx, rec)
	}
	return s.refreshCoagentMailboxCursorOG(ctx, ownerID, targetAgentID)
}

// ListRunsByOwner returns runs for the given owner, ordered by created_at
// descending, limited to the given count.
func (s *Store) ListRunsByOwner(ctx context.Context, ownerID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListRunsByOwnerOG(ctx, ownerID, limit)
}

// ListRunsByIngestionHandoff returns top-level runs for one typed ingestion
// identity. Every identity component is applied inside the owner-scoped object
// query so unrelated owners and provenance-inheriting child runs cannot consume
// a global result window before the caller sees its authoritative receipt.
func (s *Store) ListRunsByIngestionHandoff(ctx context.Context, ownerID, profile, requestID, requestKind string, limit int) ([]types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	profile = strings.TrimSpace(profile)
	requestID = strings.TrimSpace(requestID)
	requestKind = strings.TrimSpace(requestKind)
	if ownerID == "" || profile == "" || requestID == "" || requestKind == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	objects, err := s.ogListByOwnerAndBody(ctx, ogKindRun, ownerID, []objectgraph.JSONFieldMatch{
		{JSONPath: "$.metadata.ingestion_handoff_request_id", Value: requestID},
		{JSONPath: "$.metadata.ingestion_handoff_request_kind", Value: requestKind},
		{JSONPath: "$.agent_profile", Value: profile},
		{JSONPath: "$.requested_by_run_id", Value: "", MissingMatchesEmpty: true},
	}, limit)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, min(limit, len(objects)))
	for _, object := range objects {
		var rec types.RunRecord
		if err := ogDecode(object, &rec); err != nil {
			return nil, err
		}
		if strings.TrimSpace(rec.OwnerID) != ownerID ||
			!strings.EqualFold(strings.TrimSpace(rec.AgentProfile), profile) ||
			strings.TrimSpace(rec.RequestedByRunID) != "" {
			continue
		}
		kind, _ := rec.Metadata["ingestion_handoff_request_kind"].(string)
		id, _ := rec.Metadata["ingestion_handoff_request_id"].(string)
		if strings.TrimSpace(id) != requestID || strings.TrimSpace(kind) != requestKind {
			continue
		}
		runs = append(runs, rec)
		if len(runs) >= limit {
			break
		}
	}
	return runs, nil
}

// ListRunsBySelfDevelopmentOperation returns the unique top-level Super run
// bound to a durable self-development operation.
func (s *Store) ListRunsBySelfDevelopmentOperation(ctx context.Context, ownerID, operationID string, limit int) ([]types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	operationID = strings.TrimSpace(operationID)
	if ownerID == "" || operationID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 2
	}
	objects, err := s.ogListByOwnerAndBody(ctx, ogKindRun, ownerID, []objectgraph.JSONFieldMatch{
		{JSONPath: "$.metadata.self_development_operation_id", Value: operationID},
		{JSONPath: "$.requested_by_run_id", Value: "", MissingMatchesEmpty: true},
	}, limit)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, min(limit, len(objects)))
	for _, object := range objects {
		var rec types.RunRecord
		if err := ogDecode(object, &rec); err != nil {
			return nil, err
		}
		boundID, _ := rec.Metadata["self_development_operation_id"].(string)
		if strings.TrimSpace(rec.OwnerID) == ownerID && strings.TrimSpace(rec.RequestedByRunID) == "" && strings.TrimSpace(boundID) == operationID {
			runs = append(runs, rec)
		}
	}
	return runs, nil
}

// ListRunsByState returns runs in the given state, ordered by created_at
// descending, limited to the given count.
func (s *Store) ListRunsByState(ctx context.Context, state types.RunState, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListRunsByStateOG(ctx, state, limit)
}

// ListAllRunsByState exhausts all keyset pages for the requested state.
func (s *Store) ListAllRunsByState(ctx context.Context, state types.RunState) ([]types.RunRecord, error) {
	return s.ListAllRunsByStateOG(ctx, state)
}

// ListRuns returns recent runs ordered by created_at descending, limited
// to the given count.
func (s *Store) ListRuns(ctx context.Context, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListAllRunsOG(ctx, limit)
}

// ListRunsByChannel returns runs for a specific coordination channel, ordered by creation time descending.
func (s *Store) ListRunsByChannel(ctx context.Context, ownerID, channelID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	// Fetch a large window since ogListByMetadata orders by updated_at
	// DESC and we need to filter by owner_id before applying the limit.
	// Using limit*4 could miss runs when channel IDs are shared across
	// owners.
	objs, err := s.ogListByMetadata(ctx, ogKindRun, "channel_id", channelID, 100000)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if rec.ChannelID != channelID {
			continue
		}
		runs = append(runs, rec)
		if len(runs) >= limit {
			break
		}
	}
	return runs, nil
}

// ListActiveRunsByTrajectory returns pending/running/blocked activations on a
// trajectory. The metadata keyset is exhausted before owner and state filters
// are applied. A non-positive limit returns every matching active run.
func (s *Store) ListActiveRunsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if ownerID == "" || trajectoryID == "" {
		return nil, nil
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindRun, "trajectory_id", trajectoryID)
	if err != nil {
		return nil, err
	}
	resultCapacity := len(objs)
	if limit > 0 && limit < resultCapacity {
		resultCapacity = limit
	}
	runs := make([]types.RunRecord, 0, resultCapacity)
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.TrajectoryID == "" {
			rec.TrajectoryID = runTrajectoryID(rec)
		}
		if rec.OwnerID != ownerID || rec.TrajectoryID != trajectoryID || !rec.State.Active() {
			continue
		}
		runs = append(runs, rec)
		if limit > 0 && len(runs) >= limit {
			break
		}
	}
	return runs, nil
}

// ClaimCoSuperSlot atomically claims (owner, trajectory, slot) for a co-super
// run. If a live run already owns the slot, that run is returned and claimed is
// false. If the previous owner is terminal, the slot is advanced to runID.
func (s *Store) ClaimCoSuperSlot(ctx context.Context, ownerID, trajectoryID, slot, runID, agentID, requesterRunID string) (types.RunRecord, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	slot = strings.TrimSpace(slot)
	runID = strings.TrimSpace(runID)
	if ownerID == "" || trajectoryID == "" || slot == "" || runID == "" {
		return types.RunRecord{}, false, fmt.Errorf("claim co-super slot: owner_id, trajectory_id, slot, and run_id are required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO co_super_slots (owner_id, trajectory_id, slot, run_id, agent_id, requested_by_run_id, claimed_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE run_id = run_id`,
		ownerID, trajectoryID, slot, runID, agentID, requesterRunID, now, now,
	)
	if err != nil {
		return types.RunRecord{}, false, fmt.Errorf("insert co-super slot claim: %w", err)
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 1 {
		return types.RunRecord{}, true, nil
	}

	existingRunID, err := s.coSuperSlotRunID(ctx, ownerID, trajectoryID, slot)
	if err != nil {
		return types.RunRecord{}, false, err
	}
	existing, err := s.GetRun(ctx, existingRunID)
	if err == nil && existing.State.Active() {
		return existing, false, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return types.RunRecord{}, false, err
	}

	res, err = s.db.ExecContext(ctx,
		`UPDATE co_super_slots
		    SET run_id = ?,
		        agent_id = ?,
		        requested_by_run_id = ?,
		        claimed_at = ?,
		        updated_at = ?
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		    AND slot = ?
		    AND run_id = ?`,
		runID, agentID, requesterRunID, now, now, ownerID, trajectoryID, slot, existingRunID,
	)
	if err != nil {
		return types.RunRecord{}, false, fmt.Errorf("advance co-super slot claim: %w", err)
	}
	if rows, err := res.RowsAffected(); err != nil {
		return types.RunRecord{}, false, fmt.Errorf("check co-super slot claim rows: %w", err)
	} else if rows == 0 {
		existingRunID, err = s.coSuperSlotRunID(ctx, ownerID, trajectoryID, slot)
		if err != nil {
			return types.RunRecord{}, false, err
		}
		existing, err := s.GetRun(ctx, existingRunID)
		return existing, false, err
	}
	return types.RunRecord{}, true, nil
}

// ReleaseCoSuperSlotClaim releases a newly claimed co-super slot only if it is
// still owned by runID. It is used to avoid leaving a durable claim behind when
// run creation fails after the slot claim succeeds.
func (s *Store) ReleaseCoSuperSlotClaim(ctx context.Context, ownerID, trajectoryID, slot, runID string) error {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	slot = strings.TrimSpace(slot)
	runID = strings.TrimSpace(runID)
	if ownerID == "" || trajectoryID == "" || slot == "" || runID == "" {
		return fmt.Errorf("release co-super slot: owner_id, trajectory_id, slot, and run_id are required")
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM co_super_slots
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		    AND slot = ?
		    AND run_id = ?`,
		ownerID,
		trajectoryID,
		slot,
		runID,
	); err != nil {
		return fmt.Errorf("release co-super slot claim: %w", err)
	}
	return nil
}

// CoSuperSlotRecord is the durable trajectory-slot ownership record for a
// co-super activation. RequestedByRunID is retained as requester provenance,
// not as the authority relation.
type CoSuperSlotRecord struct {
	OwnerID          string
	TrajectoryID     string
	Slot             string
	RunID            string
	AgentID          string
	RequestedByRunID string
	ClaimedAt        time.Time
	UpdatedAt        time.Time
}

func (s *Store) coSuperSlotRunID(ctx context.Context, ownerID, trajectoryID, slot string) (string, error) {
	var runID string
	err := s.db.QueryRowContext(ctx,
		`SELECT run_id
		   FROM co_super_slots
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		    AND slot = ?`,
		ownerID,
		trajectoryID,
		slot,
	).Scan(&runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("query co-super slot: %w", err)
	}
	return runID, nil
}

// CoSuperSlotByAgent returns the most recent trajectory slot record for a
// co-super agent. Authority callers use this instead of active-run parentage.
func (s *Store) CoSuperSlotByAgent(ctx context.Context, ownerID, agentID string) (CoSuperSlotRecord, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return CoSuperSlotRecord{}, false, nil
	}
	var rec CoSuperSlotRecord
	err := s.db.QueryRowContext(ctx,
		`SELECT owner_id, trajectory_id, slot, run_id, agent_id, requested_by_run_id, claimed_at, updated_at
		   FROM co_super_slots
		  WHERE owner_id = ?
		    AND agent_id = ?
		  ORDER BY updated_at DESC, claimed_at DESC, trajectory_id DESC, slot DESC
		  LIMIT 1`,
		ownerID,
		agentID,
	).Scan(
		&rec.OwnerID,
		&rec.TrajectoryID,
		&rec.Slot,
		&rec.RunID,
		&rec.AgentID,
		&rec.RequestedByRunID,
		&rec.ClaimedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CoSuperSlotRecord{}, false, nil
		}
		return CoSuperSlotRecord{}, false, fmt.Errorf("query co-super slot by agent: %w", err)
	}
	return rec, true, nil
}

// CoSuperSlotByAgentAndTrajectory returns the trajectory slot record for a
// co-super agent on a specific trajectory.
func (s *Store) CoSuperSlotByAgentAndTrajectory(ctx context.Context, ownerID, trajectoryID, agentID string) (CoSuperSlotRecord, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || trajectoryID == "" || agentID == "" {
		return CoSuperSlotRecord{}, false, nil
	}
	var rec CoSuperSlotRecord
	err := s.db.QueryRowContext(ctx,
		`SELECT owner_id, trajectory_id, slot, run_id, agent_id, requested_by_run_id, claimed_at, updated_at
		   FROM co_super_slots
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		    AND agent_id = ?
		  ORDER BY updated_at DESC, claimed_at DESC, slot DESC
		  LIMIT 1`,
		ownerID,
		trajectoryID,
		agentID,
	).Scan(
		&rec.OwnerID,
		&rec.TrajectoryID,
		&rec.Slot,
		&rec.RunID,
		&rec.AgentID,
		&rec.RequestedByRunID,
		&rec.ClaimedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CoSuperSlotRecord{}, false, nil
		}
		return CoSuperSlotRecord{}, false, fmt.Errorf("query co-super slot by agent and trajectory: %w", err)
	}
	return rec, true, nil
}

// CoSuperSlotRun returns the run currently recorded for a co-super
// (trajectory, slot), including terminal and passivated history. Admission
// callers use this as the trajectory slot authority instead of parent-child
// ancestry.
func (s *Store) CoSuperSlotRun(ctx context.Context, ownerID, trajectoryID, slot string) (types.RunRecord, bool, error) {
	runID, err := s.coSuperSlotRunID(ctx, ownerID, trajectoryID, slot)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return types.RunRecord{}, false, nil
		}
		return types.RunRecord{}, false, err
	}
	rec, err := s.GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return types.RunRecord{}, false, nil
		}
		return types.RunRecord{}, false, err
	}
	return rec, true, nil
}

// ActiveCoSuperSlotRun returns the live run currently claiming a
// (trajectory, slot), if any. Passivated runs are reusable non-terminal
// history, not active slot owners.
func (s *Store) ActiveCoSuperSlotRun(ctx context.Context, ownerID, trajectoryID, slot string) (types.RunRecord, bool, error) {
	rec, found, err := s.CoSuperSlotRun(ctx, ownerID, trajectoryID, slot)
	if err != nil {
		return types.RunRecord{}, false, err
	}
	if !found || !rec.State.Active() {
		return types.RunRecord{}, false, nil
	}
	return rec, true, nil
}

// CountActiveCoSuperSlots returns the number of trajectory-scoped co-super
// slots whose recorded activation is still active.
func (s *Store) CountActiveCoSuperSlots(ctx context.Context, ownerID, trajectoryID string) (int, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if ownerID == "" || trajectoryID == "" {
		return 0, nil
	}
	// Fetch slot run IDs from SQL (co_super_slots table not yet in OG)
	// and check run state via OG.
	rows, err := s.db.QueryContext(ctx,
		`SELECT run_id FROM co_super_slots WHERE owner_id = ? AND trajectory_id = ?`,
		ownerID, trajectoryID,
	)
	if err != nil {
		return 0, fmt.Errorf("count active co-super slots: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var runID string
		if err := rows.Scan(&runID); err != nil {
			return 0, fmt.Errorf("count active co-super slots: scan: %w", err)
		}
		rec, err := s.GetRunOG(ctx, runID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return 0, fmt.Errorf("count active co-super slots: get run: %w", err)
		}
		if rec.State.Active() {
			count++
		}
	}
	return count, rows.Err()
}

// GetLatestActiveRunByAgent returns the most recent non-terminal run for an agent.
func (s *Store) GetLatestActiveRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, error) {
	objs, err := s.ogListByMetadata(ctx, ogKindRun, "agent_id", agentID, 200)
	if err != nil {
		return types.RunRecord{}, err
	}
	var latest *types.RunRecord
	for i := range objs {
		var rec types.RunRecord
		if err := ogDecode(objs[i], &rec); err != nil {
			return types.RunRecord{}, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if !rec.State.Active() {
			continue
		}
		if latest == nil || rec.UpdatedAt.After(latest.UpdatedAt) {
			recCopy := rec
			latest = &recCopy
		}
	}
	if latest == nil {
		return types.RunRecord{}, ErrNotFound
	}
	return *latest, nil
}

// GetLatestPassivatedRunByAgent returns the most recent passivated activation
// for an actor identity. Durable actor reactivation uses this before minting a
// replacement run.
func (s *Store) GetLatestPassivatedRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, error) {
	objs, err := s.ogListByMetadata(ctx, ogKindRun, "agent_id", agentID, 200)
	if err != nil {
		return types.RunRecord{}, err
	}
	var latest *types.RunRecord
	for i := range objs {
		var rec types.RunRecord
		if err := ogDecode(objs[i], &rec); err != nil {
			return types.RunRecord{}, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if rec.State != types.RunPassivated {
			continue
		}
		if latest == nil || rec.UpdatedAt.After(latest.UpdatedAt) {
			recCopy := rec
			latest = &recCopy
		}
	}
	if latest == nil {
		return types.RunRecord{}, ErrNotFound
	}
	return *latest, nil
}

func (s *Store) listRunsWhere(ctx context.Context, where string, args []any, limit int) ([]types.RunRecord, error) {
	query := `SELECT loop_id, agent_id, channel_id, requested_by_run_id, trajectory_id, agent_profile, agent_role, owner_id, sandbox_id, state, prompt, result, error, created_at, updated_at, finished_at, metadata_json
	            FROM runs`
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []types.RunRecord
	for rows.Next() {
		rec, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return runs, nil
}

// AppendEvent appends an event record with an auto-assigned sequence number.
// The Seq field on the input record is overwritten with the next sequence
// number for the run.
func (s *Store) AppendEvent(ctx context.Context, rec *types.EventRecord) error {
	if len(rec.Payload) == 0 {
		rec.Payload = json.RawMessage(`{}`)
	}

	// Serialize event sequence allocation to prevent two concurrent
	// goroutines from reading the same max and assigning duplicate Seq
	// or StreamSeq values. The old SQL path relied on UNIQUE(loop_id, seq).
	s.eventMu.Lock()
	defer s.eventMu.Unlock()

	// OG requires a non-empty owner_id. Synthesize a system owner for
	// ownerless runtime events (e.g. health/degraded events from
	// Runtime.SetHealth).
	if rec.OwnerID == "" {
		rec.OwnerID = "__system__"
	}

	// Compute the next sequence number for this run from OG.
	existing, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", rec.RunID, 10000)
	if err != nil {
		return fmt.Errorf("append event: list existing: %w", err)
	}
	maxSeq := int64(0)
	for _, obj := range existing {
		var ev types.EventRecord
		if err := ogDecode(obj, &ev); err != nil {
			continue
		}
		if ev.OwnerID != rec.OwnerID {
			continue
		}
		if ev.Seq > maxSeq {
			maxSeq = ev.Seq
		}
	}
	rec.Seq = maxSeq + 1

	// Compute stream_seq from OG: take the max of all OG events, then +1.
	allEvents, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:  ogKindEvent,
		Limit: 100000,
	})
	if err != nil {
		return fmt.Errorf("append event: list all events: %w", err)
	}
	maxStreamSeq := int64(0)
	for _, obj := range allEvents {
		var ev types.EventRecord
		if err := ogDecode(obj, &ev); err != nil {
			continue
		}
		if ev.StreamSeq > maxStreamSeq {
			maxStreamSeq = ev.StreamSeq
		}
	}
	rec.StreamSeq = maxStreamSeq + 1

	return s.AppendEventOG(ctx, rec)
}

// ListEvents returns events for the given run, ordered by sequence ascending.
func (s *Store) ListEvents(ctx context.Context, runID string, limit int) ([]types.EventRecord, error) {
	return s.ListEventsAfter(ctx, runID, 0, limit)
}

// ListEventsAfter returns events for the given run with sequence > afterSeq,
// ordered by sequence ascending, limited to the given count.
func (s *Store) ListEventsAfter(ctx context.Context, runID string, afterSeq int64, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	// Fetch a large window since ogListByMetadata orders by updated_at
	// DESC, not by seq. We need all events to filter by seq and sort.
	objs, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", runID, 100000)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	events := make([]types.EventRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.Seq <= afterSeq {
			continue
		}
		events = append(events, rec)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Seq < events[j].Seq })
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

// ListEventsByOwner returns events for the given owner, ordered by timestamp
// descending, limited to the given count.
func (s *Store) ListEventsByOwner(ctx context.Context, ownerID string, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindEvent,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("query events by owner: %w", err)
	}
	events := make([]types.EventRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp.After(events[j].Timestamp) })
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

// ListEventsByOwnerAfter returns events for the given owner with stream_seq >
// afterSeq across all runs, ordered by stream_seq ascending, limited to the
// given count. This supports SSE catch-up after reconnection where the client
// needs events newer than a previously seen sequence number.
func (s *Store) ListEventsByOwnerAfter(ctx context.Context, ownerID string, afterSeq int64, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	// Fetch a large window since ListObjects orders by updated_at DESC,
	// not by stream_seq. We need all events to filter by stream_seq and
	// then apply the caller's limit.
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindEvent,
		OwnerID: ownerID,
		Limit:   100000,
	})
	if err != nil {
		return nil, fmt.Errorf("query events by owner after seq: %w", err)
	}
	events := make([]types.EventRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.StreamSeq <= afterSeq {
			continue
		}
		events = append(events, rec)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].StreamSeq < events[j].StreamSeq })
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

// ListEventsByChannel returns recent events for the given coordination channel.
func (s *Store) ListEventsByChannel(ctx context.Context, ownerID, channelID string, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	// Fetch a large window since ogListByMetadata orders by updated_at DESC.
	objs, err := s.ogListByMetadata(ctx, ogKindEvent, "channel_id", channelID, 10000)
	if err != nil {
		return nil, fmt.Errorf("query events by channel: %w", err)
	}
	var events []types.EventRecord
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		events = append(events, rec)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp.Before(events[j].Timestamp) })
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

// ListEventsByTrajectory returns recent events for a specific user trajectory.
func (s *Store) ListEventsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	// Fetch a large window since ogListByMetadata orders by updated_at DESC.
	objs, err := s.ogListByMetadata(ctx, ogKindEvent, "trajectory_id", trajectoryID, 10000)
	if err != nil {
		return nil, fmt.Errorf("query events by trajectory: %w", err)
	}
	var events []types.EventRecord
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		events = append(events, rec)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].StreamSeq < events[j].StreamSeq })
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

// ListEventsByTrajectoryAfter returns trajectory-scoped events newer than the
// provided stream sequence, ordered by stream_seq ascending.
func (s *Store) ListEventsByTrajectoryAfter(ctx context.Context, ownerID, trajectoryID string, afterSeq int64, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	// Fetch a large window since ogListByMetadata orders by updated_at DESC,
	// not by stream_seq. We need all matching records to filter by afterSeq
	// and sort by stream_seq before applying the caller's limit.
	objs, err := s.ogListByMetadata(ctx, ogKindEvent, "trajectory_id", trajectoryID, 10000)
	if err != nil {
		return nil, fmt.Errorf("query events by trajectory after seq: %w", err)
	}
	var events []types.EventRecord
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if rec.StreamSeq <= afterSeq {
			continue
		}
		events = append(events, rec)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].StreamSeq < events[j].StreamSeq })
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

// AppendChannelMessage persists a message to a coordination channel and assigns the next cursor sequence.
func (s *Store) AppendChannelMessage(ctx context.Context, message *types.ChannelMessage, ownerID string) error {
	// Serialize channel message sequence allocation to prevent two
	// concurrent sends from reading the same maxSeq and both assigning
	// the same sequence number.
	s.channelMsgMu.Lock()
	defer s.channelMsgMu.Unlock()

	// Compute the next sequence number from OG. Sequences are global
	// per channel (not per-owner) to match the old SQL `MAX(seq) WHERE
	// channel_id = ?` semantics.
	existing, err := s.ogListByMetadata(ctx, ogKindChannelMsg, "channel_id", message.ChannelID, 10000)
	if err != nil {
		return fmt.Errorf("append channel message: list existing: %w", err)
	}
	maxSeq := int64(0)
	for _, obj := range existing {
		var msg types.ChannelMessage
		if err := ogDecode(obj, &msg); err != nil {
			continue
		}
		if msg.Seq > maxSeq {
			maxSeq = msg.Seq
		}
	}
	message.Seq = maxSeq + 1
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}
	return s.AppendChannelMessageOG(ctx, message, ownerID)
}

// ListChannelMessages returns channel messages after the provided cursor, ordered by sequence ascending.
func (s *Store) ListChannelMessages(ctx context.Context, ownerID, channelID string, afterSeq int64, limit int) ([]types.ChannelMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	msgs, err := s.ListChannelMessagesOG(ctx, ownerID, channelID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("query channel messages: %w", err)
	}
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].Seq < msgs[j].Seq })
	return msgs, nil
}

// ListChannelMessagesByTrajectory returns durable channel messages for a
// specific trajectory, ordered by channel sequence ascending.
func (s *Store) ListChannelMessagesByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.ChannelMessage, error) {
	if limit <= 0 {
		limit = 500
	}
	// Fetch a large window since ogListByMetadata orders by updated_at
	// DESC, not by seq/timestamp. We need all matching messages to sort
	// by timestamp/seq and then apply the caller's limit.
	objs, err := s.ogListByMetadata(ctx, ogKindChannelMsg, "trajectory_id", trajectoryID, 100000)
	if err != nil {
		return nil, fmt.Errorf("query channel messages by trajectory: %w", err)
	}
	msgs := make([]types.ChannelMessage, 0, len(objs))
	for _, obj := range objs {
		if obj.OwnerID != ownerID {
			continue
		}
		var msg types.ChannelMessage
		if err := ogDecode(obj, &msg); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	sort.Slice(msgs, func(i, j int) bool {
		if !msgs[i].Timestamp.Equal(msgs[j].Timestamp) {
			return msgs[i].Timestamp.Before(msgs[j].Timestamp)
		}
		return msgs[i].Seq < msgs[j].Seq
	})
	if len(msgs) > limit {
		msgs = msgs[:limit]
	}
	return msgs, nil
}

// GetWorkerUpdate returns a previously dispatched structured worker update.
func (s *Store) GetWorkerUpdate(ctx context.Context, ownerID, updateID string) (types.CoagentSourcePacket, error) {
	return s.GetWorkerUpdateOG(ctx, ownerID, updateID)
}

// BindWorkerUpdateTerminalOutcome attaches immutable terminal outcome identity
// to an existing update without racing delivery marking. RunRecord remains the
// outcome authority; the digest is only a witness that later consumers must
// recompute from that record.
func (s *Store) BindWorkerUpdateTerminalOutcome(ctx context.Context, ownerID, updateID, sourceRunID, outcomeSHA256 string) (types.CoagentSourcePacket, error) {
	ownerID = strings.TrimSpace(ownerID)
	updateID = strings.TrimSpace(updateID)
	sourceRunID = strings.TrimSpace(sourceRunID)
	outcomeSHA256 = strings.TrimSpace(outcomeSHA256)
	if ownerID == "" || updateID == "" || sourceRunID == "" || outcomeSHA256 == "" {
		return types.CoagentSourcePacket{}, fmt.Errorf("bind worker update terminal outcome: owner_id, update_id, source_run_id, and source_outcome_sha256 are required")
	}

	s.workerUpdateMu.Lock()
	defer s.workerUpdateMu.Unlock()

	rec, err := s.GetWorkerUpdateOG(ctx, ownerID, updateID)
	if err != nil {
		return types.CoagentSourcePacket{}, err
	}
	if rec.SourceRunID != "" && rec.SourceRunID != sourceRunID {
		return types.CoagentSourcePacket{}, fmt.Errorf("bind worker update terminal outcome: source_run_id mismatch: %w", ErrConcurrentStateChange)
	}
	if rec.SourceOutcomeSHA256 != "" && rec.SourceOutcomeSHA256 != outcomeSHA256 {
		return types.CoagentSourcePacket{}, fmt.Errorf("bind worker update terminal outcome: source_outcome_sha256 mismatch: %w", ErrConcurrentStateChange)
	}
	if rec.SourceRunID == sourceRunID && rec.SourceOutcomeSHA256 == outcomeSHA256 {
		return rec, nil
	}
	rec.SourceRunID = sourceRunID
	rec.SourceOutcomeSHA256 = outcomeSHA256
	if err := s.CreateWorkerUpdateOG(ctx, rec); err != nil {
		return types.CoagentSourcePacket{}, fmt.Errorf("bind worker update terminal outcome: %w", err)
	}
	return rec, nil
}

// ListWorkerUpdatesBySourceRun exhausts terminal-bound updates for one run.
func (s *Store) ListWorkerUpdatesBySourceRun(ctx context.Context, ownerID, sourceRunID string) ([]types.CoagentSourcePacket, error) {
	return s.ListWorkerUpdatesBySourceRunOG(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(sourceRunID))
}

// ListWorkerUpdatesByTrajectory returns structured worker updates for one
// trajectory ordered by creation time ascending.
func (s *Store) ListWorkerUpdatesByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 500
	}
	return s.ListWorkerUpdatesByTrajectoryOG(ctx, ownerID, trajectoryID, limit)
}

// ListPendingWorkerUpdates returns undelivered update_coagent records for one
// target actor. These records are the durable wake backlog; channel_messages is
// only the audit/replay surface.
func (s *Store) ListPendingWorkerUpdates(ctx context.Context, ownerID, targetAgentID string, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 200
	}
	return s.ListPendingWorkerUpdatesOG(ctx, ownerID, targetAgentID, limit)
}

// ListPendingWorkerUpdatesAll returns undelivered update_coagent records across
// targets. Runtime boot sweep uses this as the cold-actor backlog oracle.
func (s *Store) ListPendingWorkerUpdatesAll(ctx context.Context, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 500
	}
	// Fetch a large window since ListObjects orders by updated_at DESC.
	// We need all pending records to sort by created_at and then apply
	// the caller's limit in memory. Using a small limit can miss older
	// undelivered records when many delivered records are more recently
	// updated.
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:  objectgraph.ObjectKind("choir.worker_update"),
		Limit: 100000,
	})
	if err != nil {
		return nil, fmt.Errorf("query pending worker updates: %w", err)
	}
	var updates []types.CoagentSourcePacket
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.LifecycleVersion > 0 {
			if rec.Disposition != types.UpdatePending {
				continue
			}
		} else if rec.DeliveredToRunID != "" {
			continue
		}
		updates = append(updates, rec)
	}
	sort.Slice(updates, func(i, j int) bool {
		if !updates[i].CreatedAt.Equal(updates[j].CreatedAt) {
			return updates[i].CreatedAt.Before(updates[j].CreatedAt)
		}
		return updates[i].UpdateID < updates[j].UpdateID
	})
	if len(updates) > limit {
		updates = updates[:limit]
	}
	return updates, nil
}

// ListCoagentMailboxBacklog returns update_coagent records after the durable
// actor mailbox cursor. delivered_at is audit compatibility; the contiguous
// cursor is the actor-facing processing boundary.
func (s *Store) ListCoagentMailboxBacklog(ctx context.Context, ownerID, targetAgentID string, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 200
	}
	ownerID = strings.TrimSpace(ownerID)
	targetAgentID = strings.TrimSpace(targetAgentID)
	if ownerID == "" || targetAgentID == "" {
		return nil, fmt.Errorf("list coagent mailbox backlog: owner_id and target_agent_id are required")
	}
	cursor, _, _, cursorErr := s.GetCoagentMailboxCursor(ctx, ownerID, targetAgentID)
	if cursorErr != nil {
		return nil, fmt.Errorf("list coagent mailbox backlog: read cursor: %w", cursorErr)
	}
	// Fetch a large window since ogListByMetadata orders by updated_at DESC,
	// not by message_seq. We need all matching records to sort by message_seq
	// and then apply the caller's limit in memory.
	objs, err := s.ogListByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "target_agent_id", targetAgentID, 10000)
	if err != nil {
		return nil, fmt.Errorf("query coagent mailbox backlog: %w", err)
	}
	var updates []types.CoagentSourcePacket
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if rec.LifecycleVersion > 0 {
			if rec.Disposition != types.UpdatePending {
				continue
			}
		} else if rec.DeliveredToRunID != "" && rec.MessageSeq <= cursor {
			continue
		}
		updates = append(updates, rec)
	}
	sort.Slice(updates, func(i, j int) bool {
		if updates[i].MessageSeq != updates[j].MessageSeq {
			return updates[i].MessageSeq < updates[j].MessageSeq
		}
		if !updates[i].CreatedAt.Equal(updates[j].CreatedAt) {
			return updates[i].CreatedAt.Before(updates[j].CreatedAt)
		}
		return updates[i].UpdateID < updates[j].UpdateID
	})
	if len(updates) > limit {
		updates = updates[:limit]
	}
	return updates, nil
}

// ListCoagentMailboxBacklogAll returns actor mailbox backlog rows across
// targets for boot-time re-warm sweeps.
func (s *Store) ListCoagentMailboxBacklogAll(ctx context.Context, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 500
	}
	// Fetch a large window since ListObjects orders by updated_at DESC.
	// We need all records to filter by cursor/pending state before
	// applying the caller's limit. Using a small limit can miss older
	// undelivered updates when many delivered records are more recently
	// updated.
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:  objectgraph.ObjectKind("choir.worker_update"),
		Limit: 100000,
	})
	if err != nil {
		return nil, fmt.Errorf("query coagent mailbox backlog all: %w", err)
	}
	var updates []types.CoagentSourcePacket
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.LifecycleVersion > 0 {
			if rec.Disposition != types.UpdatePending {
				continue
			}
		} else {
			cursor, _, _, cursorErr := s.GetCoagentMailboxCursor(ctx, rec.OwnerID, rec.TargetAgentID)
			if cursorErr != nil {
				return nil, fmt.Errorf("query coagent mailbox backlog all: read cursor for %s/%s: %w", rec.OwnerID, rec.TargetAgentID, cursorErr)
			}
			if rec.DeliveredToRunID != "" && rec.MessageSeq <= cursor {
				continue
			}
		}
		updates = append(updates, rec)
	}
	sort.Slice(updates, func(i, j int) bool {
		if updates[i].OwnerID != updates[j].OwnerID {
			return updates[i].OwnerID < updates[j].OwnerID
		}
		if updates[i].TargetAgentID != updates[j].TargetAgentID {
			return updates[i].TargetAgentID < updates[j].TargetAgentID
		}
		if updates[i].MessageSeq != updates[j].MessageSeq {
			return updates[i].MessageSeq < updates[j].MessageSeq
		}
		if !updates[i].CreatedAt.Equal(updates[j].CreatedAt) {
			return updates[i].CreatedAt.Before(updates[j].CreatedAt)
		}
		return updates[i].UpdateID < updates[j].UpdateID
	})
	if len(updates) > limit {
		updates = updates[:limit]
	}
	return updates, nil
}

// CountPendingWorkerUpdatesByTrajectory returns undelivered updates for the
// silent-stall oracle.
func (s *Store) CountPendingWorkerUpdatesByTrajectory(ctx context.Context, ownerID, trajectoryID string) (int, error) {
	objs, err := s.ogListByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "trajectory_id", trajectoryID, 10000)
	if err != nil {
		return 0, fmt.Errorf("count pending worker updates by trajectory: %w", err)
	}
	count := 0
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return 0, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if rec.LifecycleVersion > 0 {
			if rec.Disposition == types.UpdatePending {
				count++
			}
		} else if rec.DeliveredToRunID == "" {
			count++
		}
	}
	return count, nil
}

// MarkWorkerUpdatesDelivered marks update_coagent records addressed to
// targetAgentID as consumed by the loop that woke for them.
func (s *Store) MarkWorkerUpdatesDelivered(ctx context.Context, ownerID, targetAgentID string, updateIDs []string, runID string) error {
	if len(updateIDs) == 0 {
		return nil
	}
	targetAgentID = strings.TrimSpace(targetAgentID)
	if targetAgentID == "" {
		return fmt.Errorf("mark worker updates delivered: target_agent_id is required")
	}
	// Serialize delivery marking to preserve the compare-and-set semantics
	// that the old SQL `WHERE delivered_at IS NULL AND delivered_to_loop_id
	// = ''` provided atomically. Without this lock, two concurrent
	// activations can both read DeliveredToRunID == "" and both upsert a
	// delivered copy.
	s.workerUpdateMu.Lock()
	defer s.workerUpdateMu.Unlock()
	now := time.Now().UTC()
	for _, id := range updateIDs {
		rec, err := s.GetWorkerUpdateOG(ctx, ownerID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return fmt.Errorf("mark worker updates delivered: get %s: %w", id, err)
		}
		if rec.TargetAgentID != targetAgentID {
			continue // not addressed to this agent
		}
		if rec.LifecycleVersion > 0 {
			continue
		}
		if rec.DeliveredToRunID != "" {
			continue // already delivered
		}
		rec.DeliveredToRunID = runID
		rec.DeliveredAt = &now
		if err := s.CreateWorkerUpdateOG(ctx, rec); err != nil {
			return fmt.Errorf("mark worker updates delivered: put %s: %w", id, err)
		}
	}
	// Refresh the persisted cursor after marking.
	return s.refreshCoagentMailboxCursorOG(ctx, ownerID, targetAgentID)
}

// GetCoagentMailboxCursor returns the durable contiguous processed cursor for
// one addressed actor mailbox. The cursor is persisted in the object graph
// and only refreshed when worker updates are marked delivered.
func (s *Store) GetCoagentMailboxCursor(ctx context.Context, ownerID, agentID string) (int64, string, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return 0, "", false, fmt.Errorf("get coagent mailbox cursor: owner_id and agent_id are required")
	}
	// Read the persisted cursor from OG. Search by agent_id and filter
	// by owner_id, since multiple owners can have the same agent_id.
	objs, err := s.ogListByMetadata(ctx, ogKindCoagentMail, "agent_id", agentID, 100)
	if err != nil {
		return 0, "", false, fmt.Errorf("get coagent mailbox cursor: %w", err)
	}
	for _, obj := range objs {
		if obj.OwnerID != ownerID {
			continue
		}
		var mb struct {
			AgentID      string `json:"agent_id"`
			ChannelID    string `json:"channel_id"`
			ProcessedSeq int64  `json:"processed_message_seq"`
		}
		if err := ogDecode(obj, &mb); err != nil {
			return 0, "", false, fmt.Errorf("get coagent mailbox cursor: %w", err)
		}
		return mb.ProcessedSeq, mb.ChannelID, true, nil
	}
	// No persisted cursor yet; compute initial value from worker updates.
	return s.computeAndPersistCoagentMailboxCursor(ctx, ownerID, agentID)
}

// computeAndPersistCoagentMailboxCursor computes the cursor from worker
// update delivery state and persists it to OG.
func (s *Store) computeAndPersistCoagentMailboxCursor(ctx context.Context, ownerID, agentID string) (int64, string, bool, error) {
	objs, err := s.ogListByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "target_agent_id", agentID, 10000)
	if err != nil {
		return 0, "", false, fmt.Errorf("compute coagent mailbox cursor: %w", err)
	}
	var maxSeq int64
	var minPendingSeq int64
	var channelID string
	hasAny := false
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return 0, "", false, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if rec.LifecycleVersion > 0 {
			continue
		}
		if rec.MessageSeq <= 0 {
			continue
		}
		hasAny = true
		if rec.MessageSeq > maxSeq {
			maxSeq = rec.MessageSeq
		}
		if rec.DeliveredToRunID == "" {
			if minPendingSeq == 0 || rec.MessageSeq < minPendingSeq {
				minPendingSeq = rec.MessageSeq
			}
		}
		if channelID == "" && rec.ChannelID != "" {
			channelID = rec.ChannelID
		}
	}
	if !hasAny {
		return 0, "", false, nil
	}
	cursor := maxSeq
	if minPendingSeq > 0 {
		cursor = minPendingSeq - 1
	}
	// Persist the cursor.
	mbRec := struct {
		AgentID      string `json:"agent_id"`
		ChannelID    string `json:"channel_id"`
		ProcessedSeq int64  `json:"processed_message_seq"`
	}{
		AgentID:      agentID,
		ChannelID:    channelID,
		ProcessedSeq: cursor,
	}
	metadata := map[string]any{
		"agent_id":              agentID,
		"channel_id":            channelID,
		"processed_message_seq": cursor,
	}
	if _, err := s.ogPut(ctx, ogKindCoagentMail, ownerID, agentID, mbRec, metadata, time.Now().UTC()); err != nil {
		return 0, "", false, fmt.Errorf("persist coagent mailbox cursor: %w", err)
	}
	return cursor, channelID, true, nil
}

// refreshCoagentMailboxCursorOG recomputes and persists the cursor after
// worker updates are marked delivered.
func (s *Store) refreshCoagentMailboxCursorOG(ctx context.Context, ownerID, agentID string) error {
	_, _, _, err := s.computeAndPersistCoagentMailboxCursor(ctx, ownerID, agentID)
	return err
}

// DispatchWorkerUpdate atomically persists a structured worker update with its
// addressed channel audit message. The worker_updates row is the durable wake
// backlog; the update_id is idempotent per owner, so retries can return the
// existing update without duplicating delivery.
func (s *Store) DispatchWorkerUpdate(ctx context.Context, update types.CoagentSourcePacket, message *types.ChannelMessage) (types.CoagentSourcePacket, bool, error) {
	// Serialize the entire dispatch path to preserve idempotency:
	if strings.TrimSpace(update.ComputerID) != "" && strings.TrimSpace(update.TrajectoryID) != "" {
		if _, err := s.GetLifecycleTrajectory(ctx, update.OwnerID, update.ComputerID, update.TrajectoryID); err == nil {
			return types.CoagentSourcePacket{}, false, ErrLifecycleAuthorityRequired
		} else if !errors.Is(err, ErrNotFound) {
			return types.CoagentSourcePacket{}, false, err
		}
	}
	// the dedupe check and the channel message sequence allocation
	// must be in the same critical section so two concurrent retries
	// can't both pass the dedupe check and both append messages.
	s.channelMsgMu.Lock()
	defer s.channelMsgMu.Unlock()

	// Check for an existing worker update in OG (dedup).
	existing, err := s.GetWorkerUpdateOG(ctx, update.OwnerID, update.UpdateID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return types.CoagentSourcePacket{}, false, err
	}

	// Compute the next sequence number from OG. Sequences are global
	// per channel (not per-owner) to match the old SQL semantics.
	existingMsgs, err := s.ogListByMetadata(ctx, ogKindChannelMsg, "channel_id", message.ChannelID, 10000)
	if err != nil {
		return types.CoagentSourcePacket{}, false, fmt.Errorf("dispatch worker update: list existing messages: %w", err)
	}
	maxSeq := int64(0)
	for _, obj := range existingMsgs {
		var msg types.ChannelMessage
		if err := ogDecode(obj, &msg); err != nil {
			continue
		}
		if msg.Seq > maxSeq {
			maxSeq = msg.Seq
		}
	}
	if update.MessageSeq > 0 {
		message.Seq = update.MessageSeq
	} else {
		message.Seq = maxSeq + 1
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}
	update.MessageSeq = message.Seq
	if update.CreatedAt.IsZero() {
		update.CreatedAt = message.Timestamp
	}

	// Store the channel message in OG.
	if err := s.AppendChannelMessageOG(ctx, message, update.OwnerID); err != nil {
		return types.CoagentSourcePacket{}, false, fmt.Errorf("dispatch worker update: store channel message: %w", err)
	}

	// Store the worker update in OG. If this fails, compensate by
	// deleting the channel message we just wrote so a retry doesn't
	// end up with a duplicate audit message.
	if err := s.CreateWorkerUpdateOG(ctx, update); err != nil {
		// Best-effort compensating delete of the channel message.
		// List all messages on the channel and find the one with the
		// exact seq we just appended, since ogGetByKey may return a
		// different message on the same channel.
		objs, delErr := s.ogListByMetadata(ctx, ogKindChannelMsg, "channel_id", message.ChannelID, 10000)
		if delErr == nil {
			for _, obj := range objs {
				var existing types.ChannelMessage
				if ogDecode(obj, &existing) == nil && existing.Seq == message.Seq && existing.ChannelID == message.ChannelID {
					_ = s.ogDelete(ctx, obj.CanonicalID)
					break
				}
			}
		}
		return types.CoagentSourcePacket{}, false, fmt.Errorf("dispatch worker update: store worker update: %w", err)
	}
	return update, true, nil
}

// scanRun scans a run record from a single row.
func scanRun(row interface{ Scan(...any) error }) (types.RunRecord, error) {
	var rec types.RunRecord
	var createdAt, updatedAt string
	var finishedAt sql.NullString
	var metadataJSON string

	err := row.Scan(
		&rec.RunID,
		&rec.AgentID,
		&rec.ChannelID,
		&rec.RequestedByRunID,
		&rec.TrajectoryID,
		&rec.AgentProfile,
		&rec.AgentRole,
		&rec.OwnerID,
		&rec.SandboxID,
		&rec.State,
		&rec.Prompt,
		&rec.Result,
		&rec.Error,
		&createdAt,
		&updatedAt,
		&finishedAt,
		&metadataJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.RunRecord{}, ErrNotFound
		}
		return types.RunRecord{}, fmt.Errorf("scan run: %w", err)
	}

	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.RunRecord{}, fmt.Errorf("parse created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.RunRecord{}, fmt.Errorf("parse updated_at: %w", err)
	}
	if finishedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, finishedAt.String)
		if err != nil {
			return types.RunRecord{}, fmt.Errorf("parse finished_at: %w", err)
		}
		rec.FinishedAt = &t
	}

	if metadataJSON != "" && metadataJSON != "{}" {
		if err := json.Unmarshal([]byte(metadataJSON), &rec.Metadata); err != nil {
			return types.RunRecord{}, fmt.Errorf("parse metadata: %w", err)
		}
	}

	return rec, nil
}

// scanAgent scans an agent record from a single row.
func scanAgent(row interface{ Scan(...any) error }) (types.AgentRecord, error) {
	var rec types.AgentRecord
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.AgentID,
		&rec.OwnerID,
		&rec.SandboxID,
		&rec.Profile,
		&rec.Role,
		&rec.ChannelID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.AgentRecord{}, ErrNotFound
		}
		return types.AgentRecord{}, fmt.Errorf("scan agent: %w", err)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.AgentRecord{}, fmt.Errorf("parse agent created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.AgentRecord{}, fmt.Errorf("parse agent updated_at: %w", err)
	}
	return rec, nil
}

// scanEvent scans an event record from a single row.
func scanEvent(row interface{ Scan(...any) error }) (types.EventRecord, error) {
	var rec types.EventRecord
	var ts string
	var payloadJSON string

	err := row.Scan(
		&rec.EventID,
		&rec.RunID,
		&rec.AgentID,
		&rec.ChannelID,
		&rec.OwnerID,
		&rec.TrajectoryID,
		&rec.Seq,
		&rec.StreamSeq,
		&ts,
		&rec.Kind,
		&rec.Phase,
		&payloadJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.EventRecord{}, ErrNotFound
		}
		return types.EventRecord{}, fmt.Errorf("scan event: %w", err)
	}

	rec.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return types.EventRecord{}, fmt.Errorf("parse timestamp: %w", err)
	}
	rec.Payload = json.RawMessage(payloadJSON)

	return rec, nil
}

func scanChannelMessage(row interface{ Scan(...any) error }) (types.ChannelMessage, error) {
	var msg types.ChannelMessage
	var createdAt string
	err := row.Scan(
		&msg.ChannelID,
		&msg.Seq,
		&msg.FromAgentID,
		&msg.FromRunID,
		&msg.ToAgentID,
		&msg.ToRunID,
		&msg.TrajectoryID,
		&msg.From,
		&msg.Role,
		&msg.Content,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.ChannelMessage{}, ErrNotFound
		}
		return types.ChannelMessage{}, fmt.Errorf("scan channel message: %w", err)
	}
	msg.Timestamp, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.ChannelMessage{}, fmt.Errorf("parse channel message timestamp: %w", err)
	}
	return msg, nil
}

func scanInboxDelivery(row interface{ Scan(...any) error }) (types.InboxDelivery, error) {
	var rec types.InboxDelivery
	var createdAt string
	var deliveredAt sql.NullString
	err := row.Scan(
		&rec.DeliveryID,
		&rec.OwnerID,
		&rec.ToAgentID,
		&rec.ToRunID,
		&rec.FromAgentID,
		&rec.FromRunID,
		&rec.ChannelID,
		&rec.Role,
		&rec.Content,
		&rec.TrajectoryID,
		&createdAt,
		&rec.DeliveredToLoopID,
		&deliveredAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.InboxDelivery{}, ErrNotFound
		}
		return types.InboxDelivery{}, fmt.Errorf("scan inbox delivery: %w", err)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.InboxDelivery{}, fmt.Errorf("parse inbox delivery created_at: %w", err)
	}
	if deliveredAt.Valid {
		ts, err := time.Parse(time.RFC3339Nano, deliveredAt.String)
		if err != nil {
			return types.InboxDelivery{}, fmt.Errorf("parse inbox delivery delivered_at: %w", err)
		}
		rec.DeliveredAt = &ts
	}
	return rec, nil
}

func scanWorkerUpdate(row interface{ Scan(...any) error }) (types.CoagentSourcePacket, error) {
	var (
		rec         types.CoagentSourcePacket
		kind        string
		summary     string
		packetJSON  string
		createdAt   string
		deliveredAt sql.NullString
	)
	err := row.Scan(
		&rec.OwnerID,
		&rec.UpdateID,
		&rec.AgentID,
		&rec.TargetAgentID,
		&rec.ChannelID,
		&rec.MessageSeq,
		&rec.TrajectoryID,
		&rec.Role,
		&kind,
		&summary,
		&packetJSON,
		&rec.Content,
		&createdAt,
		&rec.DeliveredToRunID,
		&deliveredAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.CoagentSourcePacket{}, ErrNotFound
		}
		return types.CoagentSourcePacket{}, fmt.Errorf("scan worker update: %w", err)
	}
	if strings.TrimSpace(packetJSON) != "" && strings.TrimSpace(packetJSON) != "{}" {
		if err := json.Unmarshal([]byte(packetJSON), &rec.Packet); err != nil {
			return types.CoagentSourcePacket{}, fmt.Errorf("decode coagent source packet: %w", err)
		}
	}
	// Hard cutover (E3.1): a live worker_updates read with no canonical
	// packet_json is a legacy-shape row that must not be reconstructed into a
	// fake CoagentSourcePacket. The source-centric contract requires that
	// empty/invalid packet_json fail live delivery reads so legacy rows are
	// quarantined as audit-only historical data. Pre-cutover rows that still
	// carry only kind/summary are no longer deliverable as live coagent
	// packets; they remain readable as channel-message history. See
	// docs/mission-update-coagent-source-centric-deletion-v0.md §E2 Family A.
	if strings.TrimSpace(rec.Packet.SchemaVersion) == "" {
		return types.CoagentSourcePacket{}, fmt.Errorf("worker_update %s has no canonical packet_json; legacy-shape rows are not deliverable under the source-centric contract", rec.UpdateID)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.CoagentSourcePacket{}, fmt.Errorf("parse worker update created_at: %w", err)
	}
	if deliveredAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, deliveredAt.String)
		if err != nil {
			return types.CoagentSourcePacket{}, fmt.Errorf("parse worker update delivered_at: %w", err)
		}
		rec.DeliveredAt = &t
	}
	return rec, nil
}

func workerUpdateSelectSQL() string {
	return `SELECT owner_id, update_id, agent_id, target_agent_id, channel_id,
	       message_seq, trajectory_id, role, kind, summary, packet_json,
	       content, created_at, delivered_to_loop_id, delivered_at
	  FROM worker_updates`
}

func marshalStringSliceJSON(items []string) ([]byte, error) {
	if items == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(items)
}

// marshalJSON marshals a value to JSON, returning "{}" for nil.
func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(v)
}

// formatTimePtr formats a *time.Time for SQL storage, returning nil for nil.
func formatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// ----- Desktop state persistence (VAL-DESKTOP-007) -----

// GetDesktopState returns the persisted desktop state for the owner's primary
// desktop. If no state exists, it returns a default empty state with no error.
func (s *Store) GetDesktopState(ctx context.Context, ownerID string) (types.DesktopState, error) {
	return s.GetDesktopStateForDesktop(ctx, ownerID, types.PrimaryDesktopID)
}

// GetDesktopStateForDesktop returns the persisted desktop state for the given
// owner/desktop pair. If no state exists, it returns a default empty state.
func (s *Store) GetDesktopStateForDesktop(ctx context.Context, ownerID, desktopID string) (types.DesktopState, error) {
	return s.GetDesktopStateForSession(ctx, ownerID, desktopID, "")
}

// SaveDesktopState persists the desktop state for the given owner's primary
// desktop. It uses UPSERT so that both initial save and subsequent updates work.
func (s *Store) SaveDesktopState(ctx context.Context, state types.DesktopState) error {
	return s.SaveDesktopStateForDesktop(ctx, state)
}

// SaveDesktopStateForDesktop persists the desktop state for the given
// owner/desktop pair using UPSERT.
func (s *Store) SaveDesktopStateForDesktop(ctx context.Context, state types.DesktopState) error {
	return s.SaveDesktopStateForSession(ctx, state, types.DesktopSessionContext{SessionID: "legacy", IsDriver: true})
}
