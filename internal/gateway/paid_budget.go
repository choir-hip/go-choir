package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	// delta is what this process has recorded since its last save that
	// landed, by UTC day then provider. A save adds it to the file under a
	// lock, so a second gateway process (a deploy's handoff) never has its
	// calls overwritten. It is empty without a path.
	delta map[string]map[string]*providerDay
	saver spendSaver
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
		rec, exists, err := readPaidSpendFile(path)
		if errors.Is(err, errSpendFileCorrupt) {
			// Set the bad file aside (bytes kept) so this start and every
			// later one is not re-armed at $0 by it; the next save writes a
			// fresh file. If another process mended it first, use that.
			s.saver.setAsideIfCorrupt(path, func() error { _, _, e := readPaidSpendFile(path); return e })
			rec, exists, err = readPaidSpendFile(path)
		}
		switch {
		case err != nil:
			log.Printf("gateway: paid spend file %s is unreadable (%v); today's spend restarts at $0 and the file is not overwritten", path, err)
		case exists:
			s.rec = rec
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
	// Only forward: a file or clock that is behind never rolls today's spend
	// into prev (days are ISO dates, so string order is date order).
	if d := s.day(); d > s.rec.Day {
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
	call := providerDay{USD: usd, InputTokens: int64(u.InputTokens), CachedTokens: int64(u.CachedInputTokens),
		OutputTokens: int64(u.OutputTokens), Calls: 1, Computers: map[string]float64{computerID: usd}}
	if !priced {
		call.Unpriced = 1
	}
	addProviderDay(s.rec.Today, providerName, &call)
	if s.path != "" {
		if s.delta == nil {
			s.delta = map[string]map[string]*providerDay{}
		}
		if s.delta[s.rec.Day] == nil {
			s.delta[s.rec.Day] = map[string]*providerDay{}
		}
		addProviderDay(s.delta[s.rec.Day], providerName, &call)
	}
	if s.path != "" {
		defer s.saver.kickSave(s.saveOnce)
	}
	return usd
}

// addProviderDay adds src into m[name].
func addProviderDay(m map[string]*providerDay, name string, src *providerDay) {
	d := m[name]
	if d == nil {
		d = &providerDay{}
		m[name] = d
	}
	d.USD += src.USD
	d.InputTokens += src.InputTokens
	d.CachedTokens += src.CachedTokens
	d.OutputTokens += src.OutputTokens
	d.Calls += src.Calls
	d.Unpriced += src.Unpriced
	if len(src.Computers) > 0 && d.Computers == nil {
		d.Computers = map[string]float64{}
	}
	for c, v := range src.Computers {
		d.Computers[c] += v
	}
}

// readPaidSpendFile reads the spend file. An absent file is (zero, false,
// nil): empty. A file that is present but cannot be read, does not parse as
// the record, or has no day is an error: it is never to be overwritten.
func readPaidSpendFile(path string) (rec paidSpendRecord, exists bool, err error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return paidSpendRecord{}, false, nil
	}
	if err != nil {
		return paidSpendRecord{}, true, fmt.Errorf("%w: %v", errSpendFileBad, err)
	}
	if err := json.Unmarshal(raw, &rec); err != nil || rec.Day == "" {
		return paidSpendRecord{}, true, fmt.Errorf("%w: %w: %s holds no usable day record", errSpendFileBad, errSpendFileCorrupt, path)
	}
	if rec.Today == nil {
		rec.Today = map[string]*providerDay{}
	}
	return rec, true, nil
}

func cloneProviderDays(m map[string]*providerDay) map[string]*providerDay {
	out := make(map[string]*providerDay, len(m))
	for name, d := range m {
		c := *d
		c.Computers = nil
		if len(d.Computers) > 0 {
			c.Computers = make(map[string]float64, len(d.Computers))
			for k, v := range d.Computers {
				c.Computers[k] = v
			}
		}
		out[name] = &c
	}
	return out
}

func cloneDelta(d map[string]map[string]*providerDay) map[string]map[string]*providerDay {
	out := make(map[string]map[string]*providerDay, len(d))
	for day, m := range d {
		out[day] = cloneProviderDays(m)
	}
	return out
}

// subtractDelta removes what a save has written from the live delta; calls
// recorded since the snapshot stay.
func subtractDelta(live, saved map[string]map[string]*providerDay) map[string]map[string]*providerDay {
	for day, m := range saved {
		for name, sd := range m {
			ld := live[day][name]
			if ld == nil {
				continue
			}
			ld.USD -= sd.USD
			ld.InputTokens -= sd.InputTokens
			ld.CachedTokens -= sd.CachedTokens
			ld.OutputTokens -= sd.OutputTokens
			ld.Calls -= sd.Calls
			ld.Unpriced -= sd.Unpriced
			for c, v := range sd.Computers {
				ld.Computers[c] -= v
			}
			if ld.Calls <= 0 {
				delete(live[day], name)
			}
		}
		if len(live[day]) == 0 {
			delete(live, day)
		}
	}
	if len(live) == 0 {
		return nil
	}
	return live
}

