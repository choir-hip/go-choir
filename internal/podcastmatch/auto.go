package podcastmatch

// The automatic matcher: no model, no network. For each story it ranks the
// stored transcripts' passages (BM25, see Rank) and accepts a clip only when
// every rule below holds. A wrong clip on air is worse than no clip, so the
// rules refuse; a passage that clears them all is a candidate the server then
// verifies again (the story is on the front, the quote is in the transcript
// inside the clip's times).
//
// The rules, in the order they are tried (the first to refuse is the one
// counted):
//
//	time      the episode was published no more than Params.Before before the
//	          story's last update and no more than Params.After after it, so
//	          an old discussion of a recurring subject is not passed off as
//	          about today's event;
//	clip      a clip of 20 to 75 seconds exists on segment edges;
//	boundary  and its edges are sentence or speaker-turn edges (a pause of 0.9 s
//	          or more) of the transcript, never mid-sentence or mid-word;
//	score     the clip's own BM25 for the story is at least MinScore;
//	entity    the clip carries at least MinEntities of the story's distinctive
//	          terms and at least MinEntityShare of them, and at least one of
//	          those is rare in the transcript corpus (a name every show says
//	          all day is not evidence);
//	margin    the clip covers at most MaxOthers other stories (more is a
//	          roundup) and its score for this story is at least 1+MinMargin times its
//	          score for any other story of the list that is not the same
//	          event (a passage that matches five stories about equally matches
//	          none);
//	cap       at most PerStory clips per story and one per episode.
//
// Entities are found without a model: a word capitalised away from the start
// of a sentence, an all-capitals word, a word with a digit, and any title or
// dek term whose document frequency over the transcript windows is at most
// RareTermDF (rarity weights the story's own words by how few windows say
// them). Names every transcriber garbles are matched within one edit.

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Story is a story of the paper's front list.
type Story struct {
	ID      string
	Title   string
	Dek     string
	Updated time.Time
}

// Episode is a transcribed episode.
type Episode struct {
	Key             string
	Published       time.Time
	DurationSeconds float64
	Segs            []Segment
}

// Params are the thresholds. Strict, Default and Loose are the three settings
// the tests print a precision and recall table for.
type Params struct {
	Before, After  time.Duration // episode publish time versus the story's last update
	MinScore       float64       // the clip's BM25 as a share of the best a passage could earn for the story
	MinMargin      float64
	MinEntities    int
	MinEntityShare float64
	MinNameShare   float64 // of the story's names (capitalised words), the share the clip carries
	MinTopic       int     // event words (the story's other distinctive terms) the clip carries
	MinFocus       float64 // share of the clip's segments that carry a distinctive term of the story
	RareDF         float64 // a matched term this rare (share of windows) must be among the matches
	RareTermDF     float64 // a title or dek term this rare joins the story's distinctive terms
	MaxOthers      int     // other stories (not the same event) a clip may also cover; more is a roundup
	PerStory       int
	TopK           int // passages ranked per story before the rules
}

// DefaultParams are the shipped thresholds.
var DefaultParams = Params{
	Before: 48 * time.Hour, After: 6 * time.Hour,
	MinScore: 0.15, MinMargin: 0.5, MinEntities: 2, MinEntityShare: 0.3, MinNameShare: 0.4, MinTopic: 1, MinFocus: 0.4,
	RareDF: 0.05, RareTermDF: 0.03, MaxOthers: 1, PerStory: 3, TopK: 12,
}

// StrictParams refuses more than DefaultParams.
var StrictParams = func() Params {
	p := DefaultParams
	p.MinScore, p.MinMargin, p.MinEntityShare = 0.25, 0.8, 0.5
	return p
}()

// LooseParams accepts more than DefaultParams.
var LooseParams = func() Params {
	p := DefaultParams
	p.MinScore, p.MinMargin, p.MinEntityShare = 0.1, 0.25, 0.2
	return p
}()

// Rule names a refusal.
type Rule string

