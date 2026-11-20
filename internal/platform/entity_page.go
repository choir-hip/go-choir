package platform

// The public read of one entity (docs/design/entity-layer-2026-11-20.md
// section 7, station E2): GET /api/platform/entities/{id}, no key, served
// from corpusd's Store B alone with no computer in the path. It answers an
// entity of the paper's graph with its aliases, counts, and its newest
// published stories (each with the role the entity plays in it), and the
// entities that most often share those stories.
//
// Strangers call it, so it is bounded: the id must have an entity id's
// shape (checked before any read, at the proxy and here); a story walk reads
// at most entityPageMaxChecks publications and 200 mention edges; the list is
// at most entityPageMaxStories long; answers (and misses) are cached for a
// minute in a table of bounded size that evicts its oldest entry. These bound what strangers can cause;
// they are not limits on any model call.
//
// People: a private person named in a story (a pedestrian struck, a victim)
// must not get a page that gathers every story that names them. A person is
// served only with a Wikidata id, or as the central entity of at least two
// published public stories (owner question 1, default). Any other person
// answers 404, the same as an id that does not exist, so the read does not
// say who is in the graph. Organisations, places, products, works and laws
// are always served. Their links from story reads are unchanged.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	// EntityPageInternalPrefix is corpusd's route; the proxy forwards
	// types.EntityPagePrefix to it.
	EntityPageInternalPrefix = "/internal/platform/entities/"

	entityPageDefaultStories = 20
	entityPageMaxStories     = 50
	// entityPageEdgeLimit bounds the mention edges read, newest first.
	entityPageEdgeLimit = 200
	// entityPageMaxChecks bounds the publications whose public eligibility
	// one read checks.
	entityPageMaxChecks = 60
	entityPageChunk     = 10
	entityPageCoMax     = 5
	// entityPageCoStories is how many of the listed stories the co-entities
	// are read from.
	entityPageCoStories = 10
	// entityPageGateChecks bounds the central edges a person's gate reads.
	entityPageGateChecks = 40
	// entityPageCacheTTL and entityPageCacheMax bound the answer cache.
	entityPageCacheTTL = time.Minute
	entityPageCacheMax = 1000
	// entityPageMinPeopleStories is the central, published stories a person
	// needs to have a page without a Wikidata id.
	entityPageMinPeopleStories = 2
)

// EntityPageStory is one published story the entity is about.
type EntityPageStory struct {
	FrontStory
	// Role is the entity's role in it: central or incidental.
	Role string `json:"role"`
}

// EntityPage is the public read of one entity.
type EntityPage struct {
	EntityID   string   `json:"entity_id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	WikidataID string   `json:"wikidata_id,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	// Stories and CentralStories count the public stories the walk verified
	// (the role central for the second), among the newest 200 mention edges and
	// at most 60 publications. CountsAtLeast says the walk did not reach every
	// mention, so the entity may have more.
	Stories        int               `json:"stories"`
	CentralStories int               `json:"central_stories"`
	CountsAtLeast  bool              `json:"counts_at_least,omitempty"`
	StoryList      []EntityPageStory `json:"story_list"`
	CoEntities     []types.EntityRef `json:"co_entities"`
}

type entityPageEntry struct {
	at   time.Time
	page *EntityPage // nil: no page
}

type entityPageCache struct {
	mu sync.Mutex
	m  map[string]entityPageEntry
}

// entityPageDeadline bounds one walk (a variable so a test can move it). A
// walk that overruns answers an error and is not cached.
var entityPageDeadline = 8 * time.Second

// entityPageWalkHook is called once per walk (a test counts them).
var entityPageWalkHook func()

