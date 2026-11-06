package apppackage

import (
	"reflect"
	"testing"
)

// L1 failure modes, written before the code:
//
//	L1a a pack (themes, commands, keybindings, no code) is not kind pack, T0;
//	L1b an app with a window is not kind app, T1, or loses its opens facet;
//	L1c a desk-bearing package is not an app with the agent facet;
//	L1d a provider is not kind provider, T2, with its slot named;
//	L1e the ceiling drops a grant, loses its arguments or level, or its order
//	    depends on the manifest's;
//	L1f a package that asks for nothing has a nil ceiling (the Library must
//	    tell "asks for nothing" from "not recorded").
func TestLibraryFactsPerExampleKind(t *testing.T) {
	cases := []struct {
		name   string
		raw    []byte
		kind   PackageKind
		tier   Tier
		slots  []string
		facets []string
	}{
		{"pack", packJSON(t, nil), PackageKindPack, Tier0, []string{}, []string{FacetCommands, FacetKeybindings, FacetThemes}},
		{"app", v2JSON(t, func(map[string]any) {}), PackageKindApp, Tier1, []string{}, []string{FacetKinds, FacetOpens}},
		{"app+desk", v2JSON(t, deskV), PackageKindApp, Tier1, []string{}, []string{FacetAgent, FacetKinds}},
		{"desk", v2JSON(t, func(v map[string]any) { delete(v, "app"); setOpens(v, []any{}); deskV(v) }), PackageKindApp, Tier1, []string{}, []string{FacetAgent, FacetKinds}},
		{"provider", v2JSON(t, providerV), PackageKindProvider, Tier2, []string{"launcher"}, []string{FacetKinds}},
	}
	// The fixture declares data kinds, so every case but the pack has the
	// kinds facet.
	for _, c := range cases {
		m, err := ParseManifest(c.raw, OriginFirstParty)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if m.PackageKind() != c.kind || m.Tier() != c.tier || !reflect.DeepEqual(m.Slots(), c.slots) || !reflect.DeepEqual(m.Facets(), c.facets) {
			t.Errorf("%s: kind %s tier %s slots %v facets %v", c.name, m.PackageKind(), m.Tier(), m.Slots(), m.Facets())
		}
		if m.Ceiling() == nil {
			t.Errorf("L1f %s: nil ceiling", c.name)
		}
	}
}

func TestLibraryCeilingCarriesEveryGrantWithItsLevelSorted(t *testing.T) {
	m, err := ParseManifest(v2JSON(t, withCaps(
		Capability{Grant: "desk.read.kind", Args: []string{"video_asset"}},
		Capability{Grant: "desk.act"},
		Capability{Grant: "desk.request", Args: []string{"mail", "files"}},
	)), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	want := []CeilingGrant{
		{Grant: "desk.act", Level: LevelDefault},
		{Grant: "desk.read.kind", Args: []string{"video_asset"}, Level: LevelExtended},
		{Grant: "desk.request", Args: []string{"files", "mail"}, Level: LevelDefault},
	}
	if got := m.Ceiling(); !reflect.DeepEqual(got, want) {
		t.Fatalf("L1e ceiling = %+v", got)
	}
	if m.Level() != LevelExtended {
		t.Fatalf("level = %s", m.Level())
	}
}
