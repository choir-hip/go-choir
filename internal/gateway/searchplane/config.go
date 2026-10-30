package searchplane

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config controls the provider calls of one search.
type Config struct {
	// RequestTimeout bounds each provider call.
	RequestTimeout time.Duration
	EndpointFor    func(provider string) string
	// MinInterval paces calls to a provider: at most one call per interval,
	// across every computer. A per-second plan limit is a fact about our
	// traffic, not the provider's health, so it is met by pacing, not by a
	// bench (docs/archive/problems/search-providers-cooling-most-of-the-time-2026-10-14.md).
	MinInterval map[string]time.Duration
	// PaceWait is how long a call may wait for its slot; a provider whose
	// slot is further away is skipped for that search as paced.
	PaceWait time.Duration
	// CacheTTL > 0 turns on the result cache and in-flight join (cache.go);
	// CacheFreshTTL is the TTL for time-sensitive queries, CacheMaxBytes
	// bounds the result text held, and CacheShared lets one computer's
	// search answer another's (default: per computer).
	CacheTTL      time.Duration
	CacheFreshTTL time.Duration
	CacheMaxBytes int64
	CacheShared   bool

	// Routing is RoutingBudget (free providers first, paid ones only on
	// shortfall) or RoutingFanout (every eligible provider at once). The
	// zero value is fan-out, the path before budget routing; ConfigFromEnv
	// defaults to budget
	// (docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md §4.1).
	Routing string
	// Tiers gives each provider a tier and a relative cost; nil means
	// DefaultProviderTiers. A provider not named is paid at
	// UnknownProviderCost, so an unknown provider never spends first.
	Tiers map[string]ProviderTier
	// DailyCaps caps each named provider's calls per UTC day; a provider
	// not named has no cap. Calls are counted whether or not capped.
	DailyCaps map[string]int
	// MonthlyQuotas paces a provider's monthly free allowance: each UTC
	// day's cap is the period's remaining target over the days left
	// (monthly.go), lowered further by any DailyCaps entry.
	MonthlyQuotas map[string]MonthlyQuota
	// MinResults is how many distinct URLs a search wants before it stops
	// escalating (bounded by its max_results); DeepMinResults for a deep
	// search.
	MinResults     int
	DeepMinResults int
	// MaxPaid bounds the paid providers one search may add; DeepMaxPaid
	// for a deep search.
	MaxPaid     int
	DeepMaxPaid int
	// EscalateAfter is a step's soft deadline: when the calls of the
	// current step are still out after it and the search is short, the
	// next paid provider joins.
	EscalateAfter time.Duration
	// EscalateWindow: no paid provider starts later than this after the
	// search began. The stream bound grows by it (EscalationWindow).
	EscalateWindow time.Duration
}

// Routing modes.
const (
	RoutingFanout = "fanout"
	RoutingBudget = "budget"
)

// Tier is a provider's cost class.
type Tier string

const (
	// TierFree providers are always started.
	TierFree Tier = "free"
	// TierPaid providers are metered; they join cheapest first on shortfall.
	TierPaid Tier = "paid"
	// TierConstrained providers share a limit with something more important
	// (chatgpt: the ChatGPT/Codex usage limit of inference); they join last.
	TierConstrained Tier = "constrained"
)

// ProviderTier is a provider's tier and relative cost (list price in USD
// per 1,000 calls, as last seen; only the order matters).
type ProviderTier struct {
	Tier Tier
	Cost float64
}

// rank orders tiers for escalation: free, paid, constrained.
func (t ProviderTier) rank() int {
	switch t.Tier {
	case TierFree:
		return 0
	case TierConstrained:
		return 2
	default:
		return 1
	}
}

// UnknownProviderCost places a provider without a configured tier after
// every priced paid provider.
const UnknownProviderCost = 1000

// Budget routing defaults (docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md
// §4.1). Costs are list prices per 1,000 calls as last seen (verify on the
// billing pages); caps bound each paid provider to a few dollars a day.
// Brave is free on its Free AI plan within its pacing (1 request/s); it has
// no cap: on that plan a spent month is quota_limited, not billed.
const (
	DefaultProviderTiers  = "keenable=free,searxng=free,brave=free,serper=paid:1,parallel=paid:5,exa=paid:6,tavily=paid:8,serpapi=paid:15,chatgpt=constrained"
	DefaultDailyCaps      = "serper=150,parallel=150,exa=150,tavily=150,serpapi=100,chatgpt=50"
	DefaultMinResults     = 10
	DefaultDeepMinResults = 30
	DefaultMaxPaid        = 2
	DefaultDeepMaxPaid    = 4
	DefaultEscalateAfter  = 8 * time.Second
	DefaultEscalateWindow = 20 * time.Second
)

// EscalationWindow is how much longer than a fan-out search a search under
// cfg may run: the escalation window under budget routing, zero otherwise.
func EscalationWindow(cfg Config) time.Duration {
	if cfg.Routing != RoutingBudget {
		return 0
	}
	if cfg.EscalateWindow <= 0 {
		return DefaultEscalateWindow
	}
	return cfg.EscalateWindow
}

