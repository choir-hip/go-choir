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
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/yusefmosiah/go-choir/internal/jevrecord"
	"github.com/yusefmosiah/go-choir/internal/sectionlist"
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
	// SectionList is the section list the rows' probabilities answer: the
	// questions' hashes, ids, labels and thresholds. Only this job writes
	// it; corpusd applies the thresholds at read time.
	SectionList *sectionlist.Published `json:"section_list,omitempty"`
}

type scoreRow struct {
	Revision   int  `json:"revision"`
	Importance int  `json:"importance"`
	Promo      bool `json:"promo"`
	// Section is the story's primary section under the thresholds in force
	// when the row was written. It is kept for readers that predate
	// SectionP; corpusd recomputes it (and every other membership) from
	// SectionP and the list's thresholds.
	Section  string `json:"section,omitempty"`
	ScoredAt string `json:"scored_at"`
	// Judge is the sectionJudgeVersion SectionP was judged under; a row
	// under another version is asked its section questions again.
	Judge int `json:"judge,omitempty"`
	// SectionP is P(yes) for each section question, keyed by the SHA-256
	// of the normalized question text (sectionlist.Hash). With Revision
	// and Judge it is the cache: (story version, question text, judge
	// version).
	SectionP map[string]float64 `json:"section_p,omitempty"`
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

// sectionJudgeVersion versions the framing around the section questions
// (the instructions wrapper and the yes/no criteria). Bump it to ask every
// section question again.
const sectionJudgeVersion = 1

// A section answered yes by at least sectionBroadShare of at least
// sectionBroadMin stories is logged as too broad (a question that matches
// everything).
const (
	sectionBroadMin   = 20
	sectionBroadShare = 0.95
)

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
	// RequestBytes is the packing bound for one call; zero means
	// requestByteBound.
	RequestBytes int
	Budget       time.Duration
	CallTime     time.Duration
	// Retention is how long a score row for a story that has left the
	// scored window is carried forward, measured from its scored_at, so a
	// story keeps its section and importance after it leaves the newest
	// 100. Zero means defaultRetention.
	Retention time.Duration
	// Pages is how many front-list pages to read, following next_cursor;
	// 0 or 1 reads only the first.
	Pages int
	// Sections is the section list; the zero value is the house list.
	Sections sectionlist.List
}

// Main runs one scoring pass with the subcommand's flags and returns the
// process exit code.
func Main(args []string) int {
	cfg := config{}
	fs := flag.NewFlagSet("front-importance", flag.ContinueOnError)
	fs.StringVar(&cfg.FrontURL, "front-url", envOr("FRONT_IMPORTANCE_FRONT_URL", defaultFrontURL), "corpusd internal front list URL")
	fs.StringVar(&cfg.ScoresPath, "scores", envOr("FRONT_IMPORTANCE_SCORES_PATH", defaultScoresPath), "scores file to write")
	fs.IntVar(&cfg.Pages, "pages", envInt("FRONT_IMPORTANCE_PAGES", 1), "front list pages to read, following next_cursor (backfill)")
	fs.IntVar(&cfg.MaxCalls, "max-calls", 200, "most Jev calls per run")
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
	var shares []string
	for _, c := range stats.SectionCounts {
		shares = append(shares, fmt.Sprintf("%s %d", c.ID, c.Count))
	}
	log.Printf("front-importance: sections (members, a story may be in several): %s; %d placed by top section below threshold; broad: %v", strings.Join(shares, ", "), stats.Fallback, stats.Broad)
	log.Printf("front-importance: depth: %d important (>= %d, not promo) with known depth, %d cite >= %d sources, median %.1f sources; thin: %v",
		stats.Depth.Important, depthImportantAt, stats.Depth.Deep, depthDeepSources, stats.Depth.MedianSources, stats.Depth.Thin)
	return 0
}

// decisionKey carries the id of the score a call feeds to the recorder;
// questionDecisionsKey carries, for a call that packs several stories, the
// decision each question feeds.
type decisionKey struct{}
type questionDecisionsKey struct{}

// recordingDecider records each call of the wrapped decider.
type recordingDecider struct {
	inner     decider
	rec       *jevrecord.Recorder
	threshold string
}

