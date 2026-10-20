package projectionbase

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"

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
	return StoreTempDir(workspacePath, compactionDatabase)
}

// StoreTempDir is TempDir for a database other than the guest's "texture":
// <dir>/<database>/.dolt/temptf.
func StoreTempDir(dir, database string) string {
	return filepath.Join(filepath.Clean(dir), database, ".dolt", "temptf")
}

func openCompactionDB(workspacePath string) (*sql.DB, interface{ Close() error }, error) {
	return openDoltDB(workspacePath, compactionDatabase, "projection compaction")
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

	squash, err := SquashAndGC(ctx, SquashOptions{
		Dir:      workspacePath,
		Database: compactionDatabase,
		Label:    "projection compaction",
		Message: func(commits int) string {
			return fmt.Sprintf("projection compaction: %d commits squashed at canonical head %s", commits, canonicalHead)
		},
	})
	result.Squashed = squash.Squashed
	result.CommitsBefore = squash.CommitsBefore
	result.CommitsAfter = squash.CommitsAfter
	result.RootBefore = squash.RootBefore
	result.RootAfter = squash.RootAfter
	if err != nil {
		return result, err
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
