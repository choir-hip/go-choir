package cycle

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
)

// MemoryStore is an in-memory implementation of Store. It has no database
// dependency — state is ephemeral and lost on restart. Source items are
// projected to the object graph via the runtime API; the queue/cycle state
// here is only for the daemon's own dispatch management.
type MemoryStore struct {
	mu sync.Mutex

	sources         map[string]sourceRow
	fetches         []fetchRow
	items           map[string]sources.Item
	cycles          []cycleRow
	cycleEvents     []CycleEvent
	ingestionEvents map[string]IngestionEvent
	processors      map[string]ProcessorRequest
	reconcilers     map[string]ReconcilerRequest
}

type sourceRow struct {
	lastPolled    time.Time
	lastETag      string
	lastModified  string
	lastAuxCursor string
	updatedAt     time.Time
}

type fetchRow struct {
	cycleID string
	fetch   sources.FetchRecord
}

type cycleRow struct {
	summary CycleSummary
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sources:         map[string]sourceRow{},
		items:           map[string]sources.Item{},
		ingestionEvents: map[string]IngestionEvent{},
		processors:      map[string]ProcessorRequest{},
		reconcilers:     map[string]ReconcilerRequest{},
	}
}

func (m *MemoryStore) Close() error { return nil }

func (m *MemoryStore) SaveSources(registry *sources.Registry) error {
	if registry == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for _, source := range registry.Sources {
		m.sources[source.ID] = sourceRow{updatedAt: now}
	}
	return nil
}

func (m *MemoryStore) ApplySourcePollState(registry *sources.Registry) error {
	if registry == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range registry.Sources {
		row, ok := m.sources[registry.Sources[i].ID]
		if !ok {
			continue
		}
		registry.Sources[i].LastPolled = row.lastPolled
		registry.Sources[i].LastETag = row.lastETag
		registry.Sources[i].LastModified = row.lastModified
		registry.Sources[i].LastAuxCursor = row.lastAuxCursor
	}
	return nil
}

func (m *MemoryStore) SaveSourcePollState(registry *sources.Registry) error {
	if registry == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for _, source := range registry.Sources {
		row := m.sources[source.ID]
		row.lastPolled = source.LastPolled
		row.lastETag = source.LastETag
		row.lastModified = source.LastModified
		row.lastAuxCursor = source.LastAuxCursor
		row.updatedAt = now
		m.sources[source.ID] = row
	}
	return nil
}

func (m *MemoryStore) StartCycle(ctx context.Context) (string, error) {
	now := time.Now().UTC()
	cycleID := "cycle_" + sources.ContentHash(now.Format(time.RFC3339Nano))[:24]
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cycles = append(m.cycles, cycleRow{summary: CycleSummary{
		CycleID:   cycleID,
		StartedAt: now,
		Status:    "running",
	}})
	return cycleID, nil
}

func (m *MemoryStore) FinishCycle(ctx context.Context, cycleID, status string, itemCount, fetchCount int, cycleErr error) error {
	if status == "" {
		status = "completed"
	}
	errText := ""
	if cycleErr != nil {
		errText = cycleErr.Error()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.cycles {
		if m.cycles[i].summary.CycleID == cycleID {
			m.cycles[i].summary.EndedAt = time.Now().UTC()
			m.cycles[i].summary.Status = status
			m.cycles[i].summary.ItemCount = itemCount
			m.cycles[i].summary.FetchCount = fetchCount
			m.cycles[i].summary.Error = errText
			return nil
		}
	}
	return nil
}

func (m *MemoryStore) RecordCycleEvent(ctx context.Context, cycleID, sourceID, kind, message string, metadata map[string]any) error {
	now := time.Now().UTC()
	eventID := "cycleevt_" + sources.ContentHash(cycleID, sourceID, kind, message, now.Format(time.RFC3339Nano))[:24]
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cycleEvents = append(m.cycleEvents, CycleEvent{
		EventID:   eventID,
		CycleID:   cycleID,
		SourceID:  sourceID,
		Kind:      kind,
		Message:   message,
		Metadata:  metadata,
		CreatedAt: now,
	})
	return nil
}

func (m *MemoryStore) SaveCycleFetches(cycleID string, fetches []sources.FetchRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, fetch := range fetches {
		if strings.TrimSpace(fetch.FetchID) == "" {
			continue
		}
		m.fetches = append(m.fetches, fetchRow{cycleID: cycleID, fetch: fetch})
	}
	return nil
}

func (m *MemoryStore) SaveFetches(fetches []sources.FetchRecord) error {
	return m.SaveCycleFetches("", fetches)
}

func (m *MemoryStore) SaveItems(items []sources.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		if item.ID == "" {
			return fmt.Errorf("item id is required")
		}
		item = sources.NormalizeItemBodyClassification(item)
		m.items[item.ID] = item
	}
	return nil
}