func (d recordingDecider) Decide(ctx context.Context, computerID string, request gateway.JudgmentRequest) ([]byte, error) {
	id, _ := ctx.Value(decisionKey{}).(string)
	byQuestion, _ := ctx.Value(questionDecisionsKey{}).(map[string]string)
	meta := jevrecord.Meta{Site: "front_importance", DecisionID: id, Threshold: d.threshold, QuestionDecisions: byQuestion}
	raw, err := jevrecord.Observe(d.rec, meta, request.State, request.Questions, func() (json.RawMessage, error) {
		return d.inner.Decide(ctx, computerID, request)
	})
	return raw, err
}

// thresholdNote is the threshold in force when an answer is read, for the
// calibration record: the criteria version and each section's threshold.
func thresholdNote(list sectionlist.List) string {
	var b strings.Builder
	fmt.Fprintf(&b, "criteria=%d judge=%d list=%s@%d", scoringCriteria, sectionJudgeVersion, list.ID, list.Version)
	for _, s := range list.Sections {
		fmt.Fprintf(&b, " %s>=%.2f", s.ID, s.Threshold)
	}
	return b.String()
}

// errScoreDeferred marks a story the run's time budget ran out before.
var errScoreDeferred = errors.New("front-importance: time budget spent")

type runStats struct {
	Stories, Kept, Scored, Failed, Deferred int
	Depth                                   depthStats
	// SectionCounts is, per section in list order, how many stories in the
	// window are members; Fallback counts the stories placed by their top
	// section because none reached its threshold; Broad names sections
	// answered yes for nearly every story.
	SectionCounts []sectionCount
	Fallback      int
	Broad         []string
}

type sectionCount struct {
	ID    string
	Count int
}

// storyAsk is what one story needs asked: the importance and promo
// questions (new revision or older criteria) and the section questions
// whose answers are not cached.
type storyAsk struct {
	story      frontStory
	importance bool
	sections   []sectionlist.Section
}

// questions is how many questions the story's calls carry.
func (a storyAsk) questions() int {
	n := len(a.sections)
	if a.importance {
		n += 2
	}
	return n
}

// requestByteBound is the packing target for one call: the questions of as
// many stories as fit under it go in one request. It sits under Jev's
// 32,000-byte long-context line (docs/design/jev-batched-questions-2026-11-10.md,
// section 1). The host door's own cap is 1 MiB and it caps no question count.
const requestByteBound = 30000

// plan decides what a story needs given its previous row. The cache key is
// (story version, question text hash, judge version): a row at another
// revision or judge version hits for nothing; otherwise each section whose
// hash is stored is a hit.
func plan(story frontStory, old scoreRow, had bool, list sectionlist.List) storyAsk {
	a := storyAsk{story: story}
	a.importance = !had || old.Revision != story.LatestRevision || old.Criteria != scoringCriteria
	live := had && old.Revision == story.LatestRevision && old.Judge == sectionJudgeVersion
	for _, s := range list.Sections {
		if live {
			if _, ok := old.SectionP[sectionlist.Hash(s.Question)]; ok {
				continue
			}
		}
		a.sections = append(a.sections, s)
	}
	return a
}

