package types

import "testing"

// Entity identity (docs/problems/related-stories-no-entity-layer-2026-10-26.md,
// slice a; ontology docs/reports/claims-events-entities-ontology-2026-10-14.md
// §4.2 and §5.2, D1 and D5). R1 (entity sprawl, phase 3 design §6): names
// that differ only in case, punctuation, a possessive, a leading article or a
// company suffix are one entity, so they normalise alike and mint one id.

func TestNormalizeEntityNameFoldsTheSurfaceFormsOfOneName(t *testing.T) {
	same := [][]string{
		{"Anthropic", "Anthropic PBC", "Anthropic's", "ANTHROPIC", "Anthropic, Inc.", "anthropic’s"},
		{"Aleksandar Vučić", "Aleksandar  Vučić", "aleksandar vučić's"},
		{"The New York Times", "New York Times", "new-york times"},
		{"U.S. Department of Justice", "US Department of Justice"},
	}
	for _, group := range same {
		want := NormalizeEntityName(group[0])
		if want == "" {
			t.Fatalf("%q normalises to nothing", group[0])
		}
		for _, name := range group[1:] {
			if got := NormalizeEntityName(name); got != want {
				t.Errorf("NormalizeEntityName(%q) = %q, want %q (as %q)", name, got, want, group[0])
			}
			if MintedEntityID(name) != MintedEntityID(group[0]) {
				t.Errorf("MintedEntityID(%q) differs from MintedEntityID(%q)", name, group[0])
			}
		}
	}
	for _, pair := range [][2]string{{"Anthropic", "OpenAI"}, {"Joe Biden", "Biden"}, {"Georgia", "Georgia Tech"}} {
		if NormalizeEntityName(pair[0]) == NormalizeEntityName(pair[1]) {
			t.Errorf("%q and %q normalise alike", pair[0], pair[1])
		}
	}
	for _, empty := range []string{"", "  ", "The", "Inc.", "'s"} {
		if got := NormalizeEntityName(empty); got != "" {
			t.Errorf("NormalizeEntityName(%q) = %q, want empty", empty, got)
		}
		if got := MintedEntityID(empty); got != "" {
			t.Errorf("MintedEntityID(%q) = %q, want empty", empty, got)
		}
	}
}

func TestEntityIDsAreMintedOrWikidata(t *testing.T) {
	minted := MintedEntityID("Anthropic")
	if !EntityIDRE.MatchString(minted) || minted[:4] != "ent:" {
		t.Fatalf("MintedEntityID = %q, want ent:<24 hex>", minted)
	}
	if got := WikidataEntityID("Q312"); got != "wd:Q312" || !EntityIDRE.MatchString(got) {
		t.Fatalf("WikidataEntityID = %q", got)
	}
	for _, bad := range []string{"", "ent:", "ent:xyz", "wd:312", "wd:Q0", "Q312", "obj:choir.entity:x"} {
		if EntityIDRE.MatchString(bad) {
			t.Errorf("EntityIDRE accepts %q", bad)
		}
	}
}
