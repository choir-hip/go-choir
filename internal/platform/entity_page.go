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
	// Mentions and CentralMentions count the publications with a recorded
	// mention of the entity (the role central for the second), from the newest
	// 200 mention edges. They are not checked for the public route: the list
	// below is, and may be shorter. CountsAtLeast says the edge read was full,
	// so the entity may have more.
	Mentions        int               `json:"mentions"`
	CentralMentions int               `json:"central_mentions"`
	CountsAtLeast   bool              `json:"counts_at_least,omitempty"`
	StoryList       []EntityPageStory `json:"story_list"`
	CoEntities      []types.EntityRef `json:"co_entities"`
}

type entityPageEntry struct {
	at   time.Time
	page *EntityPage // nil: no page
}

type entityPageCache struct {
	mu sync.Mutex
	m  map[string]entityPageEntry
}

// EntityPage answers the entity id's page, nil when there is none (unknown,
// retired, or a person who has no page).
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
	key := entityID + "|" + strconv.Itoa(stories)
	now := time.Now()
	s.entityPages.mu.Lock()
	if e, ok := s.entityPages.m[key]; ok && now.Sub(e.at) < entityPageCacheTTL {
		s.entityPages.mu.Unlock()
		return e.page, nil
	}
	s.entityPages.mu.Unlock()

	page, err := s.computeEntityPage(ctx, entityID, stories)
	if err != nil {
		return nil, err
	}
	s.entityPages.mu.Lock()
	if s.entityPages.m == nil {
		s.entityPages.m = map[string]entityPageEntry{}
	}
	if _, have := s.entityPages.m[key]; !have && len(s.entityPages.m) >= entityPageCacheMax {
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
	s.entityPages.m[key] = entityPageEntry{at: now, page: page}
	s.entityPages.mu.Unlock()
	return page, nil
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
	edges, err := s.entityMentionsNewest(ctx, canonical, entityPageEdgeLimit)
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
		Aliases: row.Aliases, CountsAtLeast: len(edges) >= entityPageEdgeLimit,
		StoryList: []EntityPageStory{}, CoEntities: []types.EntityRef{}}
	for _, p := range pubs {
		page.Mentions++
		if p.role == types.EntityRoleCentral {
			page.CentralMentions++
		}
	}

	// The walk: the newest publications, checked for the public route in
	// small chunks, until the list is full and a person's page is decided.
	needsGate := row.Type == types.EntityTypePerson && row.WikidataID == ""
	publicCentral := 0
	for start := 0; start < len(pubs) && start < entityPageMaxChecks; start += entityPageChunk {
		if len(page.StoryList) >= limit && (!needsGate || publicCentral >= entityPageMinPeopleStories) {
			break
		}
		chunk := pubs[start:min(start+entityPageChunk, len(pubs), entityPageMaxChecks)]
		ids := map[string]bool{}
		for _, p := range chunk {
			ids[p.pub] = true
		}
		infos, err := s.relatedPublications(ctx, og, ids)
		if err != nil {
			return nil, err
		}
		for _, p := range chunk {
			info, ok := infos[p.pub]
			if !ok || !info.eligible {
				continue
			}
			story, ok, err := s.frontStory(ctx, og, frontPublicationRow{CanonicalID: p.pub, Metadata: info.obj.Metadata, UpdatedAt: info.obj.UpdatedAt.UTC()})
			if err != nil {
				return nil, err
			}
			if !ok {
				continue // not on the public route
			}
			if p.role == types.EntityRoleCentral {
				publicCentral++
			}
			if len(page.StoryList) < limit {
				page.StoryList = append(page.StoryList, EntityPageStory{FrontStory: story, Role: p.role})
			}
		}
	}
	if needsGate && publicCentral < entityPageMinPeopleStories {
		return nil, nil
	}
	page.CoEntities = s.entityPageCoEntities(ctx, og, canonical, page.StoryList)
	return page, nil
}

// entityMentionsNewest is the entity's live mention edges from publications,
// newest first. A capture's link to the entity (station E3) is not a story's
// mention and must not fill the window the page reads (it uses
// idx_og_edges_to_kind_created).
func (s *Service) entityMentionsNewest(ctx context.Context, canonical string, limit int) ([]objectgraph.Edge, error) {
	rows, err := s.store.corpus().QueryContext(ctx, `SELECT edge_id, from_id, metadata, created_at FROM og_edges
		WHERE to_id = ? AND kind = ? AND NOT tombstone
			AND from_id >= 'obj:choir.publication:' AND from_id < 'obj:choir.publication;'
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
