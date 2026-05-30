package cycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"

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

func (e *Engine) PollAll(ctx context.Context) []sources.Item {
	var allItems []sources.Item
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
			
			var items []sources.Item
			var err error

			switch src.Type {
			case sources.SourceTypeRSS:
				items, err = rss.Poll(ctx, src)
			case sources.SourceTypeTelegram:
				items, err = tg.Poll(ctx, src)
			case sources.SourceTypeGDELT:
				items, err = gdelt.Poll(ctx, src)
			default:
				log.Printf("Unknown source type: %s", src.Type)
				return
			}

			if err != nil {
				log.Printf("Error polling source %s: %v", src.ID, err)
				return
			}

			mu.Lock()
			allItems = append(allItems, items...)
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	return e.Dedup(allItems)
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
	h := sha256.New()
	h.Write([]byte(item.SourceID))
	h.Write([]byte(item.OriginalID))
	h.Write([]byte(item.URL))
	return hex.EncodeToString(h.Sum(nil))
}

func (e *Engine) Cluster(items []sources.Item) [][]sources.Item {
	// For the V0, we'll do a very simple clustering by vertical
	// In V1, this would be semantic clustering via embeddings
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
