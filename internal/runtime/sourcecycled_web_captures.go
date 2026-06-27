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
	SynthesisClusterCount    int    `json:"synthesis_cluster_count,omitempty"`
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
		SynthesisClusterCount:    synthesis.ClusterCount,
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
	ClusterCount    int
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
	groups := universalWireDeterministicStorySourceGroups(sources)
	if len(groups) == 0 {
		if len(sources) >= 2 && !universalWireSourcesHaveKnownStoryConcept(sources) {
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
				ClusterCount:    1,
			}, nil
		}
		return universalWireGraphSynthesisResult{SourceCount: len(sources)}, nil
	}
	var out universalWireGraphSynthesisResult
	for _, group := range groups {
		doc, rev, editionRef, err := rt.synthesizeUniversalWireSourceClusterTextureArticle(ctx, universalWireSynthesisClusterRequest{
			ClusterID: group.ClusterID,
			Headline:  universalWireLiveSynthesisHeadline(group.Sources),
			Summary:   universalWireLiveSynthesisSummary(group.Sources),
			Tension:   "Further reporting should revise this article if the timeline, affected people, or official account changes.",
			Sources:   group.Sources,
			Now:       now,
		})
		if err != nil {
			return universalWireGraphSynthesisResult{}, err
		}
		out = universalWireGraphSynthesisResult{
			Triggered:       true,
			Doc:             doc,
			Revision:        rev,
			EditionRef:      editionRef,
			ClusterID:       group.ClusterID,
			ClusterObjectID: universalWireStoryClusterObjectID(universalWirePlatformOwnerID(), group.ClusterID),
			SourceCount:     len(group.Sources),
			ClusterCount:    out.ClusterCount + 1,
		}
	}
	return out, nil
}

type universalWireDeterministicStorySourceGroup struct {
	ClusterID string
	Sources   []universalWireSynthesisSource
	concepts  map[string]bool
}

func universalWireDeterministicStorySourceGroups(sources []universalWireSynthesisSource) []universalWireDeterministicStorySourceGroup {
	var groups []universalWireDeterministicStorySourceGroup
	for _, source := range normalizedUniversalWireSynthesisSources(sources) {
		concepts := universalWireStoryConceptSet(source)
		if len(concepts) == 0 {
			continue
		}
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
	return out
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
	text := strings.Join([]string{source.Title, source.Body, source.CanonicalURL, source.URL}, " ")
	concepts := map[string]bool{}
	fallback := map[string]bool{}
	for _, token := range universalWireStoryTokens(text) {
		tokenConcepts := universalWireStoryConcepts(token)
		if len(tokenConcepts) > 0 {
			for _, concept := range tokenConcepts {
				concepts[concept] = true
			}
			continue
		}
		if !universalWireStoryTokenStopword(token) && len(token) >= 5 {
			fallback[token] = true
		}
	}
	if len(concepts) > 0 {
		return concepts
	}
	concepts = fallback
	return concepts
}

func universalWireSourcesHaveKnownStoryConcept(sources []universalWireSynthesisSource) bool {
	for _, source := range sources {
		text := strings.Join([]string{source.Title, source.Body, source.CanonicalURL, source.URL}, " ")
		for _, token := range universalWireStoryTokens(text) {
			if len(universalWireStoryConcepts(token)) > 0 {
				return true
			}
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
	case "rail", "railway", "railroad", "train", "trains", "ferroviario", "ferroviaire", "corredor", "corridor":
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
		return "The available sources describe a developing story that needs continued source-grounded revision."
	}
	if len(sources) == 1 {
		return fmt.Sprintf("%s gives the clearest current account.", sources[0].Title)
	}
	return fmt.Sprintf("%s gives the clearest current account, while %s adds a second sourced angle.", sources[0].Title, sources[1].Title)
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
