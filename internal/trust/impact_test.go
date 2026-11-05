package trust

// Failure modes (MA7.1, written first):
//  1. a catalogue grant (manifest or first-party lease grant) with no row
//     would run as an unclassed act: the completeness tests fail it.
//  2. a row whose class is not I0..I4 or a grant the catalogue lacks.
//  3. the table edited without a version bump: the digest pin fails.
//  4. a provider call classed I3 again: the not-impact test fails it.
//  5. a family grant (desk.selfdev.x) must resolve to its family's row.
//  6. an agent lowering a kind's class: ClassOf takes the higher of the
//     grants' table class and the frozen class.

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestEveryCatalogueGrantHasAnImpactRow(t *testing.T) {
	for _, g := range apppackage.GrantCatalogueRows() {
		name := g.Name
		if g.Family {
			name += ".x"
		}
		if _, ok := GrantImpact(name); !ok {
			t.Errorf("manifest grant %q has no impact row", name)
		}
	}
	for _, name := range lease.GrantNames() {
		if _, ok := GrantImpact(name); !ok {
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
	for _, r := range ImpactTable().Grants {
		if !real[r.Grant] {
			t.Errorf("impact row %q names no catalogue grant", r.Grant)
		}
		if _, ok := r.Impact.Rank(); !ok || r.Impact == "" {
			t.Errorf("impact row %q has class %q", r.Grant, r.Impact)
		}
	}
}

func TestLevelAndImpactAgree(t *testing.T) {
	// a privileged grant is never below I2; a default one never above I1.
	for _, g := range apppackage.GrantCatalogueRows() {
		name := g.Name
		if g.Family {
			name += ".x"
		}
		c, _ := GrantImpact(name)
		rank, _ := c.Rank()
		switch g.Level {
		case apppackage.LevelDefault:
			if rank > 1 {
				t.Errorf("default-level grant %s is %s", name, c)
			}
		}
		if g.Level == apppackage.LevelPrivileged && g.Name != "desk.run.deadline" && g.Name != "desk.world.read" && rank < 2 {
			t.Errorf("privileged grant %s is only %s", name, c)
		}
	}
}

func TestFamilyGrantResolves(t *testing.T) {
	c, ok := GrantImpact("desk.selfdev.apply")
	if !ok || c != types.ImpactI4 {
		t.Fatalf("desk.selfdev.apply = %v %v", c, ok)
	}
	if _, ok := GrantImpact("desk.selfdev"); ok {
		t.Fatal("the bare family name is not a grant")
	}
	if _, ok := GrantImpact("desk.nonsense"); ok {
		t.Fatal("unknown grant resolved")
	}
}

func TestProviderCallIsNotAnImpactClass(t *testing.T) {
	tb := ImpactTable()
	found := false
	for _, n := range tb.NotImpact {
		if n.Name == "model.provider_call" {
			found = true
		}
	}
	if !found {
		t.Fatal("the table must name the provider call as logged, not classed")
	}
	if _, ok := GrantImpact("model.provider_call"); ok {
		t.Fatal("a provider call must not resolve to an impact class")
	}
	for _, c := range tb.Classes {
		if c.Class == types.ImpactI3 && containsFold(c.Examples, "provider call") {
			t.Fatalf("I3 examples still list a provider call: %q", c.Examples)
		}
	}
}

func TestImpactNumbersAndTau(t *testing.T) {
	want := map[types.ImpactClass]float64{types.ImpactI0: 0, types.ImpactI1: 1, types.ImpactI2: 3, types.ImpactI3: 10, types.ImpactI4: 30}
	for c, w := range want {
		if got := ImpactNumber(c); got != w {
			t.Errorf("impact(%s) = %v, want %v", c, got, w)
		}
	}
	if _, ok := Tau(types.ImpactI0); ok {
		t.Error("I0 has no tau")
	}
	if tau, ok := Tau(types.ImpactI3); !ok || tau != 0.95 {
		t.Errorf("tau(I3) = %v %v", tau, ok)
	}
}

func TestImpactTableDigestIsPinned(t *testing.T) {
	if got := ImpactTableDigest(); got != PinnedImpactTableDigest {
		t.Fatalf("the impact table changed (digest %s, pinned %s): bump the version in impact_table.json and the pin together", got, PinnedImpactTableDigest)
	}
}

func TestRequestKindFloorsFitTheTable(t *testing.T) {
	// every request kind's catalogue floor is at most the class of the
	// grant that sends it.
	reqClass, _ := GrantImpact("desk.request")
	rr, _ := reqClass.Rank()
	for _, e := range types.MaterialityCatalogue {
		fr, _ := e.ImpactFloor.Rank()
		if fr > rr {
			t.Errorf("kind %s floors at %s, above desk.request %s", e.Kind, e.ImpactFloor, reqClass)
		}
	}
}

func TestClassOfTakesTheHigherOfGrantsAndFrozenClass(t *testing.T) {
	rec := types.CommitmentRecord{
		Action:               &types.CommitmentAction{Intent: "x", DeclaredCapabilities: []string{"desk.act", "desk.net.egress"}},
		MaterialityProcedure: &types.MaterialityFreeze{ImpactClass: types.ImpactI1},
	}
	if got := ClassOf(rec); got.Impact != types.ImpactI3 || got.Grant != "desk.net.egress" {
		t.Fatalf("declared egress: %+v", got)
	}
	// an agent that states I0 for a capsule mutation is still I4
	rec = types.CommitmentRecord{
		Action:      &types.CommitmentAction{DeclaredCapabilities: []string{"desk.capsule.mutate"}},
		ImpactClass: types.ImpactI0,
	}
	if got := ClassOf(rec); got.Impact != types.ImpactI4 {
		t.Fatalf("capsule mutate stated I0: %+v", got)
	}
	// no declared grant: a cast is desk.request at its frozen class
	rec = types.CommitmentRecord{ImpactClass: types.ImpactI2, Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast}, Kind: types.CommitmentKindDirective}
	if got := ClassOf(rec); got.Grant != "desk.request" || got.Impact != types.ImpactI2 {
		t.Fatalf("cast: %+v", got)
	}
	if got := ClassOf(rec).String(); got != "desk.request/I2" {
		t.Fatalf("string %s", got)
	}
}
