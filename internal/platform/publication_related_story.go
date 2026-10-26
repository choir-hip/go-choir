package platform

// The related_story edge of the related-stories walk (docs/problems/related-
// stories-thin-no-subject-nodes-2026-10-20.md, "Build: the related_story
// edge"; design docs/reports/related-stories-phase3-research-review-2026-10-20.md).
// Two stories that share no capture can still be about one subject. The
// research desk reads both, asks System 1, and records a judged pair on its
// tape (choir.RelateStories); the computer exports the pair here as one
// related_story edge between the two publications, and the walk reads it as
// an extra candidate.
//
// The tape is the authority (design R7); this edge is its one-way, re-runnable
// export. The edge is symmetric, stored once with from < to, and its metadata
// is exactly {"relation":"<relation>"}, so its id is deterministic and a
// repeat writes the same edge. The probability, the System 1 receipt and the
// reason stay on the tape.
//
// Two switches, both off by default. The writing computer's research.related_story
// setting gates the verb. CORPUSD_RELATED_STORY=on gates the read below
// (corpusd-retrieval.env and a corpusd restart, as CORPUSD_CAPTURE_SEARCH),
// so pairs can be written and audited before any reader sees them. The
// capture threshold and the capture hub cutoff are not touched.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// edgeRelatedStory is the edge between two publications the research desk
// judged related. It is read only by the related-stories walk, and only with
// the read switch on.
const edgeRelatedStory objectgraph.EdgeKind = "related_story"

// relatedStoryEnv is the read switch: CORPUSD_RELATED_STORY=on makes the walk
// read related_story edges. Absent or any other value, it does not. Read per
// request, so it flips with a corpusd restart and no deploy.
const relatedStoryEnv = "CORPUSD_RELATED_STORY"

func relatedStoryOn() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(relatedStoryEnv)), "on")
}

// relatedModeRelations is the basis mode of an answer that read the edges.
const relatedModeRelations = "captures+relations"

// maxStoryRelationsPerRequest bounds one export; the desk's verb takes the
// same 12.
const maxStoryRelationsPerRequest = 12

// relatedStoryEdgeLimit bounds one edge read of a publication.
const relatedStoryEdgeLimit = 200

// storyRelationRank orders the relations, strongest first. A relation not in
// the table is not accepted.
var storyRelationRank = map[string]int{
	types.StoryRelationSameIncident:     0,
	types.StoryRelationLaterDevelopment: 1,
	types.StoryRelationSameSubject:      2,
}

// relatedStoryMetadata is the edge's whole metadata for a relation.
func relatedStoryMetadata(relation string) json.RawMessage {
	return json.RawMessage(`{"relation":"` + relation + `"}`)
}

// relatedStoryRelation reads the relation of an edge, "" when it has none the
// table knows.
func relatedStoryRelation(e objectgraph.Edge) string {
	var meta struct {
		Relation string `json:"relation"`
	}
	if json.Unmarshal(e.Metadata, &meta) != nil {
		return ""
	}
	if _, ok := storyRelationRank[meta.Relation]; !ok {
		return ""
	}
	return meta.Relation
}

// relatedStoryNeighbors are the publications joined to pub by a live
// related_story edge, in either direction, with the strongest relation of the
// pair. Self edges and edges whose relation is unknown are ignored.
func (s *Service) relatedStoryNeighbors(ctx context.Context, og *ObjectGraphStore, pub string) (map[string]string, error) {
	live := false
	out := map[string]string{}
	for _, f := range []objectgraph.EdgeFilter{
		{FromID: pub, Kind: edgeRelatedStory, Tombstone: &live, Limit: relatedStoryEdgeLimit},
		{ToID: pub, Kind: edgeRelatedStory, Tombstone: &live, Limit: relatedStoryEdgeLimit},
	} {
		edges, err := og.ListEdges(ctx, f)
		if err != nil {
			return nil, fmt.Errorf("related: related_story edges of %s: %w", pub, err)
		}
		for _, e := range edges {
			other := e.ToID
			if e.ToID == pub {
				other = e.FromID
			}
			rel := relatedStoryRelation(e)
			if other == pub || rel == "" {
				continue
			}
			if have, ok := out[other]; !ok || storyRelationRank[rel] < storyRelationRank[have] {
				out[other] = rel
			}
		}
	}
	return out, nil
}

