package calibration

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// fakeHeardLedger answers the heard read with fixed rows. It answers both the
// subject-ref list and the predicate list, so the test does not depend on
// which one the read uses.
type fakeHeardLedger struct {
	recs        []types.CommitmentRecord
	undecodable int
}

func (f fakeHeardLedger) ListCommitmentRecordsBySubjectRefOfComputer(_ context.Context, _, _, _, _ string, _ int) ([]types.CommitmentRecord, int, int, error) {
	return f.recs, f.undecodable, len(f.recs) + f.undecodable, nil
}

func (f fakeHeardLedger) ListCommitmentRecordsByBodyOfComputer(_ context.Context, _, _ string, _ map[string]string, _ int) ([]types.CommitmentRecord, int, int, error) {
	return f.recs, f.undecodable, len(f.recs) + f.undecodable, nil
}

func heardSettlement(t *testing.T, seg int, heard bool, resolvedAt string) types.CommitmentRecord {
	t.Helper()
	claim, err := NewAutoradioForecast(forecastInput(t, "st-heard", seg), mustTime(t, testNow))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := NewAutoradioSettlementRecord(claim, AutoradioHeardRule, heard, "air-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if resolvedAt != "" {
		rec.Provenance.ResolvedAt = resolvedAt
	}
	return rec
}

// R12: a settlement whose time does not parse is counted, never silently
// skipped; an undecodable row never fails the read.
func TestListAutoradioHeardCountsWhatItCannotUse(t *testing.T) {
	led := fakeHeardLedger{
		recs: []types.CommitmentRecord{
			heardSettlement(t, 0, true, ""),
			heardSettlement(t, 1, true, "not a time"),
			heardSettlement(t, 2, false, ""),
		},
		undecodable: 2,
	}
	got, rd, err := ListAutoradioHeard(context.Background(), led, "o", "c", 100)
	if err != nil {
		t.Fatalf("an undecodable row failed the read: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("heard = %d, want 1 (the true one that parses; the false one is not heard)", len(got))
	}
	if rd.Undecodable != 2 || rd.Unparsable != 1 {
		t.Errorf("read = %+v, want 2 undecodable and 1 unparsable", rd)
	}
}
