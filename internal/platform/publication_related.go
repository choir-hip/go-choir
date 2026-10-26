package platform

// Related stories, computed from the object graph (phase 1 of
// docs/reports/related-stories-graph-design-2026-10-19.md). A story's
// related stories are the other current Autopaper stories that cite the
// same captures, or captures of the same page (a shared canonical key).
// Nothing is stored: the list is a read over Store B, so nothing stored can
// go stale; only a short in-process cache of the derived answer can lag.
// The walk calls no model and no computer, and a story's resolve path does
// not change: the reader asks for the list after the article has rendered.
//
// The walk (5.5), phase 1 signals only:
//
//  1. The story's latest version and its cites_capture edges.
//  2. Each cited capture's source key: the canonical key of its page in the
//     capture projection (one signal per page, however many recaptures),
//     or the capture itself when the projection cannot say.
//  3. For each source key, the versions that cite any capture of the page
//     (the (to_id, kind) index of og_edges), mapped to their publication.
//  4. A publication counts only when its LATEST version is the one that
//     cites, and only when it is a published Autopaper story. A source
//     cited by more than HubStories such stories is a hub and has no vote.
//  5. score = sum of idf(source) over shared sources, times a recency
//     factor; a candidate must clear Threshold and pass frontStory (the
//     front list's own test of a public story).
//  6. With CORPUSD_RELATED_STORY=on, the story's related_story edges (a pair
//     the research desk judged related, publication_related_story.go) add
//     candidates. A judged pair needs no score and ranks ahead of capture
//     links, by relation; the threshold and the hub cutoff above are not
//     changed.
//  7. Unless CORPUSD_RELATED_ENTITIES=off, the stories centrally about an
//     entity the story is centrally about (mentions_entity edges, written by
//     the research desk's choir.Relate, publication_related_entities.go) add
//     candidates. An entity central to more than HubEntityStories stories
//     has no vote. An entity link needs no score and ranks after judged
//     pairs and ahead of capture links, by shared entities.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// relatedKillSwitchEnv is corpusd's rollback switch: CORPUSD_RELATED=off
// makes every related answer links [] without walking. Absent or any other
// value, the walk runs. Read per request, so it flips with a corpusd
// restart and no deploy (the env file beside CORPUSD_CAPTURE_SEARCH).
const relatedKillSwitchEnv = "CORPUSD_RELATED"

func relatedDisabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(relatedKillSwitchEnv)), "off")
}

// Basis modes: which signals the answer used. relatedModeRelations (the
// related_story edges too) is in publication_related_story.go.
const (
	relatedModeCaptures = "captures"
	relatedModeOff      = "off"
)

// relatedMode is the mode a walk now runs in.
func relatedMode() string {
	mode := relatedModeCaptures
	if relatedStoryOn() {
		mode = relatedModeRelations
	}
	if relatedEntitiesOn() {
		mode += relatedModeEntitiesSuffix
	}
	return mode
}

// Link reasons.
const (
	relatedReasonCapture = "capture" // a capture both stories cite
	relatedReasonSource  = "source"  // a recapture of one page
	relatedReasonEntity  = "entity"  // a central entity both stories are about
)

// relatedTuning is the one table of starting values (design 5.5, D5): they
// are hypotheses to calibrate on live data, and a calibration is a
// one-line change here.
type relatedTuning struct {
	// MaxCaptures bounds the story's cited captures the walk reads (R5).
	MaxCaptures int
	// MaxSiblings bounds the captures read per page.
	MaxSiblings int
	// MaxRawHits bounds one capture's reverse edge read; a capture that
	// fills it is a hub.
	MaxRawHits int
	// HubStories: a source cited by more than this many stories has no vote.
	HubStories int
	// MaxCandidates bounds the candidate rows built (frontStory reads).
	MaxCandidates int
	// MaxRelatedEdges bounds a story's related_story neighbours: more is a
	// hub for that signal and gives no vote, on write and on read.
	MaxRelatedEdges int
	// HubEntityStories: an entity central to more than this many stories
	// has no vote (phase 3 §4.1: hubs are down-weighted as in phase 1).
	HubEntityStories int
	// HalfLifeDays and RecencyFloor shape the recency factor.
	HalfLifeDays float64
	RecencyFloor float64
	// SameSection multiplies the rank (never the threshold) of a story in
	// the same section.
	SameSection float64
	// Threshold is the least score a link may have.
	Threshold float64
	// DefaultLinks and MaxLinks bound the list.
	DefaultLinks int
	MaxLinks     int
	// Budget bounds one request; an overrun answers empty and partial.
	Budget time.Duration
	// TTL and CacheCap bound the answer cache.
	TTL      time.Duration
	CacheCap int
	// VersionPubCap bounds the version to publication memo.
	VersionPubCap int
}

