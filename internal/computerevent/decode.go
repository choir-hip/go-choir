package computerevent

// Frozen V1 vocabulary decode with two centralized decode roots.
//
// Mission-2 landing step 3. The frozen V1 tables below are a deliberately
// duplicated snapshot of the V1 acceptor sets frozen in
// docs/evidence/choir-rlm-v2-mapping-2026-09-10.md §1. They MUST NOT call
// agentprofile.Canonical or modelpolicy.NormalizeRole: those stay live and
// will stop accepting V1 aliases at the writer cutover, while this decoder
// keeps interpreting history under frozen V1 rules forever.
//
// The two roots:
//   - DecodeHistoricEvent / DecodeHistoricDurableEvents: raw-preserving
//     historic decode. Takes digest-bearing bytes, never mutates them, and
//     returns the Event view. Unmarked rows default V1.
//   - DecodeAdmissionCASRequest: V2 write-admission decode for the live
//     append path. V1 aliases are still accepted live at this step, so it
//     behaves like the historic root; the cutover flips it to V2-only.
//     The tape-append gate (AppendNew entrypoints) stays the enforcement
//     point, never Event.Validate (digest path).
//
// Every production Event/CASRequest/DurableEvent decode routes through one
// of these roots, DecodeProjectionBatch (format discriminator, vocabulary
// neutral), or ParseDescriptor (descriptor seam). Tests are the only
// exception. scripts/check-decode-roots.sh enforces this in CI.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// frozenV1Canonical maps every frozen V1 desk token to its canonical V1
// profile constant. Live stays-live profiles map to themselves. Tokens
// outside this table are not V1 desk vocabulary.
var frozenV1Canonical = map[string]string{
	// Management desk.
	"super": "super",
	// Engineering desk.
	"cosuper": "co-super", "co-super": "co-super",
	"coagent": "co-super", "co-agent": "co-super",
	// Research desk.
	"researcher": "researcher", "researchers": "researcher",
	"research": "researcher", "research-agent": "researcher",
	"web-research": "researcher", "web-researcher": "researcher",
	// Stays-live profiles.
	"texture": "texture", "texture-agent": "texture", "document-agent": "texture",
	"conductor":    "conductor",
	"processor":    "processor",
	"reconciler":   "reconciler",
	"email":        "email",
	"verifier":     "verifier",
	"owner":        "owner",
	"trusted-core": "trusted-core",
}

// frozenV1Normalize extends the frozen table with the NormalizeRole-only V1
// extras, keyed in normalized form (underscores already folded to hyphens by
// the lookup). Canonical has no branch for cosuper-coding; the overlapping
// keys (cosuper, co_super, co-super) resolve through the canonical table via
// the fallthrough below, exactly mirroring the frozen §1b acceptor set.
var frozenV1Normalize = map[string]string{
	"cosuper-coding": "co-super",
}

// VocabularyV1 is the frozen historic vocabulary selector. Unmarked tape,
// content, and persistence rows default V1.
const VocabularyV1 = "v1"

// VocabularyV2 is the live vocabulary selector after the writer cutover.
const VocabularyV2 = "v2"

// ResolveVocabulary maps a carrier version marker to the vocabulary selector.
// Empty or unmarked markers default V1; only an explicit v2 marker selects
// the live vocabulary. Unknown markers fail closed.
func ResolveVocabulary(marker string) (string, error) {
	switch strings.TrimSpace(marker) {
	case "", VocabularyV1:
		return VocabularyV1, nil
	case VocabularyV2:
		return VocabularyV2, nil
	default:
		return "", fmt.Errorf("decode: unknown vocabulary marker %q", marker)
	}
}

// CanonicalActorProfile interprets an ActorProfile token under frozen V1
// rules and reports whether the token is known V1 desk vocabulary. It is a
// read-only view for replay verification: it never mutates bytes and never
// authorizes live admission. Callers needing per-function strictness
// (Canonical vs NormalizeRole acceptor sets) use CanonicalActorProfileAs.
func CanonicalActorProfile(token string) (string, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(token), "_", "-"))
	if canonical, ok := frozenV1Canonical[normalized]; ok {
		return canonical, true
	}
	return "", false
}

// CanonicalActorProfileAs interprets token under one frozen per-function
// table: "canonical" or "normalize". The tables are never unioned.
func CanonicalActorProfileAs(table, token string) (string, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(token), "_", "-"))
	switch table {
	case "canonical":
		canonical, ok := frozenV1Canonical[normalized]
		return canonical, ok
	case "normalize":
		if canonical, ok := frozenV1Normalize[normalized]; ok {
			return canonical, true
		}
		return CanonicalActorProfile(token)
	default:
		return "", false
	}
}

// DecodeHistoricEvent decodes raw event bytes under frozen V1 rules without
// mutating them. The input slice is never modified; digest-bearing callers
// keep verifying against their original bytes.
func DecodeHistoricEvent(raw []byte) (Event, error) {
	var event Event
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&event); err != nil {
		return Event{}, fmt.Errorf("historic event decode: %w", err)
	}
	return event, nil
}

// DecodeHistoricDurableEvents decodes a raw replay-page body into durable
// records under frozen V1 rules without mutating the input bytes.
func DecodeHistoricDurableEvents(raw []byte) ([]DurableEvent, error) {
	var page []DurableEvent
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&page); err != nil {
		return nil, fmt.Errorf("historic durable page decode: %w", err)
	}
	return page, nil
}

// DecodeAdmissionCASRequest decodes a live append request body for the V2
// write-admission path. At this landing step V1 aliases are still accepted
// live, so admission decode matches historic semantics; unknown fields are
// refused exactly as the previous inline handler did (no body limit change).
// The cutover flips this root to V2-only vocabulary; Event.Validate itself
// stays digest-compatible and version-neutral.
func DecodeAdmissionCASRequest(body io.Reader) (CASRequest, error) {
	var request CASRequest
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return CASRequest{}, fmt.Errorf("admission request decode: %w", err)
	}
	return request, nil
}
