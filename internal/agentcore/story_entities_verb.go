package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// choir.Relate, the research desk's record of what a published story is
// about (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice a;
// design docs/reports/related-stories-phase3-research-review-2026-10-20.md
// §3.2, §6 and §11 slice 1). A research cell has read the story and asked
// System 1, per candidate entity, whether the story is centrally or
// incidentally about it; this host step checks each attachment against the
// System 1 answer it cites, asks the platform validate-only (which holds the
// entity rows and refuses a duplicate name, R1), writes one story.entities
// event on the computer's tape, and exports the attachments to Store B.
//
// What it guarantees, as choir.RelateStories does (story_relation_verb.go):
//   - research only, and only with the research.related_story setting on
//     (one switch for the related-stories graph; off is the rollback);
//   - every role rests on an answered System 1 question of this run whose
//     choices include it; the probability is read from that answer;
//   - the desk decides: a role System 1 did not rank first needs a reason of
//     at least 20 characters, and the record says it overrode;
//   - a new entity's id is the host's (types.MintedEntityID, or wd:Q<n>), so
//     two calls naming it alike name one node;
//   - the platform is asked first, so the tape holds no attachment that could
//     never become an edge; the tape is then the authority (R7) and the export
//     a re-runnable projection: a repeat records nothing new and exports
//     again, carrying the description of an entity this tape minted;
//   - nothing resumes (restart rule): a crash between the record and the
//     export leaves the record and no edge until the desk repeats the call.

const storyEntitiesPhaseRecorded = "recorded"

type relateAttached struct {
	EntityID    string  `json:"entity_id"`
	Name        string  `json:"name,omitempty"`
	Role        string  `json:"role"`
	New         bool    `json:"new"`
	Probability float64 `json:"probability"`
	Top         bool    `json:"top"`
	Overrode    bool    `json:"overrode"`
	Replayed    bool    `json:"replayed"`
}

type relateResult struct {
	Story       string           `json:"story"`
	Attached    []relateAttached `json:"attached"`
	Exported    bool             `json:"exported"`
	ExportError string           `json:"export_error,omitempty"`
}

// storyEntityAttachment is one attachment as the tape records it.
type storyEntityAttachment struct {
	EntityID    string  `json:"entity_id"`
	Name        string  `json:"name,omitempty"`
	Type        string  `json:"type,omitempty"`
	WikidataID  string  `json:"wikidata_id,omitempty"`
	New         bool    `json:"new,omitempty"`
	Role        string  `json:"role"`
	Probability float64 `json:"probability"`
	Top         bool    `json:"top"`
	Overrode    bool    `json:"overrode"`
	Reason      string  `json:"reason,omitempty"`
	JudgmentRef string  `json:"judgment_ref"`
	QuestionID  string  `json:"question_id"`
}

type storyEntitiesRecord struct {
	Computer    string                  `json:"computer_id"`
	Story       string                  `json:"story"`
	Attachments []storyEntityAttachment `json:"attachments"`
	Model       string                  `json:"model,omitempty"`
}

// StoryEntityPublisher is the optional third face of a StoryPublisher: it
// exports the research desk's entity attachments (choir.Relate).
// ValidateEntities runs every check the export runs and writes nothing; the
// verb calls it before the tape record. A publisher without these cannot
// check or export, and the verb records nothing.
type StoryEntityPublisher interface {
	ValidateEntities(ctx context.Context, req types.StoryEntitiesRequest) error
	PublishEntities(ctx context.Context, req types.StoryEntitiesRequest) error
}

