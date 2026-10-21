package vmmanager

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/mirrorgrant"
)

// Capsule package mirrors, host side (station FP10,
// docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md). The
// right to point a computer's L2 capsules at go-choir-pkgmirror is a grant
// vmctl writes onto the computer's root-only credential disk at cold boot.
// Off while the computer list or the ecosystem list is empty
// (nix/node-b.nix, go-choir-vmctl Environment):
//
//	CHOIR_CAPSULE_MIRROR_COMPUTERS   ComputerIDs and/or class:test
//	CHOIR_CAPSULE_MIRROR_ECOSYSTEMS  npm, pypi, crates, git
//	CHOIR_CAPSULE_MIRRORS_ONLY       1/true: also take the mirrored
//	                                 registries off the CONNECT allowlist
//
// An ecosystem must also be on in the host unit's -ecosystems, or the
// capsule's tools are pointed at a prefix that answers 404. A snapshot
// resume keeps the old credential disk, so a change reaches a computer at
// its next cold boot.
const (
	capsuleMirrorComputersEnv  = "CHOIR_CAPSULE_MIRROR_COMPUTERS"
	capsuleMirrorEcosystemsEnv = "CHOIR_CAPSULE_MIRROR_ECOSYSTEMS"
	capsuleMirrorsOnlyEnv      = "CHOIR_CAPSULE_MIRRORS_ONLY"
)

// CapsuleMirrorConfig is the host switch.
type CapsuleMirrorConfig struct {
	Computers   map[string]bool
	ClassTest   bool
	Ecosystems  []string
	MirrorsOnly bool
}

func loadCapsuleMirrorConfig(getenv func(string) string) CapsuleMirrorConfig {
	cfg := CapsuleMirrorConfig{Computers: map[string]bool{}, Ecosystems: mirrorgrant.ParseEcosystems(getenv(capsuleMirrorEcosystemsEnv))}
	for _, entry := range strings.FieldsFunc(getenv(capsuleMirrorComputersEnv), func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		if entry == mirrorgrant.ClassTestToken {
			cfg.ClassTest = true
			continue
		}
		cfg.Computers[entry] = true
	}
	switch strings.ToLower(strings.TrimSpace(getenv(capsuleMirrorsOnlyEnv))) {
	case "1", "true", "yes", "on":
		cfg.MirrorsOnly = true
	}
	return cfg
}

// capsuleMirrorMaterial is the grant a computer's credential disk carries,
// or nothing for a computer the switch does not name or when no ecosystem
// is on. warmnessClass is the class vmctl booted the computer under.
func (c CapsuleMirrorConfig) capsuleMirrorMaterial(computerID, warmnessClass string, now time.Time) (map[string][]byte, string) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || len(c.Ecosystems) == 0 {
		return nil, ""
	}
	basis := ""
	switch {
	case c.Computers[computerID]:
		basis = mirrorgrant.BasisComputer
	case c.ClassTest && strings.TrimSpace(warmnessClass) == "test":
		basis = mirrorgrant.BasisClassTest
	default:
		return nil, ""
	}
	raw, _ := json.Marshal(mirrorgrant.Grant{
		ComputerID: computerID, Status: mirrorgrant.GrantGranted, Basis: basis,
		Ecosystems: c.Ecosystems, MirrorsOnly: c.MirrorsOnly, GrantedAt: now.UTC().Format(time.RFC3339),
	})
	status := "granted (" + basis + ") " + strings.Join(c.Ecosystems, ",")
	if c.MirrorsOnly {
		status += ", mirrors only"
	}
	return map[string][]byte{mirrorgrant.GrantFile: append(raw, '\n')}, status
}