// relatedStoryCandidates are the stories the research desk judged related to
// self, as walk candidates. A story with more than MaxRelatedEdges neighbours
// is a hub for this signal and has no vote, in its own list and in its
// neighbours'. A neighbour must still be a published Autopaper story with a
// latest version, and must pass frontStory, now.
func (s *Service) relatedStoryCandidates(ctx context.Context, og *ObjectGraphStore, selfPub string, t relatedTuning) ([]relatedCandidate, error) {
	neighbors, err := s.relatedStoryNeighbors(ctx, og, selfPub)
	if err != nil {
		return nil, err
	}
	if len(neighbors) == 0 || len(neighbors) > t.MaxRelatedEdges {
		return nil, nil
	}
	ids := map[string]bool{}
	for p := range neighbors {
		ids[p] = true
	}
	pubs, err := s.relatedPublications(ctx, og, ids)
	if err != nil {
		return nil, err
	}
	var order []string
	for p := range neighbors {
		if info, ok := pubs[p]; ok && info.eligible {
			order = append(order, p)
		}
	}
	sort.Strings(order)
	var stories []FrontStory
	var relations []string
	for _, p := range order {
		other, err := s.relatedStoryNeighbors(ctx, og, p)
		if err != nil {
			return nil, err
		}
		if len(other) > t.MaxRelatedEdges {
			continue
		}
		info := pubs[p]
		row, ok, err := s.frontStory(ctx, og, frontPublicationRow{CanonicalID: p, Metadata: info.obj.Metadata, UpdatedAt: info.obj.UpdatedAt.UTC()})
		if err != nil {
			return nil, err
		}
		if ok {
			stories = append(stories, row)
			relations = append(relations, neighbors[p])
		}
	}
	s.attachFrontImportance(stories)
	cands := make([]relatedCandidate, 0, len(stories))
	for i, story := range stories {
		cands = append(cands, relatedCandidate{Story: story, Relation: relations[i]})
	}
	return cands, nil
}

// mergeRelatedStoryCandidates adds the judged candidates to the capture
// candidates: a story both found keeps its capture tally and takes the
// judged relation (the pair is listed once).
func mergeRelatedStoryCandidates(cands, judged []relatedCandidate) []relatedCandidate {
	byPub := map[string]int{}
	for i, c := range cands {
		byPub[c.Story.PublicationID] = i
	}
	for _, j := range judged {
		if i, ok := byPub[j.Story.PublicationID]; ok {
			cands[i].Relation = j.Relation
			continue
		}
		byPub[j.Story.PublicationID] = len(cands)
		cands = append(cands, j)
	}
	return cands
}

// publicationCanonicalFor is the canonical id of an owner's publication from
// its public id (pub-<32 hex>); the same derivation publication identity uses.
func publicationCanonicalFor(ownerID, publicationID string) (string, error) {
	if !publicationPublicIDRE.MatchString(publicationID) {
		return "", fmt.Errorf("not a publication id")
	}
	return objectgraph.BuildCanonicalID("choir.publication", ownerID, objectgraph.StableSuffixFromKey(publicationID))
}

var publicationPublicIDRE = regexp.MustCompile(`^pub-[0-9a-f]{32}$`)

// storyRelationPair is one relation between two canonical publication ids,
// stored once with a < b.
type storyRelationPair struct{ a, b, relation string }

// RecordStoryRelations stores a computer's judged pair relations in Store B
// as related_story edges. It accepts a platform-tier computer only (a private
// computer cannot write the public paper's graph), only pairs of that
// owner's published Autopaper stories that the public route shows, and no
// pair that would take a story past MaxRelatedEdges neighbours. The request
// is checked whole before anything is written. It returns how many distinct
// edges it wrote; with ValidateOnly it runs the same checks, writes nothing
// and returns how many distinct edges it would write.
func (s *Service) RecordStoryRelations(ctx context.Context, req types.StoryRelationsRequest) (int, error) {
	pairs, og, err := s.checkStoryRelations(ctx, req)
	if err != nil {
		return 0, err
	}
	if req.ValidateOnly {
		return len(pairs), nil
	}
	now := time.Now().UTC()
	for _, p := range pairs {
		edge := makeEdge(p.a, p.b, edgeRelatedStory, now, relatedStoryMetadata(p.relation))
		if err := og.PutEdge(ctx, edge); err != nil {
			return 0, fmt.Errorf("story relations: write %s: %w", edge.EdgeID, err)
		}
	}
	s.related.reset()
	return len(pairs), nil
}

