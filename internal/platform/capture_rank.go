package platform

// The one ranked corpus-search answer (station V4 of
// docs/archive/definitions/choir-night-exact-vectors-learning-2026-11-19.md; the rule
// is the measured one of docs/design/corpus-first-retrieval-and-research-2026-11-18.md
// "Fusion and windows").
//
// For a query corpusd makes one answer, in this order:
//
//  1. the vector half: the query is embedded once, and the nearest story
//     passages and the nearest capture leads are merged by cosine score (a
//     capture before a story on a tie). A story counts once, by its best
//     passage, and only when the passage is of the story's latest version; it
//     is replaced in the order by the captures its latest version cites (the
//     cites_capture edges, at most rankStoryCitedMax a story, read in one
//     query for all the stories at once), marked as reached through the story;
//  2. BM25's top three pages (the full-page text search, already run) are
//     guaranteed a place: a page of the three that is not within the first
//     Size pages of that order is inserted at its BM25 rank (first, second or
//     third), the vector pages below move down, and the pages that no longer
//     fit are dropped from the bottom, never one of BM25's three (placeBM25);
//  3. a capture that arrives more than one way is listed once, at its best
//     rank, with every reason (text, meaning, story).
//
// Reciprocal-rank fusion measured 0.856 recall at ten against 0.926 for this
// and is not used. Whatever the vector half cannot do, the answer is BM25's
// pages in BM25's order and says so (CaptureRanking.VectorUnavailable): a
// search for meaning never fails the search. The vector half has one
// deadline (captureVectorDeadline), which is also the statement deadline of
// its SQL; the answer does not wait past it. Which story passages and capture
// leads the vector half may return is decided by V2's one eligibility
// predicate in its own SQL (a withdrawn, tombstoned, private or superseded row
// is not returned); a story is walked only through the version that passed it.
//
// Every capture in the answer passes the same checks whichever way it came:
// it is of the request's owner and computer, public (not owner-private, not
// tombstoned, not superseded), and in the time window (published time, else
// fetched time, else outside any window; since inclusive, until exclusive). A story outside the window
// is not walked.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	// rankStoryCitedMax is how many cited captures one story carries into an
	// answer (the first in capture-id order).
	rankStoryCitedMax = 8
	// rankStoryCitedRead is how many cited captures of a story are read: the
	// cap rankStoryCitedMax is applied after the eligibility checks, so a story
	// whose first citations are private or of another scope still contributes.
	rankStoryCitedRead = 4 * rankStoryCitedMax
	// rankBM25Guaranteed is how many of BM25's best pages are guaranteed a place.
	rankBM25Guaranteed = 3
	rankDefaultSize    = 10
	rankMaxSize        = 25
	// rankVectorAsk is how many vector rows of each kind are asked for each
	// slot of the answer (at most vectorSearchMaxLimit): the vector half
	// over-asks because the latest-version, privacy and window checks drop rows.
	rankVectorAsk = 4
	// rankGather is how many candidate pages are gathered, per slot, before
	// the checks and the cut.
	rankGather = 3
)

// captureVectorDeadline is how long the answer waits for the vector half (the
// query's embedding, which can queue behind one worker batch of four passages,
// and the nearest-neighbour scans). A deadline, not a limit on calls: past it
// the answer is BM25's and says so.
var captureVectorDeadline = 4 * time.Second

// Reasons a page is in the answer.
const (
	RankReasonText    = "text"
	RankReasonMeaning = "meaning"
	RankReasonStory   = "story"
)

var rankReasonOrder = map[string]int{RankReasonText: 0, RankReasonMeaning: 1, RankReasonStory: 2}

// RankedCapture is one page of the answer.
type RankedCapture struct {
	CaptureID string   `json:"capture_id"`
	Reasons   []string `json:"reasons"`
	// Stories are the publications whose latest version cites this page, among
	// the stories the vector half found.
	Stories []string `json:"stories,omitempty"`
}

