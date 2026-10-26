package types

// The entity lexicon's learning loop and growth read
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice c0 §6
// and §7). A story's entities export carries the review that produced it.

// The learning bounds.
const (
	// EntityStopwordStories is how many distinct stories must judge a
	// phrase not an entity before it is stopped (N).
	EntityStopwordStories = 3
	// EntityStopwordMinP is the least probability of "not an entity" that
	// counts as evidence.
	EntityStopwordMinP = 0.8
	// EntityStopwordRevokeDays is how long a revoked stopword is not
	// relearned.
	EntityStopwordRevokeDays = 30
	// EntityAliasesMax bounds one entity's learned aliases.
	EntityAliasesMax = 16
	// EntityReviewMaxItems bounds a review's not-entities and its aliases.
	EntityReviewMaxItems = 48
	// EntityNoveltyYesP is the novelty answer's p(yes) at which research may
	// name an entity the analysis missed, and at which a review counts as a
	// novelty "yes" in the growth read.
	EntityNoveltyYesP = 0.35
	// EntityGrowthMaxDays bounds the growth read's window.
	EntityGrowthMaxDays = 90
)

// StoryEntityReview is one review of a story's entities: what the analysis
// offered and research attached, by source, and what it taught the lexicon.
type StoryEntityReview struct {
	// RecordID is the story.entities tape record's id: a re-export of one
	// record is one review.
	RecordID string            `json:"record_id"`
	Counts   StoryEntityCounts `json:"counts"`
	// CategoryGaps are the categories research suggested when System 1
	// answered "none of these fit".
	CategoryGaps []string `json:"category_gaps,omitempty"`
	// NoveltyYes is System 1's p(yes) that the story is centrally about a
	// subject the candidates miss.
	NoveltyYes float64 `json:"novelty_yes"`
	// NotEntities are the phrases System 1 judged not an entity.
	NotEntities []StoryEntityNotEntity `json:"not_entities,omitempty"`
	// Aliases are the forms research's attachments taught.
	Aliases []StoryEntityAlias `json:"aliases,omitempty"`
}

// StoryEntityCounts are a review's counts.
type StoryEntityCounts struct {
	// Candidates the analysis offered, and Hits among them (existing
	// entities).
	Candidates int `json:"candidates"`
	Hits       int `json:"hits"`
	// Attachments by source.
	Existing      int `json:"existing"`
	Extracted     int `json:"extracted"`
	ResearchNamed int `json:"research_named"`
	Overridden    int `json:"overridden"`
	CategoryGap   int `json:"category_gap"`
	// ExistingUnmatched counts attachments of existing entities the
	// analysis did not offer (the alias gap).
	ExistingUnmatched int `json:"existing_unmatched"`
}

// StoryEntityNotEntity is a phrase System 1 judged not an entity, with its
// probability.
type StoryEntityNotEntity struct {
	Phrase string  `json:"phrase"`
	P      float64 `json:"p"`
}

// StoryEntityAlias is a form of an entity.
type StoryEntityAlias struct {
	EntityID string `json:"entity_id"`
	Alias    string `json:"alias"`
}

// EntityGrowthPath is the public growth read.
const EntityGrowthPath = "/api/platform/entities/growth"

// EntityGrowthResponse is the growth read: a series per UTC day, oldest
// first, and the lexicon now.
type EntityGrowthResponse struct {
	Days     []EntityGrowthDay `json:"days"`
	Lexicon  EntityLexiconSize `json:"lexicon"`
	Stoplist []string          `json:"stoplist"`
}

// EntityGrowthDay is one UTC day of the learning loop.
type EntityGrowthDay struct {
	Day             string            `json:"day"`
	StoriesReviewed int               `json:"stories_reviewed"`
	Attachments     StoryEntityCounts `json:"attachments"`
	// HitRate is gazetteer hits over attachments.
	HitRate float64 `json:"hit_rate"`
	// ExistingUnmatched and its share of attachments.
	ExistingUnmatched      int     `json:"existing_unmatched"`
	ExistingUnmatchedShare float64 `json:"existing_unmatched_share"`
	// Minted entities, and AliasSprawl among them: a name contained in, or
	// containing, another entity's name.
	Minted           int     `json:"minted"`
	AliasSprawl      int     `json:"alias_sprawl"`
	AliasSprawlShare float64 `json:"alias_sprawl_share"`
	// KeywordsAdded are entities minted plus aliases learned.
	KeywordsAdded  int `json:"keywords_added"`
	AliasesLearned int `json:"aliases_learned"`
	StopwordsAdded int `json:"stopwords_added"`
	// CategoryGaps counts the suggested categories by name.
	CategoryGaps map[string]int `json:"category_gaps"`
	// NoveltyYesRate is the share of reviews whose novelty answer is at
	// least EntityNoveltyYesP.
	NoveltyYesRate float64 `json:"novelty_yes_rate"`
}
