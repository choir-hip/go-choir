package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

// The guest store is one SQLite file (StateDBPath) opened through dbx: a
// single serialized write connection and a read pool, WAL, synchronous=FULL.
// SQL COMMIT is the durability point; there is no commit history. A sidecar
// directory beside the file (TexturePath, the legacy "<base>.texture" name)
// holds the file-shaped metadata the updater and the lease logic read: the
// schema receipt, the store floor and the lease untrust marker
// (internal/storeschema), plus the vocabulary report and the deposit upcast
// ledger. Keeping that directory means the updater's CHOIR_STORE_SCHEMA_PATH
// and the floor survive the Dolt to SQLite migration unchanged.

// ErrCloseLeftOpen is Close's answer when a statement still holds the write
// connection after the close deadline: the pools stay open and Close can be
// called again.
var ErrCloseLeftOpen = errors.New("runtime store: close left the store open")

// ErrPoolStatementLive names the cause of ErrCloseLeftOpen: a statement or
// transaction holds the write connection.
var ErrPoolStatementLive = fmt.Errorf("%w: a statement holds the write connection", ErrCloseLeftOpen)

// closeDeadline bounds how long Close waits for the write connection.
const closeDeadline = 10 * time.Second

// testHookAfterWorkspaceOpen runs in Open right after the database is up
// and the store exists; an error from it fails the Open (tests only).
var testHookAfterWorkspaceOpen func(s *Store) error

// closeStoreForFailedOpen is the Close closeFailedOpen runs (a seam for
// tests).
var closeStoreForFailedOpen = func(s *Store) error { return s.Close() }

// closeFailedOpen closes a store whose Open failed and returns Open's error.
// A Close that fails closed is retried a bounded number of times; a Close
// error that remains is logged and joined to Open's.
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

// bootSubstep runs one named step of a boot phase and logs its start, end and
// duration, so a step that hangs is named by the last line that has no end.
func bootSubstep(phase, name string, fn func() error) error {
	done := objectgraph.BootPhase("store: open", phase+"/"+name)
	err := fn()
	done(err)
	return err
}

// OpenFresh creates a new store only when its SQLite file and sidecar
// directory can both be exclusively reserved among OpenFresh callers. It is
// the required constructor for disposable replay projections; it never
// removes an existing store to make the call succeed.
func OpenFresh(dbPath string) (*Store, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("%w: path is required", ErrFreshWorkspaceRequiresEmpty)
	}
	statePath := StateDBPath(dbPath)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return nil, fmt.Errorf("runtime store: create fresh parent directory: %w", err)
	}
	sidecar := deriveTextureWorkspacePath(dbPath)
	if _, err := os.Stat(sidecar); err == nil {
		return nil, fmt.Errorf("%w: %s exists", ErrFreshWorkspaceRequiresEmpty, sidecar)
	}
	// An empty file is a valid new SQLite database; O_EXCL reserves it.
	reserve, err := os.OpenFile(statePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("%w: %s exists", ErrFreshWorkspaceRequiresEmpty, statePath)
		}
		return nil, fmt.Errorf("runtime store: reserve fresh state file: %w", err)
	}
	if err := reserve.Close(); err != nil {
		_ = os.Remove(statePath)
		return nil, fmt.Errorf("runtime store: close fresh state file: %w", err)
	}
	return Open(dbPath)
}

// WorkspaceReplaceReceipt is the loss/quarantine record for a VM-local
// workspace replacement. It is evidence, not a checkpoint or restore authority.
type WorkspaceReplaceReceipt struct {
	OriginalPath string `json:"original_path"`
	// WorkspacePath is the sidecar directory (TexturePath); the SQLite file
	// is StatePath.
	WorkspacePath        string `json:"workspace_path"`
	StatePath            string `json:"state_path"`
	QuarantineDir        string `json:"quarantine_dir"`
	QuarantinedMarker    string `json:"quarantined_marker"`
	QuarantinedWorkspace string `json:"quarantined_workspace"`
}

