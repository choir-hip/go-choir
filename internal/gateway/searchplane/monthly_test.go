package searchplane

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// Keenable's monthly pace and rate limit
// (docs/archive/problems/search-throughput-keenable-underused-2026-10-15.md).
// Ways it can fail, written before the code:
//  1. The period total passes the target (and so the allowance): on any
//     day, under demand above the pace, across restarts, or by concurrent
//     searches racing the day's cap.
//  2. A gateway restart forgets the period's use, so the pace restarts as
//     if nothing had been spent.
//  3. The period boundary is off: renewal day, a renewal day past the
//     month's end, the year end, the day before renewal.
//  4. The seed (use the counts never saw) is ignored, counted outside its
//     period, or added on top of counts it already covers.
//  5. The day's cap is all open at midnight, so a morning burst leaves the
//     afternoon without the provider; or it never fully opens.
//  6. Keenable is called faster than 8 a second (above its 10/s limit) by
//     concurrent searches from many computers.
//  7. Keenable, while it has budget, is not started first on a budget
//     search, or a paid provider starts beside it; once capped it counts as
//     a failure or a strike, is in N, or stops the search escalating.
//  8. A provider without a monthly quota changes behavior.
//  9. The env knobs: defaults wrong, pacing for one provider dropped when
//     another's rate is set, allowance 0 not turning the pace off.
//  10. The failure log line carries a key, a query or a response body, or
//      does not say why (status, stage).