// mergeDelta adds this process's unsaved calls into the file's record, each
// to the day it was made on: a finished day's calls land in prev, and a day
// the file has not reached rolls it first. Calls older than prev are dropped
// (the file keeps two days).
func mergeDelta(base *paidSpendRecord, delta map[string]map[string]*providerDay) {
	days := make([]string, 0, len(delta))
	for day := range delta {
		days = append(days, day)
	}
	sort.Strings(days)
	for _, day := range days {
		var dst map[string]*providerDay
		switch {
		case day == base.Day:
			dst = base.Today
		case day == base.PrevDay || (base.PrevDay == "" && day < base.Day):
			base.PrevDay = day
			if base.Prev == nil {
				base.Prev = map[string]*providerDay{}
			}
			dst = base.Prev
		case day > base.Day:
			base.PrevDay, base.Prev = base.Day, base.Today
			base.Day, base.Today = day, map[string]*providerDay{}
			dst = base.Today
		default:
			continue
		}
		for name, pd := range delta[day] {
			addProviderDay(dst, name, pd)
		}
	}
}

// Flush saves now and returns the error, if any. The gateway calls it once
// after its server has drained (a SIGTERM's last save); tests call it to read
// the file.
func (s *PaidSpend) Flush() error {
	if s == nil || s.path == "" {
		return nil
	}
	return s.saver.run(s.saveOnce)
}

// unsavedCalls is the number of calls the file has not seen.
func (s *PaidSpend) unsavedCalls() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, m := range s.delta {
		for _, d := range m {
			n += d.Calls
		}
	}
	return n
}

// Persistence is the save status shown in the snapshot.
func (s *PaidSpend) Persistence() SpendPersistence {
	if s == nil {
		return SpendPersistence{}
	}
	return s.saver.status(s.unsavedCalls())
}

// saveOnce adds this process's unsaved calls to the file and takes the
// merged record back, so another gateway process's spend counts here too.
//
// It runs on the flusher, never on a guest's call, and holds s.mu only for
// the two short copies: the file lock (up to spendLockWait), the read and
// the write happen outside it. A file that is absent is empty; one that is
// present but unreadable, unparseable or without a day is never written
// over (the other process's spend may be in it): the error is returned, the
// delta is kept, and the caps keep counting memory plus delta, which holds
// everything this process has spent since it started or last merged. A lock
// not taken in spendLockWait or a failed write likewise keeps the delta.
//
// A crash loses at most the calls not yet saved: those recorded during one
// save (milliseconds when healthy), or, while saves are failing, every call
// since the last good save (the status says how many). A SIGTERM runs Flush
// after the server has drained.
func (s *PaidSpend) saveOnce() error {
	s.mu.Lock()
	empty := len(s.delta) == 0
	s.mu.Unlock()
	if empty {
		return nil
	}
	unlock, err := lockSpendFile(s.path)
	if err != nil {
		return err
	}
	defer unlock()
	base, exists, err := readPaidSpendFile(s.path)
	if errors.Is(err, errSpendFileCorrupt) {
		// Never overwritten: set aside under the lock we hold (a second
		// process that saw the same file waits on the lock, then finds the
		// fresh file), and carry on from memory.
		to, qerr := quarantineSpendFile(s.path, time.Now())
		if qerr != nil {
			return qerr
		}
		s.saver.noteBadMoved(s.path, to)
		base, exists, err = paidSpendRecord{}, false, nil
	}
	if err != nil {
		return err
	}
	s.mu.Lock()
	snap := cloneDelta(s.delta)
	var merged paidSpendRecord
	if exists {
		merged = base
		mergeDelta(&merged, snap)
	} else {
		// Nobody has saved: memory is this process's whole day.
		merged = paidSpendRecord{Day: s.rec.Day, Today: cloneProviderDays(s.rec.Today),
			PrevDay: s.rec.PrevDay, Prev: cloneProviderDays(s.rec.Prev)}
	}
	s.mu.Unlock()
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	if err := writeSpendFile(s.path, raw); err != nil {
		return err
	}
	if h := spendSaveWindowHook.Load(); h != nil {
		(*h)()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delta = subtractDelta(s.delta, snap)
	s.rec = merged
	if len(s.delta) > 0 {
		mergeDelta(&s.rec, s.delta) // calls made during the save, on top of the saved day
	}
	s.rollLocked()
	return nil
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
	// Persistence says whether the day's spend is reaching the file.
	Persistence *SpendPersistence `json:"persistence,omitempty"`
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
	if s.path != "" {
		var unsaved int64
		for _, m := range s.delta {
			for _, d := range m {
				unsaved += d.Calls
			}
		}
		p := s.saver.status(unsaved)
		snap.Persistence = &p
	}
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
	if pe := snap.Persistence; pe != nil && (pe.UnsavedCalls > 0 || pe.LastError != "" || pe.BadFilesMoved > 0) {
		fmt.Fprintf(&b, " [spend file: unsaved_calls=%d bad_files_moved=%d last_error=%q]", pe.UnsavedCalls, pe.BadFilesMoved, pe.LastError)
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
	// relay_persistence says whether the Anthropic relay's day spend is
	// reaching its file (unsaved charges, last error), beside the paid
	// ledger's own persistence block.
	writeGatewayJSON(w, http.StatusOK, map[string]any{"paid_spend": h.paidSpend.Snapshot(), "relay_persistence": h.anthropicRelay.Persistence()})
}
