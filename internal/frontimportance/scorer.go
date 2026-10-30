// Package frontimportance scores the Autopaper front list's stories for
// importance with Jev and writes the scores to a JSON file corpusd reads
// (internal/platform/publication_front_importance.go). A Node B systemd
// timer runs it every ten minutes as `corpusd front-importance` (a corpusd
// subcommand, so it ships in the corpusd binary with no new package). It is best effort throughout: any
// failure logs and leaves the previous scores file in place, and a missing
// file leaves the front page ranked by recency, as before scores existed.
//
// corpusd has no model access; this host job calls Jev through the gateway's
// host route (internal/gateway/host_judgment.go) with the host-internal
// credential and holds no provider key (SEC-16).
package frontimportance

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/gateway"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

const (
	defaultFrontURL   = "http://127.0.0.1:8086/internal/platform/publications/front?limit=100"
	defaultScoresPath = "/var/lib/go-choir/front-importance/scores.json"
	scorerComputerID  = "host:front-importance"
	// defaultRetention bounds how long a score row outlives its story's
	// time in the scored window (docs/archive/problems/autopaper-section-pages-empty-2026-10-15.md).
	defaultRetention = 14 * 24 * time.Hour
)

// scoresFile is the scores file schema (version 1), shared with corpusd.
type scoresFile struct {
	Version  int                 `json:"version"`
	ScoredAt string              `json:"scored_at"`
	Scores   map[string]scoreRow `json:"scores"`
}

type scoreRow struct {
	Revision   int  `json:"revision"`
	Importance int  `json:"importance"`
	Promo      bool `json:"promo"`
	// Section is the front-page section (one of sectionChoices); absent in
	// rows scored before sections existed, which the next runs rescore.
	Section  string `json:"section,omitempty"`
	ScoredAt string `json:"scored_at"`
	// Criteria is the scoringCriteria version the row was judged under;
	// absent in rows judged before versions existed. A row under older
	// criteria is rescored. corpusd does not read it.
	Criteria int `json:"criteria,omitempty"`
}

// scoringCriteria versions the judgement prompt in scoreStory. Bump it when
// the criteria change so every row is rescored under the new text (within
// the per-run call bound). 2: general editorial criteria for seriousness
// and promotion (docs/archive/problems/front-importance-underrates-major-stories-2026-10-14.md).
const scoringCriteria = 2

// frontStory is the part of a front row the scorer reads.
type frontStory struct {
	PublicationID  string `json:"publication_id"`
	Title          string `json:"title"`
	Dek            string `json:"dek"`
	LatestRevision int    `json:"latest_revision"`
	// SourceCount is the latest version's distinct cited sources; nil when
	// corpusd does not report it.
	SourceCount *int `json:"source_count"`
}

// decider is the Jev transport (gateway.JevTransport in production).
type decider interface {
	Decide(ctx context.Context, computerID string, request gateway.JudgmentRequest) ([]byte, error)
}

type config struct {
	FrontURL   string
	ScoresPath string
	MaxCalls   int
	Budget     time.Duration
	CallTime   time.Duration
	// Retention is how long a score row for a story that has left the
	// scored window is carried forward, measured from its scored_at, so a
	// story keeps its section and importance after it leaves the newest
	// 100. Zero means defaultRetention.
	Retention time.Duration
	// Pages is how many front-list pages to read, following next_cursor;
	// 0 or 1 reads only the first.
	Pages int
}

