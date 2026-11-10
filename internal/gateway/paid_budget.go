package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Per-provider daily dollar spend, and the owner's daily budgets for the paid
// providers.
//
// Owner, 2026-11-09: "We should be monitoring the spend ... for everything,
// because with synthetic, we have limited usage. With Bedrock, we have like
// $200 of credits, and I don't want to spend them all quickly ... We should
// definitely last through Thursday ... And for River, we only have like $20
// or something tokens. So I think our self-dev work should really be no more
// than like $1 per day ... If we can keep the paper to $20 per day, that would
// be pretty cool."
//
// Every provider call is counted (cost = llmcost price x usage), per UTC day,
// per provider and per computer. Bedrock and River also have a platform-wide
// daily dollar budget; Synthetic and the free providers are counted and shown
// and never capped (Synthetic is a usage-limited subscription whose own 429s
// already drive the availability memory). The budgets are the owner's spend
// grant, like the Claude delegate's daily token cap, not a feature switch.
//
// Ways this could fail, written down before the code:
//   - cost is computed wrong (cached tokens at the full rate, or ignored),
//     so a budget trips late or early;
//   - the day does not reset at UTC midnight, or resets twice, so a budget
//     stays shut all week or a restart forgets the morning's spend;
//   - a gateway restart forgets the day's spend (no persistence), so the
//     budget re-arms to zero at every deploy;
//   - the refusal is an untyped error, so the pool treats a spent budget as an
//     outage and retries it every beat instead of holding the member until
//     the reset;
//   - a free or Synthetic provider is refused;
//   - a model with no price counts as $0 on a paid provider and escapes the
//     budget;
//   - the log line prints per call, not once per refusal kind per day;
//   - the snapshot hides which computer spent.
const (
	// PaidBudgetBedrockEnv and PaidBudgetRiverEnv override the daily dollar
	// budgets; PaidSpendFileEnv names the file that holds the day's spend
	// across gateway restarts.
	PaidBudgetBedrockEnv = "GATEWAY_PAID_BUDGET_BEDROCK_USD"
	PaidBudgetRiverEnv   = "GATEWAY_PAID_BUDGET_RIVER_USD"
	PaidSpendFileEnv     = "GATEWAY_PAID_SPEND_FILE"

	// defaultBedrockDailyUSD: about $200 of credits that must last through
	// Thursday and keep the paper running through the owner's meetings.
	// Owner, 2026-11-10: "we can increase our haiku limit to $40/day", while
	// the Devin free allowance runs out minutes into each hour.
	defaultBedrockDailyUSD = 40.0
	// defaultRiverDailyUSD: owner 2026-11-10: "$1/day cap can be lifted. Yes,
	// this would use river and synthetic faster than ideal, but it's only for
	// swe-2-flash's downtime." The orchestrator's conservative reading is $10
	// a day (was $1: self-development only; River now also takes about a tenth
	// of the default pool's tail, modelcatalog.Pools).
	defaultRiverDailyUSD = 10.0

	paidSpendSummaryEvery  = time.Hour
	paidSpendUnknownCaller = "(unknown)"
)

// unpricedPaidRate prices a call on a capped provider whose model has no
// table entry, per million tokens, so it counts against the budget instead
// of escaping it at $0. It is the dearest Bedrock rate (an estimate).
var unpricedPaidRate = struct{ in, out float64 }{5.0, 25.0}

// providerDay is one provider's totals for one UTC day.
type providerDay struct {
	USD          float64            `json:"usd"`
	InputTokens  int64              `json:"input_tokens"`
	CachedTokens int64              `json:"cached_input_tokens"`
	OutputTokens int64              `json:"output_tokens"`
	Calls        int64              `json:"calls"`
	Unpriced     int64              `json:"unpriced_calls,omitempty"`
	Computers    map[string]float64 `json:"computers_usd,omitempty"`
}

type paidSpendRecord struct {
	Day     string                  `json:"day"`
	Today   map[string]*providerDay `json:"today"`
	PrevDay string                  `json:"prev_day,omitempty"`
	Prev    map[string]*providerDay `json:"prev,omitempty"`
}

