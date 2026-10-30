package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	// SourceAPI is the gateway's keyed source APIs (choir.SourceAPI); nil
	// when this computer has no gateway.
	SourceAPI search.SourceAPIClient
}

// newSearchWireCorpusTool is choir.SearchWireCorpus: a keyword search of
// this computer's own corpus, its stories (Texture documents, in results)
// and its captures (the shared store's web_capture objects this computer
// wrote, with Jev's labels as topic and outlet hints, in captures).
// Research calls it before the web, so what the paper already holds is
// cited, not fetched again. It is scoped by the run's owner and computer; a
// call with no run context names the owner, and the environment's owner is
// the last fallback. Each list is bounded (corpusMaxLimit).
func newSearchWireCorpusTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		Query   string `json:"query"`
		Limit   int    `json:"limit,omitempty"`
		OwnerID string `json:"owner_id,omitempty"`
	}
	return toolregistry.Tool{Name: "search_wire_corpus",
		Description: "Search this computer's own corpus by topic, title, outlet or keywords: the world events it has resolved (events: event_id, headline, outlets, origins, corroborated, first_seen, last_seen, capture_ids), its stories (results: doc_id, title, snippet) and its captures (captures: capture_id, title, url, outlet, language, topic, fetched_at), strongest and newest matches first. A capture_id is a citable handle. It also lists published Autopaper stories and opinion pieces by any author (published: route_path, title, snippet, kind story|opinion, author, read_url); to cite one, Read its read_url and cite the capture_id that returns, never the snippet. Search here before the web.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"query":    map[string]any{"type": "string", "description": "Search terms: topic, title, entity, outlet, or keywords"},
			"limit":    map[string]any{"type": "integer", "description": "Max results per list (default 10, at most 25)"},
			"owner_id": map[string]any{"type": "string", "description": "Scope to owner when the call carries no run (default: this computer's owner)"},
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
			limit := corpusLimit(in.Limit)
			exec := toolregistry.ExecutionContextFrom(ctx)
			ownerID := firstNonEmptyString(exec.OwnerID, in.OwnerID, universalWirePlatformOwnerID())
			storiesStart := time.Now()
			results, err := deps.Store.SearchDocuments(ctx, query, ownerID, limit)
			storiesElapsed := time.Since(storiesStart)
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
			// A capture-store outage keeps the stories and says so; it never
			// reads as "no captures".
			capturesStart := time.Now()
			captures, note, err := searchCaptures(ctx, deps, ownerID, exec.ComputerID, corpusQueryTerms(query), limit)
			if err != nil {
				captures, note = nil, "captures not searched: "+err.Error()
			}
			logCorpusSearch(query, exec.ComputerID, storiesElapsed, len(out), time.Since(capturesStart), captures, note)
			if captures == nil {
				captures = []CorpusCapture{}
			}
			// Events first: what the paper already knows about an
			// occurrence, from whom, since when (event layer V2). A store
			// failure keeps the rest and says so.
			events, err := searchEvents(ctx, deps.Store, ownerID, exec.ComputerID, corpusQueryTerms(query), limit)
			eventsNote := ""
			if err != nil {
				events, eventsNote = []CorpusEvent{}, "events not searched: "+err.Error()
			}
			// Published Autopaper stories and opinion pieces, from every
			// computer: citable once Read captures their read_url.
			published, publishedNote := searchPublished(ctx, deps, query, limit)
			body := map[string]any{"events": events, "event_count": len(events), "results": out, "count": len(out), "query": query, "captures": captures, "capture_count": len(captures),
				"published": published, "published_count": len(published)}
			if publishedNote != "" {
				body["published_note"] = publishedNote
			}
			if note != "" {
				body["captures_note"] = note
			}
			if eventsNote != "" {
				body["events_note"] = eventsNote
			}
			return toolregistry.ResultJSON(body)
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

// universalWirePlatformOwnerID is search_wire_corpus's last-resort owner,
// used only when the call carries neither a run's owner nor an owner_id:
// this computer's owner (the guest boot's choir.owner_id).
func universalWirePlatformOwnerID() string {
	ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}

// logCorpusSearch writes one journal line per corpus search (rung 0 of
// docs/archive/reports/object-graph-retrieval-research-2026-10-14.md): the query,
// both halves' latency and counts, and the capture ids returned, so the
// retrieval eval set can be grown from real searches and their hits.
func logCorpusSearch(query, computerID string, storiesElapsed time.Duration, stories int, capturesElapsed time.Duration, captures []CorpusCapture, note string) {
	ids := make([]string, 0, len(captures))
	for i, c := range captures {
		if i == 10 {
			break
		}
		ids = append(ids, c.CaptureID)
	}
	if len(query) > 200 {
		query = query[:200]
	}
	log.Printf("corpus search computer=%s stories_ms=%d stories=%d captures_ms=%d captures=%d note=%q query=%q capture_ids=%s",
		computerID, storiesElapsed.Milliseconds(), stories, capturesElapsed.Milliseconds(), len(captures), note, query, strings.Join(ids, ","))
}