func defaultRelatedTuning() relatedTuning {
	return relatedTuning{
		MaxCaptures:      24,
		MaxSiblings:      20,
		MaxRawHits:       200,
		HubStories:       6,
		MaxCandidates:    40,
		MaxRelatedEdges:  20,
		HubEntityStories: 30,
		HalfLifeDays:     7,
		RecencyFloor:     0.2,
		SameSection:      1.1,
		Threshold:        1.0,
		DefaultLinks:     5,
		MaxLinks:         10,
		Budget:           400 * time.Millisecond,
		TTL:              5 * time.Minute,
		CacheCap:         500,
		VersionPubCap:    50000,
	}
}

// RelatedStories is the answer for one story: the same body shape whether
// the story has links, none, or is unknown.
type RelatedStories struct {
	Route       string        `json:"route"`
	Revision    int           `json:"revision"`
	Basis       RelatedBasis  `json:"basis"`
	Partial     bool          `json:"partial"`
	GeneratedAt time.Time     `json:"generated_at"`
	Links       []RelatedLink `json:"links"`
}

// RelatedBasis says why a list is what it is, so an operator or a test can
// read an empty list without SSH: how many captures the story cites, how
// many events they reach (always 0 in phase 1), and which signals were on.
type RelatedBasis struct {
	Captures int    `json:"captures"`
	Events   int    `json:"events"`
	Mode     string `json:"mode"`
}

// RelatedLink is a front row plus why it is related.
type RelatedLink struct {
	FrontStory
	Reason string        `json:"reason"`
	Shared RelatedShared `json:"shared"`
}

// RelatedShared counts what the two stories share. Citers is the largest
// number of stories citing any shared source that counted (never above the
// hub cutoff): it lets a reader of the response check the hub rule.
type RelatedShared struct {
	Events  int `json:"events"`
	Sources int `json:"sources"`
	Citers  int `json:"citers"`
	// Entities counts the central entities both stories are about (each
	// central to at most HubEntityStories stories).
	Entities int `json:"entities"`
}

// relatedCache is the in-process state of the related walk: the answer
// cache (5.6), the version to publication memo, and the story count. All of
// it is derived and memory-only: a corpusd restart empties it and the next
// request refills it, so nothing durable depends on it. The zero value is
// usable; NewService and the methods call init.
type relatedCache struct {
	mu         sync.Mutex
	ready      bool
	tuning     relatedTuning
	now        func() time.Time
	entries    map[string]relatedEntry
	order      []string
	versionPub map[string]string
	storyN     int
	storyNAt   time.Time
}

type relatedEntry struct {
	res *RelatedStories
	at  time.Time
}

func newRelatedCache() *relatedCache {
	c := &relatedCache{}
	c.init()
	return c
}

func (c *relatedCache) init() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
}

func (c *relatedCache) initLocked() {
	if c.ready {
		return
	}
	c.ready = true
	c.tuning = defaultRelatedTuning()
	c.now = func() time.Time { return time.Now() }
	c.entries = map[string]relatedEntry{}
	c.versionPub = map[string]string{}
}

func (c *relatedCache) clock() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	return c.now()
}

func (c *relatedCache) tune() relatedTuning {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	return c.tuning
}

// reset empties every derived thing.
func (c *relatedCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	c.entries = map[string]relatedEntry{}
	c.order = nil
	c.versionPub = map[string]string{}
	c.storyN, c.storyNAt = 0, time.Time{}
}

