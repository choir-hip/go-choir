package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Corpus search reads stories (L2a R2-8). A story is a lifecycle document:
// the legacy revision getter refuses its head, so a search that reads heads
// through it never matches a story's content.
func TestSearchDocumentsReadsAStoryHead(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	receipt := mustCommitInbox(t, s, "cell-records:search-1", nil, openingCast("search-1:cast:c1"))
	docID := strings.TrimPrefix(receipt.Records[0].Inbox, "texture:")
	doc, err := s.GetLifecycleDocument(ctx, storyOwner, storyComputer, docID)
	if err != nil {
		t.Fatalf("story document: %v", err)
	}
	if _, err := s.GetTextureRevisionOG(ctx, storyOwner, doc.CurrentRevisionID); !errors.Is(err, ErrLifecycleAuthorityRequired) {
		t.Fatalf("legacy getter on a story head = %v; the fixture no longer exercises the lifecycle path", err)
	}
	rev, err := s.searchHeadRevision(ctx, doc)
	if err != nil || rev.RevisionID != doc.CurrentRevisionID {
		t.Fatalf("search head = %+v, %v; want the story's seed %s", rev.RevisionID, err, doc.CurrentRevisionID)
	}
	for _, owner := range []string{storyOwner, ""} {
		results, err := s.SearchDocuments(ctx, "harbor storm", owner, 10)
		if err != nil || len(results) != 1 || results[0].DocID != docID {
			t.Fatalf("search (owner %q) = %+v, %v; want the story %s", owner, results, err, docID)
		}
	}
	if results, _ := s.SearchDocuments(ctx, "harbor", "someone-else", 10); len(results) != 0 {
		t.Fatalf("search found another owner's story: %+v", results)
	}
}
