// Package podcastmatch finds the passages of podcast transcripts that may
// source a news story: overlapping windows of timed segments, ranked by BM25
// over normalised tokens with a tolerant match for names (every transcriber
// garbles them), and a clip chosen inside a window on segment boundaries
// (docs/design/podcast-clips-2026-11-15.md section 3). It is pure: no model,
// no store.
package podcastmatch

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"
)

// Segment is one sentence or pause bounded stretch of a transcript.
type Segment struct {
	Start, End float64
	Text       string
}

// Window is a run of consecutive segments of one transcript.
type Window struct {
	Key         string // the caller's id for the transcript (an episode id)
	First, Last int    // segment indices, inclusive
	Start, End  float64
	Text        string
	terms       map[string]int
	length      int
}

const (
	// WindowSeconds and StrideSeconds are the TREC baseline: overlapping two
	// minute segments, here with a one minute stride.
	WindowSeconds = 120.0
	StrideSeconds = 60.0
	// ClipMinSeconds and ClipMaxSeconds bound a chosen clip.
	ClipMinSeconds = 20.0
	ClipMaxSeconds = 75.0

	bm25K1 = 1.2
	bm25B  = 0.75
	// fuzzyWeight is the share of a term's weight a near-miss spelling earns.
	fuzzyWeight = 0.5
	// MaxWindowsPerTranscript bounds the windows of one transcript (about seven
	// hours at one a minute), so no transcript costs the ranking without bound.
	MaxWindowsPerTranscript = 420
	// QuoteMaxRunes is the longest display quote.
	QuoteMaxRunes = 280
)

var stopwords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`a about after again all also am an and any are as at be because been before being but by can could did do does doing down during each few for from further had has have having he her here hers him his how i if in into is it its just like me more most my no nor not now of off on once only or other our out over own said say says she should so some such than that the their them then there these they this those through to too under until up us very was we were what when where which while who whom why will with would you your yes yeah okay well really going get got one two`) {
		m[w] = true
	}
	return m
}()

// Tokens normalises text into comparable terms: lower case, letters and
// digits only, accents folded to their base letter where common, stop words
// dropped and a crude stem.
func Tokens(text string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		w := stem(cur.String())
		cur.Reset()
		if len(w) < 2 || stopwords[w] {
			return
		}
		out = append(out, w)
	}
	for _, r := range strings.ToLower(text) {
		r = fold(r)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else if r == '\'' || r == '’' {
			continue // joins "it's" to "its" and "Biden's" to "Bidens", then stemmed
		} else {
			flush()
		}
	}
	flush()
	return out
}

func fold(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'ö', 'õ':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	case 'ñ':
		return 'n'
	}
	return r
}

func stem(w string) string {
	if stopwords[w] {
		return w
	}
	for _, suf := range []string{"ing", "ed", "es", "s"} {
		if len(w) > len(suf)+3 && strings.HasSuffix(w, suf) {
			return strings.TrimSuffix(w, suf)
		}
	}
	return w
}

// Windows cuts a transcript into overlapping windows of about WindowSeconds,
// one starting each StrideSeconds, each holding whole segments.
func Windows(key string, segs []Segment) []Window {
	var out []Window
	if len(segs) == 0 {
		return nil
	}
	lastStart := -1
	for next := segs[0].Start; len(out) < MaxWindowsPerTranscript; next += StrideSeconds {
		if next+StrideSeconds <= next {
			break // the float no longer advances
		}
		i := sort.Search(len(segs), func(i int) bool { return segs[i].Start >= next })
		if i >= len(segs) {
			break
		}
		if i == lastStart {
			continue
		}
		lastStart = i
		j := i
		for j+1 < len(segs) && segs[j+1].End-segs[i].Start <= WindowSeconds {
			j++
		}
		var b strings.Builder
		for k := i; k <= j; k++ {
			if k > i {
				b.WriteByte(' ')
			}
			b.WriteString(segs[k].Text)
		}
		w := Window{Key: key, First: i, Last: j, Start: segs[i].Start, End: segs[j].End, Text: b.String()}
		w.index()
		out = append(out, w)
		if j == len(segs)-1 {
			break
		}
	}
	return out
}

func (w *Window) index() {
	w.terms = map[string]int{}
	for _, t := range Tokens(w.Text) {
		w.terms[t]++
		w.length++
	}
}

// Query is a story's words.
type Query struct {
	ID   string
	Text string
}

