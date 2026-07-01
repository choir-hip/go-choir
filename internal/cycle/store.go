package cycle

import (
	"context"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
)

// Compile-time checks that both implementations satisfy Store.
var _ Store = (*Storage)(nil)
var _ Store = (*MemoryStore)(nil)

// Store is the persistence interface used by the sourcecycled daemon. The
// concrete implementation may be database-backed (Storage) or in-memory
// (MemoryStore). The daemon uses MemoryStore so it has no relational database
// dependency — source items are projected to the object graph via the runtime
// API, and the queue/cycle state is ephemeral.
type Store interface {
	Close() error
	SaveSources(registry *sources.Registry) error
	ApplySourcePollState(registry *sources.Registry) error
	SaveSourcePollState(registry *sources.Registry) error
	StartCycle(ctx context.Context) (string, error)
	FinishCycle(ctx context.Context, cycleID, status string, itemCount, fetchCount int, cycleErr error) error
	RecordCycleEvent(ctx context.Context, cycleID, sourceID, kind, message string, metadata map[string]any) error
	SaveCycleFetches(cycleID string, fetches []sources.FetchRecord) error
	SaveFetches(fetches []sources.FetchRecord) error
	SaveItems(items []sources.Item) error
	SaveIngestionEvents(ctx context.Context, events []IngestionEvent) error
	SaveProcessorRequests(ctx context.Context, requests []ProcessorRequest) error
	SaveReconcilerRequests(ctx context.Context, requests []ReconcilerRequest) error
	UpdateProcessorRequestRuntimeRun(ctx context.Context, requestID, status, runtimeRunID string) error
	UpdateProcessorRequestRuntimeStatus(ctx context.Context, requestID, runtimeStatus, runtimeRunID string) error
	UpdateProcessorRequestVerdictStatus(ctx context.Context, requestID, status string) error
	ResetProcessorRequestSubmission(ctx context.Context, requestID string) error
	ResetStaleSubmittedProcessorRequests(ctx context.Context, cutoff time.Time) (int, error)
	SupersedeQueuedProcessorRequests(ctx context.Context, replacements []ProcessorRequest) (int, error)
	SupersedeQueuedReconcilersWithSupersededProcessors(ctx context.Context) (int, error)
	ListQueuedProcessorRequests(ctx context.Context, limit int) ([]ProcessorRequest, error)
	CountQueuedProcessorRequests(ctx context.Context) (int, error)
	ListReconcilableProcessorRequests(ctx context.Context, limit int) ([]ProcessorRequest, error)
	CountRecentlySubmittedProcessorRequests(ctx context.Context, since time.Time) (int, error)
	ValidateProcessorRequestIngestionEvents(ctx context.Context, req ProcessorRequest) (bool, error)
	CountItems(ctx context.Context) (int, error)
	CountFetches(ctx context.Context) (int, error)
	SearchItems(ctx context.Context, query string, limit int) ([]sources.Item, error)
	GetItem(ctx context.Context, itemID string) (sources.Item, error)
	LatestCycleSummary(ctx context.Context) (CycleSummary, error)
	ListProcessorRequests(ctx context.Context, cycleID string, limit int) ([]ProcessorRequest, error)
}