const (
	RuleTime     Rule = "time"
	RuleClip     Rule = "clip"
	RuleBoundary Rule = "boundary"
	RuleScore    Rule = "score"
	RuleEntity   Rule = "entity"
	RuleMargin   Rule = "margin"
	RuleCap      Rule = "cap"
)

// Rules lists them in the order they are tried.
var Rules = []Rule{RuleTime, RuleClip, RuleBoundary, RuleScore, RuleEntity, RuleMargin, RuleCap}

// Accepted is a clip that cleared every rule.
type Accepted struct {
	StoryID     string
	EpisodeKey  string
	Start, End  float64
	Quote       string
	Text        string
	Score       float64
	Margin      float64 // the clip's score over the next story's, 0 when no other story scored
	EntityShare float64
	Support     float64
}

// Result is one run's answer and its counts.
type Result struct {
	Accepted   []Accepted
	Rejected   map[Rule]int
	Stories    int // stories considered
	Scored     int // (story, window) pairs scored
	Passages   int // passages the rules were tried on
	Candidates int // passages ranked for the stories (before the rules)
}

// Match runs the matcher over stories and episodes.
func Match(ctx context.Context, stories []Story, eps []Episode, p Params) (Result, error) {
	return MatchSubjects(ctx, stories, nil, eps, p)
}

// MatchSubjects is Match for the stories named in subjects only (nil means all
// of them); the others still count as competitors for the margin rule.
func MatchSubjects(ctx context.Context, stories []Story, subjects map[string]bool, eps []Episode, p Params) (Result, error) {
	res := Result{Rejected: map[Rule]int{}}
	byKey := map[string]*Episode{}
	var wins []Window
	for i := range eps {
		byKey[eps[i].Key] = &eps[i]
		wins = append(wins, Windows(eps[i].Key, eps[i].Segs)...)
	}
	c := newCorpus(wins)
	for _, st := range stories {
		if subjects == nil || subjects[st.ID] {
			res.Stories++
		}
	}
	if c.n == 0 || c.avg == 0 || len(stories) == 0 {
		return res, nil
	}
	topK := p.TopK
	if topK <= 0 {
		topK = 12
	}
	gists := make([]string, len(stories))
	qts := make([]map[string]int, len(stories))
	ents := make([]entitySet, len(stories))
	for i, st := range stories {
		gists[i] = strings.TrimSpace(st.Title + ". " + st.Dek)
		qts[i] = queryTerms(gists[i])
		ents[i] = c.entities(gists[i], p.RareTermDF)
	}
	for i, st := range stories {
		if subjects != nil && !subjects[st.ID] {
			continue
		}
		hits, err := c.rankOne(ctx, Query{ID: st.ID, Text: gists[i]}, topK)
		if err != nil {
			return res, err
		}
		res.Scored += len(wins)
		res.Candidates += len(hits)
		taken := map[string]bool{}
		accepted := 0
		for _, h := range hits {
			ep := byKey[h.Window.Key]
			if ep == nil {
				continue
			}
			res.Passages++
			rule, acc := c.judge(ctx, stories, qts, ents, i, ep, h, p)
			if rule == "" {
				if taken[ep.Key] || accepted >= p.PerStory {
					res.Rejected[RuleCap]++
					continue
				}
				taken[ep.Key] = true
				accepted++
				res.Accepted = append(res.Accepted, acc)
				continue
			}
			res.Rejected[rule]++
		}
	}
	sort.SliceStable(res.Accepted, func(a, b int) bool { return res.Accepted[a].Support > res.Accepted[b].Support })
	return res, ctx.Err()
}