// DefaultProviderRPS paces Brave to its Free AI plan's one request a second
// and Keenable to 8 a second, under its plan's 10
// (docs/archive/problems/search-throughput-keenable-underused-2026-10-15.md).
const DefaultProviderRPS = "brave=1,keenable=8"

// DefaultPaceWait is the default PaceWait.
const DefaultPaceWait = 3 * time.Second

// Result cache defaults (docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md
// §1): 177 of 300 measured repeats came within 10 minutes, 255 within an
// hour.
const (
	DefaultCacheTTL      = 30 * time.Minute
	DefaultCacheFreshTTL = 10 * time.Minute
	DefaultCacheMaxMB    = 64
)

// ConfigFromEnv loads router settings from environment variables.
// CHOIR_SEARCH_PROVIDER_RPS ("brave=1,tavily=5"; rate 0 removes that
// provider's pacing) is merged over DefaultProviderRPS, so setting one
// provider's rate never drops another's. CHOIR_SEARCH_CACHE_TTL_SECONDS=0 turns the
// result cache and in-flight join off; CHOIR_SEARCH_CACHE_SCOPE=shared
// shares answers across computers.
//
// Budget routing: CHOIR_SEARCH_ROUTING=fanout restores fan-out to every
// eligible provider (default budget). CHOIR_SEARCH_PROVIDER_TIERS
// ("tavily=paid:8,brave=free,chatgpt=constrained") and
// CHOIR_SEARCH_PROVIDER_DAILY_CAPS ("tavily=500,exa=none"; "none" alone
// removes every cap) are merged over the defaults. CHOIR_SEARCH_MIN_RESULTS,
// CHOIR_SEARCH_DEEP_MIN_RESULTS, CHOIR_SEARCH_MAX_PAID,
// CHOIR_SEARCH_DEEP_MAX_PAID, CHOIR_SEARCH_ESCALATE_AFTER_MS and
// CHOIR_SEARCH_ESCALATE_WINDOW_MS tune escalation.
//
// Keenable's monthly pace: CHOIR_SEARCH_KEENABLE_MONTHLY_ALLOWANCE (free
// requests a period, default 100000; 0 turns the pace off),
// CHOIR_SEARCH_KEENABLE_RENEW_DAY (day of month the period starts, UTC,
// default 5), CHOIR_SEARCH_KEENABLE_MONTHLY_TARGET_PCT (share of the
// allowance to spend, default 90) and CHOIR_SEARCH_KEENABLE_PERIOD_USED
// ("N@YYYY-MM-DD": N calls made earlier in that day's period that the
// daily counts never saw, from the billing page).
func ConfigFromEnv() Config {
	routing := RoutingBudget
	if strings.EqualFold(strings.TrimSpace(os.Getenv("CHOIR_SEARCH_ROUTING")), RoutingFanout) {
		routing = RoutingFanout
	}
	return Config{
		Routing:        routing,
		Tiers:          ParseProviderTiers(DefaultProviderTiers + "," + os.Getenv("CHOIR_SEARCH_PROVIDER_TIERS")),
		DailyCaps:      ParseDailyCaps(DefaultDailyCaps, os.Getenv("CHOIR_SEARCH_PROVIDER_DAILY_CAPS")),
		MinResults:     envInt("CHOIR_SEARCH_MIN_RESULTS", DefaultMinResults),
		DeepMinResults: envInt("CHOIR_SEARCH_DEEP_MIN_RESULTS", DefaultDeepMinResults),
		MaxPaid:        envInt("CHOIR_SEARCH_MAX_PAID", DefaultMaxPaid),
		DeepMaxPaid:    envInt("CHOIR_SEARCH_DEEP_MAX_PAID", DefaultDeepMaxPaid),
		EscalateAfter:  time.Duration(envInt("CHOIR_SEARCH_ESCALATE_AFTER_MS", int(DefaultEscalateAfter/time.Millisecond))) * time.Millisecond,
		EscalateWindow: time.Duration(envInt("CHOIR_SEARCH_ESCALATE_WINDOW_MS", int(DefaultEscalateWindow/time.Millisecond))) * time.Millisecond,
		RequestTimeout: time.Duration(envInt("CHOIR_SEARCH_REQUEST_TIMEOUT_SECONDS", 30)) * time.Second,
		MinInterval:    providerRPSFromEnv(),
		MonthlyQuotas:  monthlyQuotasFromEnv(),
		PaceWait:       time.Duration(envInt("CHOIR_SEARCH_PACE_WAIT_MS", int(DefaultPaceWait/time.Millisecond))) * time.Millisecond,
		CacheTTL:       time.Duration(envInt("CHOIR_SEARCH_CACHE_TTL_SECONDS", int(DefaultCacheTTL/time.Second))) * time.Second,
		CacheFreshTTL:  time.Duration(envInt("CHOIR_SEARCH_CACHE_FRESH_TTL_SECONDS", int(DefaultCacheFreshTTL/time.Second))) * time.Second,
		CacheMaxBytes:  int64(envInt("CHOIR_SEARCH_CACHE_MAX_MB", DefaultCacheMaxMB)) << 20,
		CacheShared:    strings.EqualFold(strings.TrimSpace(os.Getenv("CHOIR_SEARCH_CACHE_SCOPE")), "shared"),
	}
}