func mustDay(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestMonthlyQuota_PeriodBoundaries(t *testing.T) { // 3
	q := MonthlyQuota{Allowance: 100, RenewDay: 5}
	cases := []struct {
		renew      int
		now        string
		start, end string
	}{
		{5, "2026-10-15", "2026-10-05", "2026-11-05"},
		{5, "2026-10-05", "2026-10-05", "2026-11-05"},
		{5, "2026-10-04", "2026-09-05", "2026-10-05"},
		{5, "2026-01-02", "2025-12-05", "2026-01-05"},
		{5, "2026-12-20", "2026-12-05", "2027-01-05"},
		{31, "2026-02-28", "2026-02-28", "2026-03-31"},
		{31, "2026-02-27", "2026-01-31", "2026-02-28"},
		{31, "2026-04-30", "2026-04-30", "2026-05-31"},
		{1, "2026-03-01", "2026-03-01", "2026-04-01"},
	}
	for _, c := range cases {
		q.RenewDay = c.renew
		start, end := q.Period(mustDay(c.now).Add(13 * time.Hour))
		if DayOf(start) != c.start || DayOf(end) != c.end {
			t.Errorf("renew %d on %s: period %s..%s, want %s..%s", c.renew, c.now, DayOf(start), DayOf(end), c.start, c.end)
		}
	}
}

func TestMonthlyQuota_PaceNeverPassesTheTarget(t *testing.T) { // 1, 5
	q := MonthlyQuota{Allowance: 100000, RenewDay: 5, TargetPct: 90}
	for _, demand := range []int{0, 1000, 2900, 3000, 5000, 15000, 1000000} {
		counts := map[string]int{}
		total := 0
		for d := mustDay("2026-10-05"); d.Before(mustDay("2026-11-05")); d = d.AddDate(0, 0, 1) {
			p, err := q.pace(d.Add(time.Hour), func(s string) (int, error) { return counts[s], nil })
			if err != nil {
				t.Fatal(err)
			}
			if p.Used != total {
				t.Fatalf("demand %d %s: used %d, counted %d", demand, DayOf(d), p.Used, total)
			}
			spent := demand
			if spent > p.Cap {
				spent = p.Cap
			}
			counts[DayOf(d)] = spent
			total += spent
		}
		if total > q.Target() {
			t.Fatalf("demand %d/day spent %d, target %d", demand, total, q.Target())
		}
		// Demand at or above the even pace spends nearly all of the target
		// (rounding leaves under one call a day).
		if demand >= 3000 && total < q.Target()-31 {
			t.Fatalf("demand %d/day spent only %d of %d", demand, total, q.Target())
		}
	}
}

func TestMonthlyQuota_Seed(t *testing.T) { // 4
	counts := map[string]int{"2026-10-13": 3000, "2026-10-14": 7000, "2026-10-15": 400}
	callsOn := func(s string) (int, error) { return counts[s], nil }
	q := MonthlyQuota{Allowance: 100000, RenewDay: 5, TargetPct: 90, SeedUsed: 10776, SeedDay: "2026-10-15"}
	p, err := q.pace(mustDay("2026-10-16"), callsOn)
	if err != nil {
		t.Fatal(err)
	}
	// The seed replaces the counts before its day; the 15th is counted.
	if p.Used != 10776+400 || p.DaysLeft != 20 || p.Cap != (90000-11176)/20 {
		t.Fatalf("pace = %+v", p)
	}
	// Next period: the seed no longer applies.
	p, _ = q.pace(mustDay("2026-11-05"), callsOn)
	if p.Used != 0 || p.Cap != 90000/30 {
		t.Fatalf("next period pace = %+v", p)
	}
	// Without a seed the earlier counts are what there is.
	q.SeedDay = ""
	p, _ = q.pace(mustDay("2026-10-16"), callsOn)
	if p.Used != 10400 {
		t.Fatalf("unseeded pace = %+v", p)
	}
	if n, d, ok := ParseQuotaSeed(" 10776@2026-10-15 "); !ok || n != 10776 || d != "2026-10-15" {
		t.Fatalf("seed parse = %d %s %v", n, d, ok)
	}
	for _, bad := range []string{"", "10776", "x@2026-10-15", "-1@2026-10-15", "5@2026-13-01"} {
		if _, _, ok := ParseQuotaSeed(bad); ok {
			t.Fatalf("seed %q accepted", bad)
		}
	}
}

func keenableQuotaConfig(allowance int) Config {
	cfg := budgetConfig()
	cfg.MonthlyQuotas = map[string]MonthlyQuota{"keenable": {Allowance: allowance, RenewDay: 5, TargetPct: 90}}
	return cfg
}

func TestMonthlyPace_AccruesThroughTheDay(t *testing.T) { // 5
	// 3100*0.9 = 2790 over 31 days: 90 a day.
	r := NewRouter(nil, NewMemoryHealthStore(), keenableQuotaConfig(3100))
	midnight := mustDay("2026-10-05")
	cases := []struct {
		at   time.Duration
		want int
	}{
		{0, 3},               // an hour's share open at midnight
		{11 * time.Hour, 45}, // half the day
		{23 * time.Hour, 90}, // all of it from 23:00
		{23*time.Hour + 59*time.Minute, 90},
	}
	for _, c := range cases {
		limit, until := r.limitAt("keenable", midnight.Add(c.at))
		if limit != c.want {
			t.Fatalf("at %v: limit %d, want %d", c.at, limit, c.want)
		}
		if c.want < 90 {
			// The next call opens when the accrual reaches limit+1.
			next, _ := r.limitAt("keenable", until)
			before, _ := r.limitAt("keenable", until.Add(-time.Second))
			if next != c.want+1 || before != c.want {
				t.Fatalf("at %v: until %v gives %d (a second before: %d)", c.at, until, next, before)
			}
		} else if !until.Equal(mustDay("2026-10-06")) {
			t.Fatalf("full cap until %v, want UTC midnight", until)
		}
	}
	// A provider without a quota is not paced or capped (8).
	if limit, _ := r.limitAt("brave", midnight); limit != NoCap {
		t.Fatalf("brave limit %d", limit)
	}
}

// countingKeenable answers 12 distinct results and counts its calls.
func countingKeenable() *stubProvider {
	return &stubProvider{name: "keenable", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		return manyResults("k", 12), nil
	}}
}

