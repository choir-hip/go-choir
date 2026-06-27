package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/cycle"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

const (
	defaultIngestionProcessorDispatchLimit  = 1
	defaultIngestionRuntimeDispatchRetries  = 8
	defaultIngestionRuntimeRetryDelay       = 2 * time.Second
	defaultIngestionQueueDrainInterval      = 1 * time.Minute
	defaultIngestionProcessorInFlightWindow = 15 * time.Minute
	defaultObjectGraphBackfillLimit         = 50
)

type runtimeRunSubmitRequest struct {
	OwnerID  string         `json:"owner_id"`
	Prompt   string         `json:"prompt"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type runtimeRunStatusResponse struct {
	RunID               string                                    `json:"loop_id"`
	AgentID             string                                    `json:"agent_id"`
	ChannelID           string                                    `json:"channel_id,omitempty"`
	AgentProfile        string                                    `json:"agent_profile,omitempty"`
	AgentRole           string                                    `json:"agent_role,omitempty"`
	State               string                                    `json:"state,omitempty"`
	ActiveChildRuns     int                                       `json:"active_child_runs,omitempty"`
	Trajectory          *runtimeTrajectoryStatusResponse          `json:"trajectory,omitempty"`
	ProcessorResolution *runtimeProcessorResolutionStatusResponse `json:"processor_resolution,omitempty"`
}

type runtimeTrajectoryStatusResponse struct {
	TrajectoryID      string   `json:"trajectory_id"`
	Status            string   `json:"status,omitempty"`
	SettlementReady   bool     `json:"settlement_ready"`
	WaitingOn         []string `json:"waiting_on,omitempty"`
	OpenWorkItemCount int      `json:"open_work_item_count"`
}

type runtimeProcessorResolutionStatusResponse struct {
	WorkItemID              string `json:"work_item_id"`
	Status                  string `json:"status,omitempty"`
	ResolutionState         string `json:"resolution_state,omitempty"`
	SourceItemCount         int    `json:"source_item_count,omitempty"`
	ResolvedSourceItemCount int    `json:"resolved_source_item_count,omitempty"`
	LastDecision            string `json:"last_decision,omitempty"`
	StoryDocID              string `json:"story_doc_id,omitempty"`
	CoveredByDocID          string `json:"covered_by_doc_id,omitempty"`
}

type ingestionRuntimeDispatcher struct {
	baseURL              string
	socketPath           string // UDS socket path; if set, uses unix transport and proxy path for sandbox
	ownerID              string
	maxProcessorRequests int
	inFlightWindow       time.Duration
	client               *http.Client
	retryAttempts        int
	retryDelay           time.Duration
}

type ingestionDispatchResult struct {
	ProcessorSubmitted  int
	ProcessorFailed     int
	ProcessorSkipped    int
	ReconcilerSubmitted int
	ReconcilerFailed    int
	ReconcilerSkipped   int
	RunIDs              []string
	Errors              []string
}

type webCaptureProjectionSummary struct {
	Mode              string
	Target            string
	CaptureCount      int
	SourceEntityCount int
	CapturedFromEdges int
	SkippedItemCount  int
}

type runtimeWebCaptureProjectionRequest struct {
	OwnerID    string         `json:"owner_id"`
	ComputerID string         `json:"computer_id,omitempty"`
	Items      []sources.Item `json:"items"`
	Now        string         `json:"now,omitempty"`
}

type runtimeWebCaptureProjectionResponse struct {
	Status            string `json:"status"`
	CaptureCount      int    `json:"capture_count"`
	SourceEntityCount int    `json:"source_entity_count"`
	CapturedFromEdges int    `json:"captured_from_edges"`
	SkippedItemCount  int    `json:"skipped_item_count"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Choir Universal Wire sourcecycled daemon (V0)")

	// 1. Load Configuration
	configPath := sourceServiceConfigPath()
	configData, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var registry sources.Registry
	if err := json.Unmarshal(configData, &registry); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}
	log.Printf("Loaded %d sources from registry", len(registry.Sources))

	store, err := cycle.NewStorage(sourceServiceDBPath())
	if err != nil {
		log.Fatalf("Failed to initialize source service storage: %v", err)
	}
	defer store.Close()
	if err := store.SaveSources(&registry); err != nil {
		log.Fatalf("Failed to save source registry: %v", err)
	}
	if err := store.ApplySourcePollState(&registry); err != nil {
		log.Fatalf("Failed to load source poll state: %v", err)
	}

	// 2. Setup Context and Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// Clean up stale submitted processor requests on startup.
	// Runs submitted before restart are orphaned when the platform VM recycles.
	inFlightWindow := time.Duration(parsePositiveInt(
		firstEnv("SOURCE_SERVICE_AGENT_DISPATCH_INFLIGHT_WINDOW_SECONDS", "SOURCECYCLED_INFLIGHT_WINDOW_SECONDS"),
		int(defaultIngestionProcessorInFlightWindow/time.Second),
	)) * time.Second
	cutoff := time.Now().UTC().Add(-inFlightWindow)
	if cleaned, err := store.ResetStaleSubmittedProcessorRequests(ctx, cutoff); err != nil {
		log.Printf("Warning: failed to clean stale submitted processor requests: %v", err)
	} else if cleaned > 0 {
		log.Printf("Cleaned %d stale submitted processor requests (cutoff %s)", cleaned, cutoff.Format(time.RFC3339))
	}
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, terminating...")
		cancel()
	}()

	server := startSourceServiceAPI(ctx, store)

	// 3. Main Ingestion Loop (15-minute source cycle plus queue drain)
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	drainTicker := time.NewTicker(ingestionQueueDrainIntervalFromEnv())
	defer drainTicker.Stop()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Source Service API shutdown failed: %v", err)
		}
	}()

	// Run the first cycle immediately
	log.Println("Initiating first cycle...")
	runCycle(ctx, &registry, store)

	for {
		select {
		case <-ctx.Done():
			log.Println("Daemon stopped.")
			return
		case <-ticker.C:
			log.Println("Initiating scheduled cycle...")
			runCycle(ctx, &registry, store)
		case <-drainTicker.C:
			log.Println("Initiating queued ingestion handoff dispatch drain...")
			dispatchQueuedIngestionHandoffs(ctx, store)
		}
	}
}

func sourceServiceDBPath() string {
	if dbPath := os.Getenv("SOURCE_SERVICE_DB_PATH"); strings.TrimSpace(dbPath) != "" {
		return strings.TrimSpace(dbPath)
	}
	if dbPath := os.Getenv("SOURCECYCLED_DB_PATH"); strings.TrimSpace(dbPath) != "" {
		return strings.TrimSpace(dbPath)
	}
	return "var/sourcecycled.db"
}

