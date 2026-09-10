package computerevent

// Landing step 3 contracts: the frozen V1 decoder interprets history under
// frozen rules, never mutates digest-bearing bytes, and both roots still
// accept V1 (aliases live until the writer cutover).

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFrozenV1CanonicalTable(t *testing.T) {
	cases := map[string]string{
		"super":    "super",
		"co-super": "co-super", "cosuper": "co-super",
		"coagent": "co-super", "co-agent": "co-super",
		"researcher": "researcher", "researchers": "researcher",
		"research": "researcher", "research-agent": "researcher",
		"web-research": "researcher", "web-researcher": "researcher",
		"texture": "texture", "conductor": "conductor",
		"processor": "processor", "reconciler": "reconciler",
		"email": "email", "verifier": "verifier",
		"owner": "owner", "trusted-core": "trusted-core",
		" co_super ": "co-super",
	}
	for token, want := range cases {
		got, ok := CanonicalActorProfile(token)
		if !ok || got != want {
			t.Errorf("CanonicalActorProfile(%q) = (%q, %v), want (%q, true)", token, got, ok, want)
		}
	}
	for _, unknown := range []string{"", "management", "engineering", "root", "admin"} {
		if _, ok := CanonicalActorProfile(unknown); ok {
			t.Errorf("CanonicalActorProfile(%q) known, want unknown", unknown)
		}
	}
}

func TestFrozenPerFunctionTablesNeverUnioned(t *testing.T) {
	// cosuper_coding is NormalizeRole-only: Canonical must not know it.
	if _, ok := CanonicalActorProfileAs("canonical", "cosuper_coding"); ok {
		t.Error("canonical table knows cosuper_coding, tables unioned")
	}
	if got, ok := CanonicalActorProfileAs("normalize", "cosuper_coding"); !ok || got != "co-super" {
		t.Errorf("normalize table cosuper_coding = (%q, %v), want (co-super, true)", got, ok)
	}
	if _, ok := CanonicalActorProfileAs("bogus", "super"); ok {
		t.Error("unknown table name accepted")
	}
}

func TestResolveVocabularyDefaultsV1(t *testing.T) {
	for _, marker := range []string{"", "v1", " v1 "} {
		got, err := ResolveVocabulary(marker)
		if err != nil || got != VocabularyV1 {
			t.Errorf("ResolveVocabulary(%q) = (%q, %v), want (v1, nil)", marker, got, err)
		}
	}
	if got, err := ResolveVocabulary("v2"); err != nil || got != VocabularyV2 {
		t.Errorf("ResolveVocabulary(v2) = (%q, %v), want (v2, nil)", got, err)
	}
	if _, err := ResolveVocabulary("v3"); err == nil {
		t.Error("ResolveVocabulary(v3) accepted, want fail-closed")
	}
}

func TestHistoricDecodePreservesBytes(t *testing.T) {
	eventID, err := NewEventID()
	if err != nil {
		t.Fatalf("fixture event id: %v", err)
	}
	want := Event{
		SchemaVersion: SchemaVersionV1, ReducerVersion: ReducerVersionV1,
		EventID: eventID, ComputerID: "computer-test", Sequence: 7,
		PreviousHead: ZeroHead, EventKind: EventResearcherUpdate,
		OccurredAt:     time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey: "fixture-1", RequestCommitment: testDigestC,
		ActorProfile: "co-super", AuthorityRef: "owner",
		PayloadCommitment:                testDigestA,
		PrivacyClass:                     "owner",
		ExpectedDesiredEventHead:         ZeroHead,
		ExpectedEffectiveEventHead:       ZeroHead,
		ExpectedDesiredStateCommitment:   ZeroHead,
		ExpectedEffectiveStateCommitment: ZeroHead,
	}
	raw, err := want.CanonicalBytes()
	if err != nil {
		t.Fatalf("fixture canonical bytes: %v", err)
	}
	before := append([]byte(nil), raw...)
	event, err := DecodeHistoricEvent(raw)
	if err != nil {
		t.Fatalf("historic decode refused V1 event: %v", err)
	}
	if event.ActorProfile != "co-super" || event.Sequence != 7 || event.EventID != eventID {
		t.Fatal("historic decode misread the V1 event")
	}
	recanonical, err := event.CanonicalBytes()
	if err != nil {
		t.Fatalf("decoded event recanonicalize: %v", err)
	}
	if string(recanonical) != string(raw) {
		t.Fatal("historic decode is not byte-identical under recanonicalization")
	}
	if string(raw) != string(before) {
		t.Fatal("historic decode mutated input bytes")
	}
	again, err := DecodeHistoricEvent(raw)
	if err != nil || !reflect.DeepEqual(again, event) {
		t.Fatal("historic decode is not deterministic over preserved bytes")
	}
	if got, ok := CanonicalActorProfile(event.ActorProfile); !ok || got != "co-super" {
		t.Fatalf("frozen V1 view of decoded profile = (%q, %v)", got, ok)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("decoded V1 event fails Validate: %v", err)
	}
}

func TestHistoricDecodeRefusesMalformed(t *testing.T) {
	for _, raw := range [][]byte{nil, {}, []byte("{"), []byte(`"str" paddle`)} {
		if _, err := DecodeHistoricEvent(raw); err == nil {
			t.Errorf("historic decode accepted %q", string(raw))
		}
	}
}

func TestAdmissionDecodeAcceptsV1Today(t *testing.T) {
	// Pre-cutover contract: aliases still live, so admission matches historic
	// semantics. The cutover flips this root to V2-only; this test then flips
	raw := `{"event":{"actor_profile":"researcher"},"transition_input":{}}`
	request, err := DecodeAdmissionCASRequest(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("admission decode refused V1 request: %v", err)
	}
	if request.Event.ActorProfile != "researcher" {
		t.Fatalf("admission decode misread profile %q", request.Event.ActorProfile)
	}
}

func TestAdmissionDecodeRefusesUnknownFields(t *testing.T) {
	raw := `{"event":{},"bogus_field":1}`
	if _, err := DecodeAdmissionCASRequest(strings.NewReader(raw)); err == nil {
		t.Error("admission decode accepted unknown field")
	}
}
