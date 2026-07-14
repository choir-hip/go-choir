package agentcore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/workitem"
)

func TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	coveredByDocID := seedPublishedCoverageDoc(t, s, "user-alice", "wire-existing-coverage")

	run, err := rt.StartRunWithMetadata(ctx, "review this batch", "user-alice", map[string]any{
		runMetadataAgentProfile:        agentprofile.Processor,
		runMetadataAgentRole:           agentprofile.Processor,
		"ingestion_handoff_request_id": "processor-request-explicit",
		runMetadataProcessorKey:        "processor:global_firehose:global:gdelt",
		"source_item_ids":              []string{"source-item-1"},
		"source_count":                 1,
		"source_network_request_id":    "source-request-1",
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}

	registry := toolregistry.NewToolRegistry()
	if err := RegisterWireProcessorTools(registry, rt); err != nil {
		t.Fatalf("register wire processor tools: %v", err)
	}
	raw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(run)), "record_wire_processor_decision", json.RawMessage(`{
		"decision":"already_covered",
		"summary":"Existing article already covers this source batch.",
		"covered_by_doc_id":"`+coveredByDocID+`"
	}`))
	if err != nil {
		t.Fatalf("record_wire_processor_decision: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode record_wire_processor_decision response: %v", err)
	}
	if resp["decision"] != "already_covered" || resp["status"] != string(types.WorkItemCompleted) {
		t.Fatalf("unexpected decision response: %+v", resp)
	}

	requestItem, found, err := s.FindWorkItemByFingerprint(ctx, "user-alice", run.TrajectoryID, workitem.ProcessorDecisionFingerprint(run.TrajectoryID))
	if err != nil {
		t.Fatalf("find processor request work item: %v", err)
	}
	if !found {
		t.Fatal("processor request work item not found")
	}
	if requestItem.Status != types.WorkItemCompleted {
		t.Fatalf("processor request status = %s, want completed", requestItem.Status)
	}
	if requestItem.Details["last_decision"] != "already_covered" || requestItem.Details["resolution_state"] != "all_source_items_suppressed_against_published_corpus" {
		t.Fatalf("processor request details = %+v", requestItem.Details)
	}

	sourceItem, found, err := s.FindWorkItemByFingerprint(ctx, "user-alice", run.TrajectoryID, workitem.SourceItemDecisionFingerprint(run.TrajectoryID, "source-item-1"))
	if err != nil {
		t.Fatalf("find source-item work item: %v", err)
	}
	if !found {
		t.Fatal("source-item work item not found")
	}
	if sourceItem.Status != types.WorkItemCompleted {
		t.Fatalf("source-item status = %s, want completed", sourceItem.Status)
	}
	if sourceItem.Details["decision"] != "already_covered" || sourceItem.Details["decision_summary"] != "Existing article already covers this source batch." {
		t.Fatalf("source-item decision details = %+v", sourceItem.Details)
	}
	if sourceItem.Details["covered_by_doc_id"] != coveredByDocID || sourceItem.Details["covered_by_route_path"] != "wire-existing-coverage" {
		t.Fatalf("source-item covered-by details = %+v", sourceItem.Details)
	}

	obligations, err := rt.TrajectoryObligations(ctx, "user-alice", run.TrajectoryID)
	if err != nil {
		t.Fatalf("trajectory obligations: %v", err)
	}
	if len(obligations.OpenWorkItems) != 0 {
		t.Fatalf("open obligations after explicit non-publication verdict = %+v", obligations.OpenWorkItems)
	}
	if obligations.Trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("trajectory status = %s, want cancelled", obligations.Trajectory.Status)
	}
}

func TestRecordWireProcessorDecisionToolRejectsAlreadyCoveredWithoutPublishedDoc(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)

	unpublishedDoc := types.Document{
		DocID:     "doc-unpublished-coverage",
		OwnerID:   "user-alice",
		Title:     "Unpublished Coverage",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateDocument(ctx, unpublishedDoc); err != nil {
		t.Fatalf("create unpublished doc: %v", err)
	}

	run, err := rt.StartRunWithMetadata(ctx, "review this batch", "user-alice", map[string]any{
		runMetadataAgentProfile:        agentprofile.Processor,
		runMetadataAgentRole:           agentprofile.Processor,
		"ingestion_handoff_request_id": "processor-request-unpublished",
		runMetadataProcessorKey:        "processor:global_firehose:global:gdelt",
		"source_item_ids":              []string{"source-item-1"},
		"source_count":                 1,
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}

	registry := toolregistry.NewToolRegistry()
	if err := RegisterWireProcessorTools(registry, rt); err != nil {
		t.Fatalf("register wire processor tools: %v", err)
	}
	_, err = registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(run)), "record_wire_processor_decision", json.RawMessage(`{
		"decision":"already_covered",
		"summary":"Existing draft allegedly covers this source batch.",
		"covered_by_doc_id":"doc-unpublished-coverage"
	}`))
	if err == nil || err.Error() != "wire already covered decision: covered_by_doc_id doc-unpublished-coverage has no current revision" && err.Error() != "wire already covered decision: covered_by_doc_id doc-unpublished-coverage is not published" {
		t.Fatalf("already_covered unpublished doc error = %v", err)
	}
}

