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
	ListCalibrationClaimsByTagPage(ctx context.Context, ownerID, computerID, tag, after string, limit int) ([]types.CommitmentRecord, string, error)
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

// StretchForecasts reads the forecast claims of one stretch by its tag.
func (r Reader) StretchForecasts(ctx context.Context, ownerID, computerID, stretchID string) (claims []types.CommitmentRecord, truncated bool, err error) {
	page, max := r.sizes()
	return readPages(func(after string, limit int) ([]types.CommitmentRecord, string, error) {
		return r.Ledger.ListCalibrationClaimsByTagPage(ctx, ownerID, computerID, "stretch:"+stretchID, after, limit)
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