// checkStoryRelations is every check of a relations request, shared by the
// write and the validate-only call so the two cannot disagree. It returns the
// distinct pairs to write.
func (s *Service) checkStoryRelations(ctx context.Context, req types.StoryRelationsRequest) ([]storyRelationPair, *ObjectGraphStore, error) {
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" {
		return nil, nil, fmt.Errorf("owner_id is required")
	}
	if computer == "" {
		return nil, nil, fmt.Errorf("computer_id is required")
	}
	if n := len(req.Relations); n == 0 || n > maxStoryRelationsPerRequest {
		return nil, nil, fmt.Errorf("a relations request carries 1 to %d relations, got %d", maxStoryRelationsPerRequest, n)
	}
	tier := s.relationTier
	if tier == nil {
		tier = platformComputer
	}
	if platform, err := tier(computer); err != nil || !platform {
		return nil, nil, fmt.Errorf("story relations are accepted from a platform computer only")
	}
	og := s.ogStore()
	if og == nil {
		return nil, nil, fmt.Errorf("story relations: store unavailable")
	}

	var pairs []storyRelationPair
	seen := map[storyRelationPair]bool{}
	ids := map[string]bool{}
	for i, r := range req.Relations {
		a, b, relation := strings.TrimSpace(r.StoryA), strings.TrimSpace(r.StoryB), strings.TrimSpace(r.Relation)
		if _, known := storyRelationRank[relation]; !known {
			return nil, nil, fmt.Errorf("relation %d: unknown relation %q", i+1, relation)
		}
		if a == b {
			return nil, nil, fmt.Errorf("relation %d: a story cannot be related to itself", i+1)
		}
		var canon [2]string
		for j, id := range []string{a, b} {
			c, err := publicationCanonicalFor(owner, id)
			if err != nil {
				return nil, nil, fmt.Errorf("relation %d: %q is not a publication id", i+1, id)
			}
			canon[j] = c
		}
		// Edges join canonical ids, stored once with from < to.
		a, b = canon[0], canon[1]
		if a > b {
			a, b = b, a
		}
		p := storyRelationPair{a, b, relation}
		if seen[p] {
			continue
		}
		seen[p] = true
		pairs = append(pairs, p)
		ids[a], ids[b] = true, true
	}

	pubs, err := s.relatedPublications(ctx, og, ids)
	if err != nil {
		return nil, nil, err
	}
	for id := range ids {
		info, ok := pubs[id]
		if !ok || !info.eligible {
			return nil, nil, fmt.Errorf("%s is not a published Autopaper story of owner %s", id, owner)
		}
		if info.obj.OwnerID != owner {
			return nil, nil, fmt.Errorf("%s is not a published Autopaper story of owner %s", id, owner)
		}
		// The read shows a story only on the public route; a story it would
		// never show must not take one of the 20 edges of its neighbours.
		manifest, err := s.frontManifest(ctx, og, info.latest)
		if err != nil {
			return nil, nil, err
		}
		if !publicationAccessPolicyAllowsPublicRoute(manifest.AccessPolicy) {
			return nil, nil, fmt.Errorf("%s is not on the public route, so a relation to it would never be shown", id)
		}
	}

	// The degree bound: a story may have MaxRelatedEdges distinct neighbours.
	bound := s.related.tune().MaxRelatedEdges
	neighbors := map[string]map[string]bool{}
	load := func(id string) (map[string]bool, error) {
		if set, ok := neighbors[id]; ok {
			return set, nil
		}
		have, err := s.relatedStoryNeighbors(ctx, og, id)
		if err != nil {
			return nil, err
		}
		set := map[string]bool{}
		for p := range have {
			set[p] = true
		}
		neighbors[id] = set
		return set, nil
	}
	for _, p := range pairs {
		sa, err := load(p.a)
		if err != nil {
			return nil, nil, err
		}
		sb, err := load(p.b)
		if err != nil {
			return nil, nil, err
		}
		for _, end := range []struct {
			id, other string
			set       map[string]bool
		}{{p.a, p.b, sa}, {p.b, p.a, sb}} {
			if !end.set[end.other] && len(end.set) >= bound {
				return nil, nil, fmt.Errorf("story %s is at its bound of %d related stories; relations for it are refused", end.id, bound)
			}
			end.set[end.other] = true
		}
	}

	return pairs, og, nil
}

// HandleStoryRelations serves POST types.StoryRelationsPath for a computer
// exporting its judged pair relations (straight from the guest, whose platform
// URL is corpusd, or through the proxy) under the one bound-guest write rule.
func (h *Handler) HandleStoryRelations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req types.StoryRelationsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if err := requireInternalOrBoundGuestWrite(r.Context(), h.guestLookup, r, req.OwnerID, req.ComputerID); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	n, err := h.service.RecordStoryRelations(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: story relations from %s: %v", strings.TrimSpace(req.ComputerID), err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	if req.ValidateOnly {
		writeJSON(w, http.StatusOK, map[string]any{"validated": n})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recorded": n})
}
