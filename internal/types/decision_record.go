package types

import "time"

// The decision record (docs/definitions/choir-records-and-measurement-2026-11-23.md,
// station R0): one append-only row per capture and stage, so the question
// "what was decided about this capture, by whom, from which candidates" can be
// answered a week later.
//
// A row is written by host code from what it already observes of a decision,
// never by the decider: management writes nothing extra for it (owner,
// 2026-11-24: management is overloaded and decides too much already). The
// stage and the decider are plain words, so the redesign's stages (copy
// check, embed, neighbours, new-words read, grouping, routing, batch call)
// write the same row with no schema change.

// Decider kinds: who made a decision, generically.
const (
	DeciderRule        = "rule"
	DeciderHostService = "host_service"
	DeciderSystem1     = "system1"
	DeciderModelPool   = "model_pool"
	DeciderDesk        = "desk"
)

// Stages instrumented so far.
const (
	// DecisionStageManagementRoute is today's management choice for a
	// capture: feed an existing story, open a story, or decline. It is the
	// baseline the redesign's stages are compared with.
	DecisionStageManagementRoute = "management_route"
	// DecisionStagePageRender is one render of the front list: the stories
	// in the order the page ranks them, each with its score and the inputs to
	// it. The "capture" is the page ("front"); the decider is the rank rule
	// (internal/frontrank), which the pages and the daily digest also run.
	DecisionStagePageRender = "page_render"
	// DecisionRenderedPage is the decision word of a page render and
	// DecisionPageID the id of the page rendered.
	DecisionRenderedPage = "render"
	DecisionPageID       = "front"
)

// Decisions of the management route stage.
const (
	DecisionFeed    = "feed"
	DecisionOpen    = "open"
	DecisionDecline = "decline"
)

// DecisionRecord is one row of the decision record.
type DecisionRecord struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	CaptureID  string `json:"capture_id"`
	Stage      string `json:"stage"`
	// Decision is the stage's word for the choice (feed, open, decline).
	Decision string `json:"decision"`
	// Target is what the decision pointed at: the story fed or opened, "" for
	// none.
	Target string `json:"target,omitempty"`
	// DeciderKind is one of the Decider constants; Decider names it (a desk,
	// a rule, a service, a model pool).
	DeciderKind string `json:"decider_kind"`
	Decider     string `json:"decider"`
	// RunID and CommandID say which run and which committed command made
	// the decision; a capture decided twice has two rows.
	RunID     string `json:"run_id,omitempty"`
	CommandID string `json:"command_id,omitempty"`
	// Candidates are the ids shown to the decider for this capture;
	// CandidatesShown is how many the decider saw in all when the list is
	// shorter than that.
	Candidates      []string `json:"candidates,omitempty"`
	CandidatesShown int      `json:"candidates_shown,omitempty"`
	// Probability is the decider's stated probability where one exists; nil
	// where it gave none.
	Probability *float64  `json:"probability,omitempty"`
	At          time.Time `json:"at"`
	// Shown is a page render's ranked list, first story first; empty for the
	// stages that decide one capture.
	Shown []DecisionShown `json:"shown,omitempty"`
	// Rule names the rule that ordered Shown and what it leaves out.
	Rule string `json:"rule,omitempty"`
}

// DecisionShown is one story of a page render with the inputs its score was
// made from, so the order can be rebuilt later from the row alone. A count
// that was unknown to the page is nil.
type DecisionShown struct {
	Rank          int      `json:"rank"`
	PublicationID string   `json:"publication_id"`
	Score         *float64 `json:"score,omitempty"`
	Demoted       bool     `json:"demoted,omitempty"`
	Importance    *float64 `json:"importance,omitempty"`
	Promo         bool     `json:"promo,omitempty"`
	SourceCount   *float64 `json:"source_count,omitempty"`
	Words         *float64 `json:"words,omitempty"`
	VersionCount  float64  `json:"version_count"`
	UpdatedAt     string   `json:"updated_at"`
	Section       string   `json:"section,omitempty"`
	Title         string   `json:"title"`
}

// DecisionFilter selects rows to read.
type DecisionFilter struct {
	CaptureID string
	Stage     string
	Since     time.Time
	Limit     int
}
