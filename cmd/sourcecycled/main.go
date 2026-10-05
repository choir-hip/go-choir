package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/cycle"
	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

const (
	defaultObjectGraphBackfillLimit = 50
)

type webCaptureProjectionSummary struct {
	Mode              string
	Target            string
	CaptureCount      int
	SourceEntityCount int
	CapturedFromEdges int
	SkippedItemCount  int
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

	// Keep source poll, cycle, and dispatch state in the platform Dolt store so
	// deploys do not replay the whole corpus or erase in-flight request history.
	var store *cycle.Storage
	var storeErr error
	for attempt := 1; attempt <= 20; attempt++ {
		store, storeErr = cycle.NewStorage(sourceServiceDBDSN())
		if storeErr == nil {
			break
		}
		log.Printf("sourcecycled store: attempt %d/20: %v", attempt, storeErr)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	if storeErr != nil {
		log.Fatalf("Failed to initialize source service storage: %v", storeErr)
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
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, terminating...")
		cancel()
	}()

	server := startSourceServiceAPI(ctx, store)

	// 3. Main ingestion loop (per-source-type tickers).
	//
	// GDELT stays on a 15-minute cadence. RSS and Telegram get faster
	// configurable intervals so high-frequency sources are not gated by the
	// slowest one. Each ticker runs its own cycle filtered to that source
	// type.
	rssTicker := time.NewTicker(sourceCycledRSSIntervalFromEnv())
	defer rssTicker.Stop()
	telegramTicker := time.NewTicker(sourceCycledTelegramIntervalFromEnv())
	defer telegramTicker.Stop()
	gdeltTicker := time.NewTicker(sourceCycledGDELTIntervalFromEnv())
	defer gdeltTicker.Stop()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Source Service API shutdown failed: %v", err)
		}
	}()

	// Run the first cycle for every source type immediately.
	log.Println("Initiating first cycle (all source types)...")
	runCycle(ctx, &registry, store, "")

	for {
		select {
		case <-ctx.Done():
			log.Println("Daemon stopped.")
			return
		case <-rssTicker.C:
			log.Println("Initiating scheduled RSS cycle...")
			runCycle(ctx, &registry, store, sources.SourceTypeRSS)
		case <-telegramTicker.C:
			log.Println("Initiating scheduled Telegram cycle...")
			runCycle(ctx, &registry, store, sources.SourceTypeTelegram)
		case <-gdeltTicker.C:
			log.Println("Initiating scheduled GDELT cycle...")
			runCycle(ctx, &registry, store, sources.SourceTypeGDELT)
		}
	}
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

func sourceServiceDBDSN() string {
	if dsn := strings.TrimSpace(os.Getenv("SOURCECYCLED_DOLT_DSN")); dsn != "" {
		return dsn
	}
	if dsn := strings.TrimSpace(os.Getenv("SOURCE_SERVICE_DOLT_DSN")); dsn != "" {
		return dsn
	}
	return "root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true&clientFoundRows=true"
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

func sourceServiceObjectGraphOwnerID() string {
	ownerID := strings.TrimSpace(firstEnv(
		"SOURCE_SERVICE_OBJECTGRAPH_OWNER_ID",
		"SOURCECYCLED_OBJECTGRAPH_OWNER_ID",
	))
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}

func sourceServiceObjectGraphComputerID() string {
	return strings.TrimSpace(firstEnv("SOURCE_SERVICE_OBJECTGRAPH_COMPUTER_ID", "SOURCECYCLED_OBJECTGRAPH_COMPUTER_ID"))
}

func sourceServiceObjectGraphBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(firstEnv(
		"SOURCE_SERVICE_OBJECTGRAPH_BASE_URL",
		"SOURCECYCLED_OBJECTGRAPH_BASE_URL",
	)), "/")
}

func sourceServiceVMCTLURL() string {
	return strings.TrimRight(strings.TrimSpace(firstEnv("SOURCE_SERVICE_VMCTL_URL", "SOURCECYCLED_VMCTL_URL")), "/")
}

func sourceServiceObjectGraphBackfillLimit() int {
	return parsePositiveInt(firstEnv("SOURCE_SERVICE_OBJECTGRAPH_BACKFILL_LIMIT", "SOURCECYCLED_OBJECTGRAPH_BACKFILL_LIMIT"), defaultObjectGraphBackfillLimit)
}

