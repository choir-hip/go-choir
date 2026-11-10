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
// this computer's own world corpus: its captures (the shared store's
// web_capture objects this computer wrote, with Jev's labels as topic and
// outlet hints), the world events it resolved and published Autopaper pieces.
// Research calls it before the web, so what the paper already holds is cited,
// not fetched again. It never searches the owner's stories: that is the
// Texture custodian's guarded search (newSearchTextureTool), served only to the
// research desk's Texture child (plan MA4.6, section 8.6). It is scoped by the
// run's owner and computer; a call with no run context names the owner, and
// the environment's owner is the last fallback. Each list is bounded
// (corpusMaxLimit).
func newSearchWireCorpusTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		Query   string `json:"query"`
		Limit   int    `json:"limit,omitempty"`
		OwnerID string `json:"owner_id,omitempty"`
	}
	return toolregistry.Tool{Name: "search_wire_corpus",
		Description: "Search this computer's own corpus by topic, title, outlet or keywords: the world events it has resolved (events: event_id, headline, outlets, origins, corroborated, first_seen, last_seen, capture_ids) and its captures (captures: capture_id, title, url, outlet, language, topic, fetched_at), strongest and newest matches first. A capture_id is a citable handle. It also lists published Autopaper stories and opinion pieces by any author (published: route_path, title, snippet, kind story|opinion, author, read_url); to cite one, Read its read_url and cite the capture_id that returns, never the snippet. Search here before the web.",
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
			// A capture-store outage keeps the rest and says so; it never
			// reads as "no captures".
			capturesStart := time.Now()
			captures, note, err := searchCaptures(ctx, deps, ownerID, exec.ComputerID, corpusQueryTerms(query), limit)
			if err != nil {
				captures, note = nil, "captures not searched: "+err.Error()
			}
			logCorpusSearch(query, exec.ComputerID, time.Since(capturesStart), captures, note)
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
			body := wireCorpusResult{
				Query:     query,
				Published: published, PublishedCount: len(published), PublishedNote: publishedNote,
				Events: events, EventCount: len(events), EventsNote: eventsNote,
				Captures: captures, CaptureCount: len(captures), CapturesNote: note,
			}
			if len(published) > 0 {
				body.PublishedFirstNote = publishedFirstNote
			}
			return toolregistry.ResultJSON(body)
		}}
}

// publishedFirstNote leads a corpus answer that holds published pieces. The
// paper's own earlier stories are the first thing research should read; the
// captures listed after them are citable as they are, and in the staging runs
// that listed captures first the research desk skipped the paper's story.
const publishedFirstNote = "published lists the paper's own earlier stories and opinion pieces matching these words. Read the ones that bear on the subject at their read_url before the web, then cite the capture Read returns."

// wireCorpusResult is the search_wire_corpus answer. A struct, not a map: JSON
// keys must come out published first (a map sorts them alphabetically, which
// put captures ahead of the paper's own stories). Field names and meanings are
// unchanged; only the order and published_first_note are new.
type wireCorpusResult struct {
	PublishedFirstNote string           `json:"published_first_note,omitempty"`
	Published          []PublishedPiece `json:"published"`
	PublishedCount     int              `json:"published_count"`
	PublishedNote      string           `json:"published_note,omitempty"`
	Query              string           `json:"query"`
	Events             []CorpusEvent    `json:"events"`
	EventCount         int              `json:"event_count"`
	EventsNote         string           `json:"events_note,omitempty"`
	Captures           []CorpusCapture  `json:"captures"`
	CaptureCount       int              `json:"capture_count"`
	CapturesNote       string           `json:"captures_note,omitempty"`
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
func logCorpusSearch(query, computerID string, capturesElapsed time.Duration, captures []CorpusCapture, note string) {
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
	log.Printf("corpus search computer=%s captures_ms=%d captures=%d note=%q query=%q capture_ids=%s",
		computerID, capturesElapsed.Milliseconds(), len(captures), note, query, strings.Join(ids, ","))
}
