package projectionbase

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"

	embedded "github.com/dolthub/driver/v2"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// Projection store compaction (docs/problems/guest-store-history-bloat-and-
// memory-shape-2026-10-09.md). The projection store is a cache of the tape
// (O21) and nothing reads its Dolt history: Texture history follows the
// revision parent chain (internal/store/texture.go getHistory). History and
// old chunk generations were 81% of the owner store (8.9 GiB vs 1.7 GiB live).
//
// CompactWorkspace squashes the store's single branch to one commit on top of
// its root and runs a full GC. It refuses unless the store's root value hash
// (every schema and row) is identical before and after; optionally it also
// compares the content witness (schema and per-table content digests). It mutates the
// workspace in place: run it on a copy, a reflink snapshot, or a scratch store
// whose loss is recoverable from the tape. Never on a store a runtime has open.

// CompactResult reports one compaction.
type CompactResult struct {
	Squashed        bool
	CommitsBefore   int
	CommitsAfter    int
	NomsBytesBefore int64
	NomsBytesAfter  int64
	RootBefore      string
	RootAfter       string
	WitnessBefore   selfdevprotocol.VMLocalContentWitness
	WitnessAfter    selfdevprotocol.VMLocalContentWitness
}

// CompactOptions selects the content check.
//
// The default check compares Dolt root value hashes: the working root before
// the squash against the HEAD and working roots after the GC. Prolly trees are
// history-independent, so the same schemas and rows give the same root hash
// however the history looked; it costs nothing to compute. FullWitness also
// extracts the VM-local content witness before and after, which reads and
// sorts every row (on the paper's 48 GiB store it held 8 GB for over half an
// hour before the squash); use it when the witness itself is the evidence.
type CompactOptions struct {
	FullWitness bool
}

const compactionDatabase = "texture"

// gcIncrementalFileSize bounds the full GC's memory (docs/problems/paper-
// disk-full-again-history-growth-2026-10-19.md). With incremental files, leaf
// chunks skip the GC's visited set and the copier rotates its output, so the
// per-chunk bookkeeping resets per file. Archive level 0 writes plain table
// files, skipping the archive writer and its zstd dictionaries. Unbounded,
// DOLT_GC('--full') on the paper's 48 GiB store took 13 GB plus 15 GB of swap
// and never finished; bounded, it took 6.1 GB and 2 min 19 s.
const gcIncrementalFileSize = "268435456"

// TempDir is where Dolt stages GC output and table files for a workspace:
// inside the store's own .dolt, on the same filesystem as the files they
// become. The embedded driver otherwise stages them in os.TempDir().
func TempDir(workspacePath string) string {
	return filepath.Join(filepath.Clean(workspacePath), compactionDatabase, ".dolt", "temptf")
}

