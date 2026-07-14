package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
	"github.com/yusefmosiah/go-choir/internal/workitem"
)

func wireCanonicalRevisionEligibleForPublication(doc types.Document, rev types.Revision, rec *types.RunRecord) bool {
	return wirepublish.EligibleForAutonomousPublish(doc, rev, rec, universalWirePlatformOwnerID())
}

func (rt *Runtime) recoverOpenWirePublicationClaims(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	items, err := rt.store.ListOpenWorkItemsByKind(ctx, "wire_publication", 0)
	if err != nil {
		log.Printf("runtime: recover open wire publication claims: %v", err)
		return
	}
	recovered := make(map[[2]string]struct{}, len(items))
	for _, item := range items {
		ownerID := strings.TrimSpace(item.OwnerID)
		trajectoryID := strings.TrimSpace(item.TrajectoryID)
		key := [2]string{ownerID, trajectoryID}
		if _, ok := recovered[key]; ok {
			continue
		}
		recovered[key] = struct{}{}
		if _, err := rt.cancelTrajectoryAuthority(ctx, ownerID, trajectoryID); err != nil {
			log.Printf("runtime: recover wire publication claim work_item=%s trajectory=%s: %v", item.WorkItemID, trajectoryID, err)
		}
	}
}

func (rt *Runtime) maybeAutonomousPublishWireArticle(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) {
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
	rt.noteWireEligiblePublish(ctx, doc.DocID, rev.RevisionID, rec)
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

func (rt *Runtime) beginWireProcessorDecisionWorkItem(ctx context.Context, rec *types.RunRecord) (string, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return "", fmt.Errorf("wire processor decision work item: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if ownerID == "" || trajectoryID == "" {
		return "", fmt.Errorf("wire processor decision work item: identity unavailable")
	}
	requestID := firstNonEmpty(
		metadataStringValue(rec.Metadata, "ingestion_handoff_request_id"),
		metadataStringValue(rec.Metadata, "source_network_request_id"),
		rec.RunID,
	)
	item, err := rt.store.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              ownerID,
		TrajectoryID:         trajectoryID,
		Objective:            "resolve processor request into explicit publication decisions",
		Reason:               "processor request started without durable per-item decision ledger yet",
		AuthorityProfile:     agentprofile.Processor,
		ObjectiveFingerprint: workitem.ProcessorDecisionFingerprint(trajectoryID),
		CreatedByRunID:       rec.RunID,
		Details: map[string]any{
			"kind":                       "wire_processor_request_resolution",
			"request_id":                 requestID,
			"processor_key":              metadataStringValue(rec.Metadata, runMetadataProcessorKey),
			"source_item_ids":            append([]string(nil), metadataStringSlice(rec.Metadata["source_item_ids"])...),
			"source_count":               metadataIntValue(rec.Metadata, "source_count"),
			"source_item_count":          len(wireProcessorSourceItemIDs(rec)),
			"resolved_source_item_count": 0,
			wireDetailKeyResolutionState: sourceapi.ResolutionStateAwaitingSourceItemDecisions,
			"continuity_ref":             metadataStringValue(rec.Metadata, "continuity_ref"),
			"source_request_id":          metadataStringValue(rec.Metadata, "source_network_request_id"),
		},
	})
	if err != nil {
		return "", err
	}
	return item.WorkItemID, nil
}

