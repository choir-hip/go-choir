package calibration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list for the notebook's and the measurement's reads (design 4.3),
// written before the code:
//  1. a read stops at 10000 records, or at any count, without saying so;
//  2. a read that stops early (the record cap) says truncated=false;
//  3. a read that exhausts the set says truncated=true, or costs an extra call;
//  4. a record is repeated or skipped across pages of size 1, 2, 3 and larger;
//  5. the stretch's forecast read returns another stretch's, a settlement, or
//     is not paged;
//  6. AutoradioSummary scores anything but the reserved agent, scores by a
//     second path (its Overall differs from Summarize over the joined
//     records), or loses the Truncated flag;
//  7. the paged summary reads settlements that arrive on a later page as open.

func seedStretch(t *testing.T, s *store.Store, stretch string, n int, settleEvery int, at time.Time) []types.CommitmentRecord {
	t.Helper()
	var out []types.CommitmentRecord
	for i := 0; i < n; i++ {
		in := forecastInput(t, stretch, 0)
		in.SegmentID = fmt.Sprintf("%s.s%d", stretch, i)
		in.P = 0.6
		in.Cause, in.CausedBy, in.LessonID = CauseExplore, "", ""
		in.StretchStart = at
		rec, err := NewAutoradioForecast(in, at.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		rec.ComputerID = testComputer
		if _, err := s.AppendCommitmentRecord(context.Background(), testOwner, testComputer, rec); err != nil {
			t.Fatal(err)
		}
		out = append(out, rec)
		if settleEvery > 0 && i%settleEvery == 0 {
			res, err := SettleAutoradio(context.Background(), s, testOwner, testComputer, rec.RecordID, AutoradioHeardRule, i%2 == 0, fmt.Sprintf("ev-%s-%d", stretch, i), at.Add(time.Hour+time.Duration(i)*time.Second))
			if err != nil || !res.Won {
				t.Fatalf("settle: %v", err)
			}
			out = append(out, res.Record)
		}
	}
	return out
}

func TestReaderPagesAndReportsTruncation(t *testing.T) {
	s := openStore(t)
	at := mustTime(t, testStart)
	all := seedStretch(t, s, "rd", 9, 2, at) // 9 forecasts, 5 settlements
	seedStretch(t, s, "rd2", 3, 0, at)       // another stretch
	other := newOrdinaryClaim(t, s)
	_ = other
	ctx := context.Background()

	for _, size := range []int{1, 2, 3, 100} {
		r := Reader{Ledger: s, PageSize: size}
		recs, truncated, err := r.Records(ctx, testOwner, testComputer)
		if err != nil || truncated {
			t.Fatalf("size %d: truncated=%v err=%v", size, truncated, err)
		}
		if len(recs) != len(all)+3 {
			t.Fatalf("size %d: %d records, want %d", size, len(recs), len(all)+3)
		}
		seen := map[string]bool{}
		for _, rec := range recs {
			if seen[rec.RecordID] {
				t.Fatalf("size %d: %s repeated", size, rec.RecordID)
			}
			seen[rec.RecordID] = true
			if rec.Calibration.Agent != AutoradioAgent {
				t.Fatalf("read a record of agent %q", rec.Calibration.Agent)
			}
		}
	}
	// The cap stops the read and says so.
	r := Reader{Ledger: s, PageSize: 2, MaxRecords: 5}
	recs, truncated, err := r.Records(ctx, testOwner, testComputer)
	if err != nil || !truncated || len(recs) != 5 {
		t.Fatalf("capped: %d records truncated=%v err=%v", len(recs), truncated, err)
	}
	// A cap equal to the set size is not truncation.
	r = Reader{Ledger: s, PageSize: 2, MaxRecords: len(all) + 3}
	recs, truncated, err = r.Records(ctx, testOwner, testComputer)
	if err != nil || truncated || len(recs) != len(all)+3 {
		t.Fatalf("cap = size: %d truncated=%v err=%v", len(recs), truncated, err)
	}
	// More than the generic route's 10000 are readable: the default has no such cap.
	if DefaultAutoradioMaxRecords <= DefaultMaxRecords {
		t.Fatalf("default autoradio cap %d is not above the generic %d", DefaultAutoradioMaxRecords, DefaultMaxRecords)
	}

	// Forecasts of one stretch, by tag, claims only.
	fr := Reader{Ledger: s, PageSize: 4}
	claims, truncated, err := fr.StretchForecasts(ctx, testOwner, testComputer, "rd")
	if err != nil || truncated || len(claims) != 9 {
		t.Fatalf("stretch forecasts: %d truncated=%v err=%v", len(claims), truncated, err)
	}
	for _, c := range claims {
		if !IsClaim(&c) || !contains(c.Calibration.Claim.Tags, "stretch:rd") {
			t.Fatalf("returned %s", c.RecordID)
		}
	}
	fr.MaxRecords = 4
	claims, truncated, _ = fr.StretchForecasts(ctx, testOwner, testComputer, "rd")
	if !truncated || len(claims) != 4 {
		t.Fatalf("capped stretch read: %d truncated=%v", len(claims), truncated)
	}
}

func newOrdinaryClaim(t *testing.T, s *store.Store) types.CommitmentRecord {
	t.Helper()
	rec := claimRecord(t, "calib:plain:z", "someone-else")
	rec.ComputerID = testComputer
	rec.Provenance.AgentID = "agent:someone-else"
	if _, err := s.AppendCommitmentRecord(context.Background(), testOwner, testComputer, rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestAutoradioSummaryIsTheSameCalibrationCode(t *testing.T) {
	s := openStore(t)
	at := mustTime(t, testStart)
	seedStretch(t, s, "sm", 12, 1, at)
	newOrdinaryClaim(t, s)
	now := at.Add(48 * time.Hour)
	r := Reader{Ledger: s, PageSize: 5}
	sum, err := r.Summary(context.Background(), testOwner, testComputer, []string{"project"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Truncated {
		t.Fatal("truncated")
	}
	if sum.Filters.Agent != AutoradioAgent || sum.Overall.Claims != 12 || sum.Overall.Settled != 12 {
		t.Fatalf("overall = %+v filters %+v", sum.Overall, sum.Filters)
	}
	// Same arithmetic as Summarize over the same records.
	recs, _, _ := r.Records(context.Background(), testOwner, testComputer)
	want := Summarize(JoinClaims(recs, now), SummaryOptions{GroupBy: []string{"project"}, Filters: Filters{Agent: AutoradioAgent}}, now)
	if sum.Overall.BrierStrict == nil || want.Overall.BrierStrict == nil || *sum.Overall.BrierStrict != *want.Overall.BrierStrict ||
		sum.Overall.CausalBaseRateBrier == nil || *sum.Overall.CausalBaseRateBrier != *want.Overall.CausalBaseRateBrier {
		t.Fatalf("summary differs from Summarize: %+v vs %+v", sum.Overall, want.Overall)
	}
	if len(sum.Groups) != 1 || sum.Groups[0].Project != "story" {
		t.Fatalf("groups = %+v", sum.Groups)
	}
	// A capped read says truncated in the summary.
	capped := Reader{Ledger: s, PageSize: 5, MaxRecords: 7}
	sum, err = capped.Summary(context.Background(), testOwner, testComputer, nil, now)
	if err != nil || !sum.Truncated {
		t.Fatalf("capped summary truncated=%v err=%v", sum.Truncated, err)
	}
}
