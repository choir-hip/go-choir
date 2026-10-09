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
// its root and runs a full GC. It refuses unless the content witness (schema
// and per-table content digests) is identical before and after. It mutates the
// workspace in place: run it on a copy, a reflink snapshot, or a scratch store
// whose loss is recoverable from the tape. Never on a store a runtime has open.

// CompactResult reports one compaction.
type CompactResult struct {
	Squashed        bool
	CommitsBefore   int
	CommitsAfter    int
	NomsBytesBefore int64
	NomsBytesAfter  int64
	WitnessBefore   selfdevprotocol.VMLocalContentWitness
	WitnessAfter    selfdevprotocol.VMLocalContentWitness
}

const compactionDatabase = "texture"

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

// CompactWorkspace compacts a closed projection store workspace in place.
func CompactWorkspace(ctx context.Context, computerID, canonicalHead, workspacePath string) (CompactResult, error) {
	var result CompactResult
	result.NomsBytesBefore = workspaceBytes(workspacePath)
	before, err := witnessForWorkspace(ctx, computerID, canonicalHead, workspacePath)
	if err != nil {
		return result, err
	}
	result.WitnessBefore = before

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
	if _, err := db.ExecContext(ctx, "CALL DOLT_GC('--full')"); err != nil {
		return fail(fmt.Errorf("projection compaction: full gc: %w", err))
	}
	if err := closeDB(); err != nil {
		return result, fmt.Errorf("projection compaction: close workspace: %w", err)
	}

	after, err := witnessForWorkspace(ctx, computerID, canonicalHead, workspacePath)
	if err != nil {
		return result, err
	}
	result.WitnessAfter = after
	if err := selfdevprotocol.WitnessContentMatches(after, before); err != nil {
		return result, fmt.Errorf("projection compaction: refused: content witness changed: %w", err)
	}
	result.NomsBytesAfter = workspaceBytes(workspacePath)
	return result, nil
}
