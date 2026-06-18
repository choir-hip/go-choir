package types

import (
	"bytes"
	"testing"
	"time"
)

func TestProvenanceCanonicalJSONDeterministic(t *testing.T) {
	at := time.Date(2026, 6, 18, 14, 0, 0, 0, time.UTC)
	base := Provenance{
		SchemaVersion:  ProvenanceSchemaVersion,
		AuthoringModel: ProvenanceModel{Provider: "fireworks", Model: "test-model"},
		AuthoredAt:     at,
		QueriesExecuted: []ProvenanceQuery{
			{Tool: "web_search", Query: "first query", ResultCount: 3},
			{Tool: "web_search", Query: "second query"},
		},
		Sources: []SourceEntity{
			{EntityID: "src_bbbb", Kind: "content_item"},
			{EntityID: "src_aaaa", Kind: "youtube_video"},
		},
	}

	got1, err := base.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	got2, err := base.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON (second): %v", err)
	}
	if !bytes.Equal(got1, got2) {
		t.Fatalf("CanonicalJSON not stable across calls:\n%s\n%s", got1, got2)
	}
}

func TestProvenanceCanonicalJSONSourceOrderIndependent(t *testing.T) {
	at := time.Date(2026, 6, 18, 14, 0, 0, 0, time.UTC)
	a := Provenance{
		SchemaVersion: ProvenanceSchemaVersion,
		AuthoredAt:    at,
		Sources: []SourceEntity{
			{EntityID: "src_aaaa", Kind: "content_item"},
			{EntityID: "src_bbbb", Kind: "youtube_video"},
		},
	}
	b := Provenance{
		SchemaVersion: ProvenanceSchemaVersion,
		AuthoredAt:    at,
		Sources: []SourceEntity{
			{EntityID: "src_bbbb", Kind: "youtube_video"},
			{EntityID: "src_aaaa", Kind: "content_item"},
		},
	}
	ab, err := a.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON a: %v", err)
	}
	bb, err := b.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON b: %v", err)
	}
	if !bytes.Equal(ab, bb) {
		t.Fatalf("source order changed canonical bytes:\n%s\n%s", ab, bb)
	}
}

func TestProvenanceCanonicalJSONDoesNotMutateReceiver(t *testing.T) {
	p := Provenance{
		Sources: []SourceEntity{
			{EntityID: "src_zzzz"},
			{EntityID: "src_aaaa"},
		},
	}
	if _, err := p.CanonicalJSON(); err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	if p.Sources[0].EntityID != "src_zzzz" {
		t.Fatalf("CanonicalJSON mutated receiver source order: got %q first", p.Sources[0].EntityID)
	}
}
