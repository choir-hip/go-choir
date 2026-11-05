package trust

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Cost attribution and the anomaly notice (trust draft 9, BT7, MA7.7).
//
// There is no spend ceiling (owner, R8): nothing here refuses or caps a
// call. Every model call is attributed to computer, desk, package and
// publisher; a desk may precommit an estimated cost (E8, which moves
// calibration, not a cap); and spend far above the desk's own history
// notifies the owner. The anomaly is itself I2-material, so an untrusted desk
// burning money meets the act gate (MA7.8), not a ceiling.

// UnattributedDesk groups spend whose entry names no desk.
const UnattributedDesk = "(unattributed)"

// SpendAnomalyClass is the class of a spend anomaly: the trust draft's I2
// example "spend above the cost-anomaly line".
var SpendAnomalyClass = ActionClass{Grant: "spend.anomaly", Impact: types.ImpactI2}

// CostPoint is one metered model call, attributed.
type CostPoint struct {
	Desk         string
	Model        string
	USD          float64
	InputTokens  int
	OutputTokens int
	At           time.Time
}

// PointsFromEntries attributes cost entries to desks by their agent id.
func PointsFromEntries(entries []llmcost.CostEntry) []CostPoint {
	out := make([]CostPoint, 0, len(entries))
	for _, e := range entries {
		out = append(out, CostPoint{Desk: DeskOf(e.AgentID), Model: e.Model, USD: e.Cost.USD,
			InputTokens: e.Cost.InputTokens, OutputTokens: e.Cost.OutputTokens, At: e.Timestamp})
	}
	return out
}

