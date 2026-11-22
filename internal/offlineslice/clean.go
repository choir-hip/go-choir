package offlineslice

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/publicsuffix"

	"github.com/yusefmosiah/go-choir/internal/entitytext"
)

// Cleaner drops candidates that are furniture or common words, by rules that
// are model-free and use only what EARLIER DAYS showed: Observe adds a capture
// to today's counts, StartDay folds them into what Drop may read.
type Cleaner struct {
	dfMax       float64 // a new name in more than this share of earlier captures is junk
	dfMaxKnown  float64 // the same for a name some entity answers to (an actor can be common; a lexicon's junk entity is commoner)
	commonRatio float64 // a one-word name that is lower case in more than this share of earlier running text is a common noun
	commonMin   int     // and was seen at least this often
	df          map[string]int
	nDocs       int
	lower, capt map[string]int
	dayDF       map[string]int
	dayDocs     int
	dayLower    map[string]int
	dayCapt     map[string]int
}

// NewCleaner builds a cleaner.
func NewCleaner(dfMax, commonRatio float64, commonMin int) *Cleaner {
	c := &Cleaner{dfMax: dfMax, dfMaxKnown: dfMax, commonRatio: commonRatio, commonMin: commonMin}
	c.df, c.lower, c.capt = map[string]int{}, map[string]int{}, map[string]int{}
	c.dayDF, c.dayLower, c.dayCapt = map[string]int{}, map[string]int{}, map[string]int{}
	return c
}

// SetKnownDF sets the document-frequency bound for names an entity answers to.
func (c *Cleaner) SetKnownDF(x float64) { c.dfMaxKnown = x }

// StartDay makes the days observed so far readable.
func (c *Cleaner) StartDay() {
	for k, v := range c.dayDF {
		c.df[k] += v
	}
	for k, v := range c.dayLower {
		c.lower[k] += v
	}
	for k, v := range c.dayCapt {
		c.capt[k] += v
	}
	c.nDocs += c.dayDocs
	c.dayDF, c.dayLower, c.dayCapt, c.dayDocs = map[string]int{}, map[string]int{}, map[string]int{}, 0
}

var wordRe = regexp.MustCompile(`\pL[\pL'’-]*`)

// Observe counts a capture: its mention keys (once each) and, for the
// common-noun test, how its words are written in running text.
func (c *Cleaner) Observe(ms []Mention, title, lead string) {
	c.dayDocs++
	seen := map[string]bool{}
	for _, m := range ms {
		if !seen[m.Key] {
			seen[m.Key] = true
			c.dayDF[m.Key]++
		}
	}
	text := title + "\n\n" + lead
	for _, loc := range wordRe.FindAllStringIndex(text, -1) {
		w := text[loc[0]:loc[1]]
		key := entitytext.Key(w)
		if key == "" {
			continue
		}
		rs := []rune(w)
		switch {
		case allLower(rs):
			c.dayLower[key]++
		case unicode.IsUpper(rs[0]) && allLower(rs[1:]) && !sentenceStart(text, loc[0]):
			c.dayCapt[key]++
		}
	}
}

func allLower(rs []rune) bool {
	for _, r := range rs {
		if unicode.IsLetter(r) && !unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// sentenceStart: the word at byte i begins the text or follows a sentence end, a colon or a line break.
func sentenceStart(text string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		switch text[j] {
		case ' ', '\t', '"', '\'', '(', '[':
			continue
		case '.', '!', '?', ':', '\n', '\r', '|':
			return true
		default:
			return false
		}
	}
	return true
}

// outletLabel is the first label of a host's registrable domain.
func outletLabel(host string) string {
	host = strings.ToLower(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www."))
	if d, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		host = d
	}
	if i := strings.Index(host, "."); i > 0 {
		host = host[:i]
	}
	return host
}

// Drop says why a mention is junk ("outlet", "df", "commonnoun"), or "".
func (c *Cleaner) Drop(m Mention, host string) string {
	if host != "" {
		label := outletLabel(host)
		flat := strings.ReplaceAll(strings.TrimPrefix(m.Key, "the "), " ", "")
		if label != "" && flat == label {
			return "outlet"
		}
	}
	lim := c.dfMax
	if len(m.Cands) > 0 {
		lim = c.dfMaxKnown
	}
	if c.nDocs > 0 && float64(c.df[m.Key])/float64(c.nDocs) > lim {
		return "df"
	}
	if !m.Acronym && !strings.Contains(m.Key, " ") {
		l, p := c.lower[m.Key], c.capt[m.Key]
		if l+p >= c.commonMin && float64(l)/float64(l+p) > c.commonRatio {
			return "commonnoun"
		}
	}
	return ""
}

var (
	urlRe     = regexp.MustCompile(`(?i)(?:https?://(?:www\.\s)?|www\.\s?)\S+(?:/\s[a-z0-9][\w.%~-]*-[\w.%~-]*)*`)
	hashtagRe = regexp.MustCompile(`#\s?[\pL\pN_]+`)
	mentionRe = regexp.MustCompile(`(?:RT\s+)?@[\pL\pN_.]+(@[\pL\pN_.]+)?:?`)
)

// CleanText takes the furniture of a post out of a lead before the rules read
// it: links (including the broken "www. host/path" form the extractor leaves),
// hashtags and account handles. A hashtag is a label the poster chose, not a
// name the text states, and "EuropeSays" or "UkraineWar" are not subjects.
func CleanText(s string) string {
	s = urlRe.ReplaceAllString(s, " ")
	s = hashtagRe.ReplaceAllString(s, " ")
	s = mentionRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}
