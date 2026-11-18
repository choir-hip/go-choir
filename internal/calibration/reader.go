package calibration

import (
	"context"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	// DefaultAutoradioPageSize is how many records one page of the notebook's
	// and the measurement's reads holds.
	DefaultAutoradioPageSize = 500
	// DefaultAutoradioMaxRecords bounds the memory of one read. It is far above
	// the generic route's DefaultMaxRecords: listening writes about 60 records
	// an hour (claims and settlements), so this is months of it. A read that
	// reaches it says so.
	DefaultAutoradioMaxRecords = 200000
)

// PagedLedger is the cursor-paged slice of the guest store the notebook and
// the measurement read through (design 4.3). A page's next cursor is "" when
// it is the last.
type PagedLedger interface {
	ListCalibrationRecordsPage(ctx context.Context, ownerID, computerID, agent, after string, limit int) ([]types.CommitmentRecord, string, error)
	ListCalibrationClaimsByTagPage(ctx context.Context, ownerID, computerID, agent, recordKind, tag, after string, limit int) ([]types.CommitmentRecord, string, error)
}

// Reader reads the reserved namespace by cursor until it is exhausted. The
// notebook and the measurement are not capped at the generic route's 10000
// records; a read that stops at MaxRecords says truncated, never silently.
type Reader struct {
	Ledger     PagedLedger
	PageSize   int // 0: DefaultAutoradioPageSize
	MaxRecords int // 0: DefaultAutoradioMaxRecords
}

func (r Reader) sizes() (page, max int) {
	page, max = r.PageSize, r.MaxRecords
	if page <= 0 {
		page = DefaultAutoradioPageSize
	}
	if max <= 0 {
		max = DefaultAutoradioMaxRecords
	}
	return page, max
}

type pageFn func(after string, limit int) ([]types.CommitmentRecord, string, error)

func readPages(fetch pageFn, pageSize, maxRecords int) ([]types.CommitmentRecord, bool, error) {
	var out []types.CommitmentRecord
	after := ""
	for {
		limit := min(pageSize, maxRecords-len(out))
		recs, next, err := fetch(after, limit)
		if err != nil {
			return nil, false, err
		}
		out = append(out, recs...)
		if next == "" {
			return out, false, nil
		}
		if len(out) >= maxRecords {
			return out, true, nil
		}
		after = next
	}
}

// Records reads every calibration record of agent autoradio-dj, claims and
// settlements, oldest first. truncated is true only when the read stopped at
// MaxRecords with more to read.
func (r Reader) Records(ctx context.Context, ownerID, computerID string) (records []types.CommitmentRecord, truncated bool, err error) {
	page, max := r.sizes()
	return readPages(func(after string, limit int) ([]types.CommitmentRecord, string, error) {
		return r.Ledger.ListCalibrationRecordsPage(ctx, ownerID, computerID, AutoradioAgent, after, limit)
	}, page, max)
}

// StretchForecasts reads the forecast claims of one stretch by its tag: claims
// of agent autoradio-dj and the forecast record kind only, so an ordinary
// claim that carries the same tag is not one.
func (r Reader) StretchForecasts(ctx context.Context, ownerID, computerID, stretchID string) (claims []types.CommitmentRecord, truncated bool, err error) {
	page, max := r.sizes()
	return readPages(func(after string, limit int) ([]types.CommitmentRecord, string, error) {
		return r.Ledger.ListCalibrationClaimsByTagPage(ctx, ownerID, computerID, AutoradioAgent, "autoradio.forecast", "stretch:"+stretchID, after, limit)
	}, page, max)
}

// Summary is the calibration summary of the reserved namespace: the same
// JoinClaims and Summarize the generic route uses, over every record read by
// cursor, with agent=autoradio-dj. There is no second scoring path.
func (r Reader) Summary(ctx context.Context, ownerID, computerID string, groupBy []string, now time.Time) (Summary, error) {
	records, truncated, err := r.Records(ctx, ownerID, computerID)
	if err != nil {
		return Summary{}, err
	}
	sum := Summarize(JoinClaims(records, now), SummaryOptions{GroupBy: groupBy, Filters: Filters{Agent: AutoradioAgent}}, now)
	sum.Truncated = truncated
	return sum, nil
}

// CauseStats is the scores of the forecasts that stated one cause class
// (the tag cause:<class>, design 12.1).
type CauseStats struct {
	Cause string `json:"cause"`
	GroupStats
}

// Scorecard is the listener's view of the DJ's forecasts: the overall
// summary, the summary by segment kind (the claim's project) and by cause
// class, and every claim joined with its settlements, newest first. It is
// Summarize over the same joined claims as Summary: there is no second
// scoring path.
type Scorecard struct {
	Overall   GroupStats
	ByKind    []GroupStats
	ByCause   []CauseStats
	Claims    []ClaimView
	Truncated bool
}

// Scorecard reads the reserved namespace once and scores it.
func (r Reader) Scorecard(ctx context.Context, ownerID, computerID string, now time.Time) (Scorecard, error) {
	records, truncated, err := r.Records(ctx, ownerID, computerID)
	if err != nil {
		return Scorecard{}, err
	}
	views := JoinClaims(records, now)
	opts := SummaryOptions{GroupBy: []string{"project"}, Filters: Filters{Agent: AutoradioAgent}}
	sum := Summarize(views, opts, now)
	card := Scorecard{Overall: sum.Overall, ByKind: sum.Groups, Claims: views, Truncated: truncated}
	for _, cause := range []string{CauseInstruction, CauseEdition, CauseImportance, CauseLesson, CauseTrying, CauseExplore} {
		var sub []ClaimView
		for _, v := range views {
			for _, t := range v.Tags {
				if t == "cause:"+cause {
					sub = append(sub, v)
					break
				}
			}
		}
		if len(sub) == 0 {
			continue
		}
		card.ByCause = append(card.ByCause, CauseStats{Cause: cause, GroupStats: Summarize(sub, opts, now).Overall})
	}
	return card, nil
}