// RankedStory is a story the vector half found, with the captures its latest
// version cites that passed the checks.
type RankedStory struct {
	PublicationID string   `json:"publication_id"`
	VersionID     string   `json:"version_id"`
	Revision      int      `json:"revision"`
	Title         string   `json:"title,omitempty"`
	Score         float64  `json:"score"`
	Day           string   `json:"day"`
	Captures      []string `json:"captures"`
}

// CaptureRanking is the one ranked answer. Entries is the answer, best first,
// at most Size long.
type CaptureRanking struct {
	Entries []RankedCapture `json:"entries"`
	Stories []RankedStory   `json:"stories,omitempty"`
	// VectorAnswered is true when the vector half answered (even with no
	// rows); VectorRows is how many rows it returned (story passages and
	// captures). When it did not answer, VectorUnavailable says why and the
	// answer is BM25's.
	VectorAnswered    bool   `json:"vector_answered"`
	VectorRows        int    `json:"vector_rows"`
	VectorUnavailable string `json:"vector_unavailable,omitempty"`
	// VectorState says how the vector lists were answered: "exact" (every
	// list is the true nearest of its filtered rows: at most 100,000 rows, or
	// the index came back short and was read again exactly), "approximate"
	// (a list is the HNSW index's answer on a larger list) or "partial" (an
	// exact read passed its deadline; VectorPartial says which). Empty when
	// the vector half did not answer.
	VectorState   string `json:"vector_state,omitempty"`
	VectorPartial string `json:"vector_partial,omitempty"`
	// BM25Kept is how many of BM25's top three pages had to be inserted.
	BM25Kept int `json:"bm25_kept"`
	// Note names a part of the vector half that failed while the rest answered.
	Note string `json:"note,omitempty"`
}

func rankSize(n int) int {
	switch {
	case n <= 0:
		return rankDefaultSize
	case n > rankMaxSize:
		return rankMaxSize
	}
	return n
}

// placeBM25 returns the first size of vec (page ids in vector order) with the
// first min(3, size) of bm25 guaranteed a place, and how many of them had to
// be inserted. An absent page of BM25's is inserted at index i (its BM25 rank
// minus one); then pages are dropped from the bottom, never one of BM25's.
func placeBM25(vec, bm25 []string, size int) ([]string, int) {
	if size <= 0 {
		return nil, 0
	}
	var want []string
	seenWant := map[string]bool{}
	for _, id := range bm25 {
		if seenWant[id] {
			continue
		}
		seenWant[id] = true
		want = append(want, id)
		if len(want) == min(rankBM25Guaranteed, size) {
			break
		}
	}
	out := append([]string(nil), vec[:min(size, len(vec))]...)
	in := map[string]bool{}
	for _, id := range out {
		in[id] = true
	}
	kept := 0
	for i, id := range want {
		if in[id] {
			continue
		}
		at := min(i, len(out))
		out = append(out, "")
		copy(out[at+1:], out[at:])
		out[at] = id
		in[id] = true
		kept++
	}
	for len(out) > size {
		cut := -1
		for j := len(out) - 1; j >= 0; j-- {
			if !containsID(want, out[j]) {
				cut = j
				break
			}
		}
		if cut < 0 {
			out = out[:size]
			break
		}
		out = append(out[:cut], out[cut+1:]...)
	}
	return out, kept
}

func containsID(list []string, id string) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

// dayInWindow reports whether a day (YYYY-MM-DD, UTC) can hold a moment in
// [since, until): a day is in unless it ends at or before since or starts at
// or after until.
func dayInWindow(day string, since, until time.Time) bool {
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		return true
	}
	if !since.IsZero() && !d.Add(24*time.Hour).After(since) {
		return false
	}
	if !until.IsZero() && !d.Before(until) {
		return false
	}
	return true
}

// captureWindowTime is the one time a window judges a capture by: published,
// else fetched, else none (a capture with neither is outside every window).
// The term half's SQL (COALESCE(published_at, fetched_at)), the page-text
// index (its effective time, 0 for none) and the guest's inWindow
// (researchtools captureTime) are the same rule.
func captureWindowTime(obj objectgraph.Object) (time.Time, bool) {
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		return time.Time{}, false
	}
	if t := parseCaptureTime(meta.PublishedAt); t.Valid {
		return t.Time.UTC(), true
	}
	if t := parseCaptureTime(meta.FetchedAt); t.Valid {
		return t.Time.UTC(), true
	}
	return time.Time{}, false
}

