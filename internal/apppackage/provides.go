package apppackage

import (
	"strings"
)

// Provides: a package that draws a shell surface (plan section 11, slices H1
// to H4; docs/design/modular-architecture-2026-11-04.md sections 2.1, 2.3 and
// 4b). Each entry names one shell service slot, the slot's ABI and the Svelte
// component at the package root that renders it. A provider draws shell UI,
// so it is tier T2: it must hold ui.shell, a privileged grant, which no
// publisher but the platform holds until sandboxed renderers exist (R24); the
// privileged-grant refusal in ValidateOn therefore refuses every published
// provider. Tier is computed, never declared (publisherDeclared).

// Provide is one shell service slot a package provides.
type Provide struct {
	Service string `json:"service"`
	ABI     string `json:"abi"`
	// Entry is the component at the package root, <Name>.surface.svelte.
	Entry string `json:"entry"`
	// RequiresSDK is the SDK range the surface needs (^MAJOR.MINOR).
	RequiresSDK string `json:"requires_sdk,omitempty"`
	// Priority ranks the provider among others of the slot, 0 to 50 (above
	// 50 needs rank.high, which comes only with a platform review record).
	Priority int `json:"priority,omitempty"`
}

// ShellServices are the shell service slots a package may provide.
var ShellServices = []string{
	"contextMenu.renderer",
	"desktop.icons",
	"launcher",
	"notifications.host",
	"overview",
	"prompt",
}

// ClassProvider is a package that draws a shell surface and has no window.
const ClassProvider Class = "provider"

const (
	maxProvides      = 4
	maxPriority      = 50
	providerABIStart = "choir.service/"
	providerABIEnd   = "/v1"
	surfaceEntryEnd  = ".surface.svelte"
)

func isShellService(s string) bool {
	for _, n := range ShellServices {
		if n == s {
			return true
		}
	}
	return false
}

func (m Manifest) validateProvides() error {
	if len(m.Provides) == 0 {
		return nil
	}
	if len(m.Provides) > maxProvides {
		return invalid("provides: at most %d", maxProvides)
	}
	if m.App != nil {
		return invalid("provides: a package draws a window or shell surfaces, not both")
	}
	shell := false
	for _, c := range m.Capabilities {
		if c.Grant == "ui.shell" {
			shell = true
		}
	}
	if !shell {
		return invalid("provides: a shell surface needs the ui.shell grant")
	}
	seen := map[string]bool{}
	entries := map[string]bool{}
	for i, p := range m.Provides {
		if !isShellService(p.Service) {
			return invalid("provides[%d]: %q is not a shell service (%s)", i, p.Service, strings.Join(ShellServices, ", "))
		}
		if seen[p.Service] {
			return invalid("provides[%d]: %s is provided twice", i, p.Service)
		}
		seen[p.Service] = true
		if p.ABI != providerABIStart+p.Service+providerABIEnd {
			return invalid("provides[%d]: abi %q is not %s%s%s", i, p.ABI, providerABIStart, p.Service, providerABIEnd)
		}
		base := strings.TrimSuffix(p.Entry, surfaceEntryEnd)
		if base == p.Entry || base == "" || !pathSegmentRe.MatchString(p.Entry) || strings.ContainsAny(base, "./\\") {
			return invalid("provides[%d]: entry %q must be <Name>%s at the package root", i, p.Entry, surfaceEntryEnd)
		}
		if entries[p.Entry] {
			return invalid("provides[%d]: entry %s renders two services", i, p.Entry)
		}
		entries[p.Entry] = true
		if p.RequiresSDK != "" && !sdkRangePattern.MatchString(p.RequiresSDK) {
			return invalid("provides[%d]: requires_sdk %q is not ^MAJOR.MINOR", i, p.RequiresSDK)
		}
		if p.Priority < 0 || p.Priority > maxPriority {
			return invalid("provides[%d]: priority must be 0 to %d", i, maxPriority)
		}
	}
	return nil
}

// surfaceProvides is the provides list as the behaviour surface hashes it,
// sorted by service; nil when the package provides nothing, so every earlier
// hash holds.
func (m Manifest) surfaceProvides() []Provide {
	if len(m.Provides) == 0 {
		return nil
	}
	out := append([]Provide{}, m.Provides...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Service < out[j-1].Service; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