func (c *relatedCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *relatedCache) get(key string) (*RelatedStories, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.now().Sub(e.at) >= c.tuning.TTL {
		delete(c.entries, key)
		return nil, false
	}
	return e.res, true
}

// put stores an answer; at the cap the oldest entry goes first.
func (c *relatedCache) put(key string, res *RelatedStories) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	if _, ok := c.entries[key]; !ok {
		c.order = append(c.order, key)
	}
	c.entries[key] = relatedEntry{res: res, at: c.now()}
	for len(c.entries) > c.tuning.CacheCap && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
}

// The version to publication mapping never changes (a version belongs to
// one publication for good), so it is memoised without a TTL.
func (c *relatedCache) pubOf(version string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	p, ok := c.versionPub[version]
	return p, ok
}

func (c *relatedCache) setPubOf(version, pub string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	if len(c.versionPub) >= c.tuning.VersionPubCap {
		c.versionPub = map[string]string{}
	}
	c.versionPub[version] = pub
}

// relatedCandidate is a story the walk found, before ranking.
type relatedCandidate struct {
	Story   FrontStory
	Raw     float64 // the sum of idf over shared sources
	Sources int     // shared sources that counted
	Exact   bool    // some shared source is a capture both cite
	Citers  int     // the largest citing-story count among them
	// Relation is the research desk's judged relation to the story
	// (related_story edge); "" for a capture-only candidate. A judged
	// pair needs no score: the judgment is its evidence.
	Relation string
	// Entities counts the shared central entities that voted; a candidate
	// with any needs no score.
	Entities int
}

// recencyFactor halves every HalfLifeDays, never below RecencyFloor; a
// story from the future counts as new.
func recencyFactor(ageDays float64, t relatedTuning) float64 {
	if ageDays <= 0 {
		return 1
	}
	f := math.Pow(0.5, ageDays/t.HalfLifeDays)
	if f < t.RecencyFloor {
		return t.RecencyFloor
	}
	return f
}

// rankRelated scores, filters and orders candidates. A capture link needs a
// score of at least Threshold; a shared section raises a link's rank, never
// its score, so it cannot create a link. A pair the research desk judged
// related (Relation) needs no score. The list is cut at limit and is never
// padded. Order: judged pairs by relation, then newer; then capture links by
// rank, then newer, then publication id.
func rankRelated(self FrontStory, cands []relatedCandidate, now time.Time, t relatedTuning, limit int) []RelatedLink {
	type scored struct {
		c    relatedCandidate
		rank float64
	}
	var kept []scored
	for _, c := range cands {
		if c.Story.PublicationID == self.PublicationID {
			continue
		}
		age := now.Sub(c.Story.UpdatedAt).Hours() / 24
		score := c.Raw * recencyFactor(age, t)
		// A judged pair (c.Relation) needs no score; any other candidate
		// must clear the threshold.
		if c.Relation == "" && c.Entities == 0 && score < t.Threshold {
			continue
		}
		rank := score
		if self.Section != "" && self.Section == c.Story.Section {
			rank *= t.SameSection
		}
		kept = append(kept, scored{c: c, rank: rank})
	}
	sort.SliceStable(kept, func(i, j int) bool {
		a, b := kept[i], kept[j]
		// Judged pairs first, by relation, then newer; then entity links
		// by shared entities, then rank; then capture links by rank.
		if (a.c.Relation != "") != (b.c.Relation != "") {
			return a.c.Relation != ""
		}
		if a.c.Relation != "" {
			if ra, rb := storyRelationRank[a.c.Relation], storyRelationRank[b.c.Relation]; ra != rb {
				return ra < rb
			}
		} else {
			if a.c.Entities != b.c.Entities {
				return a.c.Entities > b.c.Entities
			}
			if a.rank != b.rank {
				return a.rank > b.rank
			}
		}
		if !a.c.Story.UpdatedAt.Equal(b.c.Story.UpdatedAt) {
			return a.c.Story.UpdatedAt.After(b.c.Story.UpdatedAt)
		}
		return a.c.Story.PublicationID < b.c.Story.PublicationID
	})
	if limit > 0 && len(kept) > limit {
		kept = kept[:limit]
	}
	links := make([]RelatedLink, 0, len(kept))
	for _, k := range kept {
		reason := relatedReasonSource
		if k.c.Exact {
			reason = relatedReasonCapture
		}
		if k.c.Entities > 0 {
			reason = relatedReasonEntity
		}
		if k.c.Relation != "" {
			reason = k.c.Relation
		}
		links = append(links, RelatedLink{
			FrontStory: k.c.Story,
			Reason:     reason,
			Shared:     RelatedShared{Sources: k.c.Sources, Citers: k.c.Citers, Entities: k.c.Entities},
		})
	}
	return links
}

