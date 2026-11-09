package vocabmigrate

// Fence contracts: V1-active passes every frozen V1 token (aliases
// included), stays-live, and frozen protocol, refusing V2-only names and
// unknowns; V2-active passes exactly the live set plus frozen protocol,
// refusing V1 aliases so no unmigrated row holds post-cutover authority;
// unknown active vocabularies fail closed; empty values skip.

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
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

// A registered package desk's role serves under v2, so its agent object can
// persist; an unregistered app: name, or a retired one, is still refused
// (docs/definitions/choir-modular-architecture-2026-11-04.ledger.md,
// 2026-11-09).
func TestServingFenceAdmitsRegisteredPackageDeskRoles(t *testing.T) {
	const name = "app:fenceprobe"
	field := Field{Key: "og:choir.agent:metadata.role", Value: name}
	if err := VerifyServingVocabulary(VocabularyV2, field); err == nil {
		t.Fatal("an unregistered package desk role passed the fence")
	}
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: name, Verbs: []string{"Report", "Done"}, Taint: deskmanifest.TaintUntrustedInput, UID: 62950, WorkerMaxBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	if err := VerifyServingVocabulary(VocabularyV2, field); err != nil {
		t.Fatalf("a registered package desk role was refused: %v", err)
	}
	if err := VerifyServingVocabulary(VocabularyV2, Field{Key: "role", Value: "app:fenceprobe:owner-1"}); err == nil {
		t.Fatal("an inbox address passed as a role")
	}
	if err := deskmanifest.RetireApp(name); err != nil {
		t.Fatal(err)
	}
	if err := VerifyServingVocabulary(VocabularyV2, field); err == nil {
		t.Fatal("a retired package desk role passed the fence")
	}
}
