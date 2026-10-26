package platform

// The entity layer of the Autopaper object graph
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice a;
// ontology docs/reports/claims-events-entities-ontology-2026-10-14.md §4.2,
// §5.2, §5.4 and D1 to D5; related stories, phase 3, §3.2 and §6 R1). The
// research desk reads a published story, asks System 1 which candidate
// entities it is centrally or incidentally about, and records its decision on
// its tape (choir.Relate, a story.entities event). The computer exports the
// attachments here: one choir.entity row per entity, written once, and one
// mentions_entity edge from the publication to the entity with the role.
//
// The tape is the authority (phase 3 R7); these rows and edges are its
// one-way, re-runnable export. An entity's key is its Wikidata id (wd:Q<n>)
// or the id its normalised name mints (ent:<hex>); the row carries the
// normalised name so a second node of another id with the same name (R1,
// sprawl) is refused with the match named. The edge's metadata is exactly
// {"role":"<role>"}, so its id is deterministic and a repeat writes the same
// edge; a role change tombstones the old role's edge.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	ogKindEntity       = objectgraph.ObjectKind("choir.entity")
	edgeMentionsEntity = objectgraph.EdgeKind("mentions_entity")

	// maxStoryEntitiesPerRequest bounds one export; the desk's verb takes
	// the same 12.
	maxStoryEntitiesPerRequest = 12
	// mentionEdgeLimit bounds one edge read of a publication's entities.
	mentionEdgeLimit = 200
	// maxReviewItems bounds a review's not-entity phrases and its aliases:
	// at most the 24 candidates' worth, with room.
	maxReviewItems = types.EntityReviewMaxItems
)