func writeSourceItemsToObjectGraph(ctx context.Context, store cycle.Store, cycleID string, items []sources.Item, now time.Time, eventType, message string) error {
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
	baseURL := sourceServiceObjectGraphBaseURL()
	if baseURL == "" {
		return webCaptureProjectionSummary{}, fmt.Errorf("sourcecycled: canonical objectgraph URL is required (set SOURCE_SERVICE_OBJECTGRAPH_BASE_URL)")
	}
	httpStore := objectgraph.NewHTTPStore(baseURL)
	graph := objectgraph.NewService(objectgraph.Config{
		Memory:  objectgraph.NewMemoryStore(),
		Durable: httpStore,
	})
	defer graph.Close()
	result, err := cycle.WriteWebCaptureGraphObjects(ctx, graph, items, cycle.WebCaptureGraphProjectionConfig{
		OwnerID:    sourceServiceObjectGraphOwnerID(),
		ComputerID: sourceServiceObjectGraphComputerID(),
		Now:        now,
	})
	if err != nil {
		return webCaptureProjectionSummary{}, fmt.Errorf("publish web captures to canonical objectgraph: %w", err)
	}
	return webCaptureProjectionSummary{
		Mode:              "corpusd_api",
		Target:            baseURL,
		CaptureCount:      len(result.Captures),
		SourceEntityCount: len(result.SourceEntities),
		CapturedFromEdges: result.EdgeCount,
		SkippedItemCount:  result.Skipped,
	}, nil
}

