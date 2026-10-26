package types

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

// The entity layer of the Autopaper object graph
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md; ontology
// docs/reports/claims-events-entities-ontology-2026-10-14.md §4.2, §5.2 and
// D1 to D5; related stories, phase 3, §3.2 and §6 R1). An entity is a person,
// organization, place, product, work or law a published story is about. Its
// id is its Wikidata QID when research links one (wd:Q<n>, D1 and D5),
// otherwise minted from its normalised name (ent:<24 hex>), so two names that
// differ only in case, punctuation, a possessive, a leading article or a
// company suffix are one node. Store B holds the rows (D3, D4: the paper's
// entities are public); the research desk's tape record is the authority.

// StoryEntitiesPath records a published story's entity attachments in Store
// B as choir.entity rows and mentions_entity edges.
const StoryEntitiesPath = "/internal/platform/story-entities"

// The roles an entity plays in a story. Only a central entity creates
// related-story links (phase 3 R2: two stories that both mention an entity
// incidentally are not related).
const (
	EntityRoleCentral    = "central"
	EntityRoleIncidental = "incidental"
)

// EntityRoles are the roles an attachment may have.
var EntityRoles = map[string]bool{EntityRoleCentral: true, EntityRoleIncidental: true}

// The entity types (ontology §4.2).
const (
	EntityTypePerson       = "person"
	EntityTypeOrganization = "organization"
	EntityTypePlace        = "place"
	EntityTypeProduct      = "product"
	EntityTypeWork         = "work"
	EntityTypeLaw          = "law"
)

// EntityTypes are the types a new entity may have.
var EntityTypes = map[string]bool{
	EntityTypePerson: true, EntityTypeOrganization: true, EntityTypePlace: true,
	EntityTypeProduct: true, EntityTypeWork: true, EntityTypeLaw: true,
}

// EntityIDRE is an entity id: minted (ent:<24 hex>) or Wikidata (wd:Q<n>).
var EntityIDRE = regexp.MustCompile(`^(ent:[0-9a-f]{24}|wd:Q[1-9][0-9]{0,11})$`)

// EntityNameMaxRunes bounds a new entity's name, at the verb and at the
// corpusd route alike.
const EntityNameMaxRunes = 120

// WikidataQIDRE is a Wikidata item id.
var WikidataQIDRE = regexp.MustCompile(`^Q[1-9][0-9]{0,11}$`)

// entityNameSuffixes are company forms dropped from the end of a name.
var entityNameSuffixes = map[string]bool{
	"inc": true, "incorporated": true, "corp": true, "corporation": true, "co": true,
	"ltd": true, "llc": true, "plc": true, "pbc": true, "ag": true, "sa": true, "gmbh": true,
}

// NormalizeEntityName folds a name to the form entity identity compares:
// lower case, a trailing possessive dropped, periods and apostrophes removed,
// other punctuation a space, a leading "the" and trailing company forms
// dropped, spaces collapsed. Letters keep their diacritics, so "Vučić" and
// "Vucic" stay two forms (an alias, which this layer does not match yet).
func NormalizeEntityName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	for _, poss := range []string{"'s", "’s", "s'", "s’"} {
		if strings.HasSuffix(s, poss) {
			if poss[0] == 's' {
				s = strings.TrimSuffix(s, poss[1:]) // "states'" -> "states"
			} else {
				s = strings.TrimSuffix(s, poss)
			}
			break
		}
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '.' || r == '\'' || r == '’':
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	words := strings.Fields(b.String())
	if len(words) > 0 && words[0] == "the" {
		words = words[1:]
	}
	for len(words) > 0 && entityNameSuffixes[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// MintedEntityID is the id of an entity research names without a Wikidata
// id: derived from its normalised name, "" when the name normalises to
// nothing.
func MintedEntityID(name string) string {
	norm := NormalizeEntityName(name)
	if norm == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("choir.entity\x00" + norm))
	return "ent:" + hex.EncodeToString(sum[:])[:24]
}

// StoryPublicationID is the publication id corpusd gives the story of
// owner's document docID (internal/platform publicationIdentityFor; a test
// pins the two together), so a computer names its own story without a read.
func StoryPublicationID(ownerID, docID string) string {
	sum := sha256.Sum256([]byte("texture-publication\x00" + ownerID + "\x00" + docID))
	return "pub-" + hex.EncodeToString(sum[:])[:32]
}

// WikidataEntityID is the id of an entity linked to a Wikidata item.
func WikidataEntityID(qid string) string { return "wd:" + strings.TrimSpace(qid) }

// StoryEntitiesRequest carries one published story's entity attachments to
// Store B. ValidateOnly asks for every check the write runs and no write: the
// computer calls it before the tape record, so the tape holds only
// attachments the platform would take. The answer carries "validated",
// never "recorded".
type StoryEntitiesRequest struct {
	OwnerID      string        `json:"owner_id"`
	ComputerID   string        `json:"computer_id"`
	Story        string        `json:"story"`
	Entities     []StoryEntity `json:"entities"`
	ValidateOnly bool          `json:"validate_only,omitempty"`
	// Pending marks a story not published yet (slice c): with ValidateOnly
	// every entity check runs and only the published-story check is
	// skipped. A pending request is never a write.
	Pending bool `json:"pending,omitempty"`
	// Review is the review that produced the attachments (slice c0 §6): its
	// counts, and what it taught the lexicon. With a review the request may
	// carry no attachments (a story about none of its candidates).
	Review *StoryEntityReview `json:"review,omitempty"`
}

// StoryEntity is the part of an attachment Store B keeps: the entity id and
// the role, and for an entity the computer minted (New), its name, type and
// Wikidata id so the platform can write its row. The probability, the System
// 1 receipt and the reason stay on the computer's tape.
type StoryEntity struct {
	EntityID   string `json:"entity_id"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	WikidataID string `json:"wikidata_id,omitempty"`
	New        bool   `json:"new,omitempty"`
	Role       string `json:"role"`
}
