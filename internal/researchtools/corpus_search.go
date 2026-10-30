package researchtools

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The corpus search's capture half (symptom 3 of
// docs/archive/problems/prompt-bar-research-slow-unsynthesized-mangled-2026-10-14.md):
// research finds what this computer already captured before it goes to the
// web. It is a keyword search over each capture's metadata (title, outlet,
// source id, language) and Jev's label for it (outlet, script, and the
// story headline the label ties it to), never over block text: a capture's
// row carries no body, and reading every body would cost one corpusd read
// per capture.
// corpusCaptureScan is how many of the computer's newest captures the
// window path reads from the shared store (the same bound Texture's URL
// cite lookup uses). A variable so tests can shrink the window.
var corpusCaptureScan = textureCaptureListLimit

const (
	// corpusLabelScan is how many of the computer's newest labels are read
	// as hints; a relabel writes a second label object.
	corpusLabelScan = 2 * textureCaptureListLimit
	// corpusDefaultLimit and corpusMaxLimit bound each result list.
	corpusDefaultLimit = 10
	corpusMaxLimit     = 25
	// corpusMaxTerms bounds the query's words.
	corpusMaxTerms = 8
)

// CorpusCapture is one capture a corpus search found. CaptureID is a handle
// research cites (target.capture_id) and Texture's {{cite}} accepts.
type CorpusCapture struct {
	CaptureID   string   `json:"capture_id"`
	Title       string   `json:"title,omitempty"`
	URL         string   `json:"url"`
	Outlet      string   `json:"outlet,omitempty"`
	SourceID    string   `json:"source_id,omitempty"`
	Language    string   `json:"language,omitempty"`
	Script      string   `json:"script,omitempty"`
	Topic       string   `json:"topic,omitempty"`
	FetchedAt   string   `json:"fetched_at,omitempty"`
	PublishedAt string   `json:"published_at,omitempty"`
	BlockCount  int      `json:"block_count,omitempty"`
	Matched     []string `json:"matched"`
}

// corpusField is one searchable field of a capture and the weight of a
// query word found in it.
type corpusField struct {
	name   string
	text   string
	weight int
}

// corpusQueryTerms lowercases the query, strips punctuation from each word,
// and drops function words, keeping at most corpusMaxTerms words.
func corpusQueryTerms(query string) []string {
	var terms []string
	seen := map[string]bool{}
	for _, word := range strings.Fields(strings.ToLower(query)) {
		word = strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		word = strings.TrimSuffix(word, "'s")
		if word == "" || corpusStopwords[word] || seen[word] {
			continue
		}
		seen[word] = true
		terms = append(terms, word)
		if len(terms) == corpusMaxTerms {
			break
		}
	}
	return terms
}

var corpusStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "of": true, "in": true, "on": true, "at": true,
	"to": true, "for": true, "from": true, "by": true, "with": true, "about": true, "is": true, "are": true,
	"was": true, "were": true, "what": true, "whats": true, "what's": true, "who": true, "how": true, "why": true,
	"when": true, "where": true, "which": true, "latest": true, "current": true, "recent": true,
}

// corpusTermIn reports whether term starts a word of lower (already
// lowercased). Scripts written without spaces between words (Han, kana,
// Thai and the like), and scripts that attach prefixes to a word (Arabic
// and Hebrew articles and conjunctions), match anywhere.
func corpusTermIn(lower, term string) bool {
	if term == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(term)
	anywhere := unicode.In(first, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar, unicode.Arabic, unicode.Hebrew)
	for from := 0; from < len(lower); {
		i := strings.Index(lower[from:], term)
		if i < 0 {
			return false
		}
		at := from + i
		if anywhere || at == 0 {
			return true
		}
		if r, _ := utf8.DecodeLastRuneInString(lower[:at]); !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return true
		}
		from = at + len(term)
	}
	return false
}

// corpusNeeded is how many of n query words a capture must match: every
// word of a one- or two-word query, at least half of a longer one.
func corpusNeeded(n int) int {
	if n <= 2 {
		return n
	}
	return (n + 1) / 2
}

// corpusLimit clamps a requested result count.
func corpusLimit(limit int) int {
	if limit <= 0 {
		return corpusDefaultLimit
	}
	if limit > corpusMaxLimit {
		return corpusMaxLimit
	}
	return limit
}

