package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
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
	// Passage is the part of the page's text that matched the query, when
	// the text search found the page: quote it only after ReadCapture, which
	// returns the page's blocks.
	Passage        string `json:"passage,omitempty"`
	PassageOrdinal int    `json:"passage_ordinal,omitempty"`
	PassageCount   int    `json:"passage_count,omitempty"`
	// Reasons say why the page is listed: "text" (its words match), "meaning"
	// (its title and opening are near the question in meaning) and "story"
	// (a published story cites it; via_stories names the stories' publication
	// ids, whose title and captures are in the result's stories list). Absent
	// from an answer that did not come from corpusd's ranking.
	Reasons    []string `json:"reasons,omitempty"`
	ViaStories []string `json:"via_stories,omitempty"`
}

// CorpusStory is a published story corpusd found by meaning and walked: the
// captures its latest version cites that are listed in captures.
type CorpusStory struct {
	PublicationID string   `json:"publication_id"`
	Title         string   `json:"title,omitempty"`
	Revision      int      `json:"revision"`
	Day           string   `json:"day,omitempty"`
	CaptureIDs    []string `json:"capture_ids"`
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

// corpusRequest is one corpus search over the captures: the query as the
// model wrote it (the text search takes every word of it), the words the
// title and label half keeps, and a time window and order.
type corpusRequest struct {
	Query string
	Terms []string
	// Since is inclusive and Until exclusive; a page's time is its
	// published time when known, else the time it was fetched.
	Since, Until time.Time
	// Order is "relevance" (default) or "recent".
	Order string
}

// corpusSearchInfo counts how a capture search went, for the journal line.
type corpusSearchInfo struct {
	// TextUsed: corpusd's text search answered. TextIndexed and Captures
	// say how much of the computer it covers.
	TextUsed    bool
	TextIndexed int64
	Captures    int
	// Of the captures returned: found by the page text only, by the title
	// only, by both, and by the title and labels without the text search.
	Body, Title, Both, LabelsOnly int
	// Meaning counts the pages found by meaning or through a story that the
	// text half did not find: they are not label-only pages.
	Meaning int
	// Capped: a candidate bound was hit (the index's or the window's).
	Capped bool
	// Dropped are the query words the text search did not look for (pruned
	// for weight, or capped), named to the model in the note and the result.
	Dropped []string
	// VectorAnswered: corpusd's vector half answered; VectorRows is the rows
	// it returned and BM25Kept how many of BM25's top three pages had to be
	// inserted. Stories are the stories it walked.
	VectorAnswered bool
	VectorRows     int
	// VectorState is exact, approximate or partial when the vector half
	// answered (see platform.CaptureRanking.VectorState).
	VectorState string
	BM25Kept    int
	Stories     []CorpusStory
}

// searchCaptures is the title-and-label search alone (the words kept by
// corpusQueryTerms), as before the text index.
func searchCaptures(ctx context.Context, deps Dependencies, ownerID, computerID string, terms []string, limit int) ([]CorpusCapture, string, bool, error) {
	out, note, info, err := searchCapturesFor(ctx, deps, ownerID, computerID, corpusRequest{Terms: terms}, limit)
	return out, note, info.Capped, err
}

// searchCapturesFor finds the computer's captures for a request. Two halves
// answer and are merged:
//
//   - the text half, in corpusd: BM25 over the passages of every indexed
//     page's text, title words counting three times, every query word
//     counting (no word gate), each hit with its best passage;
//   - the title and label half, as it always was: the capture index's term
//     table, or the window of the newest captures, matched by this file's
//     matcher against title, outlet, language and Jev's labels.
//
// The text half is corpusd's one ranked answer (platform.CaptureRanking):
// vector order with BM25's top three pages guaranteed a place, or BM25's
// pages alone when the vectors could not answer. The guest does not merge
// it with anything: pages the title and label half found that the answer
// does not hold are listed after it, in their own order, up to the limit
// (so a page not yet indexed by text is still found while the index catches
// up), and a page in both gets the label half's evidence added. A note says
// which half answered and how much the text index covers.
func searchCapturesFor(ctx context.Context, deps Dependencies, ownerID, computerID string, req corpusRequest, limit int) ([]CorpusCapture, string, corpusSearchInfo, error) {
	var info corpusSearchInfo
	if deps.Captures == nil {
		return nil, "captures not searched: the shared capture store is not configured", info, nil
	}
	if strings.TrimSpace(computerID) == "" {
		return nil, "captures not searched: the run names no computer", info, nil
	}
	terms := req.Terms
	query := strings.TrimSpace(req.Query)
	if len(terms) == 0 && query == "" {
		return nil, "captures not searched: the query has no search words", info, nil
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
	var answer CaptureSearchAnswer
	searched := false
	if searcher, ok := deps.Captures.(CaptureSearcher); ok && corpusIndexEnabled() {
		q := CaptureSearchQuery{OwnerID: ownerID, ComputerID: computerID, Words: terms,
			Need: corpusNeeded(len(terms)), Candidates: corpusIndexCandidates, IDs: labelMatchedCaptureIDs(labels, terms)}
		if query != "" {
			q.Query, q.Order, q.Limit, q.Size = query, req.Order, 2*limit, limit
			if !req.Since.IsZero() {
				q.Since = req.Since.UTC().Format(time.RFC3339)
			}
			if !req.Until.IsZero() {
				q.Until = req.Until.UTC().Format(time.RFC3339)
			}
		}
		got, err := searcher.SearchCaptures(ctx, q)
		if err == nil {
			answer, objs, searched = got, got.Objects, !got.TermsIncomplete
			if got.Capped {
				info.Capped = true
				notes = append(notes, fmt.Sprintf("searched all %d captures of this computer; ranked the strongest, newest %d of the %d that match enough words", got.Searched, corpusIndexCandidates, got.Matched))
			}
		} else {
			notes = append(notes, "capture index not used ("+err.Error()+")")
		}
	}
	if !searched {
		window, err := deps.Captures.ListCaptures(ctx, ownerID, computerID, corpusCaptureScan)
		if err != nil {
			return nil, "", info, err
		}
		objs = window
		if len(objs) >= corpusCaptureScan {
			info.Capped = true
			notes = append(notes, fmt.Sprintf("searched this computer's newest %d captures; older captures were not searched", corpusCaptureScan))
		}
	}
	objs = dropOwnerPrivate(inWindow(objs, req.Since, req.Until))
	var labelHalf []CorpusCapture
	if len(terms) > 0 {
		labelHalf = rankCaptures(objs, labels, terms, corpusMaxLimit)
	}
	var textHalf []CorpusCapture
	if ft := answer.FullText; ft != nil {
		switch {
		case ft.Used:
			info.TextUsed, info.TextIndexed, info.Captures = true, ft.Indexed, ft.Captures
			textHalf = textHitCaptures(answer, labels)
			if ft.Capped {
				info.Capped = true
				notes = append(notes, "the page-text search ranked only its best 300 passages (at most 3 per page); more passages matched")
			}
			if strings.EqualFold(req.Order, "recent") {
				notes = append(notes, "recent order lists, newest first, only the pages at least half as relevant as the best match")
			}
			info.Dropped = append(append([]string(nil), ft.Pruned...), ft.TermCapped...)
			if len(info.Dropped) > 0 {
				notes = append(notes, "words not searched in page text (found in no page, or so common they carry no weight): "+strings.Join(info.Dropped, ", "))
			}
			if len(ft.Excluded) > 0 {
				notes = append(notes, "excluded from page text: "+strings.Join(ft.Excluded, ", ")+" (a passage holding one is not returned)")
			}
			if len(ft.PhraseFallbacks) > 0 {
				notes = append(notes, "no page holds the phrase "+strings.Join(ft.PhraseFallbacks, ", ")+"; its words were searched instead")
			}
			if ft.Indexed < int64(ft.Captures) {
				notes = append(notes, fmt.Sprintf("the page text of %d of this computer's %d captures is searchable so far; the rest are found by title and labels only", ft.Indexed, ft.Captures))
			}
		case ft.Reason != "":
			notes = append(notes, "page text not searched ("+ft.Reason+")")
		}
	}
	primary := textHalf
	if rk := answer.Ranked; rk != nil {
		primary = rankedCaptures(answer, labels)
		info.VectorAnswered, info.VectorRows, info.VectorState, info.BM25Kept = rk.VectorAnswered, rk.VectorRows, rk.VectorState, rk.BM25Kept
		info.Stories = rankedStories(rk, primary)
		if !rk.VectorAnswered && rk.VectorUnavailable != "" {
			notes = append(notes, "search by meaning did not answer ("+rk.VectorUnavailable+"); the pages are those whose text matches")
		}
		if rk.VectorState == "partial" {
			notes = append(notes, "search by meaning is partial ("+rk.VectorPartial+"): the pages found by meaning may lack nearer ones")
		} else if rk.VectorState == "approximate" {
			notes = append(notes, "search by meaning is approximate: the pages found by meaning are the index's nearest, not exactly the nearest")
		}
		if rk.Note != "" {
			notes = append(notes, rk.Note)
		}
	}
	merged := appendLabelHalf(primary, labelHalf, limit, req.Order)
	textFound := map[string][]string{}
	for _, c := range textHalf {
		textFound[c.CaptureID] = c.Matched
	}
	for _, c := range merged {
		if matched, ok := textFound[c.CaptureID]; ok {
			title, body := containsString(matched, "title"), containsString(matched, "body")
			switch {
			case title && body:
				info.Both++
			case title:
				info.Title++
			default:
				info.Body++
			}
		} else if len(c.Matched) > 0 {
			// The label half matched the page (title, outlet, topic, language) and the
			// vector half returned it too: it matched the words.
			info.LabelsOnly++
		} else if containsString(c.Reasons, "meaning") || containsString(c.Reasons, "story") {
			info.Meaning++
		} else {
			info.LabelsOnly++
		}
	}
	return merged, strings.Join(notes, "; "), info, nil
}

// inWindow keeps the captures whose time (published when known, else
// fetched) is in [since, until). A capture with no readable time is kept
// only when there is no window.
func inWindow(objs []objectgraph.Object, since, until time.Time) []objectgraph.Object {
	if since.IsZero() && until.IsZero() {
		return objs
	}
	var out []objectgraph.Object
	for _, obj := range objs {
		meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
		if err != nil {
			continue
		}
		at, ok := captureTime(meta)
		if !ok || (!since.IsZero() && at.Before(since)) || (!until.IsZero() && !at.Before(until)) {
			continue
		}
		out = append(out, obj)
	}
	return out
}

// captureTime is a capture's time: published when known, else fetched.
func captureTime(meta objectgraph.WebCaptureMetadata) (time.Time, bool) {
	for _, raw := range []string{meta.PublishedAt, meta.FetchedAt} {
		if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// textHitCaptures turns corpusd's text hits into results, best first.
func textHitCaptures(answer CaptureSearchAnswer, labels map[string]store.CaptureLabel) []CorpusCapture {
	byID := map[string]objectgraph.Object{}
	for _, obj := range answer.HitObjects {
		byID[obj.CanonicalID] = obj
	}
	var out []CorpusCapture
	for _, h := range answer.Hits {
		obj, ok := byID[h.CaptureID]
		if !ok {
			continue
		}
		if types.LabelsFromMetadata(obj.Metadata).OwnerPrivate() {
			continue
		}
		meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
		if err != nil {
			continue
		}
		c := newCorpusCapture(obj.CanonicalID, meta, labels[obj.CanonicalID])
		if h.TitleMatch {
			c.Matched = append(c.Matched, "title")
		}
		if h.BodyMatch || !h.TitleMatch {
			c.Matched = append(c.Matched, "body")
		}
		c.Passage, c.PassageOrdinal, c.PassageCount = h.Snippet, h.Ordinal+1, h.Passages
		out = append(out, c)
	}
	return out
}

// rankedCaptures turns corpusd's ranked answer into results, in its order.
// A page found by text carries its passage; one found only by meaning or
// through a story has none.
func rankedCaptures(answer CaptureSearchAnswer, labels map[string]store.CaptureLabel) []CorpusCapture {
	byID := map[string]objectgraph.Object{}
	for _, obj := range answer.HitObjects {
		byID[obj.CanonicalID] = obj
	}
	hits := map[string]CaptureTextHit{}
	for _, h := range answer.Hits {
		hits[h.CaptureID] = h
	}
	out := []CorpusCapture{}
	for _, e := range answer.Ranked.Entries {
		obj, ok := byID[e.CaptureID]
		if !ok || types.LabelsFromMetadata(obj.Metadata).OwnerPrivate() {
			continue
		}
		meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
		if err != nil {
			continue
		}
		c := newCorpusCapture(obj.CanonicalID, meta, labels[obj.CanonicalID])
		if h, ok := hits[e.CaptureID]; ok {
			if h.TitleMatch {
				c.Matched = append(c.Matched, "title")
			}
			if h.BodyMatch || !h.TitleMatch {
				c.Matched = append(c.Matched, "body")
			}
			c.Passage, c.PassageOrdinal, c.PassageCount = h.Snippet, h.Ordinal+1, h.Passages
		}
		c.Reasons = append([]string(nil), e.Reasons...)
		c.ViaStories = append([]string(nil), e.Stories...)
		out = append(out, c)
	}
	return out
}

// rankedStories are the walked stories whose captures are listed, with the
// listed capture ids only.
func rankedStories(rk *CaptureRanking, listed []CorpusCapture) []CorpusStory {
	have := map[string]bool{}
	for _, c := range listed {
		have[c.CaptureID] = true
	}
	var out []CorpusStory
	for _, s := range rk.Stories {
		cs := CorpusStory{PublicationID: s.PublicationID, Title: s.Title, Revision: s.Revision, Day: s.Day, CaptureIDs: []string{}}
		for _, id := range s.Captures {
			if have[id] {
				cs.CaptureIDs = append(cs.CaptureIDs, id)
			}
		}
		if len(cs.CaptureIDs) > 0 {
			out = append(out, cs)
		}
	}
	return out
}

// appendLabelHalf lists the primary answer as it is, adds the label half's
// evidence to a page both hold, and lists the label half's other pages after
// the answer, in their own order, up to limit. It ranks nothing together.
func appendLabelHalf(primary, labelHalf []CorpusCapture, limit int, order string) []CorpusCapture {
	if strings.EqualFold(order, "recent") {
		// The tail is chosen newest first; the whole list is then put in one time order.
		labelHalf = append([]CorpusCapture(nil), labelHalf...)
		sort.SliceStable(labelHalf, func(i, j int) bool {
			a, _ := time.Parse(time.RFC3339Nano, firstNonEmptyString(labelHalf[i].PublishedAt, labelHalf[i].FetchedAt))
			b, _ := time.Parse(time.RFC3339Nano, firstNonEmptyString(labelHalf[j].PublishedAt, labelHalf[j].FetchedAt))
			return a.After(b)
		})
	}
	labelByID := map[string]CorpusCapture{}
	for _, c := range labelHalf {
		labelByID[c.CaptureID] = c
	}
	out := make([]CorpusCapture, 0, limit)
	seen := map[string]bool{}
	for _, c := range primary {
		if len(out) == limit {
			break
		}
		seen[c.CaptureID] = true
		if l, ok := labelByID[c.CaptureID]; ok {
			for _, m := range l.Matched {
				if !containsString(c.Matched, m) {
					c.Matched = append(c.Matched, m)
				}
			}
		}
		out = append(out, c)
	}
	for _, c := range labelHalf {
		if len(out) == limit {
			break
		}
		if !seen[c.CaptureID] {
			seen[c.CaptureID] = true
			out = append(out, c)
		}
	}
	if strings.EqualFold(order, "recent") {
		// One time order under recent (the rows are the same; only their order).
		sort.SliceStable(out, func(i, j int) bool {
			a, _ := time.Parse(time.RFC3339Nano, firstNonEmptyString(out[i].PublishedAt, out[i].FetchedAt))
			b, _ := time.Parse(time.RFC3339Nano, firstNonEmptyString(out[j].PublishedAt, out[j].FetchedAt))
			return a.After(b)
		})
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// newCorpusCapture is a capture's result row before any matching.
func newCorpusCapture(id string, meta objectgraph.WebCaptureMetadata, label store.CaptureLabel) CorpusCapture {
	return CorpusCapture{CaptureID: id, Title: firstNonEmptyString(meta.Title, label.Title), URL: meta.URL,
		Outlet: firstNonEmptyString(label.Outlet, corpusHost(meta.URL)), SourceID: meta.SourceID, Language: firstNonEmptyString(meta.Language, label.Language),
		Script: label.Script, Topic: firstNonEmptyString(label.EventHeadline, label.TopicHeadline), FetchedAt: meta.FetchedAt,
		PublishedAt: meta.PublishedAt, BlockCount: meta.BlockCount, Matched: []string{}}
}

// corpusIndexCandidates bounds the candidates corpusd reads and returns for
// the guest's ranking: four times the most a search lists (corpusMaxLimit), a
// margin because the guest's order inside one word count (field weights, Jev's
// labels) is not corpusd's (newest first). Before it was 500, and with up to
// 2,000 label hints a search of a common word read about 1,500 rows and took
// 8 to 14 seconds on the paper (docs/problems/corpus-search-takes-two-seconds-
// on-the-paper-2026-11-19.md). corpusd says how many matched in all (the
// note says so when it cut).
const corpusIndexCandidates = 4 * corpusMaxLimit

// corpusIDHints bounds the captures sent by id because a label holds a query
// word: the ones whose labels hold the most of the words.
const corpusIDHints = corpusMaxLimit * 4

// labelMatchedCaptureIDs are the captures whose labels hold a query word:
// a capture can reach the needed words through its label alone, and the
// index holds only the capture's own fields, so these are sent by id: at
// most corpusIDHints of them, those whose labels hold the most query words
// (then by id), however many labels hold one.
func labelMatchedCaptureIDs(labels map[string]store.CaptureLabel, terms []string) []string {
	type hint struct {
		id    string
		words int
	}
	var hints []hint
	for id, label := range labels {
		text := strings.ToLower(strings.Join([]string{label.Title, label.EventHeadline, label.TopicHeadline, label.Outlet, label.Language, label.Script}, "\n"))
		n := 0
		for _, term := range terms {
			if corpusTermIn(text, term) {
				n++
			}
		}
		if n > 0 {
			hints = append(hints, hint{id, n})
		}
	}
	sort.Slice(hints, func(i, j int) bool {
		if hints[i].words != hints[j].words {
			return hints[i].words > hints[j].words
		}
		return hints[i].id > hints[j].id
	})
	if len(hints) > corpusIDHints {
		hints = hints[:corpusIDHints]
	}
	out := make([]string, len(hints))
	for i, h := range hints {
		out[i] = h.id
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
		c := newCorpusCapture(obj.CanonicalID, meta, label)
		c.Matched = nil
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

// corpusWindow is a search's time bound and order.
type corpusWindow struct {
	Since, Until time.Time
	Order        string
}

// parseCorpusWindow reads the window and order from the tool's fields or
// from operators in the query text (since:2026-11-10 until:2026-11-18
// order:recent), which a cell reaches when it passes only a query. The
// operators are removed from the query. An unreadable date is an error the
// model can correct.
func parseCorpusWindow(query, since, until, order string) (string, corpusWindow, error) {
	var w corpusWindow
	var kept []string
	for _, word := range strings.Fields(query) {
		lower := strings.ToLower(word)
		switch {
		// An operator has a value; "order:" alone is the word order.
		case strings.HasPrefix(lower, "since:") && len(word) > len("since:"):
			if since == "" {
				since = word[len("since:"):]
			}
		case strings.HasPrefix(lower, "until:") && len(word) > len("until:"):
			if until == "" {
				until = word[len("until:"):]
			}
		case strings.HasPrefix(lower, "order:") && len(word) > len("order:"):
			if order == "" {
				order = word[len("order:"):]
			}
		default:
			kept = append(kept, word)
		}
	}
	parse := func(name, raw string) (time.Time, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return time.Time{}, nil
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
			if t, err := time.Parse(layout, raw); err == nil {
				return t.UTC(), nil
			}
		}
		return time.Time{}, fmt.Errorf("%s %q is not a date: use 2026-11-18 or 2026-11-18T09:30:00Z", name, raw)
	}
	var err error
	if w.Since, err = parse("since", since); err != nil {
		return "", w, err
	}
	if w.Until, err = parse("until", until); err != nil {
		return "", w, err
	}
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "", "relevance":
		w.Order = "relevance"
	case "recent", "newest":
		w.Order = "recent"
	default:
		return "", w, fmt.Errorf("order %q is not relevance or recent", order)
	}
	return strings.Join(kept, " "), w, nil
}

// CorpusResultCarriesPassage reads a search_wire_corpus result for the host's
// labelling: carries is true when some capture in it holds a non-empty
// passage (verbatim page text), and understood is false when the result is
// not a JSON object with a "captures" list. It parses the same fields the
// tool writes (wireCorpusResult and CorpusCapture), so a renamed field breaks
// the test that marshals the real struct, and the host fails closed on what it
// does not understand.
func CorpusResultCarriesPassage(result []byte) (carries, understood bool) {
	var body struct {
		Captures json.RawMessage `json:"captures"`
	}
	if json.Unmarshal(result, &body) != nil || len(body.Captures) == 0 {
		return false, false
	}
	var list []struct {
		Passage string `json:"passage"`
	}
	if string(body.Captures) != "null" && json.Unmarshal(body.Captures, &list) != nil {
		return false, false
	}
	for _, c := range list {
		if strings.TrimSpace(c.Passage) != "" {
			return true, true
		}
	}
	return false, true
}

// dropOwnerPrivate removes the captures whose record carries an owner-private
// label. The search hides them from every caller, the research desk included:
// a passage or a title of one would leave through a result labelled with the
// capture floor, and ReadCapture's gate refuses its body to a non-custodian.
func dropOwnerPrivate(objs []objectgraph.Object) []objectgraph.Object {
	var out []objectgraph.Object
	for _, obj := range objs {
		if types.LabelsFromMetadata(obj.Metadata).OwnerPrivate() {
			continue
		}
		out = append(out, obj)
	}
	return out
}
