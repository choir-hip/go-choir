// Package guestdolt is the one-shot guest migrator of the data system
// cutover (docs/design/data-system-build-sheet-2026-10-30.md, decision 12):
// it copies a computer's legacy embedded Dolt workspace into the new SQLite
// state file, verifies every table by row count and digest, and renames the
// finished file into place. The Dolt workspace is only read and is kept; C5
// deletes it once the differential and keep-set checks are green.
//
// The new schema is not defined here. By default the migrator makes it by
// opening the real store (store.Open) in a scratch directory beside the target
// and closing it, then copies rows into the resulting state.db. A caller may
// instead pass an OpenNew, which opens the new store's SQLite file at the path
// it is given (creating the schema); its handle's Close must release every
// connection to the file, because the migrator closes it before renaming and
// no WAL sidecar may remain.
//
// Caller contract (autoputer's migrateLegacyStoreIfNeeded):
//
//	if _, err := os.Stat(store.StateDBPath(storePath)); os.IsNotExist(err) {
//	    rep, err := guestdolt.Migrate(ctx, store.LegacyDoltWorkspacePath(storePath), store.StateDBPath(storePath))
//	    ...
//	}
package guestdolt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/migrate/doltsrc"
	"github.com/yusefmosiah/go-choir/internal/migrate/rowcopy"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// OpenNew opens (and creates the schema of) the new SQLite state database at
// path. The migrator closes the returned handle.
type OpenNew func(path string) (*dbx.SQLiteDB, error)

// Report is the migration outcome, also written as JSON next to the SQLite
// file (ReportPath).
type Report struct {
	DoltWorkspace string          `json:"dolt_workspace"`
	SQLitePath    string          `json:"sqlite_path"`
	Skipped       bool            `json:"skipped,omitempty"`
	SkipReason    string          `json:"skip_reason,omitempty"`
	OK            bool            `json:"ok"`
	Error         string          `json:"error,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	FinishedAt    time.Time       `json:"finished_at"`
	Copy          *rowcopy.Report `json:"copy,omitempty"`
}

// ReportPath is where Migrate writes its report for sqlitePath.
func ReportPath(sqlitePath string) string { return sqlitePath + ".migration.json" }

// TempPath is the file the migration writes before the atomic rename.
func TempPath(sqlitePath string) string { return sqlitePath + ".migrating" }

// MigrateStore migrates the legacy workspace and state file derived from
// storePath (store.LegacyDoltWorkspacePath, store.StateDBPath).
func MigrateStore(ctx context.Context, storePath string, openNew ...OpenNew) (Report, error) {
	return Migrate(ctx, store.LegacyDoltWorkspacePath(storePath), store.StateDBPath(storePath), openNew...)
}

// Migrate copies doltWorkspace into a new SQLite file at sqlitePath. It does
// nothing, with Skipped set, when the workspace is absent or sqlitePath
// already exists. On failure the partial file is deleted and the Dolt
// workspace is untouched.
func Migrate(ctx context.Context, doltWorkspace, sqlitePath string, openNewOpt ...OpenNew) (rep Report, err error) {
	rep = Report{DoltWorkspace: doltWorkspace, SQLitePath: sqlitePath, StartedAt: time.Now().UTC()}
	defer func() {
		rep.FinishedAt = time.Now().UTC()
		if err != nil {
			rep.Error = err.Error()
		}
		if werr := writeReport(sqlitePath, &rep); werr != nil && err == nil && !rep.Skipped {
			err = werr
		}
	}()
	if _, serr := os.Stat(sqlitePath); serr == nil {
		rep.Skipped, rep.SkipReason = true, "state file already exists"
		return rep, nil
	}
	if !doltsrc.Exists(doltWorkspace) {
		rep.Skipped, rep.SkipReason = true, "no legacy Dolt workspace"
		return rep, nil
	}
	var openNew OpenNew
	if len(openNewOpt) > 0 {
		openNew = openNewOpt[0]
	}
	tmp := TempPath(sqlitePath)
	removeSQLite(tmp)
	scratch := ""
	if openNew == nil {
		// Default: the real store makes the schema in a scratch directory.
		scratch = tmp + ".d"
		_ = os.RemoveAll(scratch)
		var serr error
		tmp, serr = createStoreSchema(scratch)
		if serr != nil {
			_ = os.RemoveAll(scratch)
			return rep, fmt.Errorf("guestdolt: create new store: %w", serr)
		}
		openNew = func(path string) (*dbx.SQLiteDB, error) { return dbx.OpenSQLite(path, dbx.SQLiteOptions{}) }
	}
	defer func() {
		if scratch != "" {
			_ = os.RemoveAll(scratch)
		}
		if err != nil {
			removeSQLite(tmp)
		}
	}()

	src, err := doltsrc.Open(ctx, doltWorkspace, doltsrc.DatabaseName)
	if err != nil {
		return rep, err
	}
	defer src.Close()

	dst, err := openNew(tmp)
	if err != nil {
		return rep, fmt.Errorf("guestdolt: create new store: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = dst.Close()
		}
	}()

	cp, err := rowcopy.Run(ctx, src.DB, dst.Write, dbx.SQLite, rowcopy.Options{})
	rep.Copy = cp
	if err != nil {
		return rep, fmt.Errorf("guestdolt: copy: %w", err)
	}
	var check string
	if err = dst.Write.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return rep, fmt.Errorf("guestdolt: quick_check: %w", err)
	}
	if check != "ok" {
		err = fmt.Errorf("guestdolt: quick_check: %s", check)
		return rep, err
	}
	closed = true
	if err = dst.Close(); err != nil {
		return rep, fmt.Errorf("guestdolt: close new store: %w", err)
	}
	for _, side := range []string{"-wal", "-shm"} {
		if st, serr := os.Stat(tmp + side); serr == nil {
			if st.Size() > 0 && side == "-wal" {
				err = fmt.Errorf("guestdolt: %s%s still has data after close; the OpenNew handle left a connection open", filepath.Base(tmp), side)
				return rep, err
			}
			_ = os.Remove(tmp + side)
		}
	}
	if err = syncFile(tmp); err != nil {
		return rep, fmt.Errorf("guestdolt: sync: %w", err)
	}
	if err = os.Rename(tmp, sqlitePath); err != nil {
		return rep, fmt.Errorf("guestdolt: rename: %w", err)
	}
	_ = syncDir(filepath.Dir(sqlitePath))
	rep.OK = true
	return rep, nil
}

func removeSQLite(path string) {
	for _, s := range []string{"", "-wal", "-shm", "-journal"} {
		_ = os.Remove(path + s)
	}
}

func syncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func writeReport(sqlitePath string, rep *Report) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return err
	}
	tmp := ReportPath(sqlitePath) + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ReportPath(sqlitePath))
}