// SpendRow is the spend of one desk, package or publisher.
type SpendRow struct {
	Key          string  `json:"key"`
	USD          float64 `json:"usd"`
	Calls        int     `json:"calls"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
}

// SpendReport groups spend by desk, package and publisher.
type SpendReport struct {
	TotalUSD    float64    `json:"total_usd"`
	ByDesk      []SpendRow `json:"by_desk"`
	ByPackage   []SpendRow `json:"by_package"`
	ByPublisher []SpendRow `json:"by_publisher"`
}

// AttributeCost groups points by desk, and through the resolver by package and
// publisher. Spend with no desk is kept under UnattributedDesk, never dropped.
func AttributeCost(points []CostPoint, resolve SubjectResolver) SpendReport {
	desk, pkg, pub := map[string]*SpendRow{}, map[string]*SpendRow{}, map[string]*SpendRow{}
	add := func(m map[string]*SpendRow, key string, p CostPoint) {
		r := m[key]
		if r == nil {
			r = &SpendRow{Key: key}
			m[key] = r
		}
		r.USD += p.USD
		r.Calls++
		r.InputTokens += p.InputTokens
		r.OutputTokens += p.OutputTokens
	}
	var rep SpendReport
	for _, p := range points {
		rep.TotalUSD += p.USD
		d := p.Desk
		if d == "" {
			d = UnattributedDesk
		}
		add(desk, d, p)
		if resolve != nil && p.Desk != "" {
			info := resolve(p.Desk)
			if info.Package != "" {
				add(pkg, info.Package, p)
			}
			if info.Publisher != "" {
				add(pub, info.Publisher, p)
			}
		}
	}
	flat := func(m map[string]*SpendRow) []SpendRow {
		out := []SpendRow{}
		for _, r := range m {
			out = append(out, *r)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].USD != out[j].USD {
				return out[i].USD > out[j].USD
			}
			return out[i].Key < out[j].Key
		})
		return out
	}
	rep.ByDesk, rep.ByPackage, rep.ByPublisher = flat(desk), flat(pkg), flat(pub)
	return rep
}

// AnomalyParams set when spend counts as anomalous. They are starting values,
// logged with every scan.
type AnomalyParams struct {
	// Window is the length of the recent window and of each history bucket.
	Window time.Duration
	// MaxHistory bounds how far back the baseline reads.
	MaxHistory time.Duration
	// MinActive is the least number of earlier windows with spend a desk
	// needs before it has a baseline.
	MinActive int
	// K scales the robust deviation: threshold = median + K * scale.
	K float64
	// MinMultiple is the least multiple of the median that counts.
	MinMultiple float64
	// MinUSD is the absolute floor: spend below it never pages the owner.
	MinUSD float64
}

// DefaultAnomalyParams are hourly windows over a week.
func DefaultAnomalyParams() AnomalyParams {
	return AnomalyParams{Window: time.Hour, MaxHistory: 7 * 24 * time.Hour, MinActive: 3, K: 4, MinMultiple: 3, MinUSD: 0.25}
}

// Anomaly is a desk whose spend in the recent window is far above its own
// history.
type Anomaly struct {
	Desk, Package, Publisher string
	WindowStart, WindowEnd   time.Time
	USD                      float64
	Median, Threshold        float64
	Multiple                 float64
	Class                    ActionClass
}

// DetectCostAnomalies finds the desks whose spend in (now-Window, now] is far
// above the robust baseline (median and MAD of their earlier windows, from
// their first window of spend). A desk with fewer than MinActive earlier
// windows of spend has no baseline and no anomaly.
func DetectCostAnomalies(points []CostPoint, now time.Time, p AnomalyParams, resolve SubjectResolver) []Anomaly {
	if p.Window <= 0 {
		p = DefaultAnomalyParams()
	}
	byDesk := map[string][]CostPoint{}
	for _, pt := range points {
		if pt.Desk != "" && !pt.At.After(now) && pt.USD > 0 {
			byDesk[pt.Desk] = append(byDesk[pt.Desk], pt)
		}
	}
	desks := make([]string, 0, len(byDesk))
	for d := range byDesk {
		desks = append(desks, d)
	}
	sort.Strings(desks)
	recentStart := now.Add(-p.Window)
	var out []Anomaly
	for _, d := range desks {
		var recent float64
		hist := map[int64]float64{}
		first := now
		for _, pt := range byDesk[d] {
			if pt.At.After(recentStart) {
				recent += pt.USD
				continue
			}
			if now.Sub(pt.At) > p.MaxHistory {
				continue
			}
			// bucket index counted back from the recent window
			b := int64(recentStart.Sub(pt.At) / p.Window)
			hist[b] += pt.USD
			if pt.At.Before(first) {
				first = pt.At
			}
		}
		if len(hist) < p.MinActive || recent <= 0 {
			continue
		}
		span := int64(recentStart.Sub(first)/p.Window) + 1
		vals := make([]float64, 0, span)
		for b := int64(0); b < span; b++ {
			vals = append(vals, hist[b])
		}
		med := median(vals)
		devs := make([]float64, len(vals))
		for i, v := range vals {
			devs[i] = math.Abs(v - med)
		}
		scale := math.Max(math.Max(1.4826*median(devs), 0.25*med), 0.01)
		threshold := math.Max(math.Max(med+p.K*scale, p.MinMultiple*med), p.MinUSD)
		if recent <= threshold {
			continue
		}
		a := Anomaly{Desk: d, WindowStart: recentStart, WindowEnd: now, USD: recent, Median: med, Threshold: threshold, Class: SpendAnomalyClass}
		if med > 0 {
			a.Multiple = recent / med
		}
		if resolve != nil {
			info := resolve(d)
			a.Package, a.Publisher = info.Package, info.Publisher
		}
		out = append(out, a)
	}
	return out
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	if n := len(c); n%2 == 1 {
		return c[n/2]
	}
	return (c[len(c)/2-1] + c[len(c)/2]) / 2
}

const costAnomalyAgentID = "system:cost-anomaly"

// NoticeRecordID is deterministic in the desk and the window, so a repeated
// scan of the same window commits the same record.
func (a Anomaly) NoticeRecordID() string {
	return fmt.Sprintf("cost-anomaly:%s:%d", a.Desk, a.WindowEnd.UTC().Truncate(time.Hour).Unix())
}

// Notice is the owner notice for an anomaly: a system record to the
// management inbox (as the unattended-request notice), which wakes no one.
// It states the fact and that there is no ceiling.
func (a Anomaly) Notice(managementInbox, computerID string) types.CommitmentRecord {
	body := fmt.Sprintf("Spend by the %s desk in the last %s was $%.2f, %.1f times its usual $%.2f per window (the notice line is $%.2f). There is no spend ceiling: this is a notice, and an anomaly is I2-material, so a desk without a strong record in this class meets the act gate.",
		a.Desk, a.WindowEnd.Sub(a.WindowStart).Round(time.Minute), a.USD, a.Multiple, a.Median, a.Threshold)
	refs := map[string]string{"desk": a.Desk, "usd": fmt.Sprintf("%.4f", a.USD), "median_usd": fmt.Sprintf("%.4f", a.Median),
		"threshold_usd": fmt.Sprintf("%.4f", a.Threshold), "class": a.Class.String()}
	if a.Package != "" {
		refs["package"] = a.Package
	}
	if a.Publisher != "" {
		refs["publisher"] = a.Publisher
	}
	inbox := strings.TrimSpace(managementInbox)
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: a.NoticeRecordID(), Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: costAnomalyAgentID, CommittedAt: a.WindowEnd.UTC().Format(time.RFC3339)},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
		Inbox: inbox, SenderInbox: "cost:" + a.Desk, Addressee: inbox,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Body: body},
		SubjectRefs: refs,
	}
}
