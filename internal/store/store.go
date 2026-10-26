// Package store provides durable runtime storage for the go-choir autoputer runtime.
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
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
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

// SetKernelMode marks the store post-ontology-kernel. Once set, bare OG
// state transitions (UpdateRunOG and siblings) that change lifecycle-owned
// state fail closed with ErrLifecycleAuthorityRequired instead of writing a
// non-event-backed mutation. Called by Runtime.SetKernelMode.
func (s *Store) SetKernelMode() {
	s.kernelMode.Store(true)
}

// KernelMode reports whether the store is in post-ontology-kernel mode.
func (s *Store) KernelMode() bool {
	return s.kernelMode.Load()
}

// ErrLifecycleAuthorityRequired is returned when a legacy writer attempts to
// mutate state owned by the durable lifecycle reducer.
var ErrLifecycleAuthorityRequired = errors.New("durable lifecycle authority required")

// ErrFreshWorkspaceRequiresEmpty is returned when a clean replay or genesis
// workspace would accidentally reuse an existing runtime marker or Dolt
// workspace.
var ErrFreshWorkspaceRequiresEmpty = errors.New("fresh workspace must be empty")

// ErrWorkspaceReplaceRequiresQuarantine is returned when a VM-local workspace
// replacement is requested without an exclusive quarantine directory.
var ErrWorkspaceReplaceRequiresQuarantine = errors.New("workspace replace requires an empty quarantine directory")

// ErrInvalidTextureRevision is returned when a Texture revision write fails
// structured body/source validation before persistence.
var ErrInvalidTextureRevision = errors.New("invalid texture revision")

// ErrNoPendingActorOccurrence marks a durable actor wake whose backing
// canonical occurrence no longer exists — the source row was consumed,
// disposed, or delivery-exhausted before the outbox wake projected. Dispatch
// can never succeed; the sweep must mark the wake projected rather than
// retry forever.
var ErrNoPendingActorOccurrence = errors.New("no pending canonical actor occurrence")

func sanitizeStoreText(value string) string {
	return strings.ToValidUTF8(value, "\uFFFD")
}

// Store wraps the embedded Dolt connection and provides persistence for run
// records, agent records, channel messages, event records, and Texture state.
type doltConnector interface {
	driver.Connector
	Close() error
}

// testHookAfterWorkspaceOpen runs in Open right after the engine is up
// and the store exists; an error from it fails the Open (tests only).
var testHookAfterWorkspaceOpen func(s *Store) error

// testHookWrapConnector wraps each Dolt connector the store's pools are
// built on (tests only).
var testHookWrapConnector func(doltConnector) doltConnector

// errWorkspaceGone is Close's answer when the workspace's Dolt files are
// gone from disk under the open engine.
// It is a fail-closed Close (an ErrCloseLeftOpen) that never clears: the
// engine is abandoned for process exit, so every later Close returns it
// again and Reopen refuses.
var errWorkspaceGone = fmt.Errorf("%w: the Dolt workspace is gone under the open engine; the engine is left for process exit", ErrCloseLeftOpen)

// closeStoreForFailedOpen is the Close closeFailedOpen runs (a seam for
// tests).
var closeStoreForFailedOpen = func(s *Store) error { return s.Close() }

// doltWorkspaceIntact reports whether the workspace root and every Dolt
// database's .dolt/noms in it are still on disk. Closing an engine whose
// noms directory is gone makes Dolt crash the process: the journal cannot
// flush its root to the manifest, and every SQL-engine database is set to
// crash on that fatal I/O error (doltdb.SetCrashOnFatalError).
func doltWorkspaceIntact(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("%w: %v", errWorkspaceGone, err)
	}
	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dotDolt := filepath.Join(root, entry.Name(), ".dolt")
		if _, err := os.Stat(dotDolt); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dotDolt, "noms")); err != nil {
			return fmt.Errorf("%w: %v", errWorkspaceGone, err)
		}
		found = true
	}
	if !found {
		return fmt.Errorf("%w: no Dolt database under %s", errWorkspaceGone, root)
	}
	return nil
}

// poolConnector hands a Dolt connector to database/sql without its Close,
// so DB.Close never tears the embedded engine down. Store.Close closes the
// engine itself, last, after every pinned connection is back in a closed
// pool (pinPools).
type poolConnector struct{ driver.Connector }

// defaultFinalizeCheckpointEvery bounds Dolt commit growth: one checkpoint per
// hundred finalized events instead of one per event (2026-09-03: 138k tape
// rows / ~6k commits backed a 9.8 GB journal for ~0.5 GB of live data).
const defaultFinalizeCheckpointEvery = 100

type Store struct {
	db            *sql.DB
	readDB        *sql.DB
	path          string
	textureDB     *sql.DB
	texturePath   string
	doltConnector doltConnector
	jsonPatchMu   sync.Mutex
	trajectoryMu  sync.Mutex
	// graphHomeMu serializes every engineering-slot claim and release in both
	// homes, and the table-to-graph imports (graph_home.go): a claim reads the
	// setting, consults the other home and writes under it.
	graphHomeMu sync.Mutex
	// slotClaimHook, when set by a test, runs in a graph-home slot claim after
	// the table check and before the slot is read (graph_home.go).
	slotClaimHook func()
	textureRevMu  sync.Mutex
	doltCommitMu  sync.Mutex
	// onlineGCSince is the UnixNano start of the online Dolt GC pass that is
	// running now, 0 when none is (runOnlineDoltGC, OnlineGCStatus).
	onlineGCSince atomic.Int64
	// onlineGCExecHook, when set by a test, stands in for the pass's SQL call.
	onlineGCExecHook func(context.Context) error
	// closeMu serializes Close and Reopen: a second Close waits for the
	// first. closedOK (under closeMu) is set once a Close has closed the
	// pools, and cleared by Reopen.
	closeMu  sync.Mutex
	closedOK bool
	// engineAbandoned (under closeMu) is the errWorkspaceGone of a Close
	// that left the engine for process exit; it never clears.
	engineAbandoned error
	// dirtyGen counts marks; cleanGen (under doltCommitMu) is the mark
	// count the last successful commit read before it ran. The store is
	// dirty while they differ. A mark is one atomic add and never waits on
	// a commit in flight (checkpoint_cadence.go).
	dirtyGen atomic.Uint64
	cleanGen uint64
	// checkpoint is the commit cadence and its counters
	// (checkpoint_cadence.go).
	checkpoint checkpointCadence
	// checkpointExecHook runs after a commit reads the dirty generation and
	// before DOLT_COMMIT with the commit's context; an error from it stands
	// for a failed commit (tests only).
	checkpointExecHook func(context.Context) error
	// actorWakeSignal wakes the outbox projector after committed wake
	// writes (SL slice 2, actor_wake_signal.go).
	actorWakeSignal     chan struct{}
	actorWakeSignalOnce sync.Once
	// announcedWakes are the outbox rows committed since the projector last
	// took them, so a signalled sweep reads them by key.
	announcedWakes announcedActorWakes
	// finalizeSinceCheckpoint counts finalized events since the last Dolt
	// checkpoint. finalizeBatch commits at most every doltCheckpointEvery
	// events (0 = defaultFinalizeCheckpointEvery) instead of once per event:
	// the canonical tape is the durability authority (replay rebuilds), while
	// Dolt commits exist for AS OF addressability, which needs only
	// macro-boundaries. Reads that need current history flush via the
	// dirty-gated checkpoint in GetHistory.
	finalizeSinceCheckpoint uint64
	doltCheckpointEvery     uint64
	workerUpdateMu          sync.Mutex
	runMemoryMu             sync.Mutex
	// runMemoryAgentList counts and observes ListRunMemoryEntriesForAgent
	// (run_memory_observe.go).
	runMemoryAgentList runMemoryAgentListObserver
	textureMutationMu  sync.Mutex
	channelMsgMu       sync.Mutex
	eventMu            sync.Mutex
	presenceMu         sync.Mutex
	// mediaStateMu serialises the tape-bound writes of media_progress,
	// media_recents and user_preferences; mediaStateSeededTape is the tape
	// whose seed of those tables has been appended (media_projection.go).
	mediaStateMu         sync.Mutex
	mediaStateSeededTape *projectionTape
	sessionPresence      map[string]types.DesktopSessionContext
	projectionTape       *projectionTape
	og                   *objectgraph.Service
	ogStore              *objectgraph.DoltStore
	ogReadStore          *objectgraph.DoltStore
	// derivedColumns is the explicit og_objects derived-column migration
	// (derived_columns.go); boot never runs it.
	derivedColumns derivedColumnMigration
	// vocabCutover marks the store post-cutover: the OG write guard refuses
	// declared role fields outside the V2 vocabulary. Set when the migration
	// report exists (open) or when the fenced cutover persists it.
	vocabCutover atomic.Bool
	// depositUpcastMu guards the lazy, once-per-process resolution of
	// depositUpcast: the versioned replayed-deposit upcaster and its alias
	// ledger (see vocab_upcast_deposit.go).
	depositUpcastMu sync.Mutex
	depositUpcast   *depositUpcaster
	// retainReplayDeposits keeps recorded deposit bytes on a fresh or
	// cut-over replay: set on a staged store whose live counterpart never
	// upcast (RetainReplayDeposits).
	retainReplayDeposits atomic.Bool
	// vocabReplay records this process's replay since the last vocabulary
	// fence, so the boot fence can check only the replayed rows
	// (vocab_replay_fence.go).
	vocabReplay vocabReplayTrack
	// kernelMode marks the store post-ontology-kernel: bare OG state
	// transitions that bypass the canonical reducer path fail closed instead
	// of silently writing a non-event-backed mutation. Set by
	// Runtime.SetKernelMode via Store.SetKernelMode.
	kernelMode atomic.Bool
	// maxStreamSeq is the in-memory high-water mark for EventRecord.StreamSeq:
	// the last number the allocator handed out. It is only ever raised by the
	// allocator (allocateStreamSeqLocked) and by a completed seed
	// (runSeeder), under eventMu, and it never exceeds lease.reserved, the
	// durable lease (stream_seq_lease.go). A restart seeds it from the lease
	// on the tape, or from the exact seed when the lease is not trusted
	// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md).
	maxStreamSeq int64
	// lease is the stream_seq lease allocator state, under eventMu.
	lease leaseAllocator
	// leaseWriteHook and leaseScanHook are test seams for the lease writes and
	// the exact seed's pages; production never sets them (a source guard).
	leaseWriteHook func(phase string, reserved int64) error
	leaseScanHook  func(page int)
	// eventSeqs is the in-memory high-water mark of EventRecord.Seq per
	// (run, owner), under eventMu. Each entry is seeded once per process
	// from the store (the same bounded read the per-append scan did), so it
	// is correct across restarts; AppendEvent is the only writer of a
	// non-zero Seq (the lifecycle batch path writes Seq 0).
	eventSeqs map[eventSeqKey]int64
	// telemetryStats is the batching instrumentation (telemetry_stats.go);
	// telemetry is the window sealer, nil until batching is first configured
	// on in this process (telemetry_sealer.go).
	telemetryStats telemetryCounters
	telemetry      atomic.Pointer[telemetrySealer]
	// eventTableWrites is store.event_table as the runtime applied it: the
	// writer stamps the storage class's ops for choir_events (event_table.go).
	eventTableWrites atomic.Bool
	// traceLog is the trace segment log beside the workspace (trace_log.go,
	// R7 slice S2): opened when its directory exists, closed by Close,
	// reopened by Reopen.
	traceLog traceLogState
	// searchIdx caches tokenized head text for SearchDocuments
	// (search_index.go).
	searchIdx searchIndex
	// storyKinds caches trajectory kinds for the story library (stories.go).
	storyKinds storyKindCache

	// dryRunPositionHook, set only by tests, sees the tape position that
	// DryRunProjectionBatch (computer_events.go) passes to projectBatch
	// (project.go); F6 in event_table_test.go reads it.
	// TestEventTableDryRunHookIsTestOnly keeps non-test code from setting it.
	// It runs inside the dry run's transaction: a hook that did store I/O
	// would block on the embedded connection.
	dryRunPositionHook func(pos uint64)
	// eventTableDecodeHook, set only by tests, runs after a choir_events
	// half released the engine mutex and before it decodes (findings B, F).
	eventTableDecodeHook func(op string)
	// eventUnionHook, set only by tests, runs between the halves of a union
	// read: stage "after_object_graph" (finding G).
	eventUnionHook func(stage string)
}

