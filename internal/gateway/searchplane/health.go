package searchplane

import (
	"strings"
	"sync"
	"time"
)

// HealthStore persists provider health across gateway restarts.
type HealthStore interface {
	Snapshot() (map[string]ProviderHealth, error)
	Get(provider string) (ProviderHealth, error)
	RecordOutcome(outcome Outcome) (ProviderHealth, error)
	ResetProvider(provider string) (ProviderHealth, error)
	ResetAll() error
}

// MemoryHealthStore is an in-process HealthStore for tests and dev defaults.
type MemoryHealthStore struct {
	mu      sync.Mutex
	records map[string]ProviderHealth
	policy  BackoffPolicy
}

// NewMemoryHealthStore creates an empty in-memory health store.
func NewMemoryHealthStore() *MemoryHealthStore {
	return &MemoryHealthStore{
		records: map[string]ProviderHealth{},
		policy:  DefaultBackoffPolicy(),
	}
}

func (s *MemoryHealthStore) Snapshot() (map[string]ProviderHealth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]ProviderHealth, len(s.records))
	for k, v := range s.records {
		out[k] = normalizeExpiredLocked(v, time.Now())
	}
	return out, nil
}

func (s *MemoryHealthStore) Get(provider string) (ProviderHealth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(provider), nil
}

func (s *MemoryHealthStore) RecordOutcome(outcome Outcome) (ProviderHealth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.getLocked(outcome.Provider)
	now := outcome.At
	if now.IsZero() {
		now = time.Now()
	}
	rec.UpdatedAt = now
	rec.WindowAttempts++
	if outcome.Class == OutcomeSuccess {
		rec.WindowSuccesses++
		rec.WindowResultsTotal += outcome.Results
	}
	applyOutcome(&rec, outcome, s.policy, now)
	s.records[outcome.Provider] = rec
	return rec, nil
}

func (s *MemoryHealthStore) ResetProvider(provider string) (ProviderHealth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := ProviderHealth{
		Provider:  provider,
		State:     StateActive,
		UpdatedAt: time.Now(),
	}
	s.records[provider] = rec
	return rec, nil
}

func (s *MemoryHealthStore) ResetAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = map[string]ProviderHealth{}
	return nil
}

func (s *MemoryHealthStore) getLocked(provider string) ProviderHealth {
	rec, ok := s.records[provider]
	if !ok {
		return ProviderHealth{
			Provider:  provider,
			State:     StateActive,
			UpdatedAt: time.Now(),
		}
	}
	rec = normalizeExpiredLocked(rec, time.Now())
	s.records[provider] = rec
	return rec
}

func normalizeExpiredLocked(rec ProviderHealth, now time.Time) ProviderHealth {
	if rec.State == StateCoolingDown && rec.CooldownUntil != nil && !rec.CooldownUntil.After(now) {
		rec.State = StateActive
		rec.CooldownUntil = nil
	}
	return rec
}

func applyOutcome(rec *ProviderHealth, outcome Outcome, policy BackoffPolicy, now time.Time) {
	switch outcome.Class {
	case OutcomeSuccess:
		rec.State = StateActive
		rec.CooldownUntil = nil
		rec.StrikeCount = 0
		rec.LastFailureClass = ""
		rec.LastErrorSummary = ""
	case OutcomeSkippedCoolingDown:
		return
	case OutcomeAuthError:
		rec.StrikeCount++
		rec.LastFailureClass = string(outcome.Class)
		rec.LastErrorSummary = truncateSummary(outcome.Error)
		rec.State = StateDisabled
		until := now.Add(policy.CooldownDuration(outcome.Class, rec.StrikeCount))
		rec.CooldownUntil = &until
	default:
		rec.StrikeCount++
		rec.LastFailureClass = string(outcome.Class)
		rec.LastErrorSummary = truncateSummary(outcome.Error)
		d := policy.CooldownDuration(outcome.Class, rec.StrikeCount)
		if d > 0 {
			rec.State = StateCoolingDown
			until := now.Add(d)
			rec.CooldownUntil = &until
		}
	}
}

func truncateSummary(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) > 240 {
		return msg[:240] + "..."
	}
	return msg
}

// IsEligible reports whether a provider may receive HTTP traffic right now.
func IsEligible(rec ProviderHealth, now time.Time) bool {
	rec = normalizeExpiredLocked(rec, now)
	if rec.State == StateDisabled {
		return false
	}
	if rec.State == StateCoolingDown && rec.CooldownUntil != nil && rec.CooldownUntil.After(now) {
		return false
	}
	return true
}
