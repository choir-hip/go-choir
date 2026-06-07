package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestGlobalWireExistingArticleVTextBodyRepair(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ownerID := "global-wire-repair-user"

	stories, err := s.ListGlobalWireStories(ctx, ownerID)
	if err != nil {
		t.Fatalf("seed global wire stories: %v", err)
	}
	if len(stories) == 0 {
		t.Fatal("seeded no global wire stories")
	}
	story := stories[0]
	doc, err := s.GetDocument(ctx, story.StoryVTextDoc, ownerID)
	if err != nil {
		t.Fatalf("get seeded story document: %v", err)
	}

	stale := strings.Join([]string{
		"# " + story.Headline,
		"",
		"Style source: Style.vtext: Global Wire Story id: " + story.ID,
		"",
		"Projection",
		"",
		"Claims",
		"",
		"Source Manifest",
		"",
		"Related VTexts",
		"",
		"Non-oracle note",
		"",
		"My Edit",
	}, "\n")
	if err := s.CreateRevision(ctx, types.Revision{
		RevisionID:       "stale-global-wire-head",
		DocID:            doc.DocID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "Global Wire",
		Content:          stale,
		Citations:        json.RawMessage(`[]`),
		Metadata:         json.RawMessage(`{"created_from":"old_global_wire_projection"}`),
		ParentRevisionID: doc.CurrentRevisionID,
		CreatedAt:        time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create stale head: %v", err)
	}

	if _, err := s.ListGlobalWireStories(ctx, ownerID); err != nil {
		t.Fatalf("repair global wire stories: %v", err)
	}
	repairedDoc, err := s.GetDocument(ctx, doc.DocID, ownerID)
	if err != nil {
		t.Fatalf("get repaired story document: %v", err)
	}
	repaired, err := s.GetRevision(ctx, repairedDoc.CurrentRevisionID, ownerID)
	if err != nil {
		t.Fatalf("get repaired head: %v", err)
	}
	if repaired.ParentRevisionID != "stale-global-wire-head" {
		t.Fatalf("repaired parent = %q, want stale head", repaired.ParentRevisionID)
	}
	for _, forbidden := range []string{
		"Style source:",
		"Projection\n",
		"Claims\n",
		"Source Manifest",
		"Related VTexts",
		"Non-oracle note",
		"My Edit",
	} {
		if strings.Contains(repaired.Content, forbidden) {
			t.Fatalf("repaired content still contains %q:\n%s", forbidden, repaired.Content)
		}
	}
	if !strings.Contains(repaired.Content, "source:gw-src-") {
		t.Fatalf("repaired content has no native source refs:\n%s", repaired.Content)
	}
	var meta map[string]any
	if err := json.Unmarshal(repaired.Metadata, &meta); err != nil {
		t.Fatalf("unmarshal repaired metadata: %v", err)
	}
	if meta["created_from"] != "global_wire_article_body_repair" {
		t.Fatalf("created_from = %v, want repair metadata", meta["created_from"])
	}
	if meta["repaired_from_revision_id"] != "stale-global-wire-head" {
		t.Fatalf("repaired_from_revision_id = %v, want stale head", meta["repaired_from_revision_id"])
	}

	before, err := s.CountRevisionsByDoc(ctx, doc.DocID, ownerID)
	if err != nil {
		t.Fatalf("count revisions before idempotency check: %v", err)
	}
	if _, err := s.ListGlobalWireStories(ctx, ownerID); err != nil {
		t.Fatalf("rerun repair: %v", err)
	}
	after, err := s.CountRevisionsByDoc(ctx, doc.DocID, ownerID)
	if err != nil {
		t.Fatalf("count revisions after idempotency check: %v", err)
	}
	if after != before {
		t.Fatalf("repair not idempotent: revisions before=%d after=%d", before, after)
	}
}
