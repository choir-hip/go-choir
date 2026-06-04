package cycle

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
)

type Engine struct {
	Registry *sources.Registry
	Seen     map[string]bool
	Mu       sync.RWMutex
}

func NewEngine(registry *sources.Registry) *Engine {
	return &Engine{
		Registry: registry,
		Seen:     make(map[string]bool),
	}
}

type PollAllResult struct {
	Items   []sources.Item
	Fetches []sources.FetchRecord
}

func (e *Engine) PollAll(ctx context.Context) PollAllResult {
	var allItems []sources.Item
	var fetches []sources.FetchRecord
	var mu sync.Mutex
	var wg sync.WaitGroup

	rss := sources.NewRSSPoller(e.Registry.UserAgent)
	tg := sources.NewTelegramScraper(e.Registry.UserAgent)
	gdelt := sources.NewGDELTFetcher(e.Registry.UserAgent)

	for i := range e.Registry.Sources {
		s := &e.Registry.Sources[i]
		wg.Add(1)
		go func(src *sources.Source) {
			defer wg.Done()

			var result sources.PollResult
			var err error

			switch src.Type {
			case sources.SourceTypeRSS:
				result, err = rss.Poll(ctx, src)
			case sources.SourceTypeTelegram:
				result, err = tg.Poll(ctx, src)
			case sources.SourceTypeGDELT:
				result, err = gdelt.Poll(ctx, src)
			default:
				log.Printf("Unknown source type: %s", src.Type)
				started := time.Now().UTC()
				fetch := sources.NewFetchRecord(*src, src.URL, started)
				fetch.Status = "unsupported_source_type"
				fetch.EndedAt = time.Now().UTC()
				fetch.ErrorClass = "unsupported_source_type"
				fetch.Error = "unsupported source type"
				mu.Lock()
				fetches = append(fetches, fetch)
				mu.Unlock()
				return
			}

			if err != nil {
				log.Printf("Error polling source %s: %v", src.ID, err)
			}

			mu.Lock()
			if result.Fetch.FetchID != "" {
				fetches = append(fetches, result.Fetch)
			}
			allItems = append(allItems, result.Items...)
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	return PollAllResult{
		Items:   e.Dedup(allItems),
		Fetches: fetches,
	}
}

func (e *Engine) Dedup(items []sources.Item) []sources.Item {
	e.Mu.Lock()
	defer e.Mu.Unlock()

	var unique []sources.Item
	for _, item := range items {
		hash := e.hashItem(item)
		if !e.Seen[hash] {
			e.Seen[hash] = true
			unique = append(unique, item)
		}
	}
	return unique
}

func (e *Engine) hashItem(item sources.Item) string {
	if item.ID != "" {
		return item.ID
	}
	return sources.ContentHash(item.SourceID, item.OriginalID, item.URL)
}

func (e *Engine) Cluster(items []sources.Item) [][]sources.Item {
	// P0 keeps clustering intentionally shallow; provenance is carried by item
	// IDs and manifests rather than by this editorial grouping.
	clusters := make(map[string][]sources.Item)
	for _, item := range items {
		for _, v := range item.Verticals {
			clusters[v] = append(clusters[v], item)
		}
	}

	var result [][]sources.Item
	for _, c := range clusters {
		result = append(result, c)
	}
	return result
}