// ReplaceWorkspace closes the current store, moves its SQLite file and
// sidecar directory into an exclusive quarantine directory, and opens a
// current-DDL store at the original path through OpenFresh. It appends no
// event and copies no rows. Failure after quarantine leaves the original path
// empty; the quarantined bytes are the rollback ref.
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
	statePath := StateDBPath(originalPath)
	workspacePath := deriveTextureWorkspacePath(originalPath)
	receipt.OriginalPath = originalPath
	receipt.WorkspacePath = workspacePath
	receipt.StatePath = statePath
	receipt.QuarantineDir = quarantineDir
	receipt.QuarantinedMarker = filepath.Join(quarantineDir, filepath.Base(statePath))
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
	if err := os.Rename(statePath, receipt.QuarantinedMarker); err != nil && !os.IsNotExist(err) {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: quarantine state file: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Rename(statePath+suffix, receipt.QuarantinedMarker+suffix); err != nil && !os.IsNotExist(err) {
			return nil, receipt, fmt.Errorf("runtime store: replace workspace: quarantine state file%s: %w", suffix, err)
		}
	}
	if err := os.Rename(workspacePath, receipt.QuarantinedWorkspace); err != nil && !os.IsNotExist(err) {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: quarantine workspace: %w", err)
	}
	fresh, err := OpenFresh(originalPath)
	if err != nil {
		return nil, receipt, fmt.Errorf("runtime store: replace workspace: open current schema: %w", err)
	}
	// Benign today (the fresh store has no lease row, and the runtime
	// restarts afterwards), and it makes the swap-point guard uniform.
	if err := storeschema.WriteLeaseUntrusted(fresh.TexturePath(), leaseMarkerReplace); err != nil {
		return nil, receipt, errors.Join(fmt.Errorf("runtime store: replace workspace: lease untrust marker: %w", err), fresh.Close())
	}
	return fresh, receipt, nil
}

// Open opens (or creates) the SQLite state file derived from dbPath
// (StateDBPath) and applies the current schema. A legacy Dolt workspace or
// SQLite file at the old paths is never read here; the guest migrator
// copies a legacy workspace into the state file
// before Open runs.
func Open(dbPath string) (*Store, error) {
	s, err := openStore(dbPath)
	if err != nil {
		return nil, err
	}
	s.startProjectionBatchCatchUp()
	return s, nil
}

// openStore is Open without the background work, which belongs to the store
// that is published: Reopen opens a scratch store and moves its fields into
// the receiver, so a goroutine started on the scratch would read a zeroed
// store.
func openStore(dbPath string) (*Store, error) {
	statePath := StateDBPath(dbPath)
	donePrepare := objectgraph.BootPhase("store: open", "prepare-path")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		donePrepare(err)
		return nil, fmt.Errorf("runtime store: create directory: %w", err)
	}
	sidecar := deriveTextureWorkspacePath(dbPath)
	stateInfo, statErr := os.Stat(statePath)
	if statErr != nil && !os.IsNotExist(statErr) {
		donePrepare(statErr)
		return nil, fmt.Errorf("runtime store: stat state file: %w", statErr)
	}
	freshStore := statErr != nil || stateInfo.Size() == 0
	donePrepare(nil)
	log.Printf("store: open fresh=%t", freshStore)

	// The receipt this store carried before this Open rewrites it. It is
	// read here, before anything can change it: receipt 3 written over a
	// receipt 2 would hide that a binary that does not know the stream_seq
	// lease ran in between (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md,
	// section 3). The decision to write the untrust marker is made below, just
	// before the receipt is rewritten.
	priorReceipt, priorFound, priorReadErr := storeschema.Read(filepath.Join(sidecar, storeschema.File))
	priorHadData := !freshStore || workspaceHoldsData(sidecar)

	// The store floor (storeschema/floor.go): a binary below it never opens
	// the store, and refuses before any DDL or receipt write.
	if err := storeschema.CheckFloor(sidecar); err != nil {
		return nil, err
	}

	doneOpen := objectgraph.BootPhase("store: open", "workspace-open")
	sdb, err := dbx.OpenSQLite(statePath, dbx.SQLiteOptions{})
	doneOpen(err)
	if err != nil {
		return nil, fmt.Errorf("runtime store: open sqlite state: %w", err)
	}
	if err := os.MkdirAll(sidecar, 0o755); err != nil {
		_ = sdb.Close()
		return nil, fmt.Errorf("runtime store: create sidecar directory: %w", err)
	}

	s := &Store{db: sdb.Write, readDB: sdb.Read, sqlite: sdb, path: dbPath, statePath: statePath, texturePath: sidecar}
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

	// Initialize object-graph persistence in the same SQLite file: the write
	// store over the single writer, the read store over the read pool so OG
	// reads do not queue behind a write transaction.
	ogWriteStore := objectgraph.NewSQLStore(s.db)
	doneOGSchema := objectgraph.BootPhase("store: open", "objectgraph-schema")
	if err := ogWriteStore.EnsureSchema(context.Background()); err != nil {
		doneOGSchema(err)
		return nil, s.closeFailedOpen(fmt.Errorf("runtime store: bootstrap object graph: %w", err))
	}
	doneOGSchema(nil)
	s.ogStore = ogWriteStore
	ogWriteStore.SetWriteValidator(s.vocabWriteGuard)
	// A persisted migration report means this store already cut over: the
	// write guard activates immediately on reopen.
	s.armVocabFenceFromReport()
	if s.readDB != nil {
		s.ogReadStore = objectgraph.NewSQLStore(s.readDB)
		// Share the write store's validated derived-key families so the read
		// pool uses the same index paths.
		s.ogReadStore.ShareDerivedKeys(ogWriteStore)
	}
	s.og = objectgraph.NewService(objectgraph.Config{
		Durable: ogWriteStore,
	})

	// Persist the running build's store schema epoch beside the state file so
	// the updater's pre-mutation state-compat gate (S2-d) can refuse a release
	// whose binary does not understand this store.
	doneEpoch := objectgraph.BootPhase("store: open", "store-schema-epoch")
	// A prior receipt below the epoch that writes leases (or unreadable, or
	// absent on a store that holds data) writes the untrust marker, durably,
	// BEFORE the receipt is rewritten. A marker-write failure aborts Open
	// here: there is no state in which receipt 3 exists without the marker
	// the old receipt called for.
	if reason := priorReceiptUntrustReason(priorFound, priorReceipt.Version, priorReadErr, priorHadData); reason != "" {
		if err := storeschema.WriteLeaseUntrusted(sidecar, reason); err != nil {
			doneEpoch(err)
			return nil, s.closeFailedOpen(err)
		}
	}
	if err := storeschema.Write(sidecar); err != nil {
		doneEpoch(err)
		return nil, s.closeFailedOpen(err)
	}
	// A store holding stamped events (a replay or base install into a fresh
	// store) is floored here; a one-row probe, not a scan.
	if err := s.ensureEventTableFloor(); err != nil {
		doneEpoch(err)
		return nil, s.closeFailedOpen(err)
	}
	doneEpoch(nil)

	s.initLeaseObs()

	return s, nil
}