func openCompactionDB(workspacePath string) (*sql.DB, interface{ Close() error }, error) {
	dsn := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true",
		filepath.Clean(workspacePath), compactionDatabase)
	cfg, err := embedded.ParseDSN(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("projection compaction: parse dsn: %w", err)
	}
	connector, err := embedded.NewConnector(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("projection compaction: open workspace: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	return db, connector, nil
}

func workspaceBytes(workspacePath string) int64 {
	var total int64
	_ = filepath.WalkDir(workspacePath, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, infoErr := d.Info(); infoErr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// CompactWorkspace compacts a closed projection store workspace in place with
// the default (root hash) content check.
func CompactWorkspace(ctx context.Context, computerID, canonicalHead, workspacePath string) (CompactResult, error) {
	return CompactWorkspaceWith(ctx, computerID, canonicalHead, workspacePath, CompactOptions{})
}

func rootHash(ctx context.Context, db *sql.DB, ref string) (string, error) {
	var h string
	if err := db.QueryRowContext(ctx, "SELECT dolt_hashof_db(?)", ref).Scan(&h); err != nil {
		return "", fmt.Errorf("projection compaction: %s root hash: %w", ref, err)
	}
	return h, nil
}

// CompactWorkspaceWith compacts a closed projection store workspace in place.
func CompactWorkspaceWith(ctx context.Context, computerID, canonicalHead, workspacePath string, opts CompactOptions) (CompactResult, error) {
	var result CompactResult
	result.NomsBytesBefore = workspaceBytes(workspacePath)
	if opts.FullWitness {
		before, err := witnessForWorkspace(ctx, computerID, canonicalHead, workspacePath)
		if err != nil {
			return result, err
		}
		result.WitnessBefore = before
	}

	db, closer, err := openCompactionDB(workspacePath)
	if err != nil {
		return result, err
	}
	closeDB := func() error {
		dbErr := db.Close()
		connErr := closer.Close()
		if dbErr != nil {
			return dbErr
		}
		return connErr
	}
	fail := func(err error) (CompactResult, error) {
		_ = closeDB()
		return result, err
	}
	var branches int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_branches").Scan(&branches); err != nil {
		return fail(fmt.Errorf("projection compaction: count branches: %w", err))
	}
	if branches != 1 {
		return fail(fmt.Errorf("projection compaction: refused: %d branches keep history reachable; want exactly one", branches))
	}
	rootBefore, err := rootHash(ctx, db, "WORKING")
	if err != nil {
		return fail(err)
	}
	result.RootBefore = rootBefore
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_log").Scan(&result.CommitsBefore); err != nil {
		return fail(fmt.Errorf("projection compaction: count commits: %w", err))
	}
	if result.CommitsBefore > 2 {
		var root string
		if err := db.QueryRowContext(ctx, "SELECT commit_hash FROM dolt_log ORDER BY date ASC, commit_hash ASC LIMIT 1").Scan(&root); err != nil {
			return fail(fmt.Errorf("projection compaction: find root commit: %w", err))
		}
		if _, err := db.ExecContext(ctx, "CALL DOLT_RESET('--soft', ?)", root); err != nil {
			return fail(fmt.Errorf("projection compaction: soft reset to root: %w", err))
		}
		message := fmt.Sprintf("projection compaction: %d commits squashed at canonical head %s", result.CommitsBefore, canonicalHead)
		if _, err := db.ExecContext(ctx, "CALL DOLT_COMMIT('-A', '--allow-empty', '-m', ?)", message); err != nil {
			return fail(fmt.Errorf("projection compaction: squash commit: %w", err))
		}
		result.Squashed = true
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_log").Scan(&result.CommitsAfter); err != nil {
		return fail(fmt.Errorf("projection compaction: count commits after squash: %w", err))
	}
	if _, err := db.ExecContext(ctx, "CALL DOLT_GC('--full', '--archive-level', '0', '--incremental-file-size', ?)", gcIncrementalFileSize); err != nil {
		return fail(fmt.Errorf("projection compaction: full gc: %w", err))
	}
	if err := closeDB(); err != nil {
		return result, fmt.Errorf("projection compaction: close workspace: %w", err)
	}

	// Reopen, so the check reads what the GC left on disk.
	db, closer, err = openCompactionDB(workspacePath)
	if err != nil {
		return result, err
	}
	headAfter, err := rootHash(ctx, db, "HEAD")
	if err != nil {
		return fail(err)
	}
	workingAfter, err := rootHash(ctx, db, "WORKING")
	if err != nil {
		return fail(err)
	}
	if err := closeDB(); err != nil {
		return result, fmt.Errorf("projection compaction: close workspace: %w", err)
	}
	result.RootAfter = headAfter
	if headAfter != rootBefore || workingAfter != rootBefore {
		return result, fmt.Errorf("projection compaction: refused: root changed: working before %s, head after %s, working after %s", rootBefore, headAfter, workingAfter)
	}

	if opts.FullWitness {
		after, err := witnessForWorkspace(ctx, computerID, canonicalHead, workspacePath)
		if err != nil {
			return result, err
		}
		result.WitnessAfter = after
		if err := selfdevprotocol.WitnessContentMatches(after, result.WitnessBefore); err != nil {
			return result, fmt.Errorf("projection compaction: refused: content witness changed: %w", err)
		}
	}
	result.NomsBytesAfter = workspaceBytes(workspacePath)
	return result, nil
}
