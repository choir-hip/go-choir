package projectionbase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	embedded "github.com/dolthub/driver/v2"

	"github.com/dolthub/dolt/go/store/util/tempfiles"
)

// SquashAndGC is the one implementation of the history squash both the guest
// store compaction (CompactWorkspaceWith) and the platform Store A weekly
// squash (cmd/platform-dolt-squash) use. It is the method of
// docs/problems/paper-disk-full-again-history-growth-2026-10-19.md:
//
//  1. refuse unless exactly one branch and no tag, remote or stash keeps
//     history reachable, and the log has exactly one root commit; these
//     refusals mutate nothing and return a *RefusedError;
//  2. record the WORKING root hash;
//  3. soft-reset to the root commit and commit everything as one commit;
//  4. refuse (before the GC) unless the new HEAD root equals the recorded
//     root and the log holds at most two commits;
//  5. bounded full GC;
//  6. reopen and refuse unless HEAD and WORKING roots both equal the recorded
//     root.
//
// Prolly trees are history independent, so equal root hashes mean the same
// schemas and rows. It mutates the store in place: the caller takes the
// backup and holds every other opener off (a closed store, never one a
// sql-server has open). The caller also points Dolt's process-wide temp
// provider at the store's own filesystem (UseStoreTempDir).
type SquashOptions struct {
	// Dir is the directory holding the database directory (the embedded
	// driver's "file://" root); Database is the database directory name.
	Dir      string
	Database string
	// Label prefixes error messages.
	Label string
	// Message builds the squash commit message from the commit count.
	Message func(commits int) string
}

// SquashResult reports one squash.
type SquashResult struct {
	Squashed      bool
	CommitsBefore int
	CommitsAfter  int
	RootBefore    string
	RootAfter     string
}

// RefusedError is a refusal made before anything was changed: the store on
// disk is as it was.
type RefusedError struct {
	Label  string
	Reason string
}

func (e *RefusedError) Error() string {
	return fmt.Sprintf("%s: refused: %s", e.Label, e.Reason)
}

// IsRefusedBeforeMutation reports whether err is a *RefusedError.
func IsRefusedBeforeMutation(err error) bool {
	var refused *RefusedError
	return errors.As(err, &refused)
}

func openDoltDB(dir, database, label string) (*sql.DB, interface{ Close() error }, error) {
	dsn := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true",
		filepath.Clean(dir), database)
	cfg, err := embedded.ParseDSN(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: parse dsn: %w", label, err)
	}
	connector, err := embedded.NewConnector(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: open workspace: %w", label, err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	return db, connector, nil
}

func rootHash(ctx context.Context, db *sql.DB, label, ref string) (string, error) {
	var h string
	if err := db.QueryRowContext(ctx, "SELECT dolt_hashof_db(?)", ref).Scan(&h); err != nil {
		return "", fmt.Errorf("%s: %s root hash: %w", label, ref, err)
	}
	return h, nil
}

func countRows(ctx context.Context, db *sql.DB, label, what, query string) (int, error) {
	var n int
	if err := db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, fmt.Errorf("%s: count %s: %w", label, what, err)
	}
	return n, nil
}

// UseStoreTempDir stages Dolt's GC output and table files inside the store's
// own filesystem (<dir>/<database>/.dolt/temptf) instead of os.TempDir()
// (the killed 2026-10-19 run left 4.2 GB in the host's /tmp). The returned
// cleanup removes the directory and restores the previous provider.
func UseStoreTempDir(dir, database string) (func(), error) {
	tempDir := StoreTempDir(dir, database)
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return nil, fmt.Errorf("store temp dir: %w", err)
	}
	previous := tempfiles.MovableTempFileProvider
	tempfiles.MovableTempFileProvider = tempfiles.NewTempFileProviderAt(tempDir)
	return func() {
		tempfiles.MovableTempFileProvider = previous
		_ = os.RemoveAll(tempDir)
	}, nil
}

