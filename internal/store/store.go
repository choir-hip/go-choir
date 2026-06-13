// Package store provides durable runtime storage for the go-choir sandbox runtime.
//
// The store persists run records, agent records, channel messages, and event
// records using the same embedded Dolt workspace that owns VText state, enabling
// stable run IDs, durable agent/channel identity, and restart-safe recovery
// (VAL-RUNTIME-003, VAL-RUNTIME-010).
//
// Design decisions:
//   - One embedded Dolt workspace per user computer owns both runtime/control
//     state and VText/app state.
//   - Legacy SQLite runtime files are imported into Dolt and left in place as
//     rollback inputs during cutover.
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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("record not found")

// ErrStaleDocumentHead is returned when a caller tries to create a revision
// against an older parent while the document head has already moved on.
var ErrStaleDocumentHead = errors.New("stale document head")

func sanitizeStoreText(value string) string {
	return strings.ToValidUTF8(value, "\uFFFD")
}

// Store wraps the embedded Dolt connection and provides persistence for run
// records, agent records, channel messages, event records, and VText state.
type doltConnector interface {
	driver.Connector
	Close() error
}

type Store struct {
	db            *sql.DB
	readDB        *sql.DB
	path          string
	vtextDB       *sql.DB
	vtextPath     string
	doltConnector doltConnector
	jsonPatchMu   sync.Mutex
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
	parent_loop_id VARCHAR(255) NOT NULL DEFAULT '',
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

CREATE TABLE IF NOT EXISTS computer_source_lineages (
	owner_id             VARCHAR(255) NOT NULL DEFAULT '',
	computer_id          VARCHAR(255) NOT NULL DEFAULT '',
	computer_kind        VARCHAR(64) NOT NULL DEFAULT '',
	active_source_ref    LONGTEXT NOT NULL DEFAULT '',
	runtime_digest       VARCHAR(255) NOT NULL DEFAULT '',
	ui_digest            VARCHAR(255) NOT NULL DEFAULT '',
	route_profile        LONGTEXT NOT NULL DEFAULT '',
	default_base_profile LONGTEXT NOT NULL DEFAULT '',
	last_adoption_id     VARCHAR(255) NOT NULL DEFAULT '',
	last_package_id      VARCHAR(255) NOT NULL DEFAULT '',
	last_candidate_ref   LONGTEXT NOT NULL DEFAULT '',
	created_at           DATETIME NOT NULL,
	updated_at           DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id)
);

