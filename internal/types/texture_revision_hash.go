package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// RevisionHashScheme prefixes every revision hash so the hashing scheme is
// versioned. Bump it if the canonical payload below ever changes.
const RevisionHashScheme = "rev1"

// ComputeRevisionHash returns the tamper-evident hash for a revision, chaining
// it to its parent. The hash is taken over a canonical payload of the parent
// hash plus the revision's body, citations, and provenance. Determinism comes
// from a fixed struct field order and from feeding canonical provenance bytes
// (Provenance.CanonicalJSON) as the provenance input. Empty citations/provenance
// normalize to "[]" / "{}" so a revision with no provenance still chains stably.
//
// The genesis revision (no parent) passes parentHash == "". This is the signable
// spine for the versioned-artifact mission: a future signature signs these
// hashes; changing any earlier revision's content changes every later hash.
func ComputeRevisionHash(parentHash, body string, citations, provenance []byte) string {
	citationsJSON := json.RawMessage(citations)
	if len(strings.TrimSpace(string(citations))) == 0 {
		citationsJSON = json.RawMessage("[]")
	}
	provenanceJSON := json.RawMessage(provenance)
	if len(strings.TrimSpace(string(provenance))) == 0 {
		provenanceJSON = json.RawMessage("{}")
	}
	payload := struct {
		Scheme     string          `json:"scheme"`
		ParentHash string          `json:"parent_hash"`
		Body       string          `json:"body"`
		Citations  json.RawMessage `json:"citations"`
		Provenance json.RawMessage `json:"provenance"`
	}{
		Scheme:     RevisionHashScheme,
		ParentHash: parentHash,
		Body:       body,
		Citations:  citationsJSON,
		Provenance: provenanceJSON,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		// Fields are all JSON-safe; marshal cannot realistically fail. Fall back
		// to hashing the body alone rather than panicking on a canonical path.
		sum := sha256.Sum256([]byte(body))
		return RevisionHashScheme + ":" + hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(data)
	return RevisionHashScheme + ":" + hex.EncodeToString(sum[:])
}
