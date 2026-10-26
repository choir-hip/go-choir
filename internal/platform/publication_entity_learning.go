package platform

// The entity lexicon's learning loop and growth read
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice c0 §6
// and §7). A story's entities export carries its review. Corpusd records
// the review once per tape record (a choir.entity_review row), learns the
// stopwords and aliases it teaches into Store B and the in-memory lexicon at
// once, and serves the loop's daily health as a public, cached read.
//
//   - Stopwords: a phrase System 1 judged not an entity at p >= 0.8 in three
//     distinct stories is stopped (a choir.entity_stopword row), and costs no
//     more System 1 questions. An entity's name or alias is never stopped: it
//     is refused at write, and hits are matched before the stoplist applies.
//     An owner act revokes a stopword; its evidence is cleared and it is not
//     relearned for 30 days.
//   - Aliases: a form research's attachment taught is recorded on the entity
//     row, unless it is the entity's own name, another entity's name or
//     alias, or past the bound, and the next request matches it.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/entitytext"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	ogKindEntityReview   = objectgraph.ObjectKind("choir.entity_review")
	ogKindEntityStopword = objectgraph.ObjectKind("choir.entity_stopword")

	// stopwordEvidenceMax bounds the stories kept as one phrase's evidence.
	stopwordEvidenceMax = 8
	// growthCacheTTL is the growth read's cache window.
	growthCacheTTL = 5 * time.Minute
	// growthStoplistMax bounds the stoplist the growth read lists.
	growthStoplistMax = 500
	// growthDefaultDays is the growth read's window when none is asked.
	growthDefaultDays = 30
	// entityGrowthInternalPath is the growth read the proxy forwards
	// types.EntityGrowthPath to.
	entityGrowthInternalPath = "/internal/platform/entities/growth"
)

// entityReviewRow is a choir.entity_review row: one review's counts.
type entityReviewRow struct {
	Story          string                  `json:"story"`
	RecordID       string                  `json:"record_id"`
	ComputerID     string                  `json:"computer_id"`
	RecordedAt     time.Time               `json:"recorded_at"`
	Day            string                  `json:"day"`
	Counts         types.StoryEntityCounts `json:"counts"`
	CategoryGaps   []string                `json:"category_gaps,omitempty"`
	NoveltyYes     float64                 `json:"novelty_yes"`
	AliasesLearned int                     `json:"aliases_learned"`
}

// entityStopwordRow is a choir.entity_stopword row: one phrase's
// not-an-entity evidence and state.
type entityStopwordRow struct {
	Key         string    `json:"key"`
	Phrase      string    `json:"phrase"`
	Stories     []string  `json:"stories"`
	Active      bool      `json:"active"`
	ActivatedAt time.Time `json:"activated_at,omitempty"`
	RevokedAt   time.Time `json:"revoked_at,omitempty"`
}

func (s *Service) lexiconNow() time.Time {
	if s.lexicons.now != nil {
		return s.lexicons.now().UTC()
	}
	return time.Now().UTC()
}

// learnFromReview records a review and learns its stopwords and aliases. It
// runs after the attachments are written. A re-export of the same record
// writes nothing new.
func (s *Service) learnFromReview(ctx context.Context, og *ObjectGraphStore, owner, computer, pub string, review types.StoryEntityReview) error {
	recordID := strings.TrimSpace(review.RecordID)
	reviewID, err := objectgraph.BuildCanonicalID(ogKindEntityReview, owner, objectgraph.StableSuffixFromKey(pub+"\x00"+recordID))
	if err != nil {
		return fmt.Errorf("entity review: %w", err)
	}
	if _, err := og.GetObject(ctx, reviewID); err == nil {
		return nil // this record's review is in already
	} else if notFoundIsNil(err) != nil {
		return fmt.Errorf("entity review: read %s: %w", reviewID, err)
	}
	lex, err := s.entityLexicon(ctx, og, owner)
	if err != nil {
		return fmt.Errorf("entity review: %w", err)
	}
	now := s.lexiconNow()

	learned := 0
	for _, a := range review.Aliases {
		ok, err := s.learnAlias(ctx, og, lex, owner, a, now)
		if err != nil {
			return err
		}
		if ok {
			learned++
		}
	}
	for _, ne := range review.NotEntities {
		if ne.P < types.EntityStopwordMinP {
			continue
		}
		if err := s.learnStopwordEvidence(ctx, og, lex, owner, pub, ne.Phrase, now); err != nil {
			return err
		}
	}

	row := entityReviewRow{
		Story: pub, RecordID: recordID, ComputerID: computer, RecordedAt: now, Day: now.Format("2006-01-02"),
		Counts: review.Counts, CategoryGaps: boundedNames(review.CategoryGaps), NoveltyYes: review.NoveltyYes,
		AliasesLearned: learned,
	}
	obj, err := checkObject(ogKindEntityReview, owner, pub+"\x00"+recordID, row, map[string]any{
		"story": pub, "record_id": recordID, "day": row.Day, "computer_id": computer,
	}, now)
	if err != nil {
		return fmt.Errorf("entity review: %w", err)
	}
	obj.ComputerID = computer
	if _, err := og.InsertObjectIfAbsent(ctx, obj); err != nil {
		return fmt.Errorf("entity review: write: %w", err)
	}
	return nil
}

