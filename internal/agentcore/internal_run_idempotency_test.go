package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func processorHandoffSubmissionFixture(ownerID, requestID, requestKind string, sourceItemIDs []string) internalRunSubmitRequest {
	processorKey := "processor:general:global:rss"
	return internalRunSubmitRequest{
		OwnerID: ownerID,
		Prompt:  "Processor " + processorKey + ": ingest SourceItems by handle.",
		Metadata: map[string]any{
			runMetadataAgentID:               "processor-v2:processor-general-global-rss",
			runMetadataChannelID:             "processor-v2:processor-general-global-rss",
			runMetadataAgentProfile:          agentprofile.Processor,
			runMetadataAgentRole:             agentprofile.Processor,
			"request_source":                 "sourcecycled",
			"activation_origin":              "ingestion_event",
			"ingestion_event_ids":            []string{"ingestionevt-test"},
			"source_network_cycle_id":        "cycle-test",
			"source_network_request_id":      requestID,
			"source_network_request_kind":    requestKind,
			"ingestion_handoff_request_kind": requestKind,
			"ingestion_handoff_request_id":   requestID,
			"ingestion_handoff_cycle_id":     "cycle-test",
			runMetadataProcessorKey:          processorKey,
			"source_item_ids":                sourceItemIDs,
			"source_count":                   len(sourceItemIDs),
			"source_types":                   []string{"rss"},
			"verticals":                      []string{"general"},
			"regions":                        []string{"global"},
			"continuity_ref":                 "sourcecycled://processor/" + processorKey + "/latest",
		},
	}
}

func reconcilerHandoffSubmissionFixture(ownerID, requestID, requestKind, scope string) internalRunSubmitRequest {
	return internalRunSubmitRequest{
		OwnerID: ownerID,
		Prompt:  "Reconcile the story corpus.",
		Metadata: map[string]any{
			runMetadataAgentProfile:          agentprofile.Reconciler,
			runMetadataAgentRole:             agentprofile.Reconciler,
			"request_source":                 "sourcecycled",
			"ingestion_handoff_request_kind": requestKind,
			"ingestion_handoff_request_id":   requestID,
			"ingestion_handoff_cycle_id":     "cycle-test",
			runMetadataReconcilerScope:       scope,
			"source_item_ids":                []string{"source-item-1"},
			"processor_request_ids":          []string{"processor-request-1"},
		},
	}
}

func postInternalRunSubmissionFixture(t *testing.T, handler *APIHandler, submission internalRunSubmitRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(submission)
	if err != nil {
		t.Fatalf("marshal internal run submission: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs", strings.NewReader(string(body)))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalRunSubmission(w, req)
	return w
}

func decodeInternalRunSubmissionStatus(t *testing.T, w *httptest.ResponseRecorder) runStatusResponse {
	t.Helper()
	var status runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode internal run status: %v; body=%s", err, w.Body.String())
	}
	return status
}

func assertRunIdentityLivesOnlyInBodyForTest(t *testing.T, rt *Runtime, runID, wantRequestID, wantRequestKind, wantProfile string) {
	t.Helper()
	var rawMetadata string
	if err := rt.Store().DB().QueryRowContext(context.Background(), `SELECT metadata FROM og_objects
		WHERE object_kind = 'choir.run'
		AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.run_id')) = ?`, runID).Scan(&rawMetadata); err != nil {
		t.Fatalf("load run object metadata: %v", err)
	}
	var projected map[string]any
	if err := json.Unmarshal([]byte(rawMetadata), &projected); err != nil {
		t.Fatalf("decode run object metadata: %v", err)
	}
	if _, ok := projected["ingestion_handoff_request_id"]; ok {
		t.Fatalf("run %s unexpectedly duplicated ingestion identity into object metadata: %+v", runID, projected)
	}
	var bodyRequestID, bodyRequestKind, bodyProfile string
	if err := rt.Store().DB().QueryRowContext(context.Background(), `SELECT
		JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.metadata.ingestion_handoff_request_id')),
		JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.metadata.ingestion_handoff_request_kind')),
		JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.agent_profile'))
		FROM og_objects WHERE object_kind = 'choir.run'
		AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.run_id')) = ?`, runID).Scan(&bodyRequestID, &bodyRequestKind, &bodyProfile); err != nil {
		t.Fatalf("extract run ingestion identity from body: %v", err)
	}
	if bodyRequestID != wantRequestID || bodyRequestKind != wantRequestKind || bodyProfile != wantProfile {
		t.Fatalf("run body identity = %q/%q profile=%q, want %q/%q profile=%q", bodyRequestID, bodyRequestKind, bodyProfile, wantRequestID, wantRequestKind, wantProfile)
	}
}

