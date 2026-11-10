package types

import (
	"testing"
	"time"
)

func TestProvenanceWithoutParentLinksKeepsItsBytesAndHash(t *testing.T) {
	// The bytes a revision written before ParentLinks existed was hashed
	// over: the new field is omitempty, so nothing about them changes.
	const old = `{"schema_version":1,"authoring_model":{"provider":"p","model":"m"},"authored_at":"2026-01-02T03:04:05Z"}`
	prov := Provenance{
		SchemaVersion:  ProvenanceSchemaVersion,
		AuthoringModel: ProvenanceModel{Provider: "p", Model: "m"},
		AuthoredAt:     time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	got, err := prov.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != old {
		t.Fatalf("canonical bytes changed:\n got %s\nwant %s", got, old)
	}
	want := ComputeStructuredRevisionHash("parent", "text", []byte(`{}`), nil, []byte(old))
	if hash := ComputeStructuredRevisionHash("parent", "text", []byte(`{}`), nil, got); hash != want {
		t.Fatalf("hash changed: %s != %s", hash, want)
	}
}

func TestRevisionHashCoversParentLinks(t *testing.T) {
	base := Provenance{SchemaVersion: ProvenanceSchemaVersion, ParentLinks: []ParentLink{
		{Kind: ParentLinkSource, PublicationVersionID: "pv-b", ManifestHash: "h-b"},
		{Kind: ParentLinkBase, RevisionID: "rev-1"},
		{Kind: ParentLinkSource, PublicationVersionID: "pv-a", ManifestHash: "h-a"},
	}}
	hashOf := func(p Provenance) string {
		raw, err := p.CanonicalJSON()
		if err != nil {
			t.Fatal(err)
		}
		return ComputeStructuredRevisionHash("parent", "text", []byte(`{}`), nil, raw)
	}
	original := hashOf(base)
	// The order the links were collected in does not change the hash.
	shuffled := base
	shuffled.ParentLinks = []ParentLink{base.ParentLinks[2], base.ParentLinks[0], base.ParentLinks[1]}
	if hashOf(shuffled) != original {
		t.Fatal("link order changed the hash")
	}
	// Any altered link does.
	for name, mutate := range map[string]func(*ParentLink){
		"manifest hash": func(l *ParentLink) { l.ManifestHash = "forged" },
		"version id":    func(l *ParentLink) { l.PublicationVersionID = "pv-other" },
		"kind":          func(l *ParentLink) { l.Kind = ParentLinkBase },
	} {
		altered := base
		altered.ParentLinks = append([]ParentLink(nil), base.ParentLinks...)
		mutate(&altered.ParentLinks[0])
		if hashOf(altered) == original {
			t.Fatalf("altering the %s did not change the hash", name)
		}
	}
	// Dropping every link does.
	none := base
	none.ParentLinks = nil
	if hashOf(none) == original {
		t.Fatal("dropping the links did not change the hash")
	}
}