func TestMonthlyPace_RouterNeverPassesTheTargetAcrossRestarts(t *testing.T) { // 1, 2
	path := filepath.Join(t.TempDir(), "search-health.db")
	cfg := keenableQuotaConfig(1000) // target 900 over 31 days
	keenable := countingKeenable()
	total := 0
	for d := mustDay("2026-10-05"); d.Before(mustDay("2026-11-05")); d = d.AddDate(0, 0, 1) {
		// A restart every day: a fresh store handle and router.
		store, err := OpenFileHealthStore(path)
		if err != nil {
			t.Fatal(err)
		}
		router := NewRouter([]Provider{keenable}, store, cfg)
		late := d.Add(23*time.Hour + 30*time.Minute)
		router.now = func() time.Time { return late }
		before := keenable.searchCnt.Load()
		// Demand of 40 a day, above the ~29 a day pace, in concurrent bursts.
		var wg sync.WaitGroup
		for i := 0; i < 40; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s, err := router.Start(context.Background(), "q", 40)
				if err == nil {
					drain(t, s)
				}
			}()
		}
		wg.Wait()
		spent := int(keenable.searchCnt.Load() - before)
		pace, _ := router.monthlyPace("keenable", late)
		if spent > pace.Cap || pace.Used != total {
			t.Fatalf("%s: spent %d, pace %+v, counted %d", DayOf(d), spent, pace, total)
		}
		total += spent
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if total > 900 || total < 900-31 {
		t.Fatalf("period spent %d, want just under the 900 target", total)
	}
}

func TestMonthlyPace_RestartMidDayKeepsTheDaysCount(t *testing.T) { // 2
	path := filepath.Join(t.TempDir(), "search-health.db")
	cfg := keenableQuotaConfig(3100) // 90 a day
	keenable := countingKeenable()
	late := mustDay("2026-10-05").Add(23*time.Hour + 30*time.Minute)
	run := func(n int) {
		store, err := OpenFileHealthStore(path)
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		router := NewRouter([]Provider{keenable}, store, cfg)
		router.now = func() time.Time { return late }
		for i := 0; i < n; i++ {
			s, _ := router.Start(context.Background(), "q", 40)
			drain(t, s)
		}
	}
	run(60)
	run(60) // after a restart, only 30 more fit today
	if n := keenable.searchCnt.Load(); n != 90 {
		t.Fatalf("calls across a restart = %d, want the day's cap 90", n)
	}
}

func TestMonthlyPace_KeenableFirstThenEscalatesWhenCapped(t *testing.T) { // 7
	var l callLog
	keenable := logged(&l, "keenable", 0, 12)
	serper, tavily := logged(&l, "serper", 0, 12), logged(&l, "tavily", 0, 12)
	cfg := keenableQuotaConfig(3100) // 90 a day, 3 open at midnight
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{tavily, serper, keenable}, store, cfg)
	midnight := mustDay("2026-10-05")
	router.now = func() time.Time { return midnight }
	for i := 0; i < 3; i++ {
		s, _ := router.Start(context.Background(), "q", 40)
		if got := names(drain(t, s)); got != "keenable" || fmt.Sprint(s.Route.Summary().Reserve) != "[serper tavily]" {
			t.Fatalf("search %d with budget: arrivals %s reserve %v", i, got, s.Route.Summary().Reserve)
		}
	}
	if serper.searchCnt.Load()+tavily.searchCnt.Load() != 0 {
		t.Fatalf("paid called while keenable had budget: %s", l.String())
	}
	s, _ := router.Start(context.Background(), "q", 40)
	got := drain(t, s)
	if len(s.Cooling) != 1 || s.Cooling[0].Provider != "keenable" || s.Cooling[0].Reason != "capped" {
		t.Fatalf("capped keenable cooling = %+v", s.Cooling)
	}
	if want := midnight.Add(4 * time.Minute); !s.Cooling[0].Until.Equal(want) {
		t.Fatalf("capped until %v, want %v (next accrual)", s.Cooling[0].Until, want)
	}
	if names(got) != "serper" || reasons(s.Route.Summary()) != "no_free:serper" {
		t.Fatalf("capped keenable: arrivals %s escalations %s", names(got), reasons(s.Route.Summary()))
	}
	rec, _ := store.Get("keenable")
	if rec.StrikeCount != 0 || rec.State != StateActive {
		t.Fatalf("cap struck keenable: %+v", rec)
	}
	// The accrual reopens it.
	router.now = func() time.Time { return midnight.Add(4 * time.Minute) }
	s, _ = router.Start(context.Background(), "q", 40)
	if got := names(drain(t, s)); got != "keenable" {
		t.Fatalf("after accrual arrivals %s", got)
	}
}

