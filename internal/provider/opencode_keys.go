package provider

import (
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// OpenCode key pool (docs/archive/problems/opencode-key-exhausted-no-failover-2026-10-15.md).
// The gateway may hold several OpenCode accounts: OPENCODE_API_KEY (or the
// older OPENCODE_GO_API_KEY) and OPENCODE_API_KEY_ALT_<n>. A key that answers
// 402 (out of funds) or 401 (a revoked or expired credential,
// docs/problems/opencode-401-key-never-parked-2026-10-22.md) is parked for
// hours; one that answers 429 is parked for a minute. A session keeps one
// healthy key while it stays healthy, so OpenCode's per-session upstream
// routing keeps its prompt cache. Keys are
// named by their variable, never by value.

const (
	openCodeFundsPark     = 6 * time.Hour
	openCodeRateLimitPark = 60 * time.Second
	// A revoked or expired credential (401) answers the same on every call
	// until the owner replaces it, which takes a gateway restart (the keys
	// are read once) and so clears every park. 403 does not park: OpenCode
	// also answers 403 for a model the plan does not cover, and the pool is
	// shared by every model.
	openCodeCredentialPark = 6 * time.Hour
	openCodeAltKeyPrefix   = "OPENCODE_API_KEY_ALT_"
	// openCodeKeysPolicyEnv is the gateway policy naming which configured key
	// variables serve (comma or space separated). Unset, every configured key
	// serves; set, only the listed ones, so a list naming no configured key
	// leaves OpenCode with none. It is policy, set in the gateway unit, never
	// in the credential file (owner, 2026-10-22).
	openCodeKeysPolicyEnv = "GATEWAY_OPENCODE_KEYS"
)

type openCodeKey struct {
	name  string
	value string
}

type openCodeKeyPool struct {
	mu     sync.Mutex
	keys   []openCodeKey
	parked map[string]time.Time
	now    func() time.Time
}

func newOpenCodeKeyPool(keys []openCodeKey) *openCodeKeyPool {
	return &openCodeKeyPool{keys: keys, parked: map[string]time.Time{}, now: time.Now}
}

var (
	envOpenCodeKeysOnce sync.Once
	envOpenCodeKeys     *openCodeKeyPool
)

// sharedOpenCodeKeyPool is the process-wide pool from the environment, shared
// by every OpenCode Go and Zen provider so a parked key is parked for all.
func sharedOpenCodeKeyPool() *openCodeKeyPool {
	envOpenCodeKeysOnce.Do(func() {
		keys := openCodeKeysFromEnv()
		logOpenCodeKeysInService(keys)
		envOpenCodeKeys = newOpenCodeKeyPool(keys)
	})
	return envOpenCodeKeys
}

var openCodeKeysLogOnce sync.Once

// logOpenCodeKeysInService logs, once per process, which keys serve, by name
// (never value). The presence check calls it too, so a policy that admits no
// key, and so builds no provider, still says so.
func logOpenCodeKeysInService(keys []openCodeKey) {
	openCodeKeysLogOnce.Do(func() {
		names := make([]string, len(keys))
		for i, k := range keys {
			names[i] = k.name
		}
		policy := "unset, every configured key"
		if openCodeKeysPolicy() != nil {
			policy = fmt.Sprintf("%q", os.Getenv(openCodeKeysPolicyEnv))
		}
		log.Printf("provider: OpenCode keys in service: [%s] (%s: %s)", strings.Join(names, " "), openCodeKeysPolicyEnv, policy)
	})
}

// openCodeKeysFromEnv returns the configured keys in order: the primary,
// then ALT_0, ALT_1, ..., with duplicate values dropped.
func openCodeKeysFromEnv() []openCodeKey {
	var keys []openCodeKey
	seen := map[string]bool{}
	allowed := openCodeKeysPolicy()
	add := func(name string) {
		if allowed != nil && !allowed[name] {
			return
		}
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		keys = append(keys, openCodeKey{name: name, value: value})
	}
	add("OPENCODE_API_KEY")
	add("OPENCODE_GO_API_KEY")
	type alt struct {
		n    int
		name string
	}
	var alts []alt
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(name, openCodeAltKeyPrefix) {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(name, openCodeAltKeyPrefix)); err == nil && n >= 0 {
			alts = append(alts, alt{n, name})
		}
	}
	sort.Slice(alts, func(i, j int) bool { return alts[i].n < alts[j].n })
	for _, a := range alts {
		add(a.name)
	}
	return keys
}

// openCodeKeysPolicy returns the key variable names the policy admits, or
// nil when the variable is unset (every configured key serves).
func openCodeKeysPolicy() map[string]bool {
	raw, ok := os.LookupEnv(openCodeKeysPolicyEnv)
	if !ok {
		return nil
	}
	// Set but blank names no key, so none serve: a blank must not fail open.
	allowed := map[string]bool{}
	for _, name := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		allowed[name] = true
	}
	return allowed
}

func (p *openCodeKeyPool) size() int {
	if p == nil {
		return 0
	}
	return len(p.keys)
}

// pick returns the healthy key for session, skipping names in exclude. The
// choice is stable for a session over the current healthy set.
func (p *openCodeKeyPool) pick(session string, exclude map[string]bool) (openCodeKey, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	var healthy []openCodeKey
	for _, k := range p.keys {
		if exclude[k.name] {
			continue
		}
		if until, ok := p.parked[k.name]; ok {
			if now.Before(until) {
				continue
			}
			delete(p.parked, k.name)
		}
		healthy = append(healthy, k)
	}
	if len(healthy) == 0 {
		return openCodeKey{}, false
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(session))
	return healthy[int(h.Sum32()%uint32(len(healthy)))], true
}

// park benches a key after a quota or dead-credential status. It reports
// whether the status is one that parks (and so may be retried on another key).
func (p *openCodeKeyPool) park(name string, status int) bool {
	var d time.Duration
	switch status {
	case http.StatusPaymentRequired:
		d = openCodeFundsPark
	case http.StatusTooManyRequests:
		d = openCodeRateLimitPark
	case http.StatusUnauthorized:
		d = openCodeCredentialPark
	default:
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if status == http.StatusUnauthorized && !p.otherKeyHealthyLocked(name) {
		// A 401 on the last healthy key may be upstream-wide (an auth outage,
		// a proxy rejection), not this credential: benching every key for
		// hours would outlast it, so the call fails as it did before.
		return false
	}
	p.parked[name] = p.now().Add(d)
	return true
}

func (p *openCodeKeyPool) otherKeyHealthyLocked(name string) bool {
	now := p.now()
	for _, k := range p.keys {
		if k.name == name {
			continue
		}
		if until, ok := p.parked[k.name]; !ok || !now.Before(until) {
			return true
		}
	}
	return false
}