// searchCaptures finds the computer's captures matching terms, ranked by
// how many query words they match, then by how strongly (a title word
// outweighs an outlet or label word), then newest first. Candidates come
// from corpusd's capture index over all of the computer's captures; when
// the index refuses (or CHOIR_CORPUS_SEARCH=window), from the window of
// its newest corpusCaptureScan captures. A note says which, and when the
// capture store is not configured or a bound was hit.
func searchCaptures(ctx context.Context, deps Dependencies, ownerID, computerID string, terms []string, limit int) ([]CorpusCapture, string, error) {
	if deps.Captures == nil {
		return nil, "captures not searched: the shared capture store is not configured", nil
	}
	if strings.TrimSpace(computerID) == "" {
		return nil, "captures not searched: the run names no computer", nil
	}
	if len(terms) == 0 {
		return nil, "captures not searched: the query has no search words", nil
	}
	labels := map[string]store.CaptureLabel{}
	var notes []string
	if deps.Store != nil {
		if got, err := deps.Store.RecentCaptureLabels(ctx, ownerID, computerID, corpusLabelScan); err == nil {
			labels = got
		} else {
			notes = append(notes, "label hints not read: "+err.Error())
		}
	}
	var objs []objectgraph.Object
	searched := false
	if searcher, ok := deps.Captures.(CaptureSearcher); ok && corpusIndexEnabled() {
		answer, err := searcher.SearchCaptures(ctx, CaptureSearchQuery{OwnerID: ownerID, ComputerID: computerID, Words: terms,
			Need: corpusNeeded(len(terms)), Candidates: corpusIndexCandidates, IDs: labelMatchedCaptureIDs(labels, terms)})
		if err == nil {
			objs, searched = answer.Objects, true
			if answer.Capped {
				notes = append(notes, fmt.Sprintf("searched all %d captures of this computer; ranked the strongest, newest %d of the %d that match enough words", answer.Searched, corpusIndexCandidates, answer.Matched))
			}
		} else {
			notes = append(notes, "capture index not used ("+err.Error()+")")
		}
	}
	if !searched {
		window, err := deps.Captures.ListCaptures(ctx, ownerID, computerID, corpusCaptureScan)
		if err != nil {
			return nil, "", err
		}
		objs = window
		if len(objs) >= corpusCaptureScan {
			notes = append(notes, fmt.Sprintf("searched this computer's newest %d captures; older captures were not searched", corpusCaptureScan))
		}
	}
	return rankCaptures(objs, labels, terms, limit), strings.Join(notes, "; "), nil
}

// corpusIndexCandidates bounds the candidates corpusd returns for ranking.
const corpusIndexCandidates = 500

// labelMatchedCaptureIDs are the captures whose labels hold a query word:
// a capture can reach the needed words through its label alone, and the
// index holds only the capture's own fields, so these are sent by id.
func labelMatchedCaptureIDs(labels map[string]store.CaptureLabel, terms []string) []string {
	var out []string
	for id, label := range labels {
		text := strings.ToLower(strings.Join([]string{label.Title, label.EventHeadline, label.TopicHeadline, label.Outlet, label.Language, label.Script}, "\n"))
		for _, term := range terms {
			if corpusTermIn(text, term) {
				out = append(out, id)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// rankCaptures matches terms against each capture's fields and its label,
// keeps those matching enough words, and ranks them. objs is newest first
// (the last tie-break).
func rankCaptures(objs []objectgraph.Object, labels map[string]store.CaptureLabel, terms []string, limit int) []CorpusCapture {
	type hit struct {
		capture CorpusCapture
		words   int
		score   int
		fetched time.Time
		order   int
	}
	var hits []hit
	for i, obj := range objs {
		meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
		if err != nil {
			continue
		}
		label := labels[obj.CanonicalID]
		host := corpusHost(meta.URL)
		c := CorpusCapture{CaptureID: obj.CanonicalID, Title: firstNonEmptyString(meta.Title, label.Title), URL: meta.URL,
			Outlet: firstNonEmptyString(label.Outlet, host), SourceID: meta.SourceID, Language: firstNonEmptyString(meta.Language, label.Language),
			Script: label.Script, Topic: firstNonEmptyString(label.EventHeadline, label.TopicHeadline), FetchedAt: meta.FetchedAt,
			PublishedAt: meta.PublishedAt, BlockCount: meta.BlockCount}
		fields := []corpusField{
			{"title", c.Title, 3},
			{"topic", label.EventHeadline + "\n" + label.TopicHeadline, 2},
			{"outlet", label.Outlet + "\n" + host + "\n" + meta.SourceID, 2},
			{"language", c.Language + "\n" + c.Script, 1},
		}
		words, score := 0, 0
		matched := map[string]bool{}
		for _, term := range terms {
			best := 0
			for _, f := range fields {
				if f.weight > 0 && corpusTermIn(strings.ToLower(f.text), term) {
					matched[f.name] = true
					if f.weight > best {
						best = f.weight
					}
				}
			}
			if best > 0 {
				words++
				score += best
			}
		}
		if words == 0 || words < corpusNeeded(len(terms)) {
			continue
		}
		for _, f := range fields {
			if matched[f.name] {
				c.Matched = append(c.Matched, f.name)
			}
		}
		fetched, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(meta.FetchedAt))
		hits = append(hits, hit{capture: c, words: words, score: score, fetched: fetched, order: i})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.words != b.words {
			return a.words > b.words
		}
		if a.score != b.score {
			return a.score > b.score
		}
		if !a.fetched.Equal(b.fetched) {
			return a.fetched.After(b.fetched)
		}
		return a.order < b.order
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]CorpusCapture, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.capture)
	}
	return out
}

// corpusHost is a URL's host without "www.".
func corpusHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
