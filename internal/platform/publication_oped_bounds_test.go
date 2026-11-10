package platform

// Bounds on what one reader can cause (review of lane C2, B1 and B2):
//  - a republished fork notifies the story's owner once, not once per version;
//  - one publisher's new notices a day are capped (contract D11);
//  - the paper's response never cites an op-ed its own gate rejected.

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestRepublishUpdatesTheNoticeAndNeverRenotifies(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	for n := 1; n <= 4; n++ {
		if _, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", n, nil, ref); err != nil {
			t.Fatalf("v%d: %v", n, err)
		}
	}
	feed, err := f.svc.ListOpedsForOwner(context.Background(), "paper-owner", "", 50)
	if err != nil || len(feed.Items) != 1 {
		t.Fatalf("four versions of one fork gave %d feed items (%v), want 1", len(feed.Items), err)
	}
	notices, _ := f.svc.ListNoticesForOwner(context.Background(), "paper-owner", "", 50)
	if len(notices.Items) != 1 {
		t.Fatalf("notices = %d, want 1", len(notices.Items))
	}
}

func TestSpamLinkOneNoticePerChild(t *testing.T) {
	f := newOpedFixture(t)
	var refs []PublishParentRef
	for i := 0; i < noticesPerPublisherPerDay+4; i++ {
		s := publishAs(t, f.svc, fmt.Sprintf("paper-%d", i), PublishModeAuto, fmt.Sprintf("doc-s%d", i), 1, "Story", fmt.Sprintf("Story number %d.", i), nil)
		refs = append(refs, PublishParentRef{PublicationVersionID: s.PublicationVersionID, ManifestHash: s.ManifestHash})
	}
	// Three owner-mode documents of eight parents each: the publisher's new notices stop at the cap.
	for d := 0; d < 3; d++ {
		lo := d * maxPublishParents
		hi := lo + maxPublishParents
		if hi > len(refs) {
			hi = len(refs)
		}
		if lo >= hi {
			break
		}
		if _, err := f.svc.PublishTexture(context.Background(), PublishTextureRequest{
			OwnerID: "spammer", SourceDocID: fmt.Sprintf("doc-spam-%d", d), SourceRevisionID: fmt.Sprintf("doc-spam-%d-rev", d), RevisionNumber: 1,
			Title: "Notes", Content: "Some notes.", PublishMode: PublishModeOwner, Parents: refs[lo:hi],
		}); err != nil {
			t.Fatal(err)
		}
	}
	total := 0
	for i := range refs {
		n, _ := f.svc.ListNoticesForOwner(context.Background(), fmt.Sprintf("paper-%d", i), "", 50)
		total += len(n.Items)
	}
	if total != noticesPerPublisherPerDay {
		t.Fatalf("one publisher caused %d notices, want the cap %d", total, noticesPerPublisherPerDay)
	}
}

func TestPaperResponseNeverCitesARejectedOped(t *testing.T) {
	f := newOpedFixture(t)
	ref := f.ref()
	op, err := f.publishOped(t, "reader-1", "writer-aa11bb", "doc-fork", 1, nil, ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.PostOpedVerdict(context.Background(), OpedVerdictPost{OwnerID: "paper-owner", VersionID: op.PublicationVersionID,
		Run: 1, Verdict: "rejected_abusive", VerdictHash: "h", TapeRecordID: "r"}); err != nil {
		t.Fatal(err)
	}
	opRef := PublishParentRef{PublicationVersionID: op.PublicationVersionID, ManifestHash: op.ManifestHash}
	_, err = f.svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "paper-owner", SourceDocID: "doc-story", SourceRevisionID: "doc-story-rev-2", RevisionNumber: 2,
		Title: "Harbor reopens", Content: "A response.", PublishMode: PublishModeAuto, Parents: []PublishParentRef{opRef},
	})
	if !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("citing a rejected op-ed = %v, want ErrInvalidParent", err)
	}
	// An accepted later run lifts it.
	if _, err := f.svc.PostOpedVerdict(context.Background(), OpedVerdictPost{OwnerID: "paper-owner", VersionID: op.PublicationVersionID,
		Run: 2, Verdict: "accepted", VerdictHash: "h2", TapeRecordID: "r2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "paper-owner", SourceDocID: "doc-story", SourceRevisionID: "doc-story-rev-2", RevisionNumber: 2,
		Title: "Harbor reopens", Content: "A response.", PublishMode: PublishModeAuto, Parents: []PublishParentRef{opRef},
	}); err != nil {
		t.Fatalf("citing an accepted op-ed: %v", err)
	}
}
