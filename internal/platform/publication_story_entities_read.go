package platform

// The entities a story is centrally about, read for the front list and the
// bundle so a client can shade its ranking by "more of this"
// (docs/design/autoradio-app-ui-2026-11-10.md, section 1, server gap 1).
// It reads what the research desk already wrote (mentions_entity edges with
// role central, publication_story_entities.go); it makes no model call and
// writes nothing. Ways it can fail, written before the code:
//
//  E1 An incidental mention is served as a subject (the phone would boost
//     "Serbia" for every story that names it once).
//  E2 A retracted (tombstoned) mention, or one whose role changed to
//     incidental, is still served.
//  E3 A story without entities serves null or an error instead of an absent
//     field, or one unreadable entity row fails the whole front list.
//  E4 The answer is unbounded or unordered, so the same story serves a
//     different list on each read.
//  E5 An entity row's body leaks beyond {id, label} (owner, wikidata, aliases).
//  E6 More than 12 entities serve, or the 12 differ between reads (the edges
//     hold no weight, so the choice is canonical-key order, fixed).
//  E7 The cost grows with the page: a query pair per row.

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// StoryEntityRef is one central entity of a story: its id and display name.
type StoryEntityRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// frontEntityChunk bounds the stories one edge query names.
const frontEntityChunk = 40

// storiesCentralEntities returns, for each story, its live central entities:
// at most maxStoryEntitiesPerRequest, in canonical entity-key order. The mention edges
// carry only a role (no weight, salience or rank), so there is nothing
// meaningful to order by but the key; the order is fixed so a story serves the
// same list on every read. The cost for any number of stories is one edge
// query per frontEntityChunk stories and one object query for all their
// entities. A read that fails answers nothing: the entities are a hint, never
// a reason to drop a story.
func storiesCentralEntities(ctx context.Context, og *ObjectGraphStore, publicationCanonicals []string) map[string][]StoryEntityRef {
	out := map[string][]StoryEntityRef{}
	byStory := map[string][]string{}
	seen := map[string]bool{}
	for start := 0; start < len(publicationCanonicals); start += frontEntityChunk {
		end := start + frontEntityChunk
		if end > len(publicationCanonicals) {
			end = len(publicationCanonicals)
		}
		chunk := publicationCanonicals[start:end]
		edges, err := og.ListEdgesFromAny(ctx, chunk, edgeMentionsEntity, len(chunk)*mentionEdgeLimit)
		if err != nil {
			continue
		}
		for _, e := range edges {
			if mentionRole(e) == types.EntityRoleCentral && !seen[e.FromID+"|"+e.ToID] {
				seen[e.FromID+"|"+e.ToID] = true
				byStory[e.FromID] = append(byStory[e.FromID], e.ToID)
			}
		}
	}
	var all []string
	for story, ids := range byStory {
		sort.Strings(ids)
		if len(ids) > maxStoryEntitiesPerRequest {
			ids = ids[:maxStoryEntitiesPerRequest]
		}
		byStory[story] = ids
		all = append(all, ids...)
	}
	if len(all) == 0 {
		return out
	}
	rows, err := og.GetObjects(ctx, all)
	if err != nil {
		return out
	}
	for story, ids := range byStory {
		for _, id := range ids {
			obj, ok := rows[id]
			if !ok {
				continue
			}
			var row entityRow
			if json.Unmarshal(obj.Metadata, &row) != nil || strings.TrimSpace(row.EntityID) == "" || strings.TrimSpace(row.Name) == "" {
				continue
			}
			out[story] = append(out[story], StoryEntityRef{ID: row.EntityID, Label: row.Name})
		}
	}
	return out
}

// storyCentralEntities is storiesCentralEntities for one story.
func storyCentralEntities(ctx context.Context, og *ObjectGraphStore, publicationCanonical string) []StoryEntityRef {
	return storiesCentralEntities(ctx, og, []string{publicationCanonical})[publicationCanonical]
}

// attachFrontEntities sets Entities on a page of rows with one batched read.
func (s *Service) attachFrontEntities(ctx context.Context, og *ObjectGraphStore, stories []FrontStory) {
	ids := make([]string, 0, len(stories))
	for _, st := range stories {
		if st.canonical != "" {
			ids = append(ids, st.canonical)
		}
	}
	got := storiesCentralEntities(ctx, og, ids)
	for i := range stories {
		stories[i].Entities = got[stories[i].canonical]
	}
}
