package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type internalSourcecycledWebCapturesRequest struct {
	OwnerID    string         `json:"owner_id"`
	ComputerID string         `json:"computer_id,omitempty"`
	Items      []sources.Item `json:"items"`
	Now        string         `json:"now,omitempty"`
}

type internalSourcecycledWebCapturesResponse struct {
	Status                   string `json:"status"`
	CaptureCount             int    `json:"capture_count"`
	SourceEntityCount        int    `json:"source_entity_count"`
	CapturedFromEdges        int    `json:"captured_from_edges"`
	SkippedItemCount         int    `json:"skipped_item_count"`
	SynthesisStatus          string `json:"synthesis_status,omitempty"`
	SynthesisDocID           string `json:"synthesis_doc_id,omitempty"`
	SynthesisRevisionID      string `json:"synthesis_revision_id,omitempty"`
	SynthesisClusterID       string `json:"synthesis_cluster_id,omitempty"`
	SynthesisClusterObjectID string `json:"synthesis_cluster_object_id,omitempty"`
	SynthesisSourceCount     int    `json:"synthesis_source_count,omitempty"`
	SynthesisEditionRef      string `json:"synthesis_edition_ref,omitempty"`
	SynthesisSkipReason      string `json:"synthesis_skip_reason,omitempty"`
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
	synthesisSkipReason := "fewer than two eligible graph-backed source captures"
	if synthesis.Triggered {
		synthesisStatus = "ok"
		synthesisSkipReason = ""
	}
	writeAPIJSON(w, http.StatusCreated, internalSourcecycledWebCapturesResponse{
		Status:                   "ok",
		CaptureCount:             len(result.Captures),
		SourceEntityCount:        len(result.SourceEntities),
		CapturedFromEdges:        result.EdgeCount,
		SkippedItemCount:         result.Skipped,
		SynthesisStatus:          synthesisStatus,
		SynthesisDocID:           synthesis.Doc.DocID,
		SynthesisRevisionID:      synthesis.Revision.RevisionID,
		SynthesisClusterID:       synthesis.ClusterID,
		SynthesisClusterObjectID: synthesis.ClusterObjectID,
		SynthesisSourceCount:     synthesis.SourceCount,
		SynthesisEditionRef:      synthesis.EditionRef,
		SynthesisSkipReason:      synthesisSkipReason,
	})
}

type universalWireGraphSynthesisResult struct {
	Triggered       bool
	Doc             types.Document
	Revision        types.Revision
	EditionRef      string
	ClusterID       string
	ClusterObjectID string
	SourceCount     int
}

const universalWireLiveSourcecycledClusterID = "sourcecycled-live"

func (rt *Runtime) synthesizeUniversalWireLiveSourcecycledClusterFromGraphCaptures(ctx context.Context, now time.Time) (universalWireGraphSynthesisResult, error) {
	if rt == nil || rt.ObjectGraph() == nil {
		return universalWireGraphSynthesisResult{}, nil
	}
	notTombstoned := false
	objects, err := rt.ObjectGraph().ListObjects(ctx, objectgraph.ListFilter{
		Kind:      objectgraph.WebCaptureObjectKind,
		OwnerID:   universalWirePlatformOwnerID(),
		Limit:     24,
		Tombstone: &notTombstoned,
	})
	if err != nil {
		return universalWireGraphSynthesisResult{}, fmt.Errorf("select universal wire graph captures: %w", err)
	}
	sources, err := rt.universalWireSynthesisSourcesFromGraphCaptures(ctx, objects)
	if err != nil {
		return universalWireGraphSynthesisResult{}, err
	}
	if len(sources) < 2 {
		return universalWireGraphSynthesisResult{SourceCount: len(sources)}, nil
	}
	doc, rev, editionRef, err := rt.synthesizeUniversalWireSourceClusterTextureArticle(ctx, universalWireSynthesisClusterRequest{
		ClusterID: universalWireLiveSourcecycledClusterID,
		Headline:  universalWireLiveSynthesisHeadline(sources),
		Summary:   universalWireLiveSynthesisSummary(sources),
		Tension:   "Further reporting should revise this article if the timeline, affected people, or official account changes.",
		Sources:   sources,
		Now:       now,
	})
	if err != nil {
		return universalWireGraphSynthesisResult{}, err
	}
	return universalWireGraphSynthesisResult{
		Triggered:       true,
		Doc:             doc,
		Revision:        rev,
		EditionRef:      editionRef,
		ClusterID:       universalWireLiveSourcecycledClusterID,
		ClusterObjectID: universalWireStoryClusterObjectID(universalWirePlatformOwnerID(), universalWireLiveSourcecycledClusterID),
		SourceCount:     len(sources),
	}, nil
}

func universalWireLiveSynthesisHeadline(sources []universalWireSynthesisSource) string {
	if len(sources) == 0 {
		return "Developing story from incoming reports"
	}
	return "Multiple reports converge on " + truncateRunes(sources[0].Title, 90)
}

func universalWireLiveSynthesisSummary(sources []universalWireSynthesisSource) string {
	if len(sources) == 0 {
		return "Incoming reports point to a developing story that needs continued source-grounded revision."
	}
	if len(sources) == 1 {
		return fmt.Sprintf("One incoming report points to a developing story: %s.", sources[0].Title)
	}
	count := "Two"
	if len(sources) > 2 {
		count = fmt.Sprintf("%d", len(sources))
	}
	return fmt.Sprintf("%s incoming reports point to the same developing story. %s provides the lead signal, while %s adds a second angle for readers.", count, sources[0].Title, sources[1].Title)
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
		if fields.ItemID == "" {
			return universalWireSynthesisSource{}, false, nil
		}
		source.ItemID = firstNonEmpty(fields.ItemID, source.ItemID)
		source.SourceID = fields.SourceID
		source.FetchID = fields.FetchID
		source.Language = fields.Language
		source.CanonicalURL = firstNonEmpty(fields.CanonicalURL, source.CanonicalURL)
		source.URL = firstNonEmpty(fields.URL, source.URL)
	}
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
