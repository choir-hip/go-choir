package vocabmigrate

// Fence contracts: V1-active passes every frozen V1 token (aliases
// included), stays-live, and frozen protocol, refusing V2-only names and
// unknowns; V2-active passes exactly the live set plus frozen protocol,
// refusing V1 aliases so no unmigrated row holds post-cutover authority;
// unknown active vocabularies fail closed; empty values skip.

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

func TestFenceV1PassesHistoricSuite(t *testing.T) {
	v1tokens := []string{
		"super", "co-super", "cosuper", "coagent", "co-agent",
		"co_super", "cosuper_coding", "co-super-coding", "engineering",
		"researcher", "researchers", "research", "research-agent",
		"web-research", "web-researcher",
		"texture", "conductor", "processor", "reconciler", "email",
		"verifier", "owner", "trusted-core", "",
	}
	fields := make([]Field, 0, len(v1tokens))
	for i, tok := range v1tokens {
		fields = append(fields, Field{Key: string(rune('a' + i)), Value: tok})
	}
	if err := VerifyServingVocabulary(VocabularyV1, fields...); err != nil {
		t.Fatalf("fence refused historic suite under v1: %v", err)
	}
}

func TestFenceV1RefusesV2NamesAndUnknowns(t *testing.T) {
	for _, tok := range []string{"management", "root", "admin", "vapor"} {
		err := VerifyServingVocabulary(VocabularyV1, Field{Key: "run.agent_profile", Value: tok})
		if err == nil {
			t.Errorf("fence passed %q under v1", tok)
		}
	}
}

func TestFenceV2PassesLiveSetRefusesAliases(t *testing.T) {
	live := []string{"management", "engineering", "research", "texture",
		"conductor", "processor", "reconciler", "email", "verifier",
		"owner", "trusted-core"}
	for _, tok := range live {
		if err := VerifyServingVocabulary(VocabularyV2, Field{Key: "k", Value: tok}); err != nil {
			t.Errorf("fence refused live %q under v2: %v", tok, err)
		}
	}
	// V1 aliases must refuse under v2 even though they share spellings with
	// history: the row stamp, not the token, decides authority.
	for _, tok := range []string{"super", "co-super", "cosuper", "coagent",
		"researcher", "researchers", "engineering", "co_super", "boss"} {
		if tok == "engineering" || tok == "research" {
			continue // V2 canonicals, covered above
		}
		if err := VerifyServingVocabulary(VocabularyV2, Field{Key: "k", Value: tok}); err == nil {
			t.Errorf("fence passed V1 alias %q under v2", tok)
		}
	}
	// research is both V1 alias and V2 canonical: accepted as live name.
	if err := VerifyServingVocabulary(VocabularyV2, Field{Key: "k", Value: "research"}); err != nil {
		t.Errorf("fence refused V2 canonical research: %v", err)
	}
}

func TestFenceRejectsUnknownVocabulary(t *testing.T) {
	if err := VerifyServingVocabulary("v3", Field{Key: "k", Value: "super"}); err == nil {
		t.Error("fence accepted unknown active vocabulary")
	}
	if err := VerifyServingVocabulary("", Field{Key: "k", Value: "super"}); err == nil {
		t.Error("fence accepted empty active vocabulary")
	}
}

func TestEventActorFieldsProjection(t *testing.T) {
	fields := EventActorFields("co-super")
	if len(fields) != 1 || fields[0].Key != "event.actor_profile" || fields[0].Value != "co-super" {
		t.Fatalf("actor projection = %+v", fields)
	}
	if err := VerifyServingVocabulary(VocabularyV1, fields...); err != nil {
		t.Fatalf("fence refused V1 actor: %v", err)
	}
}

func TestVocabularySelectorsMatchSeam(t *testing.T) {
	if VocabularyV1 != projectionbase.CurrentVocabularyVersion {
		t.Fatalf("fence V1 selector %q != seam %q", VocabularyV1, projectionbase.CurrentVocabularyVersion)
	}
	if !projectionbase.IsKnownVocabularyVersion(VocabularyV2) {
		t.Fatalf("fence V2 selector %q not in widened known set", VocabularyV2)
	}
}
