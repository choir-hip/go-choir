package platform

import (
	"context"
	"strings"
	"testing"
)

// The capture eligibility predicate must be run as a probe per row (one
// primary-key read of og_objects for each row of the driving table), never
// pulled up into a join the planner may drive from og_objects. On the paper's
// computer (216,942 projected captures, 2.2 million objects, 2026-11-19) the
// pulled-up form was planned as a nested loop from og_objects' web captures
// (estimated 2 rows, really 221,335) into an index range of
// proj_capture_index that matches every capture of the computer: 221,335
// times 216,942 rows. The worker's first statement, the gap count, ran 25
// minutes without ending and nothing was embedded. With the fence the same
// count took 2.3 seconds there.
//
// A small test table cannot show the slow plan, so the test holds the shape:
// the plan of each statement built on the predicate has the predicate as a
// SubPlan, which is what PostgreSQL prints for a sublink it did not pull up.
func TestCaptureEligibilityIsProbedPerRowNotJoined(t *testing.T) {
	f := newVecFixture(t)
	ctx := context.Background()
	if err := f.store.ensureVectorTables(ctx); err != nil {
		t.Fatal(err)
	}
	for name, q := range map[string]string{
		"want":      `SELECT COUNT(*) FROM proj_capture_index v WHERE ` + vecCaptureEligibleSQL,
		"have":      `SELECT COUNT(*) FROM vec_captures v WHERE v.model = 'm' AND ` + vecCaptureEligibleSQL,
		"gap count": `SELECT COUNT(*) FROM proj_capture_index v WHERE ` + vecCaptureEligibleSQL + ` AND NOT EXISTS (SELECT 1 FROM vec_captures x WHERE x.capture_id = v.capture_id AND x.model = 'm')`,
		"gap batch": `SELECT v.capture_id FROM proj_capture_index v WHERE ` + vecCaptureEligibleSQL + ` AND NOT EXISTS (SELECT 1 FROM vec_captures x WHERE x.capture_id = v.capture_id AND x.model = 'm') ORDER BY COALESCE(v.published_at, v.fetched_at, v.updated_at) DESC, v.capture_id DESC LIMIT 16`,
		"sweep":     `DELETE FROM vec_captures v WHERE NOT (` + vecCaptureEligibleSQL + `)`,
	} {
		rows, err := f.store.corpus().QueryContext(ctx, `EXPLAIN `+q)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var plan []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, line)
		}
		rows.Close()
		text := strings.Join(plan, "\n")
		if !strings.Contains(text, "SubPlan") {
			t.Errorf("%s: the eligibility predicate was pulled up into a join; the planner may drive it from og_objects:\n%s", name, text)
		}
	}
}