func sourceServiceAddr() string {
	if addr := strings.TrimSpace(os.Getenv("SOURCE_SERVICE_ADDR")); addr != "" {
		return addr
	}
	if addr := strings.TrimSpace(os.Getenv("SOURCECYCLED_ADDR")); addr != "" {
		return addr
	}
	return "127.0.0.1:8787"
}

func sourceServiceConfigPath() string {
	if configPath := os.Getenv("SOURCE_SERVICE_CONFIG_PATH"); strings.TrimSpace(configPath) != "" {
		return strings.TrimSpace(configPath)
	}
	if configPath := os.Getenv("SOURCECYCLED_CONFIG_PATH"); strings.TrimSpace(configPath) != "" {
		return strings.TrimSpace(configPath)
	}
	return filepath.Join("configs", "sources.json")
}

func sourceServiceObjectGraphDBPath() string {
	if dbPath := firstEnv("SOURCE_SERVICE_OBJECTGRAPH_DB_PATH", "SOURCECYCLED_OBJECTGRAPH_DB_PATH"); strings.TrimSpace(dbPath) != "" {
		return strings.TrimSpace(dbPath)
	}
	runtimeStorePath := strings.TrimSpace(firstEnv("SOURCE_SERVICE_RUNTIME_STORE_PATH", "RUNTIME_STORE_PATH"))
	if runtimeStorePath == "" {
		return ""
	}
	if runtimeStorePath == ":memory:" {
		return ":memory:"
	}
	return filepath.Join(filepath.Dir(runtimeStorePath), filepath.Base(runtimeStorePath)+".objectgraph.db")
}

func sourceServiceObjectGraphOwnerID() string {
	ownerID := strings.TrimSpace(firstEnv(
		"SOURCE_SERVICE_OBJECTGRAPH_OWNER_ID",
		"SOURCECYCLED_OBJECTGRAPH_OWNER_ID",
		"SOURCE_SERVICE_RUNTIME_OWNER_ID",
		"SOURCECYCLED_RUNTIME_OWNER_ID",
	))
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}

func sourceServiceObjectGraphComputerID() string {
	return strings.TrimSpace(firstEnv("SOURCE_SERVICE_OBJECTGRAPH_COMPUTER_ID", "SOURCECYCLED_OBJECTGRAPH_COMPUTER_ID"))
}

func sourceServiceObjectGraphBackfillLimit() int {
	return parsePositiveInt(firstEnv("SOURCE_SERVICE_OBJECTGRAPH_BACKFILL_LIMIT", "SOURCECYCLED_OBJECTGRAPH_BACKFILL_LIMIT"), defaultObjectGraphBackfillLimit)
}

func sourcecycledObjectGraphServiceFromEnv() (*objectgraph.Service, string, error) {
	dbPath := sourceServiceObjectGraphDBPath()
	if strings.TrimSpace(dbPath) == "" {
		return nil, "", nil
	}
	sqliteStore, err := objectgraph.NewSQLiteStore(dbPath)
	if err != nil {
		return nil, dbPath, err
	}
	return objectgraph.NewService(objectgraph.Config{
		Memory: objectgraph.NewMemoryStore(),
		SQLite: sqliteStore,
	}), dbPath, nil
}

func writeSourceItemsToObjectGraph(ctx context.Context, store *cycle.Storage, cycleID string, items []sources.Item, now time.Time, eventType, message string) error {
	summary, err := projectSourceItemsToObjectGraph(ctx, items, now)
	if err != nil {
		return err
	}
	if store != nil {
		_ = store.RecordCycleEvent(ctx, cycleID, "", eventType, message, map[string]any{
			"objectgraph_mode":    summary.Mode,
			"objectgraph_target":  summary.Target,
			"capture_count":       summary.CaptureCount,
			"source_entity_count": summary.SourceEntityCount,
			"captured_from_edges": summary.CapturedFromEdges,
			"skipped_item_count":  summary.SkippedItemCount,
		})
	}
	return nil
}

func projectSourceItemsToObjectGraph(ctx context.Context, items []sources.Item, now time.Time) (webCaptureProjectionSummary, error) {
	if dispatcher := ingestionRuntimeDispatcherFromEnv(); dispatcher != nil && strings.TrimSpace(dispatcher.baseURL) != "" {
		return dispatcher.projectWebCaptures(ctx, items, now)
	}
	graph, graphPath, err := sourcecycledObjectGraphServiceFromEnv()
	if err != nil {
		return webCaptureProjectionSummary{}, err
	}
	if graph == nil {
		return webCaptureProjectionSummary{}, nil
	}
	result, writeErr := cycle.WriteWebCaptureGraphObjects(ctx, graph, items, cycle.WebCaptureGraphProjectionConfig{
		OwnerID:    sourceServiceObjectGraphOwnerID(),
		ComputerID: sourceServiceObjectGraphComputerID(),
		Now:        now,
	})
	closeErr := graph.Close()
	if writeErr != nil {
		return webCaptureProjectionSummary{}, writeErr
	}
	if closeErr != nil {
		return webCaptureProjectionSummary{}, closeErr
	}
	return webCaptureProjectionSummary{
		Mode:              "sqlite_sidecar",
		Target:            graphPath,
		CaptureCount:      len(result.Captures),
		SourceEntityCount: len(result.SourceEntities),
		CapturedFromEdges: result.EdgeCount,
		SkippedItemCount:  result.Skipped,
	}, nil
}

