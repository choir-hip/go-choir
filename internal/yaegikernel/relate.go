package yaegikernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// choir.Relate is the research desk's record of what a published story is
// about (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice
// a; design docs/reports/related-stories-phase3-research-review-2026-10-20.md
// §3.2 and §11 slice 1, the entity attachments; pair relations stay
// choir.RelateStories). The desk has read the story and asked System 1 for
// each candidate entity whether the story is centrally or incidentally about
// it; this verb records the desk's decision on the tape, and the computer
// exports it to Store B as choir.entity rows and mentions_entity edges. The
// probability is never the cell's: the host reads it from the System 1
// answer each attachment cites. This file is the one definition of a
// well-formed call, shared by the cell-side verb (fast refusal) and the host
// (authority).

// Bounds on one call.
const (
	RelateMaxEntities  = 12
	RelateMaxNameRunes = 120
)

// RelateEntity is one attachment: an existing entity by EntityID (ent:<hex>
// or wd:Q<n>, as an earlier Relate result or a refusal named it), or a new
// one by Name and Type with an optional WikidataID; its Role in the story;
// and the System 1 answer it rests on. Reason is the desk's one line; it is
// required when the role is not System 1's top choice.
type RelateEntity struct {
	EntityID    string `json:"entity_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	WikidataID  string `json:"wikidata_id,omitempty"`
	Role        string `json:"role"`
	JudgmentRef string `json:"judgment_ref"`
	QuestionID  string `json:"question_id"`
	Reason      string `json:"reason,omitempty"`
}

// ResolvedID is the attachment's entity id: its EntityID, or for a new
// entity its Wikidata id, else the id its name mints.
func (e RelateEntity) ResolvedID() string {
	switch {
	case e.EntityID != "":
		return e.EntityID
	case e.WikidataID != "":
		return types.WikidataEntityID(e.WikidataID)
	default:
		return types.MintedEntityID(e.Name)
	}
}

// RelatePayload defines ActionRelate: one story (its publication_id) and its
// attachments.
type RelatePayload struct {
	Story    string         `json:"story"`
	Entities []RelateEntity `json:"entities"`
}

// ValidateRelate checks a call against the bounds and returns it normalized:
// fields trimmed, role and type lower case. It names the first offending
// field. One entity may appear once per call, by id or by a new name (two
// new names that normalise alike are one entity).
func ValidateRelate(p RelatePayload) (RelatePayload, error) {
	p.Story = strings.TrimSpace(p.Story)
	if !publicationIDRE.MatchString(p.Story) {
		return RelatePayload{}, fmt.Errorf("choir.Relate: story %q is not a publication id (use the publication_id, pub-<32 hex>, of a published story)", p.Story)
	}
	if len(p.Entities) == 0 {
		return RelatePayload{}, fmt.Errorf("choir.Relate needs at least one entity")
	}
	if len(p.Entities) > RelateMaxEntities {
		return RelatePayload{}, fmt.Errorf("choir.Relate takes at most %d entities per call, got %d", RelateMaxEntities, len(p.Entities))
	}
	out := RelatePayload{Story: p.Story, Entities: make([]RelateEntity, 0, len(p.Entities))}
	seen := map[string]bool{}
	for i, e := range p.Entities {
		n := i + 1
		e.EntityID, e.Name = strings.TrimSpace(e.EntityID), strings.TrimSpace(e.Name)
		e.Type, e.WikidataID = strings.ToLower(strings.TrimSpace(e.Type)), strings.TrimSpace(e.WikidataID)
		e.Role = strings.ToLower(strings.TrimSpace(e.Role))
		e.JudgmentRef, e.QuestionID, e.Reason = strings.TrimSpace(e.JudgmentRef), strings.TrimSpace(e.QuestionID), strings.TrimSpace(e.Reason)
		if (e.EntityID == "") == (e.Name == "") {
			return RelatePayload{}, fmt.Errorf("entity %d: give entity_id or name: entity_id for an existing entity, name and type for a new one", n)
		}
		if e.EntityID != "" {
			if !types.EntityIDRE.MatchString(e.EntityID) {
				return RelatePayload{}, fmt.Errorf("entity %d: entity_id %q must be ent:<24 hex> or wd:Q<n>, as a Relate result or refusal named it", n, e.EntityID)
			}
			if e.Type != "" || e.WikidataID != "" {
				return RelatePayload{}, fmt.Errorf("entity %d: an existing entity is named by entity_id alone; type and wikidata_id describe a new one", n)
			}
		} else {
			if utf8.RuneCountInString(e.Name) > RelateMaxNameRunes || types.NormalizeEntityName(e.Name) == "" {
				return RelatePayload{}, fmt.Errorf("entity %d: name must be the entity's name, at most %d characters", n, RelateMaxNameRunes)
			}
			if !types.EntityTypes[e.Type] {
				return RelatePayload{}, fmt.Errorf("entity %d: type %q must be person, organization, place, product, work or law", n, e.Type)
			}
			if e.WikidataID != "" && !types.WikidataQIDRE.MatchString(e.WikidataID) {
				return RelatePayload{}, fmt.Errorf("entity %d: wikidata_id %q must be a Wikidata item id (Q<n>)", n, e.WikidataID)
			}
		}
		if !types.EntityRoles[e.Role] {
			return RelatePayload{}, fmt.Errorf("entity %d: role %q must be %s or %s (an entity the story is not about is not attached)", n, e.Role, types.EntityRoleCentral, types.EntityRoleIncidental)
		}
		if !strings.HasPrefix(e.JudgmentRef, "system1:") || len(e.JudgmentRef) == len("system1:") || len(e.JudgmentRef) > relateStoriesMaxRefLen {
			return RelatePayload{}, fmt.Errorf("entity %d: judgment_ref must be the receipt_ref (system1:<id>) of the choir.System1 call this role rests on", n)
		}
		if e.QuestionID == "" || len(e.QuestionID) > system1MaxIDLen || !system1IDRE.MatchString(e.QuestionID) {
			return RelatePayload{}, fmt.Errorf("entity %d: question_id must be the id of the question in that call that decided this role", n)
		}
		if r := utf8.RuneCountInString(e.Reason); r > RelateStoriesMaxReasonRunes {
			return RelatePayload{}, fmt.Errorf("entity %d: reason is at most %d characters, got %d", n, RelateStoriesMaxReasonRunes, r)
		}
		id := e.ResolvedID()
		if seen[id] {
			return RelatePayload{}, fmt.Errorf("entity %d: %s is named twice in this call; attach an entity once, with one role", n, id)
		}
		seen[id] = true
		out.Entities = append(out.Entities, e)
	}
	return out, nil
}

// relateEntities reads the cell's argument: a list of attachment maps (or the
// typed list). Unknown fields are refused, so a probability the cell supplies
// is an error, not silently dropped: the host reads it from the judgment.
func relateEntities(arg any) ([]RelateEntity, error) {
	if typed, ok := arg.([]RelateEntity); ok {
		return typed, nil
	}
	raw, err := jsonCellArg(arg)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") {
		return nil, fmt.Errorf("choir.Relate takes a story's publication_id and a list of {entity_id | name, type, wikidata_id, role, judgment_ref, question_id, reason} maps")
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var entities []RelateEntity
	if err := dec.Decode(&entities); err != nil {
		return nil, fmt.Errorf("choir.Relate takes a list of {entity_id | name, type, wikidata_id, role, judgment_ref, question_id, reason} maps; the probability is read from the System 1 answer, never supplied: %w", err)
	}
	return entities, nil
}

// Relate records what a published story is about: {story, attached:
// [{entity_id, name, role, new, probability, top, overrode, replayed}],
// exported, export_error}. Each attachment names an existing entity by
// entity_id, or a new one by name and type (and wikidata_id when you are
// sure of it), with its role, central or incidental, and rests on a
// choir.System1 answer of this run (receipt_ref and question id) whose
// choices include the role; the host reads the probability from it. A role
// System 1 did not rank first needs a reason of at least 20 characters. A new
// name that matches an existing entity is refused with that entity's id: attach
// it by entity_id. The record goes on the tape, then to the public graph; if
// the export fails the record stays and repeating the call exports it again.
// It may be off on this computer.
func (s *ChoirScope) Relate(story string, entities any) (JSON, error) {
	es, err := relateEntities(entities)
	if err != nil {
		return nil, err
	}
	p, err := ValidateRelate(RelatePayload{Story: story, Entities: es})
	if err != nil {
		return nil, err
	}
	var out JSON
	err = s.call(ActionRelate, p, &out)
	return out, err
}
