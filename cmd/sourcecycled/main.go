package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/cycle"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Choir Global Wire sourcecycled daemon (V0)")

	// 1. Load Configuration
	configPath := filepath.Join("configs", "sources.json")
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

	// 3. Main Ingestion Loop (15-minute cycle)
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

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

	// Phase 3: Vertical Scoring and Clustering
	clusters := engine.Cluster(items)
	log.Printf("Formed %d clusters based on verticals", len(clusters))

	// Phase 4: LLM Synthesis (The 4,000-word Global Wire)
	log.Println("Phase 4: LLM Synthesis...")
	synth, err := cycle.NewSynthesizer()
	if err != nil {
		log.Printf("Failed to initialize synthesizer: %v", err)
		_ = store.RecordCycleEvent(ctx, cycleID, "", "synthesis_skipped", "synthesizer unavailable; source ledger persisted", map[string]any{"error": err.Error()})
		_ = store.FinishCycle(ctx, cycleID, "completed_without_synthesis", len(items), len(pollResult.Fetches), nil)
		return
	}

	issueContent, err := synth.Synthesize(ctx, clusters)
	if err != nil {
		log.Printf("Synthesis failed: %v", err)
		_ = store.RecordCycleEvent(ctx, cycleID, "", "synthesis_failed", "source ledger persisted; synthesis failed", map[string]any{"error": err.Error()})
		_ = store.FinishCycle(ctx, cycleID, "completed_without_synthesis", len(items), len(pollResult.Fetches), nil)
		return
	}
	log.Printf("Successfully synthesized %d-character issue", len(issueContent))

	// Phase 5: Artifact Storage
	log.Println("Phase 5: Artifact Storage...")

	var itemIDs []string
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}

	if err := store.SaveIssue(issueContent, itemIDs, synth.Model, 0); err != nil {
		log.Printf("Failed to save issue: %v", err)
		_ = store.FinishCycle(ctx, cycleID, "error", len(items), len(pollResult.Fetches), err)
		return
	}

	cycleDuration := time.Since(cycleStartTime)
	_ = store.RecordCycleEvent(ctx, cycleID, "", "cycle_completed", "source cycle completed", map[string]any{"duration_ms": cycleDuration.Milliseconds(), "item_count": len(items), "fetch_count": len(pollResult.Fetches)})
	_ = store.FinishCycle(ctx, cycleID, "completed", len(items), len(pollResult.Fetches), nil)
	log.Printf("Cycle completed in %v", cycleDuration)
	log.Printf("--- LATEST GLOBAL WIRE ISSUE ---\n%s\n--- END ISSUE ---", issueContent)
}
