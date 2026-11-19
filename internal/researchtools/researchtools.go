package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
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
	// OnCaptured, when set, is told of each new capture a research verb wrote
	// (choir.Read, poll_source), with its title, URL and lead, so the runtime
	// can queue it for the event layer. The scheduled poll reports its own.
	// researchRead is true for a page choir.Read fetched. It must not block.
	OnCaptured func(ctx context.Context, ownerID, computerID, sourceID string, captured []sourcegraph.CapturedItem, researchRead bool)
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
		Since   string `json:"since,omitempty"`
		Until   string `json:"until,omitempty"`
		Order   string `json:"order,omitempty"`
	}
	return toolregistry.Tool{Name: "search_wire_corpus",
		Description: "Search this computer's own corpus by topic, title, outlet, keywords or a whole question: the world events it has resolved (events: event_id, headline, outlets, origins, corroborated, first_seen, last_seen, capture_ids) and its captures (captures: capture_id, title, url, outlet, language, topic, fetched_at, and passage: the part of the page's own text that matched), strongest first. A capture's full text is searched, not only its title: a fact that appears only in a page's body is found. The page-text search takes every word you write (stop words such as the, of, was are dropped), a \"quoted phrase\" as a phrase (searched as its words when no page holds it), and -word or -\"a phrase\" as an exclusion (a passage holding it is not returned). Words found in no page, or so common they carry no weight, are not searched, and the result names them (capture_words_dropped, and captures_note). Punctuation splits words: C++ is searched as c and AT&T as t, so put the distinctive word beside them. since and until (a date such as 2026-11-18, also written since:2026-11-18) restrict captures to pages published or, when the publish time is not known, fetched in that window; order recent (or order:recent) lists, newest first, only the pages at least half as relevant as the best match, and the note says when only the best 300 passages were ranked. Captures marked owner-private are never listed. Pages are also found by meaning, not only by their words: a page can be listed for its meaning alone (no passage), or because a published story cites it (reasons: text, meaning, story; via_stories names the story, and stories lists each with the captures it cites). Search by meaning can be unavailable; the note says so and the pages are then those whose text matches. A capture_id is a citable handle. It also lists published Autopaper stories and opinion pieces by any author (published: route_path, title, snippet, kind story|opinion, author, read_url); to cite one, Read its read_url and cite the capture_id that returns, never the snippet. Search here before the web.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"query":    map[string]any{"type": "string", "description": "Search terms: topic, title, entity, outlet, or keywords"},
			"limit":    map[string]any{"type": "integer", "description": "Max results per list (default 10, at most 25)"},
			"since":    map[string]any{"type": "string", "description": "Only captures published (else fetched) on or after this date, e.g. 2026-11-10. The query may also say since:2026-11-10"},
			"until":    map[string]any{"type": "string", "description": "Only captures published (else fetched) before this date, e.g. 2026-11-18. The query may also say until:2026-11-18"},
			"order":    map[string]any{"type": "string", "description": "relevance (default) or recent (newest first). The query may also say order:recent"},
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
			query, window, err := parseCorpusWindow(in.Query, in.Since, in.Until, in.Order)
			if err != nil {
				return "", err
			}
			if query == "" {
				return "", fmt.Errorf("query is required")
			}
			limit := corpusLimit(in.Limit)
			exec := toolregistry.ExecutionContextFrom(ctx)
			ownerID := firstNonEmptyString(exec.OwnerID, in.OwnerID, universalWirePlatformOwnerID())
			// A capture-store outage keeps the rest and says so; it never
			// reads as "no captures".
			capturesStart := time.Now()
			captures, note, info, err := searchCapturesFor(ctx, deps, ownerID, exec.ComputerID,
				corpusRequest{Query: query, Terms: corpusQueryTerms(query), Since: window.Since, Until: window.Until, Order: window.Order}, limit)
			if err != nil {
				captures, note = nil, "captures not searched: "+err.Error()
			}
			capturesElapsed := time.Since(capturesStart)
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
			logCorpusSearch(ctx, query, exec.ComputerID, capturesElapsed, captures, note, len(published), len(events), info.Capped, info)
			body := wireCorpusResult{
				Query:     query,
				Published: published, PublishedCount: len(published), PublishedNote: publishedNote,
				Events: events, EventCount: len(events), EventsNote: eventsNote,
				Captures: captures, CaptureCount: len(captures), CapturesNote: note, CaptureWordsDropped: info.Dropped,
				Stories: info.Stories,
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
	// CaptureWordsDropped are the query words the page-text search did not
	// look for (see CapturesNote).
	CaptureWordsDropped []string `json:"capture_words_dropped,omitempty"`
	CaptureCount        int      `json:"capture_count"`
	CapturesNote        string   `json:"captures_note,omitempty"`
	// Stories are the published stories found by meaning whose cited
	// captures are in captures (each capture's via_stories names them).
	Stories []CorpusStory `json:"stories,omitempty"`
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
func logCorpusSearch(ctx context.Context, query, computerID string, capturesElapsed time.Duration, captures []CorpusCapture, note string, published, events int, capped bool, infos ...corpusSearchInfo) {
	var info corpusSearchInfo
	if len(infos) > 0 {
		info = infos[0]
	}
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
	run, desk, conv := corpusAttribution(ctx)
	// vector= is the rows corpusd's vector half returned, or "unavailable";
	// vector_state= is exact, approximate or partial (or unavailable);
	// bm25_kept= is how many of BM25's top three pages had to be inserted.
	vector, state := "unavailable", "unavailable"
	if info.VectorAnswered {
		vector, state = strconv.Itoa(info.VectorRows), info.VectorState
	}
	log.Printf("corpus search computer=%s captures_ms=%d captures=%d text_index=%t text_indexed=%d computer_pages=%d by_body=%d by_title=%d by_both=%d by_labels_only=%d by_meaning=%d note=%q query=%q capture_ids=%s run=%s desk=%s conv=%s published=%d events=%d capped=%t vector=%s vector_state=%s bm25_kept=%d caller=%s",
		computerID, capturesElapsed.Milliseconds(), len(captures), info.TextUsed, info.TextIndexed, info.Captures, info.Body, info.Title, info.Both, info.LabelsOnly, info.Meaning, note, query, strings.Join(ids, ","), run, desk, conv, published, events, capped, vector, state, info.BM25Kept, CorpusCallerFrom(ctx))
}

// CorpusCallerBrief is the caller of a search the host makes for a research
// run's brief; a search called from a desk's cell names no caller and is
// logged as CorpusCallerCell.
const (
	CorpusCallerBrief = "brief"
	CorpusCallerCell  = "cell"
)

type corpusCallerKey struct{}

// WithCorpusCaller names who calls a corpus search, for the journal line
// (caller=): the brief's searches must be told apart from the desk's own, or a
// count of runs that searched the web before any corpus read would take the
// brief for the desk's first corpus read.
func WithCorpusCaller(ctx context.Context, caller string) context.Context {
	return context.WithValue(ctx, corpusCallerKey{}, caller)
}

// CorpusCallerFrom is the caller a context names, CorpusCallerCell by default.
func CorpusCallerFrom(ctx context.Context) string {
	if c, _ := ctx.Value(corpusCallerKey{}).(string); strings.TrimSpace(c) != "" {
		return strings.TrimSpace(c)
	}
	return CorpusCallerCell
}

// corpusAttribution is the run, desk and conversation of a call, cleaned for
// the log; "-" for a field the call does not carry.
func corpusAttribution(ctx context.Context) (run, desk, conv string) {
	exec := toolregistry.ExecutionContextFrom(ctx)
	a := search.Attribution{RunID: exec.RunID, Desk: firstNonEmptyString(exec.Role, exec.Profile), Conversation: exec.ChannelID}.Clean()
	dash := func(s string) string {
		if s == "" {
			return "-"
		}
		return s
	}
	return dash(a.RunID), dash(a.Desk), dash(a.Conversation)
}

// corpusReadLine is the journal line for one Read of a capture: the run, the
// capture id and held, which says when the capture came to be: "false" for the
// Read that made it, "own" for a capture this run made with an earlier Read,
// "true" for one that existed before this run's first Read of it (see
// runCaptureState). via is the verb.
func corpusReadLine(ctx context.Context, captureID, held, via string) string {
	run, desk, conv := corpusAttribution(ctx)
	return fmt.Sprintf("corpus read run=%s desk=%s conv=%s capture=%s held=%s via=%s", run, desk, conv, captureID, held, via)
}

// runCaptures remembers, per run, the captures that run's own Reads made, so a
// later Read of one is "own", not "held before the run began". It lives in
// this process and is cleared when it holds more than runCapturesMax runs
// (an in-flight run then reads as "true" for its own captures: the line
// overstates held for it, never understates).
var runCaptures struct {
	sync.Mutex
	byRun map[string]map[string]bool
}

const runCapturesMax = 512

func runCaptureState(ctx context.Context, captureID string, made bool) string {
	run := toolregistry.ExecutionContextFrom(ctx).RunID
	if run == "" {
		if made {
			return "false"
		}
		return "true"
	}
	runCaptures.Lock()
	defer runCaptures.Unlock()
	if runCaptures.byRun == nil || len(runCaptures.byRun) > runCapturesMax {
		runCaptures.byRun = map[string]map[string]bool{}
	}
	set := runCaptures.byRun[run]
	if set == nil {
		set = map[string]bool{}
		runCaptures.byRun[run] = set
	}
	switch {
	case made:
		set[captureID] = true
		return "false"
	case set[captureID]:
		return "own"
	}
	return "true"
}