func boundedNames(names []string) []string {
	var out []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || len(n) > 64 {
			continue
		}
		out = append(out, n)
		if len(out) == 8 {
			break
		}
	}
	return out
}

// learnAlias records one alias on its entity's row and in the lexicon, and
// reports whether it was learned.
func (s *Service) learnAlias(ctx context.Context, og *ObjectGraphStore, lex *entityLexicon, owner string, a types.StoryEntityAlias, now time.Time) (bool, error) {
	id, alias := strings.TrimSpace(a.EntityID), strings.TrimSpace(a.Alias)
	key := entitytext.Key(alias)
	if key == "" || !entitytext.Matchable(alias) || len([]rune(alias)) > types.EntityNameMaxRunes {
		return false, nil
	}
	e, ok := lex.entity(id)
	if !ok || key == entitytext.Key(e.Name) {
		return false, nil
	}
	if len(lex.gaz.Lookup(key)) > 0 {
		return false, nil // another entity's name or alias (G8), or already this one's
	}
	obj, err := og.GetObject(ctx, e.Canonical)
	if err != nil {
		return false, fmt.Errorf("entity alias: read %s: %w", id, err)
	}
	var row entityRow
	if err := json.Unmarshal(obj.Body, &row); err != nil {
		return false, fmt.Errorf("entity alias: decode %s: %w", id, err)
	}
	if len(row.Aliases) >= types.EntityAliasesMax {
		return false, nil
	}
	for _, have := range row.Aliases {
		if entitytext.Key(have) == key {
			return false, nil
		}
	}
	row.Aliases = append(row.Aliases, alias)
	var meta map[string]any
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		return false, fmt.Errorf("entity alias: decode metadata of %s: %w", id, err)
	}
	meta["aliases"] = row.Aliases
	next, err := checkObject(ogKindEntity, owner, id, row, meta, now)
	if err != nil {
		return false, fmt.Errorf("entity alias: %w", err)
	}
	next.ComputerID, next.CreatedAt = obj.ComputerID, obj.CreatedAt
	if err := og.PutObject(ctx, next); err != nil {
		return false, fmt.Errorf("entity alias: write %s: %w", id, err)
	}
	lex.gaz.Add(id, alias)
	return true, nil
}

// learnStopwordEvidence adds one story's not-an-entity judgment of a phrase
// and stops the phrase at the third distinct story, unless it is an entity's
// name or alias, or was revoked within the hold.
func (s *Service) learnStopwordEvidence(ctx context.Context, og *ObjectGraphStore, lex *entityLexicon, owner, pub, phrase string, now time.Time) error {
	key := entitytext.Key(phrase)
	if key == "" || len(lex.gaz.Lookup(key)) > 0 {
		return nil // never an entity's name or alias (G7)
	}
	id, err := objectgraph.BuildCanonicalID(ogKindEntityStopword, owner, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		return fmt.Errorf("entity stopword: %w", err)
	}
	s.lexicons.writeMu.Lock()
	defer s.lexicons.writeMu.Unlock()
	row := entityStopwordRow{Key: key, Phrase: strings.TrimSpace(phrase)}
	created := now
	if obj, err := og.GetObject(ctx, id); err == nil {
		if err := json.Unmarshal(obj.Body, &row); err != nil {
			return fmt.Errorf("entity stopword: decode %s: %w", id, err)
		}
		created = obj.CreatedAt
	} else if notFoundIsNil(err) != nil {
		return fmt.Errorf("entity stopword: read %s: %w", id, err)
	}
	if row.Active {
		return nil
	}
	seen := false
	for _, have := range row.Stories {
		if have == pub {
			seen = true
		}
	}
	if !seen && len(row.Stories) < stopwordEvidenceMax {
		row.Stories = append(row.Stories, pub)
	}
	if seen && len(row.Stories) < types.EntityStopwordStories {
		return nil // nothing new to write
	}
	held := !row.RevokedAt.IsZero() && now.Sub(row.RevokedAt) < types.EntityStopwordRevokeDays*24*time.Hour
	if len(row.Stories) >= types.EntityStopwordStories && !held {
		row.Active, row.ActivatedAt = true, now
	}
	if err := s.putStopword(ctx, og, owner, id, row, created, now); err != nil {
		return err
	}
	if row.Active {
		lex.setStop(row)
	}
	return nil
}

