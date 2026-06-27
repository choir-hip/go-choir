package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type internalSourcecycledWebCapturesRequest struct {
	OwnerID    string         `json:"owner_id"`
	ComputerID string         `json:"computer_id,omitempty"`
	CycleID    string         `json:"cycle_id,omitempty"`
	Items      []sources.Item `json:"items"`
	Now        string         `json:"now,omitempty"`
}

type internalSourcecycledWebCapturesResponse struct {
	Status                    string `json:"status"`
	CaptureCount              int    `json:"capture_count"`
	SourceEntityCount         int    `json:"source_entity_count"`
	CapturedFromEdges         int    `json:"captured_from_edges"`
	SkippedItemCount          int    `json:"skipped_item_count"`
	SynthesisStatus           string `json:"synthesis_status,omitempty"`
	SynthesisDocID            string `json:"synthesis_doc_id,omitempty"`
	SynthesisRevisionID       string `json:"synthesis_revision_id,omitempty"`
	SynthesisClusterID        string `json:"synthesis_cluster_id,omitempty"`
	SynthesisClusterObjectID  string `json:"synthesis_cluster_object_id,omitempty"`
	SynthesisSourceCount      int    `json:"synthesis_source_count,omitempty"`
	SynthesisKnownSourceCount int    `json:"synthesis_known_source_count,omitempty"`
	SynthesisCandidateGroups  int    `json:"synthesis_candidate_groups,omitempty"`
	SynthesisClusterCount     int    `json:"synthesis_cluster_count,omitempty"`
	SynthesisRefreshedGroups  int    `json:"synthesis_refreshed_groups,omitempty"`
	SynthesisEditionRef       string `json:"synthesis_edition_ref,omitempty"`
	SynthesisSkipReason       string `json:"synthesis_skip_reason,omitempty"`
}

// HandleInternalSourcecycledWebCaptures projects source-service items into this
// runtime's durable objectgraph. It is internal-only; browser clients should
// consume the resulting objects through the normal Universal Wire read route.
func (h *APIHandler) HandleInternalSourcecycledWebCaptures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if h == nil || h.rt == nil || h.rt.ObjectGraph() == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "objectgraph unavailable"})
		return
	}
	var req internalSourcecycledWebCapturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		ownerID = universalWirePlatformOwnerID()
	}
	if ownerID != universalWirePlatformOwnerID() {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unsupported sourcecycled owner"})
		return
	}
	now := time.Now().UTC()
	if rawNow := strings.TrimSpace(req.Now); rawNow != "" {
		parsed, err := time.Parse(time.RFC3339Nano, rawNow)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid now timestamp"})
			return
		}
		now = parsed.UTC()
	}
	result, err := sourcegraph.WriteWebCaptureGraphObjects(r.Context(), h.rt.ObjectGraph(), req.Items, sourcegraph.WebCaptureGraphProjectionConfig{
		OwnerID:    ownerID,
		ComputerID: strings.TrimSpace(req.ComputerID),
		Now:        now,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	synthesis, err := h.rt.synthesizeUniversalWireLiveSourcecycledClusterFromGraphCaptures(r.Context(), now)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	synthesisStatus := "skipped"
	synthesisSkipReason := universalWireLiveSynthesisSkipReason(synthesis)
	if synthesis.Triggered {
		synthesisStatus = "ok"
		synthesisSkipReason = ""
	}
	status := universalWireLiveArrivalStatus{
		SchemaVersion:             objectgraph.UniversalWireLiveArrivalStatusSchemaVersion,
		BoundaryID:                universalWireLiveArrivalBoundaryID(req.CycleID, now),
		CycleID:                   strings.TrimSpace(req.CycleID),
		ObservedAt:                now.UTC().Format(time.RFC3339Nano),
		UpdatedAt:                 now.UTC().Format(time.RFC3339Nano),
		Phase:                     "web_captures_graph_written",
		Status:                    "ok",
		ObjectGraphMode:           "runtime_api",
		SourceItemCount:           len(req.Items),
		CaptureCount:              len(result.Captures),
		SourceEntityCount:         len(result.SourceEntities),
		CapturedFromEdges:         result.EdgeCount,
		SkippedItemCount:          result.Skipped,
		SynthesisStatus:           synthesisStatus,
		SynthesisDocID:            synthesis.Doc.DocID,
		SynthesisRevisionID:       synthesis.Revision.RevisionID,
		SynthesisClusterID:        synthesis.ClusterID,
		SynthesisClusterObjectID:  synthesis.ClusterObjectID,
		SynthesisSourceCount:      synthesis.SourceCount,
		SynthesisKnownSourceCount: synthesis.KnownSourceCount,
		SynthesisCandidateGroups:  synthesis.CandidateGroupCount,
		SynthesisClusterCount:     synthesis.ClusterCount,
		SynthesisRefreshedGroups:  synthesis.RefreshedGroupCount,
		SynthesisEditionRef:       synthesis.EditionRef,
		SynthesisSkipReason:       synthesisSkipReason,
	}
	if err := h.rt.recordUniversalWireLiveArrivalStatus(r.Context(), status, now); err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusCreated, internalSourcecycledWebCapturesResponse{
		Status:                    "ok",
		CaptureCount:              len(result.Captures),
		SourceEntityCount:         len(result.SourceEntities),
		CapturedFromEdges:         result.EdgeCount,
		SkippedItemCount:          result.Skipped,
		SynthesisStatus:           synthesisStatus,
		SynthesisDocID:            synthesis.Doc.DocID,
		SynthesisRevisionID:       synthesis.Revision.RevisionID,
		SynthesisClusterID:        synthesis.ClusterID,
		SynthesisClusterObjectID:  synthesis.ClusterObjectID,
		SynthesisSourceCount:      synthesis.SourceCount,
		SynthesisKnownSourceCount: synthesis.KnownSourceCount,
		SynthesisCandidateGroups:  synthesis.CandidateGroupCount,
		SynthesisClusterCount:     synthesis.ClusterCount,
		SynthesisRefreshedGroups:  synthesis.RefreshedGroupCount,
		SynthesisEditionRef:       synthesis.EditionRef,
		SynthesisSkipReason:       synthesisSkipReason,
	})
}