func TestHandleInternalRunSubmissionDeduplicatesTypedProcessorHandoff(t *testing.T) {
	rt, handler := testAPISetup(t)
	t.Setenv("RUNTIME_MAX_PROCESSOR_RUNS", "10")
	ctx := context.Background()

	submission := processorHandoffSubmissionFixture("owner-autopaper", "processor-idempotent", "processor", []string{"source-item-1"})
	firstW := postInternalRunSubmissionFixture(t, handler, submission)
	if firstW.Code != http.StatusAccepted {
		t.Fatalf("first submission status = %d, want 202; body=%s", firstW.Code, firstW.Body.String())
	}
	first := decodeInternalRunSubmissionStatus(t, firstW)
	if first.RunID == "" {
		t.Fatal("first submission returned empty run id")
	}
	stored, err := rt.Store().GetRun(ctx, first.RunID)
	if err != nil {
		t.Fatalf("load first processor run: %v", err)
	}
	if metadataStringValue(stored.Metadata, internalRunSubmissionFingerprintMetadataKey) == "" {
		t.Fatalf("processor run did not persist submission fingerprint: %+v", stored.Metadata)
	}

	// Descendant runs can inherit ingestion provenance and even a processor
	// profile. They are not top-level handoff receipts and must not collide.
	now := time.Now().UTC()
	if err := rt.Store().CreateRun(ctx, types.RunRecord{
		RunID:            "run-child-processor-provenance",
		AgentID:          "agent-child-processor-provenance",
		ChannelID:        "channel-child-processor-provenance",
		RequestedByRunID: first.RunID,
		AgentProfile:     agentprofile.Processor,
		AgentRole:        agentprofile.Processor,
		OwnerID:          submission.OwnerID,
		SandboxID:        "sandbox-test",
		State:            types.RunPending,
		Prompt:           "process inherited provenance",
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata: map[string]any{
			runMetadataAgentProfile:          agentprofile.Processor,
			runMetadataAgentRole:             agentprofile.Processor,
			"ingestion_handoff_request_id":   "processor-idempotent",
			"ingestion_handoff_request_kind": "processor",
		},
	}); err != nil {
		t.Fatalf("create provenance-inheriting processor child: %v", err)
	}

	secondW := postInternalRunSubmissionFixture(t, handler, submission)
	if secondW.Code != http.StatusAccepted {
		t.Fatalf("duplicate submission status = %d, want 202; body=%s", secondW.Code, secondW.Body.String())
	}
	second := decodeInternalRunSubmissionStatus(t, secondW)
	if second.RunID != first.RunID {
		t.Fatalf("duplicate submission run id = %q, want original %q", second.RunID, first.RunID)
	}
	matches, err := rt.Store().ListRunsByIngestionHandoff(ctx, submission.OwnerID, agentprofile.Processor, "processor-idempotent", "processor", 10)
	if err != nil {
		t.Fatalf("list processor handoff runs: %v", err)
	}
	if len(matches) != 1 || matches[0].RunID != first.RunID {
		t.Fatalf("processor handoff matches = %+v, want one original run", matches)
	}

	conflict := processorHandoffSubmissionFixture(submission.OwnerID, "processor-idempotent", "processor", []string{"source-item-conflict"})
	conflictW := postInternalRunSubmissionFixture(t, handler, conflict)
	if conflictW.Code != http.StatusConflict {
		t.Fatalf("conflicting submission status = %d, want 409; body=%s", conflictW.Code, conflictW.Body.String())
	}

	// Owner and request kind are both part of the idempotency identity.
	otherOwner := processorHandoffSubmissionFixture("owner-autopaper-other", "processor-idempotent", "processor", []string{"source-item-1"})
	otherOwnerW := postInternalRunSubmissionFixture(t, handler, otherOwner)
	if otherOwnerW.Code != http.StatusAccepted {
		t.Fatalf("other-owner submission status = %d, want 202; body=%s", otherOwnerW.Code, otherOwnerW.Body.String())
	}
	if got := decodeInternalRunSubmissionStatus(t, otherOwnerW).RunID; got == first.RunID {
		t.Fatalf("other owner reused first owner's run id %q", got)
	}
	otherKind := processorHandoffSubmissionFixture(submission.OwnerID, "processor-idempotent", "processor-refresh", []string{"source-item-1"})
	otherKindW := postInternalRunSubmissionFixture(t, handler, otherKind)
	if otherKindW.Code != http.StatusAccepted {
		t.Fatalf("other-kind submission status = %d, want 202; body=%s", otherKindW.Code, otherKindW.Body.String())
	}
	if got := decodeInternalRunSubmissionStatus(t, otherKindW).RunID; got == first.RunID {
		t.Fatalf("other request kind reused original run id %q", got)
	}
	otherProfile := reconcilerHandoffSubmissionFixture(submission.OwnerID, "processor-idempotent", "processor", "story-corpus")
	otherProfileW := postInternalRunSubmissionFixture(t, handler, otherProfile)
	if otherProfileW.Code != http.StatusAccepted {
		t.Fatalf("other-profile submission status = %d, want 202; body=%s", otherProfileW.Code, otherProfileW.Body.String())
	}
	if got := decodeInternalRunSubmissionStatus(t, otherProfileW).RunID; got == first.RunID {
		t.Fatalf("other profile reused original processor run id %q", got)
	}
}