func TestMonthlyPace_ConcurrentSearchesNeverPassTheCap(t *testing.T) { // 1
	keenable := countingKeenable()
	router := NewRouter([]Provider{keenable}, NewMemoryHealthStore(), keenableQuotaConfig(3100))
	late := mustDay("2026-10-05").Add(23*time.Hour + 30*time.Minute)
	router.now = func() time.Time { return late }
	var wg sync.WaitGroup
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := router.Start(context.Background(), "q", 40)
			drain(t, s)
		}()
	}
	wg.Wait()
	if n := keenable.searchCnt.Load(); n != 90 {
		t.Fatalf("calls = %d, want the day's cap 90", n)
	}
}

func TestKeenableRateLimit_EightPerSecondAcrossConcurrentSearches(t *testing.T) { // 6
	t.Setenv("CHOIR_SEARCH_PROVIDER_RPS", "")
	env := ConfigFromEnv()
	cfg := budgetConfig()
	cfg.MinInterval = env.MinInterval
	cfg.PaceWait = 5 * time.Second
	var mu sync.Mutex
	var starts []time.Time
	keenable := &stubProvider{name: "keenable", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		return manyResults("k", 12), nil
	}}
	router := NewRouter([]Provider{keenable}, NewMemoryHealthStore(), cfg)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := router.Start(context.Background(), "q", 40)
			drain(t, s)
		}()
	}
	wg.Wait()
	sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
	if len(starts) != 24 {
		t.Fatalf("calls = %d", len(starts))
	}
	for i := 0; i+8 < len(starts); i++ {
		// Nine calls span at least a second, less timer jitter: never more
		// than 8 in a second, far under Keenable's 10.
		if gap := starts[i+8].Sub(starts[i]); gap < 950*time.Millisecond {
			t.Fatalf("calls %d..%d within %v", i, i+8, gap)
		}
	}
}

