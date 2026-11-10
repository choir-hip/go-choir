package platform

import (
	"context"
	"errors"
	"testing"
)

// A fork of a story answers it: corpusd derives responds_to from the verified
// parents, so the proxy never has to name it.
func TestOpinionForkOfAStoryDerivesRespondsTo(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	op, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 1, nil, ref)
	if err != nil {
		t.Fatal(err)
	}
	feed, err := f.svc.ListOpedsForOwner(context.Background(), "paper-owner", "", 50)
	if err != nil || len(feed.Items) != 1 || feed.Items[0].VersionID != op.PublicationVersionID {
		t.Fatalf("feed = %+v %v", feed.Items, err)
	}
	// A later version keeps the answer; naming another story is refused.
	other := publishAs(t, f.svc, "other-paper", PublishModeAuto, "doc-other", 1, "Other story", "Another story.", nil)
	otherRef := PublishParentRef{PublicationVersionID: other.PublicationVersionID, ManifestHash: other.ManifestHash}
	if _, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 2, &otherRef, otherRef); !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("an op-ed answering a second story = %v, want ErrInvalidParent", err)
	}
	if _, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 2, nil); err != nil {
		t.Fatalf("v2 with no links: %v", err)
	}
}