func (rt *Runtime) beginWireProcessorSourceDecisionWorkItems(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil {
		return fmt.Errorf("wire processor source decision work items: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if ownerID == "" || trajectoryID == "" {
		return fmt.Errorf("wire processor source decision work items: identity unavailable")
	}
	requestID := firstNonEmpty(
		metadataStringValue(rec.Metadata, "ingestion_handoff_request_id"),
		metadataStringValue(rec.Metadata, "source_network_request_id"),
		rec.RunID,
	)
	for _, sourceItemID := range wireProcessorSourceItemIDs(rec) {
		if _, err := rt.store.CreateWorkItem(ctx, types.WorkItemRecord{
			OwnerID:              ownerID,
			TrajectoryID:         trajectoryID,
			Objective:            "resolve source item into explicit publication decision",
			Reason:               "processor request started with source item awaiting typed publication verdict",
			AuthorityProfile:     agentprofile.Processor,
			ObjectiveFingerprint: workitem.SourceItemDecisionFingerprint(trajectoryID, sourceItemID),
			CreatedByRunID:       rec.RunID,
			Details: map[string]any{
				"kind":              "wire_source_item_resolution",
				"request_id":        requestID,
				"source_item_id":    sourceItemID,
				"processor_key":     metadataStringValue(rec.Metadata, runMetadataProcessorKey),
				"continuity_ref":    metadataStringValue(rec.Metadata, "continuity_ref"),
				"source_request_id": metadataStringValue(rec.Metadata, "source_network_request_id"),
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

type wireProcessorDecisionVerdict string

const (
	wireProcessorDecisionOpenedTexture      wireProcessorDecisionVerdict = "opened_texture"
	wireProcessorDecisionAlreadyCovered     wireProcessorDecisionVerdict = "already_covered"
	wireProcessorDecisionNotNewsworthy      wireProcessorDecisionVerdict = "not_newsworthy"
	wireProcessorDecisionInsufficientSignal wireProcessorDecisionVerdict = "insufficient_evidence"
	wireProcessorDecisionDeferred           wireProcessorDecisionVerdict = "deferred"
)

// Work-item detail keys for wire processor decisions. The request-level item
// carries the "last_" trio (latest decision across source items) and only
// carries the plain trio when the request has no per-item ledger.
const (
	wireDetailKeyDecision            = "decision"
	wireDetailKeyDecisionSummary     = "decision_summary"
	wireDetailKeyDecisionRunID       = "decision_run_id"
	wireDetailKeyDecisionRecordedAt  = "decision_recorded_at"
	wireDetailKeyLastDecision        = "last_decision"
	wireDetailKeyLastDecisionSummary = "last_decision_summary"
	wireDetailKeyLastDecisionRunID   = "last_decision_run_id"
	wireDetailKeyResolutionState     = "resolution_state"
	wireDetailKeyStoryDocID          = "story_doc_id"
	wireDetailKeyRevisionRunID       = "revision_run_id"
	wireDetailKeyCoveredByDocID      = "covered_by_doc_id"
	wireDetailKeyCoveredByRoutePath  = "covered_by_route_path"
)

// wireProcessorVerdictTransitionAllowed reports whether an existing verdict
// may be replaced. Deferred is the only nonterminal verdict by design; every
// other verdict is immutable once recorded.
func wireProcessorVerdictTransitionAllowed(existing, next string) bool {
	return existing == "" || existing == next || existing == string(wireProcessorDecisionDeferred)
}

// wireProcessorDecisionDetailPatch stamps the decision trio (as the latest
// decision when asLast is set) plus the optional story/coverage refs.
func wireProcessorDecisionDetailPatch(update wireProcessorDecisionUpdate, runID string, asLast bool) map[string]any {
	decisionKey, summaryKey, runIDKey := wireDetailKeyDecision, wireDetailKeyDecisionSummary, wireDetailKeyDecisionRunID
	if asLast {
		decisionKey, summaryKey, runIDKey = wireDetailKeyLastDecision, wireDetailKeyLastDecisionSummary, wireDetailKeyLastDecisionRunID
	}
	patch := map[string]any{
		decisionKey: string(update.Verdict),
		summaryKey:  strings.TrimSpace(update.Summary),
		runIDKey:    runID,
	}
	if docID := strings.TrimSpace(update.DocID); docID != "" {
		patch[wireDetailKeyStoryDocID] = docID
	}
	if revisionRunID := strings.TrimSpace(update.RevisionRunID); revisionRunID != "" {
		patch[wireDetailKeyRevisionRunID] = revisionRunID
	}
	if coveredByDocID := strings.TrimSpace(update.CoveredByDocID); coveredByDocID != "" {
		patch[wireDetailKeyCoveredByDocID] = coveredByDocID
	}
	if coveredByRoutePath := strings.TrimSpace(update.CoveredByRoutePath); coveredByRoutePath != "" {
		patch[wireDetailKeyCoveredByRoutePath] = coveredByRoutePath
	}
	return patch
}

type wireProcessorDecisionUpdate struct {
	Verdict            wireProcessorDecisionVerdict
	Summary            string
	DocID              string
	RevisionRunID      string
	SourceItemIDs      []string
	CoveredByDocID     string
	CoveredByRoutePath string
	Complete           bool
}

func (rt *Runtime) recordWireProcessorDecision(ctx context.Context, rec *types.RunRecord, update wireProcessorDecisionUpdate) (types.WorkItemRecord, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return types.WorkItemRecord{}, fmt.Errorf("wire processor decision record: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if ownerID == "" || trajectoryID == "" {
		return types.WorkItemRecord{}, fmt.Errorf("wire processor decision record: identity unavailable")
	}
	if update.Verdict == "" {
		return types.WorkItemRecord{}, fmt.Errorf("wire processor decision record: verdict is required")
	}
	if update.Verdict == wireProcessorDecisionAlreadyCovered {
		if err := rt.validateWireAlreadyCoveredDecision(ctx, rec, &update); err != nil {
			return types.WorkItemRecord{}, err
		}
	} else if strings.TrimSpace(update.CoveredByDocID) != "" {
		return types.WorkItemRecord{}, fmt.Errorf("wire processor decision record: covered_by_doc_id is only valid for already_covered")
	}
	item, found, err := rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, workitem.ProcessorDecisionFingerprint(trajectoryID))
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	if !found {
		if _, err := rt.beginWireProcessorDecisionWorkItem(ctx, rec); err != nil {
			return types.WorkItemRecord{}, err
		}
		item, found, err = rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, workitem.ProcessorDecisionFingerprint(trajectoryID))
		if err != nil {
			return types.WorkItemRecord{}, err
		}
		if !found {
			return types.WorkItemRecord{}, fmt.Errorf("wire processor decision record: work item missing after create")
		}
	}
	nextVerdict := string(update.Verdict)
	existingVerdict := strings.TrimSpace(metadataStringValue(item.Details, wireDetailKeyDecision))
	if !wireProcessorVerdictTransitionAllowed(existingVerdict, nextVerdict) {
		return types.WorkItemRecord{}, fmt.Errorf("wire processor decision record: verdict already set to %s", existingVerdict)
	}
	sourceItemIDs := trimNonEmpty(update.SourceItemIDs)
	if len(sourceItemIDs) > 0 {
		if err := rt.recordWireProcessorSourceItemDecisions(ctx, rec, sourceItemIDs, update); err != nil {
			return types.WorkItemRecord{}, err
		}
	}
	item, err = rt.store.UpdateWorkItemDetails(ctx, ownerID, item.WorkItemID, wireProcessorRequestResolutionPatch(rec, item, update))
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	if len(sourceItemIDs) == 0 && update.Complete && item.Status != types.WorkItemCompleted {
		item, err = rt.store.UpdateWorkItemStatus(ctx, ownerID, item.WorkItemID, types.WorkItemCompleted)
		if err != nil {
			return types.WorkItemRecord{}, err
		}
	}
	if len(sourceItemIDs) > 0 {
		item, err = rt.reconcileWireProcessorRequestResolution(ctx, rec)
		if err != nil {
			return types.WorkItemRecord{}, err
		}
	}
	return item, nil
}

func (rt *Runtime) recordWireProcessorSourceItemDecisions(ctx context.Context, rec *types.RunRecord, sourceItemIDs []string, update wireProcessorDecisionUpdate) error {
	if rt == nil || rt.store == nil || rec == nil {
		return fmt.Errorf("wire processor source item decisions: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if ownerID == "" || trajectoryID == "" {
		return fmt.Errorf("wire processor source item decisions: identity unavailable")
	}
	for _, sourceItemID := range sourceItemIDs {
		item, found, err := rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, workitem.SourceItemDecisionFingerprint(trajectoryID, sourceItemID))
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("wire processor source item decision: work item missing for %s", sourceItemID)
		}
		existingVerdict := strings.TrimSpace(metadataStringValue(item.Details, wireDetailKeyDecision))
		if !wireProcessorVerdictTransitionAllowed(existingVerdict, string(update.Verdict)) {
			return fmt.Errorf("wire processor source item decision: source item %s already set to %s", sourceItemID, existingVerdict)
		}
		patch := wireProcessorDecisionDetailPatch(update, rec.RunID, false)
		if metadataStringValue(item.Details, wireDetailKeyDecisionRecordedAt) == "" {
			patch[wireDetailKeyDecisionRecordedAt] = time.Now().UTC().Format(time.RFC3339Nano)
		}
		item, err = rt.store.UpdateWorkItemDetails(ctx, ownerID, item.WorkItemID, patch)
		if err != nil {
			return err
		}
		if item.Status != types.WorkItemCompleted {
			if _, err := rt.store.UpdateWorkItemStatus(ctx, ownerID, item.WorkItemID, types.WorkItemCompleted); err != nil {
				return err
			}
		}
	}
	return nil
}

func wireProcessorRequestResolutionPatch(rec *types.RunRecord, item types.WorkItemRecord, update wireProcessorDecisionUpdate) map[string]any {
	patch := wireProcessorDecisionDetailPatch(update, rec.RunID, true)
	if len(update.SourceItemIDs) == 0 {
		for key, value := range wireProcessorDecisionDetailPatch(update, rec.RunID, false) {
			patch[key] = value
		}
		if metadataStringValue(item.Details, wireDetailKeyDecisionRecordedAt) == "" {
			patch[wireDetailKeyDecisionRecordedAt] = time.Now().UTC().Format(time.RFC3339Nano)
		}
	} else {
		patch["resolved_source_item_ids"] = trimNonEmpty(update.SourceItemIDs)
	}
	return patch
}

func (rt *Runtime) reconcileWireProcessorRequestResolution(ctx context.Context, rec *types.RunRecord) (types.WorkItemRecord, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return types.WorkItemRecord{}, fmt.Errorf("wire processor request resolution: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if ownerID == "" || trajectoryID == "" {
		return types.WorkItemRecord{}, fmt.Errorf("wire processor request resolution: identity unavailable")
	}
	requestItem, found, err := rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, workitem.ProcessorDecisionFingerprint(trajectoryID))
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	if !found {
		return types.WorkItemRecord{}, fmt.Errorf("wire processor request resolution: request work item missing")
	}
	sourceItemIDs := wireProcessorSourceItemIDs(rec)
	if len(sourceItemIDs) == 0 {
		return requestItem, nil
	}
	resolved := 0
	hasOpenedTexture := false
	allAlreadyCovered := true
	allTerminalWithoutStory := true
	hasDeferred := false
	for _, sourceItemID := range sourceItemIDs {
		item, found, err := rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, workitem.SourceItemDecisionFingerprint(trajectoryID, sourceItemID))
		if err != nil {
			return types.WorkItemRecord{}, err
		}
		if !found {
			continue
		}
		if item.Status == types.WorkItemCompleted {
			resolved++
		}
		decision := metadataStringValue(item.Details, wireDetailKeyDecision)
		coveredByDocID := strings.TrimSpace(metadataStringValue(item.Details, wireDetailKeyCoveredByDocID))
		if decision == string(wireProcessorDecisionOpenedTexture) {
			hasOpenedTexture = true
		}
		if decision != string(wireProcessorDecisionAlreadyCovered) || coveredByDocID == "" {
			allAlreadyCovered = false
		}
		switch decision {
		case string(wireProcessorDecisionAlreadyCovered):
			if coveredByDocID == "" {
				allTerminalWithoutStory = false
			}
		case string(wireProcessorDecisionNotNewsworthy), string(wireProcessorDecisionInsufficientSignal):
			// Explicit terminal no-story decisions.
		case string(wireProcessorDecisionDeferred):
			hasDeferred = true
			allTerminalWithoutStory = false
		default:
			allTerminalWithoutStory = false
		}
	}
	patch := map[string]any{
		"resolved_source_item_count": resolved,
		"source_item_count":          len(sourceItemIDs),
	}
	switch {
	case resolved < len(sourceItemIDs):
		patch[wireDetailKeyResolutionState] = sourceapi.ResolutionStateAwaitingSourceItemDecisions
	case hasOpenedTexture:
		patch[wireDetailKeyResolutionState] = sourceapi.ResolutionStateDecidedWithStoryRoute
	case allAlreadyCovered:
		patch[wireDetailKeyResolutionState] = sourceapi.ResolutionStateSuppressedAgainstPublishedCorpus
	case hasDeferred:
		patch[wireDetailKeyResolutionState] = sourceapi.ResolutionStateDeferredWithoutStoryRoute
	default:
		patch[wireDetailKeyResolutionState] = sourceapi.ResolutionStateDecidedWithoutStoryRoute
	}
	requestItem, err = rt.store.UpdateWorkItemDetails(ctx, ownerID, requestItem.WorkItemID, patch)
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	// One rule: every item resolved closes the request; no story route means
	// nothing on this trajectory can settle, so cancel it. Deferred items keep
	// the request open until a later decision upgrades them.
	if resolved == len(sourceItemIDs) && (hasOpenedTexture || allTerminalWithoutStory) {
		if requestItem.Status != types.WorkItemCompleted {
			requestItem, err = rt.store.UpdateWorkItemStatus(ctx, ownerID, requestItem.WorkItemID, types.WorkItemCompleted)
			if err != nil {
				return types.WorkItemRecord{}, err
			}
		}
		if !hasOpenedTexture {
			if _, err := rt.store.UpdateTrajectoryStatus(ctx, ownerID, trajectoryID, types.TrajectoryCancelled); err != nil {
				return types.WorkItemRecord{}, err
			}
		}
	}
	return requestItem, nil
}

func (rt *Runtime) validateWireAlreadyCoveredDecision(ctx context.Context, rec *types.RunRecord, update *wireProcessorDecisionUpdate) error {
	if rt == nil || rt.store == nil || rec == nil || update == nil {
		return fmt.Errorf("wire already covered decision: runtime unavailable")
	}
	docID := strings.TrimSpace(update.CoveredByDocID)
	if docID == "" {
		return fmt.Errorf("wire already covered decision: covered_by_doc_id is required for already_covered")
	}
	doc, err := rt.store.GetDocument(ctx, docID, strings.TrimSpace(rec.OwnerID))
	if err != nil {
		return fmt.Errorf("wire already covered decision: lookup covered_by_doc_id %s: %w", docID, err)
	}
	if strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return fmt.Errorf("wire already covered decision: covered_by_doc_id %s has no current revision", docID)
	}
	rev, err := rt.store.GetRevision(ctx, doc.CurrentRevisionID, strings.TrimSpace(rec.OwnerID))
	if err != nil {
		return fmt.Errorf("wire already covered decision: load covered_by_doc_id %s current revision: %w", docID, err)
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	routePath := strings.TrimSpace(wirePlatformRoutePath(meta))
	if routePath == "" {
		return fmt.Errorf("wire already covered decision: covered_by_doc_id %s is not published", docID)
	}
	update.CoveredByDocID = docID
	update.CoveredByRoutePath = routePath
	return nil
}

func (rt *Runtime) beginWireStoryResolutionWorkItem(ctx context.Context, rec *types.RunRecord, doc types.Document) (string, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return "", fmt.Errorf("wire story resolution work item: runtime unavailable")
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	docID := strings.TrimSpace(doc.DocID)
	if ownerID == "" || trajectoryID == "" || docID == "" {
		return "", fmt.Errorf("wire story resolution work item: identity unavailable")
	}
	item, err := rt.store.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              ownerID,
		TrajectoryID:         trajectoryID,
		Objective:            "resolve wire story candidate to publication or explicit non-publication decision",
		Reason:               "processor opened a wire story Texture route",
		AuthorityProfile:     agentprofile.Texture,
		ObjectiveFingerprint: workitem.StoryResolutionFingerprint(trajectoryID, docID),
		CreatedByRunID:       rec.RunID,
		Details: map[string]any{
			"kind":   "wire_story_resolution",
			"doc_id": docID,
		},
	})
	if err != nil {
		return "", err
	}
	return item.WorkItemID, nil
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