type universalWireLiveArrivalResponse struct {
	Status string                          `json:"status"`
	Latest *universalWireLiveArrivalStatus `json:"latest,omitempty"`
}

type universalWireLiveArrivalStatus struct {
	SchemaVersion             string `json:"schema_version"`
	BoundaryID                string `json:"boundary_id"`
	CycleID                   string `json:"cycle_id,omitempty"`
	ObservedAt                string `json:"observed_at"`
	UpdatedAt                 string `json:"updated_at"`
	Phase                     string `json:"phase"`
	Status                    string `json:"status"`
	ObjectGraphMode           string `json:"objectgraph_mode,omitempty"`
	SourceItemCount           int    `json:"source_item_count"`
	CaptureCount              int    `json:"capture_count"`
	SourceEntityCount         int    `json:"source_entity_count"`
	CapturedFromEdges         int    `json:"captured_from_edges"`
	SkippedItemCount          int    `json:"skipped_item_count"`
	SynthesisStatus           string `json:"synthesis_status,omitempty"`
	SynthesisDocID            string `json:"synthesis_doc_id,omitempty"`
	SynthesisRevisionID       string `json:"synthesis_revision_id,omitempty"`
	SynthesisClusterID        string `json:"synthesis_cluster_id,omitempty"`
	SynthesisClusterObjectID  string `json:"synthesis_cluster_object_id,omitempty"`
	SynthesisSourceCount      int    `json:"synthesis_source_count,omitempty"`
	SynthesisKnownSourceCount int    `json:"synthesis_known_source_count,omitempty"`
	SynthesisCandidateGroups  int    `json:"synthesis_candidate_groups,omitempty"`
	SynthesisClusterCount     int    `json:"synthesis_cluster_count,omitempty"`
	SynthesisRefreshedGroups  int    `json:"synthesis_refreshed_groups,omitempty"`
	SynthesisEditionRef       string `json:"synthesis_edition_ref,omitempty"`
	SynthesisSkipReason       string `json:"synthesis_skip_reason,omitempty"`
}

func (h *APIHandler) HandleUniversalWireLiveArrival(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	status, ok, err := h.rt.latestUniversalWireLiveArrivalStatus(r.Context())
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if !ok {
		writeAPIJSON(w, http.StatusOK, universalWireLiveArrivalResponse{Status: "unavailable"})
		return
	}
	writeAPIJSON(w, http.StatusOK, universalWireLiveArrivalResponse{Status: "available", Latest: &status})
}

func (rt *Runtime) recordUniversalWireLiveArrivalStatus(ctx context.Context, status universalWireLiveArrivalStatus, now time.Time) error {
	if rt == nil || rt.ObjectGraph() == nil {
		return nil
	}
	if status.SchemaVersion == "" {
		status.SchemaVersion = objectgraph.UniversalWireLiveArrivalStatusSchemaVersion
	}
	if status.BoundaryID == "" {
		status.BoundaryID = universalWireLiveArrivalBoundaryID(status.CycleID, now)
	}
	if status.ObservedAt == "" {
		status.ObservedAt = now.UTC().Format(time.RFC3339Nano)
	}
	status.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	if status.Status == "" {
		status.Status = "ok"
	}
	if status.Phase == "" {
		status.Phase = "web_captures_graph_written"
	}
	_, err := rt.ObjectGraph().CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:        objectgraph.UniversalWireLiveArrivalStatusObjectKind,
		OwnerID:     universalWirePlatformOwnerID(),
		IdentityKey: "latest",
		VersionID:   status.BoundaryID,
		Metadata:    status,
		Now:         now,
	})
	if err != nil {
		return fmt.Errorf("record universal wire live-arrival status: %w", err)
	}
	return nil
}