func TestHandleInternalRunSubmissionDeduplicatesTypedReconcilerHandoff(t *testing.T) {
	rt, handler := testAPISetup(t)
	ctx := context.Background()
	submission := reconcilerHandoffSubmissionFixture("owner-reconciler", "reconciler-idempotent", "reconciler", "story-corpus")

	firstW := postInternalRunSubmissionFixture(t, handler, submission)
	if firstW.Code != http.StatusAccepted {
		t.Fatalf("first reconciler submission status = %d, want 202; body=%s", firstW.Code, firstW.Body.String())
	}
	first := decodeInternalRunSubmissionStatus(t, firstW)
	duplicateW := postInternalRunSubmissionFixture(t, handler, submission)
	if duplicateW.Code != http.StatusAccepted {
		t.Fatalf("duplicate reconciler submission status = %d, want 202; body=%s", duplicateW.Code, duplicateW.Body.String())
	}
	if got := decodeInternalRunSubmissionStatus(t, duplicateW).RunID; got != first.RunID {
		t.Fatalf("duplicate reconciler run id = %q, want original %q", got, first.RunID)
	}

	matches, err := rt.Store().ListRunsByIngestionHandoff(ctx, submission.OwnerID, agentprofile.Reconciler, "reconciler-idempotent", "reconciler", 10)
	if err != nil {
		t.Fatalf("list reconciler handoff runs: %v", err)
	}
	if len(matches) != 1 || matches[0].RunID != first.RunID {
		t.Fatalf("reconciler handoff matches = %+v, want one original run", matches)
	}
	conflict := reconcilerHandoffSubmissionFixture(submission.OwnerID, "reconciler-idempotent", "reconciler", "regional-corpus")
	conflictW := postInternalRunSubmissionFixture(t, handler, conflict)
	if conflictW.Code != http.StatusConflict {
		t.Fatalf("conflicting reconciler submission status = %d, want 409; body=%s", conflictW.Code, conflictW.Body.String())
	}
}

func TestHandleInternalRunSubmissionConcurrentRetriesCreateOneProcessorRun(t *testing.T) {
	rt, handler := testAPISetup(t)
	t.Setenv("RUNTIME_MAX_PROCESSOR_RUNS", "10")
	ctx := context.Background()
	submission := processorHandoffSubmissionFixture("owner-concurrent", "processor-concurrent", "processor", []string{"source-item-concurrent"})
	body, err := json.Marshal(submission)
	if err != nil {
		t.Fatalf("marshal concurrent internal run submission: %v", err)
	}

	const retryCount = 8
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, retryCount)
	var wg sync.WaitGroup
	for range retryCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs", strings.NewReader(string(body)))
			req.Header.Set("X-Internal-Caller", "true")
			response := httptest.NewRecorder()
			handler.HandleInternalRunSubmission(response, req)
			responses <- response
		}()
	}
	close(start)
	wg.Wait()
	close(responses)

	var originalRunID string
	for response := range responses {
		if response.Code != http.StatusAccepted {
			t.Fatalf("concurrent retry status = %d, want 202; body=%s", response.Code, response.Body.String())
		}
		status := decodeInternalRunSubmissionStatus(t, response)
		if originalRunID == "" {
			originalRunID = status.RunID
		}
		if status.RunID != originalRunID {
			t.Fatalf("concurrent retry run id = %q, want original %q", status.RunID, originalRunID)
		}
	}

	matches, err := rt.Store().ListRunsByIngestionHandoff(ctx, submission.OwnerID, agentprofile.Processor, "processor-concurrent", "processor", 10)
	if err != nil {
		t.Fatalf("list concurrent processor handoff runs: %v", err)
	}
	if len(matches) != 1 || matches[0].RunID != originalRunID {
		t.Fatalf("concurrent processor handoff matches = %+v, want one original run %q", matches, originalRunID)
	}
}

