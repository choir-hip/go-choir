package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/cycle"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Choir Global Wire sourcecycled daemon (V0)")

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

	// 2. Setup Context and Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, terminating...")
		cancel()
	}()

	server := startSourceServiceAPI(ctx, store)

	// 3. Main Ingestion Loop (15-minute cycle)
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
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

func startSourceServiceAPI(ctx context.Context, store *cycle.Storage) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/source-service/health", handleSourceServiceHealth(store))
	mux.HandleFunc("/internal/source-service/search", handleSourceServiceSearch(store))
	mux.HandleFunc("/internal/source-service/sourcemaxx/latest", handleSourceServiceSourceMaxxLatest(store))
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

func handleSourceServiceSourceMaxxLatest(store *cycle.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		summary, err := store.LatestCycleSummary(r.Context())
		if err != nil {
			http.Error(w, "latest sourcemaxx cycle: "+err.Error(), http.StatusNotFound)
			return
		}
		writeSourceServiceJSON(w, http.StatusOK, sourceapi.SourceMaxxResponse{
			Provider:           sourceapi.ProviderName,
			Cycle:              sourceAPICycleSummary(summary),
			ProcessorRequests:  sourceAPIProcessorRequests(summary.ProcessorRequests),
			ReconcilerRequests: sourceAPIReconcilerRequests(summary.ReconcilerRequests),
			Metadata: sourceapi.SourceMaxxMetadata{
				Topology:      "source-items -> processor-handoffs -> corpus-reconciler-handoff",
				AuthorityRule: "source and version provenance stay in source items and VText; handoffs are queues, not publication authority",
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
	return sourceapi.ItemResult{
		Rank:            rank,
		TargetKind:      sourceapi.TargetKind,
		ItemID:          item.ID,
		SourceID:        item.SourceID,
		SourceType:      string(item.SourceType),
		FetchID:         item.FetchID,
		OriginalID:      item.OriginalID,
		Title:           item.Title,
		Body:            item.Body,
		URL:             item.URL,
		CanonicalURL:    item.CanonicalURL,
		PublishedAt:     formatSourceTime(item.Published),
		FetchedAt:       formatSourceTime(item.FetchedAt),
		Verticals:       item.Verticals,
		Language:        item.Language,
		Region:          item.Region,
		ContentHash:     item.ContentHash,
		EvidenceLevel:   item.EvidenceLevel,
		VintagePolicy:   item.VintagePolicy,
		LookaheadStatus: item.LookaheadStatus,
		ReleaseDate:     item.ReleaseDate,
	}
}

func writeSourceServiceJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write source service response: %v", err)
	}
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
	items := pollResult.Items
	if err := store.SaveFetches(pollResult.Fetches); err != nil {
		log.Printf("Failed to save fetch records: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	log.Printf("Fetched and deduped %d new items", len(items))

	if len(items) == 0 {
		log.Println("No new items found in this cycle. Skipping synthesis.")
		_ = store.RecordCycleEvent(ctx, cycleID, "", "cycle_completed_empty", "no new items found", map[string]any{"fetch_count": len(pollResult.Fetches)})
		_ = store.FinishCycle(ctx, cycleID, "completed", 0, len(pollResult.Fetches), nil)
		return
	}

	if err := store.SaveItems(items); err != nil {
		log.Printf("Failed to save items: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}
	_ = store.RecordCycleEvent(ctx, cycleID, "", "items_saved", "source items saved", map[string]any{"item_count": len(items), "fetch_count": len(pollResult.Fetches)})

	handoff := cycle.BuildSourceMaxxHandoff(cycleID, items, time.Now().UTC())
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
	_ = store.RecordCycleEvent(ctx, cycleID, "", "sourcemaxx_handoffs_queued", "source items routed to processor and reconciler handoffs", map[string]any{
		"processor_request_count":  len(handoff.ProcessorRequests),
		"reconciler_request_count": len(handoff.ReconcilerRequests),
		"source_item_count":        len(items),
	})

	cycleDuration := time.Since(cycleStartTime)
	_ = store.RecordCycleEvent(ctx, cycleID, "", "cycle_completed", "source cycle completed", map[string]any{"duration_ms": cycleDuration.Milliseconds(), "item_count": len(items), "fetch_count": len(pollResult.Fetches)})
	_ = store.FinishCycle(ctx, cycleID, "completed", len(items), len(pollResult.Fetches), nil)
	log.Printf("Cycle completed in %v", cycleDuration)
	log.Printf("Queued %d processor request(s) and %d reconciler request(s)", len(handoff.ProcessorRequests), len(handoff.ReconcilerRequests))
}
