package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

type Dependencies struct {
	Store  *store.Store
	Search search.Client
	// HTTP is the address-guarded client choir.Read fetches with.
	HTTP *http.Client
	// Sources is the poll registry (nil: the embedded default) and
	// Captures is where poll_source and read write: the shared store behind
	// corpusd (Autopaper W1b2), never this computer's embedded graph.
	Sources  *sources.Registry
	Captures CaptureSink
}

func newSearchWireCorpusTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		Query   string `json:"query"`
		Limit   int    `json:"limit,omitempty"`
		OwnerID string `json:"owner_id,omitempty"`
	}
	return toolregistry.Tool{Name: "search_wire_corpus",
		Description: "Search the existing Texture article corpus by topic, title, or keywords. Use before creating new articles to find existing coverage. Returns matching documents with titles, IDs, and snippets. Always search before spawning Texture to avoid duplicate articles.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"query":    map[string]any{"type": "string", "description": "Search terms: topic, title, entity, or keywords"},
			"limit":    map[string]any{"type": "integer", "description": "Max results (default 10)"},
			"owner_id": map[string]any{"type": "string", "description": "Scope to owner (default: platform wire owner)"},
		}, []string{"query"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if deps.Store == nil {
				return "", fmt.Errorf("store not configured")
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode search_wire_corpus args: %w", err)
			}
			query := strings.TrimSpace(in.Query)
			if query == "" {
				return "", fmt.Errorf("query is required")
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 10
			}
			ownerID := strings.TrimSpace(in.OwnerID)
			if ownerID == "" {
				ownerID = universalWirePlatformOwnerID()
			}
			results, err := deps.Store.SearchDocuments(ctx, query, ownerID, limit)
			if err != nil {
				return "", fmt.Errorf("search wire corpus: %w", err)
			}
			type searchResult struct {
				DocID       string `json:"doc_id"`
				Title       string `json:"title"`
				UpdatedAt   string `json:"updated_at"`
				Snippet     string `json:"snippet,omitempty"`
				MatchSource string `json:"match_source"`
			}
			out := make([]searchResult, 0, len(results))
			for _, r := range results {
				out = append(out, searchResult{
					DocID:       r.DocID,
					Title:       r.Title,
					UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
					Snippet:     r.Snippet,
					MatchSource: r.MatchSource,
				})
			}
			return toolregistry.ResultJSON(map[string]any{"results": out, "count": len(out), "query": query})
		}}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// universalWirePlatformOwnerID is the corpus owner search_wire_corpus scopes
// to by default: this computer's owner (the guest boot's choir.owner_id).
func universalWirePlatformOwnerID() string {
	ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}