type eventSeqKey struct{ runID, ownerID string }

// eventSeqCacheLimit bounds eventSeqs. Past it the map is dropped and each
// run re-seeds from the store on its next append, which is what every
// append did before the high-water mark.
const eventSeqCacheLimit = 100000

// DB returns the primary embedded Dolt *sql.DB connection used by this store.
// It is exposed so layers that share the workspace (e.g. the self-development
// operation store) can wrap it without opening a second connection. The caller must not close the returned handle; the Store retains
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
	return s.commitDoltCheckpointAt(ctx, message, checkpointSiteOther)
}

func (s *Store) commitDoltCheckpointAt(ctx context.Context, message string, site checkpointSite) error {
	return s.commitDoltCheckpointLocking(ctx, message, site, false)
}

// commitDoltCheckpointLocking commits the working set. boundedLock takes
// the commit lock only until ctx is done (the closing commit, which must
// not wait out a background commit); otherwise the lock wait is
// unbounded, as before.
func (s *Store) commitDoltCheckpointLocking(ctx context.Context, message string, site checkpointSite, boundedLock bool) error {
	if s == nil || s.textureHandle() == nil {
		return fmt.Errorf("runtime store: nil database")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "vm state checkpoint"
	}

	if boundedLock {
		if err := s.lockDoltCommitBy(ctx); err != nil {
			s.checkpoint.recordCutAtClose(site)
			return fmt.Errorf("runtime store: dolt checkpoint: %w: %v", errCommitLockTimeout, err)
		}
	} else {
		s.doltCommitMu.Lock()
	}
	defer s.doltCommitMu.Unlock()
	// A background commit that passed the closed check before Close took
	// the lock after the closing commit: it does not run.
	if site == checkpointSiteBackground && s.checkpoint.isClosed() {
		return nil
	}
	// Read the generation before the commit: a mark after this read may
	// not be in the snapshot, so it keeps the store dirty.
	gen := s.dirtyGen.Load()
	if gen == s.cleanGen {
		return nil
	}
	events, ops := s.checkpoint.events.Load(), s.checkpoint.ops.Load()
	started := time.Now()
	// Pin the store's one connection for the commit, and mark the commit
	// in flight only while it holds it, so a finalize waiting at BeginTx
	// is counted exactly for the connection hold (noteFinalizeConnWait;
	// the instants between Conn returning and the mark, and between the
	// unmark and the release, are not). A pinned sql.Conn does not retry
	// on driver.ErrBadConn as DB.ExecContext does: such a commit fails and
	// the debounce window retries it.
	conn, err := s.textureHandle().Conn(ctx)
	if err != nil {
		err = fmt.Errorf("runtime store: dolt checkpoint: %w", err)
		s.recordCheckpointError(ctx, site, err)
		return err
	}
	s.checkpoint.commitsStarted.Add(1)
	if hook := s.checkpointExecHook; hook != nil {
		err = hook(ctx)
	}
	if err == nil {
		_, err = conn.ExecContext(ctx, "CALL DOLT_COMMIT('-Am', ?)", message)
	}
	s.checkpoint.commitsEnded.Add(1)
	_ = conn.Close()
	noop := err != nil && strings.Contains(err.Error(), "nothing to commit")
	if err != nil && !noop {
		err = fmt.Errorf("runtime store: dolt checkpoint: %w", err)
		s.recordCheckpointError(ctx, site, err)
		return err
	}
	s.cleanGen = gen
	s.checkpoint.recordCommit(site, time.Since(started), events, ops, noop)
	return nil
}

// pinPools checks out the single connection of each open pool (all are
// MaxOpenConns 1) by ctx. On failure it releases what it pinned and
// returns nil and an error (errCloseBudgetSpent if ctx was already done
// before any pool was tried); the caller must not close anything.
func (s *Store) pinPools(ctx context.Context) ([]*sql.Conn, error) {
	if ctx.Err() != nil {
		return nil, errCloseBudgetSpent
	}
	var pinned []*sql.Conn
	for _, pool := range []struct {
		name string
		db   *sql.DB
	}{{"texture", s.textureDB}, {"read", s.readDB}, {"write", s.db}} {
		if pool.db == nil {
			continue
		}
		conn, err := pool.db.Conn(ctx)
		if err != nil {
			for _, held := range pinned {
				_ = held.Close()
			}
			return nil, fmt.Errorf("%s pool: %w", pool.name, err)
		}
		pinned = append(pinned, conn)
	}
	return pinned, nil
}

// closeFailedOpen closes a store whose Open failed and returns Open's
// error. Nobody else holds the store, so a Close that fails closed (work
// the Open started still holds a connection) is retried a bounded number
// of times to release the engine and the workspace lock; a Close error
// that remains is logged and joined to Open's.
func (s *Store) closeFailedOpen(openErr error) error {
	err := closeStoreForFailedOpen(s)
	for attempt := 0; attempt < 2 && errors.Is(err, ErrCloseLeftOpen); attempt++ {
		err = closeStoreForFailedOpen(s)
	}
	if err != nil {
		log.Printf("runtime store: close after a failed open: %v", err)
		return errors.Join(openErr, err)
	}
	return openErr
}

// undoFailedClose restores what a Close that failed closed changed, so a
// store that keeps serving behaves as before it: the checkpoint cadence
// (open again, its timer re-armed) and the telemetry sealer. A cancelled
// derived-column migration stays cancelled.
func (s *Store) undoFailedClose(wasClosed, telemetryStopped bool) {
	s.resumeLeaseWork()
	if telemetryStopped {
		s.resumeTelemetry()
	}
	if !wasClosed {
		s.reopenCheckpoint()
	}
}

// recordCheckpointError counts a commit error: a closing commit cut by its
// close deadline, or a background commit Close cancelled, counts apart, so
// a planned restart leaves failures at 0. It is judged by what cut the
// commit's context, not by the cadence state, which a failed Close
// restores.
func (s *Store) recordCheckpointError(ctx context.Context, site checkpointSite, err error) {
	cut := (site == checkpointSiteClose && ctx.Err() != nil) ||
		(site == checkpointSiteBackground && errors.Is(context.Cause(ctx), errCancelledAtClose))
	if cut && s.checkpoint.recordCutAtClose(site) {
		return
	}
	s.checkpoint.recordFailure(err)
}

// lockDoltCommitBy takes doltCommitMu, giving up when ctx is done. It
// polls: only the closing commit uses it.
func (s *Store) lockDoltCommitBy(ctx context.Context) error {
	if s.doltCommitMu.TryLock() {
		return nil
	}
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if s.doltCommitMu.TryLock() {
				return nil
			}
		}
	}
}

// doltHistoryIsDirty reports whether a mark is not yet in a commit.
func (s *Store) doltHistoryIsDirty() bool {
	s.doltCommitMu.Lock()
	defer s.doltCommitMu.Unlock()
	return s.dirtyGen.Load() != s.cleanGen
}

func (s *Store) markDoltHistoryDirty() {
	if s == nil {
		return
	}
	s.dirtyGen.Add(1)
	s.armCheckpoint()
}

// schemaDDL creates the runtime tables if they do not already exist.
const schemaDDL = `
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


CREATE TABLE IF NOT EXISTS wire_publish_debounce_entries (
	entry_id    BIGINT NOT NULL AUTO_INCREMENT,
	owner_id    VARCHAR(255) NOT NULL,
	computer_id VARCHAR(255) NOT NULL,
	agent_id    VARCHAR(255) NOT NULL,
	content     LONGTEXT NOT NULL,
	created_at  DATETIME NOT NULL,
	PRIMARY KEY (entry_id)
);

CREATE TABLE IF NOT EXISTS wire_publish_debounce_state (
	owner_id         VARCHAR(255) NOT NULL,
	computer_id      VARCHAR(255) NOT NULL,
	agent_id         VARCHAR(255) NOT NULL,
	last_dispatch_at DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_wire_publish_debounce_entries_scope
	ON wire_publish_debounce_entries(owner_id, computer_id, agent_id, entry_id);

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

CREATE INDEX IF NOT EXISTS idx_events_loop_id_seq ON events(loop_id, seq);
CREATE INDEX IF NOT EXISTS idx_events_owner_id ON events(owner_id);
CREATE INDEX IF NOT EXISTS idx_events_agent_id ON events(agent_id);
CREATE INDEX IF NOT EXISTS idx_events_channel_id_ts ON events(channel_id, ts);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_parent ON run_memory_entries(parent_entry_id);
CREATE INDEX IF NOT EXISTS idx_co_super_slots_run_id ON co_super_slots(run_id);
CREATE INDEX IF NOT EXISTS idx_co_super_slots_agent_id ON co_super_slots(owner_id, agent_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_media_progress_owner_kind_updated ON media_progress(owner_id, media_kind, updated_at);
CREATE INDEX IF NOT EXISTS idx_media_recents_owner_kind_opened ON media_recents(owner_id, media_kind, opened_at);
CREATE INDEX IF NOT EXISTS idx_user_preferences_owner_updated ON user_preferences(owner_id, updated_at);

CREATE TABLE IF NOT EXISTS desktop_workspaces (
	owner_id       VARCHAR(255) NOT NULL,
	computer_id    VARCHAR(255) NOT NULL DEFAULT '',
	desktop_id     VARCHAR(255) NOT NULL,
	windows_json   LONGTEXT NOT NULL DEFAULT '[]',
	active_window  VARCHAR(255) NOT NULL DEFAULT '',
	updated_at     DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, desktop_id)
);

CREATE TABLE IF NOT EXISTS desktop_sessions (
	owner_id          VARCHAR(255) NOT NULL,
	computer_id       VARCHAR(255) NOT NULL DEFAULT '',
	desktop_id        VARCHAR(255) NOT NULL,
	session_id        VARCHAR(255) NOT NULL,
	device_id         VARCHAR(255) NOT NULL DEFAULT '',
	viewport_profile  VARCHAR(64) NOT NULL DEFAULT '',
	visibility_state  VARCHAR(64) NOT NULL DEFAULT '',
	last_input_at     DATETIME NULL,
	driver_until      DATETIME NULL,
	created_at        DATETIME NOT NULL,
	updated_at        DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, desktop_id, session_id)
);

CREATE TABLE IF NOT EXISTS desktop_app_instances (
	owner_id              VARCHAR(255) NOT NULL,
	computer_id           VARCHAR(255) NOT NULL DEFAULT '',
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
	PRIMARY KEY (owner_id, computer_id, desktop_id, app_instance_id)
);

CREATE TABLE IF NOT EXISTS desktop_window_placements (
	owner_id               VARCHAR(255) NOT NULL,
	computer_id            VARCHAR(255) NOT NULL DEFAULT '',
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
	PRIMARY KEY (owner_id, computer_id, desktop_id, session_id, app_instance_id)
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
` + choirEventsDDL