func backfillSourceItemsToObjectGraphIfEmpty(ctx context.Context, store *cycle.Storage, cycleID string, now time.Time) error {
	if dispatcher := ingestionRuntimeDispatcherFromEnv(); dispatcher != nil && strings.TrimSpace(dispatcher.baseURL) != "" {
		if store == nil {
			return nil
		}
		items, err := store.SearchItems(ctx, "", sourceServiceObjectGraphBackfillLimit())
		if err != nil {
			return err
		}
		if len(items) == 0 {
			_ = store.RecordCycleEvent(ctx, cycleID, "", "web_captures_graph_backfill_empty", "no stored source items available for objectgraph backfill", map[string]any{
				"objectgraph_mode": "runtime_api",
				"backfill_limit":   sourceServiceObjectGraphBackfillLimit(),
			})
			return nil
		}
		summary, err := dispatcher.projectWebCaptures(ctx, items, now)
		if err != nil {
			return err
		}
		_ = store.RecordCycleEvent(ctx, cycleID, "", "web_captures_graph_backfilled", "stored source items projected to empty objectgraph web captures", map[string]any{
			"objectgraph_mode":    summary.Mode,
			"objectgraph_target":  summary.Target,
			"backfill_limit":      sourceServiceObjectGraphBackfillLimit(),
			"backfill_item_count": len(items),
			"capture_count":       summary.CaptureCount,
			"source_entity_count": summary.SourceEntityCount,
			"captured_from_edges": summary.CapturedFromEdges,
			"skipped_item_count":  summary.SkippedItemCount,
		})
		return nil
	}
	graph, graphPath, err := sourcecycledObjectGraphServiceFromEnv()
	if err != nil {
		return err
	}
	if graph == nil {
		return nil
	}
	closeGraph := true
	defer func() {
		if closeGraph {
			_ = graph.Close()
		}
	}()
	notTombstoned := false
	existing, err := graph.ListObjects(ctx, objectgraph.ListFilter{
		Kind:      objectgraph.WebCaptureObjectKind,
		OwnerID:   sourceServiceObjectGraphOwnerID(),
		Limit:     1,
		Tombstone: &notTombstoned,
	})
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		if store != nil {
			_ = store.RecordCycleEvent(ctx, cycleID, "", "web_captures_graph_backfill_skipped", "objectgraph already has Universal Wire web captures", map[string]any{
				"objectgraph_db_path": graphPath,
				"existing_count":      len(existing),
			})
		}
		return nil
	}
	if store == nil {
		return nil
	}
	items, err := store.SearchItems(ctx, "", sourceServiceObjectGraphBackfillLimit())
	if err != nil {
		return err
	}
	if len(items) == 0 {
		_ = store.RecordCycleEvent(ctx, cycleID, "", "web_captures_graph_backfill_empty", "no stored source items available for graph backfill", map[string]any{
			"objectgraph_db_path": graphPath,
		})
		return nil
	}
	result, err := cycle.WriteWebCaptureGraphObjects(ctx, graph, items, cycle.WebCaptureGraphProjectionConfig{
		OwnerID:    sourceServiceObjectGraphOwnerID(),
		ComputerID: sourceServiceObjectGraphComputerID(),
		Now:        now,
	})
	closeErr := graph.Close()
	closeGraph = false
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	_ = store.RecordCycleEvent(ctx, cycleID, "", "web_captures_graph_backfilled", "stored source items projected to empty objectgraph web captures", map[string]any{
		"objectgraph_db_path": graphPath,
		"backfill_limit":      sourceServiceObjectGraphBackfillLimit(),
		"backfill_item_count": len(items),
		"capture_count":       len(result.Captures),
		"source_entity_count": len(result.SourceEntities),
		"captured_from_edges": result.EdgeCount,
		"skipped_item_count":  result.Skipped,
	})
	return nil
}

func ingestionRuntimeDispatcherFromEnv() *ingestionRuntimeDispatcher {
	socketPath := strings.TrimSpace(firstEnv("SOURCECYCLED_VMCTL_PROXY_SOCK", "VMCTL_SANDBOX_PROXY_SOCK"))
	ownerID := strings.TrimSpace(firstEnv("SOURCE_SERVICE_RUNTIME_OWNER_ID", "SOURCECYCLED_RUNTIME_OWNER_ID"))
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	limit := parsePositiveInt(firstEnv("SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS", "SOURCECYCLED_AGENT_DISPATCH_MAX_PROCESSORS"), defaultIngestionProcessorDispatchLimit)
	retries := parsePositiveInt(firstEnv("SOURCE_SERVICE_RUNTIME_DISPATCH_RETRIES", "SOURCECYCLED_RUNTIME_DISPATCH_RETRIES"), defaultIngestionRuntimeDispatchRetries)
	d := &ingestionRuntimeDispatcher{
		ownerID:              ownerID,
		socketPath:           socketPath,
		maxProcessorRequests: limit,
		retryAttempts:        retries,
		inFlightWindow:       time.Duration(parsePositiveInt(firstEnv("SOURCE_SERVICE_AGENT_DISPATCH_INFLIGHT_WINDOW_SECONDS", "SOURCECYCLED_INFLIGHT_WINDOW_SECONDS"), int(defaultIngestionProcessorInFlightWindow/time.Second))) * time.Second,
		retryDelay:           defaultIngestionRuntimeRetryDelay,
	}
	if socketPath != "" {
		d.client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
			Timeout: 5 * time.Minute,
		}
		d.baseURL = "http://unix" // host part is ignored by UDS dialer
	} else {
		baseURL := strings.TrimRight(strings.TrimSpace(firstEnv("SOURCE_SERVICE_RUNTIME_BASE_URL", "SOURCECYCLED_RUNTIME_BASE_URL")), "/")
		if baseURL == "" {
			return nil
		}
		d.baseURL = baseURL
		d.client = &http.Client{Timeout: 20 * time.Second}
	}
	return d
}

func ingestionQueueDrainIntervalFromEnv() time.Duration {
	raw := firstEnv("SOURCE_SERVICE_AGENT_DISPATCH_DRAIN_INTERVAL_SECONDS", "SOURCECYCLED_AGENT_DISPATCH_DRAIN_INTERVAL_SECONDS")
	seconds := parsePositiveInt(raw, int(defaultIngestionQueueDrainInterval/time.Second))
	if seconds < 10 {
		seconds = 10
	}
	return time.Duration(seconds) * time.Second
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func startSourceServiceAPI(ctx context.Context, store *cycle.Storage) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/source-service/health", handleSourceServiceHealth(store))
	mux.HandleFunc("/internal/source-service/search", handleSourceServiceSearch(store))
	mux.HandleFunc("/internal/source-service/ingestion-handoff/latest", handleSourceServiceIngestionHandoffLatest(store))
	mux.HandleFunc("/internal/source-service/items/", handleSourceServiceItem(store))
	server := &http.Server{
		Addr:              sourceServiceAddr(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() {
		log.Printf("Source Service API listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Source Service API stopped with error: %v", err)
			return
		}
		log.Println("Source Service API stopped.")
	}()
	return server
}

func handleSourceServiceHealth(store *cycle.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		itemCount, itemErr := store.CountItems(r.Context())
		fetchCount, fetchErr := store.CountFetches(r.Context())
		status := "ok"
		if itemErr != nil || fetchErr != nil {
			status = "degraded"
		}
		writeSourceServiceJSON(w, http.StatusOK, sourceapi.HealthResponse{
			Status:     status,
			ItemCount:  itemCount,
			FetchCount: fetchCount,
			CheckedAt:  time.Now().UTC(),
		})
	}
}

func handleSourceServiceSearch(store *cycle.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		limit := parsePositiveInt(r.URL.Query().Get("max_results"), 20)
		items, err := store.SearchItems(r.Context(), query, limit)
		if err != nil {
			http.Error(w, "search source items: "+err.Error(), http.StatusInternalServerError)
			return
		}
		results := make([]sourceapi.ItemResult, 0, len(items))
		for idx, item := range items {
			results = append(results, sourceAPIItemResult(idx+1, item))
		}
		writeSourceServiceJSON(w, http.StatusOK, sourceapi.SearchResponse{
			Query:    query,
			Provider: sourceapi.ProviderName,
			Results:  results,
			Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
		})
	}
}

