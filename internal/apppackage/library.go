package apppackage

import (
	"sort"
	"strings"
)

// Library facts (slice L1 of docs/design/modular-architecture-plan-2026-11-04.md
// section 11; system design section 2.3, "Kind inference"). The Library shows
// what a package is from facts the host computes from the manifest, never from
// a field the publisher wrote (a manifest that states kind or tier is refused,
// publisherDeclared):
//
//   - kind: provider when it provides a shell service, app when it has a window
//     or a desk (a desk is an app with the agent facet, system design section
//     7), otherwise pack (contributes only, runs no code). The handler and
//     backend kinds arrive with the intercepts and backend manifest keys, which
//     choir.package/v2 does not have yet;
//   - facets: every other present part (agent, commands, keybindings, menus,
//     themes, links, settings, opens, kinds);
//   - slots: the shell service slots it provides;
//   - tier: T2 for any shell surface, T1 for any window or desk, else T0;
//   - ceiling: the capability requests with each grant's catalogue level.

// PackageKind is a package's Library kind.
type PackageKind string

const (
	PackageKindApp      PackageKind = "app"
	PackageKindPack     PackageKind = "pack"
	PackageKindProvider PackageKind = "provider"
)

// Tier is a package's trust tier: T0 data the base validates and applies, T1
// code that reaches the host only through the SDK bridge, T2 code in the
// shell's realm (a shell surface, which needs ui.shell).
type Tier string

const (
	Tier0 Tier = "T0"
	Tier1 Tier = "T1"
	Tier2 Tier = "T2"
)

// Facets a Library listing may show beside its kind.
const (
	FacetAgent       = "agent"
	FacetCommands    = "commands"
	FacetKeybindings = "keybindings"
	FacetKinds       = "kinds"
	FacetLinks       = "links"
	FacetMenus       = "menus"
	FacetOpens       = "opens"
	FacetSettings    = "settings"
	FacetThemes      = "themes"
)

// PackageKind infers the Library kind; first match wins.
func (m Manifest) PackageKind() PackageKind {
	switch {
	case len(m.Provides) > 0:
		return PackageKindProvider
	case m.App != nil || len(m.Contributes.Desks) > 0:
		return PackageKindApp
	}
	return PackageKindPack
}

// Tier is the highest tier over the package's parts.
func (m Manifest) Tier() Tier {
	switch {
	case len(m.Provides) > 0:
		return Tier2
	case m.App != nil || len(m.Contributes.Desks) > 0:
		return Tier1
	}
	return Tier0
}

// Slots are the shell service slots the package provides, sorted.
func (m Manifest) Slots() []string {
	out := []string{}
	for _, p := range m.Provides {
		out = append(out, p.Service)
	}
	sort.Strings(out)
	return out
}

// Facets are the parts the package has beside the one that sets its kind,
// sorted.
func (m Manifest) Facets() []string {
	c := m.Contributes
	out := []string{}
	add := func(present bool, facet string) {
		if present {
			out = append(out, facet)
		}
	}
	add(len(c.Desks) > 0, FacetAgent)
	add(len(c.Commands) > 0, FacetCommands)
	add(len(c.Keybindings) > 0, FacetKeybindings)
	add(len(m.Kinds) > 0, FacetKinds)
	add(len(c.Links) > 0, FacetLinks)
	add(len(c.Menus) > 0, FacetMenus)
	add(len(c.Opens) > 0, FacetOpens)
	add(len(c.Settings) > 0, FacetSettings)
	add(len(c.Themes) > 0, FacetThemes)
	sort.Strings(out)
	return out
}

// CeilingGrant is one request of the package's ceiling with the grant's
// catalogue level.
type CeilingGrant struct {
	Grant string   `json:"grant"`
	Args  []string `json:"args,omitempty"`
	Level Level    `json:"level"`
}

// Ceiling is the package's capability list with each grant's level, sorted by
// grant then arguments; empty, never nil, for a package that asks for nothing.
func (m Manifest) Ceiling() []CeilingGrant {
	out := make([]CeilingGrant, 0, len(m.Capabilities))
	for _, c := range m.Capabilities {
		level, _ := GrantLevel(c.Grant)
		var args []string
		if len(c.Args) > 0 {
			args = append([]string{}, c.Args...)
			sort.Strings(args)
		}
		out = append(out, CeilingGrant{Grant: c.Grant, Args: args, Level: level})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Grant != out[j].Grant {
			return out[i].Grant < out[j].Grant
		}
		return strings.Join(out[i].Args, ",") < strings.Join(out[j].Args, ",")
	})
	return out
}