// judge tries the rules on one passage and returns the first that refuses it
// (empty when it clears them all) and the accepted clip.
func (c *corpus) judge(ctx context.Context, stories []Story, qts []map[string]int, ents []entitySet, si int, ep *Episode, h Hit, p Params) (Rule, Accepted) {
	st := stories[si]
	if !st.Updated.IsZero() && !ep.Published.IsZero() {
		if ep.Published.Before(st.Updated.Add(-p.Before)) || ep.Published.After(st.Updated.Add(p.After)) {
			return RuleTime, Accepted{}
		}
	}
	gist := strings.TrimSpace(st.Title + ". " + st.Dek)
	clip, ok := ChooseClip(ep.Segs, *h.Window, gist)
	if !ok || (ep.DurationSeconds > 0 && clip.End > ep.DurationSeconds+5) {
		return RuleClip, Accepted{}
	}
	clip, ok = ChooseSentenceClip(ep.Segs, *h.Window, gist)
	if !ok || (ep.DurationSeconds > 0 && clip.End > ep.DurationSeconds+5) {
		return RuleBoundary, Accepted{}
	}
	cw := Window{Key: ep.Key, First: clip.First, Last: clip.Last, Start: clip.Start, End: clip.End, Text: clip.Text}
	cw.index()
	ownRaw, _ := c.score(qts[si], &cw)
	ceiling := c.ceiling(qts[si])
	own := 0.0
	if ceiling > 0 {
		own = ownRaw / ceiling
	}
	if own < p.MinScore {
		return RuleScore, Accepted{}
	}
	if !c.entityGate(ents[si], qts[si], &cw, clip, ep.Segs, p) {
		return RuleEntity, Accepted{}
	}
	all := ents[si].all()
	_, share, _ := c.entityHits(all, &cw, p.RareDF)
	other, ownMargin := 0.0, ownRaw
	for j := range stories {
		if j == si || sameEvent(ents[si].all(), ents[j].all()) {
			continue
		}
		if s, _ := c.score(qts[j], &cw); s > other {
			other = s
		}
	}
	crowd := 0
	for j := range stories {
		if j == si || sameEvent(ents[si].all(), ents[j].all()) {
			continue
		}
		if m, sh, _ := c.entityHits(ents[j].all(), &cw, p.RareDF); m >= p.MinEntities && sh >= p.MinEntityShare {
			crowd++
		}
	}
	if crowd > p.MaxOthers {
		return RuleMargin, Accepted{}
	}
	margin := 0.0
	if other > 0 {
		margin = ownMargin / other
		if margin < 1+p.MinMargin {
			return RuleMargin, Accepted{}
		}
	}
	support := 0.5 + 0.3*share
	if other > 0 {
		support += 0.15 * math.Min(1, (margin-1)/3)
	} else {
		support += 0.15
	}
	return "", Accepted{
		StoryID: st.ID, EpisodeKey: ep.Key, Start: clip.Start, End: clip.End, Quote: clip.Quote, Text: clip.Text,
		Score: own, Margin: margin, EntityShare: share, Support: math.Min(0.95, support),
	}
}

// ceiling is the BM25 a window would earn for the query if it carried every
// term once at the mean length: the score a perfect passage reaches. MinScore
// is a share of it, so the threshold does not move with the corpus size.
func (c *corpus) ceiling(qterms map[string]int) float64 {
	total := 0.0
	for term := range qterms {
		d := float64(c.df[term])
		if d == 0 {
			d = 1
		}
		idf := math.Log(1 + (c.n-d+0.5)/(d+0.5))
		total += idf * (bm25K1 + 1) / (1 + bm25K1)
	}
	return total
}