// run scores the front list's stories and rewrites the scores file. It
// returns an error, writing nothing, when the front list cannot be read.
func run(ctx context.Context, cfg config, client *http.Client, jev decider, now func() time.Time) (runStats, error) {
	var stats runStats
	list := cfg.Sections
	if len(list.Sections) == 0 {
		list = sectionlist.House()
	}
	if err := list.Validate(); err != nil {
		return stats, fmt.Errorf("section list: %w", err)
	}
	pub := list.Published()
	stories, err := fetchFrontPages(ctx, client, cfg.FrontURL, cfg.Pages)
	if err != nil {
		return stats, err
	}
	stats.Stories = len(stories)
	previous := readScores(cfg.ScoresPath)
	next := scoresFile{Version: 1, Scores: map[string]scoreRow{}, SectionList: &pub}
	deadline := now().Add(cfg.Budget)
	// Every call is recorded for calibration, failures included, beside the
	// scores file (internal/jevrecord).
	recorder := jevrecord.NewRecorder(filepath.Join(filepath.Dir(cfg.ScoresPath), "jev-calls.jsonl"))
	defer recorder.Close()
	jev = recordingDecider{inner: jev, rec: recorder, threshold: thresholdNote(list)}
	// Plan first: which stories keep their row, which are asked (and which
	// of their questions), which wait for a later run. Stories are planned
	// in front-list order, so the visible page is asked first and a bounded
	// run defers the archive. Then pack them into calls, one after another;
	// every answer lands in its story's slot.
	var asked []storyAsk
	for _, story := range stories {
		id := strings.TrimSpace(story.PublicationID)
		if id == "" {
			continue
		}
		old, had := previous.Scores[id]
		ask := plan(story, old, had, list)
		if ask.questions() == 0 {
			next.Scores[id] = old
			stats.Kept++
			continue
		}
		asked = append(asked, ask)
	}
	// Pack the asked stories into calls, in front-list order, and issue them
	// one after another: Jev is never called concurrently, one call carries
	// many questions. Batches beyond MaxCalls are deferred.
	bound := cfg.RequestBytes
	if bound <= 0 {
		bound = requestByteBound
	}
	batches := packStories(asked, bound)
	if len(batches) > cfg.MaxCalls {
		for _, b := range batches[cfg.MaxCalls:] {
			for _, i := range b {
				id := strings.TrimSpace(asked[i].story.PublicationID)
				if old, had := previous.Scores[id]; had {
					// Out of budget: a newer revision keeps its old score
					// until a later run rescores it.
					next.Scores[id] = old
				}
				stats.Deferred++
			}
		}
		batches = batches[:cfg.MaxCalls]
	}
	results := make([]storyResult, len(asked))
	spent := map[int]bool{}
	for _, b := range batches {
		for _, i := range b {
			spent[i] = true
		}
		if !now().Before(deadline) {
			for _, i := range b {
				results[i] = storyResult{err: errScoreDeferred}
			}
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, cfg.CallTime)
		rows, errs := askBatch(callCtx, jev, asked, b, previous.Scores)
		cancel()
		for k, i := range b {
			results[i] = storyResult{value: rows[k], err: errs[k]}
		}
	}
	for i, ask := range asked {
		if !spent[i] {
			continue // deferred above with its old row
		}
		id := strings.TrimSpace(ask.story.PublicationID)
		old, had := previous.Scores[id]
		row, err := results[i].value, results[i].err
		if err != nil {
			if row.Revision == ask.story.LatestRevision && row.Importance >= 1 {
				// Part of the story's questions were answered before the
				// failure: keep that progress, it is cached.
				row.ScoredAt = now().UTC().Format(time.RFC3339)
				next.Scores[id] = row
			} else if had {
				next.Scores[id] = old
			}
			if errors.Is(err, errScoreDeferred) {
				stats.Deferred++
				continue
			}
			log.Printf("front-importance: score %s: %v", id, err)
			stats.Failed++
			continue
		}
		row.ScoredAt = now().UTC().Format(time.RFC3339)
		next.Scores[id] = row
		stats.Scored++
	}
	carried := carryForward(previous.Scores, next.Scores, now(), cfg.Retention)
	log.Printf("front-importance: %d rows carried forward for stories outside the window", carried)
	// Rows in the window carry only the current list's answers and the
	// primary section under today's thresholds.
	keep := map[string]bool{}
	for _, e := range pub.Entries {
		keep[e.Hash] = true
	}
	for id, row := range next.Scores {
		if len(row.SectionP) == 0 {
			continue
		}
		pruned := make(map[string]float64, len(row.SectionP))
		for h, v := range row.SectionP {
			if keep[h] {
				pruned[h] = v
			}
		}
		row.SectionP = pruned
		row.Section = pub.Place(pruned).Primary
		next.Scores[id] = row
	}
	next.ScoredAt = now().UTC().Format(time.RFC3339)
	if err := writeScores(cfg.ScoresPath, next); err != nil {
		return stats, err
	}
	stats.Depth = summarizeDepth(stories, next.Scores)
	stats.SectionCounts, stats.Fallback, stats.Broad = sectionCoverage(pub, stories, next.Scores)
	for _, id := range stats.Broad {
		log.Printf("front-importance: section %s is answered yes for %.0f%% or more of the stories; its question matches almost everything", id, sectionBroadShare*100)
	}
	return stats, nil
}