// SquashAndGC squashes the store's single branch to one commit on its root
// and runs a bounded full GC. See SquashOptions.
func SquashAndGC(ctx context.Context, o SquashOptions) (SquashResult, error) {
	var result SquashResult
	label := o.Label
	if label == "" {
		label = "dolt squash"
	}
	// A running dolt sql-server writes .dolt/sql-server.info and removes it on
	// a clean stop. An embedded open beside it is read-only (probed locally:
	// "cannot update manifest: database is read only"), so the squash would
	// fail closed anyway; refuse first, with the reason in plain words.
	serverInfo := filepath.Join(filepath.Clean(o.Dir), o.Database, ".dolt", "sql-server.info")
	if _, statErr := os.Stat(serverInfo); statErr == nil {
		return result, &RefusedError{Label: label, Reason: fmt.Sprintf("%s exists: a dolt sql-server has this store open (or died uncleanly); stop it first", serverInfo)}
	}
	db, closer, err := openDoltDB(o.Dir, o.Database, label)
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
	fail := func(err error) (SquashResult, error) {
		_ = closeDB()
		return result, err
	}
	refuse := func(format string, args ...any) (SquashResult, error) {
		return fail(&RefusedError{Label: label, Reason: fmt.Sprintf(format, args...)})
	}

	// Refs that keep history reachable. Dolt's GC roots branches, tags,
	// remotes' tracking refs and stashes; any of them makes the squash free
	// nothing.
	for _, ref := range []struct{ what, table string }{
		{"tags", "dolt_tags"},
		{"remotes", "dolt_remotes"},
		{"stashes", "dolt_stashes"},
	} {
		n, err := countRows(ctx, db, label, ref.what, "SELECT COUNT(*) FROM "+ref.table)
		if err != nil {
			return fail(err)
		}
		if n != 0 {
			return refuse("%d %s keep history reachable; want none", n, ref.what)
		}
	}
	branches, err := countRows(ctx, db, label, "branches", "SELECT COUNT(*) FROM dolt_branches")
	if err != nil {
		return fail(err)
	}
	if branches != 1 {
		return refuse("%d branches keep history reachable; want exactly one", branches)
	}
	// The root commit by topology (no parent), not "oldest by date": a clock
	// step must not leave older history reachable.
	rows, err := db.QueryContext(ctx, "SELECT commit_hash FROM dolt_commit_ancestors WHERE parent_hash IS NULL")
	if err != nil {
		return fail(fmt.Errorf("%s: find root commit: %w", label, err))
	}
	var roots []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			_ = rows.Close()
			return fail(fmt.Errorf("%s: scan root commit: %w", label, err))
		}
		roots = append(roots, h)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fail(fmt.Errorf("%s: find root commit: %w", label, err))
	}
	_ = rows.Close()
	if len(roots) != 1 {
		return refuse("%d root commits; want exactly one", len(roots))
	}

	rootBefore, err := rootHash(ctx, db, label, "WORKING")
	if err != nil {
		return fail(err)
	}
	result.RootBefore = rootBefore
	if result.CommitsBefore, err = countRows(ctx, db, label, "commits", "SELECT COUNT(*) FROM dolt_log"); err != nil {
		return fail(err)
	}
	headBefore, err := rootHash(ctx, db, label, "HEAD")
	if err != nil {
		return fail(err)
	}
	// Squash when there is history to drop, or when the working set is ahead
	// of HEAD: a store that makes no snapshot commits (Store A with
	// PLATFORM_DOLT_SNAPSHOTS=off) keeps two commits after its first squash
	// and holds every later write only in the working set. Resetting to the
	// root first keeps the log at two commits either way.
	if result.CommitsBefore > 2 || headBefore != rootBefore {
		if _, err := db.ExecContext(ctx, "CALL DOLT_RESET('--soft', ?)", roots[0]); err != nil {
			return fail(fmt.Errorf("%s: soft reset to root: %w", label, err))
		}
		message := fmt.Sprintf("history squash: %d commits", result.CommitsBefore)
		if o.Message != nil {
			message = o.Message(result.CommitsBefore)
		}
		if _, err := db.ExecContext(ctx, "CALL DOLT_COMMIT('-A', '--allow-empty', '-m', ?)", message); err != nil {
			return fail(fmt.Errorf("%s: squash commit: %w", label, err))
		}
		result.Squashed = true
	}
	if result.CommitsAfter, err = countRows(ctx, db, label, "commits after squash", "SELECT COUNT(*) FROM dolt_log"); err != nil {
		return fail(err)
	}
	if result.CommitsAfter > 2 {
		return fail(fmt.Errorf("%s: refused: %d commits remain after the squash; the GC would free nothing", label, result.CommitsAfter))
	}
	// Cheap check before the expensive GC: state that a commit cannot hold
	// (dolt_ignore'd tables) makes HEAD differ from WORKING for good.
	headSquashed, err := rootHash(ctx, db, label, "HEAD")
	if err != nil {
		return fail(err)
	}
	if headSquashed != rootBefore {
		return fail(fmt.Errorf("%s: refused: HEAD differs from the working set after the squash (state a commit cannot hold, such as dolt_ignore'd tables): working before %s, head after %s", label, rootBefore, headSquashed))
	}
	if _, err := db.ExecContext(ctx, "CALL DOLT_GC('--full', '--archive-level', '0', '--incremental-file-size', ?)", gcIncrementalFileSize); err != nil {
		return fail(fmt.Errorf("%s: full gc: %w", label, err))
	}
	if err := closeDB(); err != nil {
		return result, fmt.Errorf("%s: close workspace: %w", label, err)
	}

	// Reopen, so the check reads what the GC left on disk.
	db, closer, err = openDoltDB(o.Dir, o.Database, label)
	if err != nil {
		return result, err
	}
	headAfter, err := rootHash(ctx, db, label, "HEAD")
	if err != nil {
		return fail(err)
	}
	workingAfter, err := rootHash(ctx, db, label, "WORKING")
	if err != nil {
		return fail(err)
	}
	if err := closeDB(); err != nil {
		return result, fmt.Errorf("%s: close workspace: %w", label, err)
	}
	result.RootAfter = headAfter
	if headAfter != rootBefore || workingAfter != rootBefore {
		return result, fmt.Errorf("%s: refused: root changed: working before %s, head after %s, working after %s", label, rootBefore, headAfter, workingAfter)
	}
	return result, nil
}
