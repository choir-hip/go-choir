package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// This file historically carried the deterministic Universal Wire synthesis
// scaffold: a template-prose generator, source-label headline/summary helpers,
// direct Texture revision creation, edition linking, platform publication, and
// a story-cluster upsert. That machinery bypassed the processor -> Texture
// agent -> LLM provider pipeline and produced template copy instead of real
// synthesis. It was deleted as part of the Universal Wire heresy deletion v1
// (docs/mission-heresy-deletion-v1.md).
//
// synthesizeUniversalWireSourceClusterTextureArticle retains its call signature
// so sourcecycled ingestion and tests compile against it, but its body now
// dispatches into the agent pipeline instead of synthesizing directly: it
// translates the cluster request into sources.Items, derives the processor
// key/batch (mirroring cycle.sourceProcessorKey, which runtime cannot import
// because cycle -> provider -> runtime), and submits a processor run. The
// processor agent decides whether to open a Texture story; the Texture agent
// writes the revision via the LLM provider; wire publication/reconciler
// carries it to the edition.
//
// The processor-key derivation is inlined here rather than imported from cycle
// to avoid the cycle->provider->runtime import cycle. It mirrors
// cycle.sourceProcessorKey/cycle.stableRequestID; if cycle ever moves to a
// leaf package both can import, this should call it instead. Tracked as
// consolidation debt for the successor agent-pipeline mission.

type universalWireSynthesisClusterRequest struct {
	ClusterID         string
	Headline          string
	Summary           string
	Tension           string
	PlatformRoutePath string
	Sources           []universalWireSynthesisSource
	Now               time.Time
}

type universalWireSynthesisSource struct {
	CaptureObjectID string
	ItemID          string
	SourceID        string
	FetchID         string
	Title           string
	URL             string
	CanonicalURL    string
	Language        string
	Body            string
	FetchedAt       time.Time
}

// universalWireProcessorDispatchRunID is a sentinel returned in the edition-ref
// slot when a processor run was dispatched but no Texture revision exists yet.
// The processor agent produces the article asynchronously; callers should not
// treat the absence of a revision as a synthesis failure.
const universalWireProcessorDispatchRunID = "processor_dispatched"

// maxProcessorBatchItems mirrors cycle.maxProcessorBatchItems: the largest
// source-item batch a single processor request may cover.
const maxProcessorBatchItems = 50

// synthesizeUniversalWireSourceClusterTextureArticle is the legacy direct
// synthesis entry point. It no longer synthesizes article copy itself; instead
// it dispatches the cluster into the agent pipeline (processor run -> Texture
// agent -> publication/reconciler). The processor agent decides whether to
// open a Texture story and the Texture agent writes the revision via the LLM
// provider, so this function returns the dispatched run handle rather than a
// finished Document/Revision. On a successful dispatch it returns an empty
// Document/Revision, the dispatch sentinel as the edition-ref, and a nil
// error; callers inspect the sentinel to know a run is in flight.
func (rt *Runtime) synthesizeUniversalWireSourceClusterTextureArticle(ctx context.Context, req universalWireSynthesisClusterRequest) (types.Document, types.Revision, string, error) {
	if rt == nil {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("universal wire processor dispatch: runtime unavailable")
	}
	now := req.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	items, err := universalWireSynthesisSourcesToItems(req.Sources, now)
	if err != nil {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("universal wire processor dispatch: %w", err)
	}
	if len(items) == 0 {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("universal wire processor dispatch: no source items to dispatch")
	}
	cycleID := strings.TrimSpace(req.ClusterID)
	if cycleID == "" {
		cycleID = "universal-wire-dispatch:" + uuid.NewString()
	}
	requests := buildUniversalWireProcessorRequests(cycleID, items, now)
	if len(requests) == 0 {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("universal wire processor dispatch: no processor requests derived")
	}
	ownerID := universalWirePlatformOwnerID()
	// Dispatch each processor request. Sources are batched by processor key
	// (vertical:region:type), so a multi-vertical cluster can fan out into
	// several processor runs. Each dispatched run leaves durable per-source-item
	// work items via beginWireProcessorSourceDecisionWorkItems (invoked inside
	// createRunWithMetadata when agent_profile=processor). The last dispatched
	// run id is returned as the witness handle.
	var lastRunID string
	for _, processorReq := range requests {
		metadata := map[string]any{
			runMetadataAgentProfile:           AgentProfileProcessor,
			runMetadataAgentRole:              AgentProfileProcessor,
			runMetadataProcessorKey:           processorReq.processorKey,
			"source_item_ids":                 processorReq.sourceItemIDs,
			"ingestion_handoff_request_id":    processorReq.requestID,
			"ingestion_handoff_cycle_id":      cycleID,
			"ingestion_handoff_request_kind":  "synthesis_cluster",
			"source_network_cycle_id":         cycleID,
			"continuity_ref":                  processorReq.continuityRef,
			"universal_wire_synthesis":        true,
			"universal_wire_story_cluster_id": cycleID,
			"request_source":                  "universal_wire_dispatch",
		}
		rec, err := rt.StartRunWithMetadata(ctx, processorReq.prompt, ownerID, metadata)
		if err != nil {
			return types.Document{}, types.Revision{}, "", fmt.Errorf("universal wire processor dispatch: submit processor run for %s: %w", processorReq.processorKey, err)
		}
		if rec != nil {
			lastRunID = rec.RunID
		}
	}
	if lastRunID == "" {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("universal wire processor dispatch: no processor run id returned")
	}
	return types.Document{}, types.Revision{}, universalWireProcessorDispatchRunID + ":" + lastRunID, nil
}