// PaidSpend is the gateway's spend ledger.
type PaidSpend struct {
	mu      sync.Mutex
	now     func() time.Time
	path    string
	budgets map[string]float64
	rec     paidSpendRecord
	logged  map[string]string // "provider:kind" -> day it was logged
}

// NewPaidSpend builds a ledger with the given budgets (provider -> USD/day;
// absent or <= 0 means no cap) and persistence path ("" keeps it in memory).
// A readable spend file is loaded.
func NewPaidSpend(path string, budgets map[string]float64, now func() time.Time) *PaidSpend {
	if now == nil {
		now = time.Now
	}
	s := &PaidSpend{now: now, path: path, budgets: budgets, logged: map[string]string{}}
	s.rec = paidSpendRecord{Day: s.day(), Today: map[string]*providerDay{}}
	if path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			var rec paidSpendRecord
			if json.Unmarshal(raw, &rec) == nil && rec.Day != "" {
				if rec.Today == nil {
					rec.Today = map[string]*providerDay{}
				}
				s.rec = rec
			} else {
				log.Printf("gateway: paid spend file %s is unreadable; today's spend restarts at $0", path)
			}
		}
	}
	return s
}

// NewPaidSpendFromEnv reads the budgets and the spend file from the
// environment.
func NewPaidSpendFromEnv() *PaidSpend {
	return newPaidSpendFromGetenv(os.Getenv)
}

func newPaidSpendFromGetenv(get func(string) string) *PaidSpend {
	budgets := map[string]float64{"bedrock": defaultBedrockDailyUSD, "river": defaultRiverDailyUSD}
	for name, env := range map[string]string{"bedrock": PaidBudgetBedrockEnv, "river": PaidBudgetRiverEnv} {
		// An unset, unreadable, non-positive or infinite value is the default,
		// never none.
		if v, err := strconv.ParseFloat(strings.TrimSpace(get(env)), 64); err == nil && v > 0 && !math.IsInf(v, 0) {
			budgets[name] = v
		}
	}
	return NewPaidSpend(strings.TrimSpace(get(PaidSpendFileEnv)), budgets, nil)
}

func (s *PaidSpend) day() string { return s.now().UTC().Format("2006-01-02") }

// rollLocked moves a finished day into prev. The caller holds s.mu.
func (s *PaidSpend) rollLocked() {
	if d := s.day(); s.rec.Day != d {
		s.rec.PrevDay, s.rec.Prev = s.rec.Day, s.rec.Today
		s.rec.Day, s.rec.Today = d, map[string]*providerDay{}
	}
}

func nextUTCMidnight(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)
}

// paidCallCost prices a call. priced=false says the model had no table entry
// (the call then counts at the unpriced rate on a capped provider, else $0).
func paidCallCost(providerName, model string, u provider.Usage, capped bool) (usd float64, priced bool) {
	for _, prefix := range []string{"global.anthropic.", "us.anthropic.", "eu.anthropic.", "anthropic."} {
		model = strings.TrimPrefix(model, prefix)
	}
	c := llmcost.EstimateProviderCallWithCacheWrites(providerName, model, u.InputTokens, u.CachedInputTokens, u.CacheWrite5mTokens, u.CacheWrite1hTokens, u.OutputTokens)
	if c.Found {
		return c.USD, true
	}
	if capped {
		return float64(u.InputTokens)*unpricedPaidRate.in/1e6 + float64(u.OutputTokens)*unpricedPaidRate.out/1e6, false
	}
	return 0, false
}