func TestConfigFromEnv_KeenableMonthlyAndRate(t *testing.T) { // 9
	for _, name := range []string{"CHOIR_SEARCH_PROVIDER_RPS", "CHOIR_SEARCH_KEENABLE_MONTHLY_ALLOWANCE", "CHOIR_SEARCH_KEENABLE_RENEW_DAY", "CHOIR_SEARCH_KEENABLE_MONTHLY_TARGET_PCT", "CHOIR_SEARCH_KEENABLE_PERIOD_USED"} {
		t.Setenv(name, "")
	}
	cfg := ConfigFromEnv()
	q := cfg.MonthlyQuotas["keenable"]
	if q.Allowance != 100000 || q.RenewDay != 5 || q.TargetPct != 90 || q.SeedDay != "" || q.Target() != 90000 {
		t.Fatalf("defaults = %+v", q)
	}
	if cfg.MinInterval["keenable"] != 125*time.Millisecond || cfg.MinInterval["brave"] != time.Second {
		t.Fatalf("default pacing = %v", cfg.MinInterval)
	}
	if _, ok := cfg.DailyCaps["keenable"]; ok {
		t.Fatal("keenable has a fixed daily cap; its cap is the monthly pace")
	}

	t.Setenv("CHOIR_SEARCH_PROVIDER_RPS", "brave=2")
	if cfg := ConfigFromEnv(); cfg.MinInterval["keenable"] != 125*time.Millisecond || cfg.MinInterval["brave"] != 500*time.Millisecond {
		t.Fatalf("override dropped keenable pacing: %v", cfg.MinInterval)
	}
	t.Setenv("CHOIR_SEARCH_PROVIDER_RPS", "keenable=0")
	if cfg := ConfigFromEnv(); cfg.MinInterval["brave"] != time.Second {
		t.Fatalf("keenable=0: %v", cfg.MinInterval)
	} else if _, ok := cfg.MinInterval["keenable"]; ok {
		t.Fatalf("keenable=0 kept pacing: %v", cfg.MinInterval)
	}

	t.Setenv("CHOIR_SEARCH_KEENABLE_MONTHLY_ALLOWANCE", "50000")
	t.Setenv("CHOIR_SEARCH_KEENABLE_RENEW_DAY", "40")
	t.Setenv("CHOIR_SEARCH_KEENABLE_MONTHLY_TARGET_PCT", "80")
	t.Setenv("CHOIR_SEARCH_KEENABLE_PERIOD_USED", "10776@2026-10-15")
	q = ConfigFromEnv().MonthlyQuotas["keenable"]
	if q.Allowance != 50000 || q.RenewDay != 5 || q.TargetPct != 80 || q.SeedUsed != 10776 || q.SeedDay != "2026-10-15" {
		t.Fatalf("overrides = %+v", q)
	}
	t.Setenv("CHOIR_SEARCH_KEENABLE_MONTHLY_ALLOWANCE", "0")
	if got := ConfigFromEnv().MonthlyQuotas; len(got) != 0 {
		t.Fatalf("allowance 0 kept a quota: %v", got)
	}
}

func TestFailureFields_SayWhyWithoutBodyOrKey(t *testing.T) { // 10
	cases := []struct {
		err           error
		status, stage string
	}{
		{errors.New(`status 401 Unauthorized: {"error":"invalid api key sk-SECRET123456"}`), "401", "http_status"},
		{errors.New(`status 402 Payment Required: Insufficient credit`), "402", "http_status"},
		{errors.New("parse response: unexpected end of JSON input"), "-", "parse_response"},
		{fmt.Errorf("http request: %w", context.DeadlineExceeded), "-", "timeout"},
		{errors.New(`http request: Post "https://api.example/search?api_key=SECRET": dial tcp: refused`), "-", "http_request"},
		{errors.New("status error: Your account has run out of searches"), "-", "status_error"},
		{errors.New("SECRET weird failure"), "-", "other"},
	}
	for _, c := range cases {
		status, stage := FailureFields(c.err)
		if status != c.status || stage != c.stage {
			t.Errorf("%q: status=%s stage=%s, want %s %s", c.err, status, stage, c.status, c.stage)
		}
	}

	// Through the router: the log line says provider, class, status and
	// stage, and nothing from the body or the query.
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	exa := &stubProvider{name: "exa", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		return nil, errors.New(`status 400 Bad Request: {"error":"key=SECRET bad numResults for query QUERYTEXT"}`)
	}}
	router := NewRouter([]Provider{exa}, NewMemoryHealthStore(), Config{RequestTimeout: time.Second})
	s, _ := router.Start(context.Background(), "QUERYTEXT", 40)
	drain(t, s)
	line := buf.String()
	if !strings.Contains(line, "searchplane: exa call failed class=error http_status=400 stage=http_status") {
		t.Fatalf("log = %q", line)
	}
	if strings.Contains(line, "SECRET") || strings.Contains(line, "QUERYTEXT") {
		t.Fatalf("log leaks: %q", line)
	}
}