func handleSourceServiceIngestionHandoffLatest(store *cycle.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		summary, err := store.LatestCycleSummary(r.Context())
		if err != nil {
			http.Error(w, "latest ingestion handoff cycle: "+err.Error(), http.StatusNotFound)
			return
		}
		writeSourceServiceJSON(w, http.StatusOK, sourceapi.IngestionHandoffResponse{
			Provider:           sourceapi.ProviderName,
			Cycle:              sourceAPICycleSummary(summary),
			SourceHealth:       sourceAPISourceHealth(summary),
			ProcessorRequests:  sourceAPIProcessorRequests(summary.ProcessorRequests),
			ReconcilerRequests: sourceAPIReconcilerRequests(summary.ReconcilerRequests),
			Metadata: sourceapi.IngestionHandoffMetadata{
				Topology:      "source-items -> processor-handoffs -> corpus-reconciler-handoff",
				AuthorityRule: "source and version provenance stay in source items and Texture; handoffs are queues, not publication authority",
			},
		})
	}
}

func handleSourceServiceItem(store *cycle.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		itemID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/source-service/items/"), "/")
		if itemID == "" {
			http.Error(w, "item id is required", http.StatusBadRequest)
			return
		}
		item, err := store.GetItem(r.Context(), itemID)
		if err != nil {
			http.Error(w, "resolve source item: "+err.Error(), http.StatusNotFound)
			return
		}
		writeSourceServiceJSON(w, http.StatusOK, sourceapi.ResolveItemResponse{
			Provider: sourceapi.ProviderName,
			Item:     sourceAPIItemResult(1, item),
		})
	}
}

func sourceAPICycleSummary(summary cycle.CycleSummary) sourceapi.CycleSummary {
	return sourceapi.CycleSummary{
		CycleID:    summary.CycleID,
		StartedAt:  formatSourceTime(summary.StartedAt),
		EndedAt:    formatSourceTime(summary.EndedAt),
		Status:     summary.Status,
		ItemCount:  summary.ItemCount,
		FetchCount: summary.FetchCount,
		Error:      summary.Error,
		Events:     sourceAPICycleEvents(summary.Events),
	}
}

func sourceAPICycleEvents(events []cycle.CycleEvent) []sourceapi.CycleEventSummary {
	if len(events) == 0 {
		return nil
	}
	out := make([]sourceapi.CycleEventSummary, 0, len(events))
	for _, event := range events {
		out = append(out, sourceapi.CycleEventSummary{
			EventID:   event.EventID,
			SourceID:  event.SourceID,
			Kind:      event.Kind,
			Message:   event.Message,
			Metadata:  event.Metadata,
			CreatedAt: formatSourceTime(event.CreatedAt),
		})
	}
	return out
}

func sourceAPISourceHealth(summary cycle.CycleSummary) sourceapi.SourceHealth {
	health := sourceapi.SourceHealth{
		ConfiguredSourceCount: summary.FetchCount,
	}
	for _, fetch := range summary.Fetches {
		if sourceFetchStatusCountsAsSuccess(fetch.Status) {
			health.SuccessFetchCount++
		} else {
			health.FailedFetchCount++
			health.Failures = append(health.Failures, sourceapi.SourceFetchSummary{
				SourceID:     fetch.SourceID,
				SourceType:   string(fetch.SourceType),
				Status:       fetch.Status,
				StatusCode:   fetch.StatusCode,
				ErrorClass:   fetch.ErrorClass,
				Error:        fetch.Error,
				StartedAt:    formatSourceTime(fetch.StartedAt),
				EndedAt:      formatSourceTime(fetch.EndedAt),
				RequestURL:   fetch.RequestURL,
				CanonicalURL: fetch.CanonicalURL,
			})
		}
		if fetch.ItemCount > 0 {
			health.ItemProducingSourceCount++
		}
		health.ItemCount += fetch.ItemCount
		health.Fetches = append(health.Fetches, sourceapi.SourceFetchSummary{
			SourceID:     fetch.SourceID,
			SourceType:   string(fetch.SourceType),
			Status:       fetch.Status,
			StatusCode:   fetch.StatusCode,
			ErrorClass:   fetch.ErrorClass,
			Error:        fetch.Error,
			ItemCount:    fetch.ItemCount,
			StartedAt:    formatSourceTime(fetch.StartedAt),
			EndedAt:      formatSourceTime(fetch.EndedAt),
			RequestURL:   fetch.RequestURL,
			CanonicalURL: fetch.CanonicalURL,
		})
	}
	return health
}

func sourceFetchStatusCountsAsSuccess(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "ok", "not_modified":
		return true
	default:
		return false
	}
}

func sourceAPIProcessorRequests(requests []cycle.ProcessorRequest) []sourceapi.ProcessorRequest {
	out := make([]sourceapi.ProcessorRequest, 0, len(requests))
	for _, req := range requests {
		out = append(out, sourceapi.ProcessorRequest{
			RequestID:     req.RequestID,
			CycleID:       req.CycleID,
			ProcessorKey:  req.ProcessorKey,
			Status:        req.Status,
			RuntimeRunID:  req.RuntimeRunID,
			RuntimeStatus: req.RuntimeStatus,
			SourceItemIDs: req.SourceItemIDs,
			SourceCount:   req.SourceCount,
			SourceTypes:   req.SourceTypes,
			Verticals:     req.Verticals,
			Regions:       req.Regions,
			ContinuityRef: req.ContinuityRef,
			Prompt:        req.Prompt,
			CreatedAt:     formatSourceTime(req.CreatedAt),
			UpdatedAt:     formatSourceTime(req.UpdatedAt),
		})
	}
	return out
}

func sourceAPIReconcilerRequests(requests []cycle.ReconcilerRequest) []sourceapi.ReconcilerRequest {
	out := make([]sourceapi.ReconcilerRequest, 0, len(requests))
	for _, req := range requests {
		out = append(out, sourceapi.ReconcilerRequest{
			RequestID:           req.RequestID,
			CycleID:             req.CycleID,
			Status:              req.Status,
			RuntimeRunID:        req.RuntimeRunID,
			Scope:               req.Scope,
			SourceItemIDs:       req.SourceItemIDs,
			ProcessorRequestIDs: req.ProcessorRequestIDs,
			Prompt:              req.Prompt,
			CreatedAt:           formatSourceTime(req.CreatedAt),
			UpdatedAt:           formatSourceTime(req.UpdatedAt),
		})
	}
	return out
}

