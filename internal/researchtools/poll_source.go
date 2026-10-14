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
	// pollExamineFactor bounds the items one poll sends to the capture
	// writer (and so the articles it reads) to this many times its limit,
	// so a poll after a restart, which has forgotten what it handled, does
	// not read a whole feed's articles at once.
	pollExamineFactor = 4
	// pollSummaryRunes bounds the feed summary reported with a capture.
	pollSummaryRunes = 600
)

// pollState keeps, per source in this process, the conditional GET
// validators and the feed items already sent to the capture writer. A
// restart forgets both; the next polls re-read the feed and re-examine its
// items, and stable capture ids make that write nothing new.
var pollState = struct {
	sync.Mutex
	bySource map[string]sources.Source
	handled  map[string]map[string]struct{}
}{bySource: map[string]sources.Source{}, handled: map[string]map[string]struct{}{}}

type poller interface {
	Poll(ctx context.Context, source *sources.Source) (sources.PollResult, error)
}

// itemEnricher is a poller that leaves each item's article read to the
// caller (sources.RSSPoller with LazyReaderImport).
type itemEnricher interface {
	EnrichItem(ctx context.Context, source *sources.Source, item *sources.Item)
}

// handledKey names one feed item as polled: its stable id and the hash of
// its feed entry before any article read, so an entry the feed rewrote is
// examined again.
func handledKey(item sources.Item) string { return item.ID + "|" + item.ContentHash }

// EffectivePollMaxItems is the per-poll limit on new captures for a
// requested max_items: 20 when unset, at most 100.
func EffectivePollMaxItems(maxItems int) int {
	if maxItems <= 0 {
		return defaultPollMaxItems
	}
	if maxItems > maxPollMaxItems {
		return maxPollMaxItems
	}
	return maxItems
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
			report, err := PollSourceOnce(ctx, deps, exec.OwnerID, exec.ComputerID, in.SourceID, in.MaxItems)
			if err != nil {
				return "", err
			}
			// The tool's result names captures, never their text: the feed
			// summary rides only the schedule's report for intake labelling.
			for i := range report.Captured {
				report.Captured[i].Summary = ""
			}
			return toolregistry.ResultJSON(report)
		}}
}

// PollReport is one poll's outcome and resource use. A fetch or capture
// write failure is reported in Error; the poll itself still happened.
// Fetched counts the items the feed gave (up to the parse cap); New,
// Repeated and Skipped count the items sent to the capture writer; Deferred
// counts items not yet handled that this poll left for the next one, in
// feed order, because it reached its limit (L2a A5, defect 1).
type PollReport struct {
	SourceID    string                     `json:"source_id"`
	Status      string                     `json:"status"`
	StatusCode  int                        `json:"status_code"`
	Limit       int                        `json:"limit"`
	Fetched     int                        `json:"fetched"`
	NotModified bool                       `json:"not_modified"`
	ItemBytes   int                        `json:"item_bytes"`
	PollMS      int64                      `json:"poll_ms"`
	New         int                        `json:"new"`
	Repeated    int                        `json:"repeated"`
	Skipped     int                        `json:"skipped"`
	Deferred    int                        `json:"deferred"`
	Captured    []sourcegraph.CapturedItem `json:"captured,omitempty"`
	TotalMS     int64                      `json:"total_ms"`
	Error       string                     `json:"error,omitempty"`
}