// Main runs one scoring pass with the subcommand's flags and returns the
// process exit code.
func Main(args []string) int {
	cfg := config{}
	fs := flag.NewFlagSet("front-importance", flag.ContinueOnError)
	fs.StringVar(&cfg.FrontURL, "front-url", envOr("FRONT_IMPORTANCE_FRONT_URL", defaultFrontURL), "corpusd internal front list URL")
	fs.StringVar(&cfg.ScoresPath, "scores", envOr("FRONT_IMPORTANCE_SCORES_PATH", defaultScoresPath), "scores file to write")
	fs.IntVar(&cfg.Pages, "pages", envInt("FRONT_IMPORTANCE_PAGES", 1), "front list pages to read, following next_cursor (backfill)")
	fs.IntVar(&cfg.MaxCalls, "max-calls", 80, "most Jev calls per run")
	fs.DurationVar(&cfg.Budget, "budget", 3*time.Minute, "time budget for the run's Jev calls")
	fs.DurationVar(&cfg.CallTime, "call-timeout", 45*time.Second, "time limit for one Jev call")
	fs.DurationVar(&cfg.Retention, "retention", envDuration("FRONT_IMPORTANCE_RETENTION", defaultRetention), "how long a score row is kept after its story leaves the scored window")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// SEC-16: Jev is reached through the gateway, so this job holds no
	// provider key.
	jev, mode, err := newDecider(os.Getenv, &http.Client{Timeout: cfg.CallTime + 15*time.Second})
	if err != nil {
		log.Printf("front-importance: %v; scores left as they are", err)
		return 0
	}
	log.Printf("front-importance: Jev reached by %s", mode)
	stats, err := run(context.Background(), cfg, http.DefaultClient, jev, time.Now)
	if err != nil {
		log.Printf("front-importance: %v", err)
		return 1
	}
	log.Printf("front-importance: %d stories, %d kept, %d scored, %d failed, %d deferred", stats.Stories, stats.Kept, stats.Scored, stats.Failed, stats.Deferred)
	log.Printf("front-importance: depth: %d important (>= %d, not promo) with known depth, %d cite >= %d sources, median %.1f sources; thin: %v",
		stats.Depth.Important, depthImportantAt, stats.Depth.Deep, depthDeepSources, stats.Depth.MedianSources, stats.Depth.Thin)
	return 0
}

type runStats struct {
	Stories, Kept, Scored, Failed, Deferred int
	Depth                                   depthStats
}

// run scores the front list's unscored stories and rewrites the scores
// file. It returns an error, writing nothing, when the front list cannot
// be read.
func run(ctx context.Context, cfg config, client *http.Client, jev decider, now func() time.Time) (runStats, error) {
	var stats runStats
	stories, err := fetchFrontPages(ctx, client, cfg.FrontURL, cfg.Pages)
	if err != nil {
		return stats, err
	}
	stats.Stories = len(stories)
	previous := readScores(cfg.ScoresPath)
	next := scoresFile{Version: 1, Scores: map[string]scoreRow{}}
	deadline := now().Add(cfg.Budget)
	calls := 0
	for _, story := range stories {
		id := strings.TrimSpace(story.PublicationID)
		if id == "" {
			continue
		}
		old, had := previous.Scores[id]
		if had && old.Revision == story.LatestRevision && old.Section != "" && old.Criteria == scoringCriteria {
			next.Scores[id] = old
			stats.Kept++
			continue
		}
		if calls >= cfg.MaxCalls || !now().Before(deadline) {
			// Out of budget: a newer revision keeps its old score until a
			// later run rescores it.
			if had {
				next.Scores[id] = old
			}
			stats.Deferred++
			continue
		}
		calls++
		callCtx, cancel := context.WithTimeout(ctx, cfg.CallTime)
		row, err := scoreStory(callCtx, jev, story)
		cancel()
		if err != nil {
			log.Printf("front-importance: score %s: %v", id, err)
			stats.Failed++
			if had {
				next.Scores[id] = old
			}
			continue
		}
		row.ScoredAt = now().UTC().Format(time.RFC3339)
		next.Scores[id] = row
		stats.Scored++
	}
	carried := carryForward(previous.Scores, next.Scores, now(), cfg.Retention)
	log.Printf("front-importance: %d rows carried forward for stories outside the window", carried)
	next.ScoredAt = now().UTC().Format(time.RFC3339)
	if err := writeScores(cfg.ScoresPath, next); err != nil {
		return stats, err
	}
	stats.Depth = summarizeDepth(stories, next.Scores)
	return stats, nil
}

// carryForward copies previous rows for stories absent from the current
// window into next, so a story keeps its section after it leaves the newest
// 100. Rows scored longer ago than retention, or with no readable scored_at
// (their age cannot be bounded), are dropped. Rows already in next (the
// window's stories) are never overwritten. It returns the number carried.
func carryForward(previous, next map[string]scoreRow, now time.Time, retention time.Duration) int {
	if retention <= 0 {
		retention = defaultRetention
	}
	carried := 0
	for id, row := range previous {
		if _, inWindow := next[id]; inWindow {
			continue
		}
		scoredAt, err := time.Parse(time.RFC3339, row.ScoredAt)
		if err != nil || now.Sub(scoredAt) > retention {
			continue
		}
		next[id] = row
		carried++
	}
	return carried
}