func TestRecordWireProcessorDecisionToolCancelsExplicitNoStoryTerminalBranch(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)

	run, err := rt.StartRunWithMetadata(ctx, "review this batch", "user-alice", map[string]any{
		runMetadataAgentProfile:        agentprofile.Processor,
		runMetadataAgentRole:           agentprofile.Processor,
		"ingestion_handoff_request_id": "processor-request-not-newsworthy",
		runMetadataProcessorKey:        "processor:global_firehose:global:gdelt",
		"source_item_ids":              []string{"source-item-1"},
		"source_count":                 1,
		"source_network_request_id":    "source-request-not-newsworthy",
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}

	registry := toolregistry.NewToolRegistry()
	if err := RegisterWireProcessorTools(registry, rt); err != nil {
		t.Fatalf("register wire processor tools: %v", err)
	}
	raw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(run)), "record_wire_processor_decision", json.RawMessage(`{
		"decision":"not_newsworthy",
		"summary":"The batch does not justify a publication route."
	}`))
	if err != nil {
		t.Fatalf("record_wire_processor_decision: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode record_wire_processor_decision response: %v", err)
	}
	if resp["decision"] != "not_newsworthy" || resp["status"] != string(types.WorkItemCompleted) {
		t.Fatalf("unexpected decision response: %+v", resp)
	}

	requestItem, found, err := s.FindWorkItemByFingerprint(ctx, "user-alice", run.TrajectoryID, workitem.ProcessorDecisionFingerprint(run.TrajectoryID))
	if err != nil {
		t.Fatalf("find processor request work item: %v", err)
	}
	if !found {
		t.Fatal("processor request work item not found")
	}
	if requestItem.Status != types.WorkItemCompleted || requestItem.Details["resolution_state"] != "all_source_items_decided_without_story_route" {
		t.Fatalf("processor request item = %+v, want completed explicit no-story terminal", requestItem)
	}
	if requestItem.Details["last_decision"] != "not_newsworthy" {
		t.Fatalf("processor request last_decision = %+v, want not_newsworthy", requestItem.Details)
	}

	obligations, err := rt.TrajectoryObligations(ctx, "user-alice", run.TrajectoryID)
	if err != nil {
		t.Fatalf("trajectory obligations: %v", err)
	}
	if len(obligations.OpenWorkItems) != 0 {
		t.Fatalf("open obligations after explicit no-story terminal verdict = %+v", obligations.OpenWorkItems)
	}
	if obligations.Trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("trajectory status = %s, want cancelled", obligations.Trajectory.Status)
	}
}

func TestRecordWireProcessorDecisionToolKeepsDeferredBranchOpen(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)

	run, err := rt.StartRunWithMetadata(ctx, "review this batch", "user-alice", map[string]any{
		runMetadataAgentProfile:        agentprofile.Processor,
		runMetadataAgentRole:           agentprofile.Processor,
		"ingestion_handoff_request_id": "processor-request-deferred",
		runMetadataProcessorKey:        "processor:global_firehose:global:gdelt",
		"source_item_ids":              []string{"source-item-1"},
		"source_count":                 1,
		"source_network_request_id":    "source-request-deferred",
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}

	registry := toolregistry.NewToolRegistry()
	if err := RegisterWireProcessorTools(registry, rt); err != nil {
		t.Fatalf("register wire processor tools: %v", err)
	}
	raw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(run)), "record_wire_processor_decision", json.RawMessage(`{
		"decision":"deferred",
		"summary":"Hold this batch pending stronger corpus evidence."
	}`))
	if err != nil {
		t.Fatalf("record_wire_processor_decision: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode record_wire_processor_decision response: %v", err)
	}
	if resp["decision"] != "deferred" || resp["status"] != string(types.WorkItemOpen) {
		t.Fatalf("unexpected deferred decision response: %+v", resp)
	}

	requestItem, found, err := s.FindWorkItemByFingerprint(ctx, "user-alice", run.TrajectoryID, workitem.ProcessorDecisionFingerprint(run.TrajectoryID))
	if err != nil {
		t.Fatalf("find processor request work item: %v", err)
	}
	if !found {
		t.Fatal("processor request work item not found")
	}
	if requestItem.Status != types.WorkItemOpen || requestItem.Details["resolution_state"] != "all_source_items_deferred_without_story_route" {
		t.Fatalf("processor request item = %+v, want open deferred branch", requestItem)
	}

	obligations, err := rt.TrajectoryObligations(ctx, "user-alice", run.TrajectoryID)
	if err != nil {
		t.Fatalf("trajectory obligations: %v", err)
	}
	if len(obligations.OpenWorkItems) != 1 || obligations.OpenWorkItems[0].WorkItemID != requestItem.WorkItemID {
		t.Fatalf("open obligations after deferred verdict = %+v, want only request item open", obligations.OpenWorkItems)
	}
	if obligations.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("trajectory status = %s, want live", obligations.Trajectory.Status)
	}
}

func seedPublishedCoverageDoc(t *testing.T, s interface {
	CreateDocument(context.Context, types.Document) error
	CreateRevision(context.Context, types.Revision) error
	GetDocument(context.Context, string, string) (types.Document, error)
}, ownerID, routePath string) string {
	t.Helper()
	now := time.Now().UTC()
	docID := "doc-" + routePath
	doc := types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Covered story",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(context.Background(), doc); err != nil {
		t.Fatalf("create published coverage doc: %v", err)
	}
	meta, _ := json.Marshal(map[string]any{
		"corpusd_route_path": routePath,
		"corpusd_publication_ref": map[string]any{
			"route_path": routePath,
		},
	})
	rev := types.Revision{
		RevisionID:       "rev-" + routePath,
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		Content:          "# Covered story\n\nAlready published.",
		BodyDoc:          runtimeTestTextureBodyDoc(t, docID, "rev-"+routePath, "# Covered story\n\nAlready published."),
		Citations:        json.RawMessage("[]"),
		Metadata:         meta,
		ParentRevisionID: "",
		CreatedAt:        now,
	}
	if err := s.CreateRevision(context.Background(), rev); err != nil {
		t.Fatalf("create published coverage revision: %v", err)
	}
	return docID
}
