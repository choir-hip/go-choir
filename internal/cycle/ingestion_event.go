package cycle

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/wire/processorkey"
)

const (
	IngestionOriginSourceFetch = "source_fetch"
	IngestionOriginPromptBar   = "prompt_bar"
)

// IngestionEvent is the only lawful activation record for wire processor dispatch.
// Story creation must trace to a persisted source artifact and fetch provenance.
type IngestionEvent struct {
	EventID     string
	CycleID     string
	ArtifactID  string
	SourceID    string
	FetchID     string
	ContentHash string
	DedupeKey   string
	Origin      string
	CreatedAt   time.Time
}

// BuildIngestionEventsFromItems materializes ingestion events for newly persisted
// source items produced by a sourcecycled fetch cycle.
func BuildIngestionEventsFromItems(cycleID string, items []sources.Item, now time.Time) []IngestionEvent {
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" || len(items) == 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out := make([]IngestionEvent, 0, len(items))
	for _, item := range items {
		event, err := NewIngestionEventFromItem(cycleID, item, now)
		if err != nil {
			continue
		}
		out = append(out, event)
	}
	return out
}

func NewIngestionEventFromItem(cycleID string, item sources.Item, now time.Time) (IngestionEvent, error) {
	cycleID = strings.TrimSpace(cycleID)
	artifactID := strings.TrimSpace(item.ID)
	sourceID := strings.TrimSpace(item.SourceID)
	if cycleID == "" || artifactID == "" || sourceID == "" {
		return IngestionEvent{}, fmt.Errorf("cycle id, artifact id, and source id are required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	dedupeKey := artifactID
	contentHash := strings.TrimSpace(item.ContentHash)
	if contentHash == "" {
		contentHash = sources.ContentHash(item.Title, item.Body, item.CanonicalURL, item.URL)
	}
	eventID := processorkey.StableRequestID("ingestionevt", cycleID, artifactID, sourceID, contentHash)
	return IngestionEvent{
		EventID:     eventID,
		CycleID:     cycleID,
		ArtifactID:  artifactID,
		SourceID:    sourceID,
		FetchID:     strings.TrimSpace(item.FetchID),
		ContentHash: contentHash,
		DedupeKey:   dedupeKey,
		Origin:      IngestionOriginSourceFetch,
		CreatedAt:   now,
	}, nil
}

func ValidateIngestionEventOrigin(origin string) error {
	switch strings.TrimSpace(origin) {
	case IngestionOriginSourceFetch:
		return nil
	case IngestionOriginPromptBar:
		return fmt.Errorf("prompt-bar submissions cannot emit ingestion events")
	default:
		return fmt.Errorf("unsupported ingestion event origin %q", origin)
	}
}

// ProcessorRequestEligibleForDispatch requires processor handoffs to carry
// ingestion-event activation refs produced by a source fetch cycle.
func ProcessorRequestEligibleForDispatch(req ProcessorRequest) bool {
	return len(req.IngestionEventIDs) > 0
}

func ingestionEventIDsForItems(events []IngestionEvent, itemIDs []string) []string {
	if len(events) == 0 || len(itemIDs) == 0 {
		return nil
	}
	byArtifact := map[string]string{}
	for _, event := range events {
		byArtifact[strings.TrimSpace(event.ArtifactID)] = event.EventID
	}
	out := make([]string, 0, len(itemIDs))
	seen := map[string]bool{}
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		eventID := byArtifact[itemID]
		if eventID == "" || seen[eventID] {
			continue
		}
		seen[eventID] = true
		out = append(out, eventID)
	}
	return out
}
