package objectgraph

import (
	"context"
	"strings"
	"testing"
)

// The model provenance of a Texture document (review r1, B3;
// docs/problems/model-provenance-unwired-2026-11-05.md) reads only through
// generated, indexed metadata columns: runs by channel and trajectory, the
// engineering reports of a trajectory, and a run's model.served lines. The
// fields it no longer reads (requested_by_run_id, a report's loop_id) have
// no column, and their plans scan the kind, which is why they were dropped.
func TestModelProvenanceReadsUseTheMetadataIndexes(t *testing.T) {
	db := openTestDB(t)
	store := NewSQLStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, read := range []struct{ kind, field, column string }{
		{"choir.run", "channel_id", "og_meta_channel_id"},
		{"choir.run", "trajectory_id", "og_meta_trajectory_id"},
		{"choir.co_super_assignment_report", "trajectory_id", "og_meta_trajectory_id"},
		{"choir.event", "run_id", "og_meta_run_id"},
	} {
		query, args := metadataPageSQL(read.kind, "$."+read.field, "value", "", 200)
		assertIndexedPlan(t, explainPlan(t, db, query, args...), "idx_og_objects_"+read.column, true)
	}
	for _, read := range []struct{ kind, field string }{
		{"choir.run", "requested_by_run_id"},
		{"choir.co_super_assignment_report", "loop_id"},
	} {
		query, args := metadataPageSQL(read.kind, "$."+read.field, "value", "", 200)
		if plan := explainPlan(t, db, query, args...); strings.Contains(plan, "og_meta_") {
			t.Fatalf("%s.%s gained an indexed column; the provenance may read it again:\n%s", read.kind, read.field, plan)
		}
	}
}