// providerRPSFromEnv merges CHOIR_SEARCH_PROVIDER_RPS over
// DefaultProviderRPS; an entry with rate <= 0 removes that provider's pacing.
func providerRPSFromEnv() map[string]time.Duration {
	out := parseProviderRPS(DefaultProviderRPS)
	for _, part := range strings.Split(os.Getenv("CHOIR_SEARCH_PROVIDER_RPS"), ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		rate, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			continue
		}
		if rate <= 0 {
			delete(out, name)
			continue
		}
		out[name] = time.Duration(float64(time.Second) / rate)
	}
	return out
}

// monthlyQuotasFromEnv is Keenable's monthly quota from the environment
// (ConfigFromEnv); none when the allowance is 0.
func monthlyQuotasFromEnv() map[string]MonthlyQuota {
	q := MonthlyQuota{
		Allowance: envInt("CHOIR_SEARCH_KEENABLE_MONTHLY_ALLOWANCE", DefaultKeenableMonthlyAllowance),
		RenewDay:  envInt("CHOIR_SEARCH_KEENABLE_RENEW_DAY", DefaultKeenableRenewDay),
		TargetPct: envInt("CHOIR_SEARCH_KEENABLE_MONTHLY_TARGET_PCT", DefaultKeenableTargetPct),
	}
	if q.Allowance <= 0 {
		return map[string]MonthlyQuota{}
	}
	if q.RenewDay < 1 || q.RenewDay > 31 {
		q.RenewDay = DefaultKeenableRenewDay
	}
	if q.TargetPct < 1 || q.TargetPct > 100 {
		q.TargetPct = DefaultKeenableTargetPct
	}
	if used, day, ok := ParseQuotaSeed(os.Getenv("CHOIR_SEARCH_KEENABLE_PERIOD_USED")); ok {
		q.SeedUsed, q.SeedDay = used, day
	}
	return map[string]MonthlyQuota{"keenable": q}
}

// parseProviderRPS reads "name=rate,..." into per-provider call intervals.
// Malformed entries and rates <= 0 are left out (no pacing).
func parseProviderRPS(raw string) map[string]time.Duration {
	out := map[string]time.Duration{}
	for _, part := range strings.Split(raw, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		rate, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || rate <= 0 {
			continue
		}
		out[strings.TrimSpace(name)] = time.Duration(float64(time.Second) / rate)
	}
	return out
}

// ParseProviderTiers reads "name=free,name=paid:8,name=constrained"; a later
// entry for a name replaces an earlier one. A paid or constrained entry
// without a cost is priced at UnknownProviderCost; malformed entries are
// left out.
func ParseProviderTiers(raw string) map[string]ProviderTier {
	out := map[string]ProviderTier{}
	for _, part := range strings.Split(raw, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		tierName, costRaw, hasCost := strings.Cut(strings.TrimSpace(value), ":")
		tier := ProviderTier{Tier: Tier(strings.ToLower(strings.TrimSpace(tierName)))}
		switch tier.Tier {
		case TierFree, TierPaid, TierConstrained:
		default:
			continue
		}
		if tier.Tier != TierFree {
			tier.Cost = UnknownProviderCost
		}
		if hasCost {
			cost, err := strconv.ParseFloat(strings.TrimSpace(costRaw), 64)
			if err != nil || cost < 0 {
				continue
			}
			tier.Cost = cost
		}
		out[name] = tier
	}
	return out
}

// ParseDailyCaps merges override ("name=N,name=none") over defaults. An
// override of exactly "none" removes every cap; "name=none" removes that
// provider's cap; N >= 0 sets it (0: never call). Malformed entries are
// left out.
func ParseDailyCaps(defaults, override string) map[string]int {
	out := map[string]int{}
	if strings.EqualFold(strings.TrimSpace(override), "none") {
		return out
	}
	for _, raw := range []string{defaults, override} {
		for _, part := range strings.Split(raw, ",") {
			name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
			name = strings.TrimSpace(name)
			value = strings.TrimSpace(value)
			if !ok || name == "" {
				continue
			}
			if strings.EqualFold(value, "none") {
				delete(out, name)
				continue
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				continue
			}
			out[name] = n
		}
	}
	return out
}

// FormatDailyCaps renders caps as "name=N,..." in name order (logs).
func FormatDailyCaps(caps map[string]int) string {
	names := make([]string, 0, len(caps))
	for name := range caps {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = name + "=" + strconv.Itoa(caps[name])
	}
	return strings.Join(parts, ",")
}

// FormatProviderRPS renders pacing intervals as "name=rate,..." in name
// order (logs).
func FormatProviderRPS(intervals map[string]time.Duration) string {
	names := make([]string, 0, len(intervals))
	for name := range intervals {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		if d := intervals[name]; d > 0 {
			parts = append(parts, name+"="+strconv.FormatFloat(float64(time.Second)/float64(d), 'g', 4, 64))
		}
	}
	return strings.Join(parts, ",")
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
