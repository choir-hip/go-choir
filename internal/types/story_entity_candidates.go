package types

import "time"

// The entity candidates read (docs/problems/related-stories-no-entity-layer-
// 2026-10-26.md, slice c0 §2). A computer sends a story's title and text and
// the phrases its own extraction proposed; corpusd matches the text against
// the owner's learned lexicon and answers with the known entities found, with
// their world facts, and the phrases that are not entities yet, ranked and
// bounded. Nothing is written. Story text is data: corpusd reads it only to
// match, and the answer carries names and titles, never the text.

// StoryEntityCandidatesPath is the candidates read, behind the proxy's
// bound-guest rule.
const StoryEntityCandidatesPath = "/internal/platform/story-entities/candidates"

// Bounds of the candidates read.
const (
	// EntityCandidatesMax is the most candidates an answer carries: two
	// System 1 calls of 12 questions.
	EntityCandidatesMax = 24
	// EntityCandidatesHubSlots is the most of them that may be hubs.
	EntityCandidatesHubSlots = 4
	// EntityFactsMaxStories and EntityFactsMaxCoEntities bound an entity's
	// world facts.
	EntityFactsMaxStories    = 5
	EntityFactsMaxCoEntities = 5
)

// The sources of a candidate.
const (
	// EntityCandidateExisting is an entity of the owner's lexicon found in
	// the story, by name, alias or a near form.
	EntityCandidateExisting = "existing"
	// EntityCandidateExtracted is a phrase the extraction proposed that is
	// not an entity yet.
	EntityCandidateExtracted = "extracted"
)

// EntityCandidatesRequest asks for a story's entity candidates.
type EntityCandidatesRequest struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	Title      string `json:"title"`
	Text       string `json:"text"`
	// Proposals are the computer's own extraction (entitytext.Propose with
	// its library's document frequencies). Corpusd extracts again from the
	// bounded text and takes only each proposal's Rarity, so a proposal the
	// text does not hold is never offered.
	Proposals []EntityProposal `json:"proposals,omitempty"`
}

// EntityProposal is one proposed phrase on the wire.
type EntityProposal struct {
	Phrase    string   `json:"phrase"`
	Count     int      `json:"count"`
	Sentences int      `json:"sentences,omitempty"`
	InTitle   bool     `json:"in_title,omitempty"`
	InLead    bool     `json:"in_lead,omitempty"`
	Acronym   bool     `json:"acronym,omitempty"`
	Rarity    float64  `json:"rarity,omitempty"`
	Score     float64  `json:"score,omitempty"`
	Aliases   []string `json:"aliases,omitempty"`
}

// EntityCandidatesResponse is the answer: the candidates, ranked (title
// first, then score) and bounded, and the lexicon's size.
type EntityCandidatesResponse struct {
	Candidates []EntityCandidate `json:"candidates"`
	Lexicon    EntityLexiconSize `json:"lexicon"`
}

// EntityLexiconSize is the size of an owner's learned lexicon.
type EntityLexiconSize struct {
	Entities  int `json:"entities"`
	Keys      int `json:"keys"`
	Stopwords int `json:"stopwords"`
}

// EntityCandidate is one candidate entity of a story.
type EntityCandidate struct {
	// Source is EntityCandidateExisting or EntityCandidateExtracted.
	Source string `json:"source"`
	// EntityID, Name and Type are an existing entity's.
	EntityID string `json:"entity_id,omitempty"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	// Phrase is the form the story uses most.
	Phrase    string  `json:"phrase"`
	Count     int     `json:"count"`
	Sentences int     `json:"sentences"`
	InTitle   bool    `json:"in_title,omitempty"`
	InLead    bool    `json:"in_lead,omitempty"`
	Acronym   bool    `json:"acronym,omitempty"`
	Rarity    float64 `json:"rarity"`
	Score     float64 `json:"score"`
	// Hub: an existing entity central to more stories than the related
	// walk's hub cut.
	Hub bool `json:"hub,omitempty"`
	// Ambiguous: the matched form is filed under more than one entity.
	Ambiguous bool `json:"ambiguous,omitempty"`
	// AliasCandidates are other forms of this candidate in the story (a
	// short form merged into it, or a near form of an existing entity).
	AliasCandidates []string `json:"alias_candidates,omitempty"`
	// Facts are an existing entity's world facts.
	Facts *EntityFacts `json:"facts,omitempty"`
}

// EntityFacts are what the object graph knows of an entity.
type EntityFacts struct {
	// Stories and CentralStories count the stories with a live mention of
	// it, and those where it is central (at most the edge read's bound).
	Stories        int       `json:"stories"`
	CentralStories int       `json:"central_stories"`
	FirstSeen      time.Time `json:"first_seen"`
	// PriorStories are up to EntityFactsMaxStories published stories that
	// mention it, newest first.
	PriorStories []EntityStoryRef `json:"prior_stories,omitempty"`
	// CoEntities are up to EntityFactsMaxCoEntities entities central to
	// those stories too, most shared first.
	CoEntities []EntityRef `json:"co_entities,omitempty"`
}

// EntityStoryRef names a published story.
type EntityStoryRef struct {
	PublicationID string `json:"publication_id"`
	Title         string `json:"title"`
}

// EntityRef names an entity.
type EntityRef struct {
	EntityID string `json:"entity_id"`
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
}
