package modelpolicy

// Pools: the one resolver for "what are pool X's members", and the parser for
// the [pools.<name>] sections of the policy file
// (docs/design/programmatic-model-switching-2026-11-09.md, 3.1 and 3.2).
//
// A pool is the same object wherever it is written. Resolution order is the
// computer's own file, then the compiled modelcatalog.Pools (the release).
// This file is the only place in the package that reads the compiled table;
// TestNoDirectCompiledPoolReaders fails when another file does.

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
)

// Pool sources, as `choir policy get` shows them.
const (
	PoolSourceFile    = "file"
	PoolSourceRelease = "release"
)

// catalogPools is the compiled pool table: modelcatalog.Pools in production,
// a seam for tests that need a default pool of several members. Only the
// resolver below reads it.
var catalogPools = modelcatalog.Pools

// Pool is a resolved pool: its members in order, whether a failing member
// moves its thread to the next, and where the definition came from.
type Pool struct {
	Name     string
	Members  []modelcatalog.PoolMember
	Failover bool
	Source   string
}

// PoolView is a Pool as the resolve endpoint and `choir policy get` show it.
type PoolView struct {
	Name     string           `json:"name"`
	Source   string           `json:"source"`
	Failover bool             `json:"failover"`
	Members  []PoolMemberView `json:"members"`
}

// PoolMemberView is one member: provider/model, its assignment weight (zero
// for a fallback-only member) and the fallback flag.
type PoolMemberView struct {
	Model        string `json:"model"`
	Weight       int    `json:"weight"`
	FallbackOnly bool   `json:"fallback_only,omitempty"`
}

// resolvePool answers "members of pool name": the file's definition first,
// then the compiled pool. A compiled pool with no members is not a pool.
func resolvePool(file map[string]Pool, name string) (Pool, bool) {
	name = strings.TrimSpace(name)
	if pool, ok := file[name]; ok {
		return pool, true
	}
	members, ok := catalogPools()[name]
	if !ok || len(members) == 0 {
		return Pool{}, false
	}
	return Pool{Name: name, Members: members, Failover: !modelcatalog.NoFailover(name), Source: PoolSourceRelease}, true
}

// compiledPool is the release's definition of a pool, whatever the file says.
func compiledPool(name string) (Pool, bool) { return resolvePool(nil, name) }

