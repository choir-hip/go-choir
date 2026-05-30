package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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
	runCycle(ctx, &registry)

	for {
		select {
		case <-ctx.Done():
			log.Println("Daemon stopped.")
			return
		case <-ticker.C:
			log.Println("Initiating scheduled cycle...")
			runCycle(ctx, &registry)
		}
	}
}

var engine *cycle.Engine

func runCycle(ctx context.Context, registry *sources.Registry) {
	if engine == nil {
		engine = cycle.NewEngine(registry)
	}

	cycleStartTime := time.Now()
	log.Printf("Cycle started at %v", cycleStartTime)

	// Phase 1 & 2: Source Polling & Deduplication
	items := engine.PollAll(ctx)
	log.Printf("Fetched and deduped %d new items", len(items))

	if len(items) == 0 {
		log.Println("No new items found in this cycle. Skipping synthesis.")
		return
	}

	// Phase 3: Vertical Scoring and Clustering
	clusters := engine.Cluster(items)
	log.Printf("Formed %d clusters based on verticals", len(clusters))

	// Phase 4: LLM Synthesis (The 4,000-word Global Wire)
	log.Println("Phase 4: LLM Synthesis...")
	synth, err := cycle.NewSynthesizer()
	if err != nil {
		log.Printf("Failed to initialize synthesizer: %v", err)
		return
	}

	issueContent, err := synth.Synthesize(ctx, clusters)
	if err != nil {
		log.Printf("Synthesis failed: %v", err)
		return
	}
	log.Printf("Successfully synthesized %d-character issue", len(issueContent))

	// Phase 5: Artifact Storage
	log.Println("Phase 5: Artifact Storage...")
	store, err := cycle.NewStorage("var/sourcecycled.db")
	if err != nil {
		log.Printf("Failed to initialize storage: %v", err)
		return
	}

	if err := store.SaveItems(items); err != nil {
		log.Printf("Failed to save items: %v", err)
	}

	var itemIDs []string
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}

	if err := store.SaveIssue(issueContent, itemIDs, synth.Model, 0); err != nil {
		log.Printf("Failed to save issue: %v", err)
	}

	cycleDuration := time.Since(cycleStartTime)
	log.Printf("Cycle completed in %v", cycleDuration)
	log.Printf("--- LATEST GLOBAL WIRE ISSUE ---\n%s\n--- END ISSUE ---", issueContent)
}
