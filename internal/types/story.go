package types

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Living stories (Cut 2 unit 2b,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md
// "Living stories"; pre-flight review A6 and A6').
//
// Captures reach management as one system record per poll that wrote new
// captures. Management opens a story with a Cast addressed to
// StoryOpenInbox: the store creates the story's document and trajectory in
// that Cast's commit (desk origin management, an empty system seed that is
// no owner's input, no work item) and delivers the Cast to the new
// texture:<doc>, whose wake starts Texture's first turn. Management feeds a
// story by a Cast or Note to its texture:<doc> and never writes a Texture.

// StoryOpenInbox is the address a management Cast names to open a story.
// The store rewrites it to the new story's texture:<doc> at commit; no
// record is ever stored under it.
const StoryOpenInbox = "texture:new"

// TrajectoryKindStory is a living story's trajectory. It has no work
// items, so it never settles because its work drained: a story stays live
// for every later Cast or Note management sends it.
const TrajectoryKindStory TrajectoryKind = "story"

// Subject refs a story's trajectory carries.
const (
	// StoryRefOriginDesk names the desk that opened the story (management).
	StoryRefOriginDesk = "origin_desk"
	// StoryRefOpeningRecord names the Cast record that opened it.
	StoryRefOpeningRecord = "opening_record"
	// StoryRefPublish is CastPublishAuto when every committed revision
	// publishes as its next version.
	StoryRefPublish = "publish"
)

// StorySeedRole marks a story's seed revision: the empty revision the
// opening commits because a document needs a head. It is no owner's input,
// no Texture's writing, and never publishes.
const StorySeedRole = "story_seed"

// CommitmentCaptures is the body of a capture arrival record.
//
// With the event layer's wakes on (event layer §15,
// docs/reports/event-layer-at-ingestion-design-2026-10-16.md), a poll's
// arrival is Held: it still names every capture on the ledger, but wakes
// no one, because its captures wait on their events. An event's record is
// a captures record too, from SourceID "event:<id>", naming the event's
// members with each one's own source, and carrying the Event.
type CommitmentCaptures struct {
	SourceID string           `json:"source_id"`
	Held     bool             `json:"held,omitempty"`
	Event    *CaptureEvent    `json:"event,omitempty"`
	Items    []CaptureArrival `json:"items"`
}

// Why an event's captures record woke management.
const (
	CaptureEventCorroborated = "corroborated"
	CaptureEventGrew         = "grew"
	CaptureEventOfficial     = "official_single_outlet"
	CaptureEventUnresolved   = "unresolved"
	// CaptureEventReleased: held captures released when production stopped
	// waiting on events (the setting turned off).
	CaptureEventReleased = "released"
)

// CaptureEvent is the world event an event's captures record names.
type CaptureEvent struct {
	EventID   string   `json:"event_id,omitempty"`
	Reason    string   `json:"reason"`
	Headline  string   `json:"headline,omitempty"`
	Origins   []string `json:"origins,omitempty"` // counted origins: outlets or wire tags
	FirstSeen string   `json:"first_seen,omitempty"`
	// Covered names the stories (texture:<doc>) that opened with or were
	// fed the event's other members; Declined, that every one of them
	// that was disposed of was declined. Both from the disposition index.
	Covered  []string `json:"covered,omitempty"`
	Declined bool     `json:"declined,omitempty"`
}

// Dispositions management gives captures (L2a R2-5).
const (
	DispositionFeed    = "feed"
	DispositionDecline = "decline"
)

// CommitmentDisposition is a disposing Note's structured body.
type CommitmentDisposition struct {
	Kind     string   `json:"kind"`
	Captures []string `json:"captures"`
	Reason   string   `json:"reason,omitempty"`
}

// CaptureArrival names one new capture: its id (a Ref's doc), title, URL
// and publish time. Never its body: management reads blocks with ReadDoc.
type CaptureArrival struct {
	CaptureID   string `json:"capture_id"`
	Title       string `json:"title,omitempty"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at,omitempty"`
	// Language is the source's first language; Script is a guess from the
	// item's own title and summary (han, kana, hangul, arabic, cyrillic,
	// latin, other), so a mixed-language feed is not mislabelled (L2a A5,
	// defect 5).
	Language string `json:"language,omitempty"`
	Script   string `json:"script,omitempty"`
	// SourceID is the capture's own source, set on an event's record,
	// whose items come from many sources; Decline routes by it.
	SourceID string `json:"source_id,omitempty"`
	// Tentative marks an event's member Jev matched below the join
	// confidence: shown with the event, never counted as its source.
	Tentative bool `json:"tentative,omitempty"`
}

// CaptureArrivalSystemAgentID is the provenance of a capture arrival
// record: the runtime speaking for the poll.
const CaptureArrivalSystemAgentID = "system:captures"

// CaptureArrivalSenderInbox is the sender inbox of a source's arrival
// records, so each source's arrivals form one conversation with
// management.
func CaptureArrivalSenderInbox(sourceID string) string {
	return "source:" + strings.TrimSpace(sourceID)
}

// CaptureArrivalRecordID is deterministic in the source and the set of
// capture ids, so a replayed poll that names the same captures is one
// record, not two.
func CaptureArrivalRecordID(sourceID string, captureIDs []string) string {
	ids := append([]string(nil), captureIDs...)
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceID) + "\x00" + strings.Join(ids, "\x00")))
	return "capture-arrival:" + strings.TrimSpace(sourceID) + ":" + hex.EncodeToString(sum[:12])
}

