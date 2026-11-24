package platform

// The corpus as values for a desk's cell (owner, 2026-11-24: "we should
// definitely be able to have the whole corpus and new captures at a desks
// fingertips (repl)"; "we can run captures through vector search, then for
// each match we can do graph traversal. this is then sampled and provided with
// metadata and full data accessible in repl"). Three reads, one scope each: the
// caller's own owner and computer, the captures every other read of corpusd
// serves a desk (never an owner-private one, never a tombstoned or superseded
// one), and the published stories every guest may already search.
//
//   - page:  the computer's captures by time window and filters, newest first,
//     as metadata rows, with a cursor;
//   - near:  the nearest captures and the nearest published stories to a
//     capture, a published story or a text, with the cosine similarity;
//   - edges: the edges that exist between a capture, a published story and the
//     versions and stories around it: cites_capture, has_version and
//     related_story. Nothing else: no entity, no event, no Texture document.
//
// Bounds are on bytes and rows per call with a cursor, never on how many calls
// a desk makes. None of these makes a model call: the vectors are read from
// pgvector, and a text query is embedded by the host's query embedder, the one
// the corpus search already uses.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// corpusAnswerBytesVar bounds one answer as it is written (JSON with its
// escapes: "<" is six bytes), under the desk frame (4 MiB); a variable so a
// test can lower it.
var corpusAnswerBytesVar = 2 << 20

const (
	// corpusPageDefault and corpusPageMax are rows a page holds; corpusPageBytes
	// stops a page short of the desk frame (4 MiB) whatever the rows weigh.
	corpusPageDefault = 1000
	corpusPageMax     = 5000
	corpusRowTitle    = 240
	corpusRowURL      = 400
	// corpusNearDefault and corpusNearMax are how many captures and stories a
	// near answer holds. The vector search keeps its own bound (500).
	corpusNearDefault = 20
	corpusNearMax     = 500
	// corpusEdgeMax bounds each list of an edges answer.
	corpusEdgeMax = 500
	// Kinds of a capture row: a page a source's poll brought in, and a page a
	// research run fetched with choir.Read.
	CorpusKindSource = "source"
	CorpusKindRead   = "read"
	corpusReplBytes  = 1 << 20
)

// CorpusCapturesRequest is one page of the computer's captures. Since is
// inclusive and Until exclusive, each a day (YYYY-MM-DD) or an RFC 3339 time,
// on the time this computer fetched the page. SourceID, when set, keeps one
// source's pages; Kind keeps "source" (a source's poll) or "read" (a research
// read). Cursor is a previous page's Next.
type CorpusCapturesRequest struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	Since      string `json:"since,omitempty"`
	Until      string `json:"until,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// CorpusCaptureRow is one capture's metadata: no text.
type CorpusCaptureRow struct {
	CaptureID   string `json:"capture_id"`
	Title       string `json:"title,omitempty"`
	URL         string `json:"url"`
	Host        string `json:"host,omitempty"`
	SourceID    string `json:"source_id,omitempty"`
	Kind        string `json:"kind"`
	Language    string `json:"language,omitempty"`
	FetchedAt   string `json:"fetched_at,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	HasText     bool   `json:"has_text"`
}

// CorpusCapturesResponse is a page, newest first. Next, when set, is the
// Cursor of the next page; a page ends early (before Limit) only at the byte
// bound, and then Next is set.
type CorpusCapturesResponse struct {
	Captures []CorpusCaptureRow `json:"captures"`
	Next     string             `json:"next,omitempty"`
}

func corpusKind(sourceID string) string {
	if sourceID == sourcegraph.ReadSourceID {
		return CorpusKindRead
	}
	return CorpusKindSource
}

var errBadCorpus = errors.New("bad corpus request")

const (
	// corpusMissing is the one answer for a ref that is not a capture of this
	// computer a desk may be served, nor a public published story or version:
	// absent, foreign, private, withdrawn, unpublished and restricted look the
	// same, so an answer does not tell which exists.
	corpusMissing = "nothing in this computer's shareable corpus or the public published stories has that id"
	// corpusNoVector is the one answer for a subject whose vector is not there.
	corpusNoVector = "that capture or story has no vector to ask with: it is not in this computer's shareable corpus or the public published stories, or it is not embedded yet"
)

// jsonSize is the bytes a value takes as corpusd writes it (the encoder
// escapes <, > and & to six bytes each), and a separator.
func jsonSize(v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return len(b) + 1
}

// corpusAnswerSlack is left for an answer's fixed fields.
const corpusAnswerSlack = 1024

// fitList keeps the leading items that fit in what is left of the answer's
// byte bound and says whether it cut.
func fitList[T any](items []T, used *int) ([]T, bool) {
	for i, it := range items {
		n := jsonSize(it)
		if *used+n > corpusAnswerBytesVar {
			return items[:i], true
		}
		*used += n
	}
	return items, false
}

func corpusTime(field, v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: %s %q is not a day or a time", errBadCorpus, field, v)
}

// corpusCursor reads a page cursor: "<unix nanoseconds>|<capture id>", or
// "null|<capture id>" inside the captures with no fetch time (listed last).
func corpusCursor(c string) (at time.Time, null bool, id string, ok bool, err error) {
	c = strings.TrimSpace(c)
	if c == "" {
		return
	}
	head, id, found := strings.Cut(c, "|")
	if !found || id == "" {
		return time.Time{}, false, "", false, fmt.Errorf("%w: cursor %q is not one a page returned", errBadCorpus, c)
	}
	if head == "null" {
		return time.Time{}, true, id, true, nil
	}
	n, perr := strconv.ParseInt(head, 10, 64)
	if perr != nil {
		return time.Time{}, false, "", false, fmt.Errorf("%w: cursor %q is not one a page returned", errBadCorpus, c)
	}
	return time.Unix(0, n).UTC(), false, id, true, nil
}