// Record counts one finished call and returns its cost.
func (s *PaidSpend) Record(providerName, computerID, model string, u provider.Usage) float64 {
	if s == nil {
		return 0
	}
	if computerID == "" {
		computerID = paidSpendUnknownCaller
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rollLocked()
	_, capped := s.budgets[providerName]
	usd, priced := paidCallCost(providerName, model, u, capped)
	d := s.rec.Today[providerName]
	if d == nil {
		d = &providerDay{}
		s.rec.Today[providerName] = d
	}
	if d.Computers == nil {
		d.Computers = map[string]float64{}
	}
	d.USD += usd
	d.InputTokens += int64(u.InputTokens)
	d.CachedTokens += int64(u.CachedInputTokens)
	d.OutputTokens += int64(u.OutputTokens)
	d.Calls++
	if !priced {
		d.Unpriced++
	}
	d.Computers[computerID] += usd
	s.saveLocked()
	return usd
}

// saveLocked writes the record by atomic rename; a failed write is logged and
// the in-memory count stands.
func (s *PaidSpend) saveLocked() {
	if s.path == "" {
		return
	}
	raw, err := json.Marshal(s.rec)
	if err == nil {
		tmp := s.path + ".tmp"
		if err = os.WriteFile(tmp, raw, 0o600); err == nil {
			err = os.Rename(tmp, s.path)
		}
	}
	if err != nil {
		log.Printf("gateway: paid spend file not written: %v", err)
	}
}

// Refusal is the typed quota error for a provider whose day budget is spent,
// or nil. The reset is the next UTC midnight, so the computer's availability
// memory holds the member until then and the pool falls through. It logs one
// line per provider per day.
func (s *PaidSpend) Refusal(providerName, model string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rollLocked()
	limit := s.budgets[providerName]
	if limit <= 0 {
		return nil
	}
	d := s.rec.Today[providerName]
	if d == nil || d.USD < limit {
		return nil
	}
	if key := providerName + ":budget"; s.logged[key] != s.rec.Day {
		s.logged[key] = s.rec.Day
		log.Printf("gateway: paid budget spent: %s $%.2f of $%.2f for %s; refusing its calls until the next UTC midnight",
			providerName, d.USD, limit, s.rec.Day)
	}
	return &provider.QuotaExhaustedError{
		Provider:    providerName,
		Model:       model,
		RetryAt:     nextUTCMidnight(s.now()),
		CircuitOpen: true,
	}
}

// PaidSpendProvider is one provider's block of the snapshot.
type PaidSpendProvider struct {
	USD          float64            `json:"usd"`
	InputTokens  int64              `json:"input_tokens"`
	CachedTokens int64              `json:"cached_input_tokens"`
	OutputTokens int64              `json:"output_tokens"`
	Calls        int64              `json:"calls"`
	Unpriced     int64              `json:"unpriced_calls,omitempty"`
	BudgetUSD    float64            `json:"budget_usd,omitempty"`
	Computers    map[string]float64 `json:"computers_usd,omitempty"`
}

// PaidSpendSnapshot is the paid_spend block.
type PaidSpendSnapshot struct {
	Day         string                       `json:"day"`
	Providers   map[string]PaidSpendProvider `json:"providers"`
	PreviousDay string                       `json:"previous_day,omitempty"`
	Previous    map[string]PaidSpendProvider `json:"previous,omitempty"`
	ResetsAt    string                       `json:"resets_at"`
}

func (s *PaidSpend) blockLocked(m map[string]*providerDay, withEmptyCapped bool) map[string]PaidSpendProvider {
	out := map[string]PaidSpendProvider{}
	if withEmptyCapped { // a capped provider shows even at $0
		for name, limit := range s.budgets {
			out[name] = PaidSpendProvider{BudgetUSD: limit}
		}
	}
	for name, d := range m {
		p := PaidSpendProvider{USD: d.USD, InputTokens: d.InputTokens, CachedTokens: d.CachedTokens,
			OutputTokens: d.OutputTokens, Calls: d.Calls, Unpriced: d.Unpriced, BudgetUSD: s.budgets[name]}
		if len(d.Computers) > 0 {
			p.Computers = make(map[string]float64, len(d.Computers))
			for c, v := range d.Computers {
				p.Computers[c] = v
			}
		}
		out[name] = p
	}
	return out
}

// Snapshot is today's and the previous day's totals.
func (s *PaidSpend) Snapshot() PaidSpendSnapshot {
	if s == nil {
		return PaidSpendSnapshot{Providers: map[string]PaidSpendProvider{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rollLocked()
	snap := PaidSpendSnapshot{Day: s.rec.Day, Providers: s.blockLocked(s.rec.Today, true),
		ResetsAt: nextUTCMidnight(s.now()).Format(time.RFC3339)}
	if s.rec.PrevDay != "" {
		snap.PreviousDay, snap.Previous = s.rec.PrevDay, s.blockLocked(s.rec.Prev, false)
	}
	return snap
}

// SummaryLine is the hourly journal line.
func (s *PaidSpend) SummaryLine() string {
	snap := s.Snapshot()
	names := make([]string, 0, len(snap.Providers))
	for n := range snap.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "gateway: spend today %s", snap.Day)
	for _, n := range names {
		p := snap.Providers[n]
		if p.BudgetUSD > 0 {
			fmt.Fprintf(&b, " %s $%.2f/%g", n, p.USD, p.BudgetUSD)
		} else {
			fmt.Fprintf(&b, " %s $%.2f (est)", n, p.USD)
		}
	}
	return b.String()
}

// RunSummary logs SummaryLine every paidSpendSummaryEvery until ctx ends.
func (s *PaidSpend) RunSummary(ctx context.Context) {
	if s == nil {
		return
	}
	t := time.NewTicker(paidSpendSummaryEvery)
	defer t.Stop()
	log.Print(s.SummaryLine())
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			log.Print(s.SummaryLine())
		}
	}
}

// paidSpendProvider wraps a provider: it refuses a call whose provider's day
// budget is spent, and counts every finished call. The computer is the one
// the handler named on the context (provider.WithImageDecodeCaller).
type paidSpendProvider struct {
	inner provider.Provider
	spend *PaidSpend
}

func (p *paidSpendProvider) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	if err := p.spend.Refusal(p.inner.Name(), req.Model); err != nil {
		return nil, err
	}
	resp, err := p.inner.Call(ctx, req)
	p.count(ctx, req, resp, err)
	return resp, err
}