func (rt *Runtime) relateEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	if deskRole != yaegikernel.SessionRoleResearch {
		return nil, fmt.Errorf("choir.Relate belongs to the research desk; this session is %q", deskRole)
	}
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("choir.Relate host unavailable")
	}
	owner, computer, runID := strings.TrimSpace(exec.OwnerID), strings.TrimSpace(exec.ComputerID), strings.TrimSpace(exec.RunID)
	if owner == "" || computer == "" || runID == "" {
		return nil, fmt.Errorf("choir.Relate needs a run with an owner and a computer")
	}
	if !rt.computerSettingOn(ctx, owner, computer, store.SettingResearchRelatedStory) {
		return nil, fmt.Errorf("choir.Relate is off on this computer: the owner turns on the %s setting", store.SettingResearchRelatedStory)
	}
	var req yaegikernel.RelatePayload
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return nil, fmt.Errorf("choir.Relate: decode the call: %w", err)
	}
	req, err := yaegikernel.ValidateRelate(req)
	if err != nil {
		return nil, err
	}

	rt.relationMu.Lock()
	defer rt.relationMu.Unlock()
	tape, err := rt.system1ChannelTape(ctx, owner, computer)
	if err != nil {
		return nil, fmt.Errorf("choir.Relate: read the tape: %w", err)
	}
	answered := map[string]types.EventRecord{}
	recorded := map[string]bool{}                // story, entity, role on the tape
	minted := map[string]storyEntityAttachment{} // entities this tape minted
	for _, ev := range tape {
		switch {
		case ev.Kind == types.EventSystem1Judgment && ev.Phase == system1PhaseAnswered && ev.RunID == runID:
			answered[ev.EventID] = ev
		case ev.Kind == types.EventStoryEntities && ev.Phase == storyEntitiesPhaseRecorded:
			var rec storyEntitiesRecord
			if json.Unmarshal(ev.Payload, &rec) != nil || (rec.Computer != "" && rec.Computer != computer) {
				continue
			}
			for _, a := range rec.Attachments {
				recorded[entityAttachmentKey(rec.Story, a.EntityID, a.Role)] = true
				if a.New {
					minted[a.EntityID] = a
				}
			}
		}
	}

	// Check every attachment before recording any.
	var attachments []storyEntityAttachment
	result := relateResult{Story: req.Story}
	model := ""
	for i, e := range req.Entities {
		basis, err := system1ChoiceBasis(e.JudgmentRef, e.QuestionID, e.Role, answered, computer)
		if err != nil {
			return nil, fmt.Errorf("choir.Relate: entity %d: %w; ask with the choices %s, %s and not_about", i+1, err, types.EntityRoleCentral, types.EntityRoleIncidental)
		}
		if !basis.top && utf8.RuneCountInString(e.Reason) < yaegikernel.RelateStoriesMinOverrideReasonRunes {
			return nil, fmt.Errorf("choir.Relate: entity %d: System 1 did not rank %s first (it gave %.2f); you decide, so give a reason of at least %d characters", i+1, e.Role, basis.probability, yaegikernel.RelateStoriesMinOverrideReasonRunes)
		}
		if model == "" {
			model = basis.model
		}
		a := storyEntityAttachment{EntityID: e.ResolvedID(), Role: e.Role, Probability: basis.probability, Top: basis.top, Overrode: !basis.top,
			Reason: e.Reason, JudgmentRef: e.JudgmentRef, QuestionID: e.QuestionID}
		if e.EntityID == "" {
			a.Name, a.Type, a.WikidataID, a.New = e.Name, e.Type, e.WikidataID, true
		}
		attachments = append(attachments, a)
		result.Attached = append(result.Attached, relateAttached{EntityID: a.EntityID, Name: a.Name, Role: a.Role, New: a.New,
			Probability: a.Probability, Top: a.Top, Overrode: a.Overrode, Replayed: recorded[entityAttachmentKey(req.Story, a.EntityID, a.Role)]})
	}

	export := types.StoryEntitiesRequest{OwnerID: owner, ComputerID: computer, Story: req.Story}
	for _, a := range attachments {
		e := types.StoryEntity{EntityID: a.EntityID, Role: a.Role}
		switch {
		case a.New:
			e.Name, e.Type, e.WikidataID, e.New = a.Name, a.Type, a.WikidataID, true
		default:
			// An entity this tape minted carries its description, so a repeat
			// after a failed export can still write its row.
			if m, ok := minted[a.EntityID]; ok {
				e.Name, e.Type, e.WikidataID, e.New = m.Name, m.Type, m.WikidataID, true
			}
		}
		export.Entities = append(export.Entities, e)
	}

	publisher, ok := rt.storyPublisher().(StoryEntityPublisher)
	if !ok || publisher == nil {
		return nil, fmt.Errorf("choir.Relate: nothing was recorded: no platform to check the entities with (choir.platform_url is not set on this computer)")
	}
	// Ask the platform before anything is recorded; the lock is held so a
	// concurrent call cannot record the same attachment meanwhile.
	vctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storyRelationExportTimeout)
	err = publisher.ValidateEntities(vctx, export)
	cancel()
	if err != nil {
		log.Printf("story entities: the platform did not take %d attachments of %s on run %s: %v", len(export.Entities), req.Story, runID, err)
		return nil, fmt.Errorf("choir.Relate: nothing was recorded, the platform did not take the entities: %w", err)
	}

	// Record: the tape first, only what it does not hold yet.
	var fresh []storyEntityAttachment
	for i, a := range attachments {
		if result.Attached[i].Replayed {
			continue
		}
		fresh = append(fresh, a)
	}
	if len(fresh) > 0 {
		if err := rt.appendStoryEntitiesEvent(ctx, exec, owner, computer, runID, storyEntitiesRecord{Computer: computer, Story: req.Story, Attachments: fresh, Model: model}); err != nil {
			log.Printf("story entities: record %s on run %s: %v", req.Story, runID, err)
			return nil, fmt.Errorf("choir.Relate: the attachments could not be recorded on the tape, so they are not exported: %w", err)
		}
	}

	// Export: a projection of the tape; a failure leaves the record.
	ectx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storyRelationExportTimeout)
	err = publisher.PublishEntities(ectx, export)
	cancel()
	if err != nil {
		result.ExportError = err.Error()
		log.Printf("story entities: export of %d attachments of %s on run %s failed: %v", len(export.Entities), req.Story, runID, err)
	} else {
		result.Exported = true
	}
	return json.Marshal(result)
}

