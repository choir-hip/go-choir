package main

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbcred"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

// bootstrapSchemas creates every table the new release keeps in the control
// and graph databases, so the copy does not drop a source table for want of
// its target:
//
//   - the platform store bootstraps both databases (WP-D);
//   - vmctl's route ledger and immutable input catalog keep their tables
//     (computer_version_*) in control and create them in vmctl at start
//     (cmd/vmctl/main.go), so they are created here too.
//
// The retrieval tables (og_lookup_keys, proj_capture_*) are not copied:
// corpusd rebuilds them from og_objects at start (StartRetrievalMigrations).
// --bootstrap=false skips this and requires the schemas to exist.
func bootstrapSchemas(ctx context.Context, controlDSN, graphDSN string) error {
	s, err := platform.OpenStore(ctx, controlDSN, graphDSN)
	if err != nil {
		return err
	}
	if err := s.Close(); err != nil {
		return err
	}
	dsn, err := dbcred.ApplyPGDSN(controlDSN)
	if err != nil {
		return err
	}
	db, err := dbx.OpenPostgres(dsn, dbx.PGOptions{MaxOpenConns: 2})
	if err != nil {
		return err
	}
	defer db.Close()
	if err := computerversion.NewSQLInputCatalog(db).EnsureSchema(ctx); err != nil {
		return fmt.Errorf("immutable input catalog schema: %w", err)
	}
	if err := routeledger.NewSQLLedger(db).EnsureSchema(ctx); err != nil {
		return fmt.Errorf("route ledger schema: %w", err)
	}
	return nil
}