// fetchFrontPages reads up to pages front-list pages (one when pages < 1),
// following next_cursor by appending &cursor= to the front URL. The first
// page failing is an error; a later page failing logs and keeps the stories
// already read (carry-forward then keeps rows for the unread ones).
func fetchFrontPages(ctx context.Context, client *http.Client, frontURL string, pages int) ([]frontStory, error) {
	stories, cursor, err := fetchFront(ctx, client, frontURL)
	if err != nil {
		return nil, err
	}
	for page := 2; page <= pages && cursor != ""; page++ {
		sep := "?"
		if strings.Contains(frontURL, "?") {
			sep = "&"
		}
		more, next, err := fetchFront(ctx, client, frontURL+sep+"cursor="+url.QueryEscape(cursor))
		if err != nil {
			log.Printf("front-importance: front list page %d: %v; scoring the %d stories read", page, err, len(stories))
			break
		}
		stories = append(stories, more...)
		cursor = next
	}
	return stories, nil
}

func fetchFront(ctx context.Context, client *http.Client, frontURL string) ([]frontStory, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, frontURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("front list request: %w", err)
	}
	hostauth.MarkInternal(req)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("front list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, "", fmt.Errorf("front list read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("front list returned %s", resp.Status)
	}
	var page struct {
		Stories    []frontStory `json:"stories"`
		NextCursor string       `json:"next_cursor"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, "", fmt.Errorf("front list decode: %w", err)
	}
	return page.Stories, page.NextCursor, nil
}

// readScores returns the previous scores, or none when the file is missing
// or unreadable.
func readScores(path string) scoresFile {
	out := scoresFile{Version: 1, Scores: map[string]scoreRow{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var file scoresFile
	if err := json.Unmarshal(raw, &file); err != nil || file.Version != 1 || file.Scores == nil {
		log.Printf("front-importance: previous scores %s unreadable; rescoring", path)
		return out
	}
	return file
}

// writeScores replaces the scores file atomically: a temp file in the same
// directory, synced, then renamed over it.
func writeScores(path string, file scoresFile) error {
	raw, err := json.MarshalIndent(file, "", " ")
	if err != nil {
		return fmt.Errorf("encode scores: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("scores dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".scores-*.json")
	if err != nil {
		return fmt.Errorf("scores temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write scores: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod scores: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync scores: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close scores: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename scores: %w", err)
	}
	return nil
}

var importanceChoices = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}

// sectionChoices are the Autopaper front-page sections (owner, 2026-10-14,
// LF1): world news, U.S. politics, business and finance, tech and AI.
var sectionChoices = []string{"world", "us_politics", "business", "tech_ai"}

// scoreStory asks Jev for one story's importance (1..10) and whether it is
// promotional.
func scoreStory(ctx context.Context, jev decider, story frontStory) (scoreRow, error) {
	state, err := json.Marshal(map[string]any{
		"task": "You are the front-page editor of a general news site. Judge how important this story is for readers.",
		"story": map[string]string{
			"title": clip(story.Title, 300),
			"dek":   clip(story.Dek, 600),
		},
	})
	if err != nil {
		return scoreRow{}, err
	}
	importance, _ := json.Marshal(map[string]any{
		"type": "choice",
		"instructions": "How important is state.story as news, from 1 (not news) to 10 (major news)? Judge as the editor of a serious " +
			"newspaper's front page would. Weigh: consequence for many people; public interest and how widely it will be talked about; " +
			"novelty and surprise; and seriousness - crime, espionage or security breaches, corruption, abuse of power, scandal or " +
			"wrongdoing at a major institution, government or company, deaths, violence, war, disasters and institutional failure " +
			"all raise importance. A criminal charge, arrest, leak or investigation involving a powerful institution is serious news " +
			"even when few people are directly affected. Routine company updates rank low. A company's own announcement, case study, " +
			"customer success story, product or feature launch, advocacy or marketing post is promotional and scores 1 to 3, " +
			"however large the company. A story whose title is an internal instruction (for example one that starts \"Open a story on\" " +
			"or \"Write the story of\") is not news and scores 1.",
		"criteria": map[string]string{
			"1":  "Not news: an internal instruction or placeholder, or pure marketing.",
			"2":  "Promotional: a company's own product, feature, case study or advocacy announcement.",
			"3":  "Minor news of narrow interest, trivia, or a routine company update.",
			"4":  "Routine news of modest interest.",
			"5":  "Ordinary news of some public interest.",
			"6":  "Notable news: real consequence for a sizable group, or a serious incident at a known institution.",
			"7":  "Important news with broad public interest: a major scandal, crime or security breach at a powerful institution, a significant policy or political development.",
			"8":  "Major news with consequence for many people or a story the whole country or industry will talk about.",
			"9":  "Very major news with wide consequence and high novelty.",
			"10": "Historic news of the highest consequence.",
		},
		"choices": importanceChoices,
	})
	promo, _ := json.Marshal(map[string]any{
		"type": "choice",
		"instructions": "Is state.story promotional rather than news? Promotional means it is written from a company's own announcement to " +
			"promote it: marketing copy, a product or feature launch, a customer case study, or an advocacy post; or the title is an internal " +
			"instruction or placeholder (such as one starting \"Open a story on\"). Reporting about conflict, politics, crime or public " +
			"affairs is news even if it mentions technology or the dek is weak.",
		"criteria": map[string]string{
			"yes": "Promotional, a company's own announcement or case study, or an internal instruction or placeholder.",
			"no":  "Reported news, including reporting on war, politics, crime and public affairs.",
		},
		"choices": []string{"yes", "no"},
	})
	section, _ := json.Marshal(map[string]any{
		"type": "choice",
		"instructions": "Which one section of the paper does state.story belong in? Pick exactly one. A story about a technology or AI " +
			"company, product, research or policy is tech_ai, even when it is business news about that company. Markets, companies " +
			"outside technology, trade and the economy are business. The U.S. federal or state government, U.S. elections and parties, " +
			"and U.S. courts ruling on political questions are us_politics. Everything else, including politics, conflict and " +
			"society outside the United States, is world.",
		"criteria": map[string]string{
			"world":       "World news: politics, conflict, disasters, society and culture outside the U.S., and anything not in another section.",
			"us_politics": "U.S. politics: the U.S. government, Congress, the White House, U.S. elections and parties, U.S. courts on political questions.",
			"business":    "Business and finance: markets, companies outside tech, banks, trade, jobs and the economy.",
			"tech_ai":     "Tech and AI: technology and AI companies, products, research, security, and tech or AI policy, including their business news.",
		},
		"choices": sectionChoices,
	})
	raw, err := jev.Decide(ctx, scorerComputerID, gateway.JudgmentRequest{
		State:     state,
		Questions: map[string]json.RawMessage{"importance": importance, "promo": promo, "section": section},
	})
	if err != nil {
		return scoreRow{}, err
	}
	var decision struct {
		Answers map[string]struct {
			Choice string `json:"choice"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(raw, &decision); err != nil {
		return scoreRow{}, fmt.Errorf("decode Jev answer: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(decision.Answers["importance"].Choice))
	if err != nil || n < 1 || n > 10 {
		return scoreRow{}, fmt.Errorf("Jev importance %q is not 1..10", decision.Answers["importance"].Choice)
	}
	row := scoreRow{Revision: story.LatestRevision, Importance: n, Criteria: scoringCriteria}
	switch strings.TrimSpace(decision.Answers["promo"].Choice) {
	case "yes":
		row.Promo = true
	case "no":
	default:
		row.Promo = n <= 2
	}
	// A section outside the four is left unset (the story shows only on the
	// front page) rather than failing the importance score.
	if choice := strings.TrimSpace(decision.Answers["section"].Choice); validSection(choice) {
		row.Section = choice
	}
	return row, nil
}

func validSection(s string) bool {
	for _, c := range sectionChoices {
		if s == c {
			return true
		}
	}
	return false
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
		log.Printf("front-importance: ignoring invalid %s=%q", key, v)
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		log.Printf("front-importance: ignoring invalid %s=%q", key, v)
	}
	return fallback
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
