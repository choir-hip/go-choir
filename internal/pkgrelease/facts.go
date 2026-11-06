package pkgrelease

import (
	"encoding/json"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

// ListingFacts are what the Library shows about a release beside its record
// (slice L1, docs/design/modular-architecture-plan-2026-11-04.md section 11):
// the kind, the facets, the slots, the tier, the publishing level and the
// ceiling. The host derives them at publish from the manifest bytes it
// verified (VerifyHostFacts) and keeps them in the release object's metadata,
// beside the description: the record format is unchanged, so a revert is a
// true revert and no publisher can claim a fact.
//
// Install never reads them: the installing computer derives its own from the
// manifest it builds (install review reads the ceiling there). They are a
// display of the same derivation, not an authority.
type ListingFacts struct {
	Kind   apppackage.PackageKind `json:"kind"`
	Tier   apppackage.Tier        `json:"tier"`
	Level  apppackage.Level       `json:"level"`
	Slots  []string               `json:"slots"`
	Facets []string               `json:"facets"`
	// Ceiling is every grant the package may ever request, with its level.
	Ceiling []apppackage.CeilingGrant `json:"ceiling"`
	// Recorded is true when the host kept the facts at publish. A release
	// published before L1 has its kind, tier and level read from the record's
	// manifest summary; its facets, slots and ceiling were not kept.
	Recorded bool `json:"recorded"`
}

// MetadataFactsKey is the release object's metadata key the facts live under.
const MetadataFactsKey = "facts"

// FactsOf derives the listing facts of a parsed manifest.
func FactsOf(m apppackage.Manifest) ListingFacts {
	return ListingFacts{
		Kind: m.PackageKind(), Tier: m.Tier(), Level: m.Level(),
		Slots: m.Slots(), Facets: m.Facets(), Ceiling: m.Ceiling(), Recorded: true,
	}
}

// FactsFromSummary reads what a release published before L1 still says: the
// record's manifest summary holds the class and the level the host derived.
func FactsFromSummary(s Manifest) ListingFacts {
	f := ListingFacts{Level: apppackage.Level(s.Level), Slots: []string{}, Facets: []string{}, Ceiling: []apppackage.CeilingGrant{}}
	switch apppackage.Class(s.Class) {
	case apppackage.ClassProvider:
		f.Kind, f.Tier = apppackage.PackageKindProvider, apppackage.Tier2
	case apppackage.ClassApp, apppackage.ClassAppDesk, apppackage.ClassDesk:
		f.Kind, f.Tier = apppackage.PackageKindApp, apppackage.Tier1
	default:
		f.Kind, f.Tier = apppackage.PackageKindPack, apppackage.Tier0
	}
	if apppackage.ClassDesk == apppackage.Class(s.Class) || apppackage.ClassAppDesk == apppackage.Class(s.Class) {
		f.Facets = []string{apppackage.FacetAgent}
	}
	return f
}

// FactsFromMetadata reads the facts the host kept in a release object's
// metadata, falling back to the record's summary for a release published
// before L1 (or metadata that does not decode as facts).
func FactsFromMetadata(metadata json.RawMessage, summary Manifest) ListingFacts {
	var meta struct {
		Facts *ListingFacts `json:"facts"`
	}
	if err := json.Unmarshal(metadata, &meta); err == nil && meta.Facts != nil && meta.Facts.Kind != "" && meta.Facts.Tier != "" {
		f := *meta.Facts
		if f.Slots == nil {
			f.Slots = []string{}
		}
		if f.Facets == nil {
			f.Facets = []string{}
		}
		if f.Ceiling == nil {
			f.Ceiling = []apppackage.CeilingGrant{}
		}
		f.Recorded = true
		return f
	}
	return FactsFromSummary(summary)
}