CREATE TABLE IF NOT EXISTS app_change_packages (
	package_id                    VARCHAR(255) PRIMARY KEY,
	owner_id                      VARCHAR(255) NOT NULL DEFAULT '',
	app_id                        VARCHAR(255) NOT NULL DEFAULT '',
	status                        VARCHAR(64) NOT NULL DEFAULT '',
	visibility                    VARCHAR(64) NOT NULL DEFAULT '',
	source_computer_id            VARCHAR(255) NOT NULL DEFAULT '',
	source_candidate_id           VARCHAR(255) NOT NULL DEFAULT '',
	source_active_ref             LONGTEXT NOT NULL DEFAULT '',
	candidate_source_ref          LONGTEXT NOT NULL DEFAULT '',
	runtime_source_delta          LONGTEXT NOT NULL DEFAULT '',
	ui_source_delta               LONGTEXT NOT NULL DEFAULT '',
	runtime_source_delta_sha256   VARCHAR(128) NOT NULL DEFAULT '',
	ui_source_delta_sha256        VARCHAR(128) NOT NULL DEFAULT '',
	package_manifest_sha256      VARCHAR(128) NOT NULL DEFAULT '',
	app_protocol_contract         LONGTEXT NOT NULL DEFAULT '',
	app_protocol_contract_sha256  VARCHAR(128) NOT NULL DEFAULT '',
	source_runtime_artifact_digest VARCHAR(255) NOT NULL DEFAULT '',
	source_ui_artifact_digest     VARCHAR(255) NOT NULL DEFAULT '',
	manifest_json                 LONGTEXT NOT NULL DEFAULT '{}',
	verifier_contracts_json       LONGTEXT NOT NULL DEFAULT '[]',
	provenance_refs_json          LONGTEXT NOT NULL DEFAULT '[]',
	trace_id                      VARCHAR(255) NOT NULL DEFAULT '',
	created_at                    DATETIME NOT NULL,
	updated_at                    DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS app_adoptions (
	adoption_id                              VARCHAR(255) PRIMARY KEY,
	owner_id                                 VARCHAR(255) NOT NULL DEFAULT '',
	package_id                               VARCHAR(255) NOT NULL DEFAULT '',
	app_id                                   VARCHAR(255) NOT NULL DEFAULT '',
	target_computer_id                       VARCHAR(255) NOT NULL DEFAULT '',
	target_computer_kind                     VARCHAR(64) NOT NULL DEFAULT '',
	target_candidate_id                      VARCHAR(255) NOT NULL DEFAULT '',
	status                                   VARCHAR(64) NOT NULL DEFAULT '',
	target_active_source_ref_at_candidate_start LONGTEXT NOT NULL DEFAULT '',
	target_active_source_ref_at_cutover      LONGTEXT NOT NULL DEFAULT '',
	candidate_source_ref                     LONGTEXT NOT NULL DEFAULT '',
	foreground_tail_merge_result            LONGTEXT NOT NULL DEFAULT '',
	merge_strategy                           VARCHAR(255) NOT NULL DEFAULT '',
	merge_conflicts_json                     LONGTEXT NOT NULL DEFAULT '[]',
	runtime_artifact_digest                  VARCHAR(255) NOT NULL DEFAULT '',
	ui_artifact_digest                       VARCHAR(255) NOT NULL DEFAULT '',
	verifier_results_json                    LONGTEXT NOT NULL DEFAULT '[]',
	rollback_profile_json                    LONGTEXT NOT NULL DEFAULT '{}',
	route_profile                            LONGTEXT NOT NULL DEFAULT '',
	default_base_profile                     LONGTEXT NOT NULL DEFAULT '',
	trace_id                                 VARCHAR(255) NOT NULL DEFAULT '',
	error                                    LONGTEXT NOT NULL DEFAULT '',
	created_at                              DATETIME NOT NULL,
	updated_at                              DATETIME NOT NULL
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

CREATE TABLE IF NOT EXISTS research_findings (
	owner_id          VARCHAR(255) NOT NULL DEFAULT '',
	finding_id        VARCHAR(255) NOT NULL DEFAULT '',
	agent_id          VARCHAR(255) NOT NULL DEFAULT '',
	target_agent_id   VARCHAR(255) NOT NULL DEFAULT '',
	channel_id        VARCHAR(255) NOT NULL DEFAULT '',
	message_seq       BIGINT NOT NULL DEFAULT 0,
	trajectory_id     VARCHAR(255) NOT NULL DEFAULT '',
	findings_json     LONGTEXT NOT NULL DEFAULT '[]',
	evidence_ids_json LONGTEXT NOT NULL DEFAULT '[]',
	notes_json        LONGTEXT NOT NULL DEFAULT '[]',
	questions_json    LONGTEXT NOT NULL DEFAULT '[]',
	content           LONGTEXT NOT NULL DEFAULT '',
	created_at        DATETIME NOT NULL,
	PRIMARY KEY (owner_id, finding_id)
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
	findings_json     LONGTEXT NOT NULL DEFAULT '[]',
	evidence_ids_json LONGTEXT NOT NULL DEFAULT '[]',
	artifacts_json    LONGTEXT NOT NULL DEFAULT '[]',
	refs_json         LONGTEXT NOT NULL DEFAULT '[]',
	tests_json        LONGTEXT NOT NULL DEFAULT '[]',
	questions_json    LONGTEXT NOT NULL DEFAULT '[]',
	proposals_json    LONGTEXT NOT NULL DEFAULT '[]',
	capability_requests_json LONGTEXT NOT NULL DEFAULT '[]',
	notes_json        LONGTEXT NOT NULL DEFAULT '[]',
	content           LONGTEXT NOT NULL DEFAULT '',
	created_at        DATETIME NOT NULL,
	delivered_to_loop_id VARCHAR(255) NOT NULL DEFAULT '',
	delivered_at      DATETIME,
	PRIMARY KEY (owner_id, update_id)
);

CREATE TABLE IF NOT EXISTS co_super_slots (
	owner_id       VARCHAR(255) NOT NULL DEFAULT '',
	trajectory_id  VARCHAR(255) NOT NULL DEFAULT '',
	slot           VARCHAR(64) NOT NULL DEFAULT '',
	run_id         VARCHAR(255) NOT NULL DEFAULT '',
	agent_id       VARCHAR(255) NOT NULL DEFAULT '',
	parent_loop_id VARCHAR(255) NOT NULL DEFAULT '',
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
CREATE INDEX IF NOT EXISTS idx_runs_channel_id ON runs(channel_id);
CREATE INDEX IF NOT EXISTS idx_runs_parent_loop_id ON runs(parent_loop_id);
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
CREATE INDEX IF NOT EXISTS idx_inbox_deliveries_owner_target ON inbox_deliveries(owner_id, to_agent_id, delivered_at);
CREATE INDEX IF NOT EXISTS idx_inbox_deliveries_created_at ON inbox_deliveries(created_at);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_loop_seq ON run_memory_entries(loop_id, seq);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_owner_loop_seq ON run_memory_entries(owner_id, loop_id, seq);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_parent ON run_memory_entries(parent_entry_id);
CREATE INDEX IF NOT EXISTS idx_computer_source_lineages_owner_kind ON computer_source_lineages(owner_id, computer_kind, updated_at);
CREATE INDEX IF NOT EXISTS idx_app_change_packages_owner_status ON app_change_packages(owner_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_app_change_packages_visibility ON app_change_packages(visibility, updated_at);
CREATE INDEX IF NOT EXISTS idx_app_change_packages_trace_id ON app_change_packages(trace_id);
CREATE INDEX IF NOT EXISTS idx_app_adoptions_owner_status ON app_adoptions(owner_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_app_adoptions_package ON app_adoptions(package_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_app_adoptions_target ON app_adoptions(owner_id, target_computer_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_app_adoptions_trace_id ON app_adoptions(trace_id);
CREATE INDEX IF NOT EXISTS idx_run_acceptances_owner_updated ON run_acceptances(owner_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_acceptances_owner_trajectory ON run_acceptances(owner_id, trajectory_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_acceptances_owner_loop ON run_acceptances(owner_id, loop_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_acceptances_target_mission ON run_acceptances(target_mission_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_continuations_owner_status ON run_continuations(owner_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_run_continuations_source_loop ON run_continuations(source_loop_id);
CREATE INDEX IF NOT EXISTS idx_run_continuations_next_loop ON run_continuations(next_loop_id);
CREATE INDEX IF NOT EXISTS idx_browser_sessions_owner_updated ON browser_sessions(owner_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_research_findings_channel_id ON research_findings(channel_id, created_at);
CREATE INDEX IF NOT EXISTS idx_research_findings_target_agent_id ON research_findings(target_agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_worker_updates_channel_id ON worker_updates(channel_id, created_at);
CREATE INDEX IF NOT EXISTS idx_worker_updates_target_agent_id ON worker_updates(target_agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_worker_updates_trajectory_id ON worker_updates(trajectory_id, created_at);
CREATE INDEX IF NOT EXISTS idx_worker_updates_pending_target ON worker_updates(owner_id, target_agent_id, delivered_at, created_at);
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

CREATE INDEX IF NOT EXISTS idx_desktop_workspaces_owner_id ON desktop_workspaces(owner_id);
CREATE INDEX IF NOT EXISTS idx_desktop_sessions_driver ON desktop_sessions(owner_id, desktop_id, driver_until);
CREATE INDEX IF NOT EXISTS idx_desktop_app_instances_stack ON desktop_app_instances(owner_id, desktop_id, shared_stack_rank);
CREATE INDEX IF NOT EXISTS idx_desktop_window_placements_instance ON desktop_window_placements(owner_id, desktop_id, app_instance_id, updated_at);
`

// Open opens (or creates) the unified embedded Dolt workspace derived from
// dbPath and applies the runtime and vtext schemas. If dbPath points at a
// legacy runtime SQLite database, its rows are imported into Dolt once and the
// SQLite file is left in place as a rollback source.
func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("runtime store: create directory: %w", err)
	}

	freshStore := false
	if _, err := os.Stat(dbPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("runtime store: stat marker or legacy sqlite: %w", err)
		}
		freshStore = true
		_ = os.RemoveAll(deriveVTextWorkspacePath(dbPath))
	}

	db, workspacePath, connector, err := openVTextWorkspaceDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("runtime store: open unified Dolt workspace: %w", err)
	}

	readDB := sql.OpenDB(connector)
	configureEmbeddedDoltDB(readDB)

	s := &Store{db: db, readDB: readDB, path: dbPath, vtextPath: workspacePath, doltConnector: connector}
	if err := s.bootstrap(); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("runtime store: bootstrap: %w", err)
	}

	// Apply the vtext schema to the embedded Dolt workspace.
	if err := s.EnsureVTextSchema(); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("runtime store: bootstrap vtext: %w", err)
	}

	if err := s.importLegacySQLiteRuntime(dbPath); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("runtime store: import legacy sqlite: %w", err)
	}

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
		{"runs", "parent_loop_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"runs", "agent_profile", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"runs", "agent_role", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"runs", "trajectory_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"events", "agent_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"events", "channel_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"events", "trajectory_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"events", "stream_seq", "BIGINT NOT NULL DEFAULT 0"},
		{"channel_messages", "to_agent_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"channel_messages", "to_loop_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"channel_messages", "trajectory_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
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
		{"worker_updates", "capability_requests_json", "LONGTEXT NOT NULL DEFAULT '[]'"},
		{"worker_updates", "delivered_to_loop_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"worker_updates", "delivered_at", "DATETIME"},
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
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_trajectory_stream_seq ON events(trajectory_id, stream_seq)`); err != nil {
		return fmt.Errorf("create idx_events_trajectory_stream_seq: %w", err)
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
	if db := s.vtextDB; db != nil {
		func() {
			defer func() {
				if r := recover(); r != nil && err == nil {
					err = fmt.Errorf("close vtext workspace: %v", r)
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

// VTextPath returns the filesystem path backing the embedded vtext workspace.
func (s *Store) VTextPath() string {
	return s.vtextPath
}

// UpsertAgent persists a durable agent record.
func (s *Store) UpsertAgent(ctx context.Context, rec types.AgentRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agents (agent_id, owner_id, sandbox_id, profile, role, channel_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   owner_id = VALUES(owner_id),
		   sandbox_id = VALUES(sandbox_id),
		   profile = VALUES(profile),
		   role = VALUES(role),
		   channel_id = VALUES(channel_id),
		   updated_at = VALUES(updated_at)`,
		rec.AgentID,
		rec.OwnerID,
		rec.SandboxID,
		rec.Profile,
		rec.Role,
		rec.ChannelID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert agent: %w", err)
	}
	return nil
}

// GetAgent returns the agent with the given ID.
func (s *Store) GetAgent(ctx context.Context, agentID string) (types.AgentRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT agent_id, owner_id, sandbox_id, profile, role, channel_id, created_at, updated_at
		   FROM agents
		  WHERE agent_id = ?`,
		agentID,
	)
	return scanAgent(row)
}

// CreateRun inserts a new run record.
func (s *Store) CreateRun(ctx context.Context, rec types.RunRecord) error {
	metadata, err := marshalJSON(rec.Metadata)
	if err != nil {
		return fmt.Errorf("marshal run metadata: %w", err)
	}
	prompt := sanitizeStoreText(rec.Prompt)
	result := sanitizeStoreText(rec.Result)
	runErr := sanitizeStoreText(rec.Error)

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO runs (loop_id, agent_id, channel_id, parent_loop_id, trajectory_id, agent_profile, agent_role, owner_id, sandbox_id, state, prompt, result, error, created_at, updated_at, finished_at, metadata_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.RunID,
		rec.AgentID,
		rec.ChannelID,
		rec.ParentRunID,
		rec.TrajectoryID,
		rec.AgentProfile,
		rec.AgentRole,
		rec.OwnerID,
		rec.SandboxID,
		rec.State,
		prompt,
		result,
		runErr,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		formatTimePtr(rec.FinishedAt),
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

// GetRun returns the run with the given run ID.
func (s *Store) GetRun(ctx context.Context, runID string) (types.RunRecord, error) {
	row := s.queryDB().QueryRowContext(ctx,
		`SELECT loop_id, agent_id, channel_id, parent_loop_id, trajectory_id, agent_profile, agent_role, owner_id, sandbox_id, state, prompt, result, error, created_at, updated_at, finished_at, metadata_json
		   FROM runs
		  WHERE loop_id = ?`,
		runID,
	)
	return scanRun(row)
}

// UpdateRun updates an existing run record.
func (s *Store) UpdateRun(ctx context.Context, rec types.RunRecord) error {
	metadata, err := marshalJSON(rec.Metadata)
	if err != nil {
		return fmt.Errorf("marshal run metadata: %w", err)
	}
	prompt := sanitizeStoreText(rec.Prompt)
	runResult := sanitizeStoreText(rec.Result)
	runErr := sanitizeStoreText(rec.Error)

	result, err := s.db.ExecContext(ctx,
		`UPDATE runs
		    SET agent_id = ?,
		        channel_id = ?,
		        parent_loop_id = ?,
		        trajectory_id = ?,
		        agent_profile = ?,
		        agent_role = ?,
		        owner_id = ?,
		        sandbox_id = ?,
		        state = ?,
		        prompt = ?,
		        result = ?,
		        error = ?,
		        updated_at = ?,
		        finished_at = ?,
		        metadata_json = ?
		  WHERE loop_id = ?`,
		rec.AgentID,
		rec.ChannelID,
		rec.ParentRunID,
		rec.TrajectoryID,
		rec.AgentProfile,
		rec.AgentRole,
		rec.OwnerID,
		rec.SandboxID,
		rec.State,
		prompt,
		runResult,
		runErr,
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		formatTimePtr(rec.FinishedAt),
		string(metadata),
		rec.RunID,
	)
	if err != nil {
		return fmt.Errorf("update run: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated run rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: run %s", ErrNotFound, rec.RunID)
	}
	return nil
}

// UpdateRunAndMarkWorkerUpdatesDelivered updates a run and marks its waking
// update_coagent records for the run's agent delivered in the same
// runtime-store transaction.
func (s *Store) UpdateRunAndMarkWorkerUpdatesDelivered(ctx context.Context, rec types.RunRecord, ownerID string, updateIDs []string) error {
	if len(updateIDs) == 0 {
		return s.UpdateRun(ctx, rec)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin run/update delivery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := updateRunInTx(ctx, tx, rec); err != nil {
		return err
	}
	if err := markWorkerUpdatesDeliveredWithExec(ctx, tx, ownerID, rec.AgentID, updateIDs, rec.RunID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit run/update delivery transaction: %w", err)
	}
	return nil
}

func updateRunInTx(ctx context.Context, tx *sql.Tx, rec types.RunRecord) error {
	metadata, err := marshalJSON(rec.Metadata)
	if err != nil {
		return fmt.Errorf("marshal run metadata: %w", err)
	}
	prompt := sanitizeStoreText(rec.Prompt)
	runResult := sanitizeStoreText(rec.Result)
	runErr := sanitizeStoreText(rec.Error)

	result, err := tx.ExecContext(ctx,
		`UPDATE runs
		    SET agent_id = ?,
		        channel_id = ?,
		        parent_loop_id = ?,
		        trajectory_id = ?,
		        agent_profile = ?,
		        agent_role = ?,
		        owner_id = ?,
		        sandbox_id = ?,
		        state = ?,
		        prompt = ?,
		        result = ?,
		        error = ?,
		        updated_at = ?,
		        finished_at = ?,
		        metadata_json = ?
		  WHERE loop_id = ?`,
		rec.AgentID,
		rec.ChannelID,
		rec.ParentRunID,
		rec.TrajectoryID,
		rec.AgentProfile,
		rec.AgentRole,
		rec.OwnerID,
		rec.SandboxID,
		rec.State,
		prompt,
		runResult,
		runErr,
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		formatTimePtr(rec.FinishedAt),
		string(metadata),
		rec.RunID,
	)
	if err != nil {
		return fmt.Errorf("update run: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated run rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: run %s", ErrNotFound, rec.RunID)
	}
	return nil
}

// ListRunsByOwner returns runs for the given owner, ordered by created_at
// descending, limited to the given count.
func (s *Store) ListRunsByOwner(ctx context.Context, ownerID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.listRunsWhere(ctx, "owner_id = ?", []any{ownerID}, limit)
}

// ListRunsByState returns runs in the given state, ordered by created_at
// descending, limited to the given count.
func (s *Store) ListRunsByState(ctx context.Context, state types.RunState, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.listRunsWhere(ctx, "state = ?", []any{string(state)}, limit)
}

// ListRuns returns recent runs ordered by created_at descending, limited
// to the given count.
func (s *Store) ListRuns(ctx context.Context, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.listRunsWhere(ctx, "", nil, limit)
}

// ListRunsByChannel returns runs for a specific coordination channel, ordered by creation time descending.
func (s *Store) ListRunsByChannel(ctx context.Context, ownerID, channelID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.listRunsWhere(ctx, "owner_id = ? AND channel_id = ?", []any{ownerID, channelID}, limit)
}

// ListActiveRunsByTrajectory returns pending/running/blocked activations on a
// trajectory. Trajectory cancellation uses this instead of parent_loop_id
// recursion so spawned_by provenance does not decide lifecycle control.
func (s *Store) ListActiveRunsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.RunRecord, error) {
	return s.ListActiveRunsByTrajectoryExcluding(ctx, ownerID, trajectoryID, nil, limit)
}

// ListActiveRunsByTrajectoryExcluding returns active trajectory activations
// excluding run IDs already handled by a caller-side drain loop.
func (s *Store) ListActiveRunsByTrajectoryExcluding(ctx context.Context, ownerID, trajectoryID string, excludeRunIDs []string, limit int) ([]types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if ownerID == "" || trajectoryID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	where := "owner_id = ? AND trajectory_id = ? AND state IN ('pending', 'running', 'blocked')"
	args := []any{ownerID, trajectoryID}
	if len(excludeRunIDs) > 0 {
		placeholders := make([]string, 0, len(excludeRunIDs))
		for _, runID := range excludeRunIDs {
			runID = strings.TrimSpace(runID)
			if runID == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, runID)
		}
		if len(placeholders) > 0 {
			where += " AND loop_id NOT IN (" + strings.Join(placeholders, ",") + ")"
		}
	}
	return s.listRunsWhere(ctx, where, args, limit)
}

// CountActiveChildRuns returns the number of non-terminal direct child runs
// for the given parent. It is used by runtime authority budgets before
// launching more child goroutines in constrained worker sandboxes.
func (s *Store) CountActiveChildRuns(ctx context.Context, parentRunID string) (int, error) {
	var count int
	row := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		   FROM runs
		  WHERE parent_loop_id = ?
		    AND state IN ('pending', 'running', 'blocked')`,
		parentRunID,
	)
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count active child runs: %w", err)
	}
	return count, nil
}

// ListActiveChildRuns returns non-terminal direct child runs for the given
// parent. Runtime tool guards use it to make constrained delegation idempotent
// before launching another child goroutine.
func (s *Store) ListActiveChildRuns(ctx context.Context, parentRunID string) ([]types.RunRecord, error) {
	return s.listRunsWhere(ctx,
		"parent_loop_id = ? AND state IN ('pending', 'running', 'blocked')",
		[]any{parentRunID},
		100,
	)
}

// ListChildRuns returns direct child runs for the given parent, including
// terminal children. Evidence collection uses this to avoid redoing or
// cancelling work after a child has already exported a candidate artifact.
func (s *Store) ListChildRuns(ctx context.Context, parentRunID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.listRunsWhere(ctx, "parent_loop_id = ?", []any{parentRunID}, limit)
}

// ClaimCoSuperSlot atomically claims (owner, trajectory, slot) for a co-super
// run. If a live run already owns the slot, that run is returned and claimed is
// false. If the previous owner is terminal, the slot is advanced to runID.
func (s *Store) ClaimCoSuperSlot(ctx context.Context, ownerID, trajectoryID, slot, runID, agentID, parentRunID string) (types.RunRecord, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	slot = strings.TrimSpace(slot)
	runID = strings.TrimSpace(runID)
	if ownerID == "" || trajectoryID == "" || slot == "" || runID == "" {
		return types.RunRecord{}, false, fmt.Errorf("claim co-super slot: owner_id, trajectory_id, slot, and run_id are required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO co_super_slots (owner_id, trajectory_id, slot, run_id, agent_id, parent_loop_id, claimed_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE run_id = run_id`,
		ownerID, trajectoryID, slot, runID, agentID, parentRunID, now, now,
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
		        parent_loop_id = ?,
		        claimed_at = ?,
		        updated_at = ?
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		    AND slot = ?
		    AND run_id = ?`,
		runID, agentID, parentRunID, now, now, ownerID, trajectoryID, slot, existingRunID,
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
// co-super activation. ParentRunID is retained as spawned-by provenance, not
// as the authority relation.
type CoSuperSlotRecord struct {
	OwnerID      string
	TrajectoryID string
	Slot         string
	RunID        string
	AgentID      string
	ParentRunID  string
	ClaimedAt    time.Time
	UpdatedAt    time.Time
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
		`SELECT owner_id, trajectory_id, slot, run_id, agent_id, parent_loop_id, claimed_at, updated_at
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
		&rec.ParentRunID,
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
		`SELECT owner_id, trajectory_id, slot, run_id, agent_id, parent_loop_id, claimed_at, updated_at
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
		&rec.ParentRunID,
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
	var count int
	row := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		   FROM co_super_slots s
		   JOIN runs r ON r.loop_id = s.run_id
		  WHERE s.owner_id = ?
		    AND s.trajectory_id = ?
		    AND r.owner_id = ?
		    AND r.state IN ('pending', 'running', 'blocked')`,
		ownerID,
		trajectoryID,
		ownerID,
	)
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count active co-super slots: %w", err)
	}
	return count, nil
}

// GetLatestActiveRunByAgent returns the most recent non-terminal run for an agent.
func (s *Store) GetLatestActiveRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT loop_id, agent_id, channel_id, parent_loop_id, trajectory_id, agent_profile, agent_role, owner_id, sandbox_id, state, prompt, result, error, created_at, updated_at, finished_at, metadata_json
		   FROM runs
		  WHERE owner_id = ?
		    AND agent_id = ?
		    AND state IN ('pending', 'running', 'blocked')
		  ORDER BY updated_at DESC
		  LIMIT 1`,
		ownerID,
		agentID,
	)
	return scanRun(row)
}

func (s *Store) listRunsWhere(ctx context.Context, where string, args []any, limit int) ([]types.RunRecord, error) {
	query := `SELECT loop_id, agent_id, channel_id, parent_loop_id, trajectory_id, agent_profile, agent_role, owner_id, sandbox_id, state, prompt, result, error, created_at, updated_at, finished_at, metadata_json
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Compute the next sequence number for this run.
	row := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM events WHERE loop_id = ?`,
		rec.RunID,
	)
	if err := row.Scan(&rec.Seq); err != nil {
		return fmt.Errorf("query next event sequence: %w", err)
	}
	row = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(stream_seq), 0) + 1 FROM events`,
	)
	if err := row.Scan(&rec.StreamSeq); err != nil {
		return fmt.Errorf("query next event stream sequence: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (event_id, loop_id, agent_id, channel_id, owner_id, trajectory_id, seq, stream_seq, ts, kind, phase, payload_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.EventID,
		rec.RunID,
		rec.AgentID,
		rec.ChannelID,
		rec.OwnerID,
		rec.TrajectoryID,
		rec.Seq,
		rec.StreamSeq,
		rec.Timestamp.UTC().Format(time.RFC3339Nano),
		rec.Kind,
		rec.Phase,
		string(rec.Payload),
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit event insert: %w", err)
	}

	return nil
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

	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, loop_id, agent_id, channel_id, owner_id, trajectory_id, seq, stream_seq, ts, kind, phase, payload_json
		   FROM events
		  WHERE loop_id = ?
		    AND seq > ?
		  ORDER BY seq ASC
		  LIMIT ?`,
		runID,
		afterSeq,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []types.EventRecord
	for rows.Next() {
		rec, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

// ListEventsByOwner returns events for the given owner, ordered by timestamp
// descending, limited to the given count.
func (s *Store) ListEventsByOwner(ctx context.Context, ownerID string, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 200
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, loop_id, agent_id, channel_id, owner_id, trajectory_id, seq, stream_seq, ts, kind, phase, payload_json
		   FROM events
		  WHERE owner_id = ?
		  ORDER BY ts DESC
		  LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events by owner: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []types.EventRecord
	for rows.Next() {
		rec, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events by owner: %w", err)
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

	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, loop_id, agent_id, channel_id, owner_id, trajectory_id, seq, stream_seq, ts, kind, phase, payload_json
		   FROM events
		  WHERE owner_id = ?
		    AND stream_seq > ?
		  ORDER BY stream_seq ASC
		  LIMIT ?`,
		ownerID,
		afterSeq,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events by owner after seq: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []types.EventRecord
	for rows.Next() {
		rec, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events by owner after seq: %w", err)
	}
	return events, nil
}

// ListEventsByChannel returns recent events for the given coordination channel.
func (s *Store) ListEventsByChannel(ctx context.Context, ownerID, channelID string, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, loop_id, agent_id, channel_id, owner_id, trajectory_id, seq, stream_seq, ts, kind, phase, payload_json
		   FROM events
		  WHERE owner_id = ?
		    AND channel_id = ?
		  ORDER BY ts ASC
		  LIMIT ?`,
		ownerID,
		channelID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events by channel: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []types.EventRecord
	for rows.Next() {
		rec, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events by channel: %w", err)
	}
	return events, nil
}

// ListEventsByTrajectory returns recent events for a specific user trajectory.
func (s *Store) ListEventsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, loop_id, agent_id, channel_id, owner_id, trajectory_id, seq, stream_seq, ts, kind, phase, payload_json
		   FROM events
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		  ORDER BY stream_seq ASC
		  LIMIT ?`,
		ownerID,
		trajectoryID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events by trajectory: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []types.EventRecord
	for rows.Next() {
		rec, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events by trajectory: %w", err)
	}
	return events, nil
}

// ListEventsByTrajectoryAfter returns trajectory-scoped events newer than the
// provided stream sequence, ordered by stream_seq ascending.
func (s *Store) ListEventsByTrajectoryAfter(ctx context.Context, ownerID, trajectoryID string, afterSeq int64, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, loop_id, agent_id, channel_id, owner_id, trajectory_id, seq, stream_seq, ts, kind, phase, payload_json
		   FROM events
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		    AND stream_seq > ?
		  ORDER BY stream_seq ASC
		  LIMIT ?`,
		ownerID,
		trajectoryID,
		afterSeq,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events by trajectory after seq: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []types.EventRecord
	for rows.Next() {
		rec, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events by trajectory after seq: %w", err)
	}
	return events, nil
}

// AppendChannelMessage persists a message to a coordination channel and assigns the next cursor sequence.
func (s *Store) AppendChannelMessage(ctx context.Context, message *types.ChannelMessage, ownerID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin channel message transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM channel_messages WHERE channel_id = ?`,
		message.ChannelID,
	)
	if err := row.Scan(&message.Seq); err != nil {
		return fmt.Errorf("query next channel message sequence: %w", err)
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO channel_messages (channel_id, seq, owner_id, from_agent_id, from_loop_id, to_agent_id, to_loop_id, trajectory_id, from_name, role, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ChannelID,
		message.Seq,
		ownerID,
		message.FromAgentID,
		message.FromRunID,
		message.ToAgentID,
		message.ToRunID,
		message.TrajectoryID,
		message.From,
		message.Role,
		message.Content,
		message.Timestamp.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert channel message: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit channel message: %w", err)
	}
	return nil
}

// ListChannelMessages returns channel messages after the provided cursor, ordered by sequence ascending.
func (s *Store) ListChannelMessages(ctx context.Context, ownerID, channelID string, afterSeq int64, limit int) ([]types.ChannelMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `SELECT channel_id, seq, from_agent_id, from_loop_id, to_agent_id, to_loop_id, trajectory_id, from_name, role, content, created_at
		   FROM channel_messages
		  WHERE channel_id = ?
		    AND seq > ?`
	args := []any{channelID, afterSeq}
	if strings.TrimSpace(ownerID) != "" {
		query += ` AND owner_id = ?`
		args = append(args, ownerID)
	}
	query += ` ORDER BY seq ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query channel messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []types.ChannelMessage
	for rows.Next() {
		msg, err := scanChannelMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel messages: %w", err)
	}
	return messages, nil
}

// ListChannelMessagesByTrajectory returns durable channel messages for a
// specific trajectory, ordered by channel sequence ascending.
func (s *Store) ListChannelMessagesByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.ChannelMessage, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT channel_id, seq, from_agent_id, from_loop_id, to_agent_id, to_loop_id, trajectory_id, from_name, role, content, created_at
		   FROM channel_messages
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		  ORDER BY created_at ASC, seq ASC
		  LIMIT ?`,
		ownerID,
		trajectoryID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query channel messages by trajectory: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []types.ChannelMessage
	for rows.Next() {
		msg, err := scanChannelMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel messages by trajectory: %w", err)
	}
	return messages, nil
}

// GetResearchFinding returns a previously dispatched researcher findings bundle.
func (s *Store) GetResearchFinding(ctx context.Context, ownerID, findingID string) (types.ResearchFindingRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT owner_id, finding_id, agent_id, target_agent_id, channel_id, message_seq, trajectory_id, findings_json, evidence_ids_json, notes_json, questions_json, content, created_at
		   FROM research_findings
		  WHERE owner_id = ? AND finding_id = ?`,
		ownerID, findingID,
	)
	return scanResearchFinding(row)
}

// ListResearchFindingsByTrajectory returns researcher dispatch bundles for one
// trajectory ordered by creation time ascending.
func (s *Store) ListResearchFindingsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.ResearchFindingRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT owner_id, finding_id, agent_id, target_agent_id, channel_id, message_seq, trajectory_id, findings_json, evidence_ids_json, notes_json, questions_json, content, created_at
		   FROM research_findings
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		  ORDER BY created_at ASC
		  LIMIT ?`,
		ownerID,
		trajectoryID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query research findings by trajectory: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var findings []types.ResearchFindingRecord
	for rows.Next() {
		rec, err := scanResearchFinding(rows)
		if err != nil {
			return nil, err
		}
		findings = append(findings, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate research findings by trajectory: %w", err)
	}
	return findings, nil
}

// GetWorkerUpdate returns a previously dispatched structured worker update.
func (s *Store) GetWorkerUpdate(ctx context.Context, ownerID, updateID string) (types.WorkerUpdateRecord, error) {
	row := s.db.QueryRowContext(ctx,
		workerUpdateSelectSQL()+` WHERE owner_id = ? AND update_id = ?`,
		ownerID, updateID,
	)
	return scanWorkerUpdate(row)
}

// ListWorkerUpdatesByTrajectory returns structured worker updates for one
// trajectory ordered by creation time ascending.
func (s *Store) ListWorkerUpdatesByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.WorkerUpdateRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		workerUpdateSelectSQL()+` WHERE owner_id = ?
		    AND trajectory_id = ?
		  ORDER BY created_at ASC
		  LIMIT ?`,
		ownerID,
		trajectoryID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query worker updates by trajectory: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var updates []types.WorkerUpdateRecord
	for rows.Next() {
		rec, err := scanWorkerUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worker updates by trajectory: %w", err)
	}
	return updates, nil
}

// ListPendingWorkerUpdates returns undelivered update_coagent records for one
// target actor. These records are the durable wake backlog; channel_messages is
// only the audit/replay surface.
func (s *Store) ListPendingWorkerUpdates(ctx context.Context, ownerID, targetAgentID string, limit int) ([]types.WorkerUpdateRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		workerUpdateSelectSQL()+` WHERE owner_id = ?
		    AND target_agent_id = ?
		    AND delivered_at IS NULL
		  ORDER BY created_at ASC, update_id ASC
		  LIMIT ?`,
		ownerID,
		targetAgentID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending worker updates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var updates []types.WorkerUpdateRecord
	for rows.Next() {
		rec, err := scanWorkerUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending worker updates: %w", err)
	}
	return updates, nil
}

// ListPendingWorkerUpdatesAll returns undelivered update_coagent records across
// targets. Runtime boot sweep uses this as the cold-actor backlog oracle.
func (s *Store) ListPendingWorkerUpdatesAll(ctx context.Context, limit int) ([]types.WorkerUpdateRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		workerUpdateSelectSQL()+` WHERE delivered_at IS NULL
		  ORDER BY created_at ASC, update_id ASC
		  LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending worker updates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var updates []types.WorkerUpdateRecord
	for rows.Next() {
		rec, err := scanWorkerUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending worker updates: %w", err)
	}
	return updates, nil
}

// CountPendingWorkerUpdatesByTrajectory returns undelivered updates for the
// silent-stall oracle.
func (s *Store) CountPendingWorkerUpdatesByTrajectory(ctx context.Context, ownerID, trajectoryID string) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		   FROM worker_updates
		  WHERE owner_id = ?
		    AND trajectory_id = ?
		    AND delivered_at IS NULL`,
		ownerID,
		trajectoryID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pending worker updates by trajectory: %w", err)
	}
	return count, nil
}

// MarkWorkerUpdatesDelivered marks update_coagent records addressed to
// targetAgentID as consumed by the loop that woke for them.
func (s *Store) MarkWorkerUpdatesDelivered(ctx context.Context, ownerID, targetAgentID string, updateIDs []string, runID string) error {
	if len(updateIDs) == 0 {
		return nil
	}
	return markWorkerUpdatesDeliveredWithExec(ctx, s.db, ownerID, targetAgentID, updateIDs, runID)
}

type workerUpdateDeliveryExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func markWorkerUpdatesDeliveredWithExec(ctx context.Context, exec workerUpdateDeliveryExecer, ownerID, targetAgentID string, updateIDs []string, runID string) error {
	targetAgentID = strings.TrimSpace(targetAgentID)
	if targetAgentID == "" {
		return fmt.Errorf("mark worker updates delivered: target_agent_id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(updateIDs)), ",")
	args := make([]any, 0, len(updateIDs)+4)
	args = append(args, runID, now, ownerID, targetAgentID)
	for _, id := range updateIDs {
		args = append(args, id)
	}
	query := fmt.Sprintf(
		`UPDATE worker_updates
		    SET delivered_to_loop_id = ?,
		        delivered_at = ?
		  WHERE owner_id = ?
		    AND target_agent_id = ?
		    AND update_id IN (%s)
		    AND delivered_at IS NULL
		    AND delivered_to_loop_id = ''`,
		placeholders,
	)
	if _, err := exec.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mark worker updates delivered: %w", err)
	}
	return nil
}

// DispatchResearchFinding atomically persists the addressed channel message,
// inbox delivery, and finding dispatch record inside the runtime store.
// Evidence durability remains in the vtext workspace and should be handled
// before calling this method with deterministic evidence IDs.
func (s *Store) DispatchResearchFinding(ctx context.Context, finding types.ResearchFindingRecord, message *types.ChannelMessage, delivery types.InboxDelivery) (types.ResearchFindingRecord, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("begin research finding transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := scanResearchFinding(tx.QueryRowContext(ctx,
		`SELECT owner_id, finding_id, agent_id, target_agent_id, channel_id, message_seq, trajectory_id, findings_json, evidence_ids_json, notes_json, questions_json, content, created_at
		   FROM research_findings
		  WHERE owner_id = ? AND finding_id = ?`,
		finding.OwnerID, finding.FindingID,
	))
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return types.ResearchFindingRecord{}, false, err
	}

	row := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM channel_messages WHERE channel_id = ?`,
		message.ChannelID,
	)
	if err := row.Scan(&message.Seq); err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("query next research finding message sequence: %w", err)
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = message.Timestamp
	}
	finding.MessageSeq = message.Seq
	if finding.CreatedAt.IsZero() {
		finding.CreatedAt = message.Timestamp
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO channel_messages (channel_id, seq, owner_id, from_agent_id, from_loop_id, to_agent_id, to_loop_id, trajectory_id, from_name, role, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ChannelID,
		message.Seq,
		finding.OwnerID,
		message.FromAgentID,
		message.FromRunID,
		message.ToAgentID,
		message.ToRunID,
		message.TrajectoryID,
		message.From,
		message.Role,
		message.Content,
		message.Timestamp.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("insert research finding channel message: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO inbox_deliveries (delivery_id, owner_id, to_agent_id, to_loop_id, from_agent_id, from_loop_id, channel_id, role, content, trajectory_id, created_at, delivered_to_loop_id, delivered_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		delivery.DeliveryID,
		delivery.OwnerID,
		delivery.ToAgentID,
		delivery.ToRunID,
		delivery.FromAgentID,
		delivery.FromRunID,
		delivery.ChannelID,
		delivery.Role,
		delivery.Content,
		delivery.TrajectoryID,
		delivery.CreatedAt.UTC().Format(time.RFC3339Nano),
		delivery.DeliveredToLoopID,
		formatTimePtr(delivery.DeliveredAt),
	)
	if err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("insert research finding inbox delivery: %w", err)
	}

	findingsJSON, err := marshalStringSliceJSON(finding.Findings)
	if err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("marshal research findings findings: %w", err)
	}
	evidenceIDsJSON, err := marshalStringSliceJSON(finding.EvidenceIDs)
	if err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("marshal research findings evidence ids: %w", err)
	}
	notesJSON, err := marshalStringSliceJSON(finding.Notes)
	if err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("marshal research findings notes: %w", err)
	}
	questionsJSON, err := marshalStringSliceJSON(finding.Questions)
	if err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("marshal research findings questions: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO research_findings (owner_id, finding_id, agent_id, target_agent_id, channel_id, message_seq, trajectory_id, findings_json, evidence_ids_json, notes_json, questions_json, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		finding.OwnerID,
		finding.FindingID,
		finding.AgentID,
		finding.TargetAgentID,
		finding.ChannelID,
		finding.MessageSeq,
		finding.TrajectoryID,
		string(findingsJSON),
		string(evidenceIDsJSON),
		string(notesJSON),
		string(questionsJSON),
		finding.Content,
		finding.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("insert research finding record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return types.ResearchFindingRecord{}, false, fmt.Errorf("commit research finding transaction: %w", err)
	}
	return finding, true, nil
}

// DispatchWorkerUpdate atomically persists a structured worker update with its
// addressed channel audit message. The worker_updates row is the durable wake
// backlog; the update_id is idempotent per owner, so retries can return the
// existing update without duplicating delivery.
func (s *Store) DispatchWorkerUpdate(ctx context.Context, update types.WorkerUpdateRecord, message *types.ChannelMessage) (types.WorkerUpdateRecord, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("begin worker update transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := scanWorkerUpdate(tx.QueryRowContext(ctx,
		workerUpdateSelectSQL()+` WHERE owner_id = ? AND update_id = ?`,
		update.OwnerID, update.UpdateID,
	))
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return types.WorkerUpdateRecord{}, false, err
	}

	row := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM channel_messages WHERE channel_id = ?`,
		message.ChannelID,
	)
	if err := row.Scan(&message.Seq); err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("query next worker update message sequence: %w", err)
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}
	update.MessageSeq = message.Seq
	if update.CreatedAt.IsZero() {
		update.CreatedAt = message.Timestamp
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO channel_messages (channel_id, seq, owner_id, from_agent_id, from_loop_id, to_agent_id, to_loop_id, trajectory_id, from_name, role, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ChannelID,
		message.Seq,
		update.OwnerID,
		message.FromAgentID,
		message.FromRunID,
		message.ToAgentID,
		message.ToRunID,
		message.TrajectoryID,
		message.From,
		message.Role,
		message.Content,
		message.Timestamp.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("insert worker update channel message: %w", err)
	}

	findingsJSON, err := marshalStringSliceJSON(update.Findings)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update findings: %w", err)
	}
	evidenceIDsJSON, err := marshalStringSliceJSON(update.EvidenceIDs)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update evidence ids: %w", err)
	}
	artifactsJSON, err := marshalStringSliceJSON(update.Artifacts)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update artifacts: %w", err)
	}
	refsJSON, err := marshalStringSliceJSON(update.Refs)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update refs: %w", err)
	}
	testsJSON, err := marshalStringSliceJSON(update.Tests)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update tests: %w", err)
	}
	questionsJSON, err := marshalStringSliceJSON(update.Questions)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update questions: %w", err)
	}
	proposalsJSON, err := marshalStringSliceJSON(update.Proposals)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update proposals: %w", err)
	}
	capabilityRequestsJSON, err := json.Marshal(update.CapabilityRequests)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update capability requests: %w", err)
	}
	notesJSON, err := marshalStringSliceJSON(update.Notes)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("marshal worker update notes: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO worker_updates (owner_id, update_id, agent_id, target_agent_id, channel_id, message_seq, trajectory_id, role, kind, summary, findings_json, evidence_ids_json, artifacts_json, refs_json, tests_json, questions_json, proposals_json, capability_requests_json, notes_json, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		update.OwnerID,
		update.UpdateID,
		update.AgentID,
		update.TargetAgentID,
		update.ChannelID,
		update.MessageSeq,
		update.TrajectoryID,
		update.Role,
		update.Kind,
		update.Summary,
		string(findingsJSON),
		string(evidenceIDsJSON),
		string(artifactsJSON),
		string(refsJSON),
		string(testsJSON),
		string(questionsJSON),
		string(proposalsJSON),
		string(capabilityRequestsJSON),
		string(notesJSON),
		update.Content,
		update.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("insert worker update record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return types.WorkerUpdateRecord{}, false, fmt.Errorf("commit worker update transaction: %w", err)
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
		&rec.ParentRunID,
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

func scanResearchFinding(row interface{ Scan(...any) error }) (types.ResearchFindingRecord, error) {
	var (
		rec             types.ResearchFindingRecord
		findingsJSON    string
		evidenceIDsJSON string
		notesJSON       string
		questionsJSON   string
		createdAt       string
	)
	err := row.Scan(
		&rec.OwnerID,
		&rec.FindingID,
		&rec.AgentID,
		&rec.TargetAgentID,
		&rec.ChannelID,
		&rec.MessageSeq,
		&rec.TrajectoryID,
		&findingsJSON,
		&evidenceIDsJSON,
		&notesJSON,
		&questionsJSON,
		&rec.Content,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.ResearchFindingRecord{}, ErrNotFound
		}
		return types.ResearchFindingRecord{}, fmt.Errorf("scan research finding: %w", err)
	}
	if err := json.Unmarshal([]byte(findingsJSON), &rec.Findings); err != nil {
		return types.ResearchFindingRecord{}, fmt.Errorf("decode research finding findings: %w", err)
	}
	if err := json.Unmarshal([]byte(evidenceIDsJSON), &rec.EvidenceIDs); err != nil {
		return types.ResearchFindingRecord{}, fmt.Errorf("decode research finding evidence ids: %w", err)
	}
	if err := json.Unmarshal([]byte(notesJSON), &rec.Notes); err != nil {
		return types.ResearchFindingRecord{}, fmt.Errorf("decode research finding notes: %w", err)
	}
	if err := json.Unmarshal([]byte(questionsJSON), &rec.Questions); err != nil {
		return types.ResearchFindingRecord{}, fmt.Errorf("decode research finding questions: %w", err)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.ResearchFindingRecord{}, fmt.Errorf("parse research finding created_at: %w", err)
	}
	return rec, nil
}

func scanWorkerUpdate(row interface{ Scan(...any) error }) (types.WorkerUpdateRecord, error) {
	var (
		rec                    types.WorkerUpdateRecord
		findingsJSON           string
		evidenceIDsJSON        string
		artifactsJSON          string
		refsJSON               string
		testsJSON              string
		questionsJSON          string
		proposalsJSON          string
		capabilityRequestsJSON string
		notesJSON              string
		createdAt              string
		deliveredAt            sql.NullString
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
		&rec.Kind,
		&rec.Summary,
		&findingsJSON,
		&evidenceIDsJSON,
		&artifactsJSON,
		&refsJSON,
		&testsJSON,
		&questionsJSON,
		&proposalsJSON,
		&capabilityRequestsJSON,
		&notesJSON,
		&rec.Content,
		&createdAt,
		&rec.DeliveredToRunID,
		&deliveredAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.WorkerUpdateRecord{}, ErrNotFound
		}
		return types.WorkerUpdateRecord{}, fmt.Errorf("scan worker update: %w", err)
	}
	for _, item := range []struct {
		name string
		raw  string
		dst  *[]string
	}{
		{"findings", findingsJSON, &rec.Findings},
		{"evidence_ids", evidenceIDsJSON, &rec.EvidenceIDs},
		{"artifacts", artifactsJSON, &rec.Artifacts},
		{"refs", refsJSON, &rec.Refs},
		{"tests", testsJSON, &rec.Tests},
		{"questions", questionsJSON, &rec.Questions},
		{"proposals", proposalsJSON, &rec.Proposals},
		{"notes", notesJSON, &rec.Notes},
	} {
		if err := json.Unmarshal([]byte(item.raw), item.dst); err != nil {
			return types.WorkerUpdateRecord{}, fmt.Errorf("decode worker update %s: %w", item.name, err)
		}
	}
	if strings.TrimSpace(capabilityRequestsJSON) == "" {
		capabilityRequestsJSON = "[]"
	}
	if err := json.Unmarshal([]byte(capabilityRequestsJSON), &rec.CapabilityRequests); err != nil {
		return types.WorkerUpdateRecord{}, fmt.Errorf("decode worker update capability_requests: %w", err)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.WorkerUpdateRecord{}, fmt.Errorf("parse worker update created_at: %w", err)
	}
	if deliveredAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, deliveredAt.String)
		if err != nil {
			return types.WorkerUpdateRecord{}, fmt.Errorf("parse worker update delivered_at: %w", err)
		}
		rec.DeliveredAt = &t
	}
	return rec, nil
}

func workerUpdateSelectSQL() string {
	return `SELECT owner_id, update_id, agent_id, target_agent_id, channel_id,
	       message_seq, trajectory_id, role, kind, summary, findings_json,
	       evidence_ids_json, artifacts_json, refs_json, tests_json,
	       questions_json, proposals_json, capability_requests_json,
	       notes_json, content, created_at, delivered_to_loop_id, delivered_at
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
