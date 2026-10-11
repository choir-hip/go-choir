package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

const (
	defaultPollMaxItems = 20
	maxPollMaxItems     = 100
	pollSourceTimeout   = 90 * time.Second
)

// pollState keeps each source's conditional GET validators between polls in
// this process. A restart forgets them; the next poll refetches, and stable
// capture ids make the refetch write nothing new.
var pollState = struct {
	sync.Mutex
	bySource map[string]sources.Source
}{bySource: map[string]sources.Source{}}

type poller interface {
	Poll(ctx context.Context, source *sources.Source) (sources.PollResult, error)
}

// newPollSourceTool polls one registry source and writes its items as shared
// choir.web_capture objects in Store B through corpusd, bodies on host disk
// (Autopaper W1b2). It returns
// counts and capture ids, never bodies.
func newPollSourceTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		SourceID string `json:"source_id"`
		MaxItems int    `json:"max_items,omitempty"`
	}
	return toolregistry.Tool{Name: "poll_source",
		Description: "Poll one registry source into web_capture objects; returns counts and the new captures' ids, titles and URLs.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"source_id": map[string]any{"type": "string"},
			"max_items": map[string]any{"type": "integer", "minimum": 1},
		}, []string{"source_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode poll_source args: %w", err)
			}
			exec := toolregistry.ExecutionContextFrom(ctx)
			if exec.OwnerID == "" {
				return "", fmt.Errorf("poll_source missing owner context")
			}
			if deps.Captures == nil {
				return "", fmt.Errorf("poll_source: the shared capture store is not configured")
			}
			reg, err := deps.sourceRegistry()
			if err != nil {
				return "", err
			}
			source, ok := reg.Lookup(in.SourceID)
			if !ok {
				return "", fmt.Errorf("poll_source: unknown source %q; known ids include %s", in.SourceID, strings.Join(nearbySourceIDs(reg, in.SourceID, 10), ", "))
			}
			maxItems := in.MaxItems
			if maxItems <= 0 {
				maxItems = defaultPollMaxItems
			}
			if maxItems > maxPollMaxItems {
				maxItems = maxPollMaxItems
			}
			if source.MaxItemsPerPoll <= 0 || source.MaxItemsPerPoll > maxItems {
				source.MaxItemsPerPoll = maxItems
			}
			p, err := pollerFor(source, reg.UserAgent)
			if err != nil {
				return "", err
			}
			if err := deps.chargeEgressCall(ctx, "poll_source"); err != nil {
				return "", err
			}

			pollState.Lock()
			if prior, ok := pollState.bySource[source.ID]; ok {
				source.LastETag, source.LastModified, source.LastAuxCursor, source.LastPolled = prior.LastETag, prior.LastModified, prior.LastAuxCursor, prior.LastPolled
			}
			pollState.Unlock()

			pollCtx, cancel := context.WithTimeout(ctx, pollSourceTimeout)
			defer cancel()
			started := time.Now()
			polled, pollErr := p.Poll(pollCtx, &source)
			if pollErr == nil {
				pollState.Lock()
				pollState.bySource[source.ID] = source
				pollState.Unlock()
			}
			result := map[string]any{
				"source_id":    source.ID,
				"status":       polled.Fetch.Status,
				"status_code":  polled.Fetch.StatusCode,
				"fetched":      len(polled.Items),
				"not_modified": polled.Fetch.Status == "not_modified",
				"poll_ms":      time.Since(started).Milliseconds(),
			}
			if pollErr != nil {
				result["error"] = pollErr.Error()
				return toolregistry.ResultJSON(result)
			}
			written, err := deps.Captures.WriteCaptures(ctx, exec.OwnerID, exec.ComputerID, polled.Items)
			result["new"] = written.New
			result["repeated"] = written.Repeated
			result["skipped"] = written.Skipped
			result["captured"] = written.Captured
			result["total_ms"] = time.Since(started).Milliseconds()
			if err != nil {
				result["error"] = err.Error()
			}
			return toolregistry.ResultJSON(result)
		}}
}

func pollerFor(source sources.Source, userAgent string) (poller, error) {
	ua := source.EffectiveUserAgent(userAgent)
	switch source.Type {
	case sources.SourceTypeRSS:
		return sources.NewRSSPoller(ua), nil
	case sources.SourceTypeTelegram:
		return sources.NewTelegramScraper(ua), nil
	case sources.SourceTypeGDELT:
		return sources.NewGDELTFetcher(ua), nil
	default:
		return nil, fmt.Errorf("poll_source: source %s has type %q, which has no poller", source.ID, source.Type)
	}
}

func (d Dependencies) sourceRegistry() (sources.Registry, error) {
	if d.Sources != nil {
		return *d.Sources, nil
	}
	return sources.DefaultRegistry()
}

// nearbySourceIDs names known ids sharing the query's prefix (before ':'),
// or the first ids, so an unknown id error teaches the right spelling.
func nearbySourceIDs(reg sources.Registry, query string, limit int) []string {
	prefix := strings.SplitN(strings.TrimSpace(query), ":", 2)[0]
	var matched, all []string
	for _, s := range reg.Sources {
		all = append(all, s.ID)
		if prefix != "" && strings.HasPrefix(s.ID, prefix) {
			matched = append(matched, s.ID)
		}
	}
	if len(matched) == 0 {
		matched = all
	}
	sort.Strings(matched)
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched
}