func sourceAPIItemResult(rank int, item sources.Item) sourceapi.ItemResult {
	item = sources.NormalizeItemBodyClassification(item)
	return sourceapi.ItemResult{
		Rank:               rank,
		TargetKind:         sourceapi.TargetKind,
		ItemID:             item.ID,
		SourceID:           item.SourceID,
		SourceType:         string(item.SourceType),
		FetchID:            item.FetchID,
		OriginalID:         item.OriginalID,
		Title:              item.Title,
		Body:               item.Body,
		URL:                item.URL,
		CanonicalURL:       item.CanonicalURL,
		PublishedAt:        formatSourceTime(item.Published),
		FetchedAt:          formatSourceTime(item.FetchedAt),
		Verticals:          item.Verticals,
		Language:           item.Language,
		Region:             item.Region,
		ContentHash:        item.ContentHash,
		BodyKind:           item.BodyKind,
		BodyLength:         item.BodyLength,
		ReaderSnapshot:     item.ReaderSnapshot,
		SourceTOSClass:     item.SourceTOSClass,
		SourceRobotsPolicy: item.SourceRobotsPolicy,
		SourceAuthPolicy:   item.SourceAuthPolicy,
		StoreBodyPolicy:    item.StoreBodyPolicy,
		EvidenceLevel:      item.EvidenceLevel,
		VintagePolicy:      item.VintagePolicy,
		LookaheadStatus:    item.LookaheadStatus,
		ReleaseDate:        item.ReleaseDate,
	}
}

func writeSourceServiceJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write source service response: %v", err)
	}
}

func isTerminalRuntimeState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// processorRunProjection classifies what the runtime's trajectory and
// processor-resolution state imply for the request verdict. The cases are
// mutually exclusive: they require distinct trajectory statuses, or distinct
// resolution states under the same status.
type processorRunProjection int

const (
	projectionNone processorRunProjection = iota
	projectionPublishedCorpusCoverage
	projectionExplicitNoStoryTerminal
	projectionDeferredWithoutStory
	projectionPublicationSettled
)

func classifyProcessorRunProjection(run runtimeRunStatusResponse) processorRunProjection {
	trajectoryStatus := ""
	if run.Trajectory != nil {
		trajectoryStatus = strings.ToLower(strings.TrimSpace(run.Trajectory.Status))
	}
	resolutionCompleted := false
	resolutionState := ""
	coveredByDocID := ""
	if run.ProcessorResolution != nil {
		resolutionCompleted = strings.EqualFold(strings.TrimSpace(run.ProcessorResolution.Status), "completed")
		resolutionState = strings.ToLower(strings.TrimSpace(run.ProcessorResolution.ResolutionState))
		coveredByDocID = strings.TrimSpace(run.ProcessorResolution.CoveredByDocID)
	}
	switch {
	case trajectoryStatus == "cancelled" && resolutionCompleted &&
		resolutionState == sourceapi.ResolutionStateSuppressedAgainstPublishedCorpus && coveredByDocID != "":
		return projectionPublishedCorpusCoverage
	case trajectoryStatus == "cancelled" && resolutionCompleted &&
		resolutionState == sourceapi.ResolutionStateDecidedWithoutStoryRoute:
		return projectionExplicitNoStoryTerminal
	case trajectoryStatus == "live" && resolutionState == sourceapi.ResolutionStateDeferredWithoutStoryRoute:
		return projectionDeferredWithoutStory
	case trajectoryStatus == "settled":
		return projectionPublicationSettled
	default:
		return projectionNone
	}
}

// processorStoryRouteCompleted reports whether the processor request resolved
// every source item with a story route; that releases runtime admission
// capacity even while the run tree is still working the story.
func processorStoryRouteCompleted(run runtimeRunStatusResponse) bool {
	return run.ProcessorResolution != nil &&
		strings.EqualFold(strings.TrimSpace(run.ProcessorResolution.Status), "completed") &&
		strings.EqualFold(strings.TrimSpace(run.ProcessorResolution.ResolutionState), sourceapi.ResolutionStateDecidedWithStoryRoute)
}

// processorRunReconcileDecision projects a runtime run status onto at most
// one verdict write and one runtime-status write. The deferred projection
// intentionally records a verdict only once the run itself is terminal.
func processorRunReconcileDecision(run runtimeRunStatusResponse) (verdict, runtimeStatus string) {
	projection := classifyProcessorRunProjection(run)
	switch projection {
	case projectionPublishedCorpusCoverage, projectionExplicitNoStoryTerminal, projectionPublicationSettled:
		verdict = "completed"
	}
	if processorStoryRouteCompleted(run) {
		runtimeStatus = "completed"
	}
	if isTerminalRuntimeState(run.State) {
		runtimeStatus = strings.ToLower(strings.TrimSpace(run.State))
		switch {
		case projection == projectionDeferredWithoutStory:
			verdict = "deferred"
		case verdict == "" && (strings.EqualFold(run.State, "failed") || strings.EqualFold(run.State, "cancelled")):
			verdict = "dispatch_failed"
		}
	}
	return verdict, runtimeStatus
}

