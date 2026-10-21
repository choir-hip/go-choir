// Package mirrorgrant is the per-computer switch that points a computer's
// L2 capsules at the host package mirror (station FP10,
// docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md). vmctl
// writes the grant onto a named computer's root-only credential disk at
// cold boot (internal/vmmanager/capsule_mirrors.go); the guest's capsule
// executor reads it when it starts a capsule's egress
// (internal/capsule/mirror.go). No grant, no change: capsules keep the
// egress they had.
package mirrorgrant

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// GrantFile is the grant's name on the credential disk.
	GrantFile = "capsule-mirrors.json"
	// GrantGranted is the only status that turns mirrors on.
	GrantGranted = "granted"
	// ClassTestToken in the host list grants every test computer.
	ClassTestToken = "class:test"
	// BasisComputer and BasisClassTest say why a computer was granted.
	BasisComputer  = "computer"
	BasisClassTest = ClassTestToken
)

// The ecosystems the host mirror serves (internal/pkgmirror).
const (
	NPM    = "npm"
	PyPI   = "pypi"
	Crates = "crates"
	Git    = "git"
)

var known = map[string]bool{NPM: true, PyPI: true, Crates: true, Git: true}

// Grant is the grant record.
type Grant struct {
	ComputerID string   `json:"computer_id"`
	Status     string   `json:"status"`
	Basis      string   `json:"basis"`
	Ecosystems []string `json:"ecosystems"`
	// MirrorsOnly takes the mirrored registries off the capsule's CONNECT
	// allowlist.
	MirrorsOnly bool   `json:"mirrors_only"`
	GrantedAt   string `json:"granted_at"`
}

// Status is the switch as this computer reads it.
type Status struct {
	Granted     bool     `json:"granted"`
	Reason      string   `json:"reason"`
	Ecosystems  []string `json:"ecosystems,omitempty"`
	MirrorsOnly bool     `json:"mirrors_only,omitempty"`
}

// ParseEcosystems reads a comma- or space-separated list, keeping the
// known ecosystems once each, sorted.
func ParseEcosystems(s string) []string {
	return clean(strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }))
}

func clean(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, e := range in {
		if e = strings.TrimSpace(e); known[e] && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

// ReadGrant reads the grant from the credential directory. Anything but a
// well-formed granted record naming a known ecosystem is off. Reasons never
// quote file contents.
func ReadGrant(dir string) Status {
	if strings.TrimSpace(dir) == "" {
		return Status{Reason: "no credential directory is configured"}
	}
	raw, err := os.ReadFile(filepath.Join(dir, GrantFile))
	if errors.Is(err, os.ErrNotExist) {
		return Status{Reason: "this computer is not granted the host package mirror (no grant on the credential disk)"}
	}
	if err != nil {
		return Status{Reason: "the package mirror grant is unreadable"}
	}
	var g Grant
	if err := json.Unmarshal(raw, &g); err != nil {
		return Status{Reason: "the package mirror grant is malformed"}
	}
	if g.Status != GrantGranted {
		return Status{Reason: "the host withheld the package mirror"}
	}
	ecos := clean(g.Ecosystems)
	if len(ecos) == 0 {
		return Status{Reason: "the package mirror grant names no known ecosystem"}
	}
	return Status{Granted: true, Ecosystems: ecos, MirrorsOnly: g.MirrorsOnly, Reason: "granted (" + g.Basis + ")"}
}