// EntityPage answers the entity id's page, nil when there is none (unknown,
// retired, or a person who has no page). One walk per entity answers every
// `stories` value: the page is computed at the largest size, cached once under
// the entity's id, and cut to the size asked on read.
func (s *Service) EntityPage(ctx context.Context, entityID string, stories int) (*EntityPage, error) {
	if !types.EntityIDRE.MatchString(entityID) {
		return nil, fmt.Errorf("not an entity id")
	}
	switch {
	case stories <= 0:
		stories = entityPageDefaultStories
	case stories > entityPageMaxStories:
		stories = entityPageMaxStories
	}
	now := time.Now()
	s.entityPages.mu.Lock()
	if e, ok := s.entityPages.m[entityID]; ok && now.Sub(e.at) < entityPageCacheTTL {
		s.entityPages.mu.Unlock()
		return cutEntityPage(e.page, stories), nil
	}
	s.entityPages.mu.Unlock()

	if entityPageWalkHook != nil {
		entityPageWalkHook()
	}
	wctx, cancel := context.WithTimeout(ctx, entityPageDeadline)
	defer cancel()
	page, err := s.computeEntityPage(wctx, entityID, entityPageMaxStories)
	if err != nil {
		return nil, err
	}
	s.entityPages.mu.Lock()
	if s.entityPages.m == nil {
		s.entityPages.m = map[string]entityPageEntry{}
	}
	if _, have := s.entityPages.m[entityID]; !have && len(s.entityPages.m) >= entityPageCacheMax {
		// Full: drop the oldest entry (and any expired one met), not the
		// table, so a stranger cannot keep the hits cold by filling it.
		oldest, oldestAt := "", now
		for k, e := range s.entityPages.m {
			if now.Sub(e.at) >= entityPageCacheTTL {
				delete(s.entityPages.m, k)
			} else if oldest == "" || e.at.Before(oldestAt) {
				oldest, oldestAt = k, e.at
			}
		}
		if len(s.entityPages.m) >= entityPageCacheMax && oldest != "" {
			delete(s.entityPages.m, oldest)
		}
	}
	s.entityPages.m[entityID] = entityPageEntry{at: now, page: page}
	s.entityPages.mu.Unlock()
	return cutEntityPage(page, stories), nil
}

// cutEntityPage is the page with at most n stories listed (a copy; the cached
// page is never changed).
func cutEntityPage(p *EntityPage, n int) *EntityPage {
	if p == nil {
		return nil
	}
	out := *p
	if len(out.StoryList) > n {
		out.StoryList = out.StoryList[:n:n]
	}
	return &out
}

// publicStoryMemo remembers, within one walk, which publications are on the
// public route and their rows.
type publicStoryMemo struct {
	s     *Service
	og    *ObjectGraphStore
	known map[string]*FrontStory // nil value: checked, not public
}