// Hit is a ranked window for a query.
type Hit struct {
	QueryID string
	Window  *Window
	Score   float64
	// Matched is how many distinct query terms the window carries, near-miss
	// spellings included.
	Matched int
}

// corpus holds the statistics BM25 needs: the document frequency of each term
// over the windows, the number of windows and the mean window length.
type corpus struct {
	windows []Window
	n, avg  float64
	df      map[string]int
}

func newCorpus(windows []Window) *corpus {
	c := &corpus{windows: windows, n: float64(len(windows)), df: map[string]int{}}
	for i := range windows {
		c.avg += float64(windows[i].length)
		for t := range windows[i].terms {
			c.df[t]++
		}
	}
	if c.n > 0 {
		c.avg /= c.n
	}
	return c
}

// score is the BM25 of one window for a query's terms, and how many distinct
// query terms the window carries (near-miss spellings included).
func (c *corpus) score(qterms map[string]int, w *Window) (score float64, matched int) {
	for term := range qterms {
		tf := float64(w.terms[term])
		weight := 1.0
		if tf == 0 && len(term) >= 6 {
			tf = nearTF(w, term)
			weight = fuzzyWeight
		}
		if tf == 0 {
			continue
		}
		matched++
		d := float64(c.df[term])
		if d == 0 {
			d = 1
		}
		idf := math.Log(1 + (c.n-d+0.5)/(d+0.5))
		score += weight * idf * (tf * (bm25K1 + 1)) / (tf + bm25K1*(1-bm25B+bm25B*float64(w.length)/c.avg))
	}
	return score, matched
}

func queryTerms(text string) map[string]int {
	qterms := map[string]int{}
	for _, t := range Tokens(text) {
		qterms[t]++
	}
	return qterms
}