type universalWireProcessorRequest struct {
	requestID     string
	processorKey  string
	sourceItemIDs []string
	continuityRef string
	prompt        string
}

// buildUniversalWireProcessorRequests derives processor requests from source
// items, mirroring cycle.BuildIngestionHandoff: it groups items by processor
// key (vertical:region:type), batches each group up to maxProcessorBatchItems,
// and emits one request per batch with a stable id, continuity ref, and the
// standard processor handoff prompt.
func buildUniversalWireProcessorRequests(cycleID string, items []sources.Item, now time.Time) []universalWireProcessorRequest {
	if strings.TrimSpace(cycleID) == "" || len(items) == 0 {
		return nil
	}
	batches := map[string][]sources.Item{}
	for _, item := range items {
		key := universalWireSourceProcessorKey(item)
		batches[key] = append(batches[key], item)
	}
	keys := make([]string, 0, len(batches))
	for key := range batches {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]universalWireProcessorRequest, 0, len(keys))
	for _, key := range keys {
		itemsForKey := batches[key]
		for batchIndex, batch := range chunkUniversalWireItems(itemsForKey, maxProcessorBatchItems) {
			sourceItemIDs := universalWireOrderedSourceItemIDs(batch)
			out = append(out, universalWireProcessorRequest{
				requestID:     universalWireStableRequestID("processor", cycleID, key, fmt.Sprintf("%d", batchIndex)),
				processorKey:  key,
				sourceItemIDs: sourceItemIDs,
				continuityRef: "sourcecycled://processor/" + key + "/latest",
				prompt:        universalWireProcessorHandoffPrompt(key, batch),
			})
		}
	}
	return out
}

// universalWireSourceProcessorKey mirrors cycle.sourceProcessorKey: GDELT items
// route to the global firehose processor by region; everything else routes by
// primary vertical:region:type. Kept in sync with cycle so dispatched runs land
// in the same processor batch the source-service handoff would produce.
func universalWireSourceProcessorKey(item sources.Item) string {
	if item.SourceType == sources.SourceTypeGDELT {
		region := strings.TrimSpace(strings.ToLower(item.Region))
		if region == "" {
			region = "global"
		}
		return "processor:global_firehose:" + universalWireSafeKeyPart(region) + ":gdelt"
	}
	vertical := "general"
	for _, candidate := range item.Verticals {
		candidate = strings.TrimSpace(strings.ToLower(candidate))
		if candidate != "" {
			vertical = universalWireSafeKeyPart(candidate)
			break
		}
	}
	region := strings.TrimSpace(strings.ToLower(item.Region))
	if region == "" {
		region = "global"
	}
	sourceType := strings.TrimSpace(strings.ToLower(string(item.SourceType)))
	if sourceType == "" {
		sourceType = "source"
	}
	return "processor:" + vertical + ":" + universalWireSafeKeyPart(region) + ":" + universalWireSafeKeyPart(sourceType)
}

func universalWireSafeKeyPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		return "unknown"
	}
	return out
}

func universalWireProcessorHandoffPrompt(key string, items []sources.Item) string {
	return fmt.Sprintf("Processor %s: ingest %d SourceItems by handle, update live understanding, preserve unresolved questions/watch items, and spawn Texture agents when a story should be opened or revised.", key, len(items))
}

func universalWireStableRequestID(kind, cycleID string, parts ...string) string {
	segments := append([]string{kind, cycleID}, parts...)
	return kind + "_" + sources.ContentHash(segments...)[:24]
}

func universalWireOrderedSourceItemIDs(items []sources.Item) []string {
	ids := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func chunkUniversalWireItems(items []sources.Item, size int) [][]sources.Item {
	if size <= 0 || len(items) <= size {
		return [][]sources.Item{items}
	}
	out := [][]sources.Item{}
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[start:end])
	}
	return out
}

// universalWireSynthesisSourcesToItems translates the cluster request's
// synthesis sources into sources.Items ready for processor-key derivation.
// Items without a title or body are dropped since the processor cannot act on
// them.
func universalWireSynthesisSourcesToItems(sourcesIn []universalWireSynthesisSource, now time.Time) ([]sources.Item, error) {
	if len(sourcesIn) == 0 {
		return nil, fmt.Errorf("no sources")
	}
	items := make([]sources.Item, 0, len(sourcesIn))
	seen := map[string]bool{}
	for _, src := range sourcesIn {
		itemID := strings.TrimSpace(src.ItemID)
		if itemID == "" {
			itemID = strings.TrimSpace(src.CanonicalURL)
		}
		if itemID == "" {
			itemID = strings.TrimSpace(src.Title)
		}
		if itemID == "" || seen[itemID] {
			continue
		}
		seen[itemID] = true
		title := strings.TrimSpace(src.Title)
		body := strings.TrimSpace(src.Body)
		if title == "" || body == "" {
			continue
		}
		fetchedAt := src.FetchedAt
		if fetchedAt.IsZero() {
			fetchedAt = now
		}
		items = append(items, sources.Item{
			ID:           itemID,
			SourceID:     firstNonEmpty(strings.TrimSpace(src.SourceID), "objectgraph:web_capture"),
			SourceType:   universalWireDispatchSourceType(strings.TrimSpace(src.SourceID)),
			FetchID:      strings.TrimSpace(src.FetchID),
			OriginalID:   itemID,
			Title:        title,
			Body:         body,
			URL:          strings.TrimSpace(src.URL),
			CanonicalURL: strings.TrimSpace(src.CanonicalURL),
			Published:    fetchedAt,
			FetchedAt:    fetchedAt,
			Language:     strings.TrimSpace(src.Language),
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no sources with title and body")
	}
	return items, nil
}

// universalWireDispatchSourceType derives a sources.SourceType for the handoff
// from the capture's source id. Web captures ingested through the sourcecycled
// graph are RSS-derived by default; source ids that name a known source type
// map to that type so the processor key routes them to the right batch.
func universalWireDispatchSourceType(sourceID string) sources.SourceType {
	switch strings.ToLower(strings.TrimSpace(sourceID)) {
	case "telegram", "telegram:web_capture", "objectgraph:telegram":
		return sources.SourceTypeTelegram
	case "gdelt", "objectgraph:gdelt":
		return sources.SourceTypeGDELT
	case "polymarket", "objectgraph:polymarket":
		return sources.SourceTypePolymarket
	default:
		return sources.SourceTypeRSS
	}
}
