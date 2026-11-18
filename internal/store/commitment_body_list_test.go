package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list for ListCommitmentRecordsByBodyOfComputer's window and absent
// predicates (the heard reads), written before the code:
//
//	B1 a row last written before Since is read or returned;
//	B2 a row that holds an Absent path (a take's take_id) is returned;
//	B3 with a sparse match (3 of 20,000 commitment records) the read walks the
//	    rows outside the window, and the time is not stated.

func arHeardSettlement(t *testing.T, stretch string, at time.Time) types.CommitmentRecord {
	t.Helper()
	claim := arForecast(t, stretch, 0, at)
	rec, err := calibration.NewAutoradioSettlementRecord(claim, calibration.AutoradioHeardRule, true, "air-"+stretch, at)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func heardQuery(since time.Time) types.BodyListQuery {
	return types.BodyListQuery{Since: since, Match: map[string]string{
		"$.subject_refs.record_kind":             "autoradio.settlement",
		"$.calibration.settlement.scorer_method": types.CalibrationScorerRule,
		"$.calibration.settlement.scorer_name":   calibration.AutoradioHeardRule,
		"$.calibration.settlement.outcome":       types.CalibrationOutcomeTrue,
	}}
}

func backdate(t *testing.T, s *Store, canonicalID string, at time.Time) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), `UPDATE og_objects SET updated_at = ? WHERE canonical_id = ?`, at.UTC(), canonicalID); err != nil {
		t.Fatal(err)
	}
}

func TestB1_RowsWrittenBeforeSinceAreNotReadOrReturned(t *testing.T) {
	s := openAutoradioStore(t)
	now := time.Now()
	var ids []string
	for i := 0; i < 3; i++ {
		rec := arHeardSettlement(t, fmt.Sprintf("st%d", i), now)
		rec.ComputerID = arComputer
		id, err := s.AppendCommitmentRecord(context.Background(), arOwner, arComputer, rec)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	backdate(t, s, ids[1], now.Add(-20*24*time.Hour))
	recs, _, rows, err := s.ListCommitmentRecordsByBodyOfComputer(context.Background(), arOwner, arComputer, heardQuery(now.Add(-14*24*time.Hour)), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || rows != 2 {
		t.Errorf("(B1) %d records, %d rows read; want 2 and 2 (the 20-day-old one is neither read nor returned)", len(recs), rows)
	}
	if recs, _, rows, _ = s.ListCommitmentRecordsByBodyOfComputer(context.Background(), arOwner, arComputer, heardQuery(time.Time{}), 100); len(recs) != 3 || rows != 3 {
		t.Errorf("(B1) a zero Since reads %d rows, want all 3", rows)
	}
}

func TestB2_AnAbsentPathLeavesTakesOut(t *testing.T) {
	s := openAutoradioStore(t)
	label, _ := types.AutoradioKindLabel("autoradio.segment")
	for i, take := range []string{"", "take-0123456789abcdef"} {
		refs := map[string]string{"desk": "app:autoradio", "record_kind": "autoradio.segment", "forecast": "true"}
		if take != "" {
			refs["take_id"] = take
		}
		rec := types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: fmt.Sprintf("autoradio-seg:b2/%d", i), Kind: types.CommitmentKindDirective, Discrepancy: types.DiscrepancyUnresolved,
			Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: "t", Body: "b"},
			Provenance:  types.CommitmentProvenance{AgentID: "app:autoradio:" + arOwner, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano), Labels: types.NewLabelSet(label).Strings()},
			EmitterKind: types.CommitmentEmitterAgent, SubjectRefs: refs,
		}
		arPut(t, s, arOwner, arComputer, rec)
	}
	q := types.BodyListQuery{Match: map[string]string{"$.subject_refs.record_kind": "autoradio.segment", "$.subject_refs.forecast": "true"}, Absent: []string{"$.subject_refs.take_id"}}
	recs, _, _, err := s.ListCommitmentRecordsByBodyOfComputer(context.Background(), arOwner, arComputer, q, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].SubjectRefs["take_id"] != "" {
		t.Errorf("(B2) %d records, a take among them: %+v", len(recs), recs)
	}
}

func TestB3_ASparseMatchOverTwentyThousandRecordsIsBoundedByTheWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("seeds 20,000 records")
	}
	s := openAutoradioStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	label, _ := types.AutoradioKindLabel("autoradio.note")
	// 20,000 records of the same kind that do not match: half last written
	// inside the 14 days, half 30 days ago.
	var batch []objectgraph.Object
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: batch}); err != nil {
			t.Fatal(err)
		}
		batch = batch[:0]
	}
	for i := 0; i < 20000; i++ {
		at := now.Add(-time.Duration(i%10) * time.Hour)
		if i%2 == 1 {
			at = now.Add(-30 * 24 * time.Hour).Add(-time.Duration(i%10) * time.Hour)
		}
		rec := types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: fmt.Sprintf("autoradio-note:sparse-%d", i), Kind: types.CommitmentKindDirective, Discrepancy: types.DiscrepancyUnresolved,
			Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: "n", Body: "n"},
			Provenance:  types.CommitmentProvenance{AgentID: "app:autoradio:" + arOwner, CommittedAt: at.Format(time.RFC3339Nano), Labels: types.NewLabelSet(label).Strings()},
			EmitterKind: types.CommitmentEmitterAgent, ComputerID: arComputer, SubjectRefs: map[string]string{"desk": "app:autoradio", "record_kind": "autoradio.note"},
		}
		obj, err := lifecycleObject(ogKindCommitmentRecord, arOwner, arComputer, rec.RecordID, rec, map[string]any{"record_id": rec.RecordID}, at, at)
		if err != nil {
			t.Fatal(err)
		}
		batch = append(batch, obj)
		if len(batch) == 500 {
			flush()
		}
	}
	flush()
	// Three heard settlements inside the window and one 20 days old.
	var old string
	for i := 0; i < 4; i++ {
		rec := arHeardSettlement(t, fmt.Sprintf("sp%d", i), now)
		rec.ComputerID = arComputer
		id, err := s.AppendCommitmentRecord(ctx, arOwner, arComputer, rec)
		if err != nil {
			t.Fatal(err)
		}
		old = id
	}
	backdate(t, s, old, now.Add(-20*24*time.Hour))
	t0 := time.Now()
	recs, _, rows, err := s.ListCommitmentRecordsByBodyOfComputer(ctx, arOwner, arComputer, heardQuery(now.Add(-14*24*time.Hour)), 2000)
	took := time.Since(t0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("(B3) sparse heard read over 20,000 other records (half outside the window): %v, %d rows", took, rows)
	if len(recs) != 3 || rows != 3 {
		t.Errorf("(B3) %d records, %d rows; want 3 and 3", len(recs), rows)
	}
	if took > 2*time.Second {
		t.Errorf("(B3) the sparse read took %v, over the stated 2 s", took)
	}
}