// PollSourceOnce polls one registry source and writes its new items as
// shared captures through deps.Captures. research's choir.PollSource and the
// source schedule (Autopaper W1c) both call it. The returned error is for a
// poll that could not start (unknown source, no capture store).
func PollSourceOnce(ctx context.Context, deps Dependencies, ownerID, computerID, sourceID string, maxItems int) (PollReport, error) {
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
	// limit bounds the poll's new captures, counted after the repeat check;
	// the poller parses up to the hard cap (L2a A5, defect 1).
	limit := EffectivePollMaxItems(maxItems)
	if source.MaxItemsPerPoll > 0 && source.MaxItemsPerPoll < limit {
		limit = source.MaxItemsPerPoll
	}
	source.MaxItemsPerPoll = maxPollMaxItems
	p, err := pollerFor(source, reg.UserAgent)
	if err != nil {
		return PollReport{}, err
	}

	pollState.Lock()
	if prior, ok := pollState.bySource[source.ID]; ok {
		source.LastETag, source.LastModified, source.LastAuxCursor, source.LastPolled = prior.LastETag, prior.LastModified, prior.LastAuxCursor, prior.LastPolled
	}
	handledBefore := pollState.handled[source.ID]
	pollState.Unlock()

	pollCtx, cancel := context.WithTimeout(ctx, pollSourceTimeout)
	defer cancel()
	started := time.Now()
	polled, pollErr := p.Poll(pollCtx, &source)
	// The feed's validators are kept only once its items are safely written
	// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md) and
	// none were deferred; otherwise the next poll's 304 would lose them.
	keepValidators := func() {
		pollState.Lock()
		pollState.bySource[source.ID] = source
		pollState.Unlock()
	}
	if pollErr == nil && len(polled.Items) == 0 {
		keepValidators()
	}
	report := PollReport{
		SourceID:    source.ID,
		Status:      polled.Fetch.Status,
		StatusCode:  polled.Fetch.StatusCode,
		Limit:       limit,
		Fetched:     len(polled.Items),
		NotModified: polled.Fetch.Status == "not_modified",
		PollMS:      time.Since(started).Milliseconds(),
	}
	if pollErr != nil {
		report.Error = pollErr.Error()
		report.TotalMS = report.PollMS
		return report, nil
	}
	if len(polled.Items) == 0 {
		report.TotalMS = report.PollMS
		return report, nil
	}

	// Items not yet handled, in feed order. They go to the writer in
	// batches no larger than the new captures still allowed, so repeats do
	// not spend the limit, until the limit is reached, the items run out,
	// or the poll has examined its bound.
	var pending []sources.Item
	for _, item := range polled.Items {
		if _, ok := handledBefore[handledKey(item)]; !ok {
			pending = append(pending, item)
		}
	}
	enricher, _ := p.(itemEnricher)
	budget := limit * pollExamineFactor
	handledNow := map[string]struct{}{}
	examined := 0
	for examined < len(pending) && examined < budget && report.New < limit {
		if pollCtx.Err() != nil {
			// Out of time: what is left is deferred, not read in a hurry.
			break
		}
		n := min(limit-report.New, len(pending)-examined, budget-examined)
		batch := make([]sourcegraph.CaptureItem, 0, n)
		for _, item := range pending[examined : examined+n] {
			item.Outlet = strings.TrimSpace(source.Name)
			if item.BodyKind == sources.BodyKindFeedSummary {
				item.Summary = clipRunes(strings.TrimSpace(item.Body), pollSummaryRunes)
			}
			if enricher != nil {
				enricher.EnrichItem(pollCtx, &source, &item)
			}
			report.ItemBytes += len(item.Body)
			batch = append(batch, sourcegraph.CaptureItem{Item: item})
		}
		written, err := deps.Captures.WriteCaptures(ctx, ownerID, computerID, batch)
		report.New += written.New
		report.Repeated += written.Repeated
		report.Skipped += written.Skipped
		report.Captured = append(report.Captured, written.Captured...)
		if err != nil {
			report.Error = err.Error()
			break
		}
		for _, item := range pending[examined : examined+n] {
			handledNow[handledKey(item)] = struct{}{}
		}
		examined += n
	}
	report.Deferred = len(pending) - examined
	report.TotalMS = time.Since(started).Milliseconds()

	// Remember what was handled, only for items still in the feed, so the
	// set stays the feed's size.
	handledAfter := make(map[string]struct{}, len(polled.Items))
	for _, item := range polled.Items {
		key := handledKey(item)
		_, before := handledBefore[key]
		_, now := handledNow[key]
		if before || now {
			handledAfter[key] = struct{}{}
		}
	}
	pollState.Lock()
	pollState.handled[source.ID] = handledAfter
	pollState.Unlock()

	if report.Error == "" && report.Deferred == 0 {
		keepValidators()
	}
	return report, nil
}

func clipRunes(text string, max int) string {
	if runes := []rune(text); len(runes) > max {
		return string(runes[:max])
	}
	return text
}

// newReadCaptureTool reads one shared capture (any computer's poll or read)
// from Store B and host disk through corpusd: its blocks with their ids from
// from_block, as many as fit in max_chars of text. It is a read of the
// shared store, not world egress.
func newReadCaptureTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		CaptureID string `json:"capture_id"`
		FromBlock int    `json:"from_block,omitempty"`
		MaxChars  int    `json:"max_chars,omitempty"`
	}
	return toolregistry.Tool{Name: "read_capture",
		Description: "Read one shared web_capture's blocks with their ids.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"capture_id": map[string]any{"type": "string"},
			"from_block": map[string]any{"type": "integer", "minimum": 0},
			"max_chars":  map[string]any{"type": "integer", "minimum": 1},
		}, []string{"capture_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode read_capture args: %w", err)
			}
			if deps.Captures == nil {
				return "", fmt.Errorf("read_capture: the shared capture store is not configured")
			}
			out, err := deps.Captures.ReadCapture(ctx, in.CaptureID, in.FromBlock, in.MaxChars)
			if err != nil {
				return "", err
			}
			return string(out), nil
		}}
}

func pollerFor(source sources.Source, userAgent string) (poller, error) {
	ua := source.EffectiveUserAgent(userAgent)
	switch source.Type {
	case sources.SourceTypeRSS:
		rss := sources.NewRSSPoller(ua)
		rss.LazyReaderImport = true
		return rss, nil
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