// --- capture projection reads ---

// captureKeyRow is one capture's keys in the capture projection.
type captureKeyRow struct {
	id, owner, computer  string
	canonicalKey, urlKey string
	url, canonicalURL    string
}

// sourceKeyOf is the identity of the page a capture is of. Captures with
// the same key, owner and computer are recaptures of one page. A capture
// with no canonical URL falls back to its URL, and with neither is its own
// page: the hash of "" must never join unrelated captures.
func sourceKeyOf(r captureKeyRow) string {
	empty := captureURLKey("")
	switch {
	case r.canonicalKey != "" && r.canonicalKey != empty:
		return "canonical|" + r.owner + "|" + r.computer + "|" + r.canonicalKey
	case r.urlKey != "" && r.urlKey != empty:
		return "url|" + r.owner + "|" + r.computer + "|" + r.urlKey
	}
	return "capture|" + r.id
}

// isLivePageURL reports a live page: a live blog or rolling updates page,
// many subjects under one URL, each recapture a different set of entries.
// Sharing one says nothing about two stories' subject (staging,
// 2026-10-20: one live blog meshed five unrelated stories), so it gives no
// vote. It reads the path only: a "live" or "liveblog" segment, or a last
// segment that names live updates.
func isLivePageURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	segments := strings.Split(strings.Trim(strings.ToLower(u.Path), "/"), "/")
	for _, seg := range segments {
		switch seg {
		case "live", "liveblog", "live-blog", "live-news", "live-updates":
			return true
		}
	}
	last := segments[len(segments)-1]
	for _, marker := range []string{"live-updates", "liveblog", "live-blog"} {
		if strings.Contains(last, marker) {
			return true
		}
	}
	return false
}