// sameEvent reports whether two stories' distinctive terms overlap by half or
// more (two versions of one event), so they do not count against each other's
// margin.
func sameEvent(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := map[string]bool{}
	for _, t := range a {
		set[t] = true
	}
	inter := 0
	for _, t := range b {
		if set[t] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	return float64(inter)/float64(union) >= 0.5
}

// entities returns the story's distinctive normalised terms: capitalised,
// all-capitals and digit words away from a sentence start, plus any term of
// the text whose document frequency is at most rareDF of the windows.
func (c *corpus) entities(text string, rareDF float64) entitySet {
	seen := map[string]bool{}
	var set entitySet
	add := func(w string, name bool) {
		for _, t := range Tokens(w) {
			if !seen[t] {
				seen[t] = true
				if name {
					set.Names = append(set.Names, t)
				} else {
					set.Topic = append(set.Topic, t)
				}
			}
		}
	}
	words := strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || r == ',' || r == ';' || r == '(' || r == ')' })
	start := true
	for i, w := range words {
		bare := strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if bare == "" {
			continue
		}
		first := []rune(bare)[0]
		hasDigit := strings.IndexFunc(bare, unicode.IsDigit) >= 0
		allCaps := len([]rune(bare)) >= 2 && strings.ToUpper(bare) == bare && strings.IndexFunc(bare, unicode.IsLetter) >= 0
		capital := unicode.IsUpper(first)
		nextCap := false
		if i+1 < len(words) {
			if nr := []rune(strings.TrimLeft(words[i+1], `"'(`)); len(nr) > 0 {
				nextCap = unicode.IsUpper(nr[0])
			}
		}
		switch {
		case hasDigit:
			add(bare, false)
		case allCaps:
			add(bare, true)
		case capital && (!start || nextCap):
			add(bare, true)
		}
		start = strings.HasSuffix(w, ".") || strings.HasSuffix(w, "?") || strings.HasSuffix(w, "!") || strings.HasSuffix(w, ":")
	}
	for _, t := range Tokens(text) {
		if seen[t] {
			continue
		}
		if c.n > 0 && float64(c.df[t])/c.n <= rareDF {
			seen[t] = true
			set.Topic = append(set.Topic, t)
		}
	}
	return set
}

// entitySet is a story's distinctive terms: Names are capitalised or all
// capitals words; Topic is the rest (numbers and rare words of the title and dek).
type entitySet struct{ Names, Topic []string }

func (e entitySet) all() []string { return append(append([]string{}, e.Names...), e.Topic...) }

// entityGate is the rule that the clip is about this story and not merely
// near its words: enough of the story's terms, at least one rare in the
// corpus, a good share of its names AND event words (a firm's name in an
// unrelated item is not the event), and most of the clip's segments on the
// story (a headline roundup is one sentence per story).
func (c *corpus) entityGate(e entitySet, qterms map[string]int, cw *Window, clip Clip, segs []Segment, p Params) bool {
	all := e.all()
	matched, share, rare := c.entityHits(all, cw, p.RareDF)
	if len(all) < p.MinEntities || matched < p.MinEntities || share < p.MinEntityShare || !rare {
		return false
	}
	nm, nShare, _ := c.entityHits(e.Names, cw, p.RareDF)
	tm, _, _ := c.entityHits(e.Topic, cw, p.RareDF)
	if len(e.Names) > 0 {
		if nShare < p.MinNameShare || nm < 1 {
			return false
		}
		if len(e.Topic) > 0 && tm < min(p.MinTopic, len(e.Topic)) {
			return false
		}
	} else if share < 0.5 {
		return false
	}
	on := 0
	for i := clip.First; i <= clip.Last; i++ {
		sw := Window{Text: segs[i].Text}
		sw.index()
		for t := range qterms {
			if sw.terms[t] > 0 && float64(c.df[t])/c.n <= 0.2 {
				on++
				break
			}
		}
	}
	return float64(on)/float64(clip.Last-clip.First+1) >= p.MinFocus
}

// entityHits counts the entity terms the clip carries (a near-miss spelling of
// a long term counts), their share of the story's terms, and whether one of
// them is rare in the corpus.
func (c *corpus) entityHits(ents []string, w *Window, rareDF float64) (matched int, share float64, rare bool) {
	if len(ents) == 0 {
		return 0, 0, false
	}
	for _, t := range ents {
		hit := w.terms[t] > 0
		if !hit && len(t) >= 6 {
			hit = nearTF(w, t) > 0
		}
		if !hit {
			continue
		}
		matched++
		if c.n > 0 && float64(c.df[t])/c.n <= rareDF {
			rare = true
		}
	}
	return matched, float64(matched) / float64(len(ents)), rare
}