func (d *ingestionRuntimeDispatcher) getRunStatus(ctx context.Context, runID string) (runtimeRunStatusResponse, error) {
	var zero runtimeRunStatusResponse
	baseURL := strings.TrimRight(strings.TrimSpace(d.baseURL), "/")
	if baseURL == "" {
		return zero, fmt.Errorf("runtime base URL is not configured")
	}
	endpoint, err := url.Parse(baseURL + "/internal/runtime/runs/" + url.PathEscape(strings.TrimSpace(runID)))
	if err != nil {
		return zero, fmt.Errorf("parse runtime status URL: %w", err)
	}
	q := endpoint.Query()
	q.Set("owner_id", d.ownerID)
	endpoint.RawQuery = q.Encode()
	client := d.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return zero, fmt.Errorf("build runtime status request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return zero, fmt.Errorf("read runtime status response: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("runtime status returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out runtimeRunStatusResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return zero, fmt.Errorf("decode runtime status response: %w", err)
	}
	return out, nil
}

func (d *ingestionRuntimeDispatcher) reconcileSubmittedProcessorRequests(ctx context.Context, store *cycle.Storage) error {
	if d == nil || store == nil {
		return nil
	}
	submitted, err := store.ListReconcilableProcessorRequests(ctx, 128)
	if err != nil {
		return err
	}
	for _, req := range submitted {
		runID := strings.TrimSpace(req.RuntimeRunID)
		if runID == "" {
			continue
		}
		run, err := d.getRunStatus(ctx, runID)
		if err != nil {
			msg := strings.ToLower(err.Error())
			if (strings.Contains(msg, "returned 404") || strings.Contains(msg, "not found")) && strings.EqualFold(strings.TrimSpace(req.RuntimeStatus), "submitted") {
				if resetErr := store.ResetProcessorRequestSubmission(ctx, req.RequestID); resetErr != nil {
					log.Printf("sourcecycled: reset missing runtime run %s for %s: %v", runID, req.RequestID, resetErr)
				}
			}
			continue
		}
		verdict, runtimeStatus := processorRunReconcileDecision(run)
		if runtimeStatus != "" {
			if err := store.UpdateProcessorRequestRuntimeStatus(ctx, req.RequestID, runtimeStatus, runID); err != nil {
				log.Printf("sourcecycled: reconcile submitted request runtime %s -> %s: %v", req.RequestID, runtimeStatus, err)
			}
		}
		if verdict != "" {
			if err := store.UpdateProcessorRequestVerdictStatus(ctx, req.RequestID, verdict); err != nil {
				log.Printf("sourcecycled: reconcile submitted request %s -> %s: %v", req.RequestID, verdict, err)
			}
		}
	}
	return nil
}

func (d *ingestionRuntimeDispatcher) dispatch(ctx context.Context, store *cycle.Storage, handoff cycle.IngestionHandoff) ingestionDispatchResult {
	var result ingestionDispatchResult
	if d == nil || strings.TrimSpace(d.baseURL) == "" {
		result.ProcessorSkipped = len(handoff.ProcessorRequests)
		return result
	}
	if store != nil {
		if err := d.reconcileSubmittedProcessorRequests(ctx, store); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("reconcile submitted processors: %v", err))
		}
	}
	processorLimit := d.maxProcessorRequests
	if processorLimit <= 0 {
		processorLimit = defaultIngestionProcessorDispatchLimit
	}
	processorRequests := handoff.ProcessorRequests
	if store != nil {
		queuedCount, err := store.CountQueuedProcessorRequests(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("count queued processors: %v", err))
			result.ProcessorSkipped += len(handoff.ProcessorRequests)
			return result
		}
		queued, err := store.ListQueuedProcessorRequests(ctx, processorLimit)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("list queued processors: %v", err))
			result.ProcessorSkipped += len(handoff.ProcessorRequests)
			return result
		}
		processorRequests = queued
		result.ProcessorSkipped += maxInt(0, queuedCount-len(queued))
	}
	// Backpressure: count recently submitted (in-flight) processors and limit new submissions
	inFlight := 0
	if store != nil {
		var err error
		inFlight, err = store.CountRecentlySubmittedProcessorRequests(ctx, time.Now().UTC().Add(-d.inFlightWindow))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("count in-flight processors: %v", err))
			// Fall through — conservative: treat as overload and skip remaining
			result.ProcessorSkipped += len(processorRequests)
			return result
		}
	}
	submitCap := processorLimit - inFlight
	log.Printf("Dispatch backpressure: in-flight=%d submitCap=%d (%d - %d)", inFlight, submitCap, processorLimit, inFlight)
	if submitCap <= 0 {
		result.ProcessorSkipped += len(processorRequests)
		return result
	}
	for _, req := range processorRequests {
		if !cycle.ProcessorRequestEligibleForDispatch(req) {
			result.ProcessorSkipped++
			continue
		}
		if store != nil {
			ok, err := store.ValidateProcessorRequestIngestionEvents(ctx, req)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: validate ingestion events: %v", req.RequestID, err))
				result.ProcessorSkipped++
				continue
			}
			if !ok {
				result.ProcessorSkipped++
				continue
			}
		}
		run, err := d.submitProcessor(ctx, req)
		if err != nil {
			if isTransientRuntimeSubmitError(err) {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: transient runtime unavailable: %v", req.RequestID, err))
				break
			}
			result.ProcessorFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", req.RequestID, err))
			if store != nil {
				_ = store.UpdateProcessorRequestRuntimeRun(ctx, req.RequestID, "dispatch_failed", "")
			}
			continue
		}
		result.ProcessorSubmitted++
		result.RunIDs = append(result.RunIDs, run.RunID)
		if store != nil {
			_ = store.UpdateProcessorRequestRuntimeRun(ctx, req.RequestID, "submitted", run.RunID)
		}
		// Enforce per-drain submission cap: stop if we have submitted enough
		if result.ProcessorSubmitted >= submitCap {
			break
		}
	}
	// Story-corpus reconciler dispatches from wire publish debounce (runtime), not ingestion.
	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (d *ingestionRuntimeDispatcher) submitProcessor(ctx context.Context, req cycle.ProcessorRequest) (runtimeRunStatusResponse, error) {
	prompt := req.Prompt + "\n\nIngestion processor request: " + req.RequestID +
		"\nCycle: " + req.CycleID +
		"\nProcessor key: " + req.ProcessorKey +
		"\nContinuity ref: " + req.ContinuityRef +
		"\nSource item handles: " + strings.Join(req.SourceItemIDs, ", ") +
		"\nDo not paste source bodies into the checkpoint. Use source_search/fetch_url by handle or URL when needed, preserve source handles, and spawn Texture agents when a story should be opened or revised."
	channelID := "processor-v2:" + strings.ReplaceAll(req.ProcessorKey, ":", "-")
	agentID := "processor-v2:" + strings.ReplaceAll(req.ProcessorKey, ":", "-")
	return d.submit(ctx, runtimeRunSubmitRequest{
		OwnerID: d.ownerID,
		Prompt:  prompt,
		Metadata: map[string]any{
			"channel_id":                     channelID,
			"agent_id":                       agentID,
			"agent_profile":                  "processor",
			"agent_role":                     "processor",
			"request_source":                 "sourcecycled",
			"activation_origin":              "ingestion_event",
			"ingestion_event_ids":            req.IngestionEventIDs,
			"source_network_cycle_id":        req.CycleID,
			"source_network_request_id":      req.RequestID,
			"source_network_request_kind":    "processor",
			"ingestion_handoff_request_kind": "processor",
			"ingestion_handoff_request_id":   req.RequestID,
			"ingestion_handoff_cycle_id":     req.CycleID,
			"processor_key":                  req.ProcessorKey,
			"source_item_ids":                req.SourceItemIDs,
			"source_count":                   req.SourceCount,
			"source_types":                   req.SourceTypes,
			"verticals":                      req.Verticals,
			"regions":                        req.Regions,
			"continuity_ref":                 req.ContinuityRef,
		},
	})
}

func (d *ingestionRuntimeDispatcher) submitReconciler(ctx context.Context, req cycle.ReconcilerRequest) (runtimeRunStatusResponse, error) {
	prompt := req.Prompt + "\n\nIngestion reconciler request: " + req.RequestID +
		"\nCycle: " + req.CycleID +
		"\nScope: " + req.Scope +
		"\nProcessor request handles: " + strings.Join(req.ProcessorRequestIDs, ", ") +
		"\nSource item handles: " + strings.Join(req.SourceItemIDs, ", ") +
		"\nReview the story corpus and source/processor state. Note consensus, contradictions, drift, research needs, and candidate Texture updates without mutating platform stories."
	return d.submit(ctx, runtimeRunSubmitRequest{
		OwnerID: d.ownerID,
		Prompt:  prompt,
		Metadata: map[string]any{
			"agent_profile":                  "reconciler",
			"agent_role":                     "reconciler",
			"request_source":                 "sourcecycled",
			"ingestion_handoff_request_kind": "reconciler",
			"ingestion_handoff_request_id":   req.RequestID,
			"ingestion_handoff_cycle_id":     req.CycleID,
			"reconciler_scope":               req.Scope,
			"source_item_ids":                req.SourceItemIDs,
			"processor_request_ids":          req.ProcessorRequestIDs,
		},
	})
}