// check reports the public rows of the publications named, reading each once.
func (m *publicStoryMemo) check(ctx context.Context, pubs []string) error {
	ids := map[string]bool{}
	for _, p := range pubs {
		if _, ok := m.known[p]; !ok {
			ids[p] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	infos, err := m.s.relatedPublications(ctx, m.og, ids)
	if err != nil {
		return err
	}
	for p := range ids {
		info, ok := infos[p]
		if !ok || !info.eligible {
			m.known[p] = nil
			continue
		}
		story, ok, err := m.s.frontStory(ctx, m.og, frontPublicationRow{CanonicalID: p, Metadata: info.obj.Metadata, UpdatedAt: info.obj.UpdatedAt.UTC()})
		if err != nil {
			return err
		}
		if !ok {
			m.known[p] = nil // not on the public route
			continue
		}
		st := story
		m.known[p] = &st
	}
	return nil
}

func (s *Service) computeEntityPage(ctx context.Context, entityID string, limit int) (*EntityPage, error) {
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("entity page: store unavailable")
	}
	var row struct {
		EntityID   string   `json:"entity_id"`
		Name       string   `json:"name"`
		Type       string   `json:"type"`
		WikidataID string   `json:"wikidata_id"`
		Aliases    []string `json:"aliases"`
	}
	canonical := ""
	for _, owner := range s.autopaperOwnerList() {
		id, err := entityCanonicalFor(owner, entityID)
		if err != nil {
			return nil, err
		}
		obj, err := og.GetObject(ctx, id)
		if err != nil {
			if notFoundIsNil(err) != nil {
				return nil, fmt.Errorf("entity page: read %s: %w", entityID, err)
			}
			continue
		}
		if obj.Tombstone || json.Unmarshal(obj.Metadata, &row) != nil || row.EntityID != entityID || strings.TrimSpace(row.Name) == "" {
			continue
		}
		canonical = id
		break
	}
	if canonical == "" {
		return nil, nil
	}
	memo := &publicStoryMemo{s: s, og: og, known: map[string]*FrontStory{}}

	// The person gate comes first and from the central edges alone: a person
	// with no Wikidata id needs two public stories they are central to, however
	// old and however many incidental mentions came since. The edges are
	// capped (entityPageGateChecks, newest first), and the walk stops at two.
	if row.Type == types.EntityTypePerson && row.WikidataID == "" {
		central, err := s.entityMentionsNewest(ctx, canonical, entityPageGateChecks, true)
		if err != nil {
			return nil, err
		}
		public := 0
		seen := map[string]bool{}
		var pubs []string
		for _, e := range central {
			if !seen[e.FromID] {
				seen[e.FromID] = true
				pubs = append(pubs, e.FromID)
			}
		}
		for start := 0; start < len(pubs) && public < entityPageMinPeopleStories; start += entityPageChunk {
			chunk := pubs[start:min(start+entityPageChunk, len(pubs))]
			if err := memo.check(ctx, chunk); err != nil {
				return nil, err
			}
			for _, p := range chunk {
				if memo.known[p] != nil {
					public++
				}
			}
		}
		if public < entityPageMinPeopleStories {
			return nil, nil
		}
	}

	edges, err := s.entityMentionsNewest(ctx, canonical, entityPageEdgeLimit, false)
	if err != nil {
		return nil, err
	}
	// Distinct publications, newest mention first, each with its role (central
	// wins); a tentative edge is counted nowhere.
	type pubRole struct{ pub, role string }
	var pubs []pubRole
	at := map[string]int{}
	for _, e := range edges {
		role := mentionRole(e)
		if role != types.EntityRoleCentral && role != types.EntityRoleIncidental {
			continue
		}
		if i, ok := at[e.FromID]; ok {
			if role == types.EntityRoleCentral {
				pubs[i].role = role
			}
			continue
		}
		at[e.FromID] = len(pubs)
		pubs = append(pubs, pubRole{e.FromID, role})
	}
	page := &EntityPage{EntityID: entityID, Name: strings.TrimSpace(row.Name), Type: row.Type, WikidataID: row.WikidataID,
		Aliases: row.Aliases, StoryList: []EntityPageStory{}, CoEntities: []types.EntityRef{}}

	// The walk: the newest publications, checked for the public route in small
	// chunks. The counts are of what the walk verified, so a reader learns
	// nothing of a publication that is not public; CountsAtLeast says the walk
	// did not reach every mention (the edge read was full, or the check bound
	// came first), so the entity may have more.
	checked := min(len(pubs), entityPageMaxChecks)
	page.CountsAtLeast = len(edges) >= entityPageEdgeLimit || len(pubs) > checked
	for start := 0; start < checked; start += entityPageChunk {
		chunk := pubs[start:min(start+entityPageChunk, checked)]
		names := make([]string, len(chunk))
		for i, p := range chunk {
			names[i] = p.pub
		}
		if err := memo.check(ctx, names); err != nil {
			return nil, err
		}
		for _, p := range chunk {
			story := memo.known[p.pub]
			if story == nil {
				continue
			}
			page.Stories++
			if p.role == types.EntityRoleCentral {
				page.CentralStories++
			}
			if len(page.StoryList) < limit {
				page.StoryList = append(page.StoryList, EntityPageStory{FrontStory: *story, Role: p.role})
			}
		}
	}
	// Publications past the check bound that the person gate already verified
	// are counted and listed too (no further reads): an old story an entity is
	// central to is not lost behind newer mentions.
	for _, p := range pubs[checked:] {
		story := memo.known[p.pub]
		if story == nil {
			continue
		}
		page.Stories++
		if p.role == types.EntityRoleCentral {
			page.CentralStories++
		}
		if len(page.StoryList) < limit {
			page.StoryList = append(page.StoryList, EntityPageStory{FrontStory: *story, Role: p.role})
		}
	}
	page.CoEntities = s.entityPageCoEntities(ctx, og, canonical, page.StoryList[:min(len(page.StoryList), entityPageCoStories)])
	return page, nil
}

// entityMentionsNewest is the entity's live mention edges from publications,
// newest first. A capture's link to the entity (station E3) is not a story's
// mention and must not fill the window the page reads (it uses
// idx_og_edges_to_kind_created).
func (s *Service) entityMentionsNewest(ctx context.Context, canonical string, limit int, centralOnly bool) ([]objectgraph.Edge, error) {
	roleCond := ""
	if centralOnly {
		// The role is read from the edge's JSON, not matched as text.
		roleCond = ` AND CASE WHEN pg_input_is_valid(metadata, 'jsonb') THEN metadata::jsonb->>'role' = 'central' ELSE FALSE END`
	}
	rows, err := s.store.corpus().QueryContext(ctx, `SELECT edge_id, from_id, metadata, created_at FROM og_edges
		WHERE to_id = ? AND kind = ? AND NOT tombstone
			AND from_id >= 'obj:choir.publication:' AND from_id < 'obj:choir.publication;'`+roleCond+`
		ORDER BY created_at DESC, edge_id LIMIT ?`, canonical, string(edgeMentionsEntity), limit)
	if err != nil {
		return nil, fmt.Errorf("entity page: mentions of %s: %w", canonical, err)
	}
	defer rows.Close()
	var out []objectgraph.Edge
	for rows.Next() {
		e := objectgraph.Edge{ToID: canonical, Kind: edgeMentionsEntity}
		var meta string
		if err := rows.Scan(&e.EdgeID, &e.FromID, &meta, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Metadata = json.RawMessage(meta)
		out = append(out, e)
	}
	return out, rows.Err()
}

// entityPageCoEntities are the entities central to the listed stories that
// the page's entity shares most, as the story pages' "about" lists show them.
// A person without a Wikidata id is left out: their own page may not exist.
func (s *Service) entityPageCoEntities(ctx context.Context, og *ObjectGraphStore, self string, stories []EntityPageStory) []types.EntityRef {
	out := []types.EntityRef{}
	if len(stories) == 0 {
		return out
	}
	canon := make([]string, 0, len(stories))
	for _, st := range stories {
		canon = append(canon, st.canonical)
	}
	edges, err := og.ListEdgesFromAny(ctx, canon, edgeMentionsEntity, len(canon)*mentionEdgeLimit)
	if err != nil {
		return out
	}
	tally := map[string]int{}
	for _, e := range edges {
		if e.ToID != self && mentionRole(e) == types.EntityRoleCentral {
			tally[e.ToID]++
		}
	}
	ids := make([]string, 0, len(tally))
	for id := range tally {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if tally[ids[i]] != tally[ids[j]] {
			return tally[ids[i]] > tally[ids[j]]
		}
		return ids[i] < ids[j]
	})
	if len(ids) > 4*entityPageCoMax {
		ids = ids[:4*entityPageCoMax]
	}
	rows, err := og.GetObjects(ctx, ids)
	if err != nil {
		return out
	}
	for _, id := range ids {
		obj, ok := rows[id]
		if !ok || obj.Tombstone {
			continue
		}
		var r entityRow
		if json.Unmarshal(obj.Metadata, &r) != nil || r.EntityID == "" || strings.TrimSpace(r.Name) == "" {
			continue
		}
		if r.Type == types.EntityTypePerson && r.WikidataID == "" {
			continue
		}
		out = append(out, types.EntityRef{EntityID: r.EntityID, Name: r.Name, Type: r.Type})
		if len(out) == entityPageCoMax {
			break
		}
	}
	return out
}

// HandleInternalEntityPage serves GET /internal/platform/entities/{id}[?stories=n]
// to the proxy.
func (h *Handler) HandleInternalEntityPage(w http.ResponseWriter, r *http.Request) {
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	h.HandlePublicEntityPage(w, r)
}

// HandlePublicEntityPage answers GET {prefix}{id}[?stories=n].
func (h *Handler) HandlePublicEntityPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, EntityPageInternalPrefix)
	if !types.EntityIDRE.MatchString(id) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "not an entity id"})
		return
	}
	stories := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("stories")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > entityPageMaxStories {
			writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("stories is 1 to %d", entityPageMaxStories)})
			return
		}
		stories = n
	}
	page, err := h.service.EntityPage(r.Context(), id, stories)
	if err != nil {
		log.Printf("corpusd: entity page %s: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "entity read failed"})
		return
	}
	if page == nil {
		w.Header().Set("Cache-Control", "public, max-age=60")
		writeJSON(w, http.StatusNotFound, apiError{Error: "no page for this entity"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, page)
}