func backfillSourceItemsToObjectGraphIfEmpty(ctx context.Context, store cycle.Store, cycleID string, now time.Time) error {
	if sourceServiceObjectGraphBaseURL() == "" || store == nil {
		return nil
	}
	items, err := store.SearchItems(ctx, "", sourceServiceObjectGraphBackfillLimit())
	if err != nil {
		return err
	}
	if len(items) == 0 {
		_ = store.RecordCycleEvent(ctx, cycleID, "", "web_captures_graph_backfill_empty", "no stored source items available for objectgraph backfill", map[string]any{
			"objectgraph_mode": "corpusd_api",
			"backfill_limit":   sourceServiceObjectGraphBackfillLimit(),
		})
		return nil
	}
	summary, err := projectSourceItemsToObjectGraph(ctx, items, now)
	if err != nil {
		return err
	}
	_ = store.RecordCycleEvent(ctx, cycleID, "", "web_captures_graph_backfilled", "stored source items projected to canonical objectgraph web captures", map[string]any{
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

const (
	defaultSourceCycledRSSInterval      = 5 * time.Minute
	defaultSourceCycledTelegramInterval = 5 * time.Minute
	defaultSourceCycledGDELTInterval    = 15 * time.Minute
)

// sourceCycledRSSIntervalFromEnv resolves the RSS poll interval. Accepts
// SOURCECYCLED_RSS_INTERVAL as a Go duration (e.g. "5m", "90s") or
// SOURCECYCLED_RSS_INTERVAL_SECONDS as an integer number of seconds.
func sourceCycledRSSIntervalFromEnv() time.Duration {
	if d := sourceCycledIntervalFromEnv("SOURCECYCLED_RSS_INTERVAL", "SOURCECYCLED_RSS_INTERVAL_SECONDS"); d > 0 {
		return d
	}
	return defaultSourceCycledRSSInterval
}

// sourceCycledTelegramIntervalFromEnv resolves the Telegram poll interval.
func sourceCycledTelegramIntervalFromEnv() time.Duration {
	if d := sourceCycledIntervalFromEnv("SOURCECYCLED_TELEGRAM_INTERVAL", "SOURCECYCLED_TELEGRAM_INTERVAL_SECONDS"); d > 0 {
		return d
	}
	return defaultSourceCycledTelegramInterval
}

// sourceCycledGDELTIntervalFromEnv resolves the GDELT poll interval. Defaults
// to 15 minutes to match the historical universal ticker cadence.
func sourceCycledGDELTIntervalFromEnv() time.Duration {
	if d := sourceCycledIntervalFromEnv("SOURCECYCLED_GDELT_INTERVAL", "SOURCECYCLED_GDELT_INTERVAL_SECONDS"); d > 0 {
		return d
	}
	return defaultSourceCycledGDELTInterval
}

func sourceCycledIntervalFromEnv(durationKey, secondsKey string) time.Duration {
	if raw := strings.TrimSpace(os.Getenv(durationKey)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	seconds := parsePositiveInt(os.Getenv(secondsKey), 0)
	if seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func startSourceServiceAPI(ctx context.Context, store cycle.Store) *http.Server {
	httpServer := &http.Server{
		Addr:              sourceServiceAddr(),
		Handler:           sourceServiceAPIHandler(store),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	go func() {
		log.Printf("Source Service API listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Source Service API stopped with error: %v", err)
			return
		}
		log.Println("Source Service API stopped.")
	}()
	return httpServer
}

func sourceServiceAPIHandler(store cycle.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/source-service/health", handleSourceServiceHealth(store))
	mux.HandleFunc("/internal/source-service/search", handleSourceServiceSearch(store))
	mux.HandleFunc("/internal/source-service/ingestion-handoff/latest", handleSourceServiceIngestionHandoffLatest(store))
	mux.HandleFunc("/internal/source-service/items/", handleSourceServiceItem(store))
	// Top-level liveness and readiness endpoints. Liveness is a cheap
	// process-alive check; readiness reports the source-service ledger as
	// its dependency surface. These are additive and do not alter the
	// existing /internal/source-service/* routes.
	mux.HandleFunc("/health", health.LivenessHandler("sourcecycled"))
	mux.HandleFunc("/health/ready", health.ReadinessHandler("sourcecycled", health.NewAggregator("sourcecycled", 5*time.Second)))
	var guestLookup *vmctl.Client
	if vmctlURL := sourceServiceVMCTLURL(); vmctlURL != "" {
		guestLookup = vmctl.NewClient(vmctlURL)
	}
	return sourceServiceAuthority(server.WithBuildIdentity("sourcecycled", mux), guestLookup)
}

func sourceServiceAuthority(next http.Handler, guestLookup *vmctl.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sourceServiceTrustedInternalTransport(r) {
			next.ServeHTTP(w, r)
			return
		}
		if guestLookup == nil {
			http.Error(w, "guest caller binding unavailable", http.StatusForbidden)
			return
		}
		ownership, err := guestLookup.LookupGuestContext(r.Context(), r.RemoteAddr)
		if err != nil || ownership == nil || !ownership.Found || strings.TrimSpace(ownership.ComputerID) == "" {
			http.Error(w, "guest caller not bound to a live computer", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sourceServiceTrustedInternalTransport(r *http.Request) bool {
	if r == nil {
		return false
	}
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" || remoteAddr == "@" || strings.HasPrefix(remoteAddr, "/") {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "192.0.2.")
}


func handleSourceServiceHealth(store cycle.Store) http.HandlerFunc {
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

func handleSourceServiceSearch(store cycle.Store) http.HandlerFunc {
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

func handleSourceServiceIngestionHandoffLatest(store cycle.Store) http.HandlerFunc {
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
				Topology:      "frozen historical processor/reconciler queue records",
				AuthorityRule: "source and version provenance stay in source items and Texture; queue records are read-only historical residue",
			},
		})
	}
}

func handleSourceServiceItem(store cycle.Store) http.HandlerFunc {
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

func runCycle(ctx context.Context, registry *sources.Registry, store cycle.Store, sourceType sources.SourceType) {
	if engine == nil {
		engine = cycle.NewEngine(registry)
	}

	cycleStartTime := time.Now()
	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		log.Printf("Failed to start durable cycle: %v", err)
		return
	}
	log.Printf("Cycle started at %v (source_type=%q)", cycleStartTime, string(sourceType))
	_ = store.RecordCycleEvent(ctx, cycleID, "", "cycle_started", "source cycle started", map[string]any{"source_type": string(sourceType)})

	// Phase 1 & 2: Source Polling & Deduplication
	pollResult := engine.PollBySourceType(ctx, sourceType)
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


	cycleDuration := time.Since(cycleStartTime)
	_ = store.RecordCycleEvent(ctx, cycleID, "", "cycle_completed", "source cycle completed", map[string]any{"duration_ms": cycleDuration.Milliseconds(), "item_count": len(items), "fetch_count": len(pollResult.Fetches)})
	_ = store.FinishCycle(ctx, cycleID, "completed", len(items), len(pollResult.Fetches), nil)
	log.Printf("Cycle completed in %v", cycleDuration)
}