// resolvedPoolNames lists every pool name a policy can name: the file's and
// the release's, sorted.
func resolvedPoolNames(file map[string]Pool) []string {
	seen := map[string]bool{}
	for name := range file {
		seen[name] = true
	}
	for name, members := range catalogPools() {
		if len(members) > 0 {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// pool resolves a pool name for this policy.
func (p Policy) pool(name string) (Pool, bool) { return resolvePool(p.Pools, name) }

// HasPool reports whether this policy can name pool name.
func (p Policy) HasPool(name string) bool {
	_, ok := p.pool(name)
	return ok
}

// poolMembersOf is the members of pool name for this policy (nil if none).
func (p Policy) poolMembersOf(name string) []modelcatalog.PoolMember {
	pool, _ := p.pool(name)
	return pool.Members
}

// PoolViews lists every pool this policy can name, with its source.
func (p Policy) PoolViews() []PoolView {
	var out []PoolView
	for _, name := range resolvedPoolNames(p.Pools) {
		// Every name came from the file or the release, so it resolves; a file
		// section whose members were all dropped is shown with no members.
		pool, _ := p.pool(name)
		view := PoolView{Name: name, Source: pool.Source, Failover: pool.Failover, Members: []PoolMemberView{}}
		for _, member := range pool.Members {
			view.Members = append(view.Members, PoolMemberView{
				Model:        member.Provider + "/" + member.Model,
				Weight:       member.SlotWeight(),
				FallbackOnly: member.FallbackOnly,
			})
		}
		out = append(out, view)
	}
	return out
}

// PoolViews lists the pools this computer's policy resolves, each with the
// source of its definition (the resolve endpoint's pools field).
func (m *Manager) PoolViews(ctx context.Context, ownerID string) []PoolView {
	policy, _ := m.Load(ctx, ownerID)
	return policy.PoolViews()
}

// memberSpec is one parsed member before the catalog check.
type memberSpec struct {
	provider, model string
	weight          int // 0: unset
	fallbackOnly    bool
}

// poolDraft is one [pools.<name>] section as read.
type poolDraft struct {
	name      string
	line      int
	members   []memberSpec
	shorthand bool
	hasList   bool
	failover  *bool
}

const poolSectionPrefix = "pools."

// poolName validates a pool section name.
func poolName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// applyPoolKey reads one key of a pools.<name> section. The members value may
// have been joined from several lines by the caller.
func (d *poolDraft) applyKey(key, value string) error {
	switch strings.TrimSpace(key) {
	case "members":
		if d.hasList {
			return fmt.Errorf("pool %q: members given twice", d.name)
		}
		specs, shorthand, err := parseMembersArray(value)
		if err != nil {
			return fmt.Errorf("pool %q: %w", d.name, err)
		}
		d.members, d.shorthand, d.hasList = specs, shorthand, true
	case "failover":
		if d.failover != nil {
			return fmt.Errorf("pool %q: failover given twice", d.name)
		}
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true":
			t := true
			d.failover = &t
		case "false":
			f := false
			d.failover = &f
		default:
			return fmt.Errorf("pool %q: failover must be true or false, got %q", d.name, value)
		}
	default:
		return fmt.Errorf("pool %q: unknown key %q (a pool has members and failover)", d.name, key)
	}
	return nil
}

// buildFilePools turns the drafts into resolved pools. It refuses what is
// ambiguous (a shorthand section over a weighted or no-failover pool, a bad
// weight) and drops, with a diagnostic, a member the catalog does not list:
// a stale member never invalidates the file. A pool left with no member falls
// back to the compiled pool of that name when there is one.
func buildFilePools(drafts []*poolDraft, diagnostics *[]string) (map[string]Pool, error) {
	if len(drafts) == 0 {
		return nil, nil
	}
	out := map[string]Pool{}
	for _, d := range drafts {
		if !d.hasList {
			return nil, fmt.Errorf("pool %q (line %d): members is required", d.name, d.line)
		}
		compiled, hasCompiled := compiledPool(d.name)
		if d.shorthand && hasCompiled {
			weighted := 0
			for _, member := range compiled.Members {
				if member.SlotWeight() > 0 {
					weighted++
				}
			}
			if weighted > 1 || !compiled.Failover {
				return nil, fmt.Errorf("pool %q: a plain list of members would flatten the release's pool of that name (%s); write the members as { model = ..., weight = N, fallback_only = bool } tables and say failover", d.name, describeShape(weighted, compiled.Failover))
			}
		}
		failover := true
		if d.failover != nil {
			failover = *d.failover
		}
		var members []modelcatalog.PoolMember
		for i, spec := range d.members {
			member := modelcatalog.PoolMember{Provider: spec.provider, Model: spec.model}
			if d.shorthand {
				member.FallbackOnly = i > 0
				if i == 0 {
					member.Weight = 1
				}
			} else {
				member.Weight, member.FallbackOnly = spec.weight, spec.fallbackOnly
			}
			if _, ok := modelcatalog.Lookup(member.Provider, member.Model); !ok {
				*diagnostics = append(*diagnostics, fmt.Sprintf("model policy pool %q names %s/%s, which the catalog does not list; dropped from the pool", d.name, member.Provider, member.Model))
				continue
			}
			members = append(members, member)
		}
		if len(members) == 0 {
			if hasCompiled {
				*diagnostics = append(*diagnostics, fmt.Sprintf("model policy pool %q has no member the catalog lists; using the release's pool of that name", d.name))
				continue
			}
			*diagnostics = append(*diagnostics, fmt.Sprintf("model policy pool %q has no member the catalog lists; roles naming it fail closed", d.name))
		}
		out[d.name] = Pool{Name: d.name, Members: members, Failover: failover, Source: PoolSourceFile}
	}
	return out, nil
}

func describeShape(weighted int, failover bool) string {
	var parts []string
	if weighted > 1 {
		parts = append(parts, fmt.Sprintf("%d weighted members", weighted))
	}
	if !failover {
		parts = append(parts, "failover disabled")
	}
	return strings.Join(parts, ", ")
}

// arrayComplete reports whether s holds a whole [...] value: brackets and
// braces balanced outside strings.
func arrayComplete(s string) bool {
	depth, seen := 0, false
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case quote == '"':
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '"' {
				quote = 0
			}
		case quote == '\'':
			if r == '\'' {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '[' || r == '{':
			depth++
			seen = true
		case r == ']' || r == '}':
			depth--
		}
	}
	return seen && depth <= 0 && quote == 0
}

// parseMembersArray reads `[ "provider/model", ... ]` (shorthand) or
// `[ { model = "provider/model", weight = N, fallback_only = bool }, ... ]`.
func parseMembersArray(text string) ([]memberSpec, bool, error) {
	s := strings.TrimSpace(text)
	if !strings.HasPrefix(s, "[") {
		return nil, false, fmt.Errorf("members must be a list")
	}
	i := 1
	var specs []memberSpec
	strings_, tables := 0, 0
	for {
		i = skipSpaceAndCommas(s, i)
		if i >= len(s) {
			return nil, false, fmt.Errorf("members list is not closed")
		}
		switch s[i] {
		case ']':
			if rest := strings.TrimSpace(s[i+1:]); rest != "" {
				return nil, false, fmt.Errorf("unexpected text after members list: %q", rest)
			}
			if len(specs) == 0 {
				return nil, false, fmt.Errorf("members must hold at least one model")
			}
			if strings_ > 0 && tables > 0 {
				return nil, false, fmt.Errorf("members mixes plain strings and tables; use one form")
			}
			return specs, strings_ > 0, nil
		case '"', '\'':
			value, next, err := readTOMLString(s, i)
			if err != nil {
				return nil, false, err
			}
			provider, model, err := splitMember(value)
			if err != nil {
				return nil, false, err
			}
			specs = append(specs, memberSpec{provider: provider, model: model})
			strings_++
			i = next
		case '{':
			spec, next, err := readMemberTable(s, i)
			if err != nil {
				return nil, false, err
			}
			specs = append(specs, spec)
			tables++
			i = next
		default:
			return nil, false, fmt.Errorf("unexpected %q in members", string(s[i]))
		}
	}
}

func skipSpaceAndCommas(s string, i int) int {
	for i < len(s) && (isBlank(s[i]) || s[i] == ',') {
		i++
	}
	return i
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' }

// splitMember splits "provider/model" at the first slash; model ids may
// contain slashes themselves.
func splitMember(value string) (string, string, error) {
	provider, model, ok := strings.Cut(strings.TrimSpace(value), "/")
	provider, model = strings.TrimSpace(provider), strings.TrimSpace(model)
	if !ok || provider == "" || model == "" {
		return "", "", fmt.Errorf("member %q must be written provider/model", value)
	}
	return provider, model, nil
}

// readTOMLString reads a basic ("...") or literal ('...') string at s[i].
func readTOMLString(s string, i int) (string, int, error) {
	quote := s[i]
	var b strings.Builder
	for j := i + 1; j < len(s); j++ {
		c := s[j]
		switch {
		case c == quote:
			return b.String(), j + 1, nil
		case c == '\\' && quote == '"':
			j++
			if j >= len(s) {
				return "", 0, fmt.Errorf("unterminated string")
			}
			switch s[j] {
			case '"', '\\':
				b.WriteByte(s[j])
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				return "", 0, fmt.Errorf("unsupported escape \\%c in string", s[j])
			}
		default:
			b.WriteByte(c)
		}
	}
	return "", 0, fmt.Errorf("unterminated string")
}

// readMemberTable reads `{ model = "...", weight = N, fallback_only = bool }`.
func readMemberTable(s string, i int) (memberSpec, int, error) {
	var spec memberSpec
	var haveModel bool
	seen := map[string]bool{}
	i++ // {
	for {
		i = skipSpaceAndCommas(s, i)
		if i >= len(s) {
			return memberSpec{}, 0, fmt.Errorf("member table is not closed")
		}
		if s[i] == '}' {
			if !haveModel {
				return memberSpec{}, 0, fmt.Errorf("member table has no model")
			}
			if spec.weight != 0 && spec.fallbackOnly {
				return memberSpec{}, 0, fmt.Errorf("member %s/%s has a weight and is fallback_only; a fallback takes no threads", spec.provider, spec.model)
			}
			return spec, i + 1, nil
		}
		start := i
		for i < len(s) && (s[i] == '_' || (s[i] >= 'a' && s[i] <= 'z')) {
			i++
		}
		key := s[start:i]
		for i < len(s) && isBlank(s[i]) {
			i++
		}
		if key == "" || i >= len(s) || s[i] != '=' {
			return memberSpec{}, 0, fmt.Errorf("member table: expected key = value near %q", strings.TrimSpace(s[start:min(len(s), start+20)]))
		}
		i++
		for i < len(s) && isBlank(s[i]) {
			i++
		}
		if i >= len(s) {
			return memberSpec{}, 0, fmt.Errorf("member table is not closed")
		}
		if seen[key] {
			return memberSpec{}, 0, fmt.Errorf("member key %q given twice", key)
		}
		seen[key] = true
		switch key {
		case "model":
			if s[i] != '"' && s[i] != '\'' {
				return memberSpec{}, 0, fmt.Errorf("member model must be a string")
			}
			value, next, err := readTOMLString(s, i)
			if err != nil {
				return memberSpec{}, 0, err
			}
			if spec.provider, spec.model, err = splitMember(value); err != nil {
				return memberSpec{}, 0, err
			}
			haveModel, i = true, next
		case "weight", "fallback_only":
			start := i
			for i < len(s) && s[i] != ',' && s[i] != '}' && !isBlank(s[i]) {
				i++
			}
			word := s[start:i]
			if key == "weight" {
				n, err := strconv.Atoi(word)
				if err != nil || n < 1 || n > 1000 {
					return memberSpec{}, 0, fmt.Errorf("member weight must be a whole number from 1 to 1000, got %q (use fallback_only = true for a member that takes no threads)", word)
				}
				spec.weight = n
			} else {
				switch word {
				case "true":
					spec.fallbackOnly = true
				case "false":
					spec.fallbackOnly = false
				default:
					return memberSpec{}, 0, fmt.Errorf("member fallback_only must be true or false, got %q", word)
				}
			}
		default:
			return memberSpec{}, 0, fmt.Errorf("unknown member key %q (a member has model, weight and fallback_only)", key)
		}
	}
}
