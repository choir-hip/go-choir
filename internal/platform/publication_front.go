package platform

// The Autopaper front list (Cut 2 unit 2b, Reader W3a,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md):
// one row per story publication, newest first, read from Store B alone.
// Building a row reads the publication, its has_version edges, its latest
// version and that version's manifest, all object-graph rows behind corpusd;
// nothing here calls a computer, so the front page keeps working while the
// paper computer is stopped.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

const (
	frontDefaultLimit = 20
	frontMaxLimit     = 100
	// frontDekRunes bounds a row's dek: the story's first paragraph, cut at
	// a word.
	frontDekRunes = 240
)

// FrontStory is one row of the front list: one story publication, shown by
// its latest version.
type FrontStory struct {
	PublicationID  string     `json:"publication_id"`
	Title          string     `json:"title"`
	Dek            string     `json:"dek"`
	UpdatedAt      time.Time  `json:"updated_at"`
	VersionCount   int        `json:"version_count"`
	LatestRevision int        `json:"latest_revision"`
	Route          FrontRoute `json:"route"`
	// Importance (1..10) and Promo come from the host front-importance
	// scores file when it scores this story; absent otherwise.
	Importance int  `json:"importance,omitempty"`
	Promo      bool `json:"promo,omitempty"`
	// Section (world, us_politics, business, tech_ai) also comes from the
	// scores file; absent when unscored or scored before sections existed.
	Section string `json:"section,omitempty"`
	// SourceCount and Words are the depth of the latest version: distinct
	// cited sources, counted as the reader's "Sources N" counts them, and
	// the body's length in words. Both are absent when the version has no
	// structured body (a projected-text publish), so absent means unknown,
	// and zero means a body with no citations.
	SourceCount *int `json:"source_count,omitempty"`
	Words       *int `json:"words,omitempty"`
	// Kind is "opinion" for a row of the Opinion section; empty for an
	// Autopaper story. Author is the opinion piece's byline (a handle, never
	// an email); empty for a story.
	Kind   string `json:"kind,omitempty"`
	Author string `json:"author,omitempty"`
}

// FrontRoute is a story's bare link (serves the latest version in place)
// and its latest version's permanent link.
type FrontRoute struct {
	Bare      string `json:"bare"`
	Permalink string `json:"permalink"`
}