// captureInWindow reports whether a capture is in [since, until) by its window
// time; with no window every capture is. A capture with no readable time is in
// no window (its update time never stands in).
func captureInWindow(obj objectgraph.Object, since, until time.Time) bool {
	if since.IsZero() && until.IsZero() {
		return true
	}
	at, ok := captureWindowTime(obj)
	return ok && timeInWindow(at, since, until)
}

func timeInWindow(at, since, until time.Time) bool {
	if !since.IsZero() && at.Before(since) {
		return false
	}
	if !until.IsZero() && !at.Before(until) {
		return false
	}
	return true
}

// searchVectorsBounded runs the vector half under captureVectorDeadline. It
// returns the reason it could not answer, or "" with the result. It stops
// waiting at the deadline even for an embedder that ignores its context.
func (s *Store) searchVectorsBounded(ctx context.Context, req VectorSearchRequest) (VectorSearchResult, string) {
	vctx, cancel := context.WithTimeout(ctx, captureVectorDeadline)
	defer cancel()
	type answer struct {
		res VectorSearchResult
		err error
	}
	ch := make(chan answer, 1)
	go func() {
		res, err := s.SearchVectors(vctx, req)
		ch <- answer{res, err}
	}()
	// why says what ended the wait: the caller (cancelled, or its own deadline
	// passed) or this half's deadline.
	ended := func() string {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return "the caller's deadline passed before the vector half answered"
			}
			return "the caller cancelled before the vector half answered"
		}
		return fmt.Sprintf("the vector half did not answer within %s", captureVectorDeadline)
	}
	select {
	case a := <-ch:
		switch {
		case a.err != nil && (ctx.Err() != nil || vctx.Err() != nil):
			return VectorSearchResult{}, ended()
		case a.err != nil:
			return VectorSearchResult{}, "the vector search failed: " + a.err.Error()
		case a.res.Unavailable != "":
			return VectorSearchResult{}, a.res.Unavailable
		}
		return a.res, ""
	case <-vctx.Done():
		return VectorSearchResult{}, ended()
	}
}

// vectorStateOf says how a vector result was answered: partial when any list
// could not be completed, else approximate when any list is the index's answer
// on a larger list, else exact.
func vectorStateOf(vr VectorSearchResult) (state, partial string) {
	switch {
	case vr.Partial != "":
		return "partial", vr.Partial
	case len(vr.Approximate) > 0:
		return "approximate", ""
	}
	return "exact", ""
}

type rankCand struct {
	id      string
	reasons map[string]bool
	stories []string
	// at is the slot (rank in the vector order) its own vector reached the
	// page at; storyAt the slot of each story that cites it. The page stands
	// at the best of what it keeps: its own vector and the stories that kept it.
	at      map[string]int
	storyAt map[string]int
}

func (c *rankCand) add(reason string) { c.reasons[reason] = true }

func (c *rankCand) reasonList() []string {
	out := make([]string, 0, len(c.reasons))
	for r := range c.reasons {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return rankReasonOrder[out[i]] < rankReasonOrder[out[j]] })
	return out
}

type storyInfo struct {
	RankedStory
	ok bool
}