func (s *Service) putStopword(ctx context.Context, og *ObjectGraphStore, owner, id string, row entityStopwordRow, created, now time.Time) error {
	obj, err := checkObject(ogKindEntityStopword, owner, row.Key, row, map[string]any{"key": row.Key, "active": row.Active}, now)
	if err != nil {
		return fmt.Errorf("entity stopword: %w", err)
	}
	obj.CreatedAt = created
	if err := og.PutObject(ctx, obj); err != nil {
		return fmt.Errorf("entity stopword: write %s: %w", id, err)
	}
	return nil
}

// RevokeEntityStopword is the owner act that takes a phrase off owner's
// stoplist: its evidence is cleared, and it is not relearned for
// types.EntityStopwordRevokeDays.
func (s *Service) RevokeEntityStopword(ctx context.Context, owner, phrase string) error {
	owner, key := strings.TrimSpace(owner), entitytext.Key(phrase)
	if owner == "" || key == "" {
		return fmt.Errorf("an owner and a phrase are required")
	}
	og := s.ogStore()
	if og == nil {
		return fmt.Errorf("entity stopword: store unavailable")
	}
	lex, err := s.entityLexicon(ctx, og, owner)
	if err != nil {
		return err
	}
	id, err := objectgraph.BuildCanonicalID(ogKindEntityStopword, owner, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		return fmt.Errorf("entity stopword: %w", err)
	}
	s.lexicons.writeMu.Lock()
	defer s.lexicons.writeMu.Unlock()
	obj, err := og.GetObject(ctx, id)
	if err != nil {
		return fmt.Errorf("entity stopword: %q is not on the stoplist: %w", phrase, err)
	}
	var row entityStopwordRow
	if err := json.Unmarshal(obj.Body, &row); err != nil {
		return fmt.Errorf("entity stopword: decode %s: %w", id, err)
	}
	now := s.lexiconNow()
	row.Active, row.Stories, row.RevokedAt = false, nil, now
	if err := s.putStopword(ctx, og, owner, id, row, obj.CreatedAt, now); err != nil {
		return err
	}
	lex.setStop(row)
	return nil
}

// growthCache is the growth read's five-minute cache, by window. Writes do
// not clear it: the read is a daily series, and a five-minute lag is its
// contract.
type growthCache struct {
	mu      sync.Mutex
	entries map[int]growthEntry
}

type growthEntry struct {
	at   time.Time
	resp types.EntityGrowthResponse
}

// EntityGrowth is the learning loop's daily series over the paper owners'
// reviews, oldest day first, with the lexicons' size and stoplist now.
// Cached for growthCacheTTL; a review or a stopword is seen when the cache
// is next filled.
func (s *Service) EntityGrowth(ctx context.Context, days int) (types.EntityGrowthResponse, error) {
	if days <= 0 {
		days = growthDefaultDays
	}
	if days > types.EntityGrowthMaxDays {
		days = types.EntityGrowthMaxDays
	}
	now := s.lexiconNow()
	s.growth.mu.Lock()
	if e, ok := s.growth.entries[days]; ok && now.Sub(e.at) < growthCacheTTL {
		s.growth.mu.Unlock()
		return e.resp, nil
	}
	s.growth.mu.Unlock()

	resp, err := s.computeEntityGrowth(ctx, days, now)
	if err != nil {
		return types.EntityGrowthResponse{}, err
	}
	s.growth.mu.Lock()
	if s.growth.entries == nil {
		s.growth.entries = map[int]growthEntry{}
	}
	s.growth.entries[days] = growthEntry{at: now, resp: resp}
	s.growth.mu.Unlock()
	return resp, nil
}