// entityRow is a choir.entity row's body: identity and display only (ontology
// §5.2: a canonical row holds no membership; membership is the edges).
type entityRow struct {
	EntityID   string `json:"entity_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	WikidataID string `json:"wikidata_id,omitempty"`
	Normalized string `json:"normalized"`
	// ComputerID is the computer whose judgment minted it.
	ComputerID string `json:"computer_id"`
	// Aliases are other forms of its name the learning loop recorded (slice
	// c0 §6), at most types.EntityAliasesMax.
	Aliases []string `json:"aliases,omitempty"`
}

// entityCanonicalFor is the canonical id of an owner's entity from its key.
func entityCanonicalFor(ownerID, entityID string) (string, error) {
	if !types.EntityIDRE.MatchString(entityID) {
		return "", fmt.Errorf("not an entity id")
	}
	return objectgraph.BuildCanonicalID(ogKindEntity, ownerID, objectgraph.StableSuffixFromKey(entityID))
}

func mentionMetadata(role string) json.RawMessage {
	return json.RawMessage(`{"role":"` + role + `"}`)
}

// mentionRole reads an edge's role, "" when it has none the layer knows.
func mentionRole(e objectgraph.Edge) string {
	var meta struct {
		Role string `json:"role"`
	}
	if json.Unmarshal(e.Metadata, &meta) != nil || !types.EntityRoles[meta.Role] {
		return ""
	}
	return meta.Role
}

// checkedEntity is one attachment ready to write.
type checkedEntity struct {
	canonical string
	role      string
	row       *objectgraph.Object // a row to write, for an entity Store B lacks
	lex       lexiconEntity       // the lexicon's entry for a row written
}

// RecordStoryEntities stores a computer's entity attachments of one published
// story in Store B. It accepts a platform-tier computer only, only a
// published Autopaper story of that owner on the public route, and checks the
// request whole before anything is written. It returns how many attachments
// it wrote; with ValidateOnly it runs the same checks, writes nothing and
// returns how many it would write.
func (s *Service) RecordStoryEntities(ctx context.Context, req types.StoryEntitiesRequest) (int, error) {
	pub, checked, og, err := s.checkStoryEntities(ctx, req)
	if err != nil {
		return 0, err
	}
	if req.ValidateOnly {
		return len(checked), nil
	}
	now := time.Now().UTC()
	live := false
	for _, c := range checked {
		if c.row != nil {
			if _, err := og.InsertObjectIfAbsent(ctx, *c.row); err != nil {
				return 0, storeFailed(fmt.Errorf("story entities: write %s: %w", c.canonical, err))
			}
			// The lexicon learns the name now: the next request matches it.
			s.noteEntityWritten(strings.TrimSpace(req.OwnerID), c.lex)
		}
		if err := og.PutEdge(ctx, makeEdge(pub, c.canonical, edgeMentionsEntity, now, mentionMetadata(c.role))); err != nil {
			return 0, storeFailed(fmt.Errorf("story entities: write the mention of %s: %w", c.canonical, err))
		}
		// One live role per story and entity: retract the other role's edge.
		edges, err := og.ListEdges(ctx, objectgraph.EdgeFilter{FromID: pub, ToID: c.canonical, Kind: edgeMentionsEntity, Tombstone: &live, Limit: 4})
		if err != nil {
			return 0, storeFailed(fmt.Errorf("story entities: read the mentions of %s: %w", c.canonical, err))
		}
		for _, e := range edges {
			if mentionRole(e) == c.role {
				continue
			}
			e.Tombstone = true
			if err := og.PutEdge(ctx, e); err != nil {
				return 0, storeFailed(fmt.Errorf("story entities: retract %s: %w", e.EdgeID, err))
			}
		}
	}
	if len(checked) > 0 {
		s.related.reset()
	}
	if req.Review != nil {
		if err := s.learnFromReview(ctx, og, strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), pub, *req.Review); err != nil {
			return 0, storeFailed(err)
		}
	}
	return len(checked), nil
}

// storeFailure marks an error of the store, not of the request: the route
// answers it 503, so a computer tells the platform failing from the
// platform refusing (review 304 item 2). An unmarked error is a refusal.
type storeFailure struct{ err error }

func (e storeFailure) Error() string { return e.err.Error() }
func (e storeFailure) Unwrap() error { return e.err }

func storeFailed(err error) error {
	if err == nil {
		return nil
	}
	return storeFailure{err: err}
}

// checkStoryEntities is every check of an entities request, shared by the
// write and the validate-only call so the two cannot disagree.
func (s *Service) checkStoryEntities(ctx context.Context, req types.StoryEntitiesRequest) (string, []checkedEntity, *ObjectGraphStore, error) {
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" {
		return "", nil, nil, fmt.Errorf("owner_id is required")
	}
	if computer == "" {
		return "", nil, nil, fmt.Errorf("computer_id is required")
	}
	min := 1
	if req.Review != nil {
		// A review may attach nothing: the story is about none of its
		// candidates, and the review's counts and lessons still count.
		min = 0
		if strings.TrimSpace(req.Review.RecordID) == "" {
			return "", nil, nil, fmt.Errorf("a review needs its record_id")
		}
		if len(req.Review.NotEntities) > maxReviewItems || len(req.Review.Aliases) > maxReviewItems {
			return "", nil, nil, fmt.Errorf("a review carries at most %d not-entities and %d aliases", maxReviewItems, maxReviewItems)
		}
	}
	if n := len(req.Entities); n < min || n > maxStoryEntitiesPerRequest {
		return "", nil, nil, fmt.Errorf("an entities request carries %d to %d entities, got %d", min, maxStoryEntitiesPerRequest, n)
	}
	tier := s.relationTier
	if tier == nil {
		tier = platformComputer
	}
	if platform, err := tier(computer); err != nil || !platform {
		return "", nil, nil, fmt.Errorf("story entities are accepted from a platform computer only")
	}
	og := s.ogStore()
	if og == nil {
		return "", nil, nil, storeFailed(fmt.Errorf("story entities: store unavailable"))
	}
	pub, err := publicationCanonicalFor(owner, strings.TrimSpace(req.Story))
	if err != nil {
		return "", nil, nil, fmt.Errorf("story %q is not a publication id", req.Story)
	}
	switch {
	case req.Pending && !req.ValidateOnly:
		return "", nil, nil, fmt.Errorf("a pending story's entities are checked validate-only; they are exported after the story publishes")
	case req.Pending:
		// Not published yet: every entity check below still runs.
	default:
		if err := s.checkPublicAutopaperStory(ctx, og, owner, pub); err != nil {
			return "", nil, nil, err
		}
	}

	now := time.Now().UTC()
	seen := map[string]bool{}
	// R1 within the request (review 298 F1): Store B holds no row of a new
	// entity yet, so two new entities of one normalised name under two ids
	// (a minted id and a Wikidata id) are compared here.
	newNames := map[string]string{} // normalised name -> the first new id
	var out []checkedEntity
	for i, e := range req.Entities {
		n := i + 1
		id, role := strings.TrimSpace(e.EntityID), strings.TrimSpace(e.Role)
		if !types.EntityRoles[role] {
			return "", nil, nil, fmt.Errorf("entity %d: unknown role %q", n, role)
		}
		canonical, err := entityCanonicalFor(owner, id)
		if err != nil {
			return "", nil, nil, fmt.Errorf("entity %d: %q is not an entity id", n, id)
		}
		if seen[canonical] {
			return "", nil, nil, fmt.Errorf("entity %d: %s is attached twice in this request", n, id)
		}
		seen[canonical] = true
		c := checkedEntity{canonical: canonical, role: role}
		existing, err := og.GetObject(ctx, canonical)
		exists := err == nil && !existing.Tombstone
		if err != nil && notFoundIsNil(err) != nil {
			return "", nil, nil, storeFailed(fmt.Errorf("entity %d: read %s: %w", n, id, err))
		}
		if !e.New {
			if !exists {
				return "", nil, nil, fmt.Errorf("entity %d: unknown entity %s; name a new entity by name and type", n, id)
			}
			out = append(out, c)
			continue
		}
		name, typ, qid := strings.TrimSpace(e.Name), strings.TrimSpace(e.Type), strings.TrimSpace(e.WikidataID)
		norm := types.NormalizeEntityName(name)
		if norm == "" {
			return "", nil, nil, fmt.Errorf("entity %d: a new entity needs a name", n)
		}
		if utf8.RuneCountInString(name) > types.EntityNameMaxRunes {
			return "", nil, nil, fmt.Errorf("entity %d: a name is at most %d characters", n, types.EntityNameMaxRunes)
		}
		if !types.EntityTypes[typ] {
			return "", nil, nil, fmt.Errorf("entity %d: unknown type %q", n, typ)
		}
		want := types.MintedEntityID(name)
		if qid != "" {
			if !types.WikidataQIDRE.MatchString(qid) {
				return "", nil, nil, fmt.Errorf("entity %d: wikidata_id %q is not a Wikidata item id", n, qid)
			}
			want = types.WikidataEntityID(qid)
		}
		if id != want {
			return "", nil, nil, fmt.Errorf("entity %d: id %s does not derive from its name or Wikidata id (want %s)", n, id, want)
		}
		if first, ok := newNames[norm]; ok && first != id {
			return "", nil, nil, fmt.Errorf("entity %d: %q has the normalised name of %s, new in this request; attach one node, not two", n, name, first)
		}
		newNames[norm] = id
		// R1: another node of the same normalised name is a duplicate.
		same, err := og.ListObjectsByMetadataForOwner(ctx, string(ogKindEntity), "$.normalized", norm, owner, 4)
		if err != nil {
			return "", nil, nil, storeFailed(fmt.Errorf("entity %d: look up %q: %w", n, name, err))
		}
		for _, other := range same {
			if other.CanonicalID == canonical || other.Tombstone {
				continue
			}
			var row entityRow
			_ = json.Unmarshal(other.Body, &row)
			return "", nil, nil, fmt.Errorf("entity %d: %q matches the existing entity %s (%q); attach it by entity_id", n, name, row.EntityID, row.Name)
		}
		if exists {
			// The same id is the same node: attach it.
			out = append(out, c)
			continue
		}
		row := entityRow{EntityID: id, Name: name, Type: typ, WikidataID: qid, Normalized: norm, ComputerID: computer}
		obj, err := checkObject(ogKindEntity, owner, id, row, map[string]any{
			"entity_id": id, "name": name, "type": typ, "normalized": norm, "wikidata_id": qid, "computer_id": computer,
		}, now)
		if err != nil {
			return "", nil, nil, fmt.Errorf("entity %d: %w", n, err)
		}
		obj.ComputerID = computer
		c.row = &obj
		c.lex = lexiconEntity{EntityID: id, Canonical: canonical, Name: name, Type: typ, FirstSeen: now}
		out = append(out, c)
	}
	return pub, out, og, nil
}

// checkPublicAutopaperStory refuses a publication that is not owner's
// published Autopaper story on the public route: the read never shows any
// other, so no judgment about one is taken.
func (s *Service) checkPublicAutopaperStory(ctx context.Context, og *ObjectGraphStore, owner, pub string) error {
	pubs, err := s.relatedPublications(ctx, og, map[string]bool{pub: true})
	if err != nil {
		return storeFailed(err)
	}
	info, ok := pubs[pub]
	if !ok || !info.eligible || info.obj.OwnerID != owner {
		return storyNotPublished{fmt.Errorf("%s is not a published Autopaper story of owner %s", pub, owner)}
	}
	manifest, err := s.frontManifest(ctx, og, info.latest)
	if err != nil {
		return storeFailed(err)
	}
	if !publicationAccessPolicyAllowsPublicRoute(manifest.AccessPolicy) {
		return storyNotPublished{fmt.Errorf("%s is not on the public route, so its entities would never be shown", pub)}
	}
	return nil
}

// storyNotPublished marks a refusal of a story that is not owner's
// published Autopaper story on the public route; the route answers 409.
type storyNotPublished struct{ error }

func (e storyNotPublished) Unwrap() error { return e.error }

// HandleStoryEntities serves POST types.StoryEntitiesPath for a computer
// exporting its entity attachments, under the one bound-guest write rule.
func (h *Handler) HandleStoryEntities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req types.StoryEntitiesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if err := requireInternalOrBoundGuestWrite(r.Context(), h.guestLookup, r, req.OwnerID, req.ComputerID); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	n, err := h.service.RecordStoryEntities(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: story entities from %s: %v", strings.TrimSpace(req.ComputerID), err)
		status := http.StatusBadRequest
		var failed storeFailure
		var unpublished storyNotPublished
		switch {
		case errors.As(err, &failed):
			status = http.StatusServiceUnavailable
		case errors.As(err, &unpublished):
			// Not published (or not public) yet: a computer exports its own
			// story's record when the story publishes (review 324 F2).
			status = http.StatusConflict
		}
		writeJSON(w, status, apiError{Error: err.Error()})
		return
	}
	if req.ValidateOnly {
		writeJSON(w, http.StatusOK, map[string]any{"validated": n})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recorded": n})
}