// sectionCoverage counts, over the window's stories that have section
// answers, each section's members and the fallback placements, and names
// the sections nearly every such story is a member of by its own answer.
func sectionCoverage(pub sectionlist.Published, stories []frontStory, rows map[string]scoreRow) (counts []sectionCount, fallback int, broad []string) {
	members, yes := map[string]int{}, map[string]int{}
	placed := 0
	for _, st := range stories {
		row, ok := rows[strings.TrimSpace(st.PublicationID)]
		if !ok || len(row.SectionP) == 0 {
			continue
		}
		p := pub.Place(row.SectionP)
		if len(p.Sections) == 0 {
			continue
		}
		placed++
		if p.Fallback {
			fallback++
		}
		for _, id := range p.Sections {
			members[id]++
		}
		for _, e := range pub.Entries {
			if v, ok := row.SectionP[e.Hash]; ok && v >= e.Threshold {
				yes[e.ID]++
			}
		}
	}
	for _, e := range pub.Entries {
		counts = append(counts, sectionCount{ID: e.ID, Count: members[e.ID]})
		if placed >= sectionBroadMin && float64(yes[e.ID]) >= sectionBroadShare*float64(placed) {
			broad = append(broad, e.ID)
		}
	}
	return counts, fallback, broad
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

// sectionQuestionName names a section question on the wire: the hash is the
// question's identity, so a renamed section is the same wire question.
func sectionQuestionName(s sectionlist.Section) string {
	return "sec_" + sectionlist.Hash(s.Question)[:16]
}

// sectionQuestion is one section as a yes/no question about state.story.
func sectionQuestion(s sectionlist.Section) json.RawMessage {
	q, _ := json.Marshal(map[string]any{
		"type": "choice",
		"instructions": "state.story is a news story. " + sectionlist.Normalize(s.Question) +
			" Answer about the story itself, not about how it is told. A story can belong to several sections; do not hold back because another section also fits.",
		"criteria": map[string]string{
			"yes": "The story belongs in this section.",
			"no":  "The story does not belong in this section.",
		},
		"choices": []string{"yes", "no"},
	})
	return q
}

func importanceQuestions() (importance, promo json.RawMessage) {
	importance, _ = json.Marshal(map[string]any{
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
	promo, _ = json.Marshal(map[string]any{
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
	return importance, promo
}

// answer is one question's answer on the wire.
type answer struct {
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// storyResult is one story's outcome from the run's calls.
type storyResult struct {
	value scoreRow
	err   error
}

// packStories groups the asked stories, in order, into batches of indexes
// whose requests each stay under bound bytes. A story larger than bound goes
// alone; a story is never split.
func packStories(asked []storyAsk, bound int) [][]int {
	var batches [][]int
	var cur []int
	size := 0
	for i, ask := range asked {
		n := storyBytes(ask)
		if len(cur) > 0 && size+n > bound {
			batches, cur, size = append(batches, cur), nil, 0
		}
		cur, size = append(cur, i), size+n
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}

// storyBytes is the request bytes one story adds to a call: its state entry
// and its questions, sized under a worst-case handle.
func storyBytes(ask storyAsk) int {
	n := 0
	for name, q := range storyQuestions(ask, "s000") {
		n += len(name) + len(q)
	}
	title, dek := clip(ask.story.Title, 300), clip(ask.story.Dek, 600)
	return n + len(title) + len(dek) + 40
}

// storyQuestions is the questions one story needs, every name and every
// reference to the story carrying its handle: `<handle>_importance`,
// `<handle>_promo`, `<handle>_sec_<hash>`, about state.stories.<handle>.
func storyQuestions(ask storyAsk, handle string) map[string]json.RawMessage {
	questions := map[string]json.RawMessage{}
	if ask.importance {
		imp, promo := importanceQuestions()
		questions[handle+"_importance"], questions[handle+"_promo"] = forHandle(imp, handle), forHandle(promo, handle)
	}
	for _, s := range ask.sections {
		questions[handle+"_"+sectionQuestionName(s)] = forHandle(sectionQuestion(s), handle)
	}
	return questions
}

// forHandle points a question's "state.story" at its story's handle.
func forHandle(q json.RawMessage, handle string) json.RawMessage {
	return bytes.ReplaceAll(q, []byte("state.story"), []byte("state.stories."+handle))
}

// baseRow is the row a story's answers merge into: its previous row when at
// the same revision (cached answers stay), else an empty one.
func baseRow(ask storyAsk, old scoreRow) scoreRow {
	row := scoreRow{Revision: ask.story.LatestRevision, Criteria: scoringCriteria, Judge: sectionJudgeVersion, SectionP: map[string]float64{}}
	if old.Revision == ask.story.LatestRevision {
		row = old
		row.Judge = sectionJudgeVersion
		row.SectionP = map[string]float64{}
		if old.Judge == sectionJudgeVersion {
			for h, v := range old.SectionP {
				row.SectionP[h] = v
			}
		}
	}
	return row
}

// askBatch asks Jev, in one call, the questions of the stories asked[idx...],
// handles s1, s2, ... by position. It returns each story's row and error in
// idx order. A failed call fails exactly the stories in it; a story whose
// answers are missing or unusable fails alone.
func askBatch(ctx context.Context, jev decider, asked []storyAsk, idx []int, previous map[string]scoreRow) ([]scoreRow, []error) {
	rows := make([]scoreRow, len(idx))
	errs := make([]error, len(idx))
	stories := map[string]map[string]string{}
	questions := map[string]json.RawMessage{}
	byQuestion := map[string]string{}
	handles := make([]string, len(idx))
	for k, i := range idx {
		ask := asked[i]
		id := strings.TrimSpace(ask.story.PublicationID)
		handles[k] = fmt.Sprintf("s%d", k+1)
		rows[k] = baseRow(ask, previous[id])
		stories[handles[k]] = map[string]string{"title": clip(ask.story.Title, 300), "dek": clip(ask.story.Dek, 600)}
		for name, q := range storyQuestions(ask, handles[k]) {
			questions[name] = q
			byQuestion[name] = fmt.Sprintf("%s@%d", id, ask.story.LatestRevision)
		}
	}
	failAll := func(err error) ([]scoreRow, []error) {
		for k, i := range idx {
			rows[k], errs[k] = failedRow(rows[k], asked[i].importance), err
		}
		return rows, errs
	}
	state, err := json.Marshal(map[string]any{
		"task":    "You are the front-page editor of a general news site. Judge how important each story in state.stories is for readers, and which sections of the paper it belongs in. Each question names its story.",
		"stories": stories,
	})
	if err != nil {
		return failAll(err)
	}
	first := asked[idx[0]].story
	ctx = context.WithValue(ctx, decisionKey{}, fmt.Sprintf("%s@%d", strings.TrimSpace(first.PublicationID), first.LatestRevision))
	ctx = context.WithValue(ctx, questionDecisionsKey{}, byQuestion)
	raw, err := jev.Decide(ctx, scorerComputerID, gateway.JudgmentRequest{State: state, Questions: questions})
	if err != nil {
		return failAll(err)
	}
	var decision struct {
		Answers map[string]answer `json:"answers"`
	}
	if err := json.Unmarshal(raw, &decision); err != nil {
		return failAll(fmt.Errorf("decode Jev answer: %w", err))
	}
	for k, i := range idx {
		ask, h := asked[i], handles[k]
		row := rows[k]
		if ask.importance {
			imp := decision.Answers[h+"_importance"]
			n, err := strconv.Atoi(strings.TrimSpace(imp.Choice))
			if err != nil || n < 1 || n > 10 {
				rows[k], errs[k] = failedRow(row, true), fmt.Errorf("Jev importance %q is not 1..10", imp.Choice)
				continue
			}
			row.Importance, row.Criteria = n, scoringCriteria
			switch strings.TrimSpace(decision.Answers[h+"_promo"].Choice) {
			case "yes":
				row.Promo = true
			case "no":
				row.Promo = false
			default:
				row.Promo = n <= 2
			}
		}
		for _, s := range ask.sections {
			// A section question that comes back unusable is left unasked;
			// it is asked again next run and fails nothing else.
			if p, ok := probabilityYes(decision.Answers[h+"_"+sectionQuestionName(s)]); ok {
				row.SectionP[sectionlist.Hash(s.Question)] = p
			}
		}
		rows[k] = row
	}
	return rows, errs
}

// failedRow is the row after a failed call: a revision's importance that was
// never answered leaves it unusable (zero revision), so the caller keeps the
// previous row instead.
func failedRow(row scoreRow, importancePending bool) scoreRow {
	if importancePending && row.Importance == 0 {
		return scoreRow{}
	}
	return row
}

// probabilityYes reads a yes/no answer as P(yes), refusing anything that is
// not a probability. Without a distribution the choice stands as 1 or 0.
func probabilityYes(a answer) (float64, bool) {
	if p, ok := a.Probabilities["yes"]; ok {
		if p != p || p < 0 || p > 1 {
			return 0, false
		}
		return p, true
	}
	switch strings.TrimSpace(a.Choice) {
	case "yes":
		return 1, true
	case "no":
		return 0, true
	}
	return 0, false
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
