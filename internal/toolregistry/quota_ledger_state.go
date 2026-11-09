package toolregistry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// Hard failures and the ledger's state file (owner, 2026-11-09: the pools
// "need to be intelligent enough that if a particular provider is not
// working, we don't keep retrying it ... some kind of memory"; see
// docs/evidence/provider-retry-audit-2026-11-09.md).
//
// A hard failure is one a wait does not cure: a key the provider refuses
// (401, 403) or a model it no longer serves (404, model not found). It is
// held from hardFailureHoldBase, doubling per consecutive failure up to
// hardFailureHoldCap, and ends on the member's first success.
const (
	hardFailureHoldBase = 15 * time.Minute
	hardFailureHoldCap  = 6 * time.Hour
	// QuotaHoldHardFailure is the source of a hard-failure hold.
	QuotaHoldHardFailure = "hard_failure"
	// maxPersistedHolds bounds the ledger's state file.
	maxPersistedHolds = 64
)

// IsHardProviderFailure reports whether err says the provider refuses our
// key or no longer serves the model, which no short wait cures. A bare 403
// or 404 is not enough: a content-policy 403 or a request-shaped 404 is
// caused by the request, and would hold each member in turn.
func IsHardProviderFailure(err error) bool {
	if err == nil || errors.Is(err, ErrNoProgress) {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"status 401", "401 unauthorized",
		"invalid api key", "invalid_api_key", "model_not_found", "model not found", "not in the catalog",
		"unsupported model", "unsupported provider",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	// "does not exist" alone is too broad; it counts only with a model named.
	return strings.Contains(text, "model") && strings.Contains(text, "does not exist")
}

// DemoteHard records a hard failure of sel at now and holds it for the
// doubled hard hold; it returns the time the hold ends.
func (l *QuotaLedger) DemoteHard(sel provideriface.LLMSelection, now time.Time) time.Time {
	if l == nil {
		return time.Time{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.initLocked()
	id := servingSelectionID(sel)
	hold := hardFailureHoldBase
	for n := l.hard[id]; n > 0 && hold < hardFailureHoldCap; n-- {
		hold *= 2
	}
	hold = min(hold, hardFailureHoldCap)
	if l.hard[id] < 16 {
		l.hard[id]++
	}
	until := now.Add(hold)
	if cur := l.until[id]; cur.After(until) {
		until = cur
	}
	l.until[id] = until
	l.setSourceLocked(id, QuotaHoldHardFailure)
	l.saveLocked()
	return until
}

func (l *QuotaLedger) initLocked() {
	if l.until == nil {
		l.until = map[string]time.Time{}
	}
	if l.strikes == nil {
		l.strikes = map[string]int{}
	}
	if l.hard == nil {
		l.hard = map[string]int{}
	}
}

// ledgerEntry is one member's record in the state file.
type ledgerEntry struct {
	Until   time.Time `json:"until,omitempty"`
	Source  string    `json:"source,omitempty"`
	Strikes int       `json:"strikes,omitempty"`
	Hard    int       `json:"hard,omitempty"`
}

type ledgerFile struct {
	Version int                    `json:"version"`
	Members map[string]ledgerEntry `json:"members"`
}

// Persist binds the ledger to a state file and loads it: the holds still in
// force at now and every member's failure counts. A crash restart must not
// resume work, but that a provider is down is not work, so it is restored.
// Every later change rewrites the file (atomically; the file is bounded). A
// missing or corrupt file starts the ledger empty and is not an error: the
// ledger is knowledge, and losing it costs only a re-learn.
func (l *QuotaLedger) Persist(path string, now time.Time) error {
	if l == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.initLocked()
	l.path = path
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f ledgerFile
	if json.Unmarshal(raw, &f) != nil {
		return nil
	}
	for id, e := range f.Members {
		if strings.TrimSpace(id) == "" {
			continue
		}
		if e.Until.After(now) {
			l.until[id] = e.Until
			l.setSourceLocked(id, e.Source)
		}
		if e.Strikes > 0 {
			l.strikes[id] = min(e.Strikes, 16)
		}
		if e.Hard > 0 {
			l.hard[id] = min(e.Hard, 16)
		}
	}
	return nil
}

// saveLocked rewrites the state file when the ledger is bound to one. A
// failed write is dropped: the ledger stays right in memory.
func (l *QuotaLedger) saveLocked() {
	if l.path == "" {
		return
	}
	members := map[string]ledgerEntry{}
	ids := map[string]bool{}
	for id := range l.until {
		ids[id] = true
	}
	for id := range l.strikes {
		ids[id] = true
	}
	for id := range l.hard {
		ids[id] = true
	}
	for id := range ids {
		members[id] = ledgerEntry{Until: l.until[id], Source: l.source[id], Strikes: l.strikes[id], Hard: l.hard[id]}
	}
	if len(members) > maxPersistedHolds {
		// Keep the members whose holds end latest.
		order := make([]string, 0, len(members))
		for id := range members {
			order = append(order, id)
		}
		sort.Slice(order, func(i, j int) bool {
			a, b := members[order[i]].Until, members[order[j]].Until
			if !a.Equal(b) {
				return a.After(b)
			}
			return order[i] < order[j]
		})
		for _, id := range order[maxPersistedHolds:] {
			delete(members, id)
		}
	}
	raw, err := json.Marshal(ledgerFile{Version: 1, Members: members})
	if err != nil {
		return
	}
	tmp := l.path + ".tmp"
	if os.MkdirAll(filepath.Dir(l.path), 0o755) != nil {
		return
	}
	if os.WriteFile(tmp, raw, 0o600) != nil {
		return
	}
	if os.Rename(tmp, l.path) != nil {
		_ = os.Remove(tmp)
	}
}
