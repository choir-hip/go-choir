package store

import (
	"database/sql"
	"errors"
	"log"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// Rollback below F-c deploy 1 (docs/design/choir-event-table-f-c-2026-10-22.md,
// status section, round 1b). The epoch fence covers only Apply of a release
// that declares an epoch; a CI revert, a platform-follow offer and the
// updater's restorePrior reach the older binary unfenced. That binary
// runs, but its replay manifest calls the empty choir_events unknown, so
// checkpoint publication is refused until a forward deploy or a drop.
//
// rollbackRecipeDropEmptyChoirEvents is the boot step the revert commit adds
// to bootstrap, after the retiredRuntimeTables loop, verbatim (the design
// quotes it). It drops choir_events only while the table is empty. A row
// means an op was stamped: the computer is below the rollback floor, and the
// table stays (the repair is forward, design 4.5). It compiles against the
// pre-deploy-1 store package, which imports database/sql, errors and log.
func rollbackRecipeDropEmptyChoirEvents(db *sql.DB) error {
	var name string
	switch err := db.QueryRow(`SHOW TABLES LIKE 'choir_events'`).Scan(&name); {
	case errors.Is(err, sql.ErrNoRows):
		return nil // never created, or already dropped
	case err != nil:
		return err
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

// The recipe, rehearsed: on a deploy-1 store with no stamped op the drop
// restores the pre-deploy-1 witness exactly (what the older manifest needs
// to publish again); a table with a row is left alone; a store without the
// table is untouched; a second run is a no-op.
func TestEventTableRollbackRecipe(t *testing.T) {
	tape := eventTableMixedTape(t)
	s, path := vocabUpcastOpenStore(t, "rollback-recipe")
	vocabUpcastApplyLiveEvent(t, s, tape[0])
	vocabUpcastApplyLiveEvent(t, s, tape[1]) // unstamped
	if _, err := s.db.Exec(`DROP TABLE choir_events`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	_, before := eventTableWitness(t, path) // the pre-deploy-1 checkpoint

	d1, err := Open(path) // deploy 1 boots: the empty table exists
	if err != nil {
		t.Fatal(err)
	}
	if tableCount(t, d1, "choir_events", "") != 0 {
		t.Fatal("fixture: deploy 1 did not create an empty table")
	}
	if err := rollbackRecipeDropEmptyChoirEvents(d1.db); err != nil {
		t.Fatalf("recipe on an empty table: %v", err)
	}
	if err := rollbackRecipeDropEmptyChoirEvents(d1.db); err != nil {
		t.Fatalf("recipe twice: %v", err)
	}
	var name string
	if err := d1.db.QueryRow(`SHOW TABLES LIKE 'choir_events'`).Scan(&name); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the empty table survived the recipe (%q, %v)", name, err)
	}
	if err := d1.Close(); err != nil {
		t.Fatal(err)
	}
	set, after := eventTableWitness(t, path)
	if observed(set, "dolt:texture:schema:choir_events") {
		t.Fatal("the dropped table is still observed")
	}
	if err := selfdevprotocol.WitnessContentMatches(after, before); err != nil {
		t.Fatalf("after the recipe the store does not match its pre-deploy-1 witness: %v", err)
	}

	// Below the floor: a stamped row keeps the table.
	stamped, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stamped.Close() })
	vocabUpcastApplyLiveEvent(t, stamped, tape[2]) // stamped
	if err := rollbackRecipeDropEmptyChoirEvents(stamped.db); err != nil {
		t.Fatal(err)
	}
	if tableCount(t, stamped, "choir_events", "") != 1 {
		t.Fatal("the recipe dropped a table holding a stamped row")
	}
}