// CorpusCaptures answers one page.
func (s *Store) CorpusCaptures(ctx context.Context, req CorpusCapturesRequest) (CorpusCapturesResponse, error) {
	res := CorpusCapturesResponse{Captures: []CorpusCaptureRow{}}
	if s == nil {
		return res, errors.New("corpus captures: nil store")
	}
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" || computer == "" {
		return res, fmt.Errorf("%w: owner_id and computer_id are required", errBadCorpus)
	}
	since, err := corpusTime("since", req.Since)
	if err != nil {
		return res, err
	}
	until, err := corpusTime("until", req.Until)
	if err != nil {
		return res, err
	}
	cat, cnull, cid, hasCursor, err := corpusCursor(req.Cursor)
	if err != nil {
		return res, err
	}
	limit := req.Limit
	switch {
	case limit <= 0:
		limit = corpusPageDefault
	case limit > corpusPageMax:
		limit = corpusPageMax
	}
	// The filters every statement shares.
	var base strings.Builder
	baseArgs := []any{owner, computer}
	if !since.IsZero() {
		base.WriteString(` AND v.fetched_at >= ?`)
		baseArgs = append(baseArgs, since)
	}
	if !until.IsZero() {
		base.WriteString(` AND v.fetched_at < ?`)
		baseArgs = append(baseArgs, until)
	}
	if src := strings.TrimSpace(req.SourceID); src != "" {
		base.WriteString(` AND v.source_id = ?`)
		baseArgs = append(baseArgs, src)
	}
	switch strings.TrimSpace(req.Kind) {
	case "":
	case CorpusKindSource:
		base.WriteString(` AND v.source_id <> ?`)
		baseArgs = append(baseArgs, sourcegraph.ReadSourceID)
	case CorpusKindRead:
		base.WriteString(` AND v.source_id = ?`)
		baseArgs = append(baseArgs, sourcegraph.ReadSourceID)
	default:
		return res, fmt.Errorf("%w: kind %q is not %q or %q", errBadCorpus, req.Kind, CorpusKindSource, CorpusKindRead)
	}
	type pageRow struct {
		row     CorpusCaptureRow
		at      string
		fetched *time.Time
	}
	var rows []pageRow
	run := func(extra string, extraArgs []any, order string, n int) error {
		args := append(append([]any{}, baseArgs...), extraArgs...)
		args = append(args, n)
		rs, err := s.corpus().QueryContext(ctx, `SELECT v.capture_id, v.title, v.url, v.host, v.source_id, v.language, v.has_blocks, v.fetched_at, v.published_at
			FROM `+captureIndexTable+` v WHERE v.owner_id = ? AND v.computer_id = ?`+base.String()+extra+` AND `+vecCaptureEligibleSQL+` ORDER BY `+order+` LIMIT ?`, args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var id, title, url, host, source, lang string
			var blocks bool
			var fetched, published *time.Time
			if err := rs.Scan(&id, &title, &url, &host, &source, &lang, &blocks, &fetched, &published); err != nil {
				return err
			}
			row := CorpusCaptureRow{CaptureID: id, Title: clipRunes(title, corpusRowTitle), URL: clipBytes(url, corpusRowURL), Host: host, SourceID: source,
				Kind: corpusKind(source), Language: lang, HasText: blocks}
			at := "null"
			if fetched != nil {
				row.FetchedAt = fetched.UTC().Format(time.RFC3339Nano)
				at = strconv.FormatInt(fetched.UnixNano(), 10)
			}
			if published != nil {
				row.PublishedAt = published.UTC().Format(time.RFC3339Nano)
			}
			rows = append(rows, pageRow{row: row, at: at, fetched: fetched})
		}
		return rs.Err()
	}
	// The pages with a fetch time come first, newest first, then the pages
	// with none. Each statement is a range of the index on (owner, computer,
	// fetched_at): the cursor is a bound on fetched_at (what the index
	// serves) and a row comparison on the pair (what it filters), never an OR
	// across the two.
	if !hasCursor || !cnull {
		extra, extraArgs := ` AND v.fetched_at IS NOT NULL`, []any(nil)
		if hasCursor {
			extra += ` AND v.fetched_at <= ? AND (v.fetched_at, v.capture_id) < (?, ?)`
			extraArgs = []any{cat, cat, cid}
		}
		if err := run(extra, extraArgs, `v.fetched_at DESC, v.capture_id DESC`, limit+1); err != nil {
			return res, err
		}
	}
	// A window on the fetch time holds no page that has none.
	if len(rows) < limit+1 && since.IsZero() && until.IsZero() {
		extra, extraArgs := ` AND v.fetched_at IS NULL`, []any(nil)
		if hasCursor && cnull {
			extra += ` AND v.capture_id < ?`
			extraArgs = []any{cid}
		}
		if err := run(extra, extraArgs, `v.capture_id DESC`, limit+1-len(rows)); err != nil {
			return res, err
		}
	}
	used := corpusAnswerSlack
	for i, r := range rows {
		n := jsonSize(r.row)
		if len(res.Captures) == limit || (used+n > corpusAnswerBytesVar && len(res.Captures) > 0) {
			last := rows[i-1]
			res.Next = last.at + "|" + last.row.CaptureID
			break
		}
		used += n
		res.Captures = append(res.Captures, r.row)
	}
	return res, nil
}

func clipBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}

// CorpusNearRequest names one thing to be near: a capture of this computer
// (CaptureID), a published story (StoryID, a publication id) or a text.
// Exactly one is set. K is how many captures and how many stories to return.
type CorpusNearRequest struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	CaptureID  string `json:"capture_id,omitempty"`
	StoryID    string `json:"story_id,omitempty"`
	Text       string `json:"text,omitempty"`
	K          int    `json:"k,omitempty"`
	Since      string `json:"since,omitempty"`
	Until      string `json:"until,omitempty"`
}

// CorpusNearCapture is a near capture, with its metadata and the cosine
// similarity of its vector to the query's (1 is the same direction).
type CorpusNearCapture struct {
	CorpusCaptureRow
	Score float64 `json:"score"`
}

// CorpusNearStory is a near published story: the latest version's title and
// the best similarity among its passages.
type CorpusNearStory struct {
	PublicationID string  `json:"publication_id"`
	VersionID     string  `json:"version_id"`
	Revision      int     `json:"revision"`
	Title         string  `json:"title,omitempty"`
	Day           string  `json:"day,omitempty"`
	Score         float64 `json:"score"`
}

// CorpusNearResponse holds both lists, best first. Unavailable says why the
// vectors could not answer (no embedder, tables not ready, or the named
// capture or story has no vector); the lists are then empty. State is exact,
// approximate or partial, as for every vector search.
type CorpusNearResponse struct {
	Captures    []CorpusNearCapture `json:"captures"`
	Stories     []CorpusNearStory   `json:"stories"`
	Cut         []string            `json:"cut,omitempty"`
	State       string              `json:"state,omitempty"`
	Partial     string              `json:"partial,omitempty"`
	Unavailable string              `json:"unavailable,omitempty"`
}

// parseHalfvec reads pgvector's text form of a vector: "[0.1,0.2,...]".
func parseHalfvec(text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if len(text) < 2 || text[0] != '[' || text[len(text)-1] != ']' {
		return nil, fmt.Errorf("not a vector: %q", clipRunes(text, 40))
	}
	parts := strings.Split(text[1:len(text)-1], ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, err
		}
		out = append(out, float32(f))
	}
	return out, nil
}