// OpenFresh creates a unified Dolt workspace only when its marker and derived
// workspace path can both be exclusively reserved among OpenFresh callers.
// It is the required constructor for disposable replay projections; it never
// removes an existing workspace to make the call succeed.
func OpenFresh(dbPath string) (*Store, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("%w: path is required", ErrFreshWorkspaceRequiresEmpty)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("runtime store: create fresh parent directory: %w", err)
	}

	marker, err := os.OpenFile(dbPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("%w: %s exists", ErrFreshWorkspaceRequiresEmpty, dbPath)
		}
		return nil, fmt.Errorf("runtime store: reserve fresh marker: %w", err)
	}
	if err := marker.Close(); err != nil {
		_ = os.Remove(dbPath)
		return nil, fmt.Errorf("runtime store: close fresh marker: %w", err)
	}

	workspacePath := deriveTextureWorkspacePath(dbPath)
	if err := os.Mkdir(workspacePath, 0o755); err != nil {
		_ = os.Remove(dbPath)
		if os.IsExist(err) {
			return nil, fmt.Errorf("%w: %s exists", ErrFreshWorkspaceRequiresEmpty, workspacePath)
		}
		return nil, fmt.Errorf("runtime store: reserve fresh workspace: %w", err)
	}

	store, err := Open(dbPath)
	if err != nil {
		return nil, err
	}
	return store, nil
}

// WorkspaceReplaceReceipt is the loss/quarantine record for a VM-local
// workspace replacement. It is evidence, not a checkpoint or restore authority.
type WorkspaceReplaceReceipt struct {
	OriginalPath         string `json:"original_path"`
	WorkspacePath        string `json:"workspace_path"`
	QuarantineDir        string `json:"quarantine_dir"`
	QuarantinedMarker    string `json:"quarantined_marker"`
	QuarantinedWorkspace string `json:"quarantined_workspace"`
}

// ReplaceWorkspace closes the current store, moves its marker and Dolt
// workspace into an exclusive quarantine directory, and opens a current-DDL
// workspace at the original path through OpenFresh. It appends no event and
// copies no rows. Failure after quarantine leaves the original path empty;
// the quarantined bytes are the rollback ref.
func (s *Store) ReplaceWorkspace(quarantineDir string) (*Store, WorkspaceReplaceReceipt, error) {
	var receipt WorkspaceReplaceReceipt
	if s == nil {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: nil store")
	}
	originalPath := strings.TrimSpace(s.path)
	if originalPath == "" {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: path is required")
	}
	quarantineDir = strings.TrimSpace(quarantineDir)
	if quarantineDir == "" {
		return nil, receipt, fmt.Errorf("%w: path is required", ErrWorkspaceReplaceRequiresQuarantine)
	}
	workspacePath := deriveTextureWorkspacePath(originalPath)
	receipt.OriginalPath = originalPath
	receipt.WorkspacePath = workspacePath
	receipt.QuarantineDir = quarantineDir
	receipt.QuarantinedMarker = filepath.Join(quarantineDir, filepath.Base(originalPath))
	receipt.QuarantinedWorkspace = filepath.Join(quarantineDir, filepath.Base(workspacePath))

	if err := s.Close(); err != nil {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: close: %w", err)
	}
	if err := os.Mkdir(quarantineDir, 0o755); err != nil {
		if os.IsExist(err) {
			return nil, receipt, fmt.Errorf("%w: %s exists", ErrWorkspaceReplaceRequiresQuarantine, quarantineDir)
		}
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: create quarantine: %w", err)
	}
	if err := os.Rename(originalPath, receipt.QuarantinedMarker); err != nil {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: quarantine marker: %w", err)
	}
	if err := os.Rename(workspacePath, receipt.QuarantinedWorkspace); err != nil {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: quarantine workspace: %w", err)
	}
	fresh, err := OpenFresh(originalPath)
	if err != nil {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: open current schema: %w", err)
	}
	// Benign today (the fresh workspace has no lease row, and the runtime
	// restarts afterwards), and it makes the swap-point guard uniform.
	if err := storeschema.WriteLeaseUntrusted(fresh.TexturePath(), leaseMarkerReplace); err != nil {
		return nil, receipt, errors.Join(fmt.Errorf("runtime store: replace workspace: lease untrust marker: %w", err), fresh.Close())
	}
	return fresh, receipt, nil
}

// Open opens (or creates) the unified embedded Dolt workspace derived from
// dbPath and applies the runtime and texture schemas. A legacy SQLite file at
// dbPath is retained only as inert rollback evidence; serving startup never
// reads or imports it.
func Open(dbPath string) (*Store, error) {
	donePrepare := objectgraph.BootPhase("store: open", "prepare-path")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		donePrepare(err)
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
	donePrepare(nil)
	log.Printf("store: open fresh=%t", freshStore)

	// The receipt this workspace carried before this Open rewrites it. It is
	// read here, before anything can change it: receipt 3 written over a
	// receipt 2 would hide that a binary that does not know the stream_seq
	// lease ran in between (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md,
	// section 3). The decision to write the untrust marker is made below, just
	// before the receipt is rewritten.
	priorWorkspace := deriveTextureWorkspacePath(dbPath)
	priorReceipt, priorFound, priorReadErr := storeschema.Read(filepath.Join(priorWorkspace, storeschema.File))
	priorHadData := workspaceHoldsData(priorWorkspace)

	doneWorkspace := objectgraph.BootPhase("store: open", "workspace-open")
	db, workspacePath, connector, err := openTextureWorkspaceDB(dbPath)
	doneWorkspace(err)
	if err != nil {
		return nil, fmt.Errorf("runtime store: open unified Dolt workspace: %w", err)
	}

	// The store floor (storeschema/floor.go): a binary below it never opens
	// the store, and refuses before any DDL or receipt write.
	if err := storeschema.CheckFloor(workspacePath); err != nil {
		_ = db.Close()
		return nil, err
	}

	readDB := sql.OpenDB(poolConnector{connector})
	configureEmbeddedDoltDB(readDB)

	s := &Store{db: db, readDB: readDB, path: dbPath, texturePath: workspacePath, doltConnector: connector}
	if hook := testHookAfterWorkspaceOpen; hook != nil {
		if err := hook(s); err != nil {
			return nil, s.closeFailedOpen(fmt.Errorf("runtime store: open: %w", err))
		}
	}
	doneRuntimeSchema := objectgraph.BootPhase("store: open", "runtime-schema")
	if err := s.bootstrap(); err != nil {
		doneRuntimeSchema(err)
		return nil, s.closeFailedOpen(fmt.Errorf("runtime store: bootstrap: %w", err))
	}
	doneRuntimeSchema(nil)

	// Initialize object-graph persistence on the same private Dolt workspace.
	ogDoltStore := objectgraph.NewDoltStore(db)
	doneOGSchema := objectgraph.BootPhase("store: open", "objectgraph-schema")
	if err := ogDoltStore.EnsureSchema(context.Background()); err != nil {
		doneOGSchema(err)
		return nil, s.closeFailedOpen(fmt.Errorf("runtime store: bootstrap object graph: %w", err))
	}
	doneOGSchema(nil)
	s.ogStore = ogDoltStore
	ogDoltStore.SetWriteValidator(s.vocabWriteGuard)
	// A persisted migration report means this store already cut over: the
	// write guard activates immediately on reopen.
	s.armVocabFenceFromReport()
	// Create a read-only DoltStore using the read connection pool so OG
	// reads don't block during write transactions on the main connection.
	// Both stores share one embedded engine, which is not safe for concurrent
	// queries - serialize them on a single mutex.
	if readDB != nil {
		s.ogReadStore = objectgraph.NewDoltStore(readDB)
		s.ogReadStore.ShareEngineMutex(ogDoltStore)
		// Mirror the write store's live generated-column set so the read
		// pool's metadata queries use the index path too. Without this the
		// read store's metaColumns is empty and every hot metadata lookup
		// silently falls back to a JSON_EXTRACT full scan (measured:
		// ~1.4s/call under drain).
		s.ogReadStore.InheritMetadataColumns(ogDoltStore)
		// The read store must see which derived-key families the write store
		// has validated, and withdraw them on a stale read.
		s.ogReadStore.ShareDerivedKeys(ogDoltStore)
	}
	s.og = objectgraph.NewService(objectgraph.Config{
		Durable: ogDoltStore,
	})

	// Apply the texture schema to the embedded Dolt workspace.
	doneTextureSchema := objectgraph.BootPhase("store: open", "texture-schema")
	if err := s.EnsureTextureSchema(); err != nil {
		doneTextureSchema(err)
		log.Printf("ERROR EnsureTextureSchema failed: %v", err)
		return nil, s.closeFailedOpen(fmt.Errorf("runtime store: bootstrap texture: %w", err))
	}
	doneTextureSchema(nil)

	// The run-memory kind index: created on an empty table, only reported on a
	// table with rows (run_memory_kind_index.go). Never a build on a populated
	// table at boot.
	// Its error is in the boot phase log (status=failed) and never fails Open.
	_ = bootSubstep("runtime-schema", "run-memory-kind-index", func() error {
		return s.ensureRunMemoryKindIndexAtOpen(context.Background())
	})

	// R5: the duplicate idx_run_memory_entries_loop_seq is dropped from an empty
	// table and only reported on one with rows (run_memory_duplicate_index.go).
	// schemaDDL no longer creates it. Same rule: never DDL at boot on a table
	// with rows; the operation drops it.
	_ = bootSubstep("runtime-schema", "run-memory-loop-seq-index", func() error {
		return s.dropRunMemoryLoopSeqIndexAtOpen(context.Background())
	})

	// The index diet: idx_..._owner_loop_seq and idx_..._owner_agent_created
	// are dropped from an empty table and only reported on one with rows
	// (run_memory_index_diet.go); schemaDDL no longer creates them. The
	// operation drops them. Runs after the kind index hook, which they need.
	_ = bootSubstep("runtime-schema", "run-memory-diet-indexes", func() error {
		return s.dropRunMemoryDietIndexesAtOpen(context.Background())
	})

	// Persist the running build's store schema epoch inside the workspace so
	// the updater's pre-mutation state-compat gate (S2-d) can refuse a release
	// whose binary does not understand this store — the vm-3dc68688
	// stale-binary failure class.
	doneEpoch := objectgraph.BootPhase("store: open", "store-schema-epoch")
	// A prior receipt below the epoch that writes leases (or unreadable, or
	// absent on a workspace that holds data) writes the untrust marker,
	// durably, BEFORE the receipt is rewritten. A marker-write failure aborts
	// Open here: there is no state in which receipt 3 exists without the
	// marker the old receipt called for.
	if reason := priorReceiptUntrustReason(priorFound, priorReceipt.Version, priorReadErr, priorHadData); reason != "" {
		if err := storeschema.WriteLeaseUntrusted(workspacePath, reason); err != nil {
			doneEpoch(err)
			return nil, s.closeFailedOpen(err)
		}
	}
	if err := storeschema.Write(workspacePath); err != nil {
		doneEpoch(err)
		return nil, s.closeFailedOpen(err)
	}
	// A workspace holding stamped events (a replay or base install into a
	// fresh workspace) is floored here; a one-row probe, not a scan.
	if err := s.ensureEventTableFloor(); err != nil {
		doneEpoch(err)
		return nil, s.closeFailedOpen(err)
	}
	doneEpoch(nil)

	if freshStore {
		if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
			return nil, s.closeFailedOpen(fmt.Errorf("runtime store: write marker: %w", err))
		}
	}
	s.initLeaseObs()
	s.openTraceLogIfPresent()

	return s, nil
}

// retiredRuntimeTables are the legacy relational runtime tables dropped at
// bootstrap. They are declared retired_absent in the replay airworthiness
// manifest.
var retiredRuntimeTables = []string{
	"agents", "runs", "channel_messages", "inbox_deliveries", "run_acceptances",
	"run_continuations", "trajectories", "work_items", "browser_sessions",
	"worker_updates", "coagent_mailboxes", "desktop_state", "computer_effective_state",
}

// retiredTextureTables are the legacy relational texture-workspace tables
// dropped at texture bootstrap.
var retiredTextureTables = []string{
	"texture_documents", "texture_controller_checkpoints", "content_items", "podcast_subscriptions",
}

// bootSubstep runs one named step of a boot phase and logs its start, end and
// duration, so a step that hangs is named by the last line that has no end.
func bootSubstep(phase, name string, fn func() error) error {
	done := objectgraph.BootPhase("store: open", phase+"/"+name)
	err := fn()
	done(err)
	return err
}

