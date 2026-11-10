package platform

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestReadPublicationVersionRefusesTombstonedAndUnpublished(t *testing.T) {
	ctx := context.Background()
	svc, _ := newVersionTestService(t)
	og := svc.ogStore()

	// A tombstoned version.
	v1 := publishRevision(t, svc, 1, "rev-a", "Harbor opens.")
	found, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.publication_version_id", v1.PublicationVersionID, 1)
	if err != nil || len(found) != 1 {
		t.Fatalf("find version: %v %d", err, len(found))
	}
	obj := found[0]
	if _, err := svc.ReadPublicationVersion(ctx, obj.CanonicalID); err != nil {
		t.Fatalf("read before tombstone: %v", err)
	}
	obj.Tombstone = true
	if err := og.PutObject(ctx, obj); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{obj.CanonicalID, v1.PublicationVersionID} {
		if _, err := svc.ReadPublicationVersion(ctx, id); !errors.Is(err, errVersionNotReadable) {
			t.Fatalf("tombstoned version read by %s: err=%v, want errVersionNotReadable", id, err)
		}
	}

	// A version whose publication is no longer published.
	resp, err := svc.PublishTexture(ctx, PublishTextureRequest{
		OwnerID: "owner-2", SourceDocID: "doc-other", SourceRevisionID: "rev-x", RevisionNumber: 1,
		RevisionHash: "revhash-x", Title: "Other", Content: "Other story.", RequestedBy: "owner-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReadPublicationVersion(ctx, resp.PublicationVersionID); err != nil {
		t.Fatalf("read before unpublish: %v", err)
	}
	identity, err := publicationIdentityFor("owner-2", "doc-other")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := og.GetObject(ctx, identity.CanonicalID)
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(pub.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	meta["state"] = "unpublished"
	pub.Metadata, _ = json.Marshal(meta)
	if err := og.PutObject(ctx, pub); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReadPublicationVersion(ctx, resp.PublicationVersionID); !errors.Is(err, errVersionNotReadable) {
		t.Fatalf("unpublished version: err=%v, want errVersionNotReadable", err)
	}
}
