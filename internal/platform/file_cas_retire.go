package platform

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"
)

// fileCASRootRetention is the window of file roots every computer keeps, on top
// of its latest root. It is a floor: a GC grace shorter than this never shortens
// it (docs/problems/platform-artifact-retention-2026-10-22.md section 8, F4).
const fileCASRootRetention = 7 * 24 * time.Hour

// retireFileCASRoots deletes computer_file_roots rows that are older than
// max(grace, 7 days) and are not their computer's latest root. The rule is the
// same for every computer, owner computers included: the latest root is never
// retired and no root inside the window is. A retired row's manifest is no
// longer in the file-cas-roots live set, so the sweep that follows collects it.
// The chunk sweep already ignores old roots, so which chunks are live does not
// change. Rows stay in Dolt history. It deletes at most limit rows; apply=false
// only counts (dry-run).
func (s *Service) retireFileCASRoots(ctx context.Context, started time.Time, grace time.Duration, limit int, apply bool) (int, error) {
	window := fileCASRootRetention
	if grace > window {
		window = grace
	}
	cutoff := started.UTC().Add(-window)
	rows, err := s.store.db.QueryContext(ctx, `SELECT computer_id,root,manifest_ref,head_sequence,created_at FROM computer_file_roots WHERE created_at<? ORDER BY computer_id, created_at, root`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("list old roots: %w", err)
	}
	var old []FileRootRecord
	for rows.Next() {
		var r FileRootRecord
		if err := rows.Scan(&r.ComputerID, &r.Root, &r.ManifestRef, &r.HeadSequence, &r.CreatedAt); err != nil {
			rows.Close()
			return 0, err
		}
		old = append(old, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	latest := map[string]string{}
	var retire []FileRootRecord
	for _, r := range old {
		if len(retire) >= limit {
			break
		}
		keep, ok := latest[r.ComputerID]
		if !ok {
			recs, err := s.store.LatestFileRoots(ctx, r.ComputerID, 1)
			if err != nil {
				return 0, err
			}
			if len(recs) == 0 {
				continue
			}
			keep = recs[0].Root
			latest[r.ComputerID] = keep
		}
		if r.Root == keep {
			continue
		}
		retire = append(retire, r)
	}
	if len(retire) == 0 || !apply {
		return len(retire), nil
	}

	// The log must be writable before any row is deleted.
	logFile, err := openArtifactGCLogForAppend(s.artifactsRoot, started)
	if err != nil {
		return 0, fmt.Errorf("deletion log unavailable, nothing retired: %w", err)
	}
	defer logFile.Close()

	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var done []FileRootRecord
	for _, r := range retire {
		// Each delete re-states the guards: still outside the window, still
		// not the latest root.
		res, err := tx.ExecContext(ctx, `DELETE FROM computer_file_roots WHERE computer_id=? AND root=? AND created_at<? AND root<>?`, r.ComputerID, r.Root, cutoff, latest[r.ComputerID])
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			done = append(done, r)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	runID := started.UTC().Format("20060102T150405.000000000Z")
	var logErr error
	for _, r := range done {
		if err := logFile.append(artifactGCLogRecord{Run: runID, Op: "root_retired", Namespace: "file-cas-roots", Path: filepath.ToSlash(filepath.Join(r.ComputerID, r.Root)), ModTime: r.CreatedAt.UTC().Format(time.RFC3339Nano), Reason: "older than " + window.String() + " and not the latest root; manifest " + r.ManifestRef}); err != nil {
			logErr = err
		}
	}
	if err := logFile.sync(); err != nil && logErr == nil {
		logErr = err
	}
	if err := s.store.commitBoundary(ctx, fmt.Sprintf("retire %d old file roots", len(done))); err != nil {
		return len(done), fmt.Errorf("rows deleted, dolt commit failed: %w", err)
	}
	if logErr != nil {
		return len(done), fmt.Errorf("rows deleted, log write failed: %w", logErr)
	}
	return len(done), nil
}