func TestHandleInternalRunSubmissionResolvesIdentityBeforeOverload(t *testing.T) {
	rt, handler := testAPISetup(t)
	t.Setenv("RUNTIME_MAX_PROCESSOR_RUNS", "1")
	ctx := context.Background()

	submission := processorHandoffSubmissionFixture("owner-overload", "processor-running", "processor", []string{"source-item-running"})
	// Simulate a running run admitted before request identity was projected into
	// object metadata. Its body remains durable, and no new-version UpdateRun is
	// allowed to backfill the projection before the retry arrives.
	now := time.Now().UTC()
	existing := types.RunRecord{
		RunID:        "run-legacy-processor-running",
		AgentID:      metadataStringValue(submission.Metadata, runMetadataAgentID),
		ChannelID:    metadataStringValue(submission.Metadata, runMetadataChannelID),
		AgentProfile: agentprofile.Processor,
		AgentRole:    agentprofile.Processor,
		OwnerID:      submission.OwnerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       submission.Prompt,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     cloneMetadata(submission.Metadata),
	}
	if err := rt.Store().CreateRun(ctx, existing); err != nil {
		t.Fatalf("create legacy seeded processor run: %v", err)
	}
	assertRunIdentityLivesOnlyInBodyForTest(t, rt, existing.RunID, "processor-running", "processor", agentprofile.Processor)

	duplicateW := postInternalRunSubmissionFixture(t, handler, submission)
	if duplicateW.Code != http.StatusAccepted {
		t.Fatalf("duplicate under overload status = %d, want 202; body=%s", duplicateW.Code, duplicateW.Body.String())
	}
	if got := decodeInternalRunSubmissionStatus(t, duplicateW).RunID; got != existing.RunID {
		t.Fatalf("duplicate under overload run id = %q, want %q", got, existing.RunID)
	}

	conflict := processorHandoffSubmissionFixture(submission.OwnerID, "processor-running", "processor", []string{"source-item-conflict"})
	conflictW := postInternalRunSubmissionFixture(t, handler, conflict)
	if conflictW.Code != http.StatusConflict {
		t.Fatalf("conflict under overload status = %d, want 409; body=%s", conflictW.Code, conflictW.Body.String())
	}
	partialID := processorHandoffSubmissionFixture(submission.OwnerID, "processor-partial-id", "processor", []string{"source-item-partial"})
	delete(partialID.Metadata, "ingestion_handoff_request_kind")
	partialIDW := postInternalRunSubmissionFixture(t, handler, partialID)
	if partialIDW.Code != http.StatusBadRequest {
		t.Fatalf("id-only ingestion identity status = %d, want 400; body=%s", partialIDW.Code, partialIDW.Body.String())
	}
	partialKind := processorHandoffSubmissionFixture(submission.OwnerID, "processor-partial-kind", "processor", []string{"source-item-partial"})
	delete(partialKind.Metadata, "ingestion_handoff_request_id")
	partialKindW := postInternalRunSubmissionFixture(t, handler, partialKind)
	if partialKindW.Code != http.StatusBadRequest {
		t.Fatalf("kind-only ingestion identity status = %d, want 400; body=%s", partialKindW.Code, partialKindW.Body.String())
	}
	invalidTypedIdentity := processorHandoffSubmissionFixture(submission.OwnerID, "processor-invalid-identity", "processor", []string{"source-item-partial"})
	invalidTypedIdentity.Metadata["ingestion_handoff_request_id"] = 42
	invalidTypedIdentityW := postInternalRunSubmissionFixture(t, handler, invalidTypedIdentity)
	if invalidTypedIdentityW.Code != http.StatusBadRequest {
		t.Fatalf("non-string ingestion identity status = %d, want 400; body=%s", invalidTypedIdentityW.Code, invalidTypedIdentityW.Body.String())
	}
	wrongProfile := processorHandoffSubmissionFixture(submission.OwnerID, "researcher-typed-handoff", "processor", []string{"source-item-partial"})
	wrongProfile.Metadata[runMetadataAgentProfile] = agentprofile.Researcher
	wrongProfile.Metadata[runMetadataAgentRole] = agentprofile.Researcher
	wrongProfileW := postInternalRunSubmissionFixture(t, handler, wrongProfile)
	if wrongProfileW.Code != http.StatusBadRequest {
		t.Fatalf("non-ingestion profile handoff identity status = %d, want 400; body=%s", wrongProfileW.Code, wrongProfileW.Body.String())
	}

	fresh := processorHandoffSubmissionFixture(submission.OwnerID, "processor-fresh", "processor", []string{"source-item-fresh"})
	freshW := postInternalRunSubmissionFixture(t, handler, fresh)
	if freshW.Code != http.StatusTooManyRequests {
		t.Fatalf("fresh submission under overload status = %d, want 429; body=%s", freshW.Code, freshW.Body.String())
	}
}
