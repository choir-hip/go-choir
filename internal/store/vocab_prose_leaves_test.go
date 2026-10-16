package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The V1→V2 vocabulary rules rewrite desk tokens and desk-bearing IDs, never
// prose or URLs (problems/live-writes-refused-by-deposit-upcast-prose-leaves-2026-10-16.md).
// On 2026-10-16 the paper's live writes were refused (commitment record,
// source schedule, event membership) and its 12:49 boot was refused, because
// a title or URL holding "-super-" read as a migrated identity. Ways this can
// fail, written before the fix:
//
//   - a leaf with interior whitespace (a title, a reason) is rewritten;
//   - a URL whose slug holds "-super-" is rewritten;
//   - a kind without an identity formula is refused for such a leaf, on the
//     replay or live deposit path or in the boot rescan;
//   - the fix overreaches and genuine V1 identities ("super:root",
//     "run-super-assignment-1", the token "super") stop migrating.
func TestVocabLeafMigrateLeavesProseAndURLs(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		changed  bool
	}{
		{"Ex-super-majority leader returns", "Ex-super-majority leader returns", false},
		{"Egypt's Climate-Resilient Super-Corals", "Egypt's Climate-Resilient Super-Corals", false},
		{"a\tmulti-super-region\nstory", "a\tmulti-super-region\nstory", false},
		{"https://www.wired.com/story/egypts-climate-resilient-super-corals-are-starting-to-feel-the-heat", "https://www.wired.com/story/egypts-climate-resilient-super-corals-are-starting-to-feel-the-heat", false},
		{"super:root", "management:root", true},
		{"run-super-assignment-1", "run-management-assignment-1", true},
		{"super", "management", true},
	} {
		got, changed := ogLeafMigrate(tc.in)
		if got != tc.want || changed != tc.changed {
			t.Errorf("ogLeafMigrate(%q) = (%q, %v), want (%q, %v)", tc.in, got, changed, tc.want, tc.changed)
		}
	}
}

func vocabProseMembership(t *testing.T, created time.Time) objectgraph.Object {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"capture_id": "obj:choir.web_capture:abc:key-1",
		"event_id":   "ev-f93eddc1cb36cf551467",
		"url":        "https://www.wired.com/story/egypts-climate-resilient-super-corals-are-starting-to-feel-the-heat",
		"title":      "Ex-super-majority leader returns",
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{"event_id": "ev-f93eddc1cb36cf551467"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := objectgraph.BuildCanonicalID("choir.event_membership", vocabUpcastOwnerID, objectgraph.StableSuffixFromKey("membership-of-a-wired-capture"))
	if err != nil {
		t.Fatal(err)
	}
	return objectgraph.Object{
		CanonicalID: id, ObjectKind: "choir.event_membership", OwnerID: vocabUpcastOwnerID,
		ComputerID: vocabUpcastComputerID, ContentHash: objectgraph.ContentHash("choir.event_membership", body, metadata),
		Body: body, Metadata: metadata, CreatedAt: created, UpdatedAt: created,
	}
}

func TestVocabDepositUpcastKeepsProseLeavesOfKindsWithoutFormula(t *testing.T) {
	ctx := context.Background()
	s, _ := vocabUpcastOpenStore(t, "prose")
	base := time.Date(2026, 10, 16, 15, 39, 0, 0, time.UTC)

	genesisEvent, genesisInput, genesisNext := vocabUpcastTestEvent(t, 1, computerevent.ZeroHead, computerevent.EventGenesisImported, nil)
	vocabUpcastApplyTapeEvent(t, s, vocabUpcastTapeEvent{
		Event: genesisEvent, Input: genesisInput, Next: genesisNext, Receipt: vocabUpcastReceipt(t, genesisEvent),
	})

	obj := vocabProseMembership(t, base)
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
		t.Fatalf("replay refused an event membership whose URL and title hold -super-: %v", err)
	}
	var gotID, gotBody string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id, body FROM og_objects WHERE object_kind='choir.event_membership'`).Scan(&gotID, &gotBody); err != nil {
		t.Fatal(err)
	}
	if gotID != obj.CanonicalID {
		t.Fatalf("membership id changed: %s -> %s", obj.CanonicalID, gotID)
	}
	if gotBody != string(obj.Body) || strings.Contains(gotBody, "-management-") {
		t.Fatalf("membership body rewritten:\n got %s\nwant %s", gotBody, obj.Body)
	}
}

func TestVocabBootRescanKeepsProseLeavesOfKindsWithoutFormula(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	base := time.Date(2026, 10, 16, 9, 5, 0, 0, time.UTC)
	obj := vocabProseMembership(t, base)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?,?,?,?,?,?,?,?,?,?,0,'')`,
		obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID, obj.VersionID,
		obj.ContentHash, obj.Body, string(obj.Metadata), base, base); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	// replayed=true forces the full rescan, as the paper's 12:49 boot did.
	if _, err := s.MigrateAndFenceServingVocabulary(ctx, true, nil); err != nil {
		t.Fatalf("boot rescan refused an event membership whose URL and title hold -super-: %v", err)
	}
	var gotID, gotBody string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id, body FROM og_objects WHERE object_kind='choir.event_membership'`).Scan(&gotID, &gotBody); err != nil {
		t.Fatal(err)
	}
	if gotID != obj.CanonicalID || gotBody != string(obj.Body) {
		t.Fatalf("boot rescan changed the membership: id %s -> %s, body %s", obj.CanonicalID, gotID, gotBody)
	}
}