func (m *MemoryStore) SaveIngestionEvents(ctx context.Context, events []IngestionEvent) error {
	if len(events) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, event := range events {
		if err := ValidateIngestionEventOrigin(event.Origin); err != nil {
			return err
		}
		if strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.CycleID) == "" || strings.TrimSpace(event.ArtifactID) == "" {
			return fmt.Errorf("ingestion event id, cycle id, and artifact id are required")
		}
		createdAt := event.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		event.CreatedAt = createdAt
		m.ingestionEvents[event.EventID] = event
	}
	return nil
}

func (m *MemoryStore) ValidateProcessorRequestIngestionEvents(ctx context.Context, req ProcessorRequest) (bool, error) {
	if !ProcessorRequestEligibleForDispatch(req) {
		return false, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, eventID := range req.IngestionEventIDs {
		eventID = strings.TrimSpace(eventID)
		if eventID == "" {
			return false, nil
		}
		event, ok := m.ingestionEvents[eventID]
		if !ok {
			return false, nil
		}
		if event.CycleID != strings.TrimSpace(req.CycleID) || event.Origin != IngestionOriginSourceFetch {
			return false, nil
		}
		if !stringSliceContains(req.SourceItemIDs, event.ArtifactID) {
			return false, nil
		}
	}
	return true, nil
}

func (m *MemoryStore) SaveProcessorRequests(ctx context.Context, requests []ProcessorRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, req := range requests {
		if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.CycleID) == "" || strings.TrimSpace(req.ProcessorKey) == "" {
			return fmt.Errorf("processor request id, cycle id, and processor key are required")
		}
		status := strings.TrimSpace(req.Status)
		if status == "" {
			status = "queued"
		}
		runtimeStatus := strings.TrimSpace(req.RuntimeStatus)
		if runtimeStatus == "" {
			runtimeStatus = status
		}
		now := time.Now().UTC()
		createdAt := req.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		updatedAt := req.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}
		sourceCount := req.SourceCount
		if sourceCount == 0 {
			sourceCount = len(req.SourceItemIDs)
		}
		req.Status = status
		req.RuntimeStatus = runtimeStatus
		req.SourceCount = sourceCount
		req.CreatedAt = createdAt
		req.UpdatedAt = updatedAt
		m.processors[req.RequestID] = req
	}
	return nil
}

func (m *MemoryStore) SaveReconcilerRequests(ctx context.Context, requests []ReconcilerRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, req := range requests {
		if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.CycleID) == "" || strings.TrimSpace(req.Scope) == "" {
			return fmt.Errorf("reconciler request id, cycle id, and scope are required")
		}
		status := strings.TrimSpace(req.Status)
		if status == "" {
			status = "queued"
		}
		now := time.Now().UTC()
		createdAt := req.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		updatedAt := req.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}
		req.Status = status
		req.CreatedAt = createdAt
		req.UpdatedAt = updatedAt
		m.reconcilers[req.RequestID] = req
	}
	return nil
}

