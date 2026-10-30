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

// fileRootPageSize bounds how many old rows one query materializes.
const fileRootPageSize = 500

type rowQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// retirableFileRoots lists, oldest first, up to limit rows that are older than
// cutoff and are not their computer's latest root, together with each such
// computer's latest root. It reads through q, so inside a transaction the
// choice and the later DELETE see one snapshot. It pages with a keyset cursor,
// so memory is bounded by the page and the result, not by the table.
func retirableFileRoots(ctx context.Context, q rowQuerier, cutoff time.Time, limit int) ([]FileRootRecord, map[string]string, error) {
	latest := map[string]string{}
	var out []FileRootRecord
	var cur *FileRootRecord
	for len(out) < limit {
		var rows *sql.Rows
		var err error
		if cur == nil {
			rows, err = q.QueryContext(ctx, `SELECT computer_id,root,manifest_ref,head_sequence,created_at FROM computer_file_roots WHERE created_at<? ORDER BY created_at, computer_id, root LIMIT ?`, cutoff, fileRootPageSize)
		} else {
			rows, err = q.QueryContext(ctx, `SELECT computer_id,root,manifest_ref,head_sequence,created_at FROM computer_file_roots WHERE created_at<? AND (created_at>? OR (created_at=? AND (computer_id>? OR (computer_id=? AND root>?)))) ORDER BY created_at, computer_id, root LIMIT ?`,
				cutoff, cur.CreatedAt, cur.CreatedAt, cur.ComputerID, cur.ComputerID, cur.Root, fileRootPageSize)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("list old roots: %w", err)
		}
		var page []FileRootRecord
		for rows.Next() {
			var r FileRootRecord
			if err := rows.Scan(&r.ComputerID, &r.Root, &r.ManifestRef, &r.HeadSequence, &r.CreatedAt); err != nil {
				rows.Close()
				return nil, nil, err
			}
			page = append(page, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, nil, err
		}
		for _, r := range page {
			if len(out) >= limit {
				break
			}
			keep, ok := latest[r.ComputerID]
			if !ok {
				lrows, err := q.QueryContext(ctx, `SELECT root FROM computer_file_roots WHERE computer_id=? ORDER BY created_at DESC, root DESC LIMIT 1`, r.ComputerID)
				if err != nil {
					return nil, nil, err
				}
				if lrows.Next() {
					if err := lrows.Scan(&keep); err != nil {
						lrows.Close()
						return nil, nil, err
					}
				}
				err = lrows.Err()
				lrows.Close()
				if err != nil {
					return nil, nil, err
				}
				latest[r.ComputerID] = keep
			}
			if keep == "" || r.Root == keep {
				continue
			}
			out = append(out, r)
		}
		if len(page) < fileRootPageSize {
			break
		}
		last := page[len(page)-1]
		cur = &last
	}
	return out, latest, nil
}

// retireFileCASRoots deletes computer_file_roots rows that are older than
// max(grace, 7 days) and are not their computer's latest root. The rule is the
// same for every computer, owner computers included: the latest root is never
// retired and no root inside the window is. A retired row's manifest is no
// longer in the file-cas-roots live set, so the sweep that follows collects it.
// The chunk sweep already ignores old roots, so which chunks are live does not
// change. Rows stay in Dolt history.
//
// The caller has already loaded and verified the live sets. This function
// chooses the rows and deletes them in one serializable transaction, writes the
// intent to the log (durable) between the choice and the DELETE, and checks
// before commit that every touched computer still has the same latest root; any
// mismatch rolls the whole transaction back. It retires at most limit rows;
// apply=false only counts (dry-run).
func (s *Service) retireFileCASRoots(ctx context.Context, started time.Time, grace time.Duration, limit int, apply bool) (int, error) {
	window := fileCASRootRetention
	if grace > window {
		window = grace
	}
	cutoff := started.UTC().Add(-window)
	if !apply {
		retire, _, err := retirableFileRoots(ctx, s.store.db, cutoff, limit)
		return len(retire), err
	}

	// The log must be writable before any row is chosen for deletion.
	logFile, err := openArtifactGCLogForAppend(s.artifactsRoot, started)
	if err != nil {
		return 0, fmt.Errorf("deletion log unavailable, nothing retired: %w", err)
	}
	defer logFile.Close()
	runID := started.UTC().Format("20060102T150405.000000000Z")

	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	retire, latest, err := retirableFileRoots(ctx, tx, cutoff, limit)
	if err != nil || len(retire) == 0 {
		return 0, err
	}
	reason := "older than " + window.String() + " and not the latest root"
	pathOf := func(r FileRootRecord) string { return filepath.ToSlash(filepath.Join(r.ComputerID, r.Root)) }
	for _, r := range retire {
		if err := logFile.append(artifactGCLogRecord{Run: runID, Op: "root_retire_intent", Namespace: "file-cas-roots", Path: pathOf(r), ModTime: r.CreatedAt.UTC().Format(time.RFC3339Nano), Reason: reason + "; manifest " + r.ManifestRef}); err != nil {
			return 0, fmt.Errorf("deletion log write failed, nothing retired: %w", err)
		}
	}
	if err := logFile.sync(); err != nil {
		return 0, fmt.Errorf("deletion log write failed, nothing retired: %w", err)
	}
	abort := func(cause error) (int, error) {
		_ = logFile.append(artifactGCLogRecord{Run: runID, Op: "aborted", Namespace: "file-cas-roots", Error: cause.Error()})
		_ = logFile.sync()
		return 0, cause
	}
	var done []FileRootRecord
	for _, r := range retire {
		// Each delete re-states the guards: still outside the window, still
		// not the latest root.
		res, err := tx.ExecContext(ctx, `DELETE FROM computer_file_roots WHERE computer_id=? AND root=? AND created_at<? AND root<>?`, r.ComputerID, r.Root, cutoff, latest[r.ComputerID])
		if err != nil {
			return abort(err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			done = append(done, r)
		}
	}
	// Verify before commit: every touched computer still has exactly the same
	// latest root, so none lost its root and none became rootless.
	for computer, keep := range latest {
		var got string
		if err := tx.QueryRowContext(ctx, `SELECT root FROM computer_file_roots WHERE computer_id=? ORDER BY created_at DESC, root DESC LIMIT 1`, computer).Scan(&got); err != nil || got != keep {
			return abort(fmt.Errorf("latest root of %s changed during retirement (%q, want %q): rolled back", computer, got, keep))
		}
	}
	if err := tx.Commit(); err != nil {
		return abort(err)
	}
	var logErr error
	for _, r := range done {
		if err := logFile.append(artifactGCLogRecord{Run: runID, Op: "root_retired", Namespace: "file-cas-roots", Path: pathOf(r), ModTime: r.CreatedAt.UTC().Format(time.RFC3339Nano), Reason: reason + "; manifest " + r.ManifestRef}); err != nil {
			logErr = err
		}
	}
	if err := logFile.sync(); err != nil && logErr == nil {
		logErr = err
	}
	if logErr != nil {
		return len(done), fmt.Errorf("rows deleted, log write failed: %w", logErr)
	}
	return len(done), nil
}