func (d *ingestionRuntimeDispatcher) submit(ctx context.Context, payload runtimeRunSubmitRequest) (runtimeRunStatusResponse, error) {
	if d == nil || d.client == nil {
		return runtimeRunStatusResponse{}, fmt.Errorf("runtime dispatcher is not configured")
	}
	attempts := d.retryAttempts
	if attempts <= 0 {
		attempts = defaultIngestionRuntimeDispatchRetries
	}
	delay := d.retryDelay
	if delay <= 0 {
		delay = defaultIngestionRuntimeRetryDelay
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		out, err := d.submitOnce(ctx, payload)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !isTransientRuntimeSubmitError(err) || attempt == attempts {
			break
		}
		log.Printf("Ingestion runtime dispatch attempt %d/%d failed transiently: %v", attempt, attempts, err)
		select {
		case <-ctx.Done():
			return runtimeRunStatusResponse{}, ctx.Err()
		case <-time.After(delay):
		}
	}
	return runtimeRunStatusResponse{}, lastErr
}

func (d *ingestionRuntimeDispatcher) runtimeRunsEndpoint() string {
	if d.socketPath != "" {
		return d.baseURL + "/internal/vmctl/sandbox-proxy/" + d.ownerID + "/internal/runtime/runs"
	}
	return d.baseURL + "/internal/runtime/runs"
}

func (d *ingestionRuntimeDispatcher) runtimeWebCapturesEndpoint() string {
	if d.socketPath != "" {
		return d.baseURL + "/internal/vmctl/sandbox-proxy/" + d.ownerID + "/internal/runtime/objectgraph/web-captures"
	}
	return d.baseURL + "/internal/runtime/objectgraph/web-captures"
}

func (d *ingestionRuntimeDispatcher) projectWebCaptures(ctx context.Context, items []sources.Item, now time.Time) (webCaptureProjectionSummary, error) {
	if d == nil || d.client == nil {
		return webCaptureProjectionSummary{}, fmt.Errorf("runtime dispatcher is not configured")
	}
	payload := runtimeWebCaptureProjectionRequest{
		OwnerID:    d.ownerID,
		ComputerID: sourceServiceObjectGraphComputerID(),
		Items:      items,
		Now:        now.UTC().Format(time.RFC3339Nano),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return webCaptureProjectionSummary{}, fmt.Errorf("marshal runtime web capture projection request: %w", err)
	}
	endpoint := d.runtimeWebCapturesEndpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return webCaptureProjectionSummary{}, fmt.Errorf("create runtime web capture projection request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Caller", "true")
	resp, err := d.client.Do(httpReq)
	if err != nil {
		return webCaptureProjectionSummary{}, fmt.Errorf("project web captures into runtime: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(resp.Body)
		return webCaptureProjectionSummary{}, runtimeSubmitError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Message:    strings.TrimSpace(string(responseBody)),
		}
	}
	var out runtimeWebCaptureProjectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return webCaptureProjectionSummary{}, fmt.Errorf("decode runtime web capture projection response: %w", err)
	}
	return webCaptureProjectionSummary{
		Mode:              "runtime_api",
		Target:            endpoint,
		CaptureCount:      out.CaptureCount,
		SourceEntityCount: out.SourceEntityCount,
		CapturedFromEdges: out.CapturedFromEdges,
		SkippedItemCount:  out.SkippedItemCount,
	}, nil
}

func (d *ingestionRuntimeDispatcher) submitOnce(ctx context.Context, payload runtimeRunSubmitRequest) (runtimeRunStatusResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return runtimeRunStatusResponse{}, fmt.Errorf("marshal runtime run request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.runtimeRunsEndpoint(), bytes.NewReader(body))
	if err != nil {
		return runtimeRunStatusResponse{}, fmt.Errorf("create runtime run request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Caller", "true")
	resp, err := d.client.Do(httpReq)
	if err != nil {
		return runtimeRunStatusResponse{}, fmt.Errorf("submit runtime run: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if strings.TrimSpace(apiErr.Error) == "" {
			apiErr.Error = resp.Status
		}
		return runtimeRunStatusResponse{}, runtimeSubmitError{StatusCode: resp.StatusCode, Status: resp.Status, Message: apiErr.Error}
	}
	var out runtimeRunStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return runtimeRunStatusResponse{}, fmt.Errorf("decode runtime run response: %w", err)
	}
	if strings.TrimSpace(out.RunID) == "" {
		return runtimeRunStatusResponse{}, fmt.Errorf("runtime accepted run without loop_id")
	}
	return out, nil
}

type runtimeSubmitError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e runtimeSubmitError) Error() string {
	return fmt.Sprintf("runtime returned %s: %s", e.Status, e.Message)
}

func isTransientRuntimeSubmitError(err error) bool {
	if err == nil {
		return false
	}
	var statusErr runtimeSubmitError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= 500
	}
	return true
}

func ingestionDispatchResultHasActivity(result ingestionDispatchResult) bool {
	return result.ProcessorSubmitted > 0 ||
		result.ReconcilerSubmitted > 0 ||
		result.ProcessorFailed > 0 ||
		result.ReconcilerFailed > 0 ||
		result.ProcessorSkipped > 0 ||
		result.ReconcilerSkipped > 0 ||
		len(result.Errors) > 0
}

