package trust_test

// The catalogue completeness tests of MA7.1 (failure modes 1 and 2 of
// impact_test.go), in the external test package: the lease service imports
// trust for its policy (MA7.4), so an internal test of trust cannot import
// lease.

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/trust"
)

func TestEveryCatalogueGrantHasAnImpactRow(t *testing.T) {
	for _, g := range apppackage.GrantCatalogueRows() {
		name := g.Name
		if g.Family {
			name += ".x"
		}
		if _, ok := trust.GrantImpact(name); !ok {
			t.Errorf("manifest grant %q has no impact row", name)
		}
	}
	for _, name := range lease.GrantNames() {
		if _, ok := trust.GrantImpact(name); !ok {
			t.Errorf("first-party grant %q has no impact row", name)
		}
	}
}

func TestEveryImpactRowNamesARealGrant(t *testing.T) {
	real := map[string]bool{}
	for _, g := range apppackage.GrantCatalogueRows() {
		real[g.Name] = true
	}
	for _, name := range lease.GrantNames() {
		real[name] = true
	}
	for _, r := range trust.ImpactTable().Grants {
		if !real[r.Grant] {
			t.Errorf("impact row %q names no catalogue grant", r.Grant)
		}
		if _, ok := r.Impact.Rank(); !ok || r.Impact == "" {
			t.Errorf("impact row %q has class %q", r.Grant, r.Impact)
		}
	}
}

// The lease policy's action classes are rows of the table.
func TestTheLeasePolicyClassesHaveImpactRows(t *testing.T) {
	for _, name := range []string{lease.ClassReadKind, lease.ClassReadCaptures} {
		if _, ok := trust.GrantImpact(name); !ok {
			t.Errorf("lease policy class %q has no impact row", name)
		}
	}
}
