package store

import (
	"context"
	"testing"
)

func TestListObjectGraphFingerprintsReadsRows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES ('obj:b', 'run', 'o', 'c', 'v1', 'h-b', 'body-b', '{}', NOW(), NOW(), FALSE, ''),
		       ('obj:a', 'work_item', 'o', 'c', '', 'h-a', NULL, '{"k":1}', NOW(), NOW(), TRUE, 'obj:z')`); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListObjectGraphFingerprints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("got %d rows", len(got))
	}
	var a, b *ObjectGraphFingerprint
	for i := range got {
		switch got[i].CanonicalID {
		case "obj:a":
			a = &got[i]
		case "obj:b":
			b = &got[i]
		}
	}
	if a == nil || b == nil {
		t.Fatalf("rows missing: %+v", got)
	}
	if a.ObjectKind != "work_item" || !a.Tombstone || a.SupersededBy != "obj:z" || a.MetadataDigest == b.MetadataDigest || a.BodyDigest == b.BodyDigest || b.UpdatedAt == "" {
		t.Fatalf("fingerprints = %+v / %+v", *a, *b)
	}
}