func parsePositiveInt(raw string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

func formatSourceTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

var engine *cycle.Engine

func runCycle(ctx context.Context, registry *sources.Registry, store *cycle.Storage) {
	if engine == nil {
		engine = cycle.NewEngine(registry)
	}

	cycleStartTime := time.Now()
	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		log.Printf("Failed to start durable cycle: %v", err)
		return
	}
	log.Printf("Cycle started at %v", cycleStartTime)
	_ = store.RecordCycleEvent(ctx, cycleID, "", "cycle_started", "source cycle started", nil)

	// Phase 1 & 2: Source Polling & Deduplication
	pollResult := engine.PollAll(ctx)
	if err := store.SaveSourcePollState(registry); err != nil {
		log.Printf("Failed to save source poll state: %v", err)
	}
	items := pollResult.Items
	if err := store.SaveCycleFetches(cycleID, pollResult.Fetches); err != nil {
		log.Printf("Failed to save fetch records: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	log.Printf("Fetched and deduped %d new items", len(items))

	if len(items) == 0 {
		log.Println("No new items found in this cycle. Skipping synthesis.")
		_ = store.RecordCycleEvent(ctx, cycleID, "", "cycle_completed_empty", "no new items found", map[string]any{"fetch_count": len(pollResult.Fetches)})
		if err := backfillSourceItemsToObjectGraphIfEmpty(ctx, store, cycleID, time.Now().UTC()); err != nil {
			log.Printf("Failed to backfill sourcecycled web captures to objectgraph: %v", err)
			_ = store.FinishCycle(ctx, cycleID, "error", 0, len(pollResult.Fetches), err)
			return
		}
		dispatchResult := ingestionRuntimeDispatcherFromEnv().dispatch(ctx, store, cycle.IngestionHandoff{})
		if ingestionDispatchResultHasActivity(dispatchResult) {
			_ = store.RecordCycleEvent(ctx, cycleID, "", "ingestion_handoff_queue_drain", "queued ingestion handoffs drained during empty source cycle", map[string]any{
				"processor_submitted":  dispatchResult.ProcessorSubmitted,
				"processor_failed":     dispatchResult.ProcessorFailed,
				"processor_skipped":    dispatchResult.ProcessorSkipped,
				"reconciler_submitted": dispatchResult.ReconcilerSubmitted,
				"reconciler_failed":    dispatchResult.ReconcilerFailed,
				"reconciler_skipped":   dispatchResult.ReconcilerSkipped,
				"runtime_run_ids":      dispatchResult.RunIDs,
				"errors":               dispatchResult.Errors,
			})
		}
		_ = store.FinishCycle(ctx, cycleID, "completed", 0, len(pollResult.Fetches), nil)
		return
	}

	if err := store.SaveItems(items); err != nil {
		log.Printf("Failed to save items: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	now := time.Now().UTC()
	if err := writeSourceItemsToObjectGraph(ctx, store, cycleID, items, now, "web_captures_graph_written", "source items projected to objectgraph web captures"); err != nil {
		log.Printf("Failed to write sourcecycled web captures to objectgraph: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	ingestionEvents := cycle.BuildIngestionEventsFromItems(cycleID, items, now)
	if err := store.SaveIngestionEvents(ctx, ingestionEvents); err != nil {
		log.Printf("Failed to save ingestion events: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	_ = store.RecordCycleEvent(ctx, cycleID, "", "items_saved", "source items saved", map[string]any{"item_count": len(items), "fetch_count": len(pollResult.Fetches)})
	_ = store.RecordCycleEvent(ctx, cycleID, "", "ingestion_events_emitted", "source fetch emitted ingestion activation events", map[string]any{
		"ingestion_event_count": len(ingestionEvents),
		"item_count":            len(items),
	})

	handoff := cycle.BuildIngestionHandoff(cycleID, items, ingestionEvents, now)
	if err := store.SaveProcessorRequests(ctx, handoff.ProcessorRequests); err != nil {
		log.Printf("Failed to save processor requests: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	if err := store.SaveReconcilerRequests(ctx, handoff.ReconcilerRequests); err != nil {
		log.Printf("Failed to save reconciler requests: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	supersededProcessors, err := store.SupersedeQueuedProcessorRequests(ctx, handoff.ProcessorRequests)
	if err != nil {
		log.Printf("Failed to supersede stale processor requests: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	supersededReconcilers, err := store.SupersedeQueuedReconcilersWithSupersededProcessors(ctx)
	if err != nil {
		log.Printf("Failed to supersede stale reconciler requests: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	_ = store.RecordCycleEvent(ctx, cycleID, "", "ingestion_handoffs_queued", "source items routed to processor and reconciler handoffs", map[string]any{
		"processor_request_count":      len(handoff.ProcessorRequests),
		"reconciler_request_count":     len(handoff.ReconcilerRequests),
		"source_item_count":            len(items),
		"superseded_processor_count":   supersededProcessors,
		"superseded_reconciler_count":  supersededReconcilers,
		"processor_continuity_refresh": supersededProcessors > 0,
	})
	dispatchResult := ingestionRuntimeDispatcherFromEnv().dispatch(ctx, store, handoff)
	if ingestionDispatchResultHasActivity(dispatchResult) {
		_ = store.RecordCycleEvent(ctx, cycleID, "", "ingestion_handoff_runs_dispatched", "ingestion handoffs submitted to processor/reconciler agent profiles", map[string]any{
			"processor_submitted":  dispatchResult.ProcessorSubmitted,
			"processor_failed":     dispatchResult.ProcessorFailed,
			"processor_skipped":    dispatchResult.ProcessorSkipped,
			"reconciler_submitted": dispatchResult.ReconcilerSubmitted,
			"reconciler_failed":    dispatchResult.ReconcilerFailed,
			"reconciler_skipped":   dispatchResult.ReconcilerSkipped,
			"runtime_run_ids":      dispatchResult.RunIDs,
			"errors":               dispatchResult.Errors,
		})
	}

	cycleDuration := time.Since(cycleStartTime)
	_ = store.RecordCycleEvent(ctx, cycleID, "", "cycle_completed", "source cycle completed", map[string]any{"duration_ms": cycleDuration.Milliseconds(), "item_count": len(items), "fetch_count": len(pollResult.Fetches)})
	_ = store.FinishCycle(ctx, cycleID, "completed", len(items), len(pollResult.Fetches), nil)
	log.Printf("Cycle completed in %v", cycleDuration)
	log.Printf("Queued %d processor request(s) and %d reconciler request(s)", len(handoff.ProcessorRequests), len(handoff.ReconcilerRequests))
}

func dispatchQueuedIngestionHandoffs(ctx context.Context, store *cycle.Storage) {
	if store == nil {
		return
	}
	dispatcher := ingestionRuntimeDispatcherFromEnv()
	if dispatcher == nil {
		log.Println("Queued ingestion handoff dispatch drain skipped: runtime dispatcher is not configured")
		return
	}
	result := dispatcher.dispatch(ctx, store, cycle.IngestionHandoff{})
	if !ingestionDispatchResultHasActivity(result) {
		log.Println("Queued ingestion handoff dispatch drain found no dispatchable work")
		return
	}
	log.Printf("Queued ingestion handoff dispatch drain: processor_submitted=%d processor_failed=%d processor_skipped=%d reconciler_submitted=%d reconciler_failed=%d reconciler_skipped=%d errors=%d",
		result.ProcessorSubmitted,
		result.ProcessorFailed,
		result.ProcessorSkipped,
		result.ReconcilerSubmitted,
		result.ReconcilerFailed,
		result.ReconcilerSkipped,
		len(result.Errors),
	)
	for _, errText := range result.Errors {
		log.Printf("Queued ingestion handoff dispatch drain error: %s", errText)
	}
}