// bootstrap applies the schema DDL to the database.
func (s *Store) bootstrap() error {
	if err := bootSubstep("runtime-schema", "apply-ddl", func() error {
		_, err := s.db.Exec(schemaDDL)
		return err
	}); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	// The solitaire tables are retired with the Candidate A solitaire engine
	// (no reader or writer remains). Drop any table a pre-retirement computer
	// still carries so replay eligibility sees the declared retired_absent
	// contract.
	for _, table := range []string{"solitaire_moves", "solitaire_games"} {
		if _, err := s.db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("drop retired %s: %w", table, err)
		}
	}
	// The legacy relational runtime tables are retired (Cut 1 L2, 6c): runs,
	// agents, messages, deliveries, acceptances, work items and updates live
	// on the object graph and the canonical event tape, and no production
	// path reads or writes these tables. Drop any table a pre-retirement
	// computer still carries so replay eligibility sees the declared
	// retired_absent contract.
	for _, table := range retiredRuntimeTables {
		if _, err := s.db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("drop retired %s: %w", table, err)
		}
	}

	if err := bootSubstep("runtime-schema", "ensure-columns", func() error { return s.ensureColumns(runtimeColumnMigrations) }); err != nil {
		return err
	}
	if err := bootSubstep("runtime-schema", "events-indexes", func() error {
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_owner_stream_seq ON events(owner_id, stream_seq)`); err != nil {
			return fmt.Errorf("create idx_events_owner_stream_seq: %w", err)
		}
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_trajectory_stream_seq ON events(trajectory_id, stream_seq)`); err != nil {
			return fmt.Errorf("create idx_events_trajectory_stream_seq: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := bootSubstep("runtime-schema", "desktop-primary-keys", s.ensureDesktopPrimaryKeys); err != nil {
		return err
	}
	return bootSubstep("runtime-schema", "backfill-events-stream-seq", s.backfillDerivedRuntimeState)
}

// runtimeColumnMigrations are the columns added to existing runtime tables.
var runtimeColumnMigrations = []struct {
	table string
	name  string
	ddl   string
}{
	{"co_super_slots", "requested_by_run_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
	{"events", "agent_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
	{"events", "channel_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
	{"events", "trajectory_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
	{"events", "stream_seq", "BIGINT NOT NULL DEFAULT 0"},
	{"self_development_operations", "route_receipt", "VARCHAR(128) NOT NULL DEFAULT ''"},
	{"self_development_operations", "release_digest", "CHAR(64) NOT NULL DEFAULT ''"},
	{"self_development_operations", "code_ref", "VARCHAR(96) NOT NULL DEFAULT ''"},
	{"self_development_operations", "artifact_program_ref", "VARCHAR(128) NOT NULL DEFAULT ''"},
	{"self_development_operations", "decision_receipt", "VARCHAR(255) NOT NULL DEFAULT ''"},
	{"self_development_operations", "last_intent_error", "LONGTEXT NOT NULL DEFAULT ''"},
	{"desktop_workspaces", "computer_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
	{"desktop_sessions", "computer_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
	{"desktop_app_instances", "computer_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
	{"desktop_window_placements", "computer_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
}

func (s *Store) backfillDerivedRuntimeState() error {
	if _, err := s.db.Exec(`UPDATE events SET stream_seq = seq WHERE stream_seq = 0`); err != nil {
		return fmt.Errorf("backfill events.stream_seq: %w", err)
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

// ensureColumns checks all requested columns in one information_schema query
// and ALTERs only the missing ones. This replaces N sequential ensureColumn
// calls (each ~95ms on embedded Dolt) with a single round-trip.
func (s *Store) ensureColumns(migrations []struct {
	table string
	name  string
	ddl   string
}) error {
	if len(migrations) == 0 {
		return nil
	}
	// Collect unique tables for the IN clause.
	tables := make(map[string]struct{})
	for _, m := range migrations {
		if err := validateIdentifier(m.table); err != nil {
			return err
		}
		if err := validateIdentifier(m.name); err != nil {
			return err
		}
		tables[m.table] = struct{}{}
	}
	// Build IN clause with placeholders.
	tableList := make([]string, 0, len(tables))
	for t := range tables {
		tableList = append(tableList, t)
	}
	placeholders := make([]string, len(tableList))
	args := make([]any, len(tableList))
	for i, t := range tableList {
		placeholders[i] = "?"
		args[i] = t
	}
	query := fmt.Sprintf(`
SELECT table_name, column_name
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name IN (%s)`, strings.Join(placeholders, ","))
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("information_schema.columns batch: %w", err)
	}
	existing := make(map[string]map[string]struct{})
	for rows.Next() {
		var table, col string
		if err := rows.Scan(&table, &col); err != nil {
			rows.Close()
			return err
		}
		if existing[table] == nil {
			existing[table] = make(map[string]struct{})
		}
		existing[table][col] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	// ALTER only missing columns.
	for _, m := range migrations {
		if _, ok := existing[m.table][m.name]; ok {
			continue
		}
		done := objectgraph.BootPhase("store: open", "runtime-schema/add-column "+m.table+"."+m.name)
		_, alterErr := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.table, m.name, m.ddl))
		done(alterErr)
		if alterErr != nil {
			return fmt.Errorf("alter table %s add column %s: %w", m.table, m.name, alterErr)
		}
		// Record the addition so a duplicate entry later in the list skips
		// instead of double-ALTERing (matches old per-column loop semantics).
		if existing[m.table] == nil {
			existing[m.table] = make(map[string]struct{})
		}
		existing[m.table][m.name] = struct{}{}
	}
	return nil
}

// reopenBeforeAdoptHook is a test seam: it runs in Reopen after the adopted
// workspace is floored and before the store's handles are swapped to it.
// Always nil in production.
var reopenBeforeAdoptHook func(adoptedWorkspace string)

// Reopen opens path and adopts its handles into s. The caller must Close s
// first so the workspace files can be replaced. Process-start copies of the
// same *Store keep working because the pointer does not change.
func (s *Store) Reopen(path string) error {
	if s == nil {
		return fmt.Errorf("runtime store: reopen requires a store")
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if err := s.checkpoint.leftOpen(); err != nil {
		return fmt.Errorf("runtime store: reopen: %w", err)
	}
	if !s.closedOK {
		// Adopting new handles over open ones would orphan them.
		return fmt.Errorf("runtime store: reopen requires a closed store")
	}
	s.CancelDerivedColumnMigration()
	live, err := Open(path)
	if err != nil {
		return err
	}
	// The writer's switch survives a reopen; the adopted workspace (a restore
	// or base install) may carry no floor, so it gets one BEFORE it is adopted
	// and before any stamp can land in it (bunch 148, Devin 2a). If that fails
	// the writer goes off first.
	if s.eventTableWrites.Load() {
		if err := writeEventTableFloorAt(live.texturePath); err != nil {
			s.eventTableWrites.Store(false)
			log.Printf("runtime store: reopen: event-table writer off: %v", err)
		}
	}
	if reopenBeforeAdoptHook != nil {
		reopenBeforeAdoptHook(live.texturePath)
	}
	// The adopted workspace may carry a different deposit-upcast ledger (a
	// rematerialize flip or base install replaces the files wholesale), so
	// the cached decision, its alias map and its appended file handle must
	// not survive the reopen: the handle points at the quarantined
	// realization once the flip renames the workspace.
	if err := s.closeDepositUpcastLedger(); err != nil {
		// The opened store is not adopted: close it rather than leak its
		// engine and workspace lock.
		return errors.Join(fmt.Errorf("runtime store: reopen deposit upcast ledger: %w", err), live.Close())
	}
	s.depositUpcastMu.Lock()
	s.depositUpcast = nil
	s.depositUpcastMu.Unlock()
	s.vocabReplay.reset()
	// The adopted workspace may hold media-state rows this tape has not
	// recorded (a restore, a base install): derive the seed again for it
	// (media_projection.go; bunch 188).
	s.mediaStateMu.Lock()
	s.mediaStateSeededTape = nil
	s.mediaStateMu.Unlock()
	s.db = live.db
	s.readDB = live.readDB
	s.path = live.path
	s.textureDB = live.textureDB
	s.texturePath = live.texturePath
	s.doltConnector = live.doltConnector
	// The adopted workspace may differ from the last commit: keep it dirty.
	s.dirtyGen.Add(1)
	// The object-graph store is new, and BindProjectionTape installed its
	// mutation interceptor on the one it replaces. A bound store must hand
	// the new one the same interceptor before the store is published here
	// (s.ogStore and s.og below), or an og_objects/og_edges write after the
	// swap is direct SQL with no tape event while ProjectionTapeBound() still
	// answers true. Installing it on live.ogStore first leaves no window: the
	// store is closed until Reopen returns, and nothing can reach the new
	// store before this function does
	// (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md).
	if s.projectionTape != nil {
		live.ogStore.SetMutationInterceptor(s.projectionTape.interceptOG)
	}
	// Open bound the adopted store's write validator to live
	// (SetWriteValidator(live.vocabWriteGuard)), and *live is zeroed below, so
	// the vocabulary fence would read a zeroed vocabCutover and pass every
	// write. Bind it to s, and keep the fence armed if either side is cut
	// over: live read the adopted workspace's migration report in Open, s
	// may have cut over in this process, and a Reopen never disarms a fence
	// (fail closed). Open binds nothing else on live that Reopen drops: its
	// other work is the handles copied here, the metadata, derived-key and
	// engine-mutex sharing between the two object-graph stores (which refer
	// to those stores, not to live), and the Store-level state Reopen resets.
	live.ogStore.SetWriteValidator(s.vocabWriteGuard)
	if live.vocabCutover.Load() {
		s.vocabCutover.Store(true)
	}
	s.og = live.og
	s.ogStore = live.ogStore
	s.ogReadStore = live.ogReadStore
	// The trace log lives outside the swapped workspace: the scratch store
	// reopened it, and it moves to s before the scratch store is zeroed.
	s.adoptTraceLog(live)
	*live = Store{}
	s.closedOK = false
	s.reopenCheckpoint()
	// The lease state (ready or unseeded, the reserved block, the counters)
	// stays as it was; only the refusal Close set is lifted. The workspace is
	// re-read first, while Close's refusal still holds: a seeder that starts
	// the moment the lease resumes must read the adopted workspace's untrust
	// marker, not the one this store just left. The adopted workspace may
	// hold an older lease row or none, so refreshLeaseWorkspace also forgets
	// that the latest lease row is on the tape, and the kick below re-affirms
	// the block with a fresh row on the tape (bunch 178, Devin 1 and DeepSeek 1).
	s.refreshLeaseWorkspace()
	s.resumeLeaseWork()
	s.kickLeaseRowOntoTape()
	// The workspace was swapped under the trace log (a restore, a base
	// install): record where, so a reader can tell lines of an abandoned
	// branch from the restored one (trace-log design 7.3).
	s.writeTraceLogRestoreBoundary()
	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closedOK {
		if s.engineAbandoned != nil {
			// An earlier Close abandoned the engine over a removed
			// workspace: the store stays failed closed.
			return s.checkpoint.leaveOpen(s.engineAbandoned)
		}
		return nil
	}
	// The lease goroutines (seeder, refill) stop first: they append through the
	// tape and read the pools Close is about to close.
	s.cancelLeaseWork()
	// Graceful close seals the telemetry window first. A crash does not, which
	// is the loss the ruling allows.
	telemetryStopped := s.closeTelemetry()
	wasClosed := s.checkpoint.isClosed()
	// A running derived-column migration stops before its next step; a
	// step in flight is not waited for (database/sql Close does not wait
	// for a connection in use), so it can land after the closing commit or
	// fail on the closed pool. Cancel it before the closing checkpoint so
	// no new step starts after that final commit, and commit the
	// checkpoint before the pools below close, since it writes through
	// s.db. One deadline (the close deadline plus one grace) bounds the
	// checkpoint waits, the commit-lock fence and the pin; the telemetry
	// drain above (its own 10 s), the wait for closeMu and the pool
	// teardown below are outside it. If a Dolt commit or another statement
	// is still using the engine at it, Close fails closed: closing the
	// pools would tear the embedded engine down under it, so they stay
	// open (process exit reclaims them) and Close returns an
	// ErrCloseLeftOpen naming the cause. Close again once it finishes.
	// While Close waits, readers wait too: the pin holds the read pool's
	// connection while it waits for the write pool's.
	s.CancelDerivedColumnMigration()
	deadline := time.Now().Add(s.checkpoint.closeLimit())
	hard := deadline.Add(checkpointCloseGrace)
	if err := s.closeCheckpoint(deadline, hard); err != nil {
		s.undoFailedClose(wasClosed, telemetryStopped)
		return err
	}
	// The fence, in every mode: every Dolt commit (background, inline,
	// history, closing, maintenance) runs under doltCommitMu, so holding it
	// means no commit holds a pinned connection while the pools close.
	fence, cancelFence := context.WithDeadline(context.Background(), hard)
	defer cancelFence()
	if err := s.lockDoltCommitBy(fence); err != nil {
		err = s.checkpoint.leaveOpen(fmt.Errorf("%w (a Dolt commit holds the commit lock)", ErrCheckpointCommitLive))
		s.undoFailedClose(wasClosed, telemetryStopped)
		return err
	}
	defer s.doltCommitMu.Unlock()
	// The lease goroutines end before the pools are pinned (a statement of
	// theirs in flight would fail the pin closed).
	s.drainLeaseWork(hard)
	// Pin the pools (owner decision of 2026-10-22): check out each pool's
	// single connection by the same deadline, so no statement that is not
	// a commit (a statement in flight, an open transaction) is using the
	// engine when it closes. Waiters are failed by the pool closes below.
	pinned, pinErr := s.pinPools(fence)
	if pinErr != nil {
		var leftErr error
		if errors.Is(pinErr, errCloseBudgetSpent) {
			leftErr = fmt.Errorf("%w (%v)", ErrCloseLeftOpen, pinErr)
		} else {
			leftErr = fmt.Errorf("%w (%v)", ErrPoolStatementLive, pinErr)
		}
		err := s.checkpoint.leaveOpen(leftErr)
		s.undoFailedClose(wasClosed, telemetryStopped)
		return err
	}
	s.checkpoint.clearLeftOpen()
	s.closedOK = true
	// The trace log fsyncs, seals and releases its directory with the store
	// (design 4.1). It cannot fail Close.
	s.closeTraceLog()
	var err error
	if closeErr := s.closeDepositUpcastLedger(); closeErr != nil {
		err = closeErr
	}
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
	// Released into closed pools, the pinned connections are closed, with
	// the engine still up; then the engine closes with no connection open.
	for _, conn := range pinned {
		_ = conn.Close()
	}
	// Close the engine only over its files: if the workspace was removed
	// under it, closing it crashes the process (doltWorkspaceIntact), so
	// it is left open for process exit to reclaim and Close says so.
	if s.doltConnector != nil && s.texturePath != "" {
		if goneErr := doltWorkspaceIntact(s.texturePath); goneErr != nil {
			s.engineAbandoned = goneErr
			s.checkpoint.enginesLeftOpen.Add(1)
			return errors.Join(err, s.checkpoint.leaveOpen(goneErr))
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

// ResolveLegacyAgentScope resolves a pre-scoping mailbox identity only when
// every surviving agent and run witness names one owner on the current computer.
func (s *Store) ResolveLegacyAgentScope(ctx context.Context, computerID, agentID string) (types.AgentRecord, error) {
	return s.ResolveLegacyAgentScopeOG(ctx, computerID, agentID)
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

const (
	lifecycleTerminalSettlementAttempts = 8
	lifecycleTerminalSettlementKey      = "lifecycle_terminal_settlement_requested"
)

func lifecycleTerminalSettlementCommandID(runID string) string {
	return "lifecycle-terminal-settlement:" + strings.TrimSpace(runID)
}

func lifecycleTerminalSettlementRequested(rec types.RunRecord) bool {
	requested, _ := rec.Metadata[lifecycleTerminalSettlementKey].(bool)
	return requested
}

// LifecycleTerminalSettlementRequested reports the durable boot-repair
// trigger copied onto a terminal lifecycle run. Callers may skip GetLifecycleRun
// when the listed projection is already false.
func LifecycleTerminalSettlementRequested(rec types.RunRecord) bool {
	return lifecycleTerminalSettlementRequested(rec)
}

// ReconcileLifecycleSettlementForTerminalRun invokes the canonical settlement
// reducer for a durable active-to-terminal trigger. The trigger is evidence
// that a product caller should try; it is not settlement authority.
func (s *Store) ReconcileLifecycleSettlementForTerminalRun(ctx context.Context, rec types.RunRecord) error {
	rec.TrajectoryID = runTrajectoryID(rec)
	stored, err := s.GetLifecycleRun(ctx, rec.OwnerID, rec.ComputerID, rec.RunID)
	if err != nil {
		return err
	}
	if !stored.State.Terminal() || !lifecycleTerminalSettlementRequested(stored) {
		return nil
	}
	for attempt := 0; attempt < lifecycleTerminalSettlementAttempts; attempt++ {
		snapshot, snapshotErr := s.GetLifecycleSnapshot(ctx, stored.OwnerID, stored.ComputerID, stored.TrajectoryID)
		if snapshotErr != nil {
			return snapshotErr
		}
		if snapshot.Trajectory.Status != types.TrajectoryLive {
			return nil
		}
		prospective := snapshot.Trajectory
		prospective.TerminalArtifactHeadRef = snapshot.Document.CurrentRevisionID
		ready, readyErr := s.lifecycleSettlementReady(ctx, prospective, nil, nil)
		if readyErr != nil {
			return readyErr
		}
		if !ready {
			return nil
		}
		req := types.SettleLifecycleTrajectoryRequest{
			OwnerID:                  stored.OwnerID,
			ComputerID:               stored.ComputerID,
			CommandID:                lifecycleTerminalSettlementCommandID(stored.RunID),
			TrajectoryID:             stored.TrajectoryID,
			ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
			ExpectedHeadRevisionID:   snapshot.Document.CurrentRevisionID,
		}
		req.CommandDigest, _ = ComputeSettleLifecycleTrajectoryDigest(req)
		if _, err = s.SettleLifecycleTrajectory(ctx, req); err == nil {
			return nil
		}
		if !errors.Is(err, ErrConcurrentStateChange) {
			return err
		}
	}
	return ErrConcurrentStateChange
}

func (s *Store) persistLifecycleRun(ctx context.Context, rec types.RunRecord) (bool, error) {
	return s.persistLifecycleRunWithEvent(ctx, rec, nil)
}

// persistLifecycleRunWithEvent routes a lifecycle-bound run through the
// canonical projection. When event is non-nil it is folded into the same
// atomic batch as the run projection so the state change and its runtime
// event commit together.
func (s *Store) persistLifecycleRunWithEvent(ctx context.Context, rec types.RunRecord, event *types.EventRecord) (bool, error) {
	rec.TrajectoryID = runTrajectoryID(rec)
	if strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.ComputerID) == "" || strings.TrimSpace(rec.TrajectoryID) == "" {
		return false, nil
	}
	trajectory, err := s.GetLifecycleTrajectory(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return true, err
	}

	// The event is written by the projection batch, not by AppendEvent, so it
	// takes its numbers here, before the command digest covers it.
	if event != nil {
		if err := s.AllocateEventSeq(ctx, event); err != nil {
			return true, fmt.Errorf("allocate folded event seq: %w", err)
		}
	}

	body, err := json.Marshal(rec)
	if err != nil {
		return true, err
	}
	req := types.ReplaceLifecycleActivationRequest{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID,
		CommandID:    "lifecycle-activation:" + strings.TrimPrefix(objectgraph.SHA256(body), "sha256:"),
		TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: rec,
		Event: event,
	}
	req.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(req)
	if trajectory.Status == types.TrajectoryLive {
		_, err = s.ReplaceLifecycleActivation(ctx, req)
	} else {
		_, err = s.ProjectTerminalLifecycleRun(ctx, req)
	}
	if err != nil {
		return true, err
	}
	if rec.State.Terminal() {
		if reconcileErr := s.ReconcileLifecycleSettlementForTerminalRun(ctx, rec); reconcileErr != nil {
			log.Printf("store: reconcile terminal lifecycle settlement run=%s: %v", rec.RunID, reconcileErr)
		}
	}
	return true, nil
}

// CreateRun inserts a new run record.
func (s *Store) CreateRun(ctx context.Context, rec types.RunRecord) error {
	if handled, err := s.persistLifecycleRun(ctx, rec); handled {
		return err
	}
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
	return s.UpdateRunWithEvent(ctx, rec, nil)
}

// UpdateRunWithEvent updates a run and, when event is non-nil and the run is
// lifecycle-bound, folds the runtime event into the same atomic batch as the
// run projection so the state change and its choir.event commit together.
// For non-lifecycle runs the event is appended separately after the run
// write (the projection batch does not exist for them).
func (s *Store) UpdateRunWithEvent(ctx context.Context, rec types.RunRecord, event *types.EventRecord) error {
	if handled, err := s.persistLifecycleRunWithEvent(ctx, rec, event); handled {
		return err
	}
	if !rec.State.Active() {
		if err := s.UpdateRunOG(ctx, rec); err != nil {
			return err
		}
		if event != nil {
			return s.AppendEvent(ctx, event)
		}
		return nil
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
	if err := s.UpdateRunOG(ctx, rec); err != nil {
		return err
	}
	if event != nil {
		return s.AppendEvent(ctx, event)
	}
	return nil
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
	if err := s.UpdateRun(ctx, rec); err != nil {
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
		if lifecycleRunProjection(object, rec) {
			continue
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

// ListRunsBySelfDevelopmentOperation returns the unique top-level Management run
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
		if lifecycleRunProjection(object, rec) {
			continue
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
	objs, err := s.ogListAllByMetadata(ctx, ogKindRun, "channel_id", channelID)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, len(objs))
	for _, obj := range objs {
		if obj.OwnerID != ownerID {
			continue
		}
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if lifecycleRunProjection(obj, rec) || rec.ChannelID != channelID {
			continue
		}
		runs = append(runs, rec)
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if len(runs) > limit {
		runs = runs[:limit]
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
		if lifecycleRunProjection(obj, rec) {
			continue
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

// ClaimEngineeringSlot atomically claims (owner, trajectory, slot) for a Engineering
// run. If a live run already owns the slot, that run is returned and claimed is
// false. If the previous owner is terminal, the slot is advanced to runID.
func (s *Store) ClaimEngineeringSlot(ctx context.Context, ownerID, trajectoryID, slot, runID, agentID, requesterRunID string) (types.RunRecord, bool, error) {
	// One lock covers the whole decision, in both homes: the setting is read,
	// the other home is consulted and the claim is written without another
	// claim (or import) interleaving. The setting is read inside the lock so a
	// claim that waited acts on the setting as it stands when its turn comes.
	s.graphHomeMu.Lock()
	defer s.graphHomeMu.Unlock()
	if s.graphHomeOn(ctx, ownerID, SettingEngineeringSlotsGraph) {
		return s.claimEngineeringSlotGraph(ctx, ownerID, trajectoryID, slot, runID, agentID, requesterRunID)
	}
	return s.claimEngineeringSlotTable(ctx, ownerID, trajectoryID, slot, runID, agentID, requesterRunID)
}

// claimEngineeringSlotTable is the legacy home, used while
// engineering.slots_graph is off. A slot the graph holds for a live run is
// still held (a rollback must not hand out a held slot twice). The caller
// holds graphHomeMu.
func (s *Store) claimEngineeringSlotTable(ctx context.Context, ownerID, trajectoryID, slot, runID, agentID, requesterRunID string) (types.RunRecord, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	slot = strings.TrimSpace(slot)
	runID = strings.TrimSpace(runID)
	if ownerID == "" || trajectoryID == "" || slot == "" || runID == "" {
		return types.RunRecord{}, false, fmt.Errorf("claim co-super slot: owner_id, trajectory_id, slot, and run_id are required")
	}
	if held, ok, err := s.engineeringSlotGraphHolder(ctx, ownerID, trajectoryID, slot); err != nil {
		return types.RunRecord{}, false, err
	} else if ok {
		return held, false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	// Read the row first, then insert only when there is none. The old
	// INSERT ... ON DUPLICATE KEY UPDATE run_id = run_id cannot be told from a
	// fresh insert here: the driver counts the matched row, so RowsAffected was
	// 1 for a duplicate and every second claim of a held slot returned
	// claimed=true (docs/problems/data-has-more-than-one-home-2026-10-24.md,
	// the table home's claim). graphHomeMu serializes claims in this process
	// and the primary key refuses a second insert.
	existingRunID, err := s.engineeringSlotRunIDTable(ctx, ownerID, trajectoryID, slot)
	if errors.Is(err, ErrNotFound) {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO co_super_slots (owner_id, trajectory_id, slot, run_id, agent_id, requested_by_run_id, claimed_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			ownerID, trajectoryID, slot, runID, agentID, requesterRunID, now, now,
		); err != nil {
			return types.RunRecord{}, false, fmt.Errorf("insert co-super slot claim: %w", err)
		}
		return types.RunRecord{}, true, nil
	}
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

	res, err := s.db.ExecContext(ctx,
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
		existingRunID, err = s.engineeringSlotRunIDTable(ctx, ownerID, trajectoryID, slot)
		if err != nil {
			return types.RunRecord{}, false, err
		}
		existing, err := s.GetRun(ctx, existingRunID)
		return existing, false, err
	}
	return types.RunRecord{}, true, nil
}

// ReleaseEngineeringSlotClaim releases a newly claimed Engineering slot only if it is
// still owned by runID. It is used to avoid leaving a durable claim behind when
// run creation fails after the slot claim succeeds.
func (s *Store) ReleaseEngineeringSlotClaim(ctx context.Context, ownerID, trajectoryID, slot, runID string) error {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	slot = strings.TrimSpace(slot)
	runID = strings.TrimSpace(runID)
	if ownerID == "" || trajectoryID == "" || slot == "" || runID == "" {
		return fmt.Errorf("release co-super slot: owner_id, trajectory_id, slot, and run_id are required")
	}
	s.graphHomeMu.Lock()
	defer s.graphHomeMu.Unlock()
	if err := s.releaseEngineeringSlotGraph(ctx, ownerID, trajectoryID, slot, runID); err != nil {
		return err
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

// EngineeringSlotRecord is the durable trajectory-slot ownership record for a
// Engineering activation. RequestedByRunID is retained as requester provenance,
// not as the authority relation.
type EngineeringSlotRecord struct {
	OwnerID          string    `json:"owner_id"`
	TrajectoryID     string    `json:"trajectory_id"`
	Slot             string    `json:"slot"`
	RunID            string    `json:"run_id"`
	AgentID          string    `json:"agent_id"`
	RequestedByRunID string    `json:"requested_by_run_id"`
	ClaimedAt        time.Time `json:"claimed_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (s *Store) engineeringSlotRunIDTable(ctx context.Context, ownerID, trajectoryID, slot string) (string, error) {
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

// EngineeringSlotByAgent returns the most recent trajectory slot record for a
// Engineering agent. Authority callers use this instead of active-run parentage.
func (s *Store) engineeringSlotByAgentTable(ctx context.Context, ownerID, agentID string) (EngineeringSlotRecord, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return EngineeringSlotRecord{}, false, nil
	}
	var rec EngineeringSlotRecord
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
			return EngineeringSlotRecord{}, false, nil
		}
		return EngineeringSlotRecord{}, false, fmt.Errorf("query co-super slot by agent: %w", err)
	}
	return rec, true, nil
}

// EngineeringSlotByAgentAndTrajectory returns the trajectory slot record for a
// Engineering agent on a specific trajectory.
func (s *Store) engineeringSlotByAgentAndTrajectoryTable(ctx context.Context, ownerID, trajectoryID, agentID string) (EngineeringSlotRecord, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || trajectoryID == "" || agentID == "" {
		return EngineeringSlotRecord{}, false, nil
	}
	var rec EngineeringSlotRecord
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
			return EngineeringSlotRecord{}, false, nil
		}
		return EngineeringSlotRecord{}, false, fmt.Errorf("query co-super slot by agent and trajectory: %w", err)
	}
	return rec, true, nil
}

// EngineeringSlotRun returns the run currently recorded for a Engineering
// (trajectory, slot), including terminal and passivated history. Admission
// callers use this as the trajectory slot authority instead of parent-child
// ancestry.
func (s *Store) EngineeringSlotRun(ctx context.Context, ownerID, trajectoryID, slot string) (types.RunRecord, bool, error) {
	runID, err := s.engineeringSlotRunID(ctx, ownerID, trajectoryID, slot)
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

// ActiveEngineeringSlotRun returns the live run currently claiming a
// (trajectory, slot), if any. Passivated runs are reusable non-terminal
// history, not active slot owners.
func (s *Store) ActiveEngineeringSlotRun(ctx context.Context, ownerID, trajectoryID, slot string) (types.RunRecord, bool, error) {
	rec, found, err := s.EngineeringSlotRun(ctx, ownerID, trajectoryID, slot)
	if err != nil {
		return types.RunRecord{}, false, err
	}
	if !found || !rec.State.Active() {
		return types.RunRecord{}, false, nil
	}
	return rec, true, nil
}

// CountActiveEngineeringSlots returns the number of trajectory-scoped Engineering
// slots whose recorded activation is still active.
func (s *Store) CountActiveEngineeringSlots(ctx context.Context, ownerID, trajectoryID string) (int, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if ownerID == "" || trajectoryID == "" {
		return 0, nil
	}
	// The slots of the trajectory from both homes (the table and the graph,
	// the newer record per slot), then each run's state from the graph.
	slots, err := s.engineeringSlotsOfTrajectory(ctx, ownerID, trajectoryID)
	if err != nil {
		return 0, fmt.Errorf("count active co-super slots: %w", err)
	}
	count := 0
	for _, slotRec := range slots {
		rec, err := s.GetRunOG(ctx, slotRec.RunID)
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
	return count, nil
}

// GetLatestActiveRunByAgent returns the most recent non-terminal run for an agent.
// ListRunsByAgent returns all runs for an exact agent.
func (s *Store) ListRunsByAgent(ctx context.Context, ownerID, agentID string, limit int) ([]types.RunRecord, error) {
	return s.ListRunsByAgentOG(ctx, ownerID, agentID, limit)
}
func (s *Store) GetLatestRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindRun, "agent_id", agentID)
	if err != nil {
		return types.RunRecord{}, err
	}
	var latest *types.RunRecord
	for i := range objs {
		var rec types.RunRecord
		if err := ogDecode(objs[i], &rec); err != nil {
			return types.RunRecord{}, err
		}
		if lifecycleRunProjection(objs[i], rec) {
			continue
		}
		if rec.OwnerID != ownerID {
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

func (s *Store) GetLatestActiveRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindRun, "agent_id", agentID)
	if err != nil {
		return types.RunRecord{}, err
	}
	var latest *types.RunRecord
	for i := range objs {
		var rec types.RunRecord
		if err := ogDecode(objs[i], &rec); err != nil {
			return types.RunRecord{}, err
		}
		if lifecycleRunProjection(objs[i], rec) {
			continue
		}
		if rec.OwnerID != ownerID {
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
	if !latest.State.Active() {
		return types.RunRecord{}, ErrNotFound
	}
	return *latest, nil
}

// GetLatestPassivatedRunByAgent returns the most recent passivated activation
// for an actor identity. Durable actor reactivation uses this before minting a
// replacement run.
func (s *Store) GetLatestPassivatedRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindRun, "agent_id", agentID)
	if err != nil {
		return types.RunRecord{}, err
	}
	var latest *types.RunRecord
	for i := range objs {
		var rec types.RunRecord
		if err := ogDecode(objs[i], &rec); err != nil {
			return types.RunRecord{}, err
		}
		if lifecycleRunProjection(objs[i], rec) {
			continue
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

const latestRunByAgentScanCap = 8192

func (s *Store) latestLifecycleRunByAgent(ctx context.Context, ownerID, computerID, agentID string, match func(types.RunState) bool) (types.RunRecord, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return types.RunRecord{}, err
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return types.RunRecord{}, ErrLifecycleInvalidTransition
	}
	// Scheduling authority reads the PRIMARY object graph, not the read pool:
	// the read pool's transaction snapshot can predate a just-committed mint,
	// and a stale "no resident" answer here mints a second Management and breaks the
	// I26 singleton (one active Management run per computer). This query is bounded
	// (refs scan + single object loads), so primary reads are proportionate.
	graph := s.ogStore
	if graph == nil {
		graph = s.ogReadStore
	}
	if graph == nil {
		return types.RunRecord{}, fmt.Errorf("lifecycle runs: object graph not initialized")
	}
	refs, err := graph.ListObjectRefsByKindOwner(ctx, string(ogKindRun), ownerID, latestRunByAgentScanCap)
	if err != nil {
		return types.RunRecord{}, err
	}
	for _, ref := range refs {
		if ref.Tombstone {
			continue
		}
		if computerID != "" && strings.TrimSpace(ref.ComputerID) != "" && strings.TrimSpace(ref.ComputerID) != computerID {
			continue
		}
		meta := map[string]any{}
		if len(ref.Metadata) > 0 {
			if err := json.Unmarshal(ref.Metadata, &meta); err != nil {
				return types.RunRecord{}, fmt.Errorf("latest lifecycle run: decode metadata %s: %w", ref.CanonicalID, err)
			}
		}
		if ogMetadataText(meta, "agent_id") != agentID {
			continue
		}
		obj, err := graph.GetObject(ctx, ref.CanonicalID)
		if err != nil {
			return types.RunRecord{}, fmt.Errorf("latest lifecycle run: get %s: %w", ref.CanonicalID, err)
		}
		rec, decodeErr := decodeLifecycleObject[types.RunRecord](obj)
		if decodeErr != nil {
			return types.RunRecord{}, decodeErr
		}
		if rec.OwnerID != ownerID || rec.ComputerID != computerID || rec.AgentID != agentID {
			continue
		}
		if match != nil && !match(rec.State) {
			return types.RunRecord{}, ErrNotFound
		}
		return rec, nil
	}
	return types.RunRecord{}, ErrNotFound
}

func (s *Store) GetLatestActiveLifecycleRunByAgent(ctx context.Context, ownerID, computerID, agentID string) (types.RunRecord, error) {
	return s.latestLifecycleRunByAgent(ctx, ownerID, computerID, agentID, types.RunState.Active)
}

func (s *Store) GetLatestPassivatedLifecycleRunByAgent(ctx context.Context, ownerID, computerID, agentID string) (types.RunRecord, error) {
	return s.latestLifecycleRunByAgent(ctx, ownerID, computerID, agentID, func(state types.RunState) bool {
		return state == types.RunPassivated
	})
}

// allocateEventSeqLocked assigns rec.StreamSeq (the next store-wide number,
// always at or below the durable lease) and rec.Seq (the next number for the
// run) from the in-memory high-water marks, seeding them from the store on
// first use. The caller holds eventMu, which the stream_seq allocation
// releases while it waits for a seed or a refill and holds again on return;
// it takes the stream number first, so a wait never leaves a run's seq ahead
// of its stream_seq. On a stream error nothing is taken. folded is the
// AllocateEventSeq caller: a record that gained both numbers while it waited
// keeps them.
// It is the only allocator of both numbers: AppendEvent calls it, and so does
// AllocateEventSeq for the events folded into a lifecycle batch. sl is the
// telemetry sealer or nil.
func (s *Store) allocateEventSeqLocked(ctx context.Context, rec *types.EventRecord, sl *telemetrySealer, folded bool) (eventSeqKey, error) {
	// OG requires a non-empty owner_id. Synthesize a system owner for
	// ownerless runtime events (e.g. health/degraded events from
	// Runtime.SetHealth).
	if rec.OwnerID == "" {
		rec.OwnerID = "__system__"
	}
	took, err := s.allocateStreamSeqLocked(ctx, rec, folded)
	if err != nil {
		return eventSeqKey{}, err
	}
	if !took {
		// A folded record another caller numbered while this one waited for
		// the lease keeps its numbers: callers sharing one record take one.
		return eventSeqKey{}, nil
	}

	// Next sequence number for this run: an in-memory high-water mark, seeded
	// from the store on the run's first append in this process. Reading and
	// decoding every earlier event of the run on each append made a run's
	// appends O(n²) (4.5 ms at 1 event to 25.6 ms at 800, measured;
	// guest-cpu-is-unindexed-store-scans-2026-10-14, lever C5).
	key := eventSeqKey{runID: rec.RunID, ownerID: rec.OwnerID}
	maxSeq, seeded := s.eventSeqs[key]
	if !seeded {
		existing, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", rec.RunID, 10000)
		if err != nil {
			return eventSeqKey{}, fmt.Errorf("append event: list existing: %w", err)
		}
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
		// Events stamped for choir_events are not in og_objects: seed from
		// both, or a number is handed out twice (event-table design 3.5).
		tableSeq, err := s.eventTableRunMaxSeq(ctx, rec.RunID)
		if err != nil {
			return eventSeqKey{}, err
		}
		maxSeq = max(maxSeq, tableSeq)
		// The trace log is a third source (trace-log design 4.4): a line it
		// holds may be on no tape, and a number lost with a VM crash may
		// already have been delivered live. A run the log has no line for is
		// seeded exactly as before.
		logSeq := s.traceLogRunMaxSeq(rec.OwnerID, rec.RunID)
		maxSeq = max(maxSeq, logSeq)
		if sl != nil || logSeq > 0 {
			// With the sealer a number can be published and never stored, so a
			// seed from the store alone can hand it out twice (design 6.1).
			// Skip a gap over anything the previous process could have lost.
			if maxSeq > 0 {
				maxSeq += telemetrySeqGap
			}
		}
		if sl != nil {
			// And never go below a number the queue still holds.
			if floor := sl.seqFloor(key); floor > maxSeq {
				maxSeq = floor
			}
		}
		if s.eventSeqs == nil || len(s.eventSeqs) >= eventSeqCacheLimit {
			s.eventSeqs = map[eventSeqKey]int64{}
		}
	}
	rec.Seq = maxSeq + 1
	s.eventSeqs[key] = rec.Seq
	return key, nil
}

// streamSeedWindows are the sizes, in objects of any kind, of the windows
// the lease's cross-check reads through the newest objects looking for an
// event. A variable so a test can shrink it.
var streamSeedWindows = []int{2000, 50000}

// AllocateEventSeq gives an event that is written by some path other than
// AppendEvent its seq and stream_seq. Run-state events folded into a lifecycle
// projection batch (UpdateRunWithEvent) use it, so they are numbered like
// every other run event and the seq-filtered readers see them
// (docs/problems/run-events-without-seq-2026-10-22.md). An event that already
// has both numbers keeps them, so a retry with the same event takes no second
// number; the check is under eventMu, so callers sharing one record take one
// number. A batch that then fails leaves a gap, never a duplicate.
func (s *Store) AllocateEventSeq(ctx context.Context, rec *types.EventRecord) error {
	if rec == nil {
		return nil
	}
	sl := s.telemetry.Load()
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	if rec.Seq != 0 && rec.StreamSeq != 0 {
		return nil
	}
	_, err := s.allocateEventSeqLocked(ctx, rec, sl, true)
	return err
}

// AppendEvent appends an event record with an auto-assigned sequence number.
// The Seq field on the input record is overwritten with the next sequence
// number for the run.
func (s *Store) AppendEvent(ctx context.Context, rec *types.EventRecord) error {
	if len(rec.Payload) == 0 {
		rec.Payload = json.RawMessage(`{}`)
	}
	s.telemetryStats.recordAppend(rec.Kind, len(rec.Payload))

	// Telemetry batching (docs/reports/telemetry-batching-design-2026-10-20.md).
	// sl is nil unless batching was ever configured on in this process, and
	// then every line below that mentions it is skipped: the flag-off path is
	// the code that was here before. batched means this record is queued, not
	// written: only a batched kind, only on the standalone path (this
	// function), only with the sealer accepting records and a boot watermark
	// already durable. The slot is reserved here, before eventMu, because
	// reserving may flush and a flush must never run under eventMu.
	sl := s.telemetry.Load()
	batched := false
	if sl != nil && s.projectionTape != nil && rec.Kind.Batchable() && sl.accepting() {
		batched = s.ensureBootWatermark(ctx, sl) && sl.reserve()
	}
	reservationHeld := batched

	// Serialize event sequence allocation to prevent two concurrent
	// goroutines from reading the same max and assigning duplicate Seq
	// or StreamSeq values. The old SQL path relied on UNIQUE(loop_id, seq).
	s.eventMu.Lock()
	unlocked := false
	defer func() {
		if !unlocked {
			s.eventMu.Unlock()
		}
		if reservationHeld {
			sl.release() // an early return before enqueue gives the slot back
		}
	}()
	tapeBound := s.projectionTape != nil

	key, err := s.allocateEventSeqLocked(ctx, rec, sl, false)
	if err != nil {
		return err
	}

	// The event-table stamp (store.event_table) is decided once, here, with
	// the numbers: the tape then says which table the event lands in.
	stamp := s.stampsEventTable(rec.Kind)

	if batched {
		queued, err := newTelemetryRecord(rec, stamp)
		if err == nil {
			// The refusals the acknowledged write would have made at this
			// moment, so a refused write still reaches its caller.
			err = s.ogStore.PreflightWrite(ctx, []objectgraph.Object{queued.object}, nil)
		}
		if err != nil {
			return err // numbers stay allocated: a gap, never a duplicate
		}
		reason, triggered := sl.enqueue(queued) // consumes the reservation
		reservationHeld = false
		s.telemetryStats.recordSink(rec.Kind, sinkTape) // queued for the tape: the sealer is a tape writer
		s.eventMu.Unlock()
		unlocked = true
		sl.afterEnqueue(reason, triggered)
		return nil
	}

	if tapeBound {
		s.eventMu.Unlock()
		unlocked = true
	}
	appendOne := s.AppendEventOG
	if stamp {
		appendOne = s.appendStampedEvent
	}
	if err := appendOne(ctx, rec); err != nil {
		// The sequence was not written: forget the mark so the run's next
		// append re-reads the store and takes this number again, as the
		// per-append read did. If a concurrent append to the run (possible
		// once the tape path released eventMu) already took a later number,
		// keep the mark: a gap, never a duplicate. With the sealer the mark is
		// kept always: the number may sit in a queue as well (design 6.1).
		if unlocked {
			s.eventMu.Lock()
			defer s.eventMu.Unlock()
		}
		if sl == nil && s.eventSeqs[key] == rec.Seq {
			delete(s.eventSeqs, key)
		}
		return err
	}
	s.telemetryStats.recordSink(rec.Kind, sinkTape)
	return nil
}

// ListEvents returns events for the given run, ordered by sequence ascending.
func (s *Store) ListEvents(ctx context.Context, runID string, limit int) ([]types.EventRecord, error) {
	return s.ListEventsAfter(ctx, runID, 0, limit)
}

// ListEventsAfter returns events for the given run with sequence > afterSeq,
// ordered by sequence ascending, limited to the given count. It reads the
// union of og_objects and choir_events (event-table design 4.4); the cursor
// and evidence-link semantics are unchanged.
func (s *Store) ListEventsAfter(ctx context.Context, runID string, afterSeq int64, limit int) ([]types.EventRecord, error) {
	s.beforeEventRead(ctx)
	if limit <= 0 {
		limit = 200
	}
	// The log half is read first: a line is in the log as soon as its number
	// is allocated, and a cursor must not move past an event still on its way
	// to the tape (design 6.2).
	logHalf := s.traceLogHalfRun(runID, afterSeq, limit)
	query, args := eventTableRunAfterSQL(runID, afterSeq, limit)
	first, err := s.readEventTable(ctx, "store.event_table.list_run_after", query, args)
	if err != nil {
		return nil, err
	}
	// Fetch a large window since ogListByMetadata orders by updated_at
	// DESC, not by seq. We need all events to filter by seq and sort.
	objs, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", runID, 100000)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	og, err := objectEventEntries(objs)
	if err != nil {
		return nil, err
	}
	kept := og[:0]
	for _, e := range og {
		if e.rec.Seq > afterSeq {
			kept = append(kept, e)
		}
	}
	if s.eventUnionHook != nil {
		s.eventUnionHook("after_object_graph")
	}
	again, err := s.readEventTable(ctx, "store.event_table.list_run_after", query, args)
	if err != nil {
		return nil, err
	}
	table := cursorTableHalf(first, kept, again, func(r types.EventRecord) int64 { return r.Seq }, afterSeq)
	events := mergeTraceLogHalf(unionEvents(kept, table), logHalf)
	sort.Slice(events, func(i, j int) bool {
		if events[i].rec.Seq != events[j].rec.Seq {
			return events[i].rec.Seq < events[j].rec.Seq
		}
		return events[i].rec.StreamSeq < events[j].rec.StreamSeq
	})
	return eventRecords(events, limit), nil
}

// ListEventsByOwner returns events for the given owner, ordered by timestamp
// descending, limited to the given count. It reads the union: the
// og_objects half is the newest by timestamp, the choir_events half the
// newest by stream_seq. They differ only at the cut, and only where callers'
// timestamps disagree with allocation order (event-table design 4.4).
func (s *Store) ListEventsByOwner(ctx context.Context, ownerID string, limit int) ([]types.EventRecord, error) {
	s.beforeEventRead(ctx)
	if limit <= 0 {
		limit = 200
	}
	logHalf := s.traceLogHalfOwnerNewest(ownerID, limit)
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindEvent,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("query events by owner: %w", err)
	}
	og, err := objectEventEntries(objs)
	if err != nil {
		return nil, err
	}
	query, args := eventTableOwnerNewestSQL(ownerID, limit)
	table, err := s.readEventTable(ctx, "store.event_table.list_owner_newest", query, args)
	if err != nil {
		return nil, err
	}
	events := mergeTraceLogHalf(unionEvents(og, table), logHalf)
	sort.Slice(events, func(i, j int) bool {
		if !events[i].rec.Timestamp.Equal(events[j].rec.Timestamp) {
			return events[i].rec.Timestamp.After(events[j].rec.Timestamp)
		}
		return events[i].rec.StreamSeq > events[j].rec.StreamSeq
	})
	return eventRecords(events, limit), nil
}

// ListEventsByOwnerAfter returns events for the given owner with stream_seq >
// afterSeq across all runs, ordered by stream_seq ascending, limited to the
// given count. This supports SSE catch-up after reconnection where the client
// needs events newer than a previously seen sequence number. It reads the
// union of og_objects and choir_events.
func (s *Store) ListEventsByOwnerAfter(ctx context.Context, ownerID string, afterSeq int64, limit int) ([]types.EventRecord, error) {
	s.beforeEventRead(ctx)
	if limit <= 0 {
		limit = 200
	}
	logHalf := s.traceLogHalfOwnerAfter(ownerID, afterSeq, limit)
	query, args := eventTableOwnerAfterSQL(ownerID, afterSeq, limit)
	first, err := s.readEventTable(ctx, "store.event_table.list_owner_after", query, args)
	if err != nil {
		return nil, err
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
	og, err := objectEventEntries(objs)
	if err != nil {
		return nil, err
	}
	kept := og[:0]
	for _, e := range og {
		if e.rec.StreamSeq > afterSeq {
			kept = append(kept, e)
		}
	}
	if s.eventUnionHook != nil {
		s.eventUnionHook("after_object_graph")
	}
	again, err := s.readEventTable(ctx, "store.event_table.list_owner_after", query, args)
	if err != nil {
		return nil, err
	}
	table := cursorTableHalf(first, kept, again, func(r types.EventRecord) int64 { return r.StreamSeq }, afterSeq)
	events := mergeTraceLogHalf(unionEvents(kept, table), logHalf)
	sort.Slice(events, func(i, j int) bool { return events[i].rec.StreamSeq < events[j].rec.StreamSeq })
	return eventRecords(events, limit), nil
}

// ListAcknowledgedEventsByChannel returns recent events for the given
// coordination channel. It reads og_objects only, so for events written
// after the event-table cutover it covers acknowledged kinds only: batched
// kinds carry the run's channel but are stamped into choir_events, which has
// no channel index (event-table design 1.2, Q9). Its one call site,
// System1's channel tape, serves three semantic readers (the System1
// judgments, the story.relation records and the story.entities records), all
// acknowledged kinds; a caller of a
// batched kind must add a channel index or a union (pinned by the call-site
// audit).
func (s *Store) ListAcknowledgedEventsByChannel(ctx context.Context, ownerID, channelID string, limit int) ([]types.EventRecord, error) {
	s.beforeEventRead(ctx)
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
	// Equal timestamps read in write order (StreamSeq), so records written in
	// one instant, such as a replay and the record it follows, never swap.
	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].Timestamp.Equal(events[j].Timestamp) {
			return events[i].Timestamp.Before(events[j].Timestamp)
		}
		return events[i].StreamSeq < events[j].StreamSeq
	})
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

// eventTrajectoryWindow is the newest-events window ListEventsByTrajectory
// reads before it filters and cuts.
const eventTrajectoryWindow = 10000

// ListEventsByTrajectory returns events for a specific user trajectory: of
// the trajectory's newest 10,000 events by timestamp, the owner's, the
// oldest limit by stream_seq (as before F-c; whether acceptance should read
// the newest is a reader question of its own, event-table design heresy
// delta). It rebuilds that window over the union first, so a trajectory
// whose events span both tables reads exactly what one table would give.
func (s *Store) ListEventsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.EventRecord, error) {
	s.beforeEventRead(ctx)
	if limit <= 0 {
		limit = 500
	}
	logHalf := s.traceLogHalfTrajectory(ownerID, trajectoryID, eventTrajectoryWindow)
	// Fetch a large window since ogListByMetadata orders by updated_at DESC.
	objs, err := s.ogListByMetadata(ctx, ogKindEvent, "trajectory_id", trajectoryID, eventTrajectoryWindow)
	if err != nil {
		return nil, fmt.Errorf("query events by trajectory: %w", err)
	}
	og, err := objectEventEntries(objs)
	if err != nil {
		return nil, err
	}
	query, args := eventTableTrajectoryNewestSQL(trajectoryID, eventTrajectoryWindow)
	table, err := s.readEventTable(ctx, "store.event_table.list_trajectory", query, args)
	if err != nil {
		return nil, err
	}
	window := mergeTraceLogHalf(unionEvents(og, table), logHalf)
	sort.Slice(window, func(i, j int) bool {
		if !window[i].updatedAt.Equal(window[j].updatedAt) {
			return window[i].updatedAt.After(window[j].updatedAt)
		}
		return window[i].rec.StreamSeq > window[j].rec.StreamSeq
	})
	if len(window) > eventTrajectoryWindow {
		window = window[:eventTrajectoryWindow]
	}
	events := window[:0]
	for _, e := range window {
		if e.rec.OwnerID == ownerID {
			events = append(events, e)
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].rec.StreamSeq != events[j].rec.StreamSeq {
			return events[i].rec.StreamSeq < events[j].rec.StreamSeq
		}
		return events[i].rec.Seq < events[j].rec.Seq
	})
	return eventRecords(events, limit), nil
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
	key := strings.TrimSpace(message.IdempotencyKey)
	for _, obj := range existing {
		var msg types.ChannelMessage
		if err := ogDecode(obj, &msg); err != nil {
			continue
		}
		if msg.Seq > maxSeq {
			maxSeq = msg.Seq
		}
		if key == "" || strings.TrimSpace(msg.IdempotencyKey) != key {
			continue
		}
		if msg.Content != message.Content || msg.ToAgentID != message.ToAgentID || msg.FromAgentID != message.FromAgentID {
			return fmt.Errorf("append channel message: idempotency key %q conflicts", key)
		}
		*message = msg
		message.Replayed = true
		return nil
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

// ListChannelMessagesTo returns channel messages addressed to one agent
// across all sender channels, ordered by sequence ascending. Companion to
// ListChannelMessages: emits travel on the sender's channel, so a recipient
// desk's own-channel read misses them — this is the to-agent drain read.
func (s *Store) ListChannelMessagesTo(ctx context.Context, ownerID, toAgentID string, afterSeq int64, limit int) ([]types.ChannelMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	msgs, err := s.ListChannelMessagesToOG(ctx, ownerID, toAgentID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("query channel messages to agent: %w", err)
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
	// Exhaust the kind before excluding lifecycle and delivered rows; applying a
	// fixed window first can mask older valid legacy updates.
	objs, err := s.ogListAllObjectsByKind(ctx, objectgraph.ObjectKind("choir.worker_update"))
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
			continue
		}
		if rec.DeliveredToRunID != "" {
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
	// Exhaust the target keyset before excluding lifecycle rows and applying
	// cursor/order/limit semantics in memory.
	objs, err := s.ogListAllByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "target_agent_id", targetAgentID)
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
		// Durable lifecycle mailboxes are computer-scoped and must be consumed
		// through ListPendingLifecycleUpdates.
		if rec.LifecycleVersion > 0 {
			continue
		}
		if rec.DeliveredToRunID != "" && rec.MessageSeq <= cursor {
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
	if limit < 0 {
		return nil, fmt.Errorf("query coagent mailbox backlog all: limit must not be negative")
	}
	if s.ogStore == nil {
		return nil, fmt.Errorf("query coagent mailbox backlog all: object graph not initialized")
	}
	var updates []types.CoagentSourcePacket
	cursorByTarget := make(map[string]int64)
	after := ""
	for {
		objs, err := s.ogStore.ListObjectsPage(ctx, "choir.worker_update", after, 1000)
		if err != nil {
			return nil, fmt.Errorf("query coagent mailbox backlog all: %w", err)
		}
		for _, obj := range objs {
			var rec types.CoagentSourcePacket
			if err := ogDecode(obj, &rec); err != nil {
				return nil, err
			}
			// Lifecycle subjects are re-warmed through their scoped registry;
			// this owner/agent-only sweep is legacy mailbox authority.
			if rec.LifecycleVersion > 0 {
				continue
			}
			key := rec.OwnerID + "\x00" + rec.TargetAgentID
			cursor, found := cursorByTarget[key]
			if !found {
				var cursorErr error
				cursor, _, _, cursorErr = s.GetCoagentMailboxCursor(ctx, rec.OwnerID, rec.TargetAgentID)
				if cursorErr != nil {
					return nil, fmt.Errorf("query coagent mailbox backlog all: read cursor for %s/%s: %w", rec.OwnerID, rec.TargetAgentID, cursorErr)
				}
				cursorByTarget[key] = cursor
			}
			if rec.DeliveredToRunID != "" && rec.MessageSeq <= cursor {
				continue
			}
			updates = append(updates, rec)
		}
		if len(objs) < 1000 {
			break
		}
		after = objs[len(objs)-1].CanonicalID
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
	if limit > 0 && len(updates) > limit {
		updates = updates[:limit]
	}
	return updates, nil
}

// CountPendingWorkerUpdatesByTrajectory returns undelivered updates for the
// silent-stall oracle.
func (s *Store) CountPendingWorkerUpdatesByTrajectory(ctx context.Context, ownerID, trajectoryID string) (int, error) {
	objs, err := s.ogListAllByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "trajectory_id", trajectoryID)
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
			continue
		}
		if rec.DeliveredToRunID == "" {
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
	// Exhaust matching agent IDs before filtering by owner. A limited
	// pre-filter window can hide this owner's cursor behind other owners.
	objs, err := s.ogListAllByMetadata(ctx, ogKindCoagentMail, "agent_id", agentID)
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
	objs, err := s.ogListAllByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "target_agent_id", agentID)
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
func workerMailboxAllowsAssignedEngineeringManagementReport(update types.CoagentSourcePacket) bool {
	ownerID := strings.TrimSpace(update.OwnerID)
	if ownerID == "" {
		return false
	}
	reportRole, _ := agentprofile.Canonical(update.Role)
	return update.Direction == types.LifecyclePacketDirectionProducerReport &&
		reportRole == agentprofile.Engineering &&
		strings.TrimSpace(update.TargetAgentID) == agentprofile.Management+":"+ownerID
}

func (s *Store) DispatchWorkerUpdate(ctx context.Context, update types.CoagentSourcePacket, message *types.ChannelMessage) (types.CoagentSourcePacket, bool, error) {
	// Serialize the entire dispatch path to preserve idempotency:
	if strings.TrimSpace(update.ComputerID) != "" && strings.TrimSpace(update.TrajectoryID) != "" {
		if _, err := s.GetLifecycleTrajectory(ctx, update.OwnerID, update.ComputerID, update.TrajectoryID); err == nil {
			if !workerMailboxAllowsAssignedEngineeringManagementReport(update) {
				return types.CoagentSourcePacket{}, false, ErrLifecycleAuthorityRequired
			}
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

// EngineMutexStats returns cumulative per-call-path counters for the
// embedded-Dolt engine lock shared by the write and read pools. Exposed via
// guest /health so the latency mission's drain probe can distinguish engine
// lock queueing (wait) from lock-held SQL work (hold) per call path.
func (s *Store) EngineMutexStats() map[string]objectgraph.EngineMutexOpStats {
	if s == nil || s.ogStore == nil {
		return nil
	}
	return s.ogStore.EngineMutexStats()
}

// EngineMutexLive returns who holds the engine lock now and who waits for it.
func (s *Store) EngineMutexLive() *objectgraph.EngineMutexLive {
	if s == nil || s.ogStore == nil {
		return nil
	}
	return s.ogStore.EngineMutexLive()
}

// marshalJSON marshals a value to JSON, returning "{}" for nil.
func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(v)
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

func (s *Store) ensureDesktopPrimaryKeys() error {
	for table, cols := range map[string][]string{
		"desktop_workspaces":        {"owner_id", "computer_id", "desktop_id"},
		"desktop_sessions":          {"owner_id", "computer_id", "desktop_id", "session_id"},
		"desktop_app_instances":     {"owner_id", "computer_id", "desktop_id", "app_instance_id"},
		"desktop_window_placements": {"owner_id", "computer_id", "desktop_id", "session_id", "app_instance_id"},
	} {
		rows, err := s.db.Query(`
SELECT column_name
FROM information_schema.statistics
WHERE table_schema = DATABASE()
  AND table_name = ?
  AND index_name = 'PRIMARY'
ORDER BY seq_in_index`, table)
		if err != nil {
			return fmt.Errorf("inspect %s primary key: %w", table, err)
		}
		var current []string
		for rows.Next() {
			var col string
			if err := rows.Scan(&col); err != nil {
				rows.Close()
				return err
			}
			current = append(current, col)
		}
		rows.Close()
		if !slices.Equal(current, cols) && len(current) > 0 {
			if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s DROP PRIMARY KEY, ADD PRIMARY KEY (%s)", table, strings.Join(cols, ", "))); err != nil {
				return fmt.Errorf("scope %s primary key: %w", table, err)
			}
		}
	}
	return nil
}
