package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
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
			report, err := PollSourceOnce(ctx, deps, exec.OwnerID, exec.ComputerID, in.SourceID, in.MaxItems, func() error {
				return deps.chargeEgressCall(ctx, "poll_source")
			})
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(report)
		}}
}

// PollReport is one poll's outcome and resource use. A fetch or capture
// write failure is reported in Error; the poll itself still happened.
type PollReport struct {
	SourceID    string                     `json:"source_id"`
	Status      string                     `json:"status"`
	StatusCode  int                        `json:"status_code"`
	Fetched     int                        `json:"fetched"`
	NotModified bool                       `json:"not_modified"`
	ItemBytes   int                        `json:"item_bytes"`
	PollMS      int64                      `json:"poll_ms"`
	New         int                        `json:"new"`
	Repeated    int                        `json:"repeated"`
	Skipped     int                        `json:"skipped"`
	Captured    []sourcegraph.CapturedItem `json:"captured,omitempty"`
	TotalMS     int64                      `json:"total_ms"`
	Error       string                     `json:"error,omitempty"`
}

// PollSourceOnce polls one registry source and writes its new items as
// shared captures through deps.Captures. research's choir.PollSource and the
// source schedule (Autopaper W1c) both call it. admit, when set, runs once
// before the network fetch (the activation egress ledger); the schedule
// passes nil and records usage on the schedule instead. The returned error
// is for a poll that could not start (unknown source, no capture store).
func PollSourceOnce(ctx context.Context, deps Dependencies, ownerID, computerID, sourceID string, maxItems int, admit func() error) (PollReport, error) {
	if strings.TrimSpace(ownerID) == "" {
		return PollReport{}, fmt.Errorf("poll_source missing owner context")
	}
	if deps.Captures == nil {
		return PollReport{}, fmt.Errorf("poll_source: the shared capture store is not configured")
	}
	reg, err := deps.sourceRegistry()
	if err != nil {
		return PollReport{}, err
	}
	source, ok := reg.Lookup(sourceID)
	if !ok {
		return PollReport{}, fmt.Errorf("poll_source: unknown source %q; known ids include %s", sourceID, strings.Join(nearbySourceIDs(reg, sourceID, 10), ", "))
	}
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
		return PollReport{}, err
	}
	if admit != nil {
		if err := admit(); err != nil {
			return PollReport{}, err
		}
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
	report := PollReport{
		SourceID:    source.ID,
		Status:      polled.Fetch.Status,
		StatusCode:  polled.Fetch.StatusCode,
		Fetched:     len(polled.Items),
		NotModified: polled.Fetch.Status == "not_modified",
		PollMS:      time.Since(started).Milliseconds(),
	}
	for _, item := range polled.Items {
		report.ItemBytes += len(item.Body)
	}
	if pollErr != nil {
		report.Error = pollErr.Error()
		report.TotalMS = report.PollMS
		return report, nil
	}
	written, err := deps.Captures.WriteCaptures(ctx, ownerID, computerID, polled.Items)
	report.New, report.Repeated, report.Skipped, report.Captured = written.New, written.Repeated, written.Skipped, written.Captured
	report.TotalMS = time.Since(started).Milliseconds()
	if err != nil {
		report.Error = err.Error()
	}
	return report, nil
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
