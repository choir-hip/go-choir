package platform

import (
	"context"
	"math"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
)

// The floor (docs/problems/most-new-stories-show-no-related-stories-2026-10-27.md).
// The strong signals (a shared source, a judged pair, a shared central
// entity) are never padded, so most new stories had none. The owner's rule:
// "There should always be 3+ related stories. Just a question of how
// related they are." The floor is a separate step after rankRelated: when
// the strong list is shorter than MinLinks it appends weaker links, each
// after every strong one and labelled with its own reason, from a pool of
// the paper's recent stories: stories that share the story's title and dek
// terms first (idf-weighted over the pool), then the newest.

// relatedFloorEnv is the floor's rollback switch: CORPUSD_RELATED_FLOOR=off
// answers the strong links only, as before the floor. Read per request.
const relatedFloorEnv = "CORPUSD_RELATED_FLOOR"

// relatedModeFloorSuffix marks a mode whose answers carry the floor, so the
// cache (keyed on the mode) never serves an answer made with it off.
const relatedModeFloorSuffix = "+floor"

func relatedFloorOn(t relatedTuning) bool {
	return t.MinLinks > 0 && !strings.EqualFold(strings.TrimSpace(os.Getenv(relatedFloorEnv)), "off")
}

// Weak link reasons.
const (
	relatedReasonSimilar = "similar" // title and dek terms both stories share
	relatedReasonRecent  = "recent"  // nothing shared; one of the newest stories
)

// relatedFloorStopwords are words that carry no subject.
var relatedFloorStopwords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`the and for are but not you all any can had her was one our out has him his how its may new now old see two way who did get let put say she too use
		from with that this have been were they them then than there their what when where which while will would could should about after again also amid among because before being between both down during each even every into just like more most much near only other over same some such under until upon very want what whom whose why your
		says said year years week weeks day days today amid says new news report reports`) {
		m[w] = true
	}
	return m
}()

// relatedFloorTerms are the distinct subject words of a story's title and
// dek: lower case, letters and digits, three or more runes, no stopwords.
func relatedFloorTerms(s FrontStory) map[string]bool {
	out := map[string]bool{}
	words := strings.FieldsFunc(strings.ToLower(s.Title+" "+s.Dek), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
	})
	for _, w := range words {
		w = strings.TrimSuffix(strings.Trim(w, "'"), "'s")
		if len([]rune(w)) < 3 || relatedFloorStopwords[w] {
			continue
		}
		// A plural is its singular ("votes", "vote"); "ss" is not a plural.
		if len([]rune(w)) > 4 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
			w = strings.TrimSuffix(w, "s")
		}
		out[w] = true
	}
	return out
}

// floorRelated returns have, then weak links from pool until the list holds
// min links or the pool has no more. have is never reordered or cut. Pure.
func floorRelated(self FrontStory, have []RelatedLink, pool []FrontStory, now time.Time, t relatedTuning, min int) []RelatedLink {
	if len(have) >= min {
		return have
	}
	taken := map[string]bool{self.PublicationID: true}
	for _, l := range have {
		taken[l.PublicationID] = true
	}
	type weak struct {
		story FrontStory
		score float64
		terms int
	}
	var cands []weak
	termsOf := make([]map[string]bool, len(pool))
	df := map[string]int{}
	for i, s := range pool {
		termsOf[i] = relatedFloorTerms(s)
		for w := range termsOf[i] {
			df[w]++
		}
	}
	mine := relatedFloorTerms(self)
	n := float64(len(pool))
	for i, s := range pool {
		if taken[s.PublicationID] || s.PublicationID == "" || s.Kind != "" {
			continue
		}
		taken[s.PublicationID] = true
		c := weak{story: s}
		for w := range termsOf[i] {
			if mine[w] {
				c.terms++
				c.score += math.Log(1 + n/float64(1+df[w]))
			}
		}
		if c.terms > 0 {
			if self.Section != "" && s.Section == self.Section {
				c.score *= t.SameSection
			}
			c.score *= recencyFactor(now.Sub(s.UpdatedAt).Hours()/24, t)
		}
		cands = append(cands, c)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if (a.terms > 0) != (b.terms > 0) {
			return a.terms > 0
		}
		if a.score != b.score {
			return a.score > b.score
		}
		if !a.story.UpdatedAt.Equal(b.story.UpdatedAt) {
			return a.story.UpdatedAt.After(b.story.UpdatedAt)
		}
		return a.story.PublicationID < b.story.PublicationID
	})
	out := append([]RelatedLink(nil), have...)
	for _, c := range cands {
		if len(out) >= min {
			break
		}
		reason := relatedReasonRecent
		if c.terms > 0 {
			reason = relatedReasonSimilar
		}
		out = append(out, RelatedLink{FrontStory: c.story, Reason: reason, Shared: RelatedShared{Terms: c.terms}})
	}
	return out
}

// relatedFloorPool is the paper's newest stories, read at most once per
// PoolTTL and never on the request's budget: a cold read takes longer than
// the whole walk may. Concurrent requests on a stale pool wait for one read.
func (s *Service) relatedFloorPool(ctx context.Context, t relatedTuning) []FrontStory {
	c := &s.related
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	now := c.clock()
	c.mu.Lock()
	pool, at := c.pool, c.poolAt
	c.mu.Unlock()
	if pool != nil && now.Sub(at) < t.PoolTTL {
		return pool
	}
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), t.PoolReadTimeout)
	defer cancel()
	page, err := s.ListFrontStories(readCtx, t.PoolSize, "")
	if err != nil || page == nil {
		return pool // the last good pool, or none
	}
	c.mu.Lock()
	c.pool, c.poolAt = page.Stories, now
	c.mu.Unlock()
	return page.Stories
}