// bootstrap applies the current schema to the database. The schema is one
// clean DDL (schema.go); the version row refuses a database written by a
// newer schema.
func (s *Store) bootstrap() error {
	return bootSubstep("runtime-schema", "apply-ddl", func() error {
		ctx := context.Background()
		if _, err := s.db.ExecContext(ctx, storeMetaDDL); err != nil {
			return fmt.Errorf("apply store_meta: %w", err)
		}
		var raw string
		err := s.db.QueryRowContext(ctx, `SELECT value FROM store_meta WHERE key = 'schema_version'`).Scan(&raw)
		switch {
		case err == nil:
			var version int
			if _, scanErr := fmt.Sscanf(raw, "%d", &version); scanErr != nil {
				return fmt.Errorf("store_meta schema_version %q: %w", raw, scanErr)
			}
			if version > storeSchemaVersion {
				return fmt.Errorf("store schema version %d is newer than this build's %d", version, storeSchemaVersion)
			}
		case errors.Is(err, sql.ErrNoRows):
		default:
			return fmt.Errorf("read store_meta schema_version: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, schemaDDL); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO store_meta (key, value) VALUES ('schema_version', ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value`, fmt.Sprintf("%d", storeSchemaVersion)); err != nil {
			return fmt.Errorf("write store_meta schema_version: %w", err)
		}
		return nil
	})
}

// reopenBeforeAdoptHook is a test seam: it runs in Reopen after the adopted
// store is floored and before the store's handles are swapped to it. Always
// nil in production.
var reopenBeforeAdoptHook func(adoptedWorkspace string)

// Reopen opens path and adopts its handles into s. The caller must Close s
// first so the state files can be replaced. Process-start copies of the same
// *Store keep working because the pointer does not change.
func (s *Store) Reopen(path string) error {
	if s == nil {
		return fmt.Errorf("runtime store: reopen requires a store")
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if !s.closedOK {
		// Adopting new handles over open ones would orphan them.
		return fmt.Errorf("runtime store: reopen requires a closed store")
	}
	live, err := openStore(path)
	if err != nil {
		return err
	}
	// The writer's switch survives a reopen; the adopted store (a restore or
	// base install) may carry no floor, so it gets one BEFORE it is adopted
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
	// The adopted store may carry a different deposit-upcast ledger (a
	// rematerialize flip or base install replaces the files wholesale), so
	// the cached decision, its alias map and its appended file handle must
	// not survive the reopen.
	if err := s.closeDepositUpcastLedger(); err != nil {
		return errors.Join(fmt.Errorf("runtime store: reopen deposit upcast ledger: %w", err), live.Close())
	}
	s.depositUpcastMu.Lock()
	s.depositUpcast = nil
	s.depositUpcastMu.Unlock()
	s.vocabReplay.reset()
	// The adopted store may hold media-state rows this tape has not recorded
	// (a restore, a base install): derive the seed again for it
	// (media_projection.go; bunch 188).
	s.mediaStateMu.Lock()
	s.mediaStateSeededTape = nil
	s.mediaStateMu.Unlock()
	s.db = live.db
	s.readDB = live.readDB
	s.sqlite = live.sqlite
	s.path = live.path
	s.statePath = live.statePath
	s.texturePath = live.texturePath
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
	// Open bound the adopted store's write validator to live, and *live is
	// zeroed below, so the vocabulary fence would read a zeroed vocabCutover
	// and pass every write. Bind it to s, and keep the fence armed if either
	// side is cut over (fail closed).
	live.ogStore.SetWriteValidator(s.vocabWriteGuard)
	if live.vocabCutover.Load() {
		s.vocabCutover.Store(true)
	}
	s.og = live.og
	s.ogStore = live.ogStore
	s.ogReadStore = live.ogReadStore
	*live = Store{}
	s.closedOK = false
	// The lease state (ready or unseeded, the reserved block, the counters)
	// stays as it was; only the refusal Close set is lifted. The sidecar is
	// re-read first, while Close's refusal still holds: a seeder that starts
	// the moment the lease resumes must read the adopted store's untrust
	// marker, not the one this store just left. The adopted store may hold an
	// older lease row or none, so refreshLeaseWorkspace also forgets that the
	// latest lease row is on the tape, and the kick below re-affirms the
	// block with a fresh row on the tape (bunch 178, Devin 1 and DeepSeek 1).
	s.refreshLeaseWorkspace()
	s.resumeLeaseWork()
	s.kickLeaseRowOntoTape()
	// The event index window's catch-up (event_index_trim.go) for the
	// adopted store.
	s.startProjectionBatchCatchUp()
	return nil
}

// Close checkpoints the WAL and closes the database. If a statement or
// transaction still holds the write connection after the close deadline,
// Close fails closed with ErrCloseLeftOpen and leaves the store serving.
func (s *Store) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closedOK {
		return nil
	}
	// The lease goroutines (seeder, refill) stop first: they append through the
	// tape and read the pools Close is about to close.
	s.cancelLeaseWork()
	// Graceful close seals the telemetry window first. A crash does not, which
	// is the loss the ruling allows.
	telemetryStopped := s.closeTelemetry()
	deadline := time.Now().Add(closeDeadline)
	// The lease goroutines end before the write connection is pinned (a
	// statement of theirs in flight would fail the pin closed).
	s.drainLeaseWork(deadline)
	// Pin the write connection by the deadline: no statement or transaction
	// is using the database when it closes.
	pinCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	var pinned interface{ Close() error }
	if s.db != nil {
		conn, err := s.db.Conn(pinCtx)
		if err != nil {
			s.resumeLeaseWork()
			if telemetryStopped {
				s.resumeTelemetry()
			}
			return fmt.Errorf("%w (%v)", ErrPoolStatementLive, err)
		}
		// Fold the WAL into the main file so the file alone is the state.
		_, _ = conn.ExecContext(pinCtx, `PRAGMA wal_checkpoint(TRUNCATE)`)
		pinned = conn
	}
	s.closedOK = true
	var err error
	if closeErr := s.closeDepositUpcastLedger(); closeErr != nil {
		err = closeErr
	}
	if pinned != nil {
		_ = pinned.Close()
	}
	if s.sqlite != nil {
		if closeErr := s.sqlite.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}

// StatePath returns the SQLite state file backing the store.
func (s *Store) StatePath() string {
	return s.statePath
}

// TexturePath returns the sidecar directory beside the state file: the schema
// receipt, store floor, lease marker, vocabulary report and deposit upcast
// ledger live there. It is not a database.
func (s *Store) TexturePath() string {
	return s.texturePath
}
