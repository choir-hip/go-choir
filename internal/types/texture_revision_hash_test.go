package types

import (
	"strings"
	"testing"
)

func TestComputeRevisionHashDeterministic(t *testing.T) {
	h1 := ComputeRevisionHash("", "hello body", []byte("[]"), []byte("{}"))
	h2 := ComputeRevisionHash("", "hello body", []byte("[]"), []byte("{}"))
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %q vs %q", h1, h2)
	}
	if !strings.HasPrefix(h1, RevisionHashScheme+":") {
		t.Fatalf("hash missing scheme prefix: %q", h1)
	}
}

func TestComputeRevisionHashEmptyNormalizesEqual(t *testing.T) {
	withEmpty := ComputeRevisionHash("", "body", nil, nil)
	withDefaults := ComputeRevisionHash("", "body", []byte("[]"), []byte("{}"))
	if withEmpty != withDefaults {
		t.Fatalf("empty citations/provenance did not normalize: %q vs %q", withEmpty, withDefaults)
	}
}

func TestComputeRevisionHashTamperDetected(t *testing.T) {
	base := ComputeRevisionHash("", "original body", []byte("[]"), []byte("{}"))
	tamperedBody := ComputeRevisionHash("", "tampered body", []byte("[]"), []byte("{}"))
	if base == tamperedBody {
		t.Fatalf("body change did not change hash")
	}
	tamperedProv := ComputeRevisionHash("", "original body", []byte("[]"), []byte(`{"schema_version":1}`))
	if base == tamperedProv {
		t.Fatalf("provenance change did not change hash")
	}
}

func TestComputeRevisionHashChainsToParent(t *testing.T) {
	v0 := ComputeRevisionHash("", "v0 body", []byte("[]"), []byte("{}"))
	v1 := ComputeRevisionHash(v0, "v1 body", []byte("[]"), []byte("{}"))

	// Tampering with v0's body changes v0's hash, which changes v1's hash when
	// recomputed: the chain is tamper-evident end to end.
	v0Tampered := ComputeRevisionHash("", "v0 body tampered", []byte("[]"), []byte("{}"))
	v1FromTampered := ComputeRevisionHash(v0Tampered, "v1 body", []byte("[]"), []byte("{}"))
	if v1 == v1FromTampered {
		t.Fatalf("parent tamper did not propagate to child hash")
	}
	if v0 == v1 {
		t.Fatalf("distinct revisions produced identical hashes")
	}
}

func TestComputeStructuredRevisionHashSignsBodyDocAndSourceEntities(t *testing.T) {
	bodyDoc := []byte(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"doc"},"content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"body"}]}]}}`)
	otherBodyDoc := []byte(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"doc"},"content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"changed"}]}]}}`)
	sourceEntities := []byte(`[]`)
	otherSourceEntities := []byte(`[{"source_entity_id":"src-1"}]`)

	base := ComputeStructuredRevisionHash("", "body", bodyDoc, sourceEntities, []byte("{}"))
	if !strings.HasPrefix(base, StructuredRevisionHashScheme+":") {
		t.Fatalf("structured hash missing scheme prefix: %q", base)
	}
	if base != ComputeStructuredRevisionHash("", "body", bodyDoc, sourceEntities, []byte("{}")) {
		t.Fatalf("structured hash not deterministic")
	}
	if base == ComputeStructuredRevisionHash("", "body", otherBodyDoc, sourceEntities, []byte("{}")) {
		t.Fatalf("body_doc change did not change structured hash")
	}
	if base == ComputeStructuredRevisionHash("", "body", bodyDoc, otherSourceEntities, []byte("{}")) {
		t.Fatalf("source_entities change did not change structured hash")
	}
	if base == ComputeStructuredRevisionHash("parent", "body", bodyDoc, sourceEntities, []byte("{}")) {
		t.Fatalf("parent hash change did not change structured hash")
	}
}
