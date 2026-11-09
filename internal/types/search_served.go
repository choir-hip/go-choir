package types

import (
	"sort"
	"strings"
)

// EventSearchServed is the durable record of which search providers served a
// run's web searches, written beside model.served (owner, 2026-11-09: "in the
// same sources where we show which language models are used, we should also
// be showing which search providers are used"). One line per finished
// desk search: for each provider that was asked, how many calls it answered
// and how many results it returned. It is small and kept.
const EventSearchServed EventKind = "search.served"

// SearchServedSchema versions the payload of an EventSearchServed line.
const SearchServedSchema = "choir.search_served.v1"

// SearchServedRecord is the payload of one EventSearchServed line.
type SearchServedRecord struct {
	Schema string `json:"schema"`
	// Role is the desk role the search served (a research child's role),
	// when the writer knows it better than the run record does.
	Role      string              `json:"role,omitempty"`
	Providers []SearchServedUsage `json:"providers"`
}

// SearchServedUsage is one provider's share of one search. Calls is 1 when
// the provider was asked in this search, Answered 1 when it answered, and
// Results the hits it returned.
type SearchServedUsage struct {
	Provider string `json:"provider"`
	Calls    int    `json:"calls"`
	Answered int    `json:"answered"`
	Results  int    `json:"results"`
}

// SearchProvenance is the search providers that served a Texture document's
// runs, by desk role.
type SearchProvenance struct {
	Roles []SearchProvenanceRole `json:"roles"`
}

// SearchProvenanceRole is one desk role's search providers.
type SearchProvenanceRole struct {
	Role      string                     `json:"role"`
	Providers []SearchProvenanceProvider `json:"providers"`
}

// SearchProvenanceProvider is one provider's calls for a role.
type SearchProvenanceProvider struct {
	Provider string `json:"provider"`
	Calls    int    `json:"calls"`
	Answered int    `json:"answered"`
	Results  int    `json:"results"`
}

// SearchProvenanceBuilder folds search.served records into a SearchProvenance.
type SearchProvenanceBuilder struct {
	roles map[string]map[string]*SearchProvenanceProvider
}

// AddServed folds one record under role.
func (b *SearchProvenanceBuilder) AddServed(role string, rec SearchServedRecord) {
	role = strings.TrimSpace(role)
	if role == "" {
		role = "unknown"
	}
	if b.roles == nil {
		b.roles = map[string]map[string]*SearchProvenanceProvider{}
	}
	for _, u := range rec.Providers {
		name := strings.TrimSpace(u.Provider)
		if name == "" {
			continue
		}
		if b.roles[role] == nil {
			b.roles[role] = map[string]*SearchProvenanceProvider{}
		}
		p := b.roles[role][name]
		if p == nil {
			p = &SearchProvenanceProvider{Provider: name}
			b.roles[role][name] = p
		}
		p.Calls += u.Calls
		p.Answered += u.Answered
		p.Results += u.Results
	}
}

// Build returns the provenance, roles by name and providers by calls; nil
// when no search was recorded.
func (b *SearchProvenanceBuilder) Build() *SearchProvenance {
	if len(b.roles) == 0 {
		return nil
	}
	out := &SearchProvenance{Roles: []SearchProvenanceRole{}}
	for role, providers := range b.roles {
		r := SearchProvenanceRole{Role: role}
		for _, p := range providers {
			r.Providers = append(r.Providers, *p)
		}
		sort.Slice(r.Providers, func(i, j int) bool {
			if r.Providers[i].Calls != r.Providers[j].Calls {
				return r.Providers[i].Calls > r.Providers[j].Calls
			}
			return r.Providers[i].Provider < r.Providers[j].Provider
		})
		out.Roles = append(out.Roles, r)
	}
	sort.Slice(out.Roles, func(i, j int) bool { return out.Roles[i].Role < out.Roles[j].Role })
	return out
}
