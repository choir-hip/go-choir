package searchplane

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MonthlyQuota is a provider's monthly free allowance, spent at a pace:
// each UTC day's cap is what is left of the period's target divided by the
// days left in the period, today included
// (docs/archive/problems/search-throughput-keenable-underused-2026-10-15.md).
// Because the remainder is re-divided every day, a quiet day raises the
// later days' caps, and the period total never passes the target: on day
// k of n, used + cap <= used + (target-used)/(n-k+1) <= target.
//
// The counts are the health store's durable daily counts, so a gateway
// restart does not reset the pace.
type MonthlyQuota struct {
	// Allowance is the provider's free requests per billing period; 0 means
	// no monthly quota.
	Allowance int
	// RenewDay is the day of the month (UTC) a billing period starts,
	// clamped to the month's last day (31 in a 30-day month is the 30th).
	RenewDay int
	// TargetPct is the share of Allowance the pace spends, 1..100. The
	// rest is the safety margin: billing-period edges we cannot see
	// (the provider's renewal hour), calls the counter missed.
	TargetPct int
	// SeedUsed is calls made in the period containing SeedDay, before
	// SeedDay, that the counter never saw (from the provider's billing
	// page; the daily counts began mid-period). Ignored outside that
	// period. SeedDay is YYYY-MM-DD; empty means no seed.
	SeedUsed int
	SeedDay  string
}

// Defaults for Keenable's free plan (owner, 2026-10-15: 100,000 free
// requests a month, renewing on the 5th, 10 requests/second; no payment
// method on file, so overflow is not an option).
const (
	DefaultKeenableMonthlyAllowance = 100000
	DefaultKeenableRenewDay         = 5
	DefaultKeenableTargetPct        = 90
)

// Target is the requests the pace spends in one period.
func (q MonthlyQuota) Target() int {
	pct := q.TargetPct
	if pct <= 0 || pct > 100 {
		pct = 100
	}
	return q.Allowance * pct / 100
}

// renewalIn is the period start in the month of y/m: RenewDay clamped to
// the month's length.
func (q MonthlyQuota) renewalIn(y int, m time.Month) time.Time {
	day := q.RenewDay
	if day < 1 {
		day = 1
	}
	if last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day(); day > last {
		day = last
	}
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// Period returns the billing period containing t's UTC day: [start, end).
func (q MonthlyQuota) Period(t time.Time) (start, end time.Time) {
	y, m, d := t.UTC().Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	start = q.renewalIn(y, m)
	if today.Before(start) {
		start = q.renewalIn(y, m-1)
	}
	sy, sm, _ := start.Date()
	return start, q.renewalIn(sy, sm+1)
}

// MonthlyPace is one day's monthly budget for a provider.
type MonthlyPace struct {
	Day         string
	PeriodStart string
	PeriodEnd   string
	// Used is the period's calls before Day (the seed included).
	Used     int
	Target   int
	DaysLeft int
	// Cap is Day's call cap: (Target-Used)/DaysLeft, at least 0.
	Cap int
}

// pace computes the day's cap from the counts. callsOn returns a day's
// count for the provider.
func (q MonthlyQuota) pace(now time.Time, callsOn func(day string) (int, error)) (MonthlyPace, error) {
	start, end := q.Period(now)
	y, m, d := now.UTC().Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	p := MonthlyPace{
		Day:         DayOf(today),
		PeriodStart: DayOf(start),
		PeriodEnd:   DayOf(end),
		Target:      q.Target(),
		DaysLeft:    int(end.Sub(today).Hours()/24 + 0.5),
	}
	if p.DaysLeft < 1 {
		p.DaysLeft = 1
	}
	from := start
	if seed, err := time.Parse("2006-01-02", strings.TrimSpace(q.SeedDay)); err == nil && !seed.Before(start) && seed.Before(end) {
		p.Used += q.SeedUsed
		from = seed
	}
	for day := from; day.Before(today); day = day.AddDate(0, 0, 1) {
		n, err := callsOn(DayOf(day))
		if err != nil {
			return p, err
		}
		p.Used += n
	}
	if left := p.Target - p.Used; left > 0 {
		p.Cap = left / p.DaysLeft
	}
	return p, nil
}

// ParseQuotaSeed reads "N@YYYY-MM-DD" (calls used before that day in its
// billing period). Empty or malformed is no seed.
func ParseQuotaSeed(raw string) (used int, day string, ok bool) {
	n, d, found := strings.Cut(strings.TrimSpace(raw), "@")
	if !found {
		return 0, "", false
	}
	used, err := strconv.Atoi(strings.TrimSpace(n))
	if err != nil || used < 0 {
		return 0, "", false
	}
	d = strings.TrimSpace(d)
	if _, err := time.Parse("2006-01-02", d); err != nil {
		return 0, "", false
	}
	return used, d, true
}

// FormatMonthlyQuotas renders quotas for the startup log line.
func FormatMonthlyQuotas(quotas map[string]MonthlyQuota) string {
	parts := make([]string, 0, len(quotas))
	for _, name := range sortedKeys(quotas) {
		q := quotas[name]
		entry := fmt.Sprintf("%s=%d@%d target=%d", name, q.Allowance, q.RenewDay, q.Target())
		if q.SeedDay != "" {
			entry += fmt.Sprintf(" seed=%d@%s", q.SeedUsed, q.SeedDay)
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, ",")
}

func sortedKeys(m map[string]MonthlyQuota) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