// CorpusNear answers one near request.
func (s *Store) CorpusNear(ctx context.Context, req CorpusNearRequest) (CorpusNearResponse, error) {
	res := CorpusNearResponse{Captures: []CorpusNearCapture{}, Stories: []CorpusNearStory{}}
	if s == nil {
		return res, errors.New("corpus near: nil store")
	}
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" || computer == "" {
		return res, fmt.Errorf("%w: owner_id and computer_id are required", errBadCorpus)
	}
	capID, storyID, text := strings.TrimSpace(req.CaptureID), strings.TrimSpace(req.StoryID), strings.TrimSpace(req.Text)
	set := 0
	for _, v := range []string{capID, storyID, text} {
		if v != "" {
			set++
		}
	}
	if set != 1 {
		return res, fmt.Errorf("%w: name exactly one of capture_id, story_id and text", errBadCorpus)
	}
	since, err := corpusTime("since", req.Since)
	if err != nil {
		return res, err
	}
	until, err := corpusTime("until", req.Until)
	if err != nil {
		return res, err
	}
	if !until.IsZero() && !until.Equal(until.Truncate(24*time.Hour)) {
		until = until.Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	k := req.K
	switch {
	case k <= 0:
		k = corpusNearDefault
	case k > corpusNearMax:
		k = corpusNearMax
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	vr := VectorSearchRequest{Since: since, Until: until, Owner: owner, Computer: computer,
		StoryLimit: min(3*k+3, vectorSearchMaxLimit), CaptureLimit: min(k+1, vectorSearchMaxLimit)}
	selfCapture, selfStory := "", ""
	switch {
	case text != "":
		vr.Query = text
	case capID != "":
		selfCapture = capID
		var lit string
		err := s.corpus().QueryRowContext(ctx, `SELECT v.embedding::text FROM `+vecCaptureTable+` v WHERE v.capture_id = ? AND v.model = ? AND v.owner_id = ? AND v.computer_id = ?
			AND v.embedding IS NOT NULL AND `+vecCaptureEligibleSQL, capID, embedder.ModelHash, owner, computer).Scan(&lit)
		if err != nil {
			if isNoRows(err) {
				res.Unavailable = corpusNoVector
				return res, nil
			}
			return res, err
		}
		if vr.Vector, err = parseHalfvec(lit); err != nil {
			return res, err
		}
	default:
		selfStory = storyID
		info, _, err := s.storyWalk(ctx, []string{storyID})
		if err != nil {
			return res, err
		}
		st := info[storyID]
		if st == nil || !st.ok {
			res.Unavailable = corpusNoVector
			return res, nil
		}
		okPairs, err := s.eligibleStoryPairs(ctx, [][2]string{{storyID, st.VersionID}})
		if err != nil {
			return res, err
		}
		if !okPairs[storyID+"|"+st.VersionID] {
			res.Unavailable = corpusNoVector
			return res, nil
		}
		var lit string
		err = s.corpus().QueryRowContext(ctx, `SELECT v.embedding::text FROM `+vecStoryTable+` v WHERE v.story_id = ? AND v.version_id = ? AND v.model = ? AND v.embedding IS NOT NULL
			ORDER BY v.ordinal LIMIT 1`, storyID, st.VersionID, embedder.ModelHash).Scan(&lit)
		if err != nil {
			if isNoRows(err) {
				res.Unavailable = corpusNoVector
				return res, nil
			}
			return res, err
		}
		if vr.Vector, err = parseHalfvec(lit); err != nil {
			return res, err
		}
	}
	got, err := s.SearchVectors(ctx, vr)
	if err != nil {
		return res, err
	}
	if got.Unavailable != "" {
		res.Unavailable = got.Unavailable
		return res, nil
	}
	res.State, res.Partial = vectorStateOf(got)

	var ids []string
	score := map[string]float64{}
	for _, h := range got.Captures {
		if h.CaptureID == selfCapture {
			continue
		}
		if len(ids) == k {
			break
		}
		ids = append(ids, h.CaptureID)
		score[h.CaptureID] = h.Score
	}
	rows, err := s.corpusRowsByID(ctx, owner, computer, ids)
	if err != nil {
		return res, err
	}
	for _, id := range ids {
		if row, ok := rows[id]; ok {
			res.Captures = append(res.Captures, CorpusNearCapture{CorpusCaptureRow: row, Score: score[id]})
		}
	}

	var pubs []string
	seen := map[string]bool{}
	for _, h := range got.Stories {
		if h.PublicationID != selfStory && !seen[h.PublicationID] {
			seen[h.PublicationID] = true
			pubs = append(pubs, h.PublicationID)
		}
	}
	info, _, err := s.storyWalk(ctx, pubs)
	if err != nil {
		return res, err
	}
	done := map[string]bool{}
	for _, h := range got.Stories {
		st := info[h.PublicationID]
		if h.PublicationID == selfStory || done[h.PublicationID] || st == nil || !st.ok || st.VersionID != h.VersionID {
			continue
		}
		if len(res.Stories) == k {
			break
		}
		done[h.PublicationID] = true
		res.Stories = append(res.Stories, CorpusNearStory{PublicationID: h.PublicationID, VersionID: h.VersionID, Revision: st.Revision, Title: st.Title, Day: h.Day, Score: h.Score})
	}
	used := corpusAnswerSlack
	var cut bool
	if res.Captures, cut = fitList(res.Captures, &used); cut {
		res.Cut = append(res.Cut, "captures")
	}
	if res.Stories, cut = fitList(res.Stories, &used); cut {
		res.Cut = append(res.Cut, "stories")
	}
	return res, nil
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// corpusRowsByID reads the metadata rows of the named captures of this scope
// that a desk may be served (the same predicate as every list).
func (s *Store) corpusRowsByID(ctx context.Context, owner, computer string, ids []string) (map[string]CorpusCaptureRow, error) {
	out := map[string]CorpusCaptureRow{}
	for lo := 0; lo < len(ids); lo += 500 {
		group := ids[lo:min(lo+500, len(ids))]
		args := []any{owner, computer}
		for _, id := range group {
			args = append(args, id)
		}
		rows, err := s.corpus().QueryContext(ctx, `SELECT v.capture_id, v.title, v.url, v.host, v.source_id, v.language, v.has_blocks, v.fetched_at, v.published_at
			FROM `+captureIndexTable+` v WHERE v.owner_id = ? AND v.computer_id = ? AND v.capture_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(group)), ",")+`) AND `+vecCaptureEligibleSQL, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, title, url, host, source, lang string
			var blocks bool
			var fetched, published *time.Time
			if err := rows.Scan(&id, &title, &url, &host, &source, &lang, &blocks, &fetched, &published); err != nil {
				_ = rows.Close()
				return nil, err
			}
			row := CorpusCaptureRow{CaptureID: id, Title: clipRunes(title, corpusRowTitle), URL: clipBytes(url, corpusRowURL), Host: host, SourceID: source,
				Kind: corpusKind(source), Language: lang, HasText: blocks}
			if fetched != nil {
				row.FetchedAt = fetched.UTC().Format(time.RFC3339Nano)
			}
			if published != nil {
				row.PublishedAt = published.UTC().Format(time.RFC3339Nano)
			}
			out[id] = row
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// eligibleStoryPairs says which (publication, version) pairs are a published,
// public story: the predicate every vector search of stories uses.
func (s *Store) eligibleStoryPairs(ctx context.Context, pairs [][2]string) (map[string]bool, error) {
	out := map[string]bool{}
	for lo := 0; lo < len(pairs); lo += 200 {
		group := pairs[lo:min(lo+200, len(pairs))]
		var args []any
		var vals []string
		for _, p := range group {
			vals = append(vals, "(?::text, ?::text)")
			args = append(args, p[0], p[1])
		}
		rows, err := s.corpus().QueryContext(ctx, `SELECT v.story_id, v.version_id FROM (VALUES `+strings.Join(vals, ",")+`) AS v(story_id, version_id) WHERE `+vecStoryEligibleSQL, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var p, v string
			if err := rows.Scan(&p, &v); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[p+"|"+v] = true
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// CorpusEdgesRequest names one capture, one published story (publication id)
// or one version of a story.
type CorpusEdgesRequest struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	Ref        string `json:"ref"`
}

// CorpusStoryRef is a published story, or one of its versions.
type CorpusStoryRef struct {
	PublicationID string `json:"publication_id"`
	VersionID     string `json:"version_id"`
	Revision      int    `json:"revision"`
	// Latest says the version is the story's latest.
	Latest bool   `json:"latest"`
	Title  string `json:"title,omitempty"`
}

// CorpusRelated is a story another story is joined to by a related_story edge
// (a pair the research desk judged and recorded). The edge is symmetric and
// stores no direction: Relation is same_incident, later_development or
// same_subject, and later_development does not say which story is later.
type CorpusRelated struct {
	PublicationID string `json:"publication_id"`
	Title         string `json:"title,omitempty"`
	Relation      string `json:"relation"`
}

// CorpusEdgesResponse is what one thing is joined to.
//
// For a capture (Kind "capture"): Stories are the published story versions
// that cite it, and SamePage the other captures of the same page (the same
// canonical URL). For a story or a version (Kind "story" or "version"):
// Versions, Sources (the captures the story's latest version, or the version
// named, cites, those this computer holds and a desk may be served) and Related.
type CorpusEdgesResponse struct {
	Ref      string             `json:"ref"`
	Kind     string             `json:"kind,omitempty"`
	Missing  string             `json:"missing,omitempty"`
	Capture  *CorpusCaptureRow  `json:"capture,omitempty"`
	Stories  []CorpusStoryRef   `json:"stories,omitempty"`
	SamePage []CorpusCaptureRow `json:"same_page,omitempty"`

	Story       *CorpusStoryRef    `json:"story,omitempty"`
	Versions    []CorpusStoryRef   `json:"versions,omitempty"`
	Sources     []CorpusCaptureRow `json:"sources,omitempty"`
	SourcesMore int                `json:"sources_more,omitempty"`
	Related     []CorpusRelated    `json:"related,omitempty"`
	// Cut names the lists an answer over its byte bound was shortened in.
	Cut []string `json:"cut,omitempty"`
}

func revisionOf(meta string) int {
	var m struct {
		Revision int `json:"revision"`
	}
	_ = json.Unmarshal([]byte(meta), &m)
	return m.Revision
}

// CorpusEdges answers one edges request.
func (s *Store) CorpusEdges(ctx context.Context, req CorpusEdgesRequest) (CorpusEdgesResponse, error) {
	ref := strings.TrimSpace(req.Ref)
	res := CorpusEdgesResponse{Ref: ref}
	if s == nil {
		return res, errors.New("corpus edges: nil store")
	}
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" || computer == "" || ref == "" {
		return res, fmt.Errorf("%w: owner_id, computer_id and ref are required", errBadCorpus)
	}
	if len(ref) > 256 {
		return res, fmt.Errorf("%w: ref is too long", errBadCorpus)
	}
	if strings.HasPrefix(ref, captureIDPrefix) {
		out, err := s.corpusCaptureEdges(ctx, owner, computer, res)
		return fitEdges(out), err
	}
	// A version names its publication through has_version.
	pub, version := ref, ""
	var from, meta string
	err := s.corpus().QueryRowContext(ctx, `SELECT from_id, metadata FROM og_edges WHERE to_id = ? AND kind = 'has_version' AND tombstone = FALSE LIMIT 1`, ref).Scan(&from, &meta)
	if err == nil {
		pub, version = from, ref
	} else if !isNoRows(err) {
		return res, err
	}
	out, err := s.corpusStoryEdges(ctx, owner, computer, res, pub, version)
	return fitEdges(out), err
}

// fitEdges cuts the lists of an answer to its byte bound, in order, and names
// the lists it shortened.
func fitEdges(r CorpusEdgesResponse) CorpusEdgesResponse {
	used := corpusAnswerSlack
	var cut bool
	if r.Stories, cut = fitList(r.Stories, &used); cut {
		r.Cut = append(r.Cut, "stories")
	}
	if r.SamePage, cut = fitList(r.SamePage, &used); cut {
		r.Cut = append(r.Cut, "same_page")
	}
	if r.Versions, cut = fitList(r.Versions, &used); cut {
		r.Cut = append(r.Cut, "versions")
	}
	if r.Sources, cut = fitList(r.Sources, &used); cut {
		r.Cut = append(r.Cut, "sources")
	}
	if r.Related, cut = fitList(r.Related, &used); cut {
		r.Cut = append(r.Cut, "related")
	}
	return r
}

func (s *Store) corpusCaptureEdges(ctx context.Context, owner, computer string, res CorpusEdgesResponse) (CorpusEdgesResponse, error) {
	rows, err := s.corpusRowsByID(ctx, owner, computer, []string{res.Ref})
	if err != nil {
		return res, err
	}
	row, ok := rows[res.Ref]
	if !ok {
		res.Missing = corpusMissing
		return res, nil
	}
	res.Kind, res.Capture = "capture", &row
	// The published story versions that cite it.
	rs, err := s.corpus().QueryContext(ctx, `SELECT pe.from_id, e.from_id, pe.metadata FROM og_edges e
		JOIN og_edges pe ON pe.to_id = e.from_id AND pe.kind = 'has_version' AND pe.tombstone = FALSE
		WHERE e.to_id = ? AND e.kind = 'cites_capture' AND e.tombstone = FALSE ORDER BY pe.from_id, e.from_id LIMIT ?`, res.Ref, corpusEdgeMax)
	if err != nil {
		return res, err
	}
	type cite struct {
		pub, ver string
		rev      int
	}
	var cites []cite
	var pubs []string
	seenPub := map[string]bool{}
	var pairs [][2]string
	for rs.Next() {
		var p, v, m string
		if err := rs.Scan(&p, &v, &m); err != nil {
			_ = rs.Close()
			return res, err
		}
		cites = append(cites, cite{p, v, revisionOf(m)})
		pairs = append(pairs, [2]string{p, v})
		if !seenPub[p] {
			seenPub[p] = true
			pubs = append(pubs, p)
		}
	}
	err = rs.Err()
	_ = rs.Close()
	if err != nil {
		return res, err
	}
	if len(cites) > 0 {
		info, _, err := s.storyWalk(ctx, pubs)
		if err != nil {
			return res, err
		}
		// A version is listed only if it is public itself, and the title (the
		// latest version's) only if the latest version is public too.
		for _, p := range pubs {
			if st := info[p]; st != nil && st.ok {
				pairs = append(pairs, [2]string{p, st.VersionID})
			}
		}
		elig, err := s.eligibleStoryPairs(ctx, pairs)
		if err != nil {
			return res, err
		}
		for _, c := range cites {
			st := info[c.pub]
			if st == nil || !st.ok || !elig[c.pub+"|"+c.ver] {
				continue
			}
			res.Stories = append(res.Stories, CorpusStoryRef{PublicationID: c.pub, VersionID: c.ver, Revision: c.rev, Latest: st.VersionID == c.ver, Title: publicTitle(st, elig)})
		}
	}
	// The other captures of the same page.
	rs, err = s.corpus().QueryContext(ctx, `SELECT v.capture_id FROM `+captureIndexTable+` v WHERE v.owner_id = ? AND v.computer_id = ? AND v.capture_id <> ?
		AND v.canonical_key = (SELECT c.canonical_key FROM `+captureIndexTable+` c WHERE c.capture_id = ?) AND `+vecCaptureEligibleSQL+`
		ORDER BY v.fetched_at DESC NULLS LAST, v.capture_id DESC LIMIT ?`, owner, computer, res.Ref, res.Ref, corpusEdgeMax)
	if err != nil {
		return res, err
	}
	var same []string
	for rs.Next() {
		var id string
		if err := rs.Scan(&id); err != nil {
			_ = rs.Close()
			return res, err
		}
		same = append(same, id)
	}
	err = rs.Err()
	_ = rs.Close()
	if err != nil {
		return res, err
	}
	if len(same) > 0 {
		got, err := s.corpusRowsByID(ctx, owner, computer, same)
		if err != nil {
			return res, err
		}
		for _, id := range same {
			if r, ok := got[id]; ok {
				res.SamePage = append(res.SamePage, r)
			}
		}
	}
	return res, nil
}

func (s *Store) corpusStoryEdges(ctx context.Context, owner, computer string, res CorpusEdgesResponse, pub, version string) (CorpusEdgesResponse, error) {
	info, _, err := s.storyWalk(ctx, []string{pub})
	if err != nil {
		return res, err
	}
	st := info[pub]
	if st == nil || !st.ok {
		res.Missing = corpusMissing
		return res, nil
	}
	named := st.VersionID
	if version != "" {
		named = version
	}
	elig, err := s.eligibleStoryPairs(ctx, [][2]string{{pub, named}, {pub, st.VersionID}})
	if err != nil {
		return res, err
	}
	if !elig[pub+"|"+named] {
		res.Missing = corpusMissing
		return res, nil
	}
	title := publicTitle(st, elig)
	res.Kind = "story"
	if version != "" {
		res.Kind = "version"
	}
	// Its versions.
	rs, err := s.corpus().QueryContext(ctx, `SELECT to_id, metadata FROM og_edges WHERE from_id = ? AND kind = 'has_version' AND tombstone = FALSE ORDER BY to_id LIMIT ?`, pub, corpusEdgeMax)
	if err != nil {
		return res, err
	}
	for rs.Next() {
		var v, m string
		if err := rs.Scan(&v, &m); err != nil {
			_ = rs.Close()
			return res, err
		}
		res.Versions = append(res.Versions, CorpusStoryRef{PublicationID: pub, VersionID: v, Revision: revisionOf(m), Latest: v == st.VersionID, Title: title})
	}
	err = rs.Err()
	_ = rs.Close()
	if err != nil {
		return res, err
	}
	// Only the versions that are public themselves are listed.
	var vpairs [][2]string
	for _, v := range res.Versions {
		vpairs = append(vpairs, [2]string{pub, v.VersionID})
	}
	velig, err := s.eligibleStoryPairs(ctx, vpairs)
	if err != nil {
		return res, err
	}
	kept := res.Versions[:0]
	for _, v := range res.Versions {
		if velig[pub+"|"+v.VersionID] {
			kept = append(kept, v)
		}
	}
	res.Versions = kept
	sort.SliceStable(res.Versions, func(i, j int) bool { return res.Versions[i].Revision < res.Versions[j].Revision })
	rev := st.Revision
	for _, v := range res.Versions {
		if v.VersionID == named {
			rev = v.Revision
		}
	}
	res.Story = &CorpusStoryRef{PublicationID: pub, VersionID: named, Revision: rev, Latest: named == st.VersionID, Title: title}
	// What the named version cites, of what this computer holds.
	rs, err = s.corpus().QueryContext(ctx, `SELECT to_id FROM og_edges WHERE from_id = ? AND kind = 'cites_capture' AND tombstone = FALSE ORDER BY to_id LIMIT ?`, named, corpusEdgeMax+1)
	if err != nil {
		return res, err
	}
	var cited []string
	for rs.Next() {
		var id string
		if err := rs.Scan(&id); err != nil {
			_ = rs.Close()
			return res, err
		}
		cited = append(cited, id)
	}
	err = rs.Err()
	_ = rs.Close()
	if err != nil {
		return res, err
	}
	if len(cited) > corpusEdgeMax {
		cited = cited[:corpusEdgeMax]
		res.SourcesMore = 1
	}
	got, err := s.corpusRowsByID(ctx, owner, computer, cited)
	if err != nil {
		return res, err
	}
	for _, id := range cited {
		if r, ok := got[id]; ok {
			res.Sources = append(res.Sources, r)
		}
	}
	// The stories a judged pair joins it to, in either direction.
	rel := map[string]string{}
	for _, q := range []string{
		`SELECT to_id, metadata FROM og_edges WHERE from_id = ? AND kind = 'related_story' AND tombstone = FALSE LIMIT ?`,
		`SELECT from_id, metadata FROM og_edges WHERE to_id = ? AND kind = 'related_story' AND tombstone = FALSE LIMIT ?`,
	} {
		rs, err := s.corpus().QueryContext(ctx, q, pub, relatedStoryEdgeLimit)
		if err != nil {
			return res, err
		}
		for rs.Next() {
			var other, m string
			if err := rs.Scan(&other, &m); err != nil {
				_ = rs.Close()
				return res, err
			}
			var rm struct {
				Relation string `json:"relation"`
			}
			_ = json.Unmarshal([]byte(m), &rm)
			if _, known := storyRelationRank[rm.Relation]; !known || other == pub {
				continue
			}
			if have, ok := rel[other]; !ok || storyRelationRank[rm.Relation] < storyRelationRank[have] {
				rel[other] = rm.Relation
			}
		}
		err = rs.Err()
		_ = rs.Close()
		if err != nil {
			return res, err
		}
	}
	if len(rel) > 0 {
		var others []string
		for o := range rel {
			others = append(others, o)
		}
		sort.Strings(others)
		oinfo, _, err := s.storyWalk(ctx, others)
		if err != nil {
			return res, err
		}
		var pairs [][2]string
		for _, o := range others {
			if st := oinfo[o]; st != nil && st.ok {
				pairs = append(pairs, [2]string{o, st.VersionID})
			}
		}
		ok, err := s.eligibleStoryPairs(ctx, pairs)
		if err != nil {
			return res, err
		}
		for _, o := range others {
			if st := oinfo[o]; st != nil && st.ok && ok[o+"|"+st.VersionID] {
				res.Related = append(res.Related, CorpusRelated{PublicationID: o, Title: st.Title, Relation: rel[o]})
			}
		}
	}
	return res, nil
}

// publicTitle is a story's title only when its latest version is public: the
// title is the latest version's.
func publicTitle(st *storyInfo, elig map[string]bool) string {
	if elig[st.PublicationID+"|"+st.VersionID] {
		return st.Title
	}
	return ""
}

// corpusReplGate is the admission every corpus route shares: a POST by an
// internal caller or by the guest vmctl binds to the owner and computer named.
func (h *ObjectGraphHandler) corpusReplGate(w http.ResponseWriter, r *http.Request, v any, owner, computer func() string) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, corpusReplBytes)).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return false
	}
	o, c := strings.TrimSpace(owner()), strings.TrimSpace(computer())
	if o == "" || c == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "owner_id and computer_id are required"})
		return false
	}
	if err := h.requireInternalOrBoundGuest(r, o, c, false); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return false
	}
	if status, msg := h.captureIndexRefusal(); status != 0 {
		writeJSON(w, status, apiError{Error: msg})
		return false
	}
	return true
}

func corpusReplFailure(w http.ResponseWriter, op string, err error) {
	if errors.Is(err, errBadCorpus) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	log.Printf("corpusd corpus_repl op=%s err=%v", op, err)
	writeJSON(w, http.StatusInternalServerError, apiError{Error: "corpus " + op + " failed"})
}

// HandleCorpusCaptures handles POST /internal/platform/captures/page.
func (h *ObjectGraphHandler) HandleCorpusCaptures(w http.ResponseWriter, r *http.Request) {
	var req CorpusCapturesRequest
	if !h.corpusReplGate(w, r, &req, func() string { return req.OwnerID }, func() string { return req.ComputerID }) {
		return
	}
	res, err := h.captureIndex.CorpusCaptures(r.Context(), req)
	if err != nil {
		corpusReplFailure(w, "page", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// HandleCorpusNear handles POST /internal/platform/captures/near.
func (h *ObjectGraphHandler) HandleCorpusNear(w http.ResponseWriter, r *http.Request) {
	var req CorpusNearRequest
	if !h.corpusReplGate(w, r, &req, func() string { return req.OwnerID }, func() string { return req.ComputerID }) {
		return
	}
	res, err := h.captureIndex.CorpusNear(r.Context(), req)
	if err != nil {
		corpusReplFailure(w, "near", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// HandleCorpusEdges handles POST /internal/platform/captures/edges.
func (h *ObjectGraphHandler) HandleCorpusEdges(w http.ResponseWriter, r *http.Request) {
	var req CorpusEdgesRequest
	if !h.corpusReplGate(w, r, &req, func() string { return req.OwnerID }, func() string { return req.ComputerID }) {
		return
	}
	res, err := h.captureIndex.CorpusEdges(r.Context(), req)
	if err != nil {
		corpusReplFailure(w, "edges", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// CorpusTextsRequest names captures of this scope to read.
type CorpusTextsRequest struct {
	OwnerID    string   `json:"owner_id"`
	ComputerID string   `json:"computer_id"`
	IDs        []string `json:"ids"`
	MaxChars   int      `json:"max_chars,omitempty"`
}

// CorpusTextEntry is one capture's text.
type CorpusTextEntry struct {
	CaptureID string `json:"capture_id"`
	Title     string `json:"title,omitempty"`
	URL       string `json:"url,omitempty"`
	Blocks    []struct {
		ID   string `json:"id,omitempty"`
		Text string `json:"text"`
	} `json:"blocks,omitempty"`
	Body      string   `json:"body,omitempty"`
	Chars     int      `json:"chars"`
	Truncated bool     `json:"truncated,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// CorpusTextsResponse: one entry for each capture reached, in the order asked,
// and Unreached, the ids left for a next call at the byte bound.
type CorpusTextsResponse struct {
	Texts     []CorpusTextEntry `json:"texts"`
	Unreached []string          `json:"unreached,omitempty"`
}

const (
	corpusTextsMaxIDs       = 200
	corpusTextsDefaultChars = 20000
	corpusTextsMaxChars     = 200000
)

// CorpusTexts reads the text of the named captures of this scope, in the
// order asked. A capture a desk may not be served (another computer's or
// owner's, owner-private, tombstoned, superseded, unknown) is one uniform
// refusal: the same predicate as every list. The answer stops at its byte
// bound, as written, and names the ids it did not reach.
func (h *ObjectGraphHandler) CorpusTexts(ctx context.Context, req CorpusTextsRequest) (CorpusTextsResponse, error) {
	res := CorpusTextsResponse{Texts: []CorpusTextEntry{}}
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" || computer == "" {
		return res, fmt.Errorf("%w: owner_id and computer_id are required", errBadCorpus)
	}
	if len(req.IDs) == 0 || len(req.IDs) > corpusTextsMaxIDs {
		return res, fmt.Errorf("%w: name 1 to %d capture ids, got %d", errBadCorpus, corpusTextsMaxIDs, len(req.IDs))
	}
	if h == nil || h.captureIndex == nil || h.service == nil || h.captureBodies == nil {
		return res, errors.New("capture store unavailable")
	}
	maxChars := req.MaxChars
	if maxChars <= 0 {
		maxChars = corpusTextsDefaultChars
	}
	maxChars = min(maxChars, corpusTextsMaxChars)
	var ids []string
	for _, id := range req.IDs {
		ids = append(ids, strings.TrimSpace(id))
	}
	held, err := h.captureIndex.corpusRowsByID(ctx, owner, computer, ids)
	if err != nil {
		return res, err
	}
	used := corpusAnswerSlack
	for _, id := range ids {
		used += len(id) + 4 // room for the id in Unreached, should the answer be cut
	}
	for i, id := range ids {
		entry := CorpusTextEntry{CaptureID: id}
		if _, ok := held[id]; !ok {
			entry.Error = corpusMissing
		} else if err := h.corpusTextOf(ctx, id, maxChars, &entry); err != nil {
			entry = CorpusTextEntry{CaptureID: id, Error: "the text could not be read"}
		}
		n := jsonSize(entry)
		if used+n > corpusAnswerBytesVar && len(res.Texts) > 0 {
			res.Unreached = append(res.Unreached, ids[i:]...)
			break
		}
		used += n
		res.Texts = append(res.Texts, entry)
	}
	return res, nil
}

// corpusTextOf fills entry from the capture's blocks (or its flat text, for a
// capture written before blocks), cut at maxChars characters.
func (h *ObjectGraphHandler) corpusTextOf(ctx context.Context, id string, maxChars int, entry *CorpusTextEntry) error {
	obj, err := h.service.GetObject(ctx, id)
	if err != nil {
		return err
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		return err
	}
	entry.Title, entry.URL = clipRunes(meta.Title, corpusRowTitle), clipBytes(meta.URL, corpusRowURL)
	entry.Labels = types.LabelsFromMetadata(obj.Metadata).Strings()
	left := maxChars
	if meta.BlocksBlobID != "" {
		if raw, err := h.captureBodies.Body(strings.TrimPrefix(meta.BlocksBlobID, "sha256:")); err == nil {
			var doc sourcegraph.CaptureBlocks
			if json.Unmarshal(raw, &doc) == nil && len(doc.Blocks) > 0 {
				for _, b := range doc.Blocks {
					text := b.Text
					if n := utf8.RuneCountInString(text); n > left {
						text, entry.Truncated = clipRunes(text, left), true
					}
					entry.Blocks = append(entry.Blocks, struct {
						ID   string `json:"id,omitempty"`
						Text string `json:"text"`
					}{b.ID, text})
					entry.Chars += utf8.RuneCountInString(text)
					left -= utf8.RuneCountInString(text)
					if entry.Truncated || left <= 0 {
						entry.Truncated = entry.Truncated || left <= 0
						break
					}
				}
				return nil
			}
		}
	}
	body, err := h.captureBodies.Body(strings.TrimPrefix(meta.ExtractedTextBlobID, "sha256:"))
	if err != nil {
		return err
	}
	text := string(body)
	if utf8.RuneCountInString(text) > maxChars {
		text, entry.Truncated = clipRunes(text, maxChars), true
	}
	entry.Body, entry.Chars = text, utf8.RuneCountInString(text)
	return nil
}

// HandleCorpusTexts handles POST /internal/platform/captures/texts.
func (h *ObjectGraphHandler) HandleCorpusTexts(w http.ResponseWriter, r *http.Request) {
	var req CorpusTextsRequest
	if !h.corpusReplGate(w, r, &req, func() string { return req.OwnerID }, func() string { return req.ComputerID }) {
		return
	}
	res, err := h.CorpusTexts(r.Context(), req)
	if err != nil {
		corpusReplFailure(w, "texts", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
