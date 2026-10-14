package searchplane

import (
	"sync"
	"time"
)

// CallCounter counts provider calls per UTC day. The health store is its
// one authority: FileHealthStore keeps the counts in the health DB, so they
// survive a gateway restart; MemoryHealthStore keeps them in memory.
type CallCounter interface {
	// ReserveCall counts one call to provider on day unless that would
	// exceed limit (limit < 0: no cap). It returns the day's count after
	// the call (or the count that refused it) and whether it may be sent.
	// Check and count are one step, so concurrent searches cannot overshoot.
	ReserveCall(provider, day string, limit int) (calls int, ok bool, err error)
	// CallsOn returns every provider's count for day.
	CallsOn(day string) (map[string]int, error)
}

// DayOf is the counting day of t: its UTC date. A new day starts at UTC
// midnight whatever the host's zone.
func DayOf(t time.Time) string { return t.UTC().Format("2006-01-02") }

// NoCap is the limit for a provider without a daily cap.
const NoCap = -1

// countRetentionDays is how long daily counts are kept.
const countRetentionDays = 40

// memoryCallCounter is an in-process CallCounter.
type memoryCallCounter struct {
	mu   sync.Mutex
	days map[string]map[string]int
}

func newMemoryCallCounter() *memoryCallCounter {
	return &memoryCallCounter{days: map[string]map[string]int{}}
}

func (m *memoryCallCounter) ReserveCall(provider, day string, limit int) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	counts := m.days[day]
	if counts == nil {
		counts = map[string]int{}
		m.days[day] = counts
		m.pruneLocked(day)
	}
	if limit >= 0 && counts[provider] >= limit {
		return counts[provider], false, nil
	}
	counts[provider]++
	return counts[provider], true, nil
}

func (m *memoryCallCounter) CallsOn(day string) (map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]int{}
	for name, n := range m.days[day] {
		out[name] = n
	}
	return out, nil
}

func (m *memoryCallCounter) pruneLocked(today string) {
	cutoff := pruneCutoff(today)
	for day := range m.days {
		if day < cutoff {
			delete(m.days, day)
		}
	}
}

// pruneCutoff is the oldest day kept when today is today.
func pruneCutoff(today string) string {
	t, err := time.Parse("2006-01-02", today)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -countRetentionDays).Format("2006-01-02")
}

func (s *MemoryHealthStore) ReserveCall(provider, day string, limit int) (int, bool, error) {
	return s.calls.ReserveCall(provider, day, limit)
}

func (s *MemoryHealthStore) CallsOn(day string) (map[string]int, error) {
	return s.calls.CallsOn(day)
}