func (rt *Runtime) latestUniversalWireLiveArrivalStatus(ctx context.Context) (universalWireLiveArrivalStatus, bool, error) {
	if rt == nil || rt.ObjectGraph() == nil {
		return universalWireLiveArrivalStatus{}, false, nil
	}
	notTombstoned := false
	objects, err := rt.ObjectGraph().ListObjects(ctx, objectgraph.ListFilter{
		Kind:      objectgraph.UniversalWireLiveArrivalStatusObjectKind,
		OwnerID:   universalWirePlatformOwnerID(),
		Limit:     1,
		Tombstone: &notTombstoned,
	})
	if err != nil {
		return universalWireLiveArrivalStatus{}, false, fmt.Errorf("read universal wire live-arrival status: %w", err)
	}
	if len(objects) == 0 {
		return universalWireLiveArrivalStatus{}, false, nil
	}
	var status universalWireLiveArrivalStatus
	if err := json.Unmarshal(objects[0].Metadata, &status); err != nil {
		return universalWireLiveArrivalStatus{}, false, fmt.Errorf("decode universal wire live-arrival status: %w", err)
	}
	if status.UpdatedAt == "" && !objects[0].UpdatedAt.IsZero() {
		status.UpdatedAt = objects[0].UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return status, true, nil
}

func universalWireLiveArrivalBoundaryID(cycleID string, observedAt time.Time) string {
	if cycleID = strings.TrimSpace(cycleID); cycleID != "" {
		return cycleID
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	return "sourcecycled-observed-" + observedAt.UTC().Format("20060102T150405.000000000Z")
}

type universalWireGraphSynthesisResult struct {
	Triggered           bool
	Doc                 types.Document
	Revision            types.Revision
	EditionRef          string
	ClusterID           string
	ClusterObjectID     string
	SourceCount         int
	KnownSourceCount    int
	CandidateGroupCount int
	ClusterCount        int
	RefreshedGroupCount int
}

const universalWireLiveSourcecycledClusterID = "sourcecycled-live"
const universalWireLiveSourcecycledCaptureSynthesisLimit = 768
const universalWireSemanticSignatureMaxTopics = 4
const universalWireSemanticSignatureMaxSignals = 12

func (rt *Runtime) synthesizeUniversalWireLiveSourcecycledClusterFromGraphCaptures(ctx context.Context, now time.Time) (universalWireGraphSynthesisResult, error) {
	if rt == nil || rt.ObjectGraph() == nil {
		return universalWireGraphSynthesisResult{}, nil
	}
	notTombstoned := false
	objects, err := rt.ObjectGraph().ListObjects(ctx, objectgraph.ListFilter{
		Kind:      objectgraph.WebCaptureObjectKind,
		OwnerID:   universalWirePlatformOwnerID(),
		Limit:     universalWireLiveSourcecycledCaptureSynthesisLimit,
		Tombstone: &notTombstoned,
	})
	if err != nil {
		return universalWireGraphSynthesisResult{}, fmt.Errorf("select universal wire graph captures: %w", err)
	}
	sources, err := rt.universalWireSynthesisSourcesFromGraphCaptures(ctx, objects)
	if err != nil {
		return universalWireGraphSynthesisResult{}, err
	}
	groups, grouping := universalWireDeterministicStorySourceGroupsWithDiagnostics(sources)
	result := universalWireGraphSynthesisResult{
		SourceCount:         len(sources),
		KnownSourceCount:    grouping.KnownSourceCount,
		CandidateGroupCount: grouping.CandidateGroupCount,
		ClusterCount:        len(groups),
	}
	if len(groups) == 0 {
		return result, nil
	}
	out := result
	synthesizedClusterCount := 0
	for _, group := range groups {
		group.ClusterID = rt.resolveUniversalWireStoryClusterID(ctx, group)
		semanticState := rt.universalWireSemanticStoryState(ctx, group.ClusterID, group.Sources, now)
		if semanticState.LatestChange.ChangeType == "state_refreshed" {
			out.RefreshedGroupCount++
			continue
		}
		doc, rev, editionRef, err := rt.synthesizeUniversalWireSourceClusterTextureArticle(ctx, universalWireSynthesisClusterRequest{
			ClusterID:     group.ClusterID,
			Headline:      semanticState.Headline,
			Summary:       semanticState.Summary,
			Tension:       semanticState.Tension,
			Sources:       group.Sources,
			SemanticState: semanticState,
			Now:           now,
		})
		if err != nil {
			return universalWireGraphSynthesisResult{}, err
		}
		synthesizedClusterCount++
		out = universalWireGraphSynthesisResult{
			Triggered:           true,
			Doc:                 doc,
			Revision:            rev,
			EditionRef:          editionRef,
			ClusterID:           group.ClusterID,
			ClusterObjectID:     universalWireStoryClusterObjectID(universalWirePlatformOwnerID(), group.ClusterID),
			SourceCount:         len(group.Sources),
			KnownSourceCount:    grouping.KnownSourceCount,
			CandidateGroupCount: grouping.CandidateGroupCount,
			ClusterCount:        synthesizedClusterCount,
			RefreshedGroupCount: out.RefreshedGroupCount,
		}
	}
	return out, nil
}

func universalWireLiveSynthesisSkipReason(result universalWireGraphSynthesisResult) string {
	switch {
	case result.SourceCount < 2:
		return "fewer than two graph-backed synthesis sources"
	case result.KnownSourceCount == 0:
		return "no graph-backed synthesis sources matched known story concepts"
	case result.ClusterCount == 0:
		return "no deterministic story group reached two sources with a shared topic and story signal"
	case result.RefreshedGroupCount >= result.ClusterCount:
		return "all deterministic story groups were already current"
	default:
		return "no deterministic story group produced a new or updated Texture article"
	}
}

type universalWireSemanticStoryState struct {
	SchemaVersion     string                           `json:"schema_version"`
	WorldModelKind    string                           `json:"world_model_kind"`
	StoryID           string                           `json:"story_id"`
	ClusterID         string                           `json:"cluster_id"`
	SemanticSignature []string                         `json:"semantic_signature"`
	TopicConcepts     []string                         `json:"topic_concepts"`
	SignalConcepts    []string                         `json:"signal_concepts"`
	SourceItemIDs     []string                         `json:"source_item_ids"`
	SourceCaptureIDs  []string                         `json:"source_capture_ids,omitempty"`
	SourceCount       int                              `json:"source_count"`
	Languages         []string                         `json:"languages,omitempty"`
	Headline          string                           `json:"headline"`
	Summary           string                           `json:"summary"`
	Tension           string                           `json:"tension"`
	LatestChange      universalWireSemanticStoryChange `json:"latest_change"`
}

type universalWireSemanticStoryChange struct {
	ChangeType          string   `json:"change_type"`
	PreviousSourceCount int      `json:"previous_source_count"`
	CurrentSourceCount  int      `json:"current_source_count"`
	AddedSourceItemIDs  []string `json:"added_source_item_ids,omitempty"`
	AddedCaptureIDs     []string `json:"added_capture_ids,omitempty"`
	AddedConcepts       []string `json:"added_concepts,omitempty"`
	ChangedAt           string   `json:"changed_at"`
}

func (rt *Runtime) universalWireSemanticStoryState(ctx context.Context, clusterID string, sources []universalWireSynthesisSource, now time.Time) universalWireSemanticStoryState {
	sources = normalizedUniversalWireSynthesisSources(sources)
	signature, topics, signals := universalWireSemanticSignature(sources)
	sourceItemIDs := make([]string, 0, len(sources))
	sourceCaptureIDs := make([]string, 0, len(sources))
	languages := []string{}
	for _, source := range sources {
		if source.ItemID != "" && !containsWireString(sourceItemIDs, source.ItemID) {
			sourceItemIDs = append(sourceItemIDs, source.ItemID)
		}
		if source.CaptureObjectID != "" && !containsWireString(sourceCaptureIDs, source.CaptureObjectID) {
			sourceCaptureIDs = append(sourceCaptureIDs, source.CaptureObjectID)
		}
		if source.Language != "" && !containsWireString(languages, source.Language) {
			languages = append(languages, source.Language)
		}
	}
	sort.Strings(sourceItemIDs)
	sort.Strings(sourceCaptureIDs)
	sort.Strings(languages)

	previousState, hasPreviousState := rt.universalWirePreviousSemanticStorySnapshot(ctx, clusterID)
	previousSources := previousState.SourceItemIDs
	previousCaptures := previousState.SourceCaptureIDs
	previousConcepts := previousState.SemanticSignature
	addedSources := missingWireStrings(sourceItemIDs, previousSources)
	addedCaptures := missingWireStrings(sourceCaptureIDs, previousCaptures)
	addedConcepts := missingWireStrings(signature, previousConcepts)
	changeType := "story_created"
	if len(previousSources) > 0 {
		changeType = "state_refreshed"
		if len(addedSources) > 0 || len(addedConcepts) > 0 {
			changeType = "source_added"
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	storyID := stableSourceEntityID("universal_wire_semantic_story", strings.Join(signature, "|"))
	if hasPreviousState && strings.TrimSpace(previousState.StoryID) != "" {
		storyID = previousState.StoryID
	}
	state := universalWireSemanticStoryState{
		SchemaVersion:     "choir.universal_wire_story_cluster.semantic.v1",
		WorldModelKind:    "universal_wire_semantic_story",
		StoryID:           storyID,
		ClusterID:         strings.TrimSpace(clusterID),
		SemanticSignature: signature,
		TopicConcepts:     topics,
		SignalConcepts:    signals,
		SourceItemIDs:     sourceItemIDs,
		SourceCaptureIDs:  sourceCaptureIDs,
		SourceCount:       len(sourceItemIDs),
		Languages:         languages,
		LatestChange: universalWireSemanticStoryChange{
			ChangeType:          changeType,
			PreviousSourceCount: len(previousSources),
			CurrentSourceCount:  len(sourceItemIDs),
			AddedSourceItemIDs:  addedSources,
			AddedCaptureIDs:     addedCaptures,
			AddedConcepts:       addedConcepts,
			ChangedAt:           now.UTC().Format(time.RFC3339Nano),
		},
	}
	state.Headline = universalWireSemanticStoryHeadline(sources, state)
	state.Summary = universalWireSemanticStorySummary(sources, state)
	state.Tension = universalWireSemanticStoryTension(state)
	return state
}

func (rt *Runtime) universalWirePreviousSemanticStoryState(ctx context.Context, clusterID string) ([]string, []string, []string) {
	state, ok := rt.universalWirePreviousSemanticStorySnapshot(ctx, clusterID)
	if !ok {
		return nil, nil, nil
	}
	return append([]string(nil), state.SourceItemIDs...), append([]string(nil), state.SourceCaptureIDs...), append([]string(nil), state.SemanticSignature...)
}

func (rt *Runtime) universalWirePreviousSemanticStorySnapshot(ctx context.Context, clusterID string) (universalWireSemanticStoryState, bool) {
	if rt == nil || rt.ObjectGraph() == nil || strings.TrimSpace(clusterID) == "" {
		return universalWireSemanticStoryState{}, false
	}
	obj, err := rt.ObjectGraph().GetObject(ctx, universalWireStoryClusterObjectID(universalWirePlatformOwnerID(), clusterID))
	if err != nil {
		return universalWireSemanticStoryState{}, false
	}
	var state universalWireSemanticStoryState
	if err := json.Unmarshal(obj.Body, &state); err == nil && state.SchemaVersion != "" {
		return state, true
	}
	var meta map[string]any
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		return universalWireSemanticStoryState{}, false
	}
	return universalWireSemanticStoryState{
		StoryID:           metadataString(meta, "semantic_story_id"),
		SourceItemIDs:     metadataStringSlice(meta["source_item_ids"]),
		SourceCaptureIDs:  metadataStringSlice(meta["source_capture_ids"]),
		SemanticSignature: metadataStringSlice(meta["semantic_signature"]),
	}, true
}

func universalWireSemanticSignature(sources []universalWireSynthesisSource) ([]string, []string, []string) {
	concepts := map[string]bool{}
	for _, source := range sources {
		for concept := range universalWireStoryConceptSet(source) {
			concepts[concept] = true
		}
	}
	topics := []string{}
	signals := []string{}
	for concept := range concepts {
		switch {
		case universalWireStoryConceptIsTopic(concept):
			topics = append(topics, strings.TrimPrefix(concept, "topic:"))
		case universalWireStoryConceptIsSpecific(concept):
			signals = append(signals, strings.TrimPrefix(concept, "signal:"))
		}
	}
	sort.Strings(topics)
	sort.Strings(signals)
	if len(topics) > universalWireSemanticSignatureMaxTopics {
		topics = topics[:universalWireSemanticSignatureMaxTopics]
	}
	if len(signals) > universalWireSemanticSignatureMaxSignals {
		signals = signals[:universalWireSemanticSignatureMaxSignals]
	}
	signature := append([]string{}, topics...)
	signature = append(signature, signals...)
	return signature, topics, signals
}

func universalWireSemanticStoryHeadline(sources []universalWireSynthesisSource, state universalWireSemanticStoryState) string {
	if len(sources) == 0 {
		return "Developing story"
	}
	return truncateRunes(sources[0].Title, 96)
}

func universalWireSemanticStorySummary(sources []universalWireSynthesisSource, state universalWireSemanticStoryState) string {
	if len(sources) == 0 {
		return "The available reporting describes a developing story that remains open to revision."
	}
	summary := universalWireSynthesisSummaryFromSources(sources)
	if state.LatestChange.ChangeType == "source_added" && len(state.LatestChange.AddedSourceItemIDs) > 0 {
		added := universalWireLatestAddedSourceTitle(sources, state.LatestChange.AddedSourceItemIDs[0])
		if added != "" {
			return summary + " The latest arrival adds detail from " + added + "."
		}
	}
	return summary
}

func universalWireLatestAddedSourceTitle(sources []universalWireSynthesisSource, itemID string) string {
	for _, source := range sources {
		if source.ItemID == itemID && strings.TrimSpace(source.Title) != "" {
			return source.Title
		}
	}
	if len(sources) > 0 {
		return sources[len(sources)-1].Title
	}
	return "a later source"
}

func universalWireSemanticStoryTension(state universalWireSemanticStoryState) string {
	switch state.LatestChange.ChangeType {
	case "source_added":
		return "This article should be revised here while later reporting still fits the same event and should split only when a new timeline, location, or official explanation emerges."
	case "story_created":
		return "Later reporting should update this account if the timeline, affected people, or official explanation changes."
	default:
		return "This article remains open to revision as reporting develops."
	}
}

func missingWireStrings(current, previous []string) []string {
	seen := map[string]bool{}
	for _, value := range previous {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	out := []string{}
	for _, value := range current {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

type universalWireDeterministicStorySourceGroup struct {
	ClusterID string
	Sources   []universalWireSynthesisSource
	concepts  map[string]bool
}

type universalWireStoryGroupingDiagnostics struct {
	KnownSourceCount    int
	CandidateGroupCount int
}

func universalWireDeterministicStorySourceGroups(sources []universalWireSynthesisSource) []universalWireDeterministicStorySourceGroup {
	groups, _ := universalWireDeterministicStorySourceGroupsWithDiagnostics(sources)
	return groups
}

func universalWireDeterministicStorySourceGroupsWithDiagnostics(sources []universalWireSynthesisSource) ([]universalWireDeterministicStorySourceGroup, universalWireStoryGroupingDiagnostics) {
	var groups []universalWireDeterministicStorySourceGroup
	var diagnostics universalWireStoryGroupingDiagnostics
	for _, source := range normalizedUniversalWireSynthesisSources(sources) {
		concepts := universalWireStoryConceptSet(source)
		if len(concepts) == 0 {
			continue
		}
		diagnostics.KnownSourceCount++
		best := -1
		bestOverlap := 0
		for i := range groups {
			overlap, sameTopic, storyOverlap := universalWireStoryConceptOverlap(groups[i].concepts, concepts)
			if sameTopic && storyOverlap && overlap > bestOverlap {
				best = i
				bestOverlap = overlap
			}
		}
		if best >= 0 {
			groups[best].Sources = append(groups[best].Sources, source)
			for concept := range concepts {
				groups[best].concepts[concept] = true
			}
			continue
		}
		groups = append(groups, universalWireDeterministicStorySourceGroup{
			Sources:  []universalWireSynthesisSource{source},
			concepts: concepts,
		})
	}
	diagnostics.CandidateGroupCount = len(groups)
	out := make([]universalWireDeterministicStorySourceGroup, 0, len(groups))
	for _, group := range groups {
		if len(group.Sources) < 2 {
			continue
		}
		group.ClusterID = universalWireLiveSourcecycledClusterID + "-" + universalWireStoryClusterSlug(group.concepts)
		out = append(out, group)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Sources[0].FetchedAt.After(out[j].Sources[0].FetchedAt)
	})
	return out, diagnostics
}

func (rt *Runtime) resolveUniversalWireStoryClusterID(ctx context.Context, group universalWireDeterministicStorySourceGroup) string {
	fallback := strings.TrimSpace(group.ClusterID)
	if rt == nil || rt.ObjectGraph() == nil {
		return fallback
	}
	currentSourceIDs := universalWireSourceItemIDSet(group.Sources)
	if len(currentSourceIDs) == 0 {
		return fallback
	}
	notTombstoned := false
	clusters, err := rt.ObjectGraph().ListObjects(ctx, objectgraph.ListFilter{
		Kind:      objectgraph.UniversalWireStoryClusterObjectKind,
		OwnerID:   universalWirePlatformOwnerID(),
		Limit:     64,
		Tombstone: &notTombstoned,
	})
	if err != nil {
		return fallback
	}
	bestClusterID := ""
	bestOverlap := 0
	for _, cluster := range clusters {
		state, ok := universalWireSemanticStoryStateFromObject(cluster)
		if !ok {
			continue
		}
		overlap := 0
		for _, sourceID := range state.SourceItemIDs {
			if currentSourceIDs[sourceID] {
				overlap++
			}
		}
		if overlap > bestOverlap {
			bestOverlap = overlap
			bestClusterID = strings.TrimSpace(state.ClusterID)
		}
	}
	if bestOverlap > 0 && bestClusterID != "" {
		return bestClusterID
	}
	return fallback
}

func universalWireSourceItemIDSet(sources []universalWireSynthesisSource) map[string]bool {
	out := map[string]bool{}
	for _, source := range normalizedUniversalWireSynthesisSources(sources) {
		if source.ItemID != "" {
			out[source.ItemID] = true
		}
	}
	return out
}

func universalWireSemanticStoryStateFromObject(obj objectgraph.Object) (universalWireSemanticStoryState, bool) {
	var state universalWireSemanticStoryState
	if err := json.Unmarshal(obj.Body, &state); err == nil && strings.TrimSpace(state.ClusterID) != "" {
		return state, true
	}
	var meta map[string]any
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		return universalWireSemanticStoryState{}, false
	}
	clusterID := metadataString(meta, "cluster_id")
	if clusterID == "" {
		return universalWireSemanticStoryState{}, false
	}
	return universalWireSemanticStoryState{
		StoryID:       metadataString(meta, "semantic_story_id"),
		ClusterID:     clusterID,
		SourceItemIDs: metadataStringSlice(meta["source_item_ids"]),
	}, true
}

func universalWireStoryConceptOverlap(left, right map[string]bool) (int, bool, bool) {
	overlap := 0
	sameTopic := false
	storyOverlap := false
	for concept := range right {
		if left[concept] {
			overlap++
			switch {
			case universalWireStoryConceptIsTopic(concept):
				sameTopic = true
			case universalWireStoryConceptIsSpecific(concept):
				storyOverlap = true
			}
		}
	}
	return overlap, sameTopic, storyOverlap
}

func universalWireStoryClusterSlug(concepts map[string]bool) string {
	topics := []string{}
	specifics := []string{}
	for concept := range concepts {
		switch {
		case strings.HasPrefix(concept, "topic:"):
			topics = append(topics, strings.TrimPrefix(concept, "topic:"))
		case universalWireStoryConceptIsSpecific(concept):
			specifics = append(specifics, universalWireSlug(strings.TrimPrefix(concept, "signal:")))
		}
	}
	sort.Strings(topics)
	sort.Strings(specifics)
	tokens := append([]string{}, topics...)
	tokens = append(tokens, specifics...)
	if len(tokens) > 4 {
		tokens = tokens[:4]
	}
	if len(tokens) == 0 {
		return "uncategorized"
	}
	return strings.Join(tokens, "-")
}

func universalWireStoryConceptIsSpecific(concept string) bool {
	return strings.HasPrefix(concept, "signal:")
}

func universalWireStoryConceptIsTopic(concept string) bool {
	return strings.HasPrefix(concept, "topic:")
}

func universalWireStoryConceptSet(source universalWireSynthesisSource) map[string]bool {
	titleConcepts := universalWireKnownConceptSet(source.Title)
	concepts := map[string]bool{}
	titleTopics := map[string]bool{}
	titleHasTopic := false
	for concept := range titleConcepts {
		concepts[concept] = true
		if universalWireStoryConceptIsTopic(concept) {
			titleTopics[concept] = true
			titleHasTopic = true
		}
	}
	if len(titleConcepts) == 0 && universalWireBodyNegatesStoryConceptRelevance(source.Body) {
		return concepts
	}
	for concept := range universalWireKnownConceptSet(source.Body) {
		switch {
		case universalWireStoryConceptIsSpecific(concept):
			concepts[concept] = true
		case titleTopics[concept], !titleHasTopic:
			concepts[concept] = true
		}
	}
	return concepts
}

func universalWireBodyNegatesStoryConceptRelevance(body string) bool {
	normalized := strings.Join(universalWireStoryTokens(body), " ")
	for _, phrase := range []string{
		"no relation to",
		"not related to",
		"unrelated to",
		"sem relacao com",
		"sin relacion con",
		"sans rapport avec",
	} {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

func universalWireKnownConceptSet(text string) map[string]bool {
	concepts := map[string]bool{}
	for _, token := range universalWireStoryTokens(text) {
		tokenConcepts := universalWireStoryConcepts(token)
		for _, concept := range tokenConcepts {
			concepts[concept] = true
		}
	}
	return concepts
}

func universalWireSourcesHaveKnownStoryConcept(sources []universalWireSynthesisSource) bool {
	for _, source := range sources {
		if len(universalWireKnownConceptSet(strings.Join([]string{source.Title, source.Body}, " "))) > 0 {
			return true
		}
	}
	return false
}

func universalWireStoryTokens(text string) []string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(universalWireFoldRune(r))
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

func universalWireFoldRune(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ã', 'ä', 'å':
		return 'a'
	case 'ç':
		return 'c'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ñ':
		return 'n'
	case 'ó', 'ò', 'ô', 'õ', 'ö':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	default:
		return ' '
	}
}

func universalWireStoryConcepts(token string) []string {
	switch token {
	case "rail", "railway", "railroad", "ferroviario", "ferroviaire", "corredor", "corridor":
		return []string{"topic:transport", "signal:rail-corridor"}
	case "transport", "transit", "commuter", "commuters", "passenger", "passengers", "pasajeros", "estacion", "estaciones", "station", "stations", "bus", "buses", "drivers":
		return []string{"topic:transport"}
	case "harbor", "harbour", "port", "porto", "pilots", "pilot", "maritime":
		return []string{"topic:harbor"}
	case "channel", "tide", "vessel", "vessels", "cargo", "boats":
		return []string{"topic:harbor", "signal:harbor-access"}
	case "river", "gauges", "gauge":
		return []string{"topic:flood"}
	case "energy", "power", "grid", "electric", "electricity", "substation", "blackout":
		return []string{"topic:energy"}
	case "health", "hospital", "clinic", "patients", "patient", "vaccine", "disease":
		return []string{"topic:health"}
	case "reopen", "reopens", "reopened", "reopening", "reabre", "reabriu", "reprise", "restait", "partial", "partially", "parcial", "parcialmente", "partielle":
		return []string{"signal:reopening"}
	case "inspection", "inspections", "inspecoes", "revisiones", "checks", "soundings":
		return []string{"signal:inspection"}
	case "delay", "delays", "delayed", "demora", "demoras", "atrasos", "atrasaram", "slower":
		return []string{"signal:delay"}
	case "flood", "flooding", "floods", "enchentes", "chuvas", "rain":
		return []string{"signal:flood"}
	case "strike", "strikes", "walkout", "walkouts", "huelga":
		return []string{"signal:strike"}
	default:
		return nil
	}
}

func universalWireStoryTokenStopword(token string) bool {
	switch token {
	case "https", "http", "www", "example", "test", "com", "after", "about", "above", "while", "with", "without", "into", "from", "that", "this", "they", "their", "them", "were", "will", "para", "por", "las", "los", "uma", "que", "des", "les", "une", "and", "the", "for", "are", "was", "said", "officials", "authorities", "regional", "source", "report", "reports", "update", "updates":
		return true
	default:
		return false
	}
}

func universalWireLiveSynthesisHeadline(sources []universalWireSynthesisSource) string {
	if len(sources) == 0 {
		return "Developing story"
	}
	return truncateRunes(sources[0].Title, 96)
}

func universalWireLiveSynthesisSummary(sources []universalWireSynthesisSource) string {
	if len(sources) == 0 {
		return "The available reporting describes a developing story that remains open to revision."
	}
	return universalWireSynthesisSummaryFromSources(sources)
}

func (rt *Runtime) universalWireSynthesisSourcesFromGraphCaptures(ctx context.Context, captures []objectgraph.Object) ([]universalWireSynthesisSource, error) {
	out := make([]universalWireSynthesisSource, 0, len(captures))
	seen := map[string]bool{}
	for _, capture := range captures {
		source, ok, err := rt.universalWireSynthesisSourceFromGraphCapture(ctx, capture)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		key := firstNonEmpty(source.ItemID, source.CanonicalURL)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, source)
	}
	return out, nil
}

func (rt *Runtime) universalWireSynthesisSourceFromGraphCapture(ctx context.Context, capture objectgraph.Object) (universalWireSynthesisSource, bool, error) {
	metadata, err := objectgraph.WebCaptureMetadataFromObject(capture)
	if err != nil {
		return universalWireSynthesisSource{}, false, nil
	}
	body := strings.TrimSpace(string(capture.Body))
	if body == "" {
		return universalWireSynthesisSource{}, false, nil
	}
	fetchedAt := capture.UpdatedAt
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(metadata.FetchedAt)); err == nil {
		fetchedAt = parsed
	}
	source := universalWireSynthesisSource{
		CaptureObjectID: capture.CanonicalID,
		ItemID:          capture.CanonicalID,
		Title:           firstNonEmpty(metadata.Title, metadata.CanonicalURL, metadata.URL),
		URL:             metadata.URL,
		CanonicalURL:    firstNonEmpty(metadata.CanonicalURL, metadata.URL),
		Body:            body,
		FetchedAt:       fetchedAt,
	}
	if rt != nil && rt.ObjectGraph() != nil {
		fields, err := universalWireFirstCapturedFromSourceEntityFields(ctx, rt.ObjectGraph(), capture)
		if err != nil {
			return universalWireSynthesisSource{}, false, err
		}
		if fields.ItemID != "" {
			source.ItemID = fields.ItemID
			source.SourceID = fields.SourceID
			source.FetchID = fields.FetchID
			source.Language = fields.Language
			source.CanonicalURL = firstNonEmpty(fields.CanonicalURL, source.CanonicalURL)
			source.URL = firstNonEmpty(fields.URL, source.URL)
		}
	}
	source.SourceID = firstNonEmpty(source.SourceID, "objectgraph:web_capture")
	source.FetchID = firstNonEmpty(source.FetchID, capture.VersionID)
	return source, true, nil
}

type universalWireCapturedFromSourceFields struct {
	ItemID       string
	SourceID     string
	FetchID      string
	Language     string
	URL          string
	CanonicalURL string
}

func universalWireFirstCapturedFromSourceEntityFields(ctx context.Context, graph *objectgraph.Service, capture objectgraph.Object) (universalWireCapturedFromSourceFields, error) {
	if graph == nil || strings.TrimSpace(capture.CanonicalID) == "" {
		return universalWireCapturedFromSourceFields{}, nil
	}
	notTombstoned := false
	edges, err := graph.ListEdges(ctx, objectgraph.EdgeFilter{
		FromID:    capture.CanonicalID,
		Kind:      "captured_from",
		Tombstone: &notTombstoned,
		Limit:     1,
	})
	if err != nil {
		return universalWireCapturedFromSourceFields{}, err
	}
	for _, edge := range edges {
		sourceObj, err := graph.GetObject(ctx, edge.ToID)
		if err != nil {
			if err == objectgraph.ErrNotFound {
				continue
			}
			return universalWireCapturedFromSourceFields{}, err
		}
		if sourceObj.ObjectKind != "choir.source_entity" || sourceObj.Tombstone {
			continue
		}
		var meta struct {
			Target map[string]any `json:"target"`
		}
		if err := json.Unmarshal(sourceObj.Metadata, &meta); err != nil {
			return universalWireCapturedFromSourceFields{}, err
		}
		return universalWireCapturedFromSourceFields{
			ItemID:       wireStringFromMap(meta.Target, "item_id"),
			SourceID:     wireStringFromMap(meta.Target, "source_id"),
			FetchID:      wireStringFromMap(meta.Target, "fetch_id"),
			Language:     wireStringFromMap(meta.Target, "language"),
			URL:          wireStringFromMap(meta.Target, "url"),
			CanonicalURL: wireStringFromMap(meta.Target, "canonical_url"),
		}, nil
	}
	return universalWireCapturedFromSourceFields{}, nil
}
