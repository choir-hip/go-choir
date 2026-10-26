package platform

// Shared central entities as a related-stories signal
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice b;
// design docs/reports/related-stories-phase3-research-review-2026-10-20.md
// §4.1 and §5). Two stories are related when both are centrally about one
// entity (a mentions_entity edge with role central, written by the research
// desk's choir.Relate export, publication_story_entities.go). An incidental
// mention never links (R2). An entity central to more than HubEntityStories
// stories is a hub and has no vote, as a capture cited by too many stories has
// none. The read is on by default; CORPUSD_RELATED_ENTITIES=off (corpusd's
// env file and a restart, no deploy) drops the signal at once.

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// relatedEntitiesEnv is the entity read's kill switch.
const relatedEntitiesEnv = "CORPUSD_RELATED_ENTITIES"

// relatedModeEntitiesSuffix marks a basis mode that read the entity edges.
const relatedModeEntitiesSuffix = "+entities"

func relatedEntitiesOn() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(relatedEntitiesEnv)), "off")
}

// relatedEntityCandidates are the stories centrally about an entity the story
// is centrally about, each with how many such entities they share. A
// candidate must be a published Autopaper story and pass frontStory now.
func (s *Service) relatedEntityCandidates(ctx context.Context, og *ObjectGraphStore, selfPub string, t relatedTuning) ([]relatedCandidate, error) {
	live := false
	own, err := og.ListEdges(ctx, objectgraph.EdgeFilter{FromID: selfPub, Kind: edgeMentionsEntity, Tombstone: &live, Limit: mentionEdgeLimit})
	if err != nil {
		return nil, fmt.Errorf("related: entities of %s: %w", selfPub, err)
	}
	entities := map[string]bool{}
	for _, e := range own {
		if mentionRole(e) == types.EntityRoleCentral {
			entities[e.ToID] = true
		}
	}
	if len(entities) == 0 {
		return nil, nil
	}
	ordered := make([]string, 0, len(entities))
	for id := range entities {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)

	type tally struct {
		shared int
		newest int64 // the newest mention edge, a recency proxy before rows are read
	}
	tallies := map[string]*tally{}
	for _, entity := range ordered {
		edges, err := og.ListEdges(ctx, objectgraph.EdgeFilter{ToID: entity, Kind: edgeMentionsEntity, Tombstone: &live, Limit: t.MaxRawHits})
		if err != nil {
			return nil, fmt.Errorf("related: stories about %s: %w", entity, err)
		}
		if len(edges) >= t.MaxRawHits {
			continue // a hub by the read bound alone
		}
		citing := map[string]int64{}
		for _, e := range edges {
			if mentionRole(e) != types.EntityRoleCentral {
				continue
			}
			if at, ok := citing[e.FromID]; !ok || e.CreatedAt.Unix() > at {
				citing[e.FromID] = e.CreatedAt.Unix()
			}
		}
		citing[selfPub] = citing[selfPub] // the story counts toward its entity's stories
		if len(citing) > t.HubEntityStories {
			continue
		}
		for p, at := range citing {
			if p == selfPub {
				continue
			}
			ta := tallies[p]
			if ta == nil {
				ta = &tally{}
				tallies[p] = ta
			}
			ta.shared++
			if at > ta.newest {
				ta.newest = at
			}
		}
	}
	if len(tallies) == 0 {
		return nil, nil
	}

	// The strongest candidates become rows: more shared entities, then the
	// newest mention.
	picks := make([]string, 0, len(tallies))
	for p := range tallies {
		picks = append(picks, p)
	}
	sort.Slice(picks, func(i, j int) bool {
		a, b := tallies[picks[i]], tallies[picks[j]]
		if a.shared != b.shared {
			return a.shared > b.shared
		}
		if a.newest != b.newest {
			return a.newest > b.newest
		}
		return picks[i] < picks[j]
	})
	if len(picks) > t.MaxCandidates {
		picks = picks[:t.MaxCandidates]
	}
	ids := map[string]bool{}
	for _, p := range picks {
		ids[p] = true
	}
	pubs, err := s.relatedPublications(ctx, og, ids)
	if err != nil {
		return nil, err
	}
	var stories []FrontStory
	var order []string
	for _, p := range picks {
		info, ok := pubs[p]
		if !ok || !info.eligible {
			continue
		}
		row, ok, err := s.frontStory(ctx, og, frontPublicationRow{CanonicalID: p, Metadata: info.obj.Metadata, UpdatedAt: info.obj.UpdatedAt.UTC()})
		if err != nil {
			return nil, err
		}
		if ok {
			stories = append(stories, row)
			order = append(order, p)
		}
	}
	s.attachFrontImportance(stories)
	cands := make([]relatedCandidate, 0, len(stories))
	for i, story := range stories {
		cands = append(cands, relatedCandidate{Story: story, Entities: tallies[order[i]].shared})
	}
	return cands, nil
}

// mergeRelatedEntityCandidates adds the entity candidates: a story another
// signal found keeps its tally and judged relation and takes the shared
// entity count (the pair is listed once).
func mergeRelatedEntityCandidates(cands, shared []relatedCandidate) []relatedCandidate {
	byPub := map[string]int{}
	for i, c := range cands {
		byPub[c.Story.PublicationID] = i
	}
	for _, e := range shared {
		if i, ok := byPub[e.Story.PublicationID]; ok {
			cands[i].Entities = e.Entities
			continue
		}
		byPub[e.Story.PublicationID] = len(cands)
		cands = append(cands, e)
	}
	return cands
}