func (s *Service) computeEntityGrowth(ctx context.Context, days int, now time.Time) (types.EntityGrowthResponse, error) {
	resp := types.EntityGrowthResponse{Days: make([]types.EntityGrowthDay, days), Stoplist: []string{}}
	index := map[string]int{}
	first := now.Truncate(24*time.Hour).AddDate(0, 0, -(days - 1))
	for i := range resp.Days {
		d := first.AddDate(0, 0, i).Format("2006-01-02")
		resp.Days[i] = types.EntityGrowthDay{Day: d, CategoryGaps: map[string]int{}}
		index[d] = i
	}
	og := s.ogStore()
	if og == nil {
		return resp, fmt.Errorf("entity growth: store unavailable")
	}
	novelty := make([]int, days)
	for _, owner := range s.autopaperOwnerList() {
		after := ""
		for {
			page, err := og.ListObjectsByMetadataPage(ctx, string(ogKindEntityReview), "", "", owner, after, lexiconPageSize)
			if err != nil {
				return resp, fmt.Errorf("entity growth: reviews: %w", err)
			}
			for _, obj := range page {
				var row entityReviewRow
				if obj.Tombstone || json.Unmarshal(obj.Body, &row) != nil {
					continue
				}
				i, ok := index[row.Day]
				if !ok {
					continue
				}
				d := &resp.Days[i]
				d.StoriesReviewed++
				addCounts(&d.Attachments, row.Counts)
				d.ExistingUnmatched += row.Counts.ExistingUnmatched
				d.AliasesLearned += row.AliasesLearned
				d.KeywordsAdded += row.AliasesLearned
				for _, g := range row.CategoryGaps {
					d.CategoryGaps[g]++
				}
				if row.NoveltyYes >= types.EntityNoveltyYesP {
					novelty[i]++
				}
			}
			if len(page) < lexiconPageSize {
				break
			}
			after = page[len(page)-1].CanonicalID
		}
		lex, err := s.entityLexicon(ctx, og, owner)
		if err != nil {
			return resp, err
		}
		size := lex.size()
		resp.Lexicon.Entities += size.Entities
		resp.Lexicon.Keys += size.Keys
		resp.Lexicon.Stopwords += size.Stopwords
		for _, e := range lex.all() {
			i, ok := index[e.FirstSeen.Format("2006-01-02")]
			if !ok {
				continue
			}
			resp.Days[i].Minted++
			resp.Days[i].KeywordsAdded++
			if lex.sprawls(e) {
				resp.Days[i].AliasSprawl++
			}
		}
		for _, st := range lex.stops() {
			if len(resp.Stoplist) < growthStoplistMax {
				resp.Stoplist = append(resp.Stoplist, st.Phrase)
			}
			if i, ok := index[st.ActivatedAt.Format("2006-01-02")]; ok {
				resp.Days[i].StopwordsAdded++
			}
		}
	}
	sort.Strings(resp.Stoplist)
	for i := range resp.Days {
		d := &resp.Days[i]
		total := d.Attachments.Existing + d.Attachments.Extracted + d.Attachments.ResearchNamed
		if total > 0 {
			d.HitRate = float64(d.Attachments.Existing) / float64(total)
			d.ExistingUnmatchedShare = float64(d.ExistingUnmatched) / float64(total)
		}
		if d.Minted > 0 {
			d.AliasSprawlShare = float64(d.AliasSprawl) / float64(d.Minted)
		}
		if d.StoriesReviewed > 0 {
			d.NoveltyYesRate = float64(novelty[i]) / float64(d.StoriesReviewed)
		}
	}
	return resp, nil
}

func addCounts(dst *types.StoryEntityCounts, c types.StoryEntityCounts) {
	dst.Candidates += c.Candidates
	dst.Hits += c.Hits
	dst.Existing += c.Existing
	dst.Extracted += c.Extracted
	dst.ResearchNamed += c.ResearchNamed
	dst.Overridden += c.Overridden
	dst.CategoryGap += c.CategoryGap
	dst.ExistingUnmatched += c.ExistingUnmatched
}

// autopaperOwnerList is the paper owners, sorted; none when unset.
func (s *Service) autopaperOwnerList() []string {
	out := make([]string, 0, len(s.autopaperOwners))
	for o := range s.autopaperOwners {
		out = append(out, o)
	}
	sort.Strings(out)
	return out
}

// HandleInternalEntityGrowth serves the growth read to the proxy.
func (h *Handler) HandleInternalEntityGrowth(w http.ResponseWriter, r *http.Request) {
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	h.HandlePublicEntityGrowth(w, r)
}

// HandlePublicEntityGrowth answers GET ?days=N (1 to 90, default 30).
func (h *Handler) HandlePublicEntityGrowth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	days := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > types.EntityGrowthMaxDays {
			writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("days is 1 to %d", types.EntityGrowthMaxDays)})
			return
		}
		days = n
	}
	resp, err := h.service.EntityGrowth(r.Context(), days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "growth read failed"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, resp)
}
