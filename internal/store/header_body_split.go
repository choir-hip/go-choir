package store

// The header/body split migration (docs/design/store-header-body-split-2026-11-01.md
// section 6). Slice 0 lands the contract; slice D fills it.

import (
	"context"
	"database/sql"
)

// migratePartitionSplit migrates one telemetry partition file from the
// pre-split choir_events layout to choirEventsDDL, in one transaction, before
// openFile runs the DDL. A file already split, or new, is left as it is.
// Stub: slice D implements it; slice B calls it from openFile.
func migratePartitionSplit(ctx context.Context, db *sql.DB) error {
	_, _ = ctx, db
	return nil
}