func (m *MemoryStore) UpdateProcessorRequestRuntimeRun(ctx context.Context, requestID, status, runtimeRunID string) error {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(status) == "" {
		return fmt.Errorf("processor request id and status are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.processors[requestID]
	if !ok {
		return nil
	}
	req.Status = strings.TrimSpace(status)
	req.RuntimeStatus = strings.TrimSpace(status)
	req.RuntimeRunID = strings.TrimSpace(runtimeRunID)
	req.UpdatedAt = time.Now().UTC()
	m.processors[requestID] = req
	return nil
}

func (m *MemoryStore) UpdateProcessorRequestRuntimeStatus(ctx context.Context, requestID, runtimeStatus, runtimeRunID string) error {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(runtimeStatus) == "" {
		return fmt.Errorf("processor request id and runtime status are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.processors[requestID]
	if !ok {
		return nil
	}
	req.RuntimeStatus = strings.TrimSpace(runtimeStatus)
	if strings.TrimSpace(runtimeRunID) != "" {
		req.RuntimeRunID = strings.TrimSpace(runtimeRunID)
	}
	req.UpdatedAt = time.Now().UTC()
	m.processors[requestID] = req
	return nil
}

func (m *MemoryStore) UpdateProcessorRequestVerdictStatus(ctx context.Context, requestID, status string) error {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(status) == "" {
		return fmt.Errorf("processor request id and verdict status are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.processors[requestID]
	if !ok {
		return nil
	}
	req.Status = strings.TrimSpace(status)
	req.UpdatedAt = time.Now().UTC()
	m.processors[requestID] = req
	return nil
}

func (m *MemoryStore) ResetProcessorRequestSubmission(ctx context.Context, requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.processors[requestID]
	if !ok {
		return nil
	}
	switch req.Status {
	case "submitted":
		req.Status = "queued"
		req.RuntimeStatus = "queued"
	case "dispatch_failed":
		req.RuntimeStatus = "failed"
	default:
		req.RuntimeStatus = "completed"
	}
	req.RuntimeRunID = ""
	req.UpdatedAt = time.Now().UTC()
	m.processors[requestID] = req
	return nil
}

func (m *MemoryStore) ResetStaleSubmittedProcessorRequests(ctx context.Context, cutoff time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for id, req := range m.processors {
		if req.RuntimeStatus != "submitted" || !req.UpdatedAt.Before(cutoff) {
			continue
		}
		switch req.Status {
		case "submitted":
			req.Status = "queued"
			req.RuntimeStatus = "queued"
		case "dispatch_failed":
			req.RuntimeStatus = "failed"
		default:
			req.RuntimeStatus = "completed"
		}
		req.RuntimeRunID = ""
		req.UpdatedAt = time.Now().UTC()
		m.processors[id] = req
		total++
	}
	return total, nil
}

func (m *MemoryStore) SupersedeQueuedProcessorRequests(ctx context.Context, replacements []ProcessorRequest) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, replacement := range replacements {
		continuityRef := strings.TrimSpace(replacement.ContinuityRef)
		if continuityRef == "" {
			continue
		}
		createdAt := replacement.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		for id, req := range m.processors {
			if (req.Status != "queued" && req.Status != "deferred") ||
				req.ContinuityRef != continuityRef ||
				id == replacement.RequestID ||
				!req.CreatedAt.Before(createdAt) {
				continue
			}
			req.Status = "superseded"
			req.UpdatedAt = time.Now().UTC()
			m.processors[id] = req
			total++
		}
	}
	return total, nil
}

func (m *MemoryStore) SupersedeQueuedReconcilersWithSupersededProcessors(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for id, rec := range m.reconcilers {
		if rec.Status != "queued" || len(rec.ProcessorRequestIDs) == 0 {
			continue
		}
		hasSuperseded := false
		for _, pid := range rec.ProcessorRequestIDs {
			if p, ok := m.processors[pid]; ok && p.Status == "superseded" {
				hasSuperseded = true
				break
			}
		}
		if !hasSuperseded {
			continue
		}
		rec.Status = "superseded"
		rec.UpdatedAt = time.Now().UTC()
		m.reconcilers[id] = rec
		total++
	}
	return total, nil
}

func (m *MemoryStore) ListQueuedProcessorRequests(ctx context.Context, limit int) ([]ProcessorRequest, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ProcessorRequest
	for _, req := range m.processors {
		if req.Status == "queued" && len(req.IngestionEventIDs) > 0 {
			out = append(out, req)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].RequestID < out[j].RequestID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryStore) CountQueuedProcessorRequests(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, req := range m.processors {
		if req.Status == "queued" && len(req.IngestionEventIDs) > 0 {
			count++
		}
	}
	return count, nil
}

func (m *MemoryStore) ListReconcilableProcessorRequests(ctx context.Context, limit int) ([]ProcessorRequest, error) {
	if limit <= 0 {
		limit = 100
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ProcessorRequest
	for _, req := range m.processors {
		if req.Status == "submitted" || req.RuntimeStatus == "submitted" {
			out = append(out, req)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.Before(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryStore) CountRecentlySubmittedProcessorRequests(ctx context.Context, since time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, req := range m.processors {
		if req.RuntimeStatus == "submitted" && !req.UpdatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (m *MemoryStore) CountItems(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items), nil
}

func (m *MemoryStore) CountFetches(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.fetches), nil
}

func (m *MemoryStore) SearchItems(ctx context.Context, query string, limit int) ([]sources.Item, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if itemIDs := sourceSearchItemIDs(query); len(itemIDs) > 0 {
		var out []sources.Item
		for _, id := range itemIDs {
			if item, ok := m.items[id]; ok {
				out = append(out, item)
				if len(out) >= limit {
					break
				}
			}
		}
		return out, nil
	}
	terms := sourceSearchTerms(query)
	var out []sources.Item
	for _, item := range m.items {
		if len(terms) > 0 {
			matched := false
			for _, term := range terms {
				lowerTitle := strings.ToLower(item.Title)
				lowerBody := strings.ToLower(item.Body)
				lowerSource := strings.ToLower(item.SourceID)
				if strings.Contains(lowerTitle, term) || strings.Contains(lowerBody, term) || strings.Contains(lowerSource, term) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Published.Equal(out[j].Published) {
			return out[i].FetchedAt.After(out[j].FetchedAt)
		}
		return out[i].Published.After(out[j].Published)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryStore) GetItem(ctx context.Context, itemID string) (sources.Item, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return sources.Item{}, fmt.Errorf("item id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[itemID]
	if !ok {
		return sources.Item{}, fmt.Errorf("item not found: %s", itemID)
	}
	return sources.NormalizeItemBodyClassification(item), nil
}

func (m *MemoryStore) LatestCycleSummary(ctx context.Context) (CycleSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.cycles) == 0 {
		return CycleSummary{}, fmt.Errorf("no cycles")
	}
	last := m.cycles[len(m.cycles)-1].summary
	// Populate events, fetches, processors, reconcilers for this cycle.
	for _, evt := range m.cycleEvents {
		if evt.CycleID == last.CycleID {
			last.Events = append(last.Events, evt)
		}
	}
	for _, fr := range m.fetches {
		if fr.cycleID == last.CycleID {
			last.Fetches = append(last.Fetches, fr.fetch)
		}
	}
	for _, req := range m.processors {
		if req.CycleID == last.CycleID {
			last.ProcessorRequests = append(last.ProcessorRequests, req)
		}
	}
	for _, req := range m.reconcilers {
		if req.CycleID == last.CycleID {
			last.ReconcilerRequests = append(last.ReconcilerRequests, req)
		}
	}
	return last, nil
}

func (m *MemoryStore) ListProcessorRequests(ctx context.Context, cycleID string, limit int) ([]ProcessorRequest, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ProcessorRequest
	for _, req := range m.processors {
		if cycleID != "" && req.CycleID != cycleID {
			continue
		}
		out = append(out, req)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