func (p *paidSpendProvider) Stream(ctx context.Context, req provider.LLMRequest, onChunk func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	if err := p.spend.Refusal(p.inner.Name(), req.Model); err != nil {
		return nil, err
	}
	resp, err := p.inner.Stream(ctx, req, onChunk)
	p.count(ctx, req, resp, err)
	return resp, err
}

func (p *paidSpendProvider) count(ctx context.Context, req provider.LLMRequest, resp *provider.LLMResponse, err error) {
	if err != nil || resp == nil {
		return
	}
	model := resp.Model
	if model == "" {
		model = req.Model
	}
	p.spend.Record(p.inner.Name(), provider.ImageDecodeCaller(ctx), model, resp.Usage)
}

func (p *paidSpendProvider) Name() string { return p.inner.Name() }
func (p *paidSpendProvider) IsReal() bool { return p.inner.IsReal() }

// SetPaidSpend attaches the ledger and wraps every provider in it. Call it
// after the circuit breakers are registered.
func (h *Handler) SetPaidSpend(s *PaidSpend) {
	h.paidSpend = s
	if s == nil {
		return
	}
	if h.providers != nil {
		out := provider.NewMultiProvider()
		for _, name := range h.providers.Names() {
			if p := h.providers.Get(name); p != nil {
				out.Register(name, &paidSpendProvider{inner: p, spend: s})
			}
		}
		h.providers = out
	}
	if h.provider != nil {
		h.provider = &paidSpendProvider{inner: h.provider, spend: s}
	}
}

// HandlePaidSpend handles GET /provider/v1/spend, an operator route.
func (h *Handler) HandlePaidSpend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	if !isAuthorizedCredentialCaller(r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "operator route"})
		return
	}
	writeGatewayJSON(w, http.StatusOK, map[string]any{"paid_spend": h.paidSpend.Snapshot()})
}