// captureKeyRows reads the keys of captures from the projection. It returns
// nothing when the projection is not ready: the caller then treats each
// capture as its own page.
func (s *Store) captureKeyRows(ctx context.Context, ids []string) (map[string]captureKeyRow, error) {
	if s == nil || s.db == nil || !s.retrievalState.captureIndexReady.Load() || len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(ids))
	marks := make([]string, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
		marks = append(marks, "?")
	}
	rows, err := s.corpus().QueryContext(ctx,
		`SELECT capture_id, owner_id, computer_id, canonical_key, url_key, url, canonical_url FROM `+captureIndexTable+` WHERE capture_id IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]captureKeyRow{}
	for rows.Next() {
		var r captureKeyRow
		if err := rows.Scan(&r.id, &r.owner, &r.computer, &r.canonicalKey, &r.urlKey, &r.url, &r.canonicalURL); err != nil {
			return nil, err
		}
		out[r.id] = r
	}
	return out, rows.Err()
}

// capturesSharingKey lists the captures of one page: the same owner and
// computer and the same key, newest fetch first.
func (s *Store) capturesSharingKey(ctx context.Context, owner, computer, column, key string, limit int) ([]string, error) {
	if column != "canonical_key" && column != "url_key" {
		return nil, fmt.Errorf("related: unknown key column %q", column)
	}
	rows, err := s.corpus().QueryContext(ctx,
		`SELECT capture_id FROM `+captureIndexTable+` WHERE owner_id = ? AND computer_id = ? AND `+column+` = ? ORDER BY fetched_at DESC LIMIT ?`,
		owner, computer, key, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// --- the walk ---

// relatedGroup is one source (page) the story cites: the story's own
// captures of it and every capture of the same page.
type relatedGroup struct {
	own     map[string]bool
	members []string
	// owner, computer, column and key locate siblings; empty for a capture
	// that is its own page.
	owner, computer, column, key string
	// live: the page is a live page (isLivePageURL); it gives no vote.
	live bool
}

func (g *relatedGroup) add(id string) {
	for _, m := range g.members {
		if m == id {
			return
		}
	}
	g.members = append(g.members, id)
}

// RelatedStories answers a story's route (bare, or a /v<n> permalink: both
// show the story's current list, computed from its latest version) with up
// to limit other stories. It answers the same shape for an unknown route, a
// story that is not a public Autopaper story, a story with nothing to
// link, and the kill switch: links [], never null, never an error, so the
// route leaks no existence signal that resolve does not. An error is a
// store failure.
func (s *Service) RelatedStories(ctx context.Context, routePath string, limit int) (*RelatedStories, error) {
	if s == nil || s.store == nil || s.store.db == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	c := &s.related
	t := c.tune()
	now := c.clock()
	out := &RelatedStories{Route: strings.TrimSpace(routePath), Links: []RelatedLink{}, GeneratedAt: now.UTC(), Basis: RelatedBasis{Mode: relatedMode()}}
	if relatedDisabled() {
		out.Basis.Mode = relatedModeOff
		return out, nil
	}
	if limit <= 0 {
		limit = t.DefaultLinks
	}
	if limit > t.MaxLinks {
		limit = t.MaxLinks
	}
	bare, _, ok := parsePublicationRoute(normalizePublicationRoutePath(routePath))
	if !ok {
		return out, nil
	}
	out.Route = bare

	ctx, cancel := context.WithTimeout(ctx, t.Budget)
	defer cancel()
	res, err := s.relatedWalk(ctx, bare, limit, t, now, out)
	if err != nil {
		if ctx.Err() != nil {
			// Over budget: empty, flagged partial, never cached.
			partial := &RelatedStories{Route: bare, Links: []RelatedLink{}, GeneratedAt: now.UTC(), Basis: RelatedBasis{Mode: out.Basis.Mode}, Partial: true}
			return partial, nil
		}
		return nil, err
	}
	return res, nil
}

func (s *Service) relatedWalk(ctx context.Context, bare string, limit int, t relatedTuning, now time.Time, out *RelatedStories) (*RelatedStories, error) {
	c := &s.related
	head, err := s.resolvePublicationHead(ctx, bare, newResolveSteps())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, nil
		}
		return nil, err
	}
	og := head.og
	latest := head.versions[len(head.versions)-1]
	out.Revision = latest.Revision
	key := fmt.Sprintf("%s|%d|%s|%d", head.routeMeta.TargetID, latest.Revision, out.Basis.Mode, limit)
	if cached, ok := c.get(key); ok {
		return cached, nil
	}

	pubObj, err := og.GetObject(ctx, head.routeMeta.TargetID)
	if err != nil {
		return nil, fmt.Errorf("related: get publication: %w", err)
	}
	selfRow := frontPublicationRow{CanonicalID: pubObj.CanonicalID, Metadata: pubObj.Metadata, UpdatedAt: pubObj.UpdatedAt.UTC()}
	self, ok, err := s.frontStory(ctx, og, selfRow)
	if err != nil {
		return nil, err
	}
	if !ok {
		// Not a public Autopaper story: nothing to relate, nothing to say.
		return out, nil
	}
	selfs := []FrontStory{self}
	s.attachFrontImportance(selfs)
	self = selfs[0]

	cited, err := s.relatedCitedCaptures(ctx, og, latest.CanonicalID)
	if err != nil {
		return nil, err
	}
	out.Basis.Captures = len(cited)
	walked := cited
	if len(walked) > t.MaxCaptures {
		walked = walked[:t.MaxCaptures]
	}
	cands, err := s.relatedCandidates(ctx, og, self, pubObj.CanonicalID, walked, t, now)
	if err != nil {
		return nil, err
	}
	out.Links = rankRelated(self, cands, now, t, limit)
	c.put(key, out)
	return out, nil
}

// relatedCitedCaptures are the story's latest version's cites_capture
// targets, sorted and distinct.
func (s *Service) relatedCitedCaptures(ctx context.Context, og *ObjectGraphStore, version string) ([]string, error) {
	live := false
	edges, err := og.ListEdges(ctx, objectgraph.EdgeFilter{FromID: version, Kind: edgeCitesCapture, Tombstone: &live, Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("related: cited captures: %w", err)
	}
	seen := map[string]bool{}
	var ids []string
	for _, e := range edges {
		if !seen[e.ToID] {
			seen[e.ToID] = true
			ids = append(ids, e.ToID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// relatedCandidates are the stories that share a source with the story (the
// capture walk) and, with CORPUSD_RELATED_STORY=on, the stories the research
// desk judged related to it. The judged ones are read even for a story that
// cites no capture.
func (s *Service) relatedCandidates(ctx context.Context, og *ObjectGraphStore, self FrontStory, selfPub string, captures []string, t relatedTuning, now time.Time) ([]relatedCandidate, error) {
	cands, err := s.relatedCaptureCandidates(ctx, og, self, selfPub, captures, t, now)
	if err != nil {
		return nil, err
	}
	if relatedStoryOn() {
		judged, err := s.relatedStoryCandidates(ctx, og, selfPub, t)
		if err != nil {
			return nil, err
		}
		cands = mergeRelatedStoryCandidates(cands, judged)
	}
	if relatedEntitiesOn() {
		shared, err := s.relatedEntityCandidates(ctx, og, selfPub, t)
		if err != nil {
			return nil, err
		}
		cands = mergeRelatedEntityCandidates(cands, shared)
	}
	return cands, nil
}

func (s *Service) relatedCaptureCandidates(ctx context.Context, og *ObjectGraphStore, self FrontStory, selfPub string, captures []string, t relatedTuning, now time.Time) ([]relatedCandidate, error) {
	if len(captures) == 0 {
		return nil, nil
	}
	groups, err := s.relatedGroups(ctx, captures, t)
	if err != nil {
		return nil, err
	}

	// Versions that cite any member of each group (the reverse index).
	live := false
	groupVersions := make([]map[string]bool, len(groups)) // version -> via an own capture
	hub := make([]bool, len(groups))
	allVersions := map[string]bool{}
	for gi, g := range groups {
		groupVersions[gi] = map[string]bool{}
		for _, m := range g.members {
			edges, err := og.ListEdges(ctx, objectgraph.EdgeFilter{ToID: m, Kind: edgeCitesCapture, Tombstone: &live, Limit: t.MaxRawHits})
			if err != nil {
				return nil, fmt.Errorf("related: stories citing %s: %w", m, err)
			}
			if len(edges) >= t.MaxRawHits {
				hub[gi] = true
				break
			}
			for _, e := range edges {
				groupVersions[gi][e.FromID] = groupVersions[gi][e.FromID] || g.own[m]
				allVersions[e.FromID] = true
			}
		}
	}

	// Version -> publication, then publication -> (object, latest version).
	pubOf := map[string]string{}
	for v := range allVersions {
		p, err := s.relatedPublicationOf(ctx, og, v)
		if err != nil {
			return nil, err
		}
		if p != "" {
			pubOf[v] = p
		}
	}
	pubIDs := map[string]bool{}
	for _, p := range pubOf {
		pubIDs[p] = true
	}
	pubs, err := s.relatedPublications(ctx, og, pubIDs)
	if err != nil {
		return nil, err
	}

	n := s.relatedStoryCount(ctx, t, now)
	type tally struct {
		raw     float64
		sources int
		exact   bool
		citers  int
	}
	tallies := map[string]*tally{}
	for gi := range groups {
		if hub[gi] || groups[gi].live {
			continue
		}
		// The stories whose latest version cites this source.
		citing := map[string]bool{} // publication -> via an own capture
		for v, exact := range groupVersions[gi] {
			p := pubOf[v]
			info, ok := pubs[p]
			if p == "" || !ok || !info.eligible || info.latest != v {
				continue
			}
			citing[p] = citing[p] || exact
		}
		citing[selfPub] = citing[selfPub] || true
		df := len(citing)
		if df > t.HubStories {
			continue
		}
		if n < df {
			n = df
		}
		idf := math.Log(1 + float64(n)/float64(1+df))
		for p, exact := range citing {
			if p == selfPub {
				continue
			}
			ta := tallies[p]
			if ta == nil {
				ta = &tally{}
				tallies[p] = ta
			}
			ta.raw += idf
			ta.sources++
			ta.exact = ta.exact || exact
			if df > ta.citers {
				ta.citers = df
			}
		}
	}

	// The strongest candidates become rows (frontStory also tests that the
	// story is public now).
	type pick struct {
		pub string
		raw float64
	}
	var picks []pick
	for p, ta := range tallies {
		if ta.raw*t.SameSection >= t.Threshold {
			picks = append(picks, pick{p, ta.raw})
		}
	}
	sort.Slice(picks, func(i, j int) bool {
		if picks[i].raw != picks[j].raw {
			return picks[i].raw > picks[j].raw
		}
		return picks[i].pub < picks[j].pub
	})
	if len(picks) > t.MaxCandidates {
		picks = picks[:t.MaxCandidates]
	}
	var stories []FrontStory
	var order []string
	for _, p := range picks {
		info := pubs[p.pub]
		row, ok, err := s.frontStory(ctx, og, frontPublicationRow{CanonicalID: p.pub, Metadata: info.obj.Metadata, UpdatedAt: info.obj.UpdatedAt.UTC()})
		if err != nil {
			return nil, err
		}
		if ok {
			stories = append(stories, row)
			order = append(order, p.pub)
		}
	}
	s.attachFrontImportance(stories)
	cands := make([]relatedCandidate, 0, len(stories))
	for i, story := range stories {
		ta := tallies[order[i]]
		cands = append(cands, relatedCandidate{Story: story, Raw: ta.raw, Sources: ta.sources, Exact: ta.exact, Citers: ta.citers})
	}
	return cands, nil
}

// relatedGroups groups the story's cited captures by the page they are of,
// and adds the other captures of each page.
func (s *Service) relatedGroups(ctx context.Context, captures []string, t relatedTuning) ([]*relatedGroup, error) {
	rows, err := s.store.captureKeyRows(ctx, captures)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// The projection is a derived index. Without it each capture is
		// its own page and exact shared captures still link.
		log.Printf("corpusd: related: capture keys unavailable, using exact captures: %v", err)
		rows = nil
	}
	byKey := map[string]*relatedGroup{}
	var groups []*relatedGroup
	for _, id := range captures {
		row, ok := rows[id]
		if !ok {
			row = captureKeyRow{id: id}
		}
		gid := sourceKeyOf(row)
		g := byKey[gid]
		if g == nil {
			g = &relatedGroup{own: map[string]bool{}}
			switch {
			case strings.HasPrefix(gid, "canonical|"):
				g.owner, g.computer, g.column, g.key = row.owner, row.computer, "canonical_key", row.canonicalKey
			case strings.HasPrefix(gid, "url|"):
				g.owner, g.computer, g.column, g.key = row.owner, row.computer, "url_key", row.urlKey
			}
			byKey[gid] = g
			groups = append(groups, g)
		}
		g.own[id] = true
		g.live = g.live || isLivePageURL(row.url) || isLivePageURL(row.canonicalURL)
		g.add(id)
	}
	for _, g := range groups {
		if g.column == "" {
			continue
		}
		siblings, err := s.store.capturesSharingKey(ctx, g.owner, g.computer, g.column, g.key, t.MaxSiblings)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			log.Printf("corpusd: related: siblings unavailable, using exact captures: %v", err)
			continue
		}
		for _, id := range siblings {
			g.add(id)
		}
	}
	return groups, nil
}

// relatedPublicationOf maps a version to its publication through the
// has_version edge that points at it. "" means no live publication has it.
func (s *Service) relatedPublicationOf(ctx context.Context, og *ObjectGraphStore, version string) (string, error) {
	if p, ok := s.related.pubOf(version); ok {
		return p, nil
	}
	live := false
	edges, err := og.ListEdges(ctx, objectgraph.EdgeFilter{ToID: version, Kind: "has_version", Tombstone: &live, Limit: 4})
	if err != nil {
		return "", fmt.Errorf("related: publication of %s: %w", version, err)
	}
	if len(edges) == 0 {
		return "", nil
	}
	s.related.setPubOf(version, edges[0].FromID)
	return edges[0].FromID, nil
}

type relatedPubInfo struct {
	obj      objectgraph.Object
	eligible bool   // a published Autopaper story
	latest   string // the canonical id of its latest version
}

// relatedPublications reads the publications once: whether each is a
// published Autopaper story, and which version is its latest.
func (s *Service) relatedPublications(ctx context.Context, og *ObjectGraphStore, ids map[string]bool) (map[string]relatedPubInfo, error) {
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	sort.Strings(list)
	objs := map[string]objectgraph.Object{}
	const chunk = 100
	for i := 0; i < len(list); i += chunk {
		end := i + chunk
		if end > len(list) {
			end = len(list)
		}
		got, err := og.GetObjects(ctx, list[i:end])
		if err != nil {
			return nil, fmt.Errorf("related: publications: %w", err)
		}
		for id, obj := range got {
			objs[id] = obj
		}
	}
	out := make(map[string]relatedPubInfo, len(objs))
	for id, obj := range objs {
		info := relatedPubInfo{obj: obj}
		var meta frontPublicationMeta
		if err := json.Unmarshal(obj.Metadata, &meta); err == nil && !obj.Tombstone && meta.State == "published" && s.isAutopaperStory(meta) {
			versions, err := publishedVersions(ctx, og, id)
			if err != nil {
				return nil, fmt.Errorf("related: versions of %s: %w", id, err)
			}
			if len(versions) > 0 {
				info.eligible = true
				info.latest = versions[len(versions)-1].CanonicalID
			}
		}
		out[id] = info
	}
	return out, nil
}

// relatedStoryCount is idf's N: the number of publications of the allowed
// owners, counted by the (object_kind, owner_id) index and cached for the
// TTL. It counts every publication of those owners, so it can only be a
// little high, which makes every idf a little higher and the hub cutoff
// (a count of stories, not of idf) unaffected.
func (s *Service) relatedStoryCount(ctx context.Context, t relatedTuning, now time.Time) int {
	c := &s.related
	c.mu.Lock()
	c.initLocked()
	if c.storyN > 0 && now.Sub(c.storyNAt) < t.TTL {
		n := c.storyN
		c.mu.Unlock()
		return n
	}
	c.mu.Unlock()
	query := `SELECT COUNT(*) FROM og_objects WHERE object_kind = 'choir.publication' AND tombstone = FALSE`
	var args []any
	s.frontMu.RLock()
	if len(s.autopaperOwners) > 0 {
		marks := make([]string, 0, len(s.autopaperOwners))
		for owner := range s.autopaperOwners {
			marks = append(marks, "?")
			args = append(args, owner)
		}
		query += ` AND owner_id IN (` + strings.Join(marks, ",") + `)`
	}
	s.frontMu.RUnlock()
	var n int
	if err := s.store.corpus().QueryRowContext(ctx, query, args...).Scan(&n); err != nil || n < 1 {
		return 1
	}
	c.mu.Lock()
	c.storyN, c.storyNAt = n, now
	c.mu.Unlock()
	return n
}

// HandleInternalRelatedStories handles GET
// /internal/platform/publications/related?route=<bare or /v<n> route>&limit=<n>.
// The proxy forwards the public route here. Unknown, held and
// non-Autopaper routes answer 200 with links [].
func (h *Handler) HandleInternalRelatedStories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "route is required"})
		return
	}
	limit := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "limit must be a non-negative integer"})
			return
		}
		limit = n
	}
	out, err := h.service.RelatedStories(r.Context(), routePath, limit)
	if err != nil {
		log.Printf("corpusd: related stories %s: %v", routePath, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list related stories"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}