// rankCaptureAnswer fills resp.Ranked and adds the objects of the answer's
// pages to resp.HitObjects. It never returns an error: what it cannot do it
// says in resp.Ranked.
func (s *Store) rankCaptureAnswer(ctx context.Context, req CaptureSearchRequest, resp *CaptureSearchResponse) {
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	size := rankSize(req.Size)
	done := s.retrievalTimerExtra(retrievalCaptureRank, retrievalCaptureSearch)
	ranking := &CaptureRanking{Entries: []RankedCapture{}}
	resp.Ranked = ranking
	defer func() {
		extra := " vector=unavailable vector_state=unavailable bm25_kept=0"
		if ranking.VectorAnswered {
			extra = fmt.Sprintf(" vector=%d vector_state=%s bm25_kept=%d", ranking.VectorRows, ranking.VectorState, ranking.BM25Kept)
		}
		done("ranked", len(ranking.Entries), nil, extra)
	}()

	var bm25 []string
	bm25Set := map[string]bool{}
	for _, h := range resp.Hits {
		if !bm25Set[h.CaptureID] {
			bm25Set[h.CaptureID] = true
			bm25 = append(bm25, h.CaptureID)
		}
	}
	bm25Only := func(reason string) {
		ranking.VectorAnswered, ranking.VectorUnavailable = false, reason
		for i, id := range bm25 {
			if i == size {
				break
			}
			ranking.Entries = append(ranking.Entries, RankedCapture{CaptureID: id, Reasons: []string{RankReasonText}})
		}
	}
	since, _ := parseSearchTime("since", req.Since)
	until, _ := parseSearchTime("until", req.Until)
	vecUntil := until
	if !until.IsZero() && !until.Equal(until.Truncate(24*time.Hour)) {
		vecUntil = until.Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	ask := min(size*rankVectorAsk, vectorSearchMaxLimit)
	vr, why := s.searchVectorsBounded(ctx, VectorSearchRequest{Query: req.Query, Since: since, Until: vecUntil,
		Owner: owner, Computer: computer, StoryLimit: ask, CaptureLimit: ask})
	if why != "" {
		bm25Only(why)
		return
	}
	if rows := len(vr.Stories) + len(vr.Captures); rows == 0 && vr.Partial != "" && len(vr.ExactScan) == 0 {
		// No list gave a row, the result is partial and no list was read
		// exactly: an empty list that may lack nearer rows is not an answer.
		// (An empty list read exactly is complete and is an answer.)
		bm25Only("the vector lists could not be read: " + vr.Partial)
		return
	}
	ranking.VectorAnswered = true
	ranking.VectorRows = len(vr.Stories) + len(vr.Captures)
	ranking.VectorState, ranking.VectorPartial = vectorStateOf(vr)

	// Stories: the latest version of each publication, its title, its citations.
	storyHits := vr.Stories
	var pubs []string
	seenPub := map[string]bool{}
	for _, h := range storyHits {
		if !seenPub[h.PublicationID] {
			seenPub[h.PublicationID] = true
			pubs = append(pubs, h.PublicationID)
		}
	}
	stories, cited, err := s.storyWalk(ctx, pubs)
	if err != nil {
		ranking.Note = "the stories were not walked: " + err.Error()
		log.Printf("corpusd capture rank computer=%s story walk: %v", computer, err)
		stories, cited = map[string]*storyInfo{}, map[string][]string{}
	}

	type slot struct {
		score   float64
		capture *VectorCaptureHit
		story   *VectorStoryHit
	}
	var slots []slot
	storySeen := map[string]bool{}
	usedHit := map[string]VectorStoryHit{}
	for i := range vr.Captures {
		slots = append(slots, slot{score: vr.Captures[i].Score, capture: &vr.Captures[i]})
	}
	for i := range storyHits {
		h := &storyHits[i]
		st := stories[h.PublicationID]
		if st == nil || !st.ok || st.VersionID != h.VersionID || storySeen[h.PublicationID] || !dayInWindow(h.Day, since, until) {
			continue
		}
		storySeen[h.PublicationID] = true
		usedHit[h.PublicationID] = *h
		slots = append(slots, slot{score: h.Score, story: h})
	}
	sort.SliceStable(slots, func(i, j int) bool { return slots[i].score > slots[j].score })

	cands := map[string]*rankCand{}
	var order []string
	touch := func(id, reason string, slotIdx int) *rankCand {
		c := cands[id]
		if c == nil {
			c = &rankCand{id: id, reasons: map[string]bool{}, at: map[string]int{}, storyAt: map[string]int{}}
			cands[id] = c
			order = append(order, id)
		}
		c.add(reason)
		if _, ok := c.at[reason]; !ok {
			c.at[reason] = slotIdx
		}
		return c
	}
	gather := size * rankGather
	for slotIdx, sl := range slots {
		if len(order) >= gather {
			break
		}
		if sl.capture != nil {
			touch(sl.capture.CaptureID, RankReasonMeaning, slotIdx)
			continue
		}
		st := stories[sl.story.PublicationID]
		for _, id := range cited[st.VersionID] {
			c := touch(id, RankReasonStory, slotIdx)
			if !containsID(c.stories, st.PublicationID) {
				c.stories = append(c.stories, st.PublicationID)
				c.storyAt[st.PublicationID] = slotIdx
			}
		}
	}
	for _, id := range order {
		if bm25Set[id] {
			cands[id].add(RankReasonText)
		}
	}

	// The checks, on the capture objects read under this owner and computer.
	objs, err := s.captureObjectsByID(ctx, owner, computer, order)
	if err != nil {
		bm25Only("the vector pages could not be read: " + err.Error())
		ranking.VectorAnswered, ranking.VectorRows = false, 0
		return
	}
	byID := map[string]objectgraph.Object{}
	for _, o := range resp.HitObjects {
		byID[o.CanonicalID] = o
	}
	var vecOrder []string
	for _, o := range objs {
		byID[o.CanonicalID] = o
	}
	for _, id := range order {
		o, ok := byID[id]
		if !ok {
			continue
		}
		if _, _, public := captureVectorSource(o); !public {
			continue
		}
		if !captureInWindow(o, since, until) {
			continue
		}
		vecOrder = append(vecOrder, id)
	}

	// The cap on a story's citations, after the checks: its first
	// rankStoryCitedMax eligible citations stay; a page reached only through a
	// citation beyond them is dropped.
	eligible := map[string]bool{}
	for _, id := range vecOrder {
		eligible[id] = true
	}
	allowed := map[string][]string{}
	for _, p := range pubs {
		st := stories[p]
		if st == nil || !st.ok {
			continue
		}
		for _, id := range cited[st.VersionID] {
			if eligible[id] && len(allowed[p]) < rankStoryCitedMax {
				allowed[p] = append(allowed[p], id)
			}
		}
	}
	var kept2 []string
	for _, id := range vecOrder {
		c := cands[id]
		var stk []string
		for _, p := range c.stories {
			if containsID(allowed[p], id) {
				stk = append(stk, p)
			}
		}
		c.stories = stk
		if len(stk) == 0 {
			delete(c.reasons, RankReasonStory)
		}
		if len(c.reasons) == 0 {
			continue
		}
		kept2 = append(kept2, id)
	}
	// A page that kept only its own vector stands where that vector put it,
	// not where the story it no longer counts for stood.
	pos := func(id string) int {
		best := 1 << 30
		c := cands[id]
		if at, ok := c.at[RankReasonMeaning]; ok && c.reasons[RankReasonMeaning] {
			best = at
		}
		for _, p := range c.stories { // only the stories that kept it
			if at := c.storyAt[p]; at < best {
				best = at
			}
		}
		return best
	}
	sort.SliceStable(kept2, func(i, j int) bool { return pos(kept2[i]) < pos(kept2[j]) })
	vecOrder = kept2

	final, kept := placeBM25(vecOrder, bm25, size)
	ranking.BM25Kept = kept
	for _, id := range final {
		c := cands[id]
		if c == nil {
			c = &rankCand{id: id, reasons: map[string]bool{RankReasonText: true}}
		}
		ranking.Entries = append(ranking.Entries, RankedCapture{CaptureID: id, Reasons: c.reasonList(), Stories: c.stories})
	}
	if strings.EqualFold(strings.TrimSpace(req.Order), "recent") {
		sort.SliceStable(ranking.Entries, func(i, j int) bool {
			a, aok := byID[ranking.Entries[i].CaptureID]
			b, bok := byID[ranking.Entries[j].CaptureID]
			if !aok || !bok {
				return aok && !bok
			}
			at, aok := captureWindowTime(a)
			bt, bok := captureWindowTime(b)
			if aok != bok {
				return aok
			}
			return at.After(bt)
		})
	}

	// The stories that brought a page into the answer, with their eligible citations.
	used := map[string]bool{}
	for _, e := range ranking.Entries {
		for _, p := range e.Stories {
			used[p] = true
		}
	}
	for _, p := range pubs {
		st := stories[p]
		if st == nil || !st.ok || !used[p] {
			continue
		}
		rs := st.RankedStory
		rs.Captures = allowed[p]
		rs.Score, rs.Day = usedHit[p].Score, usedHit[p].Day
		ranking.Stories = append(ranking.Stories, rs)
	}

	have := map[string]bool{}
	for _, o := range resp.HitObjects {
		have[o.CanonicalID] = true
	}
	for _, e := range ranking.Entries {
		if o, ok := byID[e.CaptureID]; ok && !have[e.CaptureID] {
			have[e.CaptureID] = true
			resp.HitObjects = append(resp.HitObjects, o)
		}
	}
}

// storyWalk reads, for publications, each one's latest version, its title
// and the captures that version cites (at most rankStoryCitedRead each, first
// in id order; the cap of rankStoryCitedMax comes after the checks), in three queries however many publications there are. A
// withdrawn (tombstoned) or unknown publication is not ok.
func (s *Store) storyWalk(ctx context.Context, pubs []string) (map[string]*storyInfo, map[string][]string, error) {
	out := map[string]*storyInfo{}
	cited := map[string][]string{}
	if len(pubs) == 0 {
		return out, cited, nil
	}
	in := strings.TrimSuffix(strings.Repeat("?, ", len(pubs)), ", ")
	args := make([]any, len(pubs))
	for i, p := range pubs {
		args[i] = p
	}
	rows, err := s.corpus().QueryContext(ctx, `SELECT from_id, to_id, metadata FROM og_edges WHERE from_id IN (`+in+`) AND kind = 'has_version' AND tombstone = FALSE`, args...)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var from, to, meta string
		if err := rows.Scan(&from, &to, &meta); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		var m struct {
			Revision int `json:"revision"`
		}
		if json.Unmarshal([]byte(meta), &m) != nil || m.Revision <= 0 {
			continue
		}
		st := out[from]
		if st == nil {
			st = &storyInfo{}
			st.PublicationID = from
			out[from] = st
		}
		if m.Revision > st.Revision {
			st.Revision, st.VersionID = m.Revision, to
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, nil, err
	}
	rows, err = s.corpus().QueryContext(ctx, `SELECT canonical_id, metadata FROM og_objects WHERE canonical_id IN (`+in+`) AND tombstone = FALSE`, args...)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id, meta string
		if err := rows.Scan(&id, &meta); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		if st := out[id]; st != nil && st.VersionID != "" {
			var pm frontPublicationMeta
			_ = json.Unmarshal([]byte(meta), &pm)
			st.Title, st.ok = strings.TrimSpace(pm.Title), true
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, nil, err
	}
	var versions []any
	for _, st := range out {
		if st.ok {
			versions = append(versions, st.VersionID)
		}
	}
	if len(versions) == 0 {
		return out, cited, nil
	}
	vin := strings.TrimSuffix(strings.Repeat("?, ", len(versions)), ", ")
	rows, err = s.corpus().QueryContext(ctx, `SELECT from_id, to_id FROM (
		SELECT from_id, to_id, ROW_NUMBER() OVER (PARTITION BY from_id ORDER BY to_id) AS rn
		FROM og_edges WHERE from_id IN (`+vin+`) AND kind = 'cites_capture' AND tombstone = FALSE) t
		WHERE rn <= ? ORDER BY from_id, to_id`, append(versions, rankStoryCitedRead)...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var from, to string
		if err := rows.Scan(&from, &to); err != nil {
			return nil, nil, err
		}
		cited[from] = append(cited[from], to)
	}
	return out, cited, rows.Err()
}