func entityAttachmentKey(story, entity, role string) string {
	return story + "\x00" + entity + "\x00" + role
}

// system1ChoiceBasis finds the System 1 answer an act rests on and reads the
// probability of want from it. want must be one of the question's choices
// (spaces, hyphens and case do not matter).
func system1ChoiceBasis(judgmentRef, questionID, want string, answered map[string]types.EventRecord, computer string) (relationBasis, error) {
	id := strings.TrimPrefix(judgmentRef, "system1:")
	ev, ok := answered[id]
	if !ok {
		return relationBasis{}, fmt.Errorf("no System 1 answer with receipt %q in this run; ask choir.System1 first and cite its receipt_ref", judgmentRef)
	}
	var rec struct {
		Computer string          `json:"computer_id"`
		Model    string          `json:"model"`
		Result   json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(ev.Payload, &rec); err != nil || (rec.Computer != "" && rec.Computer != computer) {
		return relationBasis{}, fmt.Errorf("receipt %q is not an answer of this computer", judgmentRef)
	}
	var result system1Result
	if err := json.Unmarshal(rec.Result, &result); err != nil {
		return relationBasis{}, fmt.Errorf("receipt %q holds an answer that did not decode: %w", judgmentRef, err)
	}
	for _, a := range result.Answers {
		if a.ID != questionID {
			continue
		}
		if a.Error != "" {
			return relationBasis{}, fmt.Errorf("System 1 did not answer question %q: %s", questionID, a.Error)
		}
		choice := ""
		for _, c := range a.Choices {
			if normalizeRelationChoice(c) == want {
				choice = c
			}
		}
		if choice == "" {
			return relationBasis{}, fmt.Errorf("%q is not one of question %q's choices %v", want, questionID, a.Choices)
		}
		prob, ok := a.Probabilities[choice]
		if !ok {
			return relationBasis{}, fmt.Errorf("System 1 gave no probability for %q on question %q", choice, questionID)
		}
		top := true
		for c, other := range a.Probabilities {
			if c != choice && other > prob {
				top = false
			}
		}
		return relationBasis{probability: prob, top: top, model: rec.Model}, nil
	}
	return relationBasis{}, fmt.Errorf("receipt %q has no answer to question %q", judgmentRef, questionID)
}

func (rt *Runtime) appendStoryEntitiesEvent(ctx context.Context, exec toolregistry.ExecutionContext, owner, computer, runID string, rec storyEntitiesRecord) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	ev := types.EventRecord{EventID: uuid.New().String(), Timestamp: time.Now().UTC(), RunID: runID, OwnerID: owner, AgentID: exec.AgentID,
		ChannelID: system1Channel(computer), Kind: types.EventStoryEntities, Phase: storyEntitiesPhaseRecorded, Payload: raw}
	return rt.store.AppendEvent(ctx, &ev)
}
