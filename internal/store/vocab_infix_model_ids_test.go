package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The infix rule (-co-super-, -cosuper-, -researcher-, -super-) rewrites
// constructed V1 desk ids, never model ids, paths or slugs
// (problems/vocab-infix-rewrite-mangles-model-ids-2026-11-05.md). On
// 2026-11-05 the paper asked the gateway for
// "nvidia/nemotron-3.5-management-vl:free", a model that does not exist,
// because the run record's llm_model leaf was upcast. Failure modes pinned:
// a model id (with or without "/"), a file name or a slug is rewritten; a
// live writer holding one is not serving vocabulary; a replayed deposit
// holding one is rewritten; a genuine V1 constructed id stops migrating.
func TestVocabInfixRuleLeavesModelIDsPathsAndSlugs(t *testing.T) {
	for _, v := range []string{
		"nvidia/nemotron-3.5-super-vl:free",
		"openrouter/nvidia/nemotron-3.5-super-vl:free",
		"nemotron-3.5-super-vl",
		"acme-super-70b",
		"meta/llama-researcher-8b",
		"vendor/model-cosuper-x",
		"vendor/model-co-super-x",
		"openrouter/a:free,openrouter/nvidia/nemotron-3.5-super-vl:free",
		"notes-super-final.md",
		"reports/q3-researcher-notes.pdf",
		"ex-super-majority-leader-returns",
		"anti-super-pac",
	} {
		if got, changed := ogLeafMigrate(v); changed || got != v {
			t.Errorf("ogLeafMigrate(%q) = (%q, %v), want unchanged", v, got, changed)
		}
		if !IsServingVocabularyLeaf(v) {
			t.Errorf("IsServingVocabularyLeaf(%q) = false, want true", v)
		}
	}
}

func TestVocabInfixRuleStillMigratesV1ConstructedIDs(t *testing.T) {
	// The wants are what the rule produced before it was narrowed: a replay
	// of a V1 deposit must land on the same bytes it always did.
	for in, want := range map[string]string{
		"run-super-assignment-1":                            "run-management-assignment-1",
		"work-super-assignment":                             "work-management-assignment",
		"run-super-1":                                       "run-management-1",
		"run-cosuper-assignment-01":                         "run-engineering-assignment-01",
		"work-cosuper-1":                                    "work-engineering-1",
		"co-super-report:assignment-1:report-1":             "co-management-report:assignment-1:report-1",
		"co-super-open:assignment-1:1":                      "co-management-open:assignment-1:1",
		"co-super-restart-cancel:assignment-1:2":            "co-management-restart-cancel:assignment-1:2",
		"co-super-grant:sha256:0123abcd":                    "co-management-grant:sha256:0123abcd",
		"co-super-fate:sha256:0123abcd":                     "co-management-fate:sha256:0123abcd",
		"queue-super-report:sha256:0123abcd":                "queue-management-report:sha256:0123abcd",
		"persistent-super-recovery:v2:eyJhIjoxfQ":           "persistent-management-recovery:v2:eyJhIjoxfQ",
		"lifecycle-researcher-admission-recovery:v1:eyJhIn": "lifecycle-research-admission-recovery:v1:eyJhIn",
	} {
		got, changed := ogLeafMigrate(in)
		if !changed || got != want {
			t.Errorf("ogLeafMigrate(%q) = (%q, %v), want (%q, true)", in, got, changed, want)
		}
		if IsServingVocabularyLeaf(in) {
			t.Errorf("IsServingVocabularyLeaf(%q) = true for a V1 constructed id", in)
		}
	}
}

func TestVocabDepositUpcastKeepsModelIDLeaves(t *testing.T) {
	ctx := context.Background()
	s, _ := vocabUpcastOpenStore(t, "modelids")
	base := time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)

	genesisEvent, genesisInput, genesisNext := vocabUpcastTestEvent(t, 1, computerevent.ZeroHead, computerevent.EventGenesisImported, nil)
	vocabUpcastApplyTapeEvent(t, s, vocabUpcastTapeEvent{
		Event: genesisEvent, Input: genesisInput, Next: genesisNext, Receipt: vocabUpcastReceipt(t, genesisEvent),
	})

	body, err := json.Marshal(map[string]any{
		"event_id":      "ev-model-ids",
		"llm_model":     "nvidia/nemotron-3.5-super-vl:free",
		"model":         "nvidia/nemotron-3.5-super-vl:free",
		"llm_fallbacks": "openrouter/a:free,openrouter/nvidia/nemotron-3.5-super-vl:free",
		"file":          "notes-super-final.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{"event_id": "ev-model-ids"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := objectgraph.BuildCanonicalID("choir.event_membership", vocabUpcastOwnerID, objectgraph.StableSuffixFromKey("membership-with-model-ids"))
	if err != nil {
		t.Fatal(err)
	}
	obj := objectgraph.Object{
		CanonicalID: id, ObjectKind: "choir.event_membership", OwnerID: vocabUpcastOwnerID,
		ComputerID: vocabUpcastComputerID, ContentHash: objectgraph.ContentHash("choir.event_membership", body, metadata),
		Body: body, Metadata: metadata, CreatedAt: base, UpdatedAt: base,
	}
	event, input, next := vocabUpcastTestEvent(t, 2, genesisNext.CanonicalEventHead, computerevent.EventProjectionBatchRecorded, &genesisNext)
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Prepare(ctx, computerevent.CASRequest{Event: event, EventDigest: digest, EventArtifactDigest: digest, EventPinReceiptDigest: storeTestDigest('c'), Input: input, Next: next}); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: vocabUpcastComputerID, EventID: event.EventID, EventDigest: digest,
		Ops: []computerevent.ProjectionOp{vocabUpcastObjectOp(t, obj)},
	}
	if err := s.FinalizeReplayBatch(ctx, vocabUpcastComputerID, digest, vocabUpcastReceipt(t, event), &batch); err != nil {
		t.Fatalf("replay refused a deposit holding model ids: %v", err)
	}
	var gotID, gotBody string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id, b.body FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id WHERE object_kind='choir.event_membership'`).Scan(&gotID, &gotBody); err != nil {
		t.Fatal(err)
	}
	if gotID != obj.CanonicalID || gotBody != string(obj.Body) {
		t.Fatalf("replay rewrote model-id leaves:\n got %s %s\nwant %s %s", gotID, gotBody, obj.CanonicalID, obj.Body)
	}
	// The boot rescan agrees with the replay: nothing left to migrate.
	if _, err := s.MigrateAndFenceServingVocabulary(ctx, true, nil); err != nil {
		t.Fatalf("boot rescan refused model-id leaves: %v", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT b.body FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id WHERE object_kind='choir.event_membership'`).Scan(&gotBody); err != nil {
		t.Fatal(err)
	}
	if gotBody != string(obj.Body) {
		t.Fatalf("boot rescan rewrote model-id leaves: %s", gotBody)
	}
}