// StoryPublishesAuto reports a story trajectory opened with publish auto:
// every committed revision of its document publishes as its next version.
func StoryPublishesAuto(trajectory TrajectoryRecord) bool {
	return trajectory.Kind == TrajectoryKindStory && trajectory.SubjectRefs[StoryRefPublish] == CastPublishAuto
}

// Routes a computer reaches the platform by (through the proxy, which binds
// the guest to its owner and computer) to publish its stories (Cut 2 2b).
const (
	// StoryPublishPath publishes one committed revision of a story as its
	// version: the owner's publish path, run for the bound guest.
	StoryPublishPath = "/internal/platform/texture/publications"
	// StoryVerdictsPath records support verdicts in Store B, where the
	// published check overlay reads them.
	StoryVerdictsPath = "/internal/platform/support-verdicts"
	// StoryRelationsPath records the research desk's judged pair relations in
	// Store B as related_story edges, where the related-stories walk reads
	// them (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md).
	StoryRelationsPath = "/internal/platform/story-relations"
)

// The relations a judged pair can have, strongest first. A pair that is
// unrelated, or shares an entity only incidentally, gets no edge.
const (
	StoryRelationSameIncident     = "same_incident"
	StoryRelationLaterDevelopment = "later_development"
	StoryRelationSameSubject      = "same_subject"
)

// StoryRelationsRequest carries a computer's judged pair relations to Store B.
//
// ValidateOnly asks for every check the write runs and no write: the computer
// calls it before it puts a relation on its tape, so the tape holds only
// pairs the platform would take. The answer carries "validated", never
// "recorded".
type StoryRelationsRequest struct {
	OwnerID      string          `json:"owner_id"`
	ComputerID   string          `json:"computer_id"`
	Relations    []StoryRelation `json:"relations"`
	ValidateOnly bool            `json:"validate_only,omitempty"`
}

// StoryRelation is the part of a judged pair Store B keeps: two publication
// ids (pub-<32 hex>, the publication_id of the published search) and the relation. The probability, the System 1 receipt and
// the reason stay on the computer's tape.
type StoryRelation struct {
	StoryA   string `json:"story_a"`
	StoryB   string `json:"story_b"`
	Relation string `json:"relation"`
}

// StoryPublishRequest names the revision a computer publishes.
type StoryPublishRequest struct {
	DocID      string `json:"doc_id"`
	RevisionID string `json:"revision_id"`
}

// StoryVerdictsRequest carries a computer's support verdicts to Store B.
type StoryVerdictsRequest struct {
	OwnerID    string         `json:"owner_id"`
	ComputerID string         `json:"computer_id"`
	Verdicts   []StoryVerdict `json:"verdicts"`
}

// StoryVerdict is the public part of one support verdict: the pair key,
// the verdict, why, and who judged. The overlay joins it to a version's
// citations by pair key.
type StoryVerdict struct {
	Key     string `json:"key"`
	Verdict string `json:"verdict"`
	Why     string `json:"why,omitempty"`
	Judge   string `json:"judge"`
}