// FrontPage is one page of the front list. NextCursor is empty on the last
// page.
type FrontPage struct {
	Stories    []FrontStory `json:"stories"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// frontCursor is the keyset position after the last publication a page
// read: publications are ordered by (updated_at, canonical_id) descending.
type frontCursor struct {
	UpdatedAt   int64  `json:"u"`
	CanonicalID string `json:"id"`
}

func encodeFrontCursor(c frontCursor) string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeFrontCursor(s string) (*frontCursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var c frontCursor
	if err := json.Unmarshal(raw, &c); err != nil || c.CanonicalID == "" {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &c, nil
}

// ErrInvalidFrontCursor marks a cursor the front list did not mint.
var ErrInvalidFrontCursor = errors.New("invalid cursor")

// frontPublicationMeta is the part of a choir.publication's metadata the
// front list reads.
type frontPublicationMeta struct {
	PublicationID string `json:"publication_id"`
	Title         string `json:"title"`
	State         string `json:"state"`
	RoutePath     string `json:"route_path"`
	PublishMode   string `json:"publish_mode"`
	OwnerID       string `json:"owner_id"`
	// AuthorLabel is an opinion piece's byline, fixed by its first version.
	AuthorLabel string `json:"author_label"`
}

// ParseAutopaperOwners splits CORPUSD_AUTOPAPER_OWNERS: comma-separated
// owner ids, blanks dropped.
func ParseAutopaperOwners(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// SetAutopaperOwners limits the front list to these owners' stories; an
// empty list lists every owner's.
func (s *Service) SetAutopaperOwners(owners []string) {
	set := map[string]bool{}
	for _, owner := range owners {
		if owner = strings.TrimSpace(owner); owner != "" {
			set[owner] = true
		}
	}
	s.frontMu.Lock()
	defer s.frontMu.Unlock()
	if len(set) == 0 {
		set = nil
	}
	s.autopaperOwners = set
}

// isAutopaperStory decides whether a publication is an Autopaper story: a
// publication a computer opened through its story publish route
// (publish_mode auto), from an allowed owner when an allowlist is set. A
// hand-published Texture (mode owner) or one with no recorded mode is not.
func (s *Service) isAutopaperStory(meta frontPublicationMeta) bool {
	if meta.PublishMode != PublishModeAuto {
		return false
	}
	s.frontMu.RLock()
	defer s.frontMu.RUnlock()
	return s.autopaperOwners == nil || s.autopaperOwners[strings.TrimSpace(meta.OwnerID)]
}

// ListFrontStories returns one page of the front list, newest first. limit
// is clamped to [1, 100] (20 when zero); cursor is the previous page's
// NextCursor, or empty for the first page.
//
// Publications are paged by their updated_at, which each version's publish
// rewrites; rows within a page are ordered by their latest version's
// published_at (DATETIME holds seconds, published_at holds nanoseconds).
func (s *Service) ListFrontStories(ctx context.Context, limit int, cursor string) (*FrontPage, error) {
	return s.ListFrontStoriesInSection(ctx, limit, cursor, "")
}

// ErrInvalidFrontSection marks a section filter that names none of the four
// sections.
var ErrInvalidFrontSection = errors.New("invalid section")

// Section-filtered listing reads publications in batches of
// frontSectionBatchSize and gives up after scanning frontSectionScanMax of
// them in one request, so a rare section cannot make one request walk the
// whole paper; the page then carries a cursor and the client continues.
// Variables so tests can shrink them.
var (
	frontSectionBatchSize = 100
	frontSectionScanMax   = 2000
)

// ListFrontStoriesInSection is ListFrontStories restricted to stories whose
// attached section (the host scores file) is section; an empty section is
// the unfiltered front list. A section that is not one of frontSections is
// ErrInvalidFrontSection.
//
// A filtered page is filled by reading further publication batches, at most
// frontSectionScanMax publications per call. NextCursor is set whenever
// publications remain past the last one scanned, so a filtered page may be
// short, or empty, with a cursor: the client keeps following it. A page that
// ends exactly at the last match can offer a cursor whose page is empty.
func (s *Service) ListFrontStoriesInSection(ctx context.Context, limit int, cursor, section string) (*FrontPage, error) {
	if s == nil || s.store == nil || s.store.db == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	section = strings.TrimSpace(section)
	if section != "" && section != FrontSectionOpinion && !frontSections[section] {
		return nil, ErrInvalidFrontSection
	}
	if limit <= 0 {
		limit = frontDefaultLimit
	}
	if limit > frontMaxLimit {
		limit = frontMaxLimit
	}
	after, err := decodeFrontCursor(cursor)
	if err != nil {
		return nil, ErrInvalidFrontCursor
	}
	opinion := section == FrontSectionOpinion
	if opinion && !s.opinionEnabled() {
		// The kill switch hides the whole section: an empty bucket, not an
		// error, so the front renders without it.
		return &FrontPage{Stories: []FrontStory{}}, nil
	}
	if section != "" && !opinion {
		return s.listFrontSection(ctx, og, limit, after, section)
	}

	page := &FrontPage{Stories: []FrontStory{}}
	// Publications that are not listable (not a story, no public version)
	// are skipped, so keep reading batches until the page is full or the
	// publications run out.
	for len(page.Stories) < limit {
		batch, err := s.frontPublicationBatch(ctx, after, limit+1)
		if err != nil {
			return nil, err
		}
		more := len(batch) > limit
		if more {
			batch = batch[:limit]
		}
		for _, pub := range batch {
			after = &frontCursor{UpdatedAt: pub.UpdatedAt.Unix(), CanonicalID: pub.CanonicalID}
			row, ok, err := s.frontRow(ctx, og, pub, opinion)
			if err != nil {
				return nil, err
			}
			if ok {
				page.Stories = append(page.Stories, row)
				if len(page.Stories) == limit {
					break
				}
			}
		}
		if len(page.Stories) == limit {
			// A next page exists only if publications remain past the
			// last one read.
			rest, err := s.frontPublicationBatch(ctx, after, 1)
			if err != nil {
				return nil, err
			}
			if len(rest) > 0 {
				page.NextCursor = encodeFrontCursor(*after)
			}
			break
		}
		if !more {
			break
		}
	}
	sort.SliceStable(page.Stories, func(i, j int) bool {
		return page.Stories[i].UpdatedAt.After(page.Stories[j].UpdatedAt)
	})
	if !opinion {
		s.attachFrontImportance(page.Stories)
	}
	return page, nil
}

// listFrontSection fills a page with the stories scored into section. The
// scores file is the only source of a story's section, so a publication
// whose score names another section is skipped before its row is built; a
// publication whose metadata carries no publication id is built and checked
// after. With no scores file nothing has a section and the page is empty.
func (s *Service) listFrontSection(ctx context.Context, og *ObjectGraphStore, limit int, after *frontCursor, section string) (*FrontPage, error) {
	page := &FrontPage{Stories: []FrontStory{}}
	scores := s.frontImportance.load(time.Now())
	if len(scores) == 0 {
		return page, nil
	}
	inSection := func(id string) bool { return scores[id].Section == section }
	batchSize := frontSectionBatchSize
	if batchSize < 1 {
		batchSize = 1
	}
	scanned := 0
	for len(page.Stories) < limit && scanned < frontSectionScanMax {
		n := batchSize
		if left := frontSectionScanMax - scanned; n > left {
			n = left
		}
		batch, err := s.frontPublicationBatch(ctx, after, n)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, pub := range batch {
			scanned++
			after = &frontCursor{UpdatedAt: pub.UpdatedAt.Unix(), CanonicalID: pub.CanonicalID}
			var meta frontPublicationMeta
			if err := json.Unmarshal(pub.Metadata, &meta); err == nil && meta.PublicationID != "" && !inSection(meta.PublicationID) {
				continue
			}
			row, ok, err := s.frontStory(ctx, og, pub)
			if err != nil {
				return nil, err
			}
			if !ok || !inSection(row.PublicationID) {
				continue
			}
			page.Stories = append(page.Stories, row)
			if len(page.Stories) == limit {
				break
			}
		}
	}
	if after != nil {
		rest, err := s.frontPublicationBatch(ctx, after, 1)
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			page.NextCursor = encodeFrontCursor(*after)
		}
	}
	sort.SliceStable(page.Stories, func(i, j int) bool {
		return page.Stories[i].UpdatedAt.After(page.Stories[j].UpdatedAt)
	})
	s.attachFrontImportance(page.Stories)
	return page, nil
}

// frontPublicationRow is the part of a choir.publication row the front
// list pages over.
type frontPublicationRow struct {
	CanonicalID string
	Metadata    json.RawMessage
	UpdatedAt   time.Time
}

// frontPublicationWindowSQL is one window of the front list's publication
// read: after the cursor, within the window, newest first, at most want
// rows. The plan tests explain this text.
func frontPublicationWindowSQL(after *frontCursor, w newestWindow, want int) (string, []any) {
	query := `SELECT canonical_id, metadata, updated_at FROM og_objects
		WHERE object_kind = 'choir.publication' AND tombstone = FALSE`
	var args []any
	if after != nil {
		at := time.Unix(after.UpdatedAt, 0).UTC()
		query += ` AND (updated_at < ? OR (updated_at = ? AND canonical_id < ?))`
		args = append(args, at, at, after.CanonicalID)
	}
	cond, condArgs := w.windowSQL("updated_at")
	query += cond + ` ORDER BY updated_at DESC, canonical_id DESC LIMIT ?`
	return query, append(append(args, condArgs...), want)
}

// frontPublicationBatch reads up to n publications after the cursor,
// newest updated first.
func (s *Service) frontPublicationBatch(ctx context.Context, after *frontCursor, n int) (out []frontPublicationRow, err error) {
	done := s.store.retrievalTimer(retrievalFrontPublication)
	shape := ""
	defer func() { done(shape, len(out), err) }()
	anchor := time.Now()
	if after != nil {
		anchor = time.Unix(after.UpdatedAt, 0).UTC()
	}
	// Newest first, time-bounded from the cursor (retrieval_newest.go).
	shape, err = readNewestWindows(ctx, anchor, n, func(w newestWindow, want int) (int, error) {
		query, args := frontPublicationWindowSQL(after, w, want)
		rows, err := s.store.corpus().QueryContext(ctx, query, args...)
		if err != nil {
			return 0, fmt.Errorf("platform front: list publications: %w", err)
		}
		defer rows.Close()
		got := 0
		for rows.Next() {
			var row frontPublicationRow
			var metadata string
			if err := rows.Scan(&row.CanonicalID, &metadata, &row.UpdatedAt); err != nil {
				return got, fmt.Errorf("platform front: scan publication: %w", err)
			}
			row.Metadata = json.RawMessage(metadata)
			row.UpdatedAt = row.UpdatedAt.UTC()
			out = append(out, row)
			got++
		}
		if err := rows.Err(); err != nil {
			return got, fmt.Errorf("platform front: iterate publications: %w", err)
		}
		return got, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// frontStory builds a publication's row from its latest version. ok is
// false for a publication the front list does not show: not a story, not
// published, no version, or a latest version whose access policy keeps it
// off public routes.
func (s *Service) frontStory(ctx context.Context, og *ObjectGraphStore, pub frontPublicationRow) (FrontStory, bool, error) {
	return s.frontRow(ctx, og, pub, false)
}

// frontRow is frontStory for either list: the news front (opinion false:
// Autopaper stories only) or the Opinion section (opinion true: opinion
// pieces only, with the kill switch on).
func (s *Service) frontRow(ctx context.Context, og *ObjectGraphStore, pub frontPublicationRow, opinion bool) (FrontStory, bool, error) {
	var meta frontPublicationMeta
	if err := json.Unmarshal(pub.Metadata, &meta); err != nil {
		return FrontStory{}, false, nil
	}
	if meta.State != "published" || strings.TrimSpace(meta.RoutePath) == "" {
		return FrontStory{}, false, nil
	}
	if opinion {
		if !s.isOpinionPiece(meta) {
			return FrontStory{}, false, nil
		}
	} else if !s.isAutopaperStory(meta) {
		return FrontStory{}, false, nil
	}
	versions, err := publishedVersions(ctx, og, pub.CanonicalID)
	if err != nil {
		return FrontStory{}, false, fmt.Errorf("platform front: list versions of %s: %w", pub.CanonicalID, err)
	}
	if len(versions) == 0 {
		return FrontStory{}, false, nil
	}
	latest := versions[len(versions)-1]
	versionObj, err := og.GetObject(ctx, latest.CanonicalID)
	if err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return FrontStory{}, false, nil
		}
		return FrontStory{}, false, fmt.Errorf("platform front: get v%d of %s: %w", latest.Revision, pub.CanonicalID, err)
	}
	versionMeta, err := decodeVersionMetadata(versionObj)
	if err != nil {
		return FrontStory{}, false, fmt.Errorf("platform front: %w", err)
	}
	manifest, err := s.frontManifest(ctx, og, versionObj.CanonicalID)
	if err != nil {
		return FrontStory{}, false, err
	}
	if !publicationAccessPolicyAllowsPublicRoute(manifest.AccessPolicy) {
		return FrontStory{}, false, nil
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, versionMeta.PublishedAt)
	if err != nil {
		updatedAt = pub.UpdatedAt
	}
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = strings.TrimSpace(manifest.Title)
	}
	dek := dekFromBodyDoc(manifest.BodyDoc)
	if dek == "" && manifest.StorageRef != "" {
		if content, err := s.readBlob(manifest.StorageRef); err == nil {
			dek = dekFromText(string(content))
		}
	}
	bare := normalizePublicationRoutePath(meta.RoutePath)
	sourceCount, words := frontDepth(manifest.BodyDoc, manifest.Anchors)
	kind, author := "", ""
	if opinion {
		kind, author = "opinion", meta.AuthorLabel
	}
	return FrontStory{
		Kind:           kind,
		Author:         author,
		PublicationID:  firstNonEmpty(meta.PublicationID, versionMeta.PublicationID),
		Title:          title,
		Dek:            dek,
		UpdatedAt:      updatedAt.UTC(),
		VersionCount:   len(versions),
		LatestRevision: latest.Revision,
		Route:          FrontRoute{Bare: bare, Permalink: versionPermalink(bare, latest.Revision)},
		SourceCount:    sourceCount,
		Words:          words,
	}, true, nil
}

// frontManifestFields is the part of a version manifest a front row reads.
type frontManifestFields struct {
	Title        string              `json:"title"`
	StorageRef   string              `json:"storage_ref"`
	BodyDoc      json.RawMessage     `json:"body_doc"`
	AccessPolicy json.RawMessage     `json:"access_policy"`
	Anchors      []PublicationAnchor `json:"anchors"`
}

func (s *Service) frontManifest(ctx context.Context, og *ObjectGraphStore, versionCanonicalID string) (frontManifestFields, error) {
	var out frontManifestFields
	edges, err := og.ListEdgesByKind(ctx, versionCanonicalID, "has_manifest")
	if err != nil {
		return out, fmt.Errorf("platform front: manifest edges of %s: %w", versionCanonicalID, err)
	}
	if len(edges) == 0 {
		return out, fmt.Errorf("platform front: no manifest for %s", versionCanonicalID)
	}
	obj, err := og.GetObject(ctx, edges[0].ToID)
	if err != nil {
		return out, fmt.Errorf("platform front: get manifest of %s: %w", versionCanonicalID, err)
	}
	if err := json.Unmarshal(obj.Body, &out); err != nil {
		return out, fmt.Errorf("platform front: parse manifest of %s: %w", versionCanonicalID, err)
	}
	return out, nil
}

// dekFromBodyDoc is a story's first paragraph, its citations left out.
func dekFromBodyDoc(raw json.RawMessage) string {
	if len(strings.TrimSpace(string(raw))) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return ""
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	for _, block := range doc.Doc.Content {
		if block.Type != "paragraph" {
			continue
		}
		var b strings.Builder
		for _, n := range block.Content {
			switch n.Type {
			case "text":
				b.WriteString(n.Text)
			case "hard_break":
				b.WriteByte(' ')
			}
		}
		if text := truncateDek(b.String()); text != "" {
			return text
		}
	}
	return ""
}

// dekFromText is the first paragraph of a projected body that is not a
// heading, a list, a quote or a code fence.
func dekFromText(content string) string {
	for _, para := range strings.Split(content, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" || strings.HasPrefix(para, "#") || strings.HasPrefix(para, ">") || strings.HasPrefix(para, "```") || strings.HasPrefix(para, "- ") || para == "---" {
			continue
		}
		if text := truncateDek(para); text != "" {
			return text
		}
	}
	return ""
}

// truncateDek collapses whitespace and cuts at a word within frontDekRunes.
func truncateDek(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= frontDekRunes {
		return text
	}
	runes := []rune(text)
	cut := string(runes[:frontDekRunes])
	if i := strings.LastIndex(cut, " "); i > frontDekRunes/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:.-") + "…"
}
