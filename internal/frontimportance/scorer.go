// Package frontimportance scores the Autopaper front list's stories for
// importance with Jev and writes the scores to a JSON file corpusd reads
// (internal/platform/publication_front_importance.go). A Node B systemd
// timer runs it every ten minutes as `corpusd front-importance` (a corpusd
// subcommand, so it ships in the corpusd binary with no new package). It is best effort throughout: any
// failure logs and leaves the previous scores file in place, and a missing
// file leaves the front page ranked by recency, as before scores existed.
//
// corpusd has no model access; this host job holds the OpenRouter
// credential (the gateway's provider environment file) and calls Jev
// through the gateway's own transport code (internal/gateway/jev.go).
package frontimportance

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/gateway"
)

const (
	defaultFrontURL   = "http://127.0.0.1:8086/internal/platform/publications/front?limit=100"
	defaultScoresPath = "/var/lib/go-choir/front-importance/scores.json"
	scorerComputerID  = "host:front-importance"
)

// scoresFile is the scores file schema (version 1), shared with corpusd.
type scoresFile struct {
	Version  int                 `json:"version"`
	ScoredAt string              `json:"scored_at"`
	Scores   map[string]scoreRow `json:"scores"`
}

type scoreRow struct {
	Revision   int    `json:"revision"`
	Importance int    `json:"importance"`
	Promo      bool   `json:"promo"`
	ScoredAt   string `json:"scored_at"`
}

// frontStory is the part of a front row the scorer reads.
type frontStory struct {
	PublicationID  string `json:"publication_id"`
	Title          string `json:"title"`
	Dek            string `json:"dek"`
	LatestRevision int    `json:"latest_revision"`
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
}

// Main runs one scoring pass with the subcommand's flags and returns the
// process exit code.
func Main(args []string) int {
	cfg := config{}
	fs := flag.NewFlagSet("front-importance", flag.ContinueOnError)
	fs.StringVar(&cfg.FrontURL, "front-url", envOr("FRONT_IMPORTANCE_FRONT_URL", defaultFrontURL), "corpusd internal front list URL")
	fs.StringVar(&cfg.ScoresPath, "scores", envOr("FRONT_IMPORTANCE_SCORES_PATH", defaultScoresPath), "scores file to write")
	fs.IntVar(&cfg.MaxCalls, "max-calls", 80, "most Jev calls per run")
	fs.DurationVar(&cfg.Budget, "budget", 3*time.Minute, "time budget for the run's Jev calls")
	fs.DurationVar(&cfg.CallTime, "call-timeout", 45*time.Second, "time limit for one Jev call")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		log.Printf("front-importance: OpenRouter credential unavailable; scores left as they are")
		return 0
	}
	if strings.TrimSpace(os.Getenv("GATEWAY_JEV_RECEIPT_PATH")) == "" {
		_ = os.Setenv("GATEWAY_JEV_RECEIPT_PATH", filepath.Join(filepath.Dir(cfg.ScoresPath), "jev-receipts.jsonl"))
	}
	jev := gateway.NewJevTransportFromEnv(true)
	stats, err := run(context.Background(), cfg, http.DefaultClient, jev, time.Now)
	if err != nil {
		log.Printf("front-importance: %v", err)
		return 1
	}
	log.Printf("front-importance: %d stories, %d kept, %d scored, %d failed, %d deferred", stats.Stories, stats.Kept, stats.Scored, stats.Failed, stats.Deferred)
	return 0
}

type runStats struct {
	Stories, Kept, Scored, Failed, Deferred int
}

// run scores the front list's unscored stories and rewrites the scores
// file. It returns an error, writing nothing, when the front list cannot
// be read.
func run(ctx context.Context, cfg config, client *http.Client, jev decider, now func() time.Time) (runStats, error) {
	var stats runStats
	stories, err := fetchFront(ctx, client, cfg.FrontURL)
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
		if had && old.Revision == story.LatestRevision {
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
	next.ScoredAt = now().UTC().Format(time.RFC3339)
	if err := writeScores(cfg.ScoresPath, next); err != nil {
		return stats, err
	}
	return stats, nil
}

func fetchFront(ctx context.Context, client *http.Client, url string) ([]frontStory, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("front list request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("front list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("front list read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("front list returned %s", resp.Status)
	}
	var page struct {
		Stories []frontStory `json:"stories"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("front list decode: %w", err)
	}
	return page.Stories, nil
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
		"instructions": "How important is state.story as news, from 1 (not news) to 10 (major news)? Weigh its consequence for many people, " +
			"public interest, and novelty. Real reported news of public consequence scores high. Promotional or marketing copy, " +
			"a product or feature announcement, or a story whose title is an internal instruction (for example one that starts " +
			"\"Open a story on\") is not news and scores 1 or 2.",
		"criteria": map[string]string{
			"1":  "Not news: an internal instruction or placeholder, or pure marketing.",
			"2":  "Promotional: a product, feature or company announcement with little public consequence.",
			"3":  "Minor news of narrow interest.",
			"4":  "Routine news of modest interest.",
			"5":  "Ordinary news of some public interest.",
			"6":  "Notable news with real consequence for a sizable group.",
			"7":  "Important news with broad public interest.",
			"8":  "Major news with consequence for many people.",
			"9":  "Very major news with wide consequence and high novelty.",
			"10": "Historic news of the highest consequence.",
		},
		"choices": importanceChoices,
	})
	promo, _ := json.Marshal(map[string]any{
		"type":         "choice",
		"instructions": "Is state.story promotional rather than news: marketing copy, a product or feature announcement, or an internal instruction or placeholder (such as a title starting \"Open a story on\")?",
		"criteria": map[string]string{
			"yes": "Promotional, a product announcement, or an internal instruction or placeholder.",
			"no":  "Reported news.",
		},
		"choices": []string{"yes", "no"},
	})
	raw, err := jev.Decide(ctx, scorerComputerID, gateway.JudgmentRequest{
		State:     state,
		Questions: map[string]json.RawMessage{"importance": importance, "promo": promo},
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
	row := scoreRow{Revision: story.LatestRevision, Importance: n}
	switch strings.TrimSpace(decision.Answers["promo"].Choice) {
	case "yes":
		row.Promo = true
	case "no":
	default:
		row.Promo = n <= 2
	}
	return row, nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
