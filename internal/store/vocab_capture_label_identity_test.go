package store

import (
	"context"
	"testing"
	"time"
)

// The paper refused to boot on 2026-10-16 (12:49 UTC): a capture label
// override whose free text held "-super-" (a news headline) looked to the
// vocabulary migration like a migrated identity field, and the kind had no
// identity formula, so startup failed. Capture labels and their overrides
// are keyed by computer, capture, version (and record): fields no desk
// vocabulary feeds. Failure modes pinned: the migration refuses the boot;
// the override's id changes; it can no longer be read by its capture.
func TestVocabMigrationKeepsCaptureLabelOverridesWithDeskWordsInText(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	label, err := s.AppendCaptureLabel(ctx, CaptureLabel{OwnerID: "owner", ComputerID: "computer", CaptureID: "obj:choir.web_capture:abc:key-1", LabelledAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	override := CaptureLabelOverride{
		OwnerID: "owner", ComputerID: "computer", CaptureID: label.CaptureID, LabelVersion: label.Version,
		CaptureTitle: "Anti-super-PAC donors regroup", Headline: "Ex-super-majority leader returns",
		JevSaid: "decline", Disposition: "opened", RecordID: "cell:run-1:call-1:cast:tray-1", Reason: "a multi-super-region story",
	}
	if err := s.PutCaptureLabelOverride(ctx, override); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id FROM og_objects WHERE object_kind='choir.capture_label_override'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MigrateAndFenceServingVocabulary(ctx, false, nil); err != nil {
		t.Fatalf("vocabulary migration refused a capture label override: %v", err)
	}
	var after string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id FROM og_objects WHERE object_kind='choir.capture_label_override'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("override id changed across the migration: %s -> %s", before, after)
	}
}