// rankOne returns up to topK hits for one query: positive score, at least two
// distinct matched terms, best first, windows of one transcript that overlap
// a better hit left out. scored is how many windows were scored.
func (c *corpus) rankOne(ctx context.Context, q Query, topK int) (kept []Hit, err error) {
	qterms := queryTerms(q.Text)
	var hits []Hit
	for wi := range c.windows {
		// The deadline cancels the ranking: it is the one unbounded loop.
		if wi%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		w := &c.windows[wi]
		score, matched := c.score(qterms, w)
		if score > 0 && matched >= 2 {
			hits = append(hits, Hit{QueryID: q.ID, Window: w, Score: score, Matched: matched})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	for _, h := range hits {
		overlap := false
		for _, k := range kept {
			if k.Window.Key == h.Window.Key && h.Window.Start < k.Window.End && k.Window.Start < h.Window.End {
				overlap = true
				break
			}
		}
		if !overlap {
			kept = append(kept, h)
		}
		if len(kept) == topK {
			break
		}
	}
	return kept, nil
}

// Rank scores every window for every query by BM25 (document frequency from
// the windows themselves) and returns, per query, up to topK hits with a
// positive score and at least two distinct matched terms, best first, windows
// of one transcript that overlap a better hit left out.
func Rank(ctx context.Context, queries []Query, windows []Window, topK int) (map[string][]Hit, error) {
	out := map[string][]Hit{}
	c := newCorpus(windows)
	if c.n == 0 || c.avg == 0 {
		return out, nil
	}
	for _, q := range queries {
		kept, err := c.rankOne(ctx, q, topK)
		if err != nil {
			return nil, err
		}
		out[q.ID] = kept
	}
	return out, nil
}

// nearTF counts the window's tokens within edit distance 1 of term (both at
// least six letters), the garbled-name case.
func nearTF(w *Window, term string) float64 {
	n := 0
	for t, c := range w.terms {
		if len(t) >= 6 && absInt(len(t)-len(term)) <= 1 && t[0] == term[0] && withinOne(t, term) {
			n += c
		}
	}
	return float64(n)
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// withinOne reports whether a and b differ by at most one substitution,
// insertion or deletion (over bytes; terms are ASCII after folding in the
// common case, and a miss on other scripts only loses a tolerant match).
func withinOne(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	if len(a)-len(b) > 1 {
		return false
	}
	i, j, edits := 0, 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		if len(a) == len(b) {
			i++
			j++
		} else {
			i++
		}
	}
	return edits+(len(a)-i)+(len(b)-j) <= 1
}

// Clip is a chosen range of a transcript.
type Clip struct {
	Start, End float64
	First      int
	Last       int
	Quote      string
	Text       string // the spoken words of the range, for judging
}

// ChooseClip picks the range of consecutive segments inside (and a segment or
// two around) the window that best carries the query's terms: the run of 20 to
// 75 seconds that maximises the sum of each segment's weight minus a quarter
// of the best segment's weight, so irrelevant edges are trimmed. It returns
// false when no run of segments fits the bounds.
func ChooseClip(segs []Segment, win Window, query string) (Clip, bool) {
	return chooseClip(segs, win, query, false)
}

// ChooseSentenceClip is ChooseClip with the clip's edges on sentence or
// speaker-turn edges: it starts where the segment before ends a sentence (or
// a pause of PauseEdgeSeconds or more precedes it) and ends on a segment that
// ends a sentence (or is followed by such a pause). The transcript's segments
// are sentence or pause bounded, so a clip never starts or ends mid-word; this
// also keeps it from starting or ending mid-sentence.
func ChooseSentenceClip(segs []Segment, win Window, query string) (Clip, bool) {
	return chooseClip(segs, win, query, true)
}

// PauseEdgeSeconds is the silence that counts as a turn edge.
const PauseEdgeSeconds = 0.9

// SentenceEnd reports whether text ends a sentence.
func SentenceEnd(text string) bool {
	t := strings.TrimRight(strings.TrimSpace(text), `"')]}»”’`)
	return strings.HasSuffix(t, ".") || strings.HasSuffix(t, "?") || strings.HasSuffix(t, "!")
}

func startsClean(segs []Segment, i int) bool {
	return i == 0 || SentenceEnd(segs[i-1].Text) || segs[i].Start-segs[i-1].End >= PauseEdgeSeconds
}

func endsClean(segs []Segment, j int) bool {
	return j == len(segs)-1 || SentenceEnd(segs[j].Text) || segs[j+1].Start-segs[j].End >= PauseEdgeSeconds
}

func chooseClip(segs []Segment, win Window, query string, edges bool) (Clip, bool) {
	q := map[string]bool{}
	for _, t := range Tokens(query) {
		q[t] = true
	}
	lo, hi := max(0, win.First-2), min(len(segs)-1, win.Last+2)
	weight := make([]float64, len(segs))
	best := 0.0
	for i := lo; i <= hi; i++ {
		for _, t := range Tokens(segs[i].Text) {
			if q[t] {
				weight[i]++
			} else if len(t) >= 6 {
				for qt := range q {
					if len(qt) >= 6 && absInt(len(qt)-len(t)) <= 1 && qt[0] == t[0] && withinOne(qt, t) {
						weight[i] += fuzzyWeight
						break
					}
				}
			}
		}
		best = math.Max(best, weight[i])
	}
	if best == 0 {
		return Clip{}, false
	}
	tau := 0.25 * best
	found, bestScore := false, math.Inf(-1)
	var pick Clip
	for i := lo; i <= hi; i++ {
		score := 0.0
		if edges && !startsClean(segs, i) {
			continue
		}
		for j := i; j <= hi; j++ {
			dur := segs[j].End - segs[i].Start
			if dur > ClipMaxSeconds {
				break
			}
			score += weight[j] - tau
			if edges && !endsClean(segs, j) {
				continue
			}
			if dur >= ClipMinSeconds && score > bestScore {
				found, bestScore = true, score
				pick = Clip{Start: segs[i].Start, End: segs[j].End, First: i, Last: j}
			}
		}
	}
	if !found {
		return Clip{}, false
	}
	var spoken strings.Builder
	quoteSeg, quoteW := pick.First, -1.0
	for k := pick.First; k <= pick.Last; k++ {
		if k > pick.First {
			spoken.WriteByte(' ')
		}
		spoken.WriteString(segs[k].Text)
		if weight[k] > quoteW {
			quoteSeg, quoteW = k, weight[k]
		}
	}
	pick.Text = spoken.String()
	pick.Quote = cutRunes(strings.Join(strings.Fields(segs[quoteSeg].Text), " "), QuoteMaxRunes)
	return pick, true
}

// cutRunes cuts at most n runes on a word boundary and adds nothing: the quote
// is a prefix of what was said.
func cutRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	cut := n
	for cut > 0 && r[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = n
	}
	return strings.TrimSpace(string(r[:cut]))
}
