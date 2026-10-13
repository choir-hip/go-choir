package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
	"github.com/yusefmosiah/go-choir/internal/workitem"
)

func wireCanonicalRevisionEligibleForPublication(doc types.Document, rev types.Revision, rec *types.RunRecord) bool {
	return wirepublish.EligibleForAutonomousPublish(doc, rev, rec, universalWirePlatformOwnerID())
}

func (rt *Runtime) MaybeAutonomousPublishWireArticle(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) {
	if rt == nil || !wireCanonicalRevisionEligibleForPublication(doc, rev, rec) {
		return
	}
	if rec == nil || strings.TrimSpace(trajectoryIDForRun(rec)) == "" {
		log.Printf("runtime: wire publication doc=%s rev=%s: run without trajectory_id, skipping publication", doc.DocID, rev.RevisionID)
		return
	}
	publicationItemID, err := rt.beginWirePublicationWorkItem(ctx, doc, rev, rec)
	if err != nil {
		log.Printf("runtime: wire publication work item doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	publicationSucceeded := false
	defer func() {
		if publicationSucceeded {
			return
		}
		if _, err := rt.cancelTrajectoryAuthority(context.WithoutCancel(ctx), rec.OwnerID, trajectoryIDForRun(rec)); err != nil {
			log.Printf("runtime: wire publication failure cleanup doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		}
	}()
	platformResp, err := rt.publishWireArticleToPlatform(ctx, doc, rev, rec)
	if err != nil {
		log.Printf("runtime: wire platform publish doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	if err := rt.persistWirePlatformPublicationRef(ctx, doc.OwnerID, rev, platformResp); err != nil {
		log.Printf("runtime: wire publication ref doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	if err := rt.recordWirePublicationTrajectoryRef(ctx, rec, "publish_ref", wirePublicationTrajectoryRef(platformResp)); err != nil {
		log.Printf("runtime: wire publication trajectory ref doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	if err := rt.recordWirePublicationTrajectoryRef(ctx, rec, "edition_ref", wireEditionTrajectoryRef(platformResp)); err != nil {
		log.Printf("runtime: wire feed authority trajectory ref doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	if err := rt.completeWirePublicationWorkItem(ctx, rec, publicationItemID); err != nil {
		log.Printf("runtime: wire publication work item complete doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	if err := rt.completeWireStoryResolutionWorkItem(ctx, rec, doc.DocID); err != nil {
		log.Printf("runtime: wire story resolution work item complete doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	if err := rt.settleWirePublicationTrajectoryIfReady(ctx, rec); err != nil {
		log.Printf("runtime: wire publication settle doc=%s rev=%s: %v", doc.DocID, rev.RevisionID, err)
		return
	}
	publicationSucceeded = true
}

func (rt *Runtime) recordWirePublicationTrajectoryRef(ctx context.Context, rec *types.RunRecord, key, value string) error {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	value = strings.TrimSpace(value)
	if ownerID == "" || trajectoryID == "" {
		return fmt.Errorf("wire publication trajectory identity unavailable")
	}
	if value == "" {
		return fmt.Errorf("wire publication trajectory ref %q is empty", key)
	}
	_, err := rt.store.UpdateTrajectorySubjectRefs(ctx, ownerID, trajectoryID, map[string]string{key: value})
	if err != nil {
		return fmt.Errorf("patch trajectory %s subject refs: %w", trajectoryID, err)
	}
	return nil
}

func wirePublicationTrajectoryRef(pub *wirepublish.PublishTextureResponse) string {
	if pub == nil {
		return ""
	}
	publicationID := strings.TrimSpace(pub.PublicationID)
	publicationVersionID := strings.TrimSpace(pub.PublicationVersionID)
	routePath := strings.TrimSpace(pub.RoutePath)
	switch {
	case publicationID != "" && publicationVersionID != "":
		return "corpusd_publication:" + publicationID + "/" + publicationVersionID
	case publicationID != "":
		return "corpusd_publication:" + publicationID
	case routePath != "":
		return "corpusd_route:" + routePath
	default:
		return ""
	}
}

func wireEditionTrajectoryRef(pub *wirepublish.PublishTextureResponse) string {
	if pub == nil {
		return ""
	}
	routePath := strings.TrimSpace(pub.RoutePath)
	if routePath == "" {
		return ""
	}
	return "corpusd_route:" + routePath
}

func (rt *Runtime) completeWireStoryResolutionWorkItem(ctx context.Context, rec *types.RunRecord, docID string) error {
	if rt == nil || rt.store == nil || rec == nil {
		return fmt.Errorf("wire story resolution work item complete: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	docID = strings.TrimSpace(docID)
	if ownerID == "" || trajectoryID == "" || docID == "" {
		return fmt.Errorf("wire story resolution work item complete: identity unavailable")
	}
	item, found, err := rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, workitem.StoryResolutionFingerprint(trajectoryID, docID))
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if item.Status == types.WorkItemCompleted {
		return nil
	}
	_, err = rt.store.UpdateWorkItemStatus(ctx, ownerID, item.WorkItemID, types.WorkItemCompleted)
	return err
}

func (rt *Runtime) beginWirePublicationWorkItem(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) (string, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return "", fmt.Errorf("wire publication work item: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if ownerID == "" || trajectoryID == "" {
		return "", fmt.Errorf("wire publication work item: trajectory identity unavailable")
	}
	item, err := rt.store.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              ownerID,
		TrajectoryID:         trajectoryID,
		Objective:            "publish wire article revision and link it into the edition",
		Reason:               "autonomous wire publication",
		ObjectiveFingerprint: workitem.PublicationFingerprint(trajectoryID, rev.RevisionID),
		CreatedByRunID:       rec.RunID,
		Details: map[string]any{
			"kind":        "wire_publication",
			"doc_id":      strings.TrimSpace(doc.DocID),
			"revision_id": strings.TrimSpace(rev.RevisionID),
		},
	})
	if err != nil {
		return "", err
	}
	return item.WorkItemID, nil
}

func (rt *Runtime) completeWirePublicationWorkItem(ctx context.Context, rec *types.RunRecord, workItemID string) error {
	if rt == nil || rt.store == nil || rec == nil {
		return fmt.Errorf("wire publication work item complete: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	workItemID = strings.TrimSpace(workItemID)
	if ownerID == "" || workItemID == "" {
		return fmt.Errorf("wire publication work item complete: identity unavailable")
	}
	_, err := rt.store.UpdateWorkItemStatus(ctx, ownerID, workItemID, types.WorkItemCompleted)
	return err
}

func (rt *Runtime) settleWirePublicationTrajectoryIfReady(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil {
		return fmt.Errorf("wire publication settle: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if ownerID == "" || trajectoryID == "" {
		return fmt.Errorf("wire publication settle: identity unavailable")
	}
	obligations, err := rt.TrajectoryObligations(ctx, ownerID, trajectoryID)
	if err != nil {
		return err
	}
	if obligations.Trajectory.Status != types.TrajectoryLive || !obligations.SettlementReady {
		return nil
	}
	_, err = rt.store.UpdateTrajectoryStatus(ctx, ownerID, trajectoryID, types.TrajectorySettled)
	return err
}

// wireDetailKeyResolutionState is the work-item detail key the retired
// processor pipeline wrote; stored wire work items are still read by it.
const wireDetailKeyResolutionState = "resolution_state"
