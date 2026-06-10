package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

func wireCanonicalRevisionEligibleForPublication(doc types.Document, rev types.Revision, rec *types.RunRecord) bool {
	return wirepublish.EligibleForAutonomousPublish(doc, rev, rec, universalWirePlatformOwnerID())
}

func (rt *Runtime) maybeAutonomousPublishWireArticle(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) {
	if rt == nil || !wireCanonicalRevisionEligibleForPublication(doc, rev, rec) {
		return
	}
	platformResp, err := rt.publishWireArticleToPlatform(ctx, doc, rev, rec)
	if err != nil {
		log.Printf("runtime: wire platform publish doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	if err := rt.persistWirePlatformPublicationRef(ctx, doc.OwnerID, rev, platformResp); err != nil {
		log.Printf("runtime: wire publication ref doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	if err := rt.autonomousPublishWireArticleToEdition(ctx, doc, rev); err != nil {
		log.Printf("runtime: wire edition publish doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	rt.noteWireEligiblePublish(ctx, doc.DocID, rev.RevisionID)
}

func (rt *Runtime) autonomousPublishWireArticleToEdition(ctx context.Context, storyDoc types.Document, storyRev types.Revision) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("runtime store unavailable")
	}
	ownerID := universalWirePlatformOwnerID()
	if strings.TrimSpace(storyDoc.OwnerID) != ownerID {
		return nil
	}
	editionDocID, err := rt.store.GetDocumentAlias(ctx, ownerID, universalWireEditionSourcePath)
	if err != nil {
		if err == store.ErrNotFound {
			return nil
		}
		return fmt.Errorf("resolve wire edition alias: %w", err)
	}
	editionDoc, err := rt.store.GetDocument(ctx, editionDocID, ownerID)
	if err != nil {
		return fmt.Errorf("load wire edition document: %w", err)
	}
	if strings.TrimSpace(editionDoc.CurrentRevisionID) == "" {
		return fmt.Errorf("wire edition document has no current revision")
	}
	editionRev, err := rt.store.GetRevision(ctx, editionDoc.CurrentRevisionID, ownerID)
	if err != nil {
		return fmt.Errorf("load wire edition revision: %w", err)
	}
	included := universalWireEditionIncludedDocIDs(editionRev.Content, editionDoc.DocID)
	if slices.Contains(included, storyDoc.DocID) {
		return nil
	}

	headline := wireArticleArticleHeadline(storyDoc.Title, storyRev.Content)
	if headline == "" {
		headline = strings.TrimSuffix(strings.TrimSpace(storyDoc.Title), ".vtext")
	}
	if headline == "" {
		headline = "Wire article"
	}
	now := time.Now().UTC()
	newContent := strings.TrimRight(editionRev.Content, "\n")
	if newContent != "" {
		newContent += "\n\n"
	}
	newContent += fmt.Sprintf("- [%s](vtext:%s)", headline, storyDoc.DocID)

	editionMeta, _ := json.Marshal(map[string]any{
		"source":        "universal_wire_edition",
		"revision_role": vtextRevisionRoleCanonical,
		"published_doc_ids": append(append([]string(nil), included...), storyDoc.DocID),
	})
	newEditionRev := types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            editionDoc.DocID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "wire_publication_policy",
		Content:          newContent,
		Citations:        json.RawMessage("[]"),
		Metadata:         editionMeta,
		ParentRevisionID: editionRev.RevisionID,
		CreatedAt:        now,
	}
	if err := rt.store.CreateRevision(ctx, newEditionRev); err != nil {
		return fmt.Errorf("create wire edition revision: %w", err)
	}
	editionDoc.CurrentRevisionID = newEditionRev.RevisionID
	editionDoc.UpdatedAt = now
	if err := rt.store.UpdateDocument(ctx, editionDoc); err != nil {
		return fmt.Errorf("update wire edition document head: %w", err)
	}
	return nil
}
