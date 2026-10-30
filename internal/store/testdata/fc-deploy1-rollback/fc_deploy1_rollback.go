package store

// Rollback below F-c deploy 1
// (docs/archive/design/choir-event-table-f-c-2026-10-22.md, status section).
//
// This file ships in the revert commit of deploy 1, never in deploy 1
// itself: scripts/rollback/fc-deploy1-rollback.sh copies it to
// internal/store/fc_deploy1_rollback.go and adds one boot substep to
// bootstrap, right after the retiredRuntimeTables loop. It compiles against
// the pre-deploy-1 store package, which already imports database/sql,
// errors and log. The store tests keep it identical to the function they
// rehearse (TestEventTableRollbackRecipe) and build the store package with
// it in place.

import (
	"database/sql"
	"errors"
	"log"
)

// rollbackRecipeDropEmptyChoirEvents drops choir_events only while it is
// empty, so an older binary over a store deploy 1 opened sees the
// pre-deploy-1 schema and can publish checkpoints again. A row means an op
// was stamped: the computer is below the rollback floor, the table stays,
// and the repair is forward (design section 4.5). It never reads
// og_objects.
func rollbackRecipeDropEmptyChoirEvents(db *sql.DB) error {
	// An exact name: LIKE would read the underscore as a wildcard
	// (round 1d, DeepSeek).
	var tables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'choir_events'`).Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		return nil // never created, or already dropped
	}
	var one int
	switch err := db.QueryRow(`SELECT 1 FROM choir_events LIMIT 1`).Scan(&one); {
	case errors.Is(err, sql.ErrNoRows):
		_, err := db.Exec(`DROP TABLE choir_events`)
		return err
	case err != nil:
		return err
	}
	log.Printf("store: choir_events holds rows; below the F-c rollback floor, left in place")
	return nil
}
