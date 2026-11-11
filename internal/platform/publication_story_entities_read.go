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

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// StoryEntityRef is one central entity of a story: its id and display name.
type StoryEntityRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// storyCentralEntities returns the story's live central entities, at most
// maxStoryEntitiesPerRequest, ordered by id. A read that fails answers
// nothing: the entities are a hint, never a reason to drop a story.
func storyCentralEntities(ctx context.Context, og *ObjectGraphStore, publicationCanonical string) []StoryEntityRef {
	live := false
	edges, err := og.ListEdges(ctx, objectgraph.EdgeFilter{FromID: publicationCanonical, Kind: edgeMentionsEntity, Tombstone: &live, Limit: mentionEdgeLimit})
	if err != nil {
		return nil
	}
	var ids []string
	seen := map[string]bool{}
	for _, e := range edges {
		if mentionRole(e) == types.EntityRoleCentral && !seen[e.ToID] {
			seen[e.ToID] = true
			ids = append(ids, e.ToID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)
	if len(ids) > maxStoryEntitiesPerRequest {
		ids = ids[:maxStoryEntitiesPerRequest]
	}
	rows, err := og.GetObjects(ctx, ids)
	if err != nil {
		return nil
	}
	var out []StoryEntityRef
	for _, id := range ids {
		obj, ok := rows[id]
		if !ok {
			continue
		}
		var row entityRow
		if json.Unmarshal(obj.Metadata, &row) != nil || strings.TrimSpace(row.EntityID) == "" || strings.TrimSpace(row.Name) == "" {
			continue
		}
		out = append(out, StoryEntityRef{ID: row.EntityID, Label: row.Name})
	}
	return out
}
